// Package inbox is the one place a turn picks up anything that arrives while
// it is running: a line you type, a file dropped in the session's inbox
// directory by "kvit-coder steer", or an event from a background process.
//
// The agent loop drains it once per iteration. There is no file watcher: the
// drain reads the directory as part of the same call, so the drain is the poll.
package inbox

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Kind says what a message is, which decides how the loop presents it to the
// model.
type Kind string

const (
	// KindUserLine is something a person said mid-turn. It reaches the model
	// as a user message.
	KindUserLine Kind = "user_line"
	// KindProcessEvent is a background process exiting, or matching a pattern
	// something asked to be told about. It reaches the model as a reminder
	// appended to the last tool result.
	KindProcessEvent Kind = "process_event"
	// KindNotice is anything else kvit-coder wants the model to know.
	KindNotice Kind = "notice"
)

// Message is one item waiting for the turn.
type Message struct {
	Kind Kind      `json:"kind"`
	Text string    `json:"text"`
	At   time.Time `json:"at"`
}

const (
	// maxQueued bounds the in-memory queue. Someone holding a key down must
	// not be able to grow the process without limit; the oldest goes first,
	// because the newest is what they meant.
	maxQueued = 100
	// maxFileSize is the largest inbox file that is read. Anything bigger is
	// a mistake — a log redirected into the inbox, say — and is skipped.
	maxFileSize = 64 * 1024
)

// Inbox holds messages pushed in this process and picks up files dropped in
// its directory by another one.
type Inbox struct {
	dir string

	mu     sync.Mutex
	queue  []Message
	signal chan struct{}
	// asking counts the prompts currently waiting for an answer. The line
	// reader consults it so it does not report a prompt's answer as queued
	// steering, which read as the answer having been swallowed.
	asking int
	// askNotify, when set, is told when the first prompt starts waiting and
	// when the last one stops. See SetAskNotifier.
	askNotify func(bool)

	// pauseRequested is set by an empty Enter at the terminal: the turn
	// should stop at its next iteration boundary and ask for steering.
	// The loop consumes it with TakePause.
	pauseRequested bool
	// pauseMode is true only while the pause prompt's Ask is blocked, so
	// the line reader can tell an empty Enter that resumes the pause from
	// one that must stay ignored (a Question) or request a pause.
	pauseMode bool

	// Log, when set, is called for something worth saying out loud: an inbox
	// file too large to read, or one that could not be read at all.
	Log func(string)
}

// New creates an inbox backed by dir, which is a session's inbox directory. A
// empty dir gives an inbox with no directory half, which is what a test or a
// benchmark run wants.
func New(dir string) *Inbox {
	return &Inbox{dir: dir, signal: make(chan struct{}, 1)}
}

// SetAskNotifier installs a function called with true when a prompt starts
// waiting for someone to type an answer and false when the wait ends, however
// it ends. cmd/kvit-coder uses it to put the terminal's window title into its
// "waiting on you" state for as long as the wait lasts, since the front end
// is blocked on the turn and cannot see that it has stopped to ask something.
//
// Only the outermost prompt is reported: the count is what decides, so a
// prompt nested inside another would not clear the state early.
func (i *Inbox) SetAskNotifier(fn func(asking bool)) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.askNotify = fn
}

// Awaiting reports whether a prompt is waiting for someone to type an answer.
// The next line typed will be taken as that answer rather than reaching the
// model as steering.
func (i *Inbox) Awaiting() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.asking > 0
}

// RequestPause asks the turn to stop at its next iteration boundary and take
// steering. It reports whether the request is new, so the reader prints its
// notice once rather than on every empty Enter. A new request also fires the
// signal, so a tool blocked in Observe.wait wakes early and parks at the
// boundary instead of holding the pause for its full max_wait; Ask waiters
// just see a spurious wakeup and loop, since no line was pushed.
func (i *Inbox) RequestPause() bool {
	i.mu.Lock()
	if i.pauseRequested {
		i.mu.Unlock()
		return false
	}
	i.pauseRequested = true
	i.mu.Unlock()
	select {
	case i.signal <- struct{}{}:
	default:
	}
	return true
}

// PauseRequested peeks at a pending pause without consuming it. The progress
// watcher uses it to hold dots from the empty Enter until the pause prompt
// is up; TakePause is what consumes the request at the boundary.
func (i *Inbox) PauseRequested() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.pauseRequested
}

// TakePause consumes a pause request, reporting whether one was pending. The
// loop calls it at each iteration boundary.
func (i *Inbox) TakePause() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	if !i.pauseRequested {
		return false
	}
	i.pauseRequested = false
	return true
}

// SetPauseMode records whether the pause prompt is up waiting for a line.
func (i *Inbox) SetPauseMode(on bool) {
	i.mu.Lock()
	defer i.mu.Unlock()
	i.pauseMode = on
}

// PauseMode reports whether the pause prompt is up. An empty Enter then is a
// resume answer; otherwise it requests a pause or stays ignored.
func (i *Inbox) PauseMode() bool {
	i.mu.Lock()
	defer i.mu.Unlock()
	return i.pauseMode
}

// Dir is the directory this inbox picks files up from.
func (i *Inbox) Dir() string { return i.dir }

// Push adds a message from this process. It never blocks.
func (i *Inbox) Push(m Message) {
	if m.At.IsZero() {
		m.At = time.Now()
	}
	if m.Kind == "" {
		m.Kind = KindNotice
	}

	i.mu.Lock()
	i.queue = append(i.queue, m)
	if len(i.queue) > maxQueued {
		i.queue = i.queue[len(i.queue)-maxQueued:]
	}
	i.mu.Unlock()

	select {
	case i.signal <- struct{}{}:
	default:
	}
}

// Signal fires when something is pushed into this process's queue. A blocking
// tool selects on it to wake early. It says nothing about files dropped in the
// directory, which are noticed by the next Drain.
func (i *Inbox) Signal() <-chan struct{} { return i.signal }

// Drain returns everything waiting and empties the inbox: first what this
// process pushed, then the files in the directory, oldest name first. A file
// is read and then unlinked, so nothing is delivered twice.
func (i *Inbox) Drain() []Message {
	i.mu.Lock()
	queued := i.queue
	i.queue = nil
	i.mu.Unlock()

	// Clear a stale signal so a later blocking wait does not wake for messages
	// this call already took.
	select {
	case <-i.signal:
	default:
	}

	return append(queued, i.drainDir()...)
}

// PendingCount reports how many inbox files are waiting in dir without
// removing anything, so a watcher sitting at a turn boundary can decide
// whether there is anything worth waking for. It applies the same filter as
// the drain — directories and dotfiles (including half-written .partial-*)
// do not count — and missing directory means none.
func PendingCount(dir string) int {
	if dir == "" {
		return 0
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		n++
	}
	return n
}

func (i *Inbox) drainDir() []Message {
	if i.dir == "" {
		return nil
	}
	entries, err := os.ReadDir(i.dir)
	if err != nil {
		return nil
	}

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		names = append(names, e.Name())
	}
	// Names start with the nanosecond the producer wrote them, so sorting by
	// name is sorting by arrival.
	sort.Strings(names)

	var out []Message
	for _, name := range names {
		path := filepath.Join(i.dir, name)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		if info.Size() > maxFileSize {
			i.log(fmt.Sprintf("inbox: skipping %s, %d bytes is over the %d-byte limit", name, info.Size(), maxFileSize))
			_ = os.Remove(path)
			continue
		}
		data, err := os.ReadFile(path)
		if err != nil {
			i.log(fmt.Sprintf("inbox: could not read %s: %v", name, err))
			continue
		}
		_ = os.Remove(path)

		if m, ok := parse(data); ok {
			out = append(out, m)
		}
	}
	return out
}

func (i *Inbox) log(msg string) {
	if i.Log != nil {
		i.Log(msg)
	}
}

// parse reads an inbox file. Files written by kvit-coder are JSON; a file
// written by hand is taken as a line of steering, so dropping a plain text
// file in the directory works.
func parse(data []byte) (Message, bool) {
	var m Message
	if err := json.Unmarshal(data, &m); err == nil && m.Kind != "" {
		if m.At.IsZero() {
			m.At = time.Now()
		}
		return m, true
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return Message{}, false
	}
	return Message{Kind: KindUserLine, Text: text, At: time.Now()}, true
}

// Deliver writes a message into another process's inbox directory. It writes
// under a temporary name and renames into place, so a turn draining the
// directory never reads a half-written file.
func Deliver(dir string, m Message) (string, error) {
	if m.At.IsZero() {
		m.At = time.Now()
	}
	if m.Kind == "" {
		m.Kind = KindUserLine
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", fmt.Errorf("failed to marshal inbox message: %w", err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create inbox directory: %w", err)
	}

	tmp, err := os.CreateTemp(dir, ".partial-*")
	if err != nil {
		return "", fmt.Errorf("failed to write inbox message: %w", err)
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to write inbox message: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to write inbox message: %w", err)
	}

	// The name carries the arrival time so a drain reads messages in order.
	final := filepath.Join(dir, fmt.Sprintf("%d-%s", m.At.UnixNano(), filepath.Base(tmpName)[len(".partial-"):]))
	if err := os.Rename(tmpName, final); err != nil {
		os.Remove(tmpName)
		return "", fmt.Errorf("failed to deliver inbox message: %w", err)
	}
	return final, nil
}

// AskOutcome says why Ask returned.
type AskOutcome int

const (
	// AskAnswered means someone typed a line.
	AskAnswered AskOutcome = iota
	// AskCancelled means the turn was cancelled before anyone answered.
	AskCancelled
	// AskTimedOut means the time allowed ran out.
	AskTimedOut
)

// Ask puts a question to whoever is at the terminal and returns the first line
// typed after it.
//
// Every prompt kvit-coder makes goes through here, because two readers of one
// terminal is a race for each keystroke and the loser waits forever. There is
// one reader of stdin in the process — the one filling this inbox — and a
// prompt claims the next line from it rather than opening the terminal again.
//
// A line typed before the question was asked was not answering it, so it is
// held aside and put back for the loop to treat as ordinary steering. A
// timeout of zero or less waits indefinitely.
func (i *Inbox) Ask(ctx context.Context, out io.Writer, prompt string, timeout time.Duration) (string, AskOutcome) {
	i.mu.Lock()
	i.asking++
	first, notify := i.asking == 1, i.askNotify
	i.mu.Unlock()
	if first && notify != nil {
		notify(true)
	}

	var notForUs []Message
	defer func() {
		i.mu.Lock()
		i.asking--
		last, notify := i.asking == 0, i.askNotify
		i.mu.Unlock()
		if last && notify != nil {
			notify(false)
		}
		for _, m := range notForUs {
			i.Push(m)
		}
	}()
	notForUs = append(notForUs, i.Drain()...)

	askedAt := time.Now()
	if out != nil && prompt != "" {
		fmt.Fprint(out, prompt)
	}

	var deadline <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		deadline = timer.C
	}

	// Poll as well as wait on the signal: the signal fires for lines pushed in
	// this process, while a file dropped in the session inbox is only noticed
	// by a drain.
	poll := time.NewTicker(250 * time.Millisecond)
	defer poll.Stop()

	for {
		batch := i.Drain()
		for idx, m := range batch {
			if m.Kind != KindUserLine || m.At.Before(askedAt) {
				notForUs = append(notForUs, m)
				continue
			}
			// Whatever came out with the answer is the loop's.
			notForUs = append(notForUs, batch[idx+1:]...)
			return strings.TrimSpace(m.Text), AskAnswered
		}

		select {
		case <-ctx.Done():
			return "", AskCancelled
		case <-deadline:
			return "", AskTimedOut
		case <-i.Signal():
		case <-poll.C:
		}
	}
}

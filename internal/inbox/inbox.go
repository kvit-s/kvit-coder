// Package inbox is the one place a turn picks up anything that arrives while
// it is running: a line you type, a file dropped in the session's inbox
// directory by "kvit-coder steer", or an event from a background process.
//
// The agent loop drains it once per iteration. There is no file watcher: the
// drain reads the directory as part of the same call, so the drain is the poll.
package inbox

import (
	"encoding/json"
	"fmt"
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

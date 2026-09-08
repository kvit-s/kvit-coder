package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// A session is a directory, not a file:
//
//	~/.kvit-coder/sessions/<name>/
//	    history.jsonl   append-only, one timestamped event per line
//	    meta.json       when it was created and last touched, which workspace
//	                    and model, the first prompt, and the compaction chain
//	    checkpoints/    the shadow git repo the checkpoint tools commit into
//	    proc/           pidfiles and logs for background processes
//	    inbox/          files dropped here reach the model next iteration
//	    tmp/            tool output too large to put in a message
//
// Everything with a life longer than one turn lives here. That is what lets a
// turn run in its own process without losing the checkpoint history, the temp
// files whose paths the model was told, or anything else the next turn needs.
const (
	historyFile = "history.jsonl"
	metaFile    = "meta.json"
	lockFile    = ".lock"

	// CheckpointsSubdir and friends are exported so callers can build the
	// paths they hand to other components.
	CheckpointsSubdir = "checkpoints"
	ProcSubdir        = "proc"
	InboxSubdir       = "inbox"
	TmpSubdir         = "tmp"
)

// Kind names what a history line records.
type Kind string

const (
	// KindMessage is one message of the conversation as the API sees it.
	KindMessage Kind = "message"
	// KindToolResult is a message that is a tool's result, with which tool
	// produced it and how long it took.
	KindToolResult Kind = "tool_result"
	// KindNotice is something that happened to the turn rather than in the
	// conversation: the iteration budget running out, an interrupt, a
	// background process exiting.
	KindNotice Kind = "notice"
	// KindSettings records the model and options a turn ran with, so a
	// transcript read later says what produced it.
	KindSettings Kind = "settings"
	// KindRollback records that the loop discarded the last few messages,
	// which is how backtracking appears in a file that is only ever appended
	// to. Load applies it; the discarded attempt stays in the file, where it
	// says what the model tried and why it was taken back.
	KindRollback Kind = "rollback"
)

// Event is one line of history.jsonl. One JSON object per line keeps the file
// greppable and appendable without reading what is already there.
type Event struct {
	TS   time.Time `json:"ts"`
	Kind Kind      `json:"kind"`

	// Message is set on KindMessage and KindToolResult. Load returns these,
	// in order, as the conversation to send to the model, so the API history
	// is derived from the record rather than stored separately.
	Message *llm.Message `json:"message,omitempty"`

	// Tool and MS accompany KindToolResult.
	Tool string `json:"tool,omitempty"`
	MS   int64  `json:"ms,omitempty"`

	// Text accompanies KindNotice.
	Text string `json:"text,omitempty"`

	// Model, MergeThinking and Tools accompany KindSettings. Tools is what was
	// registered for the turn, which is the only record of what the model was
	// offered: the configuration that produced it can change between one
	// session and the next, so counting calls against today's config answers
	// a question about today rather than about the session being read.
	Model         string   `json:"model,omitempty"`
	MergeThinking *bool    `json:"merge_thinking,omitempty"`
	Tools         []string `json:"tools,omitempty"`

	// Dropped accompanies KindRollback: how many messages at the end of the
	// history so far are no longer part of the conversation.
	Dropped int `json:"dropped,omitempty"`
}

// Meta is meta.json: what this session is, and what it was compacted from or
// into. Parent and SucceededBy are written by compaction, which does not exist
// yet; Open already follows SucceededBy so that adding it later needs no
// change here.
type Meta struct {
	Name        string    `json:"name"`
	Created     time.Time `json:"created"`
	Updated     time.Time `json:"updated"`
	Workspace   string    `json:"workspace,omitempty"`
	Model       string    `json:"model,omitempty"`
	FirstPrompt string    `json:"first_prompt,omitempty"`
	// Title is the session's display title: a 3-6 word summary of the
	// first prompt from the summarizer model, or the prompt's first
	// words when no summarizer is configured or its call fails. Set once
	// at the start of a session (see internal/repl/title.go) and left
	// alone afterwards; Name stays the YYYY-MM-DD-random6 directory label.
	Title       string `json:"title,omitempty"`
	Summary     string `json:"summary,omitempty"`
	Parent      string `json:"parent,omitempty"`
	SucceededBy string `json:"succeeded_by,omitempty"`
}

// Session is one open session directory.
type Session struct {
	name string
	dir  string

	mu   sync.Mutex
	meta Meta
}

// Open opens or creates the named session under the default sessions
// directory, migrating a session left over from when a session was a single
// flat file, and following the compaction chain to the session that succeeded
// this one.
func Open(name string) (*Session, error) {
	m, err := NewManager()
	if err != nil {
		return nil, err
	}
	return m.Open(name)
}

// Open opens or creates the named session under this manager's directory.
func (m *Manager) Open(name string) (*Session, error) {
	if name == "" {
		return nil, fmt.Errorf("session name cannot be empty")
	}

	// Follow succeeded_by so a name that was compacted away still resolves to
	// the session that replaced it. The bound stops a chain that loops.
	seen := map[string]bool{}
	for range 16 {
		if seen[name] {
			return nil, fmt.Errorf("session %q: succeeded_by chain loops", name)
		}
		seen[name] = true

		if err := m.migrateFlatFile(name); err != nil {
			return nil, err
		}
		s, err := OpenDir(filepath.Join(m.baseDir, name))
		if err != nil {
			return nil, err
		}
		if s.meta.SucceededBy == "" {
			return s, nil
		}
		name = s.meta.SucceededBy
	}
	return nil, fmt.Errorf("session %q: succeeded_by chain is too long", name)
}

// OpenDir opens or creates a session at an explicit directory. The benchmark
// harnesses use it to give a run an ephemeral session under its own working
// directory, so the same code path serves them and a named session alike.
func OpenDir(dir string) (*Session, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve session directory: %w", err)
	}
	for _, sub := range []string{"", CheckpointsSubdir, ProcSubdir, InboxSubdir, TmpSubdir} {
		if err := os.MkdirAll(filepath.Join(abs, sub), 0755); err != nil {
			return nil, fmt.Errorf("failed to create session directory: %w", err)
		}
	}

	s := &Session{name: filepath.Base(abs), dir: abs}
	if err := s.loadMeta(); err != nil {
		return nil, err
	}
	return s, nil
}

// Name is the session's name, which is its directory's name.
func (s *Session) Name() string { return s.name }

// Dir is the session directory.
func (s *Session) Dir() string { return s.dir }

// CheckpointsDir is the shadow git repository the checkpoint tools commit into.
func (s *Session) CheckpointsDir() string { return filepath.Join(s.dir, CheckpointsSubdir) }

// ProcDir holds pidfiles and logs for processes that outlive a turn.
func (s *Session) ProcDir() string { return filepath.Join(s.dir, ProcSubdir) }

// InboxDir is where a steering message dropped as a file is picked up.
func (s *Session) InboxDir() string { return filepath.Join(s.dir, InboxSubdir) }

// TmpDir holds tool output too large to put in a message. It outlives the turn
// because the model is told these paths and may read them on a later turn.
func (s *Session) TmpDir() string { return filepath.Join(s.dir, TmpSubdir) }

// HistoryPath is the append-only transcript.
func (s *Session) HistoryPath() string { return filepath.Join(s.dir, historyFile) }

// Meta returns the session's metadata. The pointer is the session's own, so a
// caller can set fields and then call SaveMeta.
func (s *Session) Meta() *Meta {
	s.mu.Lock()
	defer s.mu.Unlock()
	return &s.meta
}

func (s *Session) loadMeta() error {
	path := filepath.Join(s.dir, metaFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read %s: %w", metaFile, err)
		}
		now := time.Now().UTC()
		s.meta = Meta{Name: s.name, Created: now, Updated: now}
		return s.SaveMeta()
	}
	if err := json.Unmarshal(data, &s.meta); err != nil {
		// A corrupt meta.json must not cost you the history next to it.
		now := time.Now().UTC()
		s.meta = Meta{Name: s.name, Created: now, Updated: now}
		return nil
	}
	if s.meta.Name == "" {
		s.meta.Name = s.name
	}
	return nil
}

// SaveMeta writes meta.json.
func (s *Session) SaveMeta() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveMetaLocked()
}

func (s *Session) saveMetaLocked() error {
	s.meta.Updated = time.Now().UTC()
	data, err := json.MarshalIndent(&s.meta, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal %s: %w", metaFile, err)
	}
	return os.WriteFile(filepath.Join(s.dir, metaFile), append(data, '\n'), 0644)
}

// Append writes one event to history.jsonl. The file is opened in append mode
// for each call, so a second process writing to the same session interleaves
// whole lines rather than overwriting.
func (s *Session) Append(ev Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.appendLocked(ev)
}

func (s *Session) appendLocked(ev Event) error {
	if ev.TS.IsZero() {
		ev.TS = time.Now().UTC()
	}
	if ev.Kind == "" {
		ev.Kind = KindMessage
	}
	line, err := json.Marshal(ev)
	if err != nil {
		return fmt.Errorf("failed to marshal session event: %w", err)
	}

	f, err := os.OpenFile(s.HistoryPath(), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open session history: %w", err)
	}
	defer f.Close()
	if _, err := f.Write(append(line, '\n')); err != nil {
		return fmt.Errorf("failed to write session event: %w", err)
	}
	return nil
}

// AppendMessages records a run of conversation messages, one event each. A
// message that is a tool result is recorded as such, so the transcript says
// which tool produced it.
func (s *Session) AppendMessages(messages []llm.Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UTC()
	for i := range messages {
		msg := messages[i]
		ev := Event{TS: now, Kind: KindMessage, Message: &msg}
		if msg.Role == llm.RoleTool {
			ev.Kind = KindToolResult
			ev.Tool = msg.Name
		}
		if err := s.appendLocked(ev); err != nil {
			return err
		}
	}
	if len(messages) > 0 {
		return s.saveMetaLocked()
	}
	return nil
}

// Rollback records that the loop discarded the last n messages. The messages
// stay in the file — it is append-only — and Load skips them.
func (s *Session) Rollback(n int) error {
	if n <= 0 {
		return nil
	}
	return s.Append(Event{Kind: KindRollback, Dropped: n})
}

// Notice records something that happened to the turn rather than in the
// conversation.
func (s *Session) Notice(text string) error {
	return s.Append(Event{Kind: KindNotice, Text: text})
}

// Settings records what this turn ran with, so a transcript read weeks later
// says which model, options and tools produced it.
func (s *Session) Settings(model string, mergeThinking bool, tools []string) error {
	return s.Append(Event{
		Kind:          KindSettings,
		Model:         model,
		MergeThinking: &mergeThinking,
		Tools:         tools,
	})
}

// LastSettingsModel returns the model the most recent turn recorded in its
// settings event ("model:effort" once model selection exists, a bare wire id
// before it), or "" when no turn has recorded one yet. A turn that switches
// models compares this against its own to decide whether the tail of the
// history was produced by a different model.
func (s *Session) LastSettingsModel() (string, error) {
	f, err := os.Open(s.HistoryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", fmt.Errorf("failed to open session history: %w", err)
	}
	defer f.Close()

	last := ""
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return "", fmt.Errorf("failed to parse session event: %w", err)
		}
		if ev.Kind == KindSettings && ev.Model != "" {
			last = ev.Model
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("failed to read session history: %w", err)
	}
	return last, nil
}

// Load returns the conversation, in order, as the messages to send to the
// model. Lines that carry no message — notices and settings — are skipped, so
// the API history is derived from the record rather than kept beside it.
func (s *Session) Load() ([]llm.Message, error) {
	return readHistory(s.HistoryPath())
}

func readHistory(path string) ([]llm.Message, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to open session history: %w", err)
	}
	defer f.Close()

	var messages []llm.Message
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, fmt.Errorf("failed to parse session event: %w", err)
		}
		if ev.Kind == "" && ev.Message == nil {
			// A line written before history was a stream of events is a bare
			// message. Reading it costs nothing and saves a migration failure
			// from losing a conversation.
			var msg llm.Message
			if err := json.Unmarshal([]byte(line), &msg); err == nil && msg.Role != "" {
				messages = append(messages, msg)
			}
			continue
		}
		if ev.Kind == KindRollback {
			// The loop took these back, so they are not part of the
			// conversation even though they are still in the file.
			if drop := min(ev.Dropped, len(messages)); drop > 0 {
				messages = messages[:len(messages)-drop]
			}
			continue
		}
		if ev.Message != nil {
			messages = append(messages, *ev.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read session history: %w", err)
	}
	return messages, nil
}

// AcquireLock takes an exclusive lock on this session, so two processes do not
// append to one history at once. The returned function releases it.
func (s *Session) AcquireLock() (func(), error) {
	return acquireLockAt(filepath.Join(s.dir, lockFile), s.name)
}

func acquireLockAt(lockPath, name string) (func(), error) {
	if err := os.MkdirAll(filepath.Dir(lockPath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, fmt.Errorf("session %q is already in use by another process", name)
	}
	_ = f.Truncate(0)
	_, _ = f.Seek(0, 0)
	fmt.Fprintf(f, "%d\n", os.Getpid())

	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		f.Close()
		os.Remove(lockPath)
	}, nil
}

// migrateFlatFile converts a session written when a session was one flat file
// into a session directory: each line becomes a message event stamped with the
// file's modification time, and the old file is renamed rather than deleted.
func (m *Manager) migrateFlatFile(name string) error {
	flat := filepath.Join(m.baseDir, name+".jsonl")
	fi, err := os.Stat(flat)
	if err != nil || !fi.Mode().IsRegular() {
		return nil
	}
	dir := filepath.Join(m.baseDir, name)
	if _, err := os.Stat(filepath.Join(dir, historyFile)); err == nil {
		// Already migrated; the flat file is a leftover.
		return nil
	}

	messages, err := readHistory(flat)
	if err != nil {
		return fmt.Errorf("failed to read session %q for migration: %w", name, err)
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create session directory for migration: %w", err)
	}

	var buf strings.Builder
	ts := fi.ModTime().UTC()
	for i := range messages {
		msg := messages[i]
		ev := Event{TS: ts, Kind: KindMessage, Message: &msg}
		if msg.Role == llm.RoleTool {
			ev.Kind = KindToolResult
			ev.Tool = msg.Name
		}
		line, err := json.Marshal(ev)
		if err != nil {
			return fmt.Errorf("failed to marshal migrated event: %w", err)
		}
		buf.Write(line)
		buf.WriteByte('\n')
	}
	if err := os.WriteFile(filepath.Join(dir, historyFile), []byte(buf.String()), 0644); err != nil {
		return fmt.Errorf("failed to write migrated history: %w", err)
	}
	if err := os.Rename(flat, flat+".migrated"); err != nil {
		return fmt.Errorf("failed to rename migrated session file: %w", err)
	}
	return nil
}

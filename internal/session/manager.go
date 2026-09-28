// Package session stores a conversation and everything else that outlives one
// turn: spilled tool output, the steering inbox and
// the record of background processes. See session.go for the directory layout.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// Manager owns the directory sessions live in.
type Manager struct {
	baseDir string // ~/.kvit-coder/sessions/
}

// SessionInfo contains metadata about a session.
type SessionInfo struct {
	Name         string
	ModTime      time.Time
	MessageCount int
	// Title is the session's display title from meta.json ("" when the
	// session predates titles or has none yet). Name stays the
	// YYYY-MM-DD-random6 directory label.
	Title string
}

// NewManager creates a new session manager.
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}
	return NewManagerIn(filepath.Join(homeDir, ".kvit-coder", "sessions"))
}

// NewManagerIn returns a manager over a named sessions directory, creating it
// if it is not there. NewManager is this with the usual location; a caller that
// keeps sessions somewhere else says so here.
func NewManagerIn(baseDir string) (*Manager, error) {
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}
	return &Manager{baseDir: baseDir}, nil
}

// BaseDir is the directory sessions live in.
func (m *Manager) BaseDir() string { return m.baseDir }

// SessionExists reports whether the named session has any history, either as a
// session directory or as a flat file from before sessions were directories.
func (m *Manager) SessionExists(name string) bool {
	if _, err := os.Stat(filepath.Join(m.baseDir, name, historyFile)); err == nil {
		return true
	}
	fi, err := os.Stat(filepath.Join(m.baseDir, name+".jsonl"))
	return err == nil && fi.Mode().IsRegular()
}

// LoadSession returns a session's conversation without opening or migrating
// it, for listing and display.
func (m *Manager) LoadSession(name string) ([]llm.Message, error) {
	dirHistory := filepath.Join(m.baseDir, name, historyFile)
	if _, err := os.Stat(dirHistory); err == nil {
		return readHistory(dirHistory)
	}
	return readHistory(filepath.Join(m.baseDir, name+".jsonl"))
}

// RunIDVar is the environment variable the LLM client's ${KVIT_RUN_ID} header
// placeholder expands from. Endpoints that route by a session header put every
// request carrying the same value on one backend.
const RunIDVar = "KVIT_RUN_ID"

// FromUIVar marks a kvit-coder run spawned by kvit-coder-ui. The UI already
// showed its own banner and echoed the prompt, so the child skips its copy
// rather than printing both.
const FromUIVar = "KVIT_CODER_FROM_UI"

// FromUI reports whether this process was spawned by kvit-coder-ui.
func FromUI() bool { return os.Getenv(FromUIVar) != "" }

// RunIDFor derives a stable run ID from a session name. Reopening the same
// session produces the same ID, so the turn lands on the backend that already
// holds the prompt cache for this conversation instead of a cold one.
func RunIDFor(name string) string {
	sum := sha256.Sum256([]byte("kvit-coder-session:" + name))
	return hex.EncodeToString(sum[:6])
}

// GenerateSessionName generates a unique session name in YYYY-MM-DD-random6 format.
// Uses crypto/rand for secure random suffix generation.
func (m *Manager) GenerateSessionName() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	// Generate 6 random bytes using crypto/rand
	randomBytes := make([]byte, 6)
	if _, err := rand.Read(randomBytes); err != nil {
		// Fallback to timestamp-based suffix if crypto/rand fails (extremely rare)
		return fmt.Sprintf("%s-%d", time.Now().Format("2006-01-02"), time.Now().UnixNano()%1000000)
	}

	// Map random bytes to charset
	suffix := make([]byte, 6)
	for i, b := range randomBytes {
		suffix[i] = charset[int(b)%len(charset)]
	}

	return fmt.Sprintf("%s-%s", time.Now().Format("2006-01-02"), string(suffix))
}

// ListSessions returns a list of all sessions with metadata, newest first.
// Sessions not yet migrated out of the flat-file layout are listed too.
func (m *Manager) ListSessions() ([]SessionInfo, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []SessionInfo
	seen := map[string]bool{}
	for _, entry := range entries {
		var name, historyPath string
		switch {
		case entry.IsDir():
			name = entry.Name()
			historyPath = filepath.Join(m.baseDir, name, historyFile)
			if _, err := os.Stat(historyPath); err != nil {
				continue
			}
		case strings.HasSuffix(entry.Name(), ".jsonl"):
			name = strings.TrimSuffix(entry.Name(), ".jsonl")
			historyPath = filepath.Join(m.baseDir, entry.Name())
		default:
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true

		info, err := os.Stat(historyPath)
		if err != nil {
			continue
		}
		msgCount := 0
		if messages, err := readHistory(historyPath); err == nil {
			msgCount = len(messages)
		}

		sessions = append(sessions, SessionInfo{
			Name:         name,
			ModTime:      info.ModTime(),
			MessageCount: msgCount,
			Title:        m.readMetaTitle(name),
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})

	return sessions, nil
}

// readMetaTitle returns the session's display title from its meta.json,
// or "" when there is none (a flat-file session from before sessions were
// directories, a missing or corrupt meta.json, or a session that predates
// titles). It never creates or migrates anything: listing must stay read-only.
func (m *Manager) readMetaTitle(name string) string {
	if meta := m.ReadMeta(name); meta != nil {
		return meta.Title
	}
	return ""
}

// ReadMeta returns the named session's meta.json, or nil when it has none or
// it cannot be read. Like readMetaTitle it never creates or migrates anything.
func (m *Manager) ReadMeta(name string) *Meta {
	data, err := os.ReadFile(filepath.Join(m.baseDir, name, metaFile))
	if err != nil {
		return nil
	}
	var meta Meta
	if err := json.Unmarshal(data, &meta); err != nil {
		return nil
	}
	return &meta
}

// SessionTitle returns the named session's display title, or "" when it has
// none yet. A session gets its title when its first turn finishes, so a
// caller that shows it has to ask again rather than reading it once at
// startup.
func (m *Manager) SessionTitle(name string) string {
	if name == "" {
		return ""
	}
	return m.readMetaTitle(name)
}

// MostRecent returns the name of the session touched last, or "" when there
// are none. It is what "continue where I left off" resolves to.
func (m *Manager) MostRecent() (string, error) {
	sessions, err := m.ListSessions()
	if err != nil {
		return "", err
	}
	if len(sessions) == 0 {
		return "", nil
	}
	// ListSessions already sorts newest first.
	return sessions[0].Name, nil
}

// DeleteSession removes a session and everything it holds: history, metadata,
// and the temp files whose cleanup now happens here rather than
// at process exit.
func (m *Manager) DeleteSession(name string) error {
	if !m.SessionExists(name) {
		return fmt.Errorf("session %q not found", name)
	}

	if err := os.RemoveAll(filepath.Join(m.baseDir, name)); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	flat := filepath.Join(m.baseDir, name+".jsonl")
	if err := os.Remove(flat); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete session: %w", err)
	}
	_ = os.Remove(flat + ".migrated")
	return nil
}

// ShowSession returns the formatted content of a session for display.
func (m *Manager) ShowSession(name string) (string, error) {
	messages, err := m.LoadSession(name)
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Session: %s (%d messages)\n", name, len(messages)))
	if title := m.readMetaTitle(name); title != "" {
		sb.WriteString(fmt.Sprintf("Title: %s\n", title))
	}
	sb.WriteString(strings.Repeat("─", 50) + "\n\n")

	for _, msg := range messages {
		switch msg.Role {
		case llm.RoleSystem:
			sb.WriteString("[system] (omitted)\n\n")
		case llm.RoleUser:
			content := msg.Content
			if len(content) > 500 {
				content = content[:497] + "..."
			}
			sb.WriteString(fmt.Sprintf("[user]\n%s\n", content))
			for _, img := range msg.Images {
				sb.WriteString(fmt.Sprintf("%s\n", img.Summary()))
			}
			sb.WriteString("\n")
		case llm.RoleAssistant:
			content := msg.Content
			if len(content) > 500 {
				content = content[:497] + "..."
			}
			sb.WriteString(fmt.Sprintf("[assistant]\n%s", content))
			if len(msg.ToolCalls) > 0 {
				sb.WriteString(fmt.Sprintf(" (+ %d tool calls)", len(msg.ToolCalls)))
			}
			sb.WriteString("\n\n")
		case llm.RoleTool:
			sb.WriteString(fmt.Sprintf("[tool: %s] (result omitted)\n\n", msg.Name))
		}
	}

	return sb.String(), nil
}

// AcquireLock takes an exclusive lock on a session by name. The lock file
// lives inside the session directory, alongside the history it protects.
func (m *Manager) AcquireLock(name string) (func(), error) {
	return acquireLockAt(filepath.Join(m.baseDir, name, lockFile), name)
}

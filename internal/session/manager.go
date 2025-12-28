// Package session provides session management for persisting conversation history.
package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// Manager handles session storage and retrieval.
type Manager struct {
	baseDir string // ~/.kvit-coder/sessions/
}

// SessionInfo contains metadata about a session.
type SessionInfo struct {
	Name         string
	ModTime      time.Time
	MessageCount int
}

// NewManager creates a new session manager.
func NewManager() (*Manager, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	baseDir := filepath.Join(homeDir, ".kvit-coder", "sessions")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return &Manager{baseDir: baseDir}, nil
}

// SessionExists checks if a session with the given name exists.
func (m *Manager) SessionExists(name string) bool {
	path := m.sessionPath(name)
	_, err := os.Stat(path)
	return err == nil
}

// LoadSession loads messages from a session file.
func (m *Manager) LoadSession(name string) ([]llm.Message, error) {
	path := m.sessionPath(name)
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	var messages []llm.Message
	scanner := bufio.NewScanner(file)
	// Increase buffer size for large messages
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 10*1024*1024) // 10MB max

	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}

		var msg llm.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil, fmt.Errorf("failed to parse message: %w", err)
		}
		messages = append(messages, msg)
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	return messages, nil
}

// SaveSession saves messages to a session file, replacing any existing content.
func (m *Manager) SaveSession(name string, messages []llm.Message) error {
	path := m.sessionPath(name)
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create session file: %w", err)
	}
	defer file.Close()

	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	return nil
}

// AppendToSession appends messages to an existing session file.
func (m *Manager) AppendToSession(name string, messages []llm.Message) error {
	path := m.sessionPath(name)
	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open session file: %w", err)
	}
	defer file.Close()

	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		if _, err := file.Write(append(data, '\n')); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
	}

	return nil
}

// GenerateSessionName generates a unique session name in YYYY-MM-DD-random6 format.
func (m *Manager) GenerateSessionName() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	suffix := make([]byte, 6)
	for i := range suffix {
		suffix[i] = charset[r.Intn(len(charset))]
	}

	return fmt.Sprintf("%s-%s", time.Now().Format("2006-01-02"), string(suffix))
}

// ListSessions returns a list of all sessions with metadata.
func (m *Manager) ListSessions() ([]SessionInfo, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var sessions []SessionInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".jsonl") {
			continue
		}

		name := strings.TrimSuffix(entry.Name(), ".jsonl")
		info, err := entry.Info()
		if err != nil {
			continue
		}

		// Count messages
		messages, err := m.LoadSession(name)
		msgCount := 0
		if err == nil {
			msgCount = len(messages)
		}

		sessions = append(sessions, SessionInfo{
			Name:         name,
			ModTime:      info.ModTime(),
			MessageCount: msgCount,
		})
	}

	// Sort by modification time (newest first)
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].ModTime.After(sessions[j].ModTime)
	})

	return sessions, nil
}

// DeleteSession removes a session file.
func (m *Manager) DeleteSession(name string) error {
	path := m.sessionPath(name)
	if err := os.Remove(path); err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}
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
			sb.WriteString(fmt.Sprintf("[user]\n%s\n\n", content))
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

// AcquireLock attempts to acquire an exclusive lock on a session.
// Returns a cleanup function that releases the lock, or an error if lock fails.
func (m *Manager) AcquireLock(name string) (func(), error) {
	lockPath := m.lockPath(name)

	// Create lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lockFile.Close()
		return nil, fmt.Errorf("session %q is already in use by another process", name)
	}

	// Write PID to lock file for debugging
	lockFile.Truncate(0)
	lockFile.Seek(0, 0)
	fmt.Fprintf(lockFile, "%d\n", os.Getpid())

	cleanup := func() {
		syscall.Flock(int(lockFile.Fd()), syscall.LOCK_UN)
		lockFile.Close()
		os.Remove(lockPath)
	}

	return cleanup, nil
}

// sessionPath returns the path to a session file.
func (m *Manager) sessionPath(name string) string {
	return filepath.Join(m.baseDir, name+".jsonl")
}

// lockPath returns the path to a session lock file.
func (m *Manager) lockPath(name string) string {
	return filepath.Join(m.baseDir, "."+name+".lock")
}

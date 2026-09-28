package session

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/procutil"
)

// turnPIDFile names the process currently running a turn for this session.
// "kvit-coder steer" reads it to find the session to steer when it is not
// told one, and stage 6's process registry lives alongside it.
const turnPIDFile = "turn.pid"

// TurnPIDPath is where the running turn records its process ID.
func (s *Session) TurnPIDPath() string { return filepath.Join(s.ProcDir(), turnPIDFile) }

// MarkTurnStart records this process as the one running the session's turn.
// The returned function removes the record and must be called when the turn
// ends, however it ends.
func (s *Session) MarkTurnStart() (func(), error) {
	path := s.TurnPIDPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return func() {}, fmt.Errorf("failed to create proc directory: %w", err)
	}
	if err := os.WriteFile(path, []byte(strconv.Itoa(os.Getpid())+"\n"), 0644); err != nil {
		return func() {}, fmt.Errorf("failed to record turn pid: %w", err)
	}
	return func() { os.Remove(path) }, nil
}

// TurnPID returns the process ID of the turn running for this session, and
// whether that process is still alive. A pidfile left behind by a process that
// died reports false, so a crash does not make a session look busy forever.
func (s *Session) TurnPID() (int, bool) {
	return readLivePID(s.TurnPIDPath())
}

func readLivePID(path string) (int, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil || pid <= 0 {
		return 0, false
	}
	// Alive asks the kernel whether the process exists without disturbing it
	// (signal 0 on unix, exit-code query on Windows where FindProcess always
	// succeeds).
	if !procutil.Alive(pid) {
		return pid, false
	}
	return pid, true
}

// ActiveTurns returns the sessions that have a live turn running, so a command
// given no session name can find the one to talk to.
func (m *Manager) ActiveTurns() ([]string, error) {
	entries, err := os.ReadDir(m.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read sessions directory: %w", err)
	}

	var active []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(m.baseDir, e.Name(), ProcSubdir, turnPIDFile)
		if _, alive := readLivePID(path); alive {
			active = append(active, e.Name())
		}
	}
	return active, nil
}

// InboxDirFor is where a message for the named session is dropped, without
// opening the session or creating anything else it needs.
func (m *Manager) InboxDirFor(name string) string {
	return filepath.Join(m.baseDir, name, InboxSubdir)
}

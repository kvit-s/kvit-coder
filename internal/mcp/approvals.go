package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"sync"
)

// Approvals remembers which MCP tools the user has already said yes to under
// the ask_once policy.
//
// Where this is kept matters more than it looks. The agent is one process per
// turn, so a map in memory is emptied every time you say anything, which turns
// ask_once into ask-once-per-instruction: a browser server would prompt again
// for every click of every turn. Keeping it in the session directory makes
// ask_once mean what it says, and matches where session-scoped shell grants
// already go (`<session>/permissions.json`).
type Approvals interface {
	// Approved reports whether this tool has already been allowed.
	Approved(tool string) bool
	// Approve records an allowance. An error is reported to the caller but
	// never refuses the call that was just approved: failing to write the file
	// should cost a repeated prompt next turn, not the work.
	Approve(tool string) error
}

// memoryApprovals is the fallback when there is no session to write to, such
// as a benchmark run. It lasts as long as the process, which is one turn.
type memoryApprovals struct {
	mu       sync.Mutex
	approved map[string]bool
}

func newMemoryApprovals() *memoryApprovals {
	return &memoryApprovals{approved: make(map[string]bool)}
}

func (a *memoryApprovals) Approved(tool string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.approved[tool]
}

func (a *memoryApprovals) Approve(tool string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.approved[tool] = true
	return nil
}

// FileApprovals persists approvals to a JSON file, normally
// `<session>/mcp-approvals.json`.
//
// The file is re-read before each check rather than cached, because turns of
// one session can overlap and a second agent's approval should be honoured
// rather than prompted for again. Reads are cheap: the file holds a handful of
// tool names.
type FileApprovals struct {
	mu   sync.Mutex
	path string
}

type approvalFile struct {
	Approved []string `json:"approved"`
}

// OpenApprovals returns a store backed by path. The file need not exist.
func OpenApprovals(path string) (*FileApprovals, error) {
	if path == "" {
		return nil, fmt.Errorf("no path for MCP approvals")
	}
	a := &FileApprovals{path: path}
	if _, err := a.read(); err != nil {
		return nil, err
	}
	return a, nil
}

// Path is the file approvals are written to, so a person can see what they
// have agreed to and take it back by editing or deleting it.
func (a *FileApprovals) Path() string { return a.path }

func (a *FileApprovals) read() (approvalFile, error) {
	var f approvalFile
	data, err := os.ReadFile(a.path)
	if err != nil {
		if os.IsNotExist(err) {
			return f, nil
		}
		return f, fmt.Errorf("failed to read %s: %w", a.path, err)
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return f, fmt.Errorf("%s is not readable: %w", a.path, err)
	}
	return f, nil
}

func (a *FileApprovals) Approved(tool string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	f, err := a.read()
	if err != nil {
		return false
	}
	return slices.Contains(f.Approved, tool)
}

func (a *FileApprovals) Approve(tool string) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	f, err := a.read()
	if err != nil {
		return err
	}
	if slices.Contains(f.Approved, tool) {
		return nil
	}
	f.Approved = append(f.Approved, tool)
	sort.Strings(f.Approved)

	if err := os.MkdirAll(filepath.Dir(a.path), 0o755); err != nil {
		return fmt.Errorf("failed to create %s: %w", filepath.Dir(a.path), err)
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	// Written through a temporary name so a crash mid-write cannot leave a
	// file that fails to parse and silently drops every approval in it.
	tmp := a.path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("failed to write %s: %w", a.path, err)
	}
	if err := os.Rename(tmp, a.path); err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to write %s: %w", a.path, err)
	}
	return nil
}

package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	tempFilePrefix = "shell-"
	tempFileMaxAge = 24 * time.Hour // Clean files older than this on startup
)

// TempFileManager tracks and manages temporary files created during the session
type TempFileManager struct {
	mu            sync.Mutex
	workspaceRoot string
	tempDir       string
	files         map[string]bool // track created temp files
}

// NewTempFileManager creates a manager that spills large tool output into the
// given directory. The directory is used as given: it is the session's tmp/,
// which already exists for the purpose. It used to append ".kvit-coder/tmp" to
// whatever it was handed, which after the move into the session produced paths
// like <session>/tmp/.kvit-coder/tmp/shell-123.
func NewTempFileManager(tempDir string) *TempFileManager {
	mgr := &TempFileManager{
		workspaceRoot: tempDir,
		tempDir:       tempDir,
		files:         make(map[string]bool),
	}

	// Clean up stale temp files from previous sessions
	mgr.cleanupStaleFiles()

	return mgr
}

// CreateTempFile creates a new temporary file and tracks it for cleanup
func (m *TempFileManager) CreateTempFile() (*os.File, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Ensure temp directory exists
	if err := os.MkdirAll(m.tempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}

	// Create temp file with our prefix
	f, err := os.CreateTemp(m.tempDir, tempFilePrefix+"*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// Track this file
	m.files[f.Name()] = true

	return f, nil
}

// CleanupAll removes all tracked temp files (called at session end)
func (m *TempFileManager) CleanupAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for path := range m.files {
		os.Remove(path) // ignore errors, file might already be deleted
		delete(m.files, path)
	}

	// Try to remove the directory if it is now empty. Its parent is the
	// session, which outlives this process and is never removed here.
	os.Remove(m.tempDir)
}

// cleanupStaleFiles removes temp files from previous sessions
func (m *TempFileManager) cleanupStaleFiles() {
	entries, err := os.ReadDir(m.tempDir)
	if err != nil {
		return // directory might not exist yet, that's fine
	}

	now := time.Now()
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}

		// Check file age
		info, err := entry.Info()
		if err != nil {
			continue
		}

		if now.Sub(info.ModTime()) > tempFileMaxAge {
			path := filepath.Join(m.tempDir, entry.Name())
			os.Remove(path) // ignore errors
		}
	}
}

// Package workspace provides workspace-level utilities including locking.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/kvit-s/kvit-coder/internal/filelock"
)

const lockFileName = ".kvit-coder.lock"

// Lock represents an acquired workspace lock.
//
// The lock used to install its own SIGINT handler that removed the lock file
// and called os.Exit(130). That is why an interrupt used to throw away the
// conversation: the process died before the turn could write down what it had
// done. Cleanup now happens on the normal exit path, and the interrupt is
// handled where the run context can be cancelled.
type Lock struct {
	file        *os.File
	lockPath    string
	mu          sync.Mutex
	cleanupOnce sync.Once
}

// AcquireLock attempts to acquire an exclusive lock on a workspace directory.
// This prevents multiple kvit-coder instances from running on the same workspace
// simultaneously, which would cause checkpoint conflicts.
// Returns a Lock that must be released by calling Release(), or an error if lock fails.
func AcquireLock(workspaceRoot string) (*Lock, error) {
	lockPath := filepath.Join(workspaceRoot, lockFileName)

	// Create lock file
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to create workspace lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	if err := filelock.Lock(lockFile); err != nil {
		lockFile.Close()
		return nil, fmt.Errorf("workspace %q is already in use by another kvit-coder instance", workspaceRoot)
	}

	// Write PID to lock file for debugging
	_ = lockFile.Truncate(0)
	_, _ = lockFile.Seek(0, 0)
	fmt.Fprintf(lockFile, "%d\n", os.Getpid())

	return &Lock{
		file:     lockFile,
		lockPath: lockPath,
	}, nil
}

// Release releases the workspace lock and removes the lock file.
func (l *Lock) Release() {
	l.mu.Lock()
	if l.file == nil {
		l.mu.Unlock()
		return
	}
	l.mu.Unlock()
	l.cleanup()
}

// cleanup releases the lock and removes the lock file, once.
func (l *Lock) cleanup() {
	l.cleanupOnce.Do(func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.file == nil {
			return
		}
		_ = filelock.Unlock(l.file)
		l.file.Close()
		os.Remove(l.lockPath)
		l.file = nil
	})
}

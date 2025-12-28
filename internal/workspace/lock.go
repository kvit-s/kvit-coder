// Package workspace provides workspace-level utilities including locking.
package workspace

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const lockFileName = ".kvit-coder.lock"

// Lock represents an acquired workspace lock.
type Lock struct {
	file     *os.File
	lockPath string
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
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lockFile.Close()
		return nil, fmt.Errorf("workspace %q is already in use by another kvit-coder instance", workspaceRoot)
	}

	// Write PID to lock file for debugging
	lockFile.Truncate(0)
	lockFile.Seek(0, 0)
	fmt.Fprintf(lockFile, "%d\n", os.Getpid())

	return &Lock{
		file:     lockFile,
		lockPath: lockPath,
	}, nil
}

// Release releases the workspace lock and removes the lock file.
func (l *Lock) Release() {
	if l.file == nil {
		return
	}
	syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
	l.file.Close()
	os.Remove(l.lockPath)
	l.file = nil
}

// Package workspace provides workspace-level utilities including locking.
package workspace

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
)

const lockFileName = ".kvit-coder.lock"

// Lock represents an acquired workspace lock.
type Lock struct {
	file        *os.File
	lockPath    string
	sigChan     chan os.Signal
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
	if err := syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lockFile.Close()
		return nil, fmt.Errorf("workspace %q is already in use by another kvit-coder instance", workspaceRoot)
	}

	// Write PID to lock file for debugging
	lockFile.Truncate(0)
	lockFile.Seek(0, 0)
	fmt.Fprintf(lockFile, "%d\n", os.Getpid())

	lock := &Lock{
		file:     lockFile,
		lockPath: lockPath,
		sigChan:  make(chan os.Signal, 1),
	}

	// Register signal handler to clean up lock file on Ctrl+C
	signal.Notify(lock.sigChan, syscall.SIGINT, syscall.SIGTERM)
	sigChan := lock.sigChan // Capture to avoid race with Release() setting to nil
	go func() {
		sig, ok := <-sigChan
		if ok && sig != nil {
			lock.cleanup()
			os.Exit(130) // 128 + SIGINT(2)
		}
	}()

	return lock, nil
}

// Release releases the workspace lock and removes the lock file.
func (l *Lock) Release() {
	l.mu.Lock()
	if l.file == nil {
		l.mu.Unlock()
		return
	}
	// Stop listening for signals
	if l.sigChan != nil {
		signal.Stop(l.sigChan)
		close(l.sigChan)
		l.sigChan = nil
	}
	l.mu.Unlock()
	l.cleanup()
}

// cleanup performs the actual file cleanup (called by both Release and signal handler)
func (l *Lock) cleanup() {
	l.cleanupOnce.Do(func() {
		l.mu.Lock()
		defer l.mu.Unlock()
		if l.file == nil {
			return
		}
		syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
		l.file.Close()
		os.Remove(l.lockPath)
		l.file = nil
	})
}

package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireLock_Success(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "workspace-lock-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Acquire lock
	lock, err := AcquireLock(tmpDir)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Verify lock file exists
	lockPath := filepath.Join(tmpDir, lockFileName)
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file should exist")
	}

	// Release lock
	lock.Release()

	// Verify lock file is removed
	if _, err := os.Stat(lockPath); !os.IsNotExist(err) {
		t.Error("lock file should be removed after release")
	}
}

func TestAcquireLock_BlocksConcurrentAccess(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "workspace-lock-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Acquire first lock
	lock1, err := AcquireLock(tmpDir)
	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err)
	}
	defer lock1.Release()

	// Try to acquire second lock - should fail
	lock2, err := AcquireLock(tmpDir)
	if err == nil {
		lock2.Release()
		t.Fatal("second lock should have failed")
	}

	// Error message should mention workspace is in use
	if lock2 != nil {
		t.Error("lock2 should be nil on failure")
	}
}

func TestAcquireLock_AllowsAfterRelease(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "workspace-lock-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Acquire and release first lock
	lock1, err := AcquireLock(tmpDir)
	if err != nil {
		t.Fatalf("failed to acquire first lock: %v", err)
	}
	lock1.Release()

	// Acquire second lock - should succeed
	lock2, err := AcquireLock(tmpDir)
	if err != nil {
		t.Fatalf("failed to acquire second lock after release: %v", err)
	}
	defer lock2.Release()
}

func TestLock_ReleaseIdempotent(t *testing.T) {
	// Create temp directory
	tmpDir, err := os.MkdirTemp("", "workspace-lock-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Acquire lock
	lock, err := AcquireLock(tmpDir)
	if err != nil {
		t.Fatalf("failed to acquire lock: %v", err)
	}

	// Release multiple times - should not panic
	lock.Release()
	lock.Release()
	lock.Release()
}

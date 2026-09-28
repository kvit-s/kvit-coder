package filelock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLockUnlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := Lock(f); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := Unlock(f); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	// Re-lock after unlock works.
	if err := Lock(f); err != nil {
		t.Fatalf("second Lock: %v", err)
	}
	if err := Unlock(f); err != nil {
		t.Fatalf("second Unlock: %v", err)
	}
}

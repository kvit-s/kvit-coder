// Package filelock provides an exclusive, non-blocking file lock with the
// same "already in use by another process" semantics on every OS.
//
// Unix uses flock(LOCK_EX|LOCK_NB); Windows uses LockFileEx with
// LOCKFILE_EXCLUSIVE_LOCK|LOCKFILE_FAIL_IMMEDIATELY. The .lock pidfile
// contents are unchanged; only the mechanism is per-OS.
package filelock

import "os"

// Lock takes a non-blocking exclusive lock on f. It returns an error when
// another process holds the lock.
func Lock(f *os.File) error {
	return lockFile(f)
}

// Unlock releases a lock taken with Lock.
func Unlock(f *os.File) error {
	return unlockFile(f)
}

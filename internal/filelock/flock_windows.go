//go:build windows

package filelock

import (
	"os"

	"golang.org/x/sys/windows"
)

const (
	lockFlags = windows.LOCKFILE_EXCLUSIVE_LOCK | windows.LOCKFILE_FAIL_IMMEDIATELY
	// Lock a large range so the whole file is covered. LockFileEx locks a
	// byte range; using the maximum DWORD pair covers everything a pidfile
	// will ever hold.
	lockLow  = 0xFFFFFFFF
	lockHigh = 0xFFFFFFFF
)

func lockFile(f *os.File) error {
	handle := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	if err := windows.LockFileEx(handle, lockFlags, 0, lockLow, lockHigh, &overlapped); err != nil {
		return err
	}
	return nil
}

func unlockFile(f *os.File) error {
	handle := windows.Handle(f.Fd())
	var overlapped windows.Overlapped
	// UnlockFileEx failure on release is best-effort; callers already ignore
	// unlock errors, so return it and let them decide.
	if err := windows.UnlockFileEx(handle, 0, lockLow, lockHigh, &overlapped); err != nil {
		return err
	}
	return nil
}

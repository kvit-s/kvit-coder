//go:build !windows

package procutil

import (
	"os/exec"
	"syscall"
	"time"
)

func detach(cmd *exec.Cmd) {
	// Setsid detaches the process into its own session, so it neither dies
	// with the turn nor receives the terminal's ctrl-c, and killing it by
	// negative pid takes everything it started with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

func alive(pid int) bool {
	return syscall.Kill(pid, 0) == nil
}

func killTree(pid int) error {
	// Negative pid means the whole process group, which Setsid made this
	// process the leader of, so nothing it started is left behind.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		return err
	}
	// Give it a moment to go quietly before insisting.
	for range 20 {
		if syscall.Kill(-pid, 0) != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := syscall.Kill(-pid, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		return err
	}
	return nil
}

// KillProcessGroup kills the process group of an already-started command.
// It is the exec.Cmd variant of KillTree for callers that hold the child.
func KillProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	} else {
		_ = cmd.Process.Kill()
	}
}

// DetachProcessGroup marks cmd as its own process group (Setpgid variant used
// by short-lived Shell tool children, as opposed to the Setsid session used
// for background processes).
func DetachProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

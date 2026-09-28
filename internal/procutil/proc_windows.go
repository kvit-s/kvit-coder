//go:build windows

package procutil

import (
	"os/exec"
	"strconv"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
)

// stillActive is STILL_ACTIVE (259): GetExitCodeProcess reports it while the
// process is running. x/sys/windows does not export the constant.
const stillActive = 259

// detach marks the child as its own process group so it neither dies with
// the turn nor receives the terminal's ctrl-c. Windows has no Setsid/Setpgid;
// CREATE_NEW_PROCESS_GROUP is the replacement: the child gets its own group
// and ctrl-c goes only to processes in the parent's group.
//
// Tree kill is handled by KillTree via taskkill /T, which walks the child
// tree at kill time. A Job Object with KILL_ON_JOB_CLOSE would also kill the
// tree, but the job handle does not survive the turn boundary that persistent
// background processes are designed to cross: when the starting process
// exits, the last handle closes and the job kills children that were meant
// to survive into the next turn. taskkill needs no handle, so it works from
// any later turn.
func detach(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= windows.CREATE_NEW_PROCESS_GROUP
}

// alive must query the exit code: os.FindProcess always succeeds on Windows,
// even for pids that no longer exist.
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	handle, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		// Access denied means the process exists but we may not query it.
		// ESRCH / invalid parameter means it is gone.
		if err == windows.ERROR_ACCESS_DENIED {
			return true
		}
		return false
	}
	defer windows.CloseHandle(handle)
	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil {
		return false
	}
	return code == stillActive
}

func killTree(pid int) error {
	// taskkill /T walks the child tree at kill time, which is what the
	// negative-pid group kill does on unix. /F is the SIGKILL equivalent;
	// there is no polite signal to send an arbitrary Windows child, so the
	// graceful-then-force sequence collapses to one immediate kill.
	cmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	_ = cmd.Run()
	// taskkill reports failure when the process is already gone; either way
	// the caller wanted it stopped. Poll briefly so a caller that checks
	// Alive right after sees the result.
	for range 20 {
		if !alive(pid) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	// Last resort: terminate the pid itself.
	if handle, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid)); err == nil {
		_ = windows.TerminateProcess(handle, 1)
		windows.CloseHandle(handle)
	}
	return nil
}

// KillProcessGroup kills the whole tree of an already-started command.
func KillProcessGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = killTree(cmd.Process.Pid)
}

// DetachProcessGroup marks cmd as detached. On Windows both background and
// short-lived children use the same mechanism; see detach.
func DetachProcessGroup(cmd *exec.Cmd) {
	Detach(cmd)
}

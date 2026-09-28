// Package procutil abstracts the per-OS process plumbing the agent needs:
//
//   - Detach marks a child so it neither dies with the turn nor receives the
//     terminal's ctrl-c, and so killing it takes its whole tree.
//   - Alive reports whether a pid is still running (signal 0 on unix,
//     OpenProcess + exit-code query on Windows, where os.FindProcess always
//     succeeds).
//   - KillTree stops a process and everything it started (negative-pid group
//     kill on unix, taskkill /T on Windows where process-group semantics do
//     not exist and a Job Object handle would not survive the turn boundary).
package procutil

import "os/exec"

// Detach detaches cmd so it outlives the turn's iterations and can be killed
// as a tree. It must be called before cmd.Start.
func Detach(cmd *exec.Cmd) {
	detach(cmd)
}

// Alive reports whether pid is still running.
func Alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return alive(pid)
}

// KillTree stops pid and everything it started. Killing an already-dead
// process is not an error.
func KillTree(pid int) error {
	if pid <= 0 {
		return nil
	}
	return killTree(pid)
}

//go:build !windows

package procutil

import (
	"os"
	"os/exec"
)

// RestoreTerminal resets a garbled terminal after the agent exits. On unix
// this is `stty sane` against the controlling terminal.
func RestoreTerminal() {
	if os.Stdin.Fd() == 0 {
		cmd := exec.Command("sh", "-c", "stty sane </dev/tty >/dev/tty 2>&1")
		_ = cmd.Run()
	}
}

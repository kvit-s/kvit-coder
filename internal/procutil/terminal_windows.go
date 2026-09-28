//go:build windows

package procutil

// RestoreTerminal resets a garbled terminal after the agent exits. On
// Windows bubbletea owns the console modes, so there is nothing to reset:
// the stty call is a no-op by design.
func RestoreTerminal() {}

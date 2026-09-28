//go:build !windows

package procutil

import "os"

// OpenConsole opens the controlling terminal for a direct confirmation read.
func OpenConsole() (*os.File, error) {
	return os.Open("/dev/tty")
}

// ConsoleName is the path OpenConsole opens, for error messages.
func ConsoleName() string { return "/dev/tty" }

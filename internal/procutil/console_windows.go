//go:build windows

package procutil

import "os"

// OpenConsole opens the console for a direct confirmation read. /dev/tty
// does not exist on Windows; CONIN$ is the equivalent.
func OpenConsole() (*os.File, error) {
	return os.OpenFile("CONIN$", os.O_RDWR, 0)
}

// ConsoleName is the path OpenConsole opens, for error messages.
func ConsoleName() string { return "CONIN$" }

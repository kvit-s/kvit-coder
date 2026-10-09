package procutil

import (
	"os"

	"github.com/mattn/go-isatty"
)

// IsTerminal reports whether f is connected to a terminal.
//
// It exists because os.File.Stat's ModeCharDevice test misses the emulated
// terminals Windows developers actually use: Git Bash (mintty), Cygwin and
// MSYS2 present stdio as pipes rather than console handles, so a
// ModeCharDevice check says there is nobody at the terminal when there is.
// go-isatty knows those pipes by name and reports them as terminals.
//
// The stdin-reader gate was built on that wrong answer: under Git Bash the
// Question tool drew its prompt and immediately reported "no one answered"
// instead of waiting, and headless progress and colors stayed off.
func IsTerminal(f *os.File) bool {
	if f == nil {
		return false
	}
	fd := f.Fd()
	return isatty.IsTerminal(fd) || isatty.IsCygwinTerminal(fd)
}

package procutil

import (
	"os"
	"testing"
)

// TestIsTerminalNil: a nil file is not a terminal.
func TestIsTerminalNil(t *testing.T) {
	if IsTerminal(nil) {
		t.Error("IsTerminal(nil) = true, want false")
	}
}

// TestIsTerminalPipe: an anonymous pipe is not a terminal, on any platform.
// (An MSYS2/mintty pty is a named pipe go-isatty recognizes; os.Pipe makes
// no such pipe.)
func TestIsTerminalPipe(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	defer r.Close()
	defer w.Close()
	if IsTerminal(r) {
		t.Error("IsTerminal(pipe reader) = true, want false")
	}
	if IsTerminal(w) {
		t.Error("IsTerminal(pipe writer) = true, want false")
	}
}

// TestIsTerminalDevNull: the null device is not a terminal even though it
// is a character device.
func TestIsTerminalDevNull(t *testing.T) {
	f, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open %s: %v", os.DevNull, err)
	}
	defer f.Close()
	if IsTerminal(f) {
		t.Errorf("IsTerminal(%s) = true, want false", os.DevNull)
	}
}

// TestIsTerminalRegularFile: a redirected file is not a terminal.
func TestIsTerminalRegularFile(t *testing.T) {
	f, err := os.CreateTemp("", "isterminal-*")
	if err != nil {
		t.Fatalf("CreateTemp: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if IsTerminal(f) {
		t.Errorf("IsTerminal(%s) = true, want false", f.Name())
	}
}

// The positive cases — a real console, and a mintty/Cygwin pty pipe — need
// the terminal itself and cannot be made portably in a test; they rest on
// go-isatty's own suite.

package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// claimStdio takes the process's standard input and output for the protocol
// and returns them. Everything else that would use them is moved aside: file
// descriptor 1 becomes a copy of standard error, so a stray print or a child
// process that inherits standard output writes to the log rather than into a
// message, and descriptor 0 reads nothing, so a command the model runs cannot
// swallow the client's next message. The two copies kept for the protocol are
// closed on exec, so no child inherits them.
func claimStdio() (io.Reader, io.Writer, error) {
	in, err := unix.FcntlInt(0, unix.F_DUPFD_CLOEXEC, 3)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot keep standard input: %w", err)
	}
	out, err := unix.FcntlInt(1, unix.F_DUPFD_CLOEXEC, 3)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot keep standard output: %w", err)
	}
	if err := unix.Dup2(2, 1); err != nil {
		return nil, nil, fmt.Errorf("cannot point standard output at standard error: %w", err)
	}
	if null, err := os.Open(os.DevNull); err == nil {
		_ = unix.Dup2(int(null.Fd()), 0)
		null.Close()
	}
	return os.NewFile(uintptr(in), "acp-input"), os.NewFile(uintptr(out), "acp-output"), nil
}

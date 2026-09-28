//go:build windows

package main

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/windows"
)

// claimStdio takes the process's standard input and output for the protocol
// and returns them. It is the Windows counterpart of the unix dup2 version:
// the protocol keeps duplicates of stdin/stdout, stdout is pointed at
// stderr so a stray print or a child inheriting standard output writes to
// the log rather than into a message, and stdin reads nothing so a command
// the model runs cannot swallow the client's next message.
func claimStdio() (io.Reader, io.Writer, error) {
	stdinH, err := windows.GetStdHandle(windows.STD_INPUT_HANDLE)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get standard input: %w", err)
	}
	stdoutH, err := windows.GetStdHandle(windows.STD_OUTPUT_HANDLE)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get standard output: %w", err)
	}
	stderrH, err := windows.GetStdHandle(windows.STD_ERROR_HANDLE)
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get standard error: %w", err)
	}

	proc, err := windows.GetCurrentProcess()
	if err != nil {
		return nil, nil, fmt.Errorf("cannot get current process: %w", err)
	}

	var inDup, outDup windows.Handle
	// Duplicate without inherit handle: the CLOEXEC equivalent, so no child
	// inherits the protocol pipes.
	if err := windows.DuplicateHandle(proc, stdinH, proc, &inDup, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		return nil, nil, fmt.Errorf("cannot keep standard input: %w", err)
	}
	if err := windows.DuplicateHandle(proc, stdoutH, proc, &outDup, 0, false, windows.DUPLICATE_SAME_ACCESS); err != nil {
		windows.CloseHandle(inDup)
		return nil, nil, fmt.Errorf("cannot keep standard output: %w", err)
	}

	// Point standard output at standard error so stray prints go to the log.
	if err := windows.SetStdHandle(windows.STD_OUTPUT_HANDLE, stderrH); err != nil {
		windows.CloseHandle(inDup)
		windows.CloseHandle(outDup)
		return nil, nil, fmt.Errorf("cannot point standard output at standard error: %w", err)
	}

	// Point standard input at NUL so reads get nothing.
	if nul, err := os.Open(os.DevNull); err == nil {
		var nulH windows.Handle
		// os.File on Windows wraps a HANDLE; Fd() returns it.
		nulH = windows.Handle(nul.Fd())
		var nulDup windows.Handle
		if err := windows.DuplicateHandle(proc, nulH, proc, &nulDup, 0, true, windows.DUPLICATE_SAME_ACCESS); err == nil {
			_ = windows.SetStdHandle(windows.STD_INPUT_HANDLE, nulDup)
			// The duplicated NUL handle stays open as stdin; close the original.
		}
		nul.Close()
	}

	return os.NewFile(uintptr(inDup), "acp-input"), os.NewFile(uintptr(outDup), "acp-output"), nil
}

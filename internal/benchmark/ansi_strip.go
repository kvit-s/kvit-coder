package benchmark

import (
	"io"
	"regexp"
)

// ansiStripWriter is an io.Writer that strips ANSI escape codes before writing.
type ansiStripWriter struct {
	writer io.Writer
	// Regex to match ANSI escape sequences
	ansiRegex *regexp.Regexp
}

// NewANSIStripWriter creates a new writer that strips ANSI escape codes.
func NewANSIStripWriter(w io.Writer) io.Writer {
	return &ansiStripWriter{
		writer:    w,
		ansiRegex: regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`),
	}
}

// Write strips ANSI codes from p and writes to the underlying writer.
func (w *ansiStripWriter) Write(p []byte) (n int, err error) {
	// Strip ANSI codes
	stripped := w.ansiRegex.ReplaceAll(p, []byte{})
	
	// Write to underlying writer
	_, err = w.writer.Write(stripped)
	
	// Return the original length to satisfy io.Writer contract
	// (callers expect us to consume all input)
	return len(p), err
}

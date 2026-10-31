package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// attachPastes copies each -paste file into the session's pastes/ and adds a
// line per file to the prompt saying which [pasteN] tag it is, how long it is
// and where it is, so that the model reads it with Read instead of receiving
// the text in the message. The files are numbered in the order given, which
// is the order the UI numbered the tags in. A file that cannot be read or
// copied is reported to the model as unavailable rather than failing the
// turn: the rest of the prompt is still worth running.
func attachPastes(prompt string, paths []string, sess *session.Session, writer *ui.Writer) string {
	if len(paths) == 0 {
		return prompt
	}
	dir := filepath.Join(os.TempDir(), "kvit-coder-pastes")
	if sess != nil {
		dir = sess.PastesDir()
	}
	stamp := time.Now().Format("20060102-150405")
	notes := make([]string, 0, len(paths))
	for i, p := range paths {
		label := ui.PasteLabel(i + 1)
		dest, about, err := copyPaste(p, dir, fmt.Sprintf("paste%d-%s-*.txt", i+1, stamp))
		if err != nil {
			writer.Warn(fmt.Sprintf("paste %q: %v", p, err))
			notes = append(notes, fmt.Sprintf("%s was pasted but could not be saved, so its text is not available.", label))
			continue
		}
		notes = append(notes, fmt.Sprintf("%s is text the user pasted (%s), saved as %s. Read that file to see it.", label, about, dest))
	}
	return strings.TrimRight(prompt, "\n") + "\n\n" + strings.Join(notes, "\n")
}

// copyPaste copies one paste into dir under a new name made from pattern and
// returns where it went and how long it is.
func copyPaste(src, dir, pattern string) (dest, about string, err error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return "", "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", "", err
	}
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return "", "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", "", err
	}
	if err := f.Close(); err != nil {
		os.Remove(f.Name())
		return "", "", err
	}
	return f.Name(), ui.DescribeText(string(data)), nil
}

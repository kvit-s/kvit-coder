package tui

import (
	"fmt"
	"os"

	"github.com/kvit-s/kvit-coder/internal/ui"
)

// StagePastedText saves a long paste where the agent will find it, the way
// SaveClipboardTemp does for a pasted image. The agent copies each -paste file
// into the session's pastes/ when the turn starts, so plain temp is enough
// here, and the UI removes its copy once the turn is over.
func StagePastedText(text string) (string, error) {
	f, err := os.CreateTemp("", "kvit-coder-paste-*.txt")
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.WriteString(text); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// printPastes lists the long pastes a turn carries, numbered like the
// [pasteN] tags in the prompt echoed above it.
func printPastes(paths []string) {
	for i, p := range paths {
		about := "unreadable"
		if data, err := os.ReadFile(p); err == nil {
			about = ui.DescribeText(string(data))
		}
		fmt.Printf("\033[38;5;136m[paste%d: %s]\033[0m\n", i+1, about)
	}
}

// unsentPastes returns the staged pastes that are not among the sent ones.
func unsentPastes(staged, sent []string) []string {
	keep := make(map[string]bool, len(sent))
	for _, p := range sent {
		keep[p] = true
	}
	var out []string
	for _, p := range staged {
		if !keep[p] {
			out = append(out, p)
		}
	}
	return out
}

// removePastes deletes the UI's temporary copies of long pastes once the
// agent has had them, or once it is clear it never will.
func removePastes(paths []string) {
	for _, p := range paths {
		os.Remove(p)
	}
}

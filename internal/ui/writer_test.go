package ui

import (
	"bytes"
	"strings"
	"testing"
)

// newProgressWriter builds a headless writer whose stderr is a buffer, with the
// terminal question answered explicitly rather than guessed from the file.
func newProgressWriter(isTTY bool) (*Writer, *bytes.Buffer) {
	var buf bytes.Buffer
	w := NewWriter(0)
	w.SetHeadless(true)
	w.SetStderr(&buf)
	w.SetStderrIsTerminal(isTTY)
	// The indicator is package state, so reset it between tests.
	progressLine = ""
	progressDotCount = 0
	return w, &buf
}

// TestHeadlessProgressReachesATerminal is the bug that made a long command look
// like a hang: the agent runs headless under the interactive UI, and progress
// was accumulated into a variable and never printed, so nothing at all appeared
// between a tool's call line and its result.
func TestHeadlessProgressReachesATerminal(t *testing.T) {
	w, buf := newProgressWriter(true)

	w.ToolProgress("✨ ")
	for range 3 {
		w.ToolProgress(".")
	}

	out := buf.String()
	if out == "" {
		t.Fatal("a headless run wrote no progress at all to a terminal")
	}
	if !strings.Contains(out, "...") {
		t.Errorf("progress does not show the accumulated dots: %q", out)
	}
	if !strings.Contains(out, "\r") {
		t.Errorf("progress is not redrawn in place: %q", out)
	}
}

// TestHeadlessProgressStaysQuietWhenPiped: redrawing in place would put control
// characters into a log file or a benchmark transcript, so a non-terminal
// stderr gets nothing — which is what it got before, and what the benchmark
// harness parses.
func TestHeadlessProgressStaysQuietWhenPiped(t *testing.T) {
	w, buf := newProgressWriter(false)

	w.ToolProgress("✨ ")
	for range 5 {
		w.ToolProgress(".")
	}

	if out := buf.String(); out != "" {
		t.Errorf("a piped headless run wrote progress characters: %q", out)
	}
}

// TestProgressFinalLineIsDrawnOnce: the closing update carries the accumulated
// dots plus the timing, and resets the line for the next tool.
func TestProgressFinalLineIsDrawnOnce(t *testing.T) {
	w, buf := newProgressWriter(true)

	w.ToolProgress("✨ ")
	w.ToolProgress(".")
	w.ToolProgress(".")
	buf.Reset()
	w.ToolProgress("  ...2s\n")

	out := buf.String()
	if !strings.Contains(out, "...2s") {
		t.Errorf("the final progress line lost its timing: %q", out)
	}
	if progressLine != "" || progressDotCount != 0 {
		t.Errorf("the indicator was not reset after the final line: %q / %d", progressLine, progressDotCount)
	}
}

// TestProgressWrapsAfterAMinute: a command running for minutes must not push a
// single line off the side of the terminal.
func TestProgressWrapsAfterAMinute(t *testing.T) {
	w, buf := newProgressWriter(true)

	w.ToolProgress("✨ ")
	for range maxDotsPerLine + 2 {
		w.ToolProgress(".")
	}

	if !strings.Contains(buf.String(), "\n") {
		t.Errorf("progress never wrapped after %d dots", maxDotsPerLine)
	}
	if progressDotCount > maxDotsPerLine {
		t.Errorf("the dot counter is %d, want it reset by the wrap", progressDotCount)
	}
}

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
	progressHeld = false
	return w, &buf
}

// TestProgressHoldsWhileAQuestionIsWaiting is the bug that made a path
// confirmation look like a hang. The agent asks "Allow this access? [y/N]:"
// and blocks; the tool's progress goroutine keeps drawing, and each update
// starts with a carriage return, so the question is overwritten by the dots a
// character at a time. Nothing on screen then says an answer is wanted, and
// the run appears wedged until it is interrupted.
func TestProgressHoldsWhileAQuestionIsWaiting(t *testing.T) {
	w, buf := newProgressWriter(true)

	asking := false
	w.SetPromptWatcher(func() bool { return asking })

	w.ToolProgress("✨ ")
	w.ToolProgress(".")

	asking = true
	buf.Reset()
	for range 5 {
		w.ToolProgress(".")
	}
	if out := buf.String(); out != "" {
		t.Errorf("progress was drawn over a waiting question: %q", out)
	}

	// Answered: the row starts again below the question rather than
	// reprinting what was collected before it.
	asking = false
	buf.Reset()
	w.ToolProgress(".")

	out := buf.String()
	if !strings.Contains(out, ".") {
		t.Errorf("progress did not resume after the question was answered: %q", out)
	}
	if progressDotCount != 1 {
		t.Errorf("the dot counter is %d, want the row restarted at 1", progressDotCount)
	}
	if progressLine != "." {
		t.Errorf("the resumed row is %q, want it to start over at a single dot", progressLine)
	}
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

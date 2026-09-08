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

// TestProgressHoldsWhilePauseRequested is the bug that garbled steering typed
// after an empty Enter: the current call finishes first, and its per-second
// dots redrew the line the type-ahead echo was on, doubling the text. A
// pending pause holds dots the same way a waiting question does, and the row
// restarts once the turn parks and resumes.
func TestProgressHoldsWhilePauseRequested(t *testing.T) {
	w, buf := newProgressWriter(true)

	pausePending := false
	w.SetPromptWatcher(func() bool { return pausePending })

	w.ToolProgress("✨ ")
	w.ToolProgress(".")

	pausePending = true
	buf.Reset()
	for range 5 {
		w.ToolProgress(".")
	}
	if out := buf.String(); out != "" {
		t.Errorf("progress was drawn over type-ahead after a pause request: %q", out)
	}

	// Parked and resumed: the row starts again rather than reprinting what
	// was held.
	pausePending = false
	buf.Reset()
	w.ToolProgress(".")

	out := buf.String()
	if !strings.Contains(out, ".") {
		t.Errorf("progress did not resume after the pause was taken: %q", out)
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

// TestProgressFoldsMinuteIntoStar: a command running for minutes stays on one
// redrawn row. A finished minute of dots folds into "*", so the row never
// stacks new lines. The row is bare dots and stars: no leading "●", which is
// reserved for Thinking step headers and reads as model thinking on a wait.
func TestProgressFoldsMinuteIntoStar(t *testing.T) {
	w, buf := newProgressWriter(true)

	w.ToolProgress("✨ ")
	for range maxDotsPerLine {
		w.ToolProgress(".")
	}
	if strings.Contains(progressLine, "*") {
		t.Fatalf("a single minute of dots folded early: %q", progressLine)
	}
	w.ToolProgress(".")
	w.ToolProgress(".")

	if progressLine != "*.." {
		t.Errorf("folded row = %q, want %q (one star, current dots, no bullet)", progressLine, "*..")
	}
	if strings.Contains(progressLine, "●") {
		t.Errorf("wait row %q starts with a thinking bullet", progressLine)
	}
	if progressDotCount != 2 {
		t.Errorf("the dot counter is %d, want 2 for the dots after the star", progressDotCount)
	}
	out := buf.String()
	if !strings.Contains(out, "*") {
		t.Errorf("the drawn row never showed its star: %q", out)
	}
	if !strings.Contains(out, "\033[K") {
		t.Errorf("the shrinking redraw did not clear to end of line: %q", out)
	}
	// Every newline the indicator draws is its "\n\033[1A" flush-and-return;
	// a bare "\n" would be a second stacked line (the old wrap).
	if n, flushed := strings.Count(out, "\n"), strings.Count(out, "\n\033[1A"); n != flushed {
		t.Errorf("the row stacked a second line: %d newlines but %d flush-and-returns in %q", n, flushed, out)
	}
}

// TestToolProgressFoldKeepsNoBullet: a tool wait is bare dots and stars —
// that was the shape that printed "● ..." under a Shell line and read as a
// thinking step.
func TestToolProgressFoldKeepsNoBullet(t *testing.T) {
	w, _ := newProgressWriter(true)

	for range maxDotsPerLine + 1 {
		w.ToolProgress(".")
	}

	if progressLine != "*." {
		t.Errorf("folded tool row = %q, want %q (star, dots, no bullet)", progressLine, "*.")
	}
	if strings.Contains(progressLine, "●") {
		t.Errorf("folding added a bullet to a tool row: %q", progressLine)
	}
	_ = w
}

// TestProgressSecondMinuteAddsSecondStar: each finished minute adds one more
// star, so a long wait reads as elapsed minutes plus current dots.
func TestProgressSecondMinuteAddsSecondStar(t *testing.T) {
	w, _ := newProgressWriter(true)

	w.ToolProgress("● ")
	for range 2*maxDotsPerLine + 2 {
		w.ToolProgress(".")
	}

	if progressLine != "**.." {
		t.Errorf("two-minute row = %q, want %q", progressLine, "**..")
	}
	_ = w
}

// TestThinkingStepHeader is the codex-style step line: bullet, brownish
// status, white message, with continuations indented.
func TestThinkingStepHeader(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.Thinking("8.4k 1%", "hello")
	out := buf.String()
	if out != "● (8.4k 1%) hello\n" {
		t.Errorf("header = %q, want bullet, status and message on one line", out)
	}
}

// TestThinkingEmptyMessageStillShowsStatus: a turn with only tool calls
// still prints its status line.
func TestThinkingEmptyMessageStillShowsStatus(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.Thinking("8.4k 1%", "")
	if out := buf.String(); out != "● (8.4k 1%)\n" {
		t.Errorf("empty message header = %q, want status alone", out)
	}
}

// TestThinkingContinuationIndented: embedded newlines stay under the header.
func TestThinkingContinuationIndented(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.Thinking("17.4k 2%", "first\nsecond")
	lines := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2: %q", len(lines), buf.String())
	}
	if !strings.HasPrefix(lines[1], "  ") {
		t.Errorf("continuation %q is not indented", lines[1])
	}
	if strings.Contains(lines[1], "●") || strings.Contains(lines[1], "(17.4k") {
		t.Errorf("continuation %q repeats the bullet or status", lines[1])
	}
}

// TestToolCallIgnoresContext: the status lives on the header, so call lines
// carry only the call, indented gray, with no repeated context.
func TestToolCallIgnoresContext(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.ToolCall("Read", `path="a.go"`, "8.4k 1%")
	if out := buf.String(); out != "  Read[path=\"a.go\"]\n" {
		t.Errorf("tool call = %q, want indented call without status", out)
	}
}

// TestToolResultSuccessSilent: size summaries no longer print an arrow line.
func TestToolResultSuccessSilent(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.ToolResult("34 lines, 572 chars", "")
	if out := buf.String(); out != "" {
		t.Errorf("success summary printed %q, want silence", out)
	}
}

// TestToolResultKeepsDuration: a slow tool still explains its dots.
func TestToolResultKeepsDuration(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.ToolResult("", "...2s")
	if out := buf.String(); out != "  ...2s\n" {
		t.Errorf("duration = %q, want indented duration line", out)
	}
}

// TestWarnErrorSingleLineIndented: warnings and errors collapse to one
// indented line each.
func TestWarnErrorSingleLineIndented(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.Warn("first\nsecond")
	if out := buf.String(); out != "  [warn] first second\n" {
		t.Errorf("warn = %q, want one indented line", out)
	}
	buf.Reset()
	progressLine = ""
	w.Error("a\nb\nc")
	if out := buf.String(); out != "  [error] a b c\n" {
		t.Errorf("error = %q, want one indented line", out)
	}
}

// TestDividerCarriesTiming: the divider separates step progress from the
// final report and embeds the turn timing, replacing the standalone
// "[39s: ...]" line that used to follow the report. A blank line leads so
// it never crowds the last step line.
func TestDividerCarriesTiming(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.Divider("[39s: 39s llm + 0s tools x22]")
	if out := buf.String(); out != "\n---------  [39s: 39s llm + 0s tools x22] --------\n" {
		t.Errorf("divider = %q, want blank line then dashes around the timing", out)
	}
}

// TestDividerSuppressed: quiet and JSON modes print no divider, keeping
// stdout clean for the answer alone or the JSON document.
func TestDividerSuppressed(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.SetQuiet(true)
	w.Divider("[1s: 1s llm]")
	if out := buf.String(); out != "" {
		t.Errorf("quiet divider printed %q, want silence", out)
	}

	w2, buf2 := newProgressWriter(false)
	w2.SetQuiet(false)
	w2.SetJSONMode(true)
	w2.Divider("[1s: 1s llm]")
	if out := buf2.String(); out != "" {
		t.Errorf("json divider printed %q, want silence", out)
	}
}

// TestFormatContextStrWithProcs: the star count rides on the status only
// while background processes run, and zero keeps the header byte-identical.
func TestFormatContextStrWithProcs(t *testing.T) {
	if got := FormatContextStrWithProcs(8400, 1000000, 0); got != "8.4k 1%" {
		t.Errorf("no procs = %q, want base status unchanged", got)
	}
	if got := FormatContextStrWithProcs(8400, 1000000, 2); got != "8.4k 1% ★2" {
		t.Errorf("two procs = %q, want %q", got, "8.4k 1% ★2")
	}
	if got := FormatContextStrWithProcs(0, 1000000, 1); got != "0k 0% ★1" {
		t.Errorf("zero tokens = %q, want star still shown", got)
	}
	if got := FormatContextStrWithProcs(8400, 0, 3); got != "8.4k ★3" {
		t.Errorf("no limit = %q, want star on the bare count", got)
	}
	if got := FormatContextStr(8400, 1000000); got != "8.4k 1%" {
		t.Errorf("base helper = %q, want no star", got)
	}
}

// TestFormatContextStrWithProcs2: ephemeral ride blue as ★N, persistent red
// as ★N!, and zero of each keeps the header byte-identical.
func TestFormatContextStrWithProcs2(t *testing.T) {
	if got := FormatContextStrWithProcs2(8400, 1000000, 0, 0); got != "8.4k 1%" {
		t.Errorf("no procs = %q, want base status unchanged", got)
	}
	if got := FormatContextStrWithProcs2(8400, 1000000, 2, 0); got != "8.4k 1% ★2" {
		t.Errorf("ephemeral = %q, want blue star without bang", got)
	}
	if got := FormatContextStrWithProcs2(8400, 1000000, 0, 1); got != "8.4k 1% ★1!" {
		t.Errorf("persistent = %q, want red star with bang", got)
	}
	if got := FormatContextStrWithProcs2(8400, 1000000, 2, 1); got != "8.4k 1% ★2 ★1!" {
		t.Errorf("both = %q, want ephemeral then persistent", got)
	}
	if got := FormatContextStrWithProcs2(0, 1000000, 0, 3); got != "0k 0% ★3!" {
		t.Errorf("zero tokens = %q, want persistent star still shown", got)
	}
	// The old single-count shape still counts everything as ephemeral.
	if got := FormatContextStrWithProcs(8400, 1000000, 2); got != "8.4k 1% ★2" {
		t.Errorf("legacy = %q, want %q", got, "8.4k 1% ★2")
	}
}

// TestThinkingShowsBackgroundProcs: a running process count appears inside
// the parens, as plain text when colors are off.
func TestThinkingShowsBackgroundProcs(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.Thinking("8.4k 1% ★2", "hello")
	if out := buf.String(); out != "● (8.4k 1% ★2) hello\n" {
		t.Errorf("header = %q, want star count inside the status", out)
	}
}

// TestThinkingStarPaintedBlue: on a terminal the star count is dim blue
// while the rest of the status stays brownish-dim.
func TestThinkingStarPaintedBlue(t *testing.T) {
	w, buf := newProgressWriter(true)
	w.Thinking("8.4k 1% ★2", "hello")
	out := buf.String()
	if !strings.Contains(out, ansiBlue+"★2"+ansiReset) {
		t.Errorf("header %q has no blue star count", out)
	}
	if !strings.Contains(out, ansiBrown+"(8.4k 1% ") {
		t.Errorf("header %q lost the brown context base", out)
	}
}

// TestThinkingWithoutStarUnchanged: no running processes means no blue and
// the same brown status as before.
func TestThinkingWithoutStarUnchanged(t *testing.T) {
	w, buf := newProgressWriter(true)
	w.Thinking("8.4k 1%", "hello")
	out := buf.String()
	if strings.Contains(out, ansiBlue) {
		t.Errorf("header %q paints blue with no background processes", out)
	}
	if !strings.Contains(out, ansiBrown+"(8.4k 1%)"+ansiReset) {
		t.Errorf("header %q lost its brown status", out)
	}
}

// TestSplitProcStar rejects anything that is not a trailing ★N count, so a
// message that happens to contain a star never steals the blue.
func TestSplitProcStar(t *testing.T) {
	if _, _, ok := splitProcStar("8.4k 1%"); ok {
		t.Error("plain status split as a star")
	}
	base, star, ok := splitProcStar("8.4k 1% ★2")
	if !ok || base != "8.4k 1%" || star != "★2" {
		t.Errorf("split = %q %q %v, want base, star and true", base, star, ok)
	}
	for _, bad := range []string{"8.4k ★", "8.4k ★x", "8.4k ★2x", "8.4k ★ 2", "★2 hello"} {
		if _, _, ok := splitProcStar(bad); ok {
			t.Errorf("%q split as a star", bad)
		}
	}
}

// TestSplitProcStars separates the blue ephemeral star from the red
// persistent one, so each keeps its color and plain text keeps the "!".
func TestSplitProcStars(t *testing.T) {
	base, blue, red, ok := splitProcStars("8.4k 1% ★2 ★1!")
	if !ok || base != "8.4k 1%" || blue != "★2" || red != "★1!" {
		t.Errorf("split = %q %q %q %v, want base and both stars", base, blue, red, ok)
	}
	if _, blue, red, ok := splitProcStars("8.4k 1% ★1!"); !ok || blue != "" || red != "★1!" {
		t.Errorf("persistent-only split = %q %q %v, want red alone", blue, red, ok)
	}
	if _, blue, red, ok := splitProcStars("8.4k 1% ★2"); !ok || blue != "★2" || red != "" {
		t.Errorf("ephemeral-only split = %q %q %v, want blue alone", blue, red, ok)
	}
	if _, _, _, ok := splitProcStars("8.4k 1%"); ok {
		t.Error("plain status split as stars")
	}
}

// TestThinkingPersistentStarPaintedRed: on a terminal the persistent count
// is dim red with its bang, the ephemeral count stays dim blue.
func TestThinkingPersistentStarPaintedRed(t *testing.T) {
	w, buf := newProgressWriter(true)
	w.Thinking("8.4k 1% ★2 ★1!", "hello")
	out := buf.String()
	if !strings.Contains(out, ansiBlue+"★2"+ansiReset) {
		t.Errorf("header %q lost the blue ephemeral star", out)
	}
	if !strings.Contains(out, ansiDimRed+"★1!"+ansiReset) {
		t.Errorf("header %q has no red persistent star", out)
	}
}

// TestThinkingPersistentPlainKeepsBang: without colors the "!" is what tells
// the surviving star from the ephemeral one.
func TestThinkingPersistentPlainKeepsBang(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.Thinking("8.4k 1% ★1!", "hello")
	if out := buf.String(); out != "● (8.4k 1% ★1!) hello\n" {
		t.Errorf("header = %q, want the bang in plain text", out)
	}
}

// TestSteeringPrintsGreenOnTerminal: the pause prompt marks the moment
// typing steering becomes available, so on a terminal it is bright green,
// not the gray of surrounding progress lines.
func TestSteeringPrintsGreenOnTerminal(t *testing.T) {
	w, buf := newProgressWriter(true)
	w.Steering("paused — type steering, Enter to resume (empty resumes):")
	out := buf.String()
	if !strings.Contains(out, ansiGreen) {
		t.Errorf("steering prompt %q has no green", out)
	}
	if !strings.Contains(out, "paused") {
		t.Errorf("steering prompt %q lost its text", out)
	}
	if strings.Contains(out, ansiGray) {
		t.Errorf("steering prompt %q reuses the gray info color", out)
	}
}

// TestSteeringPlainWhenPiped: piped output stays plain, like Info.
func TestSteeringPlainWhenPiped(t *testing.T) {
	w, buf := newProgressWriter(false)
	w.Steering("paused — type steering, Enter to resume (empty resumes):")
	if out := buf.String(); out != "  paused — type steering, Enter to resume (empty resumes):\n" {
		t.Errorf("steering = %q, want plain indented line", out)
	}
}

// TestSteeringQuietSuppresses: quiet mode prints no steering prompt.
func TestSteeringQuietSuppresses(t *testing.T) {
	w, buf := newProgressWriter(true)
	w.SetQuiet(true)
	w.Steering("paused — type steering, Enter to resume (empty resumes):")
	if out := buf.String(); out != "" {
		t.Errorf("quiet steering printed %q, want silence", out)
	}
}

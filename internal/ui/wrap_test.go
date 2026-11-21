package ui

import (
	"os"
	"strings"
	"testing"
)

// TestToolCallFromCrashedSession prints the tool call that ended a run on
// 20 November 2026 with "slice bounds out of range [98:97]" in wrapLine. It is
// the last assistant message of the session kvit-hub-gather-20261120-161851-018:
// a Shell call whose working_dir was the workspace and whose timeout was 60, so
// what reached the writer was the command and ", timeout=60s". Its first word,
// after "Shell[", is 97 characters: longer than a continuation line holds
// (100 less a four-space indent) yet shorter than the 98 the first line had
// left, and cutting 98 characters from it was the panic.
func TestToolCallFromCrashedSession(t *testing.T) {
	t.Setenv("COLUMNS", "")
	command, err := os.ReadFile("testdata/crash-shell-command.txt")
	if err != nil {
		t.Fatal(err)
	}
	args := string(command) + ", timeout=60s"

	w, buf := newProgressWriter(false)
	w.ToolCall("Shell", args, "")

	out := strings.TrimSuffix(buf.String(), "\n")
	lines := strings.Split(out, "\n")
	if len(lines) < 2 {
		t.Fatalf("expected the call wrapped over several lines, got %q", out)
	}
	if !strings.HasPrefix(lines[0], `  Shell[IN="/d/kvit-hub-rehearsal-2/`) {
		t.Errorf("first line = %q", lines[0])
	}
	for _, ln := range lines {
		if n := len([]rune(ln)); n > 100 {
			t.Errorf("a %d-column line was printed at width 100: %q", n, ln)
		}
	}
	// Nothing is lost to the wrapping: the words printed are the words of the
	// one-line form the call is shortened to.
	want := "Shell[" + SingleLine(args, maxToolArgChars) + "]"
	if got := strings.Join(strings.Fields(out), ""); got != strings.Join(strings.Fields(want), "") {
		t.Errorf("printed text differs from the call:\n got %q\nwant %q", got, want)
	}
}

// TestWrapLineAnyInput runs wrapLine over every combination of a range of
// widths, prefix lengths and word lengths, including widths narrower than the
// prefixes. It must return, keep every character of the text, and keep each
// line within the width unless the line's prefix alone is already wider.
func TestWrapLineAnyInput(t *testing.T) {
	texts := []string{"x", "a bb ccc", "ü€😀 ü€😀ü€😀"}
	for n := 1; n <= 40; n += 3 {
		texts = append(texts,
			strings.Repeat("w", n),
			"ab "+strings.Repeat("w", n),
			strings.Repeat("w", n)+" cd "+strings.Repeat("v", n+5),
			strings.Repeat("é", n)+" "+strings.Repeat("😀", n))
	}
	for width := -1; width <= 30; width++ {
		for p := 0; p <= 6; p++ {
			for c := 0; c <= 6; c++ {
				prefix, cont := strings.Repeat(">", p), strings.Repeat(" ", c)
				for _, text := range texts {
					checkWrap(t, prefix, text, width, cont)
				}
			}
		}
	}
}

func checkWrap(t *testing.T, prefix, text string, width int, cont string) {
	t.Helper()
	var lines []string
	func() {
		defer func() {
			if p := recover(); p != nil {
				t.Fatalf("wrapLine(%q, %q, %d, %q) panicked: %v", prefix, text, width, cont, p)
			}
		}()
		lines = wrapLine(prefix, text, width, cont)
	}()
	if len(lines) == 0 {
		t.Fatalf("wrapLine(%q, %q, %d, %q) returned no lines", prefix, text, width, cont)
	}
	var kept strings.Builder
	for i, ln := range lines {
		pre := cont
		if i == 0 {
			pre = prefix
		}
		if !strings.HasPrefix(ln, pre) {
			t.Fatalf("wrapLine(%q, %q, %d, %q) line %d %q lacks its prefix", prefix, text, width, cont, i, ln)
		}
		body := strings.TrimPrefix(ln, pre)
		if n := visibleLen(ln); n > width && n > visibleLen(pre)+1 {
			t.Fatalf("wrapLine(%q, %q, %d, %q) line %d %q is %d wide", prefix, text, width, cont, i, ln, n)
		}
		kept.WriteString(body)
	}
	if got, want := strings.ReplaceAll(kept.String(), " ", ""), strings.ReplaceAll(text, " ", ""); got != want {
		t.Fatalf("wrapLine(%q, %q, %d, %q) kept %q of %q", prefix, text, width, cont, got, want)
	}
}

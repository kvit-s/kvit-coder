package markdown

import (
	"strings"
	"testing"
)

func TestParseMode(t *testing.T) {
	cases := map[string]Mode{
		"":        ModeAuto,
		"auto":    ModeAuto,
		" AUTO ":  ModeAuto,
		"always":  ModeAlways,
		"Always":  ModeAlways,
		"never":   ModeNever,
		"NEVER":   ModeNever,
		"bogus":   ModeAuto,
		"tty":     ModeAuto,
		"pretty":  ModeAuto,
		"colored": ModeAuto,
	}
	for in, want := range cases {
		if got := ParseMode(in); got != want {
			t.Errorf("ParseMode(%q) = %q, want %q", in, got, want)
		}
	}
}

// TestRenderStylesHeading: the point of the package. A heading comes back
// with ANSI styling, its text intact, and no markdown source left over.
func TestRenderStylesHeading(t *testing.T) {
	out := Render("# Done\n\nsome body text\n", 80)
	if !strings.Contains(out, "\x1b[") {
		t.Errorf("styled heading has no ANSI codes: %q", out)
	}
	if !strings.Contains(out, "Done") {
		t.Errorf("styled heading lost its text: %q", out)
	}
	if strings.Contains(out, "# Done") {
		t.Errorf("styled heading kept its markdown source: %q", out)
	}
	if out == "# Done\n\nsome body text\n" {
		t.Errorf("render returned the input unchanged")
	}
}

// TestRenderKeepsCodeAndEmphasis: bold markers go, bold text stays, and
// fenced code content survives verbatim.
func TestRenderKeepsCodeAndEmphasis(t *testing.T) {
	src := "Fixed **two bugs** in `writer.go`:\n\n```go\nfmt.Println(\"hi\")\n```\n"
	out := Render(src, 80)
	// Chroma highlights code token by token, so the fence body arrives
	// with styling between tokens rather than as one contiguous string.
	for _, want := range []string{"two bugs", "writer.go", "fmt", "Println", `"hi"`} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output lost %q", want)
		}
	}
	if strings.Contains(out, "**two bugs**") {
		t.Errorf("rendered output kept emphasis markers")
	}
	if strings.Contains(out, "```") {
		t.Errorf("rendered output kept fence markers")
	}
}

// TestRenderTable: cell values survive and the table is redrawn (no raw
// pipe separators on the content lines).
func TestRenderTable(t *testing.T) {
	src := "| a | b |\n|---|---|\n| 1 | 2 |\n"
	out := Render(src, 80)
	for _, want := range []string{"a", "b", "1", "2"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered table lost %q", want)
		}
	}
	if strings.Contains(out, "| a | b |") {
		t.Errorf("rendered table kept its markdown source")
	}
}

// TestRenderListsAndQuotes: the elements a final report actually contains.
// Words are checked singly: the renderer styles spans (quote bars, code
// tokens) separately, so phrases need not be contiguous in the raw output.
func TestRenderListsAndQuotes(t *testing.T) {
	src := "- first\n- second\n\n> a note\n\n1. one\n2. two\n"
	out := Render(src, 80)
	for _, want := range []string{"first", "second", "note", "one", "two", "│"} {
		if !strings.Contains(out, want) {
			t.Errorf("rendered output lost %q", want)
		}
	}
}

// TestRenderEmptyReturnsInput: nothing to style, nothing changed.
func TestRenderEmptyReturnsInput(t *testing.T) {
	if out := Render("", 80); out != "" {
		t.Errorf("empty render = %q, want empty", out)
	}
}

// TestRenderOversizedReturnsInput: a huge report still prints, it just
// does not get colors, and rendering it must not stall the turn.
func TestRenderOversizedReturnsInput(t *testing.T) {
	src := strings.Repeat("# big\n\nbody text here\n", MaxInputBytes/22+10)
	if len(src) <= MaxInputBytes {
		t.Fatalf("test input is %d bytes, want over %d", len(src), MaxInputBytes)
	}
	if out := Render(src, 80); out != src {
		t.Errorf("oversized render changed %d bytes of input", len(src))
	}
}

// TestRenderWidthClamps: absurd widths neither crash nor lose the text.
// Words are checked singly because wrapping may split phrases across lines.
func TestRenderWidthClamps(t *testing.T) {
	src := "# Title\n\nSome paragraph text that should survive any width.\n"
	for _, width := range []int{0, -10, 40, 100, 250, 10000} {
		out := Render(src, width)
		for _, want := range []string{"Title", "paragraph", "survive", "width"} {
			if !strings.Contains(out, want) {
				t.Errorf("width %d lost %q", width, want)
			}
		}
	}
}

// TestRenderWidthWraps: a narrower width produces more lines for prose.
func TestRenderWidthWraps(t *testing.T) {
	src := "This is a long paragraph of plain prose with no markdown in it at all, " +
		"written so that wrapping behavior is what decides the line count. " +
		"Padding padding padding padding padding padding padding padding end.\n"
	narrow := len(strings.Split(strings.TrimRight(Render(src, 40), "\n"), "\n"))
	wide := len(strings.Split(strings.TrimRight(Render(src, 200), "\n"), "\n"))
	if narrow <= wide {
		t.Errorf("width 40 gave %d lines, width 200 gave %d, want more lines when narrow", narrow, wide)
	}
}

// TestRenderHeadingsStockDark: headings keep the stock dark look (h1
// yellow on a purple bar, h2 blue bold, h6 green) — reverted from the
// all-blue-bold experiment on purpose.
func TestRenderHeadingsStockDark(t *testing.T) {
	h1 := Render("# Title\n", 80)
	if !strings.Contains(h1, "48;5;63") || !strings.Contains(h1, "38;5;228") {
		t.Errorf("h1 lost its stock yellow/purple style: %q", h1)
	}
	h2 := Render("## Sub\n", 80)
	if !strings.Contains(h2, "39") {
		t.Errorf("h2 lost its stock blue style: %q", h2)
	}
	h6 := Render("###### Tiny\n", 80)
	if !strings.Contains(h6, "35") {
		t.Errorf("h6 lost its stock green style: %q", h6)
	}
}

// TestRenderInlineCodeGrayBackgroundOnly: `code` keeps the gray background
// but loses the stock red foreground, and the stock affix spaces (which
// double up with source spaces) are gone.
func TestRenderInlineCodeGrayBackgroundOnly(t *testing.T) {
	out := Render("Fixed `writer.go` foo\n", 80)
	if !strings.Contains(out, "48;5;236") {
		t.Errorf("inline code lost its gray background: %q", out)
	}
	if strings.Contains(out, "203") {
		t.Errorf("inline code kept the stock red foreground: %q", out)
	}
	visible := stripANSI(out)
	if !strings.Contains(visible, "Fixed writer.go foo") {
		t.Errorf("inline code spacing is wrong, want single spaces: %q", visible)
	}
	if strings.Contains(visible, "  ") {
		t.Errorf("inline code left double spaces: %q", visible)
	}
}

// TestRenderNoTrailingPadding: glamour pads every line to the wrap width
// with styled spaces; the renderer must strip them so short lines do not
// carry dozens of trailing spaces and lonely wrapped words do not look
// broken.
func TestRenderNoTrailingPadding(t *testing.T) {
	src := "Tradeoff vs raw mode, stated plainly: this is modal. You can't type ahead while " +
		"watching output flow — you pause, then type. Type-ahead (garbled but functional) and " +
		"second-terminal `steer` stay as-is for that. Given you don't want raw mode back, that's " +
		"the right trade: no `ttyline` package, no `Writer` funnel mutex over ~15 methods, no " +
		"restore-on-panic audit, no parent/child raw-ownership fight.\n"
	out := Render(src, 100)
	for i, ln := range strings.Split(out, "\n") {
		if vis := stripANSI(ln); vis != strings.TrimRight(vis, " \t") {
			t.Errorf("line %d has trailing padding: %q", i, vis)
		}
	}
	// The regression case: stock rendering left "stay", "ownership" and
	// "fight." each alone on a padded line (7 display lines). Full-width
	// prose plus single-spaced code fits the same paragraph in 4.
	lines := strings.Split(strings.Trim(strings.TrimRight(stripANSI(out), "\n"), "\n"), "\n")
	nonEmpty := 0
	for _, ln := range lines {
		if strings.TrimSpace(ln) != "" {
			nonEmpty++
		}
	}
	if nonEmpty > 5 {
		t.Errorf("paragraph wrapped to %d lines, want <= 5 without orphan padding lines:\n%s", nonEmpty, stripANSI(out))
	}
}

// TestRenderFullWidthProse: the stock document margin 2 indents every
// paragraph and shrinks usable width by four; prose starts at column zero.
func TestRenderFullWidthProse(t *testing.T) {
	out := Render("Hello world\n", 80)
	for _, ln := range strings.Split(stripANSI(out), "\n") {
		if strings.TrimSpace(ln) == "" {
			continue
		}
		if strings.HasPrefix(ln, "  ") {
			t.Errorf("prose kept the stock 2-space document indent: %q", ln)
		}
		if ln != "Hello world" {
			t.Errorf("prose line = %q, want %q", ln, "Hello world")
		}
		break
	}
}

// TestTrimRightANSI: unit cases for the padding stripper, including styled
// trailing spaces, blank styled lines, and clean lines left untouched.
func TestTrimRightANSI(t *testing.T) {
	styledPad := "\x1b[38;5;252mhi\x1b[0m\x1b[38;5;252m \x1b[0m\x1b[38;5;252m \x1b[0m"
	if got := trimRightANSI(styledPad); got != "\x1b[38;5;252mhi\x1b[0m" {
		t.Errorf("styled padding trim = %q, want text plus reset", got)
	}
	if got := trimRightANSI("\x1b[38;5;252m \x1b[0m\x1b[38;5;252m \x1b[0m"); got != "" {
		t.Errorf("blank padded line trim = %q, want empty", got)
	}
	clean := "\x1b[38;5;252mhi\x1b[0m"
	if got := trimRightANSI(clean); got != clean {
		t.Errorf("clean line changed: %q", got)
	}
}

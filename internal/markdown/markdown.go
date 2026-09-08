// Package markdown renders the final assistant answer as styled terminal
// output. It is the one markdown renderer in the program: Writer.Assistant
// is the only caller in non-test code.
//
// Rendering never fails a turn. Render returns the styled answer, or the
// input unchanged when the input is empty, oversized, or the renderer
// errors. Whether to render at all is decided by the caller (see
// ui.Writer), so this package does no TTY or environment checks itself:
// Render always styles, which also keeps it deterministic under test.
package markdown

import (
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
)

// MaxInputBytes caps the answer Render will style. Above it the raw
// markdown goes through unchanged: a huge report still prints, it just
// does not get colors. 256 KiB is far above any report worth styling
// line by line, and glamour on a pathological input must not stall a turn.
const MaxInputBytes = 256 * 1024

// Mode selects when the final answer is styled.
type Mode string

const (
	// ModeAuto styles only on a terminal: stdout is a TTY and neither
	// NO_COLOR nor TERM=dumb opts out. Piped output stays raw markdown.
	ModeAuto Mode = "auto"

	// ModeAlways styles even when piped. Explicit opt-out; scripts that
	// parse stdout should keep the default.
	ModeAlways Mode = "always"

	// ModeNever prints raw markdown, exactly as before rendering existed.
	ModeNever Mode = "never"
)

// ParseMode normalizes a config value. Empty and unknown values are auto,
// so an unset ui.markdown keeps today's default behavior.
func ParseMode(s string) Mode {
	switch Mode(strings.ToLower(strings.TrimSpace(s))) {
	case ModeAlways:
		return ModeAlways
	case ModeNever:
		return ModeNever
	default:
		return ModeAuto
	}
}

// headingBlue is the 256-color bright blue used for all heading levels:
// distinct from the HiWhite body text, readable on a dark terminal.
const headingBlue = "12"

// bodyGray is the dark-style body text color, reused as the inline-code
// foreground so `code` reads as gray background only, without the stock
// dark style's red foreground.
const bodyGray = "252"

// codeBackground keeps the stock dark inline-code background.
const codeBackground = "236"

func stringPtr(s string) *string { return &s }
func boolPtr(b bool) *bool       { return &b }
func uintPtr(u uint) *uint       { return &u }

// kvitStyle starts from the stock dark style and applies the program's
// palette:
//
//   - document margin 0: the stock margin 2 indents every paragraph by two
//     spaces and shrinks the usable width by four (Width = WordWrap -
//     Margin*2), so a 100-column terminal only gets 96 columns of prose
//     and every line carries a 2-space indent for no reason. Full-width
//     left-aligned prose wraps where the terminal does.
//   - headings: all six levels blue bold with no background. Stock h1 is
//     yellow on a purple full-width bar and h6 is green non-bold, so
//     titles render inconsistently; h2-h5 already inherit blue bold from
//     heading but are pinned explicitly so the contract does not depend
//     on inheritance.
//   - inline code: gray background only. Stock is red foreground (203) on
//     gray (236) with extra padding spaces around the span, which renders
//     as double spaces ("no  ttyline  package") once combined with the
//     source spaces. Clearing the affixes leaves single source spaces and
//     the background hugging the text.
func kvitStyle() ansi.StyleConfig {
	cfg := styles.DarkStyleConfig

	cfg.Document.Margin = uintPtr(0)

	cfg.Heading.Color = stringPtr(headingBlue)
	cfg.Heading.Bold = boolPtr(true)

	cfg.H1.Color = stringPtr(headingBlue)
	cfg.H1.BackgroundColor = nil
	cfg.H1.Bold = boolPtr(true)
	cfg.H1.Prefix = ""
	cfg.H1.Suffix = ""

	cfg.H2.Color = stringPtr(headingBlue)
	cfg.H2.Bold = boolPtr(true)
	cfg.H3.Color = stringPtr(headingBlue)
	cfg.H3.Bold = boolPtr(true)
	cfg.H4.Color = stringPtr(headingBlue)
	cfg.H4.Bold = boolPtr(true)
	cfg.H5.Color = stringPtr(headingBlue)
	cfg.H5.Bold = boolPtr(true)
	cfg.H6.Color = stringPtr(headingBlue)
	cfg.H6.Bold = boolPtr(true)

	cfg.Code.Color = stringPtr(bodyGray)
	cfg.Code.BackgroundColor = stringPtr(codeBackground)
	cfg.Code.Prefix = ""
	cfg.Code.Suffix = ""

	return cfg
}

// ansiSeq matches the SGR sequences glamour emits (colors, bold, reset).
var ansiSeq = regexp.MustCompile("\x1b\\[[0-9;]*m")

const ansiReset = "\x1b[0m"

// stripANSI removes SGR sequences for width checks.
func stripANSI(s string) string {
	return ansiSeq.ReplaceAllString(s, "")
}

// trimRightANSI removes glamour's end-of-line padding: the padding writer
// pads every line to the full wrap width with styled spaces, so short
// lines ("stay", "ownership", "fight.") carry dozens of trailing spaces
// and lonely wrapped words look broken. Only visible trailing spaces are
// removed; leading indent (lists, quotes) and the text's own closing
// reset are kept. A line with no visible text becomes empty.
func trimRightANSI(line string) string {
	if line == "" {
		return ""
	}
	if strings.TrimRight(stripANSI(line), " \t") == stripANSI(line) {
		// No visible trailing whitespace: nothing padded to remove.
		// (Fast path; also avoids appending anything to clean lines.)
		return line
	}

	// Tokenize into ANSI sequences and single runes so trailing styled
	// spaces ("<color> <reset>" repeats) can be dropped without touching
	// the text's own styling.
	type token struct {
		raw   string
		isANSI bool
		r     rune
	}
	var tokens []token
	for len(line) > 0 {
		if loc := ansiSeq.FindStringIndex(line); loc != nil && loc[0] == 0 {
			tokens = append(tokens, token{raw: line[:loc[1]], isANSI: true})
			line = line[loc[1]:]
			continue
		}
		// Decode one full rune (padding is ASCII, but prose is not).
		r, size := utf8.DecodeRuneInString(line)
		if size <= 0 {
			size = 1
			r = rune(line[0])
		}
		tokens = append(tokens, token{raw: line[:size], r: r})
		line = line[size:]
	}

	lastText := -1
	for i, t := range tokens {
		if !t.isANSI && t.r != ' ' && t.r != '\t' {
			lastText = i
		}
	}
	if lastText == -1 {
		return ""
	}
	// Keep the text plus any reset sequences immediately closing it
	// ("fight." + reset), and drop the padding colors and spaces after.
	end := lastText + 1
	for end < len(tokens) && tokens[end].isANSI && tokens[end].raw == ansiReset {
		end++
	}
	var b strings.Builder
	for _, t := range tokens[:end] {
		b.WriteString(t.raw)
	}
	return b.String()
}

// trimPadding strips end-of-line padding from every line while preserving
// the document's leading and trailing blank lines.
func trimPadding(out string) string {
	lines := strings.Split(out, "\n")
	for i, ln := range lines {
		lines[i] = trimRightANSI(ln)
	}
	return strings.Join(lines, "\n")
}

// Render styles src as terminal markdown wrapped at width, using the
// program's dark-derived style (blue bold headings, gray-background inline
// code, full-width prose). It returns src unchanged on empty input,
// oversized input, or any renderer error.
func Render(src string, width int) string {
	if src == "" || len(src) > MaxInputBytes {
		return src
	}
	if width < 40 {
		width = 40
	}
	if width > 250 {
		width = 250
	}
	r, err := glamour.NewTermRenderer(
		glamour.WithStyles(kvitStyle()),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil || strings.TrimSpace(stripANSI(out)) == "" {
		return src
	}
	return trimPadding(out)
}

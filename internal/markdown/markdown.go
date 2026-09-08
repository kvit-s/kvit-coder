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
	"strings"

	"github.com/charmbracelet/glamour"
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

// Render styles src as terminal markdown wrapped at width, using the dark
// style to match the surrounding step palette (white headings and body,
// gray secondary text). It returns src unchanged on empty input,
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
		glamour.WithStandardStyle("dark"),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return src
	}
	out, err := r.Render(src)
	if err != nil || strings.TrimSpace(out) == "" {
		return src
	}
	return out
}

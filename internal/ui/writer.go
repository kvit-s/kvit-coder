package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"

	"github.com/kvit-s/kvit-coder/internal/markdown"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// Color definitions for consistent UI
var (
	// Brownish, dim status for the per-step context share (e.g. "(8.4k 1%)").
	// Faint yellow reads as brown without shouting over the message.
	brownColor = color.New(color.FgYellow, color.Faint)

	// Gray color for tool calls and other secondary progress lines.
	grayColor = color.New(color.FgWhite, color.Faint)

	// Dim red for errors: visible, but not a bright alarm on every failure.
	errorColor = color.New(color.FgRed, color.Faint)

	// Dim brown for warnings, same family as the status line.
	warnColor = color.New(color.FgYellow, color.Faint)

	// Full white for assistant messages and step text.
	whiteColor = color.New(color.FgHiWhite)

	// Bright green for steering prompts: the pause prompt is the moment
	// typing becomes available, so it stands out from gray progress lines.
	greenColor = color.New(color.FgHiGreen)

	// Colors for plan rendering
	planCompletedColor = color.New(color.FgWhite, color.Faint, color.CrossedOut)
	planActiveColor    = color.New(color.FgYellow, color.Faint)
)

// Step rendering follows the codex pattern: one bullet-led line per model
// turn, then indented detail lines beneath it.
//
//	● (8.4k 1%) Analyzing textarea wrapping behavior
//	  Read[path="internal/ui/input.go"]
//
// The bullet starts the step, the brownish status shows the context share,
// the message is full white, and tool calls are gray. Continuation lines of
// a multi-line message or tool call stay indented so wrapped text never
// jumps back to column zero. Warnings and errors use the same indent with
// dim brown/red coloring, and are always collapsed to a single line.
const (
	stepBullet      = "●"
	stepIndent      = "  "
	maxWarnChars    = 200
	maxErrorChars   = 300
	maxToolArgChars = 160
)

// JSONOutput represents the structured output for --json mode
type JSONOutput struct {
	Content string     `json:"content"`         // The final LLM response
	Stats   *JSONStats `json:"stats,omitempty"` // Statistics (tokens, cost, etc.)
}

// JSONStats represents statistics in JSON output
type JSONStats struct {
	Session          string  `json:"session,omitempty"`
	PromptTokens     int     `json:"prompt_tokens"`
	CompletionTokens int     `json:"completion_tokens"`
	TotalTokens      int     `json:"total_tokens"`
	CacheReadTokens  int     `json:"cache_read_tokens,omitempty"`
	TotalCost        float64 `json:"total_cost_usd,omitempty"`
	CacheDiscount    float64 `json:"cache_discount_usd,omitempty"`
	// The turn's peak context, and the window it was measured against, so a
	// consumer of the document can tell a small turn from a near-full one.
	ContextUsed   int   `json:"context_used"`
	ContextWindow int   `json:"context_window"`
	DurationMs    int64 `json:"duration_ms"`
	Steps         int   `json:"steps"`
}

// Writer provides formatted output with consistent prefixes and optional colors.
type Writer struct {
	verboseLines int // 0 = not verbose, >0 = verbose with max lines to show
	quiet        bool
	jsonMode     bool      // Output structured JSON instead of formatted text
	headless     bool      // Route progress to stderr, final answer to stdout
	stderr       io.Writer // stderr output (defaults to os.Stderr)
	stdout       io.Writer // stdout output (defaults to os.Stdout)
	// stderrTTY overrides the check for whether stderr can be redrawn in
	// place. Tests set it; nil means work it out from the file itself.
	stderrTTY *bool
	// stdoutTTY overrides the check for whether the final answer goes to
	// a terminal. Tests set it; nil means work it out from the file
	// itself. It only matters for markdown rendering (see
	// shouldRenderMarkdown); the answer itself always goes to stdout.
	stdoutTTY *bool
	// markdownMode selects when the final answer is styled as markdown
	// (see the markdown package). The zero value is auto.
	markdownMode markdown.Mode
	// awaitingAnswer, when set, reports whether a question is on screen
	// waiting for someone to type an answer. No progress is drawn while one
	// is.
	awaitingAnswer func() bool
}

// SetPromptWatcher installs a function reporting whether a question is waiting
// for an answer. Progress indicators redraw the line they sit on, so one drawn
// while a question is on screen erases it; with this set they hold off until
// the question has been answered.
func (w *Writer) SetPromptWatcher(fn func() bool) {
	w.awaitingAnswer = fn
}

// SetStderrIsTerminal overrides the detection of whether stderr can be redrawn
// in place, so a test can exercise both paths without a pseudo-terminal.
func (w *Writer) SetStderrIsTerminal(isTTY bool) {
	w.stderrTTY = &isTTY
}

// SetStdoutIsTerminal overrides the detection of whether the final answer
// goes to a terminal, so a test can exercise the styled and raw markdown
// paths without a pseudo-terminal.
func (w *Writer) SetStdoutIsTerminal(isTTY bool) {
	w.stdoutTTY = &isTTY
}

// SetMarkdownMode selects when the final answer is styled as markdown:
// "auto" (the default), "always", or "never". Empty and unknown values
// are auto.
func (w *Writer) SetMarkdownMode(mode string) {
	w.markdownMode = markdown.ParseMode(mode)
}

// NewWriter creates a new Writer with the specified verbosity level.
// verboseLines: 0 = not verbose, >0 = show tool output up to this many lines (half top, half bottom if truncated)
func NewWriter(verboseLines int) *Writer {
	return &Writer{
		verboseLines: verboseLines,
		quiet:        false,
		jsonMode:     false,
		headless:     false,
		stderr:       os.Stderr,
		stdout:       os.Stdout,
	}
}

// IsVerbose returns true if verbose mode is enabled.
func (w *Writer) IsVerbose() bool {
	return w.verboseLines > 0
}

// VerboseLines returns the max lines to show in verbose mode.
func (w *Writer) VerboseLines() int {
	return w.verboseLines
}

// SetVerbose sets the verbose lines level.
func (w *Writer) SetVerbose(lines int) {
	w.verboseLines = lines
}

// SetQuiet enables or disables quiet mode (suppresses all output except Assistant messages).
func (w *Writer) SetQuiet(quiet bool) {
	w.quiet = quiet
}

// SetJSONMode enables or disables JSON output mode.
func (w *Writer) SetJSONMode(jsonMode bool) {
	w.jsonMode = jsonMode
}

// IsJSONMode returns true if JSON mode is enabled.
func (w *Writer) IsJSONMode() bool {
	return w.jsonMode
}

// SetHeadless enables headless mode where progress goes to stderr and final answer to stdout.
func (w *Writer) SetHeadless(headless bool) {
	w.headless = headless
}

// IsHeadless returns true if headless mode is enabled.
func (w *Writer) IsHeadless() bool {
	return w.headless
}

// SetStdout sets a custom stdout writer.
func (w *Writer) SetStdout(stdout io.Writer) {
	w.stdout = stdout
}

// SetStderr sets a custom stderr writer.
func (w *Writer) SetStderr(stderr io.Writer) {
	w.stderr = stderr
}

// SetColorOutput sets the output for colored text (used by fatih/color package).
// This should be called when you want colored output to go somewhere other than os.Stdout.
func (w *Writer) SetColorOutput(stdout io.Writer) {
	color.Output = stdout
}

// jsonContent accumulates the final content for JSON output
var jsonContent string

// SetJSONContent sets the content to be output in JSON mode
func (w *Writer) SetJSONContent(content string) {
	jsonContent = content
}

// WriteJSONOutput outputs the final JSON result to stdout
func (w *Writer) WriteJSONOutput(stats *JSONStats) {
	if !w.jsonMode {
		return
	}
	output := JSONOutput{
		Content: jsonContent,
		Stats:   stats,
	}
	data, _ := json.MarshalIndent(output, "", "  ")
	fmt.Fprintln(w.stdout, string(data))
	jsonContent = "" // Reset for next use
}

// Manual ANSI codes mirror the fatih/color palette above, but they are
// emitted explicitly so a headless run whose stderr is a terminal still gets
// colors even when stdout (the final-answer stream) is piped and the color
// package has switched itself off.
const (
	ansiReset  = "\x1b[0m"
	ansiBrown  = "\x1b[33;2m"
	ansiGray   = "\x1b[37;2m"
	ansiWhite  = "\x1b[97m"
	ansiDimRed = "\x1b[31;2m"
	// Dim bright-blue for the background-process count (★N) in the step
	// header. Same dim family as the brown/gray/dim-red around it, but the
	// bright-blue base stays readable on dark terminals where plain dim
	// blue (34;2m) nearly disappears.
	ansiBlue = "\x1b[94;2m"
	// Dim gray plus strikethrough for finished plan steps.
	ansiStrikeGray = "\x1b[37;2;9m"
	// Bright green for steering prompts, mirroring FgHiGreen above.
	ansiGreen = "\x1b[92m"
)

// useColor reports whether the progress stream should carry ANSI colors:
// headless output goes to stderr, so it depends on stderr being a terminal;
// interactive output goes through the color package, which already knows.
func (w *Writer) useColor() bool {
	if w.quiet || w.jsonMode {
		return false
	}
	if w.headless {
		if w.stderrTTY != nil {
			return *w.stderrTTY
		}
		if f, ok := w.stderr.(*os.File); ok {
			if info, err := f.Stat(); err == nil {
				return info.Mode()&os.ModeCharDevice != 0
			}
		}
		return false
	}
	return !color.NoColor
}

// out returns the stream progress lines belong on.
func (w *Writer) out() io.Writer {
	if w.headless {
		return w.stderr
	}
	return color.Output
}

// paint wraps s in an ANSI code when colors are on.
func (w *Writer) paint(code, s string) string {
	if s == "" || !w.useColor() {
		return s
	}
	return code + s + ansiReset
}

// paintThinkingLine colors one pre-wrapped step line: the bullet white,
// the "(8.4k 1%)" status brownish-dim, the ephemeral background-process
// count ("★2") dim blue, the persistent count ("★1!") dim red, and the
// message itself full white.
// Continuation lines carry only the indent plus white message text.
func (w *Writer) paintThinkingLine(plain, context string) string {
	if !w.useColor() {
		return plain
	}
	if strings.HasPrefix(plain, stepBullet+" ") {
		rest := strings.TrimPrefix(plain, stepBullet+" ")
		bullet := ansiWhite + stepBullet + ansiReset
		if context != "" {
			status := "(" + context + ")"
			if strings.HasPrefix(rest, status) {
				msg := strings.TrimPrefix(rest, status)
				if base, blue, red, ok := splitProcStars(context); ok {
					var painted string
					inner := base
					if blue != "" {
						if inner != "" {
							inner += " "
						}
						inner += ansiReset + ansiBlue + blue + ansiReset + ansiBrown
					}
					if red != "" {
						if inner != "" && (base != "" || blue != "") {
							inner += " "
						}
						inner += ansiReset + ansiDimRed + red + ansiReset + ansiBrown
					}
					painted = ansiBrown + "(" + inner + ")" + ansiReset
					return bullet + " " + painted + w.paint(ansiWhite, msg)
				}
				return bullet + " " + ansiBrown + status + ansiReset + w.paint(ansiWhite, msg)
			}
		}
		return bullet + w.paint(ansiWhite, " "+rest)
	}
	if strings.HasPrefix(plain, stepIndent) {
		return stepIndent + w.paint(ansiWhite, strings.TrimPrefix(plain, stepIndent))
	}
	return w.paint(ansiWhite, plain)
}

// splitProcStars splits a step status like "8.4k 1% ★2 ★1!" into its brown
// base ("8.4k 1%"), its blue ephemeral star ("★2"), and its red persistent
// star ("★1!"). Either star may be absent; it reports false when neither is
// present, so plain headers paint exactly as before. The "!" marks a star
// that survives the turn, so plain (colorless) output still tells them apart.
func splitProcStars(context string) (base, blue, red string, ok bool) {
	fields := strings.Fields(context)
	i := len(fields)
	var blues, reds []string
	for i > 0 && isProcStarToken(fields[i-1]) {
		tok := fields[i-1]
		if strings.HasSuffix(tok, "!") {
			reds = append([]string{tok}, reds...)
		} else {
			blues = append([]string{tok}, blues...)
		}
		i--
	}
	if len(blues) == 0 && len(reds) == 0 {
		return "", "", "", false
	}
	// The formatter emits at most one of each; if several arrive, keep the
	// last of each kind so the header stays short.
	if len(blues) > 0 {
		blue = blues[len(blues)-1]
	}
	if len(reds) > 0 {
		red = reds[len(reds)-1]
	}
	base = strings.Join(fields[:i], " ")
	return base, blue, red, true
}

// isProcStarToken reports whether a status token is a ★N or ★N! count.
func isProcStarToken(tok string) bool {
	if !strings.HasPrefix(tok, "★") {
		return false
	}
	digits := strings.TrimPrefix(tok, "★")
	digits = strings.TrimSuffix(digits, "!")
	if digits == "" {
		return false
	}
	for _, r := range digits {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// splitProcStar keeps the old single-star shape: the blue star when present,
// else the red one. New code prefers splitProcStars.
func splitProcStar(context string) (base, star string, ok bool) {
	base, blue, red, ok := splitProcStars(context)
	if !ok {
		return "", "", false
	}
	if blue != "" {
		// Rejoin the red tail (if any) onto the base so the old two-value
		// shape still round-trips "base ★blue ★red!" losslessly.
		if red != "" {
			base = strings.TrimSpace(base + " " + blue)
			return base, red, true
		}
		return base, blue, true
	}
	return base, red, true
}

// clearProgressLine erases the in-place progress row (if any) so the next
// header, tool, warning or error line starts clean rather than overwriting
// dots. Piped output never draws in place, so there is nothing to erase.
func (w *Writer) clearProgressLine() {
	if progressLine == "" {
		progressDotCount = 0
		return
	}
	if _, draw := w.progressTarget(); draw {
		if w.headless {
			fmt.Fprint(w.stderr, "\r\033[K")
		} else {
			fmt.Fprint(color.Output, "\r\033[K")
		}
	}
	progressLine = ""
	progressDotCount = 0
}

// termWidth is the display width step lines wrap at. COLUMNS wins when set;
// otherwise 100 keeps long reasoning readable without assuming a wide window.
func termWidth() int {
	if s := os.Getenv("COLUMNS"); s != "" {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil {
			if n < 40 {
				return 40
			}
			if n > 250 {
				return 250
			}
			return n
		}
	}
	return 100
}

func visibleLen(s string) int {
	return len([]rune(s))
}

// wrapLine word-wraps one logical line (no newlines) to width, keeping the
// first-line prefix and indenting wrapped continuations with contPrefix.
// A single word longer than the width is split mid-word so a long path or
// URL cannot push one display line off the side of the terminal.
func wrapLine(prefix, text string, width int, contPrefix string) []string {
	if visibleLen(prefix+text) <= width {
		return []string{prefix + text}
	}
	words := strings.Fields(text)
	if len(words) == 0 {
		return []string{prefix + text}
	}
	var lines []string
	cur := prefix
	curIsPrefix := true
	flush := func() {
		lines = append(lines, cur)
		cur = contPrefix
		curIsPrefix = true
	}
	for _, wd := range words {
		// Split an overlong word into width-sized chunks first.
		for visibleLen(wd) > width-visibleLen(contPrefix) {
			if !curIsPrefix || visibleLen(cur) > visibleLen(prefix) {
				flush()
			}
			room := width - visibleLen(cur)
			if room <= 0 {
				flush()
				room = width - visibleLen(cur)
			}
			runes := []rune(wd)
			cur += string(runes[:room])
			wd = string(runes[room:])
			flush()
		}
		sep := ""
		if !curIsPrefix || visibleLen(cur) > visibleLen(prefix) {
			sep = " "
		}
		// When the prefix itself already fills the line (a long
		// "● (status)" on a narrow terminal), start the text below it
		// rather than emitting an overlong first line.
		if curIsPrefix && visibleLen(prefix) >= width {
			flush()
			sep = ""
		}
		if visibleLen(cur+sep+wd) <= width {
			cur += sep + wd
			curIsPrefix = false
			continue
		}
		flush()
		cur += wd
		curIsPrefix = false
	}
	lines = append(lines, cur)
	return lines
}

// buildThinkingLines renders the bullet-led step header plus any message
// continuation lines, all pre-wrapped and indented. An empty message still
// yields the "● (ctx)" status line so every step shows its context share.
// The context may carry the background-process count ("8.4k 1% ★2"); it is
// plain text here, and paintThinkingLine gives the star its blue.
func buildThinkingLines(context, msg string) []string {
	width := termWidth()
	bulletPrefix := stepBullet + " "
	contPrefix := stepIndent
	status := ""
	if context != "" {
		status = "(" + context + ")"
	}
	trimmed := strings.Trim(msg, "\n")
	if strings.TrimSpace(trimmed) == "" {
		if status != "" {
			return wrapLine(bulletPrefix, status, width, contPrefix)
		}
		return []string{stepBullet}
	}
	logical := strings.Split(trimmed, "\n")
	firstText := logical[0]
	if status != "" {
		if firstText != "" {
			firstText = status + " " + firstText
		} else {
			firstText = status
		}
	}
	out := wrapLine(bulletPrefix, firstText, width, contPrefix)
	for _, ln := range logical[1:] {
		if strings.TrimSpace(ln) == "" {
			out = append(out, "")
			continue
		}
		out = append(out, wrapLine(contPrefix, ln, width, contPrefix)...)
	}
	return out
}

// buildToolLines renders one gray tool-call line plus indented continuations.
// argsDisplay is collapsed to a single short line first, so a multiline shell
// command cannot break the step layout.
func buildToolLines(name, argsDisplay string) []string {
	width := termWidth()
	basePrefix := stepIndent
	contPrefix := stepIndent + stepIndent
	single := SingleLine(argsDisplay, maxToolArgChars)
	display := name
	if single != "" {
		display = name + "[" + single + "]"
	}
	return wrapLine(basePrefix, display, width, contPrefix)
}

// StartupInfo prints startup information in brown.
func (w *Writer) StartupInfo(msg string) {
	if w.quiet {
		return
	}
	if w.headless {
		fmt.Fprintln(w.stderr, msg)
	} else {
		brownColor.Println(msg)
	}
}

// Info prints an info message in gray, indented like other step lines.
// Multi-line messages keep every continuation line indented.
func (w *Writer) Info(msg string) {
	if w.quiet {
		return
	}
	w.clearProgressLine()
	width := termWidth()
	var plains []string
	for _, ln := range strings.Split(strings.Trim(msg, "\n"), "\n") {
		if strings.TrimSpace(ln) == "" {
			plains = append(plains, "")
			continue
		}
		plains = append(plains, wrapLine(stepIndent, ln, width, stepIndent)...)
	}
	for _, plain := range plains {
		line := plain
		if plain != "" {
			line = stepIndent + w.paint(ansiGray, strings.TrimPrefix(plain, stepIndent))
			// When colors are off paint returns plain unchanged; keep the
			// pre-wrapped plain line in that case.
			if !w.useColor() {
				line = plain
			}
		}
		fmt.Fprintln(w.out(), line)
	}
}

// Steering prints a steering prompt in bright green, indented like Info.
// It marks the moment typing steering becomes available, so it stands out
// from the gray progress lines around it. Layout and wrapping match Info.
func (w *Writer) Steering(msg string) {
	if w.quiet {
		return
	}
	w.clearProgressLine()
	width := termWidth()
	var plains []string
	for _, ln := range strings.Split(strings.Trim(msg, "\n"), "\n") {
		if strings.TrimSpace(ln) == "" {
			plains = append(plains, "")
			continue
		}
		plains = append(plains, wrapLine(stepIndent, ln, width, stepIndent)...)
	}
	for _, plain := range plains {
		line := plain
		if plain != "" {
			line = stepIndent + w.paint(ansiGreen, strings.TrimPrefix(plain, stepIndent))
			// When colors are off paint returns plain unchanged; keep the
			// pre-wrapped plain line in that case.
			if !w.useColor() {
				line = plain
			}
		}
		fmt.Fprintln(w.out(), line)
	}
}

// Warn prints a warning as one indented line in dim brown.
// The message is collapsed to a single line and truncated, so a warning
// never breaks the step layout.
func (w *Writer) Warn(msg string) {
	if w.quiet {
		return
	}
	w.clearProgressLine()
	single := SingleLine(msg, maxWarnChars)
	plain := stepIndent + "[warn] " + single
	fmt.Fprintln(w.out(), w.paint(ansiBrown, plain))
}

// Error prints an error as one indented line in dim red, same layout as Warn.
func (w *Writer) Error(msg string) {
	if w.quiet {
		return
	}
	w.clearProgressLine()
	single := SingleLine(msg, maxErrorChars)
	plain := stepIndent + "[error] " + single
	fmt.Fprintln(w.out(), w.paint(ansiDimRed, plain))
}

// Tool prints a tool execution message with [tool:name] prefix (unused, kept for compatibility).
func (w *Writer) Tool(name, msg string) {
	if w.quiet {
		return
	}
	if w.headless {
		fmt.Fprintf(w.stderr, "[tool:%s] %s\n", name, msg)
	} else {
		grayColor.Printf("[tool:%s] %s\n", name, msg)
	}
}

// Assistant prints an assistant message in white.
// In headless mode, this goes to stdout (the final answer).
// In JSON mode, it stores the content to be output later with WriteJSONOutput.
//
// The message is markdown, and on a terminal it is styled as such (headings,
// lists, code, tables) instead of printed verbatim. Piped output, JSON mode,
// and NO_COLOR/TERM=dumb stay raw markdown, so scripts and logs see exactly
// what the model wrote. Rendering never fails the turn: on any error the raw
// message goes through.
func (w *Writer) Assistant(msg string) {
	if w.jsonMode {
		// Store content for later JSON output
		jsonContent = msg
		return
	}

	display := msg
	if w.shouldRenderMarkdown(msg) {
		if styled := markdown.Render(msg, termWidth()); styled != msg {
			display = strings.TrimRight(styled, "\n")
		}
	}

	if w.headless {
		// Plain text final answer to stdout
		fmt.Fprintf(w.stdout, "%s\n", display)
	} else if display != msg {
		// Already styled; the white wrapper would only reset it.
		fmt.Fprintf(color.Output, "%s\n\n", display)
	} else {
		whiteColor.Printf("%s\n\n", msg)
	}
}

// shouldRenderMarkdown reports whether the final answer should be styled.
// JSON mode never styles (the raw markdown rides in the JSON document).
// Otherwise the ui.markdown mode decides: never means raw, always means
// styled subject to the standard opt-outs, and auto (the default) means
// styled only when the answer goes to a terminal.
func (w *Writer) shouldRenderMarkdown(msg string) bool {
	if msg == "" || w.jsonMode {
		return false
	}
	if w.markdownMode == markdown.ModeNever {
		return false
	}
	if os.Getenv("NO_COLOR") != "" || os.Getenv("TERM") == "dumb" {
		return false
	}
	if w.markdownMode == markdown.ModeAlways {
		return true
	}
	return w.answerTTY()
}

// answerTTY reports whether the final answer stream is a terminal. Headless
// answers go to w.stdout; interactive ones go through the color package's
// output. A test override wins over either.
func (w *Writer) answerTTY() bool {
	if w.stdoutTTY != nil {
		return *w.stdoutTTY
	}
	out := w.stdout
	if !w.headless {
		out = color.Output
		if out == nil {
			out = os.Stdout
		}
	}
	if f, ok := out.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			return info.Mode()&os.ModeCharDevice != 0
		}
	}
	return false
}

// Divider separates the step progress from the final report. It carries the
// turn timing that used to print as a standalone "[39s: 39s llm + ...]" line
// after the report, so nothing needs to print after the report itself.
// A blank line leads so the divider never crowds the last step line.
// It goes to the progress stream (stderr when headless), keeping stdout clean
// for the final answer. Quiet and JSON modes print nothing.
func (w *Writer) Divider(statsMsg string) {
	if w.quiet || w.jsonMode {
		return
	}
	w.clearProgressLine()
	fmt.Fprintln(w.out())
	plain := "---------  " + statsMsg + " --------"
	fmt.Fprintln(w.out(), w.paint(ansiGray, plain))
}

// Debug prints a debug message in gray, only if verbose mode is enabled.
func (w *Writer) Debug(msg string) {
	if w.quiet || w.verboseLines <= 0 {
		return
	}
	if w.headless {
		fmt.Fprintf(w.stderr, "[debug] %s\n", msg)
	} else {
		grayColor.Printf("[debug] %s\n", msg)
	}
}

// Agent prints an agent/system message with [agent] prefix.
func (w *Writer) Agent(msg string) {
	if w.quiet {
		return
	}
	if w.headless {
		fmt.Fprintf(w.stderr, "[agent] %s\n", msg)
	} else {
		fmt.Printf("[agent] %s\n", msg)
	}
}

// Thinking prints one bullet-led step line per model turn: the bullet, the
// brownish context share with the blue background-process count ("★2") when
// any are running, and the message in full white. An empty message
// still prints "● (ctx)" so every step shows its status. Continuation lines
// of a multi-line message stay indented and wrapped.
func (w *Writer) Thinking(context, msg string) {
	if w.quiet {
		return
	}
	w.clearProgressLine()
	for _, plain := range buildThinkingLines(context, msg) {
		if plain == "" {
			fmt.Fprintln(w.out())
			continue
		}
		fmt.Fprintln(w.out(), w.paintThinkingLine(plain, context))
	}
}

// ToolCall prints a compact gray tool-call line indented under the step
// header. The context share lives on the header already, so the context
// argument is accepted for compatibility and ignored. Success summaries are
// not shown (see ToolResult); only the call itself appears here.
func (w *Writer) ToolCall(name, argsDisplay, context string) {
	if w.quiet {
		return
	}
	_ = context
	w.clearProgressLine()
	for _, plain := range buildToolLines(name, argsDisplay) {
		fmt.Fprintln(w.out(), w.paint(ansiGray, plain))
	}
}

// progressLine accumulates the current progress output
var progressLine string

// progressDotCount tracks marks since the last star (a minute of marks folds
// into one "*" so a long wait stays on a single redrawn row instead of
// stacking bullet-led lines that read as model thinking).
var progressDotCount int

// progressHeld records that an update was dropped because a question was
// waiting for an answer, so the row is started over once it has been answered.
var progressHeld bool

// maxDotsPerLine is how many dots (one per second) fold into one "*".
// A star is a minute, so a ten-minute tool reads as indent plus "**********"
// and the current minute's dots on one row rather than ten bullet-led lines.
const maxDotsPerLine = 60

// progressTarget says where a progress indicator should be drawn, and whether
// to draw it at all.
//
// Headless mode sends progress to stderr and the final answer to stdout, which
// is how the interactive UI runs the agent. It used to skip progress entirely,
// on the grounds that a headless run has no terminal to redraw — but when the
// UI spawns the agent, stderr is a terminal. The result was that a command
// taking two minutes printed nothing whatsoever between its tool line and its
// result, which is indistinguishable from a hang. So the question is not
// whether the run is headless but whether stderr can be redrawn.
func (w *Writer) progressTarget() (io.Writer, bool) {
	if !w.headless {
		return nil, true // nil means "the colour package's own stdout"
	}
	if w.stderrTTY != nil {
		return w.stderr, *w.stderrTTY
	}
	f, ok := w.stderr.(*os.File)
	if !ok {
		return w.stderr, false
	}
	info, err := f.Stat()
	// Piped or redirected: redrawing in place would put control characters in
	// a log file, so accumulate silently as before.
	return w.stderr, err == nil && info.Mode()&os.ModeCharDevice != 0
}

// drawProgress writes one progress update, either through the colour package
// (interactive) or to stderr (headless with a terminal).
func drawProgress(out io.Writer, text string) {
	if out == nil {
		grayColor.Print(text)
		return
	}
	fmt.Fprint(out, text)
}

// ToolProgress prints a progress indicator for a long wait: one mark per
// second, folded into "*" per finished minute (see maxDotsPerLine). Dots
// (".") are a tool running; colons (":") are the model thinking. Both waits
// share the indented row under the last tool-call line, so the mark is what
// tells them apart: a colon row under a Read line is the next model call,
// not the Read.
func (w *Writer) ToolProgress(dot string) {
	if w.quiet {
		return
	}
	// In JSON mode, skip progress dots (too noisy)
	if w.jsonMode {
		return
	}

	// A question waiting for an answer owns the terminal, as does the pause
	// prompt and the window between an empty Enter and that prompt, where
	// type-ahead echo is already on screen. Every update below
	// redraws the current line from its left edge, so a dot arriving while a
	// question is on screen paints over it one character at a time. That is
	// what made a path confirmation look like a hang: the row of dots kept
	// growing and "Allow this access? [y/N]:" was gone before it could be
	// read, so there was nothing on screen saying an answer was wanted.
	// The same redraw painted over steering typed after an empty Enter while
	// the current call finished, garbling the line into doubled text.
	if w.awaitingAnswer != nil && w.awaitingAnswer() {
		progressHeld = true
		return
	}
	// Answered. The cursor is on a fresh line below the question, so start the
	// row again rather than reprinting the dots collected before it. Waiting
	// for a person is not the tool being slow, which is why those seconds do
	// not count towards its timeout either.
	if progressHeld {
		progressHeld = false
		progressLine = ""
		progressDotCount = 0
	}

	out, draw := w.progressTarget()

	// If dot contains newline, it's the final output - print full line and reset
	if strings.Contains(dot, "\n") {
		if draw {
			drawProgress(out, "\r"+progressLine+dot)
		}
		progressLine = ""
		progressDotCount = 0
		return
	}

	// A wait starts indented: the row is marks and stars under the last
	// tool-call line, never bullet-led, so a tool wait cannot read as model
	// thinking. "●" is reserved for Thinking step headers. The "● "/"✨ "
	// arguments are legacy start markers from the caller (see callLLM);
	// both just reset. Dots are a tool running, colons are the model
	// thinking, so the mark tells a tool wait apart from the model wait
	// that follows it on the same row.
	if strings.HasPrefix(dot, "✨") || strings.HasPrefix(dot, stepBullet) {
		progressLine = ""
		progressDotCount = 0
		return
	}

	// A finished minute folds into one "*": the row stays a single redrawn
	// line ("  *..", then "  **..", and the same with colons for a model
	// wait) instead of stacking new lines.
	collapsed := false
	if dot == "." || dot == ":" {
		if progressLine == "" {
			// First mark of a new row: align under the last call line.
			progressLine = stepIndent
		}
		progressDotCount++
		if progressDotCount > maxDotsPerLine {
			progressLine = collapseProgressDots(progressLine)
			progressDotCount = 1 // this mark starts the next minute
			collapsed = true
		}
	}

	// Accumulate and reprint entire line
	progressLine += dot
	if !draw {
		return
	}

	if collapsed {
		// The row just shrank by ~59 cells, so clear to end of line or
		// the old dots linger past the star.
		drawProgress(out, "\r"+progressLine+"\033[K")
	} else {
		drawProgress(out, "\r"+progressLine)
	}
	drawProgress(out, "\n\033[1A") // Newline (flush) + move up
}

// collapseProgressDots folds the trailing minute of marks in s into one "*".
// The stepIndent alignment is preserved and any legacy "● "/"✨ " prefix is
// stripped, never kept: no wait row starts with a bullet.
func collapseProgressDots(s string) string {
	indent := ""
	body := s
	if strings.HasPrefix(body, stepIndent) {
		indent = stepIndent
		body = strings.TrimPrefix(body, stepIndent)
	}
	body = strings.TrimPrefix(body, stepBullet+" ")
	body = strings.TrimPrefix(body, "✨ ")
	if len(body) >= maxDotsPerLine {
		body = body[:len(body)-maxDotsPerLine] + "*"
	} else {
		body += "*"
	}
	return indent + body
}

// ToolResult finishes a tool line. A slow tool keeps its "...2s" duration in
// gray; the per-result size summary ("34 lines, 5.2k chars") is intentionally
// not shown, so step output stays one header plus its calls. Error and
// pending summaries still print as indented single lines.
func (w *Writer) ToolResult(summary, duration string) {
	if w.quiet {
		return
	}
	w.clearProgressLine()
	if duration != "" {
		fmt.Fprintln(w.out(), w.paint(ansiGray, stepIndent+SingleLine(duration, 32)))
	}
	single := SingleLine(summary, maxErrorChars)
	if single == "" {
		return
	}
	lower := strings.ToLower(single)
	switch {
	case strings.HasPrefix(lower, "pending"):
		fmt.Fprintln(w.out(), w.paint(ansiBrown, stepIndent+single))
	case strings.HasPrefix(lower, "error"),
		strings.HasPrefix(lower, "failed"),
		strings.HasPrefix(lower, "fatal"),
		strings.HasPrefix(lower, "blocked"),
		strings.HasPrefix(lower, "duplicate"),
		strings.HasPrefix(lower, "unknown"):
		fmt.Fprintln(w.out(), w.paint(ansiDimRed, stepIndent+single))
	default:
		// Success size summaries stay silent by design.
	}
}

// ToolContext is kept for compatibility. The context share already leads
// every step header, so printing it again per tool would repeat the status
// on each call line.
func (w *Writer) ToolContext(context string) {
	_ = context
}

// VerboseOutput prints tool output in verbose mode with truncation if needed.
// Returns true if output was printed.
func (w *Writer) VerboseOutput(output string) bool {
	if w.quiet || w.verboseLines <= 0 || output == "" {
		return false
	}

	w.clearProgressLine()
	// Use existing TruncateContent from tools package
	// maxLines = verboseLines, maxBytes = large (don't limit by bytes for verbose output)
	// truncatedLines = verboseLines (half on each side), truncatedBytes = large
	result := tools.TruncateContent([]byte(output), w.verboseLines, 1<<30, w.verboseLines, 1<<30)
	truncated := strings.TrimSuffix(result.Content, "\n")

	// Indent each line for visual grouping under the tool call
	lines := strings.Split(truncated, "\n")
	for _, line := range lines {
		fmt.Fprintln(w.out(), w.paint(ansiGray, stepIndent+stepIndent+line))
	}
	return true
}

// ActivePlan renders a plan from the <active_plan> format string.
func (w *Writer) ActivePlan(planText string) {
	if w.quiet || planText == "" {
		return
	}

	w.clearProgressLine()
	// Parse and render the plan text line by line
	lines := splitLines(planText)
	for _, line := range lines {
		// Skip XML tags
		if line == "<active_plan>" || line == "</active_plan>" {
			continue
		}

		plain := stepIndent + line
		// Completed steps stay struck through; the active step uses the
		// same dim brown as warnings so it never shouts.
		switch {
		case contains(line, "[✓]"):
			fmt.Fprintln(w.out(), w.paint(ansiStrikeGray, plain))
		case contains(line, "[→]"):
			fmt.Fprintln(w.out(), w.paint(ansiBrown, plain))
		default:
			fmt.Fprintln(w.out(), w.paint(ansiGray, plain))
		}
	}
}

// Helper functions
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && findSubstring(s, substr)
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

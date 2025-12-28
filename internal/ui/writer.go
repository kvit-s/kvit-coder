package ui

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// Color definitions for consistent UI
var (
	// Brown color for startup info
	brownColor = color.New(color.FgYellow, color.Faint)

	// Gray color for tool calls and thinking
	grayColor = color.New(color.FgWhite, color.Faint)

	// Red for errors
	errorColor = color.New(color.FgRed)

	// Yellow for warnings
	warnColor = color.New(color.FgYellow)

	// White for assistant responses
	whiteColor = color.New(color.FgWhite)

	// Colors for plan rendering
	planCompletedColor = color.New(color.FgWhite, color.Faint, color.CrossedOut)
	planActiveColor    = color.New(color.FgYellow)
)

// JSONOutput represents the structured output for --json mode
type JSONOutput struct {
	Content string     `json:"content"`          // The final LLM response
	Stats   *JSONStats `json:"stats,omitempty"`  // Statistics (tokens, cost, etc.)
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
	DurationMs       int64   `json:"duration_ms"`
	Steps            int     `json:"steps"`
}

// Writer provides formatted output with consistent prefixes and optional colors.
type Writer struct {
	verboseLines int  // 0 = not verbose, >0 = verbose with max lines to show
	quiet        bool
	jsonMode     bool     // Output structured JSON instead of formatted text
	headless     bool     // Route progress to stderr, final answer to stdout
	stderr       io.Writer // stderr output (defaults to os.Stderr)
	stdout       io.Writer // stdout output (defaults to os.Stdout)
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

// Info prints an info message with [info] prefix in gray.
func (w *Writer) Info(msg string) {
	if w.quiet {
		return
	}
	if w.headless {
		fmt.Fprintf(w.stderr, "[info] %s\n", msg)
	} else {
		grayColor.Printf("[info] %s\n", msg)
	}
}

// Warn prints a warning message with [warn] prefix in yellow.
func (w *Writer) Warn(msg string) {
	if w.quiet {
		return
	}
	if w.headless {
		fmt.Fprintf(w.stderr, "[warn] %s\n", msg)
	} else {
		warnColor.Printf("[warn] %s\n", msg)
	}
}

// Error prints an error message with [error] prefix in red.
func (w *Writer) Error(msg string) {
	if w.quiet {
		return
	}
	if w.headless {
		fmt.Fprintf(w.stderr, "[error] %s\n", msg)
	} else {
		errorColor.Printf("[error] %s\n", msg)
	}
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
func (w *Writer) Assistant(msg string) {
	if w.jsonMode {
		// Store content for later JSON output
		jsonContent = msg
		return
	}
	if w.headless {
		// Plain text final answer to stdout
		fmt.Fprintf(w.stdout, "%s\n", msg)
	} else {
		whiteColor.Printf("%s\n\n", msg)
	}
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

// Thinking prints reasoning/thinking text in gray.
func (w *Writer) Thinking(context, msg string) {
	if w.quiet {
		return
	}
	var output string
	if context != "" {
		output = fmt.Sprintf("*(%s) %s", context, msg)
	} else {
		output = fmt.Sprintf("* %s", msg)
	}
	if w.headless {
		fmt.Fprintln(w.stderr, output)
	} else {
		grayColor.Println(output)
	}
}

// ToolCall prints a compact tool call representation in gray.
func (w *Writer) ToolCall(name, argsDisplay, context string) {
	if w.quiet {
		return
	}
	// Reset progress line since a new tool is starting
	progressLine = ""
	var output string
	if context != "" {
		output = fmt.Sprintf("  (%s) %s[%s]", context, name, argsDisplay)
	} else {
		output = fmt.Sprintf("  %s[%s]", name, argsDisplay)
	}
	if w.headless {
		fmt.Fprintln(w.stderr, output)
	} else {
		grayColor.Println(output)
	}
}

// progressLine accumulates the current progress output
var progressLine string

// ToolProgress prints a progress indicator (dots) for long-running tools.
func (w *Writer) ToolProgress(dot string) {
	if w.quiet {
		return
	}
	// In JSON mode, skip progress dots (too noisy)
	if w.jsonMode {
		return
	}
	// In headless mode, skip animated progress (no terminal control)
	if w.headless {
		// Only print the final timing line
		if strings.Contains(dot, "\n") {
			progressLine = ""
		} else if strings.HasPrefix(dot, "✨") {
			progressLine = ""
		} else {
			progressLine += dot
		}
		return
	}
	// If dot contains newline, it's the final output - print full line and reset
	if strings.Contains(dot, "\n") {
		fmt.Print("\r")                       // Return to start of line
		grayColor.Print(progressLine + dot)   // Print accumulated + final
		progressLine = ""
		return
	}

	// Reset if this is a new progress line (starts with sparkle)
	if strings.HasPrefix(dot, "✨") {
		progressLine = ""
	}

	// Accumulate and reprint entire line
	progressLine += dot
	fmt.Print("\r")                  // Return to start of line
	grayColor.Print(progressLine)    // Print accumulated content
	fmt.Print("\n\033[1A")           // Newline (flush) + move up
}

// ToolResult prints a tool result summary in gray.
func (w *Writer) ToolResult(summary, duration string) {
	if w.quiet {
		return
	}
	// Reset progress line since tool is done
	progressLine = ""
	if w.headless {
		if duration != "" {
			fmt.Fprintf(w.stderr, "  %s\n", duration)
		}
		fmt.Fprintf(w.stderr, "  → %s\n", summary)
	} else {
		if duration != "" {
			grayColor.Printf("\n  %s\n", duration)
		}
		grayColor.Printf("  → %s\n", summary)
	}
}

// ToolContext prints only the context (e.g., token usage) without tool details.
func (w *Writer) ToolContext(context string) {
	if w.quiet || context == "" {
		return
	}
	if w.headless {
		fmt.Fprintf(w.stderr, "  (%s)\n", context)
	} else {
		grayColor.Printf("  (%s)\n", context)
	}
}

// VerboseOutput prints tool output in verbose mode with truncation if needed.
// Returns true if output was printed.
func (w *Writer) VerboseOutput(output string) bool {
	if w.quiet || w.verboseLines <= 0 || output == "" {
		return false
	}

	// Use existing TruncateContent from tools package
	// maxLines = verboseLines, maxBytes = large (don't limit by bytes for verbose output)
	// truncatedLines = verboseLines (half on each side), truncatedBytes = large
	result := tools.TruncateContent([]byte(output), w.verboseLines, 1<<30, w.verboseLines, 1<<30)
	truncated := strings.TrimSuffix(result.Content, "\n")

	// Indent each line for visual grouping under the tool call
	lines := strings.Split(truncated, "\n")
	if w.headless {
		for _, line := range lines {
			fmt.Fprintf(w.stderr, "    %s\n", line)
		}
	} else {
		for _, line := range lines {
			grayColor.Printf("    %s\n", line)
		}
	}
	return true
}

// ActivePlan renders a plan from the <active_plan> format string.
func (w *Writer) ActivePlan(planText string) {
	if w.quiet || planText == "" {
		return
	}

	// Parse and render the plan text line by line
	lines := splitLines(planText)
	for _, line := range lines {
		// Skip XML tags
		if line == "<active_plan>" || line == "</active_plan>" {
			continue
		}

		// In headless mode, just print plain text to stderr
		if w.headless {
			fmt.Fprintf(w.stderr, "  %s\n", line)
			continue
		}

		// Render lines with appropriate coloring
		if len(line) > 0 && line[0] >= '0' && line[0] <= '9' {
			// Step line - check for status symbols
			if contains(line, "[✓]") {
				planCompletedColor.Printf("  %s\n", line)
			} else if contains(line, "[→]") {
				planActiveColor.Printf("  %s\n", line)
			} else {
				grayColor.Printf("  %s\n", line)
			}
		} else {
			// Header lines (Task, Status, etc.)
			grayColor.Printf("  %s\n", line)
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

package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// MakePrompt creates a colored prompt with white text on gray background
func MakePrompt(text string) string {
	// ANSI codes for white on gray background
	colorStart := "\033[97;100m"
	colorEnd := "\033[0m"
	return colorStart + text + colorEnd
}

// FormatToolArgs formats tool arguments for compact display.
// Keys are sorted so the terminal line is deterministic.
func FormatToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}

	keys := make([]string, 0, len(args))
	for key := range args {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	var parts []string
	for _, key := range keys {
		val := args[key]
		var valStr string
		switch v := val.(type) {
		case string:
			// Truncate long strings
			if len(v) > 50 {
				valStr = fmt.Sprintf("%q", v[:47]+"...")
			} else {
				valStr = fmt.Sprintf("%q", v)
			}
		case float64, int, bool:
			valStr = fmt.Sprintf("%v", v)
		default:
			// For complex types, use JSON
			jsonBytes, _ := json.Marshal(v)
			valStr = string(jsonBytes)
		}
		parts = append(parts, fmt.Sprintf("%s=%s", key, valStr))
	}
	return strings.Join(parts, ", ")
}

// FormatShellDisplay renders shell command and working directory relative to workspace.
func FormatShellDisplay(cmd, workingDir, workspaceRoot string) string {
	display := cmd

	root := workspaceRoot
	if root == "" {
		if cwd, err := os.Getwd(); err == nil {
			root = cwd
		}
	}
	root = filepath.Clean(root)

	resolvedDir := root
	if workingDir != "" {
		wd := workingDir
		if strings.HasPrefix(wd, "~/") {
			if home, err := os.UserHomeDir(); err == nil {
				wd = filepath.Join(home, wd[2:])
			}
		}
		if filepath.IsAbs(wd) {
			resolvedDir = wd
		} else {
			resolvedDir = filepath.Join(root, wd)
		}
		resolvedDir = filepath.Clean(resolvedDir)
	}

	if rel, err := filepath.Rel(root, resolvedDir); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
		return fmt.Sprintf("%s@%s", display, rel)
	}

	if resolvedDir != "" && resolvedDir != root {
		return fmt.Sprintf("%s@%s", display, resolvedDir)
	}

	return display
}

// FormatDuration formats a duration in a human-readable way, omitting zero values
func FormatDuration(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	var parts []string
	if hours > 0 {
		parts = append(parts, fmt.Sprintf("%dh", hours))
	}
	if minutes > 0 {
		parts = append(parts, fmt.Sprintf("%dm", minutes))
	}
	if seconds > 0 || len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("%ds", seconds))
	}
	return strings.Join(parts, " ")
}

// FormatChars formats character count in a human-readable way (e.g., "1.5k")
func FormatChars(chars int) string {
	if chars < 1000 {
		return fmt.Sprintf("%d", chars)
	}
	k := float64(chars) / 1000.0
	if k < 10 {
		return fmt.Sprintf("%.1fk", k)
	}
	return fmt.Sprintf("%.0fk", k)
}

// GetResultSummary extracts meaningful size info from tool result
func GetResultSummary(result any) string {
	// Try to extract content or meaningful metrics
	if resultMap, ok := result.(map[string]any); ok {
		// Check for pending_confirmation status (edit/write preview mode)
		if status, ok := resultMap["status"].(string); ok && status == "pending_confirmation" {
			if nextStep, ok := resultMap["next_step"].(string); ok {
				return fmt.Sprintf("PENDING → %s", nextStep)
			}
			return "PENDING → confirm or cancel"
		}

		// Check for success/failure status first (important for edit/undo tools)
		if success, ok := resultMap["success"].(bool); ok {
			if !success {
				// Show error type if available
				if errType, ok := resultMap["error"].(string); ok {
					return fmt.Sprintf("failed: %s", errType)
				}
				return "failed"
			}
			// For fs.edit success, show replacement count
			if replacements, ok := resultMap["replacements"].(int); ok {
				if replacements == 1 {
					return "1 replacement"
				}
				return fmt.Sprintf("%d replacements", replacements)
			}
		}

		// For fs.read with lines_read (highest priority for file reads)
		if linesRead, ok := resultMap["lines_read"].(int); ok {
			// Also get char count if content is available
			if content, ok := resultMap["content"].(string); ok {
				return fmt.Sprintf("%d lines, %s chars", linesRead, FormatChars(len(content)))
			}
			return fmt.Sprintf("%d lines", linesRead)
		}

		// For results with content field
		if content, ok := resultMap["content"].(string); ok {
			lineCount := strings.Count(content, "\n")
			charCount := len(content)
			if lineCount > 0 {
				return fmt.Sprintf("%d lines, %s chars", lineCount, FormatChars(charCount))
			}
			// Single line or empty content
			if charCount > 0 {
				return fmt.Sprintf("1 line, %s chars", FormatChars(charCount))
			}
		}

		// For shell results with stdout field
		if stdout, ok := resultMap["stdout"].(string); ok {
			lineCount := strings.Count(stdout, "\n")
			charCount := len(stdout)
			if lineCount > 0 {
				return fmt.Sprintf("%d lines, %s chars", lineCount, FormatChars(charCount))
			}
			// Single line or empty output
			if charCount > 0 {
				return fmt.Sprintf("1 line, %s chars", FormatChars(charCount))
			}
			return "empty output"
		}

		// For results with count field (generic fallback)
		if count, ok := resultMap["count"].(int); ok {
			return fmt.Sprintf("%d items", count)
		}
	}

	// Fallback: count JSON lines
	jsonBytes, _ := json.MarshalIndent(result, "", "  ")
	lineCount := strings.Count(string(jsonBytes), "\n") + 1
	charCount := len(jsonBytes)
	return fmt.Sprintf("%d lines, %s chars", lineCount, FormatChars(charCount))
}

// ShortenBlockMessage creates a user-friendly short summary of a block message.
// The full message (with diff) is still sent to the LLM.
func ShortenBlockMessage(blockMsg string) string {
	// Extract file path from "pending edit on 'PATH'"
	// Format: "BLOCKED: Your X call was blocked due to a pending edit on 'PATH'."
	if idx := strings.Index(blockMsg, "pending edit on '"); idx != -1 {
		start := idx + len("pending edit on '")
		if end := strings.Index(blockMsg[start:], "'"); end != -1 {
			path := blockMsg[start : start+end]
			return fmt.Sprintf("BLOCKED: pending edit on '%s'", path)
		}
	}
	// Fallback: just return first line
	if idx := strings.Index(blockMsg, "\n"); idx != -1 {
		return blockMsg[:idx]
	}
	return blockMsg
}

// FormatContextStr formats context usage for display.
// It shows what is used and the share of the window in use, e.g. "8.4k 1%":
// the raw count on its own never said whether anything needed doing about it.
func FormatContextStr(totalTokens, contextLimit int) string {
	return FormatContextStrWithProcs(totalTokens, contextLimit, 0)
}

// FormatContextStrWithProcs adds the background-process count to the context
// status: "8.4k 1% ★2" when running > 0, the plain base otherwise. Only
// running processes count — finished ones already surface through the inbox —
// and zero keeps the header exactly as before.
func FormatContextStrWithProcs(totalTokens, contextLimit, running int) string {
	var base string
	if totalTokens <= 0 {
		if contextLimit > 0 {
			base = "0k 0%"
		} else {
			base = "0k"
		}
	} else {
		tokensK := float64(totalTokens) / 1000.0
		tokensStr := fmt.Sprintf("%.1fk", tokensK)
		if contextLimit > 0 {
			percent := 100 * float64(totalTokens) / float64(contextLimit)
			base = fmt.Sprintf("%s %.0f%%", tokensStr, percent)
		} else {
			base = tokensStr
		}
	}
	if running > 0 {
		return fmt.Sprintf("%s ★%d", base, running)
	}
	return base
}

// SingleLine collapses a message to one display line: newlines, carriage
// returns and tabs become spaces, runs of whitespace fold to one, and the
// result is trimmed and truncated to maxChars (with an ellipsis) when maxChars
// is positive.
func SingleLine(s string, maxChars int) string {
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	s = strings.Join(fields, " ")
	if maxChars > 0 && len(s) > maxChars {
		if maxChars <= 1 {
			return "…"
		}
		// Truncate on a byte boundary that keeps the string valid UTF-8.
		cut := maxChars - 1
		for cut > 0 && !isUTF8Boundary(s[cut]) {
			cut--
		}
		s = s[:cut] + "…"
	}
	return s
}

func isUTF8Boundary(b byte) bool {
	// Continuation bytes have the form 10xxxxxx.
	return b&0xC0 != 0x80
}

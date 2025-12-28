package ui

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

// FormatToolArgs formats tool arguments for compact display
func FormatToolArgs(args map[string]any) string {
	if len(args) == 0 {
		return ""
	}

	var parts []string
	for key, val := range args {
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

// FormatContextStr formats context usage for display
func FormatContextStr(totalTokens, contextLimit int) string {
	if totalTokens <= 0 {
		return "0k"
	}
	tokensK := float64(totalTokens) / 1000.0
	if contextLimit > 0 {
		contextK := float64(contextLimit) / 1000.0
		return fmt.Sprintf("%.1fk/%.0fk", tokensK, contextK)
	}
	return fmt.Sprintf("%.1fk", tokensK)
}

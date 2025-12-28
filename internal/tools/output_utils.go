package tools

import (
	"bytes"
	"fmt"
	"strings"
)

const (
	DefaultMaxLines       = 150        // No truncation threshold
	DefaultMaxBytes       = 24 * 1024  // 24KB
	DefaultTruncatedLines = 75         // Lines per side when truncated
	DefaultTruncatedBytes = 12 * 1024  // 12KB total when truncated
)

// TruncationResult contains the truncation outcome
type TruncationResult struct {
	Content      string // Full content if not truncated, formatted if truncated
	WasTruncated bool   // Whether content was truncated
	TotalLines   int    // Total lines in original
	TotalBytes   int    // Total bytes in original
}

// TruncateContent truncates content to show first and last portions if it exceeds limits
// If content is within limits, returns it as-is
// Otherwise, shows first N and last N lines/bytes with truncation marker
func TruncateContent(content []byte, maxLines, maxBytes, truncatedLines, truncatedBytes int) TruncationResult {
	lines := bytes.Split(content, []byte("\n"))
	lineCount := len(lines)

	// If output ends with newline, don't count the trailing empty string
	if lineCount > 0 && len(lines[lineCount-1]) == 0 {
		lineCount--
	}
	totalBytes := len(content)

	// Check if we need to truncate
	if lineCount <= maxLines && totalBytes <= maxBytes {
		// Small enough to return all of it
		return TruncationResult{
			Content:      string(content),
			WasTruncated: false,
			TotalLines:   lineCount,
			TotalBytes:   totalBytes,
		}
	}

	// Need to truncate - take first AND last portions
	firstPart, lastPart := truncateBothEnds(content, truncatedLines, truncatedBytes)

	// Calculate actual bytes/lines shown vs removed
	shownBytes := len(firstPart) + len(lastPart)
	removedBytes := totalBytes - shownBytes
	if removedBytes < 0 {
		removedBytes = 0
	}

	// Count lines in truncated parts
	shownLines := strings.Count(firstPart, "\n") + strings.Count(lastPart, "\n")
	removedLines := lineCount - shownLines
	if removedLines < 0 {
		removedLines = 0
	}

	// Build formatted output with truncation marker
	var sb strings.Builder
	sb.WriteString(firstPart)
	sb.WriteString("\n\n")
	sb.WriteString(fmt.Sprintf("... [truncated %d lines / %d bytes] ...\n",
		removedLines, removedBytes))
	sb.WriteString("\n")
	sb.WriteString(lastPart)

	return TruncationResult{
		Content:      sb.String(),
		WasTruncated: true,
		TotalLines:   lineCount,
		TotalBytes:   totalBytes,
	}
}

// truncateBothEnds truncates content to show first and last portions
// Returns (firstPart, lastPart) where each is up to maxLines/2 lines or maxBytes/2 bytes
func truncateBothEnds(content []byte, maxLines int, maxBytes int) (string, string) {
	lines := bytes.Split(content, []byte("\n"))

	// Calculate how many lines to take from each end
	linesPerSide := maxLines / 2
	bytesPerSide := maxBytes / 2

	// Extract first part
	firstLines := lines
	if len(lines) > linesPerSide {
		firstLines = lines[:linesPerSide]
	}
	firstPart := bytes.Join(firstLines, []byte("\n"))

	// Truncate first part by bytes if needed
	if len(firstPart) > bytesPerSide {
		firstPart = firstPart[:bytesPerSide]
		// Try to truncate at last newline to avoid cutting mid-line
		if lastNL := bytes.LastIndex(firstPart, []byte("\n")); lastNL > 0 {
			firstPart = firstPart[:lastNL]
		}
	}

	// Extract last part
	lastLines := lines
	if len(lines) > linesPerSide {
		lastLines = lines[len(lines)-linesPerSide:]
	}
	lastPart := bytes.Join(lastLines, []byte("\n"))

	// Truncate last part by bytes if needed
	if len(lastPart) > bytesPerSide {
		// Take the last bytesPerSide bytes
		startPos := len(lastPart) - bytesPerSide
		lastPart = lastPart[startPos:]
		// Try to truncate at first newline to avoid cutting mid-line
		if firstNL := bytes.Index(lastPart, []byte("\n")); firstNL >= 0 {
			lastPart = lastPart[firstNL+1:]
		}
	}

	return string(firstPart), string(lastPart)
}

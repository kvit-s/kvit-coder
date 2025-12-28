package tools

import (
	"bytes"
	"fmt"
	"os"
	"strings"
)

const (
	maxMemoryBytes = 20 * 1024 * 1024 // 20MB - threshold for spilling to temp file
	maxLLMBytes    = 12 * 1024        // 12KB - max bytes to send to LLM when truncated
	maxLLMLines    = 75               // max lines to send to LLM when truncated
)

// OutputBuffer manages shell command output with memory/file buffering and truncation
type OutputBuffer struct {
	buf         *bytes.Buffer
	tempFile    *os.File
	tempFileMgr *TempFileManager
	totalBytes  int
	spilledFile bool
}

// NewOutputBuffer creates a new output buffer
func NewOutputBuffer(tempFileMgr *TempFileManager) *OutputBuffer {
	return &OutputBuffer{
		buf:         &bytes.Buffer{},
		tempFileMgr: tempFileMgr,
	}
}

// Write implements io.Writer, buffering output in memory up to 20MB, then spilling to temp file
func (o *OutputBuffer) Write(p []byte) (n int, err error) {
	n = len(p)
	o.totalBytes += n

	// If already spilled to file, write directly to file
	if o.spilledFile {
		return o.tempFile.Write(p)
	}

	// Check if we need to spill to temp file
	if o.buf.Len()+n > maxMemoryBytes {
		// Create temp file and spill existing buffer
		if err := o.spillToTempFile(); err != nil {
			return 0, err
		}
		// Write new data to temp file
		return o.tempFile.Write(p)
	}

	// Still within memory limit, write to buffer
	return o.buf.Write(p)
}

// spillToTempFile creates a temp file and writes buffered content to it
func (o *OutputBuffer) spillToTempFile() error {
	var err error
	o.tempFile, err = o.tempFileMgr.CreateTempFile()
	if err != nil {
		return err
	}

	// Write buffered content to temp file
	if _, err := o.tempFile.Write(o.buf.Bytes()); err != nil {
		o.tempFile.Close()
		return fmt.Errorf("failed to write to temp file: %w", err)
	}

	o.spilledFile = true
	o.buf = nil // free memory
	return nil
}

// FormatForLLM returns the output formatted for LLM consumption
// - If output is small (< 150 lines and < 24KB), return all of it
// - Otherwise, save to temp file and return truncated output with file path
// Uses shared truncation utility for consistent behavior across all tools
func (o *OutputBuffer) FormatForLLM() (string, error) {
	var fullOutput []byte
	var outputPath string

	// Get the full output
	if o.spilledFile {
		// Output was already spilled to temp file during collection
		outputPath = o.tempFile.Name()

		// Close the file for writing so we can read it
		if err := o.tempFile.Close(); err != nil {
			return "", fmt.Errorf("failed to close temp file: %w", err)
		}

		// Read the entire file (we need to check size/lines)
		var err error
		fullOutput, err = os.ReadFile(outputPath)
		if err != nil {
			return "", fmt.Errorf("failed to read temp file: %w", err)
		}
	} else {
		// Output is still in memory buffer
		fullOutput = o.buf.Bytes()
	}

	// Use shared truncation utility to check if truncation is needed
	result := TruncateContent(fullOutput, DefaultMaxLines, DefaultMaxBytes, maxLLMLines, maxLLMBytes)

	if !result.WasTruncated {
		return result.Content, nil
	}

	// Output needs truncation - ensure we have a temp file for LLM to investigate
	if !o.spilledFile {
		// Create temp file and save full output
		var err error
		o.tempFile, err = o.tempFileMgr.CreateTempFile()
		if err != nil {
			return "", fmt.Errorf("failed to create temp file for truncated output: %w", err)
		}
		if _, err := o.tempFile.Write(fullOutput); err != nil {
			o.tempFile.Close()
			return "", fmt.Errorf("failed to write truncated output to temp file: %w", err)
		}
		if err := o.tempFile.Close(); err != nil {
			return "", fmt.Errorf("failed to close temp file: %w", err)
		}
		o.spilledFile = true
	}
	outputPath = o.tempFile.Name()

	// Build shell-specific header with temp file info
	var sb strings.Builder
	sb.WriteString("───────────────────────────────────────────────────────\n")
	sb.WriteString("⚠️  SHELL OUTPUT TRUNCATED\n")
	sb.WriteString(fmt.Sprintf("   Full output: %d lines, %d bytes\n", result.TotalLines, result.TotalBytes))
	sb.WriteString(fmt.Sprintf("   Complete output saved to: %s\n", outputPath))
	sb.WriteString("   Use read to investigate the full output\n")
	sb.WriteString("───────────────────────────────────────────────────────\n\n")
	sb.WriteString(result.Content) // Already contains head + marker + tail

	return sb.String(), nil
}

// Close cleans up resources (but doesn't delete temp file - that's session cleanup's job)
func (o *OutputBuffer) Close() error {
	if o.tempFile != nil {
		// Just close the file handle, don't delete it
		// The temp file manager will clean it up at session end
		return o.tempFile.Close()
	}
	return nil
}

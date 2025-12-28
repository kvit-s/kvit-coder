package tools

import (
	"strings"
)

// ApplyLineEdit applies a line-based edit to content.
// If endLine is 0, inserts newText at startLine (existing content shifts down).
// Otherwise replaces lines [startLine, endLine] (1-based, inclusive) with newText.
// Returns new content, edit start line, edit end line, and error.
func ApplyLineEdit(content string, startLine, endLine int, newText string) (string, int, int, error) {
	fileLines := strings.Split(content, "\n")
	totalLines := len(fileLines)

	// endLine == 0 means insert mode (no replacement)
	insertMode := endLine == 0

	// Validate start_line - for insert mode, allow startLine == totalLines+1 (append at end)
	maxStartLine := totalLines
	if insertMode {
		maxStartLine = totalLines + 1
	}
	if startLine < 1 || startLine > maxStartLine {
		return "", 0, 0, SemanticErrorf("start_line %d is invalid (file has %d lines)", startLine, totalLines)
	}

	// Validate end_line for replace mode
	if !insertMode && endLine > totalLines {
		return "", 0, 0, SemanticErrorf("end_line %d is beyond end of file (file has %d lines)", endLine, totalLines)
	}

	// Build new content
	var result strings.Builder

	// Write lines before start_line
	for i := 0; i < startLine-1; i++ {
		result.WriteString(fileLines[i])
		result.WriteString("\n")
	}

	// Write new text
	result.WriteString(newText)

	// Determine where to resume copying from
	resumeFrom := startLine - 1 // insert mode: keep original startLine
	if !insertMode {
		resumeFrom = endLine // replace mode: skip replaced lines
	}

	// Ensure newline before remaining content
	if resumeFrom < totalLines && !strings.HasSuffix(newText, "\n") {
		result.WriteString("\n")
	}

	// Write remaining lines
	for i := resumeFrom; i < totalLines; i++ {
		result.WriteString(fileLines[i])
		if i < totalLines-1 {
			result.WriteString("\n")
		}
	}
	newContent := result.String()

	// Calculate edit line range for the new content
	newTextLines := strings.Count(newText, "\n")
	if len(newText) > 0 && !strings.HasSuffix(newText, "\n") {
		newTextLines++ // Count the last line even without trailing newline
	}
	if newTextLines == 0 {
		newTextLines = 1
	}
	editEndLine := startLine + newTextLines - 1

	return newContent, startLine, editEndLine, nil
}

// CreateNewFileContent creates content for a new file.
// For new files, ignores line numbers and just uses the newText as content.
func CreateNewFileContent(newText string) string {
	return newText
}

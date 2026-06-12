package tools

import (
	"fmt"
	"strings"
)

// IndentAutocorrection describes a first-line indentation correction applied by
// ReconcileFirstLineIndent. It carries both the display fields surfaced to the model
// and the data needed to undo the correction (Edit.undo_autoindent).
type IndentAutocorrection struct {
	Line            int    // 1-based line that was corrected (the request start_line)
	FromSpaces      int    // original first-line indent width
	ToSpaces        int    // corrected first-line indent width
	Message         string // human-readable note for the model
	OriginalNewText string // the model's un-corrected new_text, for undo
	ReqStartLine    int    // original request start_line, for recomputing on undo
	ReqEndLine      int    // original request end_line, for recomputing on undo
}

// ResultField returns the structured note attached to the edit result so the model
// can review the correction and learn how to revert it.
func (c *IndentAutocorrection) ResultField() map[string]any {
	return map[string]any{
		"line":        c.Line,
		"from_spaces": c.FromSpaces,
		"to_spaces":   c.ToSpaces,
		"message":     c.Message,
	}
}

// leadingEditIndent returns the leading run of spaces/tabs of s.
func leadingEditIndent(s string) string {
	i := 0
	for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
		i++
	}
	return s[:i]
}

// ReconcileFirstLineIndent detects the case where the first line of newText is
// under-indented relative to the original file line at startLine, and re-indents only
// that first line to match. Continuation lines are never touched. Returns the
// (possibly adjusted) newText and a non-nil correction describing the change, or the
// original newText and nil when no correction applies.
//
// Scope: lines-mode replace only (endLine != 0). It fires only on an off-by-(≤maxFix)
// under-indent where the first line's indent is a proper prefix of the original
// indent, both are spaces-only, and the first line is non-blank. This is a no-op for
// inserts, deletions, blank first lines, tab-indented code, deeper drops, and any case
// where the model already indents correctly.
func ReconcileFirstLineIndent(content string, startLine, endLine int, newText string, maxFix int) (string, *IndentAutocorrection) {
	// Insert mode has no reference line; empty newText is a deletion.
	if endLine == 0 || newText == "" {
		return newText, nil
	}

	// Isolate the first line of newText.
	firstLine := newText
	rest := ""
	if idx := strings.IndexByte(newText, '\n'); idx >= 0 {
		firstLine = newText[:idx]
		rest = newText[idx:] // includes the leading "\n"
	}

	// Skip blank / deletion-style first lines.
	if strings.TrimSpace(firstLine) == "" {
		return newText, nil
	}

	// Reference: original file line at startLine (1-based).
	fileLines := strings.Split(content, "\n")
	if startLine < 1 || startLine > len(fileLines) {
		return newText, nil
	}
	origIndent := leadingEditIndent(fileLines[startLine-1])
	firstIndent := leadingEditIndent(firstLine)

	// Only correct a strict under-indent.
	if len(firstIndent) >= len(origIndent) {
		return newText, nil
	}

	// Spaces-only guard: never touch tab-indented code (1 tab = a full level, so a
	// legitimate one-level dedent would be deficit 1 and wrongly "corrected").
	if strings.ContainsRune(origIndent, '\t') || strings.ContainsRune(firstIndent, '\t') {
		return newText, nil
	}

	// Same whitespace style: firstIndent must be a proper prefix of origIndent.
	if !strings.HasPrefix(origIndent, firstIndent) {
		return newText, nil
	}

	// Dedent guard: only correct deficits within maxFix.
	deficit := len(origIndent) - len(firstIndent)
	if maxFix < 1 || deficit > maxFix {
		return newText, nil
	}

	// Apply: replace only the first line's leading whitespace with origIndent.
	correctedNewText := origIndent + firstLine[len(firstIndent):] + rest

	return correctedNewText, &IndentAutocorrection{
		Line:       startLine,
		FromSpaces: len(firstIndent),
		ToSpaces:   len(origIndent),
		Message: fmt.Sprintf(
			"First line indentation was auto-corrected from %d to %d spaces to match the line being replaced. "+
				"If this dedent was intentional, call Edit.undo_autoindent to revert it and get an updated diff.",
			len(firstIndent), len(origIndent)),
		OriginalNewText: newText,
		ReqStartLine:    startLine,
		ReqEndLine:      endLine,
	}
}

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

	// Ensure newline before remaining content (skip if deleting - empty newText)
	if resumeFrom < totalLines && len(newText) > 0 && !strings.HasSuffix(newText, "\n") {
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

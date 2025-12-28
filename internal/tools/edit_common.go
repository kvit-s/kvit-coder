package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// LargeFileThreshold is the file size above which we require line hints for editing
// Files larger than this won't be loaded entirely into memory
const LargeFileThreshold = 1024 * 1024 // 1MB

// StreamingEditBufferSize is the buffer size used for streaming file operations
const StreamingEditBufferSize = 64 * 1024 // 64KB

// EditPendingNextStep is the message shown when an edit is pending confirmation
const EditPendingNextStep = "STOP. You MUST call Edit.confirm or Edit.cancel next. ALL other tools are BLOCKED until you confirm or cancel this edit."

// EditTool is the interface that all edit tool implementations must satisfy
type EditTool interface {
	Tool
	// GetConfig returns the tool's config for shared functionality
	GetConfig() *config.Config
}

// BaseEditTool provides common functionality for all edit tool implementations
type BaseEditTool struct {
	Config        *config.Config
	WorkspaceRoot string
}

// GetConfig returns the tool's config
func (b *BaseEditTool) GetConfig() *config.Config {
	return b.Config
}

// ValidateAndResolvePath validates and resolves a path for editing
// Returns the full path, whether it's outside workspace, and any error
func (b *BaseEditTool) ValidateAndResolvePath(path string) (fullPath string, outside bool, err error) {
	// Check permissions
	permResult, permErr := b.Config.CheckPathPermission(path, config.AccessWrite)
	if permErr != nil && permResult == config.PermissionDenied {
		return "", false, fmt.Errorf("access denied: %w", permErr)
	}
	if permResult == config.PermissionReadOnly {
		return "", false, fmt.Errorf("path is read-only")
	}

	// Normalize and validate path
	fullPath, outside, err = NormalizeAndValidatePath(b.WorkspaceRoot, path)
	if err != nil {
		return "", false, fmt.Errorf("invalid path: %w", err)
	}

	// For outside-workspace paths, use CheckPathSafety which respects path_safety_mode
	if outside {
		if err := b.Config.CheckPathSafety("edit", path); err != nil {
			return "", outside, err
		}
	}

	return fullPath, outside, nil
}

// ReadFileForEdit reads a file for editing, handling new file creation
// Returns content, isNewFile, and error
func (b *BaseEditTool) ReadFileForEdit(fullPath string) (content string, isNewFile bool, err error) {
	data, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", true, nil
		}
		return "", false, fmt.Errorf("read file: %w", err)
	}
	return string(data), false, nil
}

// WriteFileAtomic writes content to a file atomically using temp file + rename
func (b *BaseEditTool) WriteFileAtomic(fullPath, content string, isNewFile bool) error {
	// For new files, ensure parent directory exists
	if isNewFile {
		parentDir := filepath.Dir(fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return fmt.Errorf("create parent directory: %w", err)
		}
	}

	// Write atomically - write to temp file first, then rename
	tempFile, err := os.CreateTemp(filepath.Dir(fullPath), ".edit-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath) // Clean up temp file in case of error

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Get file permissions from original file (or use default for new files)
	info, _ := os.Stat(fullPath)
	if info != nil {
		_ = os.Chmod(tempPath, info.Mode())
	} else {
		_ = os.Chmod(tempPath, 0644) // Default permissions for new files
	}

	// Atomic rename
	if err := os.Rename(tempPath, fullPath); err != nil {
		return fmt.Errorf("atomic rename failed: %w", err)
	}

	return nil
}

// CheckReadBeforeEdit validates that the file was read recently if configured
func (b *BaseEditTool) CheckReadBeforeEdit(path string) error {
	if b.Config.Tools.Edit.ReadBeforeEditMsgs <= 0 {
		return nil
	}

	// Skip check if there's a pending edit for this same file
	pendingPath := GetPendingEditPath()
	if pendingPath != "" && pendingPath == path {
		return nil
	}

	// Normalize path to check against tracked reads
	fullPath, _, err := NormalizeAndValidatePath(b.WorkspaceRoot, path)
	if err != nil {
		return SemanticErrorf("invalid path: %v", err)
	}

	// Skip check for new files
	if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
		return nil
	}

	if !globalReadTracker.WasReadRecently(fullPath, globalReadTracker.CurrentMessageID(), b.Config.Tools.Edit.ReadBeforeEditMsgs) {
		return SemanticErrorWithDetails(
			fmt.Sprintf("file not read recently: you must use read on '%s' before editing it (within last %d tool calls)", path, b.Config.Tools.Edit.ReadBeforeEditMsgs),
			map[string]any{
				"error":     "file_not_read",
				"path":      path,
				"next_step": fmt.Sprintf("read {\"path\": \"%s\"}", path),
			},
		)
	}

	return nil
}

// NormalizeWhitespace strips leading and trailing whitespace from each line
func NormalizeWhitespace(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	return strings.Join(lines, "\n")
}

// NormalizeWhitespaceRstrip strips only trailing whitespace from each line
func NormalizeWhitespaceRstrip(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}
	return strings.Join(lines, "\n")
}

// CountMatches counts how many times search appears in content
func CountMatches(content, search string) int {
	if search == "" {
		return len(content) + 1 // empty string matches at every position + end
	}
	count := 0
	pos := 0
	for {
		idx := strings.Index(content[pos:], search)
		if idx == -1 {
			break
		}
		count++
		pos += idx + len(search)
	}
	return count
}

// FindMatchPosition finds the position of search in content
// Returns start index, end index, and whether found
func FindMatchPosition(content, search string) (start, end int, found bool) {
	idx := strings.Index(content, search)
	if idx >= 0 {
		return idx, idx + len(search), true
	}
	return 0, 0, false
}

// GetLineNumber returns the 1-based line number for a byte offset in content
func GetLineNumber(content string, byteOffset int) int {
	lineNum := 1
	for i := 0; i < byteOffset && i < len(content); i++ {
		if content[i] == '\n' {
			lineNum++
		}
	}
	return lineNum
}

// GetContextAroundPosition returns a few lines of context around a byte position
func GetContextAroundPosition(content string, position int, contextLines int) string {
	lines := strings.Split(content, "\n")
	targetLine := GetLineNumber(content, position) - 1 // Convert to 0-based

	start := targetLine - contextLines
	if start < 0 {
		start = 0
	}
	end := targetLine + contextLines + 1
	if end > len(lines) {
		end = len(lines)
	}

	return strings.Join(lines[start:end], "\n")
}

// EditResult contains the result of an edit operation
type EditResult struct {
	Success bool   `json:"success"`
	Path    string `json:"path"`
	Diff    string `json:"diff,omitempty"`
	Created bool   `json:"created,omitempty"`
	Message string `json:"message,omitempty"`
}

// EditPreviewResult is returned when preview mode is enabled
type EditPreviewResult struct {
	Status    string `json:"status"` // "pending_confirmation"
	NextStep  string `json:"next_step"`
	Diff      string `json:"diff"`
	AfterEdit string `json:"after_edit,omitempty"` // Shows how file looks after edit
	Path      string `json:"path"`
	IsNewFile bool   `json:"is_new_file,omitempty"`
	Message   string `json:"message,omitempty"`
}

// BuildEditSuccessResult builds the standard success result for an edit operation.
// This is the single place where the edit success response format is defined.
func BuildEditSuccessResult(path, diff, newContent string, editStartLine, editEndLine int, isNewFile bool) map[string]any {
	afterEdit := GeneratePostEditContext(newContent, editStartLine, editEndLine)
	result := map[string]any{
		"success":    true,
		"path":       path,
		"diff":       diff,
		"after_edit": afterEdit,
	}
	if isNewFile {
		result["created"] = true
		result["message"] = "New file created"
	} else {
		result["message"] = "Edit applied successfully"
	}
	return result
}

// BuildEditPreviewResult builds the standard preview result for an edit operation.
// This is the single place where the edit preview response format is defined.
func BuildEditPreviewResult(path, diff, newContent string, editStartLine, editEndLine int, isNewFile bool) map[string]any {
	afterEdit := GeneratePostEditContext(newContent, editStartLine, editEndLine)
	result := map[string]any{
		"status":     "pending_confirmation",
		"next_step":  EditPendingNextStep,
		"path":       path,
		"diff":       diff,
		"after_edit": afterEdit,
	}
	if isNewFile {
		result["is_new_file"] = true
		result["message"] = "NEW FILE will be created"
	}
	return result
}

// StorePendingEdit stores a computed edit for preview mode
// editStartLine and editEndLine are 1-based line numbers in the new content
func StorePendingEdit(path, fullPath, oldContent, newContent, diff string, isNewFile bool, editStartLine, editEndLine int) {
	pendingEditMu.Lock()
	globalPendingEdit = &pendingEdit{
		path:          path,
		fullPath:      fullPath,
		oldContent:    oldContent,
		newContent:    newContent,
		diff:          diff,
		isNewFile:     isNewFile,
		editStartLine: editStartLine,
		editEndLine:   editEndLine,
	}
	pendingEditMu.Unlock()
}

// ClearPendingEditForPath clears any pending edit for a specific path
func ClearPendingEditForPath(path string) {
	pendingEditMu.Lock()
	if globalPendingEdit != nil && globalPendingEdit.path == path {
		globalPendingEdit = nil
	}
	pendingEditMu.Unlock()
}

// extractPathFromArgs extracts the path field from JSON arguments
func extractPathFromArgs(args json.RawMessage) (string, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return "", err
	}
	return params.Path, nil
}

// CommonEditCheck performs common validation for all edit tool modes
func CommonEditCheck(ctx context.Context, args json.RawMessage, base *BaseEditTool) error {
	path, err := extractPathFromArgs(args)
	if err != nil {
		return SemanticErrorf("invalid arguments: %v", err)
	}

	return base.CheckReadBeforeEdit(path)
}

// IsLargeFile checks if a file exceeds the large file threshold
func IsLargeFile(fullPath string) (bool, int64, error) {
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil // New files are not large
		}
		return false, 0, err
	}
	return info.Size() > LargeFileThreshold, info.Size(), nil
}

// ReadLineRange reads a specific range of lines from a file without loading the entire file
// Returns the content of lines [startLine, endLine] (1-based, inclusive) and total line count
func ReadLineRange(fullPath string, startLine, endLine int) (content string, totalLines int, err error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return "", 0, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var lines []string
	lineNum := 0

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", 0, err
		}

		if len(line) > 0 {
			lineNum++
			// Collect lines in range
			if lineNum >= startLine && lineNum <= endLine {
				lines = append(lines, strings.TrimSuffix(line, "\n"))
			}
		}

		if err == io.EOF {
			break
		}
	}

	return strings.Join(lines, "\n"), lineNum, nil
}

// StreamingLineReplace performs a streaming line-based replacement
// Replaces lines [startLine, endLine] with newText without loading entire file into memory
func (b *BaseEditTool) StreamingLineReplace(fullPath string, startLine, endLine int, newText string) error {
	// Create temp file for output
	tempFile, err := os.CreateTemp(filepath.Dir(fullPath), ".edit-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath) // Clean up on error

	// Open source file
	srcFile, err := os.Open(fullPath)
	if err != nil {
		tempFile.Close()
		return fmt.Errorf("open source file: %w", err)
	}
	defer srcFile.Close()

	reader := bufio.NewReaderSize(srcFile, StreamingEditBufferSize)
	writer := bufio.NewWriterSize(tempFile, StreamingEditBufferSize)
	lineNum := 0

	// Phase 1: Copy lines before the edit range
	for lineNum < startLine-1 {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			tempFile.Close()
			return fmt.Errorf("read line %d: %w", lineNum+1, err)
		}
		if len(line) > 0 {
			lineNum++
			if _, err := writer.WriteString(line); err != nil {
				tempFile.Close()
				return fmt.Errorf("write line %d: %w", lineNum, err)
			}
		}
		if err == io.EOF {
			break
		}
	}

	// Phase 2: Write the replacement text
	if _, err := writer.WriteString(newText); err != nil {
		tempFile.Close()
		return fmt.Errorf("write replacement: %w", err)
	}
	// Ensure newText ends with newline if there are more lines after
	if len(newText) > 0 && !strings.HasSuffix(newText, "\n") {
		// We'll check if there are more lines after
	}

	// Phase 3: Skip the lines being replaced
	for lineNum < endLine {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			tempFile.Close()
			return fmt.Errorf("skip line %d: %w", lineNum+1, err)
		}
		if len(line) > 0 {
			lineNum++
		}
		if err == io.EOF {
			break
		}
	}

	// Phase 4: Copy remaining lines
	// Check if we need to add a newline before continuing
	if len(newText) > 0 && !strings.HasSuffix(newText, "\n") {
		// Peek to see if there's more content
		peek, err := reader.Peek(1)
		if err == nil && len(peek) > 0 {
			_, _ = writer.WriteString("\n")
		}
	}

	if _, err := io.Copy(writer, reader); err != nil {
		tempFile.Close()
		return fmt.Errorf("copy remaining: %w", err)
	}

	// Flush and close
	if err := writer.Flush(); err != nil {
		tempFile.Close()
		return fmt.Errorf("flush writer: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	// Preserve original file permissions
	if info, err := os.Stat(fullPath); err == nil {
		_ = os.Chmod(tempPath, info.Mode())
	}

	// Atomic rename
	if err := os.Rename(tempPath, fullPath); err != nil {
		return fmt.Errorf("atomic rename: %w", err)
	}

	return nil
}

// StreamingSearchInRange searches for text within a specific line range
// Returns the matched content's start/end line numbers within the range
func StreamingSearchInRange(fullPath string, search string, startLine, endLine int) (matchStartLine, matchEndLine int, found bool, err error) {
	file, err := os.Open(fullPath)
	if err != nil {
		return 0, 0, false, err
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	lineNum := 0

	// Collect lines in range for searching
	var rangeContent strings.Builder
	var lineStarts []int // byte offset where each line starts in rangeContent

	for {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return 0, 0, false, err
		}

		if len(line) > 0 {
			lineNum++
			if lineNum >= startLine && lineNum <= endLine {
				lineStarts = append(lineStarts, rangeContent.Len())
				rangeContent.WriteString(line)
			}
			if lineNum > endLine {
				break
			}
		}

		if err == io.EOF {
			break
		}
	}

	content := rangeContent.String()

	// Search with normalization
	fuzzyThreshold := 0.8 // Default threshold
	matchStart, matchEnd, _, matchFound := MatchWithNormalization(content, search, fuzzyThreshold)
	if !matchFound {
		return 0, 0, false, nil
	}

	// Convert byte positions to line numbers
	matchStartLine = startLine
	matchEndLine = startLine
	for i, offset := range lineStarts {
		if offset <= matchStart {
			matchStartLine = startLine + i
		}
		if offset < matchEnd {
			matchEndLine = startLine + i
		}
	}

	return matchStartLine, matchEndLine, true, nil
}

// GenerateStreamingDiff generates a diff by reading only the affected line range
// This avoids loading the entire file for diff generation
func GenerateStreamingDiff(fullPath, path string, startLine, endLine int, oldLines, newText string) (string, error) {
	// For diff generation, we just need the old and new content of the affected region
	// The diff library handles the rest
	oldContent := oldLines
	if !strings.HasSuffix(oldContent, "\n") && oldContent != "" {
		oldContent += "\n"
	}
	newContent := newText
	if !strings.HasSuffix(newContent, "\n") && newContent != "" {
		newContent += "\n"
	}

	return generateUnifiedDiff(oldContent, newContent, path)
}

// FindMatchWithSearch uses the Search tool (ripgrep/grep) to find matches in a file
// Returns: line number of first match, total match count, error
// This is robust and handles large files, long lines, binary files, etc.
func FindMatchWithSearch(ctx context.Context, cfg *config.Config, fullPath string, search string) (matchLine int, matchCount int, err error) {
	// Create a search tool instance (nil tempFileMgr - we don't need temp files for internal matching)
	searchTool := NewSearchTool(cfg, nil)

	// Build search arguments - search for literal text in specific file
	// We use the first line of search text as the pattern (for multi-line searches)
	firstLine := search
	if idx := strings.Index(search, "\n"); idx > 0 {
		firstLine = search[:idx]
	}

	// Call the search tool
	args, _ := json.Marshal(map[string]any{
		"pattern":       firstLine,
		"path":          fullPath,
		"context_lines": 0, // We just need line numbers
	})

	result, err := searchTool.Call(ctx, args)
	if err != nil {
		return 0, 0, fmt.Errorf("search failed: %w", err)
	}

	// Parse the result
	resultMap, ok := result.(map[string]any)
	if !ok {
		return 0, 0, fmt.Errorf("unexpected search result type")
	}

	totalMatches, _ := resultMap["total_matches"].(int)
	if totalMatches == 0 {
		return 0, 0, nil
	}

	// Get the first match line number
	if matches, ok := resultMap["matches"].([]searchMatch); ok && len(matches) > 0 {
		return matches[0].Line, totalMatches, nil
	}

	// Try compact format
	if matches, ok := resultMap["matches"].([]map[string]any); ok && len(matches) > 0 {
		if line, ok := matches[0]["line"].(int); ok {
			return line, totalMatches, nil
		}
	}

	// Fallback - we know there are matches but couldn't parse line number
	return 1, totalMatches, nil
}

// PostEditContextLines is the number of surrounding lines to show in post-edit context
const PostEditContextLines = 3

// GeneratePostEditContext generates a formatted view of the file content after edit
// showing the edited region with surrounding context lines.
// editStartLine and editEndLine are 1-based line numbers in the new content.
func GeneratePostEditContext(newContent string, editStartLine, editEndLine int) string {
	if newContent == "" {
		return ""
	}
	lines := strings.Split(newContent, "\n")
	totalLines := len(lines)

	// Calculate context range (1-based)
	contextStart := editStartLine - PostEditContextLines
	if contextStart < 1 {
		contextStart = 1
	}
	contextEnd := editEndLine + PostEditContextLines
	if contextEnd > totalLines {
		contextEnd = totalLines
	}

	var sb strings.Builder

	// Calculate line number width based on total lines
	lineNumWidth := len(fmt.Sprintf("%d", totalLines))

	// Helper to format a line
	formatLine := func(lineNum int, isEdited bool) string {
		lineContent := ""
		if lineNum > 0 && lineNum <= len(lines) {
			lineContent = lines[lineNum-1]
		}
		marker := " "
		if isEdited {
			marker = ">"
		}
		return fmt.Sprintf("%s%*d│%s", marker, lineNumWidth, lineNum, lineContent)
	}

	// If context doesn't start at line 1, show line 1 first
	if contextStart > 1 {
		sb.WriteString(formatLine(1, false))
		sb.WriteString("\n")
		// If contextStart == 2, line 2 is in the context loop, no gap
		// If contextStart == 3, show line 2 as well (avoid "..." for 1-line gap)
		// If contextStart > 3, show "..." for multi-line gap
		if contextStart == 3 {
			sb.WriteString(formatLine(2, false))
			sb.WriteString("\n")
		} else if contextStart > 3 {
			sb.WriteString("...\n")
		}
	}

	// Show the context lines
	for i := contextStart; i <= contextEnd; i++ {
		isEdited := i >= editStartLine && i <= editEndLine
		sb.WriteString(formatLine(i, isEdited))
		sb.WriteString("\n")
	}

	// If context doesn't end at last line, show ellipsis and last line
	if contextEnd < totalLines {
		// If contextEnd == totalLines-1, last line follows directly, no gap
		// If contextEnd == totalLines-2, show second-to-last line (avoid "..." for 1-line gap)
		// If contextEnd < totalLines-2, show "..." for multi-line gap
		if contextEnd == totalLines-2 {
			sb.WriteString(formatLine(totalLines-1, false))
			sb.WriteString("\n")
		} else if contextEnd < totalLines-2 {
			sb.WriteString("...\n")
		}
		sb.WriteString(formatLine(totalLines, false))
		sb.WriteString("\n")
	}

	return strings.TrimSuffix(sb.String(), "\n")
}

// CalculateEditLineRange calculates the line range affected by an edit
// given the new content and the byte position where the replacement was inserted.
// Returns 1-based start and end line numbers in the new content.
func CalculateEditLineRange(newContent string, replaceStart int, replaceText string) (startLine, endLine int) {
	// Count lines before the edit start position
	startLine = 1
	for i := 0; i < replaceStart && i < len(newContent); i++ {
		if newContent[i] == '\n' {
			startLine++
		}
	}

	// Count lines in the replacement text
	replaceLines := strings.Count(replaceText, "\n")
	if len(replaceText) > 0 && !strings.HasSuffix(replaceText, "\n") {
		replaceLines++ // Count the last line even without trailing newline
	}
	if replaceLines == 0 {
		replaceLines = 1
	}

	endLine = startLine + replaceLines - 1
	return startLine, endLine
}

// FinalizeEdit handles preview mode or applies the edit with consistent result format.
// This is the single source of truth for the final edit handling logic.
func FinalizeEdit(b *BaseEditTool, path, fullPath, oldContent, newContent, diff string,
	editStartLine, editEndLine int, isNewFile bool) (any, error) {

	// Preview mode: store pending edit and return preview result
	if b.Config.Tools.Edit.PreviewMode {
		StorePendingEdit(path, fullPath, oldContent, newContent, diff, isNewFile, editStartLine, editEndLine)
		return BuildEditPreviewResult(path, diff, newContent, editStartLine, editEndLine, isNewFile), nil
	}

	// Apply the edit
	if err := b.WriteFileAtomic(fullPath, newContent, isNewFile); err != nil {
		return nil, err
	}

	return BuildEditSuccessResult(path, diff, newContent, editStartLine, editEndLine, isNewFile), nil
}

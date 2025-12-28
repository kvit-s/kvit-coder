package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// Search-replace mode metadata for UnifiedEditTool

// SearchReplaceEditDescription returns the description for search-replace edit mode
func SearchReplaceEditDescription() string {
	return "Edit a file by searching for exact text and replacing it. Content-based matching - no line numbers needed."
}

// SearchReplaceEditJSONSchema returns the JSON schema for search-replace edit mode
func SearchReplaceEditJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file (relative to workspace or absolute)",
			},
			"search": map[string]any{
				"type":        "string",
				"description": "Exact text to find in the file. Must match file content character-for-character including whitespace and indentation.",
			},
			"replace": map[string]any{
				"type":        "string",
				"description": "Text to replace the search match with.",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Optional: Starting line number hint for large files (>1MB). Limits search to lines [start_line, end_line].",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Optional: Ending line number hint for large files. Defaults to start_line + 100 if not provided.",
			},
		},
		"required": []string{"path", "search", "replace"},
	}
}

// SearchReplaceEditPromptSection returns the prompt section for search-replace edit mode
func SearchReplaceEditPromptSection(previewMode bool) string {
	base := `### Edit - Search and Replace in Files

**Usage:** ` + "`" + `Edit {"path": "<file>", "search": "<text to find>", "replace": "<replacement text>"}` + "`" + `

Examples:
- ` + "`" + `Edit {"path": "file.py", "search": "def old():\n    return 42", "replace": "def new():\n    return 43"}` + "`" + ` - replace a function
- ` + "`" + `Edit {"path": "file.py", "search": "    x = 1", "replace": "    x = 2"}` + "`" + ` - replace a line

**Parameters:**
- ` + "`path`" + ` (required): File path
- ` + "`search`" + ` (required): Exact text to find (must match character-for-character)
- ` + "`replace`" + ` (required): Replacement text
- ` + "`start_line`" + ` (optional): Line hint to speed up search in large files
- ` + "`end_line`" + ` (optional): End line hint. Defaults to start_line + 100

**Rules:**
1. The search text MUST exactly match file content (character-for-character including whitespace)
2. Include enough context lines to make the match unique
3. If multiple matches exist, add more surrounding context to disambiguate
4. Always use Read before editing a file`

	if previewMode {
		base += `
- Edit returns diff and after_edit preview with status="pending_confirmation"
- ` + "`Edit.confirm {}`" + ` to apply, ` + "`Edit.cancel {}`" + ` to retry`
	}
	return base
}

// SearchReplaceEditTool implements content-based search and replace editing
type SearchReplaceEditTool struct {
	BaseEditTool
}

// NewSearchReplaceEditTool creates a new SearchReplaceEditTool
func NewSearchReplaceEditTool(cfg *config.Config) *SearchReplaceEditTool {
	return &SearchReplaceEditTool{
		BaseEditTool: BaseEditTool{
			Config:        cfg,
			WorkspaceRoot: cfg.Workspace.Root,
		},
	}
}

func (t *SearchReplaceEditTool) Name() string {
	return "Edit"
}

func (t *SearchReplaceEditTool) Description() string {
	return "Edit a file by searching for exact text and replacing it. Content-based matching - no line numbers needed."
}

func (t *SearchReplaceEditTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file (relative to workspace or absolute)",
			},
			"search": map[string]any{
				"type":        "string",
				"description": "Exact text to find in the file. Must match file content character-for-character including whitespace and indentation. Use empty string to create a new file.",
			},
			"replace": map[string]any{
				"type":        "string",
				"description": "Text to replace the search block with. Use empty string to delete the matched content.",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "Optional: Starting line number hint for large files (>1MB). Limits search to lines [start_line, end_line]. Required for files >1MB to avoid loading entire file into memory.",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Optional: Ending line number hint for large files. Defaults to start_line + 100 if not provided.",
			},
		},
		"required": []string{"path", "search", "replace"},
	}
}

func (t *SearchReplaceEditTool) Check(ctx context.Context, args json.RawMessage) error {
	return CommonEditCheck(ctx, args, &t.BaseEditTool)
}

func (t *SearchReplaceEditTool) PromptCategory() string { return "filesystem" }
func (t *SearchReplaceEditTool) PromptOrder() int       { return 20 }
func (t *SearchReplaceEditTool) PromptSection() string {
	base := `### Edit - Search and Replace in Files

**Usage:** ` + "`" + `Edit {"path": "<file>", "search": "<text to find>", "replace": "<replacement text>"}` + "`" + `

Examples:
- ` + "`" + `Edit {"path": "file.py", "search": "def old():\n    return 42", "replace": "def new():\n    return 43"}` + "`" + ` - replace a function
- ` + "`" + `Edit {"path": "file.py", "search": "# TODO: fix this", "replace": ""}` + "`" + ` - delete content
- ` + "`" + `Edit {"path": "new.py", "search": "", "replace": "# New file content\n"}` + "`" + ` - create new file

**Parameters:**
- ` + "`path`" + ` (required): File path (creates parent directories if needed)
- ` + "`search`" + ` (required): Exact text to find (must match character-for-character). Use empty string for new files.
- ` + "`replace`" + ` (required): Replacement text. Use empty string to delete the matched content.
- ` + "`start_line`" + ` (optional): Line hint to speed up search in large files.
- ` + "`end_line`" + ` (optional): End line hint. Defaults to start_line + 100.

**Large Files (>1MB):** Handled automatically using streaming. Memory-efficient regardless of file size.

**Rules:**
1. The search text MUST exactly match file content (character-for-character including whitespace)
2. Include enough context lines to make the match unique
3. If multiple matches exist, add more surrounding context to disambiguate
4. Always use Read before editing a file`

	if t.Config.Tools.Edit.PreviewMode {
		base += `

**Preview Mode:** Edit returns a diff with status="pending_confirmation".
- ` + "`Edit.confirm {}`" + ` - Apply the edit
- ` + "`Edit.cancel {}`" + ` - Cancel and retry`
	}
	return base
}

func (t *SearchReplaceEditTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Path      string `json:"path"`
		Search    string `json:"search"`
		Replace   string `json:"replace"`
		StartLine *int   `json:"start_line"`
		EndLine   *int   `json:"end_line"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, SemanticErrorf("invalid arguments: %v", err)
	}

	// Validate and resolve path
	fullPath, _, err := t.ValidateAndResolvePath(params.Path)
	if err != nil {
		return nil, err
	}

	// Check if file is large
	isLarge, _, err := IsLargeFile(fullPath)
	if err != nil {
		return nil, err
	}

	// Clear any pending edit for this file (LLM is revising)
	ClearPendingEditForPath(params.Path)

	// If start_line provided, use streaming approach (works for any file size)
	if params.StartLine != nil {
		return t.callWithLineHint(ctx, fullPath, params.Path, params.Search, params.Replace, *params.StartLine, params.EndLine)
	}

	// For large files, automatically use streaming search + edit
	if isLarge {
		return t.callLargeFileAuto(ctx, fullPath, params.Path, params.Search, params.Replace)
	}

	// Standard approach for small files - load entire file
	content, isNewFile, err := t.ReadFileForEdit(fullPath)
	if err != nil {
		return nil, err
	}

	// Handle new file creation
	if isNewFile {
		if params.Search != "" {
			return map[string]any{
				"success": false,
				"error":   "new_file_with_search",
				"path":    params.Path,
				"message": "File does not exist. To create a new file, use empty search: {\"search\": \"\", \"replace\": \"content\"}",
			}, nil
		}
		return t.createNewFile(params.Path, fullPath, params.Replace)
	}

	// Handle empty search (not allowed for existing files)
	if params.Search == "" {
		return map[string]any{
			"success": false,
			"error":   "empty_search",
			"path":    params.Path,
			"message": "Empty search is only valid for creating new files. For existing files, specify the text to replace.",
		}, nil
	}

	// Check for identical search and replace (no-op edit)
	if params.Search == params.Replace {
		return nil, SemanticErrorf("search and replace text are identical - no change would be made")
	}

	// Find the search text with cascading normalization
	fuzzyThreshold := t.Config.Tools.Edit.FuzzyThreshold
	start, end, level, found := MatchWithNormalization(content, params.Search, fuzzyThreshold)

	if !found {
		return t.handleNoMatch(content, params.Search, params.Path)
	}

	// Check for multiple matches (only at exact level)
	if level == 0 {
		matchCount := CountMatches(content, params.Search)
		if matchCount > 1 {
			return t.handleMultipleMatches(content, params.Search, params.Path, matchCount)
		}
	}

	// Perform the replacement
	newContent := content[:start] + params.Replace + content[end:]

	// Generate diff
	diff, err := generateUnifiedDiff(content, newContent, params.Path)
	if err != nil {
		return nil, fmt.Errorf("generate diff: %w", err)
	}

	// Calculate edit line range for post-edit context
	editStartLine, editEndLine := CalculateEditLineRange(newContent, start, params.Replace)

	// Use shared finalize logic
	result, err := FinalizeEdit(&t.BaseEditTool, params.Path, fullPath, content, newContent, diff, editStartLine, editEndLine, false)
	if err != nil {
		return nil, err
	}

	// Add normalization note if applicable
	if resultMap, ok := result.(map[string]any); ok {
		switch level {
		case 1:
			resultMap["note"] = "Match found using whitespace normalization (trailing spaces ignored)"
		case 2:
			resultMap["note"] = "Match found using whitespace normalization (leading/trailing spaces ignored)"
		case 3:
			resultMap["note"] = "Match found using fuzzy matching (approximate content match)"
		}
	}

	return result, nil
}

// callLargeFileAuto handles large files by automatically streaming to find the search text
func (t *SearchReplaceEditTool) callLargeFileAuto(ctx context.Context, fullPath, path, search, replace string) (any, error) {
	// Handle empty search (not allowed for large files)
	if search == "" {
		return map[string]any{
			"success": false,
			"error":   "empty_search_large_file",
			"path":    path,
			"message": "Cannot create new file with empty search on large file path. File already exists.",
		}, nil
	}

	// Check for identical search and replace (no-op edit)
	if search == replace {
		return nil, SemanticErrorf("search and replace text are identical - no change would be made")
	}

	// Use Search tool (ripgrep/grep) to find the search text
	matchLine, matchCount, err := FindMatchWithSearch(ctx, t.Config, fullPath, search)
	if err != nil {
		return nil, fmt.Errorf("search: %w", err)
	}

	if matchCount == 0 {
		return map[string]any{
			"success": false,
			"error":   "no_match",
			"path":    path,
			"message": "Search text not found in file.",
			"hint":    "Use Read with start/limit to examine file content.",
		}, nil
	}

	if matchCount > 1 {
		return map[string]any{
			"success": false,
			"error":   "multiple_matches",
			"path":    path,
			"count":   matchCount,
			"message": fmt.Sprintf("Search text matches %d locations - add more surrounding context to make it unique", matchCount),
		}, nil
	}

	// Found exactly one match - use streaming edit at that location
	// Read a window around the match to perform the replacement
	searchLines := strings.Count(search, "\n") + 1
	windowStart := matchLine - 10 // Some context before
	if windowStart < 1 {
		windowStart = 1
	}
	windowEnd := matchLine + searchLines + 10 // Some context after

	// Read the window
	rangeContent, _, err := ReadLineRange(fullPath, windowStart, windowEnd)
	if err != nil {
		return nil, fmt.Errorf("read range: %w", err)
	}

	// Find exact position within the window (no fuzzy matching for large files)
	matchStart, matchEnd, found := FindMatchPosition(rangeContent, search)
	if !found {
		// This shouldn't happen since we just found it, but handle gracefully
		return nil, fmt.Errorf("match lost during window read - file may have changed")
	}

	// Calculate the actual line numbers of the match
	matchStartLine := windowStart
	matchEndLine := windowStart
	for i := 0; i < matchStart && i < len(rangeContent); i++ {
		if rangeContent[i] == '\n' {
			matchStartLine++
		}
	}
	for i := 0; i < matchEnd && i < len(rangeContent); i++ {
		if rangeContent[i] == '\n' {
			matchEndLine++
		}
	}

	// Build the new content for this range
	newRangeContent := rangeContent[:matchStart] + replace + rangeContent[matchEnd:]

	// Generate diff
	diff, err := GenerateStreamingDiff(fullPath, path, matchStartLine, matchEndLine, rangeContent, newRangeContent)
	if err != nil {
		return nil, fmt.Errorf("generate diff: %w", err)
	}

	// Preview mode
	if t.Config.Tools.Edit.PreviewMode {
		pendingEditMu.Lock()
		globalPendingEdit = &pendingEdit{
			path:       path,
			fullPath:   fullPath,
			oldContent: rangeContent,
			newContent: newRangeContent,
			diff:       diff,
			isNewFile:  false,
		}
		pendingEditMu.Unlock()

		return map[string]any{
			"status":           "pending_confirmation",
			"next_step":        "STOP. You MUST call Edit.confirm or Edit.cancel next. ALL other tools are BLOCKED until you confirm or cancel this edit.",
			"diff":             diff,
			"path":             path,
			"streaming_edit":   true,
			"match_start_line": matchStartLine,
			"match_end_line":   matchEndLine,
		}, nil
	}

	// Apply using streaming
	if err := t.StreamingLineReplace(fullPath, windowStart, windowEnd, newRangeContent); err != nil {
		return nil, fmt.Errorf("streaming replace: %w", err)
	}

	return map[string]any{
		"success":        true,
		"diff":           diff,
		"path":           path,
		"streaming_edit": true,
		"lines_affected": fmt.Sprintf("%d-%d", matchStartLine, matchEndLine),
	}, nil
}

// callWithLineHint handles editing with line hints using streaming (for large files)
func (t *SearchReplaceEditTool) callWithLineHint(ctx context.Context, fullPath, path, search, replace string, startLine int, endLine *int) (any, error) {
	// Default end_line to start_line + 100 if not provided
	actualEndLine := startLine + 100
	if endLine != nil {
		actualEndLine = *endLine
	}

	// Validate line range
	if startLine < 1 {
		return map[string]any{
			"success": false,
			"error":   "invalid_start_line",
			"message": "start_line must be >= 1",
		}, nil
	}
	if actualEndLine < startLine {
		return map[string]any{
			"success": false,
			"error":   "invalid_line_range",
			"message": fmt.Sprintf("end_line (%d) must be >= start_line (%d)", actualEndLine, startLine),
		}, nil
	}

	// Handle empty search (not allowed)
	if search == "" {
		return map[string]any{
			"success": false,
			"error":   "empty_search",
			"path":    path,
			"message": "Empty search is not allowed with start_line hint. Use without start_line to create new files.",
		}, nil
	}

	// Check for identical search and replace (no-op edit)
	if search == replace {
		return nil, SemanticErrorf("search and replace text are identical - no change would be made")
	}

	// Read only the specified line range
	rangeContent, totalLines, err := ReadLineRange(fullPath, startLine, actualEndLine)
	if err != nil {
		return nil, fmt.Errorf("read line range: %w", err)
	}

	// Check if start_line is beyond EOF
	if startLine > totalLines {
		return map[string]any{
			"success":     false,
			"error":       "start_line_beyond_eof",
			"path":        path,
			"start_line":  startLine,
			"total_lines": totalLines,
			"message":     fmt.Sprintf("start_line %d is beyond end of file (file has %d lines)", startLine, totalLines),
		}, nil
	}

	// Search within the range content (exact match only for large files)
	matchStart, matchEnd, found := FindMatchPosition(rangeContent, search)

	if !found {
		return map[string]any{
			"success":    false,
			"error":      "no_match_in_range",
			"path":       path,
			"start_line": startLine,
			"end_line":   actualEndLine,
			"message":    fmt.Sprintf("Search text not found in lines %d-%d. Try adjusting the line range or use Read to verify content.", startLine, actualEndLine),
			"hint":       fmt.Sprintf("Read {\"path\": \"%s\", \"start\": %d, \"limit\": %d}", path, startLine, actualEndLine-startLine+1),
		}, nil
	}

	// Check for multiple matches within range
	matchCount := CountMatches(rangeContent, search)
	if matchCount > 1 {
		return map[string]any{
			"success":    false,
			"error":      "multiple_matches_in_range",
			"path":       path,
			"count":      matchCount,
			"start_line": startLine,
			"end_line":   actualEndLine,
			"message":    fmt.Sprintf("Search text matches %d locations in lines %d-%d - add more context to make unique", matchCount, startLine, actualEndLine),
		}, nil
	}

	// Calculate the actual line numbers of the match within the file
	// Count newlines before matchStart to find match's start line
	matchStartLine := startLine
	matchEndLine := startLine
	for i := 0; i < matchStart && i < len(rangeContent); i++ {
		if rangeContent[i] == '\n' {
			matchStartLine++
		}
	}
	for i := 0; i < matchEnd && i < len(rangeContent); i++ {
		if rangeContent[i] == '\n' {
			matchEndLine++
		}
	}

	// Build the new content for this range
	newRangeContent := rangeContent[:matchStart] + replace + rangeContent[matchEnd:]

	// Generate diff (showing only the affected region)
	diff, err := GenerateStreamingDiff(fullPath, path, matchStartLine, matchEndLine, rangeContent, newRangeContent)
	if err != nil {
		return nil, fmt.Errorf("generate diff: %w", err)
	}

	// Preview mode: we can't easily store the full old/new content for large files
	// So we store just the range and note it's a streaming edit
	if t.Config.Tools.Edit.PreviewMode {
		// For preview mode with large files, we need to apply differently
		// Store enough info to apply the streaming edit
		pendingEditMu.Lock()
		globalPendingEdit = &pendingEdit{
			path:       path,
			fullPath:   fullPath,
			oldContent: rangeContent,   // Just the range
			newContent: newRangeContent, // Just the range
			diff:       diff,
			isNewFile:  false,
		}
		// Store line info for streaming apply (using a hack - encode in diff)
		pendingEditMu.Unlock()

		return map[string]any{
			"status":           "pending_confirmation",
			"next_step":        "STOP. You MUST call Edit.confirm or Edit.cancel next. ALL other tools are BLOCKED until you confirm or cancel this edit.",
			"diff":             diff,
			"path":             path,
			"streaming_edit":   true,
			"match_start_line": matchStartLine,
			"match_end_line":   matchEndLine,
		}, nil
	}

	// Apply the edit using streaming
	if err := t.StreamingLineReplace(fullPath, matchStartLine, matchEndLine, newRangeContent); err != nil {
		return nil, fmt.Errorf("streaming replace: %w", err)
	}

	return map[string]any{
		"success":        true,
		"diff":           diff,
		"path":           path,
		"streaming_edit": true,
		"lines_affected": fmt.Sprintf("%d-%d", matchStartLine, matchEndLine),
	}, nil
}

// createNewFile handles creating a new file
func (t *SearchReplaceEditTool) createNewFile(path, fullPath, content string) (any, error) {
	// Generate diff for new file
	diff, err := generateUnifiedDiff("", content, path)
	if err != nil {
		return nil, fmt.Errorf("generate diff: %w", err)
	}

	// For new files, the entire content is the edit (line 1 to last line)
	totalLines := strings.Count(content, "\n") + 1
	if content == "" {
		totalLines = 1
	}

	// Use shared finalize logic
	return FinalizeEdit(&t.BaseEditTool, path, fullPath, "", content, diff, 1, totalLines, true)
}

// handleNoMatch returns a helpful error when search text is not found
func (t *SearchReplaceEditTool) handleNoMatch(content, search, path string) (any, error) {
	// Try to find similar content to help the user
	lineNum, line, ratio := FindMostSimilarLine(content, search)

	result := map[string]any{
		"success": false,
		"error":   "no_match",
		"path":    path,
		"message": "Search text not found in file. The search text must exactly match the file content.",
		"hint":    "Use Read to see the exact file content, then copy the text you want to replace.",
	}

	// If we found a reasonably similar line, show it
	if ratio > 0.4 {
		result["similar_at_line"] = lineNum
		result["similar_text"] = strings.TrimSpace(line)
		result["similarity"] = fmt.Sprintf("%.0f%%", ratio*100)
	}

	return result, nil
}

// handleMultipleMatches returns an error with context for disambiguation
func (t *SearchReplaceEditTool) handleMultipleMatches(content, search, path string, count int) (any, error) {
	// Find positions of all matches
	var positions []int
	pos := 0
	for {
		idx := strings.Index(content[pos:], search)
		if idx == -1 {
			break
		}
		positions = append(positions, pos+idx)
		pos += idx + len(search)
	}

	// Get line numbers for each match
	lineNums := make([]int, len(positions))
	for i, pos := range positions {
		lineNums[i] = GetLineNumber(content, pos)
	}

	return map[string]any{
		"success":      false,
		"error":        "multiple_matches",
		"path":         path,
		"count":        count,
		"at_lines":     lineNums,
		"message":      fmt.Sprintf("Search text matches %d locations - add more surrounding context to make it unique", count),
		"hint":         "Include more lines before/after the text you want to change to create a unique match",
	}, nil
}

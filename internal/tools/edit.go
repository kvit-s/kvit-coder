package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// UnifiedEditTool implements all edit modes with a shared flow
type UnifiedEditTool struct {
	BaseEditTool
}

// NewUnifiedEditTool creates a new UnifiedEditTool
func NewUnifiedEditTool(cfg *config.Config) *UnifiedEditTool {
	return &UnifiedEditTool{
		BaseEditTool: BaseEditTool{
			Config:        cfg,
			WorkspaceRoot: cfg.Workspace.Root,
		},
	}
}

func (t *UnifiedEditTool) Name() string {
	return "Edit"
}

func (t *UnifiedEditTool) Description() string {
	switch t.Config.Tools.Edit.GetEditMode() {
	case "searchreplace":
		return SearchReplaceEditDescription()
	case "patch":
		return PatchEditDescription()
	default:
		return LineEditDescription()
	}
}

func (t *UnifiedEditTool) JSONSchema() map[string]any {
	switch t.Config.Tools.Edit.GetEditMode() {
	case "searchreplace":
		return SearchReplaceEditJSONSchema()
	case "patch":
		return PatchEditJSONSchema()
	default:
		return LineEditJSONSchema()
	}
}

func (t *UnifiedEditTool) Check(ctx context.Context, args json.RawMessage) error {
	var params struct {
		Path  string `json:"path"`
		Patch string `json:"patch"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return SemanticErrorf("invalid arguments: %v", err)
	}

	// Patch mode doesn't need read-before-edit check since it includes context
	if params.Patch != "" {
		return nil
	}

	// For other modes, use common check
	return CommonEditCheck(ctx, args, &t.BaseEditTool)
}

func (t *UnifiedEditTool) PromptCategory() string { return "filesystem" }
func (t *UnifiedEditTool) PromptOrder() int       { return 20 }
func (t *UnifiedEditTool) PromptSection() string {
	previewMode := t.Config.Tools.Edit.PreviewMode
	switch t.Config.Tools.Edit.GetEditMode() {
	case "searchreplace":
		return SearchReplaceEditPromptSection(previewMode)
	case "patch":
		return PatchEditPromptSection(previewMode)
	default:
		return LineEditPromptSection(previewMode)
	}
}

func (t *UnifiedEditTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		// Common
		Path string `json:"path"`
		// Search-replace mode
		Search  *string `json:"search"`
		Replace *string `json:"replace"`
		// Patch mode
		Patch string `json:"patch"`
		// Line mode
		StartLine *int   `json:"start_line"`
		EndLine   *int   `json:"end_line"`
		NewText   string `json:"new_text"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, SemanticErrorf("invalid arguments: %v", err)
	}

	// Detect mode based on parameters
	hasPatch := params.Patch != ""
	hasSearch := params.Search != nil
	hasLineRange := params.StartLine != nil || params.EndLine != nil

	// Validate mode
	modeCount := 0
	if hasPatch {
		modeCount++
	}
	if hasSearch {
		modeCount++
	}
	if hasLineRange {
		modeCount++
	}

	if modeCount == 0 {
		return nil, SemanticError("no edit mode specified - provide 'patch', 'search'+'replace', or 'start_line'+'new_text'")
	}
	if modeCount > 1 {
		return nil, SemanticError("multiple edit modes specified - use only one of: patch, search/replace, or line range")
	}

	// Dispatch to appropriate mode
	if hasPatch {
		return t.callPatchMode(ctx, params.Patch)
	}
	if hasSearch {
		if params.Replace == nil {
			return nil, SemanticError("'replace' is required when using 'search'")
		}
		return t.callSearchReplaceMode(ctx, params.Path, *params.Search, *params.Replace)
	}
	// Line mode
	if params.StartLine == nil {
		return nil, SemanticError("missing start_line")
	}
	// end_line is optional: if nil, use 0 to indicate insert mode
	endLine := 0
	if params.EndLine != nil {
		endLine = *params.EndLine
	}
	return t.callLineMode(ctx, params.Path, *params.StartLine, endLine, params.NewText)
}

// callPatchMode handles V4A patch editing
func (t *UnifiedEditTool) callPatchMode(ctx context.Context, patch string) (any, error) {
	// Parse the patch
	patches, err := ParsePatch(patch)
	if err != nil {
		return nil, SemanticErrorf("invalid patch format: %v", err)
	}

	if len(patches) == 0 {
		return nil, SemanticError("no file operations found in patch")
	}

	// For single-file patches, we can use the unified flow
	// For multi-file patches, we need to handle each file separately
	if len(patches) > 1 {
		// Multi-file patches - handle each file
		var results []map[string]any
		var allDiffs strings.Builder

		for _, fp := range patches {
			result, diff, err := t.applyFilePatch(ctx, fp)
			if err != nil {
				return map[string]any{
					"success":        false,
					"error":          "patch_failed",
					"failed_file":    fp.Path,
					"message":        err.Error(),
					"applied_so_far": results,
				}, nil
			}
			results = append(results, result)
			if diff != "" {
				allDiffs.WriteString(diff)
				allDiffs.WriteString("\n")
			}
		}

		return map[string]any{
			"success": true,
			"files":   len(patches),
			"results": results,
			"diff":    allDiffs.String(),
		}, nil
	}

	// Single-file patch - use unified flow
	fp := patches[0]
	result, _, err := t.applyFilePatch(ctx, fp)
	if err != nil {
		return nil, err
	}
	return result, nil
}

// applyFilePatch applies a single file patch
func (t *UnifiedEditTool) applyFilePatch(ctx context.Context, fp FilePatch) (map[string]any, string, error) {
	fullPath, _, err := t.ValidateAndResolvePath(fp.Path)
	if err != nil {
		return nil, "", err
	}

	switch fp.Action {
	case PatchDelete:
		return t.deleteFile(fp.Path, fullPath)
	case PatchAdd:
		return t.addFile(fp.Path, fullPath, fp.Chunks)
	case PatchUpdate:
		return t.updateFile(ctx, fp.Path, fullPath, fp.Chunks)
	default:
		return nil, "", fmt.Errorf("unknown patch action: %s", fp.Action)
	}
}

// deleteFile deletes a file
func (t *UnifiedEditTool) deleteFile(path, fullPath string) (map[string]any, string, error) {
	if _, err := os.Stat(fullPath); os.IsNotExist(err) {
		return nil, "", fmt.Errorf("file does not exist: %s", path)
	}

	if err := os.Remove(fullPath); err != nil {
		return nil, "", fmt.Errorf("delete failed: %w", err)
	}

	return map[string]any{
		"action":  "deleted",
		"path":    path,
		"success": true,
	}, "", nil
}

// addFile creates a new file from patch content
func (t *UnifiedEditTool) addFile(path, fullPath string, chunks []PatchChunk) (map[string]any, string, error) {
	// Check if file already exists
	if _, err := os.Stat(fullPath); err == nil {
		return nil, "", fmt.Errorf("file already exists: %s (use Update File instead)", path)
	}

	// Build content from additions in chunks
	var content strings.Builder
	for _, chunk := range chunks {
		for _, line := range chunk.Additions {
			content.WriteString(line)
			content.WriteString("\n")
		}
	}

	newContent := content.String()

	// Generate diff
	diff, _ := generateUnifiedDiff("", newContent, path)

	// For new files, the entire content is the edit
	totalLines := strings.Count(newContent, "\n") + 1
	if newContent == "" {
		totalLines = 1
	}

	// Use shared finalize logic
	result, err := FinalizeEdit(&t.BaseEditTool, path, fullPath, "", newContent, diff, 1, totalLines, true)
	if err != nil {
		return nil, "", err
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		resultMap = map[string]any{}
	}
	resultMap["action"] = "created"
	resultMap["lines"] = strings.Count(newContent, "\n")

	return resultMap, diff, nil
}

// updateFile modifies an existing file using patch chunks
func (t *UnifiedEditTool) updateFile(ctx context.Context, path, fullPath string, chunks []PatchChunk) (map[string]any, string, error) {
	// Check if file is large
	isLarge, _, err := IsLargeFile(fullPath)
	if err != nil {
		return nil, "", err
	}

	// For large files, delegate to streaming handler (using PatchEditTool's logic)
	if isLarge {
		// For now, load the file anyway - large file streaming can be added later
		// This is a simplification; the full implementation would use streaming
	}

	// Standard approach for small files - load entire file
	content, isNewFile, err := t.ReadFileForEdit(fullPath)
	if err != nil {
		return nil, "", err
	}
	if isNewFile {
		return nil, "", fmt.Errorf("file does not exist: %s (use Add File instead)", path)
	}

	// Apply each chunk, tracking affected line range
	newContent, editStartLine, editEndLine, err := ApplyPatchChunks(content, chunks)
	if err != nil {
		return nil, "", err
	}

	// Generate diff
	diff, _ := generateUnifiedDiff(content, newContent, path)

	// Use shared finalize logic
	result, err := FinalizeEdit(&t.BaseEditTool, path, fullPath, content, newContent, diff, editStartLine, editEndLine, false)
	if err != nil {
		return nil, "", err
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		resultMap = map[string]any{}
	}
	resultMap["chunks"] = len(chunks)
	resultMap["action"] = "updated"

	return resultMap, diff, nil
}

// callSearchReplaceMode handles search/replace editing
func (t *UnifiedEditTool) callSearchReplaceMode(ctx context.Context, path, search, replace string) (any, error) {
	// Validate and resolve path
	fullPath, _, err := t.ValidateAndResolvePath(path)
	if err != nil {
		return nil, err
	}

	// Check if file is large
	isLarge, _, err := IsLargeFile(fullPath)
	if err != nil {
		return nil, err
	}

	// Clear any pending edit for this file (LLM is revising)
	ClearPendingEditForPath(path)

	// For large files, delegate to SearchReplaceEditTool's streaming logic
	if isLarge {
		// For now, load the file anyway - large file handling can be improved later
	}

	// Read file
	content, isNewFile, err := t.ReadFileForEdit(fullPath)
	if err != nil {
		return nil, err
	}

	// Handle new file creation
	if isNewFile {
		if search != "" {
			return map[string]any{
				"success": false,
				"error":   "new_file_with_search",
				"path":    path,
				"message": "File does not exist. To create a new file, use empty search: {\"search\": \"\", \"replace\": \"content\"}",
			}, nil
		}
		return t.createNewFile(path, fullPath, replace)
	}

	// Handle empty search (not allowed for existing files)
	if search == "" {
		return map[string]any{
			"success": false,
			"error":   "empty_search",
			"path":    path,
			"message": "Empty search is only valid for creating new files. For existing files, specify the text to replace.",
		}, nil
	}

	// Check for identical search and replace (no-op edit)
	if search == replace {
		return nil, SemanticErrorf("search and replace text are identical - no change would be made")
	}

	// Find the search text with cascading normalization
	fuzzyThreshold := t.Config.Tools.Edit.FuzzyThreshold
	start, end, level, found := MatchWithNormalization(content, search, fuzzyThreshold)

	if !found {
		return HandleNoMatch(content, search, path), nil
	}

	// Check for multiple matches (only at exact level)
	if level == 0 {
		matchCount := CountMatches(content, search)
		if matchCount > 1 {
			return HandleMultipleMatches(content, search, path, matchCount), nil
		}
	}

	// Perform the replacement
	newContent := content[:start] + replace + content[end:]

	// Generate diff
	diff, err := generateUnifiedDiff(content, newContent, path)
	if err != nil {
		return nil, fmt.Errorf("generate diff: %w", err)
	}

	// Calculate edit line range for post-edit context
	editStartLine, editEndLine := CalculateEditLineRange(newContent, start, replace)

	// Use shared finalize logic
	result, err := FinalizeEdit(&t.BaseEditTool, path, fullPath, content, newContent, diff, editStartLine, editEndLine, false)
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

// createNewFile handles creating a new file in search/replace mode
func (t *UnifiedEditTool) createNewFile(path, fullPath, content string) (any, error) {
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

// callLineMode handles line-based editing
// If endLine is 0, operates in insert mode (inserts at startLine, existing content shifts down)
func (t *UnifiedEditTool) callLineMode(ctx context.Context, path string, startLine, endLine int, newText string) (any, error) {
	// Validate line numbers
	if startLine < 1 {
		return nil, SemanticError("start_line must be >= 1")
	}
	// endLine == 0 means insert mode; otherwise must be >= startLine
	if endLine != 0 && endLine < startLine {
		return nil, SemanticErrorf("end_line (%d) must be >= start_line (%d)", endLine, startLine)
	}

	// Validate and resolve path
	fullPath, _, err := t.ValidateAndResolvePath(path)
	if err != nil {
		return nil, err
	}

	// Clear any pending edit for this file (LLM is revising)
	ClearPendingEditForPath(path)

	// Read file (or prepare for new file creation)
	content, err := os.ReadFile(fullPath)
	isNewFile := false
	if err != nil {
		if os.IsNotExist(err) {
			isNewFile = true
			content = []byte{}
		} else {
			return nil, fmt.Errorf("read file: %w", err)
		}
	}

	oldContent := string(content)

	// Handle new file creation
	if isNewFile {
		newContent := newText

		// Generate diff
		diff, _ := generateUnifiedDiff("", newContent, path)

		// For new files, the entire content is the edit
		totalLines := strings.Count(newContent, "\n") + 1
		if newContent == "" {
			totalLines = 1
		}

		// Use shared finalize logic
		result, err := FinalizeEdit(&t.BaseEditTool, path, fullPath, "", newContent, diff, 1, totalLines, true)
		if err != nil {
			return nil, err
		}

		// Add warning if line numbers seem wrong
		if resultMap, ok := result.(map[string]any); ok {
			if startLine != 1 || endLine != 1 {
				resultMap["warning"] = fmt.Sprintf("Line numbers (%d-%d) are ignored for new files. The entire new_text becomes the file content.", startLine, endLine)
			}
		}

		return result, nil
	}

	// Apply line edit
	newContent, editStartLine, editEndLine, err := ApplyLineEdit(oldContent, startLine, endLine, newText)
	if err != nil {
		return nil, err
	}

	// Generate diff
	diff, _ := generateUnifiedDiff(oldContent, newContent, path)

	// Use shared finalize logic
	return FinalizeEdit(&t.BaseEditTool, path, fullPath, oldContent, newContent, diff, editStartLine, editEndLine, false)
}

// HandleNoMatch returns a helpful error result when search text is not found
func HandleNoMatch(content, search, path string) map[string]any {
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

	return result
}

// HandleMultipleMatches returns an error result with context for disambiguation
func HandleMultipleMatches(content, search, path string, count int) map[string]any {
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
		"success":  false,
		"error":    "multiple_matches",
		"path":     path,
		"count":    count,
		"at_lines": lineNums,
		"message":  fmt.Sprintf("Search text matches %d locations - add more surrounding context to make it unique", count),
		"hint":     "Include more lines before/after the text you want to change to create a unique match",
	}
}

// ApplyPatchChunks applies patch chunks to file content.
// Returns new content, edit start line, edit end line, and error.
func ApplyPatchChunks(content string, chunks []PatchChunk) (string, int, int, error) {
	lines := strings.Split(content, "\n")
	offset := 0 // Track line offset as we modify
	editStartLine := -1
	editEndLine := -1

	for i, chunk := range chunks {
		// Find where this chunk applies
		pos, err := findChunkPosition(lines, chunk, offset)
		if err != nil {
			return "", 0, 0, fmt.Errorf("chunk %d: %w", i+1, err)
		}

		// Track the affected line range (1-based)
		chunkStart := pos + 1 // Convert 0-based to 1-based
		chunkEnd := pos + len(chunk.Additions)
		if chunkEnd < chunkStart {
			chunkEnd = chunkStart
		}

		if editStartLine == -1 || chunkStart < editStartLine {
			editStartLine = chunkStart
		}
		if chunkEnd > editEndLine {
			editEndLine = chunkEnd
		}

		// Apply the chunk
		lines, err = applyChunkToLines(lines, chunk, pos)
		if err != nil {
			return "", 0, 0, fmt.Errorf("chunk %d: %w", i+1, err)
		}

		// Update offset: we removed len(Deletions) and added len(Additions)
		offset += len(chunk.Additions) - len(chunk.Deletions)
	}

	newContent := strings.Join(lines, "\n")

	// Check if patch resulted in no changes
	if content == newContent {
		return "", 0, 0, SemanticError("patch resulted in no changes - deletions and additions are identical")
	}

	// Default line range if tracking failed
	if editStartLine == -1 {
		editStartLine = 1
	}
	if editEndLine == -1 {
		editEndLine = editStartLine
	}

	return newContent, editStartLine, editEndLine, nil
}

// findChunkPosition finds where a chunk should be applied
func findChunkPosition(lines []string, chunk PatchChunk, offset int) (int, error) {
	// If we have context lines, search for them
	if len(chunk.Context) > 0 {
		// Try exact match first
		pos := matchContextLines(lines, chunk.Context, 0)
		if pos >= 0 {
			return pos + len(chunk.Context), nil
		}

		// Try with rstrip normalization
		pos = matchContextLines(lines, chunk.Context, 1)
		if pos >= 0 {
			return pos + len(chunk.Context), nil
		}

		// Try with full strip normalization
		pos = matchContextLines(lines, chunk.Context, 100)
		if pos >= 0 {
			return pos + len(chunk.Context), nil
		}
	}

	// If we have a scope hint, try to find it
	if chunk.Scope != "" {
		pos := findScopeMarker(lines, chunk.Scope)
		if pos >= 0 {
			// Return position after scope line
			return pos + 1, nil
		}
	}

	// If we have deletions but no context, search for the deletions
	if len(chunk.Deletions) > 0 && len(chunk.Context) == 0 {
		pos := matchContextLines(lines, chunk.Deletions, 0)
		if pos >= 0 {
			return pos, nil
		}
	}

	return -1, fmt.Errorf("could not locate context in file")
}

// matchContextLines searches for context lines in the file with specified fuzz level
// fuzz: 0 = exact, 1 = rstrip, 100+ = full strip
func matchContextLines(fileLines, contextLines []string, fuzz int) int {
	if len(contextLines) == 0 {
		return 0
	}
	if len(contextLines) > len(fileLines) {
		return -1
	}

	normalize := func(s string, fuzz int) string {
		switch fuzz {
		case 0:
			return s
		case 1:
			return strings.TrimRight(s, " \t")
		default:
			return strings.TrimSpace(s)
		}
	}

	for i := 0; i <= len(fileLines)-len(contextLines); i++ {
		match := true
		for j, ctx := range contextLines {
			if normalize(fileLines[i+j], fuzz) != normalize(ctx, fuzz) {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// findScopeMarker searches for a scope marker (function/class definition)
func findScopeMarker(lines []string, scope string) int {
	scopeLower := strings.ToLower(strings.TrimSpace(scope))
	for i, line := range lines {
		lineLower := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(lineLower, scopeLower) {
			return i
		}
	}
	return -1
}

// applyChunkToLines applies a single chunk at the given position
func applyChunkToLines(lines []string, chunk PatchChunk, pos int) ([]string, error) {
	// Verify deletions match
	for i, del := range chunk.Deletions {
		lineIdx := pos + i
		if lineIdx >= len(lines) {
			return nil, fmt.Errorf("deletion line %d beyond end of file", lineIdx+1)
		}
		// Check with whitespace normalization
		if strings.TrimSpace(lines[lineIdx]) != strings.TrimSpace(del) {
			return nil, fmt.Errorf("deletion mismatch at line %d: expected %q, found %q",
				lineIdx+1, del, lines[lineIdx])
		}
	}

	// Build new lines: before + additions + after
	newLines := make([]string, 0, len(lines)-len(chunk.Deletions)+len(chunk.Additions))
	newLines = append(newLines, lines[:pos]...)
	newLines = append(newLines, chunk.Additions...)
	newLines = append(newLines, lines[pos+len(chunk.Deletions):]...)

	return newLines, nil
}

// ParsePatch parses a V4A-format patch into structured FilePatch objects
func ParsePatch(patch string) ([]FilePatch, error) {
	lines := strings.Split(patch, "\n")
	var patches []FilePatch
	var currentFile *FilePatch
	var currentChunk *PatchChunk
	inPatch := false

	for i, line := range lines {
		// Check for begin/end markers
		if strings.HasPrefix(line, "*** Begin Patch") {
			inPatch = true
			continue
		}
		if strings.HasPrefix(line, "*** End Patch") {
			// Finalize current chunk and file
			if currentChunk != nil && currentFile != nil {
				currentFile.Chunks = append(currentFile.Chunks, *currentChunk)
			}
			if currentFile != nil {
				patches = append(patches, *currentFile)
			}
			currentFile = nil
			currentChunk = nil
			inPatch = false
			continue
		}

		if !inPatch {
			continue
		}

		// File action markers
		if strings.HasPrefix(line, "*** Add File:") {
			if currentChunk != nil && currentFile != nil {
				currentFile.Chunks = append(currentFile.Chunks, *currentChunk)
			}
			if currentFile != nil {
				patches = append(patches, *currentFile)
			}

			path := strings.TrimSpace(strings.TrimPrefix(line, "*** Add File:"))
			currentFile = &FilePatch{Action: PatchAdd, Path: path}
			currentChunk = &PatchChunk{}
			continue
		}

		if strings.HasPrefix(line, "*** Update File:") {
			if currentChunk != nil && currentFile != nil {
				currentFile.Chunks = append(currentFile.Chunks, *currentChunk)
			}
			if currentFile != nil {
				patches = append(patches, *currentFile)
			}

			path := strings.TrimSpace(strings.TrimPrefix(line, "*** Update File:"))
			currentFile = &FilePatch{Action: PatchUpdate, Path: path}
			currentChunk = nil
			continue
		}

		if strings.HasPrefix(line, "*** Delete File:") {
			if currentChunk != nil && currentFile != nil {
				currentFile.Chunks = append(currentFile.Chunks, *currentChunk)
			}
			if currentFile != nil {
				patches = append(patches, *currentFile)
			}

			path := strings.TrimSpace(strings.TrimPrefix(line, "*** Delete File:"))
			currentFile = &FilePatch{Action: PatchDelete, Path: path}
			currentChunk = nil
			continue
		}

		if currentFile == nil {
			continue
		}

		// Scope marker
		if strings.HasPrefix(line, "@@ ") {
			if currentChunk != nil && len(currentChunk.Context) > 0 {
				currentFile.Chunks = append(currentFile.Chunks, *currentChunk)
			}
			scope := strings.TrimPrefix(line, "@@ ")

			// Extract line hint if present
			lineHint := 0
			if match := lineHintRegex.FindStringSubmatch(scope); match != nil {
				if n, err := parseLineHint(match[1]); err == nil {
					lineHint = n
				}
				scope = lineHintRegex.ReplaceAllString(scope, "")
				scope = strings.TrimSpace(scope)
			}

			currentChunk = &PatchChunk{Scope: scope, LineHint: lineHint}
			continue
		}

		if currentChunk == nil {
			currentChunk = &PatchChunk{}
		}

		// Line prefixes
		if len(line) == 0 {
			currentChunk.Context = append(currentChunk.Context, "")
		} else if line[0] == ' ' {
			content := line[1:]
			if len(currentChunk.Deletions) > 0 || len(currentChunk.Additions) > 0 {
				currentChunk.PostContext = append(currentChunk.PostContext, content)
			} else {
				currentChunk.Context = append(currentChunk.Context, content)
			}
		} else if line[0] == '-' {
			currentChunk.Deletions = append(currentChunk.Deletions, line[1:])
		} else if line[0] == '+' {
			currentChunk.Additions = append(currentChunk.Additions, line[1:])
		} else {
			return nil, fmt.Errorf("line %d: unexpected line format (must start with space, -, +, or @@ ): %q", i+1, line)
		}
	}

	// Finalize if patch didn't have End marker
	if currentChunk != nil && currentFile != nil {
		currentFile.Chunks = append(currentFile.Chunks, *currentChunk)
	}
	if currentFile != nil {
		patches = append(patches, *currentFile)
	}

	return patches, nil
}

// parseLineHint parses a line hint number
func parseLineHint(s string) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	return n, err
}

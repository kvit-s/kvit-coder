package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// lineHintRegex matches ":line N" or ":line N" at end of scope
var lineHintRegex = regexp.MustCompile(`:line\s+(\d+)\s*$`)

// Patch mode metadata for UnifiedEditTool

// PatchEditDescription returns the description for patch edit mode
func PatchEditDescription() string {
	return "Edit a file using V4A-format patch with context lines."
}

// PatchEditJSONSchema returns the JSON schema for patch edit mode
func PatchEditJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"patch": map[string]any{
				"type":        "string",
				"description": "Complete patch in V4A format. See prompt for format details.",
			},
		},
		"required": []string{"patch"},
	}
}

// PatchEditPromptSection returns the prompt section for patch edit mode
func PatchEditPromptSection(previewMode bool) string {
	base := `### Edit - Apply Patches (V4A Format)

**Usage:** ` + "`" + `Edit {"patch": "<V4A patch>"}` + "`" + `

**Format:**
` + "```" + `
*** Begin Patch
*** Update File: src/main.py
@@ def calculate():
     x = 1
-    return x + 1
+    return x + 2
     # done
*** End Patch
` + "```" + `

**Markers:**
- ` + "`*** Begin Patch`" + ` / ` + "`*** End Patch`" + ` - wrap the entire patch
- ` + "`*** Update File: path`" + ` - file to modify
- ` + "`@@ scope`" + ` - optional: function/class name to help locate changes
- ` + "`@@ scope :line 100`" + ` - optional line hint to speed up search

**Line Prefixes:**
- ` + "` ` (space)" + ` - context line (must match file)
- ` + "`-`" + ` - line to remove
- ` + "`+`" + ` - line to add

**Rules:**
1. Include 2-3 lines of context before and after changes
2. Context lines must exactly match file content
3. Always use Read before editing a file`

	if previewMode {
		base += `
- Edit returns diff and after_edit preview with status="pending_confirmation"
- ` + "`Edit.confirm {}`" + ` to apply, ` + "`Edit.cancel {}`" + ` to retry`
	}
	return base
}

// PatchEditTool implements V4A-style patch editing
type PatchEditTool struct {
	BaseEditTool
}

// NewPatchEditTool creates a new PatchEditTool
func NewPatchEditTool(cfg *config.Config) *PatchEditTool {
	return &PatchEditTool{
		BaseEditTool: BaseEditTool{
			Config:        cfg,
			WorkspaceRoot: cfg.Workspace.Root,
		},
	}
}

func (t *PatchEditTool) Name() string {
	return "Edit"
}

func (t *PatchEditTool) Description() string {
	return "Apply a V4A-format patch to modify files. Supports multiple file changes in a single patch."
}

func (t *PatchEditTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"patch": map[string]any{
				"type":        "string",
				"description": "Complete patch in V4A format. See prompt for format details.",
			},
		},
		"required": []string{"patch"},
	}
}

func (t *PatchEditTool) Check(ctx context.Context, args json.RawMessage) error {
	// Patch mode doesn't need read-before-edit check since it includes context
	return nil
}

func (t *PatchEditTool) PromptCategory() string { return "filesystem" }
func (t *PatchEditTool) PromptOrder() int       { return 20 }
func (t *PatchEditTool) PromptSection() string {
	base := `### Edit - Apply Patches (V4A Format)

**Usage:** ` + "`" + `Edit {"patch": "<V4A patch>"}` + "`" + `

**Format:**
` + "```" + `
*** Begin Patch
*** Update File: src/main.py
@@ def calculate():
     x = 1
-    return x + 1
+    return x + 2
     # done
*** End Patch
` + "```" + `

**Markers:**
- ` + "`*** Begin Patch`" + ` and ` + "`*** End Patch`" + ` - wrap the entire patch
- ` + "`*** Update File: path`" + ` - modify existing file
- ` + "`*** Add File: path`" + ` - create new file
- ` + "`*** Delete File: path`" + ` - delete file
- ` + "`@@ scope`" + ` - optional: function/class name to help locate changes
- ` + "`@@ scope :line 100`" + ` - optional line hint to speed up search

**Line Prefixes:**
- ` + "` ` (space)" + ` - context line (must match file)
- ` + "`-`" + ` - line to delete
- ` + "`+`" + ` - line to add

**Rules:**
1. Include 2-3 lines of context before and after changes
2. Context lines must exactly match file content
3. Each file appears only once in a patch
4. Consolidate all changes to a file in order

**Large Files (>1MB):** Handled automatically using streaming. Memory-efficient regardless of file size.`

	if t.Config.Tools.Edit.PreviewMode {
		base += `

**Preview Mode:** Edit returns a diff with status="pending_confirmation".
- ` + "`Edit.confirm {}`" + ` - Apply the edit
- ` + "`Edit.cancel {}`" + ` - Cancel and retry`
	}
	return base
}

// PatchAction represents the type of file operation
type PatchAction string

const (
	PatchAdd    PatchAction = "Add"
	PatchUpdate PatchAction = "Update"
	PatchDelete PatchAction = "Delete"
)

// FilePatch represents a patch for a single file
type FilePatch struct {
	Action PatchAction
	Path   string
	Chunks []PatchChunk
}

// PatchChunk represents a single change within a file
type PatchChunk struct {
	Scope       string   // Optional @@ scope marker
	LineHint    int      // Optional line number hint for large files (from ":line N" in scope)
	Context     []string // Context lines (space prefix) - before the change
	Deletions   []string // Lines to remove (- prefix)
	Additions   []string // Lines to add (+ prefix)
	PostContext []string // Context lines after the change
}

func (t *PatchEditTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Patch string `json:"patch"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, SemanticErrorf("invalid arguments: %v", err)
	}

	if strings.TrimSpace(params.Patch) == "" {
		return nil, SemanticError("patch cannot be empty")
	}

	// Parse the patch
	patches, err := t.parsePatch(params.Patch)
	if err != nil {
		return nil, SemanticErrorf("invalid patch format: %v", err)
	}

	if len(patches) == 0 {
		return nil, SemanticError("no file operations found in patch")
	}

	// Apply each file patch
	var results []map[string]any
	var allDiffs strings.Builder

	for _, fp := range patches {
		result, diff, err := t.applyFilePatch(ctx, fp)
		if err != nil {
			return map[string]any{
				"success":         false,
				"error":           "patch_failed",
				"failed_file":     fp.Path,
				"message":         err.Error(),
				"applied_so_far":  results,
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

// parsePatch parses a V4A-format patch into structured FilePatch objects
func (t *PatchEditTool) parsePatch(patch string) ([]FilePatch, error) {
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
			// Clear state to avoid double-adding at end of function
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
			// Finalize previous file
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
			// Finalize previous file
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
			// Finalize previous file
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

		// Skip if no current file
		if currentFile == nil {
			continue
		}

		// Scope marker
		if strings.HasPrefix(line, "@@ ") {
			// Finalize previous chunk
			if currentChunk != nil && len(currentChunk.Context) > 0 {
				currentFile.Chunks = append(currentFile.Chunks, *currentChunk)
			}
			scope := strings.TrimPrefix(line, "@@ ")

			// Extract line hint if present (e.g., "@@ def foo() :line 100")
			lineHint := 0
			if match := lineHintRegex.FindStringSubmatch(scope); match != nil {
				if n, err := strconv.Atoi(match[1]); err == nil {
					lineHint = n
				}
				// Remove the line hint from scope
				scope = lineHintRegex.ReplaceAllString(scope, "")
				scope = strings.TrimSpace(scope)
			}

			currentChunk = &PatchChunk{Scope: scope, LineHint: lineHint}
			continue
		}

		// Ensure we have a chunk
		if currentChunk == nil {
			currentChunk = &PatchChunk{}
		}

		// Line prefixes
		if len(line) == 0 {
			// Empty line - treat as context
			currentChunk.Context = append(currentChunk.Context, "")
		} else if line[0] == ' ' {
			// Context line
			content := line[1:]
			// If we've seen deletions/additions, this is post-context
			if len(currentChunk.Deletions) > 0 || len(currentChunk.Additions) > 0 {
				currentChunk.PostContext = append(currentChunk.PostContext, content)
			} else {
				currentChunk.Context = append(currentChunk.Context, content)
			}
		} else if line[0] == '-' {
			// Deletion
			currentChunk.Deletions = append(currentChunk.Deletions, line[1:])
		} else if line[0] == '+' {
			// Addition
			currentChunk.Additions = append(currentChunk.Additions, line[1:])
		} else {
			// Unknown line prefix - warn but continue
			return nil, fmt.Errorf("line %d: unexpected line format (must start with space, -, +, or @@ ): %q", i+1, line)
		}
	}

	// Finalize last chunk and file if patch didn't have End marker
	if currentChunk != nil && currentFile != nil {
		currentFile.Chunks = append(currentFile.Chunks, *currentChunk)
	}
	if currentFile != nil {
		patches = append(patches, *currentFile)
	}

	return patches, nil
}

// applyFilePatch applies a single file patch
func (t *PatchEditTool) applyFilePatch(ctx context.Context, fp FilePatch) (map[string]any, string, error) {
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
func (t *PatchEditTool) deleteFile(path, fullPath string) (map[string]any, string, error) {
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
func (t *PatchEditTool) addFile(path, fullPath string, chunks []PatchChunk) (map[string]any, string, error) {
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

	// Write file
	if err := t.WriteFileAtomic(fullPath, newContent, true); err != nil {
		return nil, "", err
	}

	result := BuildEditSuccessResult(path, diff, newContent, 1, totalLines, true)
	result["action"] = "created"
	result["lines"] = strings.Count(newContent, "\n")
	return result, diff, nil
}

// updateFile modifies an existing file using patch chunks
func (t *PatchEditTool) updateFile(ctx context.Context, path, fullPath string, chunks []PatchChunk) (map[string]any, string, error) {
	// Check if file is large
	isLarge, _, err := IsLargeFile(fullPath)
	if err != nil {
		return nil, "", err
	}

	// For large files, use streaming approach
	// Line hints are optional - if not provided, we'll search for context automatically
	if isLarge {
		return t.updateFileLarge(ctx, path, fullPath, chunks)
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
	lines := strings.Split(content, "\n")
	offset := 0 // Track line offset as we modify
	editStartLine := -1
	editEndLine := -1

	for i, chunk := range chunks {
		// Find where this chunk applies
		pos, err := t.findChunkPosition(lines, chunk, offset)
		if err != nil {
			return nil, "", fmt.Errorf("chunk %d: %w", i+1, err)
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
		lines, err = t.applyChunk(lines, chunk, pos)
		if err != nil {
			return nil, "", fmt.Errorf("chunk %d: %w", i+1, err)
		}

		// Update offset: we removed len(Deletions) and added len(Additions)
		offset += len(chunk.Additions) - len(chunk.Deletions)
	}

	newContent := strings.Join(lines, "\n")

	// Check if patch resulted in no changes
	if content == newContent {
		return nil, "", SemanticError("patch resulted in no changes - deletions and additions are identical")
	}

	// Default line range if tracking failed
	if editStartLine == -1 {
		editStartLine = 1
	}
	if editEndLine == -1 {
		editEndLine = editStartLine
	}

	// Generate diff
	diff, _ := generateUnifiedDiff(content, newContent, path)

	// Use shared finalize logic for preview mode and applying
	result, err := FinalizeEdit(&t.BaseEditTool, path, fullPath, content, newContent, diff, editStartLine, editEndLine, false)
	if err != nil {
		return nil, "", err
	}

	// Add chunks count to result
	resultMap, ok := result.(map[string]any)
	if !ok {
		resultMap = map[string]any{}
	}
	resultMap["chunks"] = len(chunks)
	resultMap["action"] = "updated"

	return resultMap, diff, nil
}

// updateFileLarge handles updating large files using streaming
// If line hints are provided, uses them; otherwise searches for context automatically
func (t *PatchEditTool) updateFileLarge(ctx context.Context, path, fullPath string, chunks []PatchChunk) (map[string]any, string, error) {
	var allDiffs strings.Builder
	offset := 0 // Track cumulative line offset from previous chunks

	for i, chunk := range chunks {
		// Determine where to search for this chunk
		var windowStart, windowEnd int

		if chunk.LineHint > 0 {
			// Line hint provided - search around it
			windowStart = chunk.LineHint - 50 + offset
			if windowStart < 1 {
				windowStart = 1
			}
			windowEnd = chunk.LineHint + 50 + len(chunk.Context) + len(chunk.Deletions) + offset
		} else if len(chunk.Context) > 0 {
			// No line hint - search for context automatically using Search tool
			searchText := chunk.Context[0]
			matchLine, matchCount, err := FindMatchWithSearch(ctx, t.Config, fullPath, searchText)
			if err != nil {
				return nil, "", fmt.Errorf("chunk %d: search context: %w", i+1, err)
			}
			if matchCount == 0 {
				return nil, "", fmt.Errorf("chunk %d: context not found in file", i+1)
			}
			// If multiple matches, we'll try the first one and verify with full context
			windowStart = matchLine - 10 + offset
			if windowStart < 1 {
				windowStart = 1
			}
			windowEnd = matchLine + len(chunk.Context) + len(chunk.Deletions) + 50 + offset
		} else if len(chunk.Deletions) > 0 {
			// No context but has deletions - search for first deletion line
			searchText := chunk.Deletions[0]
			matchLine, matchCount, err := FindMatchWithSearch(ctx, t.Config, fullPath, searchText)
			if err != nil {
				return nil, "", fmt.Errorf("chunk %d: search deletions: %w", i+1, err)
			}
			if matchCount == 0 {
				return nil, "", fmt.Errorf("chunk %d: deletion text not found in file", i+1)
			}
			windowStart = matchLine - 10 + offset
			if windowStart < 1 {
				windowStart = 1
			}
			windowEnd = matchLine + len(chunk.Deletions) + 50 + offset
		} else {
			return nil, "", fmt.Errorf("chunk %d: no context, deletions, or line hint - cannot locate where to apply", i+1)
		}

		// Read just this window of lines
		rangeContent, totalLines, err := ReadLineRange(fullPath, windowStart, windowEnd)
		if err != nil {
			return nil, "", fmt.Errorf("chunk %d: read range: %w", i+1, err)
		}

		if windowStart > totalLines {
			return nil, "", fmt.Errorf("chunk %d: search location beyond end of file (%d lines)", i+1, totalLines)
		}

		// Split into lines and find the chunk position within window
		windowLines := strings.Split(rangeContent, "\n")
		pos, err := t.findChunkPosition(windowLines, chunk, 0)
		if err != nil {
			if chunk.LineHint > 0 {
				return nil, "", fmt.Errorf("chunk %d: could not find context near line %d: %w", i+1, chunk.LineHint, err)
			}
			return nil, "", fmt.Errorf("chunk %d: could not match context: %w", i+1, err)
		}

		// Build the new content for this window
		newWindowLines, err := t.applyChunk(windowLines, chunk, pos)
		if err != nil {
			return nil, "", fmt.Errorf("chunk %d: %w", i+1, err)
		}

		// Generate diff for this chunk
		contextLen := len(chunk.Context)
		if contextLen > pos {
			contextLen = pos
		}
		oldStart := pos - contextLen
		if oldStart < 0 {
			oldStart = 0
		}
		oldEnd := pos + len(chunk.Deletions)
		if oldEnd > len(windowLines) {
			oldEnd = len(windowLines)
		}
		newEnd := pos + len(chunk.Additions)
		if newEnd > len(newWindowLines) {
			newEnd = len(newWindowLines)
		}

		oldChunk := strings.Join(windowLines[oldStart:oldEnd], "\n")
		newChunk := strings.Join(newWindowLines[oldStart:newEnd], "\n")
		chunkDiff, _ := generateUnifiedDiff(oldChunk, newChunk, path)
		allDiffs.WriteString(chunkDiff)
		allDiffs.WriteString("\n")

		// Apply this chunk using streaming
		newContent := strings.Join(newWindowLines, "\n")
		if err := t.StreamingLineReplace(fullPath, windowStart, windowEnd, newContent); err != nil {
			return nil, "", fmt.Errorf("chunk %d: streaming replace: %w", i+1, err)
		}

		// Update offset for next chunk
		offset += len(chunk.Additions) - len(chunk.Deletions)
	}

	return map[string]any{
		"action":         "updated",
		"path":           path,
		"success":        true,
		"chunks":         len(chunks),
		"streaming_edit": true,
	}, allDiffs.String(), nil
}

// findChunkPosition finds where a chunk should be applied
func (t *PatchEditTool) findChunkPosition(lines []string, chunk PatchChunk, offset int) (int, error) {
	// If we have context lines, search for them
	if len(chunk.Context) > 0 {
		// Try exact match first
		pos := t.matchContext(lines, chunk.Context, 0)
		if pos >= 0 {
			return pos + len(chunk.Context), nil
		}

		// Try with rstrip normalization
		pos = t.matchContext(lines, chunk.Context, 1)
		if pos >= 0 {
			return pos + len(chunk.Context), nil
		}

		// Try with full strip normalization
		pos = t.matchContext(lines, chunk.Context, 100)
		if pos >= 0 {
			return pos + len(chunk.Context), nil
		}
	}

	// If we have a scope hint, try to find it
	if chunk.Scope != "" {
		pos := t.findScope(lines, chunk.Scope)
		if pos >= 0 {
			// Return position after scope line, adjusted by context
			return pos + 1, nil
		}
	}

	// If we have deletions but no context, search for the deletions
	if len(chunk.Deletions) > 0 && len(chunk.Context) == 0 {
		pos := t.matchContext(lines, chunk.Deletions, 0)
		if pos >= 0 {
			return pos, nil
		}
	}

	return -1, fmt.Errorf("could not locate context in file")
}

// matchContext searches for context lines in the file with specified fuzz level
// fuzz: 0 = exact, 1 = rstrip, 100+ = full strip
func (t *PatchEditTool) matchContext(fileLines, contextLines []string, fuzz int) int {
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

// findScope searches for a scope marker (function/class definition)
func (t *PatchEditTool) findScope(lines []string, scope string) int {
	scopeLower := strings.ToLower(strings.TrimSpace(scope))
	for i, line := range lines {
		lineLower := strings.ToLower(strings.TrimSpace(line))
		if strings.Contains(lineLower, scopeLower) {
			return i
		}
	}
	return -1
}

// applyChunk applies a single chunk at the given position
func (t *PatchEditTool) applyChunk(lines []string, chunk PatchChunk, pos int) ([]string, error) {
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

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/checkpoint"
	"github.com/kvit-s/kvit-coder/internal/config"
)

// RestoreFileTool restores a file to its original state at session start
type RestoreFileTool struct {
	cfg            *config.Config
	checkpointMgr  *checkpoint.Manager
}

func NewRestoreFileTool(cfg *config.Config, mgr *checkpoint.Manager) *RestoreFileTool {
	return &RestoreFileTool{
		cfg:           cfg,
		checkpointMgr: mgr,
	}
}

func (t *RestoreFileTool) Name() string {
	return "RestoreFile"
}

func (t *RestoreFileTool) Description() string {
	return "Restore a file to its original state from the start of the session. IMPORTANT: Only files that existed in the workspace at session start can be restored. Files created during this session cannot be restored (they have no original state to restore to). Use this to undo all changes made to an existing file."
}

func (t *RestoreFileTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "The file path to restore. Must be a file that existed at session start (files created during this session cannot be restored).",
			},
		},
		"required": []string{"path"},
	}
}

func (t *RestoreFileTool) PromptCategory() string { return "filesystem" }
func (t *RestoreFileTool) PromptOrder() int        { return 30 }
func (t *RestoreFileTool) PromptSection() string {
	return `### RestoreFile - Restore File to Original State
Undo ALL changes made to a file during this session, restoring it to its original state.

**IMPORTANT:** Only files that **existed at session start** can be restored. Files you created during this session cannot be restored (they have no original state).

**Parameters:**
- path: File path to restore (must have existed at session start)

**When to use:**
- Your edits broke the file and you want to start over
- Tests are failing after changes and you want to revert
- You want to try a completely different approach

**When NOT to use:**
- On files you created during this session (use rm instead if needed)
- On files that didn't exist before (you'll get an error)

**Example:**
{"path": "src/config.py"}

This will restore the file to exactly how it was when the session started.`
}

type restoreFileArgs struct {
	Path string `json:"path"`
}

func (t *RestoreFileTool) Check(ctx context.Context, args json.RawMessage) error {
	if t.checkpointMgr == nil || !t.checkpointMgr.Enabled() {
		return fmt.Errorf("file restore is not available (checkpoints not enabled)")
	}

	var params restoreFileArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	if strings.TrimSpace(params.Path) == "" {
		return fmt.Errorf("path cannot be empty")
	}

	// Normalize and validate path
	path := params.Path
	if strings.HasPrefix(path, "~/") {
		// Don't allow home directory paths
		return fmt.Errorf("cannot restore files outside workspace")
	}

	// Make absolute
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.cfg.Workspace.Root, path)
	}

	// Check if outside workspace
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	workdir := t.checkpointMgr.Workdir()
	rel, err := filepath.Rel(workdir, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return fmt.Errorf("cannot restore files outside workspace: %s", params.Path)
	}

	return nil
}

func (t *RestoreFileTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params restoreFileArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	// Normalize path
	path := params.Path
	if !filepath.IsAbs(path) {
		path = filepath.Join(t.cfg.Workspace.Root, path)
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Restore the file
	content, err := t.checkpointMgr.RestoreFile(absPath)
	if err != nil {
		return nil, err
	}

	// Make path relative to workspace for display
	displayPath := absPath
	if rel, err := filepath.Rel(t.cfg.Workspace.Root, absPath); err == nil && !strings.HasPrefix(rel, "..") {
		displayPath = rel
	}

	// Count lines in restored content
	lines := strings.Split(string(content), "\n")
	totalLines := len(lines)
	if totalLines > 0 && lines[totalLines-1] == "" {
		totalLines--
	}

	// Truncate content if too large (same limits as read)
	maxLines := 150
	maxBytes := 24 * 1024
	contentStr := string(content)
	wasTruncated := false

	if len(contentStr) > maxBytes {
		contentStr = contentStr[:maxBytes]
		wasTruncated = true
	}
	if lineCount := strings.Count(contentStr, "\n"); lineCount > maxLines {
		lines := strings.SplitN(contentStr, "\n", maxLines+1)
		contentStr = strings.Join(lines[:maxLines], "\n")
		wasTruncated = true
	}

	// Record that this file was read (for read-before-edit enforcement)
	globalReadTracker.RecordRead(absPath, globalReadTracker.CurrentMessageID())

	return map[string]any{
		"success":       true,
		"restored":      true,
		"path":          displayPath,
		"content":       contentStr,
		"was_truncated": wasTruncated,
		"total_lines":   totalLines,
		"message":       "File restored to original state. Content shown below (you can now edit this file).",
	}, nil
}

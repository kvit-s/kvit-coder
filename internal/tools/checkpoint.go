package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/checkpoint"
)

// =============================================================================
// checkpoint.list - List all checkpoints
// =============================================================================

type CheckpointListTool struct {
	manager *checkpoint.Manager
}

func NewCheckpointListTool(manager *checkpoint.Manager) *CheckpointListTool {
	return &CheckpointListTool{manager: manager}
}

func (t *CheckpointListTool) Name() string {
	return "Checkpoint.list"
}

func (t *CheckpointListTool) Description() string {
	return "List all checkpoints with files changed. Shows turn history to help decide which turn to restore to."
}

func (t *CheckpointListTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *CheckpointListTool) PromptCategory() string { return "checkpoint" }
func (t *CheckpointListTool) PromptOrder() int        { return 10 }
func (t *CheckpointListTool) PromptSection() string {
	return `Every tool response includes the current turn number (e.g., ` + "`[Turn 3]`" + `). You can use checkpoint tools to view history and restore files to any previous state.

### checkpoint.list

Lists all checkpoints with files changed in each turn. Use to see what changed and decide which turn to restore to.

**No parameters required.**

### checkpoint.restore

Restores all files to their state after a given turn. Creates a new checkpoint with the restored state.

**Parameters:**
- turn: The turn number to restore to (files will match state AFTER that turn completed)

**IMPORTANT: Restore does NOT delete history.**

When you restore to a previous turn, it creates a NEW checkpoint with the current turn number containing the file contents from the target turn. All previous turns remain accessible.

Example:
` + "```" + `
Turn 5: Added feature X → checkpoint saved (files = A)
Turn 6: Refactored feature X → checkpoint saved (files = B)
Turn 7: Added feature Y → checkpoint saved (files = C)
Turn 8: Tests failing, restore to turn 5 → files reset to A, checkpoint saved
Turn 9: Try different approach → checkpoint saved (files = D)
Turn 10: Still broken, restore to turn 7 → files reset to C, checkpoint saved
` + "```" + `

In this example:
- At turn 8, you restored to turn 5. Files match turn 5, but turns 6 and 7 still exist.
- At turn 10, you restored to turn 7 (which still exists). This works even though turn 8 was itself a restore.
- You can restore to ANY past turn at any time.

### checkpoint.diff

Shows diff between current files and their state at a given turn.

**Parameters:**
- turn: The turn number to compare against
- path: (optional) Specific file path to diff

### checkpoint.undo

Shortcut to restore to (current turn - 1). Useful for quickly undoing the last turn's changes.

**No parameters required.**

### When to Use Checkpoints

- Tests fail after changes → Use ` + "`checkpoint.list`" + ` to see recent turns, then ` + "`checkpoint.restore`" + ` to go back
- Want to try a different approach → ` + "`checkpoint.restore`" + ` to a known good state
- Need to see what changed → ` + "`checkpoint.diff`" + ` to compare with previous turn
- Quick undo → ` + "`checkpoint.undo`" + ` to revert last turn`
}

func (t *CheckpointListTool) Check(ctx context.Context, args json.RawMessage) error {
	if t.manager == nil || !t.manager.Enabled() {
		return fmt.Errorf("checkpoints are not enabled")
	}
	return nil
}

func (t *CheckpointListTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	turns, err := t.manager.List()
	if err != nil {
		return nil, err
	}

	if len(turns) == 0 {
		return map[string]any{
			"current_turn": t.manager.CurrentTurn(),
			"message":      "No checkpoints yet. Checkpoints are created after each turn with tool calls.",
			"turns":        []any{},
		}, nil
	}

	// Format turns for display
	var formattedTurns []map[string]any
	for _, turn := range turns {
		formatted := map[string]any{
			"turn":          turn.Turn,
			"files_changed": len(turn.FilesChanged),
		}

		if len(turn.FilesChanged) > 0 {
			// Show up to 5 files
			if len(turn.FilesChanged) <= 5 {
				formatted["files"] = turn.FilesChanged
			} else {
				formatted["files"] = append(turn.FilesChanged[:5], fmt.Sprintf("... and %d more", len(turn.FilesChanged)-5))
			}
		}

		if turn.IsRestore {
			formatted["is_restore"] = true
			formatted["restored_to"] = turn.RestoredTo
		}

		formattedTurns = append(formattedTurns, formatted)
	}

	return map[string]any{
		"current_turn": t.manager.CurrentTurn(),
		"turns":        formattedTurns,
	}, nil
}

// =============================================================================
// checkpoint.restore - Restore to a previous turn
// =============================================================================

type CheckpointRestoreTool struct {
	manager *checkpoint.Manager
}

func NewCheckpointRestoreTool(manager *checkpoint.Manager) *CheckpointRestoreTool {
	return &CheckpointRestoreTool{manager: manager}
}

func (t *CheckpointRestoreTool) Name() string {
	return "Checkpoint.restore"
}

func (t *CheckpointRestoreTool) Description() string {
	return "Restore all files to the state after a given turn. Creates a new checkpoint with the restored state. Use checkpoint.list to see available turns."
}

func (t *CheckpointRestoreTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"turn": map[string]any{
				"type":        "integer",
				"description": "The turn number to restore to. Files will be restored to their state AFTER this turn completed.",
				"minimum":     0,
			},
		},
		"required": []string{"turn"},
	}
}

func (t *CheckpointRestoreTool) PromptCategory() string { return "checkpoint" }
func (t *CheckpointRestoreTool) PromptOrder() int        { return 20 }
func (t *CheckpointRestoreTool) PromptSection() string   { return "" } // Docs in checkpoint.list

type checkpointRestoreArgs struct {
	Turn int `json:"turn"`
}

func (t *CheckpointRestoreTool) Check(ctx context.Context, args json.RawMessage) error {
	if t.manager == nil || !t.manager.Enabled() {
		return fmt.Errorf("checkpoints are not enabled")
	}

	var params checkpointRestoreArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	currentTurn := t.manager.CurrentTurn()
	if params.Turn < 0 {
		return fmt.Errorf("turn must be >= 0")
	}
	if params.Turn > currentTurn {
		return fmt.Errorf("turn %d does not exist (current turn: %d)", params.Turn, currentTurn)
	}

	return nil
}

func (t *CheckpointRestoreTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params checkpointRestoreArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	changedFiles, err := t.manager.Restore(params.Turn)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status":        "restored",
		"restored_to":   params.Turn,
		"current_turn":  t.manager.CurrentTurn(),
		"files_changed": len(changedFiles),
		"files":         changedFiles,
		"message":       fmt.Sprintf("Restored %d files to state after turn %d. This is now turn %d.", len(changedFiles), params.Turn, t.manager.CurrentTurn()),
	}, nil
}

// =============================================================================
// checkpoint.diff - Show diff between current state and a turn
// =============================================================================

type CheckpointDiffTool struct {
	manager *checkpoint.Manager
}

func NewCheckpointDiffTool(manager *checkpoint.Manager) *CheckpointDiffTool {
	return &CheckpointDiffTool{manager: manager}
}

func (t *CheckpointDiffTool) Name() string {
	return "Checkpoint.diff"
}

func (t *CheckpointDiffTool) Description() string {
	return "Show the diff between current files and their state at a given turn. Useful to see what changed since a turn."
}

func (t *CheckpointDiffTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"turn": map[string]any{
				"type":        "integer",
				"description": "The turn number to compare against",
				"minimum":     0,
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Optional: specific file path to diff. If omitted, diffs all changed files.",
			},
		},
		"required": []string{"turn"},
	}
}

func (t *CheckpointDiffTool) PromptCategory() string { return "checkpoint" }
func (t *CheckpointDiffTool) PromptOrder() int        { return 30 }
func (t *CheckpointDiffTool) PromptSection() string   { return "" } // Docs in checkpoint.list

type checkpointDiffArgs struct {
	Turn int    `json:"turn"`
	Path string `json:"path"`
}

func (t *CheckpointDiffTool) Check(ctx context.Context, args json.RawMessage) error {
	if t.manager == nil || !t.manager.Enabled() {
		return fmt.Errorf("checkpoints are not enabled")
	}

	var params checkpointDiffArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	currentTurn := t.manager.CurrentTurn()
	if params.Turn < 0 {
		return fmt.Errorf("turn must be >= 0")
	}
	if params.Turn > currentTurn {
		return fmt.Errorf("turn %d does not exist (current turn: %d)", params.Turn, currentTurn)
	}

	return nil
}

func (t *CheckpointDiffTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params checkpointDiffArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	diff, err := t.manager.Diff(params.Turn, params.Path)
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(diff) == "" {
		return map[string]any{
			"turn":         params.Turn,
			"current_turn": t.manager.CurrentTurn(),
			"diff":         "",
			"message":      fmt.Sprintf("No differences between current state and turn %d", params.Turn),
		}, nil
	}

	// Count changed files
	lines := strings.Split(diff, "\n")
	fileCount := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "diff --git") {
			fileCount++
		}
	}

	return map[string]any{
		"turn":          params.Turn,
		"current_turn":  t.manager.CurrentTurn(),
		"files_changed": fileCount,
		"diff":          diff,
	}, nil
}

// =============================================================================
// checkpoint.undo - Convenience alias for restore (current turn - 1)
// =============================================================================

type CheckpointUndoTool struct {
	manager *checkpoint.Manager
}

func NewCheckpointUndoTool(manager *checkpoint.Manager) *CheckpointUndoTool {
	return &CheckpointUndoTool{manager: manager}
}

func (t *CheckpointUndoTool) Name() string {
	return "Checkpoint.undo"
}

func (t *CheckpointUndoTool) Description() string {
	return "Undo the last turn's changes. Shortcut for checkpoint.restore to (current turn - 1)."
}

func (t *CheckpointUndoTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
	}
}

func (t *CheckpointUndoTool) PromptCategory() string { return "checkpoint" }
func (t *CheckpointUndoTool) PromptOrder() int        { return 40 }
func (t *CheckpointUndoTool) PromptSection() string   { return "" } // Docs in checkpoint.list

func (t *CheckpointUndoTool) Check(ctx context.Context, args json.RawMessage) error {
	if t.manager == nil || !t.manager.Enabled() {
		return fmt.Errorf("checkpoints are not enabled")
	}

	currentTurn := t.manager.CurrentTurn()
	if currentTurn < 1 {
		return fmt.Errorf("cannot undo: no previous turns (current turn: %d)", currentTurn)
	}

	return nil
}

func (t *CheckpointUndoTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	currentTurn := t.manager.CurrentTurn()
	targetTurn := currentTurn - 1

	changedFiles, err := t.manager.Restore(targetTurn)
	if err != nil {
		return nil, err
	}

	return map[string]any{
		"status":        "undone",
		"undid_turn":    currentTurn,
		"restored_to":   targetTurn,
		"current_turn":  t.manager.CurrentTurn(),
		"files_changed": len(changedFiles),
		"files":         changedFiles,
		"message":       fmt.Sprintf("Undid turn %d, restored to turn %d. This is now turn %d.", currentTurn, targetTurn, t.manager.CurrentTurn()),
	}, nil
}

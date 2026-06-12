package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// DeleteLinesTool removes a range of lines from a file. It is a first-class, explicitly
// named delete affordance (Improvement 3), gated behind tools.edit.explicit_delete. It is
// edit-mode independent (purely line-range based) and routes through the pending-edit
// machinery, so it is previewed/applied by Edit.confirm and discarded by Edit.cancel.
type DeleteLinesTool struct {
	BaseEditTool
}

// NewDeleteLinesTool creates a new DeleteLinesTool.
func NewDeleteLinesTool(cfg *config.Config, toolCtx *ToolContext) *DeleteLinesTool {
	return &DeleteLinesTool{
		BaseEditTool: BaseEditTool{
			Config:        cfg,
			WorkspaceRoot: cfg.Workspace.Root,
			ToolCtx:       toolCtx,
		},
	}
}

func (t *DeleteLinesTool) Name() string { return "DeleteLines" }

func (t *DeleteLinesTool) Description() string {
	return "Delete a range of lines from a file by line number. start_line is required (1-based); end_line is optional (defaults to start_line, inclusive). The lines are removed entirely and the file gets shorter. To blank a line but keep it, use Edit with new_text \"\"."
}

func (t *DeleteLinesTool) Check(ctx context.Context, args json.RawMessage) error {
	return CommonEditCheck(ctx, args, &t.BaseEditTool)
}

func (t *DeleteLinesTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file (relative to workspace or absolute)",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "First line to delete (1-based, inclusive). Required.",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Last line to delete (1-based, inclusive). Omit to delete only start_line.",
			},
		},
		"required": []string{"path", "start_line"},
	}
}

func (t *DeleteLinesTool) PromptCategory() string     { return "filesystem" }
func (t *DeleteLinesTool) PromptOrder() int           { return 24 }
func (t *DeleteLinesTool) PromptTemplateName() string { return "" }
func (t *DeleteLinesTool) PromptSection() string      { return "" } // Docs folded into the Edit lines-mode section

func (t *DeleteLinesTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Path      string `json:"path"`
		StartLine *int   `json:"start_line"`
		EndLine   *int   `json:"end_line"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, SemanticErrorf("invalid arguments: %v", err)
	}
	if params.Path == "" {
		return nil, SemanticError("missing path")
	}
	if params.StartLine == nil {
		return nil, SemanticError("missing start_line")
	}

	startLine := *params.StartLine
	endLine := startLine
	if params.EndLine != nil {
		endLine = *params.EndLine
	}
	if startLine < 1 {
		return nil, SemanticError("start_line must be >= 1")
	}
	if endLine < startLine {
		return nil, SemanticErrorf("end_line (%d) must be >= start_line (%d)", endLine, startLine)
	}

	fullPath, _, err := t.ValidateAndResolvePath(params.Path)
	if err != nil {
		return nil, err
	}

	// Clear any pending edit for this file (LLM is revising).
	ClearPendingEditForPath(t.ToolCtx, params.Path)

	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, SemanticErrorf("file does not exist: %s", params.Path)
		}
		return nil, fmt.Errorf("read file: %w", err)
	}
	oldContent := string(content)

	// Removal = replace [start,end] with empty string. ApplyLineEdit's ""→remove behavior
	// is exactly the deletion we want here (the addressed lines vanish, file shrinks).
	newContent, editStartLine, editEndLine, err := ApplyLineEdit(oldContent, startLine, endLine, "")
	if err != nil {
		return nil, err
	}

	diff, _ := generateUnifiedDiff(oldContent, newContent, params.Path)

	return FinalizeEdit(&t.BaseEditTool, params.Path, fullPath, oldContent, newContent, diff, editStartLine, editEndLine, false)
}

package tools

// Line mode metadata for UnifiedEditTool

// LineEditDescription returns the description for line edit mode
func LineEditDescription() string {
	return "Edit a file by replacing lines in a range. Specify start_line and end_line (both inclusive, 1-based) to replace."
}

// LineEditJSONSchema returns the JSON schema for line edit mode
func LineEditJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file (relative to workspace or absolute)",
			},
			"start_line": map[string]any{
				"type":        "integer",
				"description": "First line number to replace (1-based, inclusive). Required.",
			},
			"end_line": map[string]any{
				"type":        "integer",
				"description": "Last line to replace (1-based, inclusive). Omit to INSERT new_text at start_line (existing content shifts down).",
			},
			"new_text": map[string]any{
				"type":        "string",
				"description": "Replacement text. Will replace all content from start_line to end_line.",
			},
		},
		"required": []string{"path", "start_line", "new_text"},
	}
}

// LineEditPromptSection returns the prompt section for line edit mode
func LineEditPromptSection(previewMode bool) string {
	base := `### Edit - Edit Files

**Usage:** ` + "`" + `Edit {"path": "<file>", "start_line": N, "end_line": N, "new_text": "<text>"}` + "`" + `

Examples:
- ` + "`" + `Edit {"path": "file.py", "start_line": 10, "new_text": "new line\n"}` + "`" + ` - insert at line 10 (original line 10 shifts down)
- ` + "`" + `Edit {"path": "file.py", "start_line": 10, "end_line": 10, "new_text": "    return 43\n"}` + "`" + ` - replace line 10
- ` + "`" + `Edit {"path": "file.py", "start_line": 10, "end_line": 12, "new_text": "new content\n"}` + "`" + ` - replace lines 10-12

**Parameters:**
- ` + "`path`" + `: File path (required)
- ` + "`start_line`" + `: Line number for insert/replace (1-based, required)
- ` + "`end_line`" + `: Last line to replace (inclusive). Omit to insert without replacing.
- ` + "`new_text`" + `: Text to insert or replace with (required)
- Always use Read before editing to get correct line numbers`

	if previewMode {
		base += `
- Edit returns diff and after_edit preview with status="pending_confirmation"
- ` + "`Edit.confirm {}`" + ` to apply, ` + "`Edit.cancel {}`" + ` to retry`
	}
	return base
}

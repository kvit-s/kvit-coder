package tools

// Line mode metadata for UnifiedEditTool

// LineEditDescription returns the description for line edit mode.
// When explicitDelete is true, it tells the model that removal is done via DeleteLines
// and that empty new_text blanks the line(s) rather than deleting them.
func LineEditDescription(explicitDelete bool) string {
	base := "Edit a file by replacing lines in a range. Specify start_line and end_line (both inclusive, 1-based) to replace."
	if explicitDelete {
		base += " Empty new_text blanks the addressed line(s) (keeps them); to remove lines entirely, use the DeleteLines tool."
	}
	return base
}

// LineEditJSONSchema returns the JSON schema for line edit mode.
// When explicitDelete is true, empty new_text blanks the addressed line(s) (deletes go
// through the DeleteLines tool) instead of removing them.
func LineEditJSONSchema(explicitDelete bool) map[string]any {
	newTextDesc := "Replacement text. Use \"\" to DELETE lines entirely. Use \"\\n\" to replace with an empty line (keeps the line but clears content)."
	if explicitDelete {
		newTextDesc = "Replacement text. Use \"\" or \"\\n\" to blank the line(s) while keeping them. To remove lines entirely, use the DeleteLines tool."
	}
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
				"description": newTextDesc,
			},
		},
		"required": []string{"path", "start_line", "new_text"},
	}
}

// LineEditPromptSection returns the prompt section for line edit mode.
// When explicitDelete is true, it documents DeleteLines for removal and empty new_text
// as "blank the line" rather than delete.
func LineEditPromptSection(previewMode, explicitDelete bool) string {
	deleteExample := "- " + "`" + `Edit {"path": "file.py", "start_line": 10, "end_line": 10, "new_text": ""}` + "`" + ` - DELETE line 10 entirely`
	emptyLineExample := "- " + "`" + `Edit {"path": "file.py", "start_line": 10, "end_line": 10, "new_text": "\n"}` + "`" + ` - replace line 10 with empty line (keeps the line)`
	newTextParam := `- ` + "`new_text`" + `: Text to insert/replace. Use "" to DELETE lines. Use "\n" to replace with empty line.`
	if explicitDelete {
		deleteExample = "- " + "`" + `DeleteLines {"path": "file.py", "start_line": 10, "end_line": 10}` + "`" + ` - remove line 10 entirely (file gets shorter)`
		emptyLineExample = "- " + "`" + `Edit {"path": "file.py", "start_line": 10, "end_line": 10, "new_text": ""}` + "`" + ` - blank line 10 (keeps the line, clears content)`
		newTextParam = `- ` + "`new_text`" + `: Text to insert/replace. Use "" or "\n" to blank the line (keeps it). To remove lines, use the DeleteLines tool.`
	}

	base := `### Edit - Edit Files

**Usage:** ` + "`" + `Edit {"path": "<file>", "start_line": N, "end_line": N, "new_text": "<text>"}` + "`" + `

Examples:
- ` + "`" + `Edit {"path": "file.py", "start_line": 10, "new_text": "new line\n"}` + "`" + ` - insert at line 10 (original line 10 shifts down)
- ` + "`" + `Edit {"path": "file.py", "start_line": 10, "end_line": 10, "new_text": "    return 43\n"}` + "`" + ` - replace line 10
- ` + "`" + `Edit {"path": "file.py", "start_line": 10, "end_line": 12, "new_text": "new content\n"}` + "`" + ` - replace lines 10-12
` + deleteExample + `
` + emptyLineExample + `

**Parameters:**
- ` + "`path`" + `: File path (required)
- ` + "`start_line`" + `: Line number for insert/replace (1-based, required)
- ` + "`end_line`" + `: Last line to replace (inclusive). Omit to insert without replacing.
` + newTextParam + `
- Always use Read before editing to get correct line numbers`

	if previewMode {
		base += `
- Edit returns diff and after_edit preview with status="pending_confirmation"
- ` + "`Edit.confirm {}`" + ` to apply, ` + "`Edit.cancel {}`" + ` to retry`
	}
	return base
}

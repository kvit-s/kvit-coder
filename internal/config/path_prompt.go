package config

import "fmt"
import "strings"

// WouldPromptForPath reports whether CheckPathSafety(toolName, identifier)
// would prompt a person, without performing any I/O. It mirrors
// CheckPathSafety's decision structure exactly: inside-workspace and
// pre-permitted paths never prompt, and the warn/block/allow modes never
// prompt (warn only prints). Only ask_once (unconfirmed) and ask_always
// return true.
//
// A subagent child has no UI and would wedge the turn waiting for an answer,
// so every tool site that calls CheckPathSafety first fails closed with a
// semantic error when the context IsSubagentChild() and this helper is true
// (spec/subagents-phase2.md section 3). config cannot import tools, so the
// child flag never reaches this package — the tools side combines the two.
func (c *Config) WouldPromptForPath(toolName, identifier string) bool {
	if c == nil {
		return false
	}
	switch c.Workspace.PathSafetyMode {
	case "ask_once", "ask_always", "":
		// Empty behaves like ask_once: CheckPathSafety's switch falls
		// through to the ask_once branch by default.
	default:
		// allow, block and warn never prompt.
		return false
	}

	id := identifier
	if strings.HasPrefix(toolName, "read") || strings.HasPrefix(toolName, "edit") ||
		toolName == "glob" || toolName == "shell" ||
		toolName == "search" {
		absPath, outside, err := NormalizeAndValidatePath(c.Workspace.Root, identifier)
		if err != nil || !outside {
			return false
		}
		if c.pathIsPermitted(absPath, readOnlyTool(toolName)) {
			return false
		}
		id = absPath
	}

	if c.Workspace.PathSafetyMode == "ask_always" {
		return true
	}
	key := fmt.Sprintf("%s:%s", toolName, id)
	_, exists := c.Tools.SafetyConfirmations[key]
	return !exists
}

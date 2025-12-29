package prompt

import (
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// PromptContext contains all data available to prompt templates.
type PromptContext struct {
	// Configuration
	WorkspaceRoot string
	EditMode      string // "searchreplace", "patch", "lines"
	PreviewMode   bool

	// Enabled tools (for conditionals)
	HasRead       bool
	HasEdit       bool
	HasWrite      bool
	HasSearch     bool
	HasShell      bool
	HasPlan       bool
	HasCheckpoint bool
	HasTasks      bool

	// Derived
	Capabilities  []string
	SearchCommand string // "rg" or "grep"

	// Tool categories for iteration
	EnabledCategories []string

	// Config reference for advanced template logic
	Config *config.Config
}

// NewPromptContext creates a PromptContext from the registry and config.
func NewPromptContext(registry RegistryInterface, cfg *config.Config) PromptContext {
	ctx := PromptContext{
		WorkspaceRoot: cfg.Workspace.Root,
		Config:        cfg,
	}

	// Determine edit mode and preview settings
	if cfg.Tools.Edit.Enabled {
		ctx.EditMode = cfg.Tools.Edit.GetEditMode()
		ctx.PreviewMode = cfg.Tools.Edit.PreviewMode
	}

	// Check enabled tools
	ctx.HasRead = registry.IsEnabled("Read")
	ctx.HasEdit = registry.IsEnabled("Edit")
	ctx.HasWrite = registry.IsEnabled("Write")
	ctx.HasSearch = registry.IsEnabled("Search")
	ctx.HasShell = registry.IsEnabled("Shell")
	ctx.HasPlan = registry.IsEnabled("Plan.create")
	ctx.HasCheckpoint = registry.IsEnabled("Checkpoint.list")
	ctx.HasTasks = registry.IsEnabled("Tasks.Start")

	// Build capabilities list
	ctx.Capabilities = buildCapabilitiesList(registry)

	// Determine search command
	if tools.IsRipgrepAvailable() {
		ctx.SearchCommand = "rg"
	} else {
		ctx.SearchCommand = "grep"
	}

	// Determine enabled categories
	ctx.EnabledCategories = determineEnabledCategories(registry)

	return ctx
}

// buildCapabilitiesList returns a list of capability descriptions based on enabled tools.
func buildCapabilitiesList(registry RegistryInterface) []string {
	var capabilities []string
	if registry.IsEnabled("Read") {
		capabilities = append(capabilities, "reading files")
	}
	if registry.IsEnabled("Edit") {
		capabilities = append(capabilities, "editing files")
	}
	if registry.IsEnabled("Search") {
		capabilities = append(capabilities, "searching code")
	}
	if registry.IsEnabled("Shell") {
		capabilities = append(capabilities, "running shell commands")
	}
	if registry.IsEnabled("Plan.create") {
		capabilities = append(capabilities, "making and tracking plans")
	}
	if registry.IsEnabled("Checkpoint.list") {
		capabilities = append(capabilities, "managing checkpoints")
	}
	if registry.IsEnabled("Tasks.Start") {
		capabilities = append(capabilities, "managing tasks")
	}
	return capabilities
}

// determineEnabledCategories returns the list of tool categories that have enabled tools.
func determineEnabledCategories(registry RegistryInterface) []string {
	categories := []string{"filesystem", "shell", "plan", "checkpoint"}
	var enabled []string

	// Check each category for any enabled tools
	categoryTools := map[string][]string{
		"filesystem": {"Read", "Write", "Edit", "Search"},
		"shell":      {"Shell", "Shell.advanced"},
		"plan":       {"Plan.create", "Plan.completeStep", "Plan.addStep"},
		"checkpoint": {"Checkpoint.list", "Checkpoint.restore"},
	}

	for _, cat := range categories {
		for _, toolName := range categoryTools[cat] {
			if registry.IsEnabled(toolName) {
				enabled = append(enabled, cat)
				break
			}
		}
	}

	return enabled
}

// CapabilitiesString returns capabilities as a comma-separated string.
func (c *PromptContext) CapabilitiesString() string {
	if len(c.Capabilities) == 0 {
		return "various tools"
	}
	return joinStrings(c.Capabilities, ", ")
}

// joinStrings joins strings with a separator.
func joinStrings(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

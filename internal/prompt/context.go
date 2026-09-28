package prompt

import (
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// PromptContext contains all data available to prompt templates.
type PromptContext struct {
	// Configuration
	WorkspaceRoot string
	// Strong is the agent profile. Under it the prompt says what the schema
	// cannot and stops there: no numbered workflow, no worked example, and the
	// short form of each tool's documentation. Under the weak profile the
	// longer prompts stay, so a run on a weaker model is unchanged.
	Strong bool
	// Environment describes where this session is running: platform, shell,
	// working directory, date, git branch and status, and what is in the
	// workspace root.
	Environment string
	EditMode    string // "searchreplace", "patch", "lines"
	PreviewMode bool

	// Enabled tools (for conditionals)
	HasRead   bool
	HasEdit   bool
	HasWrite  bool
	HasSearch bool

	HasShell bool

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
		Strong:        cfg.Agent.IsStrong(),
		Environment:   Environment(cfg.Workspace.Root),
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

	return capabilities
}

// determineEnabledCategories returns the list of tool categories that have enabled tools.
func determineEnabledCategories(registry RegistryInterface) []string {
	categories := []string{"filesystem", "shell"}
	var enabled []string

	// Check each category for any enabled tools
	categoryTools := map[string][]string{
		"filesystem": {"Read", "Write", "Edit", "Search"},
		"shell":      {"Shell", "Shell.advanced"},
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

// Package prompt provides system prompt generation for the agent.
package prompt

import (
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// RegistryInterface defines the registry methods needed for prompt generation
type RegistryInterface interface {
	IsEnabled(name string) bool
	GenerateToolPrompt() string
}

// Generator builds system prompts based on enabled tools and configuration
type Generator struct {
	registry RegistryInterface
	cfg      *config.Config
}

// NewGenerator creates a new prompt generator
func NewGenerator(registry RegistryInterface, cfg *config.Config) *Generator {
	return &Generator{
		registry: registry,
		cfg:      cfg,
	}
}

// GenerateSystemPrompt builds the complete system prompt
func (g *Generator) GenerateSystemPrompt() string {
	// Generate tool documentation dynamically from registered tools
	toolDocs := g.registry.GenerateToolPrompt()
	workflowExample := g.generateWorkflowExample()

	// Build capability list based on enabled tools
	capabilities := g.buildCapabilities()

	capabilityStr := "various tools"
	if len(capabilities) > 0 {
		capabilityStr = strings.Join(capabilities, ", ")
	}

	// Build main tasks based on enabled tools
	mainTasks := g.buildMainTasks()

	// Build workflow steps based on enabled tools
	workflowSteps := g.buildWorkflowSteps()

	// Build guidelines
	guidelines := []string{
		"- Only use the tools listed below",
		"- Briefly explain what you're doing when calling a tool",
		"- Focus on completing the user's request efficiently and accurately",
	}

	// Build the prompt sections
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# ROLE\nYou are a coding assistant with access to tools for: %s.\n\n", capabilityStr))
	sb.WriteString(fmt.Sprintf("Working Directory: %s\n\n", g.cfg.Workspace.Root))

	if len(mainTasks) > 0 {
		sb.WriteString("# MAIN TASKS\n")
		sb.WriteString(strings.Join(mainTasks, "\n"))
		sb.WriteString("\n\n")
	}

	if len(workflowSteps) > 0 {
		sb.WriteString("# WORKFLOW\n")
		sb.WriteString(strings.Join(workflowSteps, "\n"))
		sb.WriteString("\n\n")
	}

	if workflowExample != "" {
		sb.WriteString("# EXAMPLE\n")
		sb.WriteString(workflowExample)
		sb.WriteString("\n")
	}

	sb.WriteString("# GUIDELINES\n")
	sb.WriteString(strings.Join(guidelines, "\n"))
	sb.WriteString("\n\n")

	sb.WriteString("# TOOLS\n")
	sb.WriteString(toolDocs)

	return sb.String()
}

// buildCapabilities returns list of enabled capabilities
func (g *Generator) buildCapabilities() []string {
	var capabilities []string
	if g.registry.IsEnabled("Read") {
		capabilities = append(capabilities, "reading files")
	}
	if g.registry.IsEnabled("Edit") {
		capabilities = append(capabilities, "editing files")
	}
	if g.registry.IsEnabled("Search") {
		capabilities = append(capabilities, "searching code")
	}
	if g.registry.IsEnabled("Shell") {
		capabilities = append(capabilities, "running shell commands")
	}
	if g.registry.IsEnabled("Plan.create") {
		capabilities = append(capabilities, "making and tracking plans")
	}
	if g.registry.IsEnabled("Checkpoint.list") {
		capabilities = append(capabilities, "managing checkpoints")
	}
	if g.registry.IsEnabled("Tasks.Start") {
		capabilities = append(capabilities, "managing tasks")
	}
	return capabilities
}

// buildMainTasks returns list of main tasks based on enabled tools
func (g *Generator) buildMainTasks() []string {
	var mainTasks []string
	if g.registry.IsEnabled("Search") || g.registry.IsEnabled("Read") {
		mainTasks = append(mainTasks, "- Exploring codebase")
	}
	if g.registry.IsEnabled("Read") {
		mainTasks = append(mainTasks, "- Reading and understanding code")
	}
	if g.registry.IsEnabled("Edit") {
		mainTasks = append(mainTasks, "- Making code modifications")
	}
	if g.registry.IsEnabled("Shell") {
		mainTasks = append(mainTasks, "- Running commands")
	}
	return mainTasks
}

// buildWorkflowSteps returns numbered workflow steps
func (g *Generator) buildWorkflowSteps() []string {
	hasShell := g.registry.IsEnabled("Shell")
	var workflowSteps []string
	stepNum := 1

	// Search step
	if g.registry.IsEnabled("Search") {
		workflowSteps = append(workflowSteps, fmt.Sprintf("%d. Search for relevant code using Search tool", stepNum))
		stepNum++
	} else if hasShell {
		searchCmd := "grep"
		if tools.IsRipgrepAvailable() {
			searchCmd = "rg"
		}
		workflowSteps = append(workflowSteps, fmt.Sprintf("%d. Search for relevant code using Shell tool (%s)", stepNum, searchCmd))
		stepNum++
	}

	// Read step
	if g.registry.IsEnabled("Read") {
		workflowSteps = append(workflowSteps, fmt.Sprintf("%d. Read files to understand context using Read tool", stepNum))
		stepNum++
	} else if hasShell {
		workflowSteps = append(workflowSteps, fmt.Sprintf("%d. Read files to understand context using Shell tool (cat)", stepNum))
		stepNum++
	}

	// Edit step
	if g.registry.IsEnabled("Edit") {
		workflowSteps = append(workflowSteps, fmt.Sprintf("%d. Make changes using Edit tool (always read before editing)", stepNum))
		stepNum++
	} else if hasShell {
		workflowSteps = append(workflowSteps, fmt.Sprintf("%d. Make changes using Shell tool (sed, awk, or echo with redirection)", stepNum))
		stepNum++
	}

	// Shell for testing/verification (only if shell enabled and not already used as fallback for everything)
	if hasShell {
		workflowSteps = append(workflowSteps, fmt.Sprintf("%d. Run commands using Shell tool to test/verify", stepNum))
	}

	return workflowSteps
}

// generateWorkflowExample generates a workflow example for the system prompt
// based on which tools are registered in the registry and edit mode configuration
func (g *Generator) generateWorkflowExample() string {
	hasEdit := g.registry.IsEnabled("Edit")
	hasEditPreview := g.registry.IsEnabled("Edit.confirm")
	hasSearch := g.registry.IsEnabled("Search")
	hasRead := g.registry.IsEnabled("Read")
	hasShell := g.registry.IsEnabled("Shell")

	// No workflow needed if no way to modify files
	if !hasEdit && !hasShell {
		return ""
	}

	editMode := ""
	if hasEdit {
		editMode = g.cfg.Tools.Edit.GetEditMode()
	}

	var sb strings.Builder
	sb.WriteString("**File Editing Example:** Add a retry_count field to TokenStats dataclass.\n\n")
	sb.WriteString("```\n")

	stepNum := 1

	// Step: Search
	if hasSearch {
		sb.WriteString(fmt.Sprintf(`# Step %d: SEARCH - Find where TokenStats is defined
Search {"pattern": "class TokenStats", "file_pattern": "*.py"}
→ app/services/llm/token_tracker.py
     8│@dataclass
     9│class TokenStats:
    10│    """Token usage statistics from LLM provider."""

`, stepNum))
		stepNum++
	} else if hasShell {
		searchCmd := "grep -rn"
		if tools.IsRipgrepAvailable() {
			searchCmd = "rg -n"
		}
		sb.WriteString(fmt.Sprintf(`# Step %d: SEARCH - Find where TokenStats is defined
Shell {"command": "%s 'class TokenStats' --include='*.py' ."}
→ app/services/llm/token_tracker.py:9:class TokenStats:

`, stepNum, searchCmd))
		stepNum++
	}

	// Step: Read
	if hasRead {
		sb.WriteString(fmt.Sprintf(`# Step %d: READ - Load the file content
Read {"path": "app/services/llm/token_tracker.py", "start": 8, "limit": 20}
→    8│@dataclass
     9│class TokenStats:
    10│    """Token usage statistics from LLM provider."""
    11│    prompt_tokens: int = 0
    12│    completion_tokens: int = 0

`, stepNum))
		stepNum++
	} else if hasShell {
		sb.WriteString(fmt.Sprintf(`# Step %d: READ - Load the file content
Shell {"command": "cat -n app/services/llm/token_tracker.py | head -30"}
→    8  @dataclass
     9  class TokenStats:
    10      """Token usage statistics from LLM provider."""
    11      prompt_tokens: int = 0
    12      completion_tokens: int = 0

`, stepNum))
		stepNum++
	}

	// Step: Edit - varies by mode
	if hasEdit {
		switch editMode {
		case "searchreplace":
			if hasEditPreview {
				sb.WriteString(fmt.Sprintf(`# Step %d: EDIT - Preview the change
Edit {"path": "app/services/llm/token_tracker.py", "search": "    completion_tokens: int = 0", "replace": "    completion_tokens: int = 0\n    retry_count: int = 0"}
→ PENDING: diff shows proposed change, after_edit shows file preview with edited lines marked ">", call Edit.confirm or Edit.cancel

# Step %d: CAREFULLY REVIEW diff and after_edit before confirming
# In after_edit: ">" marks your edits, unmarked lines show surrounding context
# VERIFY: unmarked context lines connect properly with your changes
#         (no orphaned braces, missing closures, or structural issues)
Edit.confirm {}  # If resulting code structure is valid
Edit.cancel {}   # If after_edit reveals problems, retry with fixed parameters
# Example of wrong edit and fix:
# after_edit shows:  >10│}
#                     11│}   ← orphaned brace! search didn't include enough context
# Fix: Cancel, expand search to include the closing brace, then retry
`, stepNum, stepNum+1))
			} else {
				sb.WriteString(fmt.Sprintf(`# Step %d: EDIT - Apply the change
Edit {"path": "app/services/llm/token_tracker.py", "search": "    completion_tokens: int = 0", "replace": "    completion_tokens: int = 0\n    retry_count: int = 0"}
→ success: true, diff shows applied change, after_edit shows file preview with edited lines marked ">"
# VERIFY: check diff and after_edit to confirm edit was performed correctly
#         (no orphaned braces, missing closures, or structural issues in surrounding context)
# If after_edit shows orphaned brace like:  11│}  ← make follow-up edit to remove it
`, stepNum))
			}
		case "patch":
			sb.WriteString(fmt.Sprintf(`# Step %d: EDIT - Apply the patch
Edit {"patch": "*** Begin Patch\n*** Update File: app/services/llm/token_tracker.py\n@@ class TokenStats:\n     prompt_tokens: int = 0\n     completion_tokens: int = 0\n+    retry_count: int = 0\n \n*** End Patch"}
→ success: true, diff shows applied change, after_edit shows file preview with edited lines marked ">"
# VERIFY: check diff and after_edit to confirm edit was performed correctly
#         (no orphaned braces, missing closures, or structural issues in surrounding context)
`, stepNum))
		default: // "lines" or "unified"
			if hasEditPreview {
				sb.WriteString(fmt.Sprintf(`# Step %d: EDIT - Preview the change
Edit {"path": "app/services/llm/token_tracker.py", "start_line": 12, "end_line": 12, "new_text": "    completion_tokens: int = 0\n    retry_count: int = 0"}
→ PENDING: diff shows proposed change, after_edit shows file preview with edited lines marked ">", call Edit.confirm or Edit.cancel

# Step %d: CAREFULLY REVIEW diff and after_edit before confirming
# In after_edit: ">" marks your edits, unmarked lines show surrounding context
# VERIFY: unmarked context lines connect properly with your changes
#         (no orphaned braces, missing closures, or structural issues)
Edit.confirm {}  # If resulting code structure is valid
Edit.cancel {}   # If after_edit reveals problems, retry with fixed parameters
# Example of wrong edit and fix:
# Edit {"start_line": 15, "end_line": 19, "new_text": "function foo() {\n    return 42;\n}"}
# → after_edit:  >15│function foo() {
#                >16│    return 42;
#                >17│}
#                 18│}   ← orphaned brace! end_line was too low
# Fix: Cancel, retry with end_line=20 to include the original closing brace
`, stepNum, stepNum+1))
			} else {
				sb.WriteString(fmt.Sprintf(`# Step %d: EDIT - Apply the change
Edit {"path": "app/services/llm/token_tracker.py", "start_line": 12, "end_line": 12, "new_text": "    completion_tokens: int = 0\n    retry_count: int = 0"}
→ success: true, diff shows applied change, after_edit shows file preview with edited lines marked ">"
# VERIFY: check diff and after_edit to confirm edit was performed correctly
#         (no orphaned braces, missing closures, or structural issues in surrounding context)
# If after_edit shows orphaned brace like:  18│}  ← make follow-up edit to remove it
`, stepNum))
			}
		}
	} else if hasShell {
		sb.WriteString(fmt.Sprintf(`# Step %d: EDIT - Apply the change using sed
Shell {"command": "sed -i '12a\\    retry_count: int = 0' app/services/llm/token_tracker.py"}
→ (no output on success, read file afterward to verify)
`, stepNum))
	}

	sb.WriteString("```\n\n")
	sb.WriteString("**Key Rules:**\n")
	if hasEdit {
		switch editMode {
		case "searchreplace":
			sb.WriteString("- search text must EXACTLY match file content (character-for-character)\n")
			sb.WriteString("- ALWAYS read the file before editing to see exact content\n")
		case "patch":
			sb.WriteString("- Context lines (space prefix) must exactly match file content\n")
			sb.WriteString("- Include 2-3 lines of context before and after changes\n")
		default: // "lines"
			sb.WriteString("- new_text replaces lines start_line through end_line EXACTLY\n")
			sb.WriteString("- ALWAYS read the file before editing to get correct line numbers\n")
		}
		if hasEditPreview {
			sb.WriteString("- BEFORE confirming: verify unmarked context lines in after_edit connect properly with your edit (no orphaned braces/closures/structural issues)\n")
			sb.WriteString("- If structure looks correct → Edit.confirm. If problems visible → Edit.cancel, MODIFY your edit to correct for the issues, then retry\n")
		} else {
			sb.WriteString("- AFTER editing: verify diff and after_edit show the edit was performed correctly (check for structural issues in surrounding context)\n")
		}
	} else if hasShell {
		sb.WriteString("- ALWAYS read the file before editing to understand the structure\n")
		sb.WriteString("- Use sed for line-based edits, or echo/cat for file rewrites\n")
	}
	return sb.String()
}

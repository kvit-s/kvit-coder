// Package prompt provides system prompt generation for the agent.
package prompt

import (
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
	toolsPkg "github.com/kvit-s/kvit-coder/internal/tools"
)

// RegistryInterface defines the registry methods needed for prompt generation
type RegistryInterface interface {
	IsEnabled(name string) bool
	GenerateToolPrompt() string
	ToolsInCategory(category string) []toolsPkg.Tool
	EnabledCategories() []string
}

// Generator builds system prompts based on enabled tools and configuration
type Generator struct {
	registry RegistryInterface
	cfg      *config.Config
	engine   *TemplateEngine
}

// NewGenerator creates a new prompt generator.
// Returns an error if templates are enabled but fail to load.
func NewGenerator(registry RegistryInterface, cfg *config.Config) (*Generator, error) {
	g := &Generator{
		registry: registry,
		cfg:      cfg,
	}

	// Load template engine if templates are enabled
	if cfg.Prompts.UseTemplates {
		engine, err := LoadTemplates(TemplateConfig{
			TemplatesDir: cfg.Prompts.TemplatesDir,
			HotReload:    cfg.Prompts.HotReload,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to load prompt templates: %w", err)
		}
		g.engine = engine
	}

	return g, nil
}

// NewGeneratorWithEngine creates a prompt generator with a specific template engine.
// Useful for testing or when the engine is already initialized.
func NewGeneratorWithEngine(registry RegistryInterface, cfg *config.Config, engine *TemplateEngine) *Generator {
	return &Generator{
		registry: registry,
		cfg:      cfg,
		engine:   engine,
	}
}

// GenerateSystemPrompt builds the complete system prompt.
// Returns an error if templates are enabled but fail to render.
func (g *Generator) GenerateSystemPrompt() (string, error) {
	// Use template-based generation if templates are enabled
	if g.cfg.Prompts.UseTemplates {
		if g.engine == nil {
			return "", fmt.Errorf("templates enabled but template engine not initialized")
		}
		out, err := g.generateFromTemplates()
		if err != nil {
			return "", err
		}
		return g.rewriteToolNames(out), nil
	}

	if g.cfg.Agent.IsStrong() {
		return g.rewriteToolNames(g.generateStrong()), nil
	}
	// Use hardcoded generation
	return g.rewriteToolNames(g.generateHardcoded()), nil
}

func (g *Generator) rewriteToolNames(text string) string {
	type toolNameRewriter interface {
		RewriteToolNamesForPrompt(string) string
	}
	if rewriter, ok := g.registry.(toolNameRewriter); ok {
		return rewriter.RewriteToolNamesForPrompt(text)
	}
	return text
}

// generateFromTemplates generates the system prompt using templates.
func (g *Generator) generateFromTemplates() (string, error) {
	ctx := NewPromptContext(g.registry, g.cfg)

	// Render main sections (everything except TOOLS)
	var sb strings.Builder

	// Render role
	role, err := g.engine.Render("prompts/sections/role.tmpl", &ctx)
	if err != nil {
		return "", fmt.Errorf("render role: %w", err)
	}
	sb.WriteString(role)

	sb.WriteString(ctx.Environment)

	// A numbered workflow, a list of what the tools are for and a worked
	// example are instructions to a model that needs them. Under the strong
	// profile they are left out; the tool schemas already say what each tool
	// takes, and the sections below say what the schemas cannot.
	if ctx.Strong {
		sb.WriteString(g.mechanisms(&ctx))
		guidelines, err := g.engine.Render("prompts/sections/guidelines.tmpl", &ctx)
		if err != nil {
			return "", fmt.Errorf("render guidelines: %w", err)
		}
		sb.WriteString(guidelines)
		sb.WriteString("# TOOLS\n")
		sb.WriteString(g.generateToolDocsFromTemplates(&ctx))
		return sb.String(), nil
	}

	// Render tasks if we have capabilities
	if len(ctx.Capabilities) > 0 {
		tasks, err := g.engine.Render("prompts/sections/tasks.tmpl", &ctx)
		if err != nil {
			return "", fmt.Errorf("render tasks: %w", err)
		}
		sb.WriteString(tasks)
	}

	// Render workflow
	if ctx.HasRead || ctx.HasEdit || ctx.HasSearch || ctx.HasShell {
		workflow, err := g.engine.Render("prompts/sections/workflow.tmpl", &ctx)
		if err != nil {
			return "", fmt.Errorf("render workflow: %w", err)
		}
		sb.WriteString(workflow)
	}

	// Render example if edit is enabled
	if ctx.HasEdit {
		example, err := g.engine.Render("prompts/sections/example.tmpl", &ctx)
		if err != nil {
			return "", fmt.Errorf("render example: %w", err)
		}
		sb.WriteString(example)
	}

	// Render guidelines
	guidelines, err := g.engine.Render("prompts/sections/guidelines.tmpl", &ctx)
	if err != nil {
		return "", fmt.Errorf("render guidelines: %w", err)
	}
	sb.WriteString(guidelines)

	// Render tools section
	sb.WriteString("# TOOLS\n")
	toolDocs := g.generateToolDocsFromTemplates(&ctx)
	sb.WriteString(toolDocs)

	return sb.String(), nil
}

// generateToolDocsFromTemplates generates tool documentation using templates.
func (g *Generator) generateToolDocsFromTemplates(ctx *PromptContext) string {
	var sb strings.Builder

	categories := g.registry.EnabledCategories()
	for _, cat := range categories {
		tools := g.registry.ToolsInCategory(cat)
		if len(tools) == 0 {
			continue
		}

		// Render category header
		headerTemplate := fmt.Sprintf("prompts/tools/_header-%s.tmpl", cat)
		if g.engine.HasTemplate(headerTemplate) {
			header, err := g.engine.Render(headerTemplate, ctx)
			if err == nil {
				sb.WriteString(header)
			}
		} else {
			// Fallback to CategoryHeaders
			if header, ok := toolsPkg.CategoryHeaders[cat]; ok {
				sb.WriteString(header)
				sb.WriteString("\n\n")
			}
		}

		// Render each tool
		for _, tool := range tools {
			// Try template first
			tmplName := tool.PromptTemplateName()
			if tmplName != "" && !ctx.Strong {
				fullTmplPath := fmt.Sprintf("prompts/tools/%s.tmpl", tmplName)
				if g.engine.HasTemplate(fullTmplPath) {
					rendered, err := g.engine.Render(fullTmplPath, ctx)
					if err == nil {
						sb.WriteString(rendered)
						sb.WriteString("\n")
						continue
					}
				}
			}

			// Fallback to PromptSection(), or its short form under the strong
			// profile, which keeps only the failure modes and invariants.
			if section := promptSectionFor(tool, ctx.Strong); section != "" {
				sb.WriteString(section)
				sb.WriteString("\n\n")
			}
		}

		sb.WriteString("---\n\n")
	}

	return sb.String()
}

// generateHardcoded generates the system prompt using hardcoded strings.
func (g *Generator) generateHardcoded() string {
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
		if toolsPkg.IsRipgrepAvailable() {
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
		if toolsPkg.IsRipgrepAvailable() {
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

// promptSectionFor returns a tool's documentation, preferring its short form
// under the strong profile.
func promptSectionFor(tool toolsPkg.Tool, strong bool) string {
	if strong {
		if short, ok := tool.(toolsPkg.ShortPromptTool); ok {
			if section := short.ShortPromptSection(); section != "" {
				return section
			}
		}
	}
	return tool.PromptSection()
}

// generateStrong builds the prompt for a model that does not need to be told
// how to do its job. What is left is what the tool schemas cannot say: where
// this session is running, the mechanisms that are not tool calls, and each
// tool's failure modes.
func (g *Generator) generateStrong() string {
	ctx := NewPromptContext(g.registry, g.cfg)

	var sb strings.Builder
	fmt.Fprintf(&sb, "# ROLE\nYou are a coding agent working in a real repository. You have tools for %s.\n\n",
		ctx.CapabilitiesString())
	sb.WriteString(ctx.Environment)
	sb.WriteString(g.mechanisms(&ctx))
	sb.WriteString("# GUIDELINES\n")
	sb.WriteString("- Only use the tools listed below.\n")
	sb.WriteString("- Say what you are doing when it is not obvious from the call.\n\n")
	sb.WriteString("# TOOLS\n")
	sb.WriteString(g.generateToolDocs(&ctx))
	return sb.String()
}

// generateToolDocs renders each tool's documentation without the template
// engine, which is what the non-template path uses.
func (g *Generator) generateToolDocs(ctx *PromptContext) string {
	var sb strings.Builder
	for _, cat := range g.registry.EnabledCategories() {
		tools := g.registry.ToolsInCategory(cat)
		if len(tools) == 0 {
			continue
		}
		if header, ok := toolsPkg.CategoryHeaders[cat]; ok {
			sb.WriteString(header)
			sb.WriteString("\n\n")
		}
		for _, tool := range tools {
			if section := promptSectionFor(tool, ctx.Strong); section != "" {
				sb.WriteString(section)
				sb.WriteString("\n\n")
			}
		}
		sb.WriteString("---\n\n")
	}
	return sb.String()
}

// mechanisms documents the things that are not tool calls and that nothing else
// in the prompt would tell the model about: messages that arrive mid-turn,
// reminders about background work, and what happens when the turn is
// interrupted. Only the mechanisms actually in use are described.
func (g *Generator) mechanisms(ctx *PromptContext) string {
	var items []string

	items = append(items, "- A message wrapped in <user-steering> arrived while you were working. "+
		"It is the person you are working for changing what they want, and it takes precedence "+
		"over the instruction that started the turn.")
	items = append(items, "- Text in <system-reminder> is from kvit-coder, not from anyone. "+
		"It is context, never an instruction to obey over the person's.")

	if g.cfg.Tools.Procs.Enabled {
		items = append(items, "- A command started with Shell.start keeps running after this turn ends, "+
			"so a server or a test run started now is still going next turn. When one finishes you are "+
			"told in a <system-reminder>, with its last output.")
		items = append(items, "- Waiting with Observe.wait costs one tool call however long it takes. "+
			"Checking with Shell.output in a loop costs a full round of thinking every time, so prefer waiting.")
	}
	if g.cfg.Tools.Question.Enabled {
		items = append(items, "- Ask with Question when the answer changes what you build and the code "+
			"cannot tell you. Do not ask what reading a file would answer, and do not ask permission to do "+
			"what you were asked to do. Put every question you have in one call. If no one answers, proceed "+
			"on your own judgement and say what you assumed.")
	}
	if g.cfg.Tools.Batch.Enabled {
		items = append(items, "- When the next few calls are already decided and none depends on another's "+
			"result, send them together with Batch: one request and one round of thinking instead of one each.")
	}
	items = append(items, "- The turn can be interrupted. If it is, the tool that was running is killed and "+
		"its partial output is recorded as such; the conversation is kept and can be continued.")

	var sb strings.Builder
	sb.WriteString("# HOW THIS SESSION WORKS\n")
	sb.WriteString(strings.Join(items, "\n"))
	sb.WriteString("\n\n")
	return sb.String()
}

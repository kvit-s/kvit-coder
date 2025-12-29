# Prompt Templates Refactoring Plan

## Goal

Enable prompt optimization without recompilation by moving all system prompt content from hardcoded Go strings to external template files.

## Current Architecture

### Prompt Generation Flow

```
cmd/kvit-coder/main.go
    └── prompt.NewGenerator(registry, cfg)
            └── GenerateSystemPrompt()
                    ├── buildCapabilities()      → hardcoded capability strings
                    ├── buildMainTasks()         → hardcoded task strings
                    ├── buildWorkflowSteps()     → hardcoded step strings
                    ├── generateWorkflowExample()→ large hardcoded example blocks
                    └── registry.GenerateToolPrompt()
                            └── tool.PromptSection() → hardcoded in each tool
```

### Key Files

| File | Role |
|------|------|
| `internal/prompt/prompt.go` | Main generator - assembles ROLE, TASKS, WORKFLOW, EXAMPLE, GUIDELINES, TOOLS sections |
| `internal/tools/registry.go` | Aggregates tool prompts by category with headers |
| `internal/tools/tool.go` | Tool interface requiring `PromptSection()`, `PromptCategory()`, `PromptOrder()` |
| `internal/tools/*.go` | Each tool implements `PromptSection()` returning hardcoded markdown |

### Problems with Current Approach

1. **Requires recompilation** - Any prompt change needs `go build`
2. **Scattered content** - Prompt text spread across 10+ files
3. **Difficult to iterate** - Prompt engineering requires edit→compile→test cycle
4. **No versioning** - Can't easily A/B test different prompts

---

## Proposed Architecture

### Template Engine

Use Go's `text/template` package with a thin wrapper for:
- Loading templates from embedded files or filesystem
- Template inheritance (base + overrides)
- Template functions for conditionals

### Directory Structure

```
prompts/
├── system.tmpl              # Main system prompt template
├── sections/
│   ├── role.tmpl            # ROLE section
│   ├── tasks.tmpl           # MAIN TASKS section
│   ├── workflow.tmpl        # WORKFLOW section
│   ├── guidelines.tmpl      # GUIDELINES section
│   └── examples/
│       ├── edit-searchreplace.tmpl
│       ├── edit-searchreplace-preview.tmpl
│       ├── edit-patch.tmpl
│       ├── edit-lines.tmpl
│       └── edit-lines-preview.tmpl
└── tools/
    ├── _header-filesystem.tmpl
    ├── _header-shell.tmpl
    ├── _header-plan.tmpl
    ├── _header-checkpoint.tmpl
    ├── read.tmpl
    ├── edit.tmpl
    ├── write.tmpl
    ├── search.tmpl
    ├── shell.tmpl
    ├── plan-create.tmpl
    ├── plan-update.tmpl
    ├── plan-complete.tmpl
    ├── checkpoint-list.tmpl
    ├── checkpoint-restore.tmpl
    └── tasks.tmpl
```

### Template Data Context

```go
type PromptContext struct {
    // Configuration
    WorkspaceRoot string
    EditMode      string  // "searchreplace", "patch", "lines"
    PreviewMode   bool

    // Enabled tools (for conditionals)
    HasRead       bool
    HasEdit       bool
    HasSearch     bool
    HasShell      bool
    HasPlan       bool
    HasCheckpoint bool
    HasTasks      bool

    // Derived
    Capabilities  []string
    SearchCommand string  // "rg" or "grep"
}
```

### Example Templates

**prompts/system.tmpl**
```
{{- template "role.tmpl" . -}}

{{- if .Capabilities }}
# MAIN TASKS
{{- template "tasks.tmpl" . }}
{{- end }}

# WORKFLOW
{{- template "workflow.tmpl" . }}

# EXAMPLE
{{- template (printf "examples/edit-%s%s.tmpl" .EditMode (ternary "-preview" "" .PreviewMode)) . }}

# GUIDELINES
{{- template "guidelines.tmpl" . }}

# TOOLS
{{- range .EnabledCategories }}
{{- template (printf "_header-%s.tmpl" .) $ }}
{{- range $.ToolsInCategory . }}
{{- template (printf "tools/%s.tmpl" .Name) $ }}
{{- end }}
---
{{- end }}
```

**prompts/sections/role.tmpl**
```
# ROLE
You are a coding assistant with access to tools for: {{ join .Capabilities ", " }}.

Working Directory: {{ .WorkspaceRoot }}
```

**prompts/tools/search.tmpl**
```
### Search - Find Code Patterns

**Usage:** `Search {"pattern": "<text or regex>"}`

Examples:
- `Search {"pattern": "class AuthForm", "file_pattern": "*.py"}`
- `Search {"pattern": "def authenticate", "path": "src/"}`
- `Search {"pattern": "func.*Handler", "file_pattern": "*.go"}`

**Parameters:**
- `pattern` (required): Text or regex to search for
- `path` (optional): Directory to search in (default: workspace root)
- `file_pattern` (optional): File glob, e.g., "*.py", "*.go"
- `context_lines` (optional): Lines of context around match (default: 3)
```

---

## Implementation Plan

### Phase 1: Template Infrastructure

Create core template loading and rendering system.

**Files to create:**

1. `internal/prompt/templates.go` - Template engine wrapper
   ```go
   type TemplateEngine struct {
       templates *template.Template
       fs        fs.FS  // embed.FS or os.DirFS
   }

   func NewTemplateEngine(templatesFS fs.FS) (*TemplateEngine, error)
   func (e *TemplateEngine) Render(name string, ctx PromptContext) (string, error)
   ```

2. `internal/prompt/context.go` - Template context builder
   ```go
   type PromptContext struct { ... }

   func NewPromptContext(registry RegistryInterface, cfg *config.Config) PromptContext
   ```

3. `internal/prompt/funcs.go` - Template helper functions
   ```go
   func templateFuncs() template.FuncMap {
       return template.FuncMap{
           "join":    strings.Join,
           "ternary": func(t, f any, cond bool) any { ... },
           "indent":  func(n int, s string) string { ... },
       }
   }
   ```

**Embedding strategy:**

```go
//go:embed prompts/*
var embeddedPrompts embed.FS

// Allow override from filesystem
func loadTemplates(cfg *config.Config) (fs.FS, error) {
    if cfg.Prompts.TemplatesDir != "" {
        return os.DirFS(cfg.Prompts.TemplatesDir), nil
    }
    return embeddedPrompts, nil
}
```

### Phase 2: Extract Main Prompt Sections

Move content from `prompt.go` to templates.

1. Create `prompts/sections/role.tmpl` - extract from `GenerateSystemPrompt()`
2. Create `prompts/sections/tasks.tmpl` - extract from `buildMainTasks()`
3. Create `prompts/sections/workflow.tmpl` - extract from `buildWorkflowSteps()`
4. Create `prompts/sections/guidelines.tmpl` - extract hardcoded guidelines
5. Create `prompts/sections/examples/*.tmpl` - extract from `generateWorkflowExample()`

**Refactor `prompt.go`:**

```go
func (g *Generator) GenerateSystemPrompt() string {
    ctx := NewPromptContext(g.registry, g.cfg)
    result, err := g.engine.Render("system.tmpl", ctx)
    if err != nil {
        // fallback to hardcoded or panic
    }
    return result
}
```

### Phase 3: Extract Tool Prompts

Move content from individual tool files to templates.

**Change Tool interface:**

```go
// Option A: Keep interface, change implementation
// Tools return template name instead of content
func (t *SearchTool) PromptSection() string {
    return t.templateEngine.Render("tools/search.tmpl", t.promptContext())
}

// Option B: New interface method
type Tool interface {
    // ...existing methods...
    PromptTemplateName() string  // Returns "search", "read", etc.
}
```

**Recommendation:** Option B is cleaner - tools just declare their template name, and the registry handles rendering.

**Registry changes:**

```go
func (r *Registry) GenerateToolPrompt(engine *TemplateEngine, ctx PromptContext) string {
    var sb strings.Builder
    for _, cat := range categories {
        // Render category header
        sb.WriteString(engine.Render(fmt.Sprintf("_header-%s.tmpl", cat), ctx))

        // Render each tool's section
        for _, tool := range r.toolsInCategory(cat) {
            sb.WriteString(engine.Render(
                fmt.Sprintf("tools/%s.tmpl", tool.PromptTemplateName()),
                ctx,
            ))
        }
    }
    return sb.String()
}
```

### Phase 4: Configuration Support

Add config options for template customization.

**Config additions:**

```yaml
prompts:
  # Override embedded templates with filesystem directory
  templates_dir: ""  # e.g., "/etc/kvit-coder/prompts" or "./prompts"

  # Enable hot-reload of templates (dev mode)
  hot_reload: false
```

**Hot reload for development:**

```go
func (e *TemplateEngine) Reload() error {
    // Re-parse all templates from source
    // Useful during prompt engineering
}
```

---

## Migration Strategy

### Step 1: Parallel Implementation

1. Keep existing hardcoded prompt generation working
2. Add template system alongside
3. Add config flag `prompts.use_templates: bool` (default false)
4. Compare outputs during testing

### Step 2: Validation

1. Generate prompts both ways
2. Assert they produce identical output
3. Run benchmarks to ensure template rendering doesn't add latency

### Step 3: Cutover

1. Default to templates
2. Keep hardcoded as fallback
3. Eventually remove hardcoded strings

---

## File Changes Summary

### New Files

| File | Purpose |
|------|---------|
| `internal/prompt/templates.go` | Template engine wrapper |
| `internal/prompt/context.go` | Template context builder |
| `internal/prompt/funcs.go` | Template helper functions |
| `internal/prompt/embed.go` | Embed directive for prompts/ |
| `prompts/**/*.tmpl` | All template files |

### Modified Files

| File | Changes |
|------|---------|
| `internal/prompt/prompt.go` | Use template engine instead of string building |
| `internal/tools/tool.go` | Add `PromptTemplateName() string` method |
| `internal/tools/registry.go` | Use templates for category headers, pass engine to tools |
| `internal/tools/*.go` | Change `PromptSection()` to return template name or delegate to engine |
| `internal/config/config.go` | Add `Prompts` config section |
| `cmd/kvit-coder/main.go` | Initialize template engine |

---

## Template Functions Reference

```go
template.FuncMap{
    // String manipulation
    "join":      strings.Join,
    "split":     strings.Split,
    "trim":      strings.TrimSpace,
    "lower":     strings.ToLower,
    "upper":     strings.ToUpper,
    "replace":   strings.ReplaceAll,

    // Conditionals
    "ternary":   func(t, f any, cond bool) any,
    "default":   func(def, val any) any,  // return val if non-empty, else def

    // Formatting
    "indent":    func(spaces int, s string) string,
    "backtick":  func(s string) string,  // wraps in backticks
    "codeblock": func(lang, code string) string,

    // Collections
    "contains":  func(slice []string, s string) bool,
    "first":     func(slice []string) string,
    "last":      func(slice []string) string,
}
```

---

## Benefits

1. **No recompilation** - Edit `.tmpl` files, restart app
2. **Centralized content** - All prompt text in `prompts/` directory
3. **Version control** - Track prompt changes separately from code
4. **A/B testing** - Swap template directories to test different prompts
5. **User customization** - Advanced users can provide custom templates
6. **Separation of concerns** - Logic in Go, content in templates

## Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Template syntax errors | Validate templates at startup, fail fast |
| Performance overhead | Templates are parsed once, cached; rendering is fast |
| Missing templates | Embed defaults, only load overrides from filesystem |
| Breaking changes | Keep hardcoded fallback during transition |

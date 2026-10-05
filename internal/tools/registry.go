package tools

import (
	"crypto/rand"
	"encoding/json"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// generateToolCallID generates a valid 9-character alphanumeric tool call ID
func generateToolCallID() string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 9)
	_, _ = rand.Read(b)
	for i := range b {
		b[i] = chars[int(b[i])%len(chars)]
	}
	return string(b)
}

// toolDoc holds a tool's documentation with its sort order
type toolDoc struct {
	order   int
	section string
}

// CategoryHeaders defines the section headers for each category
var CategoryHeaders = map[string]string{
	"filesystem": "## File Tools Reference",
	"shell":      "## Shell Tool",
	"plan":       "## Plan Management Tools",
	"web":        "## Web Tools",
	"mcp":        "## MCP Tools (external servers)",
	"report":     "## Ending the Turn",
}

var modelToolNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// Registry manages enabled tools
type Registry struct {
	tools map[string]Tool

	// aliases holds the tables toolNameAliases builds. Name lookups run for
	// every tool call and every message of a long conversation, so the
	// tables are built once and dropped by Enable and Disable. The pointer
	// is atomic because a Batch resolves names from several goroutines.
	aliases atomic.Pointer[toolAliases]
}

// toolAliases maps internal tool names to the names the model sees and back.
// Callers only read the maps.
type toolAliases struct {
	internalToModel map[string]string
	modelToInternal map[string]string
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Enable adds a tool to the registry (makes it available for use)
func (r *Registry) Enable(t Tool) {
	r.tools[t.Name()] = t
	r.aliases.Store(nil)
}

// Disable removes a tool from the registry
func (r *Registry) Disable(name string) {
	delete(r.tools, name)
	r.aliases.Store(nil)
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) Tool {
	name = strings.TrimSpace(name)
	if tool := r.tools[name]; tool != nil {
		return tool
	}
	return r.tools[r.InternalName(name)]
}

// ModelName returns the provider-facing alias for an internal tool name.
// OpenAI-compatible function names must match ^[A-Za-z0-9_-]+$, while several
// kvit-coder internal names use dots for grouping (Shell.advanced).
func (r *Registry) ModelName(name string) string {
	internalToModel, _ := r.toolNameAliases()
	if modelName, ok := internalToModel[name]; ok {
		return modelName
	}
	return name
}

// InternalName returns the registered internal name for a provider-facing alias.
func (r *Registry) InternalName(name string) string {
	name = strings.TrimSpace(name)
	if r.tools[name] != nil {
		return name
	}
	_, modelToInternal := r.toolNameAliases()
	if internalName, ok := modelToInternal[name]; ok {
		return internalName
	}
	if stripped := r.stripNamespacePrefix(name, modelToInternal); stripped != "" {
		return stripped
	}
	return name
}

// stripNamespacePrefix resolves a tool name carrying a leading namespace the
// model invented, such as "default.Glob" for the registered tool "Glob" or
// "default.Web.search" for "Web.search". Some models prefix every tool call
// with a namespace like "default." or "global.", which is never advertised in
// the specs. The longest suffix that names a registered tool (or one of its
// provider-facing aliases) wins, so dotted tools keep their grouping and
// nested prefixes ("a.b.Read" -> "Read") resolve as well. It returns "" when
// no suffix matches.
func (r *Registry) stripNamespacePrefix(name string, modelToInternal map[string]string) string {
	for i := 0; i < len(name); i++ {
		if name[i] != '.' {
			continue
		}
		suffix := strings.TrimSpace(name[i+1:])
		if suffix == "" {
			continue
		}
		if r.tools[suffix] != nil {
			return suffix
		}
		if internalName, ok := modelToInternal[suffix]; ok {
			return internalName
		}
	}
	return ""
}

// NormalizeToolCallName converts any known internal or provider-facing tool name
// to the provider-facing spelling that can safely be sent back to the LLM API.
func (r *Registry) NormalizeToolCallName(name string) string {
	internalName := r.InternalName(name)
	if r.tools[internalName] == nil {
		return name
	}
	return r.ModelName(internalName)
}

// RewriteToolNamesForPrompt rewrites internal dotted tool names in prompt text
// to their provider-facing aliases so examples match the advertised tool specs.
func (r *Registry) RewriteToolNamesForPrompt(text string) string {
	internalToModel, _ := r.toolNameAliases()
	names := make([]string, 0, len(internalToModel))
	for internalName, modelName := range internalToModel {
		if internalName != modelName {
			names = append(names, internalName)
		}
	}
	if len(names) == 0 {
		return text
	}
	sort.Slice(names, func(i, j int) bool {
		if len(names[i]) == len(names[j]) {
			return names[i] < names[j]
		}
		return len(names[i]) > len(names[j])
	})
	pairs := make([]string, 0, len(names)*2)
	for _, internalName := range names {
		pairs = append(pairs, internalName, internalToModel[internalName])
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// Specs returns OpenAI-compatible tool specs for all registered tools
func (r *Registry) Specs() []llm.ToolSpec {
	// Get tool names and sort them for deterministic ordering
	// This ensures consistent prompt cache hits in llama-server
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	// Build specs in sorted order
	specs := make([]llm.ToolSpec, 0, len(names))
	internalToModel, _ := r.toolNameAliases()
	for _, name := range names {
		tool := r.tools[name]
		spec := llm.ToolSpec{
			Type: "function",
		}
		spec.Function.Name = internalToModel[name]
		spec.Function.Description = tool.Description()
		spec.Function.Parameters = tool.JSONSchema()

		specs = append(specs, spec)
	}

	return specs
}

// All returns all registered tools
func (r *Registry) All() []Tool {
	tools := make([]Tool, 0, len(r.tools))
	for _, t := range r.tools {
		tools = append(tools, t)
	}
	return tools
}

// LooksLikeMalformedToolCall checks if content appears to be a malformed tool call
// (e.g., when LLM outputs "read{\"path\": \"...\"}" as text instead of a proper tool call)
func (r *Registry) LooksLikeMalformedToolCall(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}

	for _, name := range r.toolCallNames() {
		// Check if content starts with "toolname{" (no space)
		if strings.HasPrefix(content, name+"{") {
			return true
		}
		// Also check "toolname {" (with space)
		if strings.HasPrefix(content, name+" {") {
			return true
		}
	}
	return false
}

func (r *Registry) toolNameAliases() (map[string]string, map[string]string) {
	if a := r.aliases.Load(); a != nil {
		return a.internalToModel, a.modelToInternal
	}
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)

	internalToModel := make(map[string]string, len(names))
	modelToInternal := make(map[string]string, len(names))
	used := make(map[string]bool, len(names))

	assign := func(internalName, modelName string) {
		internalToModel[internalName] = modelName
		modelToInternal[modelName] = internalName
		used[modelName] = true
	}

	var invalid []string
	for _, name := range names {
		if modelToolNamePattern.MatchString(name) {
			assign(name, name)
			continue
		}
		invalid = append(invalid, name)
	}

	for _, name := range invalid {
		base := sanitizeModelToolName(name)
		modelName := base
		for i := 2; used[modelName]; i++ {
			modelName = base + "_" + strconv.Itoa(i)
		}
		assign(name, modelName)
	}

	r.aliases.Store(&toolAliases{internalToModel: internalToModel, modelToInternal: modelToInternal})
	return internalToModel, modelToInternal
}

func sanitizeModelToolName(name string) string {
	var b strings.Builder
	for _, ch := range name {
		switch {
		case ch >= 'a' && ch <= 'z':
			b.WriteRune(ch)
		case ch >= 'A' && ch <= 'Z':
			b.WriteRune(ch)
		case ch >= '0' && ch <= '9':
			b.WriteRune(ch)
		case ch == '_' || ch == '-':
			b.WriteRune(ch)
		default:
			b.WriteByte('_')
		}
	}
	if b.Len() == 0 {
		return "tool"
	}
	return b.String()
}

func (r *Registry) toolCallNames() []string {
	internalToModel, _ := r.toolNameAliases()
	seen := make(map[string]bool, len(internalToModel)*2)
	names := make([]string, 0, len(internalToModel)*2)
	for internalName, modelName := range internalToModel {
		if !seen[modelName] {
			names = append(names, modelName)
			seen[modelName] = true
		}
		if !seen[internalName] {
			names = append(names, internalName)
			seen[internalName] = true
		}
	}
	sort.Strings(names)
	return names
}

// ExtractToolCallsFromText attempts to parse tool calls from text content
// This handles cases where LLM describes tool calls in XML-like format or JSON format
func (r *Registry) ExtractToolCallsFromText(content string) []llm.ToolCall {
	var toolCalls []llm.ToolCall

	// Try to find Anthropic-style XML format:
	// <function_calls>
	//   <invoke name="tool_name">
	//     <parameter name="param1">value1</parameter>
	//   </invoke>
	// </function_calls>
	// Also handles antml: namespace prefix variants
	anthropicPattern := regexp.MustCompile(`(?is)<(?:antml:)?function_calls>(.*?)</(?:antml:)?function_calls>`)
	anthropicMatches := anthropicPattern.FindAllStringSubmatch(content, -1)

	for _, blockMatch := range anthropicMatches {
		if len(blockMatch) >= 2 {
			blockContent := blockMatch[1]

			// Find all invoke blocks within this function_calls block
			invokePattern := regexp.MustCompile(`(?is)<(?:antml:)?invoke\s+name="([^"]+)"[^>]*>(.*?)</(?:antml:)?invoke>`)
			invokeMatches := invokePattern.FindAllStringSubmatch(blockContent, -1)

			for _, invokeMatch := range invokeMatches {
				if len(invokeMatch) >= 3 {
					functionName := strings.TrimSpace(invokeMatch[1])
					invokeContent := invokeMatch[2]

					// Parse parameters from <parameter name="...">value</parameter> or <parameter ...>
					paramPattern := regexp.MustCompile(`(?is)<(?:antml:)?parameter\s+name="([^"]+)"[^>]*>(.*?)</(?:antml:)?parameter>`)
					paramMatches := paramPattern.FindAllStringSubmatch(invokeContent, -1)

					argsMap := make(map[string]interface{})
					for _, paramMatch := range paramMatches {
						if len(paramMatch) >= 3 {
							paramName := strings.TrimSpace(paramMatch[1])
							paramValue := strings.TrimSpace(paramMatch[2])
							// Try to parse as JSON first (for arrays, objects, numbers, booleans)
							var jsonVal interface{}
							if err := json.Unmarshal([]byte(paramValue), &jsonVal); err == nil {
								argsMap[paramName] = jsonVal
							} else {
								argsMap[paramName] = paramValue
							}
						}
					}

					// Convert to JSON for the tool call
					argsJSON, err := json.Marshal(argsMap)
					if err == nil {
						toolCalls = append(toolCalls, llm.ToolCall{
							ID:   generateToolCallID(),
							Type: "function",
							Function: struct {
								Name      string `json:"name"`
								Arguments string `json:"arguments"`
							}{
								Name:      r.NormalizeToolCallName(functionName),
								Arguments: string(argsJSON),
							},
						})
					}
				}
			}
		}
	}

	// If we found Anthropic-style tool calls, return early
	if len(toolCalls) > 0 {
		return toolCalls
	}

	// Try to find JSON-in-tool_call format: <tool_call> {"name": "...", "arguments": {...}}</tool_call>
	jsonToolCallPattern := regexp.MustCompile(`(?is)<tool_call>\s*(\{.*?\})\s*</tool_call>`)
	jsonToolCallMatches := jsonToolCallPattern.FindAllStringSubmatch(content, -1)

	for _, match := range jsonToolCallMatches {
		if len(match) >= 2 {
			jsonStr := strings.TrimSpace(match[1])
			// Parse the JSON structure
			var toolCallJSON struct {
				Name      string          `json:"name"`
				Arguments json.RawMessage `json:"arguments"`
			}
			if err := json.Unmarshal([]byte(jsonStr), &toolCallJSON); err == nil && toolCallJSON.Name != "" {
				// Arguments might be a JSON object or already a string
				argsStr := string(toolCallJSON.Arguments)
				// If arguments is already a valid JSON object, use it directly
				// Otherwise wrap it
				var testObj map[string]interface{}
				if json.Unmarshal(toolCallJSON.Arguments, &testObj) != nil {
					// Not a valid object, try to use as-is
					argsStr = "{}"
				}
				toolCalls = append(toolCalls, llm.ToolCall{
					ID:   generateToolCallID(),
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      r.NormalizeToolCallName(toolCallJSON.Name),
						Arguments: argsStr,
					},
				})
			}
		}
	}

	// If we found JSON-style tool calls, return early
	if len(toolCalls) > 0 {
		return toolCalls
	}

	// Try to find XML-like tool call format: <tool_call><function=Name>...</tool_call>
	// Also handle cases where <tool_call> opening tag is missing but closing tag is present
	xmlPattern := regexp.MustCompile(`(?is)(?:<tool_call>\s*)?<function=([^>]+)>(.*?)</function>\s*</tool_call>`)
	matches := xmlPattern.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) >= 3 {
			functionName := strings.TrimSpace(match[1])
			arguments := strings.TrimSpace(match[2])

			// Parse arguments from XML-like format: <parameter=name>value</parameter>
			argPattern := regexp.MustCompile(`(?is)<parameter=([^>]+)>(.*?)</parameter>`)
			argMatches := argPattern.FindAllStringSubmatch(arguments, -1)

			argsMap := make(map[string]string)
			for _, argMatch := range argMatches {
				if len(argMatch) >= 3 {
					paramName := strings.TrimSpace(argMatch[1])
					paramValue := strings.TrimSpace(argMatch[2])
					argsMap[paramName] = paramValue
				}
			}

			// Convert to JSON for the tool call
			argsJSON, err := json.Marshal(argsMap)
			if err == nil {
				toolCalls = append(toolCalls, llm.ToolCall{
					ID:   generateToolCallID(),
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      r.NormalizeToolCallName(functionName),
						Arguments: string(argsJSON),
					},
				})
			}
		}
	}

	// Try to find JSON-like tool call format: toolname{"param": "value"}
	for _, toolName := range r.toolCallNames() {
		// Look for toolName{"param": "value"}
		jsonPattern := regexp.MustCompile(`\b` + regexp.QuoteMeta(toolName) + `\s*\{([^}]*)\}`)
		jsonMatches := jsonPattern.FindAllStringSubmatch(content, -1)

		for _, jsonMatch := range jsonMatches {
			if len(jsonMatch) >= 2 {
				argsStr := "{" + jsonMatch[1] + "}"
				// Validate it's valid JSON
				var argsMap map[string]interface{}
				if err := json.Unmarshal([]byte(argsStr), &argsMap); err == nil {
					argsJSON, _ := json.Marshal(argsMap)
					toolCalls = append(toolCalls, llm.ToolCall{
						ID:   generateToolCallID(),
						Type: "function",
						Function: struct {
							Name      string `json:"name"`
							Arguments string `json:"arguments"`
						}{
							Name:      r.NormalizeToolCallName(toolName),
							Arguments: string(argsJSON),
						},
					})
				}
			}
		}
	}

	if len(toolCalls) > 0 {
		return toolCalls
	}

	// Try to find JSON object with tool name as key: {"tool_name": {"param": "value"}}
	// This handles cases where LLM outputs JSON with tool calls as object keys
	// The JSON might be embedded in text, so we try to find and extract it
	jsonStart := strings.Index(content, "{")
	if jsonStart >= 0 {
		// Find matching closing brace by counting braces
		jsonContent := content[jsonStart:]
		braceCount := 0
		jsonEnd := -1
		for i, c := range jsonContent {
			if c == '{' {
				braceCount++
			} else if c == '}' {
				braceCount--
				if braceCount == 0 {
					jsonEnd = i + 1
					break
				}
			}
		}
		if jsonEnd > 0 {
			jsonStr := jsonContent[:jsonEnd]
			var jsonObj map[string]interface{}
			if err := json.Unmarshal([]byte(jsonStr), &jsonObj); err == nil {
				for _, toolName := range r.toolCallNames() {
					if args, ok := jsonObj[toolName]; ok {
						if argsMap, ok := args.(map[string]interface{}); ok {
							argsJSON, err := json.Marshal(argsMap)
							if err == nil {
								toolCalls = append(toolCalls, llm.ToolCall{
									ID:   generateToolCallID(),
									Type: "function",
									Function: struct {
										Name      string `json:"name"`
										Arguments string `json:"arguments"`
									}{
										Name:      r.NormalizeToolCallName(toolName),
										Arguments: string(argsJSON),
									},
								})
							}
						}
					}
				}
			}
		}
	}

	return toolCalls
}

// PromptSections returns documentation for all registered tools, grouped by category
func (r *Registry) PromptSections() map[string][]toolDoc {
	sections := make(map[string][]toolDoc)
	for _, tool := range r.tools {
		category := tool.PromptCategory()
		if section := tool.PromptSection(); section != "" {
			sections[category] = append(sections[category], toolDoc{
				order:   tool.PromptOrder(),
				section: section,
			})
		}
	}
	return sections
}

// GenerateToolPrompt returns complete tool documentation for system prompt
func (r *Registry) GenerateToolPrompt() string {
	sections := r.PromptSections()
	var sb strings.Builder

	// Generate in deterministic order
	categories := []string{"filesystem", "shell", "plan", "mcp", "report"}
	for _, cat := range categories {
		docs, ok := sections[cat]
		if !ok || len(docs) == 0 {
			continue
		}

		// Write category header
		if header, ok := CategoryHeaders[cat]; ok {
			sb.WriteString(header)
			sb.WriteString("\n\n")
		}

		// Sort docs within category by PromptOrder()
		sort.Slice(docs, func(i, j int) bool {
			return docs[i].order < docs[j].order
		})

		for _, doc := range docs {
			sb.WriteString(doc.section)
			sb.WriteString("\n\n")
		}

		sb.WriteString("---\n\n")
	}
	return sb.String()
}

// IsEnabled returns true if a tool with the given name is enabled
func (r *Registry) IsEnabled(name string) bool {
	return r.tools[name] != nil
}

// ListTools returns a sorted list of all enabled tool names
func (r *Registry) ListTools() []string {
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ToolsInCategory returns tools in a given category, sorted by PromptOrder
func (r *Registry) ToolsInCategory(category string) []Tool {
	var tools []Tool
	for _, tool := range r.tools {
		if tool.PromptCategory() == category {
			tools = append(tools, tool)
		}
	}
	// Sort by PromptOrder
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].PromptOrder() < tools[j].PromptOrder()
	})
	return tools
}

// EnabledCategories returns the list of categories that have enabled tools
func (r *Registry) EnabledCategories() []string {
	categoryOrder := []string{"filesystem", "shell", "web", "plan", "mcp", "report"}
	var enabled []string
	for _, cat := range categoryOrder {
		if len(r.ToolsInCategory(cat)) > 0 {
			enabled = append(enabled, cat)
		}
	}
	return enabled
}

package tools

import (
	"crypto/rand"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

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
	"checkpoint": "## Checkpoints and Undo",
}

// Registry manages enabled tools
type Registry struct {
	tools map[string]Tool
}

func NewRegistry() *Registry {
	return &Registry{
		tools: make(map[string]Tool),
	}
}

// Enable adds a tool to the registry (makes it available for use)
func (r *Registry) Enable(t Tool) {
	r.tools[t.Name()] = t
}

// Disable removes a tool from the registry
func (r *Registry) Disable(name string) {
	delete(r.tools, name)
}

// Get retrieves a tool by name
func (r *Registry) Get(name string) Tool {
	return r.tools[name]
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
	for _, name := range names {
		tool := r.tools[name]
		spec := llm.ToolSpec{
			Type: "function",
		}
		spec.Function.Name = tool.Name()
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

	for name := range r.tools {
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
								Name:      functionName,
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
						Name:      toolCallJSON.Name,
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
						Name:      functionName,
						Arguments: string(argsJSON),
					},
				})
			}
		}
	}
	
	// Try to find JSON-like tool call format: toolname{"param": "value"}
	for toolName := range r.tools {
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
							Name:      toolName,
							Arguments: string(argsJSON),
						},
					})
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
	categories := []string{"filesystem", "shell", "plan", "checkpoint"}
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
	categoryOrder := []string{"filesystem", "shell", "plan", "checkpoint"}
	var enabled []string
	for _, cat := range categoryOrder {
		if len(r.ToolsInCategory(cat)) > 0 {
			enabled = append(enabled, cat)
		}
	}
	return enabled
}

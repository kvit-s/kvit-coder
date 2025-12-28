package llm

import (
	"regexp"
	"strings"
)

// ToolCallExtractor is an interface for extracting tool calls from text
type ToolCallExtractor interface {
	ExtractToolCallsFromText(content string) []ToolCall
}

// ResponseNormalizer handles normalization of LLM responses
type ResponseNormalizer struct {
	extractor     ToolCallExtractor
	mergeThinking bool
}

// NewResponseNormalizer creates a new response normalizer
func NewResponseNormalizer(extractor ToolCallExtractor, mergeThinking bool) *ResponseNormalizer {
	return &ResponseNormalizer{
		extractor:     extractor,
		mergeThinking: mergeThinking,
	}
}

// NormalizeResponse applies all middleware transformations to an LLM response.
// Returns true if tool calls were extracted (requires updating finish reason).
func (n *ResponseNormalizer) NormalizeResponse(msg *Message) bool {
	toolCallsExtracted := false

	// Normalize tool call types
	NormalizeToolCallTypes(msg)

	// Extract tool calls from reasoning content if no tool calls present
	if len(msg.ToolCalls) == 0 && msg.ReasoningContent != "" && n.extractor != nil {
		extractedToolCalls := n.extractor.ExtractToolCallsFromText(msg.ReasoningContent)
		if len(extractedToolCalls) > 0 {
			msg.ToolCalls = extractedToolCalls
			msg.ReasoningContent = ExtractNonToolCallContent(msg.ReasoningContent)
			toolCallsExtracted = true
		}
	}

	// Extract tool calls from content if no tool calls present
	if len(msg.ToolCalls) == 0 && msg.Content != "" && n.extractor != nil {
		extractedToolCalls := n.extractor.ExtractToolCallsFromText(msg.Content)
		if len(extractedToolCalls) > 0 {
			msg.ToolCalls = extractedToolCalls
			msg.Content = ExtractNonToolCallContent(msg.Content)
			toolCallsExtracted = true
		}
	}

	// Handle ReasoningContent based on merge_thinking config
	if len(msg.ToolCalls) > 0 && msg.ReasoningContent != "" {
		if n.mergeThinking {
			if msg.Content != "" {
				msg.Content = msg.ReasoningContent + "\n\n" + msg.Content
			} else {
				msg.Content = msg.ReasoningContent
			}
		}
		msg.ReasoningContent = ""
	}

	return toolCallsExtracted
}

// NormalizeToolCallTypes ensures all tool calls have type: "function"
func NormalizeToolCallTypes(msg *Message) {
	for i := range msg.ToolCalls {
		if msg.ToolCalls[i].Type == "" {
			msg.ToolCalls[i].Type = "function"
		}
	}
}

// ExtractNonToolCallContent extracts the non-tool-call parts from content
// that contains XML-like tool call descriptions
func ExtractNonToolCallContent(content string) string {
	// Remove XML tool call blocks completely
	regex := regexp.MustCompile("`(?is)<tool_call>.*?</tool_call>`")
	cleaned := regex.ReplaceAllString(content, "")

	// Clean up any remaining tags and whitespace
	cleaned = strings.ReplaceAll(cleaned, "<function=", "")
	cleaned = strings.ReplaceAll(cleaned, "</function>", "")
	cleaned = strings.ReplaceAll(cleaned, "<parameter=", "")
	cleaned = strings.ReplaceAll(cleaned, "</parameter>", "")
	cleaned = strings.ReplaceAll(cleaned, ">", "")
	cleaned = strings.ReplaceAll(cleaned, "<", "")

	// Trim and clean up whitespace
	cleaned = strings.TrimSpace(cleaned)
	cleaned = regexp.MustCompile(`\s+`).ReplaceAllString(cleaned, " ")

	return cleaned
}

// PreventConsecutiveAssistant checks if the last message is an assistant message
// and removes it if so, returning true if a message was removed.
func PreventConsecutiveAssistant(messages []Message) ([]Message, bool) {
	if len(messages) > 0 && messages[len(messages)-1].Role == RoleAssistant {
		return messages[:len(messages)-1], true
	}
	return messages, false
}

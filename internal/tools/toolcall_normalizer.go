package tools

import (
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// NormalizeToolCallTypes ensures all tool calls have type: "function"
// This handles cases where LLMs return tool calls with empty or missing type fields,
// which causes validation errors when replaying messages to stricter APIs like Mistral.
func NormalizeToolCallTypes(msg *llm.Message) {
	for i := range msg.ToolCalls {
		if msg.ToolCalls[i].Type == "" {
			msg.ToolCalls[i].Type = "function"
		}
	}
}

package tools

import (
	"context"
	"encoding/json"
)

// Tool is the interface all agent tools must implement
type Tool interface {
	// Name returns the tool identifier (e.g., "shell", "read")
	Name() string

	// Description returns a human-readable description for the LLM
	Description() string

	// JSONSchema returns the OpenAI-compatible function schema
	JSONSchema() map[string]any

	// Check performs validation and user confirmations before execution
	// Returns error if the tool should not be executed
	Check(ctx context.Context, args json.RawMessage) error

	// Call executes the tool with the given arguments
	// Check should be called before Call
	Call(ctx context.Context, args json.RawMessage) (any, error)

	// PromptSection returns detailed usage documentation for the system prompt.
	// Returns empty string if no additional documentation is needed.
	PromptSection() string

	// PromptCategory returns the category for grouping in the system prompt.
	// Valid categories: "filesystem", "shell", "plan", "checkpoint"
	PromptCategory() string

	// PromptOrder returns the sort order within the category (lower numbers first).
	// This ensures deterministic ordering for prompt caching.
	PromptOrder() int
}

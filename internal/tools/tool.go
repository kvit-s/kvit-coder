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
	// Deprecated: Use PromptTemplateName() with the template system instead.
	PromptSection() string

	// PromptCategory returns the category for grouping in the system prompt.
	// Valid categories: "filesystem", "shell", "plan", "checkpoint"
	PromptCategory() string

	// PromptOrder returns the sort order within the category (lower numbers first).
	// This ensures deterministic ordering for prompt caching.
	PromptOrder() int

	// PromptTemplateName returns the name of the template file for this tool's
	// system prompt documentation. Returns empty string to use PromptSection() instead.
	// Template files are located in prompts/tools/<name>.tmpl
	PromptTemplateName() string
}

// ParallelSafeTool is an optional interface a Tool implements to say it can run
// at the same time as other calls in a Batch. Only a tool that reads is safe:
// Read, Search and the process-status tools return true, while Edit, Write and
// Shell do not, because they change the workspace or the pending-edit state
// that the call after them reads. A tool that says nothing is treated as
// unsafe, which is the answer that cannot be wrong.
type ParallelSafeTool interface {
	ParallelSafe() bool
}

// SelfTimeoutTool is an optional interface a Tool can implement to opt out of the
// agent loop's blanket per-tool timeout (15s) and manage its own execution
// deadline instead. MCP tools implement it: their calls (a web fetch, a database
// query, a browser action) routinely exceed 15s, and each applies its own
// configurable per-call deadline inside Call.
type SelfTimeoutTool interface {
	SelfTimeout() bool
}

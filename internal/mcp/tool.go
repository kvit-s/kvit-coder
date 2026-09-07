package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/tools"
)

// PromptCategory is the system-prompt category MCP tools are grouped under. It
// must match the entries added to internal/tools registry category machinery.
const PromptCategory = "mcp"

// Result size caps for tool output landing in the context window. Output beyond
// these is spilled to a temp file (when a TempFileManager is available) and a
// truncated preview is returned, mirroring how Shell/Search handle large output.
const (
	mcpResultMaxLines = 150
	mcpResultMaxBytes = 24 * 1024
	mcpPreviewLines   = 75
	mcpPreviewBytes   = 12 * 1024
)

// MCPTool adapts a single tool advertised by an MCP server to the tools.Tool
// interface. The rest of kvit-coder treats it like any built-in tool; the
// adapter delegates execution to the server over the Client.
type MCPTool struct {
	server      string // server name (for namespacing and docs)
	rawName     string // tool name as the server knows it (used in tools/call)
	name        string // model-facing namespaced name
	description string
	schema      map[string]any
	order       int

	client      Client
	confirmer   *confirmer
	confirm     string // confirm policy for this server
	callTimeout time.Duration
	tempFileMgr *tools.TempFileManager
}

// newMCPTool builds an adapter for one discovered tool.
func newMCPTool(server string, desc ToolDescriptor, order int, client Client, conf *confirmer, confirmPolicy string, callTimeout time.Duration, sanitize bool, tempFileMgr *tools.TempFileManager) *MCPTool {
	schema := desc.InputSchema
	if sanitize {
		schema = SanitizeSchema(schema)
	} else if len(schema) == 0 {
		schema = map[string]any{"type": "object"}
	}
	description := strings.TrimSpace(desc.Description)
	if description == "" {
		description = fmt.Sprintf("Tool %q from MCP server %q.", desc.Name, server)
	}
	return &MCPTool{
		server:      server,
		rawName:     desc.Name,
		name:        NamespacedName(server, desc.Name),
		description: description,
		schema:      schema,
		order:       order,
		client:      client,
		confirmer:   conf,
		confirm:     confirmPolicy,
		callTimeout: callTimeout,
		tempFileMgr: tempFileMgr,
	}
}

// Name returns the namespaced, model-facing tool name.
func (t *MCPTool) Name() string { return t.name }

// Description returns the server's tool description.
func (t *MCPTool) Description() string { return t.description }

// JSONSchema returns the server's input schema (optionally sanitized), passed
// through to the model as the function parameter schema.
func (t *MCPTool) JSONSchema() map[string]any { return t.schema }

// Check enforces the confirmation policy. There is no workspace path check:
// these tools run remotely and have their own trust model.
func (t *MCPTool) Check(ctx context.Context, args json.RawMessage) error {
	if t.confirmer == nil {
		return nil
	}
	return t.confirmer.Confirm(t.name, t.server, t.confirm, argsPreview(args))
}

// Call issues tools/call with the raw tool name under the per-call deadline and
// maps the typed result back to something the model can read.
func (t *MCPTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var argMap map[string]any
	if len(strings.TrimSpace(string(args))) > 0 {
		if err := json.Unmarshal(args, &argMap); err != nil {
			return nil, tools.SemanticErrorf("invalid arguments for %s: %v", t.name, err)
		}
	}

	// Derive a deadline from the (un-capped) tool context. The agent loop
	// exempts MCP tools from its blanket 15s timeout; this is their real bound.
	callCtx := ctx
	if t.callTimeout > 0 {
		var cancel context.CancelFunc
		callCtx, cancel = context.WithTimeout(ctx, t.callTimeout)
		defer cancel()
	}

	res, err := t.client.CallTool(callCtx, t.rawName, argMap)
	if err != nil {
		// Transport/protocol failure (server crashed, timed out, bad response).
		// A plain (runtime) error: the loop records it as a non-recoverable
		// tool error rather than discarding the step for a retry.
		return nil, fmt.Errorf("MCP call to %s failed: %w", t.name, err)
	}

	text, notes := renderContent(res.Content)

	if res.IsError {
		// Tool-level failure the model can recover from: backtrackable, mirroring
		// how built-in tools report recoverable misuse.
		msg := strings.TrimSpace(text)
		if msg == "" {
			msg = "tool reported an error"
		}
		return nil, tools.SemanticErrorf("%s: %s", t.name, msg)
	}

	content := t.capContent(text)
	out := map[string]any{
		"success": true,
		"content": content,
	}
	if len(notes) > 0 {
		out["attachments"] = notes
	}
	return out, nil
}

func (t *MCPTool) PromptCategory() string     { return PromptCategory }
func (t *MCPTool) PromptOrder() int           { return t.order }
func (t *MCPTool) PromptTemplateName() string { return "" }

// PromptSection documents the tool for the system prompt. It names the source
// server so the model knows the tool is external and may have side effects.
func (t *MCPTool) PromptSection() string {
	desc := strings.TrimSpace(t.description)
	return fmt.Sprintf("### %s\n\nFrom external MCP server %q. %s", t.name, t.server, desc)
}

// SelfTimeout marks the tool as managing its own execution deadline, exempting
// it from the agent loop's blanket 15s per-tool timeout.
func (t *MCPTool) SelfTimeout() bool { return true }

// capContent truncates oversized text, spilling the full content to a temp file
// when a TempFileManager is available so the model can read it on demand.
func (t *MCPTool) capContent(text string) string {
	res := tools.TruncateContent([]byte(text), mcpResultMaxLines, mcpResultMaxBytes, mcpPreviewLines, mcpPreviewBytes)
	if !res.WasTruncated {
		return res.Content
	}
	if t.tempFileMgr == nil {
		return res.Content
	}
	f, err := t.tempFileMgr.CreateTempFile()
	if err != nil {
		return res.Content
	}
	_, _ = f.WriteString(text)
	path := f.Name()
	_ = f.Close()
	var sb strings.Builder
	sb.WriteString("───────────────────────────────────────────────────────\n")
	sb.WriteString("⚠️  MCP RESULT TRUNCATED\n")
	sb.WriteString(fmt.Sprintf("   Full result: %d lines, %d bytes\n", res.TotalLines, res.TotalBytes))
	sb.WriteString(fmt.Sprintf("   Complete result saved to: %s\n", path))
	sb.WriteString("   Use Read to investigate the full result\n")
	sb.WriteString("───────────────────────────────────────────────────────\n\n")
	sb.WriteString(res.Content)
	return sb.String()
}

// renderContent concatenates text blocks and summarizes non-text blocks as
// compact placeholders rather than dumping bytes into the context window.
func renderContent(blocks []ContentBlock) (text string, notes []string) {
	var texts []string
	for _, b := range blocks {
		switch b.Type {
		case "text":
			texts = append(texts, b.Text)
		case "image":
			notes = append(notes, fmt.Sprintf("[image %s %s]", b.MimeType, sizeOfBase64(b.Data)))
		case "audio":
			notes = append(notes, fmt.Sprintf("[audio %s %s]", b.MimeType, sizeOfBase64(b.Data)))
		case "resource_link":
			notes = append(notes, fmt.Sprintf("[resource_link uri=%s]", b.URI))
		case "resource":
			uri, _ := b.Resource["uri"].(string)
			// Inline small embedded text resources; summarize others.
			if rtext, ok := b.Resource["text"].(string); ok && len(rtext) < 4096 {
				texts = append(texts, rtext)
			} else {
				notes = append(notes, fmt.Sprintf("[resource uri=%s]", uri))
			}
		default:
			if b.Text != "" {
				texts = append(texts, b.Text)
			} else {
				notes = append(notes, fmt.Sprintf("[%s block]", b.Type))
			}
		}
	}
	return strings.Join(texts, "\n"), notes
}

// sizeOfBase64 estimates the decoded byte size of a base64 payload for display.
func sizeOfBase64(data string) string {
	n := len(data) * 3 / 4
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%dKB", n/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// argsPreview renders a short, single-line preview of call arguments for the
// confirmation prompt.
func argsPreview(args json.RawMessage) string {
	s := strings.TrimSpace(string(args))
	if s == "" || s == "{}" || s == "null" {
		return ""
	}
	const max = 200
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "…"
	}
	return s
}

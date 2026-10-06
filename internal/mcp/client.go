// Package mcp implements a Model Context Protocol (MCP) client for kvit-coder.
//
// MCP is an open client–server protocol (JSON-RPC 2.0) that lets an external
// process expose tools to an agent. This package connects to configured MCP
// servers (over stdio or HTTP), discovers the tools each one advertises, and
// wraps every remote tool in an adapter (MCPTool) that satisfies tools.Tool.
// Once registered, an MCP tool is indistinguishable from a built-in tool to the
// rest of the system: the registry's Specs(), the agent loop, loop detection,
// and the text-fallback parser all treat it uniformly.
//
// The architectural boundary is the Client interface below: transports (stdio,
// http) implement it, and nothing outside this package depends on the concrete
// transport types or on a third-party SDK. This makes the protocol
// implementation swappable — an official Go SDK could back Client later without
// touching internal/tools or the agent loop.
//
// Dependency decision: this package ships a small in-house JSON-RPC client
// rather than pulling in the official MCP Go SDK, to preserve kvit-coder's
// deliberately small dependency tree (no network fetch at build time, no large
// transitive deps). The stdio transport is a few hundred lines; the Client
// interface keeps the choice reversible.
package mcp

import (
	"context"
	"strings"
)

// Protocol version this client implements and offers during initialize. Servers
// negotiate down to a version they support; a different negotiated version is
// logged but not treated as fatal, since servers are generally backward
// compatible.
const ProtocolVersion = "2025-06-18"

// Client identity reported to servers in the initialize handshake.
const (
	clientName    = "kvit-coder"
	clientVersion = "0.1"
)

// Namespacing: MCP tools are registered as "mcp" + Separator + server +
// Separator + tool, e.g. "mcp.filesystem.read_file". The dotted form matches
// the convention already used for Shell.advanced and is
// rendered correctly by the agent's generic tool-call display. If a target
// model rejects dots in function names, change Separator to "__"
// ("mcp__filesystem__read_file", the convention Claude Code uses) here — it is
// the single source of truth.
const (
	NamePrefix = "mcp"
	Separator  = "."
)

// NamespacedName builds the model-facing tool name for a server's tool.
func NamespacedName(server, tool string) string {
	return NamePrefix + Separator + server + Separator + tool
}

// IsToolName reports whether a tool name belongs to the MCP namespace. The agent
// loop uses this to exempt MCP tools from its blanket per-tool timeout.
func IsToolName(name string) bool {
	return strings.HasPrefix(name, NamePrefix+Separator)
}

// ToolDescriptor describes one tool advertised by an MCP server (the shape of a
// tools/list entry). InputSchema is JSON Schema, passed through to the model
// almost verbatim.
type ToolDescriptor struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

// ContentBlock is one typed block of a tools/call result. Only the fields
// relevant to the block's Type are populated.
type ContentBlock struct {
	Type     string         `json:"type"` // "text" | "image" | "audio" | "resource" | "resource_link"
	Text     string         `json:"text,omitempty"`
	Data     string         `json:"data,omitempty"`     // base64 for image/audio
	MimeType string         `json:"mimeType,omitempty"` // image/audio/resource
	URI      string         `json:"uri,omitempty"`      // resource_link
	Resource map[string]any `json:"resource,omitempty"` // embedded resource (uri, mimeType, text/blob)
}

// CallResult is the outcome of a tools/call.
type CallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError"`
}

// Client is one connection to a single MCP server. Implementations are the
// stdio and HTTP transports. Methods must be safe for the agent's single-threaded
// use; transports additionally run a background reader for notifications.
type Client interface {
	// Initialize performs the initialize → notifications/initialized handshake.
	Initialize(ctx context.Context) error
	// Instructions returns the guidance the server sent with its initialize
	// result, or "" when it sent none or Initialize has not run.
	Instructions() string
	// ListTools returns every tool the server advertises (handling pagination).
	ListTools(ctx context.Context) ([]ToolDescriptor, error)
	// CallTool invokes a tool by its raw (server-side) name with the given
	// arguments (a JSON object), returning the typed result.
	CallTool(ctx context.Context, name string, args map[string]any) (*CallResult, error)
	// Close shuts the connection down and releases resources (reaps the
	// subprocess for stdio, closes idle connections for HTTP).
	Close() error
}

// Logger is the minimal logging surface the package needs, satisfied by the
// UI writer. Kept as a local interface to avoid importing internal/ui.
type Logger interface {
	Debug(msg string)
	Warn(msg string)
}

// nopLogger is used when no logger is supplied.
type nopLogger struct{}

func (nopLogger) Debug(string) {}
func (nopLogger) Warn(string)  {}

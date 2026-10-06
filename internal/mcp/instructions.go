package mcp

import (
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// An MCP server may send instructions with its initialize result: how its
// tools fit together, what to call first, what to avoid. The specification
// leaves it to the client whether the model sees them. kvit-coder passes them
// on, as Claude Code does, in one of two places depending on when the server
// is dialed.
//
// A server whose tools the model sees directly is dialed at the start of every
// turn, so its instructions are current. They go in the system prompt
// (PromptSection), in server-name order like the tools, so the prompt stays
// byte-identical from one request to the next.
//
// A server behind a tool group is not dialed until one of its tools runs. Its
// instructions are kept in the on-disk cache beside its tool list, rewritten
// whenever it is dialed, and the group shows them when it is opened
// (ToolsForServers). They are as recent as the last turn that ran one of the
// server's tools. They go in that tool result rather than the system prompt
// for the reason the tools do: the result is appended where the prompt cache
// absorbs it.

// PromptSection is the system-prompt section holding the instructions of the
// servers connected at startup, or "" when none sent any. A server whose tools
// are all filtered out is left out, since its instructions would describe
// tools the model cannot run.
func (m *Manager) PromptSection() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	var sb strings.Builder
	for _, sv := range m.servers {
		if sv.instructions == "" || !anyToolAllowed(sv.descs, sv.filter) {
			continue
		}
		if sb.Len() == 0 {
			sb.WriteString("## Instructions from MCP servers\n\n")
			sb.WriteString("These MCP servers sent instructions for using their tools:\n")
		}
		fmt.Fprintf(&sb, "\n### %s\n\n%s\n", sv.name, sv.instructions)
	}
	return strings.TrimRight(sb.String(), "\n")
}

// anyToolAllowed reports whether the server's filter lets at least one of its
// tools through.
func anyToolAllowed(descs []ToolDescriptor, filter config.MCPToolFilter) bool {
	for _, d := range descs {
		if allowTool(d.Name, filter) {
			return true
		}
	}
	return false
}

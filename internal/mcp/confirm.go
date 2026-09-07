package mcp

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"sync"
)

// Confirmation policy. MCP tools run in a separate process and can touch
// anything that process can reach, so workspace path safety does not apply to
// them. The trust model is configured by mcp.confirm (overridable per server):
//
//	block       — refuse every call (load a server for inspection only)
//	ask_once    — prompt once per distinct tool name, remember for the session
//	ask_always  — prompt on every call
//	trust       — never prompt
//
// In a headless/benchmark run with no controlling terminal, ask_* falls back to
// block rather than hanging.
const (
	ConfirmBlock     = "block"
	ConfirmAskOnce   = "ask_once"
	ConfirmAskAlways = "ask_always"
	ConfirmTrust     = "trust"
)

// confirmer enforces the policy and remembers per-session approvals. One
// instance is shared by all adapters (owned by the Manager) so "ask_once"
// spans every tool from every server.
type confirmer struct {
	mu       sync.Mutex
	approved map[string]bool
}

func newConfirmer() *confirmer {
	return &confirmer{approved: make(map[string]bool)}
}

// Confirm applies the policy for a single call. It returns nil to allow the
// call or an error (the message the model sees) to refuse it.
func (c *confirmer) Confirm(toolName, server, policy, argsPreview string) error {
	switch policy {
	case ConfirmTrust:
		return nil
	case ConfirmBlock:
		return fmt.Errorf("MCP tool %s is blocked by policy (confirm=block); do not retry", toolName)
	case ConfirmAskAlways:
		return c.ask(toolName, server, argsPreview, false)
	case ConfirmAskOnce:
		fallthrough
	default:
		c.mu.Lock()
		ok := c.approved[toolName]
		c.mu.Unlock()
		if ok {
			return nil
		}
		return c.ask(toolName, server, argsPreview, true)
	}
}

// ask prompts the user on the controlling terminal. With no /dev/tty available
// (headless/CI) it denies the call so the run never blocks.
func (c *confirmer) ask(toolName, server, argsPreview string, remember bool) error {
	tty, err := os.Open("/dev/tty")
	if err != nil {
		return fmt.Errorf("MCP tool %s requires confirmation but no terminal is available; "+
			"set mcp.confirm: trust for trusted servers to run headless", toolName)
	}
	defer tty.Close()

	fmt.Fprintf(os.Stderr, "\n")
	fmt.Fprintf(os.Stderr, "⚠️  The model wants to call an external MCP tool:\n")
	fmt.Fprintf(os.Stderr, "   tool:   %s (server: %s)\n", toolName, server)
	if argsPreview != "" {
		fmt.Fprintf(os.Stderr, "   args:   %s\n", argsPreview)
	}
	fmt.Fprintf(os.Stderr, "   This runs outside the workspace and may have side effects.\n")
	fmt.Fprintf(os.Stderr, "\nAllow this call? [y/N]: ")

	reader := bufio.NewReader(tty)
	line, err := reader.ReadString('\n')
	if err != nil && line == "" {
		return fmt.Errorf("failed to read confirmation for %s: %w", toolName, err)
	}
	resp := strings.ToLower(strings.TrimSpace(line))
	fmt.Fprintf(os.Stderr, "%s\n", resp)

	if resp != "y" && resp != "yes" {
		return fmt.Errorf("user declined MCP tool %s; do not retry", toolName)
	}
	if remember {
		c.mu.Lock()
		c.approved[toolName] = true
		c.mu.Unlock()
	}
	return nil
}

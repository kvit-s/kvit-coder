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

// linePrompter, when set, asks whoever is at the terminal a question and
// returns their answer. cmd/kvit-coder installs one that reads through the
// session inbox, the process's only reader of the terminal. Opening /dev/tty
// here as well would put two reads on one device, and the keystroke goes to
// whichever of them the kernel picks — the other waits forever.
var (
	linePrompterMu sync.Mutex
	linePrompter   func(prompt string) (string, bool)
)

// SetLinePrompter installs the function used to ask the person at the terminal
// a question. Passing nil restores reading the terminal directly.
func SetLinePrompter(fn func(prompt string) (string, bool)) {
	linePrompterMu.Lock()
	defer linePrompterMu.Unlock()
	linePrompter = fn
}

// ask prompts the user through the process's line reader, or on the controlling
// terminal when there is none. With neither available (headless/CI) it denies
// the call so the run never blocks.
func (c *confirmer) ask(toolName, server, argsPreview string, remember bool) error {
	var question strings.Builder
	question.WriteString("\n⚠️  The model wants to call an external MCP tool:\n")
	fmt.Fprintf(&question, "   tool:   %s (server: %s)\n", toolName, server)
	if argsPreview != "" {
		fmt.Fprintf(&question, "   args:   %s\n", argsPreview)
	}
	question.WriteString("   This runs outside the workspace and may have side effects.\n")
	question.WriteString("\nAllow this call? [y/N]: ")

	linePrompterMu.Lock()
	prompt := linePrompter
	linePrompterMu.Unlock()

	var resp string
	if prompt != nil {
		answer, ok := prompt(question.String())
		if !ok {
			return fmt.Errorf("no one answered the confirmation for MCP tool %s; do not retry", toolName)
		}
		resp = strings.ToLower(strings.TrimSpace(answer))
	} else {
		tty, err := os.Open("/dev/tty")
		if err != nil {
			return fmt.Errorf("MCP tool %s requires confirmation but no terminal is available; "+
				"set mcp.confirm: trust for trusted servers to run headless", toolName)
		}
		defer tty.Close()

		fmt.Fprint(os.Stderr, question.String())
		line, err := bufio.NewReader(tty).ReadString('\n')
		if err != nil && line == "" {
			return fmt.Errorf("failed to read confirmation for %s: %w", toolName, err)
		}
		resp = strings.ToLower(strings.TrimSpace(line))
		fmt.Fprintf(os.Stderr, "%s\n", resp)
	}

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

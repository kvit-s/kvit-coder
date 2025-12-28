// Package repl provides the exec mode runner for the agent.
package repl

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// RunExec runs in exec mode with a single prompt
func RunExec(runner *agent.Runner, writer *ui.Writer, cfg *config.Config, systemPrompt string, promptText string, quietMode bool, sessionName string, sessionMgr *session.Manager) {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
	}

	// Determine session name (auto-generate if exec mode without -s)
	isNewSession := true
	if sessionMgr != nil {
		if sessionName == "" {
			// Auto-generate session name for exec mode
			sessionName = sessionMgr.GenerateSessionName()
		} else if sessionMgr.SessionExists(sessionName) {
			// Load existing session
			sessionMessages, err := sessionMgr.LoadSession(sessionName)
			if err != nil {
				writer.Error(fmt.Sprintf("Failed to load session: %v", err))
			} else {
				// Filter out system messages and prepend fresh system prompt
				for _, msg := range sessionMessages {
					if msg.Role != llm.RoleSystem {
						messages = append(messages, msg)
					}
				}
				isNewSession = false
				if !quietMode {
					fmt.Fprintf(os.Stderr, "Continuing session: %s (%d messages)\n\n", sessionName, len(sessionMessages))
				}
			}
		} else {
			if !quietMode {
				fmt.Fprintf(os.Stderr, "Starting new session: %s\n\n", sessionName)
			}
		}
	}

	// Display the prompt (unless in quiet mode)
	if !quietMode {
		colorStart := "\033[97;100m"
		colorEnd := "\033[0m"
		inputLines := strings.Split(promptText, "\n")
		for _, line := range inputLines {
			fmt.Fprintf(os.Stderr, "%s%s%s\n", colorStart, line, colorEnd)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Add user message
	userMsg := llm.Message{
		Role:    llm.RoleUser,
		Content: promptText,
	}
	messages = append(messages, userMsg)

	// Run agent loop
	result, err := runner.Run(context.Background(), agent.RunConfig{
		Messages:     messages,
		UseFileFirst: false,
		QuietMode:    quietMode,
	})
	if err != nil {
		writer.Error(fmt.Sprintf("Agent error: %v", err))
		return
	}

	// Save session
	if sessionMgr != nil && sessionName != "" {
		if err := sessionMgr.SaveSession(sessionName, result.FinalMessages); err != nil {
			writer.Error(fmt.Sprintf("Failed to save session: %v", err))
		}
	}

	// Output JSON or print stats
	if writer.IsJSONMode() {
		writer.WriteJSONOutput(&ui.JSONStats{
			Session:          sessionName,
			PromptTokens:     result.Stats.TotalPromptTokens,
			CompletionTokens: result.Stats.TotalCompletionTokens,
			TotalTokens:      result.Stats.TotalPromptTokens + result.Stats.TotalCompletionTokens,
			CacheReadTokens:  result.Stats.TotalCacheReadTokens,
			TotalCost:        result.Stats.TotalCost,
			CacheDiscount:    result.Stats.CacheDiscount,
			DurationMs:       result.Stats.TotalAgentTime.Milliseconds(),
			Steps:            result.Stats.Steps,
		})
	} else {
		// Print stats to stderr
		result.Stats.PrintTo(os.Stderr)
	}

	// Print session info (skip in JSON mode - it's included in the JSON output)
	if !writer.IsJSONMode() && sessionMgr != nil && sessionName != "" {
		if quietMode {
			// In quiet mode, print minimal session info to stderr
			if isNewSession {
				fmt.Fprintf(os.Stderr, "[new] %s\n", sessionName)
			} else {
				fmt.Fprintf(os.Stderr, "[continued] %s\n", sessionName)
			}
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, strings.Repeat("─", 50))
			fmt.Fprintf(os.Stderr, "Session: %s\n", sessionName)
			fmt.Fprintf(os.Stderr, "Continue with: kvit-coder -p \"your message\" -s %s\n", sessionName)
		}
	}
}

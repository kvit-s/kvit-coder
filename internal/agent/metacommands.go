package agent

import (
	"fmt"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// HandleMetaCommand processes REPL meta-commands like :quit, :config, etc.
// Returns (shouldExit, resetMessages).
// If resetMessages is non-nil, it replaces the current message list.
func HandleMetaCommand(
	input string,
	cfg *config.Config,
	registry *tools.Registry,
	systemPrompt string,
	messages []llm.Message,
) (bool, []llm.Message) {
	switch input {
	case ":quit", ":q":
		fmt.Println("Goodbye!")
		return true, nil

	case ":config":
		fmt.Printf("Model: %s @ %s\n", cfg.LLM.Model, cfg.LLM.BaseURL)
		fmt.Printf("Workspace: %s\n", cfg.Workspace.Root)
		fmt.Printf("Max iterations: %d\n", cfg.Agent.MaxIterations)
		fmt.Printf("Temperature: %.2f\n", cfg.LLM.Temperature)
		if cfg.LLM.MaxTokens > 0 {
			fmt.Printf("Max tokens: %d\n", cfg.LLM.MaxTokens)
		}
		if cfg.LLM.Context > 0 {
			fmt.Printf("Context window: %d\n", cfg.LLM.Context)
		}
		return false, nil

	case ":tools":
		fmt.Println("Available tools:")
		for _, tool := range registry.All() {
			fmt.Printf("  - %s: %s\n", tool.Name(), tool.Description())
		}
		return false, nil

	case ":system":
		fmt.Printf("System prompt:\n%s\n", systemPrompt)
		return false, nil

	case ":clear":
		msgs := []llm.Message{{Role: llm.RoleSystem, Content: systemPrompt}}
		fmt.Println("Conversation cleared")
		return false, msgs

	case ":history":
		fmt.Printf("Conversation: %d messages\n", len(messages))
		for i, msg := range messages {
			role := msg.Role
			preview := msg.Content
			if len(preview) > 60 {
				preview = preview[:57] + "..."
			}
			if msg.Role == llm.RoleTool {
				fmt.Printf("  %d. [%s:%s] %s\n", i, role, msg.Name, preview)
			} else if len(msg.ToolCalls) > 0 {
				fmt.Printf("  %d. [%s] %d tool calls\n", i, role, len(msg.ToolCalls))
			} else {
				fmt.Printf("  %d. [%s] %s\n", i, role, preview)
			}
		}
		return false, nil

	case ":help", ":h":
		fmt.Println("Available commands:")
		fmt.Println("  :quit, :q      - Exit the REPL")
		fmt.Println("  :config        - Show current configuration")
		fmt.Println("  :tools         - List available tools")
		fmt.Println("  :system        - Show system prompt")
		fmt.Println("  :clear         - Clear conversation history")
		fmt.Println("  :history       - Show conversation history")
		fmt.Println("  :help, :h      - Show this help message")
		fmt.Println("\nREPL features:")
		fmt.Println("  - Use Up/Down arrows to navigate history")
		fmt.Println("  - Press ESC to cancel ongoing LLM requests")
		fmt.Println("  - Press Ctrl+C or Ctrl+D to exit")
		return false, nil

	default:
		fmt.Printf("Unknown command: %s\n", input)
		fmt.Println("Type :help for available commands")
		return false, nil
	}
}

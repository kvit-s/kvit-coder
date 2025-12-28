// Package tui provides the interactive terminal UI for kvit-coder-ui.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// Options contains configuration for the UI
type Options struct {
	AgentPath   string
	ConfigPath  string
	SessionName string
	SessionMgr  *session.Manager
	Config      *config.Config
}

// UI manages the interactive terminal interface
type UI struct {
	agentPath      string
	configPath     string
	currentSession string
	sessionMgr     *session.Manager
	cfg            *config.Config
	history        []string
	historyFile    string
}

// New creates a new UI instance
func New(opts Options) *UI {
	homeDir, _ := os.UserHomeDir()
	historyFile := filepath.Join(homeDir, ".kvit-coder-history")

	// Load history
	history, _ := ui.LoadHistory(historyFile)

	return &UI{
		agentPath:      opts.AgentPath,
		configPath:     opts.ConfigPath,
		currentSession: opts.SessionName,
		sessionMgr:     opts.SessionMgr,
		cfg:            opts.Config,
		history:        history,
		historyFile:    historyFile,
	}
}

// Run starts the interactive UI loop
func (u *UI) Run() error {
	// Ensure terminal is properly reset on exit
	restoreTerminal := func() {
		if os.Stdin.Fd() == 0 {
			cmd := exec.Command("sh", "-c", "stty sane </dev/tty >/dev/tty 2>&1")
			_ = cmd.Run()
		}
	}
	defer func() {
		fmt.Println()
		restoreTerminal()
	}()

	// Show startup info
	fmt.Println("\033[38;5;136mAgent REPL UI v0.1\033[0m")
	fmt.Printf("\033[38;5;136mModel: %s @ %s\033[0m\n", u.cfg.LLM.Model, u.cfg.LLM.BaseURL)
	if u.currentSession != "" {
		if u.sessionMgr.SessionExists(u.currentSession) {
			fmt.Printf("\033[38;5;136mSession: %s (continuing)\033[0m\n", u.currentSession)
		} else {
			fmt.Printf("\033[38;5;136mSession: %s (new)\033[0m\n", u.currentSession)
		}
	}
	fmt.Println("\033[38;5;136mPress Ctrl+C to exit, ':help' for commands\033[0m")
	fmt.Println()

	for {
		input, shouldExit, err := u.readInput()
		if err != nil {
			fmt.Printf("\033[31m[error] Input error: %v\033[0m\n", err)
			break
		}
		if shouldExit {
			break
		}

		if input == "" {
			continue
		}

		// Add to history and save
		u.history = append(u.history, input)
		_ = ui.SaveHistory(u.historyFile, u.history) // Silently ignore history save errors

		// Handle UI commands
		if strings.HasPrefix(input, ":") {
			shouldExit := u.handleCommand(input)
			if shouldExit {
				break
			}
			continue
		}

		// Run agent with the prompt
		u.runAgent(input)
	}

	return nil
}

// readInput reads user input using the BubbleTea-based input model
func (u *UI) readInput() (string, bool, error) {
	promptText := u.buildPromptText()

	// Create and run input model
	inputModel := ui.NewInputModel(promptText, u.history)
	p := tea.NewProgram(inputModel)
	result, err := p.Run()

	if err != nil {
		return "", false, err
	}

	// Get the result
	finalModel := result.(ui.InputModel)
	if finalModel.Cancelled() || !finalModel.Submitted() {
		return "", true, nil // User cancelled (Ctrl+C)
	}

	input := strings.TrimSpace(finalModel.Value())

	// Display the submitted input with gray background
	colorStart := "\033[97;100m"
	colorEnd := "\033[0m"

	inputLines := strings.Split(finalModel.Value(), "\n")
	for _, line := range inputLines {
		fmt.Printf("%s%s%s\n", colorStart, line, colorEnd)
	}
	fmt.Println()

	return input, false, nil
}

// buildPromptText builds the prompt text with session info
func (u *UI) buildPromptText() string {
	var sessionInfo string
	if u.currentSession != "" {
		sessionInfo = fmt.Sprintf(" %s", u.currentSession)
	}
	return ui.MakePrompt(fmt.Sprintf("[ui%s]> ", sessionInfo))
}

// handleCommand handles UI meta-commands
func (u *UI) handleCommand(input string) bool {
	cmd := strings.TrimPrefix(input, ":")
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return false
	}

	switch parts[0] {
	case "quit", "q", "exit":
		return true

	case "help", "h":
		u.showHelp()

	case "new":
		// Start a new session
		u.currentSession = u.sessionMgr.GenerateSessionName()
		fmt.Printf("Started new session: %s\n\n", u.currentSession)

	case "switch":
		if len(parts) < 2 {
			fmt.Println("Usage: :switch <session-name>")
			fmt.Println()
			return false
		}
		sessionName := parts[1]
		if !u.sessionMgr.SessionExists(sessionName) {
			fmt.Printf("Session %q not found. Use :sessions to list available sessions.\n\n", sessionName)
			return false
		}
		u.currentSession = sessionName
		fmt.Printf("Switched to session: %s\n\n", sessionName)

	case "sessions":
		sessions, err := u.sessionMgr.ListSessions()
		if err != nil {
			fmt.Printf("Error listing sessions: %v\n\n", err)
			return false
		}
		if len(sessions) == 0 {
			fmt.Println("No sessions found.")
		} else {
			fmt.Printf("%-30s  %-20s  %s\n", "NAME", "MODIFIED", "MESSAGES")
			fmt.Println("────────────────────────────────────────────────────────────")
			for _, s := range sessions {
				marker := ""
				if s.Name == u.currentSession {
					marker = " *"
				}
				fmt.Printf("%-30s  %-20s  %d%s\n", s.Name, s.ModTime.Format("2006-01-02 15:04"), s.MessageCount, marker)
			}
		}
		fmt.Println()

	case "history":
		if u.currentSession == "" {
			fmt.Println("No active session. Use :new or :switch to select a session.")
			fmt.Println()
			return false
		}
		content, err := u.sessionMgr.ShowSession(u.currentSession)
		if err != nil {
			fmt.Printf("Error showing session: %v\n\n", err)
			return false
		}
		fmt.Print(content)
		fmt.Println()

	case "clear":
		// Clear terminal
		fmt.Print("\033[2J\033[H")

	case "config":
		fmt.Printf("Config: %s\n", u.configPath)
		fmt.Printf("Model: %s\n", u.cfg.LLM.Model)
		fmt.Printf("Base URL: %s\n", u.cfg.LLM.BaseURL)
		fmt.Printf("Agent: %s\n", u.agentPath)
		if u.currentSession != "" {
			fmt.Printf("Session: %s\n", u.currentSession)
		}
		fmt.Println()

	default:
		fmt.Printf("Unknown command: %s. Type :help for available commands.\n\n", parts[0])
	}

	return false
}

// showHelp displays the help message
func (u *UI) showHelp() {
	fmt.Println("Available commands:")
	fmt.Println("  :help, :h        Show this help message")
	fmt.Println("  :quit, :q        Exit the UI")
	fmt.Println("  :new             Start a new session")
	fmt.Println("  :switch <name>   Switch to an existing session")
	fmt.Println("  :sessions        List all sessions")
	fmt.Println("  :history         Show current session history")
	fmt.Println("  :clear           Clear the terminal")
	fmt.Println("  :config          Show configuration")
	fmt.Println()
	fmt.Println("Enter any other text to send as a prompt to the agent.")
	fmt.Println()
}

// runAgent spawns kvit-coder with the given prompt
func (u *UI) runAgent(prompt string) {
	args := []string{"-p", prompt}

	// Pass config file
	if u.configPath != "" && u.configPath != "config.yaml" {
		args = append(args, "-config", u.configPath)
	}

	// Pass session if set
	if u.currentSession != "" {
		args = append(args, "-s", u.currentSession)
	}

	// Create command
	cmd := exec.Command(u.agentPath, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil // No stdin for the agent

	// Run and wait for completion
	if err := cmd.Run(); err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 2 {
				// User interrupt (SIGINT)
				fmt.Println("[cancelled]")
			}
		} else {
			fmt.Printf("\033[31m[error] Agent failed: %v\033[0m\n", err)
		}
	}

	fmt.Println()
}

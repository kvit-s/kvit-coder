package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/tui"
)

// Version info set by ldflags at build time
var (
	version    = "dev"
	commitHash = "dev"
	commitDate = "unknown"
)

func main() {
	// Parse flags
	configPath := flag.String("config", "config.yaml", "path to config file")
	agentPath := flag.String("agent-path", "", "path to kvit-coder binary (auto-detected if not specified)")
	sessionName := flag.String("s", "", "session name: continue existing session or create new one")
	showVersion := flag.Bool("version", false, "show version information and exit")

	// Session management flags (pass-through to kvit-coder)
	sessionList := flag.Bool("sessions", false, "list all sessions and exit")
	sessionDelete := flag.String("session-delete", "", "delete a session and exit")
	sessionShow := flag.String("session-show", "", "show session history and exit")

	flag.Parse()

	// Handle --version
	if *showVersion {
		fmt.Printf("%s-%s-%s\n", version, commitDate, commitHash)
		return
	}

	// Handle session management commands (delegate to session manager)
	if *sessionList || *sessionDelete != "" || *sessionShow != "" {
		sessionMgr, err := session.NewManager()
		if err != nil {
			log.Fatalf("Failed to create session manager: %v", err)
		}

		if *sessionList {
			sessions, err := sessionMgr.ListSessions()
			if err != nil {
				log.Fatalf("Failed to list sessions: %v", err)
			}
			if len(sessions) == 0 {
				fmt.Println("No sessions found.")
			} else {
				fmt.Printf("%-30s  %-20s  %s\n", "NAME", "MODIFIED", "MESSAGES")
				fmt.Println("────────────────────────────────────────────────────────────")
				for _, s := range sessions {
					fmt.Printf("%-30s  %-20s  %d\n", s.Name, s.ModTime.Format("2006-01-02 15:04"), s.MessageCount)
				}
			}
			return
		}

		if *sessionDelete != "" {
			if !sessionMgr.SessionExists(*sessionDelete) {
				log.Fatalf("Session %q not found", *sessionDelete)
			}
			if err := sessionMgr.DeleteSession(*sessionDelete); err != nil {
				log.Fatalf("Failed to delete session: %v", err)
			}
			fmt.Printf("Deleted session: %s\n", *sessionDelete)
			return
		}

		if *sessionShow != "" {
			if !sessionMgr.SessionExists(*sessionShow) {
				log.Fatalf("Session %q not found", *sessionShow)
			}
			content, err := sessionMgr.ShowSession(*sessionShow)
			if err != nil {
				log.Fatalf("Failed to show session: %v", err)
			}
			fmt.Print(content)
			return
		}
	}

	// Load config to display info
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Auto-detect kvit-coder path if not specified
	agentBinary := *agentPath
	if agentBinary == "" {
		// Try to find kvit-coder in same directory as this binary
		execPath, err := os.Executable()
		if err == nil {
			dir := filepath.Dir(execPath)
			candidate := filepath.Join(dir, "kvit-coder")
			if _, err := os.Stat(candidate); err == nil {
				agentBinary = candidate
			}
		}
		// Fallback to PATH lookup
		if agentBinary == "" {
			agentBinary = "kvit-coder"
		}
	}

	// Initialize session manager
	sessionMgr, err := session.NewManager()
	if err != nil {
		log.Fatalf("Failed to create session manager: %v", err)
	}

	// Create and run UI
	ui := tui.New(tui.Options{
		AgentPath:   agentBinary,
		ConfigPath:  *configPath,
		SessionName: *sessionName,
		SessionMgr:  sessionMgr,
		Config:      cfg,
	})

	if err := ui.Run(); err != nil {
		log.Fatalf("UI error: %v", err)
	}
}

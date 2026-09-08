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
	configPath := flag.String("config", "", "path to config file (default: the first of $KVIT_CODER_CONFIG, ./config.yaml, ~/.kvit-coder/config.yaml, config.yaml beside the binary)")
	agentPath := flag.String("agent-path", "", "path to kvit-coder binary (auto-detected if not specified)")
	sessionName := flag.String("s", "", "session name: continue existing session or create new one")
	continueLast := flag.Bool("c", false, "continue the most recent session")
	modelFlag := flag.String("model", "", "start on this model: :mN, index, id, name or wire id, with optional :effort (e.g. -m m3:xhigh)")
	modelShort := flag.String("m", "", "shorthand for --model")
	effortFlag := flag.String("effort", "", "startup reasoning effort for the selected model (overrides the :effort suffix)")
	effortShort := flag.String("e", "", "shorthand for --effort")
	yolo := flag.Bool("yolo", false, "read and write anywhere on the filesystem, without asking")
	showVersion := flag.Bool("version", false, "show version information and exit")

	// Session management flags (pass-through to kvit-coder)
	sessionList := flag.Bool("sessions", false, "list all sessions and exit")
	sessionDelete := flag.String("session-delete", "", "delete a session and exit")
	sessionShow := flag.String("session-show", "", "show session history and exit")

	flag.Parse()

	// Handle --version
	if *showVersion {
		fmt.Printf("%s-%s\n", commitDate, commitHash)
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

	// With no -config, look for the file in the usual places rather than only
	// in the directory the UI was started from, so it runs from anywhere. The
	// path is made absolute because every turn's agent process is given it.
	resolvedConfig, note, err := config.ResolvePath(*configPath)
	if note != "" {
		fmt.Fprintln(os.Stderr, note)
	}
	if err != nil {
		log.Fatalf("%v", err)
	}
	if abs, err := filepath.Abs(resolvedConfig); err == nil {
		resolvedConfig = abs
	}

	// Load config to display info
	cfg, err := config.Load(resolvedConfig)
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

	// Without a session name the UI spawns every turn with no -s, so nothing
	// the agent learns survives to the next prompt. Name the session up front.
	currentSession := *sessionName
	if currentSession == "" && *continueLast {
		recent, err := sessionMgr.MostRecent()
		if err != nil {
			log.Fatalf("Failed to find the most recent session: %v", err)
		}
		if recent == "" {
			fmt.Fprintln(os.Stderr, "No previous session to continue; starting a new one.")
		}
		currentSession = recent
	}
	if currentSession == "" {
		currentSession = sessionMgr.GenerateSessionName()
	}

	// Startup model selection (-m/--model, -e/--effort). The ref may carry
	// an inline ":effort" ("m3:xhigh"); an explicit -e wins over it. A bad
	// ref fails here, before the UI opens, with what is offered — the same
	// resolver the :mN/:eN commands and headless --model share.
	modelRef := *modelFlag
	if modelRef == "" {
		modelRef = *modelShort
	}
	effortRef := *effortFlag
	if effortRef == "" {
		effortRef = *effortShort
	}
	var initialModelSet, initialEffortSet bool
	var initialModel int
	var initialEffort string
	if modelRef != "" || effortRef != "" {
		_, inlineEff := config.SplitModelEffort(modelRef)
		idx, eff, err := cfg.ResolveSelection(modelRef, effortRef, effortRef != "")
		if err != nil {
			log.Fatalf("Failed to select model: %v", err)
		}
		initialModelSet, initialModel = true, idx
		if effortRef != "" || inlineEff != "" {
			initialEffortSet, initialEffort = true, eff
		}
	}

	// Create and run UI
	ui := tui.New(tui.Options{
		AgentPath:        agentBinary,
		ConfigPath:       resolvedConfig,
		SessionName:      currentSession,
		SessionMgr:       sessionMgr,
		Config:           cfg,
		Yolo:             *yolo,
		InitialModelSet:  initialModelSet,
		InitialModel:     initialModel,
		InitialEffortSet: initialEffortSet,
		InitialEffort:    initialEffort,
	})

	if err := ui.Run(); err != nil {
		log.Fatalf("UI error: %v", err)
	}
}

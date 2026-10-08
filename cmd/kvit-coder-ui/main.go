package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	kvitcoder "github.com/kvit-s/kvit-coder"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/tui"
	"github.com/kvit-s/kvit-coder/internal/update"
)

// Version info set by ldflags at build time. buildDate is stamped by
// goreleaser's ldflags like the rest; it is unused here but must exist for
// the link to succeed.
var (
	version    = "dev"
	commitHash = "dev"
	commitDate = "unknown"
	buildDate  = "unknown"
)

var _ = buildDate

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
	structured := flag.Bool("structured", true, "end each turn with a structured report, shown as a card at the prompt (--structured=false for prose)")
	showVersion := flag.Bool("version", false, "show version information and exit")
	openSetup := flag.Bool("setup", false, "open :setup before the first prompt, to add models")

	// Session management flags (pass-through to kvit-coder)
	sessionList := flag.Bool("sessions", false, "list all sessions and exit")
	sessionDelete := flag.String("session-delete", "", "delete a session and exit")
	sessionShow := flag.String("session-show", "", "show session history and exit")

	flag.Parse()

	// Handle --version
	if *showVersion {
		fmt.Printf("kvit-coder-ui %s (commit %s of %s)\n", version, commitHash, commitDate)
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
				fmt.Printf("%-30s  %-20s  %-8s  %s\n", "NAME", "MODIFIED", "MESSAGES", "TITLE")
				fmt.Println("────────────────────────────────────────────────────────────────────────────────")
				for _, s := range sessions {
					fmt.Printf("%-30s  %-20s  %-8d  %s\n", s.Name, s.ModTime.Format("2006-01-02 15:04"), s.MessageCount, s.Title)
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
	if err != nil && *configPath == "" && os.Getenv(config.ConfigPathEnv) == "" {
		// Nothing to start from. Rather than stop at the first run, write
		// the example and carry on; it names no model, so :setup opens next.
		resolvedConfig, err = offerExampleConfig(err)
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
		// Try to find kvit-coder in same directory as this binary. On
		// Windows the sibling is kvit-coder.exe, so both names are tried;
		// without this the lookup falls through to the bare name below,
		// which Go refuses to run from the current directory (ErrDot).
		if execPath, err := os.Executable(); err == nil {
			dir := filepath.Dir(execPath)
			names := []string{"kvit-coder"}
			if runtime.GOOS == "windows" {
				names = []string{"kvit-coder.exe", "kvit-coder"}
			}
			for _, name := range names {
				candidate := filepath.Join(dir, name)
				if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
					agentBinary = candidate
					break
				}
			}
		}
		// Fallback to PATH lookup so a clear "not found" surfaces when
		// neither the flag nor a sibling binary says where the agent is.
		// A bare name is kept only as a last resort: on Windows it
		// carries the .exe suffix Go's LookPath needs.
		if agentBinary == "" {
			if resolved, err := exec.LookPath("kvit-coder"); err == nil {
				agentBinary = resolved
			} else if runtime.GOOS == "windows" {
				agentBinary = "kvit-coder.exe"
			} else {
				agentBinary = "kvit-coder"
			}
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

	// A previous Windows :update leaves the old binaries as .old after the
	// swap; remove them now that nothing holds them.
	update.CleanupForPaths(agentBinary)

	// Create and run UI
	ui := tui.New(tui.Options{
		Version:          version,
		CommitHash:       commitHash,
		CommitDate:       commitDate,
		AgentPath:        agentBinary,
		ConfigPath:       resolvedConfig,
		SessionName:      currentSession,
		SessionMgr:       sessionMgr,
		Config:           cfg,
		Yolo:             *yolo,
		Structured:       *structured,
		InitialModelSet:  initialModelSet,
		InitialModel:     initialModel,
		InitialEffortSet: initialEffortSet,
		InitialEffort:    initialEffort,
		StatePath:        statePath(),
		OpenSetup:        *openSetup,
	})

	if err := ui.Run(); err != nil {
		log.Fatalf("UI error: %v", err)
	}
}

// statePath is ~/.kvit-coder/kcu-state.json, or "" without a home directory.
func statePath() string {
	dir, err := config.UserDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, tui.StateName)
}

// offerExampleConfig asks whether to write the example configuration to
// ~/.kvit-coder/config.yaml when no configuration was found, and returns its
// path when it was written. notFound is the search's own error, printed
// before the question.
func offerExampleConfig(notFound error) (string, error) {
	dir, err := config.UserDir()
	if err != nil {
		return "", notFound
	}
	path := filepath.Join(dir, config.DefaultConfigName)
	fmt.Fprintln(os.Stderr, notFound)
	fmt.Fprintf(os.Stderr, "Write a starting configuration to %s? [Y/n] ", path)
	answer, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "" && answer != "y" && answer != "yes" {
		return "", fmt.Errorf("no configuration was written; name one with -config, or start kvit-coder-ui again and answer y")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, kvitcoder.ExampleConfig, 0o644); err != nil {
		return "", err
	}
	fmt.Fprintf(os.Stderr, "Wrote %s.\n", path)
	return path, nil
}

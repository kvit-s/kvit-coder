package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/benchmark"
	"github.com/kvit-s/kvit-coder/internal/checkpoint"
	"github.com/kvit-s/kvit-coder/internal/config"
	ctxtools "github.com/kvit-s/kvit-coder/internal/context"
	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/mcp"
	"github.com/kvit-s/kvit-coder/internal/permissions"
	"github.com/kvit-s/kvit-coder/internal/procs"
	"github.com/kvit-s/kvit-coder/internal/prompt"
	"github.com/kvit-s/kvit-coder/internal/repl"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
	"github.com/kvit-s/kvit-coder/internal/workspace"
)

// Version info set by ldflags at build time
var (
	version    = "dev"
	commitHash = "dev"
	commitDate = "unknown"
	buildDate  = "unknown"
)

func main() {
	// Subcommands are intercepted before flag.Parse, which has no notion of them.
	if len(os.Args) > 1 && os.Args[1] == "steer" {
		os.Exit(runSteer(os.Args[2:]))
	}

	// Parse flags
	configPath := flag.String("config", "config.yaml", "path to config file")
	model := flag.String("model", "", "override model name")
	baseURL := flag.String("base-url", "", "override LLM base URL")
	logFile := flag.String("log", "kvit-coder.log", "log file path (empty to disable)")
	execPrompt := flag.String("p", "", "exec mode: run with this prompt and exit after completion")
	quietPrompt := flag.String("pq", "", "quiet exec mode: run with this prompt and only print final LLM response")
	jsonOutput := flag.Bool("json", false, "output structured JSON messages to stderr")
	showVersion := flag.Bool("version", false, "show version information and exit")

	// Benchmark flags
	benchmarkMode := flag.String("benchmark", "", "run benchmark mode (optional suffix, e.g., 'x5' uses config-x5.yaml)")
	benchmarkRuns := flag.Int("n", 10, "number of runs per benchmark")
	benchmarkCategory := flag.String("benchmark-category", "", "filter benchmarks by category (comma-separated)")
	benchmarkID := flag.String("benchmark-id", "", "run specific benchmark IDs (comma-separated)")
	benchmarkOutput := flag.String("o", "", "benchmark output file (default: .kvit-coder-benchmark/results/benchmark-{timestamp}.md)")
	benchmarkNoResume := flag.Bool("no-resume", false, "force fresh benchmark start (ignore existing results)")
	benchmarkList := flag.Bool("benchmark-list", false, "list available benchmarks and exit")

	// Haystack benchmark flags (needle retrieval - large context tests)
	benchHaystack := flag.String("bench-haystack", "", "run haystack (needle retrieval) benchmarks (optional suffix)")
	benchHaystackID := flag.String("bench-haystack-id", "", "run specific haystack benchmark IDs (comma-separated)")

	// Thinkbench benchmark flags (autonomous coding agent tasks, held-out grader)
	benchThinkbench := flag.String("bench-thinkbench", "", "run thinkbench (coding agent) benchmarks (optional suffix)")
	benchThinkbenchID := flag.String("bench-thinkbench-id", "", "run specific thinkbench task slugs (comma-separated)")
	benchThinkbenchTypes := flag.String("bench-thinkbench-types", "", "filter thinkbench tasks by type (comma-separated)")
	benchThinkbenchResume := flag.Bool("bench-thinkbench-resume", false, "resume the most recent interrupted thinkbench run (default: start fresh)")
	thinkbenchSuite := flag.String("thinkbench-suite", "", "override thinkbench suite directory")

	// Session flags
	sessionName := flag.String("s", "", "session name: continue existing session or create new one with this name")
	sessionList := flag.Bool("sessions", false, "list all sessions and exit")
	sessionDelete := flag.String("session-delete", "", "delete a session and exit")
	sessionShow := flag.String("session-show", "", "show session history and exit")
	agentFile := flag.String("agent-file", "", "path to agent file (content appended to system prompt)")

	flag.Parse()

	launchDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get launch directory: %v", err)
	}

	// Handle --version
	if *showVersion {
		fmt.Printf("%s-%s\n", commitDate, commitHash)
		return
	}

	// Handle session management commands early (before config load)
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
				fmt.Println(strings.Repeat("-", 60))
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

	// Get version string for benchmark reports
	version := fmt.Sprintf("kvit-coder %s (commit %s, built %s)", commitHash, commitDate, buildDate)

	// Determine exec mode and quiet mode
	var execMode bool
	var promptText string
	var quietMode bool

	if *quietPrompt != "" {
		execMode = true
		promptText = *quietPrompt
		quietMode = true
	} else if *execPrompt != "" {
		execMode = true
		promptText = *execPrompt
		quietMode = false
	}

	// Initialize UI writer (verbose level set after config load)
	writer := ui.NewWriter(0)
	if quietMode {
		writer.SetQuiet(true)
	}
	if *jsonOutput {
		writer.SetJSONMode(true)
	}
	// Enable headless mode for exec mode (progress to stderr, final answer to stdout)
	if execMode {
		writer.SetHeadless(true)
	}

	// Initialize logger
	logger, err := agent.NewLogger(*logFile, false)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Close()

	// Determine config path - if benchmark mode has a suffix, use config-{suffix}.yaml
	// Use "." for no suffix (plain benchmark mode)
	actualConfigPath := *configPath
	benchmarkEnabled := *benchmarkMode != ""
	benchmarkSuffix := *benchmarkMode
	if benchmarkSuffix == "." {
		benchmarkSuffix = "" // "." means no suffix
	}

	// Also handle --bench-haystack suffix
	haystackSuffix := *benchHaystack
	if haystackSuffix == "." || haystackSuffix == "true" {
		haystackSuffix = ""
	}

	// Also handle --bench-thinkbench suffix
	thinkbenchEnabled := *benchThinkbench != ""
	thinkbenchSuffix := *benchThinkbench
	if thinkbenchSuffix == "." || thinkbenchSuffix == "true" {
		thinkbenchSuffix = ""
	}

	// Use config-{suffix}.yaml if suffix is provided (from any benchmark mode)
	if benchmarkSuffix != "" && *configPath == "config.yaml" {
		actualConfigPath = fmt.Sprintf("config-%s.yaml", benchmarkSuffix)
	} else if haystackSuffix != "" && *configPath == "config.yaml" {
		actualConfigPath = fmt.Sprintf("config-%s.yaml", haystackSuffix)
	} else if thinkbenchSuffix != "" && *configPath == "config.yaml" {
		actualConfigPath = fmt.Sprintf("config-%s.yaml", thinkbenchSuffix)
	}

	// Load config
	cfg, err := config.Load(actualConfigPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Set verbose level from config
	writer.SetVerbose(cfg.LLM.Verbose)

	// Handle --benchmark-list early
	if *benchmarkList {
		if err := benchmark.ListBenchmarks(cfg); err != nil {
			log.Fatalf("Failed to list benchmarks: %v", err)
		}
		return
	}

	// Apply flag overrides
	if *model != "" {
		cfg.LLM.Model = *model
	}
	if *baseURL != "" {
		cfg.LLM.BaseURL = *baseURL
	}
	if *agentFile != "" {
		cfg.Agent.AgentFile = *agentFile
	}

	// Override workspace for benchmark mode - set BEFORE tools are initialized
	// Store original workspace root for finding benchmarks.yaml
	originalWorkspaceRoot := cfg.Workspace.Root
	if benchmarkEnabled {
		baseDir := filepath.Join(cfg.Workspace.Root, ".kvit-coder-benchmark")
		// Use workspace-{suffix} when --benchmark {suffix} is specified to allow parallel runs
		// e.g., --benchmark claude uses workspace-claude, --benchmark gemini uses workspace-gemini
		if *benchmarkMode != "" {
			cfg.Workspace.Root = filepath.Join(baseDir, "workspace-"+*benchmarkMode)
		} else {
			cfg.Workspace.Root = filepath.Join(baseDir, "workspace")
		}
		cfg.Workspace.PathSafetyMode = "block"
		cfg.Workspace.AllowOutsideWorkspace = false

		// Create workspace directory (needed before checkpoint manager initializes)
		if err := os.MkdirAll(cfg.Workspace.Root, 0755); err != nil {
			log.Fatalf("Failed to create benchmark workspace: %v", err)
		}
	}

	// Override workspace for thinkbench mode - hard sandbox, set BEFORE tools init.
	// The run workspace lives under benchmarks/thinkbench/runs/ (gitignored), kept
	// physically separate from the suite (held-out graders/references).
	if thinkbenchEnabled {
		// Anchor the run workspace to the suite location (not cwd) so the harness
		// runs identically from the repo root or from benchmarks/.
		_, tbRoot := benchmark.ResolveThinkbench(originalWorkspaceRoot, *thinkbenchSuite, cfg.Thinkbench.SuiteDir)
		wsName := "workspace"
		if thinkbenchSuffix != "" {
			wsName = "workspace-" + thinkbenchSuffix
		}
		cfg.Workspace.Root = filepath.Join(tbRoot, "runs", wsName)
		cfg.Workspace.PathSafetyMode = "block"
		cfg.Workspace.AllowOutsideWorkspace = false
		cfg.Workspace.AllowedPaths = nil
		cfg.Workspace.AllowedReadPaths = nil
		if err := os.MkdirAll(cfg.Workspace.Root, 0755); err != nil {
			log.Fatalf("Failed to create thinkbench workspace: %v", err)
		}
	}

	// Require -p or a benchmark mode before anything is created on disk.
	// kvit-coder is headless; kvit-coder-ui is the interactive front end.
	if !execMode && !benchmarkEnabled && !thinkbenchEnabled && *benchHaystack == "" {
		fmt.Fprintln(os.Stderr, "Usage: kvit-coder -p \"prompt\" [options]")
		fmt.Fprintln(os.Stderr, "       kvit-coder --benchmark [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "kvit-coder is a headless agent. Use kvit-coder-ui for interactive mode.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	// An interrupt cancels the run rather than killing the process, so the turn
	// gets to write down what it did before exiting. A second one within two
	// seconds means the caller wants out now.
	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	installInterruptHandler(cancelRun)

	// Acquire workspace lock to prevent multiple instances on same workspace
	workspaceLock, err := workspace.AcquireLock(cfg.Workspace.Root)
	if err != nil {
		log.Fatalf("Failed to acquire workspace lock: %v", err)
	}
	defer workspaceLock.Release()

	// Initialize LLM client
	llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey,
		llm.WithBackend(cfg.LLM.APIBackend),
		llm.WithHeaders(cfg.LLMHeaders()),
		llm.WithReasoningEffort(cfg.LLM.ReasoningEffort))

	// Resolve the session before anything that keeps state, because everything
	// that outlives a turn now lives in the session directory: the checkpoint
	// repository, spilled tool output, and (from here on) the steering inbox
	// and background-process records.
	//
	// A benchmark run has no session to continue, so it gets an ephemeral one
	// in a fresh temp directory, removed on exit. That is the lifetime the
	// checkpoint repository had before, and it keeps one code path.
	var sess *session.Session
	var sessionMgr *session.Manager
	if benchmarkEnabled || thinkbenchEnabled || *benchHaystack != "" {
		runDir, err := os.MkdirTemp("", "kvit-coder-run-")
		if err != nil {
			log.Fatalf("Failed to create run directory: %v", err)
		}
		defer os.RemoveAll(runDir)
		if sess, err = session.OpenDir(runDir); err != nil {
			log.Fatalf("Failed to create run session: %v", err)
		}
	} else {
		sessionMgr, err = session.NewManager()
		if err != nil {
			log.Fatalf("Failed to create session manager: %v", err)
		}
		name := *sessionName
		if name == "" {
			name = sessionMgr.GenerateSessionName()
		}
		// Pin the run ID to the session, unless the caller already chose one.
		// Endpoints that route by a session header then send every turn of
		// this conversation to the backend that already holds its prompt cache.
		if os.Getenv(session.RunIDVar) == "" {
			os.Setenv(session.RunIDVar, session.RunIDFor(name))
		}
		if sess, err = sessionMgr.Open(name); err != nil {
			log.Fatalf("Failed to open session %q: %v", name, err)
		}
		sessionUnlock, err := sess.AcquireLock()
		if err != nil {
			log.Fatalf("%v", err)
		}
		defer sessionUnlock()
	}

	// Spilled tool output lives in the session, which is outside the
	// workspace, so the model needs read permission for the paths it is told.
	cfg.Workspace.AllowedReadPaths = append(cfg.Workspace.AllowedReadPaths, sess.TmpDir())

	// Say which process is running this session's turn, so "kvit-coder steer"
	// with no -s can find it.
	endTurn, err := sess.MarkTurnStart()
	if err != nil {
		writer.Debug(fmt.Sprintf("Failed to record turn pid: %v", err))
	}
	defer endTurn()

	// Processes that outlive the turn are recorded in the session, so a dev
	// server started in one turn is still serving in the next.
	procRegistry, err := procs.New(sess.ProcDir())
	if err != nil {
		log.Fatalf("Failed to open the process registry: %v", err)
	}
	if cfg.Tools.Procs.Enabled && cfg.Tools.Procs.ShouldKillOnExit() {
		killBackgroundOnInterrupt(runCtx, procRegistry, sess, writer)
	}

	// The inbox is where anything arriving mid-turn waits. The loop drains it
	// once per iteration.
	steering := inbox.New(sess.InboxDir())
	steering.Log = func(msg string) { writer.Warn(msg) }
	interactive := startStdinReader(steering, writer)

	// Initialize temp file manager for shell command outputs. Its files are not
	// removed at exit: the model may have been given a path to read next turn.
	tempFileMgr := tools.NewTempFileManager(sess.TmpDir())

	// Initialize MCP (Model Context Protocol) client manager. A no-op when
	// cfg.MCP.Enabled is false or no servers are configured. Servers that fail
	// to connect are logged and skipped, never fatal.
	mcpMgr := mcp.NewManager(cfg.MCP, cfg.Workspace.Root, writer, tempFileMgr)
	if err := mcpMgr.Connect(context.Background()); err != nil {
		writer.Warn(fmt.Sprintf("MCP: some servers failed to connect: %v", err))
	}
	defer mcpMgr.Close()
	if summary := mcpMgr.Summary(); summary != "" {
		writer.Debug("MCP: " + summary)
	}

	// Initialize plan manager
	planManager := tools.NewPlanManager()

	// Initialize checkpoint manager. Its shadow git repository lives in the
	// session, so a checkpoint made in one turn is still there in the next.
	checkpointMgr, err := checkpoint.NewManager(
		sess.CheckpointsDir(),
		cfg.Workspace.Root,
		cfg.Tools.Checkpoint.ExcludedPatterns,
		cfg.Tools.Checkpoint.MaxFileSizeKB,
	)
	if err != nil {
		log.Fatalf("Failed to create checkpoint manager: %v", err)
	}

	// Initialize checkpoint infrastructure
	if err := checkpointMgr.Initialize(); err != nil {
		writer.Warn(fmt.Sprintf("Failed to initialize checkpoints: %v (continuing without checkpoints)", err))
		checkpointMgr.SetEnabled(false)
	} else {
		writer.Debug("Checkpoint infrastructure initialized")
	}

	// Initialize Tasks tools manager (if enabled)
	var contextMgr *ctxtools.Manager
	var contextMiddleware *ctxtools.Middleware
	if cfg.Tools.Tasks.Enabled {
		var err error
		contextMgr, err = ctxtools.NewManager(filepath.Join(sess.Dir(), "tasks"), checkpointMgr)
		if err != nil {
			writer.Warn(fmt.Sprintf("Failed to create Tasks manager: %v (continuing without Tasks tools)", err))
		} else {
			if err := contextMgr.Initialize(); err != nil {
				writer.Warn(fmt.Sprintf("Failed to initialize Tasks tools: %v (continuing without Tasks tools)", err))
				contextMgr = nil
			} else {
				writer.Debug("Tasks tools initialized")

				// Initialize middleware for turn number injection
				contextMiddleware = ctxtools.NewMiddleware(contextMgr, ctxtools.RuntimeNoticeConfig{
					TaskWarnTurns:       cfg.Tools.Tasks.TaskWarnTurns,
					TaskCriticalTurns:   cfg.Tools.Tasks.TaskCriticalTurns,
					ContextCapacityWarn: cfg.Tools.Tasks.ContextCapacityWarn,
					MaxNestedDepth:      cfg.Tools.Tasks.MaxNestedDepth,
					NotifyFileChanges:   cfg.Tools.Tasks.NotifyFileChanges,
				})
			}
		}
	}

	// Create shared tool context for this session
	toolCtx := tools.NewToolContext()
	// Question reads its answer from the same inbox the loop drains, so there
	// is one input path rather than two contending for stdin.
	toolCtx.SetInbox(steering)
	// Permission grants: what has been allowed for this session, this project
	// and this machine. All three files live under the user's home, never in
	// the workspace, because a permission file the agent can edit is not one.
	toolCtx.SetGrantor(openGrants(sess, cfg.Workspace.Root, writer))
	// A tool that waits for a person must agree with whether one is there.
	toolCtx.SetInteractive(interactive)

	// With a line reader running it is the only thing reading the terminal, so
	// every prompt in the process claims its answer from the inbox rather than
	// opening the terminal again. Two readers on one terminal is a race for
	// each keystroke, and the loser waits forever — which is what made a path
	// confirmation hang while the line you typed was queued as steering.
	if interactive {
		ask := func(prompt string) (string, bool) {
			// runCtx, not the tool's context: waiting for a person is not the
			// tool being slow, and only an interrupt should end the wait. The
			// time is recorded so the loop can subtract it from the tool's
			// own clock.
			started := time.Now()
			answer, outcome := steering.Ask(runCtx, os.Stderr, prompt, 0)
			toolCtx.AddPromptWait(time.Since(started))
			return answer, outcome == inbox.AskAnswered
		}
		config.SetLinePrompter(ask)
		mcp.SetLinePrompter(ask)
	}

	// Setup tool registry using the new setup function
	registry := tools.SetupRegistry(tools.SetupConfig{
		Cfg:           cfg,
		CheckpointMgr: checkpointMgr,
		ContextMgr:    contextMgr,
		Logger:        writer, // Writer implements DebugLogger
		TempFileMgr:   tempFileMgr,
		PlanManager:   planManager,
		ToolCtx:       toolCtx,
		MCPTools:      mcpMgr.Tools(),
		ProcRegistry:  procRegistry,
	})

	// Generate system prompt using the prompt generator
	promptGen, err := prompt.NewGenerator(registry, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	systemPrompt, err := promptGen.GenerateSystemPrompt()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating system prompt: %v\n", err)
		os.Exit(1)
	}
	if cfg.Agent.AgentFile != "" {
		agentContent, err := os.ReadFile(cfg.Agent.AgentFile)
		if err != nil {
			log.Fatalf("Failed to read agent file %q: %v", cfg.Agent.AgentFile, err)
		}
		systemPrompt += "\n\n" + string(agentContent)
	}

	// Create agent runner
	runner := agent.NewRunner(agent.RunnerOptions{
		Cfg:               cfg,
		LLMClient:         llmClient,
		Registry:          registry,
		Writer:            writer,
		Logger:            logger,
		CheckpointMgr:     checkpointMgr,
		ContextMgr:        contextMgr,
		ContextMiddleware: contextMiddleware,
		PlanManager:       planManager,
		ToolCtx:           toolCtx,
		Inbox:             steering,
		Procs:             procRegistry,
	})

	// Run benchmark mode if requested
	if benchmarkEnabled {
		writer.StartupInfo("Agent REPL Benchmark Mode")
		writer.StartupInfo(fmt.Sprintf("Model: %s @ %s", cfg.LLM.Model, cfg.LLM.BaseURL))
		fmt.Println()

		flags := benchmark.CLIFlags{
			Enabled:     true,
			Runs:        *benchmarkRuns,
			Category:    *benchmarkCategory,
			BenchmarkID: *benchmarkID,
			OutputFile:  *benchmarkOutput,
			NoResume:    *benchmarkNoResume,
			Suffix:      benchmarkSuffix,
		}

		if err := benchmark.Run(runCtx, flags, runner, cfg, systemPrompt, version, originalWorkspaceRoot); err != nil {
			log.Fatalf("Benchmark failed: %v", err)
		}
		return
	}

	// Run haystack (needle retrieval) benchmark mode if requested
	haystackEnabled := *benchHaystack != ""
	if haystackEnabled {
		writer.StartupInfo("Haystack Benchmark Mode (Needle Retrieval)")
		writer.StartupInfo(fmt.Sprintf("Model: %s @ %s", cfg.LLM.Model, cfg.LLM.BaseURL))
		fmt.Println()

		flags := benchmark.HaystackCLIFlags{
			Enabled:     true,
			Runs:        *benchmarkRuns,
			BenchmarkID: *benchHaystackID,
			OutputFile:  *benchmarkOutput,
			NoResume:    *benchmarkNoResume,
			Suffix:      haystackSuffix,
		}

		if err := benchmark.RunHaystack(runCtx, flags, cfg, version, originalWorkspaceRoot); err != nil {
			log.Fatalf("Haystack benchmark failed: %v", err)
		}
		return
	}

	// Run thinkbench (coding agent) benchmark mode if requested
	if thinkbenchEnabled {
		// Resolve trials: explicit -n wins, else config thinkbench.trials, else 3.
		nSet := false
		flag.Visit(func(f *flag.Flag) {
			if f.Name == "n" {
				nSet = true
			}
		})
		trials := *benchmarkRuns
		if !nSet {
			if cfg.Thinkbench.Trials > 0 {
				trials = cfg.Thinkbench.Trials
			} else {
				trials = 3
			}
		}

		writer.StartupInfo("Thinkbench Benchmark Mode (Coding Agent)")
		writer.StartupInfo(fmt.Sprintf("Model: %s @ %s", cfg.LLM.Model, cfg.LLM.BaseURL))
		fmt.Println()

		flags := benchmark.ThinkbenchCLIFlags{
			Enabled:     true,
			Runs:        trials,
			BenchmarkID: *benchThinkbenchID,
			Types:       *benchThinkbenchTypes,
			SuiteDir:    *thinkbenchSuite,
			OutputFile:  *benchmarkOutput,
			NoResume:    *benchmarkNoResume,
			Resume:      *benchThinkbenchResume,
			Suffix:      thinkbenchSuffix,
		}

		if err := benchmark.RunThinkbench(runCtx, flags, runner, cfg, systemPrompt, version, originalWorkspaceRoot); err != nil {
			log.Fatalf("Thinkbench benchmark failed: %v", err)
		}
		return
	}

	projectInstructions, err := repl.LoadProjectInstructions(cfg, launchDir)
	if err != nil {
		log.Fatalf("Failed to load project instructions: %v", err)
	}
	if projectInstructions != nil {
		writer.Debug(fmt.Sprintf("Project instructions: %s", projectInstructions.Path))
	}

	// Show startup info
	writer.StartupInfo("Agent REPL v0.1")
	writer.StartupInfo(fmt.Sprintf("Model: %s @ %s", cfg.LLM.Model, cfg.LLM.BaseURL))
	writer.StartupInfo(fmt.Sprintf("Tools: %s", strings.Join(registry.ListTools(), ", ")))
	if *logFile != "" {
		writer.StartupInfo(fmt.Sprintf("Logs: %s", *logFile))
	}
	fmt.Println()

	// Run in exec mode (always, since we require -p or --benchmark)
	repl.RunExec(runCtx, runner, writer, cfg, systemPrompt, promptText, quietMode, sess, projectInstructions)
}

// startStdinReader queues each line typed at the terminal for the running
// turn, and reports whether it started. It starts only when stdin is a
// terminal: a piped run and the benchmark harness have a stdin that is not a
// person, and reading it would consume input meant for something else. The
// goroutine ends with the process.
func startStdinReader(steering *inbox.Inbox, writer *ui.Writer) bool {
	if !stdinIsATerminal() {
		return false
	}
	go func() {
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" {
				continue
			}
			// A prompt waiting for an answer will take this line, so saying it
			// was queued for the model would be wrong — and reads as the
			// answer having been swallowed, which is what it looked like
			// before this check existed.
			answering := steering.Awaiting()
			steering.Push(inbox.Message{Kind: inbox.KindUserLine, Text: line})
			if !answering {
				writer.Info("→ queued")
			}
		}
	}()
	return true
}

// stdinIsATerminal reports whether there is a person on the other end of
// stdin. /dev/null is a character device like a terminal is, and a run
// redirected from it — which is how a script or a CI job starts one — has
// nobody to type an answer.
func stdinIsATerminal() bool {
	info, err := os.Stdin.Stat()
	if err != nil || info.Mode()&os.ModeCharDevice == 0 {
		return false
	}
	if devNull, err := os.Stat(os.DevNull); err == nil && os.SameFile(info, devNull) {
		return false
	}
	return true
}

// installInterruptHandler makes ctrl-c end the turn rather than end the
// process. The first interrupt cancels the run context: the loop stops where
// it is, the tool that was running is killed, and what happened is written to
// the session before the process exits normally. A second interrupt within two
// seconds exits immediately, for when that is taking too long.
func installInterruptHandler(cancelRun context.CancelFunc) {
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		var last time.Time
		for range sigCh {
			now := time.Now()
			if !last.IsZero() && now.Sub(last) < 2*time.Second {
				fmt.Fprintln(os.Stderr, "\nquitting now")
				os.Exit(130) // 128 + SIGINT(2)
			}
			last = now
			fmt.Fprintln(os.Stderr, "\ninterrupted - stopping the turn and saving it; press ctrl-c again to quit now")
			cancelRun()
		}
	}()
}

// killBackgroundOnInterrupt stops everything this session started when the run
// is cancelled, and records that it did. An interrupt that silently leaves a
// dev server holding a port is a surprise you find out about much later.
func killBackgroundOnInterrupt(ctx context.Context, registry *procs.Registry, sess *session.Session, writer *ui.Writer) {
	go func() {
		<-ctx.Done()
		killed := registry.KillAll()
		if len(killed) == 0 {
			return
		}
		msg := fmt.Sprintf("stopped %d background process(es) on interrupt: %s",
			len(killed), strings.Join(killed, ", "))
		writer.Info(msg)
		_ = sess.Notice(msg)
	}()
}

// openGrants reads the three permission files that apply to this run. A file
// that cannot be read is reported and treated as empty: a broken grant file
// should cost you a prompt, not a run.
func openGrants(sess *session.Session, workspaceRoot string, writer *ui.Writer) *permissions.Grantor {
	home, err := os.UserHomeDir()
	if err != nil {
		writer.Debug(fmt.Sprintf("Permissions: no home directory, so only this session's grants apply: %v", err))
		home = ""
	}

	open := func(path, source string) *permissions.Store {
		if path == "" {
			return nil
		}
		store, err := permissions.OpenStore(path, source)
		if err != nil {
			writer.Warn(fmt.Sprintf("Permissions: %v", err))
			return nil
		}
		return store
	}

	var project, global *permissions.Store
	if home != "" {
		project = open(permissions.ProjectStorePath(home, workspaceRoot), "project")
		global = open(permissions.GlobalStorePath(home), "global")
	}
	return permissions.NewGrantor(
		open(filepath.Join(sess.Dir(), "permissions.json"), "session"),
		project,
		global,
	)
}

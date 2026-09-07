package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/benchmark"
	"github.com/kvit-s/kvit-coder/internal/checkpoint"
	"github.com/kvit-s/kvit-coder/internal/config"
	ctxtools "github.com/kvit-s/kvit-coder/internal/context"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/mcp"
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

	// Initialize temp file manager for shell command outputs
	tempFileMgr := tools.NewTempFileManager(cfg.Workspace.Root)
	defer tempFileMgr.CleanupAll()

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

	// Initialize checkpoint manager
	sessionID := fmt.Sprintf("%d", time.Now().UnixNano())
	checkpointMgr, err := checkpoint.NewManager(
		sessionID,
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
		defer func() { _ = checkpointMgr.Cleanup() }()
	}

	// Initialize Tasks tools manager (if enabled)
	var contextMgr *ctxtools.Manager
	var contextMiddleware *ctxtools.Middleware
	if cfg.Tools.Tasks.Enabled {
		var err error
		contextMgr, err = ctxtools.NewManager(sessionID, checkpointMgr)
		if err != nil {
			writer.Warn(fmt.Sprintf("Failed to create Tasks manager: %v (continuing without Tasks tools)", err))
		} else {
			if err := contextMgr.Initialize(); err != nil {
				writer.Warn(fmt.Sprintf("Failed to initialize Tasks tools: %v (continuing without Tasks tools)", err))
				contextMgr = nil
			} else {
				writer.Debug("Tasks tools initialized")
				defer func() { _ = contextMgr.Cleanup() }()

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

		if err := benchmark.Run(context.Background(), flags, runner, cfg, systemPrompt, version, originalWorkspaceRoot); err != nil {
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

		if err := benchmark.RunHaystack(context.Background(), flags, cfg, version, originalWorkspaceRoot); err != nil {
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

		if err := benchmark.RunThinkbench(context.Background(), flags, runner, cfg, systemPrompt, version, originalWorkspaceRoot); err != nil {
			log.Fatalf("Thinkbench benchmark failed: %v", err)
		}
		return
	}

	// Require -p or --benchmark mode (kvit-coder is headless, use kvit-coder-ui for interactive mode)
	if !execMode {
		fmt.Fprintln(os.Stderr, "Usage: kvit-coder -p \"prompt\" [options]")
		fmt.Fprintln(os.Stderr, "       kvit-coder --benchmark [options]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "kvit-coder is a headless agent. Use kvit-coder-ui for interactive mode.")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Options:")
		flag.PrintDefaults()
		os.Exit(1)
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

	// Initialize session manager
	sessionMgr, err := session.NewManager()
	if err != nil {
		log.Fatalf("Failed to create session manager: %v", err)
	}

	// Acquire lock on session if specified
	if *sessionName != "" {
		sessionUnlock, err := sessionMgr.AcquireLock(*sessionName)
		if err != nil {
			log.Fatalf("%v", err)
		}
		defer sessionUnlock()
	}

	// Run in exec mode (always, since we require -p or --benchmark)
	repl.RunExec(runner, writer, cfg, systemPrompt, promptText, quietMode, *sessionName, sessionMgr, projectInstructions)
}

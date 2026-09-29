package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/benchmark"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/mcp"
	"github.com/kvit-s/kvit-coder/internal/procs"
	"github.com/kvit-s/kvit-coder/internal/prompt"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
	"github.com/kvit-s/kvit-coder/internal/workspace"
)

// turnOptions is what the caller decides about a turn before its setup: where
// the configuration comes from, which session it continues, where its output
// goes, and anything the caller lays over the configuration. `kvit-coder -p`
// fills it from its flags; `kvit-coder acp` from the client's messages.
type turnOptions struct {
	// Parent is the context the turn's run context derives from.
	Parent context.Context
	// Writer is where the turn's progress and answer go. The caller has set
	// its modes (quiet, JSON, headless) and its streams.
	Writer *ui.Writer
	// LogFile is the agent's log, empty for none.
	LogFile string

	ConfigPath string // -config; empty searches the usual places
	ConfigName string // the file name searched for

	ModelRef   string
	Effort     *stringFlag
	APIBackend string
	BaseURL    string
	AgentFile  string
	Yolo       bool
	Structured bool

	// Benchmark runs keep a sandbox of their own and an ephemeral session.
	BenchmarkMode     string // the --benchmark value, empty when not benchmarking
	HaystackEnabled   bool
	ThinkbenchEnabled bool
	ThinkbenchSuffix  string
	ThinkbenchSuite   string

	SessionName  string
	ContinueLast bool

	// Workspace, when set, is the workspace root in place of the config's,
	// the way the directory a terminal run starts in is.
	Workspace string
	// ExtraPrompt is added to the system prompt after the agent file.
	ExtraPrompt string
	// Configure has the last word on the configuration before anything is
	// built from it.
	Configure func(*config.Config)
	// ReadTerminal starts the line reader when stdin is a terminal.
	ReadTerminal bool
	// Interrupts makes ctrl-c end the turn rather than the process.
	Interrupts bool
}

// preparedTurn is everything a turn needs to run the loop, built from the
// options: the configuration as laid over, the session opened and locked, the
// tools registered and the runner made. Close releases it all, in reverse.
type preparedTurn struct {
	cfg                   *config.Config
	writer                *ui.Writer
	logger                *agent.Logger
	llmClient             *llm.Client
	runner                *agent.Runner
	registry              *tools.Registry
	toolCtx               *tools.ToolContext
	procRegistry          *procs.Registry
	inbox                 *inbox.Inbox
	sess                  *session.Session
	sessionMgr            *session.Manager
	systemPrompt          string
	originalWorkspaceRoot string
	runCtx                context.Context
	cancelRun             context.CancelFunc

	closers []func()
}

// Close runs every cleanup the setup registered, newest first, as the
// deferred calls in main used to.
func (t *preparedTurn) Close() {
	for i := len(t.closers) - 1; i >= 0; i-- {
		t.closers[i]()
	}
	t.closers = nil
}

func (t *preparedTurn) defer_(fn func()) {
	t.closers = append(t.closers, fn)
}

// promptSetupError is a failure to build the system prompt, which -p reports
// on stderr in the line it holds rather than through the log.
type promptSetupError struct{ line string }

func (e promptSetupError) Error() string { return e.line }

// prepareTurn builds a turn from its options. On failure what was already set
// up is released and the error says what failed, in the words -p has always
// used for it.
func prepareTurn(opts turnOptions) (turn *preparedTurn, err error) {
	t := &preparedTurn{writer: opts.Writer}
	defer func() {
		if err != nil {
			t.Close()
			turn = nil
		}
	}()
	writer := opts.Writer
	parent := opts.Parent
	if parent == nil {
		parent = context.Background()
	}

	// Initialize logger
	logger, err := agent.NewLogger(config.ExpandHome(opts.LogFile), false)
	if err != nil {
		return nil, fmt.Errorf("Failed to initialize logger: %v", err)
	}
	t.logger = logger
	t.defer_(func() { logger.Close() })

	// With no -config, look for the file in the usual places rather than only
	// in the directory the agent was started from, so it runs from anywhere.
	actualConfigPath, note, err := config.ResolveNamed(opts.ConfigPath, opts.ConfigName)
	if note != "" {
		fmt.Fprintln(os.Stderr, note)
	}
	if err != nil {
		return nil, err
	}

	// Load config
	cfg, err := config.Load(actualConfigPath)
	if err != nil {
		return nil, fmt.Errorf("Failed to load config: %v", err)
	}
	t.cfg = cfg

	// Set verbose level from config
	writer.SetVerbose(cfg.LLM.Verbose)

	// Style the final answer as markdown on a terminal (auto by default;
	// piped output and --json stay raw markdown).
	writer.SetMarkdownMode(cfg.UI.Markdown)

	// Apply flag overrides: the model catalog first (so --model resolves the
	// full entry, not just the name), then the explicit wire fields, which
	// win over the entry so a skewed UI/agent config pair still sends the
	// right endpoint.
	effort := opts.Effort
	if effort == nil {
		effort = &stringFlag{}
	}
	resolveModelSelection(cfg, opts.ModelRef, effort, opts.APIBackend, opts.BaseURL)
	if opts.AgentFile != "" {
		cfg.Agent.AgentFile = opts.AgentFile
	}
	// --yolo drops the workspace boundary for the whole run: every file tool
	// and every shell command may read and write anywhere, with no prompt and
	// no warning. It also answers yes to the commands that would otherwise
	// stop and ask, which is what the flag is for in a run with no terminal —
	// there an unanswered question is a refusal rather than a pause, so
	// without this a headless --yolo run fails on the first curl it needs.
	// What is refused outright stays refused, and so do dd and mkfs, where a
	// wrong answer cannot be taken back.
	//
	// It is set here, before the benchmark overrides below, so a benchmark run
	// keeps its sandbox whatever the flag says.
	if opts.Yolo {
		cfg.Workspace.PathSafetyMode = "allow"
		cfg.Tools.Shell.AllowWithoutAsking = true
		writer.Warn("--yolo: full filesystem access, no prompts (dd/mkfs still ask)")
	}

	// --structured is the only switch for the Report tool, so a scripted run
	// gets the prose ending it was written against unless it asks for
	// otherwise, and a caller that wants machine-readable output opts in per
	// invocation rather than through a config file it may not control.
	// kvit-coder-ui passes it for every turn it spawns, which is what makes
	// reports the default there and off here. The config file still supplies
	// the mode and the limits (tools.report).
	cfg.Tools.Report.Enabled = opts.Structured

	// A workspace the caller names replaces the config's, the way the
	// directory a terminal run starts in is the workspace.
	if opts.Workspace != "" {
		cfg.Workspace.Root = opts.Workspace
	}

	// Override workspace for benchmark mode - set BEFORE tools are initialized
	// Store original workspace root for finding benchmarks.yaml
	t.originalWorkspaceRoot = cfg.Workspace.Root
	benchmarkEnabled := opts.BenchmarkMode != ""
	if benchmarkEnabled {
		baseDir := filepath.Join(cfg.Workspace.Root, ".kvit-coder-benchmark")
		// Use workspace-{suffix} when --benchmark {suffix} is specified to allow parallel runs
		// e.g., --benchmark claude uses workspace-claude, --benchmark gemini uses workspace-gemini
		if opts.BenchmarkMode != "" {
			cfg.Workspace.Root = filepath.Join(baseDir, "workspace-"+opts.BenchmarkMode)
		} else {
			cfg.Workspace.Root = filepath.Join(baseDir, "workspace")
		}
		cfg.Workspace.PathSafetyMode = "block"
		cfg.Workspace.AllowOutsideWorkspace = false

		// Create workspace directory
		if err := os.MkdirAll(cfg.Workspace.Root, 0755); err != nil {
			return nil, fmt.Errorf("Failed to create benchmark workspace: %v", err)
		}
	}

	// Override workspace for thinkbench mode - hard sandbox, set BEFORE tools init.
	// The run workspace lives under benchmarks/thinkbench/runs/ (gitignored), kept
	// physically separate from the suite (held-out graders/references).
	if opts.ThinkbenchEnabled {
		// Anchor the run workspace to the suite location (not cwd) so the harness
		// runs identically from the repo root or from benchmarks/.
		_, tbRoot := benchmark.ResolveThinkbench(t.originalWorkspaceRoot, opts.ThinkbenchSuite, cfg.Thinkbench.SuiteDir)
		wsName := "workspace"
		if opts.ThinkbenchSuffix != "" {
			wsName = "workspace-" + opts.ThinkbenchSuffix
		}
		cfg.Workspace.Root = filepath.Join(tbRoot, "runs", wsName)
		cfg.Workspace.PathSafetyMode = "block"
		cfg.Workspace.AllowOutsideWorkspace = false
		cfg.Workspace.AllowedPaths = nil
		cfg.Workspace.AllowedReadPaths = nil
		if err := os.MkdirAll(cfg.Workspace.Root, 0755); err != nil {
			return nil, fmt.Errorf("Failed to create thinkbench workspace: %v", err)
		}
	}

	// What the caller lays over the configuration, last.
	if opts.Configure != nil {
		opts.Configure(cfg)
	}

	// An interrupt cancels the run rather than killing the process, so the turn
	// gets to write down what it did before exiting. A second one within two
	// seconds means the caller wants out now.
	runCtx, cancelRun := context.WithCancel(parent)
	t.runCtx, t.cancelRun = runCtx, cancelRun
	t.defer_(cancelRun)
	// interruptCtx fires only on a real SIGINT/SIGTERM, never on normal turn
	// end: runCtx is cancelled by the deferred cancelRun above on every exit,
	// so a background killer watching runCtx would SIGTERM even the persistent
	// processes on the way out, racing the process's own exit. Do NOT defer
	// signalInterrupt here: cancelling interruptCtx on normal exit would fire
	// the killer below on every turn. The context is dropped at process exit.
	// Without the handler nothing ever fires it: a caller that ends turns in
	// some other way, such as the ACP mode's session/cancel, has no ctrl-c.
	interruptCtx := context.Background()
	if opts.Interrupts {
		var signalInterrupt context.CancelFunc
		interruptCtx, signalInterrupt = context.WithCancel(context.Background())
		installInterruptHandler(cancelRun, signalInterrupt)
	}

	// One agent at a time per working directory, when workspace.lock asks for
	// it. It is off by default: the lock is held for the whole of a turn, so a
	// turn that stops to ask a permission question blocks every other agent in
	// the directory until it is answered, and the second agent dies rather than
	// waits. Running two agents on one checkout is a normal thing to want.
	if cfg.Workspace.Lock {
		workspaceLock, err := workspace.AcquireLock(cfg.Workspace.Root)
		if err != nil {
			return nil, fmt.Errorf("Failed to acquire workspace lock: %v", err)
		}
		t.defer_(func() { workspaceLock.Release() })
	}

	// Initialize LLM client
	t.llmClient = llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey,
		llm.WithAPIKeyEnv(cfg.LLM.APIKeyEnv),
		llm.WithBackend(cfg.LLM.APIBackend),
		llm.WithHeaders(cfg.LLMHeaders()),
		llm.WithReasoningEffort(cfg.LLM.ReasoningEffort),
		llm.WithEffortField(cfg.LLM.EffortField),
		llm.WithReasoningSummary(cfg.ReasoningSummaryOrDefault()),
		llm.WithTimeout(time.Duration(cfg.LLM.RequestTimeout)*time.Second),
		llm.WithRetryNotice(func(attempt, maxAttempts int, delay time.Duration, reason error) {
			// Say it out loud. A failed request that is quietly retried looks
			// exactly like a slow one from the progress indicator, and the
			// turn can sit there for the timeout times the retry count.
			writer.Warn(fmt.Sprintf("model request failed (%d/%d), retrying in %s", attempt, maxAttempts, delay.Round(time.Second)))
		}))

	// Resolve the session before anything that keeps state, because everything
	// that outlives a turn now lives in the session directory: spilled tool
	// output, and (from here on) the steering inbox and background-process
	// records.
	//
	// A benchmark run has no session to continue, so it gets an ephemeral one
	// in a fresh temp directory, removed on exit.
	var sess *session.Session
	if benchmarkEnabled || opts.ThinkbenchEnabled || opts.HaystackEnabled {
		runDir, err := os.MkdirTemp("", "kvit-coder-run-")
		if err != nil {
			return nil, fmt.Errorf("Failed to create run directory: %v", err)
		}
		t.defer_(func() { os.RemoveAll(runDir) })
		if sess, err = session.OpenDir(runDir); err != nil {
			return nil, fmt.Errorf("Failed to create run session: %v", err)
		}
	} else {
		sessionMgr, err := session.NewManager()
		if err != nil {
			return nil, fmt.Errorf("Failed to create session manager: %v", err)
		}
		t.sessionMgr = sessionMgr
		name := opts.SessionName
		if name == "" && opts.ContinueLast {
			recent, err := sessionMgr.MostRecent()
			if err != nil {
				return nil, fmt.Errorf("Failed to find the most recent session: %v", err)
			}
			if recent == "" {
				writer.Warn("no previous session to continue; starting a new one")
			}
			name = recent
		}
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
			return nil, fmt.Errorf("Failed to open session %q: %v", name, err)
		}
		sessionUnlock, err := sess.AcquireLock()
		if err != nil {
			return nil, err
		}
		t.defer_(sessionUnlock)
	}
	t.sess = sess

	// The session directory sits outside the workspace and the model is handed
	// paths into it: spilled tool output in tmp/, its own history, its inbox.
	// Only tmp/ used to be allowed, and only on the read list, which no shell
	// command qualifies for because a shell command can write. So the model
	// asked permission every time it looked at its own output with cat. The
	// whole session directory now goes on allowed_paths, read and write: it is
	// the agent's own scratch space, and the workspace root -- the source code
	// being edited -- is already fully writable without asking.
	//
	// A benchmark run keeps the narrower grant. Its workspace is a sandbox with
	// path_safety_mode=block and both allow lists cleared, and only reading the
	// spilled output has to survive that.
	if opts.ThinkbenchEnabled {
		cfg.Workspace.AllowedReadPaths = append(cfg.Workspace.AllowedReadPaths, sess.TmpDir())
	} else {
		cfg.Workspace.AllowedPaths = append(cfg.Workspace.AllowedPaths, sess.Dir())
	}

	// Say which process is running this session's turn, so "kvit-coder steer"
	// with no -s can find it.
	endTurn, err := sess.MarkTurnStart()
	if err != nil {
		writer.Debug(fmt.Sprintf("Failed to record turn pid: %v", err))
	}
	t.defer_(endTurn)

	// Processes that outlive the turn are recorded in the session, so a dev
	// server started in one turn is still serving in the next.
	procRegistry, err := procs.New(sess.ProcDir())
	if err != nil {
		return nil, fmt.Errorf("Failed to open the process registry: %v", err)
	}
	t.procRegistry = procRegistry
	if cfg.Tools.Procs.Enabled && cfg.Tools.Procs.ShouldKillOnExit() {
		// Deferred, so it runs synchronously on the way out — after RunExec
		// returns — and the save inside KillAll lands before the process
		// exits. A fire-and-forget goroutine loses that race: it SIGTERMs and
		// the turn exits out from under the save, leaving gone/-1 behind.
		t.defer_(killBackgroundOnInterrupt(interruptCtx, procRegistry, sess, writer))
	}

	// The inbox is where anything arriving mid-turn waits. The loop drains it
	// once per iteration.
	steering := inbox.New(sess.InboxDir())
	steering.Log = func(msg string) { writer.Warn(msg) }
	t.inbox = steering
	// Progress dots redraw the line they are on, so they must not run while
	// anyone owns the terminal: a question waiting for an answer, the pause
	// prompt, or the window between an empty Enter and that prompt, where
	// type-ahead would otherwise be painted over one dot at a time.
	writer.SetPromptWatcher(pauseAwareWatcher(steering))
	interactive := false
	if opts.ReadTerminal {
		interactive = startStdinReader(steering, writer)
	}

	// Initialize temp file manager for shell command outputs. Its files are not
	// removed at exit: the model may have been given a path to read next turn.
	tempFileMgr := tools.NewTempFileManager(sess.TmpDir())

	// Initialize MCP (Model Context Protocol) client manager. A no-op when
	// cfg.MCP.Enabled is false or no servers are configured. Servers that fail
	// to connect are logged and skipped, never fatal.
	mcpMgr := mcp.NewManager(cfg.MCP, cfg.Workspace.Root, writer, tempFileMgr)
	// Remember "ask_once" answers in the session rather than in memory. The
	// process lives for one turn, so an in-memory map turns ask_once into
	// ask-once-per-instruction -- painless while nothing is configured, and
	// unbearable the moment a browser server is, since every click of every
	// turn would prompt again.
	if sess != nil {
		approvals, err := mcp.OpenApprovals(filepath.Join(sess.Dir(), "mcp-approvals.json"))
		if err != nil {
			writer.Warn(fmt.Sprintf("mcp: approvals will not be remembered past this turn: %v", err))
		} else {
			mcpMgr.SetApprovals(approvals)
		}
	}
	// A server that belongs to a tool group is not dialed here. Its tools are
	// invisible until the model opens the group, so dialing it at startup would
	// pay the spawn on every turn -- half a second to two seconds for a browser
	// server -- to make something available that nothing can call yet.
	if deferred := cfg.DeferredMCPServers(); len(deferred) > 0 {
		mcpMgr.Defer(deferred)
	}
	if err := mcpMgr.Connect(context.Background()); err != nil {
		writer.Warn(fmt.Sprintf("mcp: some servers failed to connect: %v", err))
	}
	t.defer_(func() { mcpMgr.Close() })
	if summary := mcpMgr.Summary(); summary != "" {
		writer.Debug("MCP: " + summary)
	}

	// Create shared tool context for this session
	toolCtx := tools.NewToolContext()
	t.toolCtx = toolCtx
	// Question reads its answer from the same inbox the loop drains, so there
	// is one input path rather than two contending for stdin.
	toolCtx.SetInbox(steering)
	// Permission grants: what has been allowed for this session, this project
	// and this machine. All three files live under the user's home, never in
	// the workspace, because a permission file the agent can edit is not one.
	toolCtx.SetGrantor(openGrants(sess, cfg.Workspace.Root, writer))
	// A tool that waits for a person must agree with whether one is there.
	toolCtx.SetInteractive(interactive)
	// Normalized image copies live in the session, so the next turn finds them.
	toolCtx.SetSessionTmp(sess.TmpDir())

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

		// Under the front end, a prompt waiting for an answer takes the
		// window title over. Only under the front end: a headless run has
		// no long-lived process to put the title back afterwards, so it
		// leaves the title alone. See installAskTitle.
		if session.FromUI() {
			// The same label the front end shows: the session's display
			// title once it has one, its directory name until then.
			label := sess.Meta().Title
			if label == "" {
				label = sess.Name()
			}
			installAskTitle(steering, cfg, os.Stdout, label)
		}
	}

	// Setup tool registry using the new setup function
	// Tool groups: a set of tools the model cannot see until it asks for them.
	// Each is registered as one tool that returns the group's instructions and
	// a reference to every member when called with no arguments, and runs a
	// member when called with one. Members backed by an MCP server are resolved
	// through the manager, which describes them from its cached tool list and
	// dials only when something is actually run.
	var toolGroups []*tools.GroupTool
	for i, g := range cfg.EnabledToolGroups() {
		g := g
		// A server switched off in mcp.servers, or MCP switched off entirely,
		// leaves nothing for the group to hold. Registering it anyway would
		// offer the model a tool that can only fail.
		var servers []string
		for _, name := range g.MCPServers {
			if mcpMgr.ServerUsable(name) {
				servers = append(servers, name)
			} else {
				writer.Debug(fmt.Sprintf("tool group %s: server %q is unavailable", g.Name, name))
			}
		}
		if len(servers) == 0 && len(g.Tools) == 0 {
			writer.Debug(fmt.Sprintf("tool group %s has no available members, skipping", g.Name))
			continue
		}

		var source tools.GroupMemberSource
		if len(servers) > 0 {
			source = func(ctx context.Context) ([]tools.Tool, error) {
				return mcpMgr.ToolsForServers(ctx, servers)
			}
		}
		// Ordered after the built-in tools of whichever category they sit in.
		toolGroups = append(toolGroups, tools.NewGroupTool(
			g.Name, g.Description, g.Instructions, g.GetCategory(), 900+i, source))
	}

	registry := tools.SetupRegistry(tools.SetupConfig{
		Cfg:          cfg,
		Logger:       writer, // Writer implements DebugLogger
		TempFileMgr:  tempFileMgr,
		ToolCtx:      toolCtx,
		MCPTools:     mcpMgr.Tools(),
		ToolGroups:   toolGroups,
		ProcRegistry: procRegistry,
	})
	t.registry = registry

	// Generate system prompt using the prompt generator
	promptGen, err := prompt.NewGenerator(registry, cfg)
	if err != nil {
		return nil, promptSetupError{fmt.Sprintf("Error: %v", err)}
	}
	systemPrompt, err := promptGen.GenerateSystemPrompt()
	if err != nil {
		return nil, promptSetupError{fmt.Sprintf("Error generating system prompt: %v", err)}
	}
	if cfg.Agent.AgentFile != "" {
		agentContent, err := os.ReadFile(cfg.Agent.AgentFile)
		if err != nil {
			return nil, fmt.Errorf("Failed to read agent file %q: %v", cfg.Agent.AgentFile, err)
		}
		systemPrompt += "\n\n" + string(agentContent)
	}
	if opts.ExtraPrompt != "" {
		systemPrompt += "\n\n" + opts.ExtraPrompt
	}
	t.systemPrompt = systemPrompt

	// Create agent runner
	t.runner = agent.NewRunner(agent.RunnerOptions{
		Cfg:       cfg,
		LLMClient: t.llmClient,
		Registry:  registry,
		Writer:    writer,
		Logger:    logger,
		ToolCtx:   toolCtx,
		Inbox:     steering,
		Procs:     procRegistry,
	})

	// The Subagent child loop is the parent Runner itself: rebind it now
	// that both exist, breaking the tools->agent import cycle.
	if sub := registry.Get("Subagent"); sub != nil {
		if s, ok := sub.(interface{ SetRunner(tools.SubRunner) }); ok {
			s.SetRunner(t.runner)
		}
	}

	return t, nil
}

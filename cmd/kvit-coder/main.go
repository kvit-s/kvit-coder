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
	"time"

	"github.com/kvit-s/kvit-coder/internal/benchmark"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/permissions"
	"github.com/kvit-s/kvit-coder/internal/procs"
	"github.com/kvit-s/kvit-coder/internal/procutil"
	"github.com/kvit-s/kvit-coder/internal/repl"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// Version info set by ldflags at build time
var (
	version    = "dev"
	commitHash = "dev"
	commitDate = "unknown"
	buildDate  = "unknown"
)

// printUsage is the text for `kvit-coder help`, `-h`, and a run that was
// given no prompt. The Copilot model list is a subcommand, so it never
// appears among the flags flag.PrintDefaults walks.
func printUsage() {
	fmt.Fprintln(os.Stderr, `Usage: kvit-coder -p "prompt" [options]`)
	fmt.Fprintln(os.Stderr, "       kvit-coder --benchmark [options]")
	fmt.Fprintln(os.Stderr, "       kvit-coder acp [options]")
	fmt.Fprintln(os.Stderr, "       kvit-coder copilot models [-host hostname]")
	fmt.Fprintln(os.Stderr, "       kvit-coder copilot models-add [-host hostname] [-dry-run]")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "kvit-coder copilot models lists the models the signed-in GitHub Copilot account can call.")
	fmt.Fprintln(os.Stderr, "kvit-coder copilot models-add appends them to the config file's models: list,")
	fmt.Fprintln(os.Stderr, "skipping embedding models and rows already there.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "kvit-coder is a headless agent. Use kvit-coder-ui for interactive mode.")
	fmt.Fprintln(os.Stderr, "")
	fmt.Fprintln(os.Stderr, "Options:")
	flag.PrintDefaults()
}

func main() {
	// Subcommands are intercepted before flag.Parse, which has no notion of them.
	if len(os.Args) > 1 && os.Args[1] == "steer" {
		os.Exit(runSteer(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "acp" {
		os.Exit(runACP(os.Args[2:]))
	}
	if len(os.Args) > 1 && os.Args[1] == "copilot" {
		os.Exit(runCopilot(os.Args[2:]))
	}

	// Parse flags
	configPath := flag.String("config", "", "path to config file (default: the first of $KVIT_CODER_CONFIG, ./config.yaml, ~/.kvit-coder/config.yaml, config.yaml beside the binary)")
	model := flag.String("model", "", "select model: :mN, index, id, name or wire id from models:, with optional :effort (e.g. --model m3:xhigh)")
	modelShort := flag.String("m", "", "shorthand for --model")
	baseURL := flag.String("base-url", "", "override LLM base URL")
	apiBackend := flag.String("api-backend", "", "override LLM wire protocol (chat_completions or responses)")
	effort := &stringFlag{}
	flag.Var(effort, "effort", "reasoning effort for the selected model (empty clears it)")
	effortShort := &stringFlag{}
	flag.Var(effortShort, "e", "shorthand for --effort")
	logFile := flag.String("log", "~/.kvit-coder/logs/kvit-coder.log", "log file path (empty to disable)")
	execPrompt := flag.String("p", "", "exec mode: run with this prompt and exit after completion")
	quietPrompt := flag.String("pq", "", "quiet exec mode: run with this prompt and only print final LLM response")
	wakeOnly := flag.Bool("wake", false, "inbox-only turn: skip the user message and process pending inbox/proc events")
	jsonOutput := flag.Bool("json", false, "output structured JSON messages to stderr")
	structured := flag.Bool("structured", false, "end the turn with a structured report instead of prose (see docs/structured-reports.md)")
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
	continueLast := flag.Bool("c", false, "continue the most recent session")
	yolo := flag.Bool("yolo", false, "read and write anywhere on the filesystem, without asking")
	sessionList := flag.Bool("sessions", false, "list all sessions and exit")
	sessionDelete := flag.String("session-delete", "", "delete a session and exit")
	sessionShow := flag.String("session-show", "", "show session history and exit")
	killBackground := flag.String("kill-background", "", "stop running background processes for a session ('all' for every session) and exit")
	listBackground := flag.String("list-background", "", "list running background processes for a session ('all' for every session) and exit")
	agentFile := flag.String("agent-file", "", "path to agent file (content appended to system prompt)")
	var imagePaths imagePathList
	flag.Var(&imagePaths, "image", "attach an image file to the prompt so the model can see it (repeatable)")

	flag.Usage = printUsage
	if len(os.Args) > 1 && os.Args[1] == "help" {
		printUsage()
		os.Exit(0)
	}
	flag.Parse()

	launchDir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Failed to get launch directory: %v", err)
	}

	// Handle --version
	if *showVersion {
		fmt.Printf("kvit-coder %s (commit %s of %s, built %s)\n", version, commitHash, commitDate, buildDate)
		return
	}

	// Handle session management commands early (before config load)
	if *sessionList || *sessionDelete != "" || *sessionShow != "" || *killBackground != "" || *listBackground != "" {
		sessionMgr, err := session.NewManager()
		if err != nil {
			log.Fatalf("Failed to create session manager: %v", err)
		}

		if *listBackground != "" {
			if err := runListBackground(sessionMgr, *listBackground); err != nil {
				log.Fatalf("%v", err)
			}
			return
		}

		if *killBackground != "" {
			if err := runKillBackground(sessionMgr, *killBackground); err != nil {
				log.Fatalf("%v", err)
			}
			return
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
				fmt.Println(strings.Repeat("-", 80))
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
			// Stop its background processes first, otherwise deleting the
			// session orphans them: the pidfiles go away but the processes
			// keep running with no record pointing at them.
			if killed := killSessionBackground(sessionMgr, *sessionDelete); len(killed) > 0 {
				fmt.Printf("Stopped background processes: %s\n", strings.Join(killed, ", "))
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

	// Version line for benchmark reports. Named apart from the package-level
	// version, which ldflags stamps and which this line reports.
	versionLine := fmt.Sprintf("kvit-coder %s (commit %s, built %s)", version, commitHash, buildDate)

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
	} else if *wakeOnly {
		// Inbox-only turn, fired by the UI when the inbox waited while no
		// turn ran: no user message, the drain is the prompt.
		execMode = true
		promptText = ""
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

	// Determine config path - if benchmark mode has a suffix, use config-{suffix}.yaml
	// Use "." for no suffix (plain benchmark mode)
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
	configName := config.DefaultConfigName
	switch {
	case benchmarkSuffix != "":
		configName = fmt.Sprintf("config-%s.yaml", benchmarkSuffix)
	case haystackSuffix != "":
		configName = fmt.Sprintf("config-%s.yaml", haystackSuffix)
	case thinkbenchSuffix != "":
		configName = fmt.Sprintf("config-%s.yaml", thinkbenchSuffix)
	}

	// Handle --benchmark-list early: it needs only the config.
	if *benchmarkList {
		actualConfigPath, note, err := config.ResolveNamed(*configPath, configName)
		if note != "" {
			fmt.Fprintln(os.Stderr, note)
		}
		if err != nil {
			log.Fatalf("%v", err)
		}
		cfg, err := config.Load(actualConfigPath)
		if err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		if err := benchmark.ListBenchmarks(cfg); err != nil {
			log.Fatalf("Failed to list benchmarks: %v", err)
		}
		return
	}

	// Require -p or a benchmark mode before anything is created on disk.
	// kvit-coder is headless; kvit-coder-ui is the interactive front end.
	if !execMode && !benchmarkEnabled && !thinkbenchEnabled && *benchHaystack == "" {
		printUsage()
		os.Exit(1)
	}

	// -m/-e are shorthand for --model/--effort.
	modelRef := *model
	if modelRef == "" {
		modelRef = *modelShort
	}
	if !effort.set && effortShort.set {
		effort = effortShort
	}

	// Everything the turn needs, set up the way every turn is (turn.go).
	turn, err := prepareTurn(turnOptions{
		Writer:            writer,
		LogFile:           *logFile,
		ConfigPath:        *configPath,
		ConfigName:        configName,
		ModelRef:          modelRef,
		Effort:            effort,
		APIBackend:        *apiBackend,
		BaseURL:           *baseURL,
		AgentFile:         *agentFile,
		Yolo:              *yolo,
		Structured:        *structured,
		BenchmarkMode:     *benchmarkMode,
		HaystackEnabled:   *benchHaystack != "",
		ThinkbenchEnabled: thinkbenchEnabled,
		ThinkbenchSuffix:  thinkbenchSuffix,
		ThinkbenchSuite:   *thinkbenchSuite,
		SessionName:       *sessionName,
		ContinueLast:      *continueLast,
		ReadTerminal:      true,
		Interrupts:        true,
	})
	if err != nil {
		if promptErr, ok := err.(promptSetupError); ok {
			fmt.Fprintln(os.Stderr, promptErr.line)
			os.Exit(1)
		}
		log.Fatalf("%v", err)
	}
	defer turn.Close()
	cfg := turn.cfg
	runner := turn.runner
	systemPrompt := turn.systemPrompt
	runCtx := turn.runCtx
	originalWorkspaceRoot := turn.originalWorkspaceRoot

	// Run benchmark mode if requested
	if benchmarkEnabled {
		writer.StartupInfo("Agent REPL Benchmark Mode")
		writer.StartupInfo(fmt.Sprintf("Model: %s @ %s", cfg.ModelDisplay(), cfg.LLM.BaseURL))
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

		if err := benchmark.Run(runCtx, flags, runner, cfg, systemPrompt, versionLine, originalWorkspaceRoot); err != nil {
			log.Fatalf("Benchmark failed: %v", err)
		}
		return
	}

	// Run haystack (needle retrieval) benchmark mode if requested
	haystackEnabled := *benchHaystack != ""
	if haystackEnabled {
		writer.StartupInfo("Haystack Benchmark Mode (Needle Retrieval)")
		writer.StartupInfo(fmt.Sprintf("Model: %s @ %s", cfg.ModelDisplay(), cfg.LLM.BaseURL))
		fmt.Println()

		flags := benchmark.HaystackCLIFlags{
			Enabled:     true,
			Runs:        *benchmarkRuns,
			BenchmarkID: *benchHaystackID,
			OutputFile:  *benchmarkOutput,
			NoResume:    *benchmarkNoResume,
			Suffix:      haystackSuffix,
		}

		if err := benchmark.RunHaystack(runCtx, flags, cfg, versionLine, originalWorkspaceRoot); err != nil {
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
		writer.StartupInfo(fmt.Sprintf("Model: %s @ %s", cfg.ModelDisplay(), cfg.LLM.BaseURL))
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

		if err := benchmark.RunThinkbench(runCtx, flags, runner, cfg, systemPrompt, versionLine, originalWorkspaceRoot); err != nil {
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

	// Show startup info, unless the UI already showed its own banner: it
	// spawns one agent per turn, so repeating this every turn doubles the
	// header the user sees.
	if !session.FromUI() {
		writer.StartupInfo(fmt.Sprintf("kvit-coder %s", version))
		writer.StartupInfo(fmt.Sprintf("Model: %s @ %s", cfg.ModelDisplay(), cfg.LLM.BaseURL))
		writer.StartupInfo(fmt.Sprintf("Tools: %s", strings.Join(turn.registry.ListTools(), ", ")))
		if *logFile != "" {
			writer.StartupInfo(fmt.Sprintf("Logs: %s", *logFile))
		}

		fmt.Println()
	}

	// Run in exec mode (always, since we require -p or --benchmark). A turn
	// that ended in an error exits non-zero, so whatever started this process
	// can tell: the error itself has already been printed. Exiting runs no
	// deferred call, so the turn is closed first, as returning would.
	if err := repl.RunExec(runCtx, runner, writer, cfg, systemPrompt, promptText, quietMode, turn.sess, projectInstructions, []string(imagePaths)); err != nil {
		turn.Close()
		os.Exit(1)
	}
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
			routeStdinLine(scanner.Text(), steering, writer)
		}
	}()
	return true
}

// pauseAwareWatcher holds progress dots while anyone owns the terminal: a
// question waiting for an answer (Awaiting), the pause prompt itself
// (PauseMode, covering the instant between the mode flag and Ask's counter),
// or the window between an empty Enter and that prompt (PauseRequested),
// where type-ahead echo would otherwise be painted over one dot at a time.
// Dots held this way are dropped and the row restarts after the answer,
// which is also why seconds spent at these prompts do not count as a slow
// tool.
func pauseAwareWatcher(box *inbox.Inbox) func() bool {
	return func() bool { return box.Awaiting() || box.PauseMode() || box.PauseRequested() }
}

// routeStdinLine handles one line from the terminal: empty Enter pauses or
// resumes, anything else is steering or a prompt's answer as before. It is a
// separate function so a test can drive the empty-line truth table without a
// terminal.
func routeStdinLine(text string, steering *inbox.Inbox, writer *ui.Writer) {
	line := strings.TrimSpace(text)
	if line == "" {
		switch {
		case steering.PauseMode():
			// The pause prompt is up: empty is a valid answer (resume).
			// Ask returns "" answered for it — it only checks the kind
			// and the timestamp, not the text.
			steering.Push(inbox.Message{Kind: inbox.KindUserLine, Text: ""})
		case steering.Awaiting():
			// A Question or permission prompt is up: keep ignoring empty
			// lines, otherwise "" would become a free-text answer, which
			// is impossible today.
		default:
			if steering.RequestPause() {
				writer.Info("pause requested — will pause at next steerable point")
			}
		}
		return
	}

	// A prompt waiting for an answer will take this line, so saying it
	// was queued for the model would be wrong — and reads as the
	// answer having been swallowed, which is what it looked like
	// before this check existed.
	// While the pause prompt is up Awaiting is true (Ask holds it), so a
	// steering answer is consumed by Ask and never reports queued.
	answering := steering.Awaiting()
	steering.Push(inbox.Message{Kind: inbox.KindUserLine, Text: line})
	if !answering {
		writer.Info("→ queued")
	}
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
func installInterruptHandler(cancelRun, signalInterrupt context.CancelFunc) {
	sigCh := make(chan os.Signal, 4)
	signal.Notify(sigCh, procutil.InterruptSignals()...)
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
			signalInterrupt()
			cancelRun()
		}
	}()
}

// killBackgroundOnInterrupt returns the cleanup to defer: on a real interrupt
// it stops everything this session started and records that it did,
// synchronously, so the save lands before the process exits. On a normal turn
// end the interrupt context is never done and the cleanup is a no-op, leaving
// persistent processes alone for the next turn. An interrupt that silently
// leaves a dev server holding a port is a surprise you find out about much
// later.
func killBackgroundOnInterrupt(ctx context.Context, registry *procs.Registry, sess *session.Session, writer *ui.Writer) func() {
	return func() {
		select {
		case <-ctx.Done():
		default:
			return
		}
		killed := registry.KillAll()
		if len(killed) == 0 {
			return
		}

		msg := fmt.Sprintf("stopped %d background process(es) on interrupt: %s",
			len(killed), strings.Join(killed, ", "))
		writer.Info(msg)
		_ = sess.Notice(msg)
	}
}

// resolveBackgroundSessions expands a --list-background / --kill-background
// target into session names: "all" means every session with history, anything
// else means that one session, which must exist.
func resolveBackgroundSessions(mgr *session.Manager, target string) ([]string, error) {
	if target == "all" {
		sessions, err := mgr.ListSessions()
		if err != nil {
			return nil, fmt.Errorf("failed to list sessions: %w", err)
		}
		names := make([]string, 0, len(sessions))
		for _, s := range sessions {
			names = append(names, s.Name)
		}
		return names, nil
	}
	if !mgr.SessionExists(target) {
		return nil, fmt.Errorf("session %q not found", target)
	}
	return []string{target}, nil
}

// procRegistryFor opens the background-process registry for a session without
// a running turn. It reads the same proc/ directory the turn uses.
func procRegistryFor(mgr *session.Manager, name string) (*procs.Registry, error) {
	return procs.New(filepath.Join(mgr.BaseDir(), name, session.ProcSubdir))
}

// runListBackground prints the running background processes for one session
// or every session. Persistent processes carry a red "!" marker; ephemeral
// ones found here are leftovers (a crash before the turn-end cleanup, or a
// record from before the ephemeral default) — still running, still listed.
func runListBackground(mgr *session.Manager, target string) error {
	names, err := resolveBackgroundSessions(mgr, target)
	if err != nil {
		return err
	}
	any := false
	for _, name := range names {
		reg, err := procRegistryFor(mgr, name)
		if err != nil {
			continue
		}
		for _, info := range reg.List() {
			if !info.Running() {
				continue
			}
			marker := ""
			if info.Persistent {
				marker = "!"
			}
			label := info.ID + marker
			if info.Name != "" {
				label += fmt.Sprintf(" (%s)", info.Name)
			}
			cmd := strings.TrimSpace(info.Command)
			if len(cmd) > 80 {
				cmd = cmd[:77] + "..."
			}
			fmt.Printf("%s: %s pid=%d %s\n", name, label, info.PID, cmd)
			any = true
		}
	}
	if !any {
		if target == "all" {
			fmt.Println("No running background processes in any session.")
		} else {
			fmt.Printf("No running background processes in session %s.\n", target)
		}
	}
	return nil
}

// killSessionBackground stops every running background process in one
// session. It is the cleanup behind both --kill-background and
// --session-delete: deleting the pidfiles without stopping the processes
// would orphan them with no record pointing at them.
func killSessionBackground(mgr *session.Manager, name string) []string {
	reg, err := procRegistryFor(mgr, name)
	if err != nil {
		return nil
	}
	return reg.KillAll()
}

// runKillBackground stops every running background process for one session or
// every session: the reaper for red (persistent) processes left behind by
// abandoned sessions, plus any ephemeral leftovers from crashes.
func runKillBackground(mgr *session.Manager, target string) error {
	names, err := resolveBackgroundSessions(mgr, target)
	if err != nil {
		return err
	}
	total := 0
	for _, name := range names {
		killed := killSessionBackground(mgr, name)
		for _, id := range killed {
			fmt.Printf("%s: stopped %s\n", name, id)
			total++
		}
	}
	if total == 0 {
		if target == "all" {
			fmt.Println("No running background processes in any session.")
		} else {
			fmt.Printf("No running background processes in session %s.\n", target)
		}
	}
	return nil
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
			writer.Warn(fmt.Sprintf("permissions: %v", err))
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

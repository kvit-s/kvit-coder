package benchmark

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/config"
)

// ThinkbenchCLIFlags holds the command-line flags for thinkbench benchmarks.
type ThinkbenchCLIFlags struct {
	Enabled     bool
	Runs        int    // trials per task (already resolved: -n / config.trials / default)
	BenchmarkID string // comma-separated slugs
	Types       string // comma-separated type override
	SuiteDir    string // override suite directory
	OutputFile  string
	NoResume    bool // accepted for parity; thinkbench is fresh-by-default so this is a no-op
	Resume      bool // opt in to resuming the most recent interrupted run
	Suffix      string
}

// RunThinkbench runs the thinkbench benchmark family: it provisions the uv
// environment, sets up the OS sandbox, runs each (task × trial) through the
// agent loop in a hard workspace sandbox, grades with the held-out grader, and
// writes results.json + RESULTS.md. It mirrors RunHaystack's orchestration.
func RunThinkbench(ctx context.Context, flags ThinkbenchCLIFlags, runner *agent.Runner, cfg *config.Config, systemPrompt, version, originalWorkspaceRoot string) error {
	defer restoreTerminal()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		restoreTerminal()
		os.Exit(1)
	}()

	// Anchor runtime dirs to the suite location (not cwd), matching main.go.
	suiteDir, tbRoot := ResolveThinkbench(originalWorkspaceRoot, flags.SuiteDir, cfg.Thinkbench.SuiteDir)
	if suiteDir == "" {
		return fmt.Errorf("thinkbench suite not found (looked for benchmarks/thinkbench/suite/manifest.json); set thinkbench.suite_dir or --thinkbench-suite")
	}
	resultsDir := filepath.Join(tbRoot, "results")
	observedDir := filepath.Join(resultsDir, "observed")
	uvDir := filepath.Join(tbRoot, ".uv")
	workspace := cfg.Workspace.Root // already overridden to the thinkbench run workspace

	timestamp := time.Now().Format("20060102-150405")

	// Reports are timestamped + suffix-named in the working dir, matching the
	// benchmark-*.md / haystack-*.md convention. The .json is a matched pair next
	// to the .md. The CSV (for resume) and observed/failure workspaces stay under
	// the (gitignored) benchmarks/thinkbench/results tree.
	suffixPart := ""
	if flags.Suffix != "" {
		suffixPart = "-" + flags.Suffix
	}
	outputPath := flags.OutputFile
	if outputPath == "" {
		outputPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("thinkbench%s-%s.md", suffixPart, timestamp))
	}
	jsonPath := strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".json"
	csvPath := filepath.Join(resultsDir, fmt.Sprintf("thinkbench-%s.csv", timestamp))
	terminalPath := filepath.Join(originalWorkspaceRoot, fmt.Sprintf("terminal-thinkbench%s-%s.txt", suffixPart, timestamp))

	for _, d := range []string{resultsDir, observedDir, filepath.Dir(outputPath), filepath.Dir(terminalPath)} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", d, err)
		}
	}

	terminalFile, err := os.Create(terminalPath)
	if err != nil {
		return fmt.Errorf("failed to create terminal log: %w", err)
	}
	defer terminalFile.Close()

	// Tee both the harness progress and the agent's per-step transcript into the
	// terminal log (ANSI codes stripped for the file). stdout/stderr keep their
	// colors on screen.
	terminalWriter := NewANSIStripWriter(terminalFile)
	stdoutWriter := io.MultiWriter(os.Stdout, terminalWriter)
	stderrWriter := io.MultiWriter(os.Stderr, terminalWriter)
	out := stdoutWriter

	// Route the agent runner's output through the same writers so each task's
	// transcript is captured (mirrors the tool benchmark in Run()).
	rw := runner.Writer()
	rw.SetHeadless(false)
	rw.SetQuiet(false)
	rw.SetStdout(stdoutWriter)
	rw.SetStderr(stderrWriter)
	rw.SetColorOutput(stdoutWriter)

	// --- Load suite -----------------------------------------------------------
	allTasks, err := LoadThinkbenchSuite(suiteDir)
	if err != nil {
		return fmt.Errorf("failed to load suite: %w", err)
	}
	fmt.Fprintf(out, "Loaded %d thinkbench tasks from %s\n", len(allTasks), suiteDir)

	var ids, types []string
	if flags.BenchmarkID != "" {
		ids = strings.Split(flags.BenchmarkID, ",")
	}
	if flags.Types != "" {
		types = strings.Split(flags.Types, ",")
	} else {
		types = cfg.Thinkbench.Types
	}
	tasks := FilterThinkbench(allTasks, ids, types, cfg.Thinkbench.IncludeObserved)
	if len(tasks) == 0 {
		return fmt.Errorf("no thinkbench tasks match the specified filters")
	}
	fmt.Fprintf(out, "Running %d tasks × %d trials\n", len(tasks), flags.Runs)

	// --- Preflight: uv + sandbox ---------------------------------------------
	env, err := provisionUV(ctx, uvDir, cfg.Thinkbench.GetUVPython(), out)
	if err != nil {
		return fmt.Errorf("uv preflight failed: %w", err)
	}
	// Pre-install common test deps (default: pytest) so agents don't search the
	// filesystem outside the sandbox for them.
	installPipPackages(ctx, env, cfg.Thinkbench.GetPipPackages(), out)

	sandbox, err := resolveSandbox(cfg.Thinkbench.GetSandbox(), env, workspace, out)
	if err != nil {
		return fmt.Errorf("sandbox preflight failed: %w", err)
	}
	env.Sandbox = sandbox

	// Smoke-test: a read of a path OUTSIDE the workspace must be denied.
	if sandbox.Tool != "none" {
		probePath, err := writeSandboxProbeFile(tbRoot)
		if err != nil {
			return fmt.Errorf("failed to write sandbox probe: %w", err)
		}
		defer os.Remove(probePath)
		if err := smokeTestSandbox(ctx, sandbox, env, workspace, probePath, out); err != nil {
			return fmt.Errorf("sandbox preflight smoke test failed: %w", err)
		}
	}

	// Hard sandbox for structured tools + inject uv env / OS wrapper into shell.
	cfg.Workspace.PathSafetyMode = "block"
	cfg.Workspace.AllowOutsideWorkspace = false
	cfg.Workspace.AllowedPaths = nil
	cfg.Workspace.AllowedReadPaths = nil
	cfg.Tools.Shell.InjectEnv = env.InjectEnv
	cfg.Tools.Shell.ExecPrefix = sandbox.ExecPrefix
	cfg.Tools.Shell.AllowInterpreters = true

	timeout := time.Duration(cfg.Thinkbench.GetTimeoutPerRun()) * time.Second
	executor := NewTBExecutor(runner, cfg, systemPrompt, env, workspace, observedDir, timeout, stdoutWriter, stderrWriter)

	// --- Resume (opt-in) ------------------------------------------------------
	// A plain run starts fresh (matching the other bench families). Resuming the
	// most recent interrupted run is explicit, via --bench-thinkbench-resume.
	completed := make(map[string]bool)
	var allResults []TBRunResult
	if flags.Resume {
		if prior := latestThinkbenchCSV(resultsDir); prior != "" {
			loaded, _ := LoadTBResults(prior)
			for _, r := range loaded {
				completed[tbCompletedKey(r.Slug, r.Run)] = true
				allResults = append(allResults, r)
			}
			csvPath = prior // append to the same CSV so the resume chain stays intact
			fmt.Fprintf(out, "Resuming from %s: %d runs already completed\n", filepath.Base(prior), len(loaded))
		} else {
			fmt.Fprintf(out, "--bench-thinkbench-resume requested but no prior thinkbench CSV found; starting fresh\n")
		}
	}

	// Append when resuming an existing CSV; create a new one otherwise.
	csvWriter, err := NewTBCSVWriter(csvPath, flags.Resume)
	if err != nil {
		return fmt.Errorf("failed to open CSV: %w", err)
	}
	defer csvWriter.Close()

	// --- Run loop -------------------------------------------------------------
	totalRuns := len(tasks) * flags.Runs
	done := len(completed)
	startAll := time.Now()

	for run := 1; run <= flags.Runs; run++ {
		for _, task := range tasks {
			select {
			case <-ctx.Done():
				fmt.Fprintf(out, "\nThinkbench run cancelled\n")
				goto report
			default:
			}

			if completed[tbCompletedKey(task.Slug, run)] {
				continue
			}

			done++
			fmt.Fprintf(out, "\n[%d/%d] %s (%s) trial %d\n", done, totalRuns, task.Slug, task.Type, run)

			result := executor.Execute(ctx, task, run)
			allResults = append(allResults, *result)
			if err := csvWriter.WriteResult(result); err != nil {
				fmt.Fprintf(out, "Warning: CSV write failed: %v\n", err)
			}

			if task.Observed {
				fmt.Fprintf(out, "  observed (no score) — %.0fs, %d tokens, persisted to %s\n",
					float64(result.DurationMS)/1000, result.Tokens, result.WorkspaceDir)
			} else {
				status := "FAIL"
				if result.FullPass {
					status = "FULL-PASS"
				}
				fmt.Fprintf(out, "  %s score=%.3f (%d/%d) — %.0fs, %d tokens, $%.4f\n",
					status, result.Score, result.Passed, result.Total,
					float64(result.DurationMS)/1000, result.Tokens, result.Cost)
				if len(result.Errors) > 0 {
					fmt.Fprintf(out, "  errors: %s\n", truncateString(strings.Join(result.Errors, "; "), 300))
				}
			}
		}
	}

report:
	fmt.Fprintf(out, "\nTotal time: %s\n", formatDuration(time.Since(startAll)))

	// --- Reports --------------------------------------------------------------
	model := cfg.LLM.Model
	cfgMap := map[string]any{
		"provider":      providerFromURL(cfg.LLM.BaseURL),
		"endpoint":      cfg.LLM.BaseURL,
		"thinking_mode": "none",
		"trials":        flags.Runs,
		"model":         model,
	}
	results := BuildTBResults(model, time.Now().Unix(), flags.Runs, tasks, allResults, cfgMap)

	// Embed the config file used (config-<suffix>.yaml, else config.yaml).
	configYAML := ""
	configFileName := "config.yaml"
	if flags.Suffix != "" {
		configFileName = fmt.Sprintf("config-%s.yaml", flags.Suffix)
	}
	if data, err := os.ReadFile(filepath.Join(originalWorkspaceRoot, configFileName)); err == nil {
		configYAML = string(data)
	}

	if err := WriteTBResultsJSON(jsonPath, results); err != nil {
		return fmt.Errorf("failed to write results.json: %w", err)
	}
	if err := WriteTBResultsMarkdown(outputPath, model, results, allResults, configYAML); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	fmt.Fprintf(out, "\nReport written to: %s\n", outputPath)
	fmt.Fprintf(out, "JSON results:      %s\n", jsonPath)
	fmt.Fprintf(out, "CSV data:          %s\n", csvPath)
	fmt.Fprintf(out, "Terminal log:      %s\n", terminalPath)
	return nil
}

// latestThinkbenchCSV returns the path of the most recent thinkbench result CSV
// in the results dir (by timestamped filename), or "" if none exists.
func latestThinkbenchCSV(resultsDir string) string {
	entries, err := os.ReadDir(resultsDir)
	if err != nil {
		return ""
	}
	var newest string
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, "thinkbench-") && strings.HasSuffix(name, ".csv") {
			if name > newest {
				newest = name
			}
		}
	}
	if newest == "" {
		return ""
	}
	return filepath.Join(resultsDir, newest)
}

// providerFromURL makes a human-readable provider label from the endpoint URL.
func providerFromURL(url string) string {
	u := strings.ToLower(url)
	switch {
	case strings.Contains(u, "fireworks"):
		return "Fireworks AI"
	case strings.Contains(u, "openai"):
		return "OpenAI"
	case strings.Contains(u, "anthropic"):
		return "Anthropic"
	case strings.Contains(u, "localhost"), strings.Contains(u, "127.0.0.1"), strings.Contains(u, "192.168."), strings.Contains(u, "0.0.0.0"):
		return "local"
	default:
		return "custom"
	}
}

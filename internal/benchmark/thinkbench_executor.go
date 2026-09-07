package benchmark

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// TBExecutor runs a single thinkbench (task, trial) end-to-end: sandboxed setup
// → agent loop → grade (or persist, for observed tasks).
type TBExecutor struct {
	runner       *agent.Runner
	cfg          *config.Config
	systemPrompt string
	env          *thinkbenchEnv
	workspace    string // the single per-run workspace (== cfg.Workspace.Root)
	observedDir  string // results/observed root for persisting ambiguous-spec runs
	timeout      time.Duration
	stdoutWriter io.Writer
	stderrWriter io.Writer
}

// NewTBExecutor builds a thinkbench executor.
func NewTBExecutor(runner *agent.Runner, cfg *config.Config, systemPrompt string, env *thinkbenchEnv, workspace, observedDir string, timeout time.Duration, stdout, stderr io.Writer) *TBExecutor {
	return &TBExecutor{
		runner:       runner,
		cfg:          cfg,
		systemPrompt: systemPrompt,
		env:          env,
		workspace:    workspace,
		observedDir:  observedDir,
		timeout:      timeout,
		stdoutWriter: stdout,
		stderrWriter: stderr,
	}
}

// setupWorkspace cleans the single workspace and copies the task's setup/ tree
// into a package directory named after the slug (the grader imports the
// `<slug>` package from the workspace cwd; see the suite README). The grader and
// reference live under the suite dir, never inside the workspace.
func (e *TBExecutor) setupWorkspace(task TBTask) error {
	if err := os.RemoveAll(e.workspace); err != nil {
		return fmt.Errorf("failed to clean workspace: %w", err)
	}
	if err := os.MkdirAll(e.workspace, 0755); err != nil {
		return fmt.Errorf("failed to create workspace: %w", err)
	}
	if task.HasSetup {
		pkgDir := filepath.Join(e.workspace, task.Slug)
		if err := copyTree(task.SetupDir, pkgDir); err != nil {
			return fmt.Errorf("failed to copy setup tree: %w", err)
		}
	}
	return nil
}

// Execute runs one (task, trial) and returns its result.
func (e *TBExecutor) Execute(ctx context.Context, task TBTask, runID int) *TBRunResult {
	result := &TBRunResult{
		Slug:     task.Slug,
		Type:     string(task.Type),
		Run:      runID,
		Observed: task.Observed,
		Total:    task.NumChecks,
	}

	if err := e.setupWorkspace(task); err != nil {
		result.Errors = []string{err.Error()}
		return result
	}

	startTime := time.Now()

	// Run the agent (or an external CLI) against the brief.
	var finalOutput string
	if e.cfg.LLM.BenchmarkCmd != "" {
		finalOutput = e.runExternal(ctx, task, runID, result)
	} else {
		finalOutput = e.runAgent(ctx, task, result)
	}
	_ = finalOutput

	result.DurationMS = time.Since(startTime).Milliseconds()

	if task.Observed {
		// No grader: persist the workspace for later qualitative comparison.
		if dst, err := e.persistObserved(task, runID); err == nil {
			result.WorkspaceDir = dst
		} else {
			result.Errors = append(result.Errors, fmt.Sprintf("failed to persist observed workspace: %v", err))
		}
		return result
	}

	// Grade.
	card, gradeErrs := gradeTask(ctx, task, e.workspace, e.env, e.timeout)
	if len(gradeErrs) > 0 {
		result.Errors = append(result.Errors, gradeErrs...)
	}
	result.Score = card.Score
	result.Passed = card.Passed
	result.Total = card.Total
	result.ImportOK = card.ImportOK
	result.FullPass = card.Total > 0 && card.Passed == card.Total
	result.FailedChecks = summarizeFailedChecks(card)

	// Keep the workspace for debugging on anything less than a full pass.
	if !result.FullPass {
		if dst, err := e.persistFailure(task, runID); err == nil {
			result.WorkspaceDir = dst
		}
	}

	return result
}

// runAgent drives the internal agent loop and returns the final assistant output.
func (e *TBExecutor) runAgent(ctx context.Context, task TBTask, result *TBRunResult) string {
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: e.systemPrompt},
		{Role: llm.RoleUser, Content: task.Brief},
	}

	// Each task is a fresh conversation; clear any plan left over from a prior task.
	e.runner.ResetPlan()

	agentResult, err := e.runner.Run(timeoutCtx, agent.RunConfig{
		Messages:     messages,
		UseFileFirst: false,
		QuietMode:    false,
	})

	// Report the harness's own budget as such. A run stopped by timeoutCtx used
	// to surface as a bare "context deadline exceeded" printed by the runner and
	// nothing at all in the CSV, which made a task the model simply could not
	// finish in time look like a failure of the model endpoint.
	switch {
	case agentResult != nil && agentResult.TimedOut,
		errors.Is(timeoutCtx.Err(), context.DeadlineExceeded):
		result.TimedOut = true
		result.Errors = append(result.Errors,
			fmt.Sprintf("run exceeded the %s budget (thinkbench.timeout_per_run)", e.timeout))
	case err != nil:
		result.Errors = append(result.Errors, err.Error())
	case agentResult != nil && agentResult.Cancelled:
		result.Errors = append(result.Errors, "cancelled")
	}

	var finalOutput string
	if agentResult != nil {
		finalOutput = extractFinalOutput(agentResult.FinalMessages)
		if agentResult.Stats != nil {
			s := agentResult.Stats
			result.LLMCalls = s.Steps
			result.PromptTokens = s.TotalPromptTokens
			result.GeneratedTokens = s.TotalCompletionTokens
			result.Tokens = s.TotalPromptTokens + s.TotalCompletionTokens
			result.CachedTokens = s.TotalCacheReadTokens
			result.ContextUsed = s.MaxContextUsed
			result.Cost = s.TotalCost
		}
	}
	return finalOutput
}

// runExternal runs the configured external agent CLI in the sandboxed workspace
// (with the uv env injected), mirroring executeExternalCommand but without the
// internal LLM stats. Retries on non-zero exit, like the tool benchmark.
func (e *TBExecutor) runExternal(ctx context.Context, task TBTask, runID int, result *TBRunResult) string {
	cmdStr := strings.ReplaceAll(e.cfg.LLM.BenchmarkCmd, "{prompt}", shellEscape(task.Brief))

	const maxRetries = 5
	var output string
	var cmdErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			cmdErr = ctx.Err()
			break
		}
		timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
		cmd := exec.CommandContext(timeoutCtx, "sh", "-c", cmdStr)
		cmd.Dir = e.workspace
		cmd.Env = append(os.Environ(), e.env.InjectEnv...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = io.MultiWriter(&stdout, e.stdoutWriter)
		cmd.Stderr = io.MultiWriter(&stderr, e.stderrWriter)
		cmdErr = cmd.Run()
		cancel()
		output = stdout.String() + stderr.String()
		if cmdErr == nil {
			break
		}
		if attempt < maxRetries {
			backoff := time.Duration(1<<(attempt-1)) * time.Second
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				cmdErr = ctx.Err()
			}
		}
	}
	if cmdErr != nil {
		result.Errors = append(result.Errors, fmt.Sprintf("external command failed: %v", cmdErr))
	}
	result.LLMCalls = 1
	return output
}

// persistObserved copies the post-run workspace to results/observed/<slug>/run-<n>.
func (e *TBExecutor) persistObserved(task TBTask, runID int) (string, error) {
	dst := filepath.Join(e.observedDir, task.Slug, fmt.Sprintf("run-%d", runID))
	if err := os.RemoveAll(dst); err != nil {
		return "", err
	}
	if err := copyTree(e.workspace, dst); err != nil {
		return "", err
	}
	return dst, nil
}

// persistFailure copies the workspace to results/failures/<slug>/run-<n> so a
// less-than-full-pass run can be inspected.
func (e *TBExecutor) persistFailure(task TBTask, runID int) (string, error) {
	dst := filepath.Join(filepath.Dir(e.observedDir), "failures", task.Slug, fmt.Sprintf("run-%d", runID))
	if err := os.RemoveAll(dst); err != nil {
		return "", err
	}
	if err := copyTree(e.workspace, dst); err != nil {
		return "", err
	}
	return dst, nil
}

package benchmark

import (
	"bytes"
	"context"
	"encoding/json"
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

// Executor handles running a single benchmark.
type Executor struct {
	runner       *agent.Runner
	cfg          *config.Config
	systemPrompt string
	env          *Environment
	timeout      time.Duration
}

// NewExecutor creates a new benchmark executor.
func NewExecutor(runner *agent.Runner, cfg *config.Config, systemPrompt string, env *Environment, timeout time.Duration) *Executor {
	return &Executor{
		runner:       runner,
		cfg:          cfg,
		systemPrompt: systemPrompt,
		env:          env,
		timeout:      timeout,
	}
}

// ExecuteResult contains the result of executing a benchmark.
type ExecuteResult struct {
	RunResult    *RunResult
	FinalOutput  string
	ToolCalls    []ToolCallLog
	Messages     []llm.Message
	WorkspaceDir string
}

// Execute runs a single benchmark and returns the result.
func (e *Executor) Execute(ctx context.Context, benchmark BenchmarkDef, runID int) (*ExecuteResult, error) {
	// Setup workspace for this run
	workspaceDir, err := e.env.SetupBenchmark(benchmark, runID)
	if err != nil {
		return nil, fmt.Errorf("failed to setup benchmark: %w", err)
	}

	// Check if we're using an external command
	if e.cfg.LLM.BenchmarkCmd != "" {
		return e.executeExternalCommand(ctx, benchmark, runID, workspaceDir)
	}

	// Create timeout context
	timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)
	defer cancel()

	// Build messages
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: e.systemPrompt},
		{Role: llm.RoleUser, Content: benchmark.Task},
	}

	// Track start time
	startTime := time.Now()

	// Run agent
	agentResult, err := e.runner.Run(timeoutCtx, agent.RunConfig{
		Messages:     messages,
		UseFileFirst: false,
		QuietMode:    true,
	})

	completedAt := time.Now()
	durationMS := completedAt.Sub(startTime).Milliseconds()

	// Extract tool calls from messages
	toolCalls := extractToolCallsFromMessages(agentResult.FinalMessages)

	// Get final output (last assistant message content)
	finalOutput := extractFinalOutput(agentResult.FinalMessages)

	// Create run result
	result := &RunResult{
		BenchmarkID: benchmark.ID,
		Run:         runID,
		StartedAt:   startTime,
		CompletedAt: completedAt,
		DurationMS:  durationMS,
	}

	if err != nil {
		result.Success = false
		result.Errors = []string{err.Error()}
	} else if agentResult.Cancelled {
		result.Success = false
		result.Errors = []string{"cancelled"}
	} else {
		// Validate results
		validator := NewValidator(workspaceDir, finalOutput, toolCalls)
		success, validationErrors := validator.Validate(benchmark.Validation)

		result.Success = success
		result.Errors = validationErrors
	}

	// Fill in stats from agent result
	if agentResult != nil && agentResult.Stats != nil {
		stats := agentResult.Stats
		result.LLMCalls = stats.Steps
		result.Tokens = stats.TotalPromptTokens + stats.TotalCompletionTokens
		result.PromptTokens = stats.TotalPromptTokens
		result.GeneratedTokens = stats.TotalCompletionTokens
		result.CachedTokens = stats.TotalCacheReadTokens
		result.ContextUsed = stats.MaxContextUsed
		result.Cost = stats.TotalCost
		result.PromptMS = stats.TotalPromptMS
		result.GenerationMS = stats.TotalGenerationMS
		result.ToolCalls = toolCalls
	}

	return &ExecuteResult{
		RunResult:    result,
		FinalOutput:  finalOutput,
		ToolCalls:    toolCalls,
		Messages:     agentResult.FinalMessages,
		WorkspaceDir: workspaceDir,
	}, nil
}

// shellEscape escapes a string for safe use in a shell command.
func shellEscape(s string) string {
	// Replace single quotes with '\'' (end quote, escaped quote, start quote)
	escaped := strings.ReplaceAll(s, "'", "'\\''")
	return "'" + escaped + "'"
}

// executeExternalCommand runs an external command instead of the internal agent.
// The command template from cfg.LLM.BenchmarkCmd is used with {prompt} replaced by the task.
// It retries with exponential backoff on non-zero exit codes (max 5 attempts).
func (e *Executor) executeExternalCommand(ctx context.Context, benchmark BenchmarkDef, runID int, workspaceDir string) (*ExecuteResult, error) {
	// Ensure workspace directory is absolute
	absWorkspaceDir, err := filepath.Abs(workspaceDir)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve workspace directory: %w", err)
	}

	// Build the command by replacing {prompt} with the escaped task
	cmdTemplate := e.cfg.LLM.BenchmarkCmd
	escapedPrompt := shellEscape(benchmark.Task)
	cmdStr := strings.ReplaceAll(cmdTemplate, "{prompt}", escapedPrompt)

	const maxRetries = 5
	var finalOutput string
	var cmdErr error
	var startTime, completedAt time.Time
	errorCount := 0

	// Track overall start time
	startTime = time.Now()

	for attempt := 1; attempt <= maxRetries; attempt++ {
		// Check if parent context is cancelled
		if ctx.Err() != nil {
			cmdErr = ctx.Err()
			break
		}

		// Create timeout context for this attempt
		timeoutCtx, cancel := context.WithTimeout(ctx, e.timeout)

		// Execute the command in the benchmark workspace directory
		cmd := exec.CommandContext(timeoutCtx, "sh", "-c", cmdStr)
		cmd.Dir = absWorkspaceDir

		// Capture output while also displaying to user
		var stdout, stderr bytes.Buffer
		cmd.Stdout = io.MultiWriter(&stdout, os.Stdout)
		cmd.Stderr = io.MultiWriter(&stderr, os.Stderr)

		cmdErr = cmd.Run()
		cancel()

		// Combine stdout and stderr for verification
		finalOutput = stdout.String()
		if stderr.Len() > 0 {
			if finalOutput != "" {
				finalOutput += "\n"
			}
			finalOutput += stderr.String()
		}

		// If command succeeded, break out of retry loop
		if cmdErr == nil {
			break
		}

		errorCount++

		// Determine error type for logging
		errMsg := cmdErr.Error()
		if timeoutCtx.Err() == context.DeadlineExceeded {
			errMsg = "timeout"
		}

		// Log the error and retry info
		if attempt < maxRetries {
			backoff := time.Duration(1<<(attempt-1)) * time.Second // 1s, 2s, 4s, 8s
			fmt.Printf("  [benchmark %s run %d] command failed (attempt %d/%d, %d errors): %s, retrying in %v\n",
				benchmark.ID, runID, attempt, maxRetries, errorCount, errMsg, backoff)

			// Wait with backoff before retrying
			select {
			case <-time.After(backoff):
				// Continue to next attempt
			case <-ctx.Done():
				cmdErr = ctx.Err()
				break
			}
		} else {
			fmt.Printf("  [benchmark %s run %d] command failed (attempt %d/%d, %d errors): %s, giving up\n",
				benchmark.ID, runID, attempt, maxRetries, errorCount, errMsg)
		}
	}

	completedAt = time.Now()
	durationMS := completedAt.Sub(startTime).Milliseconds()

	// Create run result (no tool calls for external commands)
	var toolCalls []ToolCallLog
	result := &RunResult{
		BenchmarkID: benchmark.ID,
		Run:         runID,
		StartedAt:   startTime,
		CompletedAt: completedAt,
		DurationMS:  durationMS,
		ToolCalls:   toolCalls,
	}

	if cmdErr != nil {
		// Check if it was a timeout
		if ctx.Err() == context.DeadlineExceeded {
			result.Success = false
			result.Errors = []string{"timeout"}
		} else if ctx.Err() == context.Canceled {
			result.Success = false
			result.Errors = []string{"cancelled"}
		} else {
			result.Success = false
			result.Errors = []string{fmt.Sprintf("command failed after %d retries (%d errors): %v", maxRetries, errorCount, cmdErr)}
		}
	} else {
		// Log if we succeeded after retries
		if errorCount > 0 {
			fmt.Printf("  [benchmark %s run %d] command succeeded after %d errors\n",
				benchmark.ID, runID, errorCount)
		}

		// Validate results
		validator := NewValidator(workspaceDir, finalOutput, toolCalls)
		success, validationErrors := validator.Validate(benchmark.Validation)

		result.Success = success
		result.Errors = validationErrors
	}

	// External commands don't report LLM stats
	result.LLMCalls = 1 // Count as 1 "call" for reporting purposes
	result.Tokens = 0
	result.Cost = 0

	return &ExecuteResult{
		RunResult:    result,
		FinalOutput:  finalOutput,
		ToolCalls:    toolCalls,
		Messages:     nil, // No LLM messages for external commands
		WorkspaceDir: workspaceDir,
	}, nil
}

// extractToolCallsFromMessages extracts tool calls from LLM messages.
func extractToolCallsFromMessages(messages []llm.Message) []ToolCallLog {
	var calls []ToolCallLog

	for _, msg := range messages {
		if msg.Role != llm.RoleAssistant {
			continue
		}

		for _, tc := range msg.ToolCalls {
			calls = append(calls, ToolCallLog{
				Tool: tc.Function.Name,
				Args: json.RawMessage(tc.Function.Arguments),
			})
		}
	}

	return calls
}

// extractFinalOutput gets the final assistant message content.
func extractFinalOutput(messages []llm.Message) string {
	// Find last assistant message with content
	// Check both Content and ReasoningContent since some models put responses in reasoning
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == llm.RoleAssistant {
			// Prefer Content, fall back to ReasoningContent
			if messages[i].Content != "" {
				return messages[i].Content
			}
			if messages[i].ReasoningContent != "" {
				return messages[i].ReasoningContent
			}
		}
	}
	return ""
}

// Warmup runs a warmup task to prime the cache.
func (e *Executor) Warmup(ctx context.Context, task string) error {
	// Skip warmup when using external commands - they don't need priming
	if e.cfg.LLM.BenchmarkCmd != "" {
		return nil
	}

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: e.systemPrompt},
		{Role: llm.RoleUser, Content: task},
	}

	_, err := e.runner.Run(ctx, agent.RunConfig{
		Messages:     messages,
		UseFileFirst: false,
		QuietMode:    true,
	})

	return err
}

// IsExternalCommand returns true if the executor is configured to use an external command.
func (e *Executor) IsExternalCommand() bool {
	return e.cfg.LLM.BenchmarkCmd != ""
}

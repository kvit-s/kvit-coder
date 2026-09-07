// Package agent provides the agent runner for executing LLM interactions.
package agent

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/kvit-s/kvit-coder/internal/checkpoint"
	"github.com/kvit-s/kvit-coder/internal/config"
	ctxtools "github.com/kvit-s/kvit-coder/internal/context"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/stats"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// LLMClient is the slice of the LLM client the agent loop actually uses.
// Declaring it here rather than depending on *llm.Client lets a test drive the
// loop with a scripted sequence of responses. *llm.Client satisfies it as-is.
type LLMClient interface {
	Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)
	GetGenerationStats(context.Context, string) (*llm.GenerationStats, error)
}

// Runner executes the agent loop for LLM interactions
type Runner struct {
	cfg               *config.Config
	llmClient         LLMClient
	registry          *tools.Registry
	writer            *ui.Writer
	logger            *Logger
	checkpointMgr     *checkpoint.Manager
	contextMgr        *ctxtools.Manager
	contextMiddleware *ctxtools.Middleware
	planManager       *tools.PlanManager
	toolCtx           *tools.ToolContext
	interrogator      *Interrogator
}

// RunnerOptions contains all dependencies for creating a Runner
type RunnerOptions struct {
	Cfg               *config.Config
	LLMClient         LLMClient
	Registry          *tools.Registry
	Writer            *ui.Writer
	Logger            *Logger
	CheckpointMgr     *checkpoint.Manager
	ContextMgr        *ctxtools.Manager
	ContextMiddleware *ctxtools.Middleware
	PlanManager       *tools.PlanManager
	ToolCtx           *tools.ToolContext
}

// RunConfig contains per-run configuration options
type RunConfig struct {
	Messages     []llm.Message
	UseFileFirst bool
	QuietMode    bool
}

// RunResult contains the results of running the agent loop
type RunResult struct {
	Stats         *stats.AgentStats
	FinalMessages []llm.Message
	Cancelled     bool
	// TimedOut is set when the loop stopped because the context passed to Run
	// hit its deadline, as opposed to being cancelled by the user. Callers that
	// impose a time budget (the thinkbench executor) use it to report the
	// budget rather than the bare "context deadline exceeded".
	TimedOut bool
}

// NewRunner creates a new agent runner
func NewRunner(opts RunnerOptions) *Runner {
	runID := fmt.Sprintf("run-%s", time.Now().UTC().Format("20060102-150405"))
	r := &Runner{
		cfg:               opts.Cfg,
		llmClient:         opts.LLMClient,
		registry:          opts.Registry,
		writer:            opts.Writer,
		logger:            opts.Logger,
		checkpointMgr:     opts.CheckpointMgr,
		contextMgr:        opts.ContextMgr,
		contextMiddleware: opts.ContextMiddleware,
		planManager:       opts.PlanManager,
		toolCtx:           opts.ToolCtx,
	}
	// nil when diagnostics are disabled; all call sites are nil-safe.
	r.interrogator = NewInterrogator(opts.Cfg, opts.LLMClient, opts.Writer, opts.Logger, runID)
	return r
}

// Writer returns the UI writer for output configuration.
func (r *Runner) Writer() *ui.Writer {
	return r.writer
}

// ResetPlan clears any active plan. Used by the benchmark harness to give each
// task a fresh PlanManager (the manager is shared across tasks in a single
// process). No-op when planning is disabled.
func (r *Runner) ResetPlan() {
	if r.planManager != nil {
		r.planManager.ClearPlan()
	}
}

// backtrackResult contains the result of handleToolError
type backtrackResult struct {
	shouldBacktrack   bool
	injectUserMessage bool
	userMessage       string
	toolName          string
	reason            string
}

// runState holds mutable state for a single Run execution
type runState struct {
	messages                    []llm.Message
	loopDetector                *LoopDetector
	backtracker                 *BacktrackTracker
	lastToolName                string
	lastToolArgs                string
	consecutiveDuplicates       int
	consecutiveProviderFailures int
	contextOverflowRetries      int
	differentApproachAttempts   int
	emptyReasoningRetries       int
	requestStartTime            time.Time
	totalLLMTime                time.Duration
	totalToolTime               time.Duration
	totalToolCalls              int
	totalTokens                 int
	agentStats                  *stats.AgentStats
	normalizer                  *llm.ResponseNormalizer

	// Anomaly interrogation state (Improvement 2), per-task.
	interrogationCount int
	seenInterrogations map[string]bool
}

// llmCallResult holds the outcome of an LLM call
type llmCallResult struct {
	response       *llm.ChatResponse
	duration       time.Duration
	shouldContinue bool // retry needed
	shouldBreak    bool // fatal error, exit loop
	cancelled      bool
	timedOut       bool // the run context hit its deadline
}

// toolExecutionResult holds the outcome of executing all tools
type toolExecutionResult struct {
	shouldBacktrack     bool
	userMessageToInject string
	toolsCancelled      bool
	lastExecutedIdx     int
	tasksToolExecuted   bool
}

// Constants for loop and retry limits
const (
	maxConsecutiveDuplicates     = 3
	maxProviderFailures          = 2
	maxContextOverflowRetries    = 2
	maxDifferentApproachAttempts = 3
	maxEmptyReasoningRetries     = 3
)

// initRunState initializes all mutable state for a single Run execution
func (r *Runner) initRunState(rcfg RunConfig) *runState {
	return &runState{
		messages:     rcfg.Messages,
		loopDetector: NewLoopDetector(),
		backtracker: NewBacktrackTracker(BacktrackConfig{
			Enabled:           r.cfg.Backtrack.Enabled,
			MaxRetries:        r.cfg.Backtrack.MaxRetries,
			InjectUserMessage: r.cfg.Backtrack.InjectUserMessage,
		}),
		requestStartTime: time.Now(),
		agentStats:       &stats.AgentStats{},
		normalizer:       llm.NewResponseNormalizer(r.registry, r.cfg.LLM.MergeThinking),
	}
}

// Run executes the agent loop for a single user input.
// It returns the updated messages and stats after completion.
func (r *Runner) Run(ctx context.Context, rcfg RunConfig) (*RunResult, error) {
	state := r.initRunState(rcfg)

	result := &RunResult{
		Stats:     state.agentStats,
		Cancelled: false,
	}

	maxIters := r.cfg.Agent.MaxIterations
	if maxIters == 0 {
		maxIters = 10
	}

	for i := 0; i < maxIters; i++ {
		r.logger.AgentIteration(i, 0)

		// File-first mode: read messages from file at start of each iteration
		if rcfg.UseFileFirst && r.contextMgr != nil {
			fileMessages, err := r.contextMgr.ReadMessagesForLLM()
			if err != nil {
				r.writer.Error(fmt.Sprintf("Failed to read messages from file: %v", err))
			} else {
				state.messages = fileMessages
			}
		}

		// Save current history length for potential rollback (backtrack mode)
		rollbackPoint := len(state.messages)

		// Increment message ID for read-before-edit tracking
		r.toolCtx.ReadTracker.NextMessage()

		// Create cancellable context for this iteration
		iterCtx, iterCancel := context.WithCancel(ctx)

		// Call LLM
		llmResult, err := r.callLLM(iterCtx, state)
		if err != nil {
			iterCancel()
			result.FinalMessages = state.messages
			return result, err
		}

		if llmResult.shouldBreak {
			iterCancel()
			result.Cancelled = llmResult.cancelled
			result.TimedOut = llmResult.timedOut
			result.FinalMessages = state.messages
			break
		}

		if llmResult.shouldContinue {
			iterCancel()
			continue
		}

		// Process LLM response
		assistantMsg, ok := r.processLLMResponse(iterCtx, llmResult.response, state, llmResult.duration)
		if !ok {
			iterCancel()
			// Provider error with retry needed - continue to next iteration
			if state.consecutiveProviderFailures > 0 && state.consecutiveProviderFailures < maxProviderFailures {
				continue
			}
			result.FinalMessages = state.messages
			break
		}

		state.messages = append(state.messages, *assistantMsg)

		// Get token counts and request cost for backtracking
		promptTokens := llmResult.response.Usage.PromptTokens
		completionTokens := llmResult.response.Usage.CompletionTokens
		var requestCost float64
		if llmResult.response.ID != "" {
			genStats, err := r.llmClient.GetGenerationStats(context.Background(), llmResult.response.ID)
			if err == nil {
				requestCost = genStats.Data.TotalCost
			}
		}

		// No tool calls = final answer
		if len(assistantMsg.ToolCalls) == 0 {
			shouldContinue := r.handleFinalAnswer(llmResult.response, assistantMsg, state, rollbackPoint, false, rcfg, 0)
			iterCancel()
			if shouldContinue {
				continue
			}
			result.FinalMessages = state.messages
			break
		}

		// Reset retry counter on successful response with tool calls
		state.emptyReasoningRetries = 0

		r.logger.AgentIteration(i, len(assistantMsg.ToolCalls))

		// Display reasoning/thinking if present
		contextStr := ui.FormatContextStr(state.totalTokens, r.cfg.LLM.Context)
		if assistantMsg.ReasoningContent != "" {
			r.writer.Thinking(contextStr, assistantMsg.ReasoningContent)
		}
		if assistantMsg.Content != "" {
			r.writer.Thinking(contextStr, assistantMsg.Content)
		}

		// Start checkpoint turn
		if len(assistantMsg.ToolCalls) > 0 && r.checkpointMgr != nil && r.checkpointMgr.Enabled() {
			r.checkpointMgr.StartTurn()
		}

		// Execute tool calls
		toolResult := r.executeTools(iterCtx, assistantMsg.ToolCalls, state, rollbackPoint, promptTokens, completionTokens, requestCost, contextStr)

		// Handle post-iteration processing
		shouldBreak := r.handlePostIteration(iterCtx, assistantMsg, state, rollbackPoint, toolResult, rcfg)

		iterCancel()

		if toolResult.toolsCancelled {
			// Distinguish the caller's time budget expiring from a user
			// pressing ctrl-c: both stop the loop here, but only one is a
			// fault worth reporting against the run.
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				result.TimedOut = true
			} else {
				result.Cancelled = true
			}
			result.FinalMessages = state.messages
			break
		}

		if shouldBreak {
			result.FinalMessages = state.messages
			break
		}
	}

	// Collect backtrack stats
	discardedStats := state.backtracker.GetDiscardedStats()
	state.agentStats.DiscardedPromptTokens = discardedStats.TotalPromptTokens
	state.agentStats.DiscardedCompletionTokens = discardedStats.TotalCompletionTokens
	state.agentStats.DiscardedCost = discardedStats.TotalCost
	state.agentStats.BacktrackCount = discardedStats.DiscardCount

	result.FinalMessages = state.messages
	return result, nil
}

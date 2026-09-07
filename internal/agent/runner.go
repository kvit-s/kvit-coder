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
	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/procs"
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
	inbox             *inbox.Inbox
	procs             *procs.Registry
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
	// Inbox is where anything that arrives mid-turn waits: a line typed at
	// the terminal, a file dropped by "kvit-coder steer", a background
	// process exiting. The loop drains it once per iteration. Optional.
	Inbox *inbox.Inbox
	// Procs owns the processes that outlive a turn. The loop asks it once per
	// iteration what has happened and puts the answer in the inbox. Optional.
	Procs *procs.Registry
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
	// BudgetExhausted is set when the loop ran out of iterations
	// (agent.max_tool_iterations) while the model still wanted to keep going.
	// The run produced no final answer, and without this the caller could not
	// tell that apart from a model that simply finished.
	BudgetExhausted bool
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
		inbox:             opts.Inbox,
		procs:             opts.Procs,
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
	// contextWarned stops the "running out of context" warning repeating every
	// iteration once it has been said.
	contextWarned bool
	// lastRequestCost is what the endpoint charged for the most recent answer,
	// or 0 when llm.generation_stats is off. Backtracking records it against
	// the history it discards.
	lastRequestCost float64
	agentStats      *stats.AgentStats
	normalizer      *llm.ResponseNormalizer

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

// blanketToolTimeout bounds a tool that does not manage its own deadline.
// Time spent waiting for a person to answer a prompt is subtracted before this
// is applied, so a slow answer is not reported as a slow tool.
const blanketToolTimeout = 15 * time.Second

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
		// The extractor scrapes tool calls out of prose, for models that
		// describe a call instead of making one. A model that emits proper
		// tool calls does not need it, and scraping its prose can turn a
		// sentence about a command into a command.
		normalizer: llm.NewResponseNormalizer(r.toolCallExtractor(), r.cfg.LLM.MergeThinking),
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

	// The loop leaving without ever breaking out means the model still wanted
	// another iteration when the budget ran out. Every exit path below clears
	// this; the one that does not is the cap.
	budgetExhausted := true

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

		// Ask the background processes what has happened; whatever they say
		// goes into the inbox, so an exit reaches the model the same way a
		// typed line does.
		r.pollProcesses()

		// Anything that arrived since the last iteration goes in before the
		// rollback point, so backtracking cannot discard it.
		r.drainInbox(state)

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
			budgetExhausted = false
			result.FinalMessages = state.messages
			return result, err
		}

		if llmResult.shouldBreak {
			iterCancel()
			budgetExhausted = false
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
			budgetExhausted = false
			result.FinalMessages = state.messages
			break
		}

		state.messages = append(state.messages, *assistantMsg)

		// Token counts and request cost for backtracking. The cost comes from
		// the generation-stats call processLLMResponse already made; asking a
		// second time doubled the HTTP requests per iteration for a number the
		// loop already had.
		promptTokens := llmResult.response.Usage.PromptTokens
		completionTokens := llmResult.response.Usage.CompletionTokens
		requestCost := state.lastRequestCost

		// No tool calls = final answer
		if len(assistantMsg.ToolCalls) == 0 {
			shouldContinue := r.handleFinalAnswer(llmResult.response, assistantMsg, state, rollbackPoint, false, rcfg, 0)
			iterCancel()
			if shouldContinue {
				continue
			}
			budgetExhausted = false
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
			budgetExhausted = false
			result.FinalMessages = state.messages
			break
		}

		if shouldBreak {
			budgetExhausted = false
			result.FinalMessages = state.messages
			break
		}
	}

	// Running out of iterations used to end the turn silently, with the model
	// mid-task and no answer printed. Say so, in the history as well as on the
	// terminal, so the next turn has the fact and the caller can act on it.
	result.BudgetExhausted = budgetExhausted
	if budgetExhausted {
		notice := fmt.Sprintf("Stopped after %d iterations without a final answer: the iteration budget "+
			"(agent.max_tool_iterations) ran out while work was still in progress.", maxIters)
		r.writer.Warn(notice)
		state.messages = append(state.messages, llm.Message{
			Role:    llm.RoleAssistant,
			Content: "[" + notice + "]",
		})
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

// toolCallExtractor returns the registry when tool calls should be scraped out
// of the model's prose, and nil when they should not. Under the strong profile
// the model is expected to emit proper tool calls, and reading its prose for
// something that looks like one risks acting on a sentence about a command.
func (r *Runner) toolCallExtractor() llm.ToolCallExtractor {
	if r.cfg.Agent.IsStrong() {
		return nil
	}
	return r.registry
}

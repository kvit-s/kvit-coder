// Package agent provides the agent runner for executing LLM interactions.
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/checkpoint"
	"github.com/kvit-s/kvit-coder/internal/config"
	ctxtools "github.com/kvit-s/kvit-coder/internal/context"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/stats"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// Runner executes the agent loop for LLM interactions
type Runner struct {
	cfg               *config.Config
	llmClient         *llm.Client
	registry          *tools.Registry
	writer            *ui.Writer
	logger            *Logger
	checkpointMgr     *checkpoint.Manager
	contextMgr        *ctxtools.Manager
	contextMiddleware *ctxtools.Middleware
	planManager       *tools.PlanManager
}

// RunnerOptions contains all dependencies for creating a Runner
type RunnerOptions struct {
	Cfg               *config.Config
	LLMClient         *llm.Client
	Registry          *tools.Registry
	Writer            *ui.Writer
	Logger            *Logger
	CheckpointMgr     *checkpoint.Manager
	ContextMgr        *ctxtools.Manager
	ContextMiddleware *ctxtools.Middleware
	PlanManager       *tools.PlanManager
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
}

// NewRunner creates a new agent runner
func NewRunner(opts RunnerOptions) *Runner {
	return &Runner{
		cfg:               opts.Cfg,
		llmClient:         opts.LLMClient,
		registry:          opts.Registry,
		writer:            opts.Writer,
		logger:            opts.Logger,
		checkpointMgr:     opts.CheckpointMgr,
		contextMgr:        opts.ContextMgr,
		contextMiddleware: opts.ContextMiddleware,
		planManager:       opts.PlanManager,
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

// Run executes the agent loop for a single user input.
// It returns the updated messages and stats after completion.
func (r *Runner) Run(ctx context.Context, rcfg RunConfig) (*RunResult, error) {
	messages := rcfg.Messages

	// Reset loop detection state on new user input
	loopDetector := NewLoopDetector()

	// Initialize backtrack tracker
	backtracker := NewBacktrackTracker(BacktrackConfig{
		Enabled:           r.cfg.Backtrack.Enabled,
		MaxRetries:        r.cfg.Backtrack.MaxRetries,
		InjectUserMessage: r.cfg.Backtrack.InjectUserMessage,
	})

	// Track last tool call to detect immediate duplicates
	var lastToolName, lastToolArgs string
	var consecutiveDuplicates int
	const maxConsecutiveDuplicates = 3

	// Agent loop: iterate until final answer
	maxIters := r.cfg.Agent.MaxIterations
	if maxIters == 0 {
		maxIters = 10 // default
	}

	// Track consecutive parsing/provider failures to prevent infinite loops
	var consecutiveProviderFailures int
	const maxProviderFailures = 2

	// Track context overflow recovery attempts
	var contextOverflowRetries int
	const maxContextOverflowRetries = 2

	// Track "different approach" attempts after exhausting retries
	var differentApproachAttempts int
	const maxDifferentApproachAttempts = 3

	// Track empty response with reasoning retries
	var emptyReasoningRetries int
	const maxEmptyReasoningRetries = 3

	// Track overall timing stats
	requestStartTime := time.Now()
	var totalLLMTime time.Duration
	var totalToolTime time.Duration
	totalToolCalls := 0
	var totalTokens int

	// Track agent stats
	agentStats := &stats.AgentStats{}

	// Response normalizer
	normalizer := llm.NewResponseNormalizer(r.registry, r.cfg.LLM.MergeThinking)

	result := &RunResult{
		Stats:     agentStats,
		Cancelled: false,
	}

	for i := 0; i < maxIters; i++ {
		r.logger.AgentIteration(i, 0)

		// File-first mode: read messages from file at start of each iteration
		if rcfg.UseFileFirst && r.contextMgr != nil {
			fileMessages, err := r.contextMgr.ReadMessagesForLLM()
			if err != nil {
				r.writer.Error(fmt.Sprintf("Failed to read messages from file: %v", err))
			} else {
				messages = fileMessages
			}
		}

		// Save current history length for potential rollback (backtrack mode)
		rollbackPoint := len(messages)

		// Track if any Tasks.* tools were executed in this iteration
		tasksToolExecuted := false

		// Increment message ID for read-before-edit tracking
		tools.GetReadTracker().NextMessage()

		// Create cancellable context for this iteration
		iterCtx, iterCancel := context.WithCancel(ctx)

		// Ensure context is cancelled when iteration ends (using closure to capture iterCancel)
		cancelOnce := func(c context.CancelFunc) func() {
			return func() { c() }
		}(iterCancel)

		// No ESC listener in headless mode - cancellation is via SIGINT only
		cleanup := func() {}
		_ = cleanup // Mark cleanup as used (kept for future use)

		// Call LLM with tools
		r.writer.ToolProgress("✨ ")

		startTime := time.Now()
		llmDone := make(chan bool)
		llmDotCount := 0
		go func() {
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					r.writer.ToolProgress(".")
					llmDotCount++
				case <-llmDone:
					return
				}
			}
		}()

		resp, err := r.llmClient.Chat(iterCtx, llm.ChatRequest{
			Model:       r.cfg.LLM.Model,
			Messages:    messages,
			Tools:       r.registry.Specs(),
			ToolChoice:  "auto",
			Temperature: r.cfg.LLM.Temperature,
			MaxTokens:   r.cfg.LLM.MaxTokens,
			Stream:      false,
		})
		close(llmDone)
		time.Sleep(10 * time.Millisecond)
		duration := time.Since(startTime)
		totalLLMTime += duration

		// Print newline after sparkles/dots before showing response
		if llmDotCount == 0 {
			fmt.Print("\n")
		}

		if err != nil {
			// Check if it was a cancellation
			if iterCtx.Err() == context.Canceled {
				r.writer.Info("LLM call cancelled - returning to prompt")

				// If the last message is a tool result, add a dummy assistant message
				if len(messages) > 0 && messages[len(messages)-1].Role == llm.RoleTool {
					messages = append(messages, llm.Message{
						Role:    llm.RoleAssistant,
						Content: "[Operation cancelled by user]",
					})
				}

				agentStats.TotalAgentTime = time.Since(requestStartTime)
				agentStats.TotalLLMTime = totalLLMTime
				agentStats.TotalToolTime = totalToolTime
				result.Cancelled = true
				result.FinalMessages = messages

				cleanup()
				cancelOnce()
				break
			}

			// Handle 400 errors (likely context overflow)
			errStr := err.Error()
			if strings.Contains(errStr, "API error 400") && contextOverflowRetries < maxContextOverflowRetries {
				contextOverflowRetries++
				r.writer.Warn(fmt.Sprintf("Server error on request (attempt %d/%d) - replacing tool output and retrying...",
					contextOverflowRetries, maxContextOverflowRetries))

				// Replace all recent tool result messages with server error
				for j := len(messages) - 1; j >= 0 && messages[j].Role == llm.RoleTool; j-- {
					messages[j].Content = "[Server error processing tool output. The command ran successfully but the output cannot be processed by the server. You can try running the same command again or try a different approach.]"
				}

				cleanup()
				cancelOnce()
				continue
			}

			// Check if this is a 400 error but we've exhausted retries
			if strings.Contains(errStr, "API error 400") && contextOverflowRetries >= maxContextOverflowRetries {
				differentApproachAttempts++

				// Check if we've exhausted "different approach" attempts
				if differentApproachAttempts >= maxDifferentApproachAttempts {
					r.writer.Error(fmt.Sprintf("Server error persists after %d attempts - giving up. Last error: %s",
						maxDifferentApproachAttempts, errStr))
					agentStats.TotalAgentTime = time.Since(requestStartTime)
					agentStats.TotalLLMTime = totalLLMTime
					agentStats.TotalToolTime = totalToolTime
					result.FinalMessages = messages
					cleanup()
					cancelOnce()
					return result, fmt.Errorf("persistent server error: %s", errStr)
				}

				r.writer.Warn(fmt.Sprintf("Server error persists after %d retries - asking LLM to try a different approach (%d/%d)",
					maxContextOverflowRetries, differentApproachAttempts, maxDifferentApproachAttempts))

				for j := len(messages) - 1; j >= 0 && messages[j].Role == llm.RoleTool; j-- {
					messages[j].Content = "[Server error: Unable to process tool output after multiple retries.]"
				}

				// Add user message to prompt retry (required by some providers like Mistral
				// that expect the last message to be from user or tool role)
				messages = append(messages, llm.Message{
					Role:    llm.RoleUser,
					Content: "[System: Server error occurred. Please try a different approach.]",
				})

				contextOverflowRetries = 0
				cleanup()
				cancelOnce()
				continue
			}

			r.writer.Error(fmt.Sprintf("%v", err))
			r.logger.Error("LLM call failed", err)

			agentStats.TotalAgentTime = time.Since(requestStartTime)
			agentStats.TotalLLMTime = totalLLMTime
			agentStats.TotalToolTime = totalToolTime
			result.FinalMessages = messages

			cleanup()
			cancelOnce()
			break
		}

		// Extract assistant message
		if len(resp.Choices) == 0 {
			r.writer.Error("No response from model")

			agentStats.TotalAgentTime = time.Since(requestStartTime)
			agentStats.TotalLLMTime = totalLLMTime
			agentStats.TotalToolTime = totalToolTime
			result.FinalMessages = messages

			cleanup()
			cancelOnce()
			break
		}

		// Check for upstream provider error
		if resp.Choices[0].Error != nil {
			choiceErr := resp.Choices[0].Error
			r.writer.Warn(fmt.Sprintf("Provider error (code %d): %s - retrying...", choiceErr.Code, choiceErr.Message))

			retryStart := time.Now()
			retryResp, retryErr := r.llmClient.Chat(iterCtx, llm.ChatRequest{
				Model:       r.cfg.LLM.Model,
				Messages:    messages,
				Tools:       r.registry.Specs(),
				ToolChoice:  "auto",
				Temperature: r.cfg.LLM.Temperature,
				MaxTokens:   r.cfg.LLM.MaxTokens,
				Stream:      false,
			})
			totalLLMTime += time.Since(retryStart)

			if retryErr != nil || len(retryResp.Choices) == 0 || retryResp.Choices[0].Error != nil {
				consecutiveProviderFailures++

				if consecutiveProviderFailures >= maxProviderFailures {
					r.writer.Error(fmt.Sprintf("Provider failed %d times consecutively - stopping", consecutiveProviderFailures))

					agentStats.TotalAgentTime = time.Since(requestStartTime)
					agentStats.TotalLLMTime = totalLLMTime
					agentStats.TotalToolTime = totalToolTime
					result.FinalMessages = messages

					cleanup()
					cancelOnce()
					break
				}

				// Add messages to guide LLM to retry
				assistantContent := "[Response failed due to server error]"
				if resp.Choices[0].Message.Content != "" {
					assistantContent = resp.Choices[0].Message.Content
				}
				messages = append(messages, llm.Message{
					Role:    llm.RoleAssistant,
					Content: assistantContent,
				})
				messages = append(messages, llm.Message{
					Role:    llm.RoleUser,
					Content: "[System: Your previous response caused a server error. Please try again.]",
				})

				cleanup()
				cancelOnce()
				continue
			}

			resp = retryResp
			r.writer.Info("Retry succeeded")
		}

		consecutiveProviderFailures = 0

		assistantMsg := resp.Choices[0].Message

		// Apply response normalization middleware
		toolCallsExtracted := normalizer.NormalizeResponse(&assistantMsg)
		if toolCallsExtracted {
			resp.Choices[0].FinishReason = "tool_calls"
		}

		// Prevent consecutive assistant messages
		messages, _ = llm.PreventConsecutiveAssistant(messages)

		// Inject turn number for context tools
		if r.contextMiddleware != nil && assistantMsg.Content != "" {
			assistantMsg.Content = r.contextMiddleware.ProcessAssistantMessage(assistantMsg.Content)
		}

		messages = append(messages, assistantMsg)

		// Get token counts
		promptTokens := resp.Usage.PromptTokens
		completionTokens := resp.Usage.CompletionTokens
		totalTokens = promptTokens + completionTokens

		agentStats.TotalPromptTokens += promptTokens
		agentStats.TotalCompletionTokens += completionTokens
		if totalTokens > agentStats.MaxContextUsed {
			agentStats.MaxContextUsed = totalTokens
		}

		// Query generation stats for extended data
		var requestCost float64
		if resp.ID != "" {
			genStats, err := r.llmClient.GetGenerationStats(context.Background(), resp.ID)
			if err == nil {
				if genStats.Data.NativeTokensPrompt > 0 {
					promptTokens = genStats.Data.NativeTokensPrompt
					completionTokens = genStats.Data.NativeTokensCompletion
					totalTokens = promptTokens + completionTokens
				}
				agentStats.TotalCacheReadTokens += genStats.Data.NativeTokensCached
				agentStats.TotalCost += genStats.Data.TotalCost
				agentStats.CacheDiscount += genStats.Data.CacheDiscount
				agentStats.TotalPromptMS += genStats.Data.Latency
				agentStats.TotalGenerationMS += genStats.Data.GenerationTime
				requestCost = genStats.Data.TotalCost
			}
		}
		agentStats.Steps++

		r.logger.LLMCall(r.cfg.LLM.Model, promptTokens, completionTokens, duration)

		if llmDotCount > 0 {
			r.writer.ToolProgress(fmt.Sprintf("%.0fs\n", duration.Seconds()))
		}

		// No tool calls = final answer
		if len(assistantMsg.ToolCalls) == 0 {
			finishReason := resp.Choices[0].FinishReason
			if finishReason == "stop" && r.registry.LooksLikeMalformedToolCall(assistantMsg.Content) {
				r.writer.Warn("Detected malformed tool call in response, auto-continuing...")
				messages = append(messages, llm.Message{
					Role:    llm.RoleUser,
					Content: "continue",
				})
				continue
			}

			if assistantMsg.Content == "" {
				if assistantMsg.ReasoningContent != "" {
					if emptyReasoningRetries < maxEmptyReasoningRetries {
						emptyReasoningRetries++
						r.writer.Warn(fmt.Sprintf("LLM returned empty response with reasoning, retrying... (%d/%d)",
							emptyReasoningRetries, maxEmptyReasoningRetries))

						messages = messages[:len(messages)-1]
						cleanup()
						cancelOnce()
						continue
					}

					r.writer.Warn("LLM still confused, using directive prompt...")
					assistantMsg.Content = assistantMsg.ReasoningContent
					assistantMsg.ReasoningContent = ""
					messages[len(messages)-1] = assistantMsg

					messages = append(messages, llm.Message{
						Role:    llm.RoleUser,
						Content: "Use the appropriate tools to complete your task. Make the tool calls now.",
					})

					cleanup()
					cancelOnce()
					continue
				}

				r.writer.Warn(fmt.Sprintf("LLM returned empty response (finish_reason=%s)", finishReason))
			}

			r.writer.Assistant(assistantMsg.Content)

			// Display stats summary
			totalTime := time.Since(requestStartTime)
			var statsMsg string
			if totalToolCalls > 0 {
				statsMsg = fmt.Sprintf("[%s: %s✨ + %s🔧x%d]",
					ui.FormatDuration(totalTime),
					ui.FormatDuration(totalLLMTime),
					ui.FormatDuration(totalToolTime),
					totalToolCalls)
			} else {
				statsMsg = fmt.Sprintf("[%s: %s✨]",
					ui.FormatDuration(totalTime),
					ui.FormatDuration(totalLLMTime))
			}
			r.writer.Info(statsMsg)

			agentStats.TotalAgentTime = totalTime
			agentStats.TotalLLMTime = totalLLMTime
			agentStats.TotalToolTime = totalToolTime

			// File-first mode: persist messages
			if rcfg.UseFileFirst && r.contextMgr != nil && !tasksToolExecuted && len(messages) > rollbackPoint {
				newMessages := messages[rollbackPoint:]
				if err := r.contextMgr.AppendMessages(newMessages); err != nil {
					r.writer.Error(fmt.Sprintf("Failed to persist messages: %v", err))
				}
			}

			result.FinalMessages = messages
			cleanup()
			cancelOnce()
			break
		}

		// Reset retry counter on successful response with tool calls
		emptyReasoningRetries = 0

		r.logger.AgentIteration(i, len(assistantMsg.ToolCalls))

		// Display reasoning/thinking if present
		contextStr := ui.FormatContextStr(totalTokens, r.cfg.LLM.Context)
		if assistantMsg.ReasoningContent != "" {
			r.writer.Thinking(contextStr, assistantMsg.ReasoningContent)
		}
		if assistantMsg.Content != "" {
			r.writer.Thinking(contextStr, assistantMsg.Content)
		}

		// Execute tool calls
		toolsCancelled := false
		var lastExecutedToolIdx int = -1
		totalToolCalls += len(assistantMsg.ToolCalls)

		shouldBacktrack := false
		injectUserMessage := false
		userMessageToInject := ""

		// Start checkpoint turn
		if len(assistantMsg.ToolCalls) > 0 && r.checkpointMgr != nil && r.checkpointMgr.Enabled() {
			r.checkpointMgr.StartTurn()
		}

		for idx, tc := range assistantMsg.ToolCalls {
			tool := r.registry.Get(tc.Function.Name)
			if tool == nil {
				unknownErr := tools.SemanticErrorf("Unknown tool '%s'. Available tools can be found in the system prompt.", tc.Function.Name)
				btResult := r.handleToolError(unknownErr, tc, backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
				if btResult.shouldBacktrack {
					shouldBacktrack = true
					if btResult.injectUserMessage {
						injectUserMessage = true
						userMessageToInject = btResult.userMessage
					}
					break
				}
				r.writer.Error(fmt.Sprintf("Unknown tool: %s", tc.Function.Name))
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
					Content:    tools.FormatError(unknownErr),
				})
				continue
			}

			// Analyze pending edit state from message history
			var roles, contents, toolNames []string
			for _, msg := range messages {
				roles = append(roles, string(msg.Role))
				contents = append(contents, msg.Content)
				toolNames = append(toolNames, msg.Name) // Tool name for tool messages
			}
			pendingState := tools.AnalyzePendingEditState(roles, contents, toolNames)

			if blockErr := tools.CheckPendingEditBlockWithState(tc.Function.Name, pendingState, r.cfg); blockErr != nil {
				btResult := r.handleToolError(blockErr, tc, backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
				if btResult.shouldBacktrack {
					shouldBacktrack = true
					if btResult.injectUserMessage {
						injectUserMessage = true
						userMessageToInject = btResult.userMessage
					}
					break
				}

				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
					Content:    tools.FormatError(blockErr),
				})
				r.writer.ToolResult(fmt.Sprintf("Error: %s", ui.ShortenBlockMessage(blockErr.Error())), "")
				continue
			}

			// Normalize tool arguments for Check
			checkArgs := json.RawMessage(tc.Function.Arguments)
			if checkArgs, err = tools.NormalizeToolCallArguments(tool, checkArgs); err != nil {
				r.writer.Warn(fmt.Sprintf("Warning: Failed to normalize tool arguments for Check: %v", err))
				checkArgs = json.RawMessage(tc.Function.Arguments)
			}

			// Run safety checks
			if err := tool.Check(iterCtx, checkArgs); err != nil {
				checkErr := tools.WrapAsSemantic(err)

				btResult := r.handleToolError(checkErr, tc, backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
				if btResult.shouldBacktrack {
					shouldBacktrack = true
					if btResult.injectUserMessage {
						injectUserMessage = true
						userMessageToInject = btResult.userMessage
					}
					break
				}

				errContent := tools.FormatError(err)
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
					Content:    errContent,
				})
				var errSummary string
				var errMap map[string]any
				if json.Unmarshal([]byte(errContent), &errMap) == nil {
					if errType, ok := errMap["error"].(string); ok {
						errSummary = fmt.Sprintf("Error: %s", errType)
					}
				}
				if errSummary == "" {
					errSummary = fmt.Sprintf("Error: %v", err)
				}
				r.writer.ToolResult(errSummary, "")
				loopDetector.Record(tc.Function.Name, tc.Function.Arguments, errContent, true)
				continue
			}

			// Check for immediate duplicate call
			if tc.Function.Name == lastToolName && tc.Function.Arguments == lastToolArgs {
				consecutiveDuplicates++

				// After max consecutive duplicates, stop with error
				if consecutiveDuplicates >= maxConsecutiveDuplicates {
					r.writer.Error(fmt.Sprintf("FATAL: %s called %d times with identical arguments - stopping to prevent infinite loop",
						tc.Function.Name, consecutiveDuplicates))

					agentStats.TotalAgentTime = time.Since(requestStartTime)
					agentStats.TotalLLMTime = totalLLMTime
					agentStats.TotalToolTime = totalToolTime
					result.FinalMessages = messages

					cleanup()
					cancelOnce()
					return result, fmt.Errorf("duplicate call loop: %s called %d times with same arguments", tc.Function.Name, consecutiveDuplicates)
				}

				dupErr := tools.SemanticErrorf("DUPLICATE CALL ERROR: You just made this exact same call with identical arguments. The result will be the same. You MUST try a different approach or different arguments. Repeated duplicate calls will cause the session to terminate.")
				btResult := r.handleToolError(dupErr, tc, backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
				if btResult.shouldBacktrack {
					shouldBacktrack = true
					if btResult.injectUserMessage {
						injectUserMessage = true
						userMessageToInject = btResult.userMessage
					}
					break
				}
				errContent := tools.FormatError(dupErr)
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
					Content:    errContent,
				})
				r.writer.ToolResult("Error: duplicate call", "")
				loopDetector.Record(tc.Function.Name, tc.Function.Arguments, errContent, true)
				continue
			}

			// Reset duplicate counter on different call
			consecutiveDuplicates = 0

			// Display tool call
			var args map[string]any
			_ = json.Unmarshal([]byte(tc.Function.Arguments), &args)

			if tc.Function.Name == "Shell" {
				cmdStr, _ := args["command"].(string)
				r.writer.ToolCall("Shell", cmdStr, contextStr)
			} else if tc.Function.Name == "Shell.advanced" {
				cmdStr, _ := args["command"].(string)
				wdStr, _ := args["working_dir"].(string)
				argsDisplay := ui.FormatShellDisplay(cmdStr, wdStr, r.cfg.Workspace.Root)
				if timeoutVal, ok := args["timeout"].(float64); ok && timeoutVal > 0 && int(timeoutVal) != 30 {
					argsDisplay += fmt.Sprintf(", timeout=%ds", int(timeoutVal))
				}
				r.writer.ToolCall("Shell.advanced", argsDisplay, contextStr)
			} else if strings.HasPrefix(tc.Function.Name, "Plan.") {
				if r.writer.IsVerbose() {
					argsDisplay := ui.FormatToolArgs(args)
					r.writer.ToolCall(tc.Function.Name, argsDisplay, contextStr)
				} else {
					r.writer.ToolContext(contextStr)
				}
			} else {
				argsDisplay := ui.FormatToolArgs(args)
				r.writer.ToolCall(tc.Function.Name, argsDisplay, contextStr)
			}

			// Execute tool with timing
			toolStart := time.Now()
			progressDone := make(chan bool)
			dotCount := 0
			go func() {
				ticker := time.NewTicker(1 * time.Second)
				defer ticker.Stop()
				for {
					select {
					case <-ticker.C:
						r.writer.ToolProgress(".")
						dotCount++
					case <-progressDone:
						return
					}
				}
			}()

			// Apply timeout to non-shell tools
			toolCtx := iterCtx
			var toolCancel context.CancelFunc
			if tc.Function.Name != "Shell" && tc.Function.Name != "Shell.advanced" {
				toolCtx, toolCancel = context.WithTimeout(iterCtx, 15*time.Second)
				defer toolCancel()
			}

			// Normalize tool arguments
			normalizedArgs := json.RawMessage(tc.Function.Arguments)
			if normalizedArgs, err = tools.NormalizeToolCallArguments(tool, normalizedArgs); err != nil {
				r.writer.Warn(fmt.Sprintf("Warning: Failed to normalize tool arguments: %v", err))
				normalizedArgs = json.RawMessage(tc.Function.Arguments)
			}

			toolResult, toolErr := tool.Call(toolCtx, normalizedArgs)

			if toolCtx.Err() == context.DeadlineExceeded {
				toolErr = fmt.Errorf("tool execution timed out after 15 seconds")
			}
			toolDuration := time.Since(toolStart)
			close(progressDone)
			time.Sleep(10 * time.Millisecond)
			totalToolTime += toolDuration

			// Check if context was cancelled
			select {
			case <-iterCtx.Done():
				toolsCancelled = true
				lastExecutedToolIdx = idx
				content := "Error: operation cancelled by user"
				if toolErr == nil {
					resultJSON, _ := json.MarshalIndent(toolResult, "", "  ")
					content = string(resultJSON)
				} else {
					content = fmt.Sprintf("Error: %v", toolErr)
				}
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
					Content:    content,
				})
				r.logger.ToolExecuted(tc.Function.Name, toolDuration, false, iterCtx.Err())
			default:
				var content string
				if toolErr != nil {
					if tools.IsBacktrackable(toolErr) {
						btResult := r.handleToolError(toolErr, tc, backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
						if btResult.shouldBacktrack {
							shouldBacktrack = true
							if btResult.injectUserMessage {
								injectUserMessage = true
								userMessageToInject = btResult.userMessage
							}
							break
						}
					}
					content = fmt.Sprintf("Error: %v", toolErr)
					r.writer.Error(fmt.Sprintf("Tool error: %s", toolErr))
					r.logger.ToolExecuted(tc.Function.Name, toolDuration, false, toolErr)
				} else {
					isPlanTool := strings.HasPrefix(tc.Function.Name, "Plan.")

					resultJSON, _ := json.MarshalIndent(toolResult, "", "  ")
					content = string(resultJSON)

					if isPlanTool && r.planManager != nil {
						planText := r.planManager.FormatActivePlan()
						if planText != "" {
							r.writer.ActivePlan(planText)
						}
						r.writer.VerboseOutput(content)
						r.logger.ToolExecuted(tc.Function.Name, toolDuration, true, nil)
					} else {
						summary := ui.GetResultSummary(toolResult)
						var durationStr string
						if dotCount > 0 {
							durationStr = fmt.Sprintf("...%.0fs", toolDuration.Seconds())
						}
						r.writer.ToolResult(summary, durationStr)
						r.writer.VerboseOutput(content)
						r.logger.ToolExecuted(tc.Function.Name, toolDuration, true, nil)
					}
				}

				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
					Content:    content,
				})
				lastExecutedToolIdx = idx

				if strings.HasPrefix(tc.Function.Name, "Tasks.") {
					tasksToolExecuted = true
				}

				isError := toolErr != nil || strings.HasPrefix(content, "Error:") || strings.Contains(content, "\"success\": false")
				loopDetector.Record(tc.Function.Name, tc.Function.Arguments, content, isError)

				lastToolName = tc.Function.Name
				lastToolArgs = tc.Function.Arguments
			}

			if toolsCancelled {
				break
			}
		}

		// Handle backtrack if needed
		if shouldBacktrack {
			messages = messages[:rollbackPoint]

			if injectUserMessage && userMessageToInject != "" {
				// Only inject if last message is not already a user message (avoid role alternation violations)
				if len(messages) == 0 || messages[len(messages)-1].Role != llm.RoleUser {
					messages = append(messages, llm.Message{
						Role:    llm.RoleUser,
						Content: userMessageToInject,
					})
					r.writer.Debug("Injected user message for backtrack recovery")
				}
			}

			cleanup()
			cancelOnce()
			continue
		}

		// End checkpoint turn
		if len(assistantMsg.ToolCalls) > 0 && r.checkpointMgr != nil && r.checkpointMgr.Enabled() {
			if err := r.checkpointMgr.EndTurn(); err != nil {
				r.writer.Debug(fmt.Sprintf("Checkpoint error: %v", err))
			}
		}

		// Check for loop detection
		if loopInfo := loopDetector.DetectLoop(3); loopInfo != nil {
			var interventionMsg string

			if loopInfo.IsError {
				r.writer.Warn(fmt.Sprintf("Loop detected: %s called %d times with same error", loopInfo.ToolName, loopInfo.Count))

				interventionMsg = fmt.Sprintf("\n\n<system-reminder>\n"+
					"LOOP DETECTED: You have called '%s' %d times in a row with the same failing result. "+
					"STOP and try a DIFFERENT approach.\n"+
					"</system-reminder>", loopInfo.ToolName, loopInfo.Count)
			} else if loopInfo.IsSuccess {
				r.writer.Warn(fmt.Sprintf("Loop detected: %s called %d times with same arguments and result", loopInfo.ToolName, loopInfo.Count))

				interventionMsg = fmt.Sprintf("\n\n<system-reminder>\n"+
					"LOOP DETECTED: You have called '%s' %d times in a row with identical arguments and results. "+
					"You are stuck in a loop. STOP and try a DIFFERENT approach.\n"+
					"</system-reminder>", loopInfo.ToolName, loopInfo.Count)
			}

			if interventionMsg != "" && len(messages) > 0 && messages[len(messages)-1].Role == llm.RoleTool {
				messages[len(messages)-1].Content += interventionMsg
			}
		} else if loopInfo := loopDetector.DetectErrorLoop(4); loopInfo != nil {
			r.writer.Warn(fmt.Sprintf("Error loop detected: %s has failed %d times consecutively", loopInfo.ToolName, loopInfo.Count))

			interventionMsg := fmt.Sprintf("\n\n<system-reminder>\n"+
				"ERROR LOOP DETECTED: '%s' has failed %d times in a row. "+
				"Try a completely different approach or tool.\n"+
				"</system-reminder>", loopInfo.ToolName, loopInfo.Count)

			if len(messages) > 0 && messages[len(messages)-1].Role == llm.RoleTool {
				messages[len(messages)-1].Content += interventionMsg
			}
		} else if loopInfo := loopDetector.DetectAlternatingLoop(3); loopInfo != nil {
			r.writer.Warn(fmt.Sprintf("Alternating loop detected: %s is repeating the same cycle", loopInfo.ToolName))

			interventionMsg := fmt.Sprintf("\n\n<system-reminder>\n"+
				"ALTERNATING LOOP DETECTED: You are stuck in a cycle repeating '%s' with the same arguments followed by cancellation/undo. "+
				"This pattern has repeated %d times. STOP and try a DIFFERENT approach - perhaps the edit you're attempting is not the right solution.\n"+
				"</system-reminder>", loopInfo.ToolName, loopInfo.Count)

			if len(messages) > 0 && messages[len(messages)-1].Role == llm.RoleTool {
				messages[len(messages)-1].Content += interventionMsg
			}
		}

		// Handle cancelled tools
		if toolsCancelled {
			r.writer.Info("Tool execution cancelled - returning to prompt")

			for k := lastExecutedToolIdx + 1; k < len(assistantMsg.ToolCalls); k++ {
				tc := assistantMsg.ToolCalls[k]
				messages = append(messages, llm.Message{
					Role:       llm.RoleTool,
					Name:       tc.Function.Name,
					ToolCallID: tc.ID,
					Content:    "Error: Cancelled by user",
				})
			}

			agentStats.TotalAgentTime = time.Since(requestStartTime)
			agentStats.TotalLLMTime = totalLLMTime
			agentStats.TotalToolTime = totalToolTime

			if rcfg.UseFileFirst && r.contextMgr != nil && !tasksToolExecuted && len(messages) > rollbackPoint {
				newMessages := messages[rollbackPoint:]
				if err := r.contextMgr.AppendMessages(newMessages); err != nil {
					r.writer.Error(fmt.Sprintf("Failed to persist messages: %v", err))
				}
			}

			result.Cancelled = true
			result.FinalMessages = messages

			cleanup()
			cancelOnce()
			break
		}

		// Plan injection
		planInjectionMode := r.cfg.Tools.Plan.InjectionMode
		if planInjectionMode == "every_step" && r.planManager != nil {
			activePlan := r.planManager.GetActivePlan()
			if activePlan != nil && activePlan.Status != "complete" {
				anyPlanTool := false
				for _, tc := range assistantMsg.ToolCalls {
					if strings.HasPrefix(tc.Function.Name, "Plan.") {
						anyPlanTool = true
						break
					}
				}

				if !anyPlanTool && len(messages) > 0 {
					planJSON, err := json.MarshalIndent(map[string]any{
						"task":   activePlan.TaskName,
						"status": activePlan.Status,
						"steps":  activePlan.Steps,
					}, "", "  ")
					if err == nil {
						planContent := fmt.Sprintf("\n\n<system-reminder>\nCurrent plan status:\n```json\n%s\n```\n</system-reminder>", string(planJSON))
						lastIdx := len(messages) - 1
						if messages[lastIdx].Role == llm.RoleTool {
							messages[lastIdx].Content += planContent
						}
					}
				}
			}
		}

		// File-first mode: persist messages
		if rcfg.UseFileFirst && r.contextMgr != nil && !tasksToolExecuted && len(messages) > rollbackPoint {
			newMessages := messages[rollbackPoint:]
			if err := r.contextMgr.AppendMessages(newMessages); err != nil {
				r.writer.Error(fmt.Sprintf("Failed to persist messages: %v", err))
			}
		}

		cleanup()
		cancelOnce()
	}

	// Collect backtrack stats
	discardedStats := backtracker.GetDiscardedStats()
	agentStats.DiscardedPromptTokens = discardedStats.TotalPromptTokens
	agentStats.DiscardedCompletionTokens = discardedStats.TotalCompletionTokens
	agentStats.DiscardedCost = discardedStats.TotalCost
	agentStats.BacktrackCount = discardedStats.DiscardCount

	result.FinalMessages = messages
	return result, nil
}

// handleToolError handles errors from tool validation or execution.
// It decides whether to backtrack (discard and retry) or add error to history.
func (r *Runner) handleToolError(
	err error,
	tc llm.ToolCall,
	backtracker *BacktrackTracker,
	rollbackPoint int,
	promptTokens, completionTokens int,
	requestCost float64,
) backtrackResult {
	// Check if this is a backtrackable (semantic) error
	if tools.IsBacktrackable(err) && backtracker.ShouldBacktrack(rollbackPoint) {
		errMsg := err.Error()
		if idx := strings.Index(errMsg, "\n"); idx > 0 {
			errMsg = errMsg[:idx]
		}
		r.writer.Info(fmt.Sprintf("↩ Retry [%d/%d]: %s - %s",
			backtracker.GetRetryCount(), backtracker.GetMaxRetries(),
			tc.Function.Name, errMsg))
		backtracker.RecordDiscarded(promptTokens, completionTokens, requestCost)
		return backtrackResult{
			shouldBacktrack: true,
			toolName:        tc.Function.Name,
			reason:          err.Error(),
		}
	}

	// FALLBACK: not backtrackable or limit reached
	// Only show warning if backtracking is enabled (meaning limit was actually reached)
	if tools.IsBacktrackable(err) && backtracker.IsEnabled() {
		if backtracker.ShouldInjectUserMessage() {
			userMsg := fmt.Sprintf("STOP. Your last action failed: %s\n\n"+
				"You have retried this %d times without success. "+
				"Read the error carefully and take the correct action.",
				err.Error(), backtracker.GetMaxRetries())

			r.writer.Warn(fmt.Sprintf("⚠ Backtrack limit reached for %s, injecting user message", tc.Function.Name))
			backtracker.RecordDiscarded(promptTokens, completionTokens, requestCost)
			backtracker.ResetAtPoint()
			return backtrackResult{
				shouldBacktrack:   true,
				injectUserMessage: true,
				userMessage:       userMsg,
				toolName:          tc.Function.Name,
				reason:            err.Error(),
			}
		}
		r.writer.Warn(fmt.Sprintf("⚠ Backtrack limit reached for %s, adding error to history", tc.Function.Name))
		backtracker.ResetAtPoint()
	}

	return backtrackResult{
		shouldBacktrack: false,
		toolName:        tc.Function.Name,
		reason:          err.Error(),
	}
}

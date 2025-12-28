package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// singleToolResult holds the outcome of executing a single tool
type singleToolResult struct {
	shouldBacktrack     bool
	userMessageToInject string
	toolsCancelled      bool
}

// executeTools executes all tool calls from an assistant message.
// It handles validation, execution, error handling, and backtracking.
func (r *Runner) executeTools(
	ctx context.Context,
	toolCalls []llm.ToolCall,
	state *runState,
	rollbackPoint int,
	promptTokens, completionTokens int,
	requestCost float64,
	contextStr string,
) *toolExecutionResult {
	result := &toolExecutionResult{
		lastExecutedIdx: -1,
	}

	state.totalToolCalls += len(toolCalls)

	for idx, tc := range toolCalls {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			result.toolsCancelled = true
			result.lastExecutedIdx = idx - 1
			return result
		default:
		}

		toolResult := r.executeSingleTool(ctx, tc, state, rollbackPoint, promptTokens, completionTokens, requestCost, contextStr)

		if toolResult.shouldBacktrack {
			result.shouldBacktrack = true
			result.userMessageToInject = toolResult.userMessageToInject
			return result
		}

		if toolResult.toolsCancelled {
			result.toolsCancelled = true
			result.lastExecutedIdx = idx
			return result
		}

		result.lastExecutedIdx = idx

		if strings.HasPrefix(tc.Function.Name, "Tasks.") {
			result.tasksToolExecuted = true
		}
	}

	return result
}

// executeSingleTool executes a single tool call with validation and error handling
func (r *Runner) executeSingleTool(
	ctx context.Context,
	tc llm.ToolCall,
	state *runState,
	rollbackPoint int,
	promptTokens, completionTokens int,
	requestCost float64,
	contextStr string,
) *singleToolResult {
	result := &singleToolResult{}

	tool := r.registry.Get(tc.Function.Name)
	if tool == nil {
		unknownErr := tools.SemanticErrorf("Unknown tool '%s'. Available tools can be found in the system prompt.", tc.Function.Name)
		btResult := r.handleToolError(unknownErr, tc, state.backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
		if btResult.shouldBacktrack {
			result.shouldBacktrack = true
			if btResult.injectUserMessage {
				result.userMessageToInject = btResult.userMessage
			}
			return result
		}
		r.writer.Error(fmt.Sprintf("Unknown tool: %s", tc.Function.Name))
		state.messages = append(state.messages, llm.Message{
			Role:       llm.RoleTool,
			Name:       tc.Function.Name,
			ToolCallID: tc.ID,
			Content:    tools.FormatError(unknownErr),
		})
		return result
	}

	// Analyze pending edit state from message history
	var roles, contents, toolNames []string
	for _, msg := range state.messages {
		roles = append(roles, string(msg.Role))
		contents = append(contents, msg.Content)
		toolNames = append(toolNames, msg.Name)
	}
	pendingState := tools.AnalyzePendingEditState(roles, contents, toolNames)

	if blockErr := tools.CheckPendingEditBlockWithState(tc.Function.Name, pendingState, r.cfg, r.toolCtx); blockErr != nil {
		btResult := r.handleToolError(blockErr, tc, state.backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
		if btResult.shouldBacktrack {
			result.shouldBacktrack = true
			if btResult.injectUserMessage {
				result.userMessageToInject = btResult.userMessage
			}
			return result
		}

		state.messages = append(state.messages, llm.Message{
			Role:       llm.RoleTool,
			Name:       tc.Function.Name,
			ToolCallID: tc.ID,
			Content:    tools.FormatError(blockErr),
		})
		r.writer.ToolResult(fmt.Sprintf("Error: %s", ui.ShortenBlockMessage(blockErr.Error())), "")
		return result
	}

	// Normalize tool arguments for Check
	checkArgs := json.RawMessage(tc.Function.Arguments)
	var err error
	if checkArgs, err = tools.NormalizeToolCallArguments(tool, checkArgs); err != nil {
		r.writer.Warn(fmt.Sprintf("Warning: Failed to normalize tool arguments for Check: %v", err))
		checkArgs = json.RawMessage(tc.Function.Arguments)
	}

	// Run safety checks
	if err := tool.Check(ctx, checkArgs); err != nil {
		checkErr := tools.WrapAsSemantic(err)
		btResult := r.handleToolError(checkErr, tc, state.backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
		if btResult.shouldBacktrack {
			result.shouldBacktrack = true
			if btResult.injectUserMessage {
				result.userMessageToInject = btResult.userMessage
			}
			return result
		}

		errContent := tools.FormatError(err)
		state.messages = append(state.messages, llm.Message{
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
		state.loopDetector.Record(tc.Function.Name, tc.Function.Arguments, errContent, true)
		return result
	}

	// Check for immediate duplicate call
	if tc.Function.Name == state.lastToolName && tc.Function.Arguments == state.lastToolArgs {
		state.consecutiveDuplicates++

		if state.consecutiveDuplicates >= maxConsecutiveDuplicates {
			r.writer.Error(fmt.Sprintf("FATAL: %s called %d times with identical arguments - stopping to prevent infinite loop",
				tc.Function.Name, state.consecutiveDuplicates))

			state.agentStats.TotalAgentTime = time.Since(state.requestStartTime)
			state.agentStats.TotalLLMTime = state.totalLLMTime
			state.agentStats.TotalToolTime = state.totalToolTime
			// This is a fatal error - we need to signal breaking out of the main loop
			// For now, treat as cancelled
			result.toolsCancelled = true
			return result
		}

		dupErr := tools.SemanticErrorf("DUPLICATE CALL ERROR: You just made this exact same call with identical arguments. The result will be the same. You MUST try a different approach or different arguments. Repeated duplicate calls will cause the session to terminate.")
		btResult := r.handleToolError(dupErr, tc, state.backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
		if btResult.shouldBacktrack {
			result.shouldBacktrack = true
			if btResult.injectUserMessage {
				result.userMessageToInject = btResult.userMessage
			}
			return result
		}
		errContent := tools.FormatError(dupErr)
		state.messages = append(state.messages, llm.Message{
			Role:       llm.RoleTool,
			Name:       tc.Function.Name,
			ToolCallID: tc.ID,
			Content:    errContent,
		})
		r.writer.ToolResult("Error: duplicate call", "")
		state.loopDetector.Record(tc.Function.Name, tc.Function.Arguments, errContent, true)
		return result
	}

	// Reset duplicate counter on different call
	state.consecutiveDuplicates = 0

	// Display tool call
	r.displayToolCall(tc, contextStr)

	// Execute tool with timing
	content, toolErr, toolDuration, cancelled := r.executeToolWithTimeout(ctx, tool, tc, state)

	if cancelled {
		result.toolsCancelled = true
		state.messages = append(state.messages, llm.Message{
			Role:       llm.RoleTool,
			Name:       tc.Function.Name,
			ToolCallID: tc.ID,
			Content:    content,
		})
		return result
	}

	// Handle tool error (backtrackable)
	if toolErr != nil && tools.IsBacktrackable(toolErr) {
		btResult := r.handleToolError(toolErr, tc, state.backtracker, rollbackPoint, promptTokens, completionTokens, requestCost)
		if btResult.shouldBacktrack {
			result.shouldBacktrack = true
			if btResult.injectUserMessage {
				result.userMessageToInject = btResult.userMessage
			}
			return result
		}
	}

	// Record result
	if toolErr != nil {
		content = fmt.Sprintf("Error: %v", toolErr)
		r.writer.Error(fmt.Sprintf("Tool error: %s", toolErr))
		r.logger.ToolExecuted(tc.Function.Name, toolDuration, false, toolErr)
	}

	state.messages = append(state.messages, llm.Message{
		Role:       llm.RoleTool,
		Name:       tc.Function.Name,
		ToolCallID: tc.ID,
		Content:    content,
	})

	isError := toolErr != nil || strings.HasPrefix(content, "Error:") || strings.Contains(content, "\"success\": false")
	state.loopDetector.Record(tc.Function.Name, tc.Function.Arguments, content, isError)

	state.lastToolName = tc.Function.Name
	state.lastToolArgs = tc.Function.Arguments

	return result
}

// displayToolCall formats and displays a tool call to the user
func (r *Runner) displayToolCall(tc llm.ToolCall, contextStr string) {
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
}

// executeToolWithTimeout executes a tool with appropriate timeout and progress display
func (r *Runner) executeToolWithTimeout(
	ctx context.Context,
	tool tools.Tool,
	tc llm.ToolCall,
	state *runState,
) (content string, toolErr error, duration time.Duration, cancelled bool) {
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
	toolCtx := ctx
	var toolCancel context.CancelFunc
	if tc.Function.Name != "Shell" && tc.Function.Name != "Shell.advanced" {
		toolCtx, toolCancel = context.WithTimeout(ctx, 15*time.Second)
		defer toolCancel()
	}

	// Normalize tool arguments
	normalizedArgs := json.RawMessage(tc.Function.Arguments)
	var err error
	if normalizedArgs, err = tools.NormalizeToolCallArguments(tool, normalizedArgs); err != nil {
		r.writer.Warn(fmt.Sprintf("Warning: Failed to normalize tool arguments: %v", err))
		normalizedArgs = json.RawMessage(tc.Function.Arguments)
	}

	toolResult, toolErr := tool.Call(toolCtx, normalizedArgs)

	if toolCtx.Err() == context.DeadlineExceeded {
		toolErr = fmt.Errorf("tool execution timed out after 15 seconds")
	}
	duration = time.Since(toolStart)
	close(progressDone)
	time.Sleep(10 * time.Millisecond)
	state.totalToolTime += duration

	// Check if context was cancelled
	select {
	case <-ctx.Done():
		cancelled = true
		if toolErr == nil {
			resultJSON, _ := json.MarshalIndent(toolResult, "", "  ")
			content = string(resultJSON)
		} else {
			content = fmt.Sprintf("Error: %v", toolErr)
		}
		r.logger.ToolExecuted(tc.Function.Name, duration, false, ctx.Err())
		return
	default:
	}

	// Format successful result
	if toolErr == nil {
		isPlanTool := strings.HasPrefix(tc.Function.Name, "Plan.")

		resultJSON, _ := json.MarshalIndent(toolResult, "", "  ")
		content = string(resultJSON)

		if isPlanTool && r.planManager != nil {
			planText := r.planManager.FormatActivePlan()
			if planText != "" {
				r.writer.ActivePlan(planText)
			}
			r.writer.VerboseOutput(content)
			r.logger.ToolExecuted(tc.Function.Name, duration, true, nil)
		} else {
			summary := ui.GetResultSummary(toolResult)
			var durationStr string
			if dotCount > 0 {
				durationStr = fmt.Sprintf("...%.0fs", duration.Seconds())
			}
			r.writer.ToolResult(summary, durationStr)
			r.writer.VerboseOutput(content)
			r.logger.ToolExecuted(tc.Function.Name, duration, true, nil)
		}
	}

	return
}

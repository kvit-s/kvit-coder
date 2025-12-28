package agent

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// handleFinalAnswer processes assistant responses with no tool calls.
// Returns: shouldContinue (true = retry with modified messages, false = break loop)
func (r *Runner) handleFinalAnswer(
	resp *llm.ChatResponse,
	assistantMsg *llm.Message,
	state *runState,
	rollbackPoint int,
	tasksToolExecuted bool,
	rcfg RunConfig,
	llmDotCount int,
) bool {
	finishReason := resp.Choices[0].FinishReason

	// Check for malformed tool call
	if finishReason == "stop" && r.registry.LooksLikeMalformedToolCall(assistantMsg.Content) {
		r.writer.Warn("Detected malformed tool call in response, auto-continuing...")
		state.messages = append(state.messages, llm.Message{
			Role:    llm.RoleUser,
			Content: "continue",
		})
		return true // shouldContinue
	}

	// Handle empty response with reasoning
	if assistantMsg.Content == "" {
		if assistantMsg.ReasoningContent != "" {
			if state.emptyReasoningRetries < maxEmptyReasoningRetries {
				state.emptyReasoningRetries++
				r.writer.Warn(fmt.Sprintf("LLM returned empty response with reasoning, retrying... (%d/%d)",
					state.emptyReasoningRetries, maxEmptyReasoningRetries))

				// Remove the empty assistant message
				state.messages = state.messages[:len(state.messages)-1]
				return true // shouldContinue
			}

			r.writer.Warn("LLM still confused, using directive prompt...")
			assistantMsg.Content = assistantMsg.ReasoningContent
			assistantMsg.ReasoningContent = ""
			state.messages[len(state.messages)-1] = *assistantMsg

			state.messages = append(state.messages, llm.Message{
				Role:    llm.RoleUser,
				Content: "Use the appropriate tools to complete your task. Make the tool calls now.",
			})

			return true // shouldContinue
		}

		r.writer.Warn(fmt.Sprintf("LLM returned empty response (finish_reason=%s)", finishReason))
	}

	r.writer.Assistant(assistantMsg.Content)

	// Display stats summary
	totalTime := time.Since(state.requestStartTime)
	var statsMsg string
	if state.totalToolCalls > 0 {
		statsMsg = fmt.Sprintf("[%s: %s✨ + %s🔧x%d]",
			ui.FormatDuration(totalTime),
			ui.FormatDuration(state.totalLLMTime),
			ui.FormatDuration(state.totalToolTime),
			state.totalToolCalls)
	} else {
		statsMsg = fmt.Sprintf("[%s: %s✨]",
			ui.FormatDuration(totalTime),
			ui.FormatDuration(state.totalLLMTime))
	}
	r.writer.Info(statsMsg)

	state.agentStats.TotalAgentTime = totalTime
	state.agentStats.TotalLLMTime = state.totalLLMTime
	state.agentStats.TotalToolTime = state.totalToolTime

	// File-first mode: persist messages
	if rcfg.UseFileFirst && r.contextMgr != nil && !tasksToolExecuted && len(state.messages) > rollbackPoint {
		newMessages := state.messages[rollbackPoint:]
		if err := r.contextMgr.AppendMessages(newMessages); err != nil {
			r.writer.Error(fmt.Sprintf("Failed to persist messages: %v", err))
		}
	}

	return false // shouldBreak
}

// checkAndHandleLoops detects various loop patterns and injects intervention messages
func (r *Runner) checkAndHandleLoops(state *runState) {
	if loopInfo := state.loopDetector.DetectLoop(3); loopInfo != nil {
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

		if interventionMsg != "" && len(state.messages) > 0 && state.messages[len(state.messages)-1].Role == llm.RoleTool {
			state.messages[len(state.messages)-1].Content += interventionMsg
		}
	} else if loopInfo := state.loopDetector.DetectErrorLoop(4); loopInfo != nil {
		r.writer.Warn(fmt.Sprintf("Error loop detected: %s has failed %d times consecutively", loopInfo.ToolName, loopInfo.Count))

		interventionMsg := fmt.Sprintf("\n\n<system-reminder>\n"+
			"ERROR LOOP DETECTED: '%s' has failed %d times in a row. "+
			"Try a completely different approach or tool.\n"+
			"</system-reminder>", loopInfo.ToolName, loopInfo.Count)

		if len(state.messages) > 0 && state.messages[len(state.messages)-1].Role == llm.RoleTool {
			state.messages[len(state.messages)-1].Content += interventionMsg
		}
	} else if loopInfo := state.loopDetector.DetectAlternatingLoop(3); loopInfo != nil {
		r.writer.Warn(fmt.Sprintf("Alternating loop detected: %s is repeating the same cycle", loopInfo.ToolName))

		interventionMsg := fmt.Sprintf("\n\n<system-reminder>\n"+
			"ALTERNATING LOOP DETECTED: You are stuck in a cycle repeating '%s' with the same arguments followed by cancellation/undo. "+
			"This pattern has repeated %d times. STOP and try a DIFFERENT approach - perhaps the edit you're attempting is not the right solution.\n"+
			"</system-reminder>", loopInfo.ToolName, loopInfo.Count)

		if len(state.messages) > 0 && state.messages[len(state.messages)-1].Role == llm.RoleTool {
			state.messages[len(state.messages)-1].Content += interventionMsg
		}
	}
}

// handlePostIteration handles post-tool-execution processing including
// backtracking, checkpoints, loop detection, cancelled tools, and plan injection.
// Returns true if the main loop should break.
func (r *Runner) handlePostIteration(
	assistantMsg *llm.Message,
	state *runState,
	rollbackPoint int,
	toolResult *toolExecutionResult,
	rcfg RunConfig,
) bool {
	// Handle backtrack if needed
	if toolResult.shouldBacktrack {
		state.messages = state.messages[:rollbackPoint]

		if toolResult.userMessageToInject != "" {
			// Only inject if last message is not already a user message
			if len(state.messages) == 0 || state.messages[len(state.messages)-1].Role != llm.RoleUser {
				state.messages = append(state.messages, llm.Message{
					Role:    llm.RoleUser,
					Content: toolResult.userMessageToInject,
				})
				r.writer.Debug("Injected user message for backtrack recovery")
			}
		}

		return false // continue loop
	}

	// End checkpoint turn
	if len(assistantMsg.ToolCalls) > 0 && r.checkpointMgr != nil && r.checkpointMgr.Enabled() {
		if err := r.checkpointMgr.EndTurn(); err != nil {
			r.writer.Debug(fmt.Sprintf("Checkpoint error: %v", err))
		}
	}

	// Check for loop detection
	r.checkAndHandleLoops(state)

	// Handle cancelled tools
	if toolResult.toolsCancelled {
		r.writer.Info("Tool execution cancelled - returning to prompt")

		for k := toolResult.lastExecutedIdx + 1; k < len(assistantMsg.ToolCalls); k++ {
			tc := assistantMsg.ToolCalls[k]
			state.messages = append(state.messages, llm.Message{
				Role:       llm.RoleTool,
				Name:       tc.Function.Name,
				ToolCallID: tc.ID,
				Content:    "Error: Cancelled by user",
			})
		}

		state.agentStats.TotalAgentTime = time.Since(state.requestStartTime)
		state.agentStats.TotalLLMTime = state.totalLLMTime
		state.agentStats.TotalToolTime = state.totalToolTime

		if rcfg.UseFileFirst && r.contextMgr != nil && !toolResult.tasksToolExecuted && len(state.messages) > rollbackPoint {
			newMessages := state.messages[rollbackPoint:]
			if err := r.contextMgr.AppendMessages(newMessages); err != nil {
				r.writer.Error(fmt.Sprintf("Failed to persist messages: %v", err))
			}
		}

		return true // break loop
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

			if !anyPlanTool && len(state.messages) > 0 {
				planJSON, err := json.MarshalIndent(map[string]any{
					"task":   activePlan.TaskName,
					"status": activePlan.Status,
					"steps":  activePlan.Steps,
				}, "", "  ")
				if err == nil {
					planContent := fmt.Sprintf("\n\n<system-reminder>\nCurrent plan status:\n```json\n%s\n```\n</system-reminder>", string(planJSON))
					lastIdx := len(state.messages) - 1
					if state.messages[lastIdx].Role == llm.RoleTool {
						state.messages[lastIdx].Content += planContent
					}
				}
			}
		}
	}

	// File-first mode: persist messages
	if rcfg.UseFileFirst && r.contextMgr != nil && !toolResult.tasksToolExecuted && len(state.messages) > rollbackPoint {
		newMessages := state.messages[rollbackPoint:]
		if err := r.contextMgr.AppendMessages(newMessages); err != nil {
			r.writer.Error(fmt.Sprintf("Failed to persist messages: %v", err))
		}
	}

	return false // continue loop
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

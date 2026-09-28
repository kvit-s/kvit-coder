package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/report"
	"github.com/kvit-s/kvit-coder/internal/session"
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
	if finishReason == "stop" && !r.cfg.Agent.IsStrong() && r.registry.LooksLikeMalformedToolCall(assistantMsg.Content) {
		r.writer.Warn("malformed tool call, continuing...")
		state.messages = append(state.messages, llm.Message{
			Role:    llm.RoleUser,
			Content: "continue",
		})
		return true // shouldContinue
	}

	// Handle empty response with reasoning
	if assistantMsg.Content == "" {
		// Retrying an empty answer, and then telling the model to "make the
		// tool calls now", exists for a model that stalls. A model that
		// answers with nothing meant to answer with nothing.
		if assistantMsg.ReasoningContent != "" && !r.cfg.Agent.IsStrong() {
			if state.emptyReasoningRetries < maxEmptyReasoningRetries {
				state.emptyReasoningRetries++
				r.writer.Warn(fmt.Sprintf("empty response, retrying (%d/%d)",
					state.emptyReasoningRetries, maxEmptyReasoningRetries))

				// Remove the empty assistant message
				state.messages = state.messages[:len(state.messages)-1]
				return true // shouldContinue
			}

			r.writer.Warn("empty response, using directive prompt...")
			assistantMsg.Content = assistantMsg.ReasoningContent
			assistantMsg.ReasoningContent = ""
			state.messages[len(state.messages)-1] = *assistantMsg

			state.messages = append(state.messages, llm.Message{
				Role:    llm.RoleUser,
				Content: "Use the appropriate tools to complete your task. Make the tool calls now.",
			})

			return true // shouldContinue
		}

		r.writer.Warn(fmt.Sprintf("empty response (%s)", finishReason))
	}

	// A turn that owes a structured report and has not submitted one is not
	// finished, whatever it just wrote. Say so and let it call Report.
	if msg := r.reportGateMessage(state); msg != "" {
		state.messages = append(state.messages, llm.Message{
			Role:    llm.RoleUser,
			Content: msg,
		})
		return true // shouldContinue
	}

	// The divider separates the step progress from the final answer and
	// carries the turn timing, so nothing prints after the answer itself.
	totalTime := time.Since(state.requestStartTime)
	r.writer.Divider(turnStatsMessage(state, totalTime))
	r.writer.Assistant(assistantMsg.Content)

	r.recordTurnTimes(state, totalTime)

	// File-first mode: persist messages
	if rcfg.UseFileFirst && r.contextMgr != nil && !tasksToolExecuted && len(state.messages) > rollbackPoint {
		newMessages := state.messages[rollbackPoint:]
		if err := r.contextMgr.AppendMessages(newMessages); err != nil {
			r.writer.Error(fmt.Sprintf("cannot persist messages: %v", err))
		}
	}

	return false // shouldBreak
}

// checkAndHandleLoops detects various loop patterns and injects intervention messages
func (r *Runner) checkAndHandleLoops(ctx context.Context, state *runState) {
	if loopInfo := state.loopDetector.DetectLoop(3); loopInfo != nil {
		var interventionMsg string

		if loopInfo.IsError {
			r.writer.Warn(fmt.Sprintf("loop detected: %s called %d times with same error", loopInfo.ToolName, loopInfo.Count))

			interventionMsg = fmt.Sprintf("\n\n<system-reminder>\n"+
				"LOOP DETECTED: You have called '%s' %d times in a row with the same failing result. "+
				"STOP and try a DIFFERENT approach.\n"+
				"</system-reminder>", loopInfo.ToolName, loopInfo.Count)
		} else if loopInfo.IsSuccess {
			r.writer.Warn(fmt.Sprintf("loop detected: %s called %d times with same args and result", loopInfo.ToolName, loopInfo.Count))

			interventionMsg = fmt.Sprintf("\n\n<system-reminder>\n"+
				"LOOP DETECTED: You have called '%s' %d times in a row with identical arguments and results. "+
				"You are stuck in a loop. STOP and try a DIFFERENT approach.\n"+
				"</system-reminder>", loopInfo.ToolName, loopInfo.Count)
		}

		if interventionMsg != "" && len(state.messages) > 0 && state.messages[len(state.messages)-1].Role == llm.RoleTool {
			state.messages[len(state.messages)-1].Content += interventionMsg
		}
	} else if loopInfo := state.loopDetector.DetectErrorLoop(4); loopInfo != nil {
		r.writer.Warn(fmt.Sprintf("error loop: %s failed %d times in a row", loopInfo.ToolName, loopInfo.Count))

		interventionMsg := fmt.Sprintf("\n\n<system-reminder>\n"+
			"ERROR LOOP DETECTED: '%s' has failed %d times in a row. "+
			"Try a completely different approach or tool.\n"+
			"</system-reminder>", loopInfo.ToolName, loopInfo.Count)

		if len(state.messages) > 0 && state.messages[len(state.messages)-1].Role == llm.RoleTool {
			state.messages[len(state.messages)-1].Content += interventionMsg
		}
	} else if loopInfo := state.loopDetector.DetectAlternatingLoop(3); loopInfo != nil {
		r.writer.Warn(fmt.Sprintf("alternating loop: %s repeating same cycle", loopInfo.ToolName))

		// Interrogate on first detection of an alternating loop (e.g. Edit↔cancel cycles).
		if r.interrogator.Enabled() {
			r.interrogator.Interrogate(ctx, Episode{
				Trigger:       TriggerAlternatingLoop,
				Key:           "alternating_loop:" + loopInfo.ToolName,
				TriggerCount:  loopInfo.Count,
				OffendingTool: loopInfo.ToolName,
				PriorResult:   lastToolResult(state.messages),
			}, state)
		}

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
	ctx context.Context,
	assistantMsg *llm.Message,
	state *runState,
	rollbackPoint int,
	toolResult *toolExecutionResult,
	rcfg RunConfig,
) bool {
	// Handle backtrack if needed
	if toolResult.shouldBacktrack {
		// These messages may already be in the session, so say they were taken
		// back rather than pretend they never happened.
		r.discard(state, rollbackPoint)
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
	r.checkAndHandleLoops(ctx, state)

	// Handle cancelled tools
	if toolResult.toolsCancelled {
		r.writer.Info("tool execution cancelled, back to prompt")

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
				r.writer.Error(fmt.Sprintf("cannot persist messages: %v", err))
			}
		}

		return true // break loop
	}

	// File-first mode: persist messages
	if rcfg.UseFileFirst && r.contextMgr != nil && !toolResult.tasksToolExecuted && len(state.messages) > rollbackPoint {
		newMessages := state.messages[rollbackPoint:]
		if err := r.contextMgr.AppendMessages(newMessages); err != nil {
			r.writer.Error(fmt.Sprintf("cannot persist messages: %v", err))
		}
	}

	return false // continue loop
}

// handleToolError handles errors from tool validation or execution.
// It decides whether to backtrack (discard and retry) or add error to history.
func (r *Runner) handleToolError(
	ctx context.Context,
	err error,
	tc llm.ToolCall,
	state *runState,
	rollbackPoint int,
	promptTokens, completionTokens int,
	requestCost float64,
) backtrackResult {
	backtracker := state.backtracker

	// Check if this is a backtrackable (semantic) error
	if tools.IsBacktrackable(err) && backtracker.ShouldBacktrack(rollbackPoint) {
		errMsg := err.Error()
		if idx := strings.Index(errMsg, "\n"); idx > 0 {
			errMsg = errMsg[:idx]
		}
		// Anomaly: repeated backtrack retries at the same point. Interrogate one retry
		// before the limit (last-chance capture before recovery gives up).
		if r.interrogator.Enabled() && backtracker.GetRetryCount() >= backtracker.GetMaxRetries()-1 {
			r.interrogator.Interrogate(ctx, Episode{
				Trigger:       TriggerBacktrack,
				Key:           fmt.Sprintf("backtrack:%d:%s", rollbackPoint, tc.Function.Name),
				TriggerCount:  backtracker.GetRetryCount(),
				OffendingTool: tc.Function.Name,
				OffendingArgs: tc.Function.Arguments,
				PriorResult:   err.Error(),
			}, state)
		}
		r.writer.Info(fmt.Sprintf("retry [%d/%d]: %s - %s",
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

			r.writer.Warn(fmt.Sprintf("backtrack limit for %s, injecting user message", tc.Function.Name))
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
		r.writer.Warn(fmt.Sprintf("backtrack limit for %s, adding error to history", tc.Function.Name))
		backtracker.ResetAtPoint()
	}

	return backtrackResult{
		shouldBacktrack: false,
		toolName:        tc.Function.Name,
		reason:          err.Error(),
	}
}

// turnStatsMessage is the timing that rides on the divider above a turn's
// ending, whether that ending is prose or a report.
func turnStatsMessage(state *runState, totalTime time.Duration) string {
	if state.totalToolCalls > 0 {
		return fmt.Sprintf("[%s: %s llm + %s tools x%d]",
			ui.FormatDuration(totalTime),
			ui.FormatDuration(state.totalLLMTime),
			ui.FormatDuration(state.totalToolTime),
			state.totalToolCalls)
	}
	return fmt.Sprintf("[%s: %s llm]",
		ui.FormatDuration(totalTime),
		ui.FormatDuration(state.totalLLMTime))
}

func (r *Runner) recordTurnTimes(state *runState, totalTime time.Duration) {
	state.agentStats.TotalAgentTime = totalTime
	state.agentStats.TotalLLMTime = state.totalLLMTime
	state.agentStats.TotalToolTime = state.totalToolTime
}

// maxReportNudges bounds how many times one turn is told to submit a report it
// has not submitted. A model that will not produce one is better off ending
// with prose than spending the iteration budget being asked again.
const maxReportNudges = 2

// reportGateMessage returns what to tell a model that stopped without the
// report its turn owes, or "" when nothing is owed. A turn owes one when the
// Report tool is enabled, its mode covers what this turn did, and no report has
// been accepted; a turn that already used up its repair attempts owes nothing,
// because it was told to stop repairing and end with prose.
func (r *Runner) reportGateMessage(state *runState) string {
	if r.toolCtx == nil || r.registry == nil || r.registry.Get("Report") == nil {
		return ""
	}
	if r.toolCtx.AcceptedReport() != nil {
		return ""
	}
	if r.toolCtx.ReportRepairs() >= r.reportRepairBudget() {
		return ""
	}
	if !r.cfg.Tools.Report.Required(state.totalToolCalls > 0, r.toolCtx.MutatedThisTurn()) {
		return ""
	}
	if state.reportNudges >= maxReportNudges {
		return ""
	}
	state.reportNudges++
	r.writer.Warn("turn ended without a report, asking for one")
	return "<system-reminder>\n" +
		"This turn has not submitted a report. Call Report now with what you just said, " +
		"in the schema: a task_status, a one-sentence headline, and the typed blocks behind it. " +
		"If there is an obvious next move (apply the fix, commit the change, debug the failed test), " +
		"propose each as its own \"next\" block rather than describing it in prose. " +
		"The turn ends when the report is accepted, so put everything in the report rather than " +
		"after it.\n</system-reminder>"
}

func (r *Runner) reportRepairBudget() int {
	if r.cfg == nil || r.cfg.Tools.Report.MaxRepairAttempts <= 0 {
		return config.DefaultReportRepairAttempts
	}
	return r.cfg.Tools.Report.MaxRepairAttempts
}

// finishOnReport ends a turn on an accepted report: the divider with the
// turn's timing, then the report itself. Under the interactive front end the
// card is drawn there, from the session, so the agent prints nothing and the
// user sees one card rather than two.
func (r *Runner) finishOnReport(rep *report.Report, state *runState) {
	totalTime := time.Since(state.requestStartTime)
	r.writer.Divider(turnStatsMessage(state, totalTime))
	if !session.FromUI() {
		r.writer.Report(rep)
	}
	r.recordTurnTimes(state, totalTime)
}

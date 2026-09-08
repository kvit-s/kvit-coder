package agent

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// slowRequestNotice is how often to say how long a model request has been
// running. Long enough not to chatter during ordinary thinking, short enough
// that a stalled request is obvious well before the timeout.
const slowRequestNotice = 90 * time.Second

// callLLM makes an LLM API call with progress indicator and handles errors.
// It returns the response and metadata about what action to take next.
func (r *Runner) callLLM(ctx context.Context, state *runState) (*llmCallResult, error) {
	// Silent until the first dot: a fast turn prints only its step header.
	r.writer.ToolProgress("● ")

	startTime := time.Now()
	llmDone := make(chan bool)
	llmDotCount := 0
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		// A row of dots says something is happening but not for how long, and
		// a request that stalls for half an hour looks like one that is
		// thinking hard. Say the elapsed time now and then so the difference
		// is visible without counting dots.
		notice := time.NewTicker(slowRequestNotice)
		defer notice.Stop()
		for {
			select {
			case <-ticker.C:
				r.writer.ToolProgress(".")
				llmDotCount++
			case <-notice.C:
				r.writer.Warn(fmt.Sprintf("still waiting for the model after %s",
					time.Since(startTime).Round(time.Second)))
			case <-llmDone:
				return
			}
		}
	}()

	resp, err := r.chatWithImages(ctx, state)

	close(llmDone)
	time.Sleep(10 * time.Millisecond)
	duration := time.Since(startTime)
	state.totalLLMTime += duration

	// Dots (if any) drew in place on the progress stream; the step header
	// clears that row itself, so no newline is needed here. (The old code
	// printed one to stdout, the final-answer stream.)

	result := &llmCallResult{
		response: resp,
		duration: duration,
	}

	if err != nil {
		return r.handleLLMError(ctx, err, state, result, llmDotCount)
	}

	return result, nil
}

// chatWithImages hydrates message attachments from disk and sends the
// request, refusing early with a clear error when the images alone would
// overflow the context window rather than failing mid-loop on a 400.
func (r *Runner) chatWithImages(ctx context.Context, state *runState) (*llm.ChatResponse, error) {
	state.messages = llm.HydrateImages(state.messages)

	if maxCtx := r.cfg.LLM.Context; maxCtx > 0 {
		if est := llm.ImageTokensForMessages(state.messages); est > 0 && state.totalTokens+est >= maxCtx {
			return nil, fmt.Errorf("images need ~%d tokens but only %d of %d context remain: start a new session or continue without images",
				est, maxCtx-state.totalTokens, maxCtx)
		}
	}

	return r.llmClient.Chat(ctx, llm.ChatRequest{
		Model:       r.cfg.LLM.Model,
		Messages:    state.messages,
		Tools:       r.registry.Specs(),
		ToolChoice:  "auto",
		Temperature: r.cfg.LLM.Temperature,
		MaxTokens:   r.cfg.LLM.MaxTokens,
		Stream:      false,
	})
}

// handleLLMError processes errors from LLM calls and determines retry strategy
func (r *Runner) handleLLMError(ctx context.Context, err error, state *runState, result *llmCallResult, llmDotCount int) (*llmCallResult, error) {
	// Check if it was a cancellation
	if ctx.Err() == context.Canceled {
		r.writer.Info("llm call cancelled, back to prompt")

		// If the last message is a tool result, add a dummy assistant message
		if len(state.messages) > 0 && state.messages[len(state.messages)-1].Role == llm.RoleTool {
			state.messages = append(state.messages, llm.Message{
				Role:    llm.RoleAssistant,
				Content: "[Operation cancelled by user]",
			})
		}

		state.agentStats.TotalAgentTime = time.Since(state.requestStartTime)
		state.agentStats.TotalLLMTime = state.totalLLMTime
		state.agentStats.TotalToolTime = state.totalToolTime
		result.cancelled = true
		result.shouldBreak = true
		return result, nil
	}

	// A deadline on the run context is the caller's own time budget expiring
	// (thinkbench's timeout_per_run, say), not a fault of the model endpoint.
	// The bare error text is "context deadline exceeded", which reads like a
	// network failure, so say where it came from and how long the call had run.
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		elapsed := time.Since(state.requestStartTime)
		r.writer.Error(fmt.Sprintf(
			"run time budget expired after %s (llm call %s)",
			elapsed.Round(time.Second), result.duration.Round(time.Second)))
		r.logger.Error("run time budget expired during LLM call", err)

		state.agentStats.TotalAgentTime = elapsed
		state.agentStats.TotalLLMTime = state.totalLLMTime
		state.agentStats.TotalToolTime = state.totalToolTime
		result.timedOut = true
		result.shouldBreak = true
		return result, nil
	}

	errStr := err.Error()

	// Handle 400 errors (likely context overflow)
	if strings.Contains(errStr, "API error 400") && state.contextOverflowRetries < maxContextOverflowRetries {
		state.contextOverflowRetries++
		r.writer.Warn(fmt.Sprintf("server error, retrying (%d/%d)",
			state.contextOverflowRetries, maxContextOverflowRetries))

		// Replace all recent tool result messages with server error
		for j := len(state.messages) - 1; j >= 0 && state.messages[j].Role == llm.RoleTool; j-- {
			state.messages[j].Content = "[Server error processing tool output. The command ran successfully but the output cannot be processed by the server. You can try running the same command again or try a different approach.]"
		}

		result.shouldContinue = true
		return result, nil
	}

	// Check if this is a 400 error but we've exhausted retries
	if strings.Contains(errStr, "API error 400") && state.contextOverflowRetries >= maxContextOverflowRetries {
		state.differentApproachAttempts++

		// Check if we've exhausted "different approach" attempts
		if state.differentApproachAttempts >= maxDifferentApproachAttempts {
			r.writer.Error(fmt.Sprintf("server error persists, giving up after %d attempts: %s",
				maxDifferentApproachAttempts, ui.SingleLine(errStr, 120)))
			state.agentStats.TotalAgentTime = time.Since(state.requestStartTime)
			state.agentStats.TotalLLMTime = state.totalLLMTime
			state.agentStats.TotalToolTime = state.totalToolTime
			result.shouldBreak = true
			return result, fmt.Errorf("persistent server error: %s", errStr)
		}

		r.writer.Warn(fmt.Sprintf("server error persists, trying different approach (%d/%d)",
			state.differentApproachAttempts, maxDifferentApproachAttempts))

		for j := len(state.messages) - 1; j >= 0 && state.messages[j].Role == llm.RoleTool; j-- {
			state.messages[j].Content = "[Server error: Unable to process tool output after multiple retries.]"
		}

		// Add user message to prompt retry (required by some providers like Mistral
		// that expect the last message to be from user or tool role)
		state.messages = append(state.messages, llm.Message{
			Role:    llm.RoleUser,
			Content: "[System: Server error occurred. Please try a different approach.]",
		})

		state.contextOverflowRetries = 0
		result.shouldContinue = true
		return result, nil
	}

	r.writer.Error(fmt.Sprintf("llm call failed: %s", ui.SingleLine(err.Error(), 160)))
	r.logger.Error("LLM call failed", err)

	state.agentStats.TotalAgentTime = time.Since(state.requestStartTime)
	state.agentStats.TotalLLMTime = state.totalLLMTime
	state.agentStats.TotalToolTime = state.totalToolTime
	result.shouldBreak = true
	return result, nil
}

// processLLMResponse validates and processes the LLM response.
// It handles provider errors with retries, normalizes responses, and updates stats.
// Returns the assistant message and whether processing should continue to next iteration.
func (r *Runner) processLLMResponse(ctx context.Context, resp *llm.ChatResponse, state *runState, duration time.Duration) (*llm.Message, bool) {
	// Extract assistant message
	if len(resp.Choices) == 0 {
		r.writer.Error("no response from model")
		state.agentStats.TotalAgentTime = time.Since(state.requestStartTime)
		state.agentStats.TotalLLMTime = state.totalLLMTime
		state.agentStats.TotalToolTime = state.totalToolTime
		return nil, false
	}

	// Check for upstream provider error
	if resp.Choices[0].Error != nil {
		retryResult := r.handleProviderError(ctx, resp, state)
		if retryResult == nil {
			return nil, false
		}
		resp = retryResult
	}

	state.consecutiveProviderFailures = 0

	assistantMsg := resp.Choices[0].Message

	// Note whether the model gave us anything readable to merge, before the
	// normalizer folds it away.
	state.responsesSeen++
	if assistantMsg.ReasoningContent != "" {
		state.reasoningSeen = true
	}

	// Apply response normalization middleware
	toolCallsExtracted := state.normalizer.NormalizeResponse(&assistantMsg)
	if toolCallsExtracted {
		resp.Choices[0].FinishReason = "tool_calls"
	}

	// Prevent consecutive assistant messages
	state.messages, _ = llm.PreventConsecutiveAssistant(state.messages)

	// Inject turn number for context tools
	if r.contextMiddleware != nil && assistantMsg.Content != "" {
		assistantMsg.Content = r.contextMiddleware.ProcessAssistantMessage(assistantMsg.Content)
	}

	// Get token counts
	promptTokens := resp.Usage.PromptTokens
	completionTokens := resp.Usage.CompletionTokens
	state.totalTokens = promptTokens + completionTokens

	state.agentStats.TotalPromptTokens += promptTokens
	state.agentStats.TotalCompletionTokens += completionTokens
	if state.totalTokens > state.agentStats.MaxContextUsed {
		state.agentStats.MaxContextUsed = state.totalTokens
	}
	r.checkContextBudget(promptTokens, state)
	r.checkThinkingArrives(state)

	// Ask the endpoint for cost and native token counts. This is OpenRouter's
	// /generation endpoint and 404s elsewhere, so it is opt-in; it is also the
	// only place that learns the cost of this request, which backtracking
	// accounting reads back through state.lastRequestCost.
	state.lastRequestCost = 0
	if resp.ID != "" && r.cfg.LLM.GenerationStats {
		genStats, err := r.llmClient.GetGenerationStats(context.Background(), resp.ID)
		if err == nil {
			if genStats.Data.NativeTokensPrompt > 0 {
				promptTokens = genStats.Data.NativeTokensPrompt
				completionTokens = genStats.Data.NativeTokensCompletion
				state.totalTokens = promptTokens + completionTokens
			}
			state.lastRequestCost = genStats.Data.TotalCost
			state.agentStats.TotalCacheReadTokens += genStats.Data.NativeTokensCached
			state.agentStats.TotalCost += genStats.Data.TotalCost
			state.agentStats.CacheDiscount += genStats.Data.CacheDiscount
			state.agentStats.TotalPromptMS += genStats.Data.Latency
			state.agentStats.TotalGenerationMS += genStats.Data.GenerationTime
		}
	}
	state.agentStats.Steps++

	r.logger.LLMCall(r.cfg.LLM.Model, promptTokens, completionTokens, duration)

	return &assistantMsg, true
}

// handleProviderError handles upstream provider errors with retry logic
func (r *Runner) handleProviderError(ctx context.Context, resp *llm.ChatResponse, state *runState) *llm.ChatResponse {
	choiceErr := resp.Choices[0].Error
	r.writer.Warn(fmt.Sprintf("provider error %d, retrying: %s", choiceErr.Code, ui.SingleLine(choiceErr.Message, 120)))

	retryStart := time.Now()
	retryResp, retryErr := r.llmClient.Chat(ctx, llm.ChatRequest{
		Model:       r.cfg.LLM.Model,
		Messages:    state.messages,
		Tools:       r.registry.Specs(),
		ToolChoice:  "auto",
		Temperature: r.cfg.LLM.Temperature,
		MaxTokens:   r.cfg.LLM.MaxTokens,
		Stream:      false,
	})
	state.totalLLMTime += time.Since(retryStart)

	if retryErr != nil || len(retryResp.Choices) == 0 || retryResp.Choices[0].Error != nil {
		state.consecutiveProviderFailures++

		if state.consecutiveProviderFailures >= maxProviderFailures {
			r.writer.Error(fmt.Sprintf("provider failed %d times, stopping", state.consecutiveProviderFailures))
			state.agentStats.TotalAgentTime = time.Since(state.requestStartTime)
			state.agentStats.TotalLLMTime = state.totalLLMTime
			state.agentStats.TotalToolTime = state.totalToolTime
			return nil
		}

		// Add messages to guide LLM to retry
		assistantContent := "[Response failed due to server error]"
		if resp.Choices[0].Message.Content != "" {
			assistantContent = resp.Choices[0].Message.Content
		}
		state.messages = append(state.messages, llm.Message{
			Role:    llm.RoleAssistant,
			Content: assistantContent,
		})
		state.messages = append(state.messages, llm.Message{
			Role:    llm.RoleUser,
			Content: "[System: Your previous response caused a server error. Please try again.]",
		})

		return nil
	}

	r.writer.Info("retry succeeded")
	return retryResp
}

// contextWarnFraction is how full the window has to be before it is worth
// saying so. Below this there is nothing useful to do about it.
const contextWarnFraction = 0.8

// checkContextBudget compares what the last request actually cost against the
// window the model has, and says so once when it is running out. Until now
// llm.context was only a number in a display string: nothing compared anything
// to it, so the first sign of trouble was a 400 from the endpoint.
func (r *Runner) checkContextBudget(promptTokens int, state *runState) {
	limit := r.cfg.LLM.Context
	if limit <= 0 || promptTokens <= 0 || state.contextWarned {
		return
	}
	if float64(promptTokens) < contextWarnFraction*float64(limit) {
		return
	}
	state.contextWarned = true
	r.writer.Warn(fmt.Sprintf(
		"context %d/%d tokens (%.0f%%), finish up; new session starts clean",
		promptTokens, limit, 100*float64(promptTokens)/float64(limit)))
}

// thinkingSilenceLimit is how many answers to wait for before concluding that
// this model is not going to send readable reasoning.
const thinkingSilenceLimit = 3

// checkThinkingArrives says so, once, when merge_thinking is on but the model
// never sends anything to merge.
//
// A reasoning model returns its thinking as encrypted blocks, which kvit-coder
// can replay but nobody can read, plus an optional summary — and the summary is
// the only readable part. Whether one is sent is the provider's choice: on the
// endpoint this was written against, summaries arrive on most turns that answer
// directly and on few that call a tool, which is nearly every turn of an agent.
// Without this notice the setting simply appears to do nothing.
func (r *Runner) checkThinkingArrives(state *runState) {
	if !r.cfg.LLM.MergeThinking || state.reasoningSeen || state.thinkingWarned {
		return
	}
	if state.responsesSeen < thinkingSilenceLimit {
		return
	}
	state.thinkingWarned = true
	r.writer.Warn(fmt.Sprintf(
		"merge_thinking on but no readable reasoning in %d answers (encrypted thinking, summaries only)",
		state.responsesSeen))
}

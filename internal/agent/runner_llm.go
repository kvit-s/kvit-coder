package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// callLLM makes an LLM API call with progress indicator and handles errors.
// It returns the response and metadata about what action to take next.
func (r *Runner) callLLM(ctx context.Context, state *runState) (*llmCallResult, error) {
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

	resp, err := r.llmClient.Chat(ctx, llm.ChatRequest{
		Model:       r.cfg.LLM.Model,
		Messages:    state.messages,
		Tools:       r.registry.Specs(),
		ToolChoice:  "auto",
		Temperature: r.cfg.LLM.Temperature,
		MaxTokens:   r.cfg.LLM.MaxTokens,
		Stream:      false,
	})
	close(llmDone)
	time.Sleep(10 * time.Millisecond)
	duration := time.Since(startTime)
	state.totalLLMTime += duration

	// Print newline after sparkles/dots before showing response
	if llmDotCount == 0 {
		fmt.Print("\n")
	}

	result := &llmCallResult{
		response: resp,
		duration: duration,
	}

	if err != nil {
		return r.handleLLMError(ctx, err, state, result, llmDotCount)
	}

	return result, nil
}

// handleLLMError processes errors from LLM calls and determines retry strategy
func (r *Runner) handleLLMError(ctx context.Context, err error, state *runState, result *llmCallResult, llmDotCount int) (*llmCallResult, error) {
	// Check if it was a cancellation
	if ctx.Err() == context.Canceled {
		r.writer.Info("LLM call cancelled - returning to prompt")

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

	errStr := err.Error()

	// Handle 400 errors (likely context overflow)
	if strings.Contains(errStr, "API error 400") && state.contextOverflowRetries < maxContextOverflowRetries {
		state.contextOverflowRetries++
		r.writer.Warn(fmt.Sprintf("Server error on request (attempt %d/%d) - replacing tool output and retrying...",
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
			r.writer.Error(fmt.Sprintf("Server error persists after %d attempts - giving up. Last error: %s",
				maxDifferentApproachAttempts, errStr))
			state.agentStats.TotalAgentTime = time.Since(state.requestStartTime)
			state.agentStats.TotalLLMTime = state.totalLLMTime
			state.agentStats.TotalToolTime = state.totalToolTime
			result.shouldBreak = true
			return result, fmt.Errorf("persistent server error: %s", errStr)
		}

		r.writer.Warn(fmt.Sprintf("Server error persists after %d retries - asking LLM to try a different approach (%d/%d)",
			maxContextOverflowRetries, state.differentApproachAttempts, maxDifferentApproachAttempts))

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

	r.writer.Error(fmt.Sprintf("%v", err))
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
		r.writer.Error("No response from model")
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

	// Query generation stats for extended data
	if resp.ID != "" {
		genStats, err := r.llmClient.GetGenerationStats(context.Background(), resp.ID)
		if err == nil {
			if genStats.Data.NativeTokensPrompt > 0 {
				promptTokens = genStats.Data.NativeTokensPrompt
				completionTokens = genStats.Data.NativeTokensCompletion
				state.totalTokens = promptTokens + completionTokens
			}
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
	r.writer.Warn(fmt.Sprintf("Provider error (code %d): %s - retrying...", choiceErr.Code, choiceErr.Message))

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
			r.writer.Error(fmt.Sprintf("Provider failed %d times consecutively - stopping", state.consecutiveProviderFailures))
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

	r.writer.Info("Retry succeeded")
	return retryResp
}

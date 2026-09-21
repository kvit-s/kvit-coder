package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// fakeClient answers Chat from a fixed script and records every request it was
// given, so a test can assert on the message history the loop built.
type fakeClient struct {
	mu       sync.Mutex
	script   []scriptStep
	next     int
	requests []llm.ChatRequest

	// statsErr, when set, is returned by GetGenerationStats. The default is an
	// error, which is what a non-OpenRouter endpoint gives.
	statsErr error
	stats    *llm.GenerationStats
	// statsCalls counts GetGenerationStats calls, so a test can assert the loop
	// asks at most once per response.
	statsCalls int
}

// scriptStep is one scripted answer. Exactly one of resp or err is used; hook,
// when set, runs before the answer is produced and can mutate shared state (a
// test uses it to push into an inbox mid-turn).
type scriptStep struct {
	resp *llm.ChatResponse
	err  error
	hook func()
}

func newFakeClient(steps ...scriptStep) *fakeClient {
	return &fakeClient{
		script:   steps,
		statsErr: fmt.Errorf("generation stats not available"),
	}
}

func (f *fakeClient) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// Copy the message slice: the runner keeps appending to its own backing
	// array, so holding the slice header alone would show later mutations.
	captured := req
	captured.Messages = append([]llm.Message(nil), req.Messages...)
	f.requests = append(f.requests, captured)

	if f.next >= len(f.script) {
		return nil, fmt.Errorf("fake client: script exhausted after %d responses", len(f.script))
	}
	step := f.script[f.next]
	f.next++
	if step.hook != nil {
		step.hook()
	}
	if step.err != nil {
		return nil, step.err
	}
	return step.resp, nil
}

func (f *fakeClient) GetGenerationStats(ctx context.Context, id string) (*llm.GenerationStats, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.statsCalls++
	if f.statsErr != nil {
		return nil, f.statsErr
	}
	return f.stats, nil
}

// callCount is how many Chat calls the script has served.
func (f *fakeClient) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.next
}

// --- response builders -------------------------------------------------------

// answer is an assistant turn with no tool calls: the final answer.
func answer(text string) scriptStep {
	return scriptStep{resp: &llm.ChatResponse{
		Choices: []llm.Choice{{
			Message:      llm.Message{Role: llm.RoleAssistant, Content: text},
			FinishReason: "stop",
		}},
	}}
}

// calls is an assistant turn that asks for the given tool calls.
func calls(tcs ...llm.ToolCall) scriptStep {
	return scriptStep{resp: &llm.ChatResponse{
		Choices: []llm.Choice{{
			Message:      llm.Message{Role: llm.RoleAssistant, ToolCalls: tcs},
			FinishReason: "tool_calls",
		}},
	}}
}

// providerError is a 200 response carrying an upstream provider error, which
// the loop retries once before giving up.
func providerError(code int, msg string) scriptStep {
	return scriptStep{resp: &llm.ChatResponse{
		Choices: []llm.Choice{{
			Message: llm.Message{Role: llm.RoleAssistant},
			Error:   &llm.ChoiceError{Code: code, Message: msg},
		}},
	}}
}

func toolCall(id, name string, args any) llm.ToolCall {
	raw, err := json.Marshal(args)
	if err != nil {
		panic(err)
	}
	return llm.ToolCall{
		ID:   id,
		Type: "function",
		Function: llm.ToolCallFunction{
			Name:      name,
			Arguments: string(raw),
		},
	}
}

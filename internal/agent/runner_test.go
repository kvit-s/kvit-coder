package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// scriptedTool is a tool whose Check and Call a test supplies. It stands in for
// Read, Edit and the rest so the loop can be exercised without touching disk.
type scriptedTool struct {
	name  string
	check func(context.Context, json.RawMessage) error
	call  func(context.Context, json.RawMessage) (any, error)
}

func (s *scriptedTool) Name() string        { return s.name }
func (s *scriptedTool) Description() string { return "test tool " + s.name }
func (s *scriptedTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{"arg": map[string]any{"type": "string"}},
	}
}
func (s *scriptedTool) Check(ctx context.Context, args json.RawMessage) error {
	if s.check == nil {
		return nil
	}
	return s.check(ctx, args)
}
func (s *scriptedTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	if s.call == nil {
		return map[string]any{"ok": true}, nil
	}
	return s.call(ctx, args)
}
func (s *scriptedTool) PromptSection() string      { return "" }
func (s *scriptedTool) PromptCategory() string     { return "filesystem" }
func (s *scriptedTool) PromptOrder() int           { return 1 }
func (s *scriptedTool) PromptTemplateName() string { return "" }

// testConfig is the configuration every runner test starts from: the loop's own
// mechanisms on, everything that would make an extra model call off.
func testConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Agent.MaxIterations = 10
	cfg.LLM.Model = "test-model"
	cfg.Workspace.Root = "/tmp"
	// An interrogator makes its own model calls, which would consume script
	// steps and desynchronise every assertion below.
	cfg.Diagnostics.InterrogateOnAnomaly = false
	cfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{}
	return cfg
}

// newTestRunner wires a Runner around a scripted client, with output captured
// rather than written to the process streams.
func newTestRunner(t *testing.T, cfg *config.Config, client LLMClient, ts ...tools.Tool) (*Runner, *bytes.Buffer) {
	t.Helper()

	var out bytes.Buffer
	writer := ui.NewWriter(0)
	writer.SetStdout(&out)
	writer.SetStderr(&out)
	// Headless routes every message through the writer's own streams; the
	// default path prints through the color package straight to os.Stdout,
	// which a test cannot capture.
	writer.SetHeadless(true)

	logger, err := NewLogger("", false)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}

	registry := tools.NewRegistry()
	for _, tool := range ts {
		registry.Enable(tool)
	}

	return NewRunner(RunnerOptions{
		Cfg:       cfg,
		LLMClient: client,
		Registry:  registry,
		Writer:    writer,
		Logger:    logger,
		ToolCtx:   tools.NewToolContext(),
	}), &out
}

// shape renders a message history as "role[:name]" strings, which is what the
// assertions below actually care about.
func shape(msgs []llm.Message) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		s := string(m.Role)
		if m.Role == llm.RoleAssistant && len(m.ToolCalls) > 0 {
			s += "+tool_calls"
		}
		if m.Role == llm.RoleTool {
			s += ":" + m.Name
		}
		out = append(out, s)
	}
	return out
}

func assertShape(t *testing.T, msgs []llm.Message, want ...string) {
	t.Helper()
	got := shape(msgs)
	if len(got) != len(want) {
		t.Fatalf("history has %d messages, want %d\n got: %v\nwant: %v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("message %d is %q, want %q\n got: %v\nwant: %v", i, got[i], want[i], got, want)
		}
	}
}

func userStart(text string) []llm.Message {
	return []llm.Message{{Role: llm.RoleUser, Content: text}}
}

// TestOneToolCallThenAnswer is the baseline: the loop asks for a tool, records
// its result, and stops on the assistant turn that has no tool calls.
func TestOneToolCallThenAnswer(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "echo", map[string]string{"arg": "hello"})),
		answer("done"),
	)
	echo := &scriptedTool{name: "echo", call: func(_ context.Context, args json.RawMessage) (any, error) {
		return map[string]any{"echoed": string(args)}, nil
	}}
	runner, _ := newTestRunner(t, testConfig(), client, echo)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("say hello")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls",
		"tool:echo",
		"assistant",
	)
	if client.callCount() != 2 {
		t.Errorf("made %d model calls, want 2", client.callCount())
	}
	if res.Cancelled || res.TimedOut {
		t.Errorf("run reported cancelled=%v timedOut=%v, want both false", res.Cancelled, res.TimedOut)
	}
	// The second request must carry the whole history the loop built so far.
	assertShape(t, client.requests[1].Messages, "user", "assistant+tool_calls", "tool:echo")
}

// TestIterationCapReached: the loop stops at max_tool_iterations and says so
// rather than ending silently with no answer.
func TestIterationCapReached(t *testing.T) {
	cfg := testConfig()
	cfg.Agent.MaxIterations = 3

	// Distinct arguments each time, so the duplicate-call guard never fires and
	// the cap is what actually stops the loop.
	client := newFakeClient(
		calls(toolCall("c1", "echo", map[string]string{"arg": "a"})),
		calls(toolCall("c2", "echo", map[string]string{"arg": "b"})),
		calls(toolCall("c3", "echo", map[string]string{"arg": "c"})),
	)
	runner, out := newTestRunner(t, cfg, client, &scriptedTool{name: "echo"})

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if client.callCount() != 3 {
		t.Errorf("made %d model calls, want 3 (the cap)", client.callCount())
	}
	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:echo",
		"assistant", // the notice that the budget ran out
	)
	if !res.BudgetExhausted {
		t.Error("BudgetExhausted is false after the loop hit the iteration cap")
	}
	if !strings.Contains(out.String(), "iteration budget") {
		t.Errorf("nothing about the iteration budget was reported:\n%s", out.String())
	}
}

// TestDuplicateCallDetection: repeating a call verbatim earns an error result,
// and doing it maxConsecutiveDuplicates times stops the loop.
func TestDuplicateCallDetection(t *testing.T) {
	same := func(id string) scriptStep {
		return calls(toolCall(id, "echo", map[string]string{"arg": "same"}))
	}
	client := newFakeClient(same("c1"), same("c2"), same("c3"), same("c4"), answer("unreachable"))
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Calls 2 and 3 come back as duplicate errors; call 4 crosses
	// maxConsecutiveDuplicates and stops the loop before a result is recorded.
	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls",
	)
	dupes := 0
	for _, m := range res.FinalMessages {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, "DUPLICATE CALL ERROR") {
			dupes++
		}
	}
	if dupes != 2 {
		t.Errorf("history has %d duplicate-call errors, want 2", dupes)
	}
	if client.callCount() != 4 {
		t.Errorf("made %d model calls, want 4", client.callCount())
	}
}

// TestBacktrackTruncatesHistory: a semantic tool error with backtracking on
// discards the assistant turn that caused it, so the model never sees it.
func TestBacktrackTruncatesHistory(t *testing.T) {
	cfg := testConfig()
	cfg.Backtrack.Enabled = true
	cfg.Backtrack.MaxRetries = 5

	client := newFakeClient(
		calls(toolCall("c1", "bad", map[string]string{"arg": "x"})),
		answer("recovered"),
	)
	bad := &scriptedTool{name: "bad", call: func(context.Context, json.RawMessage) (any, error) {
		return nil, tools.SemanticErrorf("that path does not exist")
	}}
	runner, _ := newTestRunner(t, cfg, client, bad)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The failed assistant turn and its tool result are both gone: history is
	// back to the rollback point, then the recovered answer.
	assertShape(t, res.FinalMessages, "user", "assistant")
	if res.Stats.BacktrackCount != 1 {
		t.Errorf("BacktrackCount is %d, want 1", res.Stats.BacktrackCount)
	}
	// The retry request must not mention the discarded call.
	assertShape(t, client.requests[1].Messages, "user")
}

// TestLoopReminderAppendedToLastToolResult: the loop detector's warning is
// appended to the existing tool result, never added as a new message, because a
// new message would break the assistant/tool pairing the API requires.
func TestLoopReminderAppendedToLastToolResult(t *testing.T) {
	cfg := testConfig()
	cfg.Agent.MaxIterations = 5

	// Four failures in a row on the same tool with different arguments: that is
	// DetectErrorLoop's threshold, and different arguments keep the
	// duplicate-call guard out of the way.
	client := newFakeClient(
		calls(toolCall("c1", "bad", map[string]string{"arg": "a"})),
		calls(toolCall("c2", "bad", map[string]string{"arg": "b"})),
		calls(toolCall("c3", "bad", map[string]string{"arg": "c"})),
		calls(toolCall("c4", "bad", map[string]string{"arg": "d"})),
		answer("giving up"),
	)
	bad := &scriptedTool{name: "bad", call: func(context.Context, json.RawMessage) (any, error) {
		return nil, tools.RuntimeError("no")
	}}
	runner, _ := newTestRunner(t, cfg, client, bad)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls", "tool:bad",
		"assistant+tool_calls", "tool:bad",
		"assistant+tool_calls", "tool:bad",
		"assistant+tool_calls", "tool:bad",
		"assistant",
	)

	reminders := 0
	for _, m := range res.FinalMessages {
		if strings.Contains(m.Content, "ERROR LOOP DETECTED") {
			if m.Role != llm.RoleTool {
				t.Errorf("loop reminder landed on a %s message, want it appended to a tool result", m.Role)
			}
			reminders++
		}
	}
	if reminders == 0 {
		t.Fatal("no loop-detection reminder was appended to any tool result")
	}
}

// TestProviderErrorRetry: a 200 response carrying an upstream error is retried
// once, and the retry's answer is what the history keeps.
func TestProviderErrorRetry(t *testing.T) {
	client := newFakeClient(
		providerError(502, "upstream is down"),
		answer("second time lucky"),
	)
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if client.callCount() != 2 {
		t.Errorf("made %d model calls, want 2 (original plus one retry)", client.callCount())
	}
	assertShape(t, res.FinalMessages, "user", "assistant")
	last := res.FinalMessages[len(res.FinalMessages)-1]
	if last.Content != "second time lucky" {
		t.Errorf("final answer is %q, want the retry's answer", last.Content)
	}
}

// TestProviderErrorGivesUp: two provider errors in a row stop the loop instead
// of retrying forever.
func TestProviderErrorGivesUp(t *testing.T) {
	client := newFakeClient(
		providerError(502, "down"),
		providerError(502, "still down"),
		providerError(502, "down again"),
		providerError(502, "down again"),
	)
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if client.callCount() > 4 {
		t.Errorf("made %d model calls, want at most 4 before giving up", client.callCount())
	}
	if len(res.FinalMessages) == 0 {
		t.Fatal("history is empty")
	}
}

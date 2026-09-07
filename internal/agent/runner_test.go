package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
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

// TestDuplicateCallDetection: under the weak profile, repeating a call verbatim
// earns an error result, and doing it maxConsecutiveDuplicates times stops the
// loop.
func TestDuplicateCallDetection(t *testing.T) {
	same := func(id string) scriptStep {
		return calls(toolCall(id, "echo", map[string]string{"arg": "same"}))
	}
	cfg := testConfig()
	cfg.Agent.Profile = "weak"
	client := newFakeClient(same("c1"), same("c2"), same("c3"), same("c4"), answer("unreachable"))
	runner, _ := newTestRunner(t, cfg, client, &scriptedTool{name: "echo"})

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

// TestSteeringReachesTheNextRequest: a line pushed into the inbox while the
// turn is running is in the very next request the loop makes, as a user
// message the model can tell apart from the prompt that started the turn.
func TestSteeringReachesTheNextRequest(t *testing.T) {
	box := inbox.New("")

	client := newFakeClient(
		// The first answer asks for a tool; the user types while it runs.
		scriptStep{
			resp: calls(toolCall("c1", "echo", map[string]string{"arg": "a"})).resp,
			hook: func() { box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "actually, stop"}) },
		},
		answer("stopping"),
	)
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})
	runner.inbox = box

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls", "tool:echo",
		"user", // the steering line
		"assistant",
	)
	steer := res.FinalMessages[3]
	if !strings.Contains(steer.Content, "<user-steering>") || !strings.Contains(steer.Content, "actually, stop") {
		t.Errorf("the steering message is %q, want it tagged and carrying the typed text", steer.Content)
	}
	// It has to be in the request, not just the history.
	second := client.requests[1].Messages
	if len(second) != 4 || second[3].Role != llm.RoleUser {
		t.Fatalf("the second request has %d messages, want the steering line at the end", len(second))
	}
	if !strings.Contains(second[3].Content, "actually, stop") {
		t.Errorf("the second request does not carry the steering line: %q", second[3].Content)
	}
}

// TestProcessEventRidesOnTheLastToolResult: an event that is not conversation
// is appended to the last tool result rather than added as a message, so the
// assistant/tool pairing the API requires stays intact.
func TestProcessEventRidesOnTheLastToolResult(t *testing.T) {
	box := inbox.New("")

	client := newFakeClient(
		scriptStep{
			resp: calls(toolCall("c1", "echo", map[string]string{"arg": "a"})).resp,
			hook: func() {
				box.Push(inbox.Message{Kind: inbox.KindProcessEvent, Text: "process 3 (npm test) exited with status 1"})
			},
		},
		answer("the tests failed"),
	)
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})
	runner.inbox = box

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls", "tool:echo",
		"assistant",
	)
	toolResult := res.FinalMessages[2]
	if !strings.Contains(toolResult.Content, "<system-reminder>") ||
		!strings.Contains(toolResult.Content, "exited with status 1") {
		t.Errorf("the process event was not appended to the tool result: %q", toolResult.Content)
	}
}

// TestSteeringSurvivesBacktrack: the drain happens before the rollback point
// is taken, so a backtrack in the same iteration cannot discard what someone
// just said.
func TestSteeringSurvivesBacktrack(t *testing.T) {
	cfg := testConfig()
	cfg.Backtrack.Enabled = true
	cfg.Backtrack.MaxRetries = 5

	box := inbox.New("")
	box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "look in internal/ instead"})

	client := newFakeClient(
		calls(toolCall("c1", "bad", map[string]string{"arg": "x"})),
		answer("found it"),
	)
	bad := &scriptedTool{name: "bad", call: func(context.Context, json.RawMessage) (any, error) {
		return nil, tools.SemanticErrorf("no such path")
	}}
	runner, _ := newTestRunner(t, cfg, client, bad)
	runner.inbox = box

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertShape(t, res.FinalMessages, "user", "user", "assistant")
	if !strings.Contains(res.FinalMessages[1].Content, "look in internal/ instead") {
		t.Errorf("the steering line was discarded by the backtrack: %q", res.FinalMessages[1].Content)
	}
}

// TestCancellationLeavesACompleteRecord: an interrupt stops the turn where it
// is and leaves a history that can be continued — every tool call the model
// made has a result, the one that was running says what it produced before it
// was cut short, and the ones that never ran say they were cancelled.
func TestCancellationLeavesACompleteRecord(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newFakeClient(
		calls(
			toolCall("c1", "slow", map[string]string{"arg": "a"}),
			toolCall("c2", "slow", map[string]string{"arg": "b"}),
		),
		answer("unreachable"),
	)
	// The first call is interrupted part-way and returns what it had.
	slow := &scriptedTool{name: "slow", call: func(context.Context, json.RawMessage) (any, error) {
		cancel()
		return map[string]any{"stdout": "half of the output"}, nil
	}}
	runner, _ := newTestRunner(t, testConfig(), client, slow)

	res, err := runner.Run(ctx, RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !res.Cancelled {
		t.Error("the run does not report that it was cancelled")
	}
	if res.BudgetExhausted {
		t.Error("a cancelled run reports the iteration budget as exhausted")
	}
	if client.callCount() != 1 {
		t.Errorf("made %d model calls, want 1: the loop kept going after the interrupt", client.callCount())
	}

	// Every tool call the model made has a result, which is what the API
	// requires of a history you want to continue from.
	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls",
		"tool:slow", // interrupted part-way
		"tool:slow", // never ran
	)

	interrupted := res.FinalMessages[2]
	if !strings.Contains(interrupted.Content, "interrupted") {
		t.Errorf("the interrupted tool's result is not marked as such: %q", interrupted.Content)
	}
	if !strings.Contains(interrupted.Content, "half of the output") {
		t.Errorf("the interrupted tool's partial output was lost: %q", interrupted.Content)
	}
	if interrupted.ToolCallID != "c1" {
		t.Errorf("the interrupted result is for call %q, want c1", interrupted.ToolCallID)
	}

	never := res.FinalMessages[3]
	if !strings.Contains(never.Content, "Cancelled by user") {
		t.Errorf("the tool call that never ran has no result saying so: %q", never.Content)
	}
	if never.ToolCallID != "c2" {
		t.Errorf("the cancelled result is for call %q, want c2", never.ToolCallID)
	}
}

// TestQuestionRunsLastInABatch: asking blocks the turn, so a batch that both
// starts work and asks about it must start the work first. Anything else is
// the one arrangement where asking holds up work that could have been running.
func TestQuestionRunsLastInABatch(t *testing.T) {
	var order []string

	client := newFakeClient(
		calls(
			toolCall("c1", "Question", map[string]any{
				"questions": []map[string]any{{"question": "which one?"}},
			}),
			toolCall("c2", "echo", map[string]string{"arg": "a"}),
			toolCall("c3", "echo", map[string]string{"arg": "b"}),
		),
		answer("done"),
	)
	record := func(name string) func(context.Context, json.RawMessage) (any, error) {
		return func(context.Context, json.RawMessage) (any, error) {
			order = append(order, name)
			return map[string]any{"ok": true}, nil
		}
	}
	runner, _ := newTestRunner(t, testConfig(), client,
		&scriptedTool{name: "Question", call: record("Question")},
		&scriptedTool{name: "echo", call: record("echo")},
	)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	want := []string{"echo", "echo", "Question"}
	if len(order) != len(want) {
		t.Fatalf("ran %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("ran %v, want %v", order, want)
		}
	}

	// Reordering execution must not reorder the results: every tool call still
	// gets its result, and each is tied to its own call ID.
	ids := map[string]bool{}
	for _, m := range res.FinalMessages {
		if m.Role == llm.RoleTool {
			ids[m.ToolCallID] = true
		}
	}
	for _, id := range []string{"c1", "c2", "c3"} {
		if !ids[id] {
			t.Errorf("tool call %s has no result", id)
		}
	}
}

// TestStrongProfileRunsRepeatedCalls: the duplicate-call kill switch is there
// for a model that loops. On one that does not, a legitimate repeat — polling
// the same status twice — must not end the turn.
func TestStrongProfileRunsRepeatedCalls(t *testing.T) {
	same := func(id string) scriptStep {
		return calls(toolCall(id, "echo", map[string]string{"arg": "same"}))
	}
	cfg := testConfig() // profile defaults to strong
	if !cfg.Agent.IsStrong() {
		t.Fatal("the default profile is not strong")
	}
	client := newFakeClient(same("c1"), same("c2"), same("c3"), same("c4"), answer("all four ran"))
	runner, _ := newTestRunner(t, cfg, client, &scriptedTool{name: "echo"})

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:echo",
		"assistant",
	)
	for _, m := range res.FinalMessages {
		if strings.Contains(m.Content, "DUPLICATE CALL ERROR") {
			t.Fatalf("the duplicate-call guard fired under the strong profile: %q", m.Content)
		}
	}
	if res.Cancelled {
		t.Error("the run was cancelled by the duplicate-call kill switch")
	}
}

// TestStrongProfileDoesNotScrapeProse: under the strong profile the model's
// prose is an answer, not a tool call waiting to be found in it.
func TestStrongProfileDoesNotScrapeProse(t *testing.T) {
	prose := `I would run Shell({"command": "rm -rf build"}) here, but let me explain first.`

	client := newFakeClient(answer(prose))
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "Shell"})

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertShape(t, res.FinalMessages, "user", "assistant")
	if last := res.FinalMessages[1]; len(last.ToolCalls) != 0 {
		t.Errorf("a sentence about a command was turned into %d tool calls", len(last.ToolCalls))
	}
	if client.callCount() != 1 {
		t.Errorf("made %d model calls, want 1: the answer was treated as a tool call", client.callCount())
	}
}

// TestHistoryIsWrittenPerIteration: a turn's work reaches the session as the
// turn goes, not only when it ends. Before this, a turn killed outright lost
// everything it had done, and a running turn could not be read at all.
func TestHistoryIsWrittenPerIteration(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "echo", map[string]string{"arg": "a"})),
		calls(toolCall("c2", "echo", map[string]string{"arg": "b"})),
		answer("done"),
	)
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})

	// Each flush is one batch, so the number of batches is the number of
	// moments the session was brought up to date.
	var batches [][]llm.Message
	runner.SetPersist(func(batch []llm.Message) error {
		batches = append(batches, batch)
		return nil
	}, nil)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Two iterations, each writing the model's turn and then its tool result,
	// plus the final answer: the session is brought up to date after every
	// message rather than once at the end.
	if len(batches) < 5 {
		t.Fatalf("the session was written %d time(s), want one write per message", len(batches))
	}
	for i, b := range batches {
		if len(b) != 1 {
			t.Errorf("write %d carried %d messages, want them written one at a time", i, len(b))
		}
	}

	// Everything the loop produced was written exactly once, in order, and the
	// prompt that was already in the session was not written again.
	var written []llm.Message
	for _, b := range batches {
		written = append(written, b...)
	}
	assertShape(t, written,
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:echo",
		"assistant",
	)
	if len(written) != len(res.FinalMessages)-1 {
		t.Errorf("wrote %d messages for a history of %d (excluding the prompt)",
			len(written), len(res.FinalMessages)-1)
	}
}

// TestBacktrackedMessagesAreNeverWritten: the reason the flush happens at the
// iteration boundary rather than per message. A discarded attempt must not
// reach an append-only file, which cannot take it back.
func TestBacktrackedMessagesAreNeverWritten(t *testing.T) {
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

	// The session applies a discard the way session.Load does: drop that many
	// messages from the end.
	var written []llm.Message
	var discards []int
	runner.SetPersist(
		func(batch []llm.Message) error {
			written = append(written, batch...)
			return nil
		},
		func(n int) error {
			discards = append(discards, n)
			written = written[:len(written)-min(n, len(written))]
			return nil
		},
	)

	if _, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The failed attempt was written as it happened and then taken back, so
	// what the session replays is the recovered answer alone.
	if len(discards) != 1 {
		t.Fatalf("the loop recorded %d discards, want 1 for the backtrack", len(discards))
	}
	assertShape(t, written, "assistant")
}

// TestPartialTurnIsWrittenWhenCancelled: the case that used to lose the lot.
func TestPartialTurnIsWrittenWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := newFakeClient(
		calls(toolCall("c1", "echo", map[string]string{"arg": "a"})),
		calls(toolCall("c2", "slow", map[string]string{"arg": "b"})),
		answer("unreachable"),
	)
	slow := &scriptedTool{name: "slow", call: func(context.Context, json.RawMessage) (any, error) {
		cancel()
		return map[string]any{"partial": "output"}, nil
	}}
	runner, _ := newTestRunner(t, testConfig(), client,
		&scriptedTool{name: "echo"}, slow)

	var written []llm.Message
	runner.SetPersist(func(batch []llm.Message) error {
		written = append(written, batch...)
		return nil
	}, nil)

	res, err := runner.Run(ctx, RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !res.Cancelled {
		t.Fatal("the run does not report being cancelled")
	}

	// The first iteration's work survives the interrupt, as does the record of
	// what the second one was doing when it was cut short.
	assertShape(t, written,
		"assistant+tool_calls", "tool:echo",
		"assistant+tool_calls", "tool:slow",
	)
}

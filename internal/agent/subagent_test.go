package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/stats"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

func TestRunSubagentReturnsSummary(t *testing.T) {
	client := newFakeClient(answer("auth lives in internal/auth:1"))
	parent, _ := newTestRunner(t, testConfig(), client)
	childReg := tools.NewRegistry()
	childCtx := tools.NewToolContext()
	res, err := parent.RunSubagent(context.Background(), tools.SubagentRunParams{
		System:   tools.BuildSubagentSystemPrompt(".", "research"),
		Prompt:   "How does login work?",
		MaxIters: 5,
		Registry: childReg,
		ToolCtx:  childCtx,
	})
	if err != nil {
		t.Fatalf("RunSubagent: %v", err)
	}
	if res.Text != "auth lives in internal/auth:1" {
		t.Errorf("Text %q, want summary", res.Text)
	}
	if res.BudgetExhausted || res.TimedOut {
		t.Errorf("flags %+v, want none", res)
	}
}

func TestRunSubagentBudgetExhaustedIsLabeled(t *testing.T) {
	childReg := tools.NewRegistry()
	childReg.Enable(&scriptedTool{name: "Read"})
	steps := []scriptStep{
		calls(toolCall("c1", "Read", map[string]any{"arg": "x"})),
		calls(toolCall("c2", "Read", map[string]any{"arg": "x"})),
		calls(toolCall("c3", "Read", map[string]any{"arg": "x"})),
	}
	client := newFakeClient(steps...)
	parent, _ := newTestRunner(t, testConfig(), client)
	res, err := parent.RunSubagent(context.Background(), tools.SubagentRunParams{
		System:   "sys",
		Prompt:   "keep going",
		MaxIters: 2,
		Registry: childReg,
		ToolCtx:  tools.NewToolContext(),
	})
	if err != nil {
		t.Fatalf("RunSubagent: %v", err)
	}
	if !res.BudgetExhausted {
		t.Error("BudgetExhausted is false after the child ran out of iterations")
	}
}

func TestRunSubagentUsesChildRegistry(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "Edit", map[string]any{"arg": "x"})),
		answer("could not edit: read-only child"),
	)
	parent, _ := newTestRunner(t, testConfig(), client)
	childReg := tools.NewRegistry()
	res, err := parent.RunSubagent(context.Background(), tools.SubagentRunParams{
		System:   "sys",
		Prompt:   "edit it",
		MaxIters: 5,
		Registry: childReg,
		ToolCtx:  tools.NewToolContext(),
	})
	if err != nil {
		t.Fatalf("RunSubagent: %v", err)
	}
	if res.Text != "could not edit: read-only child" {
		t.Errorf("Text %q, want child summary", res.Text)
	}
	if len(client.requests) == 0 {
		t.Fatal("child made no requests")
	}
	if len(client.requests[0].Tools) != 0 {
		t.Errorf("child offered %d tools, want 0 (filtered)", len(client.requests[0].Tools))
	}
}

func TestFoldSubagentStats(t *testing.T) {
	parent, _ := newTestRunner(t, testConfig(), newFakeClient())
	state := parent.initRunState(RunConfig{Messages: []llm.Message{}})
	child := &stubStatsCarrier{prompt: 10, completion: 20, cost: 0.5, steps: 3}
	parent.foldSubagentStats(state, child)
	if state.agentStats.TotalPromptTokens != 10 || state.agentStats.TotalCompletionTokens != 20 {
		t.Errorf("tokens %+v, want 10/20", state.agentStats)
	}
	if state.agentStats.TotalCost != 0.5 || state.agentStats.Steps != 3 {
		t.Errorf("cost/steps %+v, want 0.5/3", state.agentStats)
	}
	var _ = llm.RoleAssistant
}

type stubStatsCarrier struct {
	prompt, completion int
	cost               float64
	steps              int
}

func (s *stubStatsCarrier) ChildStats() *stats.AgentStats {
	return &stats.AgentStats{TotalPromptTokens: s.prompt, TotalCompletionTokens: s.completion, TotalCost: s.cost, Steps: s.steps}
}

func TestShallowSubagentConfigFloors(t *testing.T) {
	cfg := testConfig()
	cfg.Agent.MaxIterations = 1000
	cfg.Tools.Edit.ReadBeforeEditMsgs = 0
	cfg.Tools.Edit.PreviewMode = true
	child := shallowSubagentConfig(cfg, 7)
	if child.Agent.MaxIterations != 7 {
		t.Errorf("MaxIterations %d, want 7", child.Agent.MaxIterations)
	}
	if child.Tools.Edit.ReadBeforeEditMsgs != 7 {
		t.Errorf("ReadBeforeEditMsgs %d, want floored to 7", child.Tools.Edit.ReadBeforeEditMsgs)
	}
	if child.Tools.Edit.PreviewMode {
		t.Error("PreviewMode still on in child config")
	}
	if cfg.Tools.Edit.ReadBeforeEditMsgs != 0 || !cfg.Tools.Edit.PreviewMode {
		t.Error("parent config mutated")
	}
}

func TestStructuredOutputInlineErrorFixedInRun(t *testing.T) {
	schema := `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`
	capture := &tools.StructuredOutputCapture{}
	so, err := tools.NewStructuredOutputTool(json.RawMessage(schema), capture)
	if err != nil {
		t.Fatalf("NewStructuredOutputTool: %v", err)
	}
	childReg := tools.NewRegistry()
	childReg.Enable(so)
	// The first attempt omits the required property (the runner's
	// argument normalizer coerces scalar mismatches like 123 to "123",
	// so only a missing property deterministically fails validation).
	client := newFakeClient(
		calls(toolCall("c1", "structured_output", map[string]any{})),
		calls(toolCall("c2", "structured_output", map[string]any{"name": "auth"})),
		answer("filed"),
	)
	parent, _ := newTestRunner(t, testConfig(), client)
	sess, res, err := parent.StartSubagent(context.Background(), tools.SubagentRunParams{
		System:   "sys",
		Prompt:   "file the result",
		MaxIters: 10,
		Registry: childReg,
		ToolCtx:  tools.NewToolContext(),
	})
	if err != nil {
		t.Fatalf("StartSubagent: %v", err)
	}
	_ = sess
	called, value, _ := capture.Snapshot()
	if !called || string(value) != `{"name":"auth"}` {
		t.Errorf("captured %q, want the fixed value", value)
	}
	// The rejection reached the child as an inline tool error it could
	// fix in the same run: the follow-up request carries the tool result.
	var sawRejection bool
	for _, req := range client.requests {
		for _, m := range req.Messages {
			if m.Role == llm.RoleTool && m.Name == "structured_output" &&
				strings.Contains(m.Content, "does not match the required schema") {
				sawRejection = true
			}
		}
	}
	if !sawRejection {
		t.Error("child transcript has no inline validation error to fix")
	}
	if res.Text != "filed" {
		t.Errorf("Text %q, want the final answer", res.Text)
	}
}

func TestContinueSubagentPersistsHistory(t *testing.T) {
	client := newFakeClient(answer("first"), answer("second"))
	parent, _ := newTestRunner(t, testConfig(), client)
	sess, first, err := parent.StartSubagent(context.Background(), tools.SubagentRunParams{
		System:   "sys",
		Prompt:   "go",
		MaxIters: 5,
		Registry: tools.NewRegistry(),
		ToolCtx:  tools.NewToolContext(),
	})
	if err != nil {
		t.Fatalf("StartSubagent: %v", err)
	}
	if first.Text != "first" {
		t.Fatalf("first Text %q", first.Text)
	}
	second, err := parent.ContinueSubagent(context.Background(), sess, "and then?")
	if err != nil {
		t.Fatalf("ContinueSubagent: %v", err)
	}
	if second.Text != "second" {
		t.Errorf("second Text %q", second.Text)
	}
	// The continuation re-entered the same history: the follow-up request
	// carries the first answer.
	if len(client.requests) != 2 {
		t.Fatalf("got %d requests, want 2", len(client.requests))
	}
	joined := ""
	for _, m := range client.requests[1].Messages {
		joined += m.Content + "\n"
	}
	if !strings.Contains(joined, "first") || !strings.Contains(joined, "and then?") {
		t.Errorf("follow-up request lacks history: %q", joined)
	}
	// A session that is not ours fails closed, never a bare panic.
	if _, err := parent.ContinueSubagent(context.Background(), tools.NewSubagentSession(nil), "x"); err == nil {
		t.Error("ContinueSubagent(nil) = nil, want error")
	}
	if _, err := parent.ContinueSubagent(context.Background(), nil, "x"); err == nil {
		t.Error("ContinueSubagent(nil session) = nil, want error")
	}
}

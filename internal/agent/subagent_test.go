package agent

import (
	"context"
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

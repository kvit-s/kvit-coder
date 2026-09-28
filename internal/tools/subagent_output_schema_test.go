package tools

// Phase 2b tests (spec/subagents-phase2.md sections 6-7): output_schema
// validation, the session-local structured_output tool, ContinueSubagent
// nudges, and labeled failures. Each test asserts one thing.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

const schemaPerson = `{"type":"object","required":["name"],"properties":{"name":{"type":"string"}}}`

func TestSubagentCompileErrorBeforeSpawn(t *testing.T) {
	stub := &stubSubRunner{}
	parentCtx := NewToolContext()
	parentReg := NewRegistry()
	sub, _ := newSubagentUnderTest(t, &config.Config{}, parentCtx, parentReg, stub)
	_, err := sub.Call(context.Background(), json.RawMessage(
		`{"description":"find auth","prompt":"how does login work?","output_schema":{"type":"bogus"}}`))
	if err == nil {
		t.Fatal("Call with uncompilable schema succeeded, want SemanticError")
	}
	if !IsBacktrackable(err) {
		t.Errorf("compile error %v is not backtrackable (semantic)", err)
	}
	stub.mu.Lock()
	defer stub.mu.Unlock()
	if stub.starts != 0 {
		t.Errorf("spawned %d child runs despite compile failure, want 0", stub.starts)
	}
}

func TestStructuredOutputInlineErrorsCappedAtThree(t *testing.T) {
	schema := `{"type":"object","properties":` +
		`{"a":{"type":"string"},"b":{"type":"string"},"c":{"type":"string"},"d":{"type":"string"},"e":{"type":"string"}}}`
	capture := &StructuredOutputCapture{}
	tool, err := NewStructuredOutputTool(json.RawMessage(schema), capture)
	if err != nil {
		t.Fatalf("NewStructuredOutputTool: %v", err)
	}
	// Five failing properties: every leaf must be recorded, but only the
	// first three reach the child.
	bad := json.RawMessage(`{"a":1,"b":2,"c":3,"d":4,"e":5}`)
	_, err = tool.Call(context.Background(), bad)
	if err == nil {
		t.Fatal("invalid value accepted")
	}
	if !IsBacktrackable(err) {
		t.Errorf("validation error %v is not backtrackable (semantic)", err)
	}
	if n := strings.Count(err.Error(), "\n- "); n != maxSchemaErrors {
		t.Errorf("child sees %d error bullets, want %d", n, maxSchemaErrors)
	}
	if called, _, _ := capture.Snapshot(); called {
		t.Error("failed value marked the capture called")
	}
	if _, _, last := capture.Snapshot(); len(last) != 5 {
		t.Errorf("capture keeps %d errors, want all 5 for the nudge prompts", len(last))
	}
	// A valid value is captured as canonical JSON.
	if _, err := tool.Call(context.Background(), json.RawMessage(`{"e":"x","a":"y"}`)); err != nil {
		t.Fatalf("valid value rejected: %v", err)
	}
	if called, value, _ := capture.Snapshot(); !called || string(value) != `{"a":"y","e":"x"}` {
		t.Errorf("captured %q, want canonical compact JSON", value)
	}
}

func TestSubagentSchemaMissingCallNudgesTwice(t *testing.T) {
	var missingTool bool
	stub := &stubSubRunner{
		startFn: func(p SubagentRunParams) (*SubagentSession, SubagentRunResult, error) {
			if p.Registry.Get(StructuredOutputToolName) == nil {
				missingTool = true
			}
			return NewSubagentSession(nil),
				SubagentRunResult{Text: "partial prose", Model: "m", Iterations: 2, PromptTokens: 10}, nil
		},
		contFn: func(_ *SubagentSession, _ string) (SubagentRunResult, error) {
			return SubagentRunResult{Text: "more prose", Model: "m", Iterations: 1, PromptTokens: 5}, nil
		},
	}
	parentCtx := NewToolContext()
	parentReg := NewRegistry()
	sub, _ := newSubagentUnderTest(t, &config.Config{}, parentCtx, parentReg, stub)
	res, err := sub.Call(context.Background(), json.RawMessage(
		`{"description":"find auth","prompt":"how does login work?","output_schema":`+schemaPerson+`}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if missingTool {
		t.Fatal("structured_output missing from the child handle")
	}
	stub.mu.Lock()
	continues := stub.continues
	nudges := append([]string(nil), stub.nudges...)
	stub.mu.Unlock()
	if continues != maxNudges {
		t.Errorf("ran %d continuations, want exactly %d", continues, maxNudges)
	}
	for _, n := range nudges {
		if !strings.Contains(n, StructuredOutputToolName) {
			t.Errorf("nudge does not demand the call: %q", n)
		}
	}
	got := res.(*SubagentCallResult)
	if !strings.Contains(got.Result, "[schema_unsatisfied:") {
		t.Errorf("result %q lacks the schema_unsatisfied label", got.Result)
	}
	if !strings.Contains(got.Result, "more prose") {
		t.Errorf("result %q lost the partial summary", got.Result)
	}
	// Usage adds up across the initial run and both nudges.
	if got.Iterations != 4 || got.PromptTokens != 20 {
		t.Errorf("iterations=%d tokens=%d, want 4 and 20", got.Iterations, got.PromptTokens)
	}
}

func TestStructuredOutputFiledThroughChildBatch(t *testing.T) {
	schema := schemaPerson
	stub := &stubSubRunner{
		startFn: func(p SubagentRunParams) (*SubagentSession, SubagentRunResult, error) {
			batch := p.Registry.Get("Batch")
			if batch == nil {
				return NewSubagentSession(nil), SubagentRunResult{}, fmt.Errorf("no Batch in child handle")
			}
			out, err := batch.Call(context.Background(), mustJSON(map[string]any{
				"calls": []any{
					map[string]any{"tool": StructuredOutputToolName, "args": map[string]any{"name": "auth"}},
				},
			}))
			if err != nil {
				return NewSubagentSession(nil), SubagentRunResult{}, err
			}
			m, ok := out.(map[string]any)
			if !ok {
				return NewSubagentSession(nil), SubagentRunResult{}, fmt.Errorf("bad batch result type %T", out)
			}
			results, ok := m["results"].([]BatchResult)
			if !ok || len(results) != 1 || !results[0].OK {
				return NewSubagentSession(nil), SubagentRunResult{}, fmt.Errorf("batch did not accept the filed value: %+v", m)
			}
			return NewSubagentSession(nil), SubagentRunResult{Text: "filed via batch", Model: "m"}, nil
		},
	}
	cfg := &config.Config{}
	cfg.Tools.Batch.Enabled = true
	parentCtx := NewToolContext()
	parentReg := NewRegistry()
	parentReg.Enable(&stubNamedTool{name: "Batch"})
	sub, _ := newSubagentUnderTest(t, cfg, parentCtx, parentReg, stub)
	res, err := sub.Call(context.Background(), json.RawMessage(
		`{"description":"find auth","prompt":"how does login work?","output_schema":`+schema+`}`))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	got := res.(*SubagentCallResult)
	if strings.Contains(got.Result, "[schema_unsatisfied:") {
		t.Errorf("batch-filed value not captured: %q", got.Result)
	}
	if !strings.Contains(got.Result, `"name":"auth"`) {
		t.Errorf("result %q is not the canonical filed JSON", got.Result)
	}
}

func TestStructuredOutputAbsentWithoutSchema(t *testing.T) {
	parentCtx := NewToolContext()
	parentReg := NewRegistry()
	if parentReg.Get(StructuredOutputToolName) != nil {
		t.Fatal("parent handle carries structured_output")
	}
	for _, typ := range []string{"research", "general"} {
		child, _ := TestChildScope(parentReg, parentCtx, &config.Config{}, 5, typ)
		if child.Get(StructuredOutputToolName) != nil {
			t.Errorf("schema-less %s handle carries structured_output", typ)
		}
	}
}

func TestSubagentJSONResultTruncation(t *testing.T) {
	big := `{"data":"` + strings.Repeat("x", 100*1024) + `"}`
	stub := &stubSubRunner{fn: func(p SubagentRunParams) (SubagentRunResult, error) {
		return SubagentRunResult{Text: big, Model: "m", Iterations: 3}, nil
	}}
	parentCtx := NewToolContext()
	parentReg := NewRegistry()
	sub, _ := newSubagentUnderTest(t, &config.Config{}, parentCtx, parentReg, stub)
	res, err := sub.Call(context.Background(), mustJSON(map[string]any{
		"description": "big json",
		"prompt":      "return it all",
	}))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	got := res.(*SubagentCallResult)
	if !got.Truncated || got.OverflowPath == "" {
		t.Fatal("100KB JSON answer was not spilled to tmp/")
	}
}

func TestSubagentEmptySummaryNudge(t *testing.T) {
	// A silent child that answers the nudge: one continuation, then text.
	stub := &stubSubRunner{
		startFn: func(SubagentRunParams) (*SubagentSession, SubagentRunResult, error) {
			return NewSubagentSession(nil), SubagentRunResult{Text: "", Model: "m"}, nil
		},
		contFn: func(_ *SubagentSession, prompt string) (SubagentRunResult, error) {
			if !strings.Contains(prompt, "concise summary") {
				return SubagentRunResult{}, fmt.Errorf("nudge does not ask for a summary: %q", prompt)
			}
			return SubagentRunResult{Text: "late summary", Model: "m"}, nil
		},
	}
	parentCtx := NewToolContext()
	sub, _ := newSubagentUnderTest(t, &config.Config{}, parentCtx, NewRegistry(), stub)
	res, err := sub.Call(context.Background(), mustJSON(map[string]any{
		"description": "quiet task",
		"prompt":      "look around",
	}))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got := res.(*SubagentCallResult).Result; got != "late summary" {
		t.Errorf("result %q, want the nudged summary", got)
	}
	stub.mu.Lock()
	continues := stub.continues
	stub.mu.Unlock()
	if continues != 1 {
		t.Errorf("ran %d continuations, want 1", continues)
	}

	// A child that stays silent: exactly two continuations, then the label.
	stub2 := &stubSubRunner{
		startFn: func(SubagentRunParams) (*SubagentSession, SubagentRunResult, error) {
			return NewSubagentSession(nil), SubagentRunResult{Text: "  ", Model: "m"}, nil
		},
	}
	sub2, _ := newSubagentUnderTest(t, &config.Config{}, NewToolContext(), NewRegistry(), stub2)
	res2, err := sub2.Call(context.Background(), mustJSON(map[string]any{
		"description": "quiet task",
		"prompt":      "look around",
	}))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if got := res2.(*SubagentCallResult).Result; !strings.Contains(got, "[summary_missing:") {
		t.Errorf("result %q lacks the summary_missing label", got)
	}
	stub2.mu.Lock()
	defer stub2.mu.Unlock()
	if stub2.continues != maxNudges {
		t.Errorf("ran %d continuations, want exactly %d", stub2.continues, maxNudges)
	}
}

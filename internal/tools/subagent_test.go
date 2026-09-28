package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/permissions"
)

// stubSubRunner is a scripted child loop for tests: it records calls in order
// and answers from a function, without touching the network.
type stubSubRunner struct {
	mu        sync.Mutex
	calls     []SubagentRunParams
	fn        func(SubagentRunParams) (SubagentRunResult, error)
	startFn   func(SubagentRunParams) (*SubagentSession, SubagentRunResult, error)
	contFn    func(*SubagentSession, string) (SubagentRunResult, error)
	starts    int
	continues int
	nudges    []string
}

func (s *stubSubRunner) RunSubagent(_ context.Context, p SubagentRunParams) (SubagentRunResult, error) {
	s.mu.Lock()
	s.calls = append(s.calls, p)
	s.mu.Unlock()
	if s.fn != nil {
		return s.fn(p)
	}
	return SubagentRunResult{Text: "stub summary", Model: "test"}, nil
}

func (s *stubSubRunner) StartSubagent(_ context.Context, p SubagentRunParams) (*SubagentSession, SubagentRunResult, error) {
	s.mu.Lock()
	s.starts++
	s.calls = append(s.calls, p)
	s.mu.Unlock()
	if s.startFn != nil {
		return s.startFn(p)
	}
	if s.fn != nil {
		res, err := s.fn(p)
		return NewSubagentSession(nil), res, err
	}
	return NewSubagentSession(nil), SubagentRunResult{Text: "stub summary", Model: "test"}, nil
}

func (s *stubSubRunner) ContinueSubagent(_ context.Context, sess *SubagentSession, prompt string) (SubagentRunResult, error) {
	s.mu.Lock()
	s.continues++
	s.nudges = append(s.nudges, prompt)
	s.mu.Unlock()
	if s.contFn != nil {
		return s.contFn(sess, prompt)
	}
	return SubagentRunResult{Text: "", Model: "test"}, nil
}

func (s *stubSubRunner) orderedDescriptions() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]string, 0, len(s.calls))
	for _, c := range s.calls {
		out = append(out, c.Description)
	}
	return out
}

func testParentRegistry(t *testing.T, cfg *config.Config, toolCtx *ToolContext, tempMgr *TempFileManager) *Registry {
	t.Helper()
	reg := NewRegistry()
	// Minimal read tools as stubs sharing names with the real ones.
	reg.Enable(&stubNamedTool{name: "Read"})
	reg.Enable(&stubNamedTool{name: "Search"})
	reg.Enable(&stubNamedTool{name: "Glob"})
	reg.Enable(&stubNamedTool{name: "Web.search"})
	reg.Enable(&stubNamedTool{name: "Web.fetch"})
	reg.Enable(&stubNamedTool{name: "Edit"})
	reg.Enable(&stubNamedTool{name: "Write"})
	reg.Enable(&stubNamedTool{name: "Question"})
	reg.Enable(&stubNamedTool{name: "Report"})
	reg.Enable(&stubNamedTool{name: "Subagent"})
	reg.Enable(&stubNamedTool{name: "Edit.confirm"})
	reg.Enable(&stubNamedTool{name: "Edit.cancel"})
	reg.Enable(&stubNamedTool{name: "Write.confirm"})
	reg.Enable(&stubNamedTool{name: "Write.cancel"})
	reg.Enable(&stubNamedTool{name: "Shell.start"})
	reg.Enable(&stubNamedTool{name: "Shell.output"})
	reg.Enable(&stubNamedTool{name: "Shell.status"})
	reg.Enable(&stubNamedTool{name: "Shell.list"})
	reg.Enable(&stubNamedTool{name: "Shell.kill"})
	reg.Enable(&stubNamedTool{name: "Shell.tune"})
	reg.Enable(&stubNamedTool{name: "Observe.wait"})
	reg.Enable(&stubNamedTool{name: "Observe.add"})
	reg.Enable(&stubNamedTool{name: "mcp.github.get_issue"})
	// Real shell tools so ReadOnlyCall and permission paths are exercised.
	shellTimeout := secondsToDuration(120)
	shellTool := NewShellTool(cfg, shellTimeout, tempMgr)
	shellTool.SetToolContext(toolCtx)
	reg.Enable(shellTool)
	adv := NewShellAdvancedTool(cfg, shellTimeout, tempMgr)
	adv.SetToolContext(toolCtx)
	// The simple name wins in Get; keep both reachable by internal name.
	reg.Enable(adv)
	batch := NewBatchTool(cfg)
	batch.SetRegistry(reg)
	batch.SetToolContext(toolCtx)
	reg.Enable(batch)
	return reg
}

type stubNamedTool struct {
	name string
}

func (s *stubNamedTool) Name() string        { return s.name }
func (s *stubNamedTool) Description() string { return "stub " + s.name }
func (s *stubNamedTool) JSONSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (s *stubNamedTool) Check(_ context.Context, _ json.RawMessage) error { return nil }
func (s *stubNamedTool) Call(_ context.Context, _ json.RawMessage) (any, error) {
	return map[string]any{"ok": true}, nil
}
func (s *stubNamedTool) PromptSection() string      { return "" }
func (s *stubNamedTool) PromptCategory() string     { return "filesystem" }
func (s *stubNamedTool) PromptOrder() int           { return 1 }
func (s *stubNamedTool) PromptTemplateName() string { return "" }

func newSubagentUnderTest(t *testing.T, cfg *config.Config, parentCtx *ToolContext, parentReg *Registry, runner SubRunner) (*SubagentTool, *TempFileManager) {
	t.Helper()
	tempMgr := NewTempFileManager(t.TempDir())
	sub := NewSubagentTool(cfg, parentCtx, tempMgr)
	sub.SetRegistry(parentReg)
	sub.SetRunner(runner)
	return sub, tempMgr
}

func TestSubagentSchema(t *testing.T) {
	cfg := &config.Config{}
	sub := NewSubagentTool(cfg, NewToolContext(), nil)
	ctx := context.Background()
	for _, args := range []string{
		`{"prompt": "do it"}`,
		`{"description": "find auth"}`,
		`{}`,
	} {
		if err := sub.Check(ctx, json.RawMessage(args)); err == nil {
			t.Errorf("Check(%s) = nil, want missing-field error", args)
		}
	}
	if err := sub.Check(ctx, json.RawMessage(`{"description":"x","prompt":"y","subagent_type":"bogus"}`)); err == nil {
		t.Error("unknown subagent_type accepted, want rejection")
	}
	if err := sub.Check(ctx, json.RawMessage(`{"description":"x","prompt":"y","subagent_type":"general"}`)); err != nil {
		t.Errorf("general rejected: %v (phase 2a enables write-capable delegation)", err)
	}
	for _, bad := range []string{
		`{"description":"x","prompt":"y","output_schema":[]}`,
		`{"description":"x","prompt":"y","output_schema":"object"}`,
		`{"description":"x","prompt":"y","output_schema":42}`,
		`{"description":"x","prompt":"y","output_schema":true}`,
	} {
		if err := sub.Check(ctx, json.RawMessage(bad)); err == nil {
			t.Errorf("Check(%s) = nil, want schema-must-be-object error", bad)
		} else if !IsBacktrackable(err) {
			t.Errorf("Check(%s) error %v is not backtrackable (semantic)", bad, err)
		}
	}
	if err := sub.Check(ctx, json.RawMessage(`{"description":"x","prompt":"y","output_schema":{"type":"object"}}`)); err != nil {
		t.Errorf("valid object schema rejected: %v", err)
	}
	for _, absent := range []string{
		`{"description":"x","prompt":"y"}`,
		`{"description":"x","prompt":"y","output_schema":null}`,
	} {
		if err := sub.Check(ctx, json.RawMessage(absent)); err != nil {
			t.Errorf("Check(%s) = %v, want nil", absent, err)
		}
	}
}

func TestSubagentRegistryFilter(t *testing.T) {
	parentCtx := NewToolContext()
	tempMgr := NewTempFileManager(t.TempDir())
	pcfg := &config.Config{}
	pcfg.Workspace.Root = "."
	pcfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)
	parentReg := testParentRegistry(t, pcfg, parentCtx, tempMgr)

	child, childCtx := ResearchChildScope(parentReg, parentCtx)
	if childCtx == parentCtx {
		t.Error("child and parent contexts are the same object")
	}
	if childCtx == nil || !childCtx.IsSubagentChild() {
		t.Error("child context is not marked as a subagent child")
	}
	for _, absent := range []string{
		"Edit", "Write", "Subagent", "Question", "Report",
		"Edit.confirm", "Edit.cancel", "Write.confirm", "Write.cancel",
		"Shell.start", "Shell.output", "Shell.status", "Shell.list", "Shell.kill", "Shell.tune",
		"Observe.wait", "Observe.add", "mcp.github.get_issue",
	} {
		if child.Get(absent) != nil {
			t.Errorf("research handle has excluded tool %q", absent)
		}
	}
	for _, present := range []string{"Read", "Search", "Glob", "Web.search", "Web.fetch", "Shell", "Shell.advanced", "Batch"} {
		if child.Get(present) == nil {
			t.Errorf("research handle is missing %q", present)
		}
	}
	// No nesting through Batch: the child Batch dispatches through the same
	// filtered handle.
	batchTool := child.Get("Batch")
	if batchTool == nil {
		t.Fatal("research handle is missing Batch")
	}
	bt, ok := batchTool.(*BatchTool)
	if !ok {
		t.Fatalf("child Batch type %T", batchTool)
	}
	// A Batch call naming an excluded tool fails for that call alone.
	out, err := bt.Call(context.Background(), mustJSON(map[string]any{
		"calls": []any{
			map[string]any{"tool": "Read", "args": map[string]any{"path": "nope.go"}},
			map[string]any{"tool": "Edit", "args": map[string]any{}},
		},
	}))
	if err != nil {
		t.Fatalf("child Batch Call: %v", err)
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatalf("batch result type %T", out)
	}
	results, ok := m["results"].([]BatchResult)
	if !ok {
		t.Fatalf("batch results type %T", m["results"])
	}
	if len(results) != 2 {
		t.Fatalf("batch results len %d, want 2", len(results))
	}
	if results[1].OK {
		t.Error("excluded Edit smuggled through child Batch succeeded")
	}
}

func TestSubagentShellGate(t *testing.T) {
	pcfg := &config.Config{}
	pcfg.Workspace.Root = "."
	pcfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)
	parentCtx := NewToolContext()
	tempMgr := NewTempFileManager(t.TempDir())
	parentReg := testParentRegistry(t, pcfg, parentCtx, tempMgr)
	child, _ := ResearchChildScope(parentReg, parentCtx)
	ctx := context.Background()

	for _, cmd := range []string{"ls", "git status", "rg foo"} {
		tool := child.Get("Shell")
		if tool == nil {
			t.Fatal("child has no Shell")
		}
		args := mustJSON(map[string]any{"command": cmd})
		rc, ok := tool.(ReadOnlyCaller)
		if !ok {
			t.Fatal("child Shell does not implement ReadOnlyCaller")
		}
		if !rc.ReadOnlyCall(args) {
			t.Errorf("%q should pass ReadOnlyCall", cmd)
		}
	}
	dangerous := mustJSON(map[string]any{"command": "rm -rf ."})
	tool := child.Get("Shell")
	if err := tool.Check(ctx, dangerous); err == nil {
		t.Error("rm -rf . passed child Check, want fail-closed semantic error")
	} else if !IsBacktrackable(err) {
		t.Errorf("rm -rf . error %v is not a semantic (backtrackable) error", err)
	}
	if _, err := tool.Call(ctx, dangerous); err == nil {
		t.Error("rm -rf . Call succeeded, want fail-closed error")
	}
}

func TestSubagentTruncation(t *testing.T) {
	big := strings.Repeat("x", 100*1024)
	stub := &stubSubRunner{fn: func(p SubagentRunParams) (SubagentRunResult, error) {
		return SubagentRunResult{Text: big, Model: "m", Iterations: 3}, nil
	}}
	cfg := &config.Config{}
	parentCtx := NewToolContext()
	parentReg := NewRegistry()
	sub, _ := newSubagentUnderTest(t, cfg, parentCtx, parentReg, stub)
	res, err := sub.Call(context.Background(), mustJSON(map[string]any{
		"description": "big read",
		"prompt":      "summarize everything",
	}))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	got := res.(*SubagentCallResult)
	if !got.Truncated {
		t.Fatal("100KB answer was not truncated")
	}
	maxChars := cfg.Tools.Subagent.ResolvedResultMaxChars()
	if len(got.Result) < maxChars || len(got.Result) > maxChars+2000 {
		t.Errorf("head len %d, want ~%d plus truncation note", len(got.Result), maxChars)
	}
	if got.OverflowPath == "" {
		t.Fatal("no overflow path")
	}
	full, err := os.ReadFile(got.OverflowPath)
	if err != nil {
		t.Fatalf("overflow path unreadable: %v", err)
	}
	if string(full) != big {
		t.Errorf("overflow holds %d chars, want %d", len(full), len(big))
	}
}

func TestSubagentChangeReporting(t *testing.T) {
	cfg := &config.Config{}
	// Scripted child that edits: against a general-style handle the edit
	// flips the child ChangedThisTurn, which the parent observes exactly once.
	parentCtx := NewToolContext()
	var capturedChild *ToolContext
	stub := &stubSubRunner{fn: func(p SubagentRunParams) (SubagentRunResult, error) {
		capturedChild = p.ToolCtx
		p.ToolCtx.NoteChange()
		return SubagentRunResult{Text: "edited", Model: "m"}, nil
	}}
	parentReg := NewRegistry()
	sub, _ := newSubagentUnderTest(t, cfg, parentCtx, parentReg, stub)
	if _, err := sub.Call(context.Background(), mustJSON(map[string]any{
		"description": "make edit",
		"prompt":      "change the file",
	})); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if capturedChild == nil || capturedChild == parentCtx {
		t.Fatal("child and parent contexts are not different objects")
	}
	if !parentCtx.ChangedThisTurn() {
		t.Error("parent ChangedThisTurn is false after a child edit")
	}
	// A read-only child leaves the parent clear.
	parentCtx2 := NewToolContext()
	quiet := &stubSubRunner{fn: func(p SubagentRunParams) (SubagentRunResult, error) {
		return SubagentRunResult{Text: "read only", Model: "m"}, nil
	}}
	sub2, _ := newSubagentUnderTest(t, cfg, parentCtx2, NewRegistry(), quiet)
	if _, err := sub2.Call(context.Background(), mustJSON(map[string]any{
		"description": "read stuff",
		"prompt":      "summarize",
	})); err != nil {
		t.Fatalf("Call: %v", err)
	}
	if parentCtx2.ChangedThisTurn() {
		t.Error("read-only child flipped parent ChangedThisTurn")
	}
}

func TestSubagentPermissionsDeny(t *testing.T) {
	pcfg := &config.Config{}
	pcfg.Workspace.Root = "."
	pcfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)
	grantor := permissions.NewGrantor(nil, nil, nil)
	parentCtx := NewToolContext()
	parentCtx.SetGrantor(grantor)
	// No terminal: no inbox, not interactive.
	tempMgr := NewTempFileManager(t.TempDir())
	shellTimeout := secondsToDuration(120)
	real := NewShellAdvancedTool(pcfg, shellTimeout, tempMgr)
	real.SetToolContext(parentCtx)
	parentReg := NewRegistry()
	parentReg.Enable(real)

	child, childCtx := ResearchChildScope(parentReg, parentCtx)
	if childCtx == nil || !childCtx.IsSubagentChild() {
		t.Fatal("child context is not marked")
	}
	// Permission prompting resolves to deny inside the child. Exercise the
	// inner shell clone directly: the research gate fails non-read-only
	// commands closed first (tested above), while a command reaching the
	// permission layer denies with the subagent message rather than
	// prompting.
	gate := child.Get("Shell.advanced")
	if gate == nil {
		t.Fatal("child has no shell")
	}
	rg, ok := gate.(*readOnlyShellGate)
	if !ok {
		t.Fatalf("child shell type %T, want *readOnlyShellGate", gate)
	}
	inner, ok := rg.inner.(*ShellAdvancedTool)
	if !ok {
		t.Fatalf("inner shell type %T, want *ShellAdvancedTool", rg.inner)
	}
	err := inner.Check(context.Background(), mustJSON(map[string]any{"command": "curl https://example.com"}))
	if err == nil {
		t.Fatal("curl passed child Check, want deny with no terminal")
	}
	if !strings.Contains(strings.ToLower(err.Error()), subagentPromptDenyHint) {
		t.Errorf("child deny %q lacks %q", err.Error(), subagentPromptDenyHint)
	}

	// Path checks go through NormalizeAndValidatePath, not string prefixing:
	// /home/x/project-notes is not inside /home/x/project.
	abs, outside, err := NormalizeAndValidatePath("/home/x/project", "/home/x/project-notes/file")
	if err != nil {
		t.Fatalf("NormalizeAndValidatePath: %v", err)
	}
	if !outside {
		t.Errorf("project-notes counted as inside project: %s", abs)
	}
	if _, outside, _ := NormalizeAndValidatePath("/home/x/project", "/home/x/project/file"); outside {
		t.Error("project/file counted as outside its own project")
	}
}

func TestSubagentNoNesting(t *testing.T) {
	parentReg := NewRegistry()
	parentReg.Enable(&stubNamedTool{name: "Read"})
	parentCtx := NewToolContext()
	child, _ := ResearchChildScope(parentReg, parentCtx)
	if child.Get("Subagent") != nil {
		t.Error("child handle has Subagent: nesting must stop at depth 1")
	}
}

func TestSubagentBatchRunsInOrder(t *testing.T) {
	cfg := &config.Config{}
	parentCtx := NewToolContext()
	tempMgr := NewTempFileManager(t.TempDir())
	stub := &stubSubRunner{fn: func(p SubagentRunParams) (SubagentRunResult, error) {
		return SubagentRunResult{Text: "summary of " + p.Description, Model: "m"}, nil
	}}
	sub := NewSubagentTool(cfg, parentCtx, tempMgr)
	parentReg := NewRegistry()
	parentReg.Enable(sub)
	batch := NewBatchTool(cfg)
	batch.SetRegistry(parentReg)
	batch.SetToolContext(parentCtx)
	parentReg.Enable(batch)
	sub.SetRegistry(parentReg)
	sub.SetRunner(stub)

	out, err := batch.Call(context.Background(), mustJSON(map[string]any{
		"calls": []any{
			map[string]any{"tool": "Subagent", "args": map[string]any{"description": "first task", "prompt": "do first"}},
			map[string]any{"tool": "Subagent", "args": map[string]any{"description": "second task", "prompt": "do second"}},
		},
	}))
	if err != nil {
		t.Fatalf("Batch Call: %v", err)
	}
	m := out.(map[string]any)
	results := m["results"].([]BatchResult)
	if len(results) != 2 || !results[0].OK || !results[1].OK {
		t.Fatalf("batch results %+v, want two oks", results)
	}
	got := stub.orderedDescriptions()
	if len(got) != 2 || got[0] != "first task" || got[1] != "second task" {
		t.Errorf("Subagents ran in %v, want [first task second task] in order", got)
	}
	// Subagent stays out of the parallel phase: it is not read-only (a
	// general child edits; even research burns wall-clock), so Batch runs it
	// sequentially.
	var psTool any = sub
	if ps, ok := psTool.(ParallelSafeTool); ok && ps.ParallelSafe() {
		t.Error("Subagent reports ParallelSafe: several Subagents in one Batch must run in order")
	}
}

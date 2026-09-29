package tools

// Phase-3 tests (spec/subagents.md 5.6, phased plan in 8.3): spawn/collect
// split, max_concurrent at spawn time, per-handle isolation and results.
// Each test asserts one thing; a gate stub holds children "running" until
// the test lets them finish, so no wall-clock parallelism is asserted.

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// gateStubRunner is a SubRunner whose children stay running until released,
// like a real child loop that has not answered yet. One release channel per
// description lets a test finish children independently; fail marks
// descriptions that error instead of answering.
type gateStubRunner struct {
	mu       sync.Mutex
	release  map[string]chan struct{}
	fail     map[string]bool
	started  []SubagentRunParams
	maxIters map[string]int
}

func newGateStubRunner() *gateStubRunner {
	return &gateStubRunner{
		release:  make(map[string]chan struct{}),
		fail:     make(map[string]bool),
		maxIters: make(map[string]int),
	}
}

func (g *gateStubRunner) gateFor(desc string) chan struct{} {
	g.mu.Lock()
	defer g.mu.Unlock()
	ch, ok := g.release[desc]
	if !ok {
		ch = make(chan struct{})
		g.release[desc] = ch
	}
	return ch
}

func (g *gateStubRunner) releaseDesc(desc string) {
	g.mu.Lock()
	ch, ok := g.release[desc]
	g.mu.Unlock()
	if ok {
		select {
		case <-ch: // already closed
		default:
			close(ch)
		}
	} else {
		g.mu.Lock()
		ch = make(chan struct{})
		close(ch)
		g.release[desc] = ch
		g.mu.Unlock()
	}
}

func (g *gateStubRunner) RunSubagent(_ context.Context, p SubagentRunParams) (SubagentRunResult, error) {
	_, res, err := g.StartSubagent(context.Background(), p)
	return res, err
}

func (g *gateStubRunner) StartSubagent(ctx context.Context, p SubagentRunParams) (*SubagentSession, SubagentRunResult, error) {
	g.mu.Lock()
	g.started = append(g.started, p)
	g.maxIters[p.Description] = p.MaxIters
	fail := g.fail[p.Description]
	g.mu.Unlock()
	if fail {
		return nil, SubagentRunResult{}, fmt.Errorf("boom %s", p.Description)
	}
	select {
	case <-g.gateFor(p.Description):
	case <-ctx.Done():
		return nil, SubagentRunResult{}, ctx.Err()
	}
	return NewSubagentSession(nil), SubagentRunResult{
		Text:             "result " + p.Description,
		Model:            "m",
		Iterations:       2,
		PromptTokens:     10,
		CompletionTokens: 20,
	}, nil
}

func (g *gateStubRunner) ContinueSubagent(_ context.Context, _ *SubagentSession, _ string) (SubagentRunResult, error) {
	return SubagentRunResult{Text: "", Model: "m"}, nil
}

// phase3Fixture wires a SubagentTool (with runner + registry) and its five
// collect tools, all sharing the one pool.
func phase3Fixture(t *testing.T, cfg *config.Config, runner SubRunner) (*SubagentTool, *Registry) {
	t.Helper()
	if cfg.Workspace.Root == "" {
		cfg.Workspace.Root = t.TempDir()
	}
	if cfg.Tools.SafetyConfirmations == nil {
		cfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{}
	}
	parentCtx := NewToolContext()
	tempMgr := NewTempFileManager(t.TempDir())
	parentReg := testParentRegistry(t, cfg, parentCtx, tempMgr)
	tool := NewSubagentTool(cfg, parentCtx, tempMgr)
	tool.SetRegistry(parentReg)
	tool.SetRunner(runner)
	reg := NewRegistry()
	reg.Enable(tool)
	reg.Enable(NewSubagentStartTool(tool))
	reg.Enable(NewSubagentStatusTool(tool))
	reg.Enable(NewSubagentOutputTool(tool))
	reg.Enable(NewSubagentWaitTool(tool))
	reg.Enable(NewSubagentKillTool(tool))
	return tool, reg
}

func startChild(t *testing.T, reg *Registry, desc, prompt string) string {
	t.Helper()
	tool := reg.Get("Subagent.start")
	res, err := tool.Call(context.Background(), mustJSON(map[string]any{
		"description": desc, "prompt": prompt,
	}))
	if err != nil {
		t.Fatalf("start %s: %v", desc, err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("start result type %T", res)
	}
	handle, _ := m["handle"].(string)
	if handle == "" {
		t.Fatalf("start %s returned no handle: %v", desc, res)
	}
	if m["status"] != subagentRunning {
		t.Fatalf("start %s status %v, want running", desc, m["status"])
	}
	return handle
}

func collectChild(t *testing.T, reg *Registry, handle string) *SubagentCallResult {
	t.Helper()
	// Wait first, the way a model should: release only unblocks the child,
	// and the background goroutine still needs a scheduling moment.
	if _, err := reg.Get("Subagent.wait").Call(context.Background(),
		mustJSON(map[string]any{"handle": handle, "timeout_s": 30})); err != nil {
		t.Fatalf("wait %s: %v", handle, err)
	}
	tool := reg.Get("Subagent.output")
	res, err := tool.Call(context.Background(), mustJSON(map[string]any{"handle": handle}))
	if err != nil {
		t.Fatalf("output %s: %v", handle, err)
	}
	out, ok := res.(*SubagentCallResult)
	if !ok {
		t.Fatalf("output type %T, want *SubagentCallResult", res)
	}
	return out
}

func TestSubagentStartReturnsHandleWhileRunning(t *testing.T) {
	cfg := &config.Config{}
	gate := newGateStubRunner()
	_, reg := phase3Fixture(t, cfg, gate)
	started := time.Now()
	h := startChild(t, reg, "slow task", "take your time")
	if elapsed := time.Since(started); elapsed > 5*time.Second {
		t.Errorf("spawn took %v; spawning must return before any LLM call", elapsed)
	}
	st, err := reg.Get("Subagent.status").Call(context.Background(), mustJSON(map[string]any{"handle": h}))
	if err != nil {
		t.Fatalf("status: %v", err)
	}
	if m := st.(map[string]any); m["status"] != subagentRunning || m["done"] != false {
		t.Errorf("status %+v, want running/not-done", m)
	}
	gate.releaseDesc("slow task")
	out := collectChild(t, reg, h)
	if !strings.Contains(out.Result, "result slow task") {
		t.Errorf("result %q, want the child's summary", out.Result)
	}
}

func TestSubagentMaxConcurrentEnforcedAtSpawn(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Subagent.MaxConcurrent = 2
	gate := newGateStubRunner()
	_, reg := phase3Fixture(t, cfg, gate)
	a := startChild(t, reg, "task A", "work")
	b := startChild(t, reg, "task B", "work")

	_, err := reg.Get("Subagent.start").Call(context.Background(), mustJSON(map[string]any{
		"description": "task C", "prompt": "work",
	}))
	if err == nil {
		t.Fatal("third spawn with max_concurrent=2 succeeded, want fail-closed")
	}
	if !IsBacktrackable(err) {
		t.Errorf("cap error %v is not backtrackable (semantic)", err)
	}
	if !strings.Contains(err.Error(), "max_concurrent=2") {
		t.Errorf("cap error %q does not name the cap", err.Error())
	}

	// Killing one frees its permit; the next spawn succeeds.
	if _, err := reg.Get("Subagent.kill").Call(context.Background(), mustJSON(map[string]any{"handle": a})); err != nil {
		t.Fatalf("kill: %v", err)
	}
	c := startChild(t, reg, "task C", "work")

	gate.releaseDesc("task B")
	gate.releaseDesc("task C")
	_ = b
	out := collectChild(t, reg, c)
	if !strings.Contains(out.Result, "result task C") {
		t.Errorf("result %q", out.Result)
	}
}

func TestSubagentChildrenIsolated(t *testing.T) {
	cfg := &config.Config{}
	gate := newGateStubRunner()
	tool, reg := phase3Fixture(t, cfg, gate)
	_ = tool
	a := startChild(t, reg, "task A", "work")
	b := startChild(t, reg, "task B", "work")

	tool.poolMu.Lock()
	ha, hb := tool.handles[a], tool.handles[b]
	tool.poolMu.Unlock()
	if ha == nil || hb == nil {
		t.Fatal("handles not in pool")
	}
	if ha.prep.childCtx == hb.prep.childCtx {
		t.Error("two children share one ToolContext")
	}
	if ha.prep.childReg == hb.prep.childReg {
		t.Error("two children share one Registry")
	}
	for _, h := range []*subagentChild{ha, hb} {
		for _, absent := range []string{"Subagent", "Subagent.start", "Subagent.status",
			"Subagent.output", "Subagent.wait", "Subagent.kill", "Question", "Report"} {
			if h.prep.childReg.Get(absent) != nil {
				t.Errorf("child handle carries excluded tool %q (no nesting)", absent)
			}
		}
		if !h.prep.childCtx.IsSubagentChild() {
			t.Error("child context not marked")
		}
	}

	gate.releaseDesc("task A")
	gate.releaseDesc("task B")
	collectChild(t, reg, a)
	collectChild(t, reg, b)
}

func TestSubagentSiblingFailureIsolated(t *testing.T) {
	// The phase-3 stress case at small N: several scripted children, one
	// failure, every sibling result intact.
	cfg := &config.Config{}
	gate := newGateStubRunner()
	gate.mu.Lock()
	gate.fail["task C"] = true
	gate.mu.Unlock()
	_, reg := phase3Fixture(t, cfg, gate)
	handles := map[string]string{}
	for _, desc := range []string{"task A", "task B", "task C", "task D", "task E"} {
		handles[desc] = startChild(t, reg, desc, "work")
	}
	for _, desc := range []string{"task A", "task B", "task D", "task E"} {
		gate.releaseDesc(desc)
	}
	for _, desc := range []string{"task A", "task B", "task D", "task E"} {
		out := collectChild(t, reg, handles[desc])
		if !strings.Contains(out.Result, "result "+desc) {
			t.Errorf("%s result %q, want intact sibling summary", desc, out.Result)
		}
		if out.ChildStats() == nil || out.ChildStats().TotalPromptTokens != 10 {
			t.Errorf("%s stats %+v, want folded child ledger on collect", desc, out.ChildStats())
		}
	}
	_, err := reg.Get("Subagent.output").Call(context.Background(),
		mustJSON(map[string]any{"handle": handles["task C"]}))
	if err == nil {
		t.Fatal("failed child collected without error")
	}
	if IsBacktrackable(err) {
		t.Errorf("child failure %v is semantic; a failed run is a runtime error", err)
	}
	if !strings.Contains(err.Error(), "boom task C") {
		t.Errorf("child failure %q lost its cause", err.Error())
	}
}

func TestSubagentBudgetsSnapshottedAtSpawn(t *testing.T) {
	// Over-admission is bounded by max_concurrent, not by partitioning:
	// each child snapshots the parent's remaining iterations at spawn.
	cfg := &config.Config{}
	gate := newGateStubRunner()
	tool, reg := phase3Fixture(t, cfg, gate)
	tool.SetParentRemaining(7)
	a := startChild(t, reg, "task A", "work")
	tool.SetParentRemaining(2)
	b := startChild(t, reg, "task B", "work")
	// Spawns return before their goroutines record params; poll.
	deadline := time.Now().Add(10 * time.Second)
	var ga, gb int
	for {
		gate.mu.Lock()
		ga, gb = gate.maxIters["task A"], gate.maxIters["task B"]
		n := len(gate.maxIters)
		gate.mu.Unlock()
		if n == 2 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("only %d children recorded their budgets", n)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if ga != 7 || gb != 2 {
		t.Errorf("spawned budgets A=%d B=%d, want snapshots 7 and 2", ga, gb)
	}
	gate.releaseDesc("task A")
	gate.releaseDesc("task B")
	collectChild(t, reg, a)
	collectChild(t, reg, b)
}

func TestSubagentKillIsolatesSiblings(t *testing.T) {
	cfg := &config.Config{}
	gate := newGateStubRunner()
	_, reg := phase3Fixture(t, cfg, gate)
	a := startChild(t, reg, "task A", "work")
	b := startChild(t, reg, "task B", "work")

	killRes, err := reg.Get("Subagent.kill").Call(context.Background(), mustJSON(map[string]any{"handle": a}))
	if err != nil {
		t.Fatalf("kill: %v", err)
	}
	if m := killRes.(map[string]any); m["status"] != subagentKilled {
		t.Errorf("kill status %+v, want killed", m)
	}
	// The sibling is untouched and still running.
	st, err := reg.Get("Subagent.status").Call(context.Background(), mustJSON(map[string]any{"handle": b}))
	if err != nil {
		t.Fatalf("sibling status: %v", err)
	}
	if m := st.(map[string]any); m["status"] != subagentRunning {
		t.Errorf("sibling status %+v, want still running", m)
	}

	// Killing a finished handle is a no-op reporting its state.
	gate.releaseDesc("task B")
	collectChild(t, reg, b)
	again, err := reg.Get("Subagent.kill").Call(context.Background(), mustJSON(map[string]any{"handle": b}))
	if err != nil {
		t.Fatalf("second kill: %v", err)
	}
	if m := again.(map[string]any); m["status"] != subagentDone {
		t.Errorf("kill of finished handle %+v, want done", m)
	}

	// The killed handle collects a labeled partial result, not an error.
	out, err := reg.Get("Subagent.output").Call(context.Background(), mustJSON(map[string]any{"handle": a}))
	if err != nil {
		t.Fatalf("output after kill: %v (want labeled partial, not error)", err)
	}
	got := out.(*SubagentCallResult)
	if !got.Killed || !strings.Contains(got.Result, "[killed:") {
		t.Errorf("killed result %+v lacks the killed flag/label", got)
	}
}

func TestSubagentOutputBeforeDone(t *testing.T) {
	cfg := &config.Config{}
	gate := newGateStubRunner()
	_, reg := phase3Fixture(t, cfg, gate)
	h := startChild(t, reg, "slow task", "work")

	_, err := reg.Get("Subagent.output").Call(context.Background(), mustJSON(map[string]any{"handle": h}))
	if err == nil {
		t.Fatal("output while running succeeded without wait, want fail-closed")
	}
	if !IsBacktrackable(err) {
		t.Errorf("still-running error %v is not backtrackable (semantic)", err)
	}

	// wait:true with a short timeout also fails closed, like Observe.wait's
	// max_wait — but as an error, since output owes a result shape.
	_, err = reg.Get("Subagent.output").Call(context.Background(),
		mustJSON(map[string]any{"handle": h, "wait": true, "timeout_s": 1}))
	if err == nil {
		t.Fatal("output wait timed out without error")
	}

	// Subagent.wait itself reports running on timeout rather than erroring.
	w, err := reg.Get("Subagent.wait").Call(context.Background(),
		mustJSON(map[string]any{"handle": h, "timeout_s": 1}))
	if err != nil {
		t.Fatalf("wait: %v", err)
	}
	if m := w.(map[string]any); m["done"] != false || m["status"] != subagentRunning {
		t.Errorf("wait timeout %+v, want running/not-done", m)
	}

	// wait:true with room collects once the child finishes.
	go func() {
		time.Sleep(100 * time.Millisecond)
		gate.releaseDesc("slow task")
	}()
	res, err := reg.Get("Subagent.output").Call(context.Background(),
		mustJSON(map[string]any{"handle": h, "wait": true, "timeout_s": 30}))
	if err != nil {
		t.Fatalf("output with wait: %v", err)
	}
	if out := res.(*SubagentCallResult); !strings.Contains(out.Result, "result slow task") {
		t.Errorf("result %q", out.Result)
	}
}

func TestSubagentBatchSpawnsOverlap(t *testing.T) {
	// No Batch carve-out in any phase: three cheap sequential spawns in one
	// Batch still yield three overlapping children.
	cfg := &config.Config{}
	cfg.Tools.Batch.Enabled = true
	gate := newGateStubRunner()
	_, reg := phase3Fixture(t, cfg, gate)
	batch := NewBatchTool(cfg)
	batch.SetRegistry(reg)
	batch.SetToolContext(NewToolContext())

	out, err := batch.Call(context.Background(), mustJSON(map[string]any{
		"calls": []any{
			map[string]any{"tool": "Subagent.start", "args": map[string]any{"description": "task A", "prompt": "work"}},
			map[string]any{"tool": "Subagent.start", "args": map[string]any{"description": "task B", "prompt": "work"}},
			map[string]any{"tool": "Subagent.start", "args": map[string]any{"description": "task C", "prompt": "work"}},
		},
	}))
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	m := out.(map[string]any)
	results := m["results"].([]BatchResult)
	if len(results) != 3 {
		t.Fatalf("batch results %+v", results)
	}
	handles := make([]string, 0, 3)
	for i, r := range results {
		if !r.OK {
			t.Fatalf("spawn %d failed: %s", i, r.Error)
		}
		rm, ok := r.Result.(map[string]any)
		if !ok {
			t.Fatalf("spawn %d result type %T", i, r.Result)
		}
		h, _ := rm["handle"].(string)
		if h == "" {
			t.Fatalf("spawn %d has no handle: %v", i, r.Result)
		}
		handles = append(handles, h)
	}
	// Order preserved, and all three overlap: none finished yet. The
	// spawns return before their goroutines are scheduled, so poll.
	deadline := time.Now().Add(10 * time.Second)
	for {
		gate.mu.Lock()
		started := len(gate.started)
		gate.mu.Unlock()
		if started == 3 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("started %d children, want 3 overlapping", started)
		}
		time.Sleep(10 * time.Millisecond)
	}
	for _, h := range handles {
		st, err := reg.Get("Subagent.status").Call(context.Background(), mustJSON(map[string]any{"handle": h}))
		if err != nil {
			t.Fatal(err)
		}
		if m := st.(map[string]any); m["status"] != subagentRunning {
			t.Errorf("handle %s status %+v, want running (overlap)", h, m)
		}
	}
	for _, desc := range []string{"task A", "task B", "task C"} {
		gate.releaseDesc(desc)
	}
	for _, h := range handles {
		collectChild(t, reg, h)
	}
}

func TestSubagentChangeReportingOnCollect(t *testing.T) {
	// Spawn marks nothing; the first collect fires NoteChange and the
	// staleness handoff exactly once.
	cfg := &config.Config{}
	gate := &changeStubRunner{gateStubRunner: newGateStubRunner()}
	tool, reg := phase3Fixture(t, cfg, gate)
	parentCtx := tool.toolCtx
	h := startChild(t, reg, "edit task", "change a file")
	if parentCtx.ChangedThisTurn() {
		t.Fatal("spawn flipped parent ChangedThisTurn; reporting belongs to collect")
	}
	if len(parentCtx.EditedPaths()) != 0 {
		t.Fatal("spawn populated the parent stale set; handoff belongs to collect")
	}
	gate.releaseDesc("edit task")
	first := collectChild(t, reg, h)
	if !parentCtx.ChangedThisTurn() {
		t.Error("collect did not flip parent ChangedThisTurn")
	}
	if !parentCtx.IsStalePath("/edited.txt") {
		t.Error("collect did not hand the edited path to the parent stale set")
	}
	// Repeats return the cached result without re-firing.
	second := collectChild(t, reg, h)
	if first.Result != second.Result {
		t.Error("second collect differs; collect must be idempotent")
	}
}

// changeStubRunner mutates through the child's own context, the way a
// general child that edited would.
type changeStubRunner struct {
	*gateStubRunner
}

func (g *changeStubRunner) StartSubagent(ctx context.Context, p SubagentRunParams) (*SubagentSession, SubagentRunResult, error) {
	select {
	case <-g.gateFor(p.Description):
	case <-ctx.Done():
		return nil, SubagentRunResult{}, ctx.Err()
	}
	p.ToolCtx.NoteChange()
	p.ToolCtx.RecordEditedPath(p.ToolCtx.SessionTmp() + "/edited.txt")
	return NewSubagentSession(nil), SubagentRunResult{Text: "edited", Model: "m", Iterations: 1}, nil
}

func TestSubagentUnknownHandle(t *testing.T) {
	cfg := &config.Config{}
	_, reg := phase3Fixture(t, cfg, newGateStubRunner())
	for tool, args := range map[string]map[string]any{
		"Subagent.status": {"handle": "sub99"},
		"Subagent.output": {"handle": "sub99"},
		"Subagent.wait":   {"handle": "sub99", "timeout_s": 1},
		"Subagent.kill":   {"handle": "sub99"},
	} {
		_, err := reg.Get(tool).Call(context.Background(), mustJSON(args))
		if err == nil {
			t.Errorf("%s with unknown handle succeeded, want semantic error", tool)
		} else if !IsBacktrackable(err) {
			t.Errorf("%s unknown-handle error %v is not backtrackable", tool, err)
		}
	}
	for tool := range map[string]bool{
		"Subagent.status": true, "Subagent.output": true,
		"Subagent.wait": true, "Subagent.kill": true,
	} {
		if err := reg.Get(tool).Check(context.Background(), mustJSON(map[string]any{})); err == nil {
			t.Errorf("%s Check without handle passed", tool)
		}
	}
}

var _ = json.Marshal

package tools

// Phase-3 background fan-out (spec/subagents.md 5.6): split spawn from
// collect, following the Shell.start pattern rather than the Batch pattern.
//
// Subagent stays blocking for single delegations. Subagent.start spawns a
// child that runs while the turn continues and returns a handle
// immediately; Subagent.status/output/wait/kill collect from it. Sequential
// spawns fan out — three cheap sequential spawns (including three in one
// Batch, which needs no carve-out) yield three overlapping children.
//
// The concurrency cap (max_concurrent, default 8) is enforced at spawn
// time: a spawn while N children run fails closed with a semantic error
// the parent can reason about. The permit releases on collect, kill and
// child exit, on every path. Each child gets its own filtered registry and
// fresh ToolContext (per-child isolation is a spawn-path invariant); stats
// fold into the parent totals on collect; one child's failure never cancels
// its siblings. Handles live in memory only — one process per turn means
// they die with the turn, so a parent that fans out must wait for or
// collect every handle before the turn ends. No worktrees, no Batch
// changes, no nesting (children never see the Subagent.* tools).

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"
)

// Child handle states. A handle is terminal once its done channel closes.
const (
	subagentRunning = "running"
	subagentDone    = "done"
	subagentFailed  = "failed"
	subagentKilled  = "killed"
)

// defaultSubagentWaitTimeoutS bounds one wait/output-with-wait round when
// the call names none. Zero also selects it, never an unbounded wait.
const defaultSubagentWaitTimeoutS = 300

// subagentChild is one background child: its own filtered registry, fresh
// ToolContext, and the outcome once finished. Everything mutable is guarded
// by mu; the permit flag is guarded by the pool mutex instead, so release
// never takes two locks in the wrong order.
type subagentChild struct {
	handle      string
	description string
	parsed      *subagentArgs
	prep        *spawnPrep
	started     time.Time
	cancel      context.CancelFunc
	done        chan struct{}

	mu            sync.Mutex
	status        string
	result        *SubagentRunResult
	runErr        error
	collected     bool
	collectResult *SubagentCallResult
	collectErr    error
	released      bool
}

// acquirePermit takes one concurrency slot, enforcing max_concurrent at
// spawn time. Blocking runs hold a slot too, so they count against
// background children running alongside them.
func (t *SubagentTool) acquirePermit() bool {
	t.poolMu.Lock()
	defer t.poolMu.Unlock()
	if t.handles == nil {
		t.handles = make(map[string]*subagentChild)
	}
	cap := t.resolvedMaxConcurrent()
	if t.running >= cap {
		return false
	}
	t.running++
	return true
}

// releasePermit frees the handle's slot. It runs on child exit, on kill
// and on first collect alike; the flag makes the first call win so every
// path releases exactly once.
func (t *SubagentTool) releasePermit(h *subagentChild) {
	if h == nil {
		// Blocking runs hold an anonymous slot: always release one.
		t.poolMu.Lock()
		defer t.poolMu.Unlock()
		if t.running > 0 {
			t.running--
		}
		return
	}
	t.poolMu.Lock()
	defer t.poolMu.Unlock()
	if h.released {
		return
	}
	h.released = true
	if t.running > 0 {
		t.running--
	}
}

func (t *SubagentTool) resolvedMaxConcurrent() int {
	if t.cfg == nil {
		return 8
	}
	return t.cfg.Tools.Subagent.ResolvedMaxConcurrent()
}

// capExhaustedError fails a spawn closed when max_concurrent children are
// already running. Semantic, so the parent backs off, collects, and
// retries instead of stalling.
func (t *SubagentTool) capExhaustedError(caller string) error {
	t.poolMu.Lock()
	running, cap := t.running, t.resolvedMaxConcurrent()
	t.poolMu.Unlock()
	return SemanticErrorf("%s: already running %d of %d concurrent subagents (max_concurrent=%d). "+
		"Collect a finished child with Subagent.output or stop one with Subagent.kill, then retry.",
		caller, running, cap, cap)
}

func (t *SubagentTool) nextHandle() string {
	t.poolMu.Lock()
	defer t.poolMu.Unlock()
	t.seq++
	return fmt.Sprintf("sub%d", t.seq)
}

func (t *SubagentTool) lookupHandle(caller, handle string) (*subagentChild, error) {
	t.poolMu.Lock()
	defer t.poolMu.Unlock()
	if h, ok := t.handles[strings.TrimSpace(handle)]; ok {
		return h, nil
	}
	return nil, SemanticErrorf("%s: unknown subagent handle %q. Subagent.start returns the handle; handles live only for this turn.",
		caller, handle)
}

// spawnBackground validates nothing (the caller already parsed), prepares
// the child scope, takes a permit, and launches the run in a goroutine.
// It returns before any LLM call, so sequential spawns still overlap.
func (t *SubagentTool) spawnBackground(ctx context.Context, parentReg *Registry, parsed *subagentArgs, maxChild int) (map[string]any, error) {
	runner := t.getRunner()
	if runner == nil {
		return nil, RuntimeError("Subagent.start: no child runner is configured")
	}
	prep, err := t.prepareSpawn(parentReg, parsed, maxChild)
	if err != nil {
		return nil, err
	}
	if !t.acquirePermit() {
		return nil, t.capExhaustedError("Subagent.start")
	}
	// Detached from the iteration scope, which ends when this Call
	// returns: the child outlives it. Cancellation is explicit per handle
	// (Subagent.kill), never ctx propagation into siblings. The turn
	// ending reaps whatever is still running, since handles are in-memory.
	childCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), secondsToDuration(prep.timeoutS))
	h := &subagentChild{
		description: parsed.Description,
		parsed:      parsed,
		prep:        prep,
		started:     time.Now(),
		cancel:      cancel,
		done:        make(chan struct{}),
		status:      subagentRunning,
	}
	h.handle = t.nextHandle()
	t.poolMu.Lock()
	if t.handles == nil {
		t.handles = make(map[string]*subagentChild)
	}
	t.handles[h.handle] = h
	t.poolMu.Unlock()

	go t.runBackground(runner, h, childCtx)
	return map[string]any{
		"handle":        h.handle,
		"status":        subagentRunning,
		"description":   h.description,
		"subagent_type": parsed.SubagentType,
		"hint":          "The child runs while the turn continues. Collect it with Subagent.output, check it with Subagent.status, block with Subagent.wait, stop it with Subagent.kill.",
	}, nil
}

// runBackground drives one child to completion, including the schema and
// empty-summary follow-ups, then records the outcome. Killing one handle
// cancels only its own context; siblings keep their own.
func (t *SubagentTool) runBackground(runner SubRunner, h *subagentChild, ctx context.Context) {
	defer close(h.done)
	defer t.releasePermit(h)

	prep := h.prep
	sess, res, err := runner.StartSubagent(ctx, SubagentRunParams{
		System:      prep.system,
		Prompt:      h.parsed.Prompt,
		Description: h.parsed.Description,
		MaxIters:    prep.maxChild,
		Registry:    prep.childReg,
		ToolCtx:     prep.childCtx,
	})
	if err != nil {
		// A total failure to run: per-handle, never touching siblings.
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.status != subagentKilled {
			h.status = subagentFailed
		}
		h.runErr = err
		return
	}
	if prep.wantSchema {
		res = t.enforceSchema(ctx, runner, sess, prep.capture, res)
	} else {
		res = t.nudgeEmptySummary(ctx, runner, sess, res)
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.status == subagentKilled {
		// Kill wins over whatever the run just produced: keep the killed
		// status and whatever partial text came back.
		res.Killed = true
	}
	h.result = &res
	if h.status != subagentKilled {
		h.status = subagentDone
	}
}

// terminal reports whether the child reached a final state. Callers only
// touch childCtx after this is true, and done-closure gives the
// happens-before edge, so no race with the background goroutine.
func (h *subagentChild) terminal() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.status != subagentRunning
}

func (t *SubagentTool) childSnapshot(h *subagentChild) map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	return map[string]any{
		"handle":        h.handle,
		"status":        h.status,
		"done":          h.status != subagentRunning,
		"description":   h.description,
		"subagent_type": h.parsed.SubagentType,
		"elapsed_s":     int(time.Since(h.started).Seconds()),
	}
}

// waitForChild blocks until the handle finishes, the timeout passes, or
// the caller's context ends. True means terminal; false means still
// running (timeout or cancellation).
func (t *SubagentTool) waitForChild(ctx context.Context, h *subagentChild, timeoutS int) bool {
	if timeoutS <= 0 {
		timeoutS = defaultSubagentWaitTimeoutS
	}
	if h.terminal() {
		return true
	}
	timer := time.NewTimer(secondsToDuration(timeoutS))
	defer timer.Stop()
	select {
	case <-h.done:
		return true
	case <-timer.C:
		return h.terminal()
	case <-ctx.Done():
		return h.terminal()
	}
}

// collectOutput returns the handle's result, doing the change reporting
// and staleness handoff exactly once, on first collect. Stats ride on the
// SubagentCallResult, so the runner folds them the same way it does for a
// blocking call. Repeats return the cached result.
func (t *SubagentTool) collectOutput(ctx context.Context, handle string, wait bool, timeoutS int) (any, error) {
	h, err := t.lookupHandle("Subagent.output", handle)
	if err != nil {
		return nil, err
	}
	if !h.terminal() {
		if !wait {
			return nil, SemanticErrorf("Subagent.output: child %s is still running. "+
				"Collect it later, block with Subagent.wait, or pass wait:true with a timeout.",
				h.handle)
		}
		if !t.waitForChild(ctx, h, timeoutS) {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			return nil, SemanticErrorf("Subagent.output: child %s is still running after %ds. "+
				"Wait again, collect it later, or stop it with Subagent.kill.",
				h.handle, maxInt(timeoutS, 0))
		}
	}

	h.mu.Lock()
	if h.collected {
		res, collectErr := h.collectResult, h.collectErr
		h.mu.Unlock()
		if collectErr != nil {
			return nil, collectErr
		}
		return res, nil
	}
	status, runErr, result := h.status, h.runErr, h.result
	if result == nil {
		// Killed before the run produced anything: an empty partial
		// result, still labeled, never an error — the kill was asked
		// for. (A natural failure keeps its error below.)
		empty := &SubagentRunResult{Killed: status == subagentKilled}
		result = empty
	}
	h.mu.Unlock()

	// First collect: coarse change reporting, mirroring a blocking run.
	// The child is terminal, so reading its context is safe.
	t.finishCollect(h.prep.childCtx)
	t.releasePermit(h) // backstop; the exit path usually released already

	h.mu.Lock()
	defer h.mu.Unlock()
	h.collected = true
	if status == subagentFailed {
		h.collectErr = WrapAsRuntime(runErr)
		return nil, h.collectErr
	}
	shaped := t.shapeResult(*result, h.parsed)
	h.collectResult = shaped
	return shaped, nil
}

func maxInt(a, b int) int {
	if a < 0 {
		a = 0
	}
	if b < 0 {
		b = 0
	}
	if a > b {
		return a
	}
	return b
}

// killChild stops one running child without touching its siblings. It
// waits for the background goroutine to exit so a following output sees
// the partial result, not a race. Killing a finished handle is a no-op
// that reports its state.
func (t *SubagentTool) killChild(ctx context.Context, handle string) (map[string]any, error) {
	h, err := t.lookupHandle("Subagent.kill", handle)
	if err != nil {
		return nil, err
	}
	h.mu.Lock()
	if h.status != subagentRunning {
		h.mu.Unlock()
		return t.childSnapshot(h), nil
	}
	h.status = subagentKilled
	cancel := h.cancel
	h.mu.Unlock()

	cancel()
	select {
	case <-h.done:
	case <-ctx.Done():
	}
	t.releasePermit(h) // backstop; the exit path usually released already
	return t.childSnapshot(h), nil
}

// maxChildBudget snapshots the parent's remaining iterations at spawn
// time (spec 5.2): over-admission is bounded by max_concurrent, not by
// partitioning the budget across children.
func (t *SubagentTool) maxChildBudget() int {
	maxChild := t.cfg.Tools.Subagent.ResolvedMaxChildIterations()
	if remain, ok := t.getParentRemaining(); ok && remain < maxChild {
		maxChild = remain
	}
	if maxChild < 1 {
		maxChild = 1
	}
	return maxChild
}

// --- Subagent.start ----------------------------------------------------------

// SubagentStartTool spawns a child that runs while the turn continues and
// returns a handle immediately. Same arguments as Subagent; collect the
// handle later with Subagent.output.
type SubagentStartTool struct {
	shared *SubagentTool
}

// NewSubagentStartTool builds the tool over the shared Subagent instance,
// so budgets, the concurrency cap and handles are one pool.
func NewSubagentStartTool(shared *SubagentTool) *SubagentStartTool {
	return &SubagentStartTool{shared: shared}
}

func (t *SubagentStartTool) Name() string { return "Subagent.start" }

func (t *SubagentStartTool) Description() string {
	return "Spawn a subagent that runs while the turn continues. Same arguments as Subagent. " +
		"Returns a handle immediately; collect it later with Subagent.output, check it with " +
		"Subagent.status, block with Subagent.wait, stop it with Subagent.kill."
}

func (t *SubagentStartTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"required":   []string{"description", "prompt"},
		"properties": subagentArgProperties(),
	}
}

func (t *SubagentStartTool) PromptCategory() string     { return "filesystem" }
func (t *SubagentStartTool) PromptOrder() int           { return 61 }
func (t *SubagentStartTool) PromptTemplateName() string { return "" }
func (t *SubagentStartTool) PromptSection() string {
	return `### Subagent.start - Spawn a child that runs while the turn continues

Subagent.start({"description": "find auth flow", "prompt": "How does login work? Start at internal/auth/..."})
  → {"handle": "sub1", "status": "running", ...}

One cheap call per child: three sequential spawns (including three in one
Batch) still overlap, so fan out first, then collect each handle with
Subagent.output. At most max_concurrent (default 8) run at once; a spawn
beyond that fails closed — collect or kill one, then retry. Children die
with the turn: wait for or collect every handle before the turn ends.`
}

func (t *SubagentStartTool) Check(ctx context.Context, args json.RawMessage) error {
	_, err := t.shared.parse(args)
	return err
}

func (t *SubagentStartTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	parsed, err := t.shared.parse(args)
	if err != nil {
		return nil, err
	}
	parentReg := t.shared.getRegistry()
	if parentReg == nil {
		return nil, RuntimeError("Subagent.start: no tools are available to delegate to")
	}
	return t.shared.spawnBackground(ctx, parentReg, parsed, t.shared.maxChildBudget())
}

// --- Subagent.status ---------------------------------------------------------

// SubagentStatusTool reports whether a spawned subagent is still running.
type SubagentStatusTool struct {
	shared *SubagentTool
}

// NewSubagentStatusTool builds the tool over the shared Subagent instance.
func NewSubagentStatusTool(shared *SubagentTool) *SubagentStatusTool {
	return &SubagentStatusTool{shared: shared}
}

func (t *SubagentStatusTool) Name() string { return "Subagent.status" }

func (t *SubagentStatusTool) Description() string {
	return "Report whether a spawned subagent is still running, and its result state if not."
}

func (t *SubagentStatusTool) PromptCategory() string     { return "filesystem" }
func (t *SubagentStatusTool) PromptOrder() int           { return 62 }
func (t *SubagentStatusTool) PromptTemplateName() string { return "" }
func (t *SubagentStatusTool) PromptSection() string      { return "" }

func (t *SubagentStatusTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"handle": map[string]any{"type": "string", "description": "The subagent handle from Subagent.start."},
		},
		"required": []string{"handle"},
	}
}

func checkSubagentHandle(tool string, args json.RawMessage) (string, error) {
	var p struct {
		Handle string `json:"handle"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return "", SemanticErrorf("%s: arguments are not valid JSON: %v", tool, err)
	}
	if strings.TrimSpace(p.Handle) == "" {
		return "", SemanticErrorf("%s: 'handle' is required. Subagent.start returns it.", tool)
	}
	return p.Handle, nil
}

func (t *SubagentStatusTool) Check(ctx context.Context, args json.RawMessage) error {
	_, err := checkSubagentHandle("Subagent.status", args)
	return err
}

func (t *SubagentStatusTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	handle, err := checkSubagentHandle("Subagent.status", args)
	if err != nil {
		return nil, err
	}
	h, err := t.shared.lookupHandle("Subagent.status", handle)
	if err != nil {
		return nil, err
	}
	return t.shared.childSnapshot(h), nil
}

// --- Subagent.output ---------------------------------------------------------

// SubagentOutputTool collects a finished subagent's result. Stats fold into
// the turn totals and change reporting lands here, on collect — never on
// spawn, never twice.
type SubagentOutputTool struct {
	shared *SubagentTool
}

// NewSubagentOutputTool builds the tool over the shared Subagent instance.
func NewSubagentOutputTool(shared *SubagentTool) *SubagentOutputTool {
	return &SubagentOutputTool{shared: shared}
}

func (t *SubagentOutputTool) Name() string { return "Subagent.output" }

func (t *SubagentOutputTool) Description() string {
	return "Collect a spawned subagent's result. Fails closed while it runs unless wait is set; " +
		"repeats return the cached result."
}

func (t *SubagentOutputTool) PromptCategory() string     { return "filesystem" }
func (t *SubagentOutputTool) PromptOrder() int           { return 63 }
func (t *SubagentOutputTool) PromptTemplateName() string { return "" }
func (t *SubagentOutputTool) PromptSection() string {
	return `### Subagent.output - Collect a finished child

Subagent.output({"handle": "sub1"})
Subagent.output({"handle": "sub1", "wait": true, "timeout_s": 120})

Returns the same result shape as a blocking Subagent call. While the child
runs it fails closed — pass wait:true to block instead, or check
Subagent.status / block with Subagent.wait first. The first collect folds
the child's stats into the turn and reports its file changes; repeats return
the cached result. One child's failure comes back as that handle's error
alone and never cancels its siblings.`
}

// SelfTimeout opts out of the loop's blanket 15-second tool timeout: with
// wait set, collecting is allowed to take as long as the timeout says.
func (t *SubagentOutputTool) SelfTimeout() bool { return true }

func (t *SubagentOutputTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"handle":    map[string]any{"type": "string", "description": "The subagent handle from Subagent.start."},
			"wait":      map[string]any{"type": "boolean", "description": "Block until the child finishes instead of failing closed while it runs."},
			"timeout_s": map[string]any{"type": "integer", "description": "Seconds to wait when wait is set (default 300)."},
		},
		"required": []string{"handle"},
	}
}

func (t *SubagentOutputTool) Check(ctx context.Context, args json.RawMessage) error {
	handle, err := checkSubagentHandle("Subagent.output", args)
	if err != nil {
		return err
	}
	_ = handle
	return nil
}

func (t *SubagentOutputTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var p struct {
		Handle  string `json:"handle"`
		Wait    bool   `json:"wait"`
		Timeout int    `json:"timeout_s"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, SemanticErrorf("Subagent.output: arguments are not valid JSON: %v", err)
	}
	if strings.TrimSpace(p.Handle) == "" {
		return nil, SemanticErrorf("Subagent.output: 'handle' is required. Subagent.start returns it.")
	}
	return t.shared.collectOutput(ctx, p.Handle, p.Wait, p.Timeout)
}

// --- Subagent.wait -----------------------------------------------------------

// SubagentWaitTool blocks until a spawned subagent finishes, a timeout
// passes, or the call is cancelled — one tool call however long it takes,
// where polling Subagent.status in a loop costs a full round of thinking
// each time.
type SubagentWaitTool struct {
	shared *SubagentTool
}

// NewSubagentWaitTool builds the tool over the shared Subagent instance.
func NewSubagentWaitTool(shared *SubagentTool) *SubagentWaitTool {
	return &SubagentWaitTool{shared: shared}
}

func (t *SubagentWaitTool) Name() string { return "Subagent.wait" }

func (t *SubagentWaitTool) Description() string {
	return "Block until a spawned subagent finishes or a timeout passes. " +
		"Waiting here costs one tool call however long it takes; polling Subagent.status " +
		"in a loop costs a full round of thinking every time."
}

func (t *SubagentWaitTool) PromptCategory() string     { return "filesystem" }
func (t *SubagentWaitTool) PromptOrder() int           { return 64 }
func (t *SubagentWaitTool) PromptTemplateName() string { return "" }
func (t *SubagentWaitTool) PromptSection() string {
	return `### Subagent.wait - Block until a child finishes

Subagent.wait({"handle": "sub1"})
Subagent.wait({"handle": "sub1", "timeout_s": 60})

Blocks until the child finishes, timeout_s seconds pass (default 300), or
the call is cancelled. Then collect the result with Subagent.output. Reach
for it when there is nothing else to do; when there is, keep working and
collect later.`
}

// SelfTimeout opts out of the loop's blanket 15-second tool timeout:
// waiting is what this tool is for. It honours its own timeout instead.
func (t *SubagentWaitTool) SelfTimeout() bool { return true }

func (t *SubagentWaitTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"handle":    map[string]any{"type": "string", "description": "The subagent handle from Subagent.start."},
			"timeout_s": map[string]any{"type": "integer", "description": "Give up after this many seconds (default 300)."},
		},
		"required": []string{"handle"},
	}
}

func (t *SubagentWaitTool) Check(ctx context.Context, args json.RawMessage) error {
	_, err := checkSubagentHandle("Subagent.wait", args)
	return err
}

func (t *SubagentWaitTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var p struct {
		Handle  string `json:"handle"`
		Timeout int    `json:"timeout_s"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, SemanticErrorf("Subagent.wait: arguments are not valid JSON: %v", err)
	}
	if strings.TrimSpace(p.Handle) == "" {
		return nil, SemanticErrorf("Subagent.wait: 'handle' is required. Subagent.start returns it.")
	}
	h, err := t.shared.lookupHandle("Subagent.wait", p.Handle)
	if err != nil {
		return nil, err
	}
	done := t.shared.waitForChild(ctx, h, p.Timeout)
	if !done && ctx.Err() != nil {
		return nil, ctx.Err()
	}
	out := t.shared.childSnapshot(h)
	if !done {
		timeout := p.Timeout
		if timeout <= 0 {
			timeout = defaultSubagentWaitTimeoutS
		}
		out["timed_out_waiting"] = true
		out["timeout_s"] = timeout
	}
	return out, nil
}

// --- Subagent.kill -----------------------------------------------------------

// SubagentKillTool stops one running spawned subagent without touching its
// siblings. Killing a finished handle reports its state and changes
// nothing.
type SubagentKillTool struct {
	shared *SubagentTool
}

// NewSubagentKillTool builds the tool over the shared Subagent instance.
func NewSubagentKillTool(shared *SubagentTool) *SubagentKillTool {
	return &SubagentKillTool{shared: shared}
}

func (t *SubagentKillTool) Name() string { return "Subagent.kill" }

func (t *SubagentKillTool) Description() string {
	return "Stop a running spawned subagent without touching its siblings. " +
		"Killing a finished handle reports its state and changes nothing."
}

func (t *SubagentKillTool) PromptCategory() string     { return "filesystem" }
func (t *SubagentKillTool) PromptOrder() int           { return 65 }
func (t *SubagentKillTool) PromptTemplateName() string { return "" }
func (t *SubagentKillTool) PromptSection() string      { return "" }

func (t *SubagentKillTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"handle": map[string]any{"type": "string", "description": "The subagent handle from Subagent.start."},
		},
		"required": []string{"handle"},
	}
}

func (t *SubagentKillTool) Check(ctx context.Context, args json.RawMessage) error {
	_, err := checkSubagentHandle("Subagent.kill", args)
	return err
}

func (t *SubagentKillTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	handle, err := checkSubagentHandle("Subagent.kill", args)
	if err != nil {
		return nil, err
	}
	return t.shared.killChild(ctx, handle)
}

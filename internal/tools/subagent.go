package tools

// Subagent tool — mechanism (spawn a loop, relay usage, truncate the result).
//
// Policy (retries, nudges, concurrency cap) lives as config + constants at
// the top of this file, mirroring maki's primitives/policy split in Go form:
// this file owns the mechanism (schema, allowlists, result shaping) while
// internal/agent/subagent.go owns loop setup/teardown.
//
// Research is read-only delegation; general (phase 2a,
// spec/subagents-phase2.md) may edit files under the staleness handoff.
// output_schema enforcement is phase 2b (same amendment, section 6):
// a schema-carrying child must file its result through the session-local
// structured_output tool. Spawn/collect parallelism is phase 3.
// See spec/subagents.md.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/stats"
)

// Policy knobs. Defaults come from tools.subagent.* in the config; zero
// selects the default, never unbounded.
const (
	// maxNudges and maxSchemaErrors belong to the phase-2b output_schema
	// path. They are recorded here so phase 2b does not relitigate them.
	maxNudges       = 2
	maxSchemaErrors = 3
)

// SubagentDenyMessage is the semantic error a research child gets when a call
// would otherwise prompt a person. A nested prompt has no UI and would wedge
// the turn; failing closed pushes authors toward pre-approving the commands a
// delegated task needs.
const subagentPromptDenyHint = "subagents cannot prompt"

// SubagentSession is an opaque, in-process handle to a child loop whose
// history persists across ContinueSubagent calls. It carries the messages,
// registry and tool context one Subagent.Call needs for its follow-ups. It
// never reaches the session directory, history.jsonl, or the parent
// registry: it is built per Call and dies with the turn.
type SubagentSession struct {
	impl any
}

// NewSubagentSession wraps the driver's private session state. The agent
// package fills impl; the tool package only carries it back to Continue.
func NewSubagentSession(impl any) *SubagentSession {
	return &SubagentSession{impl: impl}
}

// Unwrap returns the driver's private session state.
func (s *SubagentSession) Unwrap() any {
	if s == nil {
		return nil
	}
	return s.impl
}

// SubagentRunParams is what the tool hands the child loop.
type SubagentRunParams struct {
	System      string
	Prompt      string
	Description string
	MaxIters    int
	Registry    *Registry
	ToolCtx     *ToolContext
}

// SubagentRunResult is what the child loop hands back.
type SubagentRunResult struct {
	Text             string
	BudgetExhausted  bool
	TimedOut         bool
	Killed           bool
	Iterations       int
	PromptTokens     int
	CompletionTokens int
	Cost             float64
	Model            string
}

// SubRunner drives a child loop synchronously inside the turn's process.
// It is implemented in internal/agent (which owns the Runner) and injected
// here via SetupConfig, mirroring how SetupConfig injects ProcRegistry.
// The interface lives in tools so tools never imports agent.
type SubRunner interface {
	RunSubagent(ctx context.Context, params SubagentRunParams) (SubagentRunResult, error)
	// StartSubagent runs the child loop to completion and returns a
	// session handle for follow-ups alongside the result.
	StartSubagent(ctx context.Context, params SubagentRunParams) (*SubagentSession, SubagentRunResult, error)
	// ContinueSubagent re-enters the session's loop with one more user
	// prompt (a structured_output or summary nudge) and runs it to
	// completion again.
	ContinueSubagent(ctx context.Context, sess *SubagentSession, prompt string) (SubagentRunResult, error)
}

// ParentBudgetSetter lets the parent loop tell the tool how many iterations
// it has left, so child_max = min(cfg max, parent remaining). The runner
// sets it before each call; Batch passes the context through, so delegation
// through a Batch sees the same budget.
type ParentBudgetSetter interface {
	SetParentRemaining(int)
}

// StatsCarrier is implemented by the Subagent result so the parent loop can
// fold the child's tokens/cost into its own totals.
type StatsCarrier interface {
	ChildStats() *stats.AgentStats
}

// SubagentCallResult is the JSON-serializable result of one Subagent call.
// It carries the truncated head text inline; the full text spills to tmp/.
type SubagentCallResult struct {
	Result           string `json:"result"`
	Truncated        bool   `json:"truncated,omitempty"`
	OverflowPath     string `json:"overflow_path,omitempty"`
	BudgetExhausted  bool   `json:"budget_exhausted,omitempty"`
	TimedOut         bool   `json:"timed_out,omitempty"`
	Killed           bool   `json:"killed,omitempty"`
	Iterations       int    `json:"iterations,omitempty"`
	Model            string `json:"model,omitempty"`
	PromptTokens     int    `json:"prompt_tokens,omitempty"`
	CompletionTokens int    `json:"completion_tokens,omitempty"`

	stats *stats.AgentStats
}

// ChildStats folds the child's ledger into the parent totals on return, so
// tokens/cost are attributed, not lost.
func (r *SubagentCallResult) ChildStats() *stats.AgentStats {
	if r == nil {
		return nil
	}
	return r.stats
}

// SubagentTool runs a child agent loop in-process with its own history and a
// restricted tool set, returning only the final summary text to the parent.
//
// It also owns the background-child pool for phase-3 fan-out: Subagent.start
// spawns into it, Subagent.status/output/wait/kill collect from it. The five
// collect tools hold this same instance, so budgets, the concurrency cap and
// handles are shared.
type SubagentTool struct {
	mu       sync.Mutex
	cfg      *config.Config
	toolCtx  *ToolContext
	registry *Registry
	tempMgr  *TempFileManager
	runner   SubRunner

	parentRemaining int
	hasRemaining    bool

	// Background-child pool (phase 3, spec/subagents.md 5.6). Handles live
	// in memory only: one process per turn means they die with the turn,
	// so there is nothing to persist and nothing to reap beyond cancelling.
	poolMu  sync.Mutex
	handles map[string]*subagentChild
	seq     int
	running int
}

// NewSubagentTool builds the tool. The parent registry it filters at Call
// time is set afterwards with SetRegistry, because the registry is what
// holds this tool. The child-loop driver is set with SetRunner after the
// agent Runner exists (turn setup), breaking the tools->agent import cycle.
func NewSubagentTool(cfg *config.Config, toolCtx *ToolContext, tempMgr *TempFileManager) *SubagentTool {
	return &SubagentTool{cfg: cfg, toolCtx: toolCtx, tempMgr: tempMgr, handles: make(map[string]*subagentChild)}
}

// SetRegistry gives the tool the parent registry it filters at Call time.
func (t *SubagentTool) SetRegistry(r *Registry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.registry = r
}

// SetRunner injects the child-loop driver (the parent Runner).
func (t *SubagentTool) SetRunner(r SubRunner) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.runner = r
}

// SetParentRemaining records the parent loop's remaining iteration budget.
func (t *SubagentTool) SetParentRemaining(n int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.parentRemaining = n
	t.hasRemaining = true
}

func (t *SubagentTool) getRunner() SubRunner {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.runner
}

func (t *SubagentTool) getRegistry() *Registry {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.registry
}

func (t *SubagentTool) getParentRemaining() (int, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.parentRemaining, t.hasRemaining
}

func (t *SubagentTool) Name() string { return "Subagent" }

func (t *SubagentTool) Description() string {
	return "Delegate a task to a subagent with its own context and a restricted tool set. " +
		"The child starts fresh: inline every file path, symbol name and constraint it needs. " +
		"Research children only read; general children may edit files. " +
		"Returns only its final summary text."
}

func (t *SubagentTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"required":   []string{"description", "prompt"},
		"properties": subagentArgProperties(),
	}
}

// subagentArgProperties is the shared argument shape for Subagent and
// Subagent.start: what to do, in whose context, and what result shape to
// demand. One copy so the two tools cannot drift apart.
func subagentArgProperties() map[string]any {
	return map[string]any{
		"description": map[string]any{
			"type":        "string",
			"description": "Short (3-5 words) description of the task. Shows in progress output.",
		},
		"prompt": map[string]any{
			"type":        "string",
			"description": "Detailed task prompt. The child starts fresh: inline every file path, symbol name and constraint it needs.",
		},
		"subagent_type": map[string]any{
			"type":        "string",
			"description": `"research" (read-only, default) or "general" (may edit files).`,
		},
		"output_schema": map[string]any{
			"type":        "object",
			"description": "Optional JSON Schema (type object) the child's final result must match. When set, the child gets a structured_output tool it must call with the result, and the result comes back as a validated JSON string.",
		},
	}
}

func (t *SubagentTool) PromptCategory() string     { return "filesystem" }
func (t *SubagentTool) PromptOrder() int           { return 60 }
func (t *SubagentTool) PromptTemplateName() string { return "" }

// SelfTimeout opts out of the loop's blanket 15-second tool timeout: a child
// loop burns minutes of wall-clock, bounded instead by tools.subagent.timeout_s
// via ctx.WithTimeout.
func (t *SubagentTool) SelfTimeout() bool { return true }

// ParallelSafe keeps Subagent in Batch's sequential phase: even research
// children burn minutes of wall-clock, and parallel delegation is the
// phase-3 spawn/collect split (Subagent.start), never a Batch carve-out.
func (t *SubagentTool) ParallelSafe() bool { return false }

func (t *SubagentTool) PromptSection() string {
	return `### Subagent - Delegate with its own context

Subagent({"description": "find auth flow", "prompt": "How does login work? Start at internal/auth/..."})
Subagent({"description": "fix retry bug", "prompt": "...", "subagent_type": "general"})

One blocking call per delegation. The child starts fresh with no parent
history: inline every file path, symbol name and constraint it needs.
Research children see a read-only tool set (Read, Search, Glob, Web.search,
Web.fetch, read-only Shell) and return only a summary — never dump code,
never write report files. General children may edit files (Edit, Write,
DeleteLines, full Shell) but only touch files that are part of the task:
give each child disjoint files, and re-read anything a child edited before
editing it yourself. All children share the turn's checkout — no worktrees,
no nesting, no background processes, no questions.

Several Subagents in one Batch run in order, but each is still its own
delegation with its own summary. With output_schema set, the child must file
its result through structured_output and the result comes back as validated
JSON.

For wall-clock parallelism, fan out with Subagent.start instead: one cheap
spawn call per child (three sequential spawns still overlap, so a Batch of
spawns works unchanged), then collect each handle with Subagent.output. At
most max_concurrent (default 8) children run at once; a spawn beyond that
fails closed — collect or kill one, then retry. Check progress with
Subagent.status, block with Subagent.wait, stop one with Subagent.kill
(killing one never touches the others). Stats fold into the turn totals on
collect, and a general child that edited marks the parent stale the same way
a blocking call does. Children die with the turn: wait for or collect every
handle before the turn ends.`
}

type subagentArgs struct {
	Description  string          `json:"description"`
	Prompt       string          `json:"prompt"`
	SubagentType string          `json:"subagent_type"`
	OutputSchema json.RawMessage `json:"output_schema"`
}

func (t *SubagentTool) parse(args json.RawMessage) (*subagentArgs, error) {
	var parsed subagentArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, SemanticErrorf("Subagent: arguments are not valid JSON: %v", err)
	}
	if strings.TrimSpace(parsed.Description) == "" {
		return nil, SemanticErrorf("Subagent: 'description' is required (3-5 words, shows in progress output).")
	}
	if strings.TrimSpace(parsed.Prompt) == "" {
		return nil, SemanticErrorf("Subagent: 'prompt' is required. The child starts fresh: inline every file path, symbol name and constraint it needs.")
	}
	typ := strings.TrimSpace(parsed.SubagentType)
	if typ == "" {
		typ = "research"
	}
	switch typ {
	case "research", "general":
	default:
		return nil, SemanticErrorf("Subagent: unknown subagent_type %q. Use \"research\" (read-only, default) or \"general\" (may edit files).", parsed.SubagentType)
	}
	parsed.SubagentType = typ
	if err := checkOutputSchemaShape(parsed.OutputSchema); err != nil {
		return nil, err
	}
	return &parsed, nil
}

func (t *SubagentTool) Check(ctx context.Context, args json.RawMessage) error {
	_, err := t.parse(args)
	return err
}

func (t *SubagentTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	parsed, err := t.parse(args)
	if err != nil {
		return nil, err
	}
	runner := t.getRunner()
	if runner == nil {
		return nil, RuntimeError("Subagent: no child runner is configured")
	}
	parentReg := t.getRegistry()
	if parentReg == nil {
		return nil, RuntimeError("Subagent: no tools are available to delegate to")
	}

	// Budgets: child_max = min(cfg subagent max, parent remaining). Parent
	// remaining is injected by the loop; without it the cfg max stands.
	maxChild := t.cfg.Tools.Subagent.ResolvedMaxChildIterations()
	if remain, ok := t.getParentRemaining(); ok && remain < maxChild {
		maxChild = remain
	}
	if maxChild < 1 {
		maxChild = 1
	}

	timeoutS := t.cfg.Tools.Subagent.ResolvedTimeoutS()
	callCtx := ctx
	var cancel context.CancelFunc
	if timeoutS > 0 {
		callCtx, cancel = context.WithTimeout(ctx, secondsToDuration(timeoutS))
		defer cancel()
	}

	// A schema that does not compile is the model's fault: fail before
	// any spawn, never a spawned run.
	wantSchema := hasOutputSchema(parsed.OutputSchema)
	if wantSchema {
		if _, err := compileOutputSchema(parsed.OutputSchema); err != nil {
			return nil, err
		}
	}

	// The concurrency cap counts blocking runs too: while this child runs
	// synchronously here, background children may be running alongside it.
	if !t.acquirePermit() {
		return nil, t.capExhaustedError("Subagent")
	}
	defer t.releasePermit(nil)

	prep, err := t.prepareSpawn(parentReg, parsed, maxChild)
	if err != nil {
		return nil, err
	}

	sess, result, err := runner.StartSubagent(callCtx, SubagentRunParams{
		System:      prep.system,
		Prompt:      parsed.Prompt,
		Description: parsed.Description,
		MaxIters:    maxChild,
		Registry:    prep.childReg,
		ToolCtx:     prep.childCtx,
	})
	if err != nil {
		// A child that exhausts its budget or its deadline returns its
		// partial summary as a labeled result, not an error — half a
		// transcript beats a bare error. Only a failure to run at all is
		// an error here.
		return nil, WrapAsRuntime(err)
	}

	if wantSchema {
		result = t.enforceSchema(callCtx, runner, sess, prep.capture, result)
	} else {
		result = t.nudgeEmptySummary(callCtx, runner, sess, result)
	}

	t.finishCollect(prep.childCtx)

	out := t.shapeResult(result, parsed)
	return out, nil
}

// spawnPrep is everything a spawn needs before the child loop starts:
// filtered registry, fresh context, session-local schema tool, system
// prompt. No LLM call happens here, so spawning stays cheap and sequential
// spawns still fan out.
type spawnPrep struct {
	childCfg   *config.Config
	childCtx   *ToolContext
	childReg   *Registry
	capture    *StructuredOutputCapture
	system     string
	wantSchema bool
	maxChild   int
	timeoutS   int
}

func (t *SubagentTool) prepareSpawn(parentReg *Registry, parsed *subagentArgs, maxChild int) (*spawnPrep, error) {
	wantSchema := hasOutputSchema(parsed.OutputSchema)
	childCfg := FloorChildConfig(t.cfg, maxChild)
	childCtx := t.newChildContext(maxChild)
	childReg := t.childScope(parentReg, childCfg, childCtx, parsed.SubagentType)

	// The session-local tool the child must file its result through. It
	// lives only in this child handle (and the child's Batch, which
	// dispatches through the same handle) and dies with the turn.
	var capture *StructuredOutputCapture
	if wantSchema {
		capture = &StructuredOutputCapture{}
		so, err := NewStructuredOutputTool(parsed.OutputSchema, capture)
		if err != nil {
			return nil, err
		}
		// Enabled directly, not via enableChildTool: the MCP/group
		// exclusion does not apply to a tool born in the child.
		childReg.Enable(so)
	}

	system := BuildSubagentSystemPrompt(t.cfg.Workspace.Root, parsed.SubagentType)
	if wantSchema {
		system += "\n# STRUCTURED RESULT\nYou must call the structured_output tool with your complete final result, matching the required schema, instead of ending your turn with prose.\n"
	}

	return &spawnPrep{
		childCfg:   childCfg,
		childCtx:   childCtx,
		childReg:   childReg,
		capture:    capture,
		system:     system,
		wantSchema: wantSchema,
		maxChild:   maxChild,
		timeoutS:   t.cfg.Tools.Subagent.ResolvedTimeoutS(),
	}, nil
}

// finishCollect performs the coarse change reporting a run owes when its
// result is collected: if the child changed files, the parent counts as
// mutating once, and the parent must re-read every file the child edited.
// Never shares the context object itself. Blocking runs do this inline;
// background children defer it to Subagent.output, so stats and staleness
// land once, on collect.
func (t *SubagentTool) finishCollect(childCtx *ToolContext) {
	if childCtx == nil {
		return
	}
	if childCtx.ChangedThisTurn() && t.toolCtx != nil {
		t.toolCtx.NoteChange()
	}
	// Staleness handoff: the parent must re-read every file the child
	// edited before editing it (spec/subagents-phase2.md section 1).
	// RequireReRead is a no-op for an empty set.
	if t.toolCtx != nil {
		t.toolCtx.RequireReRead(childCtx.EditedPaths())
	}
}

// enforceSchema runs the structured_output follow-up loop: up to maxNudges
// continuations demanding the missing call, carrying the previous
// attempt's failures. A filed value becomes the canonical JSON result;
// still missing afterwards, a labeled result with the partial summary.
func (t *SubagentTool) enforceSchema(ctx context.Context, runner SubRunner, sess *SubagentSession, capture *StructuredOutputCapture, result SubagentRunResult) SubagentRunResult {
	for i := 0; i < maxNudges; i++ {
		if called, _, _ := capture.Snapshot(); called {
			break
		}
		_, _, lastErrors := capture.Snapshot()
		cont, err := runner.ContinueSubagent(ctx, sess, schemaNudgePrompt(lastErrors))
		if err != nil {
			break
		}
		accumulateSubagentResult(&result, cont)
	}
	if called, value, _ := capture.Snapshot(); called {
		result.Text = string(value)
	} else {
		result.Text = fmt.Sprintf(schemaUnsatisfiedLabel, maxNudges) + "\n" + result.Text
	}
	return result
}

// nudgeEmptySummary re-prompts a silent child for the summary it owes, up
// to maxNudges times. Still empty afterwards, a labeled result.
func (t *SubagentTool) nudgeEmptySummary(ctx context.Context, runner SubRunner, sess *SubagentSession, result SubagentRunResult) SubagentRunResult {
	for i := 0; i < maxNudges && strings.TrimSpace(result.Text) == ""; i++ {
		cont, err := runner.ContinueSubagent(ctx, sess, emptyNudgePrompt)
		if err != nil {
			break
		}
		accumulateSubagentResult(&result, cont)
	}
	if strings.TrimSpace(result.Text) == "" {
		result.Text = fmt.Sprintf(summaryMissingLabel, maxNudges)
	}
	return result
}

// shapeResult truncates like other tools: keep the head N chars inline,
// spill the full text to tmp/ and note the path. The parent model reads the
// summary, never the dump.
func (t *SubagentTool) shapeResult(result SubagentRunResult, parsed *subagentArgs) *SubagentCallResult {
	maxChars := t.cfg.Tools.Subagent.ResolvedResultMaxChars()
	out := &SubagentCallResult{
		Iterations:       result.Iterations,
		Model:            result.Model,
		PromptTokens:     result.PromptTokens,
		CompletionTokens: result.CompletionTokens,
		BudgetExhausted:  result.BudgetExhausted,
		TimedOut:         result.TimedOut,
		Killed:           result.Killed,
		stats: &stats.AgentStats{
			TotalPromptTokens:     result.PromptTokens,
			TotalCompletionTokens: result.CompletionTokens,
			TotalCost:             result.Cost,
			Steps:                 result.Iterations,
		},
	}
	text := result.Text
	if result.BudgetExhausted {
		text = "[budget_exhausted: the child ran out of iterations; what follows is its partial summary]\n" + text
	}
	if result.TimedOut {
		text = "[timed_out: the child ran out of wall-clock; what follows is its partial summary]\n" + text
	}
	if result.Killed {
		text = "[killed: the child was stopped before finishing; what follows is its partial summary]\n" + text
	}
	if maxChars > 0 && len(text) > maxChars {
		head := text[:maxChars]
		path := ""
		if t.tempMgr != nil {
			if f, ferr := t.tempMgr.CreateTempFile(); ferr == nil {
				_, _ = f.WriteString(text)
				path = f.Name()
				_ = f.Close()
			} else if tmp, terr := os.CreateTemp(os.TempDir(), "subagent-*"); terr == nil {
				_, _ = tmp.WriteString(text)
				path = tmp.Name()
				_ = tmp.Close()
			}
		} else if tmp, terr := os.CreateTemp(os.TempDir(), "subagent-*"); terr == nil {
			_, _ = tmp.WriteString(text)
			path = tmp.Name()
			_ = tmp.Close()
		}
		out.Result = head
		out.Truncated = true
		out.OverflowPath = path
		if path != "" {
			out.Result += fmt.Sprintf("\n\n[truncated: full %d chars spilled to %s]", len(text), path)
		} else {
			out.Result += fmt.Sprintf("\n\n[truncated: full %d chars]", len(text))
		}
		return out
	}
	out.Result = text
	return out
}

// FloorChildConfig copies cfg for a subagent child loop. The iteration budget
// becomes the cap; read-before-edit is floored to cover the whole budget (a
// general child must never edit a file it has not read in its own loop);
// the preview handshake is forced off (Edit.confirm/Write.confirm are
// structurally absent from the child handle, so a pending_confirmation would
// wedge the child). SafetyConfirmations are copied, never shared. The agent
// package reuses this so the child tools and the child loop see the same
// config.
func FloorChildConfig(cfg *config.Config, maxIters int) *config.Config {
	if cfg == nil {
		return &config.Config{}
	}
	cp := *cfg
	cp.Agent.MaxIterations = maxIters
	if cp.Tools.Edit.ReadBeforeEditMsgs < maxIters {
		cp.Tools.Edit.ReadBeforeEditMsgs = maxIters
	}
	cp.Tools.Edit.PreviewMode = false
	if cfg.Tools.SafetyConfirmations != nil {
		m := make(map[string]config.SafetyConfirmation, len(cfg.Tools.SafetyConfirmations))
		for k, v := range cfg.Tools.SafetyConfirmations {
			m[k] = v
		}
		cp.Tools.SafetyConfirmations = m
	}
	return &cp
}

// subagentPathPromptDeny fails closed when a subagent child would otherwise
// prompt a person for path access. A nested prompt has no UI and would wedge
// the turn; the semantic error tells the child the parent must pre-approve
// the path. Modes that never prompt (allow, block, warn) pass through.
func subagentPathPromptDeny(toolCtx *ToolContext, cfg *config.Config, toolName, identifier string) error {
	if toolCtx != nil && toolCtx.IsSubagentChild() && cfg != nil && cfg.WouldPromptForPath(toolName, identifier) {
		return SemanticErrorf("subagents cannot prompt for path access (%s %s). The parent must pre-approve this path or work another way",
			toolName, identifier)
	}
	return nil
}

// newChildContext builds the fresh child context: its own read tracker
// (sized so the floored read-before-edit window is never trimmed away),
// pending-edit state and change counters, sharing only the grantor
// reference. Prompting paths resolve to deny (IsSubagentChild), and the
// context carries no inbox.
func (t *SubagentTool) newChildContext(maxIters int) *ToolContext {
	childCtx := NewToolContextWithReadWindow(maxIters)
	if t.toolCtx != nil {
		childCtx.SetGrantor(t.toolCtx.Grantor())
		if dir := t.toolCtx.SessionTmp(); dir != "" {
			childCtx.SetSessionTmp(dir)
		}
	}
	childCtx.SetSubagentChild(true)
	return childCtx
}

// childScope builds the filtered child view at Call time, never by mutating
// the shared registry. Exclusion is structural, not prompt text.
//
// Every tool that carries a ToolContext or reads tool config is rebuilt
// bound to the child config and context: sharing the parent's instances
// would record child reads into the parent tracker (letting parent reads
// satisfy the child's read-before-edit gate) and leak child edits into the
// parent's pending state.
func (t *SubagentTool) childScope(parent *Registry, childCfg *config.Config, childCtx *ToolContext, subagentType string) *Registry {
	child := NewRegistry()
	if childCfg == nil {
		childCfg = &config.Config{}
	}

	// Read-only tools, present in both types. Glob is read-only and included
	// though the base spec's list omits it: without it a child cannot
	// discover files (Read refuses directories, telling the model to use
	// Glob first).
	if parent.Get("Read") != nil {
		t.enableChildTool(child, NewReadFileTool(childCfg, childCtx))
	}
	if parent.Get("Search") != nil {
		g := NewSearchTool(childCfg, t.tempMgr)
		g.SetToolContext(childCtx)
		t.enableChildTool(child, g)
	}
	if parent.Get("Glob") != nil {
		g := NewGlobTool(childCfg)
		g.SetToolContext(childCtx)
		t.enableChildTool(child, g)
	}
	if parent.Get("Web.search") != nil {
		t.enableChildTool(child, NewWebSearchTool(childCfg))
	}
	if parent.Get("Web.fetch") != nil {
		t.enableChildTool(child, NewWebFetchTool(childCfg, t.tempMgr))
	}

	if subagentType == "general" {
		// Write-capable delegation: the same Edit mode the parent offers
		// (an Edit claimed into a tool group stays unreachable — documented
		// limitation, revisited only if a use case demands it), Write,
		// explicit line deletion, and full Shell bound to the child context
		// (the IsSubagentChild permission deny still applies to ask paths).
		switch e := parent.Get("Edit").(type) {
		case *UnifiedEditTool:
			t.enableChildTool(child, NewUnifiedEditTool(childCfg, childCtx))
		case *SearchReplaceEditTool:
			t.enableChildTool(child, NewSearchReplaceEditTool(childCfg, childCtx))
		case *PatchEditTool:
			t.enableChildTool(child, NewPatchEditTool(childCfg, childCtx))
		case nil:
			// Edit disabled: nothing to delegate.
		default:
			t.enableChildTool(child, e)
		}
		if w := parent.Get("Write"); w != nil {
			if _, ok := w.(*WriteFileTool); ok {
				t.enableChildTool(child, NewWriteFileTool(childCfg, childCtx))
			} else {
				t.enableChildTool(child, w)
			}
		}
		if d := parent.Get("DeleteLines"); d != nil {
			if _, ok := d.(*DeleteLinesTool); ok {
				t.enableChildTool(child, NewDeleteLinesTool(childCfg, childCtx))
			} else {
				t.enableChildTool(child, d)
			}
		}
		for _, name := range []string{"Shell", "Shell.advanced"} {
			if tool := parent.Get(name); tool != nil {
				t.enableChildTool(child, cloneShellForChild(tool, childCtx))
			}
		}
	} else {
		// Research: read-only Shell calls, gated per-call via ReadOnlyCall.
		for _, name := range []string{"Shell", "Shell.advanced"} {
			if tool := parent.Get(name); tool != nil {
				t.enableChildTool(child, &readOnlyShellGate{inner: cloneShellForChild(tool, childCtx), toolCtx: childCtx})
			}
		}
	}

	// Child Batch dispatches through the filtered handle, so nesting does
	// not smuggle an excluded tool back in.
	if parent.Get("Batch") != nil {
		b := NewBatchTool(childCfg)
		b.SetRegistry(child)
		b.SetToolContext(childCtx)
		t.enableChildTool(child, b)
	}
	return child
}

// enableChildTool copies one tool into the child handle, enforcing the
// two-layer MCP exclusion: GroupTools are never copied (their members stay
// behind the group), and any tool named mcp.* is skipped even if listed —
// a grouped MCP tool cannot re-enter through the copy. tools must not
// import mcp, so the name prefix is the mechanism and the test is the
// backstop.
func (t *SubagentTool) enableChildTool(child *Registry, tool Tool) {
	if tool == nil {
		return
	}
	if _, isGroup := tool.(*GroupTool); isGroup {
		return
	}
	if strings.HasPrefix(tool.Name(), "mcp.") {
		return
	}
	child.Enable(tool)
}

// cloneShellForChild rebinds a real Shell tool to the child context so its
// permission checks see the child (deny) rather than the parent. The parent
// instance is never mutated: a fresh wrapper shares config and timeout but
// carries the child ToolContext. Non-shell stubs pass through unchanged.
func cloneShellForChild(tool Tool, childCtx *ToolContext) Tool {
	switch s := tool.(type) {
	case *ShellTool:
		clone := &ShellTool{advanced: &ShellAdvancedTool{
			workspaceRoot: s.advanced.workspaceRoot,
			cfg:           s.advanced.cfg,
			timeout:       s.advanced.timeout,
			tempFileMgr:   s.advanced.tempFileMgr,
			safetyChecker: s.advanced.safetyChecker,
			toolCtx:       childCtx,
		}}
		return clone
	case *ShellAdvancedTool:
		clone := &ShellAdvancedTool{
			workspaceRoot: s.workspaceRoot,
			cfg:           s.cfg,
			timeout:       s.timeout,
			tempFileMgr:   s.tempFileMgr,
			safetyChecker: s.safetyChecker,
			toolCtx:       childCtx,
		}
		return clone
	default:
		return tool
	}
}

// ResearchChildScope is the test hook for the research allowlist: it builds
// the same filtered view Call uses, without running a child.
func ResearchChildScope(parent *Registry, parentCtx *ToolContext) (*Registry, *ToolContext) {
	return TestChildScope(parent, parentCtx, &config.Config{}, config.DefaultSubagentMaxChildIterations, "research")
}

// GeneralChildScope is the test hook for the general allowlist.
func GeneralChildScope(parent *Registry, parentCtx *ToolContext, cfg *config.Config, maxIters int) (*Registry, *ToolContext) {
	return TestChildScope(parent, parentCtx, cfg, maxIters, "general")
}

// TestChildScope builds the same filtered view Call uses for the given type,
// without running a child.
func TestChildScope(parent *Registry, parentCtx *ToolContext, cfg *config.Config, maxIters int, subagentType string) (*Registry, *ToolContext) {
	t := &SubagentTool{toolCtx: parentCtx, cfg: cfg}
	childCtx := t.newChildContext(maxIters)
	return t.childScope(parent, FloorChildConfig(cfg, maxIters), childCtx, subagentType), childCtx
}

// readOnlyShellGate wraps Shell/Shell.advanced inside a research child: a
// non-read-only invocation fails closed with a semantic error the child can
// reason about, even if the grantor would allow it.
type readOnlyShellGate struct {
	inner   Tool
	toolCtx *ToolContext
}

func (g *readOnlyShellGate) Name() string { return g.inner.Name() }

func (g *readOnlyShellGate) Description() string { return g.inner.Description() }

func (g *readOnlyShellGate) JSONSchema() map[string]any { return g.inner.JSONSchema() }

func (g *readOnlyShellGate) PromptCategory() string { return g.inner.PromptCategory() }

func (g *readOnlyShellGate) PromptOrder() int { return g.inner.PromptOrder() }

func (g *readOnlyShellGate) PromptTemplateName() string { return g.inner.PromptTemplateName() }

func (g *readOnlyShellGate) PromptSection() string { return g.inner.PromptSection() }

func (g *readOnlyShellGate) ParallelSafe() bool { return false }

func (g *readOnlyShellGate) ReadOnlyCall(args json.RawMessage) bool {
	if rc, ok := g.inner.(ReadOnlyCaller); ok {
		return rc.ReadOnlyCall(args)
	}
	return false
}

func (g *readOnlyShellGate) Check(ctx context.Context, args json.RawMessage) error {
	if g.ReadOnlyCall(args) {
		return g.inner.Check(ctx, args)
	}
	return SemanticErrorf("Subagent (research): this Shell call may change the workspace and is not allowed in a read-only child. Use Read, Search, Glob, Web.search, Web.fetch, or a read-only command (ls, git status, read-only rg).")
}

func (g *readOnlyShellGate) Call(ctx context.Context, args json.RawMessage) (any, error) {
	if !g.ReadOnlyCall(args) {
		return nil, SemanticErrorf("Subagent (research): this Shell call may change the workspace and is not allowed in a read-only child. Use Read, Search, Glob, Web.search, Web.fetch, or a read-only command (ls, git status, read-only rg).")
	}
	return g.inner.Call(ctx, args)
}

// BuildSubagentSystemPrompt assembles the child system prompt: role line,
// environment, output-discipline block and batch-first guidance. What the
// child must NOT get: the parent's role/tasks/guidelines sections,
// project-instructions after blocks, or anything that tells it it can ask
// the user. The canonical templates live in
// internal/prompt/prompts/subagents/; this builder mirrors them without
// importing internal/prompt (which imports tools).
func BuildSubagentSystemPrompt(workspaceRoot, subagentType string) string {
	role := "You are a read-only research subagent. Explore the codebase and answer with a concise summary."
	if subagentType == "general" {
		role = "You are a coding subagent that may edit files. Complete the task and answer with a concise summary."
	}
	var sb strings.Builder
	sb.WriteString("# SUBAGENT ROLE\n")
	sb.WriteString(role + "\n\n")
	sb.WriteString(EnvironmentBlock(workspaceRoot))
	sb.WriteString("# OUTPUT DISCIPLINE\n")
	sb.WriteString("Your entire response is injected into the parent agent's context. " +
		"Return a concise summary with `file:line` references. " +
		"Never dump large code blocks. Never create summary/report files.")
	if subagentType == "general" {
		sb.WriteString(" Only touch files that are part of the task.")
	}
	sb.WriteString("\n\n")
	sb.WriteString("# TOOL GUIDANCE\n")
	if subagentType == "general" {
		sb.WriteString("You have Read, Edit, Write, DeleteLines, Search, Glob, Web.search, Web.fetch and full Shell. " +
			"Read a file before editing it, and re-read it after anyone else changed it. " +
			"Prefer Batch for 2+ independent reads/greps in one request. " +
			"You cannot ask the user anything, spawn subagents, or run background processes.\n")
	} else {
		sb.WriteString("You have Read, Search, Glob, Web.search, Web.fetch and read-only Shell. " +
			"Prefer Batch for 2+ independent reads/greps in one request. " +
			"You cannot ask the user anything, spawn subagents, or run background processes.\n")
	}
	return sb.String()
}

func secondsToDuration(s int) time.Duration {
	return time.Duration(s) * time.Second
}

// EnvironmentBlock describes where the child runs, mirroring
// internal/prompt.Environment without importing it (which would cycle
// tools->prompt->tools). Keep the working directory, platform and date;
// git and directory listing stay parent-only.
func EnvironmentBlock(workspaceRoot string) string {
	var sb strings.Builder
	sb.WriteString("# ENVIRONMENT\n")
	sb.WriteString(fmt.Sprintf("Working directory: %s\n", workspaceRoot))
	sb.WriteString(fmt.Sprintf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH))
	sb.WriteString(fmt.Sprintf("Today: %s\n", time.Now().Format("2006-01-02")))
	sb.WriteString("\n")
	return sb.String()
}

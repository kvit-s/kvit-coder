package tools

// Subagent tool — mechanism (spawn a loop, relay usage, truncate the result).
//
// Policy (retries, nudges, concurrency cap) lives as config + constants at
// the top of this file, mirroring maki's primitives/policy split in Go form:
// this file owns the mechanism (schema, allowlists, result shaping) while
// internal/agent/subagent.go owns loop setup/teardown.
//
// Phase 1 is read-only delegation (research). Write-capable delegation
// (general) and output_schema enforcement are phase 2; spawn/collect
// parallelism is phase 3. See spec/subagents.md.

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
	// maxNudges and maxSchemaErrors belong to the phase-2 output_schema
	// path. They are recorded here so phase 2 does not relitigate them.
	maxNudges       = 2
	maxSchemaErrors = 3
)

// SubagentDenyMessage is the semantic error a research child gets when a call
// would otherwise prompt a person. A nested prompt has no UI and would wedge
// the turn; failing closed pushes authors toward pre-approving the commands a
// delegated task needs.
const subagentPromptDenyHint = "subagents cannot prompt"

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
type SubagentTool struct {
	mu       sync.Mutex
	cfg      *config.Config
	toolCtx  *ToolContext
	registry *Registry
	tempMgr  *TempFileManager
	runner   SubRunner

	parentRemaining int
	hasRemaining    bool
}

// NewSubagentTool builds the tool. The parent registry it filters at Call
// time is set afterwards with SetRegistry, because the registry is what
// holds this tool. The child-loop driver is set with SetRunner after the
// agent Runner exists (turn setup), breaking the tools->agent import cycle.
func NewSubagentTool(cfg *config.Config, toolCtx *ToolContext, tempMgr *TempFileManager) *SubagentTool {
	return &SubagentTool{cfg: cfg, toolCtx: toolCtx, tempMgr: tempMgr}
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
	return "Delegate a research task to a subagent with its own context and a read-only tool set. " +
		"The child starts fresh: inline every file path, symbol name and constraint it needs. " +
		"Returns only its final summary text."
}

func (t *SubagentTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":     "object",
		"required": []string{"description", "prompt"},
		"properties": map[string]any{
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
				"description": `"research" (read-only, default) or "general" (may edit files). Phase 1 is research-only; "general" is rejected until phase 2.`,
			},
			"output_schema": map[string]any{
				"type":        "object",
				"description": "Optional JSON Schema (type object) the child's final result must match. Phase 1 accepts and ignores it with a note; enforcement is phase 2.",
			},
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

func (t *SubagentTool) PromptSection() string {
	return `### Subagent - Delegate research with its own context

Subagent({"description": "find auth flow", "prompt": "How does login work? Start at internal/auth/..."})
Subagent({"description": "trace retries", "prompt": "...", "subagent_type": "research"})

One blocking call per delegation. The child starts fresh with no parent
history: inline every file path, symbol name and constraint it needs. It sees
a read-only tool set (Read, Search, Glob, Web.search, Web.fetch, read-only
Shell) and returns only its final summary text — never dump code, never write
report files.

Independent subagents still save context tokens sequentially; wall-clock
parallelism is a later change. Several Subagents in one Batch run their spawn
calls in order. Phase 1 is research-only: "general" is rejected, and
output_schema is accepted but ignored with a note.`
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
	case "research":
		// Phase 1: the only supported type.
	case "general":
		return nil, SemanticErrorf("Subagent: subagent_type \"general\" (may edit files) is not enabled yet (phase 2). Use \"research\" for read-only delegation.")
	default:
		return nil, SemanticErrorf("Subagent: unknown subagent_type %q. Use \"research\" (read-only, default).", parsed.SubagentType)
	}
	parsed.SubagentType = typ
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

	childReg, childCtx := t.childScope(parentReg)

	system := BuildSubagentSystemPrompt(t.cfg.Workspace.Root, parsed.SubagentType)

	result, err := runner.RunSubagent(callCtx, SubagentRunParams{
		System:      system,
		Prompt:      parsed.Prompt,
		Description: parsed.Description,
		MaxIters:    maxChild,
		Registry:    childReg,
		ToolCtx:     childCtx,
	})
	if err != nil {
		// A child that exhausts its budget or its deadline returns its
		// partial summary as a labeled result, not an error — half a
		// transcript beats a bare error. Only a failure to run at all is
		// an error here.
		return nil, WrapAsRuntime(err)
	}

	// Coarser-grain change reporting, mirroring Batch: if the child changed
	// files, the parent counts as mutating once. Never share the context
	// object itself — the child's counters must not flip the parent's
	// report mode mid-turn for work the parent didn't do. Phase 1 children
	// are read-only, so this stays clear; it exists for phase 2.
	if childCtx.ChangedThisTurn() && t.toolCtx != nil {
		t.toolCtx.NoteChange()
	}

	out := t.shapeResult(result, parsed)
	return out, nil
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
	if len(parsed.OutputSchema) > 0 && strings.TrimSpace(string(parsed.OutputSchema)) != "" &&
		strings.TrimSpace(string(parsed.OutputSchema)) != "null" {
		// Phase 1 accepts output_schema but ignores it: the validator +
		// structured_output local tool + nudge loop is phase 2.
		text += "\n\n[note: output_schema was accepted but ignored (phase 1 has no schema enforcement).]"
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

// childScope builds the filtered child view at Call time, never by mutating
// the shared registry. Exclusion is structural, not prompt text.
func (t *SubagentTool) childScope(parent *Registry) (*Registry, *ToolContext) {
	child := NewRegistry()
	// Fresh child context sharing only the grantor reference: pending-edit
	// state and change counters are per-loop. Prompting paths resolve to
	// deny (IsSubagentChild), and the context carries no inbox.
	childCtx := NewToolContext()
	if t.toolCtx != nil {
		childCtx.SetGrantor(t.toolCtx.Grantor())
		if dir := t.toolCtx.SessionTmp(); dir != "" {
			childCtx.SetSessionTmp(dir)
		}
	}
	childCtx.SetSubagentChild(true)
	// Research allowlist: read-only tools plus read-only Shell calls (gated
	// per-call via ReadOnlyCall) plus a Batch scoped to the same allowlist so
	// exclusion cannot be bypassed by nesting. Glob is read-only and included
	// though the spec's list omits it: without it a child cannot discover
	// files (Read refuses directories, telling the model to use Glob first).
	for _, name := range []string{"Read", "Search", "Glob", "Web.search", "Web.fetch", "Shell", "Shell.advanced"} {
		if tool := parent.Get(name); tool != nil {
			if name == "Shell" || name == "Shell.advanced" {
				child.Enable(&readOnlyShellGate{inner: cloneShellForChild(tool, childCtx), toolCtx: childCtx})
				continue
			}
			child.Enable(tool)
		}
	}
	// Child Batch dispatches through the filtered handle, so nesting does
	// not smuggle an excluded tool back in.
	if parent.Get("Batch") != nil {
		cfg := t.cfg
		if cfg == nil {
			cfg = &config.Config{}
		}
		b := NewBatchTool(cfg)
		b.SetRegistry(child)
		b.SetToolContext(childCtx)
		child.Enable(b)
	}
	return child, childCtx
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

// ResearchChildScope is the test hook for the allowlist: it builds the same
// filtered view Call uses, without running a child.
func ResearchChildScope(parent *Registry, parentCtx *ToolContext) (*Registry, *ToolContext) {
	t := &SubagentTool{toolCtx: parentCtx, cfg: &config.Config{}}
	return t.childScope(parent)
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
		"Never dump large code blocks. Never create summary/report files. ")
	if subagentType == "general" {
		sb.WriteString("Only touch files that are part of the task. ")
	}
	sb.WriteString("\n\n")
	sb.WriteString("# TOOL GUIDANCE\n")
	sb.WriteString("You have Read, Search, Glob, Web.search, Web.fetch and read-only Shell. " +
		"Prefer Batch for 2+ independent reads/greps in one request. " +
		"You cannot ask the user anything, spawn subagents, or run background processes.\n")
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

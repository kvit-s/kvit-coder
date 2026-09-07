package tools

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/procs"
)

// The tools in this file cover work that outlives the turn that started it.
// Shell stays what it was — run a command and wait for it — and these are the
// separate thing: start something, look at it later, wait for it to do
// something, stop it.

// procsToolBase is the shared half of every process tool.
type procsToolBase struct {
	cfg      *config.Config
	registry *procs.Registry
	toolCtx  *ToolContext
}

func (t *procsToolBase) workDir(dir string) string {
	if dir == "" {
		return t.cfg.Workspace.Root
	}
	resolved, outside, err := config.NormalizeAndValidatePath(t.cfg.Workspace.Root, dir)
	if err != nil || outside {
		return t.cfg.Workspace.Root
	}
	return resolved
}

func (t *procsToolBase) PromptCategory() string     { return "shell" }
func (t *procsToolBase) PromptTemplateName() string { return "" }

// --- Shell.start -------------------------------------------------------------

// ShellStartTool starts a command and returns immediately.
type ShellStartTool struct{ procsToolBase }

// NewShellStartTool builds the tool.
func NewShellStartTool(cfg *config.Config, registry *procs.Registry, toolCtx *ToolContext) *ShellStartTool {
	return &ShellStartTool{procsToolBase{cfg: cfg, registry: registry, toolCtx: toolCtx}}
}

func (t *ShellStartTool) Name() string { return "Shell.start" }
func (t *ShellStartTool) Description() string {
	return "Start a long-running command and return immediately with an id. The command keeps " +
		"running after this turn ends, so a dev server or a test run started here is still going " +
		"in the next turn. Use Shell for anything that finishes on its own."
}
func (t *ShellStartTool) PromptOrder() int { return 12 }
func (t *ShellStartTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command":     map[string]any{"type": "string", "description": "The command to start."},
			"working_dir": map[string]any{"type": "string", "description": "Directory to run in (default: workspace root)."},
			"name":        map[string]any{"type": "string", "description": "A few words naming what this is, shown when it is listed or reported."},
		},
		"required": []string{"command"},
	}
}
func (t *ShellStartTool) PromptSection() string {
	return `### Shell.start - Start something that keeps running

Shell.start({"command": "npm run dev", "name": "dev server"})
  → {"id": "bg1", ...}

The command survives this turn: it is still running in the next one. Use
Shell for anything that finishes on its own, and Observe.wait to wait for
this one to do something. Read its output with Shell.output, stop it with
Shell.kill, and see everything running with Shell.list.`
}

func (t *ShellStartTool) Check(ctx context.Context, args json.RawMessage) error {
	var p struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return SemanticErrorf("Shell.start: arguments are not valid JSON: %v", err)
	}
	if strings.TrimSpace(p.Command) == "" {
		return SemanticErrorf("Shell.start: 'command' is required.")
	}
	return nil
}

func (t *ShellStartTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var p struct {
		Command    string `json:"command"`
		WorkingDir string `json:"working_dir"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, SemanticErrorf("Shell.start: arguments are not valid JSON: %v", err)
	}

	id, err := t.registry.Start(p.Command, t.workDir(p.WorkingDir), p.Name)
	if err != nil {
		return nil, RuntimeError(err.Error())
	}
	info, _ := t.registry.Status(id)
	return map[string]any{
		"id":      id,
		"pid":     info.PID,
		"state":   string(info.State),
		"command": p.Command,
		"hint":    "It is running. Use Observe.wait to wait for it, Shell.output to read what it has printed, Shell.kill to stop it.",
	}, nil
}

// --- Shell.output ------------------------------------------------------------

// ShellOutputTool reads what a background process has written.
type ShellOutputTool struct{ procsToolBase }

// NewShellOutputTool builds the tool.
func NewShellOutputTool(cfg *config.Config, registry *procs.Registry, toolCtx *ToolContext) *ShellOutputTool {
	return &ShellOutputTool{procsToolBase{cfg: cfg, registry: registry, toolCtx: toolCtx}}
}

func (t *ShellOutputTool) Name() string { return "Shell.output" }
func (t *ShellOutputTool) Description() string {
	return "Read what a background process has written. Pass the cursor from the previous call " +
		"to see only what is new since then."
}
func (t *ShellOutputTool) PromptOrder() int { return 13 }
func (t *ShellOutputTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":     map[string]any{"type": "string", "description": "The process id from Shell.start."},
			"cursor": map[string]any{"type": "integer", "description": "Byte offset returned by the previous call. Omit to read from the start."},
		},
		"required": []string{"id"},
	}
}
func (t *ShellOutputTool) PromptSection() string { return "" }

func (t *ShellOutputTool) Check(ctx context.Context, args json.RawMessage) error {
	return checkHasID("Shell.output", args)
}

func (t *ShellOutputTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var p struct {
		ID     string `json:"id"`
		Cursor int64  `json:"cursor"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, SemanticErrorf("Shell.output: arguments are not valid JSON: %v", err)
	}

	info, err := t.registry.Status(p.ID)
	if err != nil {
		return nil, SemanticError(err.Error())
	}
	text, cursor, more, err := t.registry.Output(p.ID, p.Cursor)
	if err != nil {
		return nil, RuntimeError(err.Error())
	}
	return map[string]any{
		"id":         p.ID,
		"state":      string(info.State),
		"exit_code":  info.ExitCode,
		"output":     text,
		"cursor":     cursor,
		"new_output": more,
	}, nil
}

// --- Shell.status ------------------------------------------------------------

// ShellStatusTool reports whether a background process is still running.
type ShellStatusTool struct{ procsToolBase }

// NewShellStatusTool builds the tool.
func NewShellStatusTool(cfg *config.Config, registry *procs.Registry, toolCtx *ToolContext) *ShellStatusTool {
	return &ShellStatusTool{procsToolBase{cfg: cfg, registry: registry, toolCtx: toolCtx}}
}

func (t *ShellStatusTool) Name() string { return "Shell.status" }
func (t *ShellStatusTool) Description() string {
	return "Report whether a background process is still running, and its exit status if not."
}
func (t *ShellStatusTool) PromptOrder() int { return 14 }
func (t *ShellStatusTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "The process id from Shell.start."},
		},
		"required": []string{"id"},
	}
}
func (t *ShellStatusTool) PromptSection() string { return "" }

func (t *ShellStatusTool) Check(ctx context.Context, args json.RawMessage) error {
	return checkHasID("Shell.status", args)
}

func (t *ShellStatusTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, SemanticErrorf("Shell.status: arguments are not valid JSON: %v", err)
	}
	info, err := t.registry.Status(p.ID)
	if err != nil {
		return nil, SemanticError(err.Error())
	}
	return infoResult(info), nil
}

// --- Shell.list --------------------------------------------------------------

// ShellListTool lists everything this session has started.
type ShellListTool struct{ procsToolBase }

// NewShellListTool builds the tool.
func NewShellListTool(cfg *config.Config, registry *procs.Registry, toolCtx *ToolContext) *ShellListTool {
	return &ShellListTool{procsToolBase{cfg: cfg, registry: registry, toolCtx: toolCtx}}
}

func (t *ShellListTool) Name() string { return "Shell.list" }
func (t *ShellListTool) Description() string {
	return "List every background process this session has started, running or finished."
}
func (t *ShellListTool) PromptOrder() int { return 15 }
func (t *ShellListTool) JSONSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (t *ShellListTool) PromptSection() string { return "" }

func (t *ShellListTool) Check(ctx context.Context, args json.RawMessage) error { return nil }

func (t *ShellListTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	all := t.registry.List()
	out := make([]map[string]any, 0, len(all))
	for _, info := range all {
		out = append(out, infoResult(info))
	}
	return map[string]any{"processes": out, "count": len(out)}, nil
}

// --- Shell.kill --------------------------------------------------------------

// ShellKillTool stops a background process.
type ShellKillTool struct{ procsToolBase }

// NewShellKillTool builds the tool.
func NewShellKillTool(cfg *config.Config, registry *procs.Registry, toolCtx *ToolContext) *ShellKillTool {
	return &ShellKillTool{procsToolBase{cfg: cfg, registry: registry, toolCtx: toolCtx}}
}

func (t *ShellKillTool) Name() string { return "Shell.kill" }
func (t *ShellKillTool) Description() string {
	return "Stop a background process and everything it started."
}
func (t *ShellKillTool) PromptOrder() int { return 16 }
func (t *ShellKillTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id": map[string]any{"type": "string", "description": "The process id from Shell.start."},
		},
		"required": []string{"id"},
	}
}
func (t *ShellKillTool) PromptSection() string { return "" }

func (t *ShellKillTool) Check(ctx context.Context, args json.RawMessage) error {
	return checkHasID("Shell.kill", args)
}

func (t *ShellKillTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, SemanticErrorf("Shell.kill: arguments are not valid JSON: %v", err)
	}
	if err := t.registry.Kill(p.ID); err != nil {
		return nil, RuntimeError(err.Error())
	}
	info, _ := t.registry.Status(p.ID)
	return infoResult(info), nil
}

// --- Observe.wait ------------------------------------------------------------

// ObserveWaitTool blocks until a background process does something worth
// coming back for.
type ObserveWaitTool struct {
	procsToolBase
	// now and sleep are the clock, so a test can drive the wait.
	poll func(time.Duration) <-chan time.Time
}

// NewObserveWaitTool builds the tool.
func NewObserveWaitTool(cfg *config.Config, registry *procs.Registry, toolCtx *ToolContext) *ObserveWaitTool {
	return &ObserveWaitTool{
		procsToolBase: procsToolBase{cfg: cfg, registry: registry, toolCtx: toolCtx},
		poll:          time.After,
	}
}

func (t *ObserveWaitTool) Name() string { return "Observe.wait" }
func (t *ObserveWaitTool) Description() string {
	return "Wait for a background process to exit, or to print something matching a pattern. " +
		"Waiting here costs one tool call however long it takes; polling with Shell.output in a " +
		"loop costs a full round of thinking every time."
}
func (t *ObserveWaitTool) PromptOrder() int { return 17 }
func (t *ObserveWaitTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":       map[string]any{"type": "string", "description": "The process id from Shell.start."},
			"until":    map[string]any{"type": "string", "description": "A regular expression. Return as soon as the process prints something matching it."},
			"report":   map[string]any{"type": "string", "enum": []string{"changed", "match", "exit"}, "description": "What is worth returning for: 'exit' (default) waits for the process to end, 'match' for the 'until' pattern, 'changed' for any new output."},
			"poll":     map[string]any{"type": "integer", "description": "Seconds between checks (default 2)."},
			"max_wait": map[string]any{"type": "integer", "description": "Give up after this many seconds (default 300)."},
			"cursor":   map[string]any{"type": "integer", "description": "Byte offset to read output from, from a previous call."},
		},
		"required": []string{"id"},
	}
}
func (t *ObserveWaitTool) PromptSection() string {
	return `### Observe.wait - Wait for a background process

Observe.wait({"id": "bg1", "report": "exit"})
Observe.wait({"id": "bg1", "until": "Listening on", "report": "match"})

Blocks until the process ends, or prints something matching 'until', or
max_wait seconds pass. One tool call however long it takes, where checking
with Shell.output in a loop costs a full round of thinking each time. It
also returns early if someone types something, so you are not left waiting
when the answer has changed.`
}

// SelfTimeout opts out of the loop's blanket 15-second tool timeout: waiting
// is what this tool is for. It honours its own max_wait instead.
func (t *ObserveWaitTool) SelfTimeout() bool { return true }

func (t *ObserveWaitTool) Check(ctx context.Context, args json.RawMessage) error {
	var p struct {
		ID    string `json:"id"`
		Until string `json:"until"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return SemanticErrorf("Observe.wait: arguments are not valid JSON: %v", err)
	}
	if strings.TrimSpace(p.ID) == "" {
		return SemanticErrorf("Observe.wait: 'id' is required.")
	}
	if p.Until != "" {
		if _, err := regexp.Compile(p.Until); err != nil {
			return SemanticErrorf("Observe.wait: 'until' is not a valid regular expression: %v", err)
		}
	}
	return nil
}

func (t *ObserveWaitTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var p struct {
		ID      string `json:"id"`
		Until   string `json:"until"`
		Report  string `json:"report"`
		Poll    int    `json:"poll"`
		MaxWait int    `json:"max_wait"`
		Cursor  int64  `json:"cursor"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, SemanticErrorf("Observe.wait: arguments are not valid JSON: %v", err)
	}
	if p.Report == "" {
		if p.Until != "" {
			p.Report = "match"
		} else {
			p.Report = "exit"
		}
	}
	if p.Poll <= 0 {
		p.Poll = 2
	}
	if p.MaxWait <= 0 {
		p.MaxWait = 300
	}

	var until *regexp.Regexp
	if p.Until != "" {
		var err error
		if until, err = regexp.Compile(p.Until); err != nil {
			return nil, SemanticErrorf("Observe.wait: 'until' is not a valid regular expression: %v", err)
		}
	}

	if _, err := t.registry.Status(p.ID); err != nil {
		return nil, SemanticError(err.Error())
	}

	start := time.Now()
	deadline := start.Add(time.Duration(p.MaxWait) * time.Second)
	interval := time.Duration(p.Poll) * time.Second

	var collected strings.Builder
	cursor := p.Cursor
	ticks, quiet := 0, 0
	box := t.toolCtx.Inbox()

	finish := func(reason string) (any, error) {
		info, _ := t.registry.Status(p.ID)
		return map[string]any{
			"id":           p.ID,
			"returned_for": reason,
			"state":        string(info.State),
			"exit_code":    info.ExitCode,
			"output":       collected.String(),
			"cursor":       cursor,
			"waited_secs":  int(time.Since(start).Seconds()),
			"checks":       ticks,
			"quiet_checks": quiet,
		}, nil
	}

	for {
		info, err := t.registry.Status(p.ID)
		if err != nil {
			return nil, SemanticError(err.Error())
		}
		text, next, more, outErr := t.registry.Output(p.ID, cursor)
		if outErr == nil && more {
			collected.WriteString(text)
			cursor = next
		}
		ticks++
		if !more {
			quiet++
		}

		if !info.Running() {
			return finish("exit")
		}
		if until != nil && until.MatchString(collected.String()) {
			return finish("match")
		}
		if p.Report == "changed" && more {
			return finish("changed")
		}

		remaining := time.Until(deadline)
		if remaining <= 0 {
			return finish("max_wait")
		}
		wait := min(interval, remaining)

		var signal <-chan struct{}
		if box != nil {
			signal = box.Signal()
		}
		select {
		case <-ctx.Done():
			return finish("cancelled")
		case <-signal:
			// Someone typed something. Come back now rather than leave them
			// waiting for a process whose result may no longer matter.
			return finish("steering")
		case <-t.poll(wait):
		}
	}
}

// --- Observe.add -------------------------------------------------------------

// ObserveAddTool records a command to run again on a schedule.
type ObserveAddTool struct{ procsToolBase }

// NewObserveAddTool builds the tool.
func NewObserveAddTool(cfg *config.Config, registry *procs.Registry, toolCtx *ToolContext) *ObserveAddTool {
	return &ObserveAddTool{procsToolBase{cfg: cfg, registry: registry, toolCtx: toolCtx}}
}

func (t *ObserveAddTool) Name() string { return "Observe.add" }
func (t *ObserveAddTool) Description() string {
	return "Run a command every few seconds from now on, and be told when its output changes. " +
		"Use it to keep an eye on something while you work on something else."
}
func (t *ObserveAddTool) PromptOrder() int { return 18 }
func (t *ObserveAddTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"command":     map[string]any{"type": "string", "description": "The command to run each time."},
			"every":       map[string]any{"type": "integer", "description": "Seconds between runs (default 30)."},
			"report":      map[string]any{"type": "string", "enum": []string{"changed", "always"}, "description": "When to interrupt you: 'changed' (default) or 'always'."},
			"working_dir": map[string]any{"type": "string", "description": "Directory to run in (default: workspace root)."},
			"name":        map[string]any{"type": "string", "description": "A few words naming what this watches."},
		},
		"required": []string{"command"},
	}
}
func (t *ObserveAddTool) PromptSection() string { return "" }

func (t *ObserveAddTool) Check(ctx context.Context, args json.RawMessage) error {
	var p struct {
		Command string `json:"command"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return SemanticErrorf("Observe.add: arguments are not valid JSON: %v", err)
	}
	if strings.TrimSpace(p.Command) == "" {
		return SemanticErrorf("Observe.add: 'command' is required.")
	}
	return nil
}

func (t *ObserveAddTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var p struct {
		Command    string `json:"command"`
		Every      int    `json:"every"`
		Report     string `json:"report"`
		WorkingDir string `json:"working_dir"`
		Name       string `json:"name"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return nil, SemanticErrorf("Observe.add: arguments are not valid JSON: %v", err)
	}
	if p.Every <= 0 {
		p.Every = 30
	}

	id, err := t.registry.StartPeriodic(p.Command, t.workDir(p.WorkingDir), p.Name, p.Every, p.Report)
	if err != nil {
		return nil, RuntimeError(err.Error())
	}
	return map[string]any{
		"id":     id,
		"every":  p.Every,
		"report": p.Report,
		"hint":   "It runs from now on. You will be told when there is something to see. Stop it with Shell.kill.",
	}, nil
}

// --- shared helpers -----------------------------------------------------------

func checkHasID(tool string, args json.RawMessage) error {
	var p struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(args, &p); err != nil {
		return SemanticErrorf("%s: arguments are not valid JSON: %v", tool, err)
	}
	if strings.TrimSpace(p.ID) == "" {
		return SemanticErrorf("%s: 'id' is required. Shell.list shows the ids of everything running.", tool)
	}
	return nil
}

func infoResult(info procs.Info) map[string]any {
	out := map[string]any{
		"id":       info.ID,
		"state":    string(info.State),
		"command":  info.Command,
		"pid":      info.PID,
		"started":  info.Started.Format(time.RFC3339),
		"cwd":      info.Cwd,
		"periodic": info.Every > 0,
	}
	if info.Name != "" {
		out["name"] = info.Name
	}
	if !info.Running() {
		out["exit_code"] = info.ExitCode
		if !info.Ended.IsZero() {
			out["ended"] = info.Ended.Format(time.RFC3339)
		}
	}
	return out
}

// ProcessEvents turns what the registry has to report into inbox messages, so
// a process ending reaches the model the same way steering does.
func ProcessEvents(registry *procs.Registry, box *inbox.Inbox) {
	if registry == nil || box == nil {
		return
	}
	registry.Reconcile()
	for _, text := range registry.Events() {
		box.Push(inbox.Message{Kind: inbox.KindProcessEvent, Text: text})
	}
}

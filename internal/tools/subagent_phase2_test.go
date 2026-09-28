package tools

// Phase 2a tests (spec/subagents-phase2.md section 7): general type,
// staleness handoff, allowlist deltas, prompt-deny audit, report sanity.
// Each test asserts one thing.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/permissions"
	"github.com/kvit-s/kvit-coder/internal/report"
)

// realParentRegistry builds a parent registry with real tools (not stubs) so
// staleness, preview and gating behavior is exercised end to end.
func realParentRegistry(t *testing.T, cfg *config.Config, toolCtx *ToolContext) *Registry {
	t.Helper()
	if cfg.Workspace.Root == "" {
		cfg.Workspace.Root = t.TempDir()
	}
	if cfg.Tools.SafetyConfirmations == nil {
		cfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{}
	}
	tempMgr := NewTempFileManager(t.TempDir())
	sc := SetupConfig{Cfg: cfg, TempFileMgr: tempMgr, ToolCtx: toolCtx}
	cfg.Tools.Read.Enabled = true
	cfg.Tools.Edit.Enabled = true
	cfg.Tools.Search.Enabled = true
	cfg.Tools.Glob.Enabled = true
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Batch.Enabled = true
	if cfg.Tools.Shell.DefaultTimeout == 0 {
		cfg.Tools.Shell.DefaultTimeout = 120
	}
	if cfg.Tools.Shell.MaxTimeout == 0 {
		cfg.Tools.Shell.MaxTimeout = 600
	}
	return SetupRegistry(sc)
}

func writeTmpFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", p, err)
	}
	return p
}

func toolErrorCode(err error) string {
	te, ok := err.(*ToolError)
	if !ok {
		return ""
	}
	if code, ok := te.Details["error"].(string); ok {
		return code
	}
	return ""
}

func TestFloorChildConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Agent.MaxIterations = 1000
	cfg.Tools.Edit.ReadBeforeEditMsgs = 0
	cfg.Tools.Edit.PreviewMode = true
	cfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{
		"read:/x": {},
	}
	child := FloorChildConfig(cfg, 7)
	if child.Agent.MaxIterations != 7 {
		t.Errorf("MaxIterations %d, want 7", child.Agent.MaxIterations)
	}
	if child.Tools.Edit.ReadBeforeEditMsgs != 7 {
		t.Errorf("ReadBeforeEditMsgs %d, want floored to 7", child.Tools.Edit.ReadBeforeEditMsgs)
	}
	if child.Tools.Edit.PreviewMode {
		t.Error("PreviewMode still on: a child edit would wedge on pending_confirmation")
	}
	// The copy owns its map: mutating it must not touch the parent.
	child.Tools.SafetyConfirmations["read:/y"] = config.SafetyConfirmation{}
	if _, ok := cfg.Tools.SafetyConfirmations["read:/y"]; ok {
		t.Error("SafetyConfirmations shared with parent")
	}
	// The parent is untouched.
	if cfg.Tools.Edit.ReadBeforeEditMsgs != 0 || !cfg.Tools.Edit.PreviewMode || cfg.Agent.MaxIterations != 1000 {
		t.Error("parent config mutated by FloorChildConfig")
	}
	// A larger parent window is kept, never lowered.
	cfg.Tools.Edit.ReadBeforeEditMsgs = 100
	if got := FloorChildConfig(cfg, 7).Tools.Edit.ReadBeforeEditMsgs; got != 100 {
		t.Errorf("ReadBeforeEditMsgs %d, want kept at 100", got)
	}
	if FloorChildConfig(nil, 5) == nil {
		t.Error("nil cfg must yield an empty config, not nil")
	}
}

func TestGeneralChildScopeAllowlist(t *testing.T) {
	cfg := &config.Config{}
	cfg.Tools.Edit.ExplicitDelete = true
	parentCtx := NewToolContext()
	parent := NewRegistry()
	tempMgr := NewTempFileManager(t.TempDir())
	// Real tools for everything the child may copy.
	parent.Enable(NewReadFileTool(cfg, parentCtx))
	parent.Enable(NewSearchTool(cfg, tempMgr))
	parent.Enable(NewGlobTool(cfg))
	parent.Enable(NewWebSearchTool(cfg))
	parent.Enable(NewWebFetchTool(cfg, tempMgr))
	parent.Enable(NewUnifiedEditTool(cfg, parentCtx))
	parent.Enable(NewWriteFileTool(cfg, parentCtx))
	parent.Enable(NewDeleteLinesTool(cfg, parentCtx))
	shellTimeout := secondsToDuration(120)
	st := NewShellTool(cfg, shellTimeout, tempMgr)
	st.SetToolContext(parentCtx)
	parent.Enable(st)
	adv := NewShellAdvancedTool(cfg, shellTimeout, tempMgr)
	adv.SetToolContext(parentCtx)
	parent.Enable(adv)
	batch := NewBatchTool(cfg)
	batch.SetRegistry(parent)
	batch.SetToolContext(parentCtx)
	parent.Enable(batch)
	// Exclusions, seeded as both stubs and the real grouped Question.
	for _, name := range []string{"Subagent", "Report", "Edit.confirm", "Edit.cancel",
		"Write.confirm", "Write.cancel", "Shell.start", "Shell.output", "Shell.status",
		"Shell.list", "Shell.kill", "Shell.tune", "Observe.wait", "Observe.add",
		"mcp.github.get_issue"} {
		parent.Enable(&stubNamedTool{name: name})
	}
	group := NewGroupTool("Extras", "extra tools", "instructions", "filesystem", 900, nil)
	group.AddLocalMembers(NewQuestionTool(cfg, parentCtx))
	parent.Enable(group)

	child, childCtx := GeneralChildScope(parent, parentCtx, cfg, 5)
	if childCtx == parentCtx || !childCtx.IsSubagentChild() {
		t.Fatal("child context is not a fresh marked context")
	}
	for _, name := range []string{"Read", "Search", "Glob", "Web.search", "Web.fetch",
		"Edit", "Write", "DeleteLines", "Shell", "Shell.advanced", "Batch"} {
		if child.Get(name) == nil {
			t.Errorf("general handle is missing %q", name)
		}
	}
	if _, ok := child.Get("Edit").(*UnifiedEditTool); !ok {
		t.Errorf("child Edit type %T, want same mode as parent (*UnifiedEditTool)", child.Get("Edit"))
	}
	if _, isGate := child.Get("Shell").(*readOnlyShellGate); isGate {
		t.Error("general Shell is still read-only-gated; general gets full Shell")
	}
	if _, ok := child.Get("Shell").(*ShellTool); !ok {
		t.Errorf("child Shell type %T, want *ShellTool", child.Get("Shell"))
	}
	if _, ok := child.Get("Shell.advanced").(*ShellAdvancedTool); !ok {
		t.Errorf("child Shell.advanced type %T, want *ShellAdvancedTool", child.Get("Shell.advanced"))
	}
	for _, absent := range []string{"Subagent", "Question", "Report",
		"Edit.confirm", "Edit.cancel", "Write.confirm", "Write.cancel",
		"Shell.start", "Shell.output", "Shell.status", "Shell.list", "Shell.kill", "Shell.tune",
		"Observe.wait", "Observe.add", "mcp.github.get_issue", "Extras"} {
		if child.Get(absent) != nil {
			t.Errorf("general handle has excluded tool %q", absent)
		}
	}
	// Backstop sweep: no copied tool may be a group or carry the MCP prefix.
	for _, tool := range child.All() {
		if _, isGroup := tool.(*GroupTool); isGroup {
			t.Errorf("child handle holds group %q", tool.Name())
		}
		if strings.HasPrefix(tool.Name(), "mcp.") {
			t.Errorf("MCP tool %q re-entered the child handle", tool.Name())
		}
	}

	// Research keeps the read-only shape: no edit paths, gated Shell.
	research, _ := TestChildScope(parent, parentCtx, cfg, 5, "research")
	for _, absent := range []string{"Edit", "Write", "DeleteLines"} {
		if research.Get(absent) != nil {
			t.Errorf("research handle has %q", absent)
		}
	}
	if _, isGate := research.Get("Shell").(*readOnlyShellGate); !isGate {
		t.Errorf("research Shell type %T, want *readOnlyShellGate", research.Get("Shell"))
	}
}

func TestStalenessChildBlindEdit(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Workspace.Root = dir
	cfg.Tools.Edit.ReadBeforeEditMsgs = 0 // parent gate off
	parentCtx := NewToolContext()
	parent := realParentRegistry(t, cfg, parentCtx)
	f := writeTmpFile(t, dir, "f.txt", "line1\nline2\n")

	// The parent read the file. That must not satisfy the child's gate:
	// parent reads don't leak across the fresh child context.
	parentCtx.ReadTracker.RecordRead(f, parentCtx.ReadTracker.CurrentMessageID())

	child, _ := GeneralChildScope(parent, parentCtx, cfg, 5)
	edit := child.Get("Edit")
	if edit == nil {
		t.Fatal("general handle has no Edit")
	}
	err := edit.Check(context.Background(), mustJSON(map[string]any{
		"path": "f.txt", "start_line": 1, "end_line": 1, "new_text": "blind",
	}))
	if err == nil {
		t.Fatal("blind child edit passed; want file_not_read")
	}
	if code := toolErrorCode(err); code != "file_not_read" {
		t.Errorf("error code %q, want file_not_read", code)
	}
}

func TestStalenessMtime(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Workspace.Root = dir
	parentCtx := NewToolContext()
	parent := realParentRegistry(t, cfg, parentCtx)
	writeTmpFile(t, dir, "f.txt", "v1\n")

	child, _ := GeneralChildScope(parent, parentCtx, cfg, 5)
	read := child.Get("Read")
	edit := child.Get("Edit")
	if _, err := read.Call(context.Background(), mustJSON(map[string]any{"path": "f.txt"})); err != nil {
		t.Fatalf("child read: %v", err)
	}
	// Outside edit: rewrite with a different size so the stat differs even
	// at coarse modTime granularity.
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("v1 plus much more content here\n"), 0644); err != nil {
		t.Fatal(err)
	}
	err := edit.Check(context.Background(), mustJSON(map[string]any{
		"path": "f.txt", "start_line": 1, "end_line": 1, "new_text": "stale",
	}))
	if err == nil {
		t.Fatal("edit after outside change passed; want file_changed_since_read")
	}
	if code := toolErrorCode(err); code != "file_changed_since_read" {
		t.Errorf("error code %q, want file_changed_since_read", code)
	}
}

// TestStalenessHandoff drives a real child edit through Subagent.Call with a
// stub runner, then asserts the parent must re-read before editing.
func TestStalenessHandoff(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Workspace.Root = dir
	cfg.Tools.Edit.ReadBeforeEditMsgs = 0 // parent gate off: stale still bites
	parentCtx := NewToolContext()
	parentReg := realParentRegistry(t, cfg, parentCtx)
	writeTmpFile(t, dir, "f.txt", "line1\nline2\n")

	stub := &stubSubRunner{fn: func(p SubagentRunParams) (SubagentRunResult, error) {
		read, edit := p.Registry.Get("Read"), p.Registry.Get("Edit")
		if read == nil || edit == nil {
			return SubagentRunResult{}, fmt.Errorf("child missing Read/Edit")
		}
		if _, err := read.Call(ctx, mustJSON(map[string]any{"path": "f.txt"})); err != nil {
			return SubagentRunResult{}, err
		}
		if _, err := edit.Call(ctx, mustJSON(map[string]any{
			"path": "f.txt", "start_line": 1, "end_line": 1, "new_text": "child was here",
		})); err != nil {
			return SubagentRunResult{}, err
		}
		// Mirror the child runner's Changes() accounting.
		p.ToolCtx.NoteChange()
		return SubagentRunResult{Text: "edited f.txt:1", Model: "m"}, nil
	}}
	sub, _ := newSubagentUnderTest(t, cfg, parentCtx, parentReg, stub)
	if _, err := sub.Call(ctx, mustJSON(map[string]any{
		"description": "edit f", "prompt": "change f.txt", "subagent_type": "general",
	})); err != nil {
		t.Fatalf("Subagent Call: %v", err)
	}
	if !parentCtx.ChangedThisTurn() {
		t.Error("delegated edit did not flip parent ChangedThisTurn")
	}

	parentEdit := parentReg.Get("Edit")
	editArgs := mustJSON(map[string]any{
		"path": "f.txt", "start_line": 2, "end_line": 2, "new_text": "parent clobber",
	})
	if err := parentEdit.Check(ctx, editArgs); err == nil {
		t.Fatal("parent edit after child edit passed; want file_changed_by_subagent")
	} else if code := toolErrorCode(err); code != "file_changed_by_subagent" {
		t.Fatalf("error code %q, want file_changed_by_subagent (%v)", code, err)
	}

	// A fresh parent Read clears the requirement; the edit then succeeds
	// even with the gate off.
	if _, err := parentReg.Get("Read").Call(ctx, mustJSON(map[string]any{"path": "f.txt"})); err != nil {
		t.Fatalf("parent re-read: %v", err)
	}
	if err := parentEdit.Check(ctx, editArgs); err != nil {
		t.Errorf("parent edit after re-read: %v", err)
	}
}

func TestStalenessHandoffGateOn(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Workspace.Root = dir
	cfg.Tools.Edit.ReadBeforeEditMsgs = 3 // parent gate on
	parentCtx := NewToolContext()
	parentReg := realParentRegistry(t, cfg, parentCtx)
	abs := writeTmpFile(t, dir, "f.txt", "line1\nline2\n")

	// Simulate the Call handoff: the child edited F.
	parentCtx.RequireReRead([]string{abs})
	parentEdit := parentReg.Get("Edit")
	editArgs := mustJSON(map[string]any{
		"path": "f.txt", "start_line": 2, "end_line": 2, "new_text": "parent clobber",
	})
	if err := parentEdit.Check(ctx, editArgs); err == nil {
		t.Fatal("stale parent edit passed with gate on; want file_changed_by_subagent")
	} else if code := toolErrorCode(err); code != "file_changed_by_subagent" {
		t.Fatalf("error code %q, want file_changed_by_subagent", code)
	}
	// The window path still passes after a re-read.
	if _, err := parentReg.Get("Read").Call(ctx, mustJSON(map[string]any{"path": "f.txt"})); err != nil {
		t.Fatalf("parent re-read: %v", err)
	}
	if err := parentEdit.Check(ctx, editArgs); err != nil {
		t.Errorf("parent edit after re-read with gate on: %v", err)
	}
}

func TestTrackerSizing(t *testing.T) {
	tc := NewToolContextWithReadWindow(50)
	if tc.ReadTracker == nil {
		t.Fatal("no tracker")
	}
	// maxEntries is private; same-package access asserts the sizing rule.
	if tc.ReadTracker.maxEntries != 50 {
		t.Errorf("maxEntries %d, want 50", tc.ReadTracker.maxEntries)
	}
	if NewToolContextWithReadWindow(3).ReadTracker.maxEntries != 10 {
		t.Error("window below 10 must floor to 10")
	}
	dir := t.TempDir()
	const reads = 150 // 3 reads per message over 50 messages
	for i := 0; i < reads; i++ {
		p := filepath.Join(dir, fmt.Sprintf("f%03d.txt", i))
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
		tc.ReadTracker.RecordRead(p, i/3+1)
	}
	// Every in-window entry must survive: with the old maxEntries=10 the
	// trim (10*5=50) would have evicted the first 100.
	for _, i := range []int{0, 49, 50, 99, 149} {
		p := filepath.Join(dir, fmt.Sprintf("f%03d.txt", i))
		if !tc.ReadTracker.WasReadRecently(p, 50, 50) {
			t.Errorf("%s evicted inside its window", p)
		}
	}
	// And the child context carries the sizing: max(10, MaxIters).
	_, childCtx := GeneralChildScope(NewRegistry(), NewToolContext(), &config.Config{}, 42)
	if childCtx.ReadTracker.maxEntries != 42 {
		t.Errorf("child maxEntries %d, want 42", childCtx.ReadTracker.maxEntries)
	}
}

func TestPreviewOff(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Workspace.Root = dir
	cfg.Tools.Edit.PreviewMode = true // parent previews; the child must not
	parentCtx := NewToolContext()
	parent := realParentRegistry(t, cfg, parentCtx)
	writeTmpFile(t, dir, "f.txt", "line1\nline2\n")

	child, _ := GeneralChildScope(parent, parentCtx, cfg, 5)
	for _, name := range []string{"Edit.confirm", "Edit.cancel", "Write.confirm", "Write.cancel"} {
		if child.Get(name) != nil {
			t.Errorf("child handle has handshake tool %q", name)
		}
	}
	if _, err := child.Get("Read").Call(ctx, mustJSON(map[string]any{"path": "f.txt"})); err != nil {
		t.Fatalf("child read: %v", err)
	}
	res, err := child.Get("Edit").Call(ctx, mustJSON(map[string]any{
		"path": "f.txt", "start_line": 1, "end_line": 1, "new_text": "direct",
	}))
	if err != nil {
		t.Fatalf("child edit: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("edit result type %T", res)
	}
	if m["status"] == "pending_confirmation" {
		t.Error("child edit staged for confirm: PreviewMode must be off in the child")
	}
	if m["success"] != true {
		t.Errorf("child edit result %+v, want success", m)
	}
	if parentCtx.HasPendingEdit() || parentCtx.HasPendingWrite() {
		t.Error("parent holds a pending edit/write from the child's edit")
	}
}

func TestWriteGate(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	cfg := &config.Config{}
	cfg.Workspace.Root = dir
	parentCtx := NewToolContext()
	parent := realParentRegistry(t, cfg, parentCtx)
	writeTmpFile(t, dir, "f.txt", "hello\n")

	child, _ := GeneralChildScope(parent, parentCtx, cfg, 5)
	write := child.Get("Write")
	overwrite := mustJSON(map[string]any{"path": "f.txt", "text": "no read\n"})
	if err := write.Check(ctx, overwrite); err == nil {
		t.Fatal("child Write overwrite without read passed; want file_not_read")
	} else if code := toolErrorCode(err); code != "file_not_read" {
		t.Fatalf("error code %q, want file_not_read", code)
	}
	if _, err := write.Call(ctx, overwrite); err == nil {
		t.Fatal("child Write.Call overwrite without read passed; want file_not_read")
	}
	// Creating a new file always passes.
	if _, err := write.Call(ctx, mustJSON(map[string]any{"path": "new.txt", "text": "fresh\n"})); err != nil {
		t.Errorf("child Write create-new: %v", err)
	}
	// After a child-local read the overwrite succeeds.
	if _, err := child.Get("Read").Call(ctx, mustJSON(map[string]any{"path": "f.txt"})); err != nil {
		t.Fatalf("child read: %v", err)
	}
	if err := write.Check(ctx, overwrite); err != nil {
		t.Errorf("child Write after read: %v", err)
	}

	// Parent behavior is unchanged: gate-off overwrite without read succeeds.
	cfg.Tools.Edit.ReadBeforeEditMsgs = 0
	if err := parent.Get("Write").Check(ctx, mustJSON(map[string]any{"path": "other.txt", "text": "x\n"})); err != nil {
		t.Errorf("parent Write create check: %v", err)
	}
}

func TestShellAskerSetDeny(t *testing.T) {
	pcfg := &config.Config{}
	pcfg.Workspace.Root = "."
	pcfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)
	grantor := permissions.NewGrantor(nil, nil, nil)
	parentCtx := NewToolContext()
	parentCtx.SetGrantor(grantor)
	// Even an asker that allows everything must not be consulted: there is
	// no UI in the child, so the deny precedes the grantor switch.
	parentCtx.SetPermissionAsker(func(PermissionRequest) PermissionAnswer {
		return PermissionAllowSession
	})
	tempMgr := NewTempFileManager(t.TempDir())
	real := NewShellAdvancedTool(pcfg, secondsToDuration(120), tempMgr)
	real.SetToolContext(parentCtx)
	parentReg := NewRegistry()
	parentReg.Enable(real)

	child, _ := GeneralChildScope(parentReg, parentCtx, pcfg, 5)
	tool := child.Get("Shell.advanced")
	inner, ok := tool.(*ShellAdvancedTool)
	if !ok {
		t.Fatalf("general shell type %T, want full *ShellAdvancedTool", tool)
	}
	err := inner.Check(context.Background(), mustJSON(map[string]any{"command": "curl https://example.com"}))
	if err == nil {
		t.Fatal("curl passed child Check with asker set; want deny")
	}
	if !strings.Contains(strings.ToLower(err.Error()), subagentPromptDenyHint) {
		t.Errorf("child deny %q lacks %q", err.Error(), subagentPromptDenyHint)
	}
}

func TestSubagentPathPromptDeny(t *testing.T) {
	dir := t.TempDir()
	outside := string(filepath.Separator) + filepath.Join("definitely", "outside", "file.txt")

	askCfg := &config.Config{}
	askCfg.Workspace.Root = dir
	askCfg.Workspace.PathSafetyMode = "ask_once"
	askCfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{}
	parentCtx := NewToolContext()
	parent := NewRegistry()
	parent.Enable(NewReadFileTool(askCfg, parentCtx))

	// Through the child Read path: outside + ask_once + unconfirmed denies
	// with the subagent message instead of prompting.
	child, _ := TestChildScope(parent, parentCtx, askCfg, 5, "research")
	err := mustCallRead(child, outside)
	if err == nil {
		t.Fatal("outside read passed in child; want subagents-cannot-prompt deny")
	}
	if !strings.Contains(strings.ToLower(err.Error()), subagentPromptDenyHint) {
		t.Errorf("child deny %q lacks %q", err.Error(), subagentPromptDenyHint)
	}
	if !IsBacktrackable(err) {
		t.Errorf("child deny %v is not backtrackable (semantic)", err)
	}

	// Modes that never prompt pass through to the ordinary check.
	blockCfg := &config.Config{}
	blockCfg.Workspace.Root = dir
	blockCfg.Workspace.PathSafetyMode = "block"
	blockCfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{}
	parent2 := NewRegistry()
	parent2.Enable(NewReadFileTool(blockCfg, NewToolContext()))
	child2, _ := TestChildScope(parent2, NewToolContext(), blockCfg, 5, "research")
	err = mustCallRead(child2, outside)
	if err == nil {
		t.Fatal("block mode outside read passed; want the block error")
	}
	if strings.Contains(strings.ToLower(err.Error()), subagentPromptDenyHint) {
		t.Errorf("block mode should pass through, got subagent deny %q", err.Error())
	}

	// The project-notes containment case through the new path: a sibling
	// directory sharing a string prefix is outside, so it denies.
	notesCfg := &config.Config{}
	notesCfg.Workspace.Root = filepath.Join(dir, "project")
	notesCfg.Workspace.PathSafetyMode = "ask_once"
	notesCfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{}
	if !notesCfg.WouldPromptForPath("read", filepath.Join(dir, "project-notes", "f")) {
		t.Error("project-notes counted as inside project")
	}
	if notesCfg.WouldPromptForPath("read", filepath.Join(dir, "project", "f")) {
		t.Error("project/file counted as prompting inside its own project")
	}
}

func TestSubagentPathPromptDenyGlobSearch(t *testing.T) {
	dir := t.TempDir()
	outside := string(filepath.Separator) + filepath.Join("definitely", "outside")
	askCfg := &config.Config{}
	askCfg.Workspace.Root = dir
	askCfg.Workspace.PathSafetyMode = "ask_once"
	askCfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{}
	parent := NewRegistry()
	parent.Enable(&stubNamedTool{name: "Glob"})
	parent.Enable(&stubNamedTool{name: "Search"})
	child, _ := TestChildScope(parent, NewToolContext(), askCfg, 5, "research")

	globErr := func() error {
		_, err := child.Get("Glob").Call(context.Background(), mustJSON(map[string]any{"path": outside}))
		return err
	}()
	if globErr == nil {
		t.Fatal("outside Glob passed in child; want subagents-cannot-prompt deny")
	}
	if !strings.Contains(strings.ToLower(globErr.Error()), subagentPromptDenyHint) {
		t.Errorf("Glob deny %q lacks %q", globErr.Error(), subagentPromptDenyHint)
	}
	if !IsBacktrackable(globErr) {
		t.Errorf("Glob deny %v is not backtrackable (semantic)", globErr)
	}

	searchErr := func() error {
		_, err := child.Get("Search").Call(context.Background(), mustJSON(map[string]any{
			"pattern": "x", "path": outside,
		}))
		return err
	}()
	if searchErr == nil {
		t.Fatal("outside Search passed in child; want subagents-cannot-prompt deny")
	}
	if !strings.Contains(strings.ToLower(searchErr.Error()), subagentPromptDenyHint) {
		t.Errorf("Search deny %q lacks %q", searchErr.Error(), subagentPromptDenyHint)
	}
}

func mustCallRead(child *Registry, path string) error {
	read := child.Get("Read")
	if read == nil {
		return fmt.Errorf("child has no Read")
	}
	_, err := read.Call(context.Background(), mustJSON(map[string]any{"path": path}))
	return err
}

func TestGroupedQuestionStaysOut(t *testing.T) {
	cfg := &config.Config{}
	parentCtx := NewToolContext()
	parent := NewRegistry()
	parent.Enable(&stubNamedTool{name: "Read"})
	// Question lives only behind the group, as after a tool_groups: claim.
	group := NewGroupTool("Extras", "extra tools", "instructions", "filesystem", 900, nil)
	group.AddLocalMembers(NewQuestionTool(cfg, parentCtx))
	parent.Enable(group)

	for _, typ := range []string{"research", "general"} {
		child, _ := TestChildScope(parent, parentCtx, cfg, 5, typ)
		if child.Get("Extras") != nil {
			t.Errorf("%s handle copied group Extras", typ)
		}
		if child.Get("Question") != nil {
			t.Errorf("%s handle has a question path via the group", typ)
		}
	}
}

func TestReportIsolation(t *testing.T) {
	parentCtx := NewToolContext()
	child, childCtx := GeneralChildScope(NewRegistry(), parentCtx, &config.Config{}, 5)
	_ = child
	childCtx.AcceptReport(&report.Report{})
	if parentCtx.AcceptedReport() != nil {
		t.Error("child AcceptedReport leaked into the parent context")
	}
	childCtx.RecordReportRepair()
	if parentCtx.ReportRepairs() != 0 {
		t.Error("child ReportRepairs leaked into the parent context")
	}
}

var _ = json.Marshal

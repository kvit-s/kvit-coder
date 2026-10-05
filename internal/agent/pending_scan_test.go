package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// newEditRunner builds a runner whose real Edit tool shares the runner's tool
// context, so an edit staged under preview_mode is what the pending-edit check
// sees, as in a real turn.
func newEditRunner(t *testing.T, cfg *config.Config, client LLMClient, probe tools.Tool) *Runner {
	t.Helper()
	var out strings.Builder
	writer := ui.NewWriter(0)
	writer.SetStdout(&out)
	writer.SetStderr(&out)
	writer.SetHeadless(true)
	logger, err := NewLogger("", false)
	if err != nil {
		t.Fatalf("NewLogger: %v", err)
	}
	toolCtx := tools.NewToolContext()
	registry := tools.NewRegistry()
	registry.Enable(tools.NewSearchReplaceEditTool(cfg, toolCtx))
	registry.Enable(probe)
	return NewRunner(RunnerOptions{
		Cfg:       cfg,
		LLMClient: client,
		Registry:  registry,
		Writer:    writer,
		Logger:    logger,
		ToolCtx:   toolCtx,
	})
}

func editConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := testConfig()
	cfg.Workspace.Root = t.TempDir()
	cfg.Tools.Edit.Enabled = true
	cfg.Tools.Edit.Mode = "searchreplace"
	if err := os.WriteFile(filepath.Join(cfg.Workspace.Root, "a.txt"), []byte("hello\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return cfg
}

func toolMessage(msgs []llm.Message, name string) *llm.Message {
	for i := range msgs {
		if msgs[i].Role == llm.RoleTool && msgs[i].Name == name {
			return &msgs[i]
		}
	}
	return nil
}

// While this process holds an edit staged by preview_mode, every other tool is
// blocked until the edit is confirmed or cancelled.
func TestStagedEditBlocksOtherTools(t *testing.T) {
	cfg := editConfig(t)
	cfg.Tools.Edit.PreviewMode = true
	probeRan := false
	probe := &scriptedTool{name: "Probe", call: func(context.Context, json.RawMessage) (any, error) {
		probeRan = true
		return map[string]any{"ok": true}, nil
	}}
	client := newFakeClient(
		calls(toolCall("c1", "Edit", map[string]any{"path": "a.txt", "search": "hello", "replace": "bye"})),
		calls(toolCall("c2", "Probe", map[string]any{"arg": "x"})),
		answer("done"),
	)
	r := newEditRunner(t, cfg, client, probe)
	res, err := r.Run(context.Background(), RunConfig{
		Messages: []llm.Message{{Role: llm.RoleUser, Content: "change it"}},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if edit := toolMessage(res.FinalMessages, "Edit"); edit == nil || !strings.Contains(edit.Content, "pending_confirmation") {
		t.Fatalf("the edit was not staged: %+v", edit)
	}
	if probeRan {
		t.Error("Probe ran while an edit was staged")
	}
	if msg := toolMessage(res.FinalMessages, "Probe"); msg == nil || !strings.Contains(msg.Content, "BLOCKED") {
		t.Errorf("Probe result = %+v, want a BLOCKED error", msg)
	}
}

// A pending edit recorded in the history by an earlier process blocks
// nothing: the process that staged it has exited, so nothing is held.
func TestPendingEditFromEarlierProcessDoesNotBlock(t *testing.T) {
	cfg := editConfig(t)
	probeRan := false
	probe := &scriptedTool{name: "Probe", call: func(context.Context, json.RawMessage) (any, error) {
		probeRan = true
		return map[string]any{"ok": true}, nil
	}}
	client := newFakeClient(
		calls(toolCall("c2", "Probe", map[string]any{"arg": "x"})),
		answer("done"),
	)
	r := newEditRunner(t, cfg, client, probe)
	_, err := r.Run(context.Background(), RunConfig{
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "change it"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{toolCall("c1", "Edit", map[string]any{"path": "a.txt"})}},
			{Role: llm.RoleTool, Name: "Edit", ToolCallID: "c1", Content: `{"status":"pending_confirmation","path":"a.txt"}`},
			{Role: llm.RoleUser, Content: "go on"},
		},
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !probeRan {
		t.Error("Probe was blocked by a pending edit nothing holds")
	}
}

package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

func writeToolConfig(t *testing.T, previewMode bool) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Workspace.Root = t.TempDir()
	cfg.Tools.Edit.Enabled = true
	cfg.Tools.Edit.PreviewMode = previewMode
	cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)
	return cfg
}

func callWrite(t *testing.T, tool *WriteFileTool, path, text string) map[string]any {
	t.Helper()
	args, err := json.Marshal(map[string]string{"path": path, "text": text})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	res, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("Write(%q): %v", path, err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("Write returned %T, want a map", res)
	}
	return m
}

// TestWriteOverwritesWithoutAHandshake: overwriting an existing file used to be
// staged for confirmation whatever the configuration said, so a profile with
// the handshake off still got a pending_confirmation it had no Write.confirm to
// answer with.
func TestWriteOverwritesWithoutAHandshake(t *testing.T) {
	cfg := writeToolConfig(t, false)
	path := filepath.Join(cfg.Workspace.Root, "notes.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0644); err != nil {
		t.Fatalf("seed the file: %v", err)
	}

	tool := NewWriteFileTool(cfg, NewToolContext())
	m := callWrite(t, tool, path, "after\n")

	if m["status"] == "pending_confirmation" {
		t.Fatalf("the overwrite was staged with preview_mode off: %v", m)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(got) != "after\n" {
		t.Errorf("file holds %q, want the overwrite to have been applied", got)
	}
}

// TestWriteStagesTheOverwriteUnderPreviewMode: preview_mode is what the
// handshake is for, and it still works.
func TestWriteStagesTheOverwriteUnderPreviewMode(t *testing.T) {
	cfg := writeToolConfig(t, true)
	path := filepath.Join(cfg.Workspace.Root, "notes.txt")
	if err := os.WriteFile(path, []byte("before\n"), 0644); err != nil {
		t.Fatalf("seed the file: %v", err)
	}

	toolCtx := NewToolContext()
	m := callWrite(t, NewWriteFileTool(cfg, toolCtx), path, "after\n")
	if m["status"] != "pending_confirmation" {
		t.Fatalf("the overwrite was applied with preview_mode on: %v", m)
	}
	if got, _ := os.ReadFile(path); string(got) != "before\n" {
		t.Fatalf("the file changed before it was confirmed: %q", got)
	}

	if _, err := NewConfirmWriteTool(cfg, toolCtx).Call(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Fatalf("Write.confirm: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != "after\n" {
		t.Errorf("file holds %q after Write.confirm, want the overwrite applied", got)
	}
}

// TestWriteConfirmIsRegisteredOnlyWithTheHandshake: a tool with nothing it can
// ever confirm should not be in the list the model reads.
func TestWriteConfirmIsRegisteredOnlyWithTheHandshake(t *testing.T) {
	for _, previewMode := range []bool{false, true} {
		cfg := writeToolConfig(t, previewMode)
		registry := SetupRegistry(SetupConfig{Cfg: cfg, ToolCtx: NewToolContext()})

		names := map[string]bool{}
		for _, name := range registry.ListTools() {
			names[name] = true
		}
		for _, name := range []string{"Write.confirm", "Write.cancel", "Edit.confirm", "Edit.cancel"} {
			if names[name] != previewMode {
				t.Errorf("preview_mode=%v: %s registered=%v, want %v",
					previewMode, name, names[name], previewMode)
			}
		}
		if !names["Write"] {
			t.Errorf("preview_mode=%v: Write itself is missing", previewMode)
		}
	}
}

// TestPendingEditFromAnExitedProcessBlocksNothing: the history spans turns and
// the staged edit does not — it lives in the memory of a process that has since
// exited. Blocking on it sent the model to Edit.confirm, which answered
// no_pending_operation, and cost it the turn.
func TestPendingEditFromAnExitedProcessBlocksNothing(t *testing.T) {
	cfg := writeToolConfig(t, true)
	state := PendingEditState{HasPending: true, PendingPath: "main.go"}

	fresh := NewToolContext()
	if err := CheckPendingEditBlockWithState("Read", state, cfg, fresh); err != nil {
		t.Errorf("Read was blocked by a pending edit this process does not hold: %v", err)
	}

	// A pending edit this process does hold still blocks, which is the whole
	// point of the handshake.
	held := NewToolContext()
	held.SetPendingEdit(&pendingEdit{path: "main.go", fullPath: "/tmp/main.go"})
	if err := CheckPendingEditBlockWithState("Read", state, cfg, held); err == nil {
		t.Error("Read ran with an edit pending in this process, want it blocked")
	}
}

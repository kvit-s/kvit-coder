package tools

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// deleteToolWithConfig builds a DeleteLinesTool with explicit_delete on and the given
// preview mode.
func deleteToolWithConfig(t *testing.T, tmpDir string, preview bool) (*DeleteLinesTool, *ToolContext) {
	t.Helper()
	cfg := newTestEditConfig(tmpDir)
	cfg.Tools.Edit.ExplicitDelete = true
	cfg.Tools.Edit.PreviewMode = preview
	toolCtx := NewToolContext()
	return NewDeleteLinesTool(cfg, toolCtx), toolCtx
}

func TestDeleteLinesSingleAndRange(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := deleteToolWithConfig(t, tmpDir, false) // apply directly (no preview)

	// Single line.
	if err := os.WriteFile(filepath.Join(tmpDir, "a.txt"), []byte("l1\nl2\nl3\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Call(context.Background(), mustJSON(map[string]any{
		"path": "a.txt", "start_line": 2,
	})); err != nil {
		t.Fatalf("delete single: %v", err)
	}
	if got := readFileStr(t, tmpDir, "a.txt"); got != "l1\nl3\n" {
		t.Errorf("single delete = %q, want %q", got, "l1\nl3\n")
	}

	// Range.
	if err := os.WriteFile(filepath.Join(tmpDir, "b.txt"), []byte("l1\nl2\nl3\nl4\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := tool.Call(context.Background(), mustJSON(map[string]any{
		"path": "b.txt", "start_line": 2, "end_line": 3,
	})); err != nil {
		t.Fatalf("delete range: %v", err)
	}
	if got := readFileStr(t, tmpDir, "b.txt"); got != "l1\nl4\n" {
		t.Errorf("range delete = %q, want %q", got, "l1\nl4\n")
	}
}

func TestDeleteLinesValidation(t *testing.T) {
	tmpDir := t.TempDir()
	tool, _ := deleteToolWithConfig(t, tmpDir, false)
	if err := os.WriteFile(filepath.Join(tmpDir, "c.txt"), []byte("l1\nl2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cases := []map[string]any{
		{"path": "c.txt"},                                 // missing start_line
		{"path": "c.txt", "start_line": 0},                // start_line < 1
		{"path": "c.txt", "start_line": 2, "end_line": 1}, // end < start
		{"path": "c.txt", "start_line": 5},                // beyond EOF
		{"path": "missing.txt", "start_line": 1},          // file does not exist
	}
	for i, args := range cases {
		if _, err := tool.Call(context.Background(), mustJSON(args)); err == nil {
			t.Errorf("case %d (%v): expected error, got nil", i, args)
		}
	}
}

func TestDeleteLinesPreviewAndConfirm(t *testing.T) {
	tmpDir := t.TempDir()
	tool, toolCtx := deleteToolWithConfig(t, tmpDir, true) // preview mode
	if err := os.WriteFile(filepath.Join(tmpDir, "d.txt"), []byte("keep1\ngone\nkeep2\n"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := tool.Call(context.Background(), mustJSON(map[string]any{
		"path": "d.txt", "start_line": 2, "end_line": 2,
	}))
	if err != nil {
		t.Fatalf("delete preview: %v", err)
	}
	if res.(map[string]any)["status"] != "pending_confirmation" {
		t.Fatalf("expected pending_confirmation, got %v", res)
	}
	// Not applied yet.
	if got := readFileStr(t, tmpDir, "d.txt"); got != "keep1\ngone\nkeep2\n" {
		t.Errorf("file changed before confirm: %q", got)
	}
	// Confirm applies it.
	confirm := NewConfirmEditTool(tool.Config, toolCtx)
	if _, err := confirm.Call(context.Background(), mustJSON(map[string]any{})); err != nil {
		t.Fatalf("confirm: %v", err)
	}
	if got := readFileStr(t, tmpDir, "d.txt"); got != "keep1\nkeep2\n" {
		t.Errorf("after confirm = %q, want %q", got, "keep1\nkeep2\n")
	}
}

// editToolWithDelete builds a UnifiedEditTool with explicit_delete set as given.
func editToolWithDelete(t *testing.T, tmpDir string, explicitDelete bool) *UnifiedEditTool {
	t.Helper()
	cfg := newTestEditConfig(tmpDir)
	cfg.Tools.Edit.ExplicitDelete = explicitDelete
	cfg.Tools.Edit.PreviewMode = false
	return NewUnifiedEditTool(cfg, NewToolContext())
}

func TestEmptyNewTextFlip(t *testing.T) {
	t.Run("flag on: empty new_text blanks the line", func(t *testing.T) {
		tmpDir := t.TempDir()
		tool := editToolWithDelete(t, tmpDir, true)
		if err := os.WriteFile(filepath.Join(tmpDir, "f.txt"), []byte("l1\nl2\nl3\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := tool.Call(context.Background(), mustJSON(map[string]any{
			"path": "f.txt", "start_line": 2, "end_line": 2, "new_text": "",
		})); err != nil {
			t.Fatalf("edit: %v", err)
		}
		// Line 2 kept but blank.
		if got := readFileStr(t, tmpDir, "f.txt"); got != "l1\n\nl3\n" {
			t.Errorf("flag-on empty new_text = %q, want %q (blank line kept)", got, "l1\n\nl3\n")
		}
	})

	t.Run("flag off: empty new_text deletes (back-compat)", func(t *testing.T) {
		tmpDir := t.TempDir()
		tool := editToolWithDelete(t, tmpDir, false)
		if err := os.WriteFile(filepath.Join(tmpDir, "g.txt"), []byte("l1\nl2\nl3\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if _, err := tool.Call(context.Background(), mustJSON(map[string]any{
			"path": "g.txt", "start_line": 2, "end_line": 2, "new_text": "",
		})); err != nil {
			t.Fatalf("edit: %v", err)
		}
		if got := readFileStr(t, tmpDir, "g.txt"); got != "l1\nl3\n" {
			t.Errorf("flag-off empty new_text = %q, want %q (deleted)", got, "l1\nl3\n")
		}
	})

	t.Run("newline new_text blanks in both modes", func(t *testing.T) {
		for _, flag := range []bool{false, true} {
			tmpDir := t.TempDir()
			tool := editToolWithDelete(t, tmpDir, flag)
			if err := os.WriteFile(filepath.Join(tmpDir, "h.txt"), []byte("l1\nl2\nl3\n"), 0644); err != nil {
				t.Fatal(err)
			}
			if _, err := tool.Call(context.Background(), mustJSON(map[string]any{
				"path": "h.txt", "start_line": 2, "end_line": 2, "new_text": "\n",
			})); err != nil {
				t.Fatalf("edit (flag=%v): %v", flag, err)
			}
			if got := readFileStr(t, tmpDir, "h.txt"); got != "l1\n\nl3\n" {
				t.Errorf("flag=%v newline new_text = %q, want %q", flag, got, "l1\n\nl3\n")
			}
		}
	})

	t.Run("flag on: insert mode empty new_text unaffected", func(t *testing.T) {
		tmpDir := t.TempDir()
		tool := editToolWithDelete(t, tmpDir, true)
		if err := os.WriteFile(filepath.Join(tmpDir, "i.txt"), []byte("l1\nl2\n"), 0644); err != nil {
			t.Fatal(err)
		}
		// Insert mode (no end_line) with "" - should not be turned into a blank line.
		if _, err := tool.Call(context.Background(), mustJSON(map[string]any{
			"path": "i.txt", "start_line": 2, "new_text": "",
		})); err != nil {
			t.Fatalf("edit insert: %v", err)
		}
		if got := readFileStr(t, tmpDir, "i.txt"); got != "l1\nl2\n" {
			t.Errorf("insert mode empty new_text changed file: %q", got)
		}
	})
}

func readFileStr(t *testing.T, dir, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

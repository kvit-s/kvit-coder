package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReconcileFirstLineIndent(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		startLine int
		endLine   int
		newText   string
		maxFix    int
		wantText  string
		wantFix   bool // whether a correction is expected
	}{
		{
			// E7-shaped: 3 spaces -> 4 spaces
			name:      "E7 off-by-one return",
			content:   "def f():\n    return 41\n",
			startLine: 2,
			endLine:   2,
			newText:   "   return 42;",
			maxFix:    1,
			wantText:  "    return 42;",
			wantFix:   true,
		},
		{
			// C2-shaped: 1 space -> 2 spaces
			name:      "C2 off-by-one port",
			content:   "server:\n  port: 80\n",
			startLine: 2,
			endLine:   2,
			newText:   " port: 8080",
			maxFix:    1,
			wantText:  "  port: 8080",
			wantFix:   true,
		},
		{
			// E11-shaped (benchmark run 2): first line off-by-one 7->8, continuation
			// already 8 and left untouched.
			name:      "E11 first line corrected continuation untouched",
			content:   "def f():\n        validate()\n        more()\n",
			startLine: 2,
			endLine:   3,
			newText:   "       validate_input()\n        transform_data()",
			maxFix:    1,
			wantText:  "        validate_input()\n        transform_data()",
			wantFix:   true,
		},
		{
			name:      "no-op when already correct",
			content:   "def f():\n    return 41\n",
			startLine: 2,
			endLine:   2,
			newText:   "    return 42",
			maxFix:    1,
			wantText:  "    return 42",
			wantFix:   false,
		},
		{
			name:      "no-op on over-indent",
			content:   "def f():\n    return 41\n",
			startLine: 2,
			endLine:   2,
			newText:   "        return 42",
			maxFix:    1,
			wantText:  "        return 42",
			wantFix:   false,
		},
		{
			name:      "insert mode untouched",
			content:   "def f():\n    return 41\n",
			startLine: 2,
			endLine:   0, // insert
			newText:   "   inserted",
			maxFix:    1,
			wantText:  "   inserted",
			wantFix:   false,
		},
		{
			name:      "blank first line untouched",
			content:   "def f():\n    return 41\n",
			startLine: 2,
			endLine:   2,
			newText:   "   \n    body",
			maxFix:    1,
			wantText:  "   \n    body",
			wantFix:   false,
		},
		{
			name:      "empty newText (deletion) untouched",
			content:   "def f():\n    return 41\n",
			startLine: 2,
			endLine:   2,
			newText:   "",
			maxFix:    1,
			wantText:  "",
			wantFix:   false,
		},
		{
			name:      "deliberate 4-space dedent not corrected at maxFix 1",
			content:   "def f():\n        deep = 1\n",
			startLine: 2,
			endLine:   2,
			newText:   "    deep = 1", // intentional one-level dedent, deficit 4
			maxFix:    1,
			wantText:  "    deep = 1",
			wantFix:   false,
		},
		{
			name:      "tab indentation not corrected (spaces-only guard)",
			content:   "func f() {\n\t\tx := 1\n}\n",
			startLine: 2,
			endLine:   2,
			newText:   "\tx := 1", // 1 tab vs 2 tabs, deficit 1, but tabs -> skip
			maxFix:    1,
			wantText:  "\tx := 1",
			wantFix:   false,
		},
		{
			name:      "mixed style not corrected (proper-prefix guard)",
			content:   "def f():\n\t return 41\n", // tab+space orig indent
			startLine: 2,
			endLine:   2,
			newText:   "  return 42", // two spaces, not a prefix of "\t "
			maxFix:    1,
			wantText:  "  return 42",
			wantFix:   false,
		},
		{
			name:      "off-by-two corrected when maxFix raised",
			content:   "def f():\n        body = 1\n",
			startLine: 2,
			endLine:   2,
			newText:   "      body = 1", // 6 vs 8, deficit 2
			maxFix:    2,
			wantText:  "        body = 1",
			wantFix:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotText, gotCorr := ReconcileFirstLineIndent(tt.content, tt.startLine, tt.endLine, tt.newText, tt.maxFix)
			if gotText != tt.wantText {
				t.Errorf("newText = %q, want %q", gotText, tt.wantText)
			}
			if (gotCorr != nil) != tt.wantFix {
				t.Errorf("correction present = %v, want %v (corr=%+v)", gotCorr != nil, tt.wantFix, gotCorr)
			}
			if gotCorr != nil {
				if gotCorr.OriginalNewText != tt.newText {
					t.Errorf("OriginalNewText = %q, want %q", gotCorr.OriginalNewText, tt.newText)
				}
				if gotCorr.ReqStartLine != tt.startLine || gotCorr.ReqEndLine != tt.endLine {
					t.Errorf("Req range = %d-%d, want %d-%d", gotCorr.ReqStartLine, gotCorr.ReqEndLine, tt.startLine, tt.endLine)
				}
			}
		})
	}
}

// editToolForAutoindent builds a UnifiedEditTool with preview + autoindent on.
func editToolForAutoindent(t *testing.T, tmpDir string) (*UnifiedEditTool, *ToolContext) {
	t.Helper()
	cfg := newTestEditConfig(tmpDir)
	cfg.Tools.Edit.PreviewMode = true
	cfg.Tools.Edit.SmartFirstLineIndent = true
	cfg.Tools.Edit.MaxAutoindentFix = 1
	toolCtx := NewToolContext()
	tool := NewUnifiedEditTool(cfg, toolCtx)
	return tool, toolCtx
}

func TestAutoindentPreviewAndUndo(t *testing.T) {
	tmpDir := t.TempDir()
	tool, toolCtx := editToolForAutoindent(t, tmpDir)

	// Seed a file with an 8-space-indented body line.
	content := "def f():\n        validate()\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "a.py"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Model under-indents the first line by one (7 spaces, should be 8).
	args := mustJSON(map[string]any{
		"path":       "a.py",
		"start_line": 2,
		"end_line":   2,
		"new_text":   "       validate_input()",
	})
	res, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("Edit call: %v", err)
	}
	m := res.(map[string]any)
	if m["status"] != "pending_confirmation" {
		t.Fatalf("expected pending_confirmation, got %v", m["status"])
	}
	if m["indent_autocorrected"] == nil {
		t.Fatalf("expected indent_autocorrected note, result: %v", m)
	}

	// Pending edit should carry the corrected content (8 spaces).
	pending := toolCtx.GetPendingEdit()
	if pending == nil || pending.autoindent == nil {
		t.Fatalf("expected pending edit with autoindent, got %+v", pending)
	}
	if !strings.Contains(pending.newContent, "        validate_input()") {
		t.Errorf("pending content not corrected to 8 spaces: %q", pending.newContent)
	}

	// Now undo the auto-correction.
	undo := NewUndoAutoindentTool(tool.Config, toolCtx)
	ures, err := undo.Call(context.Background(), mustJSON(map[string]any{}))
	if err != nil {
		t.Fatalf("undo call: %v", err)
	}
	um := ures.(map[string]any)
	if um["status"] != "pending_confirmation" {
		t.Fatalf("expected fresh pending_confirmation after undo, got %v", um)
	}

	// After undo, the pending edit uses the model's ORIGINAL indentation (7 spaces)
	// and the correction is suppressed.
	pending = toolCtx.GetPendingEdit()
	if pending == nil {
		t.Fatal("expected pending edit after undo")
	}
	if pending.autoindent != nil {
		t.Error("autoindent should be suppressed after undo")
	}
	if !strings.Contains(pending.newContent, "       validate_input()") {
		t.Errorf("pending content after undo should use original 7 spaces: %q", pending.newContent)
	}

	// Confirm applies the model's original indentation verbatim.
	confirm := NewConfirmEditTool(tool.Config, toolCtx)
	if _, err := confirm.Call(context.Background(), mustJSON(map[string]any{})); err != nil {
		t.Fatalf("confirm call: %v", err)
	}
	gotBytes, err := os.ReadFile(filepath.Join(tmpDir, "a.py"))
	if err != nil {
		t.Fatal(err)
	}
	got := string(gotBytes)
	if !strings.Contains(got, "       validate_input()") {
		t.Errorf("applied file should have original 7-space indent: %q", got)
	}
}

func TestUndoAutoindentNoPendingAndNotCorrected(t *testing.T) {
	tmpDir := t.TempDir()
	tool, toolCtx := editToolForAutoindent(t, tmpDir)
	undo := NewUndoAutoindentTool(tool.Config, toolCtx)

	// No pending edit -> friendly error.
	res, err := undo.Call(context.Background(), mustJSON(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	if res.(map[string]any)["error"] != "no_pending_operation" {
		t.Errorf("expected no_pending_operation, got %v", res)
	}

	// Pending edit that was NOT auto-corrected -> friendly no-op.
	content := "def f():\n    return 41\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "b.py"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	args := mustJSON(map[string]any{
		"path":       "b.py",
		"start_line": 2,
		"end_line":   2,
		"new_text":   "    return 42", // already correct
	})
	if _, err := tool.Call(context.Background(), args); err != nil {
		t.Fatal(err)
	}
	res, err = undo.Call(context.Background(), mustJSON(map[string]any{}))
	if err != nil {
		t.Fatal(err)
	}
	rm := res.(map[string]any)
	if rm["success"] != true {
		t.Errorf("expected friendly no-op success, got %v", rm)
	}
	// Pending edit should remain intact (undo is a no-op, not a resolve).
	if toolCtx.GetPendingEdit() == nil {
		t.Error("pending edit should remain after no-op undo")
	}
}

// --- small test helpers ---

func mustJSON(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

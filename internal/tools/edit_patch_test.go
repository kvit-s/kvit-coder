package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

func TestNewPatchEditTool(t *testing.T) {
	cfg := &config.Config{}
	cfg.Workspace.Root = "/test"
	cfg.Tools.Edit.Enabled = true
	cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)

	tool := NewPatchEditTool(cfg)

	if tool.Name() != "Edit" {
		t.Errorf("Name() = %q, want 'Edit'", tool.Name())
	}
	if tool.Description() == "" {
		t.Error("Description() should not be empty")
	}
	if tool.PromptCategory() != "filesystem" {
		t.Errorf("PromptCategory() = %q, want 'filesystem'", tool.PromptCategory())
	}
}

func TestPatchEditTool_JSONSchema(t *testing.T) {
	cfg := &config.Config{}
	cfg.Workspace.Root = "/test"
	cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)

	tool := NewPatchEditTool(cfg)
	schema := tool.JSONSchema()

	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema should have properties")
	}

	if _, exists := props["patch"]; !exists {
		t.Error("schema missing 'patch' property")
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("schema should have required array")
	}
	if len(required) != 1 || required[0] != "patch" {
		t.Errorf("required should be ['patch'], got %v", required)
	}
}

func TestPatchEditTool_ParsePatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	tests := []struct {
		name          string
		patch         string
		wantFiles     int
		wantActions   []PatchAction
		wantErr       bool
	}{
		{
			name: "single file update",
			patch: `*** Begin Patch
*** Update File: test.txt
@@ main
 context line
-old line
+new line
 more context
*** End Patch`,
			wantFiles:   1,
			wantActions: []PatchAction{PatchUpdate},
			wantErr:     false,
		},
		{
			name: "add new file",
			patch: `*** Begin Patch
*** Add File: new.txt
+line 1
+line 2
+line 3
*** End Patch`,
			wantFiles:   1,
			wantActions: []PatchAction{PatchAdd},
			wantErr:     false,
		},
		{
			name: "delete file",
			patch: `*** Begin Patch
*** Delete File: old.txt
*** End Patch`,
			wantFiles:   1,
			wantActions: []PatchAction{PatchDelete},
			wantErr:     false,
		},
		{
			name: "multiple files",
			patch: `*** Begin Patch
*** Update File: file1.txt
 context
-old
+new
*** Update File: file2.txt
 other context
-remove this
+add this
*** End Patch`,
			wantFiles:   2,
			wantActions: []PatchAction{PatchUpdate, PatchUpdate},
			wantErr:     false,
		},
		{
			name: "mixed operations",
			patch: `*** Begin Patch
*** Add File: new.txt
+new content
*** Update File: existing.txt
 context
-old
+new
*** Delete File: obsolete.txt
*** End Patch`,
			wantFiles:   3,
			wantActions: []PatchAction{PatchAdd, PatchUpdate, PatchDelete},
			wantErr:     false,
		},
		{
			name: "invalid line prefix",
			patch: `*** Begin Patch
*** Update File: test.txt
invalid prefix line
*** End Patch`,
			wantFiles: 0,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patches, err := tool.parsePatch(tt.patch)
			if (err != nil) != tt.wantErr {
				t.Errorf("parsePatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			if len(patches) != tt.wantFiles {
				t.Errorf("parsePatch() got %d files, want %d", len(patches), tt.wantFiles)
				return
			}

			for i, patch := range patches {
				if patch.Action != tt.wantActions[i] {
					t.Errorf("file %d action = %v, want %v", i, patch.Action, tt.wantActions[i])
				}
			}
		})
	}
}

func TestPatchEditTool_ApplyUpdatePatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	t.Run("simple update", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "update_simple.txt")
		content := "line1\nold_line\nline3\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		patch := `*** Begin Patch
*** Update File: ` + testFile + `
 line1
-old_line
+new_line
 line3
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatal(err)
		}
		expected := "line1\nnew_line\nline3\n"
		if string(data) != expected {
			t.Errorf("file content = %q, want %q", string(data), expected)
		}
	})

	t.Run("multiple changes in one file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "update_multi.txt")
		content := "func foo() {\n    old1\n}\n\nfunc bar() {\n    old2\n}\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		patch := `*** Begin Patch
*** Update File: ` + testFile + `
 func foo() {
-    old1
+    new1
 }
@@ func bar
 func bar() {
-    old2
+    new2
 }
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatal(err)
		}
		expected := "func foo() {\n    new1\n}\n\nfunc bar() {\n    new2\n}\n"
		if string(data) != expected {
			t.Errorf("file content = %q, want %q", string(data), expected)
		}
	})

	t.Run("add multiple lines", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "add_lines.txt")
		content := "before\nmarker\nafter\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		patch := `*** Begin Patch
*** Update File: ` + testFile + `
 before
-marker
+new1
+new2
+new3
 after
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatal(err)
		}
		expected := "before\nnew1\nnew2\nnew3\nafter\n"
		if string(data) != expected {
			t.Errorf("file content = %q, want %q", string(data), expected)
		}
	})

	t.Run("delete multiple lines", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "delete_lines.txt")
		content := "keep1\ndelete1\ndelete2\ndelete3\nkeep2\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		patch := `*** Begin Patch
*** Update File: ` + testFile + `
 keep1
-delete1
-delete2
-delete3
 keep2
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatal(err)
		}
		expected := "keep1\nkeep2\n"
		if string(data) != expected {
			t.Errorf("file content = %q, want %q", string(data), expected)
		}
	})
}

func TestPatchEditTool_ApplyAddFilePatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	t.Run("add new file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "brand_new.txt")

		patch := `*** Begin Patch
*** Add File: ` + testFile + `
+line 1
+line 2
+line 3
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("file should have been created: %v", err)
		}
		expected := "line 1\nline 2\nline 3\n"
		if string(data) != expected {
			t.Errorf("file content = %q, want %q", string(data), expected)
		}
	})

	t.Run("add file in nested directory", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "new", "nested", "file.txt")

		patch := `*** Begin Patch
*** Add File: ` + testFile + `
+nested content
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("file should have been created: %v", err)
		}
		if string(data) != "nested content\n" {
			t.Errorf("file content = %q, want 'nested content\\n'", string(data))
		}
	})

	t.Run("add file that already exists fails", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "already_exists.txt")
		if err := os.WriteFile(testFile, []byte("existing\n"), 0644); err != nil {
			t.Fatal(err)
		}

		patch := `*** Begin Patch
*** Add File: ` + testFile + `
+new content
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != false {
			t.Errorf("expected success=false when adding existing file, got %v", resultMap)
		}
	})
}

func TestPatchEditTool_ApplyDeleteFilePatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	t.Run("delete existing file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "to_delete.txt")
		if err := os.WriteFile(testFile, []byte("content\n"), 0644); err != nil {
			t.Fatal(err)
		}

		patch := `*** Begin Patch
*** Delete File: ` + testFile + `
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap)
		}

		if _, err := os.Stat(testFile); !os.IsNotExist(err) {
			t.Error("file should have been deleted")
		}
	})

	t.Run("delete non-existent file fails", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "does_not_exist.txt")

		patch := `*** Begin Patch
*** Delete File: ` + testFile + `
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != false {
			t.Errorf("expected success=false when deleting non-existent file, got %v", resultMap)
		}
	})
}

func TestPatchEditTool_MultiFilePatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("file1 old\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("file2 old\n"), 0644); err != nil {
		t.Fatal(err)
	}

	patch := `*** Begin Patch
*** Update File: ` + file1 + `
-file1 old
+file1 new
*** Update File: ` + file2 + `
-file2 old
+file2 new
*** End Patch`

	args, _ := json.Marshal(map[string]string{"patch": patch})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != true {
		t.Errorf("expected success=true, got %v", resultMap)
	}
	if resultMap["files"] != 2 {
		t.Errorf("expected files=2, got %v", resultMap["files"])
	}

	data1, _ := os.ReadFile(file1)
	if string(data1) != "file1 new\n" {
		t.Errorf("file1 content = %q, want 'file1 new\\n'", string(data1))
	}

	data2, _ := os.ReadFile(file2)
	if string(data2) != "file2 new\n" {
		t.Errorf("file2 content = %q, want 'file2 new\\n'", string(data2))
	}
}

func TestPatchEditTool_ContextMatching(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	t.Run("context with trailing whitespace", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "trailing_ws.txt")
		// File has trailing spaces
		content := "context1  \nold_line\ncontext2  \n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		// Patch context without trailing spaces should match
		patch := `*** Begin Patch
*** Update File: ` + testFile + `
 context1
-old_line
+new_line
 context2
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true with trailing ws normalization, got %v", resultMap)
		}
	})

	t.Run("scope marker helps locate changes", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "scope.txt")
		content := "func foo() {\n    line1\n}\n\nfunc bar() {\n    old\n}\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		// Use scope marker to locate the change
		patch := `*** Begin Patch
*** Update File: ` + testFile + `
@@ func bar
 func bar() {
-    old
+    new
 }
*** End Patch`

		args, _ := json.Marshal(map[string]string{"patch": patch})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(data), "new") {
			t.Errorf("expected 'new' in file content, got %q", string(data))
		}
	})
}

func TestPatchEditTool_EmptyPatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	args, _ := json.Marshal(map[string]string{"patch": ""})

	_, err := tool.Call(context.Background(), args)
	if err == nil {
		t.Error("expected error for empty patch")
	}
}

func TestPatchEditTool_NoOperations(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	patch := `*** Begin Patch
*** End Patch`

	args, _ := json.Marshal(map[string]string{"patch": patch})

	_, err := tool.Call(context.Background(), args)
	if err == nil {
		t.Error("expected error for patch with no operations")
	}
}

func TestPatchEditTool_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	_, err := tool.Call(context.Background(), []byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestPatchEditTool_ContextMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	testFile := filepath.Join(tmpDir, "mismatch.txt")
	content := "actual line1\nactual line2\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Patch with wrong context
	patch := `*** Begin Patch
*** Update File: ` + testFile + `
 wrong context
-actual line2
+new line
*** End Patch`

	args, _ := json.Marshal(map[string]string{"patch": patch})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Errorf("expected success=false when context doesn't match, got %v", resultMap)
	}
}

func TestPatchEditTool_DeletionMismatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	testFile := filepath.Join(tmpDir, "deletion_mismatch.txt")
	content := "line1\nactual content\nline3\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Patch tries to delete wrong content
	patch := `*** Begin Patch
*** Update File: ` + testFile + `
 line1
-wrong content to delete
+new content
 line3
*** End Patch`

	args, _ := json.Marshal(map[string]string{"patch": patch})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Errorf("expected success=false when deletion content doesn't match, got %v", resultMap)
	}
}

func TestPatchEditTool_UpdateNonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	testFile := filepath.Join(tmpDir, "nonexistent.txt")

	patch := `*** Begin Patch
*** Update File: ` + testFile + `
 context
-old
+new
*** End Patch`

	args, _ := json.Marshal(map[string]string{"patch": patch})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Errorf("expected success=false when updating non-existent file, got %v", resultMap)
	}
}

func TestPatchEditTool_PromptSection(t *testing.T) {
	t.Run("without preview mode", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Workspace.Root = "/test"
		cfg.Tools.Edit.PreviewMode = false
		cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)

		tool := NewPatchEditTool(cfg)
		section := tool.PromptSection()

		if strings.Contains(section, "Preview Mode") {
			t.Error("prompt section should not mention preview mode when disabled")
		}
	})

	t.Run("with preview mode", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Workspace.Root = "/test"
		cfg.Tools.Edit.PreviewMode = true
		cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)

		tool := NewPatchEditTool(cfg)
		section := tool.PromptSection()

		if !strings.Contains(section, "Preview Mode") {
			t.Error("prompt section should mention preview mode when enabled")
		}
	})
}

func TestPatchEditTool_DiffOutput(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	testFile := filepath.Join(tmpDir, "diff_output.txt")
	content := "line1\nold_line\nline3\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	patch := `*** Begin Patch
*** Update File: ` + testFile + `
 line1
-old_line
+new_line
 line3
*** End Patch`

	args, _ := json.Marshal(map[string]string{"patch": patch})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	diff, ok := resultMap["diff"].(string)
	if !ok {
		t.Fatal("expected diff in result")
	}

	if !strings.Contains(diff, "-old_line") {
		t.Error("diff should contain removed line marker")
	}
	if !strings.Contains(diff, "+new_line") {
		t.Error("diff should contain added line marker")
	}
}

func TestPatchEditTool_Check(t *testing.T) {
	cfg := &config.Config{}
	cfg.Workspace.Root = "/test"
	cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)

	tool := NewPatchEditTool(cfg)

	// Patch mode doesn't require read-before-edit check
	err := tool.Check(context.Background(), []byte(`{"patch": "test"}`))
	if err != nil {
		t.Errorf("Check() should return nil for patch mode, got %v", err)
	}
}

func TestMatchContext(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	fileLines := []string{
		"package main",
		"",
		"func main() {",
		"    fmt.Println(\"hello\")",
		"}",
	}

	tests := []struct {
		name         string
		contextLines []string
		fuzz         int
		wantPos      int
	}{
		{
			name:         "exact match at start",
			contextLines: []string{"package main", ""},
			fuzz:         0,
			wantPos:      0,
		},
		{
			name:         "exact match in middle",
			contextLines: []string{"func main() {", "    fmt.Println(\"hello\")"},
			fuzz:         0,
			wantPos:      2,
		},
		{
			name:         "match with trailing space normalization",
			contextLines: []string{"package main"},
			fuzz:         1,
			wantPos:      0,
		},
		{
			name:         "no match",
			contextLines: []string{"not in file"},
			fuzz:         0,
			wantPos:      -1,
		},
		{
			name:         "empty context",
			contextLines: []string{},
			fuzz:         0,
			wantPos:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := tool.matchContext(fileLines, tt.contextLines, tt.fuzz)
			if pos != tt.wantPos {
				t.Errorf("matchContext() = %d, want %d", pos, tt.wantPos)
			}
		})
	}
}

func TestFindScope(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	lines := []string{
		"package main",
		"",
		"func foo() {",
		"    // foo body",
		"}",
		"",
		"func bar() {",
		"    // bar body",
		"}",
	}

	tests := []struct {
		name    string
		scope   string
		wantPos int
	}{
		{
			name:    "find func foo",
			scope:   "func foo",
			wantPos: 2,
		},
		{
			name:    "find func bar",
			scope:   "func bar",
			wantPos: 6,
		},
		{
			name:    "case insensitive match",
			scope:   "FUNC FOO",
			wantPos: 2,
		},
		{
			name:    "partial match",
			scope:   "bar",
			wantPos: 6,
		},
		{
			name:    "not found",
			scope:   "func baz",
			wantPos: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos := tool.findScope(lines, tt.scope)
			if pos != tt.wantPos {
				t.Errorf("findScope() = %d, want %d", pos, tt.wantPos)
			}
		})
	}
}

func TestApplyChunk(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewPatchEditTool(cfg)

	t.Run("simple replacement", func(t *testing.T) {
		lines := []string{"a", "b", "c", "d"}
		chunk := PatchChunk{
			Deletions: []string{"b"},
			Additions: []string{"B"},
		}

		result, err := tool.applyChunk(lines, chunk, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []string{"a", "B", "c", "d"}
		if len(result) != len(expected) {
			t.Fatalf("result length = %d, want %d", len(result), len(expected))
		}
		for i, line := range result {
			if line != expected[i] {
				t.Errorf("result[%d] = %q, want %q", i, line, expected[i])
			}
		}
	})

	t.Run("add lines", func(t *testing.T) {
		lines := []string{"a", "b", "c"}
		chunk := PatchChunk{
			Deletions: []string{"b"},
			Additions: []string{"b1", "b2", "b3"},
		}

		result, err := tool.applyChunk(lines, chunk, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []string{"a", "b1", "b2", "b3", "c"}
		if len(result) != len(expected) {
			t.Fatalf("result length = %d, want %d", len(result), len(expected))
		}
	})

	t.Run("delete lines", func(t *testing.T) {
		lines := []string{"a", "b1", "b2", "b3", "c"}
		chunk := PatchChunk{
			Deletions: []string{"b1", "b2", "b3"},
			Additions: []string{},
		}

		result, err := tool.applyChunk(lines, chunk, 1)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		expected := []string{"a", "c"}
		if len(result) != len(expected) {
			t.Fatalf("result length = %d, want %d", len(result), len(expected))
		}
	})

	t.Run("deletion mismatch error", func(t *testing.T) {
		lines := []string{"a", "b", "c"}
		chunk := PatchChunk{
			Deletions: []string{"wrong"},
			Additions: []string{"new"},
		}

		_, err := tool.applyChunk(lines, chunk, 1)
		if err == nil {
			t.Error("expected error for deletion mismatch")
		}
	})
}

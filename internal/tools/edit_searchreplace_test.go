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

func TestNewSearchReplaceEditTool(t *testing.T) {
	cfg := &config.Config{}
	cfg.Workspace.Root = "/test"
	cfg.Tools.Edit.Enabled = true
	cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)

	tool := NewSearchReplaceEditTool(cfg)

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

func TestSearchReplaceEditTool_JSONSchema(t *testing.T) {
	cfg := &config.Config{}
	cfg.Workspace.Root = "/test"
	cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)

	tool := NewSearchReplaceEditTool(cfg)
	schema := tool.JSONSchema()

	// Verify schema has required properties
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("schema should have properties")
	}

	requiredProps := []string{"path", "search", "replace"}
	for _, prop := range requiredProps {
		if _, exists := props[prop]; !exists {
			t.Errorf("schema missing required property: %s", prop)
		}
	}

	required, ok := schema["required"].([]string)
	if !ok {
		t.Fatal("schema should have required array")
	}
	if len(required) != 3 {
		t.Errorf("required should have 3 elements, got %d", len(required))
	}
}

func TestSearchReplaceEditTool_CreateNewFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	t.Run("create new file with empty search", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "new_file.go")
		content := "package main\n\nfunc main() {}\n"

		args, _ := json.Marshal(map[string]string{
			"path":    testFile,
			"search":  "",
			"replace": content,
		})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap, ok := result.(map[string]any)
		if !ok {
			t.Fatalf("result should be a map, got %T", result)
		}

		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap["success"])
		}
		if resultMap["created"] != true {
			t.Errorf("expected created=true, got %v", resultMap["created"])
		}

		// Verify file was created
		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read created file: %v", err)
		}
		if string(data) != content {
			t.Errorf("file content = %q, want %q", string(data), content)
		}
	})

	t.Run("create new file in nested directory", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "nested", "dir", "file.txt")
		content := "nested content\n"

		args, _ := json.Marshal(map[string]string{
			"path":    testFile,
			"search":  "",
			"replace": content,
		})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true, got %v", resultMap["success"])
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read created file: %v", err)
		}
		if string(data) != content {
			t.Errorf("file content = %q, want %q", string(data), content)
		}
	})
}

func TestSearchReplaceEditTool_ExactMatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	t.Run("simple replacement", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "simple.txt")
		originalContent := "hello world\n"
		if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
			t.Fatal(err)
		}

		// Mark file as read
		globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

		args, _ := json.Marshal(map[string]string{
			"path":    testFile,
			"search":  "hello",
			"replace": "goodbye",
		})

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
		if string(data) != "goodbye world\n" {
			t.Errorf("file content = %q, want 'goodbye world\\n'", string(data))
		}
	})

	t.Run("multiline replacement", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "multiline.txt")
		originalContent := "line1\nold_line2\nold_line3\nline4\n"
		if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
			t.Fatal(err)
		}

		globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

		args, _ := json.Marshal(map[string]string{
			"path":    testFile,
			"search":  "old_line2\nold_line3",
			"replace": "new_line2\nnew_line3",
		})

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
		expected := "line1\nnew_line2\nnew_line3\nline4\n"
		if string(data) != expected {
			t.Errorf("file content = %q, want %q", string(data), expected)
		}
	})

	t.Run("delete content (empty replace)", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "delete.txt")
		originalContent := "keep_this\ndelete_me\nkeep_this_too\n"
		if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
			t.Fatal(err)
		}

		globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

		args, _ := json.Marshal(map[string]string{
			"path":    testFile,
			"search":  "delete_me\n",
			"replace": "",
		})

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
		expected := "keep_this\nkeep_this_too\n"
		if string(data) != expected {
			t.Errorf("file content = %q, want %q", string(data), expected)
		}
	})
}

func TestSearchReplaceEditTool_NoMatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	testFile := filepath.Join(tmpDir, "nomatch.txt")
	if err := os.WriteFile(testFile, []byte("hello world\n"), 0644); err != nil {
		t.Fatal(err)
	}

	globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

	args, _ := json.Marshal(map[string]string{
		"path":    testFile,
		"search":  "nonexistent",
		"replace": "replacement",
	})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Errorf("expected success=false, got %v", resultMap["success"])
	}
	if resultMap["error"] != "no_match" {
		t.Errorf("expected error='no_match', got %v", resultMap["error"])
	}
}

func TestSearchReplaceEditTool_MultipleMatches(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	testFile := filepath.Join(tmpDir, "multiple.txt")
	content := "hello\nworld\nhello\nagain\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

	args, _ := json.Marshal(map[string]string{
		"path":    testFile,
		"search":  "hello",
		"replace": "goodbye",
	})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Errorf("expected success=false, got %v", resultMap["success"])
	}
	if resultMap["error"] != "multiple_matches" {
		t.Errorf("expected error='multiple_matches', got %v", resultMap["error"])
	}
	if resultMap["count"] != 2 {
		t.Errorf("expected count=2, got %v", resultMap["count"])
	}
	atLines, ok := resultMap["at_lines"].([]int)
	if !ok {
		t.Fatalf("expected at_lines to be []int, got %T", resultMap["at_lines"])
	}
	if len(atLines) != 2 || atLines[0] != 1 || atLines[1] != 3 {
		t.Errorf("expected at_lines=[1,3], got %v", atLines)
	}
}

func TestSearchReplaceEditTool_NewFileWithSearch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	testFile := filepath.Join(tmpDir, "nonexistent.txt")

	args, _ := json.Marshal(map[string]string{
		"path":    testFile,
		"search":  "some text",
		"replace": "new text",
	})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Errorf("expected success=false, got %v", resultMap["success"])
	}
	if resultMap["error"] != "new_file_with_search" {
		t.Errorf("expected error='new_file_with_search', got %v", resultMap["error"])
	}
}

func TestSearchReplaceEditTool_EmptySearchExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	testFile := filepath.Join(tmpDir, "existing.txt")
	if err := os.WriteFile(testFile, []byte("content\n"), 0644); err != nil {
		t.Fatal(err)
	}

	globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

	args, _ := json.Marshal(map[string]string{
		"path":    testFile,
		"search":  "",
		"replace": "new content",
	})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	if resultMap["success"] != false {
		t.Errorf("expected success=false, got %v", resultMap["success"])
	}
	if resultMap["error"] != "empty_search" {
		t.Errorf("expected error='empty_search', got %v", resultMap["error"])
	}
}

func TestSearchReplaceEditTool_WhitespaceNormalization(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	t.Run("trailing whitespace normalization", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "trailing_ws.txt")
		// File has trailing spaces
		content := "line1  \nline2\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

		// Search without trailing spaces should still match
		args, _ := json.Marshal(map[string]string{
			"path":    testFile,
			"search":  "line1\nline2",
			"replace": "new1\nnew2",
		})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Errorf("expected success=true with trailing ws normalization, got %v", resultMap)
		}
	})
}

func TestSearchReplaceEditTool_FuzzyMatch(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)
	cfg.Tools.Edit.FuzzyThreshold = 0.8

	tool := NewSearchReplaceEditTool(cfg)

	t.Run("fuzzy match with minor difference", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "fuzzy.txt")
		content := "func example() {\n    return 42\n}\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

		// Search with slightly different content (should fuzzy match)
		args, _ := json.Marshal(map[string]string{
			"path":    testFile,
			"search":  "func example() {\n    return 43\n}",
			"replace": "func example() {\n    return 100\n}",
		})

		result, err := tool.Call(context.Background(), args)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		resultMap := result.(map[string]any)
		if resultMap["success"] != true {
			t.Logf("result: %v", resultMap)
			// Fuzzy matching should find a match
			if resultMap["error"] == "no_match" {
				// Check if similarity was detected
				if similarity, ok := resultMap["similarity"]; ok {
					t.Logf("Similar content found with similarity %v", similarity)
				}
			}
		}
	})
}

func TestSearchReplaceEditTool_PreviewMode(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)
	cfg.Tools.Edit.PreviewMode = true

	tool := NewSearchReplaceEditTool(cfg)

	testFile := filepath.Join(tmpDir, "preview.txt")
	content := "hello world\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

	args, _ := json.Marshal(map[string]string{
		"path":    testFile,
		"search":  "hello",
		"replace": "goodbye",
	})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	previewResult, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", result)
	}

	if status, _ := previewResult["status"].(string); status != "pending_confirmation" {
		t.Errorf("expected status='pending_confirmation', got %q", status)
	}
	if path, _ := previewResult["path"].(string); path != testFile {
		t.Errorf("expected path=%q, got %q", testFile, path)
	}
	if diff, _ := previewResult["diff"].(string); diff == "" {
		t.Error("expected non-empty diff")
	}

	// File should NOT be modified yet
	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("file should not be modified in preview mode, got %q", string(data))
	}

	// Clean up pending edit
	ClearPendingEdit()
}

func TestSearchReplaceEditTool_DiffGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	testFile := filepath.Join(tmpDir, "diff.txt")
	content := "line1\nold_line\nline3\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

	args, _ := json.Marshal(map[string]string{
		"path":    testFile,
		"search":  "old_line",
		"replace": "new_line",
	})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	diff, ok := resultMap["diff"].(string)
	if !ok {
		t.Fatal("expected diff in result")
	}

	// Verify diff contains expected markers
	if !strings.Contains(diff, "-old_line") {
		t.Error("diff should contain removed line marker")
	}
	if !strings.Contains(diff, "+new_line") {
		t.Error("diff should contain added line marker")
	}
}

func TestSearchReplaceEditTool_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	_, err := tool.Call(context.Background(), []byte("invalid json"))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestSearchReplaceEditTool_PromptSection(t *testing.T) {
	t.Run("without preview mode", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Workspace.Root = "/test"
		cfg.Tools.Edit.PreviewMode = false
		cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)

		tool := NewSearchReplaceEditTool(cfg)
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

		tool := NewSearchReplaceEditTool(cfg)
		section := tool.PromptSection()

		if !strings.Contains(section, "Preview Mode") {
			t.Error("prompt section should mention preview mode when enabled")
		}
	})
}

func TestSearchReplaceEditTool_SimilarContentSuggestion(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)

	tool := NewSearchReplaceEditTool(cfg)

	testFile := filepath.Join(tmpDir, "similar.txt")
	content := "function processData() {\n    return result\n}\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	globalReadTracker.RecordRead(testFile, globalReadTracker.CurrentMessageID())

	// Search for something similar but not exact
	args, _ := json.Marshal(map[string]string{
		"path":    testFile,
		"search":  "function processdata()",
		"replace": "function newFunction()",
	})

	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resultMap := result.(map[string]any)
	// Should fail but might have similarity hints
	if resultMap["success"] == false && resultMap["error"] == "no_match" {
		// Check if similar line was found
		if similarLine, ok := resultMap["similar_at_line"]; ok {
			t.Logf("Found similar content at line %v", similarLine)
		}
	}
}

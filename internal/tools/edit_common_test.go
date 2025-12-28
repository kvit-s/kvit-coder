package tools

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// newTestEditConfig creates a config for edit tool tests
func newTestEditConfig(tmpDir string) *config.Config {
	cfg := &config.Config{}
	cfg.Workspace.Root = tmpDir
	cfg.Tools.Edit.Enabled = true
	cfg.Tools.Edit.FuzzyThreshold = 0.8
	cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)
	return cfg
}

func TestBaseEditTool_ValidateAndResolvePath(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)
	base := &BaseEditTool{
		Config:        cfg,
		WorkspaceRoot: tmpDir,
	}

	// Test absolute path in workspace
	t.Run("absolute path in workspace", func(t *testing.T) {
		testPath := filepath.Join(tmpDir, "test.txt")
		fullPath, outside, err := base.ValidateAndResolvePath(testPath)
		if err != nil {
			t.Errorf("ValidateAndResolvePath() error = %v, want nil", err)
			return
		}
		if outside {
			t.Error("ValidateAndResolvePath() outside = true, want false")
		}
		if fullPath == "" {
			t.Error("ValidateAndResolvePath() returned empty fullPath")
		}
	})

	// Test nested path in workspace
	t.Run("nested absolute path in workspace", func(t *testing.T) {
		testPath := filepath.Join(tmpDir, "dir", "subdir", "test.txt")
		fullPath, outside, err := base.ValidateAndResolvePath(testPath)
		if err != nil {
			t.Errorf("ValidateAndResolvePath() error = %v, want nil", err)
			return
		}
		if outside {
			t.Error("ValidateAndResolvePath() outside = true, want false")
		}
		if fullPath == "" {
			t.Error("ValidateAndResolvePath() returned empty fullPath")
		}
	})
}

func TestBaseEditTool_ReadFileForEdit(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)
	base := &BaseEditTool{
		Config:        cfg,
		WorkspaceRoot: tmpDir,
	}

	t.Run("existing file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "existing.txt")
		expectedContent := "hello world\n"
		if err := os.WriteFile(testFile, []byte(expectedContent), 0644); err != nil {
			t.Fatal(err)
		}

		content, isNew, err := base.ReadFileForEdit(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isNew {
			t.Error("expected isNew=false for existing file")
		}
		if content != expectedContent {
			t.Errorf("content = %q, want %q", content, expectedContent)
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "nonexistent.txt")

		content, isNew, err := base.ReadFileForEdit(testFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isNew {
			t.Error("expected isNew=true for non-existent file")
		}
		if content != "" {
			t.Errorf("content = %q, want empty", content)
		}
	})
}

func TestBaseEditTool_WriteFileAtomic(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)
	base := &BaseEditTool{
		Config:        cfg,
		WorkspaceRoot: tmpDir,
	}

	t.Run("write new file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "new_file.txt")
		content := "new content\n"

		err := base.WriteFileAtomic(testFile, content, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify file was written
		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(data) != content {
			t.Errorf("file content = %q, want %q", string(data), content)
		}
	})

	t.Run("overwrite existing file", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "existing_file.txt")
		if err := os.WriteFile(testFile, []byte("old content\n"), 0644); err != nil {
			t.Fatal(err)
		}

		newContent := "updated content\n"
		err := base.WriteFileAtomic(testFile, newContent, false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(data) != newContent {
			t.Errorf("file content = %q, want %q", string(data), newContent)
		}
	})

	t.Run("create parent directories", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "nested", "dirs", "file.txt")
		content := "nested file content\n"

		err := base.WriteFileAtomic(testFile, content, true)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(data) != content {
			t.Errorf("file content = %q, want %q", string(data), content)
		}
	})

	t.Run("preserves file permissions", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "perms_file.txt")
		if err := os.WriteFile(testFile, []byte("original\n"), 0755); err != nil {
			t.Fatal(err)
		}

		err := base.WriteFileAtomic(testFile, "updated\n", false)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(testFile)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		// Check that execute bit is preserved (0755 on unix)
		if info.Mode().Perm()&0100 == 0 {
			t.Error("expected execute permission to be preserved")
		}
	})
}

func TestNormalizeWhitespace(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no whitespace",
			input: "hello\nworld",
			want:  "hello\nworld",
		},
		{
			name:  "trailing spaces",
			input: "hello   \nworld  ",
			want:  "hello\nworld",
		},
		{
			name:  "leading spaces",
			input: "   hello\n  world",
			want:  "hello\nworld",
		},
		{
			name:  "both leading and trailing",
			input: "  hello  \n  world  ",
			want:  "hello\nworld",
		},
		{
			name:  "tabs",
			input: "\thello\t\n\tworld\t",
			want:  "hello\nworld",
		},
		{
			name:  "mixed whitespace",
			input: " \t hello \t \n \t world \t ",
			want:  "hello\nworld",
		},
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "only whitespace lines",
			input: "   \n\t\t\n  \t  ",
			want:  "\n\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeWhitespace() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestNormalizeWhitespaceRstrip(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "no whitespace",
			input: "hello\nworld",
			want:  "hello\nworld",
		},
		{
			name:  "trailing spaces only",
			input: "hello   \nworld  ",
			want:  "hello\nworld",
		},
		{
			name:  "leading spaces preserved",
			input: "   hello\n  world",
			want:  "   hello\n  world",
		},
		{
			name:  "mixed - trailing removed, leading preserved",
			input: "  hello  \n  world  ",
			want:  "  hello\n  world",
		},
		{
			name:  "tabs",
			input: "\thello\t\n\tworld\t",
			want:  "\thello\n\tworld",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeWhitespaceRstrip(tt.input)
			if got != tt.want {
				t.Errorf("NormalizeWhitespaceRstrip() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCountMatches(t *testing.T) {
	tests := []struct {
		name    string
		content string
		search  string
		want    int
	}{
		{
			name:    "no matches",
			content: "hello world",
			search:  "foo",
			want:    0,
		},
		{
			name:    "one match",
			content: "hello world",
			search:  "hello",
			want:    1,
		},
		{
			name:    "multiple matches",
			content: "hello hello hello",
			search:  "hello",
			want:    3,
		},
		{
			name:    "overlapping pattern",
			content: "aaaa",
			search:  "aa",
			want:    2, // non-overlapping matches
		},
		{
			name:    "empty search",
			content: "hello",
			search:  "",
			want:    6, // empty string matches at every position + end
		},
		{
			name:    "multiline match",
			content: "line1\nline2\nline1",
			search:  "line1",
			want:    2,
		},
		{
			name:    "match at boundaries",
			content: "foobar foo",
			search:  "foo",
			want:    2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CountMatches(tt.content, tt.search)
			if got != tt.want {
				t.Errorf("CountMatches() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestFindMatchPosition(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		search    string
		wantStart int
		wantEnd   int
		wantFound bool
	}{
		{
			name:      "match at start",
			content:   "hello world",
			search:    "hello",
			wantStart: 0,
			wantEnd:   5,
			wantFound: true,
		},
		{
			name:      "match at end",
			content:   "hello world",
			search:    "world",
			wantStart: 6,
			wantEnd:   11,
			wantFound: true,
		},
		{
			name:      "match in middle",
			content:   "hello beautiful world",
			search:    "beautiful",
			wantStart: 6,
			wantEnd:   15,
			wantFound: true,
		},
		{
			name:      "no match",
			content:   "hello world",
			search:    "foo",
			wantStart: 0,
			wantEnd:   0,
			wantFound: false,
		},
		{
			name:      "multiline match",
			content:   "line1\nline2\nline3",
			search:    "line2\nline3",
			wantStart: 6,
			wantEnd:   17,
			wantFound: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, found := FindMatchPosition(tt.content, tt.search)
			if found != tt.wantFound {
				t.Errorf("FindMatchPosition() found = %v, want %v", found, tt.wantFound)
			}
			if start != tt.wantStart {
				t.Errorf("FindMatchPosition() start = %d, want %d", start, tt.wantStart)
			}
			if end != tt.wantEnd {
				t.Errorf("FindMatchPosition() end = %d, want %d", end, tt.wantEnd)
			}
		})
	}
}

func TestGetLineNumber(t *testing.T) {
	content := "line1\nline2\nline3\nline4"

	tests := []struct {
		name       string
		byteOffset int
		wantLine   int
	}{
		{
			name:       "first line start",
			byteOffset: 0,
			wantLine:   1,
		},
		{
			name:       "first line middle",
			byteOffset: 3,
			wantLine:   1,
		},
		{
			name:       "second line start",
			byteOffset: 6,
			wantLine:   2,
		},
		{
			name:       "third line",
			byteOffset: 12,
			wantLine:   3,
		},
		{
			name:       "fourth line",
			byteOffset: 18,
			wantLine:   4,
		},
		{
			name:       "beyond end",
			byteOffset: 100,
			wantLine:   4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetLineNumber(content, tt.byteOffset)
			if got != tt.wantLine {
				t.Errorf("GetLineNumber() = %d, want %d", got, tt.wantLine)
			}
		})
	}
}

func TestGetContextAroundPosition(t *testing.T) {
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7"

	tests := []struct {
		name         string
		position     int
		contextLines int
		wantContains []string
	}{
		{
			name:         "middle of file",
			position:     18, // line4
			contextLines: 1,
			wantContains: []string{"line3", "line4", "line5"},
		},
		{
			name:         "start of file",
			position:     0, // line1
			contextLines: 2,
			wantContains: []string{"line1", "line2", "line3"},
		},
		{
			name:         "end of file",
			position:     36, // line7
			contextLines: 2,
			wantContains: []string{"line5", "line6", "line7"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetContextAroundPosition(content, tt.position, tt.contextLines)
			for _, want := range tt.wantContains {
				if !contains(got, want) {
					t.Errorf("GetContextAroundPosition() = %q, want to contain %q", got, want)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestIsLargeFile(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("small file", func(t *testing.T) {
		smallFile := filepath.Join(tmpDir, "small.txt")
		if err := os.WriteFile(smallFile, []byte("small content"), 0644); err != nil {
			t.Fatal(err)
		}

		isLarge, size, err := IsLargeFile(smallFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isLarge {
			t.Error("expected isLarge=false for small file")
		}
		if size == 0 {
			t.Error("expected non-zero size")
		}
	})

	t.Run("non-existent file", func(t *testing.T) {
		isLarge, size, err := IsLargeFile(filepath.Join(tmpDir, "nonexistent.txt"))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isLarge {
			t.Error("expected isLarge=false for non-existent file")
		}
		if size != 0 {
			t.Errorf("expected size=0, got %d", size)
		}
	})

	t.Run("file at threshold", func(t *testing.T) {
		// Create a file just under the threshold
		underThresholdFile := filepath.Join(tmpDir, "under_threshold.txt")
		content := make([]byte, LargeFileThreshold-1)
		if err := os.WriteFile(underThresholdFile, content, 0644); err != nil {
			t.Fatal(err)
		}

		isLarge, _, err := IsLargeFile(underThresholdFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if isLarge {
			t.Error("expected isLarge=false for file under threshold")
		}

		// Create a file just over the threshold
		overThresholdFile := filepath.Join(tmpDir, "over_threshold.txt")
		content = make([]byte, LargeFileThreshold+1)
		if err := os.WriteFile(overThresholdFile, content, 0644); err != nil {
			t.Fatal(err)
		}

		isLarge, _, err = IsLargeFile(overThresholdFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !isLarge {
			t.Error("expected isLarge=true for file over threshold")
		}
	})
}

func TestReadLineRange(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a test file with numbered lines
	testFile := filepath.Join(tmpDir, "lines.txt")
	var content string
	for i := 1; i <= 100; i++ {
		content += "line" + string(rune('0'+i/10)) + string(rune('0'+i%10)) + "\n"
	}
	// Use proper formatting
	content = ""
	for i := 1; i <= 100; i++ {
		content += fmt.Sprintf("line%02d\n", i)
	}
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("read middle range", func(t *testing.T) {
		rangeContent, totalLines, err := ReadLineRange(testFile, 10, 15)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if totalLines != 100 {
			t.Errorf("totalLines = %d, want 100", totalLines)
		}
		if rangeContent != "line10\nline11\nline12\nline13\nline14\nline15" {
			t.Errorf("rangeContent = %q", rangeContent)
		}
	})

	t.Run("read from start", func(t *testing.T) {
		rangeContent, totalLines, err := ReadLineRange(testFile, 1, 3)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if totalLines != 100 {
			t.Errorf("totalLines = %d, want 100", totalLines)
		}
		if rangeContent != "line01\nline02\nline03" {
			t.Errorf("rangeContent = %q", rangeContent)
		}
	})

	t.Run("read to end", func(t *testing.T) {
		rangeContent, totalLines, err := ReadLineRange(testFile, 98, 100)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if totalLines != 100 {
			t.Errorf("totalLines = %d, want 100", totalLines)
		}
		if rangeContent != "line98\nline99\nline100" {
			t.Errorf("rangeContent = %q", rangeContent)
		}
	})

	t.Run("read beyond end", func(t *testing.T) {
		rangeContent, totalLines, err := ReadLineRange(testFile, 98, 200)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if totalLines != 100 {
			t.Errorf("totalLines = %d, want 100", totalLines)
		}
		// Should only get lines 98-100
		if rangeContent != "line98\nline99\nline100" {
			t.Errorf("rangeContent = %q", rangeContent)
		}
	})

	t.Run("single line", func(t *testing.T) {
		rangeContent, _, err := ReadLineRange(testFile, 50, 50)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if rangeContent != "line50" {
			t.Errorf("rangeContent = %q, want 'line50'", rangeContent)
		}
	})
}

func TestStreamingLineReplace(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newTestEditConfig(tmpDir)
	base := &BaseEditTool{
		Config:        cfg,
		WorkspaceRoot: tmpDir,
	}

	t.Run("replace middle lines", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "replace_middle.txt")
		content := "line1\nline2\nline3\nline4\nline5\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		// Replace lines 2-3 with new content
		err := base.StreamingLineReplace(testFile, 2, 3, "newline2\nnewline3")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}
		expected := "line1\nnewline2\nnewline3\nline4\nline5\n"
		if string(result) != expected {
			t.Errorf("result = %q, want %q", string(result), expected)
		}
	})

	t.Run("replace first lines", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "replace_first.txt")
		content := "line1\nline2\nline3\nline4\nline5\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		err := base.StreamingLineReplace(testFile, 1, 2, "newline1\nnewline2")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}
		expected := "newline1\nnewline2\nline3\nline4\nline5\n"
		if string(result) != expected {
			t.Errorf("result = %q, want %q", string(result), expected)
		}
	})

	t.Run("replace last lines", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "replace_last.txt")
		content := "line1\nline2\nline3\nline4\nline5\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		err := base.StreamingLineReplace(testFile, 4, 5, "newline4\nnewline5\n")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}
		expected := "line1\nline2\nline3\nnewline4\nnewline5\n"
		if string(result) != expected {
			t.Errorf("result = %q, want %q", string(result), expected)
		}
	})

	t.Run("add more lines than removed", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "add_lines.txt")
		content := "line1\nline2\nline3\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		// Replace line 2 with 3 lines
		err := base.StreamingLineReplace(testFile, 2, 2, "newA\nnewB\nnewC")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}
		expected := "line1\nnewA\nnewB\nnewC\nline3\n"
		if string(result) != expected {
			t.Errorf("result = %q, want %q", string(result), expected)
		}
	})

	t.Run("remove more lines than added", func(t *testing.T) {
		testFile := filepath.Join(tmpDir, "remove_lines.txt")
		content := "line1\nline2\nline3\nline4\nline5\n"
		if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		// Replace lines 2-4 with 1 line
		err := base.StreamingLineReplace(testFile, 2, 4, "newline")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		result, err := os.ReadFile(testFile)
		if err != nil {
			t.Fatalf("failed to read result: %v", err)
		}
		expected := "line1\nnewline\nline5\n"
		if string(result) != expected {
			t.Errorf("result = %q, want %q", string(result), expected)
		}
	})
}

func TestPendingEdit(t *testing.T) {
	// Test StorePendingEdit and ClearPendingEditForPath
	t.Run("store and clear", func(t *testing.T) {
		// Store a pending edit (with edit line range 1-1)
		StorePendingEdit("test.txt", "/full/path/test.txt", "old", "new", "diff", false, 1, 1)

		// Verify it was stored
		path := GetPendingEditPath()
		if path != "test.txt" {
			t.Errorf("GetPendingEditPath() = %q, want 'test.txt'", path)
		}

		// Clear it
		ClearPendingEditForPath("test.txt")

		// Verify it was cleared
		path = GetPendingEditPath()
		if path != "" {
			t.Errorf("GetPendingEditPath() after clear = %q, want empty", path)
		}
	})

	t.Run("clear wrong path does nothing", func(t *testing.T) {
		StorePendingEdit("test.txt", "/full/path/test.txt", "old", "new", "diff", false, 1, 1)

		// Clear a different path
		ClearPendingEditForPath("other.txt")

		// Original should still be there
		path := GetPendingEditPath()
		if path != "test.txt" {
			t.Errorf("GetPendingEditPath() = %q, want 'test.txt'", path)
		}

		// Clean up
		ClearPendingEditForPath("test.txt")
	})
}

func TestGeneratePostEditContext(t *testing.T) {
	// Helper to create file content with numbered lines
	makeContent := func(numLines int) string {
		var lines []string
		for i := 1; i <= numLines; i++ {
			lines = append(lines, fmt.Sprintf("line%d", i))
		}
		return join(lines, "\n")
	}

	t.Run("empty content", func(t *testing.T) {
		result := GeneratePostEditContext("", 1, 1)
		if result != "" {
			t.Errorf("expected empty string for empty content, got %q", result)
		}
	})

	t.Run("single line file", func(t *testing.T) {
		result := GeneratePostEditContext("only line", 1, 1)
		// Should just show the one line with edit marker
		if !containsStr(result, ">") || !containsStr(result, "1│only line") {
			t.Errorf("unexpected result for single line: %q", result)
		}
		// Should not contain "..."
		if containsStr(result, "...") {
			t.Errorf("single line should not have ellipsis: %q", result)
		}
	})

	t.Run("small file shows all lines", func(t *testing.T) {
		content := makeContent(5)
		result := GeneratePostEditContext(content, 3, 3)
		// All 5 lines should be shown, no ellipsis
		for i := 1; i <= 5; i++ {
			if !containsStr(result, fmt.Sprintf("line%d", i)) {
				t.Errorf("missing line%d in result: %q", i, result)
			}
		}
		if containsStr(result, "...") {
			t.Errorf("small file should not have ellipsis: %q", result)
		}
	})

	t.Run("edit at start contextStart equals 1", func(t *testing.T) {
		// Edit at line 2, context starts at max(1, 2-3) = 1
		content := makeContent(20)
		result := GeneratePostEditContext(content, 2, 3)
		// Should show lines 1-6 (edit lines 2-3 plus 3 after)
		// No line 1 shown separately, no "..." at top
		if containsStr(result, "...\n 1│") {
			t.Errorf("contextStart=1 should not show line 1 separately: %q", result)
		}
		// Line 1 should be in the main context
		if !containsStr(result, "1│line1") {
			t.Errorf("missing line1 in context: %q", result)
		}
	})

	t.Run("edit where contextStart equals 2", func(t *testing.T) {
		// Edit at line 5, context starts at 5-3 = 2
		content := makeContent(20)
		result := GeneratePostEditContext(content, 5, 5)
		// Should show: line 1, then lines 2-8 (no "..." because line 2 follows line 1)
		if !containsStr(result, "1│line1") {
			t.Errorf("should show line 1: %q", result)
		}
		if containsStr(result, "...\n") && containsStr(result, "2│line2") {
			// Check if "..." appears between line 1 and line 2
			lines := splitLines(result)
			for i, line := range lines {
				if containsStr(line, "1│line1") && i+1 < len(lines) && lines[i+1] == "..." {
					t.Errorf("should not have ... between line 1 and line 2: %q", result)
				}
			}
		}
	})

	t.Run("edit where contextStart equals 3 shows line 2", func(t *testing.T) {
		// Edit at line 6, context starts at 6-3 = 3
		content := makeContent(20)
		result := GeneratePostEditContext(content, 6, 6)
		// Should show: line 1, line 2, lines 3-9 (no "..." because we show line 2)
		if !containsStr(result, "1│line1") {
			t.Errorf("should show line 1: %q", result)
		}
		if !containsStr(result, "2│line2") {
			t.Errorf("should show line 2: %q", result)
		}
		// No "..." between line 1 and the context
		lines := splitLines(result)
		foundEllipsisBeforeContext := false
		for i, line := range lines {
			if line == "..." && i > 0 && containsStr(lines[i-1], "line1") {
				foundEllipsisBeforeContext = true
			}
			if line == "..." && i > 0 && containsStr(lines[i-1], "line2") {
				foundEllipsisBeforeContext = true
			}
		}
		if foundEllipsisBeforeContext {
			t.Errorf("should not have ... when contextStart=3: %q", result)
		}
	})

	t.Run("edit where contextStart greater than 3 shows ellipsis", func(t *testing.T) {
		// Edit at line 10, context starts at 10-3 = 7
		content := makeContent(20)
		result := GeneratePostEditContext(content, 10, 10)
		// Should show: line 1, "...", lines 7-13, "...", line 20
		if !containsStr(result, "1│line1") {
			t.Errorf("should show line 1: %q", result)
		}
		// Should have "..." after line 1
		lines := splitLines(result)
		foundEllipsisAfterLine1 := false
		for i, line := range lines {
			if containsStr(line, "1│line1") && i+1 < len(lines) && lines[i+1] == "..." {
				foundEllipsisAfterLine1 = true
			}
		}
		if !foundEllipsisAfterLine1 {
			t.Errorf("should have ... after line 1 when contextStart>3: %q", result)
		}
	})

	t.Run("edit where contextEnd equals totalLines", func(t *testing.T) {
		// Edit at line 18 in 20-line file, context ends at min(20, 18+3) = 20
		content := makeContent(20)
		result := GeneratePostEditContext(content, 18, 20)
		// No footer needed since context includes last line
		// Count how many times "..." appears - should only be at top
		ellipsisCount := countOccurrences(result, "...")
		if ellipsisCount > 1 {
			t.Errorf("should only have ellipsis at top, found %d: %q", ellipsisCount, result)
		}
	})

	t.Run("edit where contextEnd equals totalLines minus 1", func(t *testing.T) {
		// Edit at line 16 in 20-line file, context ends at 16+3 = 19
		content := makeContent(20)
		result := GeneratePostEditContext(content, 16, 16)
		// contextEnd = 19, totalLines = 20
		// Should show last line without "..." before it
		if !containsStr(result, "20│line20") {
			t.Errorf("should show last line: %q", result)
		}
		// Check no "..." immediately before last line
		lines := splitLines(result)
		for i, line := range lines {
			if containsStr(line, "20│line20") && i > 0 && lines[i-1] == "..." {
				t.Errorf("should not have ... before last line when contextEnd=totalLines-1: %q", result)
			}
		}
	})

	t.Run("edit where contextEnd equals totalLines minus 2 shows second to last", func(t *testing.T) {
		// Edit at line 15 in 20-line file, context ends at 15+3 = 18
		content := makeContent(20)
		result := GeneratePostEditContext(content, 15, 15)
		// contextEnd = 18, totalLines = 20
		// Should show lines 19 and 20 without "..." between them
		if !containsStr(result, "19│line19") {
			t.Errorf("should show second-to-last line: %q", result)
		}
		if !containsStr(result, "20│line20") {
			t.Errorf("should show last line: %q", result)
		}
		// Check no "..." between line 18 and line 19
		lines := splitLines(result)
		for i, line := range lines {
			if containsStr(line, "18│line18") && i+1 < len(lines) && lines[i+1] == "..." {
				t.Errorf("should not have ... after context when contextEnd=totalLines-2: %q", result)
			}
		}
	})

	t.Run("edit where contextEnd less than totalLines minus 2 shows ellipsis", func(t *testing.T) {
		// Edit at line 10 in 20-line file, context ends at 10+3 = 13
		content := makeContent(20)
		result := GeneratePostEditContext(content, 10, 10)
		// contextEnd = 13, totalLines = 20
		// Should show "..." and then last line
		if !containsStr(result, "20│line20") {
			t.Errorf("should show last line: %q", result)
		}
		// Check "..." appears before last line
		lines := splitLines(result)
		foundEllipsisBeforeLast := false
		for i, line := range lines {
			if containsStr(line, "20│line20") && i > 0 && lines[i-1] == "..." {
				foundEllipsisBeforeLast = true
			}
		}
		if !foundEllipsisBeforeLast {
			t.Errorf("should have ... before last line when contextEnd<totalLines-2: %q", result)
		}
	})

	t.Run("edit markers on correct lines", func(t *testing.T) {
		content := makeContent(20)
		result := GeneratePostEditContext(content, 10, 12)
		lines := splitLines(result)
		for _, line := range lines {
			if containsStr(line, "10│line10") || containsStr(line, "11│line11") || containsStr(line, "12│line12") {
				if line[0] != '>' {
					t.Errorf("edited line should have > marker: %q", line)
				}
			} else if len(line) > 0 && line != "..." {
				if line[0] == '>' {
					t.Errorf("non-edited line should not have > marker: %q", line)
				}
			}
		}
	})

	t.Run("line numbers right aligned", func(t *testing.T) {
		content := makeContent(200)
		result := GeneratePostEditContext(content, 100, 100)
		// With 200 lines, line numbers need 3 digits
		// Line 1 should be padded: "  1│"
		if !containsStr(result, "  1│line1") {
			t.Errorf("line 1 should be right-aligned with padding: %q", result)
		}
		// Line 100 should not need padding: "100│"
		if !containsStr(result, "100│line100") {
			t.Errorf("line 100 should be present: %q", result)
		}
	})

	t.Run("full format middle of large file", func(t *testing.T) {
		content := makeContent(100)
		result := GeneratePostEditContext(content, 50, 51)
		// Should have: line 1, "...", context (47-54), "...", line 100
		if !containsStr(result, "  1│line1") {
			t.Errorf("should show line 1: %q", result)
		}
		if !containsStr(result, "100│line100") {
			t.Errorf("should show line 100: %q", result)
		}
		// Should have exactly 2 "..."
		ellipsisCount := countOccurrences(result, "...")
		if ellipsisCount != 2 {
			t.Errorf("should have exactly 2 ellipsis, found %d: %q", ellipsisCount, result)
		}
		// Edit markers on lines 50 and 51
		if !containsStr(result, "> 50│line50") {
			t.Errorf("line 50 should have edit marker: %q", result)
		}
		if !containsStr(result, "> 51│line51") {
			t.Errorf("line 51 should have edit marker: %q", result)
		}
	})

	t.Run("user reported scenario - edit lines 1-3 in 9 line file", func(t *testing.T) {
		// Simulates: original 10-line file, lines 1-5 replaced with 3 lines
		// Result is 8 lines (but counting trailing empty = 9)
		content := "function calculate(): number {\n    return 42;\n}\n}\n\nfunction other(): number {\n    return 0;\n}\n"
		// Edit replaced lines 1-5 with 3-line new text, so edited region is lines 1-3
		result := GeneratePostEditContext(content, 1, 3)
		t.Logf("Result:\n%s", result)

		lines := splitLines(result)

		// Should have > markers on lines 1, 2, 3
		foundEditedLines := 0
		for _, line := range lines {
			if len(line) > 0 && line[0] == '>' {
				foundEditedLines++
			}
		}
		if foundEditedLines != 3 {
			t.Errorf("should have 3 edited lines with > marker, found %d:\n%s", foundEditedLines, result)
		}

		// Should show lines 1-6 (3 edited + 3 context after)
		// With 9 total lines, contextEnd = min(9, 3+3) = 6
		if !containsStr(result, "1│function calculate") {
			t.Errorf("should show line 1: %s", result)
		}
		if !containsStr(result, "4│}") {
			t.Errorf("should show line 4 (first context after edit): %s", result)
		}
		if !containsStr(result, "5│") {
			t.Errorf("should show line 5: %s", result)
		}
		if !containsStr(result, "6│function other") {
			t.Errorf("should show line 6: %s", result)
		}
	})
}

// Helper functions for tests
func join(strs []string, sep string) string {
	if len(strs) == 0 {
		return ""
	}
	result := strs[0]
	for i := 1; i < len(strs); i++ {
		result += sep + strs[i]
	}
	return result
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			lines = append(lines, s[start:i])
			start = i + 1
		}
	}
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func countOccurrences(s, substr string) int {
	count := 0
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			count++
			i += len(substr) - 1
		}
	}
	return count
}

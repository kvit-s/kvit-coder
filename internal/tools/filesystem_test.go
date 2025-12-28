package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStreamReadLines(t *testing.T) {
	// Create temp file with 10 lines
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		startLine  int
		endLine    int
		wantLines  []string
		wantTotal  int
	}{
		{
			name:      "read first 3 lines",
			startLine: 1,
			endLine:   3,
			wantLines: []string{"line1", "line2", "line3"},
			wantTotal: 10,
		},
		{
			name:      "read middle lines",
			startLine: 4,
			endLine:   6,
			wantLines: []string{"line4", "line5", "line6"},
			wantTotal: 10,
		},
		{
			name:      "read last lines",
			startLine: 8,
			endLine:   10,
			wantLines: []string{"line8", "line9", "line10"},
			wantTotal: 10,
		},
		{
			name:      "start beyond file",
			startLine: 15,
			endLine:   20,
			wantLines: nil,
			wantTotal: 10,
		},
		{
			name:      "end beyond file",
			startLine: 8,
			endLine:   20,
			wantLines: []string{"line8", "line9", "line10"},
			wantTotal: 10,
		},
		{
			name:      "start and end beyond file",
			startLine: 100,
			endLine:   200,
			wantLines: nil,
			wantTotal: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := streamReadLines(testFile, tt.startLine, tt.endLine, 1024*1024) // 1MB limit for tests
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.TotalLines != tt.wantTotal {
				t.Errorf("TotalLines = %d, want %d", result.TotalLines, tt.wantTotal)
			}

			if len(result.Lines) != len(tt.wantLines) {
				t.Errorf("got %d lines, want %d", len(result.Lines), len(tt.wantLines))
				return
			}

			for i, line := range result.Lines {
				if line != tt.wantLines[i] {
					t.Errorf("line %d = %q, want %q", i, line, tt.wantLines[i])
				}
			}
		})
	}
}

func TestStreamLastNLines(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		content   string
		n         int
		wantLines []string
		wantTotal int
	}{
		{
			name:      "last 3 of 10 lines",
			content:   "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n",
			n:         3,
			wantLines: []string{"line8", "line9", "line10"},
			wantTotal: 10,
		},
		{
			name:      "last 50 but file has 10 lines",
			content:   "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n",
			n:         50,
			wantLines: []string{"line1", "line2", "line3", "line4", "line5", "line6", "line7", "line8", "line9", "line10"},
			wantTotal: 10,
		},
		{
			name:      "last 5 of 5 lines (exact match)",
			content:   "a\nb\nc\nd\ne\n",
			n:         5,
			wantLines: []string{"a", "b", "c", "d", "e"},
			wantTotal: 5,
		},
		{
			name:      "last 1 line",
			content:   "line1\nline2\nline3\n",
			n:         1,
			wantLines: []string{"line3"},
			wantTotal: 3,
		},
		{
			name:      "empty file",
			content:   "",
			n:         10,
			wantLines: nil,
			wantTotal: 0,
		},
		{
			name:      "single line no newline",
			content:   "only line",
			n:         5,
			wantLines: []string{"only line"},
			wantTotal: 1,
		},
		{
			name:      "file without trailing newline",
			content:   "line1\nline2\nline3",
			n:         2,
			wantLines: []string{"line2", "line3"},
			wantTotal: 3,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, tt.name+".txt")
			if err := os.WriteFile(testFile, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			result, err := streamLastNLines(testFile, tt.n, 1024*1024)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if result.TotalLines != tt.wantTotal {
				t.Errorf("totalLines = %d, want %d", result.TotalLines, tt.wantTotal)
			}

			if len(result.Lines) != len(tt.wantLines) {
				t.Errorf("got %d lines, want %d: %v", len(result.Lines), len(tt.wantLines), result.Lines)
				return
			}

			for i, line := range result.Lines {
				if line != tt.wantLines[i] {
					t.Errorf("line %d = %q, want %q", i, line, tt.wantLines[i])
				}
			}
		})
	}
}

func TestStreamLastNLinesLargeFile(t *testing.T) {
	// Test with a larger file to verify circular buffer works correctly
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create file with 1000 lines
	var content string
	for i := 1; i <= 1000; i++ {
		content += "line" + string(rune('0'+i%10)) + "\n"
	}
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := streamLastNLines(testFile, 5, 1024*1024)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalLines != 1000 {
		t.Errorf("totalLines = %d, want 1000", result.TotalLines)
	}

	if len(result.Lines) != 5 {
		t.Errorf("got %d lines, want 5", len(result.Lines))
	}
}

func TestAutoCharModeSwitch(t *testing.T) {
	// Test that when lines are too large, we auto-switch to char_mode output
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large_lines.txt")

	// Create file with a very long line (exceeds typical maxBytes)
	longLine := strings.Repeat("x", 30000) // 30KB line
	content := "line1\n" + longLine + "\nline3\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	result, err := streamReadLines(testFile, 1, 3, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}

	// Verify we got all 3 lines
	if len(result.Lines) != 3 {
		t.Errorf("expected 3 lines, got %d", len(result.Lines))
	}

	// Verify we have byte positions for auto char_mode switch
	if len(result.LineByteStarts) != 3 {
		t.Errorf("expected 3 byte positions, got %d", len(result.LineByteStarts))
	}

	// Verify byte positions are correct
	if result.LineByteStarts[0] != 0 {
		t.Errorf("expected first line at byte 0, got %d", result.LineByteStarts[0])
	}
	if result.LineByteStarts[1] != 6 { // "line1\n" = 6 bytes
		t.Errorf("expected second line at byte 6, got %d", result.LineByteStarts[1])
	}
}

func TestStreamReadLinesMemorySafe(t *testing.T) {
	// Test that we stop collecting content when maxBytes is exceeded
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "large.txt")

	// Create file with lines that exceed a small maxBytes limit
	content := "line1\nline2\nline3\nline4\nline5\n" // 30 bytes total
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Read with a very small maxBytes limit (15 bytes)
	result, err := streamReadLines(testFile, 1, 5, 15)
	if err != nil {
		t.Fatal(err)
	}

	// Should have stopped early
	if !result.ContentTruncated {
		t.Error("expected ContentTruncated=true")
	}

	// Should not have all 5 lines (only ~2 lines fit in 15 bytes)
	if len(result.Lines) >= 5 {
		t.Errorf("expected fewer than 5 lines due to truncation, got %d", len(result.Lines))
	}

	// But should still know total line count
	if result.TotalLines != 5 {
		t.Errorf("expected TotalLines=5, got %d", result.TotalLines)
	}

	// Should have byte positions for the lines we did collect
	if len(result.LineByteStarts) != len(result.Lines) {
		t.Errorf("LineByteStarts length mismatch: %d vs %d lines", len(result.LineByteStarts), len(result.Lines))
	}
}

func TestStreamReadLinesReturnsHints(t *testing.T) {
	// Verify that edge cases return appropriate data for hints
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")

	// Create 10-line file
	content := "line1\nline2\nline3\nline4\nline5\nline6\nline7\nline8\nline9\nline10\n"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Test start beyond file - should return empty lines but correct totalLines
	result, err := streamReadLines(testFile, 100, 200, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Lines) != 0 {
		t.Errorf("expected 0 lines for start beyond file, got %d", len(result.Lines))
	}
	if result.TotalLines != 10 {
		t.Errorf("expected totalLines=10, got %d", result.TotalLines)
	}

	// Test empty file
	emptyFile := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(emptyFile, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}

	result, err = streamReadLines(emptyFile, 1, 10, 1024*1024)
	if err != nil {
		t.Fatal(err)
	}
	if result.TotalLines != 0 {
		t.Errorf("expected totalLines=0 for empty file, got %d", result.TotalLines)
	}
}

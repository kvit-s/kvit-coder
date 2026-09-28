package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CRLF fixtures: every file a Windows editor touched has them. Decision is
// preserve-and-count-on-\n: line numbers count \n and stay stable, bytes are
// otherwise untouched.
func TestCRLFHelpers(t *testing.T) {
	if !hasCRLF("a\r\nb\r\n") || hasCRLF("a\nb\n") {
		t.Error("hasCRLF misdetects endings")
	}
	if got := normalizeForMatch("a\r\nb\r\n"); got != "a\nb\n" {
		t.Errorf("normalizeForMatch = %q", got)
	}
	if got := prevailingEnding("a\r\nb"); got != "\r\n" {
		t.Errorf("prevailingEnding CRLF = %q", got)
	}
	if got := prevailingEnding("a\nb"); got != "\n" {
		t.Errorf("prevailingEnding LF = %q", got)
	}
	if got := adaptReplacement("x\ny\n", "a\r\nb\r\n"); got != "x\r\ny\r\n" {
		t.Errorf("adaptReplacement to CRLF = %q", got)
	}
	if got := adaptReplacement("x\ny\n", "a\nb\n"); got != "x\ny\n" {
		t.Errorf("adaptReplacement to LF changed bytes: %q", got)
	}
}

func TestMatchWithNormalizationCRLF(t *testing.T) {
	content := "line one\r\nline two\r\nline three\r\n"
	start, end, _, found := MatchWithNormalization(content, "line two\nline three", 0, true)
	if !found {
		t.Fatal("CRLF-normalized exact match not found")
	}
	if got := content[start:end]; got != "line two\r\nline three" {
		t.Errorf("mapped back to %q, want CRLF bytes", got)
	}
}

func TestSearchReplaceCRLFPreservesEndings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "crlf.txt")
	original := "first\r\nsecond\r\nthird\r\n"
	if err := os.WriteFile(path, []byte(original), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := newTestEditConfig(dir)
	tool := NewSearchReplaceEditTool(cfg, NewToolContext())

	args, _ := json.Marshal(map[string]string{
		"path": "crlf.txt", "search": "second\n", "replace": "SECOND\n",
	})
	res, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	_ = res
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "first\r\nSECOND\r\nthird\r\n" {
		t.Errorf("CRLF round-trip = %q", data)
	}
	// Line numbers count \n and stay stable.
	if lines := strings.Split(string(data), "\n"); len(lines) != 4 {
		t.Errorf("line count via \\n = %d, want 4 (3 lines + trailing)", len(lines))
	}
}

func TestSearchGoFindsPattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "node_modules"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package x\n// needle here\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "node_modules", "skip.go"), []byte("needle skipped\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfg := newTestEditConfig(dir)
	// SearchTool reads workspace root from cfg; tempFileMgr nil is fine here.
	tool := NewSearchTool(cfg, nil)
	matches, err := tool.searchWithGo(context.Background(), "needle", dir, "*.go", 1)
	if err != nil {
		t.Fatalf("searchWithGo: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("matches = %d, want 1 (node_modules excluded)", len(matches))
	}
	if !strings.HasSuffix(matches[0].File, "a.go") || matches[0].Line != 2 {
		t.Errorf("match = %+v, want a.go:2", matches[0])
	}
}

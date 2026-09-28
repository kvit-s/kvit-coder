package tools

import (
	"os"
	"strings"
	"testing"
)

// A small diff travels whole: no truncation, no file.
func TestPrepareDiffSmall(t *testing.T) {
	tool := NewTasksFinishTool(nil, nil)
	diff := "diff --git a/a.go b/a.go\n+line\n"
	got, file, truncated, lines, length := tool.prepareDiff(diff, true)
	if truncated {
		t.Fatal("small diff was truncated")
	}
	if got != diff {
		t.Errorf("small diff changed:\n%q", got)
	}
	if file != "" {
		t.Errorf("small diff spilled to %q", file)
	}
	if lines == 0 || length == 0 {
		t.Errorf("small diff totals missing: lines=%d bytes=%d", lines, length)
	}
}

// No changes means nothing to truncate.
func TestPrepareDiffNoChanges(t *testing.T) {
	tool := NewTasksFinishTool(nil, nil)
	got, file, truncated, _, _ := tool.prepareDiff("", false)
	if truncated || got != "" || file != "" {
		t.Errorf("empty diff produced output: truncated=%v file=%q got=%q", truncated, file, got)
	}
}

// A large diff returns a preview and spills the full text to session tmp/
// for review, like shell/search output.
func TestPrepareDiffLargeSpillsToFile(t *testing.T) {
	mgr := NewTempFileManager(t.TempDir())
	tool := NewTasksFinishTool(nil, mgr)
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("diff --git a/file.go b/file.go: change number ")
		sb.WriteString(strings.Repeat("x", 20))
		sb.WriteString("\n")
	}
	full := sb.String()

	preview, file, truncated, totalLines, totalBytes := tool.prepareDiff(full, true)
	if !truncated {
		t.Fatal("large diff was not truncated")
	}
	if totalLines != 200 {
		t.Errorf("total lines = %d, want 200", totalLines)
	}
	if totalBytes != len(full) {
		t.Errorf("total bytes = %d, want %d", totalBytes, len(full))
	}
	if len(preview) >= len(full) {
		t.Errorf("preview (%d bytes) not shorter than full (%d)", len(preview), len(full))
	}
	if file == "" {
		t.Fatal("truncated diff has no file to review")
	}
	saved, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("cannot read spilled diff: %v", err)
	}
	if string(saved) != full {
		t.Error("spilled file does not hold the full diff")
	}
}

// Without a TempFileManager the diff still truncates, just with no file.
func TestPrepareDiffLargeWithoutTempManager(t *testing.T) {
	tool := NewTasksFinishTool(nil, nil)
	var sb strings.Builder
	for i := 0; i < 200; i++ {
		sb.WriteString("line\n")
	}
	full := sb.String()
	preview, file, truncated, totalLines, _ := tool.prepareDiff(full, true)
	if !truncated {
		t.Fatal("large diff was not truncated")
	}
	if file != "" {
		t.Errorf("nil manager spilled to %q", file)
	}
	if len(preview) >= len(full) {
		t.Error("preview not shorter than full")
	}
	if totalLines != 200 {
		t.Errorf("total lines = %d, want 200", totalLines)
	}
}

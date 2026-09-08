package tools

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// The grep fallback must speak the same regex dialect as ripgrep (extended
// regex: '|' is alternation, '\(' is a literal paren). Without -E, grep
// defaults to BRE where it is reversed, so a model-written pattern like
// `Open\(` fails with "Unmatched ( or \(" (exit status 2) instead of matching.
// Regression test for the Search failure on
// `FirstPrompt|Summary|Title|GenerateSessionName|Open\(|NewManager`.
func TestSearchWithGrepUsesExtendedRegex(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not available")
	}
	dir := t.TempDir()
	content := "package main\n\nfunc FirstPrompt() {}\nfunc Open(name string) {}\nvar NewManager = 1\n"
	target := filepath.Join(dir, "main.go")
	if err := os.WriteFile(target, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig()
	cfg.Workspace.Root = dir
	tool := NewSearchTool(cfg, nil)
	ctx := context.Background()

	// The exact shape of the reported failure: alternation plus an escaped
	// paren. Under BRE this exits 2 with "Unmatched ( or \("; under -E it
	// matches.
	out, err := tool.searchWithGrep(ctx, `FirstPrompt|Summary|Title|GenerateSessionName|Open\(|NewManager`, target, "", 0)
	if err != nil {
		t.Fatalf("searchWithGrep returned error (was exit status 2 under BRE): %v", err)
	}
	if !strings.Contains(string(out), "FirstPrompt") {
		t.Errorf("expected match for FirstPrompt, got:\n%s", out)
	}
	if !strings.Contains(string(out), "Open(") {
		t.Errorf("expected match for literal Open(, got:\n%s", out)
	}

	// Bare alternation must act as OR, not as a literal pipe.
	out, err = tool.searchWithGrep(ctx, "FirstPrompt|NewManager", target, "", 0)
	if err != nil {
		t.Fatalf("alternation search returned error: %v", err)
	}
	if !strings.Contains(string(out), "FirstPrompt") || !strings.Contains(string(out), "NewManager") {
		t.Errorf("expected both alternation branches to match, got:\n%s", out)
	}
}

// End to end through Call with the reported arguments: whatever backend is
// active (rg where installed, grep otherwise), the pattern must not fail
// with "exit status 2".
func TestSearchCallWithFunctionCallPattern(t *testing.T) {
	if _, err := exec.LookPath("grep"); err != nil {
		t.Skip("grep not available")
	}
	if _, err := exec.LookPath("rg"); err != nil && detectSearchBackend() == backendNone {
		t.Skip("no search backend available")
	}
	dir := t.TempDir()
	content := "package main\n\nfunc FirstPrompt() {}\nfunc Open(name string) {}\nvar NewManager = 1\n"
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg := newTestConfig()
	cfg.Workspace.Root = dir
	tool := NewSearchTool(cfg, nil)

	args, _ := json.Marshal(map[string]any{
		"pattern":       `FirstPrompt|Summary|Title|GenerateSessionName|Open\(|NewManager`,
		"path":          filepath.Join(dir, "main.go"),
		"context_lines": 1,
	})
	res, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("Call returned error (was 'search failed: exit status 2' under BRE): %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	if m["success"] != true {
		t.Fatalf("expected success, got: %v", res)
	}
	if n, ok := m["total_matches"].(int); !ok || n == 0 {
		t.Errorf("expected matches, got: %v", res)
	}
}

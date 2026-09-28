package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestMatchGlobSegments(t *testing.T) {
	tests := []struct {
		pattern string
		rel     string
		want    bool
	}{
		{"*.go", "main.go", true},
		{"*.go", "a/main.go", false},
		{"**/*.go", "main.go", true},
		{"**/*.go", "a/b/main.go", true},
		{"**/*.go", "a/b/main.py", false},
		{"src/**/*.ts", "src/a/b/c.ts", true},
		{"src/**/*.ts", "src/c.ts", true},
		{"src/**/*.ts", "other/c.ts", false},
		{"src/*", "src/a.go", true},
		{"src/*", "src/a/b.go", false},
		{"src/**", "src/a/b.go", true},
		{"*", "anything", true},
		{"**", "a/b/c", true},
		{"?.go", "a.go", true},
		{"?.go", "ab.go", false},
		{"*.{json,yaml}", "a.json", false}, // braces are literal in glob syntax
		{"[a-z].go", "m.go", true},
		{"[a-z].go", "5.go", false},
	}

	for _, tt := range tests {
		pat := splitSlash(normalizeGlobPattern(tt.pattern))
		rel := splitSlash(tt.rel)
		if got := matchGlobSegments(pat, rel); got != tt.want {
			t.Errorf("matchGlobSegments(%q, %q) = %v, want %v", tt.pattern, tt.rel, got, tt.want)
		}
	}
}

func splitSlash(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	return append(out, s[start:])
}

func TestNormalizeGlobPattern(t *testing.T) {
	tests := []struct{ in, want string }{
		{"", ""},
		{".", ""},
		{"./", ""},
		{"src/", "src/*"},
		{"**/*.go", "**/*.go"},
		{"/abs/path", "abs/path"},
		{`src\*.go`, "src/*.go"},
	}
	for _, tt := range tests {
		if got := normalizeGlobPattern(tt.in); got != tt.want {
			t.Errorf("normalizeGlobPattern(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

// globTestRoot builds a tree: main.go, src/a.go, src/nested/b.py,
// node_modules/dep/index.js (excluded), .git/config (excluded).
func globTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	files := []string{
		"main.go",
		"src/a.go",
		"src/nested/b.py",
		"node_modules/dep/index.js",
		".git/config",
	}
	for _, f := range files {
		p := filepath.Join(root, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func callGlob(t *testing.T, tool *GlobTool, args map[string]any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(args)
	res, err := tool.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	return m
}

func TestGlobListDirectory(t *testing.T) {
	root := globTestRoot(t)
	cfg := newTestConfig()
	cfg.Workspace.Root = root
	tool := NewGlobTool(cfg)

	m := callGlob(t, tool, map[string]any{"path": "."})
	if m["success"] != true {
		t.Fatalf("expected success, got: %v", m)
	}
	if m["type"] != "directory" {
		t.Fatalf("expected directory type, got: %v", m)
	}
	entries, ok := m["entries"].([]map[string]any)
	if !ok {
		t.Fatalf("entries has type %T, want []map[string]any", m["entries"])
	}
	// main.go, src/ first (dirs first), plus node_modules/ and .git/
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %v", entries)
	}
	if entries[0]["name"] != ".git/" || entries[0]["type"] != "dir" {
		t.Errorf("dirs first: entries[0] = %v", entries[0])
	}
	if entries[1]["name"] != "node_modules/" {
		t.Errorf("entries[1] = %v", entries[1])
	}
	if entries[2]["name"] != "src/" {
		t.Errorf("entries[2] = %v", entries[2])
	}
	if entries[3]["name"] != "main.go" {
		t.Errorf("entries[3] = %v", entries[3])
	}
}

func TestGlobPattern(t *testing.T) {
	root := globTestRoot(t)
	cfg := newTestConfig()
	cfg.Workspace.Root = root
	tool := NewGlobTool(cfg)

	m := callGlob(t, tool, map[string]any{"pattern": "**/*.go"})
	if m["success"] != true {
		t.Fatalf("expected success, got: %v", m)
	}
	files, ok := m["files"].([]string)
	if !ok {
		t.Fatalf("files has type %T", m["files"])
	}
	if len(files) != 2 || files[0] != "main.go" || files[1] != "src/a.go" {
		t.Errorf("unexpected files: %v", files)
	}

	// Excluded dirs are never walked, even when the pattern would match.
	m = callGlob(t, tool, map[string]any{"pattern": "**/*.js"})
	files, _ = m["files"].([]string)
	if len(files) != 0 {
		t.Errorf("node_modules must be excluded, got: %v", files)
	}

	// Scoped to a subdirectory.
	m = callGlob(t, tool, map[string]any{"pattern": "**/*.py", "path": "src"})
	files, _ = m["files"].([]string)
	if len(files) != 1 || files[0] != "src/nested/b.py" {
		t.Errorf("scoped search: got %v", files)
	}

	// No match is a success with an empty list.
	m = callGlob(t, tool, map[string]any{"pattern": "**/*.rs"})
	if m["success"] != true {
		t.Errorf("no-match must succeed, got: %v", m)
	}
	if n, ok := m["total_matches"].(int); !ok || n != 0 {
		t.Errorf("total_matches = %v", m["total_matches"])
	}
}

func TestGlobTruncation(t *testing.T) {
	root := t.TempDir()
	for _, f := range []string{"a.go", "b.go", "c.go"} {
		if err := os.WriteFile(filepath.Join(root, f), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := newTestConfig()
	cfg.Workspace.Root = root
	cfg.Tools.Glob.MaxResults = 2
	tool := NewGlobTool(cfg)

	m := callGlob(t, tool, map[string]any{"pattern": "*.go"})
	files, _ := m["files"].([]string)
	if len(files) != 2 {
		t.Fatalf("expected 2 shown, got: %v", files)
	}
	if m["truncated"] != true {
		t.Errorf("expected truncated flag, got: %v", m)
	}
	if n, ok := m["total_matches"].(int); !ok || n != 3 {
		t.Errorf("total_matches = %v, want 3", m["total_matches"])
	}
}

func TestGlobPathOnFile(t *testing.T) {
	root := globTestRoot(t)
	cfg := newTestConfig()
	cfg.Workspace.Root = root
	tool := NewGlobTool(cfg)

	m := callGlob(t, tool, map[string]any{"path": "main.go"})
	if m["success"] != false || m["error"] != "not_a_directory" {
		t.Errorf("expected not_a_directory, got: %v", m)
	}
}

func TestGlobRegisteredInSetup(t *testing.T) {
	cfg := newTestConfig()
	cfg.Workspace.Root = t.TempDir()
	cfg.Tools.Glob.Enabled = true
	r := SetupRegistry(SetupConfig{Cfg: cfg})
	if !r.IsEnabled("Glob") {
		t.Fatal("Glob not registered with tools.glob.enabled: true")
	}
	found := false
	for _, s := range r.Specs() {
		if s.Function.Name == "Glob" {
			found = true
		}
	}
	if !found {
		t.Error("Glob missing from advertised specs")
	}
}

// Read on a directory no longer lists it: it points at Glob.
func TestReadDirectoryPointsAtGlob(t *testing.T) {
	root := globTestRoot(t)
	cfg := newTestConfig()
	cfg.Workspace.Root = root
	tool := NewReadFileTool(cfg, NewToolContext())

	raw, _ := json.Marshal(map[string]any{"path": "src"})
	res, err := tool.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call returned error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	if m["success"] != false || m["error"] != "is_directory" {
		t.Fatalf("expected is_directory, got: %v", m)
	}
}

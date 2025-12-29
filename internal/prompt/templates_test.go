package prompt

import (
	"io/fs"
	"testing"
	"testing/fstest"
)

func TestTemplateEngine_RenderWithPointerContext(t *testing.T) {
	// Create a simple in-memory filesystem with a test template
	testFS := fstest.MapFS{
		"test.tmpl": &fstest.MapFile{
			Data: []byte("Capabilities: {{ .CapabilitiesString }}"),
		},
	}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	// Create context with capabilities
	ctx := &PromptContext{
		Capabilities: []string{"reading files", "editing files"},
	}

	// Render with pointer - this should work and call the pointer receiver method
	result, err := engine.Render("test.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Capabilities: reading files, editing files"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestTemplateEngine_RenderWithEmptyCapabilities(t *testing.T) {
	testFS := fstest.MapFS{
		"test.tmpl": &fstest.MapFile{
			Data: []byte("Capabilities: {{ .CapabilitiesString }}"),
		},
	}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	ctx := &PromptContext{
		Capabilities: []string{},
	}

	result, err := engine.Render("test.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Capabilities: various tools"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestTemplateEngine_RenderWithFields(t *testing.T) {
	testFS := fstest.MapFS{
		"test.tmpl": &fstest.MapFile{
			Data: []byte("Root: {{ .WorkspaceRoot }}, Edit: {{ .HasEdit }}, Mode: {{ .EditMode }}"),
		},
	}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	ctx := &PromptContext{
		WorkspaceRoot: "/home/user/project",
		HasEdit:       true,
		EditMode:      "lines",
	}

	result, err := engine.Render("test.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	expected := "Root: /home/user/project, Edit: true, Mode: lines"
	if result != expected {
		t.Errorf("got %q, want %q", result, expected)
	}
}

func TestTemplateEngine_HasTemplate(t *testing.T) {
	testFS := fstest.MapFS{
		"exists.tmpl": &fstest.MapFile{Data: []byte("hello")},
	}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	if !engine.HasTemplate("exists.tmpl") {
		t.Error("HasTemplate returned false for existing template")
	}

	if engine.HasTemplate("nonexistent.tmpl") {
		t.Error("HasTemplate returned true for non-existing template")
	}
}

func TestTemplateEngine_ListTemplates(t *testing.T) {
	testFS := fstest.MapFS{
		"a.tmpl": &fstest.MapFile{Data: []byte("a")},
		"b.tmpl": &fstest.MapFile{Data: []byte("b")},
		"c.txt":  &fstest.MapFile{Data: []byte("c")}, // not a template
	}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	templates := engine.ListTemplates()

	// Should have 2 templates (a.tmpl and b.tmpl), plus the root template
	foundA, foundB := false, false
	for _, name := range templates {
		if name == "a.tmpl" {
			foundA = true
		}
		if name == "b.tmpl" {
			foundB = true
		}
	}

	if !foundA || !foundB {
		t.Errorf("ListTemplates missing expected templates, got: %v", templates)
	}
}

func TestTemplateEngine_RenderInvalidTemplate(t *testing.T) {
	testFS := fstest.MapFS{
		"valid.tmpl": &fstest.MapFile{Data: []byte("hello")},
	}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	_, err = engine.Render("nonexistent.tmpl", &PromptContext{})
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestTemplateEngine_ParseError(t *testing.T) {
	testFS := fstest.MapFS{
		"bad.tmpl": &fstest.MapFile{Data: []byte("{{ .Unclosed")},
	}

	_, err := NewTemplateEngine(testFS)
	if err == nil {
		t.Error("expected parse error for invalid template syntax")
	}
}

func TestTemplateEngine_NestedDirectories(t *testing.T) {
	testFS := fstest.MapFS{
		"prompts/sections/role.tmpl": &fstest.MapFile{
			Data: []byte("Role: {{ .WorkspaceRoot }}"),
		},
		"prompts/tools/read.tmpl": &fstest.MapFile{
			Data: []byte("Read tool"),
		},
	}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	if !engine.HasTemplate("prompts/sections/role.tmpl") {
		t.Error("nested template not found")
	}

	ctx := &PromptContext{WorkspaceRoot: "/test"}
	result, err := engine.Render("prompts/sections/role.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	if result != "Role: /test" {
		t.Errorf("got %q, want %q", result, "Role: /test")
	}
}

func TestLayeredFS_OverrideTakesPrecedence(t *testing.T) {
	baseFS := fstest.MapFS{
		"shared.tmpl":    &fstest.MapFile{Data: []byte("base content")},
		"base-only.tmpl": &fstest.MapFile{Data: []byte("only in base")},
	}

	overrideFS := fstest.MapFS{
		"shared.tmpl":        &fstest.MapFile{Data: []byte("override content")},
		"override-only.tmpl": &fstest.MapFile{Data: []byte("only in override")},
	}

	layered := &layeredFS{
		override: overrideFS,
		base:     baseFS,
	}

	// Test that override takes precedence
	content, err := fs.ReadFile(layered, "shared.tmpl")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "override content" {
		t.Errorf("got %q, want %q", string(content), "override content")
	}

	// Test base-only file is accessible
	content, err = fs.ReadFile(layered, "base-only.tmpl")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "only in base" {
		t.Errorf("got %q, want %q", string(content), "only in base")
	}

	// Test override-only file is accessible
	content, err = fs.ReadFile(layered, "override-only.tmpl")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "only in override" {
		t.Errorf("got %q, want %q", string(content), "only in override")
	}
}

func TestLayeredFS_ReadDir(t *testing.T) {
	baseFS := fstest.MapFS{
		"a.tmpl": &fstest.MapFile{Data: []byte("a")},
		"b.tmpl": &fstest.MapFile{Data: []byte("b")},
	}

	overrideFS := fstest.MapFS{
		"b.tmpl": &fstest.MapFile{Data: []byte("b-override")},
		"c.tmpl": &fstest.MapFile{Data: []byte("c")},
	}

	layered := &layeredFS{
		override: overrideFS,
		base:     baseFS,
	}

	entries, err := layered.ReadDir(".")
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}

	// Should have 3 unique entries: a.tmpl, b.tmpl, c.tmpl
	if len(entries) != 3 {
		t.Errorf("got %d entries, want 3", len(entries))
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name()] = true
	}

	for _, expected := range []string{"a.tmpl", "b.tmpl", "c.tmpl"} {
		if !names[expected] {
			t.Errorf("missing entry: %s", expected)
		}
	}
}

func TestTemplateEngine_Reload(t *testing.T) {
	testFS := fstest.MapFS{
		"test.tmpl": &fstest.MapFile{Data: []byte("original")},
	}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	// Verify initial state
	if !engine.HasTemplate("test.tmpl") {
		t.Error("template should exist before reload")
	}

	// Reload should succeed
	err = engine.Reload()
	if err != nil {
		t.Fatalf("Reload failed: %v", err)
	}

	// Template should still exist
	if !engine.HasTemplate("test.tmpl") {
		t.Error("template should exist after reload")
	}
}

func TestRenderString(t *testing.T) {
	testFS := fstest.MapFS{} // empty, we're testing RenderString

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	ctx := PromptContext{
		WorkspaceRoot: "/test/path",
		HasEdit:       true,
	}

	result, err := engine.RenderString("Path: {{ .WorkspaceRoot }}", ctx)
	if err != nil {
		t.Fatalf("RenderString failed: %v", err)
	}

	if result != "Path: /test/path" {
		t.Errorf("got %q, want %q", result, "Path: /test/path")
	}
}

func TestRenderString_InvalidTemplate(t *testing.T) {
	testFS := fstest.MapFS{}

	engine, err := NewTemplateEngine(testFS)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	_, err = engine.RenderString("{{ .Invalid", PromptContext{})
	if err == nil {
		t.Error("expected error for invalid template string")
	}
}

func TestPrefixFS_StripsPrefixOnOpen(t *testing.T) {
	// Simulate override directory structure: sections/role.tmpl
	baseFS := fstest.MapFS{
		"sections/role.tmpl": &fstest.MapFile{Data: []byte("override content")},
	}

	// Wrap with prefix so "prompts/sections/role.tmpl" maps to "sections/role.tmpl"
	wrapped := &prefixFS{fs: baseFS, prefix: "prompts"}

	// Opening with prefix should work
	f, err := wrapped.Open("prompts/sections/role.tmpl")
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	f.Close()

	// ReadFile with prefix should work
	content, err := wrapped.ReadFile("prompts/sections/role.tmpl")
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if string(content) != "override content" {
		t.Errorf("got %q, want %q", string(content), "override content")
	}
}

func TestPrefixFS_WithLayeredFS(t *testing.T) {
	// Base (embedded) has prompts/sections/role.tmpl
	baseFS := fstest.MapFS{
		"prompts/sections/role.tmpl":  &fstest.MapFile{Data: []byte("embedded role")},
		"prompts/sections/tasks.tmpl": &fstest.MapFile{Data: []byte("embedded tasks")},
	}

	// Override has sections/role.tmpl (no prompts/ prefix)
	overrideRawFS := fstest.MapFS{
		"sections/role.tmpl": &fstest.MapFile{Data: []byte("override role")},
	}

	// Wrap override with prefix
	wrappedOverride := &prefixFS{fs: overrideRawFS, prefix: "prompts"}

	// Layer them
	layered := &layeredFS{
		override: wrappedOverride,
		base:     baseFS,
	}

	// role.tmpl should come from override
	content, err := fs.ReadFile(layered, "prompts/sections/role.tmpl")
	if err != nil {
		t.Fatalf("ReadFile role failed: %v", err)
	}
	if string(content) != "override role" {
		t.Errorf("role: got %q, want %q", string(content), "override role")
	}

	// tasks.tmpl should come from base (not overridden)
	content, err = fs.ReadFile(layered, "prompts/sections/tasks.tmpl")
	if err != nil {
		t.Fatalf("ReadFile tasks failed: %v", err)
	}
	if string(content) != "embedded tasks" {
		t.Errorf("tasks: got %q, want %q", string(content), "embedded tasks")
	}
}

func TestCollectTemplateFilesWithPrefix(t *testing.T) {
	testFS := fstest.MapFS{
		"sections/role.tmpl":      &fstest.MapFile{Data: []byte("role")},
		"tools/read.tmpl":         &fstest.MapFile{Data: []byte("read")},
		"other.txt":               &fstest.MapFile{Data: []byte("ignored")},
	}

	files, err := collectTemplateFilesWithPrefix(testFS, "prompts")
	if err != nil {
		t.Fatalf("collectTemplateFilesWithPrefix failed: %v", err)
	}

	expected := map[string]bool{
		"prompts/sections/role.tmpl": true,
		"prompts/tools/read.tmpl":    true,
	}

	if len(files) != len(expected) {
		t.Errorf("got %d files, want %d: %v", len(files), len(expected), files)
	}

	for _, f := range files {
		if !expected[f] {
			t.Errorf("unexpected file: %s", f)
		}
	}
}

// TestOverridePathMismatch tests the real-world scenario where override templates
// are at "sections/role.tmpl" but embedded templates are at "prompts/sections/role.tmpl".
// This was a bug where override templates were found but never actually used.
func TestOverridePathMismatch(t *testing.T) {
	// This simulates the real structure:
	// - User's override dir has: sections/role.tmpl (no prompts/ prefix)
	// - Embedded has: prompts/sections/role.tmpl (with prompts/ prefix)

	// Without prefixFS, the override would never be found when looking for
	// "prompts/sections/role.tmpl" because override only has "sections/role.tmpl"

	embeddedFS := fstest.MapFS{
		"prompts/sections/role.tmpl": &fstest.MapFile{
			Data: []byte("embedded: {{ .WorkspaceRoot }}"),
		},
	}

	// Override directory structure (no prompts/ prefix, as user would create)
	overrideFS := fstest.MapFS{
		"sections/role.tmpl": &fstest.MapFile{
			Data: []byte("override: {{ .WorkspaceRoot }}"),
		},
	}

	// This is how LoadTemplates should set it up - with prefixFS wrapper
	wrappedOverride := &prefixFS{fs: overrideFS, prefix: "prompts"}

	layered := &layeredFS{
		override: wrappedOverride,
		base:     embeddedFS,
	}

	engine, err := NewTemplateEngine(layered)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	// The template should be found and should be the OVERRIDE version
	ctx := &PromptContext{WorkspaceRoot: "/test"}
	result, err := engine.Render("prompts/sections/role.tmpl", ctx)
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// CRITICAL: This must be "override:" not "embedded:"
	// If this returns "embedded:", the override is silently ignored (the bug)
	if result != "override: /test" {
		t.Errorf("Override not used! got %q, want %q", result, "override: /test")
	}
}

// TestOverridePathMismatch_WithoutFix demonstrates what happens without the prefixFS fix.
// This test documents the bug - it would fail if we remove the prefixFS wrapper.
func TestOverridePathMismatch_WithoutFix(t *testing.T) {
	embeddedFS := fstest.MapFS{
		"prompts/sections/role.tmpl": &fstest.MapFile{
			Data: []byte("embedded"),
		},
	}

	// Override without the prefix wrapper - simulates the broken behavior
	overrideFS := fstest.MapFS{
		"sections/role.tmpl": &fstest.MapFile{
			Data: []byte("override"),
		},
	}

	// WITHOUT prefixFS wrapper - this is the buggy setup
	layered := &layeredFS{
		override: overrideFS,  // NOT wrapped
		base:     embeddedFS,
	}

	engine, err := NewTemplateEngine(layered)
	if err != nil {
		t.Fatalf("NewTemplateEngine failed: %v", err)
	}

	// Looking for "prompts/sections/role.tmpl" will NOT find "sections/role.tmpl"
	// because paths don't match - it falls back to embedded
	result, err := engine.Render("prompts/sections/role.tmpl", &PromptContext{})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	// Without the fix, this returns "embedded" (the bug)
	// This test documents the broken behavior
	if result != "embedded" {
		t.Errorf("Expected buggy behavior to return embedded, got %q", result)
	}
}

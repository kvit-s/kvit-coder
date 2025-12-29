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

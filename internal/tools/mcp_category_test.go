package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

// fakeCategoryTool is a minimal Tool used to verify category machinery without
// importing internal/mcp (which would create an import cycle).
type fakeCategoryTool struct {
	name     string
	category string
}

func (f fakeCategoryTool) Name() string                                       { return f.name }
func (f fakeCategoryTool) Description() string                                { return "fake" }
func (f fakeCategoryTool) JSONSchema() map[string]any                         { return map[string]any{"type": "object"} }
func (f fakeCategoryTool) Check(context.Context, json.RawMessage) error       { return nil }
func (f fakeCategoryTool) Call(context.Context, json.RawMessage) (any, error) { return nil, nil }
func (f fakeCategoryTool) PromptSection() string                              { return "### " + f.name + "\n\nfake section" }
func (f fakeCategoryTool) PromptCategory() string                             { return f.category }
func (f fakeCategoryTool) PromptOrder() int                                   { return 0 }
func (f fakeCategoryTool) PromptTemplateName() string                         { return "" }

func TestRegistry_MCPCategoryAbsentByDefault(t *testing.T) {
	r := NewRegistry()
	r.Enable(fakeCategoryTool{name: "Read", category: "filesystem"})

	for _, c := range r.EnabledCategories() {
		if c == "mcp" {
			t.Fatal("mcp category should be absent when no mcp tools are registered")
		}
	}
	if strings.Contains(r.GenerateToolPrompt(), CategoryHeaders["mcp"]) {
		t.Fatal("MCP header should not appear without mcp tools")
	}
}

func TestRegistry_MCPCategoryAppears(t *testing.T) {
	r := NewRegistry()
	r.Enable(fakeCategoryTool{name: "Read", category: "filesystem"})
	r.Enable(fakeCategoryTool{name: "mcp.srv.tool", category: "mcp"})

	found := false
	for _, c := range r.EnabledCategories() {
		if c == "mcp" {
			found = true
		}
	}
	if !found {
		t.Fatal("mcp category should appear when an mcp tool is registered")
	}

	prompt := r.GenerateToolPrompt()
	if !strings.Contains(prompt, CategoryHeaders["mcp"]) {
		t.Fatal("MCP header should appear in generated prompt")
	}
	// mcp section comes after the built-in categories (deterministic ordering).
	if strings.Index(prompt, CategoryHeaders["filesystem"]) > strings.Index(prompt, CategoryHeaders["mcp"]) {
		t.Fatal("mcp category should be ordered after filesystem")
	}
}

func TestSetupRegistry_MCPToolsRegistered(t *testing.T) {
	cfg := newTestConfig()
	reg := SetupRegistry(SetupConfig{
		Cfg:      cfg,
		MCPTools: []Tool{fakeCategoryTool{name: "mcp.srv.tool", category: "mcp"}},
	})
	if reg.Get("mcp.srv.tool") == nil {
		t.Fatal("MCP tool should be registered via SetupConfig.MCPTools")
	}
}

func TestSetupRegistry_NoMCPToolsIsNoop(t *testing.T) {
	cfg := newTestConfig()
	withNil := SetupRegistry(SetupConfig{Cfg: cfg, MCPTools: nil})
	withEmpty := SetupRegistry(SetupConfig{Cfg: cfg, MCPTools: []Tool{}})

	if len(withNil.ListTools()) != len(withEmpty.ListTools()) {
		t.Fatal("nil and empty MCPTools should yield identical registries")
	}
	for _, name := range withNil.ListTools() {
		if strings.HasPrefix(name, "mcp.") {
			t.Fatalf("unexpected mcp tool with no MCPTools configured: %s", name)
		}
	}
}

package mcp

import (
	"context"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

func TestManager_DisabledIsNoop(t *testing.T) {
	m := NewManager(config.MCPConfig{Enabled: false}, "", nil, nil)
	if err := m.Connect(context.Background()); err != nil {
		t.Errorf("disabled Connect should be a no-op, got %v", err)
	}
	if got := m.Tools(); len(got) != 0 {
		t.Errorf("disabled manager should expose no tools, got %d", len(got))
	}
	if m.Summary() != "" {
		t.Errorf("disabled manager should have empty summary, got %q", m.Summary())
	}
}

func TestManager_ToolsAppliesFilterAndOrder(t *testing.T) {
	m := NewManager(config.MCPConfig{Enabled: true}, "", nil, nil)
	fc := &fakeClient{}
	// Inject two connected servers directly to test adapter production.
	m.servers = []*serverConn{
		{
			name:    "alpha",
			client:  fc,
			confirm: ConfirmTrust,
			timeout: time.Second,
			descs: []ToolDescriptor{
				{Name: "keep"},
				{Name: "drop"},
			},
			filter: config.MCPToolFilter{Deny: []string{"drop"}},
		},
		{
			name:    "beta",
			client:  fc,
			confirm: ConfirmTrust,
			timeout: time.Second,
			descs: []ToolDescriptor{
				{Name: "only"},
				{Name: "nope"},
			},
			filter: config.MCPToolFilter{Allow: []string{"only"}},
		},
	}

	got := m.Tools()
	names := make([]string, len(got))
	for i, tl := range got {
		names[i] = tl.Name()
	}
	want := []string{"mcp.alpha.keep", "mcp.beta.only"}
	if len(names) != len(want) {
		t.Fatalf("expected %v, got %v", want, names)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Errorf("tool %d: expected %s, got %s", i, want[i], names[i])
		}
	}

	if got, want := m.Summary(), "2 MCP tool(s) from 2 server(s): alpha (1), beta (1)"; got != want {
		t.Errorf("summary: want %q, got %q", want, got)
	}
}

func TestManager_ResolveCwd(t *testing.T) {
	m := NewManager(config.MCPConfig{}, "/work/root", nil, nil)
	cases := []struct {
		in, want string
	}{
		{"", "/work/root"},                // empty defaults to workspace root
		{"/abs/path", "/abs/path"},        // absolute used as-is
		{"sub/dir", "/work/root/sub/dir"}, // relative resolved against workspace root
	}
	for _, c := range cases {
		if got := m.resolveCwd(c.in); got != c.want {
			t.Errorf("resolveCwd(%q)=%q, want %q", c.in, got, c.want)
		}
	}

	// With no workspace root, an empty cwd inherits (returns "").
	m2 := NewManager(config.MCPConfig{}, "", nil, nil)
	if got := m2.resolveCwd(""); got != "" {
		t.Errorf("resolveCwd with no workspace root should be empty, got %q", got)
	}
}

func TestAllowTool(t *testing.T) {
	cases := []struct {
		name   string
		filter config.MCPToolFilter
		tool   string
		want   bool
	}{
		{"empty allows all", config.MCPToolFilter{}, "x", true},
		{"allow list includes", config.MCPToolFilter{Allow: []string{"x", "y"}}, "x", true},
		{"allow list excludes", config.MCPToolFilter{Allow: []string{"y"}}, "x", false},
		{"deny removes", config.MCPToolFilter{Deny: []string{"x"}}, "x", false},
		{"deny after allow", config.MCPToolFilter{Allow: []string{"x"}, Deny: []string{"x"}}, "x", false},
	}
	for _, c := range cases {
		if got := allowTool(c.tool, c.filter); got != c.want {
			t.Errorf("%s: allowTool(%q)=%v, want %v", c.name, c.tool, got, c.want)
		}
	}
}

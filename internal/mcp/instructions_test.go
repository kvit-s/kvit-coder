package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

const echoInstructions = "Call echo before anything else.\nKeep messages short."

// echoWithInstructions is the echo server set to send instructions.
func echoWithInstructions(sc config.MCPServerConfig) config.MCPServerConfig {
	sc.Env = append(append([]string{}, sc.Env...), "ECHO_INSTRUCTIONS="+echoInstructions)
	return sc
}

// A server connected at startup has its instructions in the system prompt.
func TestConnectedServerInstructionsAreInThePromptSection(t *testing.T) {
	sc := echoWithInstructions(echoServerConfig("echo"))
	m := NewManager(config.MCPConfig{Enabled: true, Confirm: ConfirmTrust, StartupTimeout: 20, CallTimeout: 10,
		Servers: []config.MCPServerConfig{sc}}, "", nil, nil)
	defer m.Close()
	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	section := m.PromptSection()
	for _, want := range []string{"## Instructions from MCP servers", "### echo", echoInstructions} {
		if !strings.Contains(section, want) {
			t.Errorf("prompt section is missing %q:\n%s", want, section)
		}
	}
}

// Only servers that sent instructions and have a tool the model can run are
// in the section, in the order they are connected in (by name).
func TestPromptSectionLeavesOutServersWithNothingToSay(t *testing.T) {
	m := NewManager(config.MCPConfig{Enabled: true}, "", nil, nil)
	if got := m.PromptSection(); got != "" {
		t.Errorf("no servers: section %q, want empty", got)
	}
	fc := &fakeClient{}
	m.servers = []*serverConn{
		{name: "alpha", client: fc, descs: []ToolDescriptor{{Name: "a"}}, instructions: "Alpha first."},
		{name: "beta", client: fc, descs: []ToolDescriptor{{Name: "b"}}},
		{name: "gamma", client: fc, descs: []ToolDescriptor{{Name: "g"}}, instructions: "Gamma hidden.",
			filter: config.MCPToolFilter{Deny: []string{"g"}}},
		{name: "zeta", client: fc, descs: []ToolDescriptor{{Name: "z"}}, instructions: "Zeta last."},
	}
	section := m.PromptSection()
	if strings.Contains(section, "beta") || strings.Contains(section, "Gamma") {
		t.Errorf("section names a server with no instructions or no visible tools:\n%s", section)
	}
	alpha, zeta := strings.Index(section, "### alpha\n\nAlpha first."), strings.Index(section, "### zeta\n\nZeta last.")
	if alpha < 0 || zeta < 0 || alpha > zeta {
		t.Errorf("want alpha then zeta:\n%s", section)
	}
}

// A server behind a tool group: the first open dials it and records its
// instructions beside the tool list; a later open reads both from the cache
// without dialing.
func TestDeferredServerInstructionsComeWithItsTools(t *testing.T) {
	m, sc := deferredTestManager(t)
	sc = echoWithInstructions(sc)
	m.cfg.Servers = []config.MCPServerConfig{sc}
	m.Defer(map[string]bool{"echo": true})
	defer m.Close()

	_, notes, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err != nil {
		t.Fatalf("ToolsForServers: %v", err)
	}
	if len(notes) != 1 || notes[0].Source != "MCP server echo" || notes[0].Text != echoInstructions {
		t.Fatalf("notes = %+v, want the server's instructions", notes)
	}
	if cached, ok := m.readToolListCache(sc); !ok || cached.Instructions != echoInstructions {
		t.Errorf("cache holds %q, %v; want the instructions", cached.Instructions, ok)
	}

	// Different text in the cache than the server sends proves the second
	// open read the cache and did not dial.
	m.writeToolListCache(sc, []ToolDescriptor{{Name: "echo", InputSchema: map[string]any{"type": "object"}}}, "From the cache.")
	fresh := NewManager(m.cfg, "", nil, nil)
	fresh.Defer(map[string]bool{"echo": true})
	defer fresh.Close()
	_, notes, err = fresh.ToolsForServers(context.Background(), []string{"echo"})
	if err != nil {
		t.Fatalf("ToolsForServers from cache: %v", err)
	}
	if len(notes) != 1 || notes[0].Text != "From the cache." {
		t.Errorf("notes = %+v, want the cached instructions", notes)
	}
}

// A cache file written before instructions were kept is treated as missing,
// so the server is asked once more instead of its instructions going unseen
// until the file expires.
func TestCacheFromBeforeInstructionsIsRefetched(t *testing.T) {
	m, sc := deferredTestManager(t)
	sc = echoWithInstructions(sc)
	m.cfg.Servers = []config.MCPServerConfig{sc}
	m.Defer(map[string]bool{"echo": true})
	defer m.Close()

	path, key, err := toolListCachePath(sc)
	if err != nil {
		t.Fatal(err)
	}
	old, err := json.Marshal(map[string]any{
		"server": "echo", "key": key, "written": time.Now(),
		"tools": []ToolDescriptor{{Name: "ghost", InputSchema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, old, 0o644); err != nil {
		t.Fatal(err)
	}

	got, notes, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err != nil {
		t.Fatalf("ToolsForServers: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "mcp.echo.echo" {
		t.Errorf("got %v, want the server's real tool rather than the old cache's", got)
	}
	if len(notes) != 1 || notes[0].Text != echoInstructions {
		t.Errorf("notes = %+v, want the server's instructions", notes)
	}
}

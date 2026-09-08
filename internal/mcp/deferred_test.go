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

// deferredTestManager builds a manager over the echo server from stdio_test.go
// with its own HOME, so the on-disk tool-list cache is written under the test's
// temp directory rather than the developer's.
func deferredTestManager(t *testing.T) (*Manager, config.MCPServerConfig) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	sc := echoServerConfig("echo")
	cfg := config.MCPConfig{
		Enabled:        true,
		Confirm:        ConfirmTrust,
		StartupTimeout: 20,
		CallTimeout:    10,
		Servers:        []config.MCPServerConfig{sc},
	}
	return NewManager(cfg, "", nil, nil), sc
}

func TestDeferredServerIsNotDialedByConnect(t *testing.T) {
	m, _ := deferredTestManager(t)
	m.Defer(map[string]bool{"echo": true})

	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	// The point of deferring: nothing is spawned, and the model is offered
	// nothing, until its group is opened.
	if got := m.Tools(); len(got) != 0 {
		t.Errorf("deferred server contributed %d tools to the registry", len(got))
	}
	if got := m.Status(); len(got) != 0 {
		t.Errorf("deferred server reported as connected: %+v", got)
	}
	if took, _ := m.ConnectCost(); took != 0 {
		t.Errorf("Connect spent %s on a deferred server", took)
	}
}

func TestUndeferredServerStillConnectsNormally(t *testing.T) {
	m, _ := deferredTestManager(t)
	defer m.Close()

	if err := m.Connect(context.Background()); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if got := m.Tools(); len(got) != 1 || got[0].Name() != "mcp.echo.echo" {
		t.Fatalf("got %d tools, want the echo tool", len(got))
	}
}

func TestToolsForServersDialsOnceAndCachesTheToolList(t *testing.T) {
	m, sc := deferredTestManager(t)
	m.Defer(map[string]bool{"echo": true})
	defer m.Close()

	got, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err != nil {
		t.Fatalf("ToolsForServers: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "mcp.echo.echo" {
		t.Fatalf("got %+v", got)
	}

	path, _, err := toolListCachePath(sc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("tool list was not cached: %v", err)
	}
	if dir := filepath.Dir(path); !strings.Contains(dir, ".kvit-coder") {
		t.Errorf("cache written outside .kvit-coder: %s", dir)
	}
}

func TestToolsForServersAnswersFromCacheWithoutDialing(t *testing.T) {
	m, sc := deferredTestManager(t)
	m.Defer(map[string]bool{"echo": true})
	defer m.Close()

	// A tool the real server does not advertise. Getting it back proves the
	// description came from the cache and the server was never asked -- which
	// is what makes opening a group cost nothing.
	m.writeToolListCache(sc, []ToolDescriptor{{
		Name:        "ghost",
		Description: "Only in the cache",
		InputSchema: map[string]any{"type": "object"},
	}})

	got, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err != nil {
		t.Fatalf("ToolsForServers: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "mcp.echo.ghost" {
		t.Fatalf("got %+v, want the cached tool", got)
	}
	if got[0].Description() != "Only in the cache" {
		t.Errorf("description %q", got[0].Description())
	}
}

func TestStaleCacheIsRefreshedFromTheServer(t *testing.T) {
	m, sc := deferredTestManager(t)
	m.Defer(map[string]bool{"echo": true})
	defer m.Close()

	path, key, err := toolListCachePath(sc)
	if err != nil {
		t.Fatal(err)
	}
	m.writeToolListCache(sc, []ToolDescriptor{{Name: "ghost", InputSchema: map[string]any{"type": "object"}}})

	// Age it past the time-to-live. A server launched as @latest can change
	// its tools whenever it is updated, so the cache must not be forever.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cached toolListCache
	if err := json.Unmarshal(data, &cached); err != nil {
		t.Fatal(err)
	}
	if cached.Key != key {
		t.Fatalf("cache key %q, want %q", cached.Key, key)
	}
	cached.Written = time.Now().Add(-toolListCacheTTL - time.Hour)
	aged, err := json.Marshal(cached)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, aged, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err != nil {
		t.Fatalf("ToolsForServers: %v", err)
	}
	if len(got) != 1 || got[0].Name() != "mcp.echo.echo" {
		t.Fatalf("got %+v, want the server's real tool", got)
	}
}

func TestCacheKeyFollowsTheCommandLine(t *testing.T) {
	sc := echoServerConfig("echo")
	_, before, err := toolListCachePath(sc)
	if err != nil {
		t.Fatal(err)
	}

	// Editing a server's arguments has to invalidate its cache, or a changed
	// server keeps being described by the tools it used to have.
	sc.Args = append(sc.Args, "--new-flag")
	_, after, err := toolListCachePath(sc)
	if err != nil {
		t.Fatal(err)
	}
	if before == after {
		t.Error("cache key ignored a change to the server's arguments")
	}

	// Headers are deliberately not part of the key: they can hold credentials
	// and they do not decide what a server offers.
	sc = echoServerConfig("echo")
	sc.Headers = []string{"Authorization=Bearer ${TOKEN}"}
	_, withHeaders, err := toolListCachePath(sc)
	if err != nil {
		t.Fatal(err)
	}
	if withHeaders != before {
		t.Error("cache key changed with a header, which does not affect the tool list")
	}
}

func TestLazyClientDialsOnTheFirstCall(t *testing.T) {
	m, sc := deferredTestManager(t)
	m.Defer(map[string]bool{"echo": true})
	defer m.Close()

	m.writeToolListCache(sc, []ToolDescriptor{{
		Name:        "echo",
		Description: "Echoes its message back",
		InputSchema: map[string]any{"type": "object"},
	}})

	got, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err != nil {
		t.Fatalf("ToolsForServers: %v", err)
	}

	// Describing it dialed nothing; calling it must reach the real server.
	res, err := got[0].Call(context.Background(), []byte(`{"message":"yo"}`))
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	m2, ok := res.(map[string]any)
	if !ok || m2["content"] != "echo: yo" {
		t.Fatalf("got %#v, want the server's answer", res)
	}
}

func TestToolsForServersReportsAnUnknownServer(t *testing.T) {
	m, _ := deferredTestManager(t)
	defer m.Close()

	_, err := m.ToolsForServers(context.Background(), []string{"nope"})
	if err == nil || !strings.Contains(err.Error(), "not configured") {
		t.Fatalf("got %v, want an error naming the missing server", err)
	}
}

func TestToolsForServersAppliesTheServerFilter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sc := echoServerConfig("echo")
	sc.Tools = config.MCPToolFilter{Deny: []string{"echo"}}
	m := NewManager(config.MCPConfig{
		Enabled: true, Confirm: ConfirmTrust, StartupTimeout: 20, CallTimeout: 10,
		Servers: []config.MCPServerConfig{sc},
	}, "", nil, nil)
	m.Defer(map[string]bool{"echo": true})
	defer m.Close()

	got, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err != nil {
		t.Fatalf("ToolsForServers: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("denied tool reached the group: %+v", got)
	}
}

func TestToolsForServersRefusesWhenMCPIsOff(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sc := echoServerConfig("echo")
	m := NewManager(config.MCPConfig{Enabled: false, Servers: []config.MCPServerConfig{sc}}, "", nil, nil)
	defer m.Close()

	if m.ServerUsable("echo") {
		t.Error("a server is usable with MCP switched off")
	}
	_, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err == nil || !strings.Contains(err.Error(), "switched off") {
		t.Fatalf("got %v, want a refusal naming the setting", err)
	}
}

func TestToolsForServersRefusesADisabledServer(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	sc := echoServerConfig("echo")
	sc.Enabled = false
	m := NewManager(config.MCPConfig{Enabled: true, Servers: []config.MCPServerConfig{sc}}, "", nil, nil)
	defer m.Close()

	if m.ServerUsable("echo") {
		t.Error("a disabled server reports as usable")
	}
	_, err := m.ToolsForServers(context.Background(), []string{"echo"})
	if err == nil || !strings.Contains(err.Error(), "switched off in mcp.servers") {
		t.Fatalf("got %v, want a refusal naming the server", err)
	}
	if m.ServerUsable("nope") {
		t.Error("an unconfigured server reports as usable")
	}
}

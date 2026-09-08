package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// TestConnectCostLive measures what one turn pays to bring the browser server
// up. Under one process per turn this is paid on every instruction, and it is
// the cost the daemon in docs/redesign-mcp.md exists to remove, so it is worth
// a number rather than an estimate.
//
//	KVIT_MCP_LIVE=1 go test ./internal/mcp/ -run TestConnectCostLive -v
func TestConnectCostLive(t *testing.T) {
	if os.Getenv("KVIT_MCP_LIVE") == "" {
		t.Skip("set KVIT_MCP_LIVE=1 to run")
	}
	dir := t.TempDir()
	cfg := config.MCPConfig{
		Enabled: true, Confirm: ConfirmTrust,
		StartupTimeout: 60, CallTimeout: 180,
		Servers: []config.MCPServerConfig{{
			Name: "playwright", Enabled: true, Transport: "stdio", Command: "npx",
			Args: []string{"-y", "@playwright/mcp@latest", "--headless",
				"--browser=chromium", "--isolated",
				"--output-dir=" + filepath.Join(dir, "out")},
		}},
	}

	var total time.Duration
	const runs = 3
	for i := 1; i <= runs; i++ {
		mgr := NewManager(cfg, dir, nil, nil)
		start := time.Now()
		if err := mgr.Connect(context.Background()); err != nil {
			t.Fatalf("connect: %v", err)
		}
		wall := time.Since(start)
		took, slowest := mgr.ConnectCost()
		t.Logf("turn %d: Connect took %s (slowest server %q at %s), %d tools",
			i, wall.Round(time.Millisecond), slowest, took.Round(time.Millisecond), len(mgr.Tools()))
		total += wall
		mgr.Close()
	}
	t.Logf("mean per-turn reconnect cost over %d runs: %s", runs, (total / runs).Round(time.Millisecond))
}

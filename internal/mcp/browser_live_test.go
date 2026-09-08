package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// TestBrowserServerLive drives the configured browser server end to end: it
// spawns Playwright MCP, navigates, and checks that a pre-recorded approval is
// honoured without a prompt -- which is the whole point of keeping ask_once
// answers in the session rather than in memory.
//
// Gated because it spawns npx, downloads nothing but launches a real browser,
// and takes several seconds. Run it with:
//
//	KVIT_MCP_LIVE=1 go test ./internal/mcp/ -run TestBrowserServerLive -v
func TestBrowserServerLive(t *testing.T) {
	if os.Getenv("KVIT_MCP_LIVE") == "" {
		t.Skip("set KVIT_MCP_LIVE=1 to run")
	}
	dir := t.TempDir()

	cfg := config.MCPConfig{
		Enabled:        true,
		Confirm:        ConfirmAskOnce,
		StartupTimeout: 60,
		CallTimeout:    180,
		Servers: []config.MCPServerConfig{{
			Name:      "playwright",
			Enabled:   true,
			Transport: "stdio",
			Command:   "npx",
			Args: []string{"-y", "@playwright/mcp@latest", "--headless",
				"--browser=chromium", "--isolated",
				"--output-dir=" + filepath.Join(dir, "out")},
		}},
	}

	mgr := NewManager(cfg, dir, nil, nil)

	// Approve exactly one tool, the way a person answering the prompt once
	// would, and leave the rest unapproved.
	approvals, err := OpenApprovals(filepath.Join(dir, "mcp-approvals.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := approvals.Approve("mcp.playwright.browser_navigate"); err != nil {
		t.Fatal(err)
	}
	mgr.SetApprovals(approvals)

	ctx := context.Background()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer mgr.Close()

	byName := map[string]toolsInterface{}
	for _, tl := range mgr.Tools() {
		byName[tl.Name()] = tl
	}
	if len(byName) == 0 {
		t.Fatal("the server advertised no tools")
	}
	t.Logf("%d browser tools connected", len(byName))

	nav := byName["mcp.playwright.browser_navigate"]
	if nav == nil {
		t.Fatal("browser_navigate is missing")
	}
	args := json.RawMessage(`{"url":"https://example.com"}`)

	// Approved: must pass Check without a prompt. There is no terminal here, so
	// a prompt would refuse and this is a real assertion rather than a formality.
	if err := nav.Check(ctx, args); err != nil {
		t.Fatalf("an approved tool was still gated: %v", err)
	}
	out, err := nav.Call(ctx, args)
	if err != nil {
		t.Fatalf("browser_navigate: %v", err)
	}
	text := strings.ToLower(anyToString(out))
	if !strings.Contains(text, "example domain") {
		t.Errorf("navigation did not reach the page; got: %s", truncateForLog(text))
	}
	t.Logf("navigated, result mentions the page title")

	// Not approved: must still be gated, or the store is approving everything.
	if snap := byName["mcp.playwright.browser_snapshot"]; snap != nil {
		if err := snap.Check(ctx, json.RawMessage(`{}`)); err == nil {
			t.Error("an unapproved tool was allowed through")
		}
	}
}

type toolsInterface interface {
	Name() string
	Check(context.Context, json.RawMessage) error
	Call(context.Context, json.RawMessage) (any, error)
}

func anyToString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return string(b)
}

func truncateForLog(s string) string {
	if len(s) > 400 {
		return s[:400] + "..."
	}
	return s
}

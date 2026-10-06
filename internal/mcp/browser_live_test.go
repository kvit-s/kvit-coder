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
	"github.com/kvit-s/kvit-coder/internal/tools"
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

// TestBrowserGroupLive drives the browser through a tool group, which is how
// config.yaml has it: the server is deferred, so it is not dialed at startup;
// opening the group describes its tools from the on-disk cache without starting
// anything; and only running one dials the server.
//
// Gated the same way as TestBrowserServerLive:
//
//	KVIT_MCP_LIVE=1 go test ./internal/mcp/ -run TestBrowserGroupLive -v
func TestBrowserGroupLive(t *testing.T) {
	if os.Getenv("KVIT_MCP_LIVE") == "" {
		t.Skip("set KVIT_MCP_LIVE=1 to run")
	}
	// HOME is deliberately left alone: Playwright finds its installed browser
	// under ~/.cache/ms-playwright, and the tool-list cache this test exercises
	// belongs under the real ~/.kvit-coder/mcp/ anyway, which is where a real
	// run writes it.
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
	mgr.Defer(map[string]bool{"playwright": true})
	defer mgr.Close()

	approvals, err := OpenApprovals(filepath.Join(dir, "mcp-approvals.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := approvals.Approve("mcp.playwright.browser_navigate"); err != nil {
		t.Fatal(err)
	}
	mgr.SetApprovals(approvals)

	ctx := context.Background()
	started := time.Now()
	if err := mgr.Connect(ctx); err != nil {
		t.Fatalf("connect: %v", err)
	}
	// A deferred server costs the turn nothing at startup.
	if took := time.Since(started); took > 2*time.Second {
		t.Errorf("Connect spent %s on a deferred server", took.Round(time.Millisecond))
	}
	if got := mgr.Tools(); len(got) != 0 {
		t.Fatalf("deferred server put %d tools in the registry", len(got))
	}

	group := tools.NewGroupTool("Web.browsing", "Drive a real browser.",
		"Work from a snapshot.", "web", 900,
		func(ctx context.Context) ([]tools.Tool, []tools.GroupNote, error) {
			return mgr.ToolsForServers(ctx, []string{"playwright"})
		})

	// First open: nothing is cached yet, so this dials once and caches.
	res, err := group.Call(ctx, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("opening the group: %v", err)
	}
	reference, _ := res.(map[string]any)["content"].(string)
	for _, want := range []string{"browser_navigate", "browser_snapshot", "Work from a snapshot."} {
		if !strings.Contains(reference, want) {
			t.Errorf("the group's reference is missing %q", want)
		}
	}
	t.Logf("reference is %d bytes: %s", len(reference), strings.SplitN(reference, ".", 2)[0])

	// Second open, on a fresh manager: the cache answers, so this must be fast
	// and must not start a browser.
	cached := NewManager(cfg, dir, nil, nil)
	cached.Defer(map[string]bool{"playwright": true})
	defer cached.Close()
	cachedGroup := tools.NewGroupTool("Web.browsing", "d", "", "web", 900,
		func(ctx context.Context) ([]tools.Tool, []tools.GroupNote, error) {
			return cached.ToolsForServers(ctx, []string{"playwright"})
		})
	started = time.Now()
	if _, err := cachedGroup.Call(ctx, json.RawMessage(`{}`)); err != nil {
		t.Fatalf("opening the group from cache: %v", err)
	}
	if took := time.Since(started); took > time.Second {
		t.Errorf("opening a cached group took %s; it should not have dialed", took.Round(time.Millisecond))
	}

	// Running a member reaches the real browser, through the member's own
	// confirm policy under its own name.
	out, err := group.Call(ctx, json.RawMessage(`{"tool":"browser_navigate","args":{"url":"https://example.com"}}`))
	if err != nil {
		t.Fatalf("browser_navigate through the group: %v", err)
	}
	if text := strings.ToLower(anyToString(out)); !strings.Contains(text, "example domain") {
		t.Errorf("navigation did not reach the page; got: %s", truncateForLog(text))
	}

	// An unapproved member is still gated: the group dispatches, it does not
	// bypass the trust policy.
	if _, err := group.Call(ctx, json.RawMessage(`{"tool":"browser_evaluate","args":{"function":"() => 1"}}`)); err == nil {
		t.Error("an unapproved member ran through the group without a prompt")
	}
}

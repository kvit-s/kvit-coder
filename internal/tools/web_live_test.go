package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// TestWebE2E exercises both web tools against the real endpoints. It is gated
// because it reaches the network and spends search quota; the tests above cover
// the behaviour against a local server. Run it with:
//
//	BRAVE_API_KEY=... KVIT_WEB_E2E=1 go test ./internal/tools/ -run TestWebE2E -v
func TestWebE2E(t *testing.T) {
	if os.Getenv("KVIT_WEB_E2E") == "" {
		t.Skip("set KVIT_WEB_E2E=1 to run")
	}
	cfg := webCfg(t)
	cfg.Tools.Web.APIKeyEnv = "BRAVE_API_KEY"

	fetch := NewWebFetchTool(cfg, NewTempFileManager(t.TempDir()))
	for _, u := range []string{"https://go.dev/blog/context", "https://example.com"} {
		raw, _ := json.Marshal(map[string]any{"url": u})
		got, err := fetch.Call(context.Background(), raw)
		if err != nil {
			t.Fatalf("fetch %s: %v", u, err)
		}
		m := got.(map[string]any)
		t.Logf("FETCH %s: lines=%v bytes=%v truncated=%v title=%q path=%v",
			u, m["lines"], m["bytes"], m["truncated"], m["title"], m["path"])
		if ol, ok := m["outline"].([]outlineEntry); ok {
			for i, e := range ol {
				if i >= 6 {
					t.Logf("   ... %d more headings", len(ol)-6)
					break
				}
				t.Logf("   %5d  %s%s", e.Line, strings.Repeat("  ", e.Level-1), e.Heading)
			}
			body, _ := os.ReadFile(m["path"].(string))
			t.Logf("   fenced code blocks in the saved file: %d", strings.Count(string(body), "```")/2)
		}
		if w, ok := m["warning"].(string); ok {
			t.Logf("   warning: %s", w)
		}
	}

	search := NewWebSearchTool(cfg)
	raw, _ := json.Marshal(map[string]any{"query": "golang context cancellation", "count": 3})
	got, err := search.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	m := got.(map[string]any)
	t.Logf("SEARCH: count=%v remaining_this_month=%v", m["count"], m["searches_left_this_month"])
	for _, r := range m["results"].([]webSearchResult) {
		t.Logf("   %s\n      %s\n      %s", r.Title, r.URL, r.Description)
	}
}

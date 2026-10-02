package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

func webCfg(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{}
	cfg.Tools.Read.Enabled = true
	cfg.Tools.Read.MaxPartialLines = 150
	cfg.Tools.Read.MaxReadSizeKB = 24
	cfg.Tools.Web.Search.Enabled = true
	cfg.Tools.Web.Fetch.Enabled = true
	return cfg
}

func call(t *testing.T, tool Tool, args any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshalling args: %v", err)
	}
	got, err := tool.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("%s returned an error: %v", tool.Name(), err)
	}
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("%s returned %T, want a map", tool.Name(), got)
	}
	return m
}

// --- conversion -----------------------------------------------------------

func TestConvertPageDropsFurnitureAndKeepsCode(t *testing.T) {
	page := `<html><head><title>Context</title><style>b{}</style></head>
<body>
<nav>Home About Contact</nav>
<header>site banner</header>
<div role="navigation">breadcrumbs</div>
<main>
<h1>Cancelling work</h1>
<p>A Context carries a deadline.</p>
<pre><code>func Background() Context
// the second line must survive
</code></pre>
</main>
<script>tracker()</script>
<footer>copyright</footer>
</body></html>`

	md, title, err := convertPage([]byte(page))
	if err != nil {
		t.Fatalf("convertPage: %v", err)
	}
	if title != "Context" {
		t.Errorf("title = %q, want %q", title, "Context")
	}
	for _, furniture := range []string{"Home About Contact", "site banner", "breadcrumbs", "tracker()", "copyright"} {
		if strings.Contains(md, furniture) {
			t.Errorf("furniture %q survived pruning:\n%s", furniture, md)
		}
	}
	if !strings.Contains(md, "Cancelling work") || !strings.Contains(md, "A Context carries a deadline.") {
		t.Errorf("content was lost:\n%s", md)
	}
	// The code block is the part that matters most: a documentation page whose
	// samples were reflowed into prose looks like it converted successfully.
	if !strings.Contains(md, "func Background() Context") {
		t.Errorf("code block content lost:\n%s", md)
	}
	if !strings.Contains(md, "the second line must survive") {
		t.Errorf("multi-line code block was truncated:\n%s", md)
	}
	if !strings.Contains(md, "```") {
		t.Errorf("code block was not fenced:\n%s", md)
	}
}

func TestBuildOutlineNumbersLinesAndIgnoresFencedHashes(t *testing.T) {
	md := strings.Join([]string{
		"# Title",         // 1
		"",                // 2
		"intro text",      // 3
		"## First",        // 4
		"```",             // 5
		"# not a heading", // 6
		"```",             // 7
		"### Nested",      // 8
		"#no space",       // 9 - not a heading
	}, "\n")

	got := buildOutline(md)
	want := []outlineEntry{
		{Line: 1, Level: 1, Heading: "Title"},
		{Line: 4, Level: 2, Heading: "First"},
		{Line: 8, Level: 3, Heading: "Nested"},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d entries, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("entry %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// --- Web.fetch ------------------------------------------------------------

func newFetchTool(t *testing.T) *WebFetchTool {
	t.Helper()
	return NewWebFetchTool(webCfg(t), NewTempFileManager(t.TempDir()))
}

func TestWebFetchSmallPageComesBackWhole(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Small</title></head><body><h1>Small</h1>
<p>`+strings.Repeat("enough text to clear the empty-page threshold. ", 12)+`</p></body></html>`)
	}))
	defer srv.Close()

	res := call(t, newFetchTool(t), map[string]any{"url": srv.URL})
	if _, truncated := res["truncated"]; truncated {
		t.Errorf("a short page was truncated: %v", res)
	}
	content, _ := res["content"].(string)
	if !strings.Contains(content, "Small") {
		t.Errorf("content missing the page: %q", content)
	}
	if _, ok := res["outline"]; ok {
		t.Error("a page that fits should come back whole, with no outline to follow")
	}
	if res["path"] == nil {
		t.Error("the conversion should be written to disk even when it fits")
	}
}

func TestWebFetchLargePageReturnsOutlineAndPath(t *testing.T) {
	var body strings.Builder
	body.WriteString("<html><head><title>Big</title></head><body>")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&body, "<h2>Section %d</h2>", i)
		body.WriteString("<p>" + strings.Repeat("filler sentence for bulk. ", 30) + "</p>")
	}
	body.WriteString("</body></html>")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body.String())
	}))
	defer srv.Close()

	res := call(t, newFetchTool(t), map[string]any{"url": srv.URL})
	if res["truncated"] != true {
		t.Fatalf("a long page should be truncated, got %v", res)
	}
	if _, ok := res["content"]; ok {
		t.Error("a truncated page should return head and outline, not content")
	}
	outline, ok := res["outline"].([]outlineEntry)
	if !ok || len(outline) == 0 {
		t.Fatalf("outline = %#v, want entries", res["outline"])
	}

	// The outline's line numbers must be the file's line numbers, or a Read at
	// the offset it gave lands somewhere else.
	path, _ := res["path"].(string)
	if path == "" {
		t.Fatal("no path to the spilled conversion")
	}
	onDisk, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading spilled file: %v", err)
	}
	lines := strings.Split(string(onDisk), "\n")
	for _, e := range outline {
		if e.Line < 1 || e.Line > len(lines) {
			t.Fatalf("outline line %d is outside the file's %d lines", e.Line, len(lines))
		}
		if !strings.Contains(lines[e.Line-1], e.Heading) {
			t.Errorf("line %d is %q, but the outline says it holds %q", e.Line, lines[e.Line-1], e.Heading)
		}
	}
}

// The hint on a long page is followed by calling Read, so every parameter it
// names has to be one Read accepts. Read ignores one it does not know, which
// for a misnamed start parameter means reading from line 1 again.
func TestWebFetchHintNamesParametersReadAccepts(t *testing.T) {
	var body strings.Builder
	body.WriteString("<html><head><title>Big</title></head><body>")
	for i := 0; i < 40; i++ {
		fmt.Fprintf(&body, "<h2>Section %d</h2>", i)
		body.WriteString("<p>" + strings.Repeat("filler sentence for bulk. ", 30) + "</p>")
	}
	body.WriteString("</body></html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body.String())
	}))
	defer srv.Close()

	res := call(t, newFetchTool(t), map[string]any{"url": srv.URL})
	hint, _ := res["hint"].(string)
	path, _ := res["path"].(string)
	outline, _ := res["outline"].([]outlineEntry)
	if hint == "" || path == "" || len(outline) < 2 {
		t.Fatalf("a long page should come with a hint, a path and an outline: %v", res)
	}

	cfg := newTestConfig()
	cfg.Workspace.Root = filepath.Dir(path)
	read := NewReadFileTool(cfg, NewToolContext())
	accepted := read.JSONSchema()["properties"].(map[string]any)
	for _, m := range regexp.MustCompile(`"(\w+)":`).FindAllStringSubmatch(hint, -1) {
		if _, ok := accepted[m[1]]; !ok {
			t.Errorf("hint names %q, which Read does not accept: %s", m[1], hint)
		}
	}

	target := outline[len(outline)-1]
	got := call(t, read, map[string]any{"path": path, "start": target.Line, "limit": 5})
	content, _ := got["content"].(string)
	first, _, _ := strings.Cut(content, "\n")
	if !strings.Contains(first, target.Heading) {
		t.Errorf("reading from outline line %d began with %q, want the heading %q", target.Line, first, target.Heading)
	}
}

func TestWebFetchWarnsWhenJavaScriptWasNeeded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		// A real application shell is kilobytes of script wrapped around an
		// empty div, which is what distinguishes it from a small real page.
		fmt.Fprint(w, `<html><head><title>App</title></head><body><div id="root"></div><script>`+
			strings.Repeat("var padding=1;/* inlined bundle */", 200)+`</script></body></html>`)
	}))
	defer srv.Close()

	res := call(t, newFetchTool(t), map[string]any{"url": srv.URL})
	warning, _ := res["warning"].(string)
	if !strings.Contains(warning, "JavaScript") {
		t.Errorf("an empty shell page should say a browser is needed, got %v", res)
	}
}

func TestWebFetchDoesNotCryWolfOnAGenuinelySmallPage(t *testing.T) {
	// example.com converts to about 170 bytes from about 560 of HTML and is a
	// real page. Only a short conversion from a *large* document means the
	// content was never rendered.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><head><title>Example Domain</title></head><body>`+
			`<h1>Example Domain</h1><p>This domain is for use in illustrative examples.</p></body></html>`)
	}))
	defer srv.Close()

	res := call(t, newFetchTool(t), map[string]any{"url": srv.URL})
	if _, warned := res["warning"]; warned {
		t.Errorf("a small but real page should not be reported as needing a browser: %v", res)
	}
	if res["path"] == nil {
		t.Error("a real page should still be written to disk")
	}
}

func TestLooksLikeJSShellNeedsBothConditions(t *testing.T) {
	cases := []struct {
		name     string
		markdown string
		rawBytes int
		want     bool
	}{
		{"small real page", "Example Domain\n\nillustrative examples.", 559, false},
		{"application shell", "", 40000, true},
		{"long page from long source", strings.Repeat("real content. ", 100), 40000, false},
		{"short page from short source", "hi", 100, false},
	}
	for _, c := range cases {
		if got := looksLikeJSShell(c.markdown, c.rawBytes); got != c.want {
			t.Errorf("%s: looksLikeJSShell = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestStripTagsDecodesNumericEntities(t *testing.T) {
	got := stripTags("removes the parent&#x27;s <strong>reference</strong> &amp; timers")
	want := "removes the parent's reference & timers"
	if got != want {
		t.Errorf("stripTags = %q, want %q", got, want)
	}
}

func TestWebFetchServesSecondCallFromDisk(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<html><body><p>`+strings.Repeat("cached body text. ", 20)+`</p></body></html>`)
	}))
	defer srv.Close()

	tool := newFetchTool(t)
	call(t, tool, map[string]any{"url": srv.URL})
	res := call(t, tool, map[string]any{"url": srv.URL})
	if hits != 1 {
		t.Errorf("the server was hit %d times; the second fetch should come from disk", hits)
	}
	if res["cached"] != true {
		t.Errorf("the second fetch should say it was cached, got %v", res)
	}
}

func TestWebFetchReportsHTTPErrorsWithoutFailingTheTurn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()

	res := call(t, newFetchTool(t), map[string]any{"url": srv.URL})
	if res["status"] != 404 {
		t.Errorf("status = %v, want 404", res["status"])
	}
	if res["error"] == nil {
		t.Error("a 404 should come back as a result the model can read, with an error field")
	}
}

func TestWebFetchSaysWhenTheDownloadCeilingCutThePage(t *testing.T) {
	// Truncated HTML converts to a fragment that looks like a small page, so
	// the ceiling biting has to be reported rather than left to be inferred.
	var body strings.Builder
	body.WriteString("<html><head><title>Big</title></head><body>")
	for i := 0; i < 200; i++ {
		fmt.Fprintf(&body, "<h2>Section %d</h2><p>%s</p>", i, strings.Repeat("filler. ", 40))
	}
	body.WriteString("</body></html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body.String())
	}))
	defer srv.Close()

	cfg := webCfg(t)
	cfg.Tools.Web.Fetch.MaxBytes = 4096
	tool := NewWebFetchTool(cfg, NewTempFileManager(t.TempDir()))

	res := call(t, tool, map[string]any{"url": srv.URL})
	if res["source_truncated"] != true {
		t.Errorf("a page cut off by the ceiling should say so, got %v", res)
	}
	if w, _ := res["warning"].(string); !strings.Contains(w, "max_bytes") {
		t.Errorf("the warning should name the setting to raise, got %q", w)
	}
}

func TestWebFetchIgnoresACallerSuppliedByteCap(t *testing.T) {
	// max_bytes is deliberately not in the schema: it caps the source download,
	// and a caller reading it as a cap on the result truncates the document
	// mid-tree. An argument that slips through anyway must not shrink the read.
	var body strings.Builder
	body.WriteString("<html><body>")
	for i := 0; i < 60; i++ {
		fmt.Fprintf(&body, "<h2>Heading %d</h2><p>%s</p>", i, strings.Repeat("substantive text. ", 20))
	}
	body.WriteString("</body></html>")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, body.String())
	}))
	defer srv.Close()

	res := call(t, newFetchTool(t), map[string]any{"url": srv.URL, "max_bytes": 500})
	if res["source_truncated"] == true {
		t.Error("a max_bytes argument should be ignored, not honoured as a source cap")
	}
	if lines, _ := res["lines"].(int); lines < 20 {
		t.Errorf("only %d lines came back; the whole page should have been read", lines)
	}
}

func TestWebFetchRefusesNonHTTPSchemes(t *testing.T) {
	for _, bad := range []string{"file:///etc/passwd", "ftp://example.com/x", "", "not a url at all"} {
		raw, _ := json.Marshal(map[string]any{"url": bad})
		if _, err := newFetchTool(t).Call(context.Background(), raw); err == nil {
			t.Errorf("%q was accepted; only http and https should be", bad)
		}
	}
}

// --- Web.search -----------------------------------------------------------

func newSearchTool(t *testing.T, baseURL string) *WebSearchTool {
	t.Helper()
	cfg := webCfg(t)
	cfg.Tools.Web.BaseURL = baseURL
	cfg.Tools.Web.APIKeyEnv = "KVIT_TEST_BRAVE_KEY"
	t.Setenv("KVIT_TEST_BRAVE_KEY", "test-key")
	tool := NewWebSearchTool(cfg)
	tool.sleep = func(context.Context, time.Duration) error { return nil } // no waiting in tests
	return tool
}

func braveBody(descriptions ...string) string {
	var results []string
	for i, d := range descriptions {
		results = append(results, fmt.Sprintf(
			`{"title":"Result %d","url":"https://example.com/%d","description":%q,"age":"2 days ago"}`, i, i, d))
	}
	return `{"web":{"results":[` + strings.Join(results, ",") + `]}}`
}

func TestWebSearchStripsHighlightingAndReportsRemaining(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Subscription-Token"); got != "test-key" {
			t.Errorf("token header = %q", got)
		}
		w.Header().Set("x-ratelimit-remaining", "0, 1997")
		w.Header().Set("x-ratelimit-reset", "1, 1960234")
		fmt.Fprint(w, braveBody("A <strong>Context</strong> carries a &amp; deadline"))
	}))
	defer srv.Close()

	res := call(t, newSearchTool(t, srv.URL), map[string]any{"query": "context"})
	if res["searches_left_this_month"] != 1997 {
		t.Errorf("searches_left_this_month = %v, want 1997", res["searches_left_this_month"])
	}
	results, ok := res["results"].([]webSearchResult)
	if !ok || len(results) != 1 {
		t.Fatalf("results = %#v", res["results"])
	}
	if got := results[0].Description; got != "A Context carries a & deadline" {
		t.Errorf("description = %q; tags and entities should be gone", got)
	}
}

func TestWebSearchRetriesAPerSecondCollision(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("x-ratelimit-remaining", "0, 1500")
		w.Header().Set("x-ratelimit-reset", "1, 1960234")
		if hits < 3 {
			// Monthly allowance still has room, so this is a collision in the
			// one-second window and clears on its own.
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprint(w, `{"error":{"code":"RATE_LIMITED"}}`)
			return
		}
		fmt.Fprint(w, braveBody("fine in the end"))
	}))
	defer srv.Close()

	res := call(t, newSearchTool(t, srv.URL), map[string]any{"query": "context"})
	if hits != 3 {
		t.Errorf("server saw %d requests, want 3 (two refusals then a success)", hits)
	}
	if res["count"] != 1 {
		t.Errorf("count = %v, want 1", res["count"])
	}
	if note, _ := res["note"].(string); !strings.Contains(note, "attempt 3") {
		t.Errorf("note = %q; the result should say it had to wait", note)
	}
}

func TestWebSearchDoesNotRetryAnExhaustedMonth(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("x-ratelimit-remaining", "0, 0")
		w.Header().Set("x-ratelimit-reset", "1, 1960234")
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"error":{"code":"RATE_LIMITED"}}`)
	}))
	defer srv.Close()

	raw, _ := json.Marshal(map[string]any{"query": "context"})
	_, err := newSearchTool(t, srv.URL).Call(context.Background(), raw)
	if err == nil {
		t.Fatal("an exhausted month should be an error, not a result")
	}
	if hits != 1 {
		t.Errorf("server saw %d requests; an exhausted month will not clear for weeks, so it must not be retried", hits)
	}
	if !strings.Contains(err.Error(), "do not retry") {
		t.Errorf("error = %q; it should tell the model retrying is pointless", err)
	}
}

func TestWebSearchGivesUpAfterMaxAttempts(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("x-ratelimit-remaining", "0, 1500")
		w.Header().Set("x-ratelimit-reset", "1, 1960234")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	tool := newSearchTool(t, srv.URL)
	tool.config.Tools.Web.Search.MaxAttempts = 4
	raw, _ := json.Marshal(map[string]any{"query": "context"})
	if _, err := tool.Call(context.Background(), raw); err == nil {
		t.Fatal("persistent rate limiting should be an error")
	}
	if hits != 4 {
		t.Errorf("server saw %d requests, want 4 (max_attempts)", hits)
	}
}

func TestWebSearchNeedsAKey(t *testing.T) {
	cfg := webCfg(t)
	cfg.Tools.Web.APIKeyEnv = "KVIT_TEST_ABSENT_KEY"
	t.Setenv("KVIT_TEST_ABSENT_KEY", "")
	raw, _ := json.Marshal(map[string]any{"query": "x"})
	_, err := NewWebSearchTool(cfg).Call(context.Background(), raw)
	if err == nil || !strings.Contains(err.Error(), "KVIT_TEST_ABSENT_KEY") {
		t.Errorf("error = %v; it should name the variable that is missing", err)
	}
}

func TestParseRateStateReadsBothPolicies(t *testing.T) {
	h := http.Header{}
	h.Set("x-ratelimit-remaining", "0, 1997")
	h.Set("x-ratelimit-reset", "1, 1960234")
	rs := parseRateState(h)
	if !rs.present || rs.perSecondRemaining != 0 || rs.monthlyRemaining != 1997 || rs.resetSeconds != 1 {
		t.Errorf("parseRateState = %+v", rs)
	}
	if empty := parseRateState(http.Header{}); empty.present {
		t.Error("headers with no rate information should not report present")
	}
}

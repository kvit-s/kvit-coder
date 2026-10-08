package tools

import (
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

const (
	defaultSearchBaseURL  = "https://api.search.brave.com"
	defaultSearchCount    = 5
	defaultSearchAttempts = 4
	defaultSearchTimeout  = 20 * time.Second
)

// WebSearchTool queries the Brave Search API.
//
// It keeps no count of the monthly quota. The key is normally shared with other
// programs, often on other machines, so nothing here can see what they spent --
// but every response carries the account-wide state in its x-ratelimit-*
// headers, which already includes them. Reading what the server reports is both
// simpler than local accounting and correct where local accounting would not be.
type WebSearchTool struct {
	config *config.Config
	client *http.Client
	// sleep is the delay between retries, replaceable in tests.
	sleep func(context.Context, time.Duration) error
}

func NewWebSearchTool(cfg *config.Config) *WebSearchTool {
	return &WebSearchTool{
		config: cfg,
		client: &http.Client{Timeout: searchTimeout(cfg)},
		sleep:  sleepCtx,
	}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func searchTimeout(cfg *config.Config) time.Duration {
	if s := cfg.Tools.Web.Search.Timeout; s > 0 {
		return time.Duration(s) * time.Second
	}
	return defaultSearchTimeout
}

func (t *WebSearchTool) Name() string { return "Web.search" }

func (t *WebSearchTool) Description() string {
	return "Search the web. Returns titles, URLs and short descriptions -- enough to choose a page with, not enough to answer from. Fetch what you choose with Web.fetch."
}

// ParallelSafe says several searches can run at once in a Batch. The provider's
// one-per-second limit will serialize them anyway, which costs wall clock
// rather than a model round trip.
func (t *WebSearchTool) ParallelSafe() bool { return true }

// SelfTimeout opts out of the loop's blanket 15-second tool timeout: with
// retries against a one-per-second limit a call can legitimately take longer.
func (t *WebSearchTool) SelfTimeout() bool { return true }

func (t *WebSearchTool) Check(ctx context.Context, args json.RawMessage) error { return nil }

func (t *WebSearchTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"query": map[string]any{
				"type":        "string",
				"description": "What to search for",
			},
			"count": map[string]any{
				"type":        "integer",
				"description": "How many results to return",
			},
			"freshness": map[string]any{
				"type":        "string",
				"description": "Restrict by age: pd (day), pw (week), pm (month), py (year)",
			},
			"country": map[string]any{
				"type":        "string",
				"description": "Two-letter country code to bias results towards",
			},
		},
		"required": []string{"query"},
	}
}

func (t *WebSearchTool) PromptCategory() string     { return "web" }
func (t *WebSearchTool) PromptOrder() int           { return 10 }
func (t *WebSearchTool) PromptTemplateName() string { return "" }

func (t *WebSearchTool) PromptSection() string {
	return `### Web.search - Search the Web

**Usage:** ` + "`" + `Web.search {"query": "<what to look for>"}` + "`" + `

Each result is a title, a URL and a description of a couple of sentences. The
description is there to choose a page with; it is not the page. When a result
looks right, read it with ` + "`Web.fetch`" + `.

The provider allows one request per second and a fixed number per month, shared
with other programs using the same key. The tool waits and retries a collision
by itself, so a 429 is not something to work around. If the monthly allowance is
gone the result says so, and there is no point trying again in a different
shape. Every result reports how many searches remain in the month.`
}

type webSearchResult struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Description string `json:"description"`
	Age         string `json:"age,omitempty"`
}

// rateState is what a response says about the account's limits. Both fields of
// each header carry two policies: per-second first, per-month second.
type rateState struct {
	perSecondRemaining int
	monthlyRemaining   int
	resetSeconds       int
	present            bool
}

func parseRateState(h http.Header) rateState {
	rs := rateState{perSecondRemaining: -1, monthlyRemaining: -1, resetSeconds: -1}
	pair := func(name string) (int, int, bool) {
		v := h.Get(name)
		if v == "" {
			return 0, 0, false
		}
		parts := strings.Split(v, ",")
		if len(parts) < 2 {
			return 0, 0, false
		}
		a, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
		b, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err1 != nil || err2 != nil {
			return 0, 0, false
		}
		return a, b, true
	}
	if sec, month, ok := pair("x-ratelimit-remaining"); ok {
		rs.perSecondRemaining, rs.monthlyRemaining, rs.present = sec, month, true
	}
	if sec, _, ok := pair("x-ratelimit-reset"); ok {
		rs.resetSeconds = sec
	}
	return rs
}

func (t *WebSearchTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Query     string `json:"query"`
		Count     int    `json:"count"`
		Freshness string `json:"freshness"`
		Country   string `json:"country"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	if strings.TrimSpace(params.Query) == "" {
		return nil, fmt.Errorf("query is required")
	}

	// The variable, else the key kvit-coder-ui saved under its name, which
	// stays out of the environment the Shell tool's commands inherit.
	key := config.LookupKey(t.keyEnv())
	if key == "" {
		return nil, fmt.Errorf("no API key: set %s, save one with :keys in kvit-coder-ui, or change tools.web.api_key_env to the variable holding it", t.keyEnv())
	}

	count := params.Count
	if count <= 0 {
		count = t.config.Tools.Web.Search.Count
	}
	if count <= 0 {
		count = defaultSearchCount
	}

	endpoint := t.baseURL() + "/res/v1/web/search"
	q := url.Values{}
	q.Set("q", params.Query)
	q.Set("count", strconv.Itoa(count))
	if params.Freshness != "" {
		q.Set("freshness", params.Freshness)
	}
	if params.Country != "" {
		q.Set("country", params.Country)
	}
	full := endpoint + "?" + q.Encode()

	attempts := t.config.Tools.Web.Search.MaxAttempts
	if attempts <= 0 {
		attempts = defaultSearchAttempts
	}

	started := time.Now()
	var lastRate rateState
	for attempt := 1; attempt <= attempts; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, full, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Subscription-Token", key)

		resp, err := t.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("searching: %w", err)
		}
		body, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, fmt.Errorf("reading search response: %w", readErr)
		}
		lastRate = parseRateState(resp.Header)

		switch {
		case resp.StatusCode == http.StatusOK:
			results, err := decodeBraveResults(body)
			if err != nil {
				return nil, err
			}
			out := map[string]any{
				"query":   params.Query,
				"results": results,
				"count":   len(results),
			}
			if lastRate.present {
				out["searches_left_this_month"] = lastRate.monthlyRemaining
			}
			if attempt > 1 {
				out["note"] = fmt.Sprintf("succeeded on attempt %d after waiting out the per-second limit", attempt)
			}
			t.logUsage(params.Query, "ok", len(results), time.Since(started), lastRate)
			return out, nil

		case resp.StatusCode == http.StatusTooManyRequests:
			// Two different refusals wear the same status code. A collision in
			// the one-second window clears in about a second and is worth
			// retrying; a month with nothing left in it will not clear for
			// weeks, so retrying only wastes the turn.
			if lastRate.present && lastRate.monthlyRemaining <= 0 {
				t.logUsage(params.Query, "quota_exhausted", 0, time.Since(started), lastRate)
				return nil, fmt.Errorf("the monthly search allowance is used up; it does not reset for weeks, so do not retry this. Answer from what you already know, or ask for a different approach")
			}
			if attempt == attempts {
				t.logUsage(params.Query, "rate_limited", 0, time.Since(started), lastRate)
				return nil, fmt.Errorf("search was rate limited on all %d attempts (one request per second is shared with other programs using this key); try again shortly", attempts)
			}
			if err := t.sleep(ctx, retryDelay(lastRate)); err != nil {
				return nil, err
			}
			continue

		default:
			t.logUsage(params.Query, "error", 0, time.Since(started), lastRate)
			return nil, fmt.Errorf("search failed: %s: %s", resp.Status, strings.TrimSpace(truncate(string(body), 300)))
		}
	}
	return nil, fmt.Errorf("search was rate limited on all %d attempts", attempts)
}

// retryDelay waits out the one-second window the server named, plus jitter so
// that several agents that collided do not collide again on the retry. A
// refused request costs no quota, so the only price of another attempt is the
// wall clock spent here.
func retryDelay(rs rateState) time.Duration {
	seconds := rs.resetSeconds
	if seconds <= 0 {
		seconds = 1
	}
	if seconds > 5 {
		seconds = 5
	}
	jitter := time.Duration(rand.Int63n(int64(500 * time.Millisecond)))
	return time.Duration(seconds)*time.Second + jitter
}

func decodeBraveResults(body []byte) ([]webSearchResult, error) {
	var payload struct {
		Web struct {
			Results []struct {
				Title       string `json:"title"`
				URL         string `json:"url"`
				Description string `json:"description"`
				Age         string `json:"age"`
			} `json:"results"`
		} `json:"web"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, fmt.Errorf("decoding search response: %w", err)
	}
	results := make([]webSearchResult, 0, len(payload.Web.Results))
	for _, r := range payload.Web.Results {
		results = append(results, webSearchResult{
			Title: stripTags(r.Title),
			URL:   r.URL,
			// Brave marks matched terms with <strong>. Left in, they are noise
			// in the context and the model may echo them back.
			Description: stripTags(r.Description),
			Age:         r.Age,
		})
	}
	return results, nil
}

// stripTags removes the handful of inline tags Brave uses for highlighting and
// unescapes the entities that come with them.
func stripTags(s string) string {
	var sb strings.Builder
	depth := 0
	for _, r := range s {
		switch {
		case r == '<':
			depth++
		case r == '>' && depth > 0:
			depth--
		case depth == 0:
			sb.WriteRune(r)
		}
	}
	// Unescape with the standard library rather than a list of the entities we
	// happen to have seen: Brave sends numeric ones too, and "&#x27;" showing
	// up verbatim in a description is the kind of thing a model copies out.
	return strings.TrimSpace(stdhtml.UnescapeString(sb.String()))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func (t *WebSearchTool) keyEnv() string {
	return t.config.WebSearchKeyEnv()
}

func (t *WebSearchTool) baseURL() string {
	if u := strings.TrimRight(strings.TrimSpace(t.config.Tools.Web.BaseURL), "/"); u != "" {
		return u
	}
	return defaultSearchBaseURL
}

// logUsage appends one line per search to a JSONL file. This is a record of
// what was searched and when, not a counter: the quota state comes from the
// response headers, and only the provider can see what other machines spent.
func (t *WebSearchTool) logUsage(query, status string, results int, took time.Duration, rs rateState) {
	path := strings.TrimSpace(t.config.Tools.Web.UsageLog)
	if path == "" {
		return
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return
		}
		path = filepath.Join(home, path[2:])
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	entry := map[string]any{
		"ts":          time.Now().Format(time.RFC3339Nano),
		"query":       query,
		"status":      status,
		"duration_ms": took.Milliseconds(),
	}
	if results > 0 {
		entry["results"] = results
	}
	if rs.present {
		entry["monthly_remaining"] = rs.monthlyRemaining
	}
	line, err := json.Marshal(entry)
	if err != nil {
		return
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "%s\n", line)
}

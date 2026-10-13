package llm

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestPlanUsageURLIsOnlyOpenCodeGo(t *testing.T) {
	cases := map[string]string{
		"https://opencode.ai/zen/go/v1":        "https://opencode.ai/zen/go/v1/usage",
		"https://opencode.ai/inference/go/v1/": "https://opencode.ai/inference/go/v1/usage",
		"https://opencode.ai/zen/v1":           "",
		"https://example.com/zen/go/v1":        "",
		"http://localhost:8080/v1":             "",
		"":                                     "",
	}
	for base, want := range cases {
		if got := planUsageURL(base); got != want {
			t.Errorf("planUsageURL(%q) = %q, want %q", base, got, want)
		}
	}
}

// openCodeUsage is what https://opencode.ai/zen/go/v1/usage answered on
// 2026-10-12, with the weekly window made the one that is used up.
const openCodeUsage = `{"usage":{"rolling":{"status":"ok","percent":37,"resetsAt":"2026-10-13T08:08:22.000Z"},` +
	`"weekly":{"status":"rate-limited","percent":100,"resetsAt":"2026-10-19T00:00:00.000Z"},` +
	`"monthly":{"status":"ok","percent":21,"resetsAt":"2026-10-25T16:54:53.000Z"}}}`

func TestPlanUsageReadsOpenCodeGo(t *testing.T) {
	var got *http.Request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.Clone(context.Background())
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(openCodeUsage))
	}))
	defer server.Close()

	c := NewClient(server.URL+"/zen/go/v1", "sk-test", WithHeaders(map[string]string{"x-opencode-session": "kvit-coder-run"}))
	c.planUsageURL = server.URL + "/zen/go/v1/usage"
	windows, err := c.PlanUsage(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Method != "GET" || got.URL.Path != "/zen/go/v1/usage" {
		t.Errorf("request %s %s, want GET /zen/go/v1/usage", got.Method, got.URL.Path)
	}
	if got.Header.Get("Authorization") != "Bearer sk-test" || got.Header.Get("x-opencode-session") != "kvit-coder-run" {
		t.Errorf("headers %v, want the key and the session header", got.Header)
	}

	want := []struct {
		name, status string
		used         float64
		resets       time.Time
	}{
		{"five_hour", "allowed", 0.37, time.Date(2026, 10, 13, 8, 8, 22, 0, time.UTC)},
		{"seven_day", "rejected", 1, time.Date(2026, 10, 19, 0, 0, 0, 0, time.UTC)},
		{"monthly", "allowed", 0.21, time.Date(2026, 10, 25, 16, 54, 53, 0, time.UTC)},
	}
	if len(windows) != len(want) {
		t.Fatalf("windows %+v, want %d", windows, len(want))
	}
	for i, w := range want {
		g := windows[i]
		if g.Name != w.name || g.Status != w.status || g.Used == nil || *g.Used != w.used || !g.ResetsAt.Equal(w.resets) {
			t.Errorf("window %d = %+v (used %v), want %+v", i, g, g.Used, w)
		}
	}
}

func TestPlanUsageAsksNothingWithoutAPlan(t *testing.T) {
	c := NewClient("http://localhost:1/v1", "")
	windows, err := c.PlanUsage(context.Background())
	if windows != nil || err != nil {
		t.Errorf("PlanUsage = %v, %v; want nothing", windows, err)
	}
}

func TestPlanUsageReportsARefusal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"EntitlementError","message":"OpenCode Go subscription required."}}`))
	}))
	defer server.Close()

	c := NewClient(server.URL, "sk-test")
	c.planUsageURL = server.URL + "/usage"
	_, err := c.PlanUsage(context.Background())
	var status *StatusError
	if !errors.As(err, &status) || status.Code != http.StatusForbidden {
		t.Errorf("err = %v, want the 403", err)
	}
}

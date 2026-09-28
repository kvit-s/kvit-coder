package repl

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

func titleTestSession(t *testing.T) *session.Session {
	t.Helper()
	sess, err := session.OpenDir(t.TempDir())
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	return sess
}

func titleTestConfig(entries ...config.ModelEntry) *config.Config {
	cfg := &config.Config{}
	cfg.Models = entries
	return cfg
}

// Without a summarizer the title is the prompt's first 5 words, and it is
// set on the session (the caller saves it).
func TestEnsureSessionTitleFallbackWithoutSummarizer(t *testing.T) {
	sess := titleTestSession(t)
	cfg := titleTestConfig(config.ModelEntry{ID: "main", Name: "Main", Model: "main-wire"})
	got := EnsureSessionTitle(context.Background(), cfg, sess, "fix the login bug now please", true, ui.NewWriter(0))
	if got != "fix the login bug now" {
		t.Errorf("title = %q, want first 5 words", got)
	}
	if sess.Meta().Title != got {
		t.Errorf("meta.Title = %q, want the returned title %q", sess.Meta().Title, got)
	}
}

// A legacy single-model config (no models: catalog) falls back the same way.
func TestEnsureSessionTitleFallbackLegacyConfig(t *testing.T) {
	sess := titleTestSession(t)
	cfg := &config.Config{}
	cfg.LLM.Model = "solo"
	got := EnsureSessionTitle(context.Background(), cfg, sess, "add dark mode with tests", true, nil)
	if got != "add dark mode with tests" {
		t.Errorf("title = %q, want first words", got)
	}
}

// An existing title is left alone: titling happens once.
func TestEnsureSessionTitleKeepsExisting(t *testing.T) {
	sess := titleTestSession(t)
	sess.Meta().Title = "Original title"
	got := EnsureSessionTitle(context.Background(), titleTestConfig(), sess, "a completely different prompt here", true, nil)
	if got != "Original title" || sess.Meta().Title != "Original title" {
		t.Errorf("existing title was overwritten: got %q, meta %q", got, sess.Meta().Title)
	}
}

// Wake turns carry no prompt, so there is nothing to title.
func TestEnsureSessionTitleSkipsWake(t *testing.T) {
	sess := titleTestSession(t)
	if got := EnsureSessionTitle(context.Background(), titleTestConfig(), sess, "", false, nil); got != "" {
		t.Errorf("wake turn title = %q, want empty", got)
	}
	if sess.Meta().Title != "" {
		t.Errorf("wake turn set meta.Title = %q, want empty", sess.Meta().Title)
	}
}

// A working summarizer's 3-6 word answer becomes the title.
func TestEnsureSessionTitleSummarizer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"x","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"Fix login redirect bug"},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	sess := titleTestSession(t)
	cfg := titleTestConfig(config.ModelEntry{
		ID: "cheap", Name: "Cheap", Model: "cheap-wire",
		BaseURL: srv.URL, APIBackend: "chat_completions", Summarizer: true,
	})
	got := EnsureSessionTitle(context.Background(), cfg, sess, "the login redirect after SSO is broken for users with expired tokens", true, nil)
	if got != "Fix login redirect bug" {
		t.Errorf("title = %q, want the summarizer answer", got)
	}
}

// A failing summarizer call falls back to the prompt's first words rather
// than failing the turn. 400 fails fast (no retry backoff).
func TestEnsureSessionTitleSummarizerFailureFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()
	sess := titleTestSession(t)
	cfg := titleTestConfig(config.ModelEntry{
		ID: "cheap", Name: "Cheap", Model: "cheap-wire",
		BaseURL: srv.URL, APIBackend: "chat_completions", Summarizer: true,
	})
	got := EnsureSessionTitle(context.Background(), cfg, sess, "fix the login bug now please", true, nil)
	if got != "fix the login bug now" {
		t.Errorf("title = %q, want fallback first 5 words", got)
	}
}

// A 5xx from the summarizer is tried once and then given up on. The client's
// default is ten retries waiting 1s, 2s, 4s, 8s, 16s in between, which held
// the start of every new session in silence when the summarizer endpoint was
// a local server answering 502 straight away.
func TestEnsureSessionTitleRetryableFailureIsTriedOnce(t *testing.T) {
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.Error(w, "no model loaded", http.StatusBadGateway)
	}))
	defer srv.Close()
	sess := titleTestSession(t)
	cfg := titleTestConfig(config.ModelEntry{
		ID: "cheap", Name: "Cheap", Model: "cheap-wire",
		BaseURL: srv.URL, APIBackend: "chat_completions", Summarizer: true,
	})
	got := EnsureSessionTitle(context.Background(), cfg, sess, "fix the login bug now please", true, nil)
	if got != "fix the login bug now" {
		t.Errorf("title = %q, want fallback first 5 words", got)
	}
	if requests != 1 {
		t.Errorf("summarizer endpoint saw %d requests, want 1", requests)
	}
}

// An unusable summarizer answer (empty content) falls back too.
func TestEnsureSessionTitleEmptyAnswerFallsBack(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"x","model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"   "},"finish_reason":"stop"}]}`)
	}))
	defer srv.Close()
	sess := titleTestSession(t)
	cfg := titleTestConfig(config.ModelEntry{
		ID: "cheap", Name: "Cheap", Model: "cheap-wire",
		BaseURL: srv.URL, APIBackend: "chat_completions", Summarizer: true,
	})
	got := EnsureSessionTitle(context.Background(), cfg, sess, "fix the login bug now please", true, nil)
	if got != "fix the login bug now" {
		t.Errorf("title = %q, want fallback", got)
	}
}

// Backfill: a session that predates titles (FirstPrompt set, Title empty)
// is titled from its first prompt, not the current one.
func TestEnsureSessionTitleBackfillsFromFirstPrompt(t *testing.T) {
	sess := titleTestSession(t)
	sess.Meta().FirstPrompt = "original first prompt about caching"
	cfg := titleTestConfig()
	got := EnsureSessionTitle(context.Background(), cfg, sess, "some later unrelated question here", true, nil)
	if got != "original first prompt about caching" {
		t.Errorf("title = %q, want the recorded first prompt's words", got)
	}
}

// The title call runs at the cheapest offered effort, not the entry default
// that serves real turns.
func TestSummarizerEffortCheapest(t *testing.T) {
	cfg := &config.Config{}
	entry := config.ModelEntry{ID: "go", Model: "m", APIBackend: "responses", Efforts: []config.EffortOption{
		{Value: "minimal"}, {Value: "low"}, {Value: "high", Default: true}, {Value: "xhigh"},
	}}
	if got := summarizerEffort(cfg, entry); got != "minimal" {
		t.Errorf("effort = %q, want cheapest offered", got)
	}
	plain := config.ModelEntry{ID: "local", Model: "m", APIBackend: "chat_completions"}
	if got := summarizerEffort(cfg, plain); got != "" {
		t.Errorf("non-reasoning effort = %q, want empty", got)
	}
}

// Endpoint fields fall back to the llm: block when the entry leaves them empty.
func TestSummarizerClientFallsBack(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.BaseURL = "https://main.example.com/v1"
	cfg.LLM.APIKey = "main-key"
	cfg.LLM.APIBackend = "chat_completions"
	entry := config.ModelEntry{ID: "s", Model: "s-wire", Summarizer: true}
	c, err := summarizerClientFor(context.Background(), cfg, entry)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil {
		t.Fatal("nil client")
	}
	// No accessor for the client's fields; the call below proves it targets
	// the fallback endpoint by failing fast against it (400, not DNS).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusBadRequest)
	}))
	defer srv.Close()
	cfg.LLM.BaseURL = srv.URL
	if _, err := summarizerTitle(context.Background(), cfg, entry, "hello"); err == nil {
		t.Error("want an error from the 400 endpoint, got nil")
	} else if !strings.Contains(err.Error(), "400") {
		t.Errorf("error %q does not mention the 400 status", err)
	}
}

func TestTruncatePrompt(t *testing.T) {
	short := "hello"
	if truncatePrompt(short) != short {
		t.Error("short prompt must pass through")
	}
	long := strings.Repeat("word ", 500)
	if got := truncatePrompt(long); len(got) > titlePromptChars {
		t.Errorf("truncated to %d chars, want <= %d", len(got), titlePromptChars)
	}
}

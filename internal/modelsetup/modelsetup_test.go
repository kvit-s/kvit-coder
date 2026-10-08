package modelsetup

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// testdata/models-dev.json is a few models cut from models.dev's api.json of
// 7 October 2026, plus one provider kvit-coder does not use.
func fixtureCatalog(t *testing.T) *Catalog {
	t.Helper()
	f, err := os.Open("testdata/models-dev.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := ParseCatalog(f, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func provider(t *testing.T, key string) Provider {
	t.Helper()
	for _, p := range Providers {
		if p.Key == key {
			return p
		}
	}
	t.Fatalf("no provider %q", key)
	return Provider{}
}

func TestParseCatalogKeepsKnownProviders(t *testing.T) {
	c := fixtureCatalog(t)
	if _, ok := c.Providers["deepseek"]; ok {
		t.Error("kept a provider no entry of Providers uses")
	}
	if got := len(c.Providers["opencode-go"].Models); got != 4 {
		t.Errorf("opencode-go has %d models, want 4", got)
	}
}

// TestRowsFromCatalog: the protocol, address, context size and effort menu of
// each row come from models.dev, which is the lookup work :setup saves.
func TestRowsFromCatalog(t *testing.T) {
	c := fixtureCatalog(t)
	goP := provider(t, "go")
	zen := provider(t, "zen")
	cases := []struct {
		p                       Provider
		id                      string
		backend, base, effortFd string
		context                 int
		efforts                 string
		unsupported             bool
	}{
		{goP, "muse-spark-1.3-contributor", llm.BackendResponses, "https://opencode.ai/zen/go/v1", "", 1048576, "minimal–xhigh", false},
		{goP, "kimi-k3", llm.BackendChatCompletions, "https://opencode.ai/zen/go/v1", "reasoning_effort", 1048576, "max", false},
		{goP, "minimax-m3", llm.BackendMessages, "https://opencode.ai/zen/go", "", 1000000, "", false},
		{zen, "claude-opus-5", llm.BackendMessages, "https://opencode.ai/zen", "", 1000000, "low–max", false},
		{zen, "gemini-3-pro", "", "", "", 0, "", true},
	}
	for _, tc := range cases {
		cands := Candidates(tc.p, []ListedModel{{ID: tc.id}}, c)
		cand := cands[0]
		if got := cand.Unsupported != ""; got != tc.unsupported {
			t.Errorf("%s: unsupported = %v (%q), want %v", tc.id, got, cand.Unsupported, tc.unsupported)
			continue
		}
		if tc.unsupported {
			continue
		}
		row := Row(tc.p, cand, nil)
		if row.APIBackend != tc.backend || row.BaseURL != tc.base || row.EffortField != tc.effortFd {
			t.Errorf("%s: backend %q at %q with effort_field %q, want %q at %q with %q",
				tc.id, row.APIBackend, row.BaseURL, row.EffortField, tc.backend, tc.base, tc.effortFd)
		}
		if row.Context != tc.context {
			t.Errorf("%s: context %d, want %d", tc.id, row.Context, tc.context)
		}
		if got := EffortSummary(row.Efforts); got != tc.efforts {
			t.Errorf("%s: efforts %q, want %q", tc.id, got, tc.efforts)
		}
		if row.APIKeyEnv != "OPENCODE_API_KEY" || len(row.Headers) != 2 {
			t.Errorf("%s: key %q headers %v, want OpenCode's", tc.id, row.APIKeyEnv, row.Headers)
		}
	}
}

func TestRowIDsAndNames(t *testing.T) {
	c := fixtureCatalog(t)
	goP := provider(t, "go")
	cand := Candidates(goP, []ListedModel{{ID: "kimi-k3"}}, c)[0]
	taken := map[string]bool{"kimi-k3-go": true, "kimi-k3-go-2": true}
	row := Row(goP, cand, func(id string) bool { return taken[id] })
	if row.ID != "kimi-k3-go-3" {
		t.Errorf("id = %q, want kimi-k3-go-3", row.ID)
	}
	if row.Name != "Kimi K3 (Go)" {
		t.Errorf("name = %q", row.Name)
	}

	local := LocalProvider("http://box:8000/v1/")
	cand = Candidates(local, []ListedModel{{ID: "Qwen/Qwen3-Coder:30B", Context: 65536}}, nil)[0]
	row = Row(local, cand, nil)
	if row.ID != "qwen-qwen3-coder-30b-local" || row.Name != "Qwen/Qwen3-Coder:30B (local)" {
		t.Errorf("local row id %q name %q", row.ID, row.Name)
	}
	if row.BaseURL != "http://box:8000/v1" || row.APIKeyEnv != "" || row.Context != 65536 || row.APIBackend != llm.BackendChatCompletions {
		t.Errorf("local row = %+v", row)
	}
}

func TestEffortsDefaultAndOrder(t *testing.T) {
	m := &CatalogModel{Reasoning: true, ReasoningOptions: []ReasoningOption{
		{Type: "toggle"},
		{Type: "effort", Values: []string{"xhigh", "low", "High", "turbo", "low"}},
	}}
	got := Efforts(m)
	want := []config.EffortOption{{Value: "low"}, {Value: "high", Default: true}, {Value: "xhigh"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Efforts = %+v, want %+v", got, want)
	}
	if Efforts(&CatalogModel{Reasoning: true, ReasoningOptions: []ReasoningOption{{Type: "effort", Values: []string{"low", "max"}}}})[1].Default != true {
		t.Error("without high the highest level should be the default")
	}
	if Efforts(&CatalogModel{Reasoning: false, ReasoningOptions: []ReasoningOption{{Type: "effort", Values: []string{"low"}}}}) != nil {
		t.Error("a model that does not reason got an effort menu")
	}
}

func TestDetail(t *testing.T) {
	c := fixtureCatalog(t)
	cands := Candidates(provider(t, "zen"), []ListedModel{{ID: "big-pickle"}, {ID: "unknown-one"}}, c)
	if d := cands[0].Detail(); !strings.Contains(d, "free") || !strings.Contains(d, "chat completions") {
		t.Errorf("big-pickle detail %q", d)
	}
	if d := cands[1].Detail(); d != "not on models.dev" {
		t.Errorf("undescribed detail %q", d)
	}
}

func TestTokensAndSlugs(t *testing.T) {
	for n, want := range map[int]string{1048576: "1.05M", 1000000: "1M", 262144: "262K", 200000: "200K", 0: "?"} {
		if got := Tokens(n); got != want {
			t.Errorf("Tokens(%d) = %q, want %q", n, got, want)
		}
	}
	for in, want := range map[string]string{"hf:zai-org/GLM-4.7": "hf-zai-org-glm-4.7", "My Proxy!": "my-proxy"} {
		if got := Slug(in); got != want {
			t.Errorf("Slug(%q) = %q, want %q", in, got, want)
		}
	}
	if got := KeyEnvFor("my proxy-2"); got != "MY_PROXY_2_API_KEY" {
		t.Errorf("KeyEnvFor = %q", got)
	}
}

// TestLoadCatalogCache: a fresh cache is used without fetching; a stale one
// is refreshed; a failed refresh falls back to the stale copy with a warning.
func TestLoadCatalogCache(t *testing.T) {
	fixture, err := os.ReadFile("testdata/models-dev.json")
	if err != nil {
		t.Fatal(err)
	}
	var fetches atomic.Int32
	fail := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fetches.Add(1)
		if fail {
			http.Error(w, "down", http.StatusBadGateway)
			return
		}
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()
	old := CatalogURL
	CatalogURL = srv.URL
	defer func() { CatalogURL = old }()

	dir := t.TempDir()
	now := time.Date(2026, 10, 7, 12, 0, 0, 0, time.UTC)
	c, warn, err := LoadCatalog(context.Background(), srv.Client(), dir, now)
	if err != nil || warn != "" || c.Model("opencode-go", "kimi-k3") == nil {
		t.Fatalf("first load: %v %q %v", c, warn, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "models-dev.json")); err != nil {
		t.Fatalf("no cache written: %v", err)
	}
	if _, _, err := LoadCatalog(context.Background(), srv.Client(), dir, now.Add(time.Hour)); err != nil || fetches.Load() != 1 {
		t.Fatalf("fresh cache refetched: fetches=%d err=%v", fetches.Load(), err)
	}
	fail = true
	c, warn, err = LoadCatalog(context.Background(), srv.Client(), dir, now.Add(25*time.Hour))
	if err != nil || c == nil || !strings.Contains(warn, "could not refresh") {
		t.Fatalf("stale fallback: %v %q %v", c, warn, err)
	}
	if _, _, err := LoadCatalog(context.Background(), srv.Client(), t.TempDir(), now); err == nil {
		t.Fatal("no cache and a failed fetch gave no error")
	}
}

func TestListModels(t *testing.T) {
	var gotAuth, gotSession string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			gotAuth, gotSession = r.Header.Get("Authorization"), r.Header.Get("X-Opencode-Session")
			_, _ = io.WriteString(w, `{"object":"list","data":[{"id":"b"},{"id":"a","max_model_len":32768},{"id":"b"}]}`)
		case "/props":
			_, _ = io.WriteString(w, `{"default_generation_settings":{"n_ctx":65536}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	models, err := ListModels(context.Background(), srv.Client(), srv.URL+"/v1/", "sk-1", map[string]string{"x-opencode-session": "s1"})
	if err != nil {
		t.Fatal(err)
	}
	want := []ListedModel{{ID: "a", Context: 32768}, {ID: "b"}}
	if !reflect.DeepEqual(models, want) {
		t.Errorf("models = %+v, want %+v", models, want)
	}
	if gotAuth != "Bearer sk-1" || gotSession != "s1" {
		t.Errorf("sent auth %q session %q", gotAuth, gotSession)
	}
	if n := ServerContext(context.Background(), srv.Client(), srv.URL+"/v1"); n != 65536 {
		t.Errorf("ServerContext = %d, want 65536", n)
	}
	if _, err := ListModels(context.Background(), srv.Client(), srv.URL+"/nothing", "", nil); err == nil || !strings.Contains(err.Error(), "404") {
		t.Errorf("a 404 gave %v", err)
	}
}

// fakeEndpoint answers chat completions, responses or messages requests for
// the paths in serve, refuses keys other than "good", and 404s the rest.
func fakeEndpoint(t *testing.T, serve map[string]bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer good" {
			http.Error(w, `{"error":"bad key"}`, http.StatusUnauthorized)
			return
		}
		if !serve[r.URL.Path] {
			http.Error(w, `{"error":"not served here"}`, http.StatusNotFound)
			return
		}
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/chat/completions":
			_, _ = io.WriteString(w, `{"id":"x","choices":[{"index":0,"finish_reason":"stop","message":{"role":"assistant","content":"OK"}}],"usage":{"prompt_tokens":1,"completion_tokens":1}}`)
		case "/v1/responses":
			_, _ = io.WriteString(w, `{"id":"r","status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":1,"output_tokens":1}}`)
		case "/v1/messages":
			_, _ = io.WriteString(w, `{"id":"m","type":"message","role":"assistant","content":[{"type":"text","text":"OK"}],"stop_reason":"end_turn","usage":{"input_tokens":1,"output_tokens":1}}`)
		}
	}))
}

func TestProbe(t *testing.T) {
	srv := fakeEndpoint(t, map[string]bool{"/v1/responses": true})
	defer srv.Close()
	p := OtherProvider("Fake", srv.URL+"/v1", "FAKE_API_KEY")
	entry := config.ModelEntry{ID: "m-fake", Model: "m", BaseURL: p.BaseURL, APIBackend: llm.BackendChatCompletions, APIKeyEnv: "FAKE_API_KEY"}

	if res := Probe(context.Background(), p, entry, "good", nil, false); res.OK() {
		t.Error("chat completions answered on an endpoint that only serves responses")
	}
	res := Probe(context.Background(), p, entry, "good", nil, true)
	if !res.OK() || res.Backend != llm.BackendResponses {
		t.Errorf("trying the others: backend %q err %v, want responses", res.Backend, res.Err)
	}
	res = Probe(context.Background(), p, entry, "bad", nil, true)
	if res.OK() || !res.KeyRefused {
		t.Errorf("a refused key: %+v", res)
	}
	if !strings.Contains(res.Err.Error(), "401") {
		t.Errorf("refused key error %q", res.Err)
	}
}

func TestProbeMessagesAddress(t *testing.T) {
	srv := fakeEndpoint(t, map[string]bool{"/v1/messages": true})
	defer srv.Close()
	p := OtherProvider("Fake", srv.URL+"/v1", "")
	entry := config.ModelEntry{ID: "m", Model: "m", BaseURL: p.BaseURL, APIBackend: llm.BackendChatCompletions}
	res := Probe(context.Background(), p, entry, "good", nil, true)
	if !res.OK() || res.Backend != llm.BackendMessages {
		t.Fatalf("backend %q err %v, want messages", res.Backend, res.Err)
	}
	SetBackend(p, &entry, res.Backend)
	if entry.BaseURL != srv.URL || entry.APIBackend != llm.BackendMessages {
		t.Errorf("after SetBackend: %+v", entry)
	}
}

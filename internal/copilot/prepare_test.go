package copilot

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

func TestBusinessExchangeAndChat(t *testing.T) {
	business := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/models":
			if r.Header.Get("Authorization") != "Bearer tid=session" {
				t.Errorf("models auth = %q", r.Header.Get("Authorization"))
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"claude-sonnet-4.6","supported_endpoints":["/v1/messages"],"policy":{"state":"enabled"}}]}`)
		case "/v1/messages":
			body, _ := io.ReadAll(r.Body)
			if r.Header.Get("Authorization") != "Bearer tid=session" {
				t.Errorf("messages auth = %q", r.Header.Get("Authorization"))
			}
			if !strings.Contains(string(body), `"model":"claude-sonnet-4.6"`) {
				t.Errorf("body = %s", body)
			}
			if r.Header.Get("X-Initiator") != "agent" {
				t.Errorf("initiator = %q, want agent for a tool result", r.Header.Get("X-Initiator"))
			}
			_, _ = io.WriteString(w, `{"content":[{"type":"text","text":"done"}],"stop_reason":"end_turn","usage":{"input_tokens":3,"output_tokens":1}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer business.Close()

	var userHost string
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/copilot_internal/user":
			userHost = "personal"
			_, _ = io.WriteString(w, `{"chat_enabled":true,"endpoints":{"api":"https://api.githubcopilot.com"}}`)
		case "/copilot_internal/v2/token":
			if r.Header.Get("Editor-Version") == "" {
				t.Error("token exchange missing Editor-Version")
			}
			_, _ = io.WriteString(w, `{"token":"tid=session","expires_at":`+strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)+
				`,"sku":"copilot_for_business_seat_quota","endpoints":{"api":"`+business.URL+`"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer github.Close()

	g := &gate{githubAPI: github.URL}
	res, err := g.prepare(context.Background(), Options{Model: "claude-sonnet-4.6", GitHubToken: "gho_test"})
	if err != nil {
		t.Fatal(err)
	}
	if res.BaseURL != business.URL {
		t.Fatalf("base = %s, want business %s (user endpoint had named the personal host)", res.BaseURL, business.URL)
	}
	if res.Backend != llm.BackendMessages {
		t.Fatalf("backend = %s, want messages", res.Backend)
	}
	if !res.Exchanged || res.APIKey != "tid=session" {
		t.Fatalf("result = %+v", res)
	}
	if userHost != "personal" {
		t.Fatal("user endpoint was not called")
	}

	client := llm.NewClient(res.BaseURL, res.APIKey,
		llm.WithBackend(res.Backend),
		llm.WithRequestAuthorizer(res.Auth))
	resp, err := client.Chat(context.Background(), llm.ChatRequest{
		Model: "claude-sonnet-4.6",
		Messages: []llm.Message{
			{Role: llm.RoleUser, Content: "do the thing"},
			{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: "t1", Type: "function", Function: llm.ToolCallFunction{Name: "Shell", Arguments: `{"cmd":"ls"}`}}}},
			{Role: llm.RoleTool, ToolCallID: "t1", Content: "file.go"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content != "done" {
		t.Fatalf("content = %q", resp.Choices[0].Message.Content)
	}
}

func TestTokenEndpointMissingFallsBackToUserHost(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			if r.Header.Get("Authorization") != "Bearer gho_test" {
				t.Errorf("auth = %q, want the GitHub token", r.Header.Get("Authorization"))
			}
			_, _ = io.WriteString(w, `{"data":[{"id":"gpt-4.1","supported_endpoints":["/chat/completions"]}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer api.Close()
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/copilot_internal/user":
			_, _ = io.WriteString(w, `{"chat_enabled":true,"endpoints":{"api":"`+api.URL+`"}}`)
		case "/copilot_internal/v2/token":
			http.NotFound(w, r)
		default:
			http.NotFound(w, r)
		}
	}))
	defer github.Close()

	g := &gate{githubAPI: github.URL}
	res, err := g.prepare(context.Background(), Options{Model: "gpt-4.1", GitHubToken: "gho_test"})
	if err != nil {
		t.Fatal(err)
	}
	if res.Exchanged {
		t.Fatal("404 from the token endpoint should fall back to the GitHub token")
	}
	if res.BaseURL != api.URL || res.Backend != llm.BackendChatCompletions {
		t.Fatalf("result base %s backend %s", res.BaseURL, res.Backend)
	}
}

func TestTokenExchangeForbiddenFallsBackToUserHost(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			if r.Header.Get("Authorization") != "Bearer gho_test" {
				t.Errorf("auth = %q, want the GitHub token", r.Header.Get("Authorization"))
			}
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, `{"data":[{"id":"gpt-5-mini","supported_endpoints":["/responses"],"capabilities":{"limits":{"max_context_window_tokens":264000},"supports":{"reasoning_effort":["low","medium","high"]}}}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer api.Close()
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/copilot_internal/user":
			_, _ = io.WriteString(w, `{"chat_enabled":true,"endpoints":{"api":"`+api.URL+`"}}`)
		case "/copilot_internal/v2/token":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, "403 Forbidden")
		default:
			http.NotFound(w, r)
		}
	}))
	defer github.Close()

	g := &gate{githubAPI: github.URL}
	cat, err := g.list(context.Background(), Options{GitHubToken: "gho_test"})
	if err != nil {
		t.Fatal(err)
	}
	if cat.Exchanged || cat.APIBase != api.URL {
		t.Fatalf("catalog exchanged=%v base=%s", cat.Exchanged, cat.APIBase)
	}
	if len(cat.Models) != 1 || cat.Models[0].ID != "gpt-5-mini" || cat.Models[0].Context != 264000 {
		t.Fatalf("models = %+v", cat.Models)
	}
	if got := strings.Join(cat.Models[0].Efforts, ","); got != "low,medium,high" {
		t.Fatalf("efforts = %s", got)
	}
}

func TestNoEntitlement(t *testing.T) {
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/copilot_internal/user" {
			_, _ = io.WriteString(w, `{"chat_enabled":false,"can_signup_for_limited":true}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer github.Close()
	g := &gate{githubAPI: github.URL}
	_, err := g.prepare(context.Background(), Options{Model: "gpt-4.1", GitHubToken: "gho_test"})
	if err == nil || !strings.Contains(err.Error(), "not signed up") {
		t.Fatalf("error = %v", err)
	}
}

func TestPinnedBaseURLWins(t *testing.T) {
	forced := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/models" {
			_, _ = io.WriteString(w, `{"data":[{"id":"gpt-5.4","supported_endpoints":["/responses"]}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer forced.Close()
	github := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/copilot_internal/user":
			_, _ = io.WriteString(w, `{"chat_enabled":true,"endpoints":{"api":"https://api.githubcopilot.com"}}`)
		case "/copilot_internal/v2/token":
			_, _ = io.WriteString(w, `{"token":"tid=session","expires_at":`+strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10)+
				`,"endpoints":{"api":"https://api.githubcopilot.com"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer github.Close()
	g := &gate{githubAPI: github.URL}
	res, err := g.prepare(context.Background(), Options{
		Model: "gpt-5.4", GitHubToken: "gho_test", BaseURL: forced.URL,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.BaseURL != forced.URL {
		t.Fatalf("base = %s, want the pinned host", res.BaseURL)
	}
	if res.Backend != llm.BackendResponses {
		t.Fatalf("backend = %s", res.Backend)
	}
}

func TestDefaultBackendNames(t *testing.T) {
	cases := []struct {
		model, want string
	}{
		{"claude-sonnet-4.6", llm.BackendMessages},
		{"gpt-5.4", llm.BackendResponses},
		{"gpt-5-mini", llm.BackendChatCompletions},
		{"gemini-3-pro", llm.BackendResponses},
		{"gpt-4.1", llm.BackendChatCompletions},
	}
	for _, tc := range cases {
		if got := DefaultBackend(tc.model); got != tc.want {
			t.Errorf("%s → %s, want %s", tc.model, got, tc.want)
		}
	}
}

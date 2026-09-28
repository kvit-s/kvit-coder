package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

type flipAuth struct {
	invalidated int
}

func (f *flipAuth) Authorize(_ context.Context, req *http.Request, _ []byte) error {
	if f.invalidated == 0 {
		req.Header.Set("Authorization", "Bearer stale")
	} else {
		req.Header.Set("Authorization", "Bearer fresh")
	}
	return nil
}

func (f *flipAuth) Invalidate() { f.invalidated++ }

func TestAuthorizerRefreshesOnceOn401(t *testing.T) {
	var saw []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		saw = append(saw, r.Header.Get("Authorization"))
		if r.Header.Get("Authorization") == "Bearer stale" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"expired"}`)
			return
		}
		_, _ = io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`)
	}))
	defer server.Close()
	auth := &flipAuth{}
	client := NewClient(server.URL, "ignored", WithRequestAuthorizer(auth), WithMaxRetries(0))
	resp, err := client.Chat(context.Background(), ChatRequest{
		Model:    "m",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("content = %q", resp.Choices[0].Message.Content)
	}
	if auth.invalidated != 1 || len(saw) != 2 || saw[0] != "Bearer stale" || saw[1] != "Bearer fresh" {
		t.Fatalf("invalidated %d saw %v", auth.invalidated, saw)
	}
}

func TestNewClient(t *testing.T) {
	client := NewClient("http://localhost:8080/v1", "test-key")
	if client == nil {
		t.Fatal("NewClient() returned nil")
	}
	if client.baseURL != "http://localhost:8080/v1" {
		t.Errorf("baseURL = %q, want %q", client.baseURL, "http://localhost:8080/v1")
	}
	if client.apiKey != "test-key" {
		t.Errorf("apiKey = %q, want %q", client.apiKey, "test-key")
	}
	if client.client == nil {
		t.Error("HTTP client is nil")
	}
}

func TestChatSuccess(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and path
		if r.Method != "POST" {
			t.Errorf("Method = %q, want POST", r.Method)
		}
		if r.URL.Path != "/chat/completions" {
			t.Errorf("Path = %q, want /chat/completions", r.URL.Path)
		}

		// Verify headers
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", r.Header.Get("Content-Type"))
		}
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("Authorization = %q, want Bearer test-key", r.Header.Get("Authorization"))
		}

		// Decode request
		var req ChatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("Failed to decode request: %v", err)
		}

		// Verify request content
		if req.Model != "test-model" {
			t.Errorf("Request.Model = %q, want test-model", req.Model)
		}
		if len(req.Messages) != 2 {
			t.Errorf("len(Request.Messages) = %d, want 2", len(req.Messages))
		}

		// Send mock response
		resp := ChatResponse{
			ID:    "chatcmpl-123",
			Model: "test-model",
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role:    RoleAssistant,
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: Usage{
				PromptTokens:     10,
				CompletionTokens: 15,
				TotalTokens:      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client
	client := NewClient(server.URL, "test-key")

	// Make request
	req := ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: RoleSystem, Content: "You are helpful."},
			{Role: RoleUser, Content: "Hello!"},
		},
		Temperature: 0.7,
	}

	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	// Verify response
	if resp.ID != "chatcmpl-123" {
		t.Errorf("Response.ID = %q, want chatcmpl-123", resp.ID)
	}
	if resp.Model != "test-model" {
		t.Errorf("Response.Model = %q, want test-model", resp.Model)
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("len(Response.Choices) = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Content != "Hello! How can I help you?" {
		t.Errorf("Response content = %q, want %q", resp.Choices[0].Message.Content, "Hello! How can I help you?")
	}
	if resp.Usage.TotalTokens != 25 {
		t.Errorf("Response.Usage.TotalTokens = %d, want 25", resp.Usage.TotalTokens)
	}
}

func TestChatWithoutAPIKey(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify no Authorization header
		if auth := r.Header.Get("Authorization"); auth != "" {
			t.Errorf("Authorization header should be empty, got %q", auth)
		}

		// Send mock response
		resp := ChatResponse{
			ID:    "chatcmpl-123",
			Model: "test-model",
			Choices: []Choice{
				{Index: 0, Message: Message{Role: RoleAssistant, Content: "Response"}, FinishReason: "stop"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	// Create client without API key
	client := NewClient(server.URL, "")

	req := ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello!"}},
	}

	_, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
}

func TestChatHTTPError(t *testing.T) {
	// Create mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error": "Invalid API key"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "invalid-key")

	req := ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello!"}},
	}

	_, err := client.Chat(context.Background(), req)
	if err == nil {
		t.Error("Chat() should return error for HTTP 401")
	}
}

func TestChatInvalidJSON(t *testing.T) {
	// Create mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{invalid json`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	req := ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello!"}},
	}

	_, err := client.Chat(context.Background(), req)
	if err == nil {
		t.Error("Chat() should return error for invalid JSON response")
	}
}

func TestChatContextCancellation(t *testing.T) {
	// Create mock server with delay
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Never respond, but stop waiting when the client goes away, or
		// Server.Close blocks on this handler forever.
		<-r.Context().Done()
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	req := ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "Hello!"}},
	}

	// Create context that's already cancelled
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := client.Chat(ctx, req)
	if err == nil {
		t.Error("Chat() should return error when context is cancelled")
	}
}

func TestChatWithToolCalls(t *testing.T) {
	// Create mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Send response with tool calls
		resp := ChatResponse{
			ID:    "chatcmpl-123",
			Model: "test-model",
			Choices: []Choice{
				{
					Index: 0,
					Message: Message{
						Role: RoleAssistant,
						ToolCalls: []ToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: ToolCallFunction{
									Name:      "get_weather",
									Arguments: `{"location":"San Francisco"}`,
								},
							},
						},
					},
					FinishReason: "tool_calls",
				},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	req := ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "What's the weather?"}},
		Tools: []ToolSpec{
			{
				Type: "function",
				Function: ToolSpecFunction{
					Name:        "get_weather",
					Description: "Get weather",
					Parameters: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"location": map[string]any{"type": "string"},
						},
					},
				},
			},
		},
	}

	resp, err := client.Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}

	if len(resp.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(resp.Choices))
	}
	if len(resp.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(resp.Choices[0].Message.ToolCalls))
	}

	tc := resp.Choices[0].Message.ToolCalls[0]
	if tc.ID != "call_123" {
		t.Errorf("ToolCall.ID = %q, want call_123", tc.ID)
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("ToolCall.Function.Name = %q, want get_weather", tc.Function.Name)
	}
}

// TestChatEffortSendsTemplateKwargs: a selected effort on the
// chat-completions backend rides as the Qwen template's per-request override;
// empty sends nothing (server default applies); an explicit per-request
// ChatTemplateKwargs wins (the interrogator's enable_thinking:false).
func TestChatEffortSendsTemplateKwargs(t *testing.T) {
	chatWith := func(t *testing.T, client *Client, req ChatRequest) map[string]any {
		t.Helper()
		var got map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				ChatTemplateKwargs map[string]any `json:"chat_template_kwargs"`
			}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Errorf("decode request: %v", err)
			}
			got = body.ChatTemplateKwargs
			resp := ChatResponse{ID: "c", Model: "m",
				Choices: []Choice{{Index: 0, Message: Message{Role: RoleAssistant, Content: "hi"}, FinishReason: "stop"}}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()
		client.baseURL = server.URL
		if _, err := client.Chat(context.Background(), req); err != nil {
			t.Fatalf("Chat: %v", err)
		}
		return got
	}
	base := ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "hi"}}}

	got := chatWith(t, NewClient("http://x/v1", "", WithReasoningEffort("medium")), base)
	if got["reasoning_effort"] != "medium" || got["enable_thinking"] != true {
		t.Errorf("effort medium sent %v, want enable_thinking:true + reasoning_effort:medium", got)
	}

	got = chatWith(t, NewClient("http://x/v1", ""), base)
	if len(got) != 0 {
		t.Errorf("empty effort sent %v, want nothing (server default)", got)
	}

	explicit := base
	explicit.ChatTemplateKwargs = map[string]any{"enable_thinking": false}
	got = chatWith(t, NewClient("http://x/v1", "", WithReasoningEffort("xhigh")), explicit)
	if got["enable_thinking"] != false {
		t.Errorf("explicit kwargs lost: %v", got)
	}
	if _, ok := got["reasoning_effort"]; ok {
		t.Errorf("explicit kwargs gained reasoning_effort: %v", got)
	}
}

// A 5xx is retryable, so by default it is tried again and again. A caller
// that would rather fall back than wait asks for fewer attempts, and zero
// means the one attempt with no ladder behind it.
func TestChatMaxRetriesCapsAttempts(t *testing.T) {
	tests := []struct {
		name         string
		maxRetries   int
		wantRequests int
	}{
		{"no retry", 0, 1},
		{"one retry", 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var requests int
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requests++
				http.Error(w, "no model loaded", http.StatusBadGateway)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-key", WithMaxRetries(tt.maxRetries))
			_, err := client.Chat(context.Background(), ChatRequest{
				Model:    "m",
				Messages: []Message{{Role: RoleUser, Content: "hi"}},
			})
			if err == nil {
				t.Fatal("want an error from the 502 endpoint, got nil")
			}
			if requests != tt.wantRequests {
				t.Errorf("server saw %d requests, want %d", requests, tt.wantRequests)
			}
		})
	}
}

// TestChatEffortFieldReasoningEffort: an endpoint configured for OpenAI's own
// spelling gets the effort in the top-level reasoning_effort field and no
// chat_template_kwargs at all. opencode.ai's /chat/completions reads only the
// former: sent the latter it answers normally and thinks for as long as it
// likes, so a wrong field here looks exactly like a working one.
func TestChatEffortFieldReasoningEffort(t *testing.T) {
	bodyOf := func(t *testing.T, client *Client, req ChatRequest) map[string]any {
		t.Helper()
		var got map[string]any
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
				t.Errorf("decode request: %v", err)
			}
			resp := ChatResponse{ID: "c", Model: "m",
				Choices: []Choice{{Index: 0, Message: Message{Role: RoleAssistant, Content: "hi"}, FinishReason: "stop"}}}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(resp)
		}))
		defer server.Close()
		client.baseURL = server.URL
		if _, err := client.Chat(context.Background(), req); err != nil {
			t.Fatalf("Chat: %v", err)
		}
		return got
	}
	base := ChatRequest{Model: "m", Messages: []Message{{Role: RoleUser, Content: "hi"}}}

	got := bodyOf(t, NewClient("http://x/v1", "",
		WithReasoningEffort("medium"), WithEffortField(EffortFieldReasoningEffort)), base)
	if got["reasoning_effort"] != "medium" {
		t.Errorf("reasoning_effort = %v, want medium", got["reasoning_effort"])
	}
	if _, ok := got["chat_template_kwargs"]; ok {
		t.Errorf("request also sent chat_template_kwargs: %v", got["chat_template_kwargs"])
	}

	// The default is still the chat template's key, so no existing endpoint
	// changes shape.
	got = bodyOf(t, NewClient("http://x/v1", "", WithReasoningEffort("medium")), base)
	if _, ok := got["reasoning_effort"]; ok {
		t.Errorf("default sent a top-level reasoning_effort: %v", got["reasoning_effort"])
	}
	if got["chat_template_kwargs"] == nil {
		t.Errorf("default dropped chat_template_kwargs: %v", got)
	}

	// An unknown field name leaves the default rather than sending nothing.
	got = bodyOf(t, NewClient("http://x/v1", "",
		WithReasoningEffort("high"), WithEffortField("somewhere_else")), base)
	if got["chat_template_kwargs"] == nil {
		t.Errorf("unknown effort_field dropped the effort entirely: %v", got)
	}

	// Empty effort sends neither field, whichever one is selected.
	got = bodyOf(t, NewClient("http://x/v1", "", WithEffortField(EffortFieldReasoningEffort)), base)
	if _, ok := got["reasoning_effort"]; ok {
		t.Errorf("empty effort sent reasoning_effort: %v", got["reasoning_effort"])
	}
}

// A model's thinking arrives under two different names: reasoning_content
// from DeepSeek, Qwen and the local servers, and reasoning from opencode.ai
// and OpenRouter. Both have to land in ReasoningContent, or merge_thinking
// has nothing to merge and the model looks like one that never thinks.
func TestMessageAcceptsReasoningAlias(t *testing.T) {
	var msg Message
	if err := json.Unmarshal([]byte(`{"role":"assistant","content":"done","reasoning":"first I checked"}`), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.ReasoningContent != "first I checked" {
		t.Errorf("ReasoningContent = %q, want the reasoning field's text", msg.ReasoningContent)
	}

	// reasoning_content wins when both are present: it is the field this
	// program writes itself.
	msg = Message{}
	if err := json.Unmarshal([]byte(`{"role":"assistant","reasoning_content":"kept","reasoning":"ignored"}`), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.ReasoningContent != "kept" {
		t.Errorf("ReasoningContent = %q, want kept", msg.ReasoningContent)
	}

	// A structured reasoning block is not text; it is left out rather than
	// rendered as JSON into the transcript.
	msg = Message{}
	if err := json.Unmarshal([]byte(`{"role":"assistant","content":"x","reasoning":{"text":"a"}}`), &msg); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if msg.ReasoningContent != "" {
		t.Errorf("ReasoningContent = %q, want empty for a non-text reasoning field", msg.ReasoningContent)
	}

	// Nothing sends it back: the wire message has only reasoning_content.
	out, err := json.Marshal(toChatWireMessage(Message{Role: RoleAssistant, Content: "x", ReasoningContent: "t"}))
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if bytes.Contains(out, []byte(`"reasoning"`)) {
		t.Errorf("wire message carries a reasoning field: %s", out)
	}
}

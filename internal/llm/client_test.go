package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

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
			Choices: []struct {
				Index        int          `json:"index"`
				Message      Message      `json:"message"`
				FinishReason string       `json:"finish_reason"`
				Error        *ChoiceError `json:"error,omitempty"`
			}{
				{
					Index: 0,
					Message: Message{
						Role:    RoleAssistant,
						Content: "Hello! How can I help you?",
					},
					FinishReason: "stop",
				},
			},
			Usage: struct {
				PromptTokens     int `json:"prompt_tokens"`
				CompletionTokens int `json:"completion_tokens"`
				TotalTokens      int `json:"total_tokens"`
			}{
				PromptTokens:     10,
				CompletionTokens: 15,
				TotalTokens:      25,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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
			Choices: []struct {
				Index        int          `json:"index"`
				Message      Message      `json:"message"`
				FinishReason string       `json:"finish_reason"`
				Error        *ChoiceError `json:"error,omitempty"`
			}{
				{Index: 0, Message: Message{Role: RoleAssistant, Content: "Response"}, FinishReason: "stop"},
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
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
		w.Write([]byte(`{"error": "Invalid API key"}`))
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
		w.Write([]byte(`{invalid json`))
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
		// Never respond
		select {}
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
			Choices: []struct {
				Index        int          `json:"index"`
				Message      Message      `json:"message"`
				FinishReason string       `json:"finish_reason"`
				Error        *ChoiceError `json:"error,omitempty"`
			}{
				{
					Index: 0,
					Message: Message{
						Role: RoleAssistant,
						ToolCalls: []ToolCall{
							{
								ID:   "call_123",
								Type: "function",
								Function: struct {
									Name      string `json:"name"`
									Arguments string `json:"arguments"`
								}{
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
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")

	req := ChatRequest{
		Model:    "test-model",
		Messages: []Message{{Role: RoleUser, Content: "What's the weather?"}},
		Tools: []ToolSpec{
			{
				Type: "function",
				Function: struct {
					Name        string         `json:"name"`
					Description string         `json:"description"`
					Parameters  map[string]any `json:"parameters"`
				}{
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

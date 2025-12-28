package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetGenerationStats(t *testing.T) {
	// Mock server that returns generation stats
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != "GET" {
			t.Errorf("Expected GET request, got %s", r.Method)
		}
		if r.URL.Path != "/generation" {
			t.Errorf("Expected /generation path, got %s", r.URL.Path)
		}

		id := r.URL.Query().Get("id")
		if id != "gen_123_abc" {
			t.Errorf("Expected id=gen_123_abc, got %s", id)
		}

		// Return mock stats
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": {
				"id": "gen_123_abc",
				"tokens_prompt": 150,
				"tokens_completion": 50,
				"native_tokens_prompt": 150,
				"native_tokens_completion": 50,
				"native_tokens_cached": 100
			}
		}`))
	}))
	defer server.Close()

	// Create client pointing to mock server
	client := NewClient(server.URL, "")

	// Query stats
	stats, err := client.GetGenerationStats(context.Background(), "gen_123_abc")
	if err != nil {
		t.Fatalf("GetGenerationStats failed: %v", err)
	}

	// Verify stats
	if stats.Data.ID != "gen_123_abc" {
		t.Errorf("Expected ID gen_123_abc, got %s", stats.Data.ID)
	}
	if stats.Data.TokensPrompt != 150 {
		t.Errorf("Expected 150 prompt tokens, got %d", stats.Data.TokensPrompt)
	}
	if stats.Data.TokensCompletion != 50 {
		t.Errorf("Expected 50 completion tokens, got %d", stats.Data.TokensCompletion)
	}
	if stats.Data.NativeTokensCached != 100 {
		t.Errorf("Expected 100 cached tokens, got %d", stats.Data.NativeTokensCached)
	}
}

func TestGetGenerationStatsError(t *testing.T) {
	// Mock server that returns error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error": "generation not found"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	_, err := client.GetGenerationStats(context.Background(), "invalid_id")
	if err == nil {
		t.Fatal("Expected error for 404 response, got nil")
	}
}

func TestGetGenerationStatsInvalidJSON(t *testing.T) {
	// Mock server that returns invalid JSON
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data": invalid json`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	_, err := client.GetGenerationStats(context.Background(), "test_id")
	if err == nil {
		t.Fatal("Expected error for invalid JSON, got nil")
	}
}

func TestChatResponseWithGenerationID(t *testing.T) {
	// Mock server that returns chat response with generation ID
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"id": "gen_456_def",
			"model": "test-model",
			"choices": [{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello!"
				}
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 5,
				"total_tokens": 15
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	resp, err := client.Chat(context.Background(), ChatRequest{
		Model: "test-model",
		Messages: []Message{
			{Role: RoleUser, Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Chat failed: %v", err)
	}

	// Verify generation ID is captured
	if resp.ID != "gen_456_def" {
		t.Errorf("Expected ID gen_456_def, got %s", resp.ID)
	}

	// Verify we can use this ID to query stats
	// (In practice, this would be a separate call after the chat)
}

func TestGenerationStatsTokenCalculation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"data": {
				"id": "gen_789_xyz",
				"tokens_prompt": 538,
				"tokens_completion": 213,
				"native_tokens_cached": 500
			}
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")

	stats, err := client.GetGenerationStats(context.Background(), "gen_789_xyz")
	if err != nil {
		t.Fatalf("GetGenerationStats failed: %v", err)
	}

	// Calculate total tokens (what the REPL would do)
	totalTokens := stats.Data.TokensPrompt + stats.Data.TokensCompletion
	expectedTotal := 538 + 213

	if totalTokens != expectedTotal {
		t.Errorf("Expected total tokens %d, got %d", expectedTotal, totalTokens)
	}
}

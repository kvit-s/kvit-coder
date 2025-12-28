package llm

import (
	"encoding/json"
	"testing"
)

func TestMessageRoles(t *testing.T) {
	tests := []struct {
		role MessageRole
		want string
	}{
		{RoleSystem, "system"},
		{RoleUser, "user"},
		{RoleAssistant, "assistant"},
		{RoleTool, "tool"},
	}

	for _, tt := range tests {
		if string(tt.role) != tt.want {
			t.Errorf("MessageRole %v = %q, want %q", tt.role, string(tt.role), tt.want)
		}
	}
}

func TestMessageJSON(t *testing.T) {
	tests := []struct {
		name    string
		message Message
		want    string
	}{
		{
			name: "user message",
			message: Message{
				Role:    RoleUser,
				Content: "Hello, world!",
			},
			want: `{"role":"user","content":"Hello, world!"}`,
		},
		{
			name: "assistant message",
			message: Message{
				Role:    RoleAssistant,
				Content: "Hi there!",
			},
			want: `{"role":"assistant","content":"Hi there!"}`,
		},
		{
			name: "system message",
			message: Message{
				Role:    RoleSystem,
				Content: "You are a helpful assistant.",
			},
			want: `{"role":"system","content":"You are a helpful assistant."}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("json.Marshal() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("json.Marshal() = %q, want %q", got, tt.want)
			}

			// Test unmarshal
			var msg Message
			if err := json.Unmarshal([]byte(tt.want), &msg); err != nil {
				t.Fatalf("json.Unmarshal() error = %v", err)
			}
			if msg.Role != tt.message.Role {
				t.Errorf("Role = %q, want %q", msg.Role, tt.message.Role)
			}
			if msg.Content != tt.message.Content {
				t.Errorf("Content = %q, want %q", msg.Content, tt.message.Content)
			}
		})
	}
}

func TestToolCallJSON(t *testing.T) {
	toolCallJSON := `{
		"id": "call_123",
		"type": "function",
		"function": {
			"name": "get_weather",
			"arguments": "{\"location\":\"San Francisco\"}"
		}
	}`

	var tc ToolCall
	if err := json.Unmarshal([]byte(toolCallJSON), &tc); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if tc.ID != "call_123" {
		t.Errorf("ID = %q, want %q", tc.ID, "call_123")
	}
	if tc.Type != "function" {
		t.Errorf("Type = %q, want %q", tc.Type, "function")
	}
	if tc.Function.Name != "get_weather" {
		t.Errorf("Function.Name = %q, want %q", tc.Function.Name, "get_weather")
	}
	if tc.Function.Arguments != `{"location":"San Francisco"}` {
		t.Errorf("Function.Arguments = %q, want %q", tc.Function.Arguments, `{"location":"San Francisco"}`)
	}
}

func TestChatRequestJSON(t *testing.T) {
	req := ChatRequest{
		Model: "gpt-4",
		Messages: []Message{
			{Role: RoleSystem, Content: "You are helpful."},
			{Role: RoleUser, Content: "Hello!"},
		},
		Temperature: 0.7,
		MaxTokens:   100,
		Stream:      false,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ChatRequest
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Model != req.Model {
		t.Errorf("Model = %q, want %q", decoded.Model, req.Model)
	}
	if len(decoded.Messages) != len(req.Messages) {
		t.Errorf("len(Messages) = %d, want %d", len(decoded.Messages), len(req.Messages))
	}
	if decoded.Temperature != req.Temperature {
		t.Errorf("Temperature = %f, want %f", decoded.Temperature, req.Temperature)
	}
	if decoded.MaxTokens != req.MaxTokens {
		t.Errorf("MaxTokens = %d, want %d", decoded.MaxTokens, req.MaxTokens)
	}
}

func TestChatResponseJSON(t *testing.T) {
	responseJSON := `{
		"id": "chatcmpl-123",
		"model": "gpt-4",
		"choices": [
			{
				"index": 0,
				"message": {
					"role": "assistant",
					"content": "Hello! How can I help?"
				}
			}
		],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 20,
			"total_tokens": 30
		}
	}`

	var resp ChatResponse
	if err := json.Unmarshal([]byte(responseJSON), &resp); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if resp.ID != "chatcmpl-123" {
		t.Errorf("ID = %q, want %q", resp.ID, "chatcmpl-123")
	}
	if resp.Model != "gpt-4" {
		t.Errorf("Model = %q, want %q", resp.Model, "gpt-4")
	}
	if len(resp.Choices) != 1 {
		t.Fatalf("len(Choices) = %d, want 1", len(resp.Choices))
	}
	if resp.Choices[0].Message.Role != RoleAssistant {
		t.Errorf("Choices[0].Message.Role = %q, want %q", resp.Choices[0].Message.Role, RoleAssistant)
	}
	if resp.Choices[0].Message.Content != "Hello! How can I help?" {
		t.Errorf("Choices[0].Message.Content = %q, want %q", resp.Choices[0].Message.Content, "Hello! How can I help?")
	}
	if resp.Usage.PromptTokens != 10 {
		t.Errorf("Usage.PromptTokens = %d, want 10", resp.Usage.PromptTokens)
	}
	if resp.Usage.CompletionTokens != 20 {
		t.Errorf("Usage.CompletionTokens = %d, want 20", resp.Usage.CompletionTokens)
	}
	if resp.Usage.TotalTokens != 30 {
		t.Errorf("Usage.TotalTokens = %d, want 30", resp.Usage.TotalTokens)
	}
}

func TestToolSpecJSON(t *testing.T) {
	spec := ToolSpec{
		Type: "function",
	}
	spec.Function.Name = "get_weather"
	spec.Function.Description = "Get the current weather"
	spec.Function.Parameters = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"location": map[string]any{
				"type":        "string",
				"description": "The city name",
			},
		},
		"required": []string{"location"},
	}

	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}

	var decoded ToolSpec
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}

	if decoded.Type != spec.Type {
		t.Errorf("Type = %q, want %q", decoded.Type, spec.Type)
	}
	if decoded.Function.Name != spec.Function.Name {
		t.Errorf("Function.Name = %q, want %q", decoded.Function.Name, spec.Function.Name)
	}
	if decoded.Function.Description != spec.Function.Description {
		t.Errorf("Function.Description = %q, want %q", decoded.Function.Description, spec.Function.Description)
	}
}

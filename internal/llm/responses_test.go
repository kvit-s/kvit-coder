package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMessagesToResponsesInput(t *testing.T) {
	msgs := []Message{
		{Role: RoleSystem, Content: "be terse"},
		{Role: RoleUser, Content: "read calc.py"},
		{
			Role:            RoleAssistant,
			Content:         "Reading it now.",
			ReasoningBlocks: []ReasoningBlock{{ID: "rs_1", EncryptedContent: "cipher"}},
			ToolCalls: []ToolCall{{ID: "call_1", Type: "function", Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{Name: "Read", Arguments: `{"path":"calc.py"}`}}},
			ToolCallItemIDs: map[string]string{"call_1": "fc_1"},
		},
		{Role: RoleTool, ToolCallID: "call_1", Content: "def add(a, b):"},
	}

	items := messagesToResponsesInput(msgs)

	wantTypes := []string{"message", "message", "reasoning", "message", "function_call", "function_call_output"}
	if len(items) != len(wantTypes) {
		t.Fatalf("got %d items, want %d: %+v", len(items), len(wantTypes), items)
	}
	for i, want := range wantTypes {
		if items[i].Type != want {
			t.Errorf("item %d: type %q, want %q", i, items[i].Type, want)
		}
	}

	if items[0].Role != "system" || items[0].Content != "be terse" {
		t.Errorf("system message not preserved: %+v", items[0])
	}

	reasoning := items[2]
	if reasoning.EncryptedContent != "cipher" || reasoning.ID != "rs_1" {
		t.Errorf("reasoning block not replayed verbatim: %+v", reasoning)
	}

	call := items[4]
	if call.CallID != "call_1" || call.ID != "fc_1" || call.Name != "Read" {
		t.Errorf("function call not replayed: %+v", call)
	}

	result := items[5]
	if result.CallID != "call_1" || result.Output != "def add(a, b):" {
		t.Errorf("tool result not replayed: %+v", result)
	}
}

// The provider rejects a reasoning item without a summary field, so an empty
// summary has to be serialized as [] rather than omitted.
func TestReasoningItemAlwaysSerializesSummary(t *testing.T) {
	items := messagesToResponsesInput([]Message{{
		Role:            RoleAssistant,
		ReasoningBlocks: []ReasoningBlock{{ID: "rs_1", EncryptedContent: "cipher"}},
	}})

	body, err := json.Marshal(items[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(body, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	summary, ok := got["summary"]
	if !ok {
		t.Fatalf("reasoning item has no summary field: %s", body)
	}
	if list, ok := summary.([]any); !ok || len(list) != 0 {
		t.Errorf("summary = %v, want empty list", summary)
	}

	// Every other item type must leave the field out entirely.
	userItems := messagesToResponsesInput([]Message{{Role: RoleUser, Content: "hi"}})
	body, _ = json.Marshal(userItems[0])
	var userItem map[string]any
	if err := json.Unmarshal(body, &userItem); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := userItem["summary"]; ok {
		t.Errorf("non-reasoning item carries a summary field: %s", body)
	}
}

func TestToolsToResponsesTools(t *testing.T) {
	var spec ToolSpec
	spec.Type = "function"
	spec.Function.Name = "Read"
	spec.Function.Description = "read a file"
	spec.Function.Parameters = map[string]any{"type": "object"}

	got := toolsToResponsesTools([]ToolSpec{spec})
	if len(got) != 1 {
		t.Fatalf("got %d tools, want 1", len(got))
	}
	// The Responses API wants name/description/parameters flat on the tool,
	// not nested under "function".
	if got[0].Name != "Read" || got[0].Description != "read a file" || got[0].Type != "function" {
		t.Errorf("tool not flattened: %+v", got[0])
	}
	if got[0].Parameters["type"] != "object" {
		t.Errorf("parameters lost: %+v", got[0].Parameters)
	}
}

func TestResponsesToChat(t *testing.T) {
	raw := `{
	  "id": "resp_1",
	  "model": "muse-spark-1.3-contributor",
	  "status": "completed",
	  "output": [
	    {"type":"reasoning","id":"rs_1","encrypted_content":"cipher","summary":[{"type":"summary_text","text":"thinking out loud"}]},
	    {"type":"message","id":"msg_1","role":"assistant","content":[{"type":"output_text","text":"Reading it now."}]},
	    {"type":"function_call","id":"fc_1","call_id":"call_1","name":"Read","arguments":"{\"path\":\"calc.py\"}"}
	  ],
	  "usage": {"input_tokens": 11, "output_tokens": 22, "total_tokens": 33}
	}`

	var resp responsesResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	chat := responsesToChat(&resp)

	if len(chat.Choices) != 1 {
		t.Fatalf("got %d choices, want 1", len(chat.Choices))
	}
	msg := chat.Choices[0].Message
	if msg.Content != "Reading it now." {
		t.Errorf("content = %q", msg.Content)
	}
	if msg.ReasoningContent != "thinking out loud" {
		t.Errorf("reasoning summary = %q", msg.ReasoningContent)
	}
	if len(msg.ReasoningBlocks) != 1 || msg.ReasoningBlocks[0].EncryptedContent != "cipher" {
		t.Errorf("reasoning block not kept: %+v", msg.ReasoningBlocks)
	}
	if len(msg.ToolCalls) != 1 {
		t.Fatalf("got %d tool calls, want 1", len(msg.ToolCalls))
	}
	// Tool results are matched by call_id, so that is what the agent must see.
	if msg.ToolCalls[0].ID != "call_1" || msg.ToolCalls[0].Function.Name != "Read" {
		t.Errorf("tool call = %+v", msg.ToolCalls[0])
	}
	if msg.ToolCallItemIDs["call_1"] != "fc_1" {
		t.Errorf("item id not kept: %+v", msg.ToolCallItemIDs)
	}
	if chat.Choices[0].FinishReason != "tool_calls" {
		t.Errorf("finish reason = %q, want tool_calls", chat.Choices[0].FinishReason)
	}
	if chat.Usage.PromptTokens != 11 || chat.Usage.CompletionTokens != 22 || chat.Usage.TotalTokens != 33 {
		t.Errorf("usage = %+v", chat.Usage)
	}
}

func TestResponsesToChatTruncated(t *testing.T) {
	resp := responsesResponse{Status: "incomplete"}
	resp.Output = []responsesItem{{Type: "message", Content: "half an ans"}}
	resp.IncompleteDetails = &struct {
		Reason string `json:"reason"`
	}{Reason: "max_output_tokens"}

	if got := responsesToChat(&resp).Choices[0].FinishReason; got != "length" {
		t.Errorf("finish reason = %q, want length", got)
	}
}

// The Responses backend must post to /responses, carry the configured extra
// headers, and pass the reasoning effort through.
func TestChatUsesResponsesBackend(t *testing.T) {
	var gotPath, gotSession string
	var gotBody map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotSession = r.Header.Get("x-opencode-session")
		json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"id":"resp_1","output":[{"type":"message","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key",
		WithBackend(BackendResponses),
		WithHeaders(map[string]string{"x-opencode-session": "kvit-coder"}),
		WithReasoningEffort("high"))

	resp, err := client.Chat(context.Background(), ChatRequest{
		Model:    "muse-spark-1.3-contributor",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}

	if gotPath != "/responses" {
		t.Errorf("posted to %q, want /responses", gotPath)
	}
	if gotSession != "kvit-coder" {
		t.Errorf("session header = %q", gotSession)
	}
	if reasoning, ok := gotBody["reasoning"].(map[string]any); !ok || reasoning["effort"] != "high" {
		t.Errorf("reasoning effort not sent: %v", gotBody["reasoning"])
	}
	if gotBody["stream"] != false || gotBody["store"] != false {
		t.Errorf("stream/store = %v/%v, want false/false", gotBody["stream"], gotBody["store"])
	}
	if resp.Choices[0].Message.Content != "OK" {
		t.Errorf("content = %q", resp.Choices[0].Message.Content)
	}
}

// An unset or unknown backend leaves the client on chat completions.
func TestChatDefaultsToChatCompletions(t *testing.T) {
	var gotPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Write([]byte(`{"id":"1","choices":[{"index":0,"message":{"role":"assistant","content":"OK"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", WithBackend("something-else"))
	if _, err := client.Chat(context.Background(), ChatRequest{Model: "m"}); err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotPath != "/chat/completions" {
		t.Errorf("posted to %q, want /chat/completions", gotPath)
	}
}

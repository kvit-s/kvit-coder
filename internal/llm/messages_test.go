package llm

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMessagesHistoryMergesToolResultsWithTheNextUser(t *testing.T) {
	// User prompt, assistant tool calls, two tool results, then the next
	// user sentence. The two results and the sentence are one user message:
	// the messages API rejects two user messages in a row.
	raw, _ := json.Marshal(messagesHistory(llmMessageFixture()))
	var msgs []struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	}
	if err := json.Unmarshal(raw, &msgs); err != nil {
		t.Fatal(err)
	}
	if len(msgs) != 3 {
		t.Fatalf("wire messages = %d (%s), want 3", len(msgs), raw)
	}
	if msgs[0].Role != "user" || msgs[1].Role != "assistant" || msgs[2].Role != "user" {
		t.Fatalf("roles = %s %s %s", msgs[0].Role, msgs[1].Role, msgs[2].Role)
	}
	var blocks []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(msgs[2].Content, &blocks); err != nil {
		t.Fatalf("merged user content: %v %s", err, msgs[2].Content)
	}
	if len(blocks) != 3 || blocks[0].Type != "tool_result" || blocks[1].Type != "tool_result" || blocks[2].Text != "thanks" {
		t.Fatalf("merged blocks = %+v", blocks)
	}
}

func llmMessageFixture() []Message {
	return []Message{
		{Role: RoleUser, Content: "look"},
		{Role: RoleAssistant, ToolCalls: []ToolCall{
			{ID: "a", Type: "function", Function: ToolCallFunction{Name: "Read", Arguments: `{"path":"a"}`}},
			{ID: "b", Type: "function", Function: ToolCallFunction{Name: "Read", Arguments: `{"path":"b"}`}},
		}},
		{Role: RoleTool, ToolCallID: "a", Content: "file a"},
		{Role: RoleTool, ToolCallID: "b", Content: "file b"},
		{Role: RoleUser, Content: "thanks"},
	}
}

func TestMessagesRoundTripReplaysThinking(t *testing.T) {
	var got map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/messages" {
			t.Errorf("path = %s", r.URL.Path)
		}
		if r.Header.Get("anthropic-version") == "" {
			t.Error("missing anthropic-version")
		}
		if r.Header.Get("anthropic-beta") == "" {
			t.Error("missing thinking beta")
		}
		body, _ := io.ReadAll(r.Body)
		if err := json.Unmarshal(body, &got); err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(w, `{
			"content":[
				{"type":"thinking","thinking":"step","signature":"sig"},
				{"type":"tool_use","id":"toolu_1","name":"Read","input":{"path":"a.go"}},
				{"type":"text","text":"done"}
			],
			"stop_reason":"tool_use",
			"usage":{"input_tokens":10,"output_tokens":4}
		}`)
	}))
	defer server.Close()

	client := NewClient(server.URL, "k", WithBackend(BackendMessages), WithReasoningEffort("high"))
	resp, err := client.Chat(context.Background(), ChatRequest{
		Model:    "claude-sonnet-4.6",
		Messages: []Message{{Role: RoleUser, Content: "hi"}},
		Tools:    []ToolSpec{{Type: "function", Function: ToolSpecFunction{Name: "Read", Parameters: map[string]any{"type": "object"}}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	msg := resp.Choices[0].Message
	if msg.Content != "done" || resp.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("choice = %+v", resp.Choices[0])
	}
	if len(msg.ToolCalls) != 1 || msg.ToolCalls[0].Function.Arguments != `{"path":"a.go"}` {
		t.Fatalf("tool calls = %+v", msg.ToolCalls)
	}
	if msg.ReasoningContent != "step" || len(msg.ReasoningBlocks) != 1 || msg.ReasoningBlocks[0].EncryptedContent != "sig" {
		t.Fatalf("thinking = %q blocks %+v", msg.ReasoningContent, msg.ReasoningBlocks)
	}
	if effort, _ := got["output_config"].(map[string]any); effort["effort"] != "high" {
		t.Fatalf("output_config = %v", got["output_config"])
	}

	// The next call of the tool loop has to send the signature back.
	replay := messagesHistory([]Message{
		{Role: RoleUser, Content: "hi"},
		msg,
		{Role: RoleTool, ToolCallID: "toolu_1", Content: "package p"},
	})
	raw, _ := json.Marshal(replay)
	if !strings.Contains(string(raw), `"signature":"sig"`) || !strings.Contains(string(raw), `"thinking":"step"`) {
		t.Fatalf("replay = %s", raw)
	}
}

package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/iotest"
)

// streamTestRequest covers what a request body has to carry: HTML characters
// and line separators that json.Marshal escapes, an image, reasoning blocks
// and tool-call item ids for the Responses API, a tool call and its result,
// and a tool schema with a nested field named like the conversation array.
func streamTestRequest() ChatRequest {
	return ChatRequest{
		Model: "m",
		Messages: []Message{
			{Role: RoleSystem, Content: "Be <careful> & exact."},
			{Role: RoleUser, Content: "Read a.go and \"fix\" it\n\twith tabs",
				Images: []ImagePart{{Path: "/tmp/x.png", MediaType: "image/png", Width: 2, Height: 2, Data: []byte("\x89PNG fake")}}},
			{Role: RoleAssistant, ReasoningContent: "thinking",
				ReasoningBlocks: []ReasoningBlock{{ID: "rs_1", EncryptedContent: "enc", Summary: []any{map[string]any{"type": "summary_text", "text": "s"}}}},
				ToolCalls:       []ToolCall{{ID: "c1", Type: "function", Function: ToolCallFunction{Name: "Read", Arguments: `{"path":"a.go"}`}}},
				ToolCallItemIDs: map[string]string{"c1": "fc_1"}},
			{Role: RoleTool, Name: "Read", ToolCallID: "c1", Content: "package main\n// <html> &   ✓"},
			{Role: RoleAssistant, Content: "Done."},
		},
		Tools: []ToolSpec{{Type: "function", Function: ToolSpecFunction{Name: "Read", Description: "reads",
			Parameters: map[string]any{"type": "object", "properties": map[string]any{
				"messages": map[string]any{"type": "array", "default": []any{}},
				"input":    map[string]any{"type": "array", "default": []any{}},
			}}}}},
		ReasoningEffort: "high",
	}
}

// TestRequestBodyMatchesMarshal: for each wire protocol, the streamed body is
// byte for byte what json.Marshal of the whole request produced, its length
// is right, and it reads the same in one-byte reads and when opened again.
func TestRequestBodyMatchesMarshal(t *testing.T) {
	client := NewClient("http://unused", "")
	for _, msgs := range [][]Message{streamTestRequest().Messages, streamTestRequest().Messages[:1], nil} {
		req := streamTestRequest()
		req.Messages = msgs
		cases := []struct {
			name string
			want func() ([]byte, error)
			body func() (*requestBody, error)
		}{
			{"chat completions",
				func() ([]byte, error) { return json.Marshal(toChatWireRequest(req)) },
				func() (*requestBody, error) { return chatRequestBody(req) }},
			{"responses",
				func() ([]byte, error) { return json.Marshal(client.buildResponsesRequest(req)) },
				func() (*requestBody, error) { return client.responsesRequestBody(req) }},
			{"messages",
				func() ([]byte, error) { return json.Marshal(client.buildMessagesRequest(req)) },
				func() (*requestBody, error) { return client.messagesRequestBody(req) }},
		}
		for _, tc := range cases {
			want, err := tc.want()
			if err != nil {
				t.Fatalf("%s: marshal: %v", tc.name, err)
			}
			body, err := tc.body()
			if err != nil {
				t.Fatalf("%s, %d messages: %v", tc.name, len(msgs), err)
			}
			got, err := body.Bytes()
			if err != nil {
				t.Fatalf("%s: read: %v", tc.name, err)
			}
			if !bytes.Equal(got, want) {
				t.Errorf("%s, %d messages: streamed body differs from json.Marshal\n got: %s\nwant: %s", tc.name, len(msgs), got, want)
			}
			if body.Len() != int64(len(want)) {
				t.Errorf("%s, %d messages: Len = %d, want %d", tc.name, len(msgs), body.Len(), len(want))
			}
			slow, err := io.ReadAll(iotest.OneByteReader(body.Open()))
			if err != nil || !bytes.Equal(slow, want) {
				t.Errorf("%s, %d messages: one-byte reads gave %d bytes, %v", tc.name, len(msgs), len(slow), err)
			}
		}
	}
}

// TestRequestBodySentWithLength: the request reaches the server with a
// Content-Length and without chunked encoding, so an endpoint or proxy that
// expects a length sees what it saw before.
func TestRequestBodySentWithLength(t *testing.T) {
	req := streamTestRequest()
	want, err := json.Marshal(toChatWireRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, _ := io.ReadAll(r.Body)
		if r.ContentLength != int64(len(want)) || len(r.TransferEncoding) != 0 {
			t.Errorf("Content-Length %d, Transfer-Encoding %v; want %d and none", r.ContentLength, r.TransferEncoding, len(want))
		}
		if !bytes.Equal(got, want) {
			t.Errorf("server received a different body\n got: %s\nwant: %s", got, want)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	resp, err := NewClient(server.URL, "", WithMaxRetries(0)).Chat(context.Background(), req)
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "ok" {
		t.Errorf("response = %+v", resp)
	}
}

// TestSplitAtEmptyArrayIgnoresNestedFields: a field of the same name inside
// another value is not the one replaced.
func TestSplitAtEmptyArrayIgnoresNestedFields(t *testing.T) {
	frame := []byte(`{"tools":[{"messages":[]}],"model":"m","messages":[],"tail":{"messages":[]}}`)
	head, tail, err := splitAtEmptyArray(frame, "messages")
	if err != nil {
		t.Fatal(err)
	}
	if string(head) != `{"tools":[{"messages":[]}],"model":"m","messages":[` || string(tail) != `],"tail":{"messages":[]}}` {
		t.Errorf("head %s, tail %s", head, tail)
	}
	if _, _, err := splitAtEmptyArray([]byte(`{"messages":[1]}`), "messages"); err == nil {
		t.Error("a non-empty array was accepted")
	}
	if _, _, err := splitAtEmptyArray([]byte(`{"model":"m"}`), "messages"); err == nil {
		t.Error("a missing field was accepted")
	}
}

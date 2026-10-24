package llm

import (
	"errors"
	"fmt"
	"testing"
)

func TestIsContextOverflow(t *testing.T) {
	overflow := []error{
		&StatusError{Code: 400, Body: []byte(`{"error":{"message":"This model's maximum context length is 128000 tokens.","code":"context_length_exceeded"}}`)},
		&StatusError{Code: 400, Body: []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"prompt is too long: 210000 tokens > 200000 maximum"}}`)},
		&StatusError{Code: 413, Body: []byte(`request entity too large`)},
		fmt.Errorf("after 3 retries: %w", &StatusError{Code: 400, Body: []byte(`the request exceeds the context window`)}),
		errors.New("API error 400: input is too long for requested model"),
	}
	for _, err := range overflow {
		if !IsContextOverflow(err) {
			t.Errorf("IsContextOverflow(%v) = false, want true", err)
		}
	}
	other := []error{
		nil,
		&StatusError{Code: 400, Body: []byte(`{"error":{"message":"Invalid schema for function 'Read'"}}`)},
		&StatusError{Code: 401, Body: []byte(`maximum context length`)},
		&StatusError{Code: 500, Body: []byte(`internal error`)},
		errors.New("execute request: connection refused"),
	}
	for _, err := range other {
		if IsContextOverflow(err) {
			t.Errorf("IsContextOverflow(%v) = true, want false", err)
		}
	}
}

// TestSetReasoningSummary: a turn past the tier 0 point stops asking for
// readable thinking on later requests.
func TestSetReasoningSummary(t *testing.T) {
	c := NewClient("http://example.test/v1", "", WithBackend(BackendResponses), WithReasoningEffort("high"), WithReasoningSummary("auto"))
	if got := c.buildResponsesRequest(ChatRequest{Model: "m"}).Reasoning; got == nil || got.Summary != "auto" {
		t.Fatalf("before: reasoning = %+v, want summary auto", got)
	}
	c.SetReasoningSummary("")
	got := c.buildResponsesRequest(ChatRequest{Model: "m"}).Reasoning
	if got == nil || got.Summary != "" || got.Effort != "high" {
		t.Errorf("after: reasoning = %+v, want the effort kept and no summary", got)
	}
}

func TestSetMergeThinking(t *testing.T) {
	n := NewResponseNormalizer(nil, true)
	msg := Message{Role: RoleAssistant, Content: "answer", ReasoningContent: "thought"}
	n.NormalizeResponse(&msg)
	if msg.Content != "thought\n\nanswer" {
		t.Fatalf("merged content = %q", msg.Content)
	}
	n.SetMergeThinking(false)
	msg = Message{Role: RoleAssistant, Content: "answer", ReasoningContent: "thought"}
	n.NormalizeResponse(&msg)
	if msg.Content != "answer" || msg.ReasoningContent != "" || n.MergeThinking() {
		t.Errorf("after turning merging off: content %q reasoning %q", msg.Content, msg.ReasoningContent)
	}
}

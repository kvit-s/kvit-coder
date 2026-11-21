package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func callMsg(ids ...string) Message {
	m := Message{Role: RoleAssistant}
	for _, id := range ids {
		tc := ToolCall{ID: id, Type: "function"}
		tc.Function.Name = "Shell"
		tc.Function.Arguments = `{"command":"ls"}`
		m.ToolCalls = append(m.ToolCalls, tc)
	}
	return m
}

func resultMsg(id string) Message {
	return Message{Role: RoleTool, Name: "Shell", ToolCallID: id, Content: "ok"}
}

// roles is the conversation as role:id pairs, the id being a tool result's
// call, so a test can compare whole shapes.
func roles(msgs []Message) []string {
	var out []string
	for _, m := range msgs {
		s := string(m.Role)
		if m.Role == RoleTool {
			s += ":" + m.ToolCallID
			if m.Content == StoppedCallNote {
				s += "(stopped)"
			}
		}
		out = append(out, s)
	}
	return out
}

func TestLastCallsUnanswered(t *testing.T) {
	cases := []struct {
		name string
		msgs []Message
		want []string
	}{
		{"ends in a call", []Message{{Role: RoleUser}, callMsg("a")}, []string{"a"}},
		{"ends in part of a batch", []Message{{Role: RoleUser}, callMsg("a", "b", "c"), resultMsg("b")}, []string{"a", "c"}},
		{"all answered", []Message{{Role: RoleUser}, callMsg("a"), resultMsg("a")}, nil},
		{"ends in an answer", []Message{{Role: RoleUser}, callMsg("a"), resultMsg("a"), {Role: RoleAssistant, Content: "done"}}, nil},
		{"call followed by a prompt", []Message{callMsg("a"), {Role: RoleUser}}, nil},
		{"empty", nil, nil},
		{"call without an id", []Message{callMsg("")}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LastCallsUnanswered(c.msgs)
			var ids []string
			for _, m := range got {
				if m.Role != RoleTool || m.Content != StoppedCallNote || m.Name != "Shell" {
					t.Errorf("result %+v is not a stopped-call note for Shell", m)
				}
				ids = append(ids, m.ToolCallID)
			}
			if fmt.Sprint(ids) != fmt.Sprint(c.want) {
				t.Errorf("answered %v, want %v", ids, c.want)
			}
		})
	}
}

// TestAnswerUnansweredCallsInsertsAfterTheCall is the session the crash left:
// a Shell call with no result, then the prompt of the run that continued it
// and the two "[System: Server error...]" messages its retries appended.
func TestAnswerUnansweredCallsInsertsAfterTheCall(t *testing.T) {
	msgs := []Message{
		{Role: RoleSystem},
		{Role: RoleUser},
		callMsg("a"), resultMsg("a"),
		callMsg("b", "c"), resultMsg("c"),
		callMsg("d"),
		{Role: RoleUser}, {Role: RoleUser}, {Role: RoleUser},
	}
	got := AnswerUnansweredCalls(msgs)
	want := []string{
		"system", "user",
		"assistant", "tool:a",
		"assistant", "tool:c", "tool:b(stopped)",
		"assistant", "tool:d(stopped)",
		"user", "user", "user",
	}
	if fmt.Sprint(roles(got)) != fmt.Sprint(want) {
		t.Errorf("got  %v\nwant %v", roles(got), want)
	}
	if len(msgs) != 10 || msgs[6].Role != RoleAssistant || msgs[7].Role != RoleUser {
		t.Errorf("the caller's messages were changed: %v", roles(msgs))
	}
}

func TestAnswerUnansweredCallsLeavesAnAnsweredHistory(t *testing.T) {
	msgs := []Message{{Role: RoleUser}, callMsg("a", "b"), resultMsg("a"), resultMsg("b"), {Role: RoleAssistant}}
	got := AnswerUnansweredCalls(msgs)
	if &got[0] != &msgs[0] || len(got) != len(msgs) {
		t.Errorf("an answered history was copied or changed: %v", roles(got))
	}
	// A result the endpoint would accept elsewhere in the input is enough;
	// a second one for the same call would be refused as a duplicate.
	late := []Message{callMsg("a"), {Role: RoleUser}, resultMsg("a")}
	if got := AnswerUnansweredCalls(late); len(got) != 3 {
		t.Errorf("a call answered later got a second result: %v", roles(got))
	}
}

func TestIsUnpairedToolCall(t *testing.T) {
	responses := &StatusError{Code: 400, Body: []byte(`{"error":{"message":"No tool output found for function call call_fa375dd6-60f0-48b5-957e-d746aaf0b07e.","type":"invalid_request_error","param":"input","code":null}}`)}
	chat := &StatusError{Code: 400, Body: []byte(`{"error":{"message":"An assistant message with 'tool_calls' must be followed by tool messages responding to each 'tool_call_id'. The following tool_call_ids did not have response messages: call_1"}}`)}
	anthropic := &StatusError{Code: 400, Body: []byte(`{"type":"error","error":{"type":"invalid_request_error","message":"messages.3: ` + "`tool_use`" + ` ids were found without ` + "`tool_result`" + ` blocks immediately after: toolu_1."}}`)}
	for name, err := range map[string]error{
		"responses":          responses,
		"wrapped":            fmt.Errorf("persistent: %w", responses),
		"chat completions":   chat,
		"anthropic":          anthropic,
		"as text, no status": fmt.Errorf("API error 400: No tool output found for function call call_1."),
	} {
		if !IsUnpairedToolCall(err) {
			t.Errorf("%s: %v not recognised", name, err)
		}
	}
	for name, err := range map[string]error{
		"nil":      nil,
		"overflow": &StatusError{Code: 400, Body: []byte(`{"error":{"message":"maximum context length is 8192 tokens"}}`)},
		"5xx":      &StatusError{Code: 500, Body: []byte(`No tool output found for function call x`)},
	} {
		if IsUnpairedToolCall(err) {
			t.Errorf("%s: %v taken for an unpaired tool call", name, err)
		}
	}
}

// TestChatSendsAResultForAStoppedCall sends the crashed session's shape
// through the Responses API client to an endpoint that refuses, as OpenAI's
// does, any function_call without a function_call_output. The client answers
// the call in the request, right after it, and the request is accepted.
func TestChatSendsAResultForAStoppedCall(t *testing.T) {
	var input []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Input []map[string]any `json:"input"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		input = body.Input
		outputs := map[string]bool{}
		for _, it := range input {
			if it["type"] == "function_call_output" {
				outputs[it["call_id"].(string)] = true
			}
		}
		for _, it := range input {
			if it["type"] == "function_call" && !outputs[it["call_id"].(string)] {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"error":{"message":"No tool output found for function call %s."}}`, it["call_id"])
				return
			}
		}
		_, _ = w.Write([]byte(`{"id":"resp_1","output":[{"type":"message","content":[{"type":"output_text","text":"OK"}]}],"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "key", WithBackend(BackendResponses))
	_, err := client.Chat(context.Background(), ChatRequest{Model: "m", Messages: []Message{
		{Role: RoleUser, Content: "gather"},
		callMsg("call_fa375dd6"),
		{Role: RoleUser, Content: "continue"},
	}})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	var shape []string
	for _, it := range input {
		shape = append(shape, it["type"].(string))
	}
	if want := "[message function_call function_call_output message]"; fmt.Sprint(shape) != want {
		t.Errorf("input = %v, want %s", shape, want)
	}
	if input[2]["output"] != StoppedCallNote {
		t.Errorf("output = %v", input[2]["output"])
	}
}

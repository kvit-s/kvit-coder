package repl

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/tools"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// strictModel answers the way OpenAI's Responses API does: a history with a
// tool call and no result is refused with a 400 naming the call, and anything
// else gets the answer "Done.". It sees the messages as the turn built them,
// before the client's own repair, so a test using it checks what the session
// gave the turn.
type strictModel struct {
	requests [][]llm.Message
}

func (m *strictModel) Chat(_ context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	m.requests = append(m.requests, append([]llm.Message(nil), req.Messages...))
	answered := map[string]bool{}
	for _, msg := range req.Messages {
		if msg.Role == llm.RoleTool {
			answered[msg.ToolCallID] = true
		}
	}
	for _, msg := range req.Messages {
		for _, tc := range msg.ToolCalls {
			if !answered[tc.ID] {
				return nil, &llm.StatusError{Code: 400, Body: []byte(fmt.Sprintf(
					`{"error":{"message":"No tool output found for function call %s.","type":"invalid_request_error"}}`, tc.ID))}
			}
		}
	}
	return &llm.ChatResponse{Choices: []llm.Choice{{
		Message:      llm.Message{Role: llm.RoleAssistant, Content: "Done."},
		FinishReason: "stop",
	}}}, nil
}

func (m *strictModel) GetGenerationStats(context.Context, string) (*llm.GenerationStats, error) {
	return nil, fmt.Errorf("no stats")
}

func turnFixture(t *testing.T, client agent.LLMClient) (*agent.Runner, *session.Session, *config.Config, *ui.Writer, *bytes.Buffer) {
	t.Helper()
	sess, err := session.OpenDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.LLM.Context = 1_000_000
	cfg.Agent.MaxIterations = 5
	var out bytes.Buffer
	writer := ui.NewWriter(0)
	writer.SetStdout(&out)
	writer.SetStderr(&out)
	writer.SetHeadless(true)
	logger, err := agent.NewLogger("", false)
	if err != nil {
		t.Fatal(err)
	}
	runner := agent.NewRunner(agent.RunnerOptions{
		Cfg: cfg, LLMClient: client, Registry: tools.NewRegistry(), Writer: writer, Logger: logger, ToolCtx: tools.NewToolContext(),
	})
	return runner, sess, cfg, writer, &out
}

// crashedSession writes what the run of 20 November left behind: a turn whose
// last message is a Shell call that was recorded and never run, because the
// process panicked while printing it.
func crashedSession(t *testing.T, sess *session.Session) llm.ToolCall {
	t.Helper()
	tc := llm.ToolCall{ID: "call_fa375dd6-60f0-48b5-957e-d746aaf0b07e", Type: "function"}
	tc.Function.Name = "Shell"
	tc.Function.Arguments = `{"command":"wc -l blocks.txt","timeout":60}`
	earlier := llm.ToolCall{ID: "call_dfb3c9ce", Type: "function"}
	earlier.Function.Name = "Shell"
	earlier.Function.Arguments = `{"command":"ls"}`
	if err := sess.Settings("test-model", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := sess.AppendMessages([]llm.Message{
		{Role: llm.RoleUser, Content: "Gather echo-ha's days."},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{earlier}},
		{Role: llm.RoleTool, Name: "Shell", ToolCallID: earlier.ID, Content: `{"exit_code":0}`},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{tc}},
	}); err != nil {
		t.Fatal(err)
	}
	return tc
}

// TestTurnAfterAStoppedToolCall opens a session that ends in a tool call with
// no result and runs a turn in it. The call is answered in the history before
// the turn's settings line, the model is sent a history it accepts, and the
// turn finishes.
func TestTurnAfterAStoppedToolCall(t *testing.T) {
	model := &strictModel{}
	runner, sess, cfg, writer, out := turnFixture(t, model)
	tc := crashedSession(t, sess)

	result, err := RunTurn(context.Background(), runner, writer, cfg, "system", "Finish the job.", true, sess, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunTurn: %v\n%s", err, out.String())
	}
	if result.Failure != nil {
		t.Fatalf("turn failed: %v\n%s", result.Failure, out.String())
	}
	if len(model.requests) != 1 {
		t.Fatalf("%d requests, want 1", len(model.requests))
	}
	if !strings.Contains(out.String(), "the last turn stopped before its Shell call had a result") {
		t.Errorf("no warning about the stopped call:\n%s", out.String())
	}

	// The history: the answer to the call straight after it, then the new
	// turn's settings line, prompt and answer.
	data, err := os.ReadFile(sess.HistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	var kinds []string
	for _, ln := range lines {
		switch {
		case strings.Contains(ln, `"kind":"settings"`):
			kinds = append(kinds, "settings")
		case strings.Contains(ln, `"kind":"tool_result"`) && strings.Contains(ln, tc.ID):
			kinds = append(kinds, "stopped-result")
		case strings.Contains(ln, `"kind":"tool_result"`):
			kinds = append(kinds, "result")
		case strings.Contains(ln, `"role":"user"`):
			kinds = append(kinds, "user")
		case strings.Contains(ln, `"role":"assistant"`):
			kinds = append(kinds, "assistant")
		default:
			kinds = append(kinds, "other")
		}
	}
	want := "[settings user assistant result assistant stopped-result settings user assistant]"
	if fmt.Sprint(kinds) != want {
		t.Errorf("history = %v\nwant      %s", kinds, want)
	}
	msgs, err := sess.Load()
	if err != nil {
		t.Fatal(err)
	}
	if got := msgs[4]; got.Role != llm.RoleTool || got.ToolCallID != tc.ID || got.Name != "Shell" || got.Content != llm.StoppedCallNote {
		t.Errorf("recorded result = %+v", got)
	}

	// A second turn finds nothing left to answer.
	before := len(msgs)
	if _, err := RunTurn(context.Background(), runner, writer, cfg, "system", "And again.", true, sess, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	msgs, _ = sess.Load()
	if len(msgs) != before+2 {
		t.Errorf("second turn added %d messages, want its prompt and answer", len(msgs)-before)
	}
}

// TestRefusedHistoryIsNotRetried: an endpoint that refuses the history for a
// tool call without output is asked once. The turn fails at once, rather than
// sending the same history five more times and appending "[System: Server
// error occurred...]" to the session after each of the last three.
func TestRefusedHistoryIsNotRetried(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"No tool output found for function call call_fa375dd6-60f0-48b5-957e-d746aaf0b07e.","type":"invalid_request_error","param":"input","code":null}}`))
	}))
	defer server.Close()
	client := llm.NewClient(server.URL, "key", llm.WithBackend(llm.BackendResponses), llm.WithMaxRetries(0))
	runner, sess, cfg, writer, out := turnFixture(t, client)

	result, err := RunTurn(context.Background(), runner, writer, cfg, "system", "Finish the job.", true, sess, nil, nil, nil)
	if err != nil {
		t.Fatalf("RunTurn: %v", err)
	}
	if result.Failure == nil || !llm.IsUnpairedToolCall(result.Failure) {
		t.Errorf("failure = %v, want the refusal", result.Failure)
	}
	if n := requests.Load(); n != 1 {
		t.Errorf("the endpoint was asked %d times, want 1", n)
	}
	if strings.Contains(out.String(), "retrying") {
		t.Errorf("the refusal was retried:\n%s", out.String())
	}
	data, _ := os.ReadFile(sess.HistoryPath())
	if strings.Contains(string(data), "Server error occurred") {
		t.Errorf("retry messages were written to the session:\n%s", data)
	}
}

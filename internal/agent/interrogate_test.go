package agent

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/stats"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

func TestBuildInterrogationQuestion(t *testing.T) {
	cases := []struct {
		trigger string
		want    []string
	}{
		{TriggerDuplicateCall, []string{"identical arguments", "make progress", "diagnostics only"}},
		{TriggerFatal, []string{"identical arguments", "diagnostics only"}},
		{TriggerBacktrack, []string{"identical arguments", "diagnostics only"}},
		{TriggerPendingBlocked, []string{"Edit.confirm", "Edit.cancel", "diagnostics only"}},
		{TriggerAlternatingLoop, []string{"alternating loop", "make progress", "diagnostics only"}},
	}
	for _, c := range cases {
		t.Run(c.trigger, func(t *testing.T) {
			q := buildInterrogationQuestion(Episode{
				Trigger:       c.trigger,
				OffendingTool: "Edit",
				OffendingArgs: `{"path":"a.py"}`,
				PriorResult:   "some result",
				PendingDiff:   "--- a.py\n+++ a.py",
			})
			for _, sub := range c.want {
				if !strings.Contains(q, sub) {
					t.Errorf("question for %s missing %q:\n%s", c.trigger, sub, q)
				}
			}
		})
	}
}

func TestNewInterrogatorDisabled(t *testing.T) {
	cfg := &config.Config{}
	cfg.Diagnostics.InterrogateOnAnomaly = false
	if it := NewInterrogator(cfg, nil, ui.NewWriter(0), nil, "run-x"); it != nil {
		t.Error("expected nil interrogator when disabled")
	}
	// nil-safe Enabled()
	var nilIt *Interrogator
	if nilIt.Enabled() {
		t.Error("nil interrogator should report not enabled")
	}
}

func TestLastToolResult(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "hi"},
		{Role: llm.RoleTool, Content: "first"},
		{Role: llm.RoleAssistant, Content: "thinking"},
		{Role: llm.RoleTool, Content: "second"},
		{Role: llm.RoleAssistant, Content: "done"},
	}
	if got := lastToolResult(msgs); got != "second" {
		t.Errorf("lastToolResult = %q, want %q", got, "second")
	}
	if got := lastToolResult([]llm.Message{{Role: llm.RoleUser, Content: "x"}}); got != "" {
		t.Errorf("lastToolResult with no tool message = %q, want empty", got)
	}
}

func TestSanitizeForFollowup(t *testing.T) {
	// History ending in an assistant message with unanswered tool_calls.
	var tc llm.ToolCall
	tc.ID = "call-1"
	tc.Type = "function"
	tc.Function.Name = "Edit"
	tc.Function.Arguments = `{"path":"a.py"}`
	history := []llm.Message{
		{Role: llm.RoleUser, Content: "go"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{tc}},
	}
	out := sanitizeForFollowup(history)
	if len(out) != 3 {
		t.Fatalf("expected synthetic tool response appended, got %d messages", len(out))
	}
	if out[2].Role != llm.RoleTool || out[2].ToolCallID != "call-1" {
		t.Errorf("synthetic tool message wrong: %+v", out[2])
	}
	// Original slice must be untouched.
	if len(history) != 2 {
		t.Errorf("sanitize mutated input history (len=%d)", len(history))
	}

	// History ending in a tool message is left as-is.
	clean := []llm.Message{
		{Role: llm.RoleUser, Content: "go"},
		{Role: llm.RoleTool, Content: "result"},
	}
	if got := sanitizeForFollowup(clean); len(got) != 2 {
		t.Errorf("clean history should be unchanged, got %d", len(got))
	}
}

// fakeChatServer returns an httptest server that answers /chat/completions with the
// given assistant content, and records how many requests it received.
func fakeChatServer(t *testing.T, answer string, count *int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*count++
		resp := map[string]any{
			"id":    "gen-1",
			"model": "test-model",
			"choices": []map[string]any{
				{"index": 0, "message": map[string]any{"role": "assistant", "content": answer}, "finish_reason": "stop"},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func interrogationTestState() *runState {
	return &runState{
		messages: []llm.Message{
			{Role: llm.RoleUser, Content: "do the task"},
			{Role: llm.RoleTool, Name: "Edit", Content: `{"status":"pending_confirmation"}`},
		},
		agentStats: &stats.AgentStats{Steps: 3},
	}
}

func TestInterrogateEndToEndAndLog(t *testing.T) {
	var reqCount int
	srv := fakeChatServer(t, "I expected the file to already be edited.", &reqCount)

	logDir := t.TempDir()
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Diagnostics.InterrogateOnAnomaly = true
	cfg.Diagnostics.InterrogateLogDir = logDir

	client := llm.NewClient(srv.URL, "")
	it := NewInterrogator(cfg, client, ui.NewWriter(0), nil, "run-test")
	if it == nil {
		t.Fatal("expected non-nil interrogator")
	}

	state := interrogationTestState()
	ep := Episode{
		Trigger:       TriggerDuplicateCall,
		Key:           "duplicate_call:Edit:args",
		TriggerCount:  2,
		OffendingTool: "Edit",
		OffendingArgs: `{"path":"a.py"}`,
		PriorResult:   "prev result",
	}
	it.Interrogate(context.Background(), ep, state)

	if reqCount != 1 {
		t.Fatalf("expected 1 interrogation request, got %d", reqCount)
	}
	if state.interrogationCount != 1 {
		t.Fatalf("expected interrogationCount 1, got %d", state.interrogationCount)
	}

	// Same episode key again -> deduped, no new request.
	it.Interrogate(context.Background(), ep, state)
	if reqCount != 1 {
		t.Fatalf("expected dedup (still 1 request), got %d", reqCount)
	}

	// Read the log file and verify the record.
	rec := readSingleInterrogationRecord(t, filepath.Join(logDir, "run-test.jsonl"))
	if rec["trigger"] != TriggerDuplicateCall {
		t.Errorf("trigger = %v", rec["trigger"])
	}
	if rec["answer"] != "I expected the file to already be edited." {
		t.Errorf("answer = %v", rec["answer"])
	}
	if rec["model"] != "test-model" {
		t.Errorf("model = %v", rec["model"])
	}
	oc, ok := rec["offending_call"].(map[string]any)
	if !ok || oc["tool"] != "Edit" {
		t.Errorf("offending_call = %v", rec["offending_call"])
	}
}

func TestInterrogatePrintsQAToScreen(t *testing.T) {
	var reqCount int
	srv := fakeChatServer(t, "Because I thought the edit was already applied.", &reqCount)

	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Diagnostics.InterrogateOnAnomaly = true
	cfg.Diagnostics.InterrogateLogDir = t.TempDir()

	// Headless writer routes progress (Warn/Info) to stderr, which we capture.
	var screen bytes.Buffer
	writer := ui.NewWriter(0)
	writer.SetHeadless(true)
	writer.SetStderr(&screen)

	it := NewInterrogator(cfg, llm.NewClient(srv.URL, ""), writer, nil, "run-screen")
	state := interrogationTestState()

	it.Interrogate(context.Background(), Episode{
		Trigger:       TriggerDuplicateCall,
		Key:           "k",
		OffendingTool: "Edit",
		OffendingArgs: `{"path":"a.py"}`,
		PriorResult:   "prev",
	}, state)

	out := screen.String()
	if !strings.Contains(out, "Interrogation Q:") {
		t.Errorf("screen output missing question label:\n%s", out)
	}
	if !strings.Contains(out, "Interrogation A:") || !strings.Contains(out, "Because I thought the edit was already applied.") {
		t.Errorf("screen output missing answer:\n%s", out)
	}
}

func TestInterrogateRespectsPerTaskCap(t *testing.T) {
	var reqCount int
	srv := fakeChatServer(t, "answer", &reqCount)

	logDir := t.TempDir()
	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Diagnostics.InterrogateOnAnomaly = true
	cfg.Diagnostics.InterrogateLogDir = logDir
	cfg.Diagnostics.InterrogateMaxPerTask = 2

	it := NewInterrogator(cfg, llm.NewClient(srv.URL, ""), ui.NewWriter(0), nil, "run-cap")
	state := interrogationTestState()

	// Four distinct episodes, cap is 2.
	for i := 0; i < 4; i++ {
		it.Interrogate(context.Background(), Episode{
			Trigger:       TriggerDuplicateCall,
			Key:           "ep-" + string(rune('a'+i)),
			OffendingTool: "Edit",
		}, state)
	}
	if reqCount != 2 {
		t.Errorf("expected cap to limit to 2 requests, got %d", reqCount)
	}
}

func TestInterrogateTriggerFilter(t *testing.T) {
	var reqCount int
	srv := fakeChatServer(t, "answer", &reqCount)

	cfg := &config.Config{}
	cfg.LLM.Model = "test-model"
	cfg.Diagnostics.InterrogateOnAnomaly = true
	cfg.Diagnostics.InterrogateLogDir = t.TempDir()
	cfg.Diagnostics.InterrogateTriggers = []string{TriggerPendingBlocked} // only this one

	it := NewInterrogator(cfg, llm.NewClient(srv.URL, ""), ui.NewWriter(0), nil, "run-filter")
	state := interrogationTestState()

	// Disabled trigger -> no request.
	it.Interrogate(context.Background(), Episode{Trigger: TriggerDuplicateCall, Key: "k1", OffendingTool: "Edit"}, state)
	if reqCount != 0 {
		t.Errorf("expected filtered-out trigger to make 0 requests, got %d", reqCount)
	}
	// Enabled trigger -> one request.
	it.Interrogate(context.Background(), Episode{Trigger: TriggerPendingBlocked, Key: "k2", OffendingTool: "Read"}, state)
	if reqCount != 1 {
		t.Errorf("expected enabled trigger to make 1 request, got %d", reqCount)
	}
}

func readSingleInterrogationRecord(t *testing.T, path string) map[string]any {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open log: %v", err)
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	if !sc.Scan() {
		t.Fatal("log file empty")
	}
	var rec map[string]any
	if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
		t.Fatalf("unmarshal record: %v", err)
	}
	return rec
}

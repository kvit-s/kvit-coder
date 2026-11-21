package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
)

// panicOnCallLine is an output stream that panics when it is handed the line
// printing an echo call, and keeps everything else.
type panicOnCallLine struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (p *panicOnCallLine) Write(b []byte) (int, error) {
	if bytes.Contains(b, []byte("echo[")) {
		panic("printing failed")
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.buf.Write(b)
}

// TestToolRunsWhenPrintingItsCallPanics: printing a tool call happens after
// the call is recorded and before the tool runs, so a panic there used to end
// the process and leave the call without a result. The run now says it could
// not print the call, runs the tool, and finishes.
func TestToolRunsWhenPrintingItsCallPanics(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "echo", map[string]string{"arg": "hello"})),
		answer("done"),
	)
	ran := false
	echo := &scriptedTool{name: "echo", call: func(_ context.Context, args json.RawMessage) (any, error) {
		ran = true
		return map[string]any{"echoed": string(args)}, nil
	}}
	runner, _ := newTestRunner(t, testConfig(), client, echo)
	out := &panicOnCallLine{}
	runner.writer.SetStderr(out)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("say hello")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !ran {
		t.Error("the tool did not run")
	}
	assertShape(t, res.FinalMessages, "user", "assistant+tool_calls", "tool:echo", "assistant")
	if !strings.Contains(out.buf.String(), "could not print the echo call") {
		t.Errorf("no warning that the call was not printed:\n%s", out.buf.String())
	}
}

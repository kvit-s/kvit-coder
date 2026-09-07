package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/procs"
)

func procsTestSetup(t *testing.T) (*config.Config, *procs.Registry, *ToolContext, *inbox.Inbox) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Workspace.Root = t.TempDir()
	cfg.Tools.Procs.Enabled = true

	registry, err := procs.New(t.TempDir())
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	box := inbox.New("")
	toolCtx := NewToolContext()
	toolCtx.SetInbox(box)
	return cfg, registry, toolCtx, box
}

func callTool(t *testing.T, tool Tool, args any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := tool.Check(context.Background(), raw); err != nil {
		t.Fatalf("%s Check: %v", tool.Name(), err)
	}
	result, err := tool.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("%s Call: %v", tool.Name(), err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("%s returned %T, want a map", tool.Name(), result)
	}
	return m
}

// TestStartOutputAndKill walks the ordinary path: start something, read what it
// has printed, then stop it.
func TestStartOutputAndKill(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)
	output := NewShellOutputTool(cfg, registry, toolCtx)
	list := NewShellListTool(cfg, registry, toolCtx)
	kill := NewShellKillTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "echo hello; sleep 30", "name": "greeter"})
	id, _ := started["id"].(string)
	if id == "" {
		t.Fatal("Shell.start returned no id")
	}

	deadline := time.Now().Add(10 * time.Second)
	var got map[string]any
	for time.Now().Before(deadline) {
		got = callTool(t, output, map[string]any{"id": id})
		if text, _ := got["output"].(string); text != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if text, _ := got["output"].(string); text == "" {
		t.Fatal("Shell.output never returned the process's output")
	}

	listed := callTool(t, list, map[string]any{})
	if listed["count"].(int) != 1 {
		t.Errorf("Shell.list found %v processes, want 1", listed["count"])
	}

	killed := callTool(t, kill, map[string]any{"id": id})
	if state, _ := killed["state"].(string); state == "running" {
		t.Errorf("after Shell.kill the process is %q, want it stopped", state)
	}
}

// TestObserveWaitReturnsOnExit: waiting for a process that ends returns as soon
// as it does, saying that is why, with its exit status and output.
func TestObserveWaitReturnsOnExit(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)
	wait := NewObserveWaitTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "echo tests failed; exit 2"})
	id := started["id"].(string)

	got := callTool(t, wait, map[string]any{"id": id, "poll": 1, "max_wait": 10})
	if got["returned_for"] != "exit" {
		t.Errorf("returned for %v, want exit", got["returned_for"])
	}
	if got["exit_code"] != 2 {
		t.Errorf("exit code is %v, want 2", got["exit_code"])
	}
	if text, _ := got["output"].(string); text == "" {
		t.Error("the wait returned no output from the process")
	}
	if _, ok := got["waited_secs"]; !ok {
		t.Error("the result does not say how long it waited")
	}
	if _, ok := got["quiet_checks"]; !ok {
		t.Error("the result does not say how many checks produced nothing")
	}
}

// TestObserveWaitReturnsOnMatch: a pattern is what you wait for when the
// process never ends, such as a server announcing that it is listening.
func TestObserveWaitReturnsOnMatch(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)
	wait := NewObserveWaitTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 1; echo Listening on 8080; sleep 60"})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	got := callTool(t, wait, map[string]any{"id": id, "until": "Listening on \\d+", "poll": 1, "max_wait": 15})
	if got["returned_for"] != "match" {
		t.Errorf("returned for %v, want match", got["returned_for"])
	}
	if got["state"] != "running" {
		t.Errorf("the process is %v, want it still running after the match", got["state"])
	}
}

// TestObserveWaitReturnsWhenSomeoneTypes: waiting must not leave a person
// stuck behind a process whose result may no longer matter.
func TestObserveWaitReturnsWhenSomeoneTypes(t *testing.T) {
	cfg, registry, toolCtx, box := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)
	wait := NewObserveWaitTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 60"})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	go func() {
		time.Sleep(100 * time.Millisecond)
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "never mind, do something else"})
	}()

	begin := time.Now()
	got := callTool(t, wait, map[string]any{"id": id, "poll": 30, "max_wait": 300})
	if got["returned_for"] != "steering" {
		t.Errorf("returned for %v, want steering", got["returned_for"])
	}
	if elapsed := time.Since(begin); elapsed > 10*time.Second {
		t.Errorf("the wait took %s, want it to return as soon as someone typed", elapsed)
	}

	// The line is still in the inbox for the loop to present as steering.
	if left := box.Drain(); len(left) != 1 {
		t.Errorf("the inbox holds %d messages, want the typed line left for the loop", len(left))
	}
}

// TestObserveWaitGivesUp: max_wait is honoured and says so, rather than
// blocking a turn forever on something that is never going to happen.
func TestObserveWaitGivesUp(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)
	wait := NewObserveWaitTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 60"})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	got := callTool(t, wait, map[string]any{"id": id, "until": "never happens", "poll": 1, "max_wait": 1})
	if got["returned_for"] != "max_wait" {
		t.Errorf("returned for %v, want max_wait", got["returned_for"])
	}
}

// TestProcessEventsReachTheInbox: an ending is queued as a process event, which
// is how the loop hears about it between iterations.
func TestProcessEventsReachTheInbox(t *testing.T) {
	cfg, registry, toolCtx, box := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "echo done; exit 7", "name": "build"})
	id := started["id"].(string)

	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if info, err := registry.Status(id); err == nil && !info.Running() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	ProcessEvents(registry, box)
	messages := box.Drain()
	if len(messages) != 1 {
		t.Fatalf("the inbox holds %d messages, want the one exit event", len(messages))
	}
	if messages[0].Kind != inbox.KindProcessEvent {
		t.Errorf("the event is kind %q, want a process event", messages[0].Kind)
	}
	if want := "exited with status 7"; !strings.Contains(messages[0].Text, want) {
		t.Errorf("the event is %q, want it to contain %q", messages[0].Text, want)
	}

	// Polling again says nothing: the model has already been told.
	ProcessEvents(registry, box)
	if again := box.Drain(); len(again) != 0 {
		t.Errorf("the same ending was queued twice: %v", again)
	}
}

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

// TestShellStartTakesReminderPolicy: report, until and remind_every set at
// start land on the record and come back in the result, so there is no round
// trip to another tool just to say when to be told.
func TestShellStartTakesReminderPolicy(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{
		"command": "sleep 60", "name": "server",
		"report": "changed", "until": "Listening", "remind_every": 60,
	})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	if started["report"] != "changed" {
		t.Errorf("result report is %v, want changed", started["report"])
	}
	if started["until"] != "Listening" {
		t.Errorf("result until is %v, want the pattern back", started["until"])
	}
	if started["remind_every"] != 60 {
		t.Errorf("result remind_every is %v, want 60", started["remind_every"])
	}

	info, err := registry.Status(id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if info.Report != "changed" || info.Until != "Listening" || info.RemindEvery != 60 {
		t.Errorf("stored policy is %+v, want changed/Listening/60", info)
	}
}

// TestShellStartDefaultsToExitOnly: no policy means nothing while it runs
// and one notice with its last output when it ends.
func TestShellStartDefaultsToExitOnly(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 60"})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	if started["report"] != "exit" {
		t.Errorf("result report is %v, want the exit default", started["report"])
	}
	if _, ok := started["until"]; ok {
		t.Errorf("result carries until %v, want none", started["until"])
	}
	if _, ok := started["remind_every"]; ok {
		t.Errorf("result carries remind_every %v, want none", started["remind_every"])
	}
}

// TestShellStartDefaultsEphemeral: the default stops at turn end, so an
// abandoned session leaves nothing behind.
func TestShellStartDefaultsEphemeral(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 60"})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	if started["persistent"] != false {
		t.Errorf("result persistent is %v, want the ephemeral default", started["persistent"])
	}
	info, _ := registry.Status(id)
	if info.Persistent {
		t.Error("stored process is persistent, want ephemeral by default")
	}
}

// TestShellStartPersistentSurvives: persistent=true is the opt-in exception,
// recorded and echoed back.
func TestShellStartPersistentSurvives(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 60", "persistent": true})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	if started["persistent"] != true {
		t.Errorf("result persistent is %v, want true", started["persistent"])
	}
	info, _ := registry.Status(id)
	if !info.Persistent {
		t.Error("stored process is ephemeral, want persistent")
	}
}

// TestShellStartRejectsBadReminderPolicy: bad values are semantic errors at
// Check time, failing the call before anything starts.
func TestShellStartRejectsBadReminderPolicy(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)

	for name, args := range map[string]any{
		"bad report":  map[string]any{"command": "sleep 1", "report": "loud"},
		"bad pattern": map[string]any{"command": "sleep 1", "until": "(["},
		"bad ticks":   map[string]any{"command": "sleep 1", "remind_every": -5},
	} {
		raw, _ := json.Marshal(args)
		if err := start.Check(context.Background(), raw); err == nil {
			t.Errorf("%s: Check accepted a bad reminder policy", name)
		}
	}
}

// TestShellTuneAdjustsPolicy: after a reminder the model changes its mind
// without restarting the command; only what it passes changes.
func TestShellTuneAdjustsPolicy(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)
	tune := NewShellTuneTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 60"})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	got := callTool(t, tune, map[string]any{"id": id, "until": "ready", "remind_every": 30})
	if got["until"] != "ready" {
		t.Errorf("tuned until is %v, want ready", got["until"])
	}
	if got["remind_every"] != 30 {
		t.Errorf("tuned remind_every is %v, want 30", got["remind_every"])
	}
	if got["report"] != "exit" {
		t.Errorf("tuned report is %v, want the untouched exit default", got["report"])
	}

	// Clearing the pattern leaves the rest alone.
	cleared := callTool(t, tune, map[string]any{"id": id, "until": ""})
	if _, ok := cleared["until"]; ok {
		t.Errorf("cleared until still reported as %v", cleared["until"])
	}
	if cleared["remind_every"] != 30 {
		t.Errorf("clearing the pattern moved remind_every to %v", cleared["remind_every"])
	}

	info, _ := registry.Status(id)
	if info.Until != "" || info.RemindEvery != 30 {
		t.Errorf("stored policy is %+v, want pattern cleared and ticks kept", info)
	}
}

// TestShellTuneRejectsBadCalls: id is required, something tunable must be
// passed, and values are validated like at start.
func TestShellTuneRejectsBadCalls(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	tune := NewShellTuneTool(cfg, registry, toolCtx)

	raw := func(args any) json.RawMessage {
		b, _ := json.Marshal(args)
		return b
	}
	ctx := context.Background()

	if err := tune.Check(ctx, raw(map[string]any{"report": "changed"})); err == nil {
		t.Error("tuning without an id was accepted")
	}
	if err := tune.Check(ctx, raw(map[string]any{"id": "bg1"})); err == nil {
		t.Error("tuning with nothing to change was accepted")
	}
	if err := tune.Check(ctx, raw(map[string]any{"id": "bg1", "report": "loud"})); err == nil {
		t.Error("tuning with an unknown report was accepted")
	}
	if err := tune.Check(ctx, raw(map[string]any{"id": "bg1", "until": "(["})); err == nil {
		t.Error("tuning with an invalid pattern was accepted")
	}
	if err := tune.Check(ctx, raw(map[string]any{"id": "bg1", "remind_every": -1})); err == nil {
		t.Error("tuning with a negative interval was accepted")
	}
	if _, err := tune.Call(ctx, raw(map[string]any{"id": "bg999", "report": "changed"})); err == nil {
		t.Error("tuning a process that does not exist was accepted")
	}
}

// TestShellTunePersistent: the lifetime is tunable without restarting the
// command — promote an ephemeral scratch process, then demote it back.
func TestShellTunePersistent(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)
	tune := NewShellTuneTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 60"})
	id := started["id"].(string)
	defer func() { _ = registry.Kill(id) }()

	promoted := callTool(t, tune, map[string]any{"id": id, "persistent": true})
	if promoted["persistent"] != true {
		t.Errorf("tuned persistent is %v, want true", promoted["persistent"])
	}
	if info, _ := registry.Status(id); !info.Persistent {
		t.Error("stored process is ephemeral after promoting, want persistent")
	}

	demoted := callTool(t, tune, map[string]any{"id": id, "persistent": false})
	if demoted["persistent"] != false {
		t.Errorf("tuned persistent is %v, want false", demoted["persistent"])
	}
	if info, _ := registry.Status(id); info.Persistent {
		t.Error("stored process is persistent after demoting, want ephemeral")
	}
}

// TestInfoResultTimesAreLocal: started/ended are reported in the machine's
// local timezone with its numeric offset, not UTC — a reader in CDT sees
// 10:13 with -05:00, not 15:13Z. The record on disk stays UTC; only the
// report converts, so the parsed instant must equal the stored one.
func TestInfoResultTimesAreLocal(t *testing.T) {
	cfg, registry, toolCtx, _ := procsTestSetup(t)
	start := NewShellStartTool(cfg, registry, toolCtx)
	status := NewShellStatusTool(cfg, registry, toolCtx)
	kill := NewShellKillTool(cfg, registry, toolCtx)

	started := callTool(t, start, map[string]any{"command": "sleep 60"})
	id := started["id"].(string)

	stored, err := registry.Status(id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	checkInstant := func(field, value string, want time.Time) {
		t.Helper()
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			t.Fatalf("%s %q does not parse as RFC3339: %v", field, value, err)
		}
		if parsed.Unix() != want.Unix() {
			t.Errorf("%s parses to %v, want the stored instant %v", field, parsed, want)
		}
		_, wantOffset := want.Local().Zone()
		_, gotOffset := parsed.Zone()
		if gotOffset != wantOffset {
			t.Errorf("%s carries offset %d, want local offset %d (%q)", field, gotOffset, wantOffset, value)
		}
	}
	checkInstant("started", callTool(t, status, map[string]any{"id": id})["started"].(string), stored.Started)

	killed := callTool(t, kill, map[string]any{"id": id})
	if killed["state"] != string(procs.StateKilled) {
		t.Fatalf("Shell.kill left state %v, want killed", killed["state"])
	}
	got := callTool(t, status, map[string]any{"id": id})
	ended, _ := got["ended"].(string)
	if ended == "" {
		t.Fatal("Shell.status reports no ended time for a killed process")
	}
	restored, _ := registry.Status(id)
	checkInstant("ended", ended, restored.Ended)
}

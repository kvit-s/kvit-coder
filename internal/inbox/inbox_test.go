package inbox

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestDrainReturnsAndUnlinks: a file dropped in the directory is delivered
// once and then gone, so the next drain does not deliver it again.
func TestDrainReturnsAndUnlinks(t *testing.T) {
	dir := t.TempDir()
	in := New(dir)

	path, err := Deliver(dir, Message{Kind: KindUserLine, Text: "stop and run the tests"})
	if err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	got := in.Drain()
	if len(got) != 1 {
		t.Fatalf("drained %d messages, want 1", len(got))
	}
	if got[0].Text != "stop and run the tests" || got[0].Kind != KindUserLine {
		t.Errorf("drained %+v, want the delivered user line", got[0])
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("the inbox file is still there after being drained")
	}
	if again := in.Drain(); len(again) != 0 {
		t.Errorf("a second drain returned %d messages, want none", len(again))
	}
}

// TestDrainOrder: messages come out in the order they arrived, whether they
// were pushed in this process or dropped as files.
func TestDrainOrder(t *testing.T) {
	dir := t.TempDir()
	in := New(dir)

	in.Push(Message{Kind: KindUserLine, Text: "pushed first"})
	for i, text := range []string{"file one", "file two", "file three"} {
		if _, err := Deliver(dir, Message{
			Kind: KindUserLine,
			Text: text,
			At:   time.Unix(0, int64(i+1)*1_000_000),
		}); err != nil {
			t.Fatalf("Deliver: %v", err)
		}
	}

	got := in.Drain()
	want := []string{"pushed first", "file one", "file two", "file three"}
	if len(got) != len(want) {
		t.Fatalf("drained %d messages, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i].Text != want[i] {
			t.Errorf("message %d is %q, want %q", i, got[i].Text, want[i])
		}
	}
}

// TestOversizedFileSkipped: an inbox file too large to be a steering message
// is skipped and reported rather than pushed at the model.
func TestOversizedFileSkipped(t *testing.T) {
	dir := t.TempDir()
	in := New(dir)
	var logged []string
	in.Log = func(msg string) { logged = append(logged, msg) }

	big := filepath.Join(dir, "1-oversized")
	if err := os.WriteFile(big, []byte(strings.Repeat("x", maxFileSize+1)), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := Deliver(dir, Message{Kind: KindUserLine, Text: "small one"}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}

	got := in.Drain()
	if len(got) != 1 || got[0].Text != "small one" {
		t.Fatalf("drained %+v, want only the small message", got)
	}
	if len(logged) != 1 || !strings.Contains(logged[0], "over the") {
		t.Errorf("the oversized file was not reported: %v", logged)
	}
	if _, err := os.Stat(big); !os.IsNotExist(err) {
		t.Error("the oversized file was left in the inbox, so it would be re-read every drain")
	}
}

// TestPlainTextFile: a file written by hand, with no JSON envelope, is taken
// as a line of steering.
func TestPlainTextFile(t *testing.T) {
	dir := t.TempDir()
	in := New(dir)

	if err := os.WriteFile(filepath.Join(dir, "1-handwritten"), []byte("  use the other library\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	got := in.Drain()
	if len(got) != 1 {
		t.Fatalf("drained %d messages, want 1", len(got))
	}
	if got[0].Kind != KindUserLine || got[0].Text != "use the other library" {
		t.Errorf("drained %+v, want a trimmed user line", got[0])
	}
}

// TestDeliverIsAtomic: the file a drain can see is always complete, because
// Deliver writes under a name a drain skips and renames into place.
func TestDeliverIsAtomic(t *testing.T) {
	dir := t.TempDir()
	if _, err := Deliver(dir, Message{Kind: KindUserLine, Text: "complete"}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("the inbox holds %d files, want 1", len(entries))
	}
	if strings.HasPrefix(entries[0].Name(), ".") {
		t.Errorf("the delivered file is still under its temporary name %q", entries[0].Name())
	}
	data, _ := os.ReadFile(filepath.Join(dir, entries[0].Name()))
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		t.Errorf("the delivered file is not a complete message: %v", err)
	}
}

// TestQueueIsBounded: the in-memory queue drops the oldest rather than growing
// without limit, so someone holding a key down cannot exhaust memory.
func TestQueueIsBounded(t *testing.T) {
	in := New("")
	for i := range maxQueued + 50 {
		in.Push(Message{Kind: KindUserLine, Text: string(rune('a' + i%26))})
	}

	got := in.Drain()
	if len(got) != maxQueued {
		t.Errorf("the queue holds %d messages, want it capped at %d", len(got), maxQueued)
	}
}

// TestPauseRequestTakeConsume: an empty Enter's request waits until the loop
// takes it at its next iteration boundary, and taking clears it.
func TestPauseRequestTakeConsume(t *testing.T) {
	in := New("")
	if in.TakePause() {
		t.Fatal("TakePause is true before anything was requested")
	}
	if !in.RequestPause() {
		t.Fatal("the first RequestPause is not new")
	}
	if in.RequestPause() {
		t.Error("a second RequestPause is new, want it a no-op")
	}
	if !in.TakePause() {
		t.Fatal("TakePause missed the pending request")
	}
	if in.TakePause() {
		t.Error("TakePause is still true after taking, want it cleared")
	}
}

// TestPauseModeToggle: pause mode is only up while the pause prompt waits.
func TestPauseModeToggle(t *testing.T) {
	in := New("")
	if in.PauseMode() {
		t.Fatal("PauseMode is true on a fresh inbox")
	}
	in.SetPauseMode(true)
	if !in.PauseMode() {
		t.Fatal("PauseMode is false after setting it")
	}
	in.SetPauseMode(false)
	if in.PauseMode() {
		t.Error("PauseMode is still true after clearing it")
	}
}

// TestPauseRequestedPeek: the progress watcher peeks at a pending pause
// without consuming it — TakePause still sees it afterwards.
func TestPauseRequestedPeek(t *testing.T) {
	in := New("")
	if in.PauseRequested() {
		t.Fatal("PauseRequested is true before anything was requested")
	}
	if !in.RequestPause() {
		t.Fatal("the first RequestPause is not new")
	}
	if !in.PauseRequested() {
		t.Fatal("PauseRequested missed the pending request")
	}
	// Peeking twice must not consume: the loop still takes it at the boundary.
	if !in.PauseRequested() {
		t.Fatal("peeking consumed the request")
	}
	if !in.TakePause() {
		t.Fatal("TakePause missed the request after peeking")
	}
	if in.PauseRequested() {
		t.Error("PauseRequested is still true after taking, want it cleared")
	}
}

// TestRequestPauseSignals: a tool blocked in Observe.wait selects on the
// signal, so a pause request must wake it — otherwise the pause waits out
// the tool's full max_wait. Ask waiters just loop, since no line was pushed.
func TestRequestPauseSignals(t *testing.T) {
	in := New("")
	select {
	case <-in.Signal():
		t.Fatal("the signal fired before anything was requested")
	default:
	}
	if !in.RequestPause() {
		t.Fatal("the first RequestPause is not new")
	}
	select {
	case <-in.Signal():
	case <-time.After(time.Second):
		t.Fatal("the signal did not fire after a pause request")
	}
	// A repeat request is a no-op and must not queue another signal; drain
	// the one above first so the check means something.
	if in.RequestPause() {
		t.Fatal("a second RequestPause is new, want it a no-op")
	}
}

// TestSignalFires: a blocking tool waits on Signal and wakes when something is
// pushed.
func TestSignalFires(t *testing.T) {
	in := New("")
	select {
	case <-in.Signal():
		t.Fatal("the signal fired before anything was pushed")
	default:
	}

	in.Push(Message{Kind: KindUserLine, Text: "wake up"})
	select {
	case <-in.Signal():
	case <-time.After(time.Second):
		t.Fatal("the signal did not fire after a push")
	}
}

// TestAskClaimsTheNextLine: a prompt takes the line typed after it, and that
// line does not also reach the loop as steering. This is the bug that made a
// path confirmation hang: the terminal's only reader queued the "y" as
// steering while the prompt waited for a keystroke that had already been read.
func TestAskClaimsTheNextLine(t *testing.T) {
	in := New("")
	var out strings.Builder

	go func() {
		time.Sleep(50 * time.Millisecond)
		in.Push(Message{Kind: KindUserLine, Text: "y"})
	}()

	answer, outcome := in.Ask(context.Background(), &out, "Allow this access? [y/N]: ", 0)
	if outcome != AskAnswered {
		t.Fatalf("outcome is %v, want answered", outcome)
	}
	if answer != "y" {
		t.Errorf("answer is %q, want %q", answer, "y")
	}
	if !strings.Contains(out.String(), "Allow this access?") {
		t.Errorf("the prompt was not drawn: %q", out.String())
	}
	if left := in.Drain(); len(left) != 0 {
		t.Errorf("the answer also reached the loop as steering: %+v", left)
	}
}

// TestAskDoesNotTakeAnEarlierLine: something typed before the prompt appeared
// was not answering it, so it stays for the loop and the prompt keeps waiting.
func TestAskDoesNotTakeAnEarlierLine(t *testing.T) {
	in := New("")
	in.Push(Message{Kind: KindUserLine, Text: "check the tests too"})

	go func() {
		time.Sleep(80 * time.Millisecond)
		in.Push(Message{Kind: KindUserLine, Text: "y"})
	}()

	answer, outcome := in.Ask(context.Background(), nil, "", 0)
	if outcome != AskAnswered || answer != "y" {
		t.Fatalf("Ask returned (%q, %v), want the line typed after the prompt", answer, outcome)
	}

	left := in.Drain()
	if len(left) != 1 || left[0].Text != "check the tests too" {
		t.Errorf("the inbox holds %+v, want the earlier line left for the loop", left)
	}
}

// TestAskLeavesProcessEventsForTheLoop: an event is not an answer.
func TestAskLeavesProcessEventsForTheLoop(t *testing.T) {
	in := New("")

	go func() {
		time.Sleep(30 * time.Millisecond)
		in.Push(Message{Kind: KindProcessEvent, Text: "bg1 exited 0"})
		time.Sleep(30 * time.Millisecond)
		in.Push(Message{Kind: KindUserLine, Text: "1"})
	}()

	answer, outcome := in.Ask(context.Background(), nil, "", 0)
	if outcome != AskAnswered || answer != "1" {
		t.Fatalf("Ask returned (%q, %v), want the typed line", answer, outcome)
	}
	left := in.Drain()
	if len(left) != 1 || left[0].Kind != KindProcessEvent {
		t.Errorf("the inbox holds %+v, want the process event left for the loop", left)
	}
}

// TestAskCancelsAndTimesOut: the two ways a prompt ends without an answer.
func TestAskCancelsAndTimesOut(t *testing.T) {
	in := New("")

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	if _, outcome := in.Ask(ctx, nil, "", 0); outcome != AskCancelled {
		t.Errorf("outcome after cancelling is %v, want cancelled", outcome)
	}

	if _, outcome := in.Ask(context.Background(), nil, "", 100*time.Millisecond); outcome != AskTimedOut {
		t.Errorf("outcome after the timeout is %v, want timed out", outcome)
	}
}

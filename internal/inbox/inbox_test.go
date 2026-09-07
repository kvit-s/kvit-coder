package inbox

import (
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

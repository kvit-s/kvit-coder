package agent

import (
	"testing"

	"github.com/kvit-s/kvit-coder/internal/procs"
)

// TestRunningProcsCount counts only live background processes: nil or empty
// means no star, running ones count, killed ones drop off.
func TestRunningProcsCount(t *testing.T) {
	client := newFakeClient(answer("done"))
	runner, _ := newTestRunner(t, testConfig(), client)
	if n := runner.runningProcsCount(); n != 0 {
		t.Fatalf("nil registry = %d, want 0", n)
	}

	dir := t.TempDir()
	reg, err := procs.New(dir)
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	runner.procs = reg
	if n := runner.runningProcsCount(); n != 0 {
		t.Fatalf("empty registry = %d, want 0", n)
	}

	id1, err := reg.Start("sleep 30", dir, "sleeper one")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { reg.KillAll() })
	if n := runner.runningProcsCount(); n != 1 {
		t.Fatalf("one running = %d, want 1", n)
	}

	if _, err := reg.Start("sleep 30", dir, "sleeper two"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if n := runner.runningProcsCount(); n != 2 {
		t.Fatalf("two running = %d, want 2", n)
	}

	if err := reg.Kill(id1); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if n := runner.runningProcsCount(); n != 1 {
		t.Fatalf("one killed = %d, want 1", n)
	}
}

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

// TestRunningProcsCountsSplitsLifetimes: the step header paints ephemeral
// blue and persistent red, so it needs the two counts separately.
func TestRunningProcsCountsSplitsLifetimes(t *testing.T) {
	client := newFakeClient(answer("done"))
	runner, _ := newTestRunner(t, testConfig(), client)
	if e, p := runner.runningProcsCounts(); e != 0 || p != 0 {
		t.Fatalf("nil registry = %d+%d, want 0+0", e, p)
	}

	dir := t.TempDir()
	reg, err := procs.New(dir)
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	runner.procs = reg
	t.Cleanup(func() { reg.KillAll() })

	if _, err := reg.Start("sleep 30", dir, "scratch"); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if _, err := reg.StartWithOptions("sleep 30", dir, "server", "exit", "", 0, true); err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	if e, p := runner.runningProcsCounts(); e != 1 || p != 1 {
		t.Fatalf("counts = %d+%d, want 1+1", e, p)
	}
	if n := runner.runningProcsCount(); n != 2 {
		t.Fatalf("total = %d, want 2", n)
	}
}

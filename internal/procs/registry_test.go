package procs

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// TestSurvivesTheProcessThatStartedIt: the whole point of the registry. A
// second registry opened over the same directory — which is what the next
// turn's process has — finds the process still running, and can read its
// output and kill it.
func TestSurvivesTheProcessThatStartedIt(t *testing.T) {
	dir := t.TempDir()

	first, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	id, err := first.Start("echo started; sleep 30", t.TempDir(), "long sleeper")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	// A new registry over the same directory, as the next turn has.
	second, err := New(dir)
	if err != nil {
		t.Fatalf("New (second): %v", err)
	}

	waitFor(t, "the process to write its first line", func() bool {
		_, _, ok, _ := second.Output(id, 0)
		return ok
	})

	info, err := second.Status(id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if !info.Running() {
		t.Fatalf("the process is %s, want it still running", info.State)
	}
	if info.Name != "long sleeper" {
		t.Errorf("the name did not survive: %q", info.Name)
	}
	if changed := second.Reconcile(); len(changed) != 0 {
		t.Errorf("Reconcile reported %d changes for a process that is still running", len(changed))
	}

	text, cursor, ok, err := second.Output(id, 0)
	if err != nil || !ok || !strings.Contains(text, "started") {
		t.Fatalf("Output returned (%q, %d, %v, %v), want the process's first line", text, cursor, ok, err)
	}
	// Reading again from the cursor shows only what is new, which is nothing.
	if _, _, more, _ := second.Output(id, cursor); more {
		t.Error("reading from the cursor returned output that was already read")
	}

	pid := info.PID
	if err := second.Kill(id); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	waitFor(t, "the process to be gone", func() bool { return syscall.Kill(pid, 0) != nil })

	after, _ := second.Status(id)
	if after.Running() {
		t.Errorf("after Kill the process is %s, want it stopped", after.State)
	}
}

// TestExitStatusOutlivesTheStarter: the exit status is written by the command's
// own wrapper, so a later turn can read it even though the process that
// started it is long gone and there is no one left to wait() for it.
func TestExitStatusOutlivesTheStarter(t *testing.T) {
	dir := t.TempDir()
	first, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	id, err := first.Start("echo working; exit 3", t.TempDir(), "failing job")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	second, _ := New(dir)
	waitFor(t, "the exit status to be recorded", func() bool {
		info, err := second.Status(id)
		return err == nil && !info.Running()
	})

	info, _ := second.Status(id)
	if info.State != StateExited {
		t.Errorf("state is %s, want exited", info.State)
	}
	if info.ExitCode != 3 {
		t.Errorf("exit code is %d, want 3", info.ExitCode)
	}
}

// TestReconcileFindsAProcessKilledFromOutside: something that died without
// recording an exit status is not left looking alive forever.
func TestReconcileFindsAProcessKilledFromOutside(t *testing.T) {
	dir := t.TempDir()
	r, err := New(dir)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	id, err := r.Start("sleep 60", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	info, _ := r.Status(id)

	// Kill the group behind the registry's back, and remove the exit file the
	// wrapper leaves, so the only evidence is that the pid has gone.
	_ = syscall.Kill(-info.PID, syscall.SIGKILL)
	waitFor(t, "the process to die", func() bool { return syscall.Kill(info.PID, 0) != nil })
	_ = os.Remove(filepath.Join(dir, id+".exit"))

	changed := r.Reconcile()
	if len(changed) != 1 || changed[0].ID != id {
		t.Fatalf("Reconcile reported %+v, want the one process that died", changed)
	}
	if changed[0].State != StateGone {
		t.Errorf("state is %s, want gone", changed[0].State)
	}
}

// TestEventsReportEachEndingOnce: the model is told a process ended, with its
// last output, and is not told again on the next poll.
func TestEventsReportEachEndingOnce(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.Start("echo all done; exit 1", t.TempDir(), "build")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}

	waitFor(t, "the process to finish", func() bool {
		info, err := r.Status(id)
		return err == nil && !info.Running()
	})

	events := r.Events()
	if len(events) != 1 {
		t.Fatalf("got %d events, want 1: %v", len(events), events)
	}
	if !strings.Contains(events[0], "exited with status 1") {
		t.Errorf("the event does not say how it ended: %q", events[0])
	}
	if !strings.Contains(events[0], "build") {
		t.Errorf("the event does not name the process: %q", events[0])
	}
	if !strings.Contains(events[0], "all done") {
		t.Errorf("the event does not carry the last output: %q", events[0])
	}

	if again := r.Events(); len(again) != 0 {
		t.Errorf("the same ending was reported twice: %v", again)
	}
}

// TestQuietProcessRaisesNothing: a running process nobody asked to be
// interrupted about says nothing, which is what makes watching cost nothing.
func TestQuietProcessRaisesNothing(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.Start("echo noisy; sleep 30", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	waitFor(t, "the process to print something", func() bool {
		_, _, ok, _ := r.Output(id, 0)
		return ok
	})

	if events := r.Events(); len(events) != 0 {
		t.Errorf("a running process with report=exit raised %v, want nothing", events)
	}
}

// TestPeriodicProbeReportsItsOutput: a probe added with report=always is what
// the model hears from between iterations.
func TestPeriodicProbeReportsItsOutput(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.StartPeriodic("echo tick", t.TempDir(), "ticker", 1, "always")
	if err != nil {
		t.Fatalf("StartPeriodic: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	var events []string
	waitFor(t, "the probe to report", func() bool {
		events = r.Events()
		return len(events) > 0
	})
	if !strings.Contains(events[0], "tick") {
		t.Errorf("the probe's event does not carry its output: %q", events[0])
	}

	info, _ := r.Status(id)
	if info.Every != 1 {
		t.Errorf("the probe's interval was not recorded: %+v", info)
	}
	if !info.Running() {
		t.Errorf("the probe is %s, want it still running", info.State)
	}
}

// TestListIsOrderedAndComplete: every process this session started is listed,
// oldest first, whatever state it is in.
func TestListIsOrderedAndComplete(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	workdir := t.TempDir()

	firstID, _ := r.Start("exit 0", workdir, "first")
	time.Sleep(20 * time.Millisecond)
	secondID, _ := r.Start("sleep 30", workdir, "second")
	defer func() { _ = r.Kill(secondID) }()

	list := r.List()
	if len(list) != 2 {
		t.Fatalf("List returned %d processes, want 2", len(list))
	}
	if list[0].ID != firstID || list[1].ID != secondID {
		t.Errorf("List returned %s then %s, want oldest first (%s, %s)",
			list[0].ID, list[1].ID, firstID, secondID)
	}
}

// TestKillAllStopsEverything: what the interrupt handler relies on.
func TestKillAllStopsEverything(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	workdir := t.TempDir()

	var pids []int
	for range 3 {
		id, err := r.Start("sleep 60", workdir, "")
		if err != nil {
			t.Fatalf("Start: %v", err)
		}
		info, _ := r.Status(id)
		pids = append(pids, info.PID)
	}

	killed := r.KillAll()
	if len(killed) != 3 {
		t.Fatalf("KillAll stopped %d processes, want 3", len(killed))
	}
	for _, pid := range pids {
		waitFor(t, "every process to be gone", func() bool { return syscall.Kill(pid, 0) != nil })
	}
	for _, info := range r.List() {
		if info.Running() {
			t.Errorf("%s is still running after KillAll", info.ID)
		}
	}
}

// TestOutputKeepsTheTail: a process that writes more than fits in one message
// gives back the end of it, which is what says what is happening now.
func TestOutputKeepsTheTail(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.Start("for i in $(seq 1 20000); do echo line $i; done", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	waitFor(t, "the process to finish", func() bool {
		info, err := r.Status(id)
		return err == nil && !info.Running()
	})

	text, _, ok, err := r.Output(id, 0)
	if err != nil || !ok {
		t.Fatalf("Output: %v", err)
	}
	if len(text) > maxChunk+200 {
		t.Errorf("Output returned %d bytes, want it capped near %d", len(text), maxChunk)
	}
	if !strings.Contains(text, "line 20000") {
		t.Error("Output dropped the end of the log, which is the part that matters")
	}
	if !strings.Contains(text, "earlier bytes skipped") {
		t.Error("Output did not say that it had skipped anything")
	}
}

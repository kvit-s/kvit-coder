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

// TestStartWithOptionsStoresPolicy: the reminder policy given at start is
// recorded and survives a reopen, which is what the next turn reads.
func TestStartWithOptionsStoresPolicy(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.StartWithOptions("sleep 30", t.TempDir(), "server", "changed", "Listening", 60, false)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	info, err := r.Status(id)
	if err != nil {
		t.Fatalf("Status: %v", err)
	}
	if info.Report != "changed" || info.Until != "Listening" || info.RemindEvery != 60 {
		t.Fatalf("policy is %+v, want changed/Listening/60", info)
	}

	second, _ := New(dir)
	again, err := second.Status(id)
	if err != nil {
		t.Fatalf("Status after reopen: %v", err)
	}
	if again.Report != "changed" || again.Until != "Listening" || again.RemindEvery != 60 {
		t.Errorf("policy did not survive a reopen: %+v", again)
	}
}

// TestStartWithOptionsRejectsBadPolicy: bad values fail at start, not at
// the first poll.
func TestStartWithOptionsRejectsBadPolicy(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	workdir := t.TempDir()

	if _, err := r.StartWithOptions("sleep 1", workdir, "", "loud", "", 0, false); err == nil {
		t.Error("an unknown report was accepted")
	}
	if _, err := r.StartWithOptions("sleep 1", workdir, "", "exit", "([", 0, false); err == nil {
		t.Error("an invalid until pattern was accepted")
	}
	if _, err := r.StartWithOptions("sleep 1", workdir, "", "exit", "", -5, false); err == nil {
		t.Error("a negative remind_every was accepted")
	}
	if got := len(r.List()); got != 0 {
		t.Errorf("List returned %d processes, want none started", got)
	}
}

// TestConfigureRetunesPolicy: after a reminder the policy can change without
// restarting the command, and a changed tick interval restarts its clock.
func TestConfigureRetunesPolicy(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.Start("sleep 30", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	updated, err := r.Configure(id, "changed", "ready", 60, false)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if updated.Report != "changed" || updated.Until != "ready" || updated.RemindEvery != 60 {
		t.Fatalf("policy is %+v, want changed/ready/60", updated)
	}
	if updated.LastEventAt.IsZero() {
		t.Error("tuning the tick interval did not restart its clock")
	}

	// Clearing the pattern and disabling ticks leaves output reminders.
	updated, err = r.Configure(id, "changed", "", 0, false)
	if err != nil {
		t.Fatalf("Configure: %v", err)
	}
	if updated.Until != "" || updated.RemindEvery != 0 || updated.Report != "changed" {
		t.Errorf("policy is %+v, want the pattern cleared and ticks off", updated)
	}

	if _, err := r.Configure(id, "loud", "", 0, false); err == nil {
		t.Error("an unknown report was accepted")
	}
	if _, err := r.Configure(id, "exit", "([", 0, false); err == nil {
		t.Error("an invalid until pattern was accepted")
	}
	if _, err := r.Configure("bg999", "exit", "", 0, false); err == nil {
		t.Error("configuring a process that does not exist was accepted")
	}
}

// TestEventsUntilMatch: a pattern set at start fires on the new output that
// holds it, even with report=exit, and fires once.
func TestEventsUntilMatch(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.StartWithOptions("sleep 1; echo READY on 8080; sleep 30", t.TempDir(), "server", "exit", "READY on \\d+", 0, false)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	var events []string
	waitFor(t, "the pattern to be reported", func() bool {
		events = r.Events()
		return len(events) > 0
	})
	if !strings.Contains(events[0], "matched") || !strings.Contains(events[0], "READY on 8080") {
		t.Errorf("the event does not report the match: %q", events[0])
	}
	if again := r.Events(); len(again) != 0 {
		t.Errorf("the same match was reported twice: %v", again)
	}
}

// TestEventsUntilIgnoresNonMatch: output that does not hold the pattern
// stays quiet under report=exit.
func TestEventsUntilIgnoresNonMatch(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.StartWithOptions("echo something else; sleep 30", t.TempDir(), "", "exit", "NEVER-PRINTED", 0, false)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	waitFor(t, "the process to print something", func() bool {
		_, _, ok, _ := r.Output(id, 0)
		return ok
	})
	if events := r.Events(); len(events) != 0 {
		t.Errorf("non-matching output raised %v, want nothing", events)
	}
}

// TestEventsReportChangedOnOrdinaryProc: report=changed is not only for
// periodic probes — an ordinary command set that way reports its output too.
func TestEventsReportChangedOnOrdinaryProc(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.StartWithOptions("echo hello there; sleep 30", t.TempDir(), "", "changed", "", 0, false)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	var events []string
	waitFor(t, "the output to be reported", func() bool {
		events = r.Events()
		return len(events) > 0
	})
	if !strings.Contains(events[0], "hello there") {
		t.Errorf("the event does not carry the new output: %q", events[0])
	}
	if again := r.Events(); len(again) != 0 {
		t.Errorf("the same output was reported twice: %v", again)
	}
}

// TestEventsPeriodicTickQuiet: a quiet command with a tick interval still
// says it is alive, once per interval and not twice.
func TestEventsPeriodicTickQuiet(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.StartWithOptions("sleep 30", t.TempDir(), "quiet job", "exit", "", 1, false)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	if events := r.Events(); len(events) != 0 {
		t.Fatalf("a fresh tick interval raised %v, want nothing until it elapses", events)
	}
	var events []string
	waitFor(t, "the tick to fire", func() bool {
		events = r.Events()
		return len(events) > 0
	})
	if !strings.Contains(events[0], "still running") || !strings.Contains(events[0], "no new output") {
		t.Errorf("the tick does not say the process is alive and quiet: %q", events[0])
	}
	if again := r.Events(); len(again) != 0 {
		t.Errorf("the same tick fired twice without its interval passing: %v", again)
	}
}

// TestEventsPeriodicCarriesNewOutput: a tick carries what accumulated since
// the last reminder and advances past it, so the ending does not repeat it.
func TestEventsPeriodicCarriesNewOutput(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.StartWithOptions("echo first words; sleep 30", t.TempDir(), "", "exit", "", 1, false)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	var events []string
	waitFor(t, "the tick to carry the output", func() bool {
		events = r.Events()
		return len(events) > 0
	})
	if !strings.Contains(events[0], "still running") || !strings.Contains(events[0], "first words") {
		t.Fatalf("the tick does not carry the accumulated output: %q", events[0])
	}
	info, _ := r.Status(id)
	if info.ReportedOffset == 0 {
		t.Error("the tick did not advance past the output it carried")
	}
}

// TestStartDefaultsEphemeral: the plain Start is the leak-proof default —
// stopped at turn end, surviving only iterations within the turn.
func TestStartDefaultsEphemeral(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.Start("sleep 30", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	info, _ := r.Status(id)
	if info.Persistent {
		t.Error("plain Start is persistent, want ephemeral by default")
	}
}

// TestStartWithOptionsPersistent: persistent=true is recorded and survives a
// reopen, which is what the next turn reads.
func TestStartWithOptionsPersistent(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	id, err := r.StartWithOptions("sleep 30", t.TempDir(), "server", "exit", "", 0, true)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = r.Kill(id) }()

	info, _ := r.Status(id)
	if !info.Persistent {
		t.Fatalf("policy is %+v, want persistent", info)
	}
	second, _ := New(dir)
	again, _ := second.Status(id)
	if !again.Persistent {
		t.Errorf("persistent did not survive a reopen: %+v", again)
	}
}

// TestKillEphemeralStopsOnlyEphemeral: the turn-end cleanup stops the
// default processes and leaves the opt-in survivors for the next turn.
func TestKillEphemeralStopsOnlyEphemeral(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	workdir := t.TempDir()

	ephem, err := r.Start("sleep 30", workdir, "scratch")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	keep, err := r.StartWithOptions("sleep 30", workdir, "server", "exit", "", 0, true)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = r.Kill(keep) }()

	killed := r.KillEphemeral()
	if len(killed) != 1 || killed[0] != ephem {
		t.Fatalf("KillEphemeral stopped %v, want [%s]", killed, ephem)
	}
	if again := r.KillEphemeral(); len(again) != 0 {
		t.Errorf("a second KillEphemeral stopped %v, want nothing left to stop", again)
	}
	if info, _ := r.Status(keep); !info.Running() {
		t.Errorf("the persistent process is %s, want it still running", info.State)
	}
	if info, _ := r.Status(ephem); info.Running() {
		t.Errorf("the ephemeral process is still running after KillEphemeral")
	}
}

// TestRunningCountsSplitsLifetimes: the header needs blue and red separately.
func TestRunningCountsSplitsLifetimes(t *testing.T) {
	dir := t.TempDir()
	r, _ := New(dir)
	workdir := t.TempDir()

	if e, p := r.RunningCounts(); e != 0 || p != 0 {
		t.Fatalf("empty registry = %d ephemeral + %d persistent, want 0+0", e, p)
	}
	ephem, _ := r.Start("sleep 30", workdir, "")
	keep, _ := r.StartWithOptions("sleep 30", workdir, "", "exit", "", 0, true)
	defer func() { _ = r.Kill(keep) }()
	defer func() { _ = r.Kill(ephem) }()

	if e, p := r.RunningCounts(); e != 1 || p != 1 {
		t.Errorf("counts = %d ephemeral + %d persistent, want 1+1", e, p)
	}
	_ = r.Kill(ephem)
	if e, p := r.RunningCounts(); e != 0 || p != 1 {
		t.Errorf("after killing the ephemeral, counts = %d+%d, want 0+1", e, p)
	}
}

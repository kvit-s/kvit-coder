package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/procs"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// testWriter captures the progress stream, the way newTestRunner does for the
// agent tests: headless routes everything through the writer's own streams.
func testWriter(out *bytes.Buffer) *ui.Writer {
	w := ui.NewWriter(0)
	w.SetHeadless(true)
	w.SetStderr(out)
	return w
}

// TestRouteEmptyRequestsPauseOnce: an empty Enter asks for a pause and prints
// its notice; a second one while the request is pending is a silent no-op.
func TestRouteEmptyRequestsPauseOnce(t *testing.T) {
	box := inbox.New("")
	var out bytes.Buffer
	w := testWriter(&out)

	routeStdinLine("   ", box, w)
	routeStdinLine("", box, w)
	if got := strings.Count(out.String(), "pause requested"); got != 1 {
		t.Errorf("the pause notice printed %d times, want once", got)
	}
	// The second Enter was a no-op, and the first request is still pending.
	if !box.TakePause() {
		t.Error("the empty Enter did not request a pause")
	}
	if box.TakePause() {
		t.Error("taking the request did not clear it")
	}
}

// TestRouteEmptyResumesInPauseMode: while the pause prompt is up, empty is a
// valid answer that Ask claims as the resume.
func TestRouteEmptyResumesInPauseMode(t *testing.T) {
	box := inbox.New("")
	var out bytes.Buffer
	w := testWriter(&out)

	box.SetPauseMode(true)
	routeStdinLine("", box, w)

	got := box.Drain()
	if len(got) != 1 || got[0].Kind != inbox.KindUserLine {
		t.Fatalf("drained %+v, want the empty resume line", got)
	}
}

// TestRouteEmptyIgnoredWhileQuestionPending: a Question owns the next line,
// so empty stays ignored there — otherwise "" would become a free-text answer,
// which is impossible today — and no pause is requested.
func TestRouteEmptyIgnoredWhileQuestionPending(t *testing.T) {
	box := inbox.New("")
	var out bytes.Buffer
	w := testWriter(&out)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		box.Ask(ctx, nil, "Allow this access? [y/N]: ", 0)
	}()
	for range 200 {
		if box.Awaiting() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !box.Awaiting() {
		t.Fatal("the question never started waiting")
	}

	routeStdinLine("", box, w)
	cancel()
	<-done

	if box.TakePause() {
		t.Error("an empty Enter during a question requested a pause")
	}
	if left := box.Drain(); len(left) != 0 {
		t.Errorf("the ignored line leaked into the inbox: %+v", left)
	}
	if strings.Contains(out.String(), "pause requested") {
		t.Errorf("a pause was announced during a question: %q", out.String())
	}
}

// TestRouteAnswerWhilePausedIsNotQueued: a steering line typed at the pause
// prompt is consumed by Ask, so it must not also report "queued".
func TestRouteAnswerWhilePausedIsNotQueued(t *testing.T) {
	box := inbox.New("")
	var out bytes.Buffer
	w := testWriter(&out)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		text    string
		outcome inbox.AskOutcome
	}
	answered := make(chan result, 1)
	go func() {
		text, outcome := box.Ask(ctx, nil, "", 0)
		answered <- result{text, outcome}
	}()
	for range 200 {
		if box.Awaiting() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !box.Awaiting() {
		t.Fatal("the pause prompt never started waiting")
	}
	box.SetPauseMode(true)

	routeStdinLine("use the other library", box, w)

	select {
	case r := <-answered:
		if r.outcome != inbox.AskAnswered || r.text != "use the other library" {
			t.Fatalf("Ask returned (%q, %v), want the typed steering", r.text, r.outcome)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the pause prompt did not take the typed line")
	}
	if strings.Contains(out.String(), "queued") {
		t.Errorf("the pause answer was reported as queued: %q", out.String())
	}
	if left := box.Drain(); len(left) != 0 {
		t.Errorf("the answer also reached the loop as steering: %+v", left)
	}
}

// TestRouteNonEmptyStillQueues: the ordinary path is unchanged — a line typed
// mid-turn is steering and says so.
func TestRouteNonEmptyStillQueues(t *testing.T) {
	box := inbox.New("")
	var out bytes.Buffer
	w := testWriter(&out)

	routeStdinLine("check the tests first", box, w)

	got := box.Drain()
	if len(got) != 1 || got[0].Text != "check the tests first" {
		t.Fatalf("drained %+v, want the typed steering", got)
	}
	if !strings.Contains(out.String(), "queued") {
		t.Errorf("the steering line was not acknowledged: %q", out.String())
	}
}

// TestPauseAwareWatcherHoldsFromEnterToPrompt: dots must stay off from the
// empty Enter (pause requested, current call still finishing, type-ahead on
// screen) through the pause prompt itself (pause mode + Ask's counter), so
// nothing redraws the line being typed. Each state alone must hold.
func TestPauseAwareWatcherHoldsFromEnterToPrompt(t *testing.T) {
	box := inbox.New("")
	watch := pauseAwareWatcher(box)
	if watch() {
		t.Fatal("the watcher holds before anything was requested")
	}

	if !box.RequestPause() {
		t.Fatal("the first RequestPause is not new")
	}
	if !watch() {
		t.Error("the watcher does not hold while a pause is pending")
	}

	// The boundary raises the mode before consuming the request, so the hold
	// never lapses in handover: requested, then mode up, then taken.
	box.SetPauseMode(true)
	if !watch() {
		t.Error("the watcher released between mode-up and take")
	}
	if !box.TakePause() {
		t.Fatal("TakePause missed the pending request")
	}
	if !watch() {
		t.Error("the watcher does not hold while the pause prompt is up")
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		box.Ask(ctx, nil, "", 0)
	}()
	for range 200 {
		if box.Awaiting() {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !box.Awaiting() {
		t.Fatal("the pause prompt never started waiting")
	}
	if !watch() {
		t.Error("the watcher does not hold while the prompt waits")
	}
	cancel()
	<-done
	box.SetPauseMode(false)
	if watch() {
		t.Error("the watcher still holds after the pause resolved")
	}
}

// backgroundTestManager roots a session manager at a temp HOME, so the
// reaper commands run against throwaway sessions instead of the user's own.
func backgroundTestManager(t *testing.T) *session.Manager {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	mgr, err := session.NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

// backgroundTestSession opens a session with history (so ListSessions sees
// it) and a running process in it.
func backgroundTestSession(t *testing.T, mgr *session.Manager, name, command string, persistent bool) string {
	t.Helper()
	sess, err := mgr.Open(name)
	if err != nil {
		t.Fatalf("Open %s: %v", name, err)
	}
	// ListSessions only lists sessions with history; a bare Open is not
	// enough for the "all" target to find it.
	if err := sess.Notice("setup"); err != nil {
		t.Fatalf("Notice: %v", err)
	}
	reg, err := procRegistryFor(mgr, name)
	if err != nil {
		t.Fatalf("procRegistryFor: %v", err)
	}
	var id string
	if persistent {
		id, err = reg.StartWithOptions(command, t.TempDir(), "", "exit", "", 0, true)
	} else {
		id, err = reg.Start(command, t.TempDir(), "")
	}
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	return id
}

// TestKillBackgroundSingleSession: the reaper stops one session's processes
// without touching its history — the second run finds nothing to stop.
func TestKillBackgroundSingleSession(t *testing.T) {
	mgr := backgroundTestManager(t)
	id := backgroundTestSession(t, mgr, "reap-me", "sleep 30", true)

	if err := runListBackground(mgr, "reap-me"); err != nil {
		t.Fatalf("runListBackground: %v", err)
	}
	killed := killSessionBackground(mgr, "reap-me")
	if len(killed) != 1 || killed[0] != id {
		t.Fatalf("killed = %v, want [%s]", killed, id)
	}
	reg, _ := procRegistryFor(mgr, "reap-me")
	if info, _ := reg.Status(id); info.Running() {
		t.Error("the process is still running after the reaper")
	}
	if again := killSessionBackground(mgr, "reap-me"); len(again) != 0 {
		t.Errorf("a second reap stopped %v, want nothing", again)
	}
	if err := runKillBackground(mgr, "reap-me"); err != nil {
		t.Fatalf("runKillBackground: %v", err)
	}
	if !mgr.SessionExists("reap-me") {
		t.Error("reaping deleted the session history, want it kept")
	}
}

// TestKillBackgroundAll: one command reaps the red processes abandoned
// across sessions — the hundreds-of-sessions leak this exists for.
func TestKillBackgroundAll(t *testing.T) {
	mgr := backgroundTestManager(t)
	backgroundTestSession(t, mgr, "sess-a", "sleep 30", true)
	backgroundTestSession(t, mgr, "sess-b", "sleep 30", false)

	if err := runKillBackground(mgr, "all"); err != nil {
		t.Fatalf("runKillBackground all: %v", err)
	}
	for _, name := range []string{"sess-a", "sess-b"} {
		reg, _ := procRegistryFor(mgr, name)
		for _, info := range reg.List() {
			if info.Running() {
				t.Errorf("%s: %s is still running after reaping all", name, info.ID)
			}
		}
	}
}

// TestBackgroundUnknownSession: a typo fails loudly instead of silently
// reaping nothing.
func TestBackgroundUnknownSession(t *testing.T) {
	mgr := backgroundTestManager(t)
	if _, err := resolveBackgroundSessions(mgr, "no-such-session"); err == nil {
		t.Error("resolving an unknown session was accepted")
	}
	if err := runListBackground(mgr, "no-such-session"); err == nil {
		t.Error("listing an unknown session was accepted")
	}
	if err := runKillBackground(mgr, "no-such-session"); err == nil {
		t.Error("killing an unknown session was accepted")
	}
}

// TestProcRegistryForReadsSessionDir: the CLI reads the same proc/ directory
// the turn writes, or the reaper would miss what the turn started.
func TestProcRegistryForReadsSessionDir(t *testing.T) {
	mgr := backgroundTestManager(t)
	sess, err := mgr.Open("same-dir")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	viaSess, err := procs.New(sess.ProcDir())
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	id, err := viaSess.Start("sleep 30", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = viaSess.Kill(id) }()

	viaCLI, err := procRegistryFor(mgr, "same-dir")
	if err != nil {
		t.Fatalf("procRegistryFor: %v", err)
	}
	if _, err := viaCLI.Status(id); err != nil {
		t.Errorf("the CLI registry does not see %s: %v", id, err)
	}
}

package main

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/inbox"
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

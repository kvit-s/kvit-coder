package agent

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/inbox"
)

// answerPause pushes text once the runner reaches its pause prompt, the way a
// person types after the prompt appears. Polling for PauseMode rather than
// sleeping keeps it deterministic: the push always lands after Ask started, so
// Ask claims it as the answer instead of holding it aside as an earlier line.
func answerPause(box *inbox.Inbox, text string) {
	go func() {
		for range 500 {
			if box.PauseMode() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: text})
	}()
}

// TestPauseTakesSteeringAtNextBoundary: an empty Enter's request stops the
// turn at its next iteration boundary, and what is typed at the pause prompt
// reaches the model as <user-steering> — including a line typed before the
// prompt appeared, which stays ordinary steering rather than becoming the
// answer.
func TestPauseTakesSteeringAtNextBoundary(t *testing.T) {
	box := inbox.New("")
	box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "typed before pause"})
	if !box.RequestPause() {
		t.Fatal("the pause request is not new")
	}
	answerPause(box, "typed at pause")

	client := newFakeClient(answer("done"))
	runner, _ := newTestRunner(t, testConfig(), client)
	runner.inbox = box

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertShape(t, res.FinalMessages, "user", "user", "user", "assistant")
	if !strings.Contains(res.FinalMessages[1].Content, "typed before pause") {
		t.Errorf("the earlier line is not steering: %q", res.FinalMessages[1].Content)
	}
	steer := res.FinalMessages[2]
	if !strings.Contains(steer.Content, "<user-steering>") || !strings.Contains(steer.Content, "typed at pause") {
		t.Errorf("the pause answer is %q, want it tagged and carrying the typed text", steer.Content)
	}

	// Both lines are in the first request, not just the history: the pause
	// happens before the first model call.
	first := client.requests[0].Messages
	assertShape(t, first, "user", "user", "user")

	// Waiting for a person is not the tool being slow: pause time must be on
	// the prompt-wait clock, not charged against the next tool's timeout.
	if runner.toolCtx.PromptWait() <= 0 {
		t.Error("the pause spent no time on the prompt-wait clock")
	}
}

// TestPauseEmptyResumesWithoutSteering: Enter on the empty pause prompt
// resumes with nothing for the model.
func TestPauseEmptyResumesWithoutSteering(t *testing.T) {
	box := inbox.New("")
	if !box.RequestPause() {
		t.Fatal("the pause request is not new")
	}
	answerPause(box, "")

	client := newFakeClient(answer("done"))
	runner, _ := newTestRunner(t, testConfig(), client)
	runner.inbox = box

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	assertShape(t, res.FinalMessages, "user", "assistant")
	if res.Cancelled {
		t.Error("an empty resume cancelled the turn")
	}
}

// TestPauseCancelledEndsTurn: Ctrl-C at the pause prompt ends the turn as
// cancelled, like at any other prompt, with what happened saved.
func TestPauseCancelledEndsTurn(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	box := inbox.New("")
	if !box.RequestPause() {
		t.Fatal("the pause request is not new")
	}
	go func() {
		for range 500 {
			if box.PauseMode() {
				break
			}
			time.Sleep(10 * time.Millisecond)
		}
		cancel()
	}()

	client := newFakeClient(answer("unreachable"))
	runner, _ := newTestRunner(t, testConfig(), client)
	runner.inbox = box

	res, err := runner.Run(ctx, RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if !res.Cancelled {
		t.Error("a turn cancelled at the pause prompt does not report it")
	}
	if res.BudgetExhausted {
		t.Error("a cancelled run reports the iteration budget as exhausted")
	}
	assertShape(t, res.FinalMessages, "user")
	if client.callCount() != 0 {
		t.Errorf("made %d model calls, want none: the pause is before the first call", client.callCount())
	}
}

// TestPauseBetweenToolCalls: a pause requested while the first of two batched
// calls runs stops the turn between the calls, not after the batch. The
// steering typed at the prompt reaches the model as <user-steering>, and the
// pause resolves before the second call runs.
func TestPauseBetweenToolCalls(t *testing.T) {
	box := inbox.New("")
	started := make(chan struct{})
	release := make(chan struct{})
	var startOnce sync.Once
	gate := &scriptedTool{name: "gate", call: func(context.Context, json.RawMessage) (any, error) {
		startOnce.Do(func() { close(started) })
		<-release
		return map[string]any{"ok": true}, nil
	}}
	var echoAt, pushAt atomic.Int64
	echo := &scriptedTool{name: "echo", call: func(context.Context, json.RawMessage) (any, error) {
		echoAt.Store(time.Now().UnixNano())
		return map[string]any{"ok": true}, nil
	}}
	client := newFakeClient(
		calls(toolCall("c1", "gate", map[string]string{"arg": "x"}),
			toolCall("c2", "echo", map[string]string{"arg": "y"})),
		answer("done"),
	)
	runner, _ := newTestRunner(t, testConfig(), client, gate, echo)
	runner.inbox = box

	errCh := make(chan string, 1)
	go func() {
		<-started
		if !box.RequestPause() {
			errCh <- "the mid-batch pause request is not new"
			close(release)
			return
		}
		close(release)
		// Answer once the pause prompt is actually waiting, so the push
		// lands after Ask started and is taken as the answer.
		for range 500 {
			if box.Awaiting() {
				break
			}
			time.Sleep(time.Millisecond)
		}
		pushAt.Store(time.Now().UnixNano())
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "mid-batch steering"})
	}()

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("go")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	select {
	case e := <-errCh:
		t.Fatalf("reader: %s", e)
	default:
	}

	if res.Cancelled {
		t.Error("a turn paused between tools reports cancelled")
	}
	assertShape(t, res.FinalMessages,
		"user",
		"assistant+tool_calls",
		"tool:gate",
		"tool:echo",
		"user",
		"assistant",
	)
	steer := res.FinalMessages[4]
	if !strings.Contains(steer.Content, "<user-steering>") || !strings.Contains(steer.Content, "mid-batch steering") {
		t.Errorf("the pause answer is %q, want it tagged and carrying the typed text", steer.Content)
	}
	// The pause resolved between the calls: without the mid-batch check the
	// steering push would land after the second call ran.
	if push, echo := pushAt.Load(), echoAt.Load(); push == 0 || echo == 0 || push >= echo {
		t.Errorf("pause push at %d, second call at %d: want the pause resolved first", push, echo)
	}
}

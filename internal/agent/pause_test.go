package agent

import (
	"context"
	"strings"
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

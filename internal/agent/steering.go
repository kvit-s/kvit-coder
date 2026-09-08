package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// pollProcesses brings the background-process records up to date and queues
// what the model has not been told: a process that ended, or a periodic probe
// whose output moved on. Doing it here rather than with a model call per tick
// is the point of the registry — watching something costs nothing until there
// is something to say.
func (r *Runner) pollProcesses() {
	if r.procs == nil || r.inbox == nil {
		return
	}
	tools.ProcessEvents(r.procs, r.inbox)
}

// runningProcsCount reports how many background processes are still running,
// for the ★N suffix on the step header. Nil registry (procs disabled) means
// none. Only running processes count: finished ones already surface through
// the inbox, and listing refreshes liveness from the pidfiles.
func (r *Runner) runningProcsCount() int {
	ephemeral, persistent := r.runningProcsCounts()
	return ephemeral + persistent
}

// runningProcsCounts splits the running processes into ephemeral (blue, die
// at turn end) and persistent (red, survive). Nil registry means none.
func (r *Runner) runningProcsCounts() (ephemeral, persistent int) {
	if r.procs == nil {
		return 0, 0
	}
	return r.procs.RunningCounts()
}

// drainInbox takes everything that arrived since the last iteration and puts it
// in front of the model. It runs at the top of an iteration, before the
// rollback point is taken, so a backtrack later in the iteration cannot discard
// what someone just said.
func (r *Runner) drainInbox(state *runState) {
	if r.inbox == nil {
		return
	}
	messages := r.inbox.Drain()
	if len(messages) == 0 {
		return
	}

	var lines []string
	for _, m := range messages {
		if strings.TrimSpace(m.Text) == "" {
			continue
		}
		switch m.Kind {
		case inbox.KindUserLine:
			// A person talking mid-turn is a user message, tagged so the model
			// can tell it from the prompt that started the turn.
			r.writer.Info(fmt.Sprintf("steering: %s", firstLine(m.Text)))
			state.messages = append(state.messages, llm.Message{
				Role:    llm.RoleUser,
				Content: "<user-steering>\n" + m.Text + "\n</user-steering>",
			})
		default:
			lines = append(lines, m.Text)
		}
	}

	// Process events and notices are not conversation, so they ride on the
	// last tool result as a reminder — the same mechanism loop detection uses.
	if len(lines) > 0 {
		reminder := "\n\n<system-reminder>\n" + strings.Join(lines, "\n") + "\n</system-reminder>"
		appendReminder(state, reminder)
	}
}

// checkPause stops at the iteration boundary when an empty Enter asked for a
// pause, and takes steering before the next model call. Output has stopped by
// now, so the line typed after is echoed cleanly. It reports whether the turn
// was cancelled while paused: then the caller ends the run as cancelled.
func (r *Runner) checkPause(ctx context.Context) bool {
	if r.inbox == nil {
		return false
	}
	// Raise the mode before consuming the request, so the progress watcher
	// never sees a gap with neither flag up and a dot lands on type-ahead.
	r.inbox.SetPauseMode(true)
	if !r.inbox.TakePause() {
		r.inbox.SetPauseMode(false)
		return false
	}
	defer r.inbox.SetPauseMode(false)
	r.writer.Steering("paused — type steering, Enter to resume (empty resumes):")
	// Waiting for a person is not the tool being slow, so pause time is
	// kept out of the next tool's timeout, like every other prompt.
	started := time.Now()
	text, outcome := r.inbox.Ask(ctx, nil, "", 0)
	r.toolCtx.AddPromptWait(time.Since(started))
	switch outcome {
	case inbox.AskCancelled:
		return true
	case inbox.AskAnswered:
		if strings.TrimSpace(text) != "" {
			// Ask consumed the line, so push it back for the drain below
			// to pick up through the ordinary <user-steering> path.
			r.inbox.Push(inbox.Message{Kind: inbox.KindUserLine, Text: text})
		}
		return false
	default: // AskTimedOut cannot happen with a zero timeout; resume anyway.
		return false
	}
}

// appendReminder puts text on the last tool result. With no tool result to
// carry it — the model answered without calling anything — it becomes a user
// message instead, because the API has nowhere else to put it.
func appendReminder(state *runState, reminder string) {
	if n := len(state.messages); n > 0 && state.messages[n-1].Role == llm.RoleTool {
		state.messages[n-1].Content += reminder
		return
	}
	state.messages = append(state.messages, llm.Message{
		Role:    llm.RoleUser,
		Content: strings.TrimSpace(reminder),
	})
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i] + "…"
	}
	return s
}

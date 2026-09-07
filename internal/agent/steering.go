package agent

import (
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

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
			r.writer.Info(fmt.Sprintf("↩ steering: %s", firstLine(m.Text)))
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

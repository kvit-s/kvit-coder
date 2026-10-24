package session

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// The fixed texts compaction puts into the context. They are fixed so that the
// start of the context stays byte-identical from one request to the next until
// the next compaction, which is what keeps the server's prompt cache hitting.
const (
	// OlderTurnsQuestion is the user message in front of the summary that
	// stands in for the oldest turns.
	OlderTurnsQuestion = "What happened earlier in this session?"
	// CompactedLabel introduces the model's compacted version of the turn in
	// progress, after that turn's own words.
	CompactedLabel = "Work done so far in this turn, compacted:"
	// RemovedOutputNote replaces a tool's output when the model could not
	// compact the turn it belongs to.
	RemovedOutputNote = "[output removed to fit the context window]"
	// InboxTurnInput stands in for the user's words in a turn that the inbox
	// started without any.
	InboxTurnInput = "[turn started by the inbox]"
)

// Context is the conversation as the model is sent it: the history with
// whatever the latest compaction replaced, without the system prompt.
//
// Messages from TailIndex on are the conversation's own, word for word,
// starting at position TailStart. Everything before TailIndex was built by
// compaction. A turn that grows the conversation appends after the tail, so
// the message at index TailIndex+k is always the conversation's message at
// position TailStart+k.
type Context struct {
	Messages  []llm.Message
	TailIndex int
	TailStart int
}

// LoadContext reads the history and builds the context to send the model.
// Without a compaction record that is the whole conversation, as Load returns
// it.
func (s *Session) LoadContext() (*Context, *Timeline, error) {
	tl, err := s.Timeline()
	if err != nil {
		return nil, nil, err
	}
	return BuildContext(tl), tl, nil
}

// BuildContext applies the timeline's compaction records to its conversation
// (docs/compaction.md):
//
//	the summary of the oldest turns, when one was written
//	one turn summary for each turn from there to the turn in progress
//	the turn in progress up to the cut: its words and the compacted text,
//	    or, when the model could not compact it, its messages with the tool
//	    outputs removed
//	every message from the cut on, word for word
func BuildContext(tl *Timeline) *Context {
	latest := tl.LatestCompaction()
	if latest == nil {
		return &Context{Messages: append([]llm.Message(nil), tl.Messages...)}
	}
	n := len(tl.Messages)
	cut := max(0, min(latest.Cut, n))
	from := max(0, min(latest.From, cut))
	spans := tl.TurnSpans()

	var prefix []llm.Message
	summariesFrom := 0
	if older := tl.latestOlderSummary(); older != nil && older.OlderUpTo <= from {
		prefix = append(prefix,
			llm.Message{Role: llm.RoleUser, Content: OlderTurnsQuestion},
			llm.Message{Role: llm.RoleAssistant, Content: older.OlderSummary},
		)
		summariesFrom = older.OlderUpTo
	}
	for _, sp := range spans {
		if sp.Start < summariesFrom || sp.Start >= from {
			continue
		}
		sp.End = min(sp.End, from)
		prefix = append(prefix, tl.TurnSummary(sp)...)
	}
	if cut > from {
		if latest.Compacted != "" {
			words, images := tl.InProgressInput(from, cut)
			content := CompactedLabel + "\n\n" + latest.Compacted
			if words != "" {
				content = words + "\n\n" + content
			}
			prefix = append(prefix, llm.Message{Role: llm.RoleUser, Content: content, Images: images})
		} else {
			for _, m := range tl.Messages[from:cut] {
				if m.Role == llm.RoleTool {
					m.Content = RemovedOutputNote
				}
				prefix = append(prefix, m)
			}
		}
	}

	msgs := append(prefix, tl.Messages[cut:]...)
	return &Context{Messages: msgs, TailIndex: len(prefix), TailStart: cut}
}

func (tl *Timeline) latestOlderSummary() *Compaction {
	for i := len(tl.Compactions) - 1; i >= 0; i-- {
		if tl.Compactions[i].OlderSummary != "" {
			return &tl.Compactions[i]
		}
	}
	return nil
}

// TurnSpan is one turn's messages, Start to End, with the model it ran with
// and the notices written during it.
type TurnSpan struct {
	Start   int
	End     int
	Model   string
	Notices []string
}

// TurnSpans splits the conversation into turns at its settings lines. Messages
// before the first settings line, which only a session older than settings
// lines has, count as one turn.
func (tl *Timeline) TurnSpans() []TurnSpan {
	n := len(tl.Messages)
	var spans []TurnSpan
	if len(tl.Turns) == 0 || tl.Turns[0].At > 0 {
		spans = append(spans, TurnSpan{Start: 0, Notices: tl.noticesOf(-1)})
	}
	for i, t := range tl.Turns {
		spans = append(spans, TurnSpan{Start: min(t.At, n), Model: t.Model, Notices: tl.noticesOf(i)})
	}
	for i := range spans {
		if i+1 < len(spans) {
			spans[i].End = spans[i+1].Start
		} else {
			spans[i].End = n
		}
	}
	return spans
}

func (tl *Timeline) noticesOf(turn int) []string {
	var out []string
	for _, nt := range tl.Notices {
		if nt.Turn == turn {
			out = append(out, nt.Text)
		}
	}
	return out
}

// InProgressFrom is where the turn in progress begins: the end of the last
// turn that ended with an answer, or 0 when none has. Turns after it either
// ended without one — cancelled, out of iterations, failed — or are still
// running, and their work is in no final answer, so compaction treats them as
// one turn still under way.
func (tl *Timeline) InProgressFrom() int {
	spans := tl.TurnSpans()
	for i := len(spans) - 1; i >= 0; i-- {
		sp := spans[i]
		if sp.End <= sp.Start {
			continue
		}
		if _, ok, _ := tl.FinalAnswer(sp); ok {
			return sp.End
		}
	}
	return 0
}

// FinalAnswer is the answer a turn ended with: the accepted report rendered as
// text, or else the last assistant message when it calls no tools. A turn that
// was cancelled, ran out of iterations or out of time has none, and reason
// says which.
func (tl *Timeline) FinalAnswer(sp TurnSpan) (text string, ok bool, reason string) {
	for _, n := range sp.Notices {
		switch n {
		case NoticeCancelled, NoticeBudgetExhausted, NoticeTimedOut:
			return "", false, n
		}
	}
	msgs := tl.Messages[sp.Start:sp.End]
	last := -1
	for i := len(msgs) - 1; i >= 0; i-- {
		if msgs[i].Role == llm.RoleAssistant {
			last = i
			break
		}
	}
	if last < 0 {
		return "", false, ""
	}
	m := msgs[last]
	if rep := reportIn(m); rep != nil && reportAccepted(m, msgs[last+1:]) {
		return rep.PlainText(), true, ""
	}
	if len(m.ToolCalls) == 0 && strings.TrimSpace(m.Content) != "" {
		return m.Content, true, ""
	}
	return "", false, ""
}

// reportAccepted reports whether the Report call in m was accepted, which its
// tool result says as {"accepted": true}. A rejected report is an error the
// model was asked to repair, not an ending.
func reportAccepted(m llm.Message, after []llm.Message) bool {
	for _, tc := range m.ToolCalls {
		if tc.Function.Name != ReportToolName {
			continue
		}
		for _, r := range after {
			if r.Role != llm.RoleTool || r.ToolCallID != tc.ID {
				continue
			}
			var result struct {
				Accepted bool `json:"accepted"`
			}
			if json.Unmarshal([]byte(r.Content), &result) == nil && result.Accepted {
				return true
			}
		}
	}
	return false
}

// TurnSummary is a finished turn as compaction keeps it: what the user said in
// it, then the answer it ended with and one line of what it touched.
func (tl *Timeline) TurnSummary(sp TurnSpan) []llm.Message {
	if sp.End <= sp.Start {
		return nil
	}
	msgs := tl.Messages[sp.Start:sp.End]
	words, _ := userWords(msgs, true, false)
	if words == "" {
		words = InboxTurnInput
	}
	answer, ok, reason := tl.FinalAnswer(sp)
	if !ok {
		answer = "[the turn ended without an answer]"
		if reason != "" {
			answer = "[the turn ended without an answer: " + reason + "]"
		}
	}
	if line := ActivityLine(msgs); line != "" {
		answer += "\n\n" + line
	}
	return []llm.Message{
		{Role: llm.RoleUser, Content: words},
		{Role: llm.RoleAssistant, Content: answer},
	}
}

// InProgressInput is what the user said in the turn in progress between from
// and cut — the prompt of each turn it spans and any steering typed during
// them — with the pictures attached to those messages. The turn is still being
// worked on, so its pictures stay attached rather than becoming paths.
func (tl *Timeline) InProgressInput(from, cut int) (string, []llm.ImagePart) {
	var parts []string
	var images []llm.ImagePart
	for _, sp := range tl.TurnSpans() {
		start, end := max(sp.Start, from), min(sp.End, cut)
		if start >= end {
			continue
		}
		words, imgs := userWords(tl.Messages[start:end], start == sp.Start, true)
		if words != "" {
			parts = append(parts, words)
		}
		images = append(images, imgs...)
	}
	return strings.Join(parts, "\n\n"), images
}

// userWords collects the user-typed messages among msgs: the first message
// when it is the turn's opening one, and every <user-steering> message. Every
// other user-role message was put there by the agent itself — the report
// reminder, "continue", a server-error note — and is left out. Pictures are
// returned when keepImages is set, and otherwise named in the text.
func userWords(msgs []llm.Message, opensTurn, keepImages bool) (string, []llm.ImagePart) {
	var parts []string
	var images []llm.ImagePart
	for i, m := range msgs {
		if m.Role != llm.RoleUser {
			continue
		}
		steering := strings.HasPrefix(strings.TrimSpace(m.Content), "<user-steering>")
		if !(i == 0 && opensTurn) && !steering {
			continue
		}
		text := m.Content
		if keepImages {
			images = append(images, m.Images...)
		} else {
			for _, img := range m.Images {
				text += fmt.Sprintf("\n[image: %s]", img.Path)
			}
		}
		if strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, "\n\n"), images
}

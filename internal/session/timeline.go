package session

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// Timeline is history.jsonl read back with everything compaction needs: the
// conversation exactly as Load returns it, plus where each turn began, the
// notices, and the compaction records, each placed by message position.
//
// A position is an index into Messages: the number of conversation messages
// that came before the thing it marks. The file is only appended to, so a
// position never changes once written.
type Timeline struct {
	Messages    []llm.Message
	Turns       []TurnStart
	Notices     []Notice
	Compactions []Compaction
}

// TurnStart is where a turn began (its settings line) and the model it ran
// with.
type TurnStart struct {
	At    int
	Model string
}

// Notice is a notice line, the position it was written at, and the turn it
// was written in: an index into Turns, or -1 before the first settings line.
type Notice struct {
	At   int
	Turn int
	Text string
}

// Compaction is one compaction record. At is the position it was written at;
// the other fields are the record's own (see Event).
type Compaction struct {
	At           int
	From         int
	Cut          int
	Compacted    string
	OlderSummary string
	OlderUpTo    int
	TokensBefore int
	TokensAfter  int
	Model        string
}

// Timeline reads the session's history with turn boundaries, notices and
// compaction records.
func (s *Session) Timeline() (*Timeline, error) {
	return readTimeline(s.HistoryPath())
}

// AppendCompaction records a compaction.
func (s *Session) AppendCompaction(c Compaction) error {
	return s.Append(Event{
		Kind:         KindCompaction,
		From:         c.From,
		Cut:          c.Cut,
		Compacted:    c.Compacted,
		OlderSummary: c.OlderSummary,
		OlderUpTo:    c.OlderUpTo,
		TokensBefore: c.TokensBefore,
		TokensAfter:  c.TokensAfter,
		Model:        c.Model,
	})
}

// LatestCompaction is the last compaction record, or nil.
func (tl *Timeline) LatestCompaction() *Compaction {
	if len(tl.Compactions) == 0 {
		return nil
	}
	return &tl.Compactions[len(tl.Compactions)-1]
}

// WithCompaction is a copy of the timeline with c appended as if it had just
// been written, so the context it would produce can be built and measured
// before the record goes to disk.
func (tl *Timeline) WithCompaction(c Compaction) *Timeline {
	out := *tl
	c.At = len(tl.Messages)
	out.Compactions = append(append([]Compaction(nil), tl.Compactions...), c)
	return &out
}

func readTimeline(path string) (*Timeline, error) {
	tl := &Timeline{}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return tl, nil
		}
		return nil, fmt.Errorf("failed to open session history: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var ev Event
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			return nil, fmt.Errorf("failed to parse session event: %w", err)
		}
		if ev.Kind == "" && ev.Message == nil {
			// A line written before history was a stream of events is a bare
			// message. Reading it costs nothing and saves a migration failure
			// from losing a conversation.
			var msg llm.Message
			if err := json.Unmarshal([]byte(line), &msg); err == nil && msg.Role != "" {
				tl.Messages = append(tl.Messages, msg)
			}
			continue
		}
		switch ev.Kind {
		case KindRollback:
			// The loop took these back, so they are not part of the
			// conversation even though they are still in the file. A rollback
			// never reaches behind the last compaction's cut: what the
			// compaction kept word for word starts there, and its record says
			// so by position.
			floor := 0
			if c := tl.LatestCompaction(); c != nil {
				floor = min(c.Cut, len(tl.Messages))
			}
			if drop := min(ev.Dropped, len(tl.Messages)-floor); drop > 0 {
				tl.Messages = tl.Messages[:len(tl.Messages)-drop]
			}
			continue
		case KindSettings:
			tl.Turns = append(tl.Turns, TurnStart{At: len(tl.Messages), Model: ev.Model})
			continue
		case KindNotice:
			tl.Notices = append(tl.Notices, Notice{At: len(tl.Messages), Turn: len(tl.Turns) - 1, Text: ev.Text})
			continue
		case KindCompaction:
			tl.Compactions = append(tl.Compactions, Compaction{
				At:           len(tl.Messages),
				From:         ev.From,
				Cut:          ev.Cut,
				Compacted:    ev.Compacted,
				OlderSummary: ev.OlderSummary,
				OlderUpTo:    ev.OlderUpTo,
				TokensBefore: ev.TokensBefore,
				TokensAfter:  ev.TokensAfter,
				Model:        ev.Model,
			})
			continue
		}
		if ev.Message != nil {
			msg := *ev.Message
			msg.PromptTokens = ev.PromptTokens
			tl.Messages = append(tl.Messages, msg)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to read session history: %w", err)
	}
	return tl, nil
}

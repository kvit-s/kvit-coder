package tui

import (
	"path/filepath"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
)

const reportArgs = `{"task_status":"needs_action",` +
	`"headline":"Ordering works; the filtered case needs a decision.",` +
	`"blocks":[{"type":"decision","id":"filtered","summary":"Choose the filtered behaviour.",` +
	`"options":[{"id":"preserve","label":"Preserve hidden positions","effect":"dispatch","instruction":"Preserve them."},` +
	`{"id":"stop","label":"Leave it","effect":"resolve"}],` +
	`"recommendation":"preserve","recommendation_reason":"Least disruption.","response_type":"single"}]}`

// newTestUI is a front end over a sessions directory the test owns, with one
// session already holding the messages given.
func newTestUI(t *testing.T, name string, msgs []llm.Message) *UI {
	t.Helper()
	base := t.TempDir()
	mgr, err := session.NewManagerIn(base)
	if err != nil {
		t.Fatalf("NewManagerIn: %v", err)
	}
	sess, err := session.OpenDir(filepath.Join(base, name))
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	if len(msgs) > 0 {
		if err := sess.AppendMessages(msgs); err != nil {
			t.Fatalf("AppendMessages: %v", err)
		}
	}
	cfg := &config.Config{}
	return New(Options{SessionName: name, SessionMgr: mgr, Config: cfg})
}

func reportMessages() []llm.Message {
	return []llm.Message{
		{Role: llm.RoleUser, Content: "make ordering draggable"},
		{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
			ID: "r1", Type: "function",
			Function: llm.ToolCallFunction{Name: session.ReportToolName, Arguments: reportArgs},
		}}},
		{Role: llm.RoleTool, Name: session.ReportToolName, ToolCallID: "r1", Content: `{"accepted":true}`},
	}
}

// The card the front end draws comes from the session file the agent wrote,
// with no channel between the two processes beyond that file.
func TestCurrentReportComesFromTheSessionFile(t *testing.T) {
	u := newTestUI(t, "cards", reportMessages())

	rep := u.currentReport()
	if rep == nil {
		t.Fatal("no report read back from the session")
	}
	if rep.Headline != "Ordering works; the filtered case needs a decision." {
		t.Errorf("headline is %q", rep.Headline)
	}
	if !rep.HasInteractive() {
		t.Error("the decision block did not survive the round trip")
	}
}

func TestNoCardOnceTheConversationMovesOn(t *testing.T) {
	msgs := append(reportMessages(), llm.Message{Role: llm.RoleUser, Content: "yes, preserve them"})
	u := newTestUI(t, "cards", msgs)
	if rep := u.currentReport(); rep != nil {
		t.Errorf("a card was still offered after the report was answered: %v", rep.Headline)
	}
}

func TestNoCardWithoutASession(t *testing.T) {
	u := newTestUI(t, "cards", nil)
	if rep := u.currentReport(); rep != nil {
		t.Errorf("an empty session offered a card: %v", rep)
	}
	bare := New(Options{Config: &config.Config{}})
	if rep := bare.currentReport(); rep != nil {
		t.Errorf("a front end with no session offered a card: %v", rep)
	}
}

package session

import (
	"path/filepath"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

func reportCall(headline string) llm.Message {
	return llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{{
			ID: "r1", Type: "function",
			Function: llm.ToolCallFunction{
				Name: ReportToolName,
				Arguments: `{"task_status":"completed","headline":"` + headline + `",` +
					`"blocks":[{"type":"verification","id":"tests","summary":"Tests pass.","status":"passed"}]}`,
			},
		}},
	}
}

func reportResult() llm.Message {
	return llm.Message{Role: llm.RoleTool, Name: ReportToolName, ToolCallID: "r1", Content: `{"accepted":true}`}
}

func TestNoReportInAnEmptyConversation(t *testing.T) {
	rep, current, err := lastReportIn(nil)
	if err != nil {
		t.Fatalf("lastReportIn: %v", err)
	}
	if rep != nil || current {
		t.Errorf("found a report in nothing: %v %v", rep, current)
	}
}

// The report's own tool result belongs to the call above it, so it does not
// count as something that came after the report.
func TestReportIsCurrentUnderItsToolResult(t *testing.T) {
	msgs := []llm.Message{
		{Role: llm.RoleUser, Content: "rename the helper"},
		reportCall("Renamed the helper."),
		reportResult(),
	}
	rep, current, _ := lastReportIn(msgs)
	if rep == nil {
		t.Fatal("no report found")
	}
	if !current {
		t.Error("the report is not the last message, but nothing follows it")
	}
	if rep.Headline != "Renamed the helper." {
		t.Errorf("headline is %q", rep.Headline)
	}
}

// Answering a report is what takes the card away: the next turn appends
// messages, so the report stops being the last one. Nothing has to be tracked
// for that to work.
func TestAnsweringMakesTheReportStale(t *testing.T) {
	msgs := []llm.Message{
		reportCall("Renamed the helper."),
		reportResult(),
		{Role: llm.RoleUser, Content: "yes, do that"},
	}
	rep, current, _ := lastReportIn(msgs)
	if rep == nil {
		t.Fatal("the report should still be findable")
	}
	if current {
		t.Error("the report is still current after a prompt was sent")
	}
}

// A cancelled or budget-exhausted turn appends messages of its own, which has
// the same effect.
func TestALaterTurnMakesTheReportStale(t *testing.T) {
	msgs := []llm.Message{
		reportCall("Renamed the helper."),
		reportResult(),
		{Role: llm.RoleUser, Content: "now do the other thing"},
		{Role: llm.RoleAssistant, Content: "[cancelled by user]"},
	}
	_, current, _ := lastReportIn(msgs)
	if current {
		t.Error("the report is still current after a later turn")
	}
}

func TestTheMostRecentReportWins(t *testing.T) {
	msgs := []llm.Message{
		reportCall("First."),
		reportResult(),
		{Role: llm.RoleUser, Content: "again"},
		reportCall("Second."),
		reportResult(),
	}
	rep, current, _ := lastReportIn(msgs)
	if rep == nil || rep.Headline != "Second." {
		t.Fatalf("found %v, want the second report", rep)
	}
	if !current {
		t.Error("the newest report is not current")
	}
}

// A report that cannot be parsed costs a card, not a session.
func TestMalformedReportIsSkipped(t *testing.T) {
	broken := llm.Message{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{{
			ID: "r2", Function: llm.ToolCallFunction{Name: ReportToolName, Arguments: `{"task_status":`},
		}},
	}
	msgs := []llm.Message{reportCall("Good one."), reportResult(), {Role: llm.RoleUser, Content: "again"}, broken}
	rep, current, err := lastReportIn(msgs)
	if err != nil {
		t.Fatalf("a malformed report failed the read: %v", err)
	}
	if rep == nil || rep.Headline != "Good one." {
		t.Fatalf("found %v, want the earlier readable report", rep)
	}
	if current {
		t.Error("an earlier report was reported as current")
	}
}

func TestOtherToolCallsAreNotReports(t *testing.T) {
	msgs := []llm.Message{{
		Role: llm.RoleAssistant,
		ToolCalls: []llm.ToolCall{{
			ID: "e1", Function: llm.ToolCallFunction{Name: "Edit", Arguments: `{"path":"x.go"}`},
		}},
	}}
	rep, _, _ := lastReportIn(msgs)
	if rep != nil {
		t.Errorf("an Edit call was read as a report: %v", rep)
	}
}

// The reader goes through the session directory the same way the conversation
// does, so a rollback that discarded a report also takes its card away.
func TestLastReportThroughASession(t *testing.T) {
	base := t.TempDir()
	mgr := &Manager{baseDir: base}
	sess, err := OpenDir(filepath.Join(base, "cards"))
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	if err := sess.AppendMessages([]llm.Message{
		{Role: llm.RoleUser, Content: "rename the helper"},
		reportCall("Renamed the helper."),
		reportResult(),
	}); err != nil {
		t.Fatalf("AppendMessages: %v", err)
	}
	// A notice about background processes is written after the report on the
	// success path, and it is not a message, so it must not hide the card.
	if err := sess.Notice("persistent background processes still running: build"); err != nil {
		t.Fatalf("Notice: %v", err)
	}

	rep, current, err := sess.LastReport()
	if err != nil {
		t.Fatalf("LastReport: %v", err)
	}
	if rep == nil || !current {
		t.Fatalf("report=%v current=%v, want the card to show under a trailing notice", rep, current)
	}

	rep, current, err = mgr.LastReport("cards")
	if err != nil || rep == nil || !current {
		t.Fatalf("Manager.LastReport gave %v %v %v", rep, current, err)
	}
	if rep, current, err := mgr.LastReport("no-such-session"); err != nil || rep != nil || current {
		t.Errorf("a missing session gave %v %v %v, want no report and no error", rep, current, err)
	}
}

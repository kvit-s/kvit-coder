package session

import (
	"encoding/json"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/report"
)

// ReportToolName is the tool whose arguments are a structured report. The name
// is the link between the agent that submits one and the front end that draws
// it, so it is written down once here.
const ReportToolName = "Report"

// LastReport returns the most recent structured report in this session's
// history, and whether it is still the conversation's last message.
//
// Nothing about a report is stored separately: the model submitted it as the
// arguments of a Report tool call, so it is already in history.jsonl as part of
// an ordinary assistant message. Reading it back from there means there is one
// copy of it rather than two that can drift.
//
// "Current" — the second return value — is what decides whether the front end
// draws the card. It is true when nothing follows the report except its own
// tool result, which is what makes tracking unnecessary: a cancelled turn, an
// exhausted iteration budget and an answered report all append messages of
// their own, so each of them takes the card away by being there.
//
// A report that fails to parse is skipped rather than returned as an error: the
// worst case is a card that does not appear, and refusing to open a session
// over one malformed line would be worse.
func (s *Session) LastReport() (rep *report.Report, current bool, err error) {
	messages, err := s.Load()
	if err != nil {
		return nil, false, err
	}
	return lastReportIn(messages)
}

// lastReportIn is LastReport's logic over a conversation already in memory, so
// it can be tested without a session directory.
func lastReportIn(messages []llm.Message) (*report.Report, bool, error) {
	// Trailing tool results belong to the call above them, so they do not
	// count as something that came after the report.
	last := len(messages) - 1
	for last >= 0 && messages[last].Role == llm.RoleTool {
		last--
	}

	for i := len(messages) - 1; i >= 0; i-- {
		rep := reportIn(messages[i])
		if rep == nil {
			continue
		}
		return rep, i == last, nil
	}
	return nil, false, nil
}

// reportIn returns the report one message submitted, or nil.
func reportIn(msg llm.Message) *report.Report {
	if msg.Role != llm.RoleAssistant {
		return nil
	}
	for _, tc := range msg.ToolCalls {
		if tc.Function.Name != ReportToolName {
			continue
		}
		var rep report.Report
		if err := json.Unmarshal([]byte(tc.Function.Arguments), &rep); err != nil {
			continue
		}
		if rep.Headline == "" && len(rep.Blocks) == 0 {
			continue
		}
		report.Normalize(&rep)
		return &rep
	}
	return nil
}

// LastReport returns the most recent structured report in the named session,
// and whether it is still that conversation's last message. It is what the
// interactive front end asks between turns, without opening the session or
// taking its lock: a report is only ever read here, never written.
//
// A session that does not exist has no report, which is not an error.
func (m *Manager) LastReport(name string) (*report.Report, bool, error) {
	if name == "" {
		return nil, false, nil
	}
	messages, err := m.LoadSession(name)
	if err != nil {
		return nil, false, err
	}
	return lastReportIn(messages)
}

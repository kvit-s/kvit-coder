package agent

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/report"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

// readOnlyTool is a scripted tool that declares itself safe to run beside
// others, which is how a tool says it only reads. Report mode "mutating" asks
// nothing of a turn that used only these.
type readOnlyTool struct{ scriptedTool }

func (r *readOnlyTool) ParallelSafe() bool { return true }

// reportingConfig turns the Report tool on in the given mode.
func reportingConfig(mode config.ReportMode) *config.Config {
	cfg := testConfig()
	cfg.Tools.Report.Enabled = true
	cfg.Tools.Report.Mode = mode
	return cfg
}

// newReportingRunner is newTestRunner with the Report tool registered against
// the runner's own tool context, which is where an accepted report lands.
func newReportingRunner(t *testing.T, cfg *config.Config, client LLMClient, ts ...tools.Tool) *Runner {
	t.Helper()
	runner, _ := newReportingRunnerOut(t, cfg, client, ts...)
	return runner
}

func newReportingRunnerOut(t *testing.T, cfg *config.Config, client LLMClient, ts ...tools.Tool) (*Runner, *bytes.Buffer) {
	t.Helper()
	runner, out := newTestRunner(t, cfg, client, ts...)
	runner.registry.Enable(tools.NewReportTool(cfg, runner.toolCtx))
	return runner, out
}

// goodReport is the arguments of a report that passes every check.
func goodReport() map[string]any {
	return map[string]any{
		"task_status": "completed",
		"headline":    "Renamed the ordering helper and updated its callers.",
		"blocks": []any{map[string]any{
			"type": "verification", "id": "unit-tests",
			"summary": "All 14 tests in the package pass.", "status": "passed",
		}},
	}
}

func nudged(msgs []llm.Message) bool {
	for _, m := range msgs {
		if m.Role == llm.RoleUser && strings.Contains(m.Content, "has not submitted a report") {
			return true
		}
	}
	return false
}

// An accepted report ends the turn where it is: the loop does not go back to
// the model for a prose ending it would not use.
func TestAcceptedReportEndsTheTurn(t *testing.T) {
	client := newFakeClient(calls(toolCall("r1", "Report", goodReport())))
	runner := newReportingRunner(t, reportingConfig(config.ReportModeAlways), client)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("rename the helper")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	assertShape(t, res.FinalMessages, "user", "assistant+tool_calls", "tool:Report")
	if client.callCount() != 1 {
		t.Errorf("made %d model calls, want 1: the turn should end on the report", client.callCount())
	}
	rep := runner.toolCtx.AcceptedReport()
	if rep == nil || rep.TaskStatus != report.StatusCompleted {
		t.Fatalf("accepted report is %v", rep)
	}
}

// A turn that changed something and stopped without reporting is not finished.
// It is told so, and it reports.
func TestTurnThatChangedSomethingIsAskedForAReport(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "echo", map[string]string{"arg": "x"})),
		answer("I renamed the helper."),
		calls(toolCall("r1", "Report", goodReport())),
	)
	echo := &scriptedTool{name: "echo"}
	runner := newReportingRunner(t, reportingConfig(config.ReportModeMutating), client, echo)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("rename the helper")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !nudged(res.FinalMessages) {
		t.Fatalf("the turn was never asked for a report: %v", shape(res.FinalMessages))
	}
	if runner.toolCtx.AcceptedReport() == nil {
		t.Error("the turn ended with no accepted report")
	}
	if client.callCount() != 3 {
		t.Errorf("made %d model calls, want 3", client.callCount())
	}
}

// A turn that only read owes nothing under the default mode, so an ordinary
// question keeps its ordinary prose answer.
func TestReadOnlyTurnIsNotAskedForAReport(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "look", map[string]string{"arg": "x"})),
		answer("The helper is in internal/store."),
	)
	look := &readOnlyTool{scriptedTool{name: "look"}}
	runner := newReportingRunner(t, reportingConfig(config.ReportModeMutating), client, look)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("where is the helper?")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if nudged(res.FinalMessages) {
		t.Error("a read-only turn was asked for a report under mode mutating")
	}
	if client.callCount() != 2 {
		t.Errorf("made %d model calls, want 2", client.callCount())
	}
}

// Mode tools counts the same turn as owing one, which is the difference
// between the two settings.
func TestReadOnlyTurnIsAskedUnderModeTools(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "look", map[string]string{"arg": "x"})),
		answer("The helper is in internal/store."),
		calls(toolCall("r1", "Report", goodReport())),
	)
	look := &readOnlyTool{scriptedTool{name: "look"}}
	runner := newReportingRunner(t, reportingConfig(config.ReportModeTools), client, look)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("where is the helper?")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !nudged(res.FinalMessages) {
		t.Errorf("mode tools did not ask a read-only turn for a report: %v", shape(res.FinalMessages))
	}
}

func TestModeOffNeverAsks(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "echo", map[string]string{"arg": "x"})),
		answer("I renamed the helper."),
	)
	echo := &scriptedTool{name: "echo"}
	runner := newReportingRunner(t, reportingConfig(config.ReportModeOff), client, echo)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("rename the helper")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if nudged(res.FinalMessages) {
		t.Error("mode off asked for a report")
	}
	if client.callCount() != 2 {
		t.Errorf("made %d model calls, want 2", client.callCount())
	}
}

// A model that will not produce a report ends with prose rather than being
// asked until the iteration budget runs out.
func TestTheNudgeGivesUp(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("c1", "echo", map[string]string{"arg": "x"})),
		answer("first ending"),
		answer("second ending"),
		answer("third ending"),
	)
	echo := &scriptedTool{name: "echo"}
	runner := newReportingRunner(t, reportingConfig(config.ReportModeAlways), client, echo)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("rename the helper")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if client.callCount() != 1+maxReportNudges+1 {
		t.Errorf("made %d model calls, want %d", client.callCount(), 1+maxReportNudges+1)
	}
	if res.BudgetExhausted {
		t.Error("the turn ran out of iterations instead of giving up on the report")
	}
	last := res.FinalMessages[len(res.FinalMessages)-1]
	if last.Role != llm.RoleAssistant || last.Content != "third ending" {
		t.Errorf("the turn did not end on prose: %#v", last)
	}
}

// Once the model has used up its repair attempts it was told to stop, so the
// gate must not turn round and ask again.
func TestExhaustedRepairsStopTheGate(t *testing.T) {
	cfg := reportingConfig(config.ReportModeAlways)
	cfg.Tools.Report.MaxRepairAttempts = 1
	client := newFakeClient(
		calls(toolCall("r1", "Report", map[string]any{"task_status": "completed", "headline": ""})),
		answer("I could not fit the schema, so here it is in prose."),
	)
	runner := newReportingRunner(t, cfg, client)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("rename the helper")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if nudged(res.FinalMessages) {
		t.Error("the gate asked for a report after telling the model to stop trying")
	}
	if client.callCount() != 2 {
		t.Errorf("made %d model calls, want 2", client.callCount())
	}
}

// A rejected report reaches the conversation, so the next attempt can repair
// from the list rather than guessing.
func TestRejectedReportIsVisibleToTheModel(t *testing.T) {
	client := newFakeClient(
		calls(toolCall("r1", "Report", map[string]any{"task_status": "completed", "headline": ""})),
		calls(toolCall("r2", "Report", goodReport())),
	)
	runner := newReportingRunner(t, reportingConfig(config.ReportModeAlways), client)

	res, err := runner.Run(context.Background(), RunConfig{Messages: userStart("rename the helper")})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	var rejection string
	for _, m := range res.FinalMessages {
		if m.Role == llm.RoleTool && strings.Contains(m.Content, "REPORT_REJECTED") {
			rejection = m.Content
		}
	}
	if rejection == "" {
		t.Fatalf("the rejection never reached the conversation: %v", shape(res.FinalMessages))
	}
	if !strings.Contains(rejection, "/headline") {
		t.Errorf("the rejection does not name the field to fix: %s", rejection)
	}
	if runner.toolCtx.AcceptedReport() == nil {
		t.Error("the repaired report was not accepted")
	}
}

// A headless run prints the report where the prose answer would have gone, so
// piping the agent still ends with something readable.
func TestHeadlessRunPrintsTheReport(t *testing.T) {
	client := newFakeClient(calls(toolCall("r1", "Report", goodReport())))
	runner, out := newReportingRunnerOut(t, reportingConfig(config.ReportModeAlways), client)

	if _, err := runner.Run(context.Background(), RunConfig{Messages: userStart("rename the helper")}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	printed := out.String()
	if !strings.Contains(printed, "DONE ·") {
		t.Errorf("the status chip was not printed:\n%s", printed)
	}
	if !strings.Contains(printed, "All 14 tests in the package pass.") {
		t.Errorf("the report body was not printed:\n%s", printed)
	}
}

// Under the front end the card is drawn there, from the session, so the agent
// prints nothing and the user sees one card rather than two.
func TestUnderTheFrontEndTheAgentPrintsNoCard(t *testing.T) {
	t.Setenv(session.FromUIVar, "1")
	client := newFakeClient(calls(toolCall("r1", "Report", goodReport())))
	runner, out := newReportingRunnerOut(t, reportingConfig(config.ReportModeAlways), client)

	if _, err := runner.Run(context.Background(), RunConfig{Messages: userStart("rename the helper")}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if printed := out.String(); strings.Contains(printed, "DONE ·") {
		t.Errorf("the agent drew a card the front end will draw again:\n%s", printed)
	}
}

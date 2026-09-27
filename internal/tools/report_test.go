package tools

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/report"
)

func newReportTool(t *testing.T) (*ReportTool, *ToolContext) {
	t.Helper()
	cfg := newTestConfig()
	cfg.Tools.Report.Enabled = true
	tc := NewToolContext()
	return NewReportTool(cfg, tc), tc
}

const validReportArgs = `{
  "task_status": "completed",
  "headline": "Renamed the ordering helper and updated its callers.",
  "blocks": [
    {"type": "check", "id": "unit-tests",
     "summary": "All 14 tests in the package pass.",
     "status": "passed", "evidence": "go test ./internal/store"}
  ]
}`

func TestReportAcceptsAValidReport(t *testing.T) {
	tool, tc := newReportTool(t)

	res, err := tool.Call(context.Background(), json.RawMessage(validReportArgs))
	if err != nil {
		t.Fatalf("a valid report was rejected: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok || m["accepted"] != true {
		t.Fatalf("result does not say it was accepted: %#v", res)
	}

	rep := tc.AcceptedReport()
	if rep == nil {
		t.Fatal("the accepted report did not reach the tool context")
	}
	if rep.TaskStatus != report.StatusCompleted || len(rep.Blocks) != 1 {
		t.Errorf("stored report is %#v", rep)
	}
	if tc.ReportRepairs() != 0 {
		t.Errorf("an accepted report counted %d repairs", tc.ReportRepairs())
	}
}

// A rejection has to reach the conversation so the model can repair from it,
// which is why it is a runtime error rather than the kind the loop discards
// along with the message that produced it.
func TestReportRejectionIsNotBacktrackable(t *testing.T) {
	tool, _ := newReportTool(t)
	_, err := tool.Call(context.Background(), json.RawMessage(`{"task_status":"completed","headline":"","blocks":[]}`))
	if err == nil {
		t.Fatal("an empty headline was accepted")
	}
	if IsBacktrackable(err) {
		t.Error("the rejection is backtrackable, so the report would be discarded rather than repaired")
	}
}

func TestReportRejectionListsEveryProblem(t *testing.T) {
	tool, tc := newReportTool(t)
	_, err := tool.Call(context.Background(), json.RawMessage(`{
	  "task_status": "completed",
	  "headline": "",
	  "blocks": [{"type": "check", "id": "Bad Id", "summary": ""}]
	}`))
	if err == nil {
		t.Fatal("a broken report was accepted")
	}
	msg := err.Error()
	if !strings.HasPrefix(msg, "REPORT_REJECTED") {
		t.Errorf("rejection does not lead with the marker: %q", msg)
	}
	for _, want := range []string{"/headline", "/blocks/0/id", "/blocks/0/summary", "/blocks/0/status"} {
		if !strings.Contains(msg, want) {
			t.Errorf("rejection does not name %s:\n%s", want, msg)
		}
	}
	if tc.ReportRepairs() != 1 {
		t.Errorf("repairs counted %d, want 1", tc.ReportRepairs())
	}
	if tc.AcceptedReport() != nil {
		t.Error("a rejected report was stored as accepted")
	}
}

// Past the repair budget the model is told to stop and end with prose, rather
// than spending the whole turn resubmitting a report it cannot get right.
func TestReportRepairBudgetEndsTheAttempts(t *testing.T) {
	tool, tc := newReportTool(t)
	tool.cfg.Tools.Report.MaxRepairAttempts = 2
	broken := json.RawMessage(`{"task_status":"completed","headline":"","blocks":[]}`)

	_, err := tool.Call(context.Background(), broken)
	if strings.Contains(err.Error(), "Stop repairing") {
		t.Error("the first rejection already told the model to stop")
	}
	_, err = tool.Call(context.Background(), broken)
	if !strings.Contains(err.Error(), "Stop repairing") {
		t.Errorf("the last rejection did not say to stop:\n%s", err)
	}
	if tc.ReportRepairs() != 2 {
		t.Errorf("repairs counted %d, want 2", tc.ReportRepairs())
	}
}

func TestReportRejectsArgumentsThatAreNotJSON(t *testing.T) {
	tool, _ := newReportTool(t)
	_, err := tool.Call(context.Background(), json.RawMessage(`{"task_status": `))
	if err == nil {
		t.Fatal("malformed JSON was accepted")
	}
	if !strings.Contains(err.Error(), "not valid JSON") {
		t.Errorf("rejection does not say the arguments were malformed:\n%s", err)
	}
}

func TestReportHonoursTheBlockLimit(t *testing.T) {
	tool, _ := newReportTool(t)
	tool.cfg.Tools.Report.MaxBlocks = 1
	_, err := tool.Call(context.Background(), json.RawMessage(`{
	  "task_status": "completed",
	  "headline": "Two things happened.",
	  "blocks": [
	    {"type": "check", "id": "tests", "summary": "Tests pass.", "status": "passed"},
	    {"type": "change", "id": "edit", "summary": "Renamed the helper."}
	  ]
	}`))
	if err == nil || !strings.Contains(err.Error(), "at most 1") {
		t.Errorf("the configured block limit was not applied: %v", err)
	}
}

// The schema the model reads has to bound what the tool checks, or a model
// following the schema exactly still gets rejected.
func TestSchemaAgreesWithTheLimits(t *testing.T) {
	tool, _ := newReportTool(t)
	tool.cfg.Tools.Report.MaxBlocks = 7
	schema := tool.JSONSchema()
	props := schema["properties"].(map[string]any)
	blocks := props["blocks"].(map[string]any)
	if !strings.Contains(blocks["description"].(string), "0 to 7") {
		t.Errorf("the schema does not state the configured block limit: %v", blocks["description"])
	}
	required, _ := schema["required"].([]string)
	if len(required) != 3 {
		t.Errorf("schema requires %v, want task_status, headline and blocks", required)
	}
}

func TestMutatesExcludesTheReportTool(t *testing.T) {
	tool, _ := newReportTool(t)
	if Mutates(tool) {
		t.Error("Report counted as a tool that changes something, so every reported turn would look mutating")
	}
}

func TestReportModeDecidesWhoOwesOne(t *testing.T) {
	for _, tc := range []struct {
		mode              config.ReportMode
		usedTools, mutate bool
		want              bool
	}{
		{config.ReportModeOff, true, true, false},
		{config.ReportModeMutating, true, false, false},
		{config.ReportModeMutating, true, true, true},
		{config.ReportModeTools, true, false, true},
		{config.ReportModeTools, false, false, false},
		{config.ReportModeAlways, false, false, true},
	} {
		c := config.ReportToolConfig{Enabled: true, Mode: tc.mode}
		if got := c.Required(tc.usedTools, tc.mutate); got != tc.want {
			t.Errorf("mode %s with tools=%v mutated=%v gave %v, want %v",
				tc.mode, tc.usedTools, tc.mutate, got, tc.want)
		}
	}
	off := config.ReportToolConfig{Enabled: false, Mode: config.ReportModeAlways}
	if off.Required(true, true) {
		t.Error("a disabled Report tool still asked for a report")
	}
}

// The tool asks the turn what it did, so the same report is judged by what the
// turn actually changed rather than by a fixed rule.
func TestReportCheckRuleFollowsTheTurn(t *testing.T) {
	explanation := json.RawMessage(`{
	  "task_status": "completed",
	  "headline": "The retry loop never terminates when the queue is empty.",
	  "blocks": [{"type":"finding","id":"root-cause","summary":"drain() never returns on an empty queue.",
	    "impact":"Any call with an empty queue hangs the turn.","importance":"high"}]
	}`)

	tool, _ := newReportTool(t)
	if _, err := tool.Call(context.Background(), explanation); err != nil {
		t.Fatalf("a read-only turn's report was rejected: %v", err)
	}

	// A tool that could have changed something but only read, such as Shell
	// running ls, owes no check.
	tool, tc := newReportTool(t)
	tc.NoteMutatingTool()
	if _, err := tool.Call(context.Background(), explanation); err != nil {
		t.Fatalf("a turn that only ran a read-only command was asked for a check: %v", err)
	}

	tool, tc = newReportTool(t)
	tc.NoteChange()
	_, err := tool.Call(context.Background(), explanation)
	if err == nil || !strings.Contains(err.Error(), "status_needs_check") &&
		!strings.Contains(err.Error(), "how that was checked") {
		t.Errorf("a turn that changed something was not asked how it checked: %v", err)
	}
}

// jsonBlock pulls the first fenced JSON block out of some documentation.
func jsonBlock(t *testing.T, doc string) string {
	t.Helper()
	_, after, ok := strings.Cut(doc, "```json\n")
	if !ok {
		t.Fatal("no fenced JSON block in the documentation")
	}
	body, _, ok := strings.Cut(after, "\n```")
	if !ok {
		t.Fatal("the fenced JSON block is not closed")
	}
	return body
}

// The example a model is shown has to be a report the tool would accept.
// An example that fails its own validator teaches the wrong shape.
func TestTheDocumentedExamplesAreValid(t *testing.T) {
	tool, tc := newReportTool(t)
	tc.NoteMutatingTool()

	docs := map[string]string{"prompt section": tool.PromptSection()}
	tmpl, err := os.ReadFile(filepath.Join("..", "prompt", "prompts", "tools", "report.tmpl"))
	if err != nil {
		t.Fatalf("reading the template: %v", err)
	}
	docs["template"] = string(tmpl)

	for name, doc := range docs {
		example := jsonBlock(t, doc)
		var rep report.Report
		if err := json.Unmarshal([]byte(example), &rep); err != nil {
			t.Errorf("the %s example is not valid JSON: %v", name, err)
			continue
		}
		report.Normalize(&rep)
		if ps := report.Validate(&rep, report.Rules{Mutated: true}); len(ps) != 0 {
			t.Errorf("the %s example fails its own validator: %v", name, ps)
		}
	}
}

// The flag that decides whether a run uses reports is the only thing that
// registers the tool, so a run without it never sees Report in its tool list.
func TestReportToolIsRegisteredOnlyWhenEnabled(t *testing.T) {
	cfg := newTestConfig()
	if reg := SetupRegistry(SetupConfig{Cfg: cfg}); reg.Get("Report") != nil {
		t.Error("Report was registered with reports switched off")
	}
	cfg.Tools.Report.Enabled = true
	if reg := SetupRegistry(SetupConfig{Cfg: cfg}); reg.Get("Report") == nil {
		t.Error("Report was not registered with reports switched on")
	}
}

// The strong-profile prompt has to teach what the schema cannot: how the user
// reads the card, and that every obvious continuation becomes its own
// "next" block rather than prose in details.
func TestShortPromptTeachesNextBlocks(t *testing.T) {
	tool, _ := newReportTool(t)
	section := tool.ShortPromptSection()
	for _, want := range []string{
		`"next"`,
		"one per proposal",
		"almost never opens details",
		"needs_action",
	} {
		if !strings.Contains(section, want) {
			t.Errorf("short prompt does not mention %q:\n%s", want, section)
		}
	}
}

// A rejection names a buried prose proposal in one hint line, and only then:
// failing turns are not punished with extra text.
func TestRejectionHintsAtBuriedProposal(t *testing.T) {
	buried := `{
	  "task_status": "completed",
	  "headline": "Stale PATH line lists a directory that no longer exists.",
	  "blocks": [{"type": "finding", "id": "cause",
	    "summary": "Line 144 runs ls on a missing directory.",
	    "details": "How to fix (pick one): delete the line. Say the word and I will apply it."}]
	}`
	tool, _ := newReportTool(t)
	_, err := tool.Call(context.Background(), json.RawMessage(buried))
	if err == nil {
		t.Fatal("a finding without impact was accepted")
	}
	if !strings.Contains(err.Error(), `"next" block`) {
		t.Errorf("rejection names no next-block hint:\n%v", err)
	}

	plain := `{
	  "task_status": "completed",
	  "headline": "Stale PATH line lists a directory that no longer exists.",
	  "blocks": [{"type": "finding", "id": "cause",
	    "summary": "Line 144 runs ls on a missing directory.",
	    "details": "Checked the startup files; this is the only reference."}]
	}`
	tool, _ = newReportTool(t)
	_, err = tool.Call(context.Background(), json.RawMessage(plain))
	if err == nil {
		t.Fatal("a finding without impact was accepted")
	}
	if strings.Contains(err.Error(), "Hint:") {
		t.Errorf("a report with no proposal drew the hint:\n%v", err)
	}
}

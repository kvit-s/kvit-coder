package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/report"
)

// ReportTool ends a turn with a structured report instead of free prose.
//
// The model calls it with JSON in the shape internal/report describes. The tool
// normalizes what it gets, checks it, and either accepts it — which ends the
// turn — or returns every problem it found so the model can fix the whole
// report in one more call. Nothing else about the turn changes: the report is
// an ordinary tool call, so it reaches the session as one, and the front end
// reads it back from there between turns.
//
// docs/structured-reports.md is the design.
type ReportTool struct {
	cfg     *config.Config
	toolCtx *ToolContext
}

// NewReportTool builds the tool. The accepted report and the repair count live
// on the tool context, because the agent loop is what reads them.
func NewReportTool(cfg *config.Config, toolCtx *ToolContext) *ReportTool {
	return &ReportTool{cfg: cfg, toolCtx: toolCtx}
}

func (t *ReportTool) Name() string { return "Report" }

func (t *ReportTool) Description() string {
	return "End your turn with a structured report. Call it once, as the last thing you do: " +
		"the turn ends when the report is accepted, so anything you still have to do or say " +
		"belongs before it or in it. A report that does not fit the schema comes back with " +
		"every problem listed; fix them all and call again."
}

func (t *ReportTool) JSONSchema() map[string]any {
	return report.JSONSchema(t.maxBlocks())
}

func (t *ReportTool) maxBlocks() int {
	if t.cfg == nil || t.cfg.Tools.Report.MaxBlocks <= 0 {
		return report.DefaultMaxBlocks
	}
	return t.cfg.Tools.Report.MaxBlocks
}

func (t *ReportTool) maxRepairs() int {
	if t.cfg == nil || t.cfg.Tools.Report.MaxRepairAttempts <= 0 {
		return config.DefaultReportRepairAttempts
	}
	return t.cfg.Tools.Report.MaxRepairAttempts
}

func (t *ReportTool) PromptCategory() string     { return "report" }
func (t *ReportTool) PromptOrder() int           { return 10 }
func (t *ReportTool) PromptTemplateName() string { return "report" }

// Check does nothing. Everything a report can get wrong is checked in Call,
// because a Check error is treated as a misuse the loop may backtrack over,
// which would discard the report the model just wrote instead of handing back
// the list of things to fix.
func (t *ReportTool) Check(ctx context.Context, args json.RawMessage) error { return nil }

func (t *ReportTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var rep report.Report
	dec := json.NewDecoder(strings.NewReader(string(args)))
	if err := dec.Decode(&rep); err != nil {
		return nil, t.reject([]report.Problem{{
			Path: "/", Code: "arguments_invalid",
			Message: fmt.Sprintf("The arguments are not valid JSON for a report: %v", err),
		}})
	}

	report.Normalize(&rep)
	rules := report.Rules{MaxBlocks: t.maxBlocks(), Mutated: t.toolCtx.MutatedThisTurn()}
	if problems := report.Validate(&rep, rules); len(problems) > 0 {
		return nil, t.reject(problems)
	}

	t.toolCtx.AcceptReport(&rep)
	return map[string]any{
		"accepted":    true,
		"task_status": string(rep.TaskStatus),
		"blocks":      len(rep.Blocks),
		"note":        "Report accepted. The turn ends here — do not add anything after this.",
	}, nil
}

// reject builds the failure the model reads. It is a runtime error, not a
// semantic one: the point is for the problems to reach the conversation so the
// model can repair from them, and a semantic error is the kind the loop may
// discard along with the message that produced it.
func (t *ReportTool) reject(problems []report.Problem) error {
	attempt := t.toolCtx.RecordReportRepair()
	max := t.maxRepairs()
	msg := report.FormatProblems(problems)
	if attempt >= max {
		msg += fmt.Sprintf("\n\nThat was attempt %d of %d. Stop repairing the report: "+
			"answer in plain prose instead and end your turn.", attempt, max)
	} else {
		msg += fmt.Sprintf(" (attempt %d of %d)", attempt, max)
	}
	details := map[string]any{
		"report_rejected": true,
		"problems":        problems,
		"attempt":         attempt,
		"max_attempts":    max,
	}
	return RuntimeErrorWithDetails(msg, details)
}

// ShortPromptSection is what the strong profile gets: the schema says what the
// fields are, so this says only what the schema cannot — when to call it, that
// it ends the turn, and the two rules a model reliably gets wrong.
func (t *ReportTool) ShortPromptSection() string {
	return fmt.Sprintf(`### Report

End every turn that %s with one Report call, as the last thing you do: the turn
ends when the report is accepted, so say everything in the report rather than
after it. Do not call it twice.

A report is a status, a one-sentence headline, and typed blocks. Two rules the
schema cannot enforce for you:

- When the turn changed something, "completed" and "completed_with_notes" need
  a verification block saying how you checked it. If you ran nothing, say so
  with status "not_run" and a limitation, rather than putting an explanation
  in a verification block that checked nothing. A turn that only read and
  explained puts its answer in a finding and needs no verification.
- Any block that asks the user something — decision, question, blocked,
  next_step — makes the report "needs_action" or "blocked", offers %d to %d
  options, and says why you recommend the one you recommend. A "dispatch"
  option's instruction is shown to the user and sent verbatim as the next
  prompt if they pick it.

Fewer blocks read better: put related steps in one block's details rather than
one block each.`, t.modeSentence(), report.MinOptions, report.MaxOptions)
}

// modeSentence describes, in the model's terms, which turns owe a report.
func (t *ReportTool) modeSentence() string {
	switch t.cfg.Tools.Report.ResolvedMode() {
	case config.ReportModeAlways:
		return "you take"
	case config.ReportModeTools:
		return "uses any tool"
	case config.ReportModeOff:
		return "you choose to report on"
	default:
		return "changes a file, runs a command or starts a process"
	}
}

// PromptSection is the long form, used when the profile is not "strong". It is
// the short form plus a worked example, because a model that needs the long
// form needs to see the shape rather than read about it.
func (t *ReportTool) PromptSection() string {
	return t.ShortPromptSection() + `

` + reportExample
}

const reportExample = "```json\n" + `{
  "task_status": "needs_action",
  "headline": "Drag-and-drop ordering works; one filtered-view behaviour needs a decision.",
  "blocks": [
    {"type": "change", "id": "reorder-impl",
     "summary": "Added drag handles to the row list and persisted the new order.",
     "related_files": ["internal/ui/rows.go"]},
    {"type": "verification", "id": "reorder-tests",
     "summary": "14/14 relevant tests pass and ordering survives a restart.",
     "status": "passed", "evidence": "go test ./internal/store"},
    {"type": "decision", "id": "filtered-reordering",
     "summary": "Choose how reordering behaves while a filter hides some rows.",
     "options": [
       {"id": "preserve", "label": "Preserve hidden positions",
        "consequence": "Hidden rows keep their slots.",
        "effect": "dispatch",
        "instruction": "Implement reordering while preserving hidden-row positions."},
       {"id": "explain", "label": "Let me describe what I want", "effect": "collect"}
     ],
     "recommendation": "preserve",
     "recommendation_reason": "It disturbs the least unrelated state.",
     "response_type": "single"}
  ]
}` + "\n```"

package tui

import (
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/report"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

func transcriptReport() *report.Report {
	return &report.Report{
		TaskStatus: report.StatusNeedsAction,
		Headline:   "Ordering works; the filtered case needs a decision.",
		Blocks: []report.Block{
			{Type: report.BlockCheck, ID: "tests", Summary: "14 tests pass.",
				Status: report.CheckPassed, Details: "ran the suite twice"},
			{Type: report.BlockQuestion, ID: "filtered", Summary: "Choose the filtered behaviour.",
				Options: []report.Option{
					{ID: "preserve", Label: "Preserve hidden positions", Effect: report.EffectDispatch,
						Instruction: "Implement reordering while preserving hidden-row positions."},
					{ID: "stop", Label: "Leave it for now", Effect: report.EffectResolve},
				},
				Recommendation:       "preserve",
				RecommendationReason: "It disturbs the least unrelated state.",
				ResponseType:         report.ResponseSingle},
		},
	}
}

// The transcript is the expanded report, so details hidden on the card are
// shown in the scrollback copy.
func TestTranscriptExpandsEveryBlock(t *testing.T) {
	got := reportTranscript(transcriptReport(), nil)
	if !strings.Contains(got, "ran the suite twice") {
		t.Errorf("the transcript hid the details:\n%s", got)
	}
	if !strings.Contains(got, "Ordering works; the filtered case needs a decision.") {
		t.Errorf("the transcript lost the headline:\n%s", got)
	}
	for _, want := range []string{"previous turn, expanded", "tests", "filtered"} {
		if !strings.Contains(got, want) {
			t.Errorf("the transcript misses %q:\n%s", want, got)
		}
	}
}

// The transcript names what the user picked, so the scrollback says which
// option the next turn runs.
func TestTranscriptNamesThePick(t *testing.T) {
	ans := &ui.CardAnswer{
		BlockID: "filtered", OptionID: "preserve",
		Effect: report.EffectDispatch, Number: 1, Label: "Preserve hidden positions",
	}
	got := reportTranscript(transcriptReport(), ans)
	if !strings.Contains(got, "Preserve hidden positions") || !strings.Contains(got, "dispatch") {
		t.Errorf("the pick went missing:\n%s", got)
	}
	if !strings.Contains(got, "1)") || !strings.Contains(got, "filtered/preserve") {
		t.Errorf("the pick is not located:\n%s", got)
	}
}

// A resolve pick still prints: picking is the action even when no turn starts.
func TestTranscriptKeepsAResolvePick(t *testing.T) {
	ans := &ui.CardAnswer{
		BlockID: "filtered", OptionID: "stop",
		Effect: report.EffectResolve, Number: 2, Label: "Leave it for now",
	}
	got := reportTranscript(transcriptReport(), ans)
	if !strings.Contains(got, "resolve") || !strings.Contains(got, "Leave it for now") {
		t.Errorf("the resolve pick went missing:\n%s", got)
	}
}

func TestTranscriptWithoutAReportIsEmpty(t *testing.T) {
	if got := reportTranscript(nil, nil); got != "" {
		t.Errorf("transcript without a report is %q, want empty", got)
	}
}

package ui

import (
	"strings"
	"testing"
)

// The scrollback copy expands every block, so details folded on the card are
// still in the history.
func TestTranscriptExpandsEveryBlock(t *testing.T) {
	rep := decisionReport()
	rep.Blocks[0].Details = "ran the suite twice"
	got := TranscriptWidth(rep, nil, 78)
	if !strings.Contains(got, "ran the suite twice") {
		t.Errorf("the transcript hid the details:\n%s", got)
	}
	if !strings.Contains(got, "Ordering works; the filtered case needs a decision.") {
		t.Errorf("the transcript lost the headline:\n%s", got)
	}
}

// The scrollback copy wraps like the card, so a long field folds instead of
// running off the right-hand side. Plain text would leave it on one line.
func TestTranscriptWrapsLikeTheCard(t *testing.T) {
	got := TranscriptWidth(longReport(), nil, 60)
	if w := widest(got); w > 60 {
		t.Errorf("a transcript line is %d columns wide at width 60:\n%s", w, got)
	}
	flat := strings.Join(strings.Fields(got), " ")
	if !strings.Contains(flat, "none of it survives a crash or is visible to review.") {
		t.Errorf("the end of a wrapped field went missing:\n%s", got)
	}
}

// The scrollback copy keeps what was picked, so the history says which option
// the next turn runs.
func TestTranscriptKeepsThePick(t *testing.T) {
	m := press(withCard(decisionReport()), "1")
	got := TranscriptWidth(decisionReport(), m.CardAnswerPicked(), 78)
	for _, want := range []string{"1)", "Preserve hidden positions", "filtered/preserve", "dispatch"} {
		if !strings.Contains(got, want) {
			t.Errorf("the transcript misses %q:\n%s", want, got)
		}
	}
}

func TestTranscriptWithoutAReportIsEmpty(t *testing.T) {
	if got := Transcript(nil, nil); got != "" {
		t.Errorf("transcript without a report is %q, want empty", got)
	}
}

package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvit-s/kvit-coder/internal/report"
)

func enterKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyEnter} }

func pressEnter(m InputModel) InputModel {
	mod, _ := m.Update(enterKey())
	return mod.(InputModel)
}

// An empty Enter picks the highlighted recommendation, so the likeliest
// answer needs no digit.
func TestEnterPicksRecommendedDispatch(t *testing.T) {
	m := pressEnter(withCard(decisionReport()))
	if !m.Submitted() {
		t.Fatal("empty Enter did not submit the recommended dispatch option")
	}
	if m.Value() != "Implement reordering while preserving hidden-row positions." {
		t.Errorf("submitted %q, want the recommended instruction verbatim", m.Value())
	}
	ans := m.CardAnswerPicked()
	if ans == nil || ans.BlockID != "filtered" || ans.OptionID != "preserve" {
		t.Errorf("answer is %#v", ans)
	}
	if ans != nil && ans.Number != 1 {
		t.Errorf("number is %d, want 1 for the scrollback", ans.Number)
	}
}

// Typing first means the message is the prompt: Enter sends what was typed,
// not the recommendation.
func TestEnterWithTextDoesNotPickRecommended(t *testing.T) {
	m := press(withCard(decisionReport()), "f", "i", "x")
	m = pressEnter(m)
	if !m.Submitted() {
		t.Fatal("Enter with text did not submit")
	}
	if m.Value() != "fix" {
		t.Errorf("submitted %q, want the typed text", m.Value())
	}
	if ans := m.CardAnswerPicked(); ans != nil {
		t.Errorf("typing picked %#v, want no answer", ans)
	}
}

// With no recommendation there is nothing to pick: an empty Enter submits
// empty, which the driver treats as a no-op, exactly as before.
func TestEnterWithoutRecommendationSubmitsEmpty(t *testing.T) {
	rep := decisionReport()
	rep.Blocks[1].Recommendation = ""
	rep.Blocks[1].RecommendationReason = ""
	m := pressEnter(withCard(rep))
	if !m.Submitted() {
		t.Fatal("empty Enter did not submit")
	}
	if m.Value() != "" {
		t.Errorf("submitted %q, want empty", m.Value())
	}
	if ans := m.CardAnswerPicked(); ans != nil {
		t.Errorf("picked %#v with no recommendation", ans)
	}
}

// Staged images win over the recommendation: an empty Enter with attachments
// is an image-only turn, not a pick.
func TestEnterWithStagedImagesDoesNotPick(t *testing.T) {
	m := withCard(decisionReport())
	m.SetStagedImages([]string{"shot.png"})
	m = pressEnter(m)
	if !m.Submitted() {
		t.Fatal("empty Enter with images did not submit")
	}
	if m.Value() != "" {
		t.Errorf("submitted %q, want empty for an image-only turn", m.Value())
	}
	if ans := m.CardAnswerPicked(); ans != nil {
		t.Errorf("picked %#v despite staged images", ans)
	}
}

// A recommended collect stages its draft for editing, like its digit does:
// one Enter prepares the answer, a second sends it.
func TestEnterPicksRecommendedCollect(t *testing.T) {
	rep := decisionReport()
	rep.Blocks[1].Recommendation = "explain"
	m := pressEnter(withCard(rep))
	if m.Submitted() {
		t.Error("a collect pick submitted instead of staging a draft")
	}
	if got := m.textarea.Value(); !strings.Contains(got, "[report filtered]") || !strings.Contains(got, "Let me describe it") {
		t.Errorf("textarea = %q, want the block id and the label", got)
	}
	if ans := m.CardAnswerPicked(); ans == nil || ans.OptionID != "explain" {
		t.Errorf("answer is %#v", ans)
	}
}

// A recommended resolve records the pick and submits empty at once, so one
// Enter leaves the transcript and starts no turn. A digit needs a second
// Enter for the same, because there may be follow-up text to type.
func TestEnterPicksRecommendedResolve(t *testing.T) {
	rep := decisionReport()
	rep.Blocks[1].Recommendation = "stop"
	m := pressEnter(withCard(rep))
	if !m.Submitted() {
		t.Fatal("empty Enter on a recommended resolve did not submit")
	}
	if m.Value() != "" {
		t.Errorf("submitted %q, want empty for a resolve", m.Value())
	}
	if ans := m.CardAnswerPicked(); ans == nil || ans.Effect != report.EffectResolve {
		t.Errorf("answer is %#v", ans)
	}
}

// After esc dismisses the card, Enter is plain submit again: nothing is
// picked from a card the user put away.
func TestEnterAfterEscDismissDoesNotPick(t *testing.T) {
	m := press(withCard(decisionReport()), "esc")
	m = pressEnter(m)
	if !m.Submitted() {
		t.Fatal("Enter after esc did not submit")
	}
	if ans := m.CardAnswerPicked(); ans != nil {
		t.Errorf("picked %#v from a dismissed card", ans)
	}
}

// The footer names the Enter shortcut only when there is something to pick.
func TestFooterMentionsEnterForRecommended(t *testing.T) {
	if got := withCard(decisionReport()).View(); !strings.Contains(got, "Enter picks recommended") {
		t.Errorf("the footer does not offer Enter:\n%s", got)
	}
	rep := decisionReport()
	rep.Blocks[1].Recommendation = ""
	if got := withCard(rep).View(); strings.Contains(got, "Enter picks") {
		t.Errorf("the footer offered Enter with no recommendation:\n%s", got)
	}
}

// A recommendation past the ninth has no digit, but Enter still reaches it.
func TestEnterPicksRecommendedPastTheNinth(t *testing.T) {
	rep := decisionReport()
	// Nine dummy options ahead of the real decision: three blocks of three,
	// none recommended, so the real block's first option is tenth.
	rep.Blocks = []report.Block{rep.Blocks[0]}
	for i := 0; i < 3; i++ {
		id := "dummy" + string(rune('a'+i))
		rep.Blocks = append(rep.Blocks, report.Block{
			Type: report.BlockQuestion, ID: id,
			Summary: "Dummy.",
			Options: []report.Option{
				{ID: "a", Label: "A", Effect: report.EffectResolve},
				{ID: "b", Label: "B", Effect: report.EffectResolve},
				{ID: "c", Label: "C", Effect: report.EffectResolve},
			},
		})
	}
	real := decisionReport().Blocks[1]
	rep.Blocks = append(rep.Blocks, real)
	pc := rep.PrimaryChoice()
	if pc == nil {
		t.Fatal("no primary choice found")
	}
	if pc.Number != 0 {
		t.Fatalf("setup did not push the recommendation past the ninth (number %d)", pc.Number)
	}
	m := pressEnter(withCard(rep))
	if !m.Submitted() {
		t.Fatal("empty Enter did not pick the unnumbered recommendation")
	}
	if ans := m.CardAnswerPicked(); ans == nil || ans.OptionID != "preserve" {
		t.Errorf("answer is %#v", ans)
	}
}

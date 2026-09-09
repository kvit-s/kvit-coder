package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvit-s/kvit-coder/internal/report"
)

func key(s string) tea.KeyMsg {
	switch s {
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "alt+up":
		return tea.KeyMsg{Type: tea.KeyUp, Alt: true}
	case "alt+down":
		return tea.KeyMsg{Type: tea.KeyDown, Alt: true}
	}
	if strings.HasPrefix(s, "alt+") {
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s[4:]), Alt: true}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

func press(m InputModel, keys ...string) InputModel {
	for _, k := range keys {
		mod, _ := m.Update(key(k))
		m = mod.(InputModel)
	}
	return m
}

// decisionReport is a report with one thing to answer and one option of each
// effect, so a test can pick any of the three.
func decisionReport() *report.Report {
	return &report.Report{
		TaskStatus: report.StatusNeedsAction,
		Headline:   "Ordering works; the filtered case needs a decision.",
		Blocks: []report.Block{
			{Type: report.BlockVerification, ID: "tests", Summary: "14 tests pass.", Status: report.VerifyPassed,
				Details: "ran the suite twice"},
			{Type: report.BlockDecision, ID: "filtered", Summary: "Choose the filtered behaviour.",
				Options: []report.Option{
					{ID: "preserve", Label: "Preserve hidden positions", Effect: report.EffectDispatch,
						Instruction: "Implement reordering while preserving hidden-row positions."},
					{ID: "explain", Label: "Let me describe it", Effect: report.EffectCollect},
					{ID: "stop", Label: "Leave it for now", Effect: report.EffectResolve},
				},
				Recommendation:       "preserve",
				RecommendationReason: "It disturbs the least unrelated state.",
				ResponseType:         report.ResponseSingle},
		},
	}
}

func doneReport() *report.Report {
	return &report.Report{
		TaskStatus: report.StatusCompleted,
		Headline:   "Renamed the helper.",
		Blocks: []report.Block{{
			Type: report.BlockVerification, ID: "tests", Summary: "14 tests pass.", Status: report.VerifyPassed,
		}},
	}
}

func withCard(rep *report.Report) InputModel {
	m := NewInputModel(">", nil)
	m.SetReport(rep)
	return m
}

func TestNoReportLeavesTheComposerAsItWas(t *testing.T) {
	m := withCard(nil)
	if view := m.View(); strings.Contains(view, "DONE") {
		t.Errorf("a card was drawn with no report:\n%s", view)
	}
	m = press(m, "1")
	if got := m.textarea.Value(); got != "1" {
		t.Errorf("a digit was swallowed with no card: textarea = %q", got)
	}
}

func TestCardIsDrawnAboveThePrompt(t *testing.T) {
	view := withCard(decisionReport()).View()
	if !strings.Contains(view, "NEEDS DECISION") {
		t.Errorf("the status chip is missing:\n%s", view)
	}
	if !strings.Contains(view, "Preserve hidden positions") {
		t.Errorf("the options are missing:\n%s", view)
	}
	if !strings.Contains(view, "Implement reordering while preserving") {
		t.Errorf("the instruction is not shown in full:\n%s", view)
	}
	if !strings.Contains(view, "1-3 answer") {
		t.Errorf("the key hint is missing:\n%s", view)
	}
}

// The point of the card: one keypress answers a decision and starts the turn
// with the instruction the user just read.
func TestDispatchOptionSubmitsItsInstruction(t *testing.T) {
	m := press(withCard(decisionReport()), "1")
	if !m.Submitted() {
		t.Fatal("picking a dispatch option did not submit")
	}
	if m.Value() != "Implement reordering while preserving hidden-row positions." {
		t.Errorf("submitted %q, want the option's instruction verbatim", m.Value())
	}
	ans := m.CardAnswerPicked()
	if ans == nil || ans.BlockID != "filtered" || ans.OptionID != "preserve" {
		t.Errorf("answer is %#v", ans)
	}
}

// A collect option is a draft, not a send: it seeds the composer, names the
// block it answers, and leaves the user to finish the sentence.
func TestCollectOptionSeedsTheComposer(t *testing.T) {
	m := press(withCard(decisionReport()), "2")
	if m.Submitted() {
		t.Error("a collect option submitted instead of staging an answer")
	}
	if got := m.textarea.Value(); !strings.Contains(got, "[report filtered]") || !strings.Contains(got, "Let me describe it") {
		t.Errorf("textarea = %q, want the block id and the label", got)
	}
	if strings.Contains(m.View(), "NEEDS DECISION") {
		t.Error("the card stayed up after an answer was staged")
	}
}

func TestResolveOptionStartsNothing(t *testing.T) {
	m := press(withCard(decisionReport()), "3")
	if m.Submitted() {
		t.Error("a resolve option started a turn")
	}
	if m.textarea.Value() != "" {
		t.Errorf("a resolve option wrote %q into the composer", m.textarea.Value())
	}
	if ans := m.CardAnswerPicked(); ans == nil || ans.Effect != report.EffectResolve {
		t.Errorf("answer is %#v", ans)
	}
}

func TestDigitsOnlyActWhileTheTextIsEmpty(t *testing.T) {
	m := press(withCard(decisionReport()), "w", "1")
	if m.Submitted() {
		t.Error("a digit answered the card while a message was being typed")
	}
	if got := m.textarea.Value(); got != "w1" {
		t.Errorf("textarea = %q, want the digit typed", got)
	}
}

func TestUnusedDigitFallsThroughToTheComposer(t *testing.T) {
	m := press(withCard(decisionReport()), "9")
	if m.Submitted() {
		t.Error("a digit with no option behind it submitted")
	}
	if got := m.textarea.Value(); got != "9" {
		t.Errorf("textarea = %q, want the digit typed", got)
	}
}

func TestEscDismissesTheCardAndLettersStillType(t *testing.T) {
	m := press(withCard(decisionReport()), "esc")
	if strings.Contains(m.View(), "NEEDS DECISION") {
		t.Error("esc did not dismiss the card")
	}
	m = press(m, "d", "o")
	if got := m.textarea.Value(); got != "do" {
		t.Errorf("textarea = %q; a dismissed card is still eating keys", got)
	}
}

// No bare letter is claimed, so a message starting with any word can be typed
// with the card up.
func TestCardClaimsNoBareLetters(t *testing.T) {
	m := press(withCard(decisionReport()), "d", "i", "f", "f")
	if got := m.textarea.Value(); got != "diff" {
		t.Errorf("textarea = %q, want \"diff\"", got)
	}
}

func TestAltDTogglesDetails(t *testing.T) {
	m := withCard(decisionReport())
	if strings.Contains(m.View(), "ran the suite twice") {
		t.Error("details were shown before being asked for")
	}
	// The decision is selected first, being what needs attention; the block
	// with details is the one below it.
	m = press(m, "alt+down", "alt+d")
	if !strings.Contains(m.View(), "ran the suite twice") {
		t.Errorf("alt+d did not show the details:\n%s", m.View())
	}
	m = press(m, "alt+d")
	if strings.Contains(m.View(), "ran the suite twice") {
		t.Error("alt+d did not hide the details again")
	}
}

// Moving the selection moves which block a digit answers, so a report with two
// questions can answer either.
func TestAltArrowsMoveTheSelection(t *testing.T) {
	rep := decisionReport()
	second := rep.Blocks[1]
	second.ID = "second"
	second.Options = []report.Option{
		{ID: "yes", Label: "Yes", Effect: report.EffectDispatch, Instruction: "Do the second thing."},
		{ID: "no", Label: "No", Effect: report.EffectResolve},
	}
	rep.Blocks = append(rep.Blocks, second)

	m := withCard(rep)
	if b := m.activeBlock(); b == nil || b.ID != "filtered" {
		t.Fatalf("the first decision is not active: %v", b)
	}
	// Display order puts the two decisions first, so one step down selects
	// the second of them.
	m = press(m, "alt+down")
	if b := m.activeBlock(); b == nil || b.ID != "second" {
		t.Fatalf("alt+down did not move the active block: %v", b)
	}
	m = press(m, "1")
	if m.Value() != "Do the second thing." {
		t.Errorf("submitted %q, want the selected block's option", m.Value())
	}
}

// An unanswered decision holds back an inbox-only turn; a report with nothing
// to answer does not, so a finished build never waits behind a card that only
// needs reading.
func TestOnlyAnInteractiveCardHoldsTheInboxTurn(t *testing.T) {
	asking := withCard(decisionReport())
	if !asking.cardHolds() {
		t.Error("a report with a decision did not hold the inbox turn")
	}
	done := withCard(doneReport())
	if done.cardHolds() {
		t.Error("a report with nothing to answer held the inbox turn")
	}
	none := withCard(nil)
	if none.cardHolds() {
		t.Error("no report at all held the inbox turn")
	}
	dismissed := press(asking, "esc")
	if dismissed.cardHolds() {
		t.Error("esc did not lift the hold")
	}
}

func TestWakeDoesNotFireUnderAnUnansweredDecision(t *testing.T) {
	m := withCard(decisionReport())
	m.SetWakePoll(func() (int, string) { return 2, "build finished" })
	mod, _ := m.Update(wakeTickMsg{})
	m = mod.(InputModel)
	if m.WakeFired() {
		t.Error("the composer submitted itself out from under an unanswered decision")
	}
	if !strings.Contains(m.View(), "inbox: 2 pending") {
		t.Errorf("the held inbox is not shown:\n%s", m.View())
	}

	// Dismissing the card lets the held turn through.
	m = press(m, "esc")
	mod, _ = m.Update(wakeTickMsg{})
	m = mod.(InputModel)
	if !m.WakeFired() {
		t.Error("the inbox turn stayed held after the card was dismissed")
	}
}

func TestWakeStillFiresUnderAReportWithNothingToAnswer(t *testing.T) {
	m := withCard(doneReport())
	m.SetWakePoll(func() (int, string) { return 1, "build finished" })
	mod, _ := m.Update(wakeTickMsg{})
	if !mod.(InputModel).WakeFired() {
		t.Error("a finished build waited behind a card that only needed reading")
	}
}

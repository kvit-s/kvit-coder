package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvit-s/kvit-coder/internal/report"
)

func key(s string) tea.KeyMsg {
	if s == "esc" {
		return tea.KeyMsg{Type: tea.KeyEsc}
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
			{Type: report.BlockCheck, ID: "tests", Summary: "14 tests pass.", Status: report.CheckPassed,
				Details: "ran the suite twice"},
			{Type: report.BlockQuestion, ID: "filtered", Summary: "Choose the filtered behaviour.",
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
			Type: report.BlockCheck, ID: "tests", Summary: "14 tests pass.", Status: report.CheckPassed,
		}},
	}
}

// notRunReport is the docs-only turn: a change plus a check that ran nothing.
func notRunReport() *report.Report {
	return &report.Report{
		TaskStatus: report.StatusCompleted,
		Headline:   "Added docs/screen.png to README.md.",
		Blocks: []report.Block{
			{Type: report.BlockChange, ID: "add-screenshot", Summary: "Added docs/screen.png to README.md."},
			{Type: report.BlockCheck, ID: "no-check", Summary: "No build or test run for a docs-only image link.",
				Status: report.CheckNotRun, Limitation: "Markdown render not previewed."},
		},
	}
}

func withCard(rep *report.Report) InputModel {
	m := NewInputModel(">", nil)
	m.SetReport(rep)
	return m
}

func TestNoReportLeavesTheComposerAsItWas(t *testing.T) {
	m := withCard(nil)
	if view := m.View(); view != NewInputModel(">", nil).View() {
		t.Errorf("a card was drawn with no report:\n%s", view)
	}
	m = press(m, "1")
	if got := m.textarea.Value(); got != "1" {
		t.Errorf("a digit was swallowed with no card: textarea = %q", got)
	}
}

func TestCardIsDrawnAboveThePrompt(t *testing.T) {
	view := withCard(decisionReport()).View()
	if !strings.Contains(view, "Ordering works; the filtered case needs a decision.") {
		t.Errorf("the headline is missing:\n%s", view)
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
	if strings.Contains(m.View(), "Ordering works; the filtered case") {
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
	if strings.Contains(m.View(), "Ordering works; the filtered case") {
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

// A digit that no option claims opens every block's details, so there is one
// details key that needs no alt modifier and no selection.
func TestZeroTogglesAllDetails(t *testing.T) {
	rep := decisionReport()
	rep.Blocks[1].Details = "the filter hides rows 3 and 7"
	m := withCard(rep)

	m = press(m, "0")
	view := m.View()
	if !strings.Contains(view, "ran the suite twice") || !strings.Contains(view, "hides rows 3 and 7") {
		t.Errorf("0 did not open every block's details:\n%s", view)
	}
	m = press(m, "0")
	if strings.Contains(m.View(), "ran the suite twice") {
		t.Error("0 did not close them again")
	}
}

// With nothing to expand, the footer does not advertise the keys that would
// expand it, and 0 stays an ordinary character.
func TestNoDetailsKeysWhenThereAreNoDetails(t *testing.T) {
	m := withCard(doneReport())
	if strings.Contains(m.View(), "details") {
		t.Errorf("the footer offered a details key with nothing to show:\n%s", m.View())
	}
	m = press(m, "0")
	if got := m.textarea.Value(); got != "0" {
		t.Errorf("textarea = %q, want the digit typed", got)
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

// Every option is numbered continuously across blocks, so a digit reaches a
// second question without a selection to move first. Nothing on the card needs
// a modifier key: alt combinations do not reach the app in every terminal.
func TestDigitsReachEveryBlocksOptions(t *testing.T) {
	rep := decisionReport()
	second := rep.Blocks[1]
	second.ID = "second"
	second.Options = []report.Option{
		{ID: "yes", Label: "Yes", Effect: report.EffectDispatch, Instruction: "Do the second thing."},
		{ID: "no", Label: "No", Effect: report.EffectResolve},
	}
	rep.Blocks = append(rep.Blocks, second)

	// Three options on the first decision, then two on the second.
	if got := len(rep.Choices()); got != 5 {
		t.Fatalf("the report offers %d numbered choices, want 5", got)
	}
	view := withCard(rep).View()
	if !strings.Contains(view, "4) Yes") {
		t.Errorf("the second block's options were not numbered on from the first:\n%s", view)
	}
	if !strings.Contains(view, "1-5 answer") {
		t.Errorf("the footer does not offer every option:\n%s", view)
	}

	m := press(withCard(rep), "4")
	if m.Value() != "Do the second thing." {
		t.Errorf("submitted %q, want the second block's first option", m.Value())
	}
}

// A tenth option has no key, so it is shown without a number rather than with
// one that does nothing.
func TestOptionsPastTheNinthAreNotNumbered(t *testing.T) {
	rep := decisionReport()
	for i := 0; i < 3; i++ {
		b := rep.Blocks[1]
		b.ID = "extra-" + string(rune('a'+i))
		rep.Blocks = append(rep.Blocks, b)
	}
	numbered := 0
	for _, c := range rep.Choices() {
		if c.Number > 0 {
			numbered++
		}
	}
	if numbered != report.MaxChoices {
		t.Errorf("%d options got a key, want %d", numbered, report.MaxChoices)
	}
}

// longReport has fields that overrun any sensible terminal, which is what the
// card has to fold rather than push off the right-hand side.
func longReport() *report.Report {
	return &report.Report{
		TaskStatus: report.StatusCompletedWithNotes,
		Headline:   "Committed the tools-stats caveat; left the rest of the working tree uncommitted for now.",
		Blocks: []report.Block{{
			Type: report.BlockWarning, ID: "dirty-tree",
			Summary:  "The rest of the working tree is still dirty, with 28 modified files and 12 untracked ones.",
			Severity: "medium",
			Impact:   "The uncommitted work spans internal/report, the agent runner, the Report tool, the front-end card and the config, and none of it survives a crash or is visible to review.",
		}},
	}
}

func widest(text string) int {
	w := 0
	for _, line := range strings.Split(text, "\n") {
		if n := len([]rune(line)); n > w {
			w = n
		}
	}
	return w
}

func TestCardWrapsToTheTerminal(t *testing.T) {
	m := withCard(longReport())
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 90, Height: 40})
	m = mod.(InputModel)

	view := m.cardView()
	if got := widest(view); got > 90 {
		t.Errorf("a card line is %d columns wide in a 90-column terminal:\n%s", got, view)
	}
	// Folding must not lose anything.
	flat := strings.Join(strings.Fields(view), " ")
	if !strings.Contains(flat, "none of it survives a crash or is visible to review.") {
		t.Errorf("the end of a wrapped field went missing:\n%s", view)
	}
}

// A narrower terminal folds more, and still nothing overruns.
func TestCardWrapsAgainWhenTheTerminalNarrows(t *testing.T) {
	m := withCard(longReport())
	mod, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	wide := mod.(InputModel)
	mod, _ = wide.Update(tea.WindowSizeMsg{Width: 60, Height: 40})
	narrow := mod.(InputModel)

	if got := widest(narrow.cardView()); got > 60 {
		t.Errorf("a card line is %d columns wide in a 60-column terminal", got)
	}
	if len(strings.Split(narrow.cardView(), "\n")) <= len(strings.Split(wide.cardView(), "\n")) {
		t.Error("narrowing the terminal did not fold the card further")
	}
}

// Continuations hang under the field rather than returning to the margin, so a
// folded Impact still reads as one field and not as a new block.
func TestWrappedContinuationsHangUnderTheirField(t *testing.T) {
	lines := wrapCardLine("    Impact: "+strings.Repeat("word ", 40), 60)
	if len(lines) < 2 {
		t.Fatalf("nothing wrapped: %v", lines)
	}
	for _, l := range lines[1:] {
		if !strings.HasPrefix(l, "      ") {
			t.Errorf("continuation %q does not hang under its field", l)
		}
	}
}

// The scrollback copy says what was picked without looking the report up
// again, so the answer records the digit and the label behind it.
func TestPickedAnswerKeepsItsNumberAndLabel(t *testing.T) {
	m := press(withCard(decisionReport()), "1")
	ans := m.CardAnswerPicked()
	if ans == nil {
		t.Fatal("no answer recorded")
	}
	if ans.Number != 1 {
		t.Errorf("number is %d, want 1", ans.Number)
	}
	if ans.Label != "Preserve hidden positions" {
		t.Errorf("label is %q", ans.Label)
	}
	if ans.BlockID != "filtered" || ans.OptionID != "preserve" {
		t.Errorf("answer is %#v", ans)
	}

	m = press(withCard(decisionReport()), "2")
	if ans := m.CardAnswerPicked(); ans == nil || ans.Number != 2 || ans.Label != "Let me describe it" {
		t.Errorf("collect answer is %#v", ans)
	}

	m = press(withCard(decisionReport()), "3")
	if ans := m.CardAnswerPicked(); ans == nil || ans.Number != 3 || ans.Label != "Leave it for now" {
		t.Errorf("resolve answer is %#v", ans)
	}
}

func TestAnswerDescribeNamesThePickAndItsEffect(t *testing.T) {
	m := press(withCard(decisionReport()), "1")
	got := m.CardAnswerPicked().Describe()
	for _, want := range []string{"1)", "Preserve hidden positions", "filtered/preserve", "dispatch"} {
		if !strings.Contains(got, want) {
			t.Errorf("dispatch describe %q misses %q", got, want)
		}
	}

	m = press(withCard(decisionReport()), "2")
	if got := m.CardAnswerPicked().Describe(); !strings.Contains(got, "collect") {
		t.Errorf("collect describe %q misses its effect", got)
	}

	m = press(withCard(decisionReport()), "3")
	if got := m.CardAnswerPicked().Describe(); !strings.Contains(got, "resolve") {
		t.Errorf("resolve describe %q misses its effect", got)
	}

	if got := (*CardAnswer)(nil).Describe(); got != "" {
		t.Errorf("nil describe is %q, want empty", got)
	}
}

// The live card collapses a not_run check to a footnote instead of spending a
// block on saying nothing ran.
func TestCardCollapsesNotRunCheck(t *testing.T) {
	view := withCard(notRunReport()).View()
	if strings.Contains(view, "Check [no-check]") {
		t.Errorf("the card showed the collapsed block:\n%s", view)
	}
	if !strings.Contains(view, "1 check not run (hidden)") {
		t.Errorf("the footnote is missing:\n%s", view)
	}
	if !strings.Contains(view, "Change [add-screenshot]") {
		t.Errorf("the change block went missing with the check:\n%s", view)
	}
}

// A check that ran still shows in full on the card.
func TestCardKeepsPassedCheck(t *testing.T) {
	view := withCard(doneReport()).View()
	if !strings.Contains(view, "Check [tests]") {
		t.Errorf("a passed check was collapsed:\n%s", view)
	}
	if strings.Contains(view, "not run (hidden)") {
		t.Errorf("the footnote appeared with nothing hidden:\n%s", view)
	}
}

// 0 reveals the collapsed check, and 0 again hides it.
func TestZeroRevealsCollapsedNotRun(t *testing.T) {
	m := withCard(notRunReport())
	if !strings.Contains(m.View(), "0 details") {
		t.Fatalf("the footer does not offer 0:\n%s", m.View())
	}
	m = press(m, "0")
	if got := m.View(); !strings.Contains(got, "Check [no-check]") {
		t.Errorf("0 did not reveal the collapsed check:\n%s", got)
	}
	m = press(m, "0")
	if got := m.View(); strings.Contains(got, "Check [no-check]") {
		t.Errorf("0 did not hide the check again:\n%s", got)
	}
}

// The scrollback transcript keeps everything: it is the full record, the card
// is the quiet one.
func TestTranscriptKeepsNotRunInFull(t *testing.T) {
	got := Transcript(notRunReport(), nil)
	if !strings.Contains(got, "Check [no-check]") {
		t.Errorf("the transcript collapsed the block:\n%s", got)
	}
	if strings.Contains(got, "not run (hidden)") {
		t.Errorf("the footnote leaked into the transcript:\n%s", got)
	}
}

package report

import (
	"strings"
	"testing"
)

func blockIDsInOrder(r *Report) []string {
	out := []string{}
	for _, i := range r.Order() {
		out = append(out, r.Blocks[i].ID)
	}
	return out
}

// Blocks are shown in a fixed display order, not by the order the model
// happened to send them: interactive blocks first, then warning, change,
// finding, check, and unclassified last.
func TestOrderPutsWhatStopsTheWorkFirst(t *testing.T) {
	r := &Report{
		TaskStatus: StatusBlocked,
		Headline:   "Cannot continue without credentials.",
		Blocks: []Block{
			{Type: BlockChange, ID: "edit"},
			{Type: BlockCheck, ID: "tests"},
			{Type: BlockFinding, ID: "cause"},
			{Type: BlockQuestion, ID: "no-token"},
			{Type: BlockWarning, ID: "risk"},
		},
	}
	got := blockIDsInOrder(r)
	want := []string{"no-token", "risk", "edit", "cause", "tests"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order is %v, want %v", got, want)
		}
	}
}

// The model may send blocks in any order; the card shows them in the fixed
// display order: question, next, warning, change, finding, check,
// unclassified.
func TestOrderFollowsFixedDisplayOrder(t *testing.T) {
	r := &Report{
		TaskStatus: StatusNeedsAction,
		Headline:   "Scrambled submission order.",
		Blocks: []Block{
			{Type: BlockUnclassified, ID: "note"},
			{Type: BlockCheck, ID: "tests"},
			{Type: BlockFinding, ID: "cause"},
			{Type: BlockChange, ID: "edit"},
			{Type: BlockWarning, ID: "risk"},
			{Type: BlockNext, ID: "step"},
			{Type: BlockQuestion, ID: "ask"},
		},
	}
	got := blockIDsInOrder(r)
	want := []string{"ask", "step", "risk", "edit", "cause", "tests", "note"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order is %v, want %v", got, want)
		}
	}
}

func TestOrderSortsWarningsBySeverity(t *testing.T) {
	r := &Report{Blocks: []Block{
		{Type: BlockWarning, ID: "low-one", Severity: "low"},
		{Type: BlockWarning, ID: "critical-one", Severity: "critical"},
		{Type: BlockWarning, ID: "medium-one", Severity: "medium"},
	}}
	got := blockIDsInOrder(r)
	want := []string{"critical-one", "medium-one", "low-one"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order is %v, want %v", got, want)
		}
	}
}

// Blocks that tie keep the order they arrived in, so one report always renders
// the same way.
func TestOrderIsStableWithinAType(t *testing.T) {
	r := &Report{Blocks: []Block{
		{Type: BlockChange, ID: "first"},
		{Type: BlockChange, ID: "second"},
		{Type: BlockChange, ID: "third"},
	}}
	got := blockIDsInOrder(r)
	want := []string{"first", "second", "third"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order is %v, want %v", got, want)
		}
	}
}

func TestCheckGlyphSaysHowItWent(t *testing.T) {
	for status, glyph := range map[string]string{
		CheckPassed:  "✓",
		CheckFailed:  "✗",
		CheckPartial: "◐",
		CheckNotRun:  "○",
	} {
		b := Block{Type: BlockCheck, Status: status}
		if got := b.Glyph(); got != glyph {
			t.Errorf("%s renders %q, want %q", status, got, glyph)
		}
	}
}

// The first line is the headline alone: the status is not drawn.
func TestHeaderIsTheHeadline(t *testing.T) {
	r := completedReport()
	first := Render(r, Options{})
	line := strings.SplitN(first, "\n", 2)[0]
	if line != "Renamed the ordering helper and updated its callers." {
		t.Errorf("header is %q", line)
	}
}

func TestDetailsStayHiddenUntilExpanded(t *testing.T) {
	r := completedReport()
	r.Blocks[0].Details = "ran go test ./... twice"

	hidden := Render(r, Options{})
	if strings.Contains(hidden, "ran go test") {
		t.Error("details were shown without being expanded")
	}
	if !strings.Contains(hidden, "bytes of detail") {
		t.Errorf("no hint that details exist:\n%s", hidden)
	}

	shown := Render(r, Options{Expanded: map[string]bool{"unit-tests": true}})
	if !strings.Contains(shown, "ran go test") {
		t.Errorf("expanded details were not shown:\n%s", shown)
	}
	if strings.Contains(shown, "bytes of detail") {
		t.Error("the hint stayed after the details were shown")
	}
}

func TestPlainTextShowsEverything(t *testing.T) {
	r := completedReport()
	r.Blocks[0].Details = "ran go test ./... twice"
	if !strings.Contains(r.PlainText(), "ran go test") {
		t.Error("PlainText hid the details")
	}
}

// A dispatch option's instruction is what picking it sends, so the user reads
// it in full before choosing.
func TestDispatchInstructionIsShownInFull(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	r.Blocks = append(r.Blocks, questionBlock())

	text := Render(r, Options{})
	if !strings.Contains(text, "Implement reordering while preserving hidden-row positions.") {
		t.Errorf("the instruction was not shown:\n%s", text)
	}
}

// Options are numbered continuously across every block that asks something, so
// one digit reaches any of them.
func TestOptionsAreNumberedAcrossBlocks(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	first := questionBlock()
	second := questionBlock()
	second.ID = "second-question"
	r.Blocks = append(r.Blocks, first, second)

	numbers := map[string][]int{}
	for _, l := range Lines(r, Options{}) {
		if l.Kind == LineOption && l.Option > 0 {
			numbers[l.BlockID] = append(numbers[l.BlockID], l.Option)
		}
	}
	if len(numbers["filtered-reordering"]) == 0 || len(numbers["second-question"]) == 0 {
		t.Fatalf("not every block was numbered: %v", numbers)
	}
	// The second block picks up where the first left off.
	if numbers["second-question"][0] <= numbers["filtered-reordering"][0] {
		t.Errorf("numbering restarted on the second block: %v", numbers)
	}
	if c := r.Choice(3); c == nil || r.Blocks[c.Block].ID != "second-question" {
		t.Errorf("digit 3 does not reach the second block: %v", c)
	}
}

func TestRecommendationIsNamedOrItsAbsenceExplained(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	b := questionBlock()
	r.Blocks = append(r.Blocks, b)
	text := Render(r, Options{})
	if !strings.Contains(text, "Preserve hidden positions (recommended)") {
		t.Errorf("the recommended option was not marked:\n%s", text)
	}
	if !strings.Contains(text, "Why: It disturbs the least unrelated state.") {
		t.Errorf("the reason was not shown under the recommended option:\n%s", text)
	}
	if strings.Contains(text, "Recommended:") {
		t.Errorf("the recommendation was named twice:\n%s", text)
	}

	r.Blocks[1].Summary = "Which database?"
	r.Blocks[1].Recommendation = ""
	if !strings.Contains(Render(r, Options{}), "No recommendation:") {
		t.Error("a question with no recommendation did not say so")
	}
}

func TestFirstInteractiveSkipsBlocksThatAskNothing(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	r.Blocks = append(r.Blocks, questionBlock())
	i := r.FirstInteractive()
	if i < 0 || r.Blocks[i].ID != "filtered-reordering" {
		t.Errorf("FirstInteractive returned %d", i)
	}

	if completedReport().FirstInteractive() != -1 {
		t.Error("a report with nothing to answer reported an interactive block")
	}
}

func TestNilReportRendersNothing(t *testing.T) {
	var r *Report
	if got := Render(r, Options{}); got != "" {
		t.Errorf("a nil report rendered %q", got)
	}
	if r.HasInteractive() {
		t.Error("a nil report claimed to ask something")
	}
}

// notRunReport is the docs-only turn from the card-noise complaint: a change
// plus a check that ran nothing.
func notRunReport() *Report {
	return &Report{
		TaskStatus: StatusCompleted,
		Headline:   "Added docs/screen.png to README.md.",
		Blocks: []Block{
			{Type: BlockChange, ID: "add-screenshot", Summary: "Added docs/screen.png to README.md."},
			{Type: BlockCheck, ID: "no-check", Summary: "No build or test run for a docs-only image link.",
				Status: CheckNotRun, Limitation: "Markdown render not previewed."},
		},
	}
}

// Without the flag nothing collapses: the old layout is untouched.
func TestNotRunShowsInFullByDefault(t *testing.T) {
	text := Render(notRunReport(), Options{})
	if !strings.Contains(text, "Check [no-check]") {
		t.Errorf("the not_run block went missing without HideNotRun:\n%s", text)
	}
	if strings.Contains(text, "not run (hidden)") {
		t.Errorf("the footnote appeared without HideNotRun:\n%s", text)
	}
}

// With the flag the block collapses to one dim footnote.
func TestHideNotRunCollapsesToFootnote(t *testing.T) {
	text := Render(notRunReport(), Options{HideNotRun: true})
	if strings.Contains(text, "Check [no-check]") {
		t.Errorf("the not_run block was shown despite HideNotRun:\n%s", text)
	}
	if !strings.Contains(text, "1 check not run (hidden)") {
		t.Errorf("the footnote is missing:\n%s", text)
	}
	if !strings.Contains(text, "Change [add-screenshot]") {
		t.Errorf("the change block went missing with the check:\n%s", text)
	}
}

// Two collapsed checks share one footnote line.
func TestHideNotRunCountsEveryCheck(t *testing.T) {
	r := notRunReport()
	r.Blocks = append(r.Blocks, Block{Type: BlockCheck, ID: "no-check-2",
		Summary: "No second check either.", Status: CheckNotRun, Limitation: "Nothing ran."})
	text := Render(r, Options{HideNotRun: true})
	if !strings.Contains(text, "2 checks not run (hidden)") {
		t.Errorf("the footnote did not count both checks:\n%s", text)
	}
}

// A check that actually ran is never collapsed.
func TestHideNotRunLeavesPassedAlone(t *testing.T) {
	text := Render(completedReport(), Options{HideNotRun: true})
	if !strings.Contains(text, "Check [unit-tests]") {
		t.Errorf("a passed check was collapsed:\n%s", text)
	}
	if strings.Contains(text, "not run (hidden)") {
		t.Errorf("the footnote appeared with nothing hidden:\n%s", text)
	}
}

// The transcript and headless output expand everything, so they never collapse.
func TestHideNotRunExpandAllShowsFull(t *testing.T) {
	r := notRunReport()
	text := Render(r, Options{HideNotRun: true, ExpandAll: true})
	if !strings.Contains(text, "Check [no-check]") {
		t.Errorf("ExpandAll still collapsed the block:\n%s", text)
	}
	if strings.Contains(text, "not run (hidden)") {
		t.Errorf("the footnote appeared with everything expanded:\n%s", text)
	}
	if got := r.HiddenNotRunCount(Options{HideNotRun: true, ExpandAll: true}); got != 0 {
		t.Errorf("HiddenNotRunCount with ExpandAll is %d, want 0", got)
	}
}

// One expanded block shows in full while its siblings stay collapsed.
func TestHideNotRunExpandedBlockShowsFull(t *testing.T) {
	r := notRunReport()
	opts := Options{HideNotRun: true, Expanded: map[string]bool{"no-check": true}}
	text := Render(r, opts)
	if !strings.Contains(text, "Check [no-check]") {
		t.Errorf("the expanded block stayed hidden:\n%s", text)
	}
	if strings.Contains(text, "not run (hidden)") {
		t.Errorf("the footnote stayed after the block was expanded:\n%s", text)
	}
	if got := r.HiddenNotRunCount(opts); got != 0 {
		t.Errorf("HiddenNotRunCount with the block expanded is %d, want 0", got)
	}
}

func TestHiddenNotRunCount(t *testing.T) {
	r := notRunReport()
	if got := r.HiddenNotRunCount(Options{HideNotRun: true}); got != 1 {
		t.Errorf("HiddenNotRunCount is %d, want 1", got)
	}
	if got := r.HiddenNotRunCount(Options{}); got != 0 {
		t.Errorf("HiddenNotRunCount without the flag is %d, want 0", got)
	}
	if got := completedReport().HiddenNotRunCount(Options{HideNotRun: true}); got != 0 {
		t.Errorf("HiddenNotRunCount on a passed check is %d, want 0", got)
	}
}

// An option's description and consequence share its line.
func TestOptionTextIsOneLine(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	b := questionBlock()
	b.Options[1].Description = "Push hidden rows to the end"
	b.Options[1].Consequence = "Their old slots are lost."
	r.Blocks = append(r.Blocks, b)
	text := Render(r, Options{})
	if !strings.Contains(text, "Recompute the ordering — Push hidden rows to the end. Their old slots are lost.") {
		t.Errorf("description and consequence were not joined on the option line:\n%s", text)
	}
}

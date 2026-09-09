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

// Blocks are shown by how much they need attention, not by the order the model
// happened to send them.
func TestOrderPutsWhatStopsTheWorkFirst(t *testing.T) {
	r := &Report{
		TaskStatus: StatusBlocked,
		Headline:   "Cannot continue without credentials.",
		Blocks: []Block{
			{Type: BlockChange, ID: "edit"},
			{Type: BlockVerification, ID: "tests"},
			{Type: BlockFinding, ID: "cause"},
			{Type: BlockBlocked, ID: "no-token"},
			{Type: BlockWarning, ID: "risk"},
		},
	}
	got := blockIDsInOrder(r)
	want := []string{"no-token", "risk", "cause", "tests", "edit"}
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

func TestVerificationGlyphSaysHowItWent(t *testing.T) {
	for status, glyph := range map[string]string{
		VerifyPassed:  "✓",
		VerifyFailed:  "✗",
		VerifyPartial: "◐",
		VerifyNotRun:  "○",
	} {
		b := Block{Type: BlockVerification, Status: status}
		if got := b.Glyph(); got != glyph {
			t.Errorf("%s renders %q, want %q", status, got, glyph)
		}
	}
}

func TestHeaderIsTheChipAndHeadline(t *testing.T) {
	r := completedReport()
	first := Render(r, Options{})
	line := strings.SplitN(first, "\n", 2)[0]
	if line != "DONE · Renamed the ordering helper and updated its callers." {
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
	r.Blocks = append(r.Blocks, decisionBlock())

	text := Render(r, Options{})
	if !strings.Contains(text, "Implement reordering while preserving hidden-row positions.") {
		t.Errorf("the instruction was not shown:\n%s", text)
	}
}

func TestOptionsAreNumberedOnTheActiveBlockOnly(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	first := decisionBlock()
	second := decisionBlock()
	second.ID = "second-decision"
	r.Blocks = append(r.Blocks, first, second)

	numbered := map[string]int{}
	for _, l := range Lines(r, Options{}) {
		if l.Kind == LineOption && l.Option > 0 {
			numbered[l.BlockID]++
		}
	}
	if numbered["filtered-reordering"] == 0 {
		t.Error("the first decision's options were not numbered")
	}
	if numbered["second-decision"] != 0 {
		t.Error("a second decision's options were numbered too")
	}

	// Selecting the second one moves the numbers to it.
	numbered = map[string]int{}
	for _, l := range Lines(r, Options{Active: "second-decision"}) {
		if l.Kind == LineOption && l.Option > 0 {
			numbered[l.BlockID]++
		}
	}
	if numbered["second-decision"] == 0 {
		t.Error("the active block's options were not numbered")
	}
}

func TestRecommendationIsNamedOrItsAbsenceExplained(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	b := decisionBlock()
	r.Blocks = append(r.Blocks, b)
	if !strings.Contains(Render(r, Options{}), "Recommended: Preserve hidden positions") {
		t.Error("the recommendation was not named")
	}

	r.Blocks[1].Type = BlockQuestion
	r.Blocks[1].Question = "Which database?"
	r.Blocks[1].Recommendation = ""
	if !strings.Contains(Render(r, Options{}), "No recommendation:") {
		t.Error("a question with no recommendation did not say so")
	}
}

func TestFirstInteractiveSkipsBlocksThatAskNothing(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	r.Blocks = append(r.Blocks, decisionBlock())
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

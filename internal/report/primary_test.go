package report

import "testing"

func primaryFixture() *Report {
	return &Report{
		TaskStatus: StatusNeedsAction,
		Headline:   "Needs a decision.",
		Blocks: []Block{
			{Type: BlockCheck, ID: "tests", Summary: "ok", Status: CheckPassed},
			{Type: BlockQuestion, ID: "q", Summary: "Pick.",
				Options: []Option{
					{ID: "a", Label: "A", Effect: EffectDispatch, Instruction: "Do A."},
					{ID: "b", Label: "B", Effect: EffectResolve},
				},
				Recommendation: "a", ResponseType: ResponseSingle},
		},
	}
}

func TestPrimaryChoiceReturnsRecommended(t *testing.T) {
	r := primaryFixture()
	c := r.PrimaryChoice()
	if c == nil {
		t.Fatal("no primary choice")
	}
	if b := r.Blocks[c.Block]; b.ID != "q" {
		t.Errorf("block is %q, want q", b.ID)
	}
	if o := r.Blocks[c.Block].Options[c.Option]; o.ID != "a" {
		t.Errorf("option is %q, want a", o.ID)
	}
	if c.Number != 1 {
		t.Errorf("number is %d, want 1", c.Number)
	}
}

func TestPrimaryChoiceNilWithoutRecommendation(t *testing.T) {
	r := primaryFixture()
	r.Blocks[1].Recommendation = ""
	if c := r.PrimaryChoice(); c != nil {
		t.Errorf("got %#v, want nil", c)
	}
	var nilReport *Report
	if nilReport.PrimaryChoice() != nil {
		t.Error("nil report should have no primary choice")
	}
}

func TestPrimaryChoicePicksFirstRecommendedBlock(t *testing.T) {
	r := primaryFixture()
	second := r.Blocks[1]
	second.ID = "second"
	second.Options = []Option{{ID: "yes", Label: "Yes", Effect: EffectDispatch, Instruction: "Do it."}}
	second.Recommendation = "yes"
	r.Blocks = append(r.Blocks, second)
	c := r.PrimaryChoice()
	if c == nil {
		t.Fatal("no primary choice")
	}
	if got := r.Blocks[c.Block].ID; got != "q" {
		t.Errorf("block is %q, want the first recommended block q", got)
	}
}

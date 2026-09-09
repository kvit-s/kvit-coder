package report

import (
	"strings"
	"testing"
)

// completedReport is the smallest report a finished turn can submit.
func completedReport() *Report {
	return &Report{
		TaskStatus: StatusCompleted,
		Headline:   "Renamed the ordering helper and updated its callers.",
		Blocks: []Block{{
			Type:    BlockVerification,
			ID:      "unit-tests",
			Summary: "All 14 tests in the package pass.",
			Status:  VerifyPassed,
		}},
	}
}

// decisionBlock is a well-formed interactive block, used as the starting point
// for the tests that break one rule at a time.
func decisionBlock() Block {
	return Block{
		Type:    BlockDecision,
		ID:      "filtered-reordering",
		Summary: "Choose how reordering behaves while a filter hides rows.",
		Options: []Option{
			{ID: "preserve", Label: "Preserve hidden positions", Effect: EffectDispatch,
				Instruction: "Implement reordering while preserving hidden-row positions."},
			{ID: "recompute", Label: "Recompute the ordering", Effect: EffectDispatch,
				Instruction: "Implement reordering by recomputing the whole ordering."},
		},
		Recommendation:       "preserve",
		RecommendationReason: "It disturbs the least unrelated state.",
		ResponseType:         ResponseSingle,
	}
}

// codes lists the problem codes a validation produced, for assertions that care
// which rule fired rather than how it was worded.
func codes(ps []Problem) []string {
	out := make([]string, len(ps))
	for i, p := range ps {
		out[i] = p.Code
	}
	return out
}

func hasCode(ps []Problem, code string) bool {
	for _, p := range ps {
		if p.Code == code {
			return true
		}
	}
	return false
}

func TestValidAcceptsMinimalReport(t *testing.T) {
	if ps := Validate(completedReport(), Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("valid report rejected: %v", codes(ps))
	}
}

func TestNormalizeTrimsAndUnfences(t *testing.T) {
	r := completedReport()
	r.Headline = "  Renamed the ordering helper.  "
	r.Blocks[0].Details = "```go\nfunc main() {}\n```"
	r.Blocks[0].ID = " unit-tests "
	Normalize(r)

	if r.Headline != "Renamed the ordering helper." {
		t.Errorf("headline is %q, want it trimmed", r.Headline)
	}
	if r.Blocks[0].Details != "func main() {}" {
		t.Errorf("details is %q, want the fence stripped", r.Blocks[0].Details)
	}
	if r.Blocks[0].ID != "unit-tests" {
		t.Errorf("id is %q, want it trimmed", r.Blocks[0].ID)
	}
}

// A fenced value that is only partly a fence is left alone: stripping it would
// change what the model wrote.
func TestNormalizeLeavesInnerFencesAlone(t *testing.T) {
	r := completedReport()
	r.Blocks[0].Details = "Here is the code:\n```go\nx := 1\n```"
	Normalize(r)
	if !strings.Contains(r.Blocks[0].Details, "```go") {
		t.Errorf("details lost its inner fence: %q", r.Blocks[0].Details)
	}
}

func TestEveryProblemIsReportedAtOnce(t *testing.T) {
	r := &Report{
		TaskStatus: "finished",
		Headline:   "",
		Blocks: []Block{
			{Type: BlockVerification, ID: "Bad ID", Summary: ""},
		},
	}
	ps := Validate(r, Rules{Mutated: true})
	for _, want := range []string{"status_unknown", "headline_empty", "block_id_invalid", "summary_empty", "field_required"} {
		if !hasCode(ps, want) {
			t.Errorf("missing %s; got %v", want, codes(ps))
		}
	}
}

func TestHeadlineLimits(t *testing.T) {
	r := completedReport()
	r.Headline = strings.Repeat("x", HeadlineMax+1)
	if !hasCode(Validate(r, Rules{Mutated: true}), "headline_too_long") {
		t.Error("an over-long headline was accepted")
	}

	r = completedReport()
	r.Headline = "First line.\nSecond line."
	if !hasCode(Validate(r, Rules{Mutated: true}), "headline_multiline") {
		t.Error("a two-line headline was accepted")
	}
}

func TestDuplicateBlockIDs(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, Block{
		Type: BlockChange, ID: "unit-tests", Summary: "Renamed the helper.",
	})
	if !hasCode(Validate(r, Rules{Mutated: true}), "block_id_duplicate") {
		t.Error("two blocks with one id were accepted")
	}
}

func TestBlockLimit(t *testing.T) {
	r := completedReport()
	for i := 0; i < 5; i++ {
		r.Blocks = append(r.Blocks, Block{
			Type: BlockChange, ID: "change-" + string(rune('a'+i)), Summary: "Something changed.",
		})
	}
	if ps := Validate(r, Rules{MaxBlocks: 20, Mutated: true}); hasCode(ps, "blocks_too_many") {
		t.Error("6 blocks rejected against a limit of 20")
	}
	if ps := Validate(r, Rules{MaxBlocks: 3, Mutated: true}); !hasCode(ps, "blocks_too_many") {
		t.Error("6 blocks accepted against a limit of 3")
	}
}

// A completed turn has to say how it was checked. This is the rule that keeps a
// turn from claiming success it never verified.
func TestCompletedNeedsVerification(t *testing.T) {
	r := completedReport()
	r.Blocks[0] = Block{Type: BlockChange, ID: "rename", Summary: "Renamed the helper."}
	if !hasCode(Validate(r, Rules{Mutated: true}), "status_needs_verification") {
		t.Error("a completed report with no verification was accepted")
	}
}

// Saying plainly that nothing was run satisfies it; that is the intended way
// out for a turn that only read and explained.
func TestNotRunVerificationSatisfiesCompleted(t *testing.T) {
	r := completedReport()
	r.Blocks[0] = Block{
		Type: BlockVerification, ID: "nothing-run", Summary: "No tests were run.",
		Status: VerifyNotRun, Limitation: "No files changed, so no test applies.",
	}
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("not_run with a limitation rejected: %v", codes(ps))
	}
}

func TestPartialVerificationNeedsALimitation(t *testing.T) {
	r := completedReport()
	r.Blocks[0].Status = VerifyPartial
	if !hasCode(Validate(r, Rules{Mutated: true}), "verification_needs_limitation") {
		t.Error("a partial verification with no limitation was accepted")
	}
	r.Blocks[0].RequiredToVerify = "A machine with the GPU driver installed."
	if hasCode(Validate(r, Rules{Mutated: true}), "verification_needs_limitation") {
		t.Error("required_to_verify did not satisfy the rule")
	}
}

func TestInteractiveBlockForcesTheStatus(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, decisionBlock())
	if !hasCode(Validate(r, Rules{Mutated: true}), "interaction_needs_status") {
		t.Error("a completed report holding a decision was accepted")
	}

	r.TaskStatus = StatusNeedsAction
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("needs_action with a decision rejected: %v", codes(ps))
	}
}

func TestStatusesNeedTheirBlock(t *testing.T) {
	for _, tc := range []struct {
		status Status
		code   string
	}{
		{StatusNeedsAction, "status_needs_interaction"},
		{StatusBlocked, "status_needs_blocked"},
		{StatusFailed, "status_needs_explanation"},
	} {
		r := completedReport()
		r.TaskStatus = tc.status
		if !hasCode(Validate(r, Rules{Mutated: true}), tc.code) {
			t.Errorf("%s with no matching block was accepted", tc.status)
		}
	}
}

func TestOptionRules(t *testing.T) {
	base := func() *Report {
		r := completedReport()
		r.TaskStatus = StatusNeedsAction
		r.Blocks = append(r.Blocks, decisionBlock())
		return r
	}

	t.Run("too few options", func(t *testing.T) {
		r := base()
		r.Blocks[1].Options = r.Blocks[1].Options[:1]
		r.Blocks[1].Recommendation = "preserve"
		if !hasCode(Validate(r, Rules{Mutated: true}), "options_count") {
			t.Error("one option was accepted")
		}
	})

	t.Run("dispatch needs an instruction", func(t *testing.T) {
		r := base()
		r.Blocks[1].Options[0].Instruction = ""
		if !hasCode(Validate(r, Rules{Mutated: true}), "option_instruction_required") {
			t.Error("a dispatch option with no instruction was accepted")
		}
	})

	t.Run("only dispatch has an instruction", func(t *testing.T) {
		r := base()
		r.Blocks[1].Options[1].Effect = EffectCollect
		if !hasCode(Validate(r, Rules{Mutated: true}), "option_instruction_not_allowed") {
			t.Error("a collect option with an instruction was accepted")
		}
	})

	t.Run("recommendation names an option", func(t *testing.T) {
		r := base()
		r.Blocks[1].Recommendation = "something-else"
		if !hasCode(Validate(r, Rules{Mutated: true}), "recommendation_unknown") {
			t.Error("a recommendation naming nothing was accepted")
		}
	})

	t.Run("a question may have no recommendation", func(t *testing.T) {
		r := base()
		b := &r.Blocks[1]
		b.Type = BlockQuestion
		b.Question = "Which database should the exporter write to?"
		b.Recommendation = ""
		b.RecommendationReason = "Both are in use and nothing in the repository says which is canonical."
		if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
			t.Fatalf("a question with no recommendation was rejected: %v", codes(ps))
		}
	})

	t.Run("a decision may not", func(t *testing.T) {
		r := base()
		r.Blocks[1].Recommendation = ""
		if !hasCode(Validate(r, Rules{Mutated: true}), "field_required") {
			t.Error("a decision with no recommendation was accepted")
		}
	})
}

func TestNextStepNeedsAcceptAndAlternative(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	b := decisionBlock()
	b.Type = BlockNextStep
	b.ID = "apply-patch"
	r.Blocks = append(r.Blocks, b)

	ps := Validate(r, Rules{Mutated: true})
	if !hasCode(ps, "next_step_needs_alternative") {
		t.Errorf("a next_step of two dispatches was accepted: %v", codes(ps))
	}

	r.Blocks[1].Options[1] = Option{ID: "say-more", Label: "Tell me what to change instead", Effect: EffectCollect}
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("a next_step with an accept and an alternative was rejected: %v", codes(ps))
	}
}

func TestNonInteractiveBlockRejectsOptions(t *testing.T) {
	r := completedReport()
	r.Blocks[0].Options = []Option{{ID: "a", Label: "A", Effect: EffectResolve}}
	if !hasCode(Validate(r, Rules{Mutated: true}), "options_not_allowed") {
		t.Error("a verification block with options was accepted")
	}
}

func TestUnclassifiedMustSayWhy(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, Block{Type: BlockUnclassified, ID: "odd", Summary: "Something happened."})
	if !hasCode(Validate(r, Rules{Mutated: true}), "field_required") {
		t.Error("an unclassified block with no reason was accepted")
	}
}

func TestFindingNeedsItsFields(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, Block{Type: BlockFinding, ID: "root-cause", Summary: "The retry loop never terminates."})
	ps := Validate(r, Rules{Mutated: true})
	if !hasCode(ps, "field_required") {
		t.Errorf("a finding with no impact or importance was accepted: %v", codes(ps))
	}
}

func TestProblemPathsPointAtTheField(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	b := decisionBlock()
	b.Options[0].Instruction = ""
	r.Blocks = append(r.Blocks, b)

	var path string
	for _, p := range Validate(r, Rules{Mutated: true}) {
		if p.Code == "option_instruction_required" {
			path = p.Path
		}
	}
	if path != "/blocks/1/options/0/instruction" {
		t.Errorf("problem path is %q, want /blocks/1/options/0/instruction", path)
	}
}

func TestFormatProblemsLeadsWithTheMarker(t *testing.T) {
	text := FormatProblems([]Problem{{Path: "/headline", Code: "headline_empty", Message: "The headline is empty."}})
	if !strings.HasPrefix(text, "REPORT_REJECTED: ") {
		t.Errorf("rejection does not lead with the marker: %q", text)
	}
	if !strings.Contains(text, "/headline") {
		t.Errorf("rejection does not name the field: %q", text)
	}
}

// A turn that only read and explained is finished without a verification
// block. Demanding one is what pushes the explanation into a block that
// verified nothing.
func TestReadOnlyTurnNeedsNoVerification(t *testing.T) {
	blocking := false
	r := &Report{
		TaskStatus: StatusCompleted,
		Headline:   "The retry loop never terminates when the queue is empty.",
		Blocks: []Block{{
			Type: BlockFinding, ID: "root-cause",
			Summary:           "drain() loops forever because the empty case never returns.",
			Impact:            "Any call with an empty queue hangs the turn.",
			Importance:        "high",
			BlocksCurrentTask: &blocking,
		}},
	}
	if ps := Validate(r, Rules{Mutated: false}); len(ps) != 0 {
		t.Fatalf("an explanation-only report was rejected: %v", codes(ps))
	}
	// The same report from a turn that changed files still owes a verification.
	if !hasCode(Validate(r, Rules{Mutated: true}), "status_needs_verification") {
		t.Error("a turn that changed something got away with no verification")
	}
}

func TestCompletedStillNeedsSomething(t *testing.T) {
	r := &Report{TaskStatus: StatusCompleted, Headline: "Done."}
	if !hasCode(Validate(r, Rules{Mutated: false}), "status_needs_block") {
		t.Error("a report of nothing but a headline was accepted")
	}
}

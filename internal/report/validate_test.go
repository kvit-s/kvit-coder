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
			Type:    BlockCheck,
			ID:      "unit-tests",
			Summary: "All 14 tests in the package pass.",
			Status:  CheckPassed,
		}},
	}
}

// questionBlock is a well-formed interactive block, used as the starting point
// for the tests that break one rule at a time.
func questionBlock() Block {
	return Block{
		Type:    BlockQuestion,
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
			{Type: BlockCheck, ID: "Bad ID", Summary: ""},
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
func TestCompletedNeedsCheck(t *testing.T) {
	r := completedReport()
	r.Blocks[0] = Block{Type: BlockChange, ID: "rename", Summary: "Renamed the helper."}
	if !hasCode(Validate(r, Rules{Mutated: true}), "status_needs_check") {
		t.Error("a completed report with no check was accepted")
	}
}

// Saying plainly that nothing was run satisfies it; that is the intended way
// out for a turn that only read and explained.
func TestNotRunCheckSatisfiesCompleted(t *testing.T) {
	r := completedReport()
	r.Blocks[0] = Block{
		Type: BlockCheck, ID: "nothing-run", Summary: "No tests were run.",
		Status: CheckNotRun, Limitation: "No files changed, so no test applies.",
	}
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("not_run with a limitation rejected: %v", codes(ps))
	}
}

func TestPartialCheckNeedsALimitation(t *testing.T) {
	r := completedReport()
	r.Blocks[0].Status = CheckPartial
	if !hasCode(Validate(r, Rules{Mutated: true}), "check_needs_limitation") {
		t.Error("a partial check with no limitation was accepted")
	}
	r.Blocks[0].RequiredToVerify = "A machine with the GPU driver installed."
	if hasCode(Validate(r, Rules{Mutated: true}), "check_needs_limitation") {
		t.Error("required_to_verify did not satisfy the rule")
	}
}

func TestInteractiveBlockForcesTheStatus(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, questionBlock())
	if !hasCode(Validate(r, Rules{Mutated: true}), "interaction_needs_status") {
		t.Error("a completed report holding a question was accepted")
	}

	r.TaskStatus = StatusNeedsAction
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("needs_action with a question rejected: %v", codes(ps))
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
		r.Blocks = append(r.Blocks, questionBlock())
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
		r.Blocks[1].Type = BlockNext
		r.Blocks[1].Options[0].Instruction = ""
		if !hasCode(Validate(r, Rules{Mutated: true}), "option_instruction_required") {
			t.Error("a dispatch option with no instruction was accepted")
		}
	})

	// An answer to a question may be only its label: picking it sends the
	// label, anchored to the question, as the next prompt.
	t.Run("a question's answer needs no instruction", func(t *testing.T) {
		r := base()
		r.Blocks[1].Options[0].Instruction = ""
		if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
			t.Fatalf("an answer with no instruction was rejected: %v", codes(ps))
		}
		b := &r.Blocks[1]
		if got := b.Prompt(&b.Options[0]); got != "[report filtered-reordering] Preserve hidden positions" {
			t.Errorf("picking the answer sends %q", got)
		}
		if got := b.Prompt(&b.Options[1]); got != b.Options[1].Instruction {
			t.Errorf("an option with an instruction sends %q, want the instruction", got)
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
		b.Summary = "Which database should the exporter write to?"
		b.Recommendation = ""
		b.RecommendationReason = "Both are in use and nothing in the repository says which is canonical."
		if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
			t.Fatalf("a question with no recommendation was rejected: %v", codes(ps))
		}
	})

	t.Run("a next block may not", func(t *testing.T) {
		r := base()
		r.Blocks[1].Type = BlockNext
		r.Blocks[1].Recommendation = ""
		if !hasCode(Validate(r, Rules{Mutated: true}), "field_required") {
			t.Error("a next block with no recommendation was accepted")
		}
	})
}

func TestNextNeedsAcceptAndAlternative(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	b := questionBlock()
	b.Type = BlockNext
	b.ID = "apply-patch"
	r.Blocks = append(r.Blocks, b)

	ps := Validate(r, Rules{Mutated: true})
	if !hasCode(ps, "next_needs_alternative") {
		t.Errorf("a next block of two dispatches was accepted: %v", codes(ps))
	}

	r.Blocks[1].Options[1] = Option{ID: "say-more", Label: "Tell me what to change instead", Effect: EffectCollect}
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("a next block with an accept and an alternative was rejected: %v", codes(ps))
	}
}

func TestNonInteractiveBlockRejectsOptions(t *testing.T) {
	r := completedReport()
	r.Blocks[0].Options = []Option{{ID: "a", Label: "A", Effect: EffectResolve}}
	if !hasCode(Validate(r, Rules{Mutated: true}), "options_not_allowed") {
		t.Error("a check block with options was accepted")
	}
}

func TestUnclassifiedMustSayWhy(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, Block{Type: BlockUnclassified, ID: "odd", Summary: "Something happened."})
	if !hasCode(Validate(r, Rules{Mutated: true}), "field_required") {
		t.Error("an unclassified block with no reason was accepted")
	}
}

func TestFindingNeedsAnImpact(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, Block{Type: BlockFinding, ID: "root-cause", Summary: "The retry loop never terminates."})
	ps := Validate(r, Rules{Mutated: true})
	if !hasCode(ps, "field_required") {
		t.Errorf("a finding with no impact was accepted: %v", codes(ps))
	}
}

// importance is optional on a finding, but when it is given it is one of the
// levels, so the card can sort findings by it.
func TestFindingImportanceIsOptional(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, Block{Type: BlockFinding, ID: "root-cause",
		Summary: "The retry loop never terminates.", Impact: "Any call with an empty queue hangs the turn."})
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("a finding with no importance was rejected: %v", codes(ps))
	}
	r.Blocks[1].Importance = "urgent"
	if !hasCode(Validate(r, Rules{Mutated: true}), "field_unknown_value") {
		t.Error("a finding with an unknown importance was accepted")
	}
}

func TestProblemPathsPointAtTheField(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	b := questionBlock()
	b.Type = BlockNext
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

// A turn that only read and explained is finished without a check
// block. Demanding one is what pushes the explanation into a block that
// verified nothing.
func TestReadOnlyTurnNeedsNoCheck(t *testing.T) {
	r := &Report{
		TaskStatus: StatusCompleted,
		Headline:   "The retry loop never terminates when the queue is empty.",
		Blocks: []Block{{
			Type: BlockFinding, ID: "root-cause",
			Summary:    "drain() loops forever because the empty case never returns.",
			Impact:     "Any call with an empty queue hangs the turn.",
			Importance: "high",
		}},
	}
	if ps := Validate(r, Rules{Mutated: false}); len(ps) != 0 {
		t.Fatalf("an explanation-only report was rejected: %v", codes(ps))
	}
	// The same report from a turn that changed files still owes a check.
	if !hasCode(Validate(r, Rules{Mutated: true}), "status_needs_check") {
		t.Error("a turn that changed something got away with no check")
	}
}

func TestCompletedStillNeedsSomething(t *testing.T) {
	r := &Report{TaskStatus: StatusCompleted, Headline: "Done."}
	if !hasCode(Validate(r, Rules{Mutated: false}), "status_needs_block") {
		t.Error("a report of nothing but a headline was accepted")
	}
}

// BuriedProposalHint fires only when the report proposes a follow-up in prose
// while offering no interactive block — the Antigravity case, where the fix
// lived in finding.details and the card offered no keypress.
func TestBuriedProposalHint(t *testing.T) {
	prose := &Report{
		TaskStatus: StatusCompleted,
		Headline:   "Stale PATH line lists a directory that no longer exists.",
		Blocks: []Block{{
			Type: BlockFinding, ID: "cause",
			Summary: "Line 144 runs ls on a missing directory.",
			Details: "How to fix (pick one): delete the line, or guard it. " +
				"Say the word and I will apply either.",
		}},
	}
	if !BuriedProposalHint(prose) {
		t.Error("a prose proposal with no interactive block drew no hint")
	}

	withAction := &Report{
		TaskStatus: StatusNeedsAction,
		Headline:   "Stale PATH line lists a directory that no longer exists.",
		Blocks: []Block{
			{
				Type: BlockFinding, ID: "cause",
				Summary: "Line 144 runs ls on a missing directory.",
				Details: "Say the word and I will apply either fix.",
			},
			questionBlock(),
		},
	}
	// questionBlock asks something, so the report already offers a keypress.
	withAction.Blocks[1].Type = BlockNext
	if BuriedProposalHint(withAction) {
		t.Error("a report that already offers an action drew the hint")
	}

	plain := &Report{
		TaskStatus: StatusCompleted,
		Headline:   "Renamed the helper.",
		Blocks: []Block{{
			Type: BlockFinding, ID: "cause",
			Summary: "The helper lived in two packages.",
			Details: "No follow-up remains.",
		}},
	}
	if BuriedProposalHint(plain) {
		t.Error("a report with no proposal drew the hint")
	}
}

// A blocked report says what is in the way with a warning or a finding, now
// that there is no block type of its own for it.
func TestBlockedIsExplainedByAWarning(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusBlocked
	r.Blocks = append(r.Blocks, Block{
		Type: BlockWarning, ID: "no-token", Summary: "The deploy token has expired.",
		Severity: "high", Impact: "Nothing can be pushed until it is renewed.",
	})
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("a blocked report explained by a warning was rejected: %v", codes(ps))
	}
}

// Sessions saved before the rename hold blocks under their old names, and a
// model continuing one may copy them. Both are read as the type that replaced
// them rather than rejected.
func TestNormalizeTranslatesRenamedTypes(t *testing.T) {
	r := completedReport()
	r.TaskStatus = StatusNeedsAction
	r.Blocks[0].Type = "verification"
	old := questionBlock()
	old.Type = "decision"
	step := questionBlock()
	step.Type = "next_step"
	step.ID = "apply-patch"
	step.Options[1] = Option{ID: "not-now", Label: "Not now", Effect: EffectResolve}
	r.Blocks = append(r.Blocks, old, step)

	Normalize(r)
	if r.Blocks[0].Type != BlockCheck || r.Blocks[1].Type != BlockQuestion || r.Blocks[2].Type != BlockNext {
		t.Fatalf("old type names were not translated: %q, %q, %q", r.Blocks[0].Type, r.Blocks[1].Type, r.Blocks[2].Type)
	}
	if ps := Validate(r, Rules{Mutated: true}); len(ps) != 0 {
		t.Fatalf("a translated report was rejected: %v", codes(ps))
	}
}

// blocked is no longer a block type, so a model that sends one is told the
// types there are.
func TestBlockedIsNotABlockType(t *testing.T) {
	r := completedReport()
	r.Blocks = append(r.Blocks, Block{Type: "blocked", ID: "stuck", Summary: "Cannot reach the registry."})
	Normalize(r)
	if !hasCode(Validate(r, Rules{Mutated: true}), "block_type_unknown") {
		t.Error("a blocked block was accepted")
	}
}

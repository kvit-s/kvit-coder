package report

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"
)

// Problem is one thing wrong with a report. Path is a JSON Pointer into the
// submitted arguments, so the model is told which field to fix rather than
// which rule it broke. Code is stable across wordings, for counting what
// models get wrong.
type Problem struct {
	Path    string `json:"path"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// idPattern is what a block or option id must look like: lower case, digits
// and hyphens, starting with a letter or digit.
var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

// fencePattern matches a whole value wrapped in a code fence, which models add
// when a field asks for Markdown. Stripping it is normalization, not an error.
var fencePattern = regexp.MustCompile("(?s)^```[a-zA-Z0-9_+-]*\n(.*)\n?```$")

// Normalize cleans up what a model sent before it is checked: surrounding
// whitespace everywhere, a code fence wrapped around a whole Markdown value,
// a block type under a name it used to have or left out where the block's
// fields name it, and a task_status coerced into agreement with the blocks. It
// never changes meaning, so a report that only needed normalizing is accepted
// rather than bounced for a formatting habit.
func Normalize(r *Report) {
	if r == nil {
		return
	}
	r.Headline = strings.TrimSpace(r.Headline)
	r.TaskStatus = Status(strings.TrimSpace(string(r.TaskStatus)))
	for i := range r.Blocks {
		b := &r.Blocks[i]
		b.Type = BlockType(strings.TrimSpace(string(b.Type)))
		if renamed, ok := renamedTypes[b.Type]; ok {
			b.Type = renamed
		}
		b.ID = strings.TrimSpace(b.ID)
		b.Summary = strings.TrimSpace(b.Summary)
		b.Impact = strings.TrimSpace(b.Impact)
		b.Recommendation = strings.TrimSpace(b.Recommendation)
		b.Details = unfence(b.Details)
		b.Importance = strings.TrimSpace(b.Importance)
		b.Status = strings.TrimSpace(b.Status)
		b.Evidence = strings.TrimSpace(b.Evidence)
		b.Limitation = strings.TrimSpace(b.Limitation)
		b.RequiredToVerify = strings.TrimSpace(b.RequiredToVerify)
		b.Severity = strings.TrimSpace(b.Severity)
		b.ResponseType = strings.TrimSpace(b.ResponseType)
		b.RecommendationReason = strings.TrimSpace(b.RecommendationReason)
		b.ReasonUnclassified = strings.TrimSpace(b.ReasonUnclassified)
		b.SuggestedType = strings.TrimSpace(b.SuggestedType)
		for j := range b.RelatedFiles {
			b.RelatedFiles[j] = strings.TrimSpace(b.RelatedFiles[j])
		}
		for j := range b.Options {
			o := &b.Options[j]
			o.ID = strings.TrimSpace(o.ID)
			o.Label = strings.TrimSpace(o.Label)
			o.Description = strings.TrimSpace(o.Description)
			o.Consequence = strings.TrimSpace(o.Consequence)
			o.Effect = strings.TrimSpace(o.Effect)
			o.Instruction = strings.TrimSpace(o.Instruction)
			o.Preview = unfence(o.Preview)
		}
		if b.Type == "" {
			b.Type = impliedType(b)
		}
	}
	// Block types are trimmed above, so interactivity is reliable here: the
	// deprecated status is coerced into agreement with the blocks rather than
	// rejected, so a stale label never costs a repair round trip.
	r.DefaultTaskStatus()
}

// impliedType is the type of a block that left its type out, when the fields
// it did fill belong to exactly one type: a status is a check, a severity a
// warning. Models drop the type most often on a block added in a hurry to
// satisfy a rejection, whose other fields say plainly what it is. Options do
// not decide between question and next, and summary, impact and details are
// shared, so a block with only those stays untyped and is rejected.
func impliedType(b *Block) BlockType {
	if len(b.Options) > 0 || b.ResponseType != "" || b.RecommendationReason != "" {
		return ""
	}
	var implied []BlockType
	if b.Status != "" || b.Evidence != "" || b.Limitation != "" || b.RequiredToVerify != "" {
		implied = append(implied, BlockCheck)
	}
	if b.Severity != "" {
		implied = append(implied, BlockWarning)
	}
	if b.Importance != "" {
		implied = append(implied, BlockFinding)
	}
	if b.ReasonUnclassified != "" || b.SuggestedType != "" {
		implied = append(implied, BlockUnclassified)
	}
	if len(implied) != 1 {
		return ""
	}
	return implied[0]
}

func unfence(s string) string {
	s = strings.TrimSpace(s)
	if m := fencePattern.FindStringSubmatch(s); m != nil {
		return strings.TrimSpace(m[1])
	}
	return s
}

// Rules are the parts of validation that depend on the turn rather than on the
// report, so the same report can be right for one turn and wrong for another.
type Rules struct {
	// MaxBlocks bounds the block list. Zero selects DefaultMaxBlocks.
	MaxBlocks int
	// Mutated says whether the turn changed anything. It decides what
	// "completed" has to show: a turn that edited files says how it checked
	// them, while a turn that only read and explained is finished without a
	// check block. Demanding one of a question-answering turn is how the
	// explanation ends up inside a block that checked nothing.
	Mutated bool
}

// Validate checks a normalized report and returns everything wrong with it, so
// the model can fix a whole report in one pass rather than one problem per
// round trip. An empty result means the report is acceptable.
func Validate(r *Report, rules Rules) []Problem {
	maxBlocks := rules.MaxBlocks
	if maxBlocks <= 0 {
		maxBlocks = DefaultMaxBlocks
	}
	var ps []Problem
	if r == nil {
		return []Problem{{Path: "/", Code: "report_missing", Message: "The report is empty."}}
	}
	// Defensive: Normalize already coerced the deprecated status into agreement
	// with the blocks, but a caller that validates without normalizing still
	// gets the coercion rather than a rejection for a field the card does not
	// even show.
	r.DefaultTaskStatus()
	ps = append(ps, checkHeadline(r)...)

	if len(r.Blocks) > maxBlocks {
		ps = append(ps, Problem{
			Path: "/blocks", Code: "blocks_too_many",
			Message: fmt.Sprintf("%d blocks; at most %d. Summarize related steps in one block's details instead of one block each.", len(r.Blocks), maxBlocks),
		})
	}

	seen := map[string]int{}
	for i := range r.Blocks {
		ps = append(ps, checkBlock(r, i, seen)...)
	}
	ps = append(ps, checkConsistency(r, rules)...)
	return ps
}

func checkHeadline(r *Report) []Problem {
	var ps []Problem
	switch {
	case r.Headline == "":
		ps = append(ps, Problem{Path: "/headline", Code: "headline_empty",
			Message: "The headline is empty. One sentence stating the material fact of the turn."})
	case utf8.RuneCountInString(r.Headline) > HeadlineMax:
		ps = append(ps, Problem{Path: "/headline", Code: "headline_too_long",
			Message: fmt.Sprintf("The headline is %d characters; at most %d.", utf8.RuneCountInString(r.Headline), HeadlineMax)})
	}
	if strings.ContainsAny(r.Headline, "\n\r") {
		ps = append(ps, Problem{Path: "/headline", Code: "headline_multiline",
			Message: "The headline is one line. Move the rest into a block."})
	}
	return ps
}

func checkBlock(r *Report, i int, seen map[string]int) []Problem {
	b := &r.Blocks[i]
	at := func(field string) string {
		if field == "" {
			return fmt.Sprintf("/blocks/%d", i)
		}
		return fmt.Sprintf("/blocks/%d/%s", i, field)
	}
	var ps []Problem

	known := false
	for _, t := range BlockTypes {
		if b.Type == t {
			known = true
			break
		}
	}
	if !known {
		ps = append(ps, Problem{Path: at("type"), Code: "block_type_unknown",
			Message: fmt.Sprintf("%q is not a block type. Use one of: %s.", b.Type, joinBlockTypes())})
		// Every check below is about a specific type, so there is nothing
		// further to say about this block.
		return ps
	}

	switch {
	case b.ID == "":
		ps = append(ps, Problem{Path: at("id"), Code: "block_id_invalid",
			Message: "The block has no id. Give it a short kebab-case name, such as \"reorder-tests\"."})
	case !idPattern.MatchString(b.ID):
		ps = append(ps, Problem{Path: at("id"), Code: "block_id_invalid",
			Message: fmt.Sprintf("%q is not a valid id: lower case, digits and hyphens, at most %d characters.", b.ID, IDMax)})
	default:
		if first, dup := seen[b.ID]; dup {
			ps = append(ps, Problem{Path: at("id"), Code: "block_id_duplicate",
				Message: fmt.Sprintf("Block %d already uses the id %q. Ids are unique within a report.", first, b.ID)})
		} else {
			seen[b.ID] = i
		}
	}

	switch {
	case b.Summary == "":
		ps = append(ps, Problem{Path: at("summary"), Code: "summary_empty",
			Message: "The block has no summary. State its fact in one sentence that stands on its own."})
	case utf8.RuneCountInString(b.Summary) > SummaryMax:
		ps = append(ps, Problem{Path: at("summary"), Code: "summary_too_long",
			Message: fmt.Sprintf("The summary is %d characters; at most %d. Move the rest into details.", utf8.RuneCountInString(b.Summary), SummaryMax)})
	}

	if len(b.Details) > DetailsMax {
		ps = append(ps, Problem{Path: at("details"), Code: "details_too_large",
			Message: fmt.Sprintf("details is %d bytes; at most %d.", len(b.Details), DetailsMax)})
	}

	switch b.Type {
	case BlockFinding:
		// impact is optional: the card folds it away, a finding's summary
		// usually says why it matters already, and requiring it was the most
		// common reason a report was sent back.
		if b.Importance != "" {
			ps = append(ps, oneOf(at, "importance", b.Importance, Levels)...)
		}
	case BlockCheck:
		ps = append(ps, oneOf(at, "status", b.Status, CheckStatuses)...)
		if (b.Status == CheckPartial || b.Status == CheckNotRun) &&
			b.Limitation == "" && b.RequiredToVerify == "" {
			ps = append(ps, Problem{Path: at("limitation"), Code: "check_needs_limitation",
				Message: fmt.Sprintf("A %q check says what was not checked and why: set limitation, or required_to_verify.", b.Status)})
		}
	case BlockWarning:
		ps = append(ps, oneOf(at, "severity", b.Severity, Levels)...)
		ps = append(ps, required(at, "impact", b.Impact, "what breaks if this is ignored")...)
	case BlockUnclassified:
		ps = append(ps, required(at, "reason_unclassified", b.ReasonUnclassified,
			"why no other block type fits")...)
	}

	if b.Interactive() {
		ps = append(ps, checkInteractive(r, b, i, at)...)
	} else if len(b.Options) > 0 {
		ps = append(ps, Problem{Path: at("options"), Code: "options_not_allowed",
			Message: fmt.Sprintf("A %s block has no options. Use \"question\" or \"next\" to ask for something.", b.Type)})
	}

	return ps
}

// otherRecommends reports whether any interactive block besides skip carries a
// recommendation, which is what lets a secondary next block omit its own: one
// recommended block per report is the primary proposal, the rest are secondary.
func otherRecommends(r *Report, skip int) bool {
	for j := range r.Blocks {
		if j == skip {
			continue
		}
		if r.Blocks[j].Interactive() && r.Blocks[j].Recommendation != "" {
			return true
		}
	}
	return false
}

// checkInteractive covers the block types that ask the user for something.
// They share one option list, one response type and one recommendation, so
// they share one check.
func checkInteractive(r *Report, b *Block, index int, at func(string) string) []Problem {
	var ps []Problem

	if n := len(b.Options); n < MinOptions || n > MaxOptions {
		ps = append(ps, Problem{Path: at("options"), Code: "options_count",
			Message: fmt.Sprintf("A %s block offers %d to %d options; this one has %d. Enumerating the ways forward is what makes it answerable.", b.Type, MinOptions, MaxOptions, n)})
	}

	seen := map[string]bool{}
	dispatches, alternatives := 0, 0
	for j := range b.Options {
		o := &b.Options[j]
		oat := func(field string) string {
			return fmt.Sprintf("%s/options/%d/%s", at(""), j, field)
		}
		switch {
		case o.ID == "":
			ps = append(ps, Problem{Path: oat("id"), Code: "option_id_invalid",
				Message: "The option has no id. Give it a short kebab-case name."})
		case !idPattern.MatchString(o.ID):
			ps = append(ps, Problem{Path: oat("id"), Code: "option_id_invalid",
				Message: fmt.Sprintf("%q is not a valid id: lower case, digits and hyphens.", o.ID)})
		case seen[o.ID]:
			ps = append(ps, Problem{Path: oat("id"), Code: "option_id_duplicate",
				Message: fmt.Sprintf("Two options share the id %q.", o.ID)})
		default:
			seen[o.ID] = true
		}

		switch {
		case o.Label == "":
			ps = append(ps, Problem{Path: oat("label"), Code: "option_label_empty",
				Message: "The option has no label. A few words naming the choice."})
		case utf8.RuneCountInString(o.Label) > LabelMax:
			ps = append(ps, Problem{Path: oat("label"), Code: "option_label_too_long",
				Message: fmt.Sprintf("The label is %d characters; at most %d.", utf8.RuneCountInString(o.Label), LabelMax)})
		}
		if utf8.RuneCountInString(o.Description) > TextMax {
			ps = append(ps, Problem{Path: oat("description"), Code: "option_text_too_long",
				Message: fmt.Sprintf("description is %d characters; at most %d.", utf8.RuneCountInString(o.Description), TextMax)})
		}
		if utf8.RuneCountInString(o.Consequence) > TextMax {
			ps = append(ps, Problem{Path: oat("consequence"), Code: "option_text_too_long",
				Message: fmt.Sprintf("consequence is %d characters; at most %d.", utf8.RuneCountInString(o.Consequence), TextMax)})
		}
		if len(o.Preview) > PreviewMax {
			ps = append(ps, Problem{Path: oat("preview"), Code: "option_preview_too_large",
				Message: fmt.Sprintf("preview is %d bytes; at most %d.", len(o.Preview), PreviewMax)})
		}

		switch o.Effect {
		case EffectDispatch:
			dispatches++
			switch {
			case o.Instruction == "" && b.Type == BlockQuestion:
				// Picking it sends the label as the answer.
			case o.Instruction == "":
				ps = append(ps, Problem{Path: oat("instruction"), Code: "option_instruction_required",
					Message: "A dispatch option runs its instruction as the next prompt, and the user reads it before picking, so it cannot be empty."})
			case len(o.Instruction) > InstructionMax:
				ps = append(ps, Problem{Path: oat("instruction"), Code: "option_instruction_too_large",
					Message: fmt.Sprintf("instruction is %d bytes; at most %d.", len(o.Instruction), InstructionMax)})
			}
		case EffectCollect, EffectResolve:
			alternatives++
			if o.Instruction != "" {
				ps = append(ps, Problem{Path: oat("instruction"), Code: "option_instruction_not_allowed",
					Message: fmt.Sprintf("Only a dispatch option has an instruction; this one is %q.", o.Effect)})
			}
		default:
			ps = append(ps, Problem{Path: oat("effect"), Code: "option_effect_unknown",
				Message: fmt.Sprintf("%q is not an effect. Use one of: %s.", o.Effect, strings.Join(Effects, ", "))})
		}
	}

	if b.ResponseType != ResponseSingle {
		ps = append(ps, Problem{Path: at("response_type"), Code: "response_type_invalid",
			Message: fmt.Sprintf("response_type is %q; the only value is %q. Free text is always allowed alongside the options.", b.ResponseType, ResponseSingle)})
	}

	if b.RecommendationReason == "" {
		ps = append(ps, Problem{Path: at("recommendation_reason"), Code: "field_required",
			Message: "Say why you recommend what you recommend, or why no safe default can be inferred."})
	}

	// A question may have no safe default. One next block per report recommends
	// the primary proposal; further next blocks omit it and are secondary, since
	// the card lets the user pick one option per turn.
	if b.Recommendation == "" {
		if b.Type == BlockNext && !otherRecommends(r, index) {
			ps = append(ps, Problem{Path: at("recommendation"), Code: "field_required",
				Message: "A next block recommends one of its options by id. Recommend one block per report: set recommendation on the primary proposal; further next blocks omit it."})
		}

	} else if b.Option(b.Recommendation) == nil {
		ps = append(ps, Problem{Path: at("recommendation"), Code: "recommendation_unknown",
			Message: fmt.Sprintf("The recommendation %q is not one of this block's option ids.", b.Recommendation)})
	}

	if b.Type == BlockNext {
		if dispatches == 0 {
			ps = append(ps, Problem{Path: at("options"), Code: "next_needs_dispatch",
				Message: "A next block needs an accept option with effect \"dispatch\" and the instruction it would run."})
		}
		if alternatives == 0 {
			ps = append(ps, Problem{Path: at("options"), Code: "next_needs_alternative",
				Message: "A next block needs at least one alternative with effect \"collect\" or \"resolve\", so accepting is a choice rather than the only option."})
		}
	}
	return ps
}

// checkConsistency covers the content rules that do not depend on any block
// type: a turn that changed something says how it was checked, and a report
// is never empty. The deprecated task_status never gates here: whether the
// turn waits on the user is derived from question/next blocks, and an explicit
// blocked or failed is preserved for the headline dot without demanding extra
// blocks.
func checkConsistency(r *Report, rules Rules) []Problem {
	var ps []Problem
	checks := 0
	for i := range r.Blocks {
		if r.Blocks[i].Type == BlockCheck {
			checks++
		}
	}
	switch {
	case rules.Mutated && checks == 0:
		ps = append(ps, Problem{Path: "/blocks", Code: "status_needs_check",
			Message: "This turn changed something, so the report says how that was checked: add a check block. When you ran nothing, use status \"not_run\" with a limitation saying why."})
	case len(r.Blocks) == 0:
		ps = append(ps, Problem{Path: "/blocks", Code: "status_needs_block",
			Message: "A report needs at least one block. This turn changed nothing, so what it found goes in a finding rather than a check."})
	}
	return ps
}

func required(at func(string) string, field, value, what string) []Problem {
	if value != "" {
		return nil
	}
	return []Problem{{Path: at(field), Code: "field_required",
		Message: fmt.Sprintf("%s is required: %s.", field, what)}}
}

func oneOf(at func(string) string, field, value string, allowed []string) []Problem {
	for _, a := range allowed {
		if value == a {
			return nil
		}
	}
	if value == "" {
		return []Problem{{Path: at(field), Code: "field_required",
			Message: fmt.Sprintf("%s is required. Use one of: %s.", field, strings.Join(allowed, ", "))}}
	}
	return []Problem{{Path: at(field), Code: "field_unknown_value",
		Message: fmt.Sprintf("%s is %q. Use one of: %s.", field, value, strings.Join(allowed, ", "))}}
}

func joinBlockTypes() string {
	out := make([]string, len(BlockTypes))
	for i, t := range BlockTypes {
		out[i] = string(t)
	}
	return strings.Join(out, ", ")
}

// FormatProblems is the rejection the model reads. It leads with the marker so
// the problem is unmistakable in a wall of tool output, then lists every
// problem as a path and a sentence.
func FormatProblems(ps []Problem) string {
	var sb strings.Builder
	sb.WriteString("REPORT_REJECTED: ")
	if len(ps) == 1 {
		sb.WriteString("1 problem.\n")
	} else {
		fmt.Fprintf(&sb, "%d problems.\n", len(ps))
	}
	for _, p := range ps {
		fmt.Fprintf(&sb, "  %s: %s\n", p.Path, p.Message)
	}
	sb.WriteString("Fix all of them and call Report again.")
	return sb.String()
}

// Package report holds the structured end-of-turn report: its shape, the
// checks it must pass before the agent accepts it, and the rendering the agent
// and the interactive front end both draw it with.
//
// A report is what a turn ends with instead of free prose. The model calls the
// Report tool with JSON in the shape below; the tool refuses JSON that does not
// fit and says what is wrong, and the turn cannot end until an acceptable
// report exists. The front end then draws the accepted report as a card at the
// prompt, where one keypress answers a decision it raises.
//
// This package depends on nothing else in kvit-coder, so the tool that produces
// a report, the session reader that finds one and the composer that draws it
// all work from one definition of it.
//
// docs/structured-reports.md is the design.
package report

import (
	"fmt"
	"sort"
	"strings"
)

// Status is the task_status field. It is deprecated and informational only:
// the card never shows it, the validator never rejects on it, and whether the
// turn waits on the user is derived from the blocks (HasInteractive), not from
// this field. It is kept so reports saved by older sessions still decode, and
// so an explicit blocked or failed still tints the headline dot red.
type Status string

const (
	// StatusCompleted means the work is done and checked.
	StatusCompleted Status = "completed"
	// StatusCompletedWithNotes means done, with a caveat stated in the report.
	StatusCompletedWithNotes Status = "completed_with_notes"
	// StatusNeedsAction means the turn stopped where the user must choose or
	// answer something.
	StatusNeedsAction Status = "needs_action"
	// StatusBlocked means the work cannot continue without the user or
	// something outside the session. A warning or finding says what is in
	// the way.
	StatusBlocked Status = "blocked"
	// StatusFailed means the work did not succeed, with the reason recorded.
	StatusFailed Status = "failed"
)

// Statuses lists every status an older report may carry, in the order they
// used to be documented. New reports should omit task_status entirely; the
// schema keeps the field only so old sessions still decode.
var Statuses = []Status{
	StatusCompleted,
	StatusCompletedWithNotes,
	StatusNeedsAction,
	StatusBlocked,
	StatusFailed,
}

// InferStatus derives the task status from the blocks: a report that asks the
// user something needs action, anything else counts as completed. It is what
// Normalize fills an omitted or unknown status with, and what an explicit
// completed/completed_with_notes becomes when the report holds a question or
// next block (and vice versa for a needs_action with nothing to answer), so a
// stale label never costs a repair round trip. An explicit blocked or failed
// is preserved as sent, since only the model knows it is stuck.
func (r *Report) InferStatus() Status {
	if r != nil && r.HasInteractive() {
		return StatusNeedsAction
	}
	return StatusCompleted
}

// DefaultTaskStatus coerces task_status into agreement with the blocks. It is
// normalization, not validation: an omitted or unknown status is inferred
// (completed when the report asks nothing, needs_action when it does), a
// completed/completed_with_notes holding a question or next becomes
// needs_action, and a needs_action holding nothing to answer becomes
// completed. An explicit blocked or failed is left alone. The validator never
// rejects on the status, so after this call there is nothing left to reject.
func (r *Report) DefaultTaskStatus() {
	if r == nil {
		return
	}
	if r.TaskStatus == "" || !knownStatus(r.TaskStatus) {
		r.TaskStatus = r.InferStatus()
		return
	}
	if r.HasInteractive() {
		if r.TaskStatus == StatusCompleted || r.TaskStatus == StatusCompletedWithNotes {
			r.TaskStatus = StatusNeedsAction
		}
		return
	}
	if r.TaskStatus == StatusNeedsAction {
		r.TaskStatus = StatusCompleted
	}
}

// knownStatus reports whether s is one of the retained status values.
func knownStatus(s Status) bool {
	for _, known := range Statuses {
		if s == known {
			return true
		}
	}
	return false
}

// BlockType selects which shape a block has.
type BlockType string

const (
	// BlockChange is file edits and other work output.
	BlockChange BlockType = "change"
	// BlockFinding is something learned that matters to the task.
	BlockFinding BlockType = "finding"
	// BlockCheck is how the work was checked.
	BlockCheck BlockType = "check"
	// BlockWarning is a risk or caveat.
	BlockWarning BlockType = "warning"
	// BlockQuestion is something only the user can settle: a choice between
	// ways forward, or information the user has. It differs from next
	// in having no obvious continuation to accept.
	BlockQuestion BlockType = "question"
	// BlockNext is a proposed continuation the user accepts or redirects.
	BlockNext BlockType = "next"
	// BlockUnclassified is the escape hatch for something the model has to
	// say that fits no other type. It is not meant to be used routinely.
	BlockUnclassified BlockType = "unclassified"
)

// BlockTypes lists every block type a model may send.
var BlockTypes = []BlockType{
	BlockChange,
	BlockFinding,
	BlockCheck,
	BlockWarning,
	BlockQuestion,
	BlockNext,
	BlockUnclassified,
}

// renamedTypes maps block type names reports used to have onto the type that
// replaced them. Sessions saved before the rename still hold reports with the
// old names, and a model continuing such a session sees them in its history
// and may copy them; Normalize translates both rather than rejecting them.
var renamedTypes = map[BlockType]BlockType{
	"verification": BlockCheck,
	"decision":     BlockQuestion,
	"next_step":    BlockNext,
}

// Check statuses.
const (
	CheckPassed = "passed"
	CheckFailed = "failed"
	// CheckPartial and CheckNotRun both require the block to say what was
	// not checked and why.
	CheckPartial = "partial"
	CheckNotRun  = "not_run"
)

// CheckStatuses lists the allowed values of a check block's status.
var CheckStatuses = []string{CheckPassed, CheckFailed, CheckPartial, CheckNotRun}

// Levels are the values importance and severity take, least to most urgent.
var Levels = []string{"low", "medium", "high", "critical"}

// Option effects: what picking an option does.
const (
	// EffectDispatch sends the option's instruction as the next turn's prompt.
	EffectDispatch = "dispatch"
	// EffectCollect puts an answer in the composer to edit and send, rather
	// than starting a turn at once.
	EffectCollect = "collect"
	// EffectResolve records that the user is stopping here. No turn starts.
	EffectResolve = "resolve"
)

// Effects lists the allowed values of an option's effect.
var Effects = []string{EffectDispatch, EffectCollect, EffectResolve}

// ResponseSingle is the only response_type there is: one option, with free
// text always allowed alongside it.
const ResponseSingle = "single"

// Limits. They are the schema's documented bounds and the validator's checks,
// so a model reading the schema and the tool checking the result agree.
const (
	// HeadlineMax is the longest headline, in characters.
	HeadlineMax = 160
	// SummaryMax is the longest block summary, in characters.
	SummaryMax = 240
	// DetailsMax is the largest details field, in bytes.
	DetailsMax = 32 * 1024
	// LabelMax is the longest option label, in characters.
	LabelMax = 60
	// TextMax is the longest option description or consequence.
	TextMax = 200
	// PreviewMax is the largest option preview, in bytes.
	PreviewMax = 4 * 1024
	// InstructionMax is the largest dispatch instruction, in bytes.
	InstructionMax = 1024
	// IDMax is the longest block or option id, in characters.
	IDMax = 64
	// MinOptions and MaxOptions bound an interactive block's option list.
	MinOptions = 2
	MaxOptions = 6
	// DefaultMaxBlocks is the block limit when configuration sets none.
	DefaultMaxBlocks = 20
)

// Option is one way forward offered by an interactive block.
type Option struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
	Consequence string `json:"consequence,omitempty"`
	Preview     string `json:"preview,omitempty"`
	Effect      string `json:"effect"`
	// Instruction is what a dispatch option sends as the next prompt. It is
	// shown in full on the card, so the user sees what picking it will do.
	Instruction string `json:"instruction,omitempty"`
}

// Block is one typed item of a report. Every block type shares id and summary;
// the rest of the fields apply to the types that document them, and the
// validator rejects a block that omits one its type requires.
//
// One flat struct rather than one per type is deliberate: it keeps decoding to a plain
// json.Unmarshal, and it lets the validator report every problem in a report at
// once instead of stopping at the first type it cannot decode.
type Block struct {
	Type    BlockType `json:"type"`
	ID      string    `json:"id"`
	Summary string    `json:"summary"`

	// Shared optional fields.
	Impact         string   `json:"impact,omitempty"`
	Recommendation string   `json:"recommendation,omitempty"`
	Details        string   `json:"details,omitempty"`
	RelatedFiles   []string `json:"related_files,omitempty"`

	// finding
	Importance string `json:"importance,omitempty"`

	// check
	Status           string `json:"status,omitempty"`
	Evidence         string `json:"evidence,omitempty"`
	Limitation       string `json:"limitation,omitempty"`
	RequiredToVerify string `json:"required_to_verify,omitempty"`

	// warning
	Severity string `json:"severity,omitempty"`

	// question, next
	Options              []Option `json:"options,omitempty"`
	ResponseType         string   `json:"response_type,omitempty"`
	RecommendationReason string   `json:"recommendation_reason,omitempty"`

	// unclassified
	ReasonUnclassified string `json:"reason_unclassified,omitempty"`
	SuggestedType      string `json:"suggested_type,omitempty"`
}

// Report is the whole thing: what the turn amounted to, one sentence about it,
// and the typed items behind that sentence.
type Report struct {
	TaskStatus Status  `json:"task_status"`
	Headline   string  `json:"headline"`
	Blocks     []Block `json:"blocks"`
}

// Answer is the prompt that replies to a block with one of its options: the
// block's id in brackets, so the next turn can tell which question is being
// answered, then the option's label.
func Answer(blockID, label string) string {
	return fmt.Sprintf("[report %s] %s", blockID, label)
}

// Prompt is what picking o sends as the next turn's prompt, or "" when
// picking it starts no turn. A dispatch option sends its instruction. On a
// question a dispatch option may leave the instruction out, and then picking
// it sends the option's label as the answer — which is what makes the answer
// reach the model at all.
func (b *Block) Prompt(o *Option) string {
	if b == nil || o == nil || o.Effect != EffectDispatch {
		return ""
	}
	if o.Instruction != "" {
		return o.Instruction
	}
	return Answer(b.ID, o.Label)
}

// Interactive reports whether this block asks the user for something, which is
// what makes it answerable from the card and what holds back an inbox-only
// turn while it waits.
func (b *Block) Interactive() bool {
	switch b.Type {
	case BlockQuestion, BlockNext:
		return true
	}
	return false
}

// HasInteractive reports whether any block asks the user for something.
func (r *Report) HasInteractive() bool {
	if r == nil {
		return false
	}
	for i := range r.Blocks {
		if r.Blocks[i].Interactive() {
			return true
		}
	}
	return false
}

// buriedProposalMarks are prose phrases that usually mean the model already
// has a follow-up in mind but wrote it as text instead of as an interactive
// block. The list is deliberately narrow: it should fire only when the report
// half-proposes the action itself, not on every rejection.
var buriedProposalMarks = []string{
	"say the word",
	"say so and i",
	"just say",
	"just tell me",
	"pick one",
	"how to fix",
	"want me to",
	"shall i",
	"i did not edit",
	"i have not edited",
}

// BuriedProposalHint reports whether a rejected report proposes a follow-up
// in prose while offering no interactive block for it. The caller appends a
// one-line hint telling the model to expose each proposal as its own
// next block instead.
func BuriedProposalHint(r *Report) bool {
	if r == nil || r.HasInteractive() {
		return false
	}
	for i := range r.Blocks {
		text := strings.ToLower(r.Blocks[i].Summary + "\n" + r.Blocks[i].Details)
		for _, mark := range buriedProposalMarks {
			if strings.Contains(text, mark) {
				return true
			}
		}
	}
	return false
}

// FirstInteractive returns the index of the first block, in display order,
// that asks the user something, or -1. It is what a digit key answers.
func (r *Report) FirstInteractive() int {
	if r == nil {
		return -1
	}
	for _, i := range r.Order() {
		if r.Blocks[i].Interactive() {
			return i
		}
	}
	return -1
}

// PrimaryInteractive returns the index of the report's primary recommendation:
// the first block, in display order, whose recommendation names one of its
// options. The card highlights only this option as the pick and the plain
// rendering introduces it with "Recommended because:"; further recommended
// blocks are secondary. It returns -1 when nothing recommends anything,
// which is valid for a question with no safe default.
func (r *Report) PrimaryInteractive() int {
	if r == nil {
		return -1
	}
	for _, i := range r.Order() {
		if b := &r.Blocks[i]; b.Interactive() && b.Option(b.Recommendation) != nil {
			return i
		}
	}
	return -1
}

// PrimaryChoice returns the choice the primary recommendation names, or nil
// when nothing recommends anything. It is what an empty Enter picks: the
// same option a digit would pick, without needing its number. Options past
// the ninth have no digit (Number 0) but are still pickable this way, so the
// number is recorded for the scrollback rather than used as a key.
func (r *Report) PrimaryChoice() *Choice {
	if r == nil {
		return nil
	}
	pi := r.PrimaryInteractive()
	if pi < 0 {
		return nil
	}
	b := &r.Blocks[pi]
	oi := -1
	for i := range b.Options {
		if b.Options[i].ID == b.Recommendation {
			oi = i
			break
		}
	}
	if oi < 0 {
		return nil
	}
	for _, c := range r.Choices() {
		if c.Block == pi && c.Option == oi {
			cc := c
			return &cc
		}
	}
	return nil
}

// Block returns the block with the given id, or nil.
func (r *Report) Block(id string) *Block {
	if r == nil {
		return nil
	}
	for i := range r.Blocks {
		if r.Blocks[i].ID == id {
			return &r.Blocks[i]
		}
	}
	return nil
}

// Option returns the option with the given id from the given block, or nil.
func (b *Block) Option(id string) *Option {
	if b == nil {
		return nil
	}
	for i := range b.Options {
		if b.Options[i].ID == id {
			return &b.Options[i]
		}
	}
	return nil
}

// Label is the block type as it appears at the head of its line.
func (t BlockType) Label() string {
	switch t {
	case BlockChange:
		return "Change"
	case BlockFinding:
		return "Finding"
	case BlockCheck:
		return "Check"
	case BlockWarning:
		return "Warning"
	case BlockQuestion:
		return "Question"
	case BlockNext:
		return "Next"
	case BlockUnclassified:
		return "Note"
	}
	return string(t)
}

// Glyph is the one-character marker at the start of a block's line. A
// check's glyph says how the check went, so the outcome is legible without
// reading the line.
func (b *Block) Glyph() string {
	switch b.Type {
	case BlockCheck:
		switch b.Status {
		case CheckPassed:
			return "✓"
		case CheckFailed:
			return "✗"
		case CheckPartial:
			return "◐"
		default:
			return "○"
		}
	case BlockQuestion, BlockNext:
		return "?"
	case BlockWarning:
		return "⚠"
	case BlockFinding:
		return "•"
	case BlockChange:
		return "δ"
	}
	return "~"
}

// typeRank orders block types for display: what the turn did — warning,
// change, finding, check, unclassified — then what it asks the user
// (question, next), which the card draws nearest the prompt.
func typeRank(t BlockType) int {
	switch t {
	case BlockWarning:
		return 0
	case BlockChange:
		return 1
	case BlockFinding:
		return 2
	case BlockCheck:
		return 3
	case BlockQuestion:
		return 5
	case BlockNext:
		return 6
	}
	return 4
}

// levelRank orders severity and importance, most urgent first. An unset or
// unrecognized level sorts last within its type.
func levelRank(level string) int {
	for i, l := range Levels {
		if l == level {
			return len(Levels) - i
		}
	}
	return len(Levels) + 1
}

// Order returns the block indices in display order: warning, change,
// finding, check, unclassified, question, next —
// regardless of the order the model sent them in. Blocks that tie keep the
// order they were sent in, so a report renders the same way every time.
func (r *Report) Order() []int {
	if r == nil {
		return nil
	}
	idx := make([]int, len(r.Blocks))
	for i := range idx {
		idx[i] = i
	}
	sort.SliceStable(idx, func(a, b int) bool {
		ba, bb := &r.Blocks[idx[a]], &r.Blocks[idx[b]]
		ra, rb := typeRank(ba.Type), typeRank(bb.Type)
		if ra != rb {
			return ra < rb
		}
		switch ba.Type {
		case BlockWarning:
			return levelRank(ba.Severity) < levelRank(bb.Severity)
		case BlockFinding:
			return levelRank(ba.Importance) < levelRank(bb.Importance)
		}
		return false
	})
	return idx
}

// JSONSchema is the tool's parameter schema as the model sees it.
//
// The block types share one object rather than being expressed as a
// discriminated union: several endpoints handle oneOf poorly, and a model that
// cannot decode the schema sends nothing usable at all. Each field's
// description says which types require it, and Validate is what actually
// enforces that, reporting every violation at once so a wrong guess costs one
// round trip rather than one per mistake.
func JSONSchema(maxBlocks int) map[string]any {
	if maxBlocks <= 0 {
		maxBlocks = DefaultMaxBlocks
	}
	str := func(desc string) map[string]any {
		return map[string]any{"type": "string", "description": desc}
	}
	enum := func(desc string, values []string) map[string]any {
		vals := make([]any, len(values))
		for i, v := range values {
			vals[i] = v
		}
		return map[string]any{"type": "string", "description": desc, "enum": vals}
	}

	option := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":          str("Short kebab-case name for this option, unique within the block."),
			"label":       str(fmt.Sprintf("The choice in two to four words, at most %d characters. The card shows a block's labels side by side on one line.", LabelMax)),
			"description": str(fmt.Sprintf("What choosing it means, at most %d characters.", TextMax)),
			"consequence": str(fmt.Sprintf("What follows from choosing it, at most %d characters.", TextMax)),
			"preview":     str("Optional excerpt of what this option would produce."),
			"effect": enum("What picking it does. 'dispatch' sends this option's instruction as the next prompt; "+
				"'collect' puts an answer in the composer to edit and send; 'resolve' records a stop, starts no turn, "+
				"and tells you nothing. On a 'question', an option that answers it is 'dispatch'.",
				Effects),
			"instruction": str("For a 'dispatch' option only: the prompt picking it will send, written as the instruction " +
				"you want to receive, in one sentence. Required, except on a 'question': there, leaving it out sends the option's label " +
				"to you as the answer."),
		},
		"required": []string{"id", "label", "effect"},
	}

	block := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"type": enum("Which kind of item this is. 'change' is work output; 'finding' is something learned; "+
				"'check' is how the work was checked; 'warning' is a risk; 'question' is something only the user can "+
				"settle — a choice between ways forward, or information only they have; "+
				"'next' is a proposed continuation the user accepts or redirects — emit one such block per proposal "+
				"when there are several (e.g. apply the fix, commit it); 'unclassified' is the escape hatch for something "+
				"you must say that fits no other type, and is not for routine use.",
				blockTypeStrings()),
			"id":      str("Short kebab-case name for this block, unique within the report, such as \"reorder-tests\"."),
			"summary": str(fmt.Sprintf("The block's fact in one sentence that stands on its own, at most %d characters; aim for one line of about 90. On the card it is the block's whole row, with every other field folded away, so it never depends on them. On a 'question' it is the question itself; on a 'next', the proposal.", SummaryMax)),

			"impact": str("Required on 'finding' (why it matters) and 'warning' (what breaks if ignored). Optional elsewhere: what changed for the user, in one line."),

			"recommendation": str("On 'next': the id of the option you recommend. Required on the primary proposal only — one recommended block per report; further 'next' blocks omit it and are secondary. On 'question': the same, when a safe default exists. On 'finding' and 'warning': free text saying what to do."),

			"details": str(fmt.Sprintf("Markdown evidence, logs and file-by-file notes, at most %d bytes. Hidden until the user opens it, so nothing here is needed to understand the summary — and nothing the user must act on lives only here; propose continuations as 'next' blocks instead.", DetailsMax)),

			"related_files": map[string]any{"type": "array", "description": "Workspace-relative paths this block is about.", "items": map[string]any{"type": "string"}},

			"importance": enum("Optional on 'finding': how much it matters, when that is worth saying.", Levels),

			"status": enum("Required on 'check': how the check went.", CheckStatuses),

			"evidence": str("On 'check': the command or check behind the outcome."),

			"limitation":            str("Required on a 'partial' or 'not_run' check: what was not checked, and why."),
			"required_to_verify":    str("On 'check': what would be needed to check it. Accepted in place of 'limitation'."),
			"severity":              enum("Required on 'warning': how bad it is.", Levels),
			"options":               map[string]any{"type": "array", "description": fmt.Sprintf("Required on 'question' and 'next': %d to %d ways forward. A 'next' block needs one 'dispatch' accept option and at least one 'collect' or 'resolve' alternative.", MinOptions, MaxOptions), "items": option},
			"response_type":         enum("Required on any block with options. Free text is always allowed alongside them.", []string{ResponseSingle}),
			"recommendation_reason": str("Required on any block with options: why you recommend what you recommend, or why no safe default can be inferred, in one short sentence. The card shows the primary block's reason as 'Recommended because: ...' and any further recommended block as 'Suggested for this block because: ...'. The card lets the user pick one option per turn, so recommend one block per report."),
			"reason_unclassified":   str("Required on 'unclassified': why no other block type fits."),
			"suggested_type":        str("On 'unclassified': the type that came closest."),
		},
		"required": []string{"type", "id", "summary"},
	}

	// Note: task_status is deliberately absent from the schema. It stays on
// the struct only so reports saved by older sessions still decode; new
// reports omit it and Normalize coerces any stale value into agreement
// with the blocks.
return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"headline": str(fmt.Sprintf("One sentence, at most %d characters, no line break. It is the part that gets read, so put the "+
				"material fact in it rather than a label for it.", HeadlineMax)),
			"blocks": map[string]any{
				"type": "array",
				"description": fmt.Sprintf("The typed items behind the headline, 0 to %d of them. Fewer is better: several routine steps "+
					"belong in one block's details, not one block each — but each proposed continuation gets its own 'next' block.", maxBlocks),
				"items": block,
			},
		},
		"required": []string{"headline", "blocks"},
	}
}

func blockTypeStrings() []string {
	out := make([]string, len(BlockTypes))
	for i, t := range BlockTypes {
		out[i] = string(t)
	}
	return out
}

// Choice is one option the report offers, located by the block and option it
// sits in and numbered as the card shows it.
type Choice struct {
	// Block indexes Report.Blocks; Option indexes that block's Options.
	Block, Option int
	// Number is what the user presses, 1-based. Options past the ninth get 0:
	// there is no key for them, so the card shows them without a number.
	Number int
}

// MaxChoices is how many options one card can offer to a keypress, which is
// how many digits there are to press.
const MaxChoices = 9

// Choices lists every option in the report, in display order, numbered
// continuously across blocks. Numbering across blocks rather than within one
// selected block is what lets a digit answer any question the report asks
// without a selection to move first.
func (r *Report) Choices() []Choice {
	if r == nil {
		return nil
	}
	var out []Choice
	n := 0
	for _, bi := range r.Order() {
		b := &r.Blocks[bi]
		if !b.Interactive() {
			continue
		}
		for oi := range b.Options {
			c := Choice{Block: bi, Option: oi}
			if n < MaxChoices {
				n++
				c.Number = n
			}
			out = append(out, c)
		}
	}
	return out
}

// Choice returns what the given key number picks, or nil.
func (r *Report) Choice(number int) *Choice {
	for i, c := range r.Choices() {
		if c.Number == number {
			return &r.Choices()[i]
		}
	}
	return nil
}

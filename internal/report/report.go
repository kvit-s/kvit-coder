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

import "sort"

// Status is the task_status field: what the turn amounted to.
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
	// something outside the session.
	StatusBlocked Status = "blocked"
	// StatusFailed means the work did not succeed, with the reason recorded.
	StatusFailed Status = "failed"
)

// Statuses lists every status a model may send, in the order they are
// documented. The list is the schema's enum and the validator's allowed set,
// so the two cannot drift.
var Statuses = []Status{
	StatusCompleted,
	StatusCompletedWithNotes,
	StatusNeedsAction,
	StatusBlocked,
	StatusFailed,
}

// BlockType selects which shape a block has.
type BlockType string

const (
	// BlockChange is file edits and other work output.
	BlockChange BlockType = "change"
	// BlockFinding is something learned that matters to the task.
	BlockFinding BlockType = "finding"
	// BlockVerification is how the work was checked.
	BlockVerification BlockType = "verification"
	// BlockWarning is a risk or caveat.
	BlockWarning BlockType = "warning"
	// BlockDecision is a choice only the user can make.
	BlockDecision BlockType = "decision"
	// BlockQuestion is information the user has to supply.
	BlockQuestion BlockType = "question"
	// BlockBlocked is work that cannot continue.
	BlockBlocked BlockType = "blocked"
	// BlockNextStep is a proposed continuation the user accepts or redirects.
	BlockNextStep BlockType = "next_step"
	// BlockUnclassified is the last resort when nothing else fits.
	BlockUnclassified BlockType = "unclassified"
)

// BlockTypes lists every block type a model may send.
var BlockTypes = []BlockType{
	BlockChange,
	BlockFinding,
	BlockVerification,
	BlockWarning,
	BlockDecision,
	BlockQuestion,
	BlockBlocked,
	BlockNextStep,
	BlockUnclassified,
}

// Verification statuses.
const (
	VerifyPassed = "passed"
	VerifyFailed = "failed"
	// VerifyPartial and VerifyNotRun both require the block to say what was
	// not verified and why.
	VerifyPartial = "partial"
	VerifyNotRun  = "not_run"
)

// VerifyStatuses lists the allowed values of a verification block's status.
var VerifyStatuses = []string{VerifyPassed, VerifyFailed, VerifyPartial, VerifyNotRun}

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
// One flat struct rather than nine is deliberate: it keeps decoding to a plain
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
	Importance        string `json:"importance,omitempty"`
	BlocksCurrentTask *bool  `json:"blocks_current_task,omitempty"`

	// verification
	Status           string `json:"status,omitempty"`
	Evidence         string `json:"evidence,omitempty"`
	Limitation       string `json:"limitation,omitempty"`
	RequiredToVerify string `json:"required_to_verify,omitempty"`

	// warning
	Severity string `json:"severity,omitempty"`

	// question
	Question string `json:"question,omitempty"`

	// blocked
	Blocker        string `json:"blocker,omitempty"`
	RequiredAction string `json:"required_action,omitempty"`

	// decision, question, blocked, next_step
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

// Interactive reports whether this block asks the user for something, which is
// what makes it answerable from the card and what holds back an inbox-only
// turn while it waits.
func (b *Block) Interactive() bool {
	switch b.Type {
	case BlockDecision, BlockQuestion, BlockBlocked, BlockNextStep:
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

// Chip is the status as it appears at the head of the card.
func (s Status) Chip() string {
	switch s {
	case StatusCompleted:
		return "DONE"
	case StatusCompletedWithNotes:
		return "DONE · NOTES"
	case StatusNeedsAction:
		return "NEEDS DECISION"
	case StatusBlocked:
		return "BLOCKED"
	case StatusFailed:
		return "FAILED"
	}
	return string(s)
}

// Label is the block type as it appears at the head of its line.
func (t BlockType) Label() string {
	switch t {
	case BlockChange:
		return "Change"
	case BlockFinding:
		return "Finding"
	case BlockVerification:
		return "Verification"
	case BlockWarning:
		return "Warning"
	case BlockDecision:
		return "Decision"
	case BlockQuestion:
		return "Question"
	case BlockBlocked:
		return "Blocked"
	case BlockNextStep:
		return "Next step"
	case BlockUnclassified:
		return "Note"
	}
	return string(t)
}

// Glyph is the one-character marker at the start of a block's line. A
// verification's glyph says how the check went, so the outcome is legible
// without reading the line.
func (b *Block) Glyph() string {
	switch b.Type {
	case BlockVerification:
		switch b.Status {
		case VerifyPassed:
			return "✓"
		case VerifyFailed:
			return "✗"
		case VerifyPartial:
			return "◐"
		default:
			return "○"
		}
	case BlockDecision, BlockQuestion, BlockNextStep:
		return "?"
	case BlockBlocked:
		return "⛔"
	case BlockWarning:
		return "⚠"
	case BlockFinding:
		return "•"
	case BlockChange:
		return "Δ"
	}
	return "~"
}

// typeRank orders block types by how much they need attention: what stops the
// work, then what asks a question, then what warns, then what was learned,
// then what was checked and what changed.
func typeRank(t BlockType) int {
	switch t {
	case BlockBlocked:
		return 0
	case BlockDecision:
		return 1
	case BlockQuestion:
		return 2
	case BlockNextStep:
		return 3
	case BlockWarning:
		return 4
	case BlockFinding:
		return 5
	case BlockVerification:
		return 6
	case BlockChange:
		return 7
	}
	return 8
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

// Order returns the block indices in display order: by attention, not by the
// order the model happened to send them. Blocks that tie keep the order they
// were sent in, so a report renders the same way every time.
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

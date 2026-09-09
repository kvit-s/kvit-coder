package report

import (
	"fmt"
	"strings"
)

// LineKind says what a rendered line is, so a caller that paints the report —
// the composer's card — can style each kind without parsing the text back
// apart. The plain renderer ignores it.
type LineKind int

const (
	// LineHeader is the status chip and the headline.
	LineHeader LineKind = iota
	// LineBlock is a block's own line: glyph, kind, id, summary.
	LineBlock
	// LineField is one of a block's typed fields, indented under it.
	LineField
	// LineOption is one numbered option of an interactive block.
	LineOption
	// LineInstruction is the instruction a dispatch option would run.
	LineInstruction
	// LineDetails is one line of a block's Markdown details.
	LineDetails
)

// Line is one rendered line with enough structure to style or act on it.
type Line struct {
	Kind LineKind
	Text string
	// Block is the index into Report.Blocks, or -1 for the header.
	Block int
	// BlockID is that block's id, for callers that track selection by id.
	BlockID string
	// Option is the 1-based number of the option this line offers, or 0.
	// A digit key on the card acts on the option with this number.
	Option int
}

// Options controls how much of a report is drawn.
type Options struct {
	// Expanded holds the ids of blocks whose details are shown.
	Expanded map[string]bool
	// ExpandAll shows every block's details, which is what a headless run and
	// the full viewer do.
	ExpandAll bool
	// Active is the id of the block whose options are numbered. Empty numbers
	// the first block that asks the user something.
	Active string
}

const indent = "  "

// Lines renders a report as structured lines in display order.
func Lines(r *Report, opts Options) []Line {
	if r == nil {
		return nil
	}
	out := []Line{{
		Kind:  LineHeader,
		Block: -1,
		Text:  r.TaskStatus.Chip() + " · " + r.Headline,
	}}

	active := opts.Active
	if active == "" || r.Block(active) == nil || !r.Block(active).Interactive() {
		if i := r.FirstInteractive(); i >= 0 {
			active = r.Blocks[i].ID
		}
	}

	for _, i := range r.Order() {
		b := &r.Blocks[i]
		add := func(kind LineKind, option int, format string, args ...any) {
			out = append(out, Line{
				Kind: kind, Block: i, BlockID: b.ID, Option: option,
				Text: fmt.Sprintf(format, args...),
			})
		}

		add(LineBlock, 0, "%s %s [%s]: %s", b.Glyph(), b.Type.Label(), b.ID, b.Summary)

		hidden := b.Details != "" && !opts.ExpandAll && !opts.Expanded[b.ID]
		for _, f := range blockFields(b, hidden) {
			add(LineField, 0, "%s%s", indent, f)
		}

		if b.Interactive() {
			numbered := b.ID == active
			for j := range b.Options {
				o := &b.Options[j]
				n := 0
				marker := "-"
				if numbered {
					n = j + 1
					marker = fmt.Sprintf("%d)", n)
				}
				line := fmt.Sprintf("%s%s %s", indent, marker, o.Label)
				if o.ID == b.Recommendation {
					line += " (recommended)"
				}
				if o.Description != "" {
					line += " — " + o.Description
				}
				add(LineOption, n, "%s", line)
				if o.Consequence != "" {
					add(LineOption, n, "%s%s%s", indent, indent+indent, o.Consequence)
				}
				// The instruction is shown in full: it is what picking the
				// option will send, and agreeing to something unseen is not a
				// choice.
				if o.Effect == EffectDispatch && o.Instruction != "" {
					for _, l := range strings.Split(o.Instruction, "\n") {
						add(LineInstruction, n, "%s%s→ %s", indent, indent, l)
					}
				}
			}
		}

		if b.Details != "" && (opts.ExpandAll || opts.Expanded[b.ID]) {
			for _, l := range strings.Split(strings.TrimRight(b.Details, "\n"), "\n") {
				add(LineDetails, 0, "%s%s%s", indent, indent, l)
			}
		}
	}
	return out
}

// blockFields is the typed fields of one block, in the order they are read:
// what it means, then how sure it is, then what to do about it.
func blockFields(b *Block, detailsHidden bool) []string {
	var out []string
	add := func(label, value string) {
		if value != "" {
			out = append(out, label+": "+value)
		}
	}

	switch b.Type {
	case BlockChange:
		add("Impact", b.Impact)
	case BlockFinding:
		add("Impact", b.Impact)
		if b.Importance != "" {
			line := "Importance: " + b.Importance
			if b.BlocksCurrentTask != nil && *b.BlocksCurrentTask {
				line += " (blocks this task)"
			}
			out = append(out, line)
		}
		add("Recommendation", b.Recommendation)
	case BlockVerification:
		add("Evidence", b.Evidence)
		add("Not verified", b.Limitation)
		add("Would need", b.RequiredToVerify)
	case BlockWarning:
		add("Severity", b.Severity)
		add("Impact", b.Impact)
		add("Recommendation", b.Recommendation)
	case BlockQuestion:
		if b.Question != "" {
			out = append(out, b.Question)
		}
	case BlockBlocked:
		add("Blocked by", b.Blocker)
		add("Needs", b.RequiredAction)
	case BlockUnclassified:
		add("Unclassified because", b.ReasonUnclassified)
		add("Closest type", b.SuggestedType)
	}

	if b.Interactive() {
		if rec := b.Option(b.Recommendation); rec != nil {
			add("Recommended", rec.Label)
			add("Because", b.RecommendationReason)
		} else {
			add("No recommendation", b.RecommendationReason)
		}
	}

	if len(b.RelatedFiles) > 0 {
		add("Files", strings.Join(b.RelatedFiles, ", "))
	}
	if detailsHidden {
		out = append(out, fmt.Sprintf("(%d bytes of detail, hidden)", len(b.Details)))
	}
	return out
}

// Render is the report as plain text, which is what a headless run prints and
// what the clipboard gets.
func Render(r *Report, opts Options) string {
	lines := Lines(r, opts)
	texts := make([]string, len(lines))
	for i, l := range lines {
		texts[i] = l.Text
	}
	return strings.Join(texts, "\n")
}

// PlainText is the whole report with every detail shown.
func (r *Report) PlainText() string {
	return Render(r, Options{ExpandAll: true})
}

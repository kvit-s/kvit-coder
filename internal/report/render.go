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
	// LineHeader is the headline. The status is not drawn: what the turn
	// amounted to is in the headline, and the card shows what to answer.
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
	// HideNotRun collapses check blocks that ran nothing into a
	// one-line footnote, which is what the live card does: the check stays
	// mandatory, but a docs-only edit no longer spends two lines saying so.
	// ExpandAll or an entry in Expanded still shows the block in full, and
	// the scrollback transcript (which expands everything) is unaffected.
	HideNotRun bool
}

// IsNotRunCheck reports whether this block is a check that ran nothing.
// Those are the rows the live card collapses: required by the validator,
// noise on the card.
func (b *Block) IsNotRunCheck() bool {
	return b != nil && b.Type == BlockCheck && b.Status == CheckNotRun
}

// HiddenNotRunCount is how many not_run checks opts would collapse.
// The card uses it for its footnote and for offering the expand key even
// when there are no details to show.
func (r *Report) HiddenNotRunCount(opts Options) int {
	if r == nil || !opts.HideNotRun || opts.ExpandAll {
		return 0
	}
	n := 0
	for i := range r.Blocks {
		if b := &r.Blocks[i]; b.IsNotRunCheck() && !opts.Expanded[b.ID] {
			n++
		}
	}
	return n
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
		Text:  r.Headline,
	}}

	// Every option is numbered, continuously across blocks, so a digit
	// reaches any of them without a selection to move first.
	numbers := map[[2]int]int{}
	for _, c := range r.Choices() {
		numbers[[2]int{c.Block, c.Option}] = c.Number
	}

	for _, i := range r.Order() {
		b := &r.Blocks[i]
		// A not_run check the card collapses: skip its lines unless the
		// block was expanded (or everything was). The footnote below keeps
		// the signal; the transcript and headless output expand everything
		// and never collapse.
		if opts.HideNotRun && !opts.ExpandAll && !opts.Expanded[b.ID] && b.IsNotRunCheck() {
			continue
		}
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
			for j := range b.Options {
				o := &b.Options[j]
				n := numbers[[2]int{i, j}]
				marker := "-"
				if n > 0 {
					marker = fmt.Sprintf("%d)", n)
				}

				recommended := o.ID == b.Recommendation
				line := fmt.Sprintf("%s%s %s", indent, marker, o.Label)
				if recommended {
					line += " (recommended)"
				}
				// What choosing it means and what follows from it share the
				// option's line, so an option costs one line before its
				// instruction rather than two.
				if about := sentences(o.Description, o.Consequence); about != "" {
					line += " — " + about
				}
				add(LineOption, n, "%s", line)

				// The instruction is given in full: it is what picking the
				// option will send. The live card cuts it to one line until 0
				// is pressed; the scrollback copy and headless output show it
				// all.
				if o.Effect == EffectDispatch && o.Instruction != "" {
					for _, l := range strings.Split(o.Instruction, "\n") {
						add(LineInstruction, n, "%s%s→ %s", indent, indent, l)
					}
				}

				// The reason sits under the option it argues for. The option
				// line already says which one that is, so the block does not
				// name it a second time.
				if recommended && b.RecommendationReason != "" {
					add(LineField, n, "%s%sWhy: %s", indent, indent, b.RecommendationReason)
				}
			}
		}

		if b.Details != "" && (opts.ExpandAll || opts.Expanded[b.ID]) {
			for _, l := range strings.Split(strings.TrimRight(b.Details, "\n"), "\n") {
				add(LineDetails, 0, "%s%s%s", indent, indent, l)
			}
		}
	}

	// One dim line stands in for every collapsed check, so the card stays
	// honest about what it hid without spending a block per check.
	if n := r.HiddenNotRunCount(opts); n > 0 {
		checks := "check"
		if n > 1 {
			checks = "checks"
		}
		out = append(out, Line{
			Kind:  LineField,
			Block: -1,
			Text:  fmt.Sprintf("%s○ %d %s not run (hidden)", indent, n, checks),
		})
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
		add("Importance", b.Importance)
		add("Recommendation", b.Recommendation)
	case BlockCheck:
		add("Evidence", b.Evidence)
		add("Not verified", b.Limitation)
		add("Would need", b.RequiredToVerify)
	case BlockWarning:
		add("Severity", b.Severity)
		add("Impact", b.Impact)
		add("Recommendation", b.Recommendation)
	case BlockUnclassified:
		add("Unclassified because", b.ReasonUnclassified)
		add("Closest type", b.SuggestedType)
	}

	// A recommended option carries its reason on the option itself; only the
	// absence of one is said at the block.
	if b.Interactive() && b.Option(b.Recommendation) == nil {
		add("No recommendation", b.RecommendationReason)
	}

	if len(b.RelatedFiles) > 0 {
		add("Files", strings.Join(b.RelatedFiles, ", "))
	}
	if detailsHidden {
		out = append(out, fmt.Sprintf("(%d bytes of detail, hidden)", len(b.Details)))
	}
	return out
}

// sentences joins an option's description and consequence into one run of
// text, adding the full stop the first one usually lacks.
func sentences(first, second string) string {
	switch {
	case first == "":
		return second
	case second == "":
		return first
	}
	if !strings.ContainsAny(first[len(first)-1:], ".!?;:") {
		first += "."
	}
	return first + " " + second
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

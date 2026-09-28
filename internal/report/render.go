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
	// LineHeader is the headline, after a dot. The status is not drawn: what
	// the turn amounted to is in the headline, and the card shows what to
	// answer.
	LineHeader LineKind = iota
	// LineBlank separates the header, the sections and the blocks that ask
	// something.
	LineBlank
	// LineSection names one of the card's two parts: what the turn did, and
	// what it asks of the user.
	LineSection
	// LineBlock is a block's own line. For a block that reports something it
	// is the glyph and the summary; for one that asks something it is "?", the
	// type and the summary. Tag holds the part before the summary.
	LineBlock
	// LineField is one of a block's typed fields, shown when the block is
	// expanded.
	LineField
	// LineReason is why the block recommends what it recommends.
	LineReason
	// LineChoices is a block's options side by side, one entry in Choices each.
	LineChoices
	// LineOption is one option with its description and consequence, shown
	// when the block is expanded.
	LineOption
	// LineInstruction is what picking an option sends. Tag is "Sends:".
	LineInstruction
	// LineDetails is one line of a block's Markdown details.
	LineDetails
)

// Line is one rendered line with enough structure to style or act on it.
type Line struct {
	Kind LineKind
	// Text is the whole line as plain text, indentation included.
	Text string
	// Indent is how many columns of Text are indentation, and Tag and Body
	// are the rest split where a painter styles them differently: the glyph
	// or "? next" before a summary, "Sends:" before an instruction. Tag is
	// empty on lines that have no such split, and Body is then the rest.
	Indent int
	Tag    string
	Body   string
	// Block is the index into Report.Blocks, or -1 for lines that belong to
	// no block.
	Block int
	// BlockID is that block's id, for callers that track selection by id.
	BlockID string
	// Option is the 1-based number of the option this line is about, or 0.
	Option int
	// More says the block has fields or details this rendering folds away,
	// which the card marks at the end of the row.
	More bool
	// Choices are the options a LineChoices line offers, in order.
	Choices []LineChoice
}

// LineChoice is one option on a LineChoices line.
type LineChoice struct {

	// Number is the key that picks it, or 0 past the ninth.

	Number int
	Label  string
	// Recommended says this option is the block's pick. Primary says it is
	// also the report's primary recommendation — the one the card highlights
	// as the pick. A secondary recommended block keeps Recommended without
	// Primary, so the card can paint it as an outline chip.
	Recommended bool
	Primary     bool
}

// Marker is how the choice is written: "[1]", or "[-]" when no key picks it.
func (c LineChoice) Marker() string {
	if c.Number == 0 {
		return "[-]"
	}
	return fmt.Sprintf("[%d]", c.Number)
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

// Section titles.
const (
	SectionHappened = "What happened"
	SectionAsks     = "Your call"
)

// Lines renders a report as structured lines in display order: the headline,
// then what the turn did, then what it asks of the user, which is nearest the
// prompt where the answer is typed.
func Lines(r *Report, opts Options) []Line {
	if r == nil {
		return nil
	}
	head := "● "
	out := []Line{{Kind: LineHeader, Block: -1, Tag: "●", Body: r.Headline, Text: head + r.Headline}}

	// Every option is numbered, continuously across blocks, so a digit
	// reaches any of them without a selection to move first.
	numbers := map[[2]int]int{}
	for _, c := range r.Choices() {
		numbers[[2]int{c.Block, c.Option}] = c.Number
	}

	blank := func() { out = append(out, Line{Kind: LineBlank, Block: -1}) }
	section := func(title string) {
		blank()
		text := "── " + title + " ──"
		out = append(out, Line{Kind: LineSection, Block: -1, Body: text, Text: text})
		blank()
	}

	var reported, asking []int
	for _, i := range r.Order() {
		if r.Blocks[i].Interactive() {
			asking = append(asking, i)
		} else {
			reported = append(reported, i)
		}
	}

	hiddenNotRun := r.HiddenNotRunCount(opts)
	if len(reported) > 0 {
		section(SectionHappened)
	}
	for _, i := range reported {
		b := &r.Blocks[i]
		expanded := opts.ExpandAll || opts.Expanded[b.ID]
		// A not_run check the card collapses: skip its lines unless the
		// block was expanded. The footnote below keeps the signal.
		if opts.HideNotRun && !expanded && b.IsNotRunCheck() {
			continue
		}
		out = append(out, Line{
			Kind: LineBlock, Block: i, BlockID: b.ID,
			Tag: b.Glyph(), Body: b.Summary, Text: b.Glyph() + " " + b.Summary,
			More: !expanded && b.HasMore(),
		})
		if expanded {
			out = append(out, expandedLines(b, i, indent)...)
		}
	}
	// One dim row stands in for every collapsed check, so the card says what
	// it hid without spending a row per check.
	if hiddenNotRun > 0 {
		checks := "check"
		if hiddenNotRun > 1 {
			checks = "checks"
		}
		text := fmt.Sprintf("○ %d %s not run (hidden)", hiddenNotRun, checks)
		out = append(out, Line{Kind: LineField, Block: -1, Body: text, Text: text})
	}

	if len(asking) > 0 {
		section(SectionAsks)
	}

	// Only the primary recommendation gets the highlighted chip and the
	// "Recommended because:" line: the card lets the user pick one option per
	// turn, so further recommended blocks read as secondary.
	primary := r.PrimaryInteractive()

	for k, i := range asking {
		b := &r.Blocks[i]
		expanded := opts.ExpandAll || opts.Expanded[b.ID]
		if k > 0 {
			blank()
		}

		add := func(l Line) {
			l.Block, l.BlockID = i, b.ID
			if l.Text == "" {
				l.Text = strings.Repeat(" ", l.Indent) + joinTag(l.Tag, l.Body)
			}
			out = append(out, l)
		}

		tag := "? " + string(b.Type)
		add(Line{Kind: LineBlock, Tag: tag, Body: b.Summary, Text: tag + "  " + b.Summary})

		isPrimary := i == primary
		if rec := b.Option(b.Recommendation); rec != nil {
			if isPrimary {
				add(Line{Kind: LineReason, Indent: 2, Body: "# Recommended because: " + b.RecommendationReason})
			} else {
				add(Line{Kind: LineReason, Indent: 2, Body: "# Suggested for this block because: " + b.RecommendationReason})
			}
		} else if b.RecommendationReason != "" {
			add(Line{Kind: LineReason, Indent: 2, Body: "# No recommendation: " + b.RecommendationReason})
		}

		var choices []LineChoice
		var marks []string
		for j := range b.Options {
			recommended := b.Option(b.Recommendation) != nil && b.Options[j].ID == b.Recommendation
			c := LineChoice{Number: numbers[[2]int{i, j}], Label: b.Options[j].Label,
				Recommended: recommended, Primary: isPrimary && recommended}
			choices = append(choices, c)
			marks = append(marks, c.Marker()+" "+c.Label)
		}

		add(Line{Kind: LineChoices, Indent: 2, Choices: choices, Body: strings.Join(marks, "    ")})

		if !expanded {
			// What the likeliest pick sends: the recommended option's
			// instruction, or the first one there is. Every option's
			// instruction is shown when the block is expanded.
			if o := sendsOption(b); o != nil {
				add(Line{Kind: LineInstruction, Indent: 2, Option: numbers[[2]int{i, optionIndex(b, o)}],
					Tag: "Sends:", Body: o.Instruction})
			}
			continue
		}
		// Expanded, each option with more to say than its label gets a line:
		// what choosing it means, and what it sends. The rest are already
		// all there is to them on the choices line.
		for j := range b.Options {
			o := &b.Options[j]
			n := numbers[[2]int{i, j}]
			about := sentences(o.Description, o.Consequence)
			sends := o.Effect == EffectDispatch && o.Instruction != ""
			if about == "" && !sends {
				continue
			}
			body := choices[j].Marker() + " " + o.Label
			if about != "" {
				body += " — " + about
			}
			add(Line{Kind: LineOption, Indent: 2, Option: n, Body: body})
			// The instruction is given in full: it is what picking the
			// option will send. The live card cuts it to one line until 0
			// is pressed; the scrollback copy and headless output show it
			// all.
			if o.Effect == EffectDispatch && o.Instruction != "" {
				add(Line{Kind: LineInstruction, Indent: 6, Option: n, Tag: "Sends:", Body: o.Instruction})
			}
		}
		out = append(out, expandedLines(b, i, indent)...)
	}
	return out
}

// joinTag puts a tag before a body with one space, or returns the body alone.
func joinTag(tag, body string) string {
	if tag == "" {
		return body
	}
	return tag + " " + body
}

// sendsOption is the option whose instruction a collapsed block shows: the
// recommended one if it sends something, otherwise the first that does.
func sendsOption(b *Block) *Option {
	if o := b.Option(b.Recommendation); o != nil && o.Effect == EffectDispatch && o.Instruction != "" {
		return o
	}
	for j := range b.Options {
		if o := &b.Options[j]; o.Effect == EffectDispatch && o.Instruction != "" {
			return o
		}
	}
	return nil
}

func optionIndex(b *Block, o *Option) int {
	for j := range b.Options {
		if &b.Options[j] == o {
			return j
		}
	}
	return -1
}

// expandedLines is what opening a block adds under it: its typed fields, the
// files it is about, and its details.
func expandedLines(b *Block, i int, pad string) []Line {
	var out []Line
	for _, f := range blockFields(b) {
		out = append(out, Line{Kind: LineField, Block: i, BlockID: b.ID,
			Indent: len(pad), Body: f, Text: pad + f})
	}
	if b.Details != "" {
		for _, l := range strings.Split(strings.TrimRight(b.Details, "\n"), "\n") {
			out = append(out, Line{Kind: LineDetails, Block: i, BlockID: b.ID,
				Indent: len(pad) + 2, Body: l, Text: pad + "  " + l})
		}
	}
	return out
}

// HasMore reports whether the block has anything a collapsed rendering does
// not show: typed fields, files or details on a block that reports
// something; on one that asks, option descriptions and consequences, or
// instructions beyond the one shown.
func (b *Block) HasMore() bool {
	if b == nil {
		return false
	}
	if b.Details != "" || len(blockFields(b)) > 0 {
		return true
	}
	if !b.Interactive() {
		return false
	}
	sends := 0
	for j := range b.Options {
		o := &b.Options[j]
		if o.Description != "" || o.Consequence != "" {
			return true
		}
		if o.Effect == EffectDispatch && o.Instruction != "" {
			sends++
		}
	}
	return sends > 1
}

// blockFields is the typed fields of one block, in the order they are read:
// what it means, then how sure it is, then what to do about it. A block that
// asks something shows its reason on a line of its own, so it is not here.
func blockFields(b *Block) []string {
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

	if len(b.RelatedFiles) > 0 {
		add("Files", strings.Join(b.RelatedFiles, ", "))
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

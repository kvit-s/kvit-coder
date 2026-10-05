package ui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kvit-s/kvit-coder/internal/report"
)

// CardAnswer is what the user picked from the report card. The driver turns it
// into the next turn, or into nothing at all when the effect is "resolve".
type CardAnswer struct {
	// BlockID and OptionID say what was answered, for the record.
	BlockID  string
	OptionID string
	// Effect is one of report.EffectDispatch, EffectCollect or EffectResolve.
	Effect string
	// Prompt is the text the next turn runs, for a dispatch. Empty otherwise.
	Prompt string
	// Number is the digit the user pressed (1-based, as the card showed it),
	// and Label is the option's label. Both are recorded so a scrollback copy
	// of the report can say what was picked without looking the report up
	// again.
	Number int
	Label  string
}

// Describe returns the scrollback line for what the user picked, e.g.
// "[picked 1) Preserve hidden positions — filtered/preserve (dispatch, will run as next turn)]".
// It returns "" when there is nothing to describe.
func (a *CardAnswer) Describe() string {
	if a == nil {
		return ""
	}
	label := a.Label
	if label == "" {
		label = a.OptionID
	}
	where := a.BlockID
	if a.OptionID != "" {
		if where != "" {
			where += "/"
		}
		where += a.OptionID
	}
	pick := label
	if a.Number > 0 {
		pick = fmt.Sprintf("%d) %s", a.Number, label)
	}
	if where != "" {
		pick += " — " + where
	}
	switch a.Effect {
	case report.EffectDispatch:
		return fmt.Sprintf("[picked %s (dispatch, will run as next turn)]", pick)
	case report.EffectCollect:
		return fmt.Sprintf("[picked %s (collect, editing as draft)]", pick)
	case report.EffectResolve:
		return fmt.Sprintf("[picked %s (resolve, no turn)]", pick)
	default:
		if a.Effect != "" {
			return fmt.Sprintf("[picked %s (%s)]", pick, a.Effect)
		}
		return fmt.Sprintf("[picked %s]", pick)
	}
}

// SetReport installs the report the last turn ended with. Passing nil, which is
// what happens after any turn that did not end with one, leaves the composer
// exactly as it was before reports existed.
func (m *InputModel) SetReport(rep *report.Report) {
	m.card = rep
	m.cardDismissed = false
	m.cardExpanded = map[string]bool{}
	m.cardAnswer = nil
	m.cardOffset = 0
}

// CardAnswerPicked returns what the user chose from the card, or nil.
func (m InputModel) CardAnswerPicked() *CardAnswer { return m.cardAnswer }

// cardShowing reports whether the card is drawn and taking its keys.
func (m *InputModel) cardShowing() bool {
	return m.card != nil && !m.cardDismissed
}

// cardHolds reports whether the card is waiting on an answer, which is what
// stops the composer submitting itself when something lands in the inbox. A
// report with nothing to answer holds nothing: a finished build should not wait
// behind a card that only needs reading.
func (m *InputModel) cardHolds() bool {
	return m.cardShowing() && m.card.HasInteractive()
}

// cardOpts is what the live card draws with: not_run checks collapse
// to a one-line footnote, but an expanded block (or everything expanded)
// still shows in full. The transcript and headless output expand everything
// and never collapse.
func (m *InputModel) cardOpts() report.Options {
	return report.Options{Expanded: m.cardExpanded, HideNotRun: true}
}

// hiddenNotRun is how many not_run checks the card is collapsing.
func (m *InputModel) hiddenNotRun() int {
	if m.card == nil {
		return 0
	}
	return m.card.HiddenNotRunCount(m.cardOpts())
}

// anyDetails reports whether the card is holding anything back, which decides
// whether the footer offers the key that shows it: a block with folded
// fields or details, a collapsed not_run check, or an instruction cut to one
// line. While any of those stands revealed, 0 is the key that hides it again.
func (m *InputModel) anyDetails() bool {
	if m.card == nil {
		return false
	}
	if m.hiddenNotRun() > 0 {
		return true
	}
	for i := range m.card.Blocks {
		b := &m.card.Blocks[i]
		if b.HasMore() || (b.IsNotRunCheck() && m.cardExpanded[b.ID]) {
			return true
		}
	}
	_, cut := cardRows(m.card, report.Options{HideNotRun: true}, m.cardWidth(), true)
	return cut
}

// expandable reports whether 0 has anything to show for this block: folded
// fields or details, a not_run check the card collapsed, or an instruction
// it may have cut short.
func expandable(b *report.Block) bool {
	if b.HasMore() || b.IsNotRunCheck() {
		return true
	}
	for i := range b.Options {
		if b.Options[i].Effect == report.EffectDispatch && b.Options[i].Instruction != "" {
			return true
		}
	}
	return false
}

// handleCardKey gives the card first refusal on a keypress. It takes only
// digits and esc, and the digits only while the text is empty, so the composer
// never traps a message: start typing and every key is the textarea's again.
// An empty Enter picks the primary recommendation too, but through Update's
// enter case rather than here, so staged images still submit as an image-only
// turn instead of picking.
//
// Digits are the whole keyboard interface deliberately. A bare letter would
// make every message starting with it unwritable for as long as a card is up,
// alt combinations do not survive every terminal — the VS Code terminal eats
// them — and the arrows are already history navigation. Numbering every option
// continuously across blocks (report.Choices) is what makes one digit enough to
// answer anything the report asks, with no selection to move first.
func (m *InputModel) handleCardKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if !m.cardShowing() {
		return false, nil
	}
	key := msg.String()

	if key == "esc" {
		// A completion list is nearer the cursor than the card, so it gets
		// the first esc and the card gets the next one.
		if m.completionListActive() || m.compNotice != "" {
			return false, nil
		}
		m.cardDismissed = true
		return true, nil
	}

	// A card taller than the terminal scrolls under pgup/pgdn (home/end jump
	// to the ends) whenever it is truncated, even mid-draft: the keys never
	// type text, and without them the top of an expanded report is
	// unreachable while the composer is open. Anything else falls through to
	// the history, completion and textarea keys below. home/end are cursor
	// keys while a draft is being typed, so they only jump the card on empty
	// text, like the digits; pgup/pgdn never edit and always scroll the card.
	switch key {
	case "pgup":
		if m.scrollCardUp() {
			return true, nil
		}
		return false, nil
	case "pgdown":
		if m.scrollCardDown() {
			return true, nil
		}
		return false, nil
	case "home":
		if m.textarea.Value() == "" && m.scrollCardTop() {
			return true, nil
		}
		return false, nil
	case "end":
		if m.textarea.Value() == "" && m.scrollCardBottom() {
			return true, nil
		}
		return false, nil
	}

	if len(key) == 1 && m.textarea.Value() == "" {
		switch {
		case key[0] >= '1' && key[0] <= '9':
			return m.pickOption(int(key[0] - '0'))
		case key[0] == '0' && m.anyDetails():
			m.toggleAllDetails()
			return true, nil
		}
	}
	return false, nil
}

// pickOption acts on the option the given digit shows.
func (m *InputModel) pickOption(n int) (bool, tea.Cmd) {
	c := m.card.Choice(n)
	if c == nil {
		return false, nil
	}
	return m.applyPick(c.Block, c.Option, n, false)
}

// pickRecommended acts on the report's primary recommendation: the option the
// card highlights as the pick. It is what an empty Enter takes, so a
// recommended action needs no digit. A dispatch submits its instruction, a
// collect stages its draft for editing, and a resolve records the pick and
// submits empty so the driver leaves the transcript and starts no turn.
func (m *InputModel) pickRecommended() (bool, tea.Cmd) {
	if !m.cardShowing() || m.card == nil {
		return false, nil
	}
	c := m.card.PrimaryChoice()
	if c == nil {
		return false, nil
	}
	return m.applyPick(c.Block, c.Option, c.Number, true)
}

// applyPick acts on one option by its block and option indices. number is the
// digit the card shows for it (0 past the ninth) and is recorded for the
// scrollback. fromEnter says the pick came from an empty Enter: a resolve
// then submits empty at once, instead of waiting for the second Enter a digit
// pick needs, because there is no follow-up to type.
func (m *InputModel) applyPick(blockIdx, optIdx, number int, fromEnter bool) (bool, tea.Cmd) {
	b := &m.card.Blocks[blockIdx]
	o := &b.Options[optIdx]
	m.cardAnswer = &CardAnswer{BlockID: b.ID, OptionID: o.ID, Effect: o.Effect, Number: number, Label: o.Label}

	switch o.Effect {
	case report.EffectDispatch:
		// The instruction is what the user read before choosing, so it is
		// what gets sent, word for word. An answer to a question with no
		// instruction sends its label, anchored to the question.
		prompt := b.Prompt(o)
		m.cardAnswer.Prompt = prompt
		m.value = prompt
		m.submitted = true
		m.quitting = true
		return true, tea.Quit
	case report.EffectCollect:
		// An answer in the user's own words, anchored to the block it
		// answers so the next turn knows what is being replied to.
		m.textarea.SetValue(report.Answer(b.ID, o.Label))
		m.textarea.CursorEnd()
		m.adjustHeight()
		m.cardDismissed = true
		return true, nil
	default:
		// resolve: recorded, and no turn starts.
		m.cardDismissed = true
		if fromEnter {
			m.value = ""
			m.submitted = true
			m.quitting = true
			return true, tea.Quit
		}
		return true, nil
	}
}

// toggleAllDetails opens every block's details, or closes them all when any is
// already open. Collapsed not_run checks and shortened instructions ride
// along: 0 reveals them, and 0 again hides them with the details.
func (m *InputModel) toggleAllDetails() {
	open := false
	for i := range m.card.Blocks {
		if b := &m.card.Blocks[i]; m.cardExpanded[b.ID] && expandable(b) {
			open = true
		}
	}
	for i := range m.card.Blocks {
		if b := &m.card.Blocks[i]; expandable(b) {
			m.cardExpanded[b.ID] = !open
		}
	}
	// Opening jumps back to the top so the headline and the first block are
	// what is visible; closing cannot leave the offset past the shorter card.
	m.cardOffset = 0
}

// Card colors. What the turn did is drawn in grey, because it is there to be
// skimmed; what it asks is drawn bright, with the primary recommendation picked
// out, because it is what gets answered. A secondary recommendation keeps an
// outline chip so two blocks never show two equal picks.
var (
	cardDim         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cardRow         = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	cardText        = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cardHeadline    = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	cardAccent      = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	cardFailed      = lipgloss.NewStyle().Foreground(lipgloss.Color("167"))
	cardPick        = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("78"))
	cardSecondary   = lipgloss.NewStyle().Foreground(lipgloss.Color("78")).Background(lipgloss.Color("235"))
	cardInstruction = lipgloss.NewStyle().Foreground(lipgloss.Color("75"))
)

// dotStyle colors the dot before the headline: red when the turn failed or is
// stuck, green otherwise.
func dotStyle(s report.Status) lipgloss.Style {
	if s == report.StatusFailed || s == report.StatusBlocked {
		return cardFailed
	}
	return cardAccent
}

// cardWidth is how wide the card may draw. It is the terminal less the two
// columns the card indents by, so a wrapped line stops at the same place the
// unwrapped ones do.
func (m InputModel) cardWidth() int {
	w := m.termCols
	if w <= 0 {
		w = 80
	}
	if w < 40 {
		w = 40
	}
	return w - 2
}

// renderCardLines lays out a report the way the card does — wrapped and
// painted — but without the card's key hint, so both the live card and the
// scrollback copy share one layout. shortInstructions is the live card's
// setting; see cardRows.
func renderCardLines(card *report.Report, opts report.Options, width int, shortInstructions bool) string {
	rows, _ := cardRows(card, opts, width, shortInstructions)
	if len(rows) == 0 {
		return ""
	}
	return strings.Join(rows, "\n") + "\n"
}

// cardRows wraps and paints the report's lines to the card's width. With
// shortInstructions, an instruction in a block that is not expanded keeps
// only its first row, ending in "…" when anything was cut, so a proposal does
// not spend several rows on what it would send. The second result says
// whether anything was cut, which is what makes the expand key worth
// offering.
func cardRows(card *report.Report, opts report.Options, width int, shortInstructions bool) ([]string, bool) {
	var rows []string
	cut := false
	for _, line := range report.Lines(card, opts) {
		expanded := opts.ExpandAll || opts.Expanded[line.BlockID]
		switch line.Kind {
		case report.LineBlank:
			rows = append(rows, "")

		case report.LineHeader:
			for i, row := range wrapCardLine(line.Text, width) {
				if i == 0 {
					rows = append(rows, dotStyle(card.TaskStatus).Render(line.Tag)+" "+
						cardHeadline.Render(strings.TrimPrefix(row, line.Tag+" ")))
				} else {
					rows = append(rows, cardHeadline.Render(row))
				}
			}

		case report.LineSection:
			rows = append(rows, cardDim.Render(line.Text))

		case report.LineBlock:
			if line.Block >= 0 && card.Blocks[line.Block].Interactive() {
				for i, row := range wrapLine(line.Tag+"  ", line.Body, width, "  ") {
					if i == 0 {
						rows = append(rows, cardAccent.Render(line.Tag)+
							cardHeadline.Render(strings.TrimPrefix(row, line.Tag)))
					} else {
						rows = append(rows, cardHeadline.Render(row))
					}
				}
				continue
			}
			// A row of what happened: one line to skim, with a mark at the
			// right edge when there is more folded under it.
			wrapped := wrapCardLine(line.Text, width-2)
			for i, row := range wrapped {
				if i == 0 && line.More {
					row += strings.Repeat(" ", max(1, width-1-visibleLen(row))) + "▸"
				}
				rows = append(rows, cardRow.Render(row))
			}

		case report.LineChoices:
			rows = append(rows, choiceRows(line, width)...)

		case report.LineInstruction:
			pad := strings.Repeat(" ", line.Indent)
			wrapped := wrapCardLine(pad+line.Tag+" "+strings.Join(strings.Fields(line.Body), " "), width)
			if shortInstructions && !expanded && len(wrapped) > 1 {
				wrapped = []string{withEllipsis(wrapped[0], width)}
				cut = true
			}
			for i, row := range wrapped {
				if i == 0 {
					rows = append(rows, pad+cardDim.Render(line.Tag)+
						cardInstruction.Render(strings.TrimPrefix(row, pad+line.Tag)))
				} else {
					rows = append(rows, cardInstruction.Render(row))
				}
			}

		case report.LineOption:
			for _, row := range wrapCardLine(line.Text, width) {
				rows = append(rows, cardText.Render(row))
			}

		default:
			for _, row := range wrapCardLine(line.Text, width) {
				rows = append(rows, cardDim.Render(row))
			}
		}
	}
	return rows, cut
}

// choiceRows lays a block's options out side by side, wrapping to another row
// when the next one would not fit. The primary recommendation is drawn as a
// highlighted chip so the likeliest answer is found without reading; a
// secondary recommendation keeps an outline chip so two blocks never show two
// equal picks.
func choiceRows(line report.Line, width int) []string {
	pad := strings.Repeat(" ", line.Indent)
	const gap = "    "
	var rows []string
	row, rowLen := pad, line.Indent
	for _, c := range line.Choices {
		plain := c.Marker() + " " + c.Label
		painted := cardDim.Render(c.Marker()) + " " + cardText.Render(c.Label)
		if c.Primary {
			plain = " " + plain + " "
			painted = cardPick.Render(plain)
		} else if c.Recommended {
			plain = " " + plain + " "
			painted = cardSecondary.Render(plain)
		}

		n := visibleLen(plain)
		if rowLen > line.Indent {
			if rowLen+len(gap)+n > width {
				rows = append(rows, row)
				row, rowLen = pad, line.Indent
			} else {
				row += gap
				rowLen += len(gap)
			}

		}

		row += painted
		rowLen += n
	}

	return append(rows, row)
}

// withEllipsis marks a line as cut short, dropping characters from its end
// when the mark would not otherwise fit in width.
func withEllipsis(text string, width int) string {
	text = strings.TrimRight(text, " ")
	for visibleLen(text)+1 > width && text != "" {
		r := []rune(text)
		text = strings.TrimRight(string(r[:len(r)-1]), " ")
	}
	return text + "…"
}

// transcriptWidth is the width the scrollback copy wraps to. The live card
// wraps to the composer's terminal columns; the copy is printed after the
// composer is gone, so COLUMNS is the closest thing left to that width.
func transcriptWidth() int {
	w := 80
	if s := os.Getenv("COLUMNS"); s != "" {
		var n int
		if _, err := fmt.Sscanf(s, "%d", &n); err == nil && n > 0 {
			w = n
		}
	}
	if w < 40 {
		w = 40
	}
	if w > 250 {
		w = 250
	}
	return w - 2
}

// Transcript returns the scrollback copy of a report with the same colors as
// the card: headline, block lines, dim fields, bright options and
// muted instructions. Details are always expanded, so nothing the card kept
// folded is lost from the history.
func Transcript(card *report.Report, ans *CardAnswer) string {
	return TranscriptWidth(card, ans, transcriptWidth())
}

// TranscriptWidth is Transcript wrapped to an explicit width, so tests and
// callers that know the terminal can pin it.
func TranscriptWidth(card *report.Report, ans *CardAnswer, width int) string {
	if card == nil {
		return ""
	}
	if width <= 0 {
		width = transcriptWidth()
	}
	var sb strings.Builder
	sb.WriteString("\n")
	sb.WriteString(cardDim.Render("── report (previous turn, expanded) ──") + "\n")
	sb.WriteString(renderCardLines(card, report.Options{ExpandAll: true}, width, false))
	sb.WriteString(cardDim.Render("─────────────────────────────────────") + "\n")
	if ans != nil {
		if d := ans.Describe(); d != "" {
			sb.WriteString(cardDim.Render(d) + "\n")
		}
	}
	return sb.String()
}

// cardView draws the report above the prompt, or nothing when there is no
// report to draw. It is the same layout the agent prints headless, painted,
// and wrapped to the terminal so nothing runs off the right-hand side. A
// dashed rule ends it, so the card and the message being typed below it do
// not run together. not_run checks collapse to a footnote here; the
// transcript keeps them in full.
//
// A card taller than the terminal shows a window of it instead of pushing its
// own top out of view: pgup/pgdn scroll, home/end jump, and a dim line says
// how many rows are hidden each way. The window is sized so the whole frame —
// card window, indicator, rule, prompt, input and footer — fits the terminal.
func (m InputModel) cardView() string {
	if !m.cardShowing() {
		return ""
	}
	rows, _ := cardRows(m.card, m.cardOpts(), m.cardWidth(), true)
	win, above, below, truncated := m.cardWindow(rows)
	var sb strings.Builder
	sb.WriteString(strings.Join(win, "\n") + "\n")
	if truncated {
		sb.WriteString(m.cardScrollLine(above, below) + "\n")
	}
	sb.WriteString("\n" + cardDim.Render(strings.Repeat("╌", m.cardWidth())) + "\n")
	return sb.String()
}

// cardWindow cuts the card's content rows down to what fits the terminal. It
// returns the visible window plus how many rows are hidden above and below it,
// and whether anything was cut. Uncapped (no resize seen yet) or fitting cards
// show everything with no scrolling.
func (m InputModel) cardWindow(rows []string) (win []string, above, below int, truncated bool) {
	vis, cut := m.cardVisibleBudget(len(rows))
	if !cut {
		return rows, 0, 0, false
	}
	off := m.cardOffset
	if off < 0 {
		off = 0
	}
	if maxOff := len(rows) - vis; off > maxOff {
		off = maxOff
	}
	if off < 0 {
		off = 0
	}
	return rows[off : off+vis], off, len(rows) - (off + vis), true
}

// cardVisibleBudget is how many card content rows fit on screen. The second
// result says whether the card is truncated: false means show everything.
// Zero termRows (no resize yet: tests, headless) never truncates.
//
// The footer hint itself names the scroll keys while truncated, so the budget
// is first computed with the base hint and then recomputed with the full one:
// the longer footer can only shrink what fits, never un-truncate.
func (m InputModel) cardVisibleBudget(total int) (int, bool) {
	if m.termRows <= 0 {
		return total, false
	}
	base := m.baseCardHint()
	if _, cut := m.cardBudgetWithHint(total, base); !cut {
		return total, false
	}
	// Truncated: one more row goes to the scroll indicator, and the whole
	// frame still fits: vis + indicator + blank + rule + below == termRows.
	vis, _ := m.cardBudgetWithHint(total, m.cardHintWithScroll(base))
	if vis < 1 {
		vis = 1
	}
	return vis, true
}

// cardBudgetWithHint is cardVisibleBudget with an explicit footer hint. It
// takes the hint as a parameter rather than reading it so the budget can be
// computed while deciding what the hint itself says, without recursing.
func (m InputModel) cardBudgetWithHint(total int, hint string) (int, bool) {
	avail := m.termRows - m.cardBelowRowsForHint(hint) - 2
	if avail < 3 {
		avail = 3
	}
	if total <= avail {
		return total, false
	}
	vis := avail - 1
	if vis < 1 {
		vis = 1
	}
	return vis, true
}

// cardBelowRowsForHint counts the display rows the composer needs below the
// card: the prompt line, the input, the footer hint, and whatever is staged
// under the input. The budget above keeps the card window inside what is left.
// The hint is passed in so the budget can be computed while deciding what the
// hint says.
func (m InputModel) cardBelowRowsForHint(hint string) int {
	n := 1 + m.textarea.Height()
	if m.pasteNotice != "" {
		n++
	}
	if m.wakePending > 0 {
		n++
	}
	if m.compNotice != "" {
		n++
	}
	if len(m.compCandidates) > 0 {
		shown := len(m.compCandidates)
		remaining := m.compTotal - len(m.compCandidates)
		if shown > maxCompletionDisplay {
			remaining += shown - maxCompletionDisplay
			shown = maxCompletionDisplay
		}
		n += shown + 1 // candidates plus the "(↑↓/tab select, esc dismiss)" line
		if remaining > 0 {
			n++
		}
	}
	n += len(m.pastedImages)
	if hint != "" {
		// The blank line before the hint, plus the hint itself, which the
		// terminal soft-wraps on narrow screens.
		w := m.cardWidth()
		if w < 1 {
			w = 1
		}
		n += 1 + (visibleLen(hint)-1)/w + 1
	}
	return n
}

// cardScrollLine is the dim row between the card window and its rule while
// truncated: how many rows are hidden each way, and the keys that reach them.
func (m InputModel) cardScrollLine(above, below int) string {
	var parts []string
	if above > 0 {
		parts = append(parts, "↑"+itoa(above)+" more")
	}
	if below > 0 {
		parts = append(parts, "↓"+itoa(below)+" more")
	}
	msg := strings.Join(parts, " · ")
	if msg != "" {
		msg += " — pgup/pgdn to scroll"
	}
	return cardDim.Render(msg)
}

// cardScrollable reports whether the card is currently truncated to a
// scrollable window, which is when pgup/pgdn do something and the footer says
// so.
func (m InputModel) cardScrollable() bool {
	if !m.cardShowing() || m.termRows <= 0 {
		return false
	}
	rows, _ := cardRows(m.card, m.cardOpts(), m.cardWidth(), true)
	_, cut := m.cardVisibleBudget(len(rows))
	return cut
}

// clampCardOffset keeps the scroll position inside the window after a resize
// or a re-wrap changed how many rows the card has.
func (m *InputModel) clampCardOffset() {
	if !m.cardShowing() {
		m.cardOffset = 0
		return
	}
	rows, _ := cardRows(m.card, m.cardOpts(), m.cardWidth(), true)
	vis, cut := m.cardVisibleBudget(len(rows))
	if !cut {
		m.cardOffset = 0
		return
	}
	if m.cardOffset < 0 {
		m.cardOffset = 0
	}
	if maxOff := len(rows) - vis; m.cardOffset > maxOff {
		m.cardOffset = maxOff
	}
}

// cardPage is one pgup/pgdn step: the window height with one row of overlap
// for context, so repeated paging never loses the reader's place.
func (m InputModel) cardPage() int {
	rows, _ := cardRows(m.card, m.cardOpts(), m.cardWidth(), true)
	vis, cut := m.cardVisibleBudget(len(rows))
	if !cut {
		return 0
	}
	if vis > 1 {
		return vis - 1
	}
	return 1
}

// scrollCard moves the card window by delta rows, clamping at the ends, and
// reports whether there was a scrollable card to move. Zero or negative
// deltas never reach here with nothing to show: the caller falls through so
// the key keeps its normal job.
func (m *InputModel) scrollCard(delta int) bool {
	rows, _ := cardRows(m.card, m.cardOpts(), m.cardWidth(), true)
	vis, cut := m.cardVisibleBudget(len(rows))
	if !cut {
		return false
	}
	m.cardOffset += delta
	if m.cardOffset < 0 {
		m.cardOffset = 0
	}
	if maxOff := len(rows) - vis; m.cardOffset > maxOff {
		m.cardOffset = maxOff
	}
	return true
}

// scrollCardUp scrolls the card window up one page.
func (m *InputModel) scrollCardUp() bool { return m.scrollCard(-m.cardPage()) }

// scrollCardDown scrolls the card window down one page.
func (m *InputModel) scrollCardDown() bool { return m.scrollCard(m.cardPage()) }

// scrollCardTop jumps the card window to its first row.
func (m *InputModel) scrollCardTop() bool {
	if !m.cardScrollable() {
		return false
	}
	m.cardOffset = 0
	return true
}

// scrollCardBottom jumps the card window to its last row.
func (m *InputModel) scrollCardBottom() bool {
	if !m.cardScrollable() {
		return false
	}
	rows, _ := cardRows(m.card, m.cardOpts(), m.cardWidth(), true)
	vis, _ := m.cardVisibleBudget(len(rows))
	m.cardOffset = len(rows) - vis
	if m.cardOffset < 0 {
		m.cardOffset = 0
	}
	return true
}

// cardFooter is the key hint drawn under the composer while the card shows,
// where the eye is when the next key is pressed.
func (m InputModel) cardFooter() string {
	if !m.cardShowing() {
		return ""
	}
	hint := m.cardHint()
	if hint == "" {
		return ""
	}
	return "\n\n" + cardDim.Render(hint)
}

// wrapCardLine breaks one already-indented card line to fit the terminal,
// hanging the continuations two columns further in so a wrapped Impact or
// Evidence still reads as one field rather than as a new one.
func wrapCardLine(text string, width int) []string {
	indent := text[:len(text)-len(strings.TrimLeft(text, " "))]
	return wrapLine(indent, text[len(indent):], width, indent+"  ")
}

// cardHint is the footer that says which keys the card is holding. It lists
// only the ones that would do something on this report, so it never advertises
// a key that does nothing. esc and :report still work; they are in the help.
// While the card is truncated to a scrollable window it also names pgup/pgdn.
func (m InputModel) cardHint() string {
	base := m.baseCardHint()
	if !m.cardScrollable() {
		return base
	}
	return m.cardHintWithScroll(base)
}

// cardHintWithScroll names the scroll keys alongside a base hint.
func (m InputModel) cardHintWithScroll(base string) string {
	if base == "" {
		return "pgup/pgdn scroll"
	}
	return base + " · pgup/pgdn scroll"
}

// baseCardHint is the footer without the scroll keys: picks, Enter and
// details. The budget uses it first so deciding the hint never recurses.
func (m InputModel) baseCardHint() string {
	var parts []string
	if n := len(m.card.Choices()); n > 0 {
		if n > report.MaxChoices {
			n = report.MaxChoices
		}
		if n == 1 {
			parts = append(parts, "1 picks the option")
		} else {
			parts = append(parts, fmt.Sprintf("1-%d pick an option", n))
		}
	}
	if m.card.PrimaryChoice() != nil {
		parts = append(parts, "Enter picks recommended")
	}
	if m.anyDetails() {
		parts = append(parts, "0 open or close details")
	}
	return strings.Join(parts, " · ")
}

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
	b := &m.card.Blocks[c.Block]
	o := &b.Options[c.Option]
	m.cardAnswer = &CardAnswer{BlockID: b.ID, OptionID: o.ID, Effect: o.Effect, Number: n, Label: o.Label}

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
}

// Card colors. What the turn did is drawn in grey, because it is there to be
// skimmed; what it asks is drawn bright, with the recommended option picked
// out, because it is what gets answered.
var (
	cardDim         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cardRow         = lipgloss.NewStyle().Foreground(lipgloss.Color("246"))
	cardText        = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cardHeadline    = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
	cardAccent      = lipgloss.NewStyle().Foreground(lipgloss.Color("78"))
	cardFailed      = lipgloss.NewStyle().Foreground(lipgloss.Color("167"))
	cardPick        = lipgloss.NewStyle().Foreground(lipgloss.Color("16")).Background(lipgloss.Color("78"))
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
// when the next one would not fit. The recommended option is drawn as a
// highlighted chip so the likeliest answer is found without reading.
func choiceRows(line report.Line, width int) []string {
	pad := strings.Repeat(" ", line.Indent)
	const gap = "    "
	var rows []string
	row, rowLen := pad, line.Indent
	for _, c := range line.Choices {
		plain := c.Marker() + " " + c.Label
		painted := cardDim.Render(c.Marker()) + " " + cardText.Render(c.Label)
		if c.Recommended {
			plain = " " + plain + " "
			painted = cardPick.Render(plain)
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
func (m InputModel) cardView() string {
	if !m.cardShowing() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(renderCardLines(m.card, m.cardOpts(), m.cardWidth(), true))
	sb.WriteString("\n" + cardDim.Render(strings.Repeat("╌", m.cardWidth())) + "\n")
	return sb.String()
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
func (m InputModel) cardHint() string {
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
	if m.anyDetails() {
		parts = append(parts, "0 open or close details")
	}
	return strings.Join(parts, " · ")
}

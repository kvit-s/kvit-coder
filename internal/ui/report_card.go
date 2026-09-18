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

// anyDetails reports whether any block has details, which decides whether the
// footer offers the key that shows them.
func (m *InputModel) anyDetails() bool {
	if m.card == nil {
		return false
	}
	for i := range m.card.Blocks {
		if m.card.Blocks[i].Details != "" {
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
		// what gets sent, word for word.
		m.cardAnswer.Prompt = o.Instruction
		m.value = o.Instruction
		m.submitted = true
		m.quitting = true
		return true, tea.Quit
	case report.EffectCollect:
		// An answer in the user's own words, anchored to the block it
		// answers so the next turn knows what is being replied to.
		m.textarea.SetValue(fmt.Sprintf("[report %s] %s", b.ID, o.Label))
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
// already open.
func (m *InputModel) toggleAllDetails() {
	open := false
	for i := range m.card.Blocks {
		if b := &m.card.Blocks[i]; b.Details != "" && m.cardExpanded[b.ID] {
			open = true
		}
	}
	for i := range m.card.Blocks {
		if b := &m.card.Blocks[i]; b.Details != "" {
			m.cardExpanded[b.ID] = !open
		}
	}
}

// Card colors. The status chip is the only part that changes color, because it
// is the only part whose meaning is a single word.
var (
	cardDim         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cardText        = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cardOption      = lipgloss.NewStyle().Foreground(lipgloss.Color("255"))
	cardInstruction = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	cardChipDone    = lipgloss.NewStyle().Foreground(lipgloss.Color("71")).Bold(true)
	cardChipAsk     = lipgloss.NewStyle().Foreground(lipgloss.Color("178")).Bold(true)
	cardChipBad     = lipgloss.NewStyle().Foreground(lipgloss.Color("167")).Bold(true)
)

func chipStyle(s report.Status) lipgloss.Style {
	switch s {
	case report.StatusCompleted, report.StatusCompletedWithNotes:
		return cardChipDone
	case report.StatusNeedsAction:
		return cardChipAsk
	}
	return cardChipBad
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
// paintCardLine styles one wrapped card line the way the card does: block
// lines in text white, numbered options brightened because they are the part
// acted on, dispatch instructions muted, everything else dim.
func paintCardLine(kind report.LineKind, option int, text string) string {
	switch {
	case kind == report.LineBlock:
		return cardText.Render(text)
	case kind == report.LineOption && option > 0:
		return cardOption.Render(text)
	case kind == report.LineInstruction:
		return cardInstruction.Render(text)
	}
	return cardDim.Render(text)
}

// renderCardLines lays out a report the way the card does — indented,
// wrapped, painted — but without the card's key hint, so both the live card
// and the scrollback copy share one layout.
func renderCardLines(card *report.Report, opts report.Options, width int) string {
	var sb strings.Builder
	for _, line := range report.Lines(card, opts) {
		text := line.Text
		if line.Kind != report.LineHeader {
			text = "  " + text
		}
		for i, out := range wrapCardLine(text, width) {
			if line.Kind == report.LineHeader {
				// The chip is a word, so it only ever sits on the first
				// wrapped line of the headline.
				if chip, rest, found := strings.Cut(out, " · "); found && i == 0 {
					sb.WriteString(chipStyle(card.TaskStatus).Render(chip) + cardText.Render(" · "+rest))
				} else {
					sb.WriteString(cardText.Render(out))
				}
			} else {
				sb.WriteString(paintCardLine(line.Kind, line.Option, out))
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
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
// the card: chip and headline, block lines, dim fields, bright options and
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
	sb.WriteString(renderCardLines(card, report.Options{ExpandAll: true}, width))
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
// with the numbered options brightened because they are the part you act on,
// and wrapped to the terminal so nothing runs off the right-hand side.
func (m InputModel) cardView() string {
	if !m.cardShowing() {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(renderCardLines(m.card, report.Options{Expanded: m.cardExpanded}, m.cardWidth()))
	sb.WriteString(cardDim.Render(m.cardHint()) + "\n\n")
	return sb.String()
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
// a key that does nothing.
func (m InputModel) cardHint() string {
	var parts []string
	if n := len(m.card.Choices()); n > 0 {
		if n > report.MaxChoices {
			n = report.MaxChoices
		}
		if n == 1 {
			parts = append(parts, "1 answers")
		} else {
			parts = append(parts, fmt.Sprintf("1-%d answer", n))
		}
	}
	if m.anyDetails() {
		parts = append(parts, "0 details")
	}
	parts = append(parts, "esc dismiss", ":report reopen")
	return "  [" + strings.Join(parts, " · ") + "]"
}

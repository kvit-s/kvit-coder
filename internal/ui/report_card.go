package ui

import (
	"fmt"
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
}

// SetReport installs the report the last turn ended with. Passing nil, which is
// what happens after any turn that did not end with one, leaves the composer
// exactly as it was before reports existed.
func (m *InputModel) SetReport(rep *report.Report) {
	m.card = rep
	m.cardDismissed = false
	m.cardSelected = 0
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

// cardBlocks is the report's blocks in display order.
func (m *InputModel) cardBlocks() []int {
	if m.card == nil {
		return nil
	}
	return m.card.Order()
}

// activeBlock is the block a digit key answers: the highlighted one when it
// asks something, otherwise the first one that does.
func (m *InputModel) activeBlock() *report.Block {
	order := m.cardBlocks()
	if len(order) == 0 {
		return nil
	}
	if m.cardSelected >= 0 && m.cardSelected < len(order) {
		if b := &m.card.Blocks[order[m.cardSelected]]; b.Interactive() {
			return b
		}
	}
	if i := m.card.FirstInteractive(); i >= 0 {
		return &m.card.Blocks[i]
	}
	return nil
}

// handleCardKey gives the card first refusal on a keypress. It takes only the
// keys it advertises, and only while the text is empty, so the composer never
// traps a message: start typing and every key is the textarea's again.
//
// The digits are the reason the card exists — one keypress answers a decision —
// and the rest are alt combinations rather than bare letters, because a bare
// letter would make a word starting with it unwritable for as long as a card is
// up, which is most of the time.
func (m *InputModel) handleCardKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if !m.cardShowing() {
		return false, nil
	}
	key := msg.String()

	switch key {
	case "esc":
		// A completion list is nearer the cursor than the card, so it gets
		// the first esc and the card gets the next one.
		if m.completionListActive() || m.compNotice != "" {
			return false, nil
		}
		m.cardDismissed = true
		return true, nil
	case "alt+d":
		order := m.cardBlocks()
		if m.cardSelected >= 0 && m.cardSelected < len(order) {
			id := m.card.Blocks[order[m.cardSelected]].ID
			m.cardExpanded[id] = !m.cardExpanded[id]
		}
		return true, nil
	case "alt+up":
		if m.cardSelected > 0 {
			m.cardSelected--
		}
		return true, nil
	case "alt+down":
		if m.cardSelected < len(m.cardBlocks())-1 {
			m.cardSelected++
		}
		return true, nil
	}

	if len(key) == 1 && key[0] >= '1' && key[0] <= '9' && m.textarea.Value() == "" {
		return m.pickOption(int(key[0] - '0'))
	}
	return false, nil
}

// pickOption acts on the nth option of the block a digit answers.
func (m *InputModel) pickOption(n int) (bool, tea.Cmd) {
	b := m.activeBlock()
	if b == nil || n > len(b.Options) {
		return false, nil
	}
	o := &b.Options[n-1]
	m.cardAnswer = &CardAnswer{BlockID: b.ID, OptionID: o.ID, Effect: o.Effect}

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

// Card colors. The status chip is the only part that changes color, because it
// is the only part whose meaning is a single word.
var (
	cardDim         = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cardText        = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	cardInstruction = lipgloss.NewStyle().Foreground(lipgloss.Color("109"))
	cardSelectedRow = lipgloss.NewStyle().Foreground(lipgloss.Color("255")).Bold(true)
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

// cardView draws the report above the prompt, or nothing when there is no
// report to draw. It is the same layout the agent prints headless, painted.
func (m InputModel) cardView() string {
	if !m.cardShowing() {
		return ""
	}
	order := m.cardBlocks()
	selected := ""
	if m.cardSelected >= 0 && m.cardSelected < len(order) {
		selected = m.card.Blocks[order[m.cardSelected]].ID
	}

	active := ""
	if b := m.activeBlock(); b != nil {
		active = b.ID
	}

	var sb strings.Builder
	for _, line := range report.Lines(m.card, report.Options{
		Expanded: m.cardExpanded,
		Active:   active,
	}) {
		switch line.Kind {
		case report.LineHeader:
			chip, rest, found := strings.Cut(line.Text, " · ")
			if found {
				sb.WriteString(chipStyle(m.card.TaskStatus).Render(chip) + cardText.Render(" · "+rest))
			} else {
				sb.WriteString(cardText.Render(line.Text))
			}
		case report.LineBlock:
			if line.BlockID == selected && len(order) > 1 {
				sb.WriteString(cardSelectedRow.Render("▸ " + line.Text))
			} else {
				sb.WriteString(cardText.Render("  " + line.Text))
			}
		case report.LineInstruction:
			sb.WriteString(cardInstruction.Render("  " + line.Text))
		default:
			sb.WriteString(cardDim.Render("  " + line.Text))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(cardDim.Render(m.cardHint()) + "\n\n")
	return sb.String()
}

// cardHint is the footer that says which keys the card is holding, so nothing
// about it has to be remembered or guessed.
func (m InputModel) cardHint() string {
	var parts []string
	if b := m.activeBlock(); b != nil && len(b.Options) > 0 {
		parts = append(parts, fmt.Sprintf("1-%d answer", len(b.Options)))
	}
	parts = append(parts, "alt+d details")
	if len(m.cardBlocks()) > 1 {
		parts = append(parts, "alt+↑/↓ block")
	}
	parts = append(parts, "esc dismiss", ":report reopen")
	return "  [" + strings.Join(parts, " · ") + "]"
}

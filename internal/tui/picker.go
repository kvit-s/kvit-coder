package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// The lists, text prompts and waits of :setup, :models and :keys. Each one is
// a small bubbletea program drawn below the scrollback, like the composer,
// that clears itself when it ends; the caller then prints one line saying
// what was chosen, so the scrollback keeps a readable record.

var (
	pickTitle    = lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Bold(true)
	pickHint     = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	pickCurrent  = lipgloss.NewStyle().Foreground(lipgloss.Color("136"))
	pickText     = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	pickDetail   = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	pickDisabled = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	pickError    = lipgloss.NewStyle().Foreground(lipgloss.Color("167"))
)

// pickItem is one row of a list.
type pickItem struct {
	Label  string
	Detail string
	// Disabled rows are shown dimmed and cannot be chosen or ticked.
	Disabled bool
	// Checked is the starting tick in a list that takes several.
	Checked bool
}

// pickSpec is a list to show.
type pickSpec struct {
	Title string
	// Hint replaces the line of keys under the title.
	Hint  string
	Items []pickItem
	// Multi lets Space tick several rows; Enter then returns the ticked
	// ones, or the row under the cursor when none is ticked.
	Multi bool
	// Start is where the cursor begins.
	Start int
	// Delete lets the Delete key end the list on the row under the cursor,
	// as :models does to remove a model.
	Delete bool
}

// pickResult is what a list ended with.
type pickResult struct {
	// Index is the row under the cursor when the list ended.
	Index int
	// Checked are the ticked rows of a Multi list, in list order.
	Checked []int
	// Deleted is set when Delete ended the list.
	Deleted bool
}

// pickerModel is the bubbletea program behind a pickSpec. Typing filters the
// rows by their label and detail, Up and Down move, Enter chooses, and Esc
// clears the filter or, with none, closes the list.
type pickerModel struct {
	spec      pickSpec
	checked   []bool
	filter    string
	visible   []int
	cursor    int // position in visible
	offset    int // first visible row drawn
	height    int
	width     int
	result    pickResult
	done      bool
	cancelled bool
}

const pickMaxRows = 14

func newPicker(spec pickSpec) *pickerModel {
	m := &pickerModel{spec: spec, checked: make([]bool, len(spec.Items)), height: pickMaxRows, width: 100}
	for i, it := range spec.Items {
		m.checked[i] = it.Checked && !it.Disabled
	}
	m.refilter()
	for pos, idx := range m.visible {
		if idx == spec.Start {
			m.cursor = pos
		}
	}
	if m.cursor == 0 {
		m.skipDisabled(1)
	}
	m.scroll()
	return m
}

func (m *pickerModel) Init() tea.Cmd { return nil }

func (m *pickerModel) refilter() {
	m.visible = m.visible[:0]
	words := strings.Fields(strings.ToLower(m.filter))
	for i, it := range m.spec.Items {
		hay := strings.ToLower(it.Label + " " + it.Detail)
		match := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				match = false
				break
			}
		}
		if match {
			m.visible = append(m.visible, i)
		}
	}
	if m.cursor >= len(m.visible) {
		m.cursor = len(m.visible) - 1
	}
	if m.cursor < 0 {
		m.cursor = 0
	}
}

// skipDisabled moves the cursor off a disabled row in direction dir, staying
// put when every row that way is disabled.
func (m *pickerModel) skipDisabled(dir int) {
	for pos := m.cursor; pos >= 0 && pos < len(m.visible); pos += dir {
		if !m.spec.Items[m.visible[pos]].Disabled {
			m.cursor = pos
			return
		}
	}
}

func (m *pickerModel) move(delta int) {
	if len(m.visible) == 0 {
		return
	}
	dir := 1
	if delta < 0 {
		dir = -1
	}
	target := m.cursor + delta
	if target < 0 {
		target = 0
	}
	if target >= len(m.visible) {
		target = len(m.visible) - 1
	}
	old := m.cursor
	m.cursor = target
	m.skipDisabled(dir)
	if m.spec.Items[m.visible[m.cursor]].Disabled {
		m.cursor = old
	}
	m.scroll()
}

func (m *pickerModel) scroll() {
	if m.cursor < m.offset {
		m.offset = m.cursor
	}
	if m.cursor >= m.offset+m.height {
		m.offset = m.cursor - m.height + 1
	}
	if m.offset < 0 {
		m.offset = 0
	}
}

func (m *pickerModel) current() (int, bool) {
	if len(m.visible) == 0 {
		return 0, false
	}
	return m.visible[m.cursor], true
}

func (m *pickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height - 6
		if m.height > pickMaxRows {
			m.height = pickMaxRows
		}
		if m.height < 3 {
			m.height = 3
		}
		m.scroll()
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC:
			m.cancelled, m.done = true, true
			return m, tea.Quit
		case tea.KeyEsc:
			if m.filter != "" {
				m.filter = ""
				m.refilter()
				m.skipDisabled(1)
				m.scroll()
				return m, nil
			}
			m.cancelled, m.done = true, true
			return m, tea.Quit
		case tea.KeyUp, tea.KeyCtrlP:
			m.move(-1)
		case tea.KeyDown, tea.KeyCtrlN, tea.KeyTab:
			m.move(1)
		case tea.KeyPgUp:
			m.move(-m.height)
		case tea.KeyPgDown:
			m.move(m.height)
		case tea.KeyHome:
			m.move(-len(m.visible))
		case tea.KeyEnd:
			m.move(len(m.visible))
		case tea.KeyEnter:
			return m.finish(false)
		case tea.KeyDelete, tea.KeyCtrlD:
			if m.spec.Delete {
				return m.finish(true)
			}
		case tea.KeySpace:
			if m.spec.Multi {
				if idx, ok := m.current(); ok && !m.spec.Items[idx].Disabled {
					m.checked[idx] = !m.checked[idx]
				}
				m.move(1)
				return m, nil
			}
			m.filter += " "
			m.refilter()
		case tea.KeyBackspace:
			if r := []rune(m.filter); len(r) > 0 {
				m.filter = string(r[:len(r)-1])
				m.refilter()
				m.skipDisabled(1)
				m.scroll()
			}
		case tea.KeyRunes:
			m.filter += string(msg.Runes)
			m.refilter()
			m.cursor = 0
			m.skipDisabled(1)
			m.offset = 0
			m.scroll()
		}
	}
	return m, nil
}

func (m *pickerModel) finish(deleted bool) (tea.Model, tea.Cmd) {
	idx, ok := m.current()
	if checked := m.checkedRows(); m.spec.Multi && !deleted && len(checked) > 0 {
		m.result = pickResult{Index: idx, Checked: checked}
	} else {
		if !ok || (m.spec.Items[idx].Disabled && !deleted) {
			return m, nil
		}
		m.result = pickResult{Index: idx, Deleted: deleted}
		if m.spec.Multi && !deleted {
			m.result.Checked = []int{idx}
		}
	}
	m.done = true
	return m, tea.Quit
}

func (m *pickerModel) checkedRows() []int {
	var rows []int
	for i, c := range m.checked {
		if c {
			rows = append(rows, i)
		}
	}
	return rows
}

func (m *pickerModel) View() string {
	if m.done {
		return ""
	}
	var b strings.Builder
	b.WriteString(pickTitle.Render(clip(m.spec.Title, m.width-1)))
	b.WriteString("\n")
	hint := m.spec.Hint
	if hint == "" {
		hint = "type to filter · ↑↓ to move · Enter to choose · Esc to go back"
		if m.spec.Multi {
			hint = "Space to tick · Enter when done · type to filter · Esc to go back"
		}
	}
	b.WriteString(pickHint.Render(clip(hint, m.width-1)))
	b.WriteString("\n")
	if m.filter != "" {
		b.WriteString(pickHint.Render("filter: ") + pickText.Render(m.filter))
		b.WriteString("\n")
	}
	if len(m.visible) == 0 {
		b.WriteString(pickDisabled.Render("  nothing matches"))
		return b.String()
	}
	labelWidth := 0
	for _, idx := range m.visible {
		if w := lipgloss.Width(m.spec.Items[idx].Label); w > labelWidth {
			labelWidth = w
		}
	}
	if labelWidth > 44 {
		labelWidth = 44
	}
	end := m.offset + m.height
	if end > len(m.visible) {
		end = len(m.visible)
	}
	for pos := m.offset; pos < end; pos++ {
		idx := m.visible[pos]
		it := m.spec.Items[idx]
		marker := "  "
		if pos == m.cursor {
			marker = "> "
		}
		box := ""
		if m.spec.Multi {
			box = "[ ] "
			if m.checked[idx] {
				box = "[x] "
			}
			if it.Disabled {
				box = "    "
			}
		}
		label := clip(it.Label, labelWidth)
		label += strings.Repeat(" ", labelWidth-lipgloss.Width(label))
		room := m.width - 1 - lipgloss.Width(marker+box+label) - 2
		detail := ""
		if it.Detail != "" && room > 8 {
			detail = "  " + clip(it.Detail, room)
		}
		switch {
		case it.Disabled:
			b.WriteString(pickDisabled.Render(marker + box + label + detail))
		case pos == m.cursor:
			b.WriteString(pickCurrent.Render(marker+box+label) + pickDetail.Render(detail))
		default:
			b.WriteString(pickText.Render(marker+box+label) + pickDetail.Render(detail))
		}
		if pos < end-1 {
			b.WriteString("\n")
		}
	}
	if hidden := len(m.visible) - (end - m.offset); hidden > 0 {
		b.WriteString("\n" + pickHint.Render(fmt.Sprintf("  %d of %d shown; ↑↓ scrolls", end-m.offset, len(m.visible))))
	}
	return b.String()
}

// clip cuts plain text to n terminal columns, ending in "…" when it cuts.
func clip(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if lipgloss.Width(s) <= n {
		return s
	}
	var b strings.Builder
	w := 0
	for _, r := range s {
		rw := lipgloss.Width(string(r))
		if w+rw > n-1 {
			break
		}
		b.WriteRune(r)
		w += rw
	}
	return b.String() + "…"
}

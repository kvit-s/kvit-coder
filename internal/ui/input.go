package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	rw "github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
)

// InputModel wraps the textarea component for multi-line input
type InputModel struct {
	textarea      textarea.Model
	history       []string
	historyIdx    int
	submitted     bool
	cancelled     bool
	value         string
	prompt        string
	width         int
	maxHeight     int
	quitting      bool
	lastValueLen  int // Track previous value length to detect paste
	viewportStart int // Track which line is at the top of the viewport
	// onImagePaste stages a clipboard image and returns its staged path.
	// Nil when the host offers no image paste (set by the driver; the model
	// itself never touches the clipboard).
	onImagePaste func() (string, error)
	// pastedImages are clipboard images staged by the paste key this session.
	pastedImages []string
	// pasteNotice is the one-line outcome of the last paste attempt, shown
	// under the input so a failed paste is visible, not silent.
	pasteNotice string
}

// SetImagePasteHandler installs the clipboard-image stager behind the paste
// key. Passing nil disables image paste.
func (m *InputModel) SetImagePasteHandler(fn func() (string, error)) {
	m.onImagePaste = fn
}

// PastedImages returns the clipboard images staged by the paste key.
func (m InputModel) PastedImages() []string {
	return append([]string(nil), m.pastedImages...)
}

// pasteImage stages one clipboard image behind the paste key. Success and
// failure both leave a one-line notice: a paste that fails silently reads as
// a broken key, which is the report that started this.
func (m *InputModel) pasteImage() {
	if m.onImagePaste == nil {
		return
	}
	path, err := m.onImagePaste()
	if err != nil {
		m.pasteNotice = "paste: " + err.Error()
		return
	}
	m.pastedImages = append(m.pastedImages, path)
	m.pasteNotice = fmt.Sprintf("staged image %d: %s (attaches on submit)", len(m.pastedImages), filepath.Base(path))
}

// adjustHeight adjusts the textarea height to fit content, up to maxHeight.
// It counts soft-wrapped visual rows, not just hard lines, so a long single
// line without newlines still grows the input instead of scrolling the start
// of the text out of a 1-row viewport.
func (m *InputModel) adjustHeight() {
	total := m.totalSoftLines()
	newHeight := total
	if newHeight > m.maxHeight {
		newHeight = m.maxHeight
	}
	if newHeight < 1 {
		newHeight = 1
	}
	m.textarea.SetHeight(newHeight)
}

// inputWidth returns the inner width the textarea wraps at.
func (m *InputModel) inputWidth() int {
	if w := m.textarea.Width(); w > 0 {
		return w
	}
	if m.width > 0 {
		return m.width
	}
	return 80
}

// totalSoftLines returns the number of visual rows the current value occupies
// once soft-wrapped at the input width.
func (m *InputModel) totalSoftLines() int {
	return softWrappedHeight(m.textarea.Value(), m.inputWidth())
}

// cursorSoftRow returns the 0-based visual row the cursor is on, accounting
// for soft-wrapped rows in all hard lines above it.
func (m *InputModel) cursorSoftRow() int {
	width := m.inputWidth()
	hardRow := m.textarea.Line()
	rowsBefore := 0
	if v := m.textarea.Value(); v != "" {
		lines := strings.Split(v, "\n")
		for i := 0; i < hardRow && i < len(lines); i++ {
			rowsBefore += wrapLineCount([]rune(lines[i]), width)
		}
	}
	return rowsBefore + m.textarea.LineInfo().RowOffset
}

// softWrappedHeight counts visual rows for a value at the given width.
func softWrappedHeight(value string, width int) int {
	if width <= 0 {
		width = 80
	}
	if value == "" {
		return 1
	}
	total := 0
	for _, line := range strings.Split(value, "\n") {
		total += wrapLineCount([]rune(line), width)
	}
	if total < 1 {
		total = 1
	}
	return total
}

// wrapLineCount mirrors the word-wrap used by bubbles/textarea: it reports how
// many visual rows one hard line occupies at the given width.
func wrapLineCount(runes []rune, width int) int {
	return len(wrapRunes(runes, width))
}

// wrapRunes is a copy of the wrapping algorithm in bubbles/textarea, kept here
// so height and viewport accounting match exactly what the component renders.
func wrapRunes(runes []rune, width int) [][]rune {
	var (
		lines  = [][]rune{{}}
		word   = []rune{}
		row    int
		spaces int
	)

	// Word wrap the runes
	for _, r := range runes {
		if unicode.IsSpace(r) {
			spaces++
		} else {
			word = append(word, r)
		}

		if spaces > 0 { //nolint:nestif
			if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces > width {
				row++
				lines = append(lines, []rune{})
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			} else {
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			}
		} else {
			// If the last character is a double-width rune, then we may not be able to add it to this line
			// as it might cause us to go past the width.
			lastCharLen := rw.RuneWidth(word[len(word)-1])
			if uniseg.StringWidth(string(word))+lastCharLen > width {
				// If the current line has any content, let's move to the next
				// line because the current word fills up the entire line.
				if len(lines[row]) > 0 {
					row++
					lines = append(lines, []rune{})
				}
				lines[row] = append(lines[row], word...)
				word = nil
			}
		}
	}

	if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces >= width {
		lines = append(lines, []rune{})
		lines[row+1] = append(lines[row+1], word...)
		// We add an extra space at the end of the line to account for the
		// trailing space at the end of the previous soft-wrapped lines so that
		// behaviour when navigating is consistent and so that we don't need to
		// continually add edges to handle the last line of the wrapped input.
		spaces++
		lines[row+1] = append(lines[row+1], repeatSpaces(spaces)...)
	} else {
		lines[row] = append(lines[row], word...)
		spaces++
		lines[row] = append(lines[row], repeatSpaces(spaces)...)
	}

	return lines
}

func repeatSpaces(n int) []rune {
	return []rune(strings.Repeat(string(' '), n))
}

// NewInputModel creates a new input model with the given prompt
func NewInputModel(prompt string, history []string) InputModel {
	ta := textarea.New()
	ta.Prompt = "" // We'll show the prompt separately
	ta.Placeholder = "(Enter to submit, Ctrl+J newline, Alt+V paste image)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // No limit

	// Start with 1 row, will grow automatically
	ta.SetHeight(1)
	ta.SetWidth(80) // Default width

	// Configure styling - remove cursor line highlighting
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ta.FocusedStyle.Text = lipgloss.NewStyle()

	// Disable the default Enter → newline binding
	// We'll handle Enter for submission and Ctrl+J for newlines
	ta.KeyMap.InsertNewline.SetEnabled(false)

	ta.Focus()

	return InputModel{
		textarea:     ta,
		history:      history,
		historyIdx:   -1,
		submitted:    false,
		cancelled:    false,
		prompt:       prompt,
		width:        80,
		maxHeight:    20, // Max height before scrolling
		lastValueLen: 0,
	}
}

// Init initializes the input model
func (m InputModel) Init() tea.Cmd {
	return textarea.Blink
}

// Update handles input events
func (m InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		// Adjust width based on terminal size
		m.width = msg.Width - 10 // Leave some margin
		if m.width < 40 {
			m.width = 40
		}
		m.textarea.SetWidth(m.width)

		// Adjust max height based on terminal size (leave room for prompt and other UI)
		m.maxHeight = msg.Height - 5
		if m.maxHeight < 5 {
			m.maxHeight = 5
		}
		// Re-wrap at the new width: a narrower terminal can turn one hard
		// line into several visual rows.
		m.adjustHeight()

	case tea.KeyMsg:
		switch msg.String() {
		// Submit on Enter
		case "enter":
			m.value = m.textarea.Value()
			m.submitted = true
			m.quitting = true
			return m, tea.Quit

		// Insert newline on Ctrl+J
		case "ctrl+j":
			m.textarea.InsertString("\n")
			m.adjustHeight()
			return m, nil

		// Smart history navigation
		case "up":
			if len(m.history) > 0 {
				// Navigate history if:
				// 1. Textarea is empty OR
				// 2. We're in history mode (historyIdx >= 0) and on the first
				//    visual (soft-wrapped) row, so up inside a long wrapped
				//    line still moves the cursor instead of leaving history.
				isEmpty := m.textarea.Value() == ""
				inHistoryMode := m.historyIdx >= 0
				onFirstVisual := m.cursorSoftRow() == 0

				if isEmpty || (inHistoryMode && onFirstVisual) {
					// Navigate to older history
					if m.historyIdx < len(m.history)-1 {
						m.historyIdx++
						m.textarea.SetValue(m.history[len(m.history)-1-m.historyIdx])
						m.adjustHeight()
						// Move cursor to beginning: first move to start of current line
						m.textarea.CursorStart()
						// Then move up to line 0
						for m.textarea.Line() > 0 {
							m.textarea.CursorUp()
						}
						// Finally ensure we're at column 0
						m.textarea.CursorStart()
						m.viewportStart = 0
					}
					return m, nil
				}
			}
			// Otherwise, let textarea handle it for line navigation

		case "down":
			if len(m.history) > 0 {
				// Navigate history if:
				// 1. Textarea is empty OR
				// 2. We're in history mode (historyIdx >= 0) and on the last
				//    visual (soft-wrapped) row.
				isEmpty := m.textarea.Value() == ""
				inHistoryMode := m.historyIdx >= 0
				onLastVisual := m.cursorSoftRow() == m.totalSoftLines()-1

				if isEmpty || (inHistoryMode && onLastVisual) {
					// Navigate to newer history
					if m.historyIdx > 0 {
						m.historyIdx--
						m.textarea.SetValue(m.history[len(m.history)-1-m.historyIdx])
						m.adjustHeight()
						// Move cursor to beginning: first move to start of current line
						m.textarea.CursorStart()
						// Then move up to line 0
						for m.textarea.Line() > 0 {
							m.textarea.CursorUp()
						}
						// Finally ensure we're at column 0
						m.textarea.CursorStart()
						m.viewportStart = 0
					} else if m.historyIdx == 0 {
						m.historyIdx = -1
						m.textarea.SetValue("")
						m.adjustHeight()
						m.viewportStart = 0
					}
					return m, nil
				}
			}
			// Otherwise, let textarea handle it for line navigation

		case "ctrl+c":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		// Paste a clipboard image as a staged attachment. Alt+V is the
		// escape hatch for terminals that intercept Ctrl+V with a text-only
		// paste (Windows Terminal, the VS Code terminal on WSL): Ctrl+V
		// never arrives as a keypress there, so it cannot be handled here.
		case "alt+v":
			if m.onImagePaste != nil {
				m.pasteImage()
				return m, nil
			}
			// No handler: let the textarea see the key.

		case "esc":
			// ESC just clears the current input, doesn't exit
			m.textarea.SetValue("")
			m.adjustHeight()
			m.viewportStart = 0
			m.pasteNotice = ""
			return m, nil
		}
	}

	// Track value length before update to detect paste and edits
	beforeValueLen := len(m.textarea.Value())
	beforeValue := m.textarea.Value()
	wasInHistoryMode := m.historyIdx >= 0

	// Pre-grow the viewport by one visual row before the textarea sees the
	// key: it lays out and scrolls to keep the cursor visible using the old
	// height, so the keystroke that wraps onto a new row would scroll the
	// first row out of view (seen as the line shifting left instead of
	// wrapping) and the shrink-to-fit below would leave that offset
	// stranded. A single keystroke adds at most one visual row; at the cap
	// the height is already correct and scrolling is genuinely needed.
	if total := m.totalSoftLines(); total+1 <= m.maxHeight && m.textarea.Height() < total+1 {
		m.textarea.SetHeight(total + 1)
	}

	m.textarea, cmd = m.textarea.Update(msg)

	// If user edited the content while in history mode, exit history mode
	if m.historyIdx >= 0 && m.textarea.Value() != beforeValue {
		// Check if it's an actual edit (not just navigation)
		if msg, ok := msg.(tea.KeyMsg); ok {
			// Only reset if it's a typing key (not arrow keys, page up/down, etc.)
			key := msg.String()
			if len(key) == 1 || key == "backspace" || key == "delete" || key == "ctrl+u" || key == "ctrl+k" {
				m.historyIdx = -1
			}
		}
	}

	// Always adjust height to fit content, up to maxHeight
	m.adjustHeight()

	// Detect paste: if value length increased significantly in one update (>1 char),
	// it's likely a paste operation (but not history navigation)
	afterValueLen := len(m.textarea.Value())
	isPaste := afterValueLen > beforeValueLen+1 && !wasInHistoryMode

	// If we detected a paste whose wrapped content spans multiple visual
	// rows, reset the viewport to show from the beginning and move cursor
	// to end (the tracking below then keeps the cursor visible).
	if isPaste && m.totalSoftLines() > 1 {
		currentValue := m.textarea.Value()
		m.textarea.SetValue(currentValue)
		m.textarea.CursorEnd()
		m.viewportStart = 0
	}

	// Update viewport tracking to keep cursor visible.
	// This mimics the textarea's internal viewport logic, but in visual
	// (soft-wrapped) rows so long single lines track correctly.
	currentVisual := m.cursorSoftRow()
	visibleHeight := m.textarea.Height()
	totalVisual := m.totalSoftLines()

	// Ensure viewport keeps cursor visible
	if currentVisual < m.viewportStart {
		// Cursor scrolled above viewport, scroll up
		m.viewportStart = currentVisual
	} else if currentVisual >= m.viewportStart+visibleHeight {
		// Cursor scrolled below viewport, scroll down
		m.viewportStart = currentVisual - visibleHeight + 1
	}

	// Clamp viewport to valid range
	if m.viewportStart < 0 {
		m.viewportStart = 0
	}

	maxViewportStart := totalVisual - visibleHeight
	if maxViewportStart < 0 {
		maxViewportStart = 0
	}
	if m.viewportStart > maxViewportStart {
		m.viewportStart = maxViewportStart
	}

	return m, cmd
}

// View renders the input model
func (m InputModel) View() string {
	// If quitting, return empty string to clear the display
	if m.quitting {
		return ""
	}

	// Calculate scroll indicator based on tracked viewport position.
	// Use soft-wrapped visual rows so a long single line reports scrolling.
	totalVisual := softWrappedHeight(m.textarea.Value(), m.inputWidth())
	visibleHeight := m.textarea.Height()

	scrollInfo := ""
	if totalVisual > visibleHeight {
		// Calculate what's hidden above and below the viewport
		viewportEnd := m.viewportStart + visibleHeight
		if viewportEnd > totalVisual {
			viewportEnd = totalVisual
		}

		hiddenAbove := m.viewportStart
		hiddenBelow := totalVisual - viewportEnd

		if hiddenAbove > 0 || hiddenBelow > 0 {
			scrollInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
				" [" + formatScrollInfo(hiddenAbove, hiddenBelow, totalVisual, visibleHeight) + "]")
		}
	}

	// Simplified view without borders for better performance
	// Just show prompt and textarea with scroll indicator
	out := m.prompt + scrollInfo + "\n" + m.textarea.View()
	if m.pasteNotice != "" {
		out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(m.pasteNotice)
	}
	return out
}

// formatScrollInfo creates a compact scroll indicator
func formatScrollInfo(hiddenAbove, hiddenBelow, total, visible int) string {
	if hiddenAbove > 0 && hiddenBelow > 0 {
		return lipgloss.NewStyle().Render("↑" + itoa(hiddenAbove) + " ↓" + itoa(hiddenBelow))
	} else if hiddenAbove > 0 {
		return lipgloss.NewStyle().Render("↑" + itoa(hiddenAbove))
	} else if hiddenBelow > 0 {
		return lipgloss.NewStyle().Render("↓" + itoa(hiddenBelow))
	}
	return lipgloss.NewStyle().Render(itoa(visible) + "/" + itoa(total))
}

// itoa is a simple int to string converter
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// Value returns the submitted value
func (m InputModel) Value() string {
	return m.value
}

// Submitted returns whether the input was submitted
func (m InputModel) Submitted() bool {
	return m.submitted
}

// Cancelled returns whether the input was cancelled
func (m InputModel) Cancelled() bool {
	return m.cancelled
}

// LoadHistory loads history from a file
func LoadHistory(filepath string) ([]string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	// Use null byte as delimiter to preserve multi-line entries
	entries := strings.Split(string(data), "\x00")
	// Filter out empty entries
	var history []string
	for _, entry := range entries {
		if strings.TrimSpace(entry) != "" {
			history = append(history, entry)
		}
	}
	return history, nil
}

// SaveHistory saves history to a file
func SaveHistory(filepath string, history []string) error {
	// Limit history size to last 1000 entries
	const maxHistory = 1000
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	// Use null byte as delimiter to preserve multi-line entries
	return os.WriteFile(filepath, []byte(strings.Join(history, "\x00")), 0644)
}

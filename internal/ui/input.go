package ui

import (
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
}

// adjustHeight adjusts the textarea height to fit content, up to maxHeight
func (m *InputModel) adjustHeight() {
	lineCount := m.textarea.LineCount()
	newHeight := lineCount
	if newHeight > m.maxHeight {
		newHeight = m.maxHeight
	}
	if newHeight < 1 {
		newHeight = 1
	}
	m.textarea.SetHeight(newHeight)
}


// NewInputModel creates a new input model with the given prompt
func NewInputModel(prompt string, history []string) InputModel {
	ta := textarea.New()
	ta.Prompt = ""  // We'll show the prompt separately
	ta.Placeholder = "(Ctrl+J for newline, Enter to submit)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // No limit

	// Start with 1 row, will grow automatically
	ta.SetHeight(1)
	ta.SetWidth(80)  // Default width

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
				// 2. We're in history mode (historyIdx >= 0) and on the first line
				isEmpty := m.textarea.Value() == ""
				inHistoryMode := m.historyIdx >= 0
				onFirstLine := m.textarea.Line() == 0

				if isEmpty || (inHistoryMode && onFirstLine) {
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
				// 2. We're in history mode (historyIdx >= 0) and on the last line
				isEmpty := m.textarea.Value() == ""
				inHistoryMode := m.historyIdx >= 0
				onLastLine := m.textarea.Line() == m.textarea.LineCount()-1

				if isEmpty || (inHistoryMode && onLastLine) {
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

		case "esc":
			// ESC just clears the current input, doesn't exit
			m.textarea.SetValue("")
			m.adjustHeight()
			m.viewportStart = 0
			return m, nil
		}
	}

	// Track value length before update to detect paste and edits
	beforeValueLen := len(m.textarea.Value())
	beforeValue := m.textarea.Value()
	wasInHistoryMode := m.historyIdx >= 0

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

	// If we detected a paste with multi-line content,
	// reset the viewport to show from the beginning and move cursor to end
	if isPaste && m.textarea.LineCount() > 1 {
		currentValue := m.textarea.Value()
		m.textarea.SetValue(currentValue)
		m.textarea.CursorEnd()
		m.viewportStart = 0
	}

	// Update viewport tracking to keep cursor visible
	// This mimics the textarea's internal viewport logic
	currentLine := m.textarea.Line()
	visibleHeight := m.textarea.Height()
	lineCount := m.textarea.LineCount()

	// Ensure viewport keeps cursor visible
	if currentLine < m.viewportStart {
		// Cursor scrolled above viewport, scroll up
		m.viewportStart = currentLine
	} else if currentLine >= m.viewportStart+visibleHeight {
		// Cursor scrolled below viewport, scroll down
		m.viewportStart = currentLine - visibleHeight + 1
	}

	// Clamp viewport to valid range
	if m.viewportStart < 0 {
		m.viewportStart = 0
	}
	maxViewportStart := lineCount - visibleHeight
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

	// Calculate scroll indicator based on tracked viewport position
	lineCount := m.textarea.LineCount()
	visibleHeight := m.textarea.Height()

	scrollInfo := ""
	if lineCount > visibleHeight {
		// Calculate what's hidden above and below the viewport
		viewportEnd := m.viewportStart + visibleHeight
		if viewportEnd > lineCount {
			viewportEnd = lineCount
		}

		hiddenAbove := m.viewportStart
		hiddenBelow := lineCount - viewportEnd

		if hiddenAbove > 0 || hiddenBelow > 0 {
			scrollInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
				" [" + formatScrollInfo(hiddenAbove, hiddenBelow, lineCount, visibleHeight) + "]")
		}
	}

	// Simplified view without borders for better performance
	// Just show prompt and textarea with scroll indicator
	return m.prompt + scrollInfo + "\n" + m.textarea.View()
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

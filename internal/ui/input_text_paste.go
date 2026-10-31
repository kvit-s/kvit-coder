package ui

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/runeutil"
	"github.com/charmbracelet/lipgloss"
	rw "github.com/mattn/go-runewidth"
)

// Long pastes become tags.
//
// A paste of more than longPasteLines lines or longPasteChars characters does
// not go into the composer text. The driver saves it to a file (onTextPaste),
// the composer puts a [pasteN] tag where the text would have gone, and a line
// under the input says what each tag holds. On submit the tags still in the
// text are numbered again from 1, in order, and their files go to the agent
// (-paste), which tells the model where each one is so that it reads the file
// with Read instead of receiving the text in the message.
//
// That keeps three things small: the composer, which redraws its whole text
// on every key; the scrollback and the arrow-up history; and the command line
// the prompt travels to the agent on, which Windows caps at 32,767 characters.

const (
	longPasteLines = 20
	longPasteChars = 2000
)

// pastedText is one long paste the composer turned into a tag.
type pastedText struct {
	path string
	// about and first are what the list under the input shows: the size
	// (DescribeText) and the first line that is not blank.
	about string
	first string
}

// PasteLabel returns the in-text tag for the nth long paste (1-based), e.g.
// "[paste1]".
func PasteLabel(n int) string {
	return "[" + "paste" + itoa(n) + "]"
}

// SetTextPasteHandler installs the stager for long pastes: it saves the text
// to a file and returns the file's path. Nil puts every paste in the text.
func (m *InputModel) SetTextPasteHandler(fn func(string) (string, error)) {
	m.onTextPaste = fn
}

// PastedTexts returns the files of the long pastes the submitted text refers
// to, in tag order: the first is [paste1].
func (m InputModel) PastedTexts() []string {
	return append([]string(nil), m.sentPastes...)
}

// DescribeText says how long a text is the way the composer list, the
// scrollback and the model's note all put it: "250 lines, 21.9 KB".
func DescribeText(text string) string {
	n := lineCount(text)
	unit := "lines"
	if n == 1 {
		unit = "line"
	}
	return fmt.Sprintf("%d %s, %s", n, unit, formatSize(len(text)))
}

// lineCount counts lines the way an editor shows them: a final line break
// does not start another line.
func lineCount(text string) int {
	if text == "" {
		return 0
	}
	n := strings.Count(text, "\n") + 1
	if strings.HasSuffix(text, "\n") {
		n--
	}
	return n
}

func formatSize(n int) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

// normalizePaste gives pasted text the composer's line breaks.
func normalizePaste(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	return strings.ReplaceAll(s, "\r", "\n")
}

// isLongPaste reports whether a paste becomes a tag.
func isLongPaste(text string) bool {
	return lineCount(text) > longPasteLines || utf8.RuneCountInString(text) > longPasteChars
}

// pasteText puts pasted text in at the cursor: a long paste as a tag, and
// anything shorter, or a long paste that could not be saved, as text.
func (m *InputModel) pasteText(text string) {
	text = normalizePaste(text)
	if isLongPaste(text) && m.stageTextPaste(text) {
		return
	}
	m.clearCompletion()
	m.textarea.InsertString(text)
	m.historyIdx = -1
	m.afterBulkEdit()
}

// stageTextPaste saves a long paste through the driver and puts its tag at
// the cursor. It reports false when there is no stager or saving failed; a
// failure leaves a notice, and the caller puts the text in instead.
func (m *InputModel) stageTextPaste(text string) bool {
	if m.onTextPaste == nil {
		return false
	}
	path, err := m.onTextPaste(text)
	if err != nil {
		m.pasteNotice = "paste: " + err.Error() + "; pasted as text instead"
		return false
	}
	m.addPastedText(path, text)
	return true
}

// addPastedText records a saved long paste and puts its tag at the cursor.
func (m *InputModel) addPastedText(path, text string) {
	m.pastedTexts = append(m.pastedTexts, pastedText{path: path, about: DescribeText(text), first: firstLine(text)})
	m.pasteNotice = ""
	m.clearCompletion()
	m.insertToken(PasteLabel(len(m.pastedTexts)))
	m.historyIdx = -1
	m.afterBulkEdit()
}

// takePastes picks the long pastes whose tags are still in the text, in tag
// order, and numbers their tags again from 1 so that they match the order the
// agent receives the files in. A tag deleted from the text drops its paste.
// Renaming in increasing order is safe: each tag moves to a number no larger
// than its own, and any tag that had that number has already moved lower or
// was deleted.
func (m *InputModel) takePastes(text string) (string, []string) {
	var kept []string
	for i, p := range m.pastedTexts {
		label := PasteLabel(i + 1)
		if !strings.Contains(text, label) {
			continue
		}
		kept = append(kept, p.path)
		if renamed := PasteLabel(len(kept)); renamed != label {
			text = strings.ReplaceAll(text, label, renamed)
		}
	}
	return text, kept
}

// firstLine returns the first line of a text that is not blank.
func firstLine(text string) string {
	for _, line := range strings.Split(text, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			return t
		}
	}
	return ""
}

// pastesView lists each long paste under the input, and while a long Windows
// paste is still arriving, how far it has got.
func (m InputModel) pastesView() string {
	width := m.termCols
	if width <= 0 {
		width = 80
	}
	out := ""
	for i, p := range m.pastedTexts {
		line := fmt.Sprintf("[%s: %s]", "paste"+itoa(i+1), p.about)
		if p.first != "" {
			line += " " + p.first
		}
		out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(rw.Truncate(line, width-1, "…"))
	}
	if m.collectingLongPaste() {
		out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(fmt.Sprintf("[pasting: %d lines so far]", m.paste.burst.lines+1))
	}
	return out
}

// replaceRange swaps the runes [start, end) of the composer text for text and
// leaves the cursor after it.
func (m *InputModel) replaceRange(start, end int, text string) {
	runes := []rune(m.textarea.Value())
	if start < 0 || end > len(runes) || start > end {
		return
	}
	with := runeutil.NewSanitizer().Sanitize([]rune(text))
	m.textarea.SetValue(string(runes[:start]) + string(with) + string(runes[end:]))
	m.moveCursorToOffset(start + len(with))
	m.afterBulkEdit()
}

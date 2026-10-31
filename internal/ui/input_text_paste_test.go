package ui

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// logLines is a pasted log of n lines, joined with sep.
func logLines(n int, sep string) string {
	var lines []string
	for i := 0; i < n; i++ {
		lines = append(lines, fmt.Sprintf("2026-10-31 08:40:%02d INFO line %d", i%60, i))
	}
	return strings.Join(lines, sep)
}

// stager records what the composer asked it to save.
type stager struct {
	saved []string
	err   error
}

func (s *stager) stage(text string) (string, error) {
	if s.err != nil {
		return "", s.err
	}
	s.saved = append(s.saved, text)
	return fmt.Sprintf("/tmp/paste-%d.txt", len(s.saved)), nil
}

func bracketedPaste(text string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text), Paste: true}
}

// A long paste goes to a file and shows as a tag, with a line under the input
// saying what it holds.
func TestLongBracketedPasteBecomesTag(t *testing.T) {
	st := &stager{}
	m := NewInputModel(">", nil)
	m.SetTextPasteHandler(st.stage)
	m = typeText(m, "why:")
	m = updateModel(m, bracketedPaste(logLines(30, "\r\n")))
	if got := m.textarea.Value(); got != "why: [paste1] " {
		t.Errorf("textarea = %q, want the tag in place of the text", got)
	}
	if len(st.saved) != 1 || st.saved[0] != logLines(30, "\n") {
		t.Fatalf("saved %d texts; want the paste once, with CRLF made LF", len(st.saved))
	}
	view := m.View()
	if !strings.Contains(view, "[paste1: 30 lines, ") || !strings.Contains(view, "2026-10-31 08:40:00 INFO line 0") {
		t.Errorf("View does not list the paste:\n%s", view)
	}
}

// A short paste stays text the user can edit.
func TestShortPasteStaysText(t *testing.T) {
	st := &stager{}
	m := NewInputModel(">", nil)
	m.SetTextPasteHandler(st.stage)
	m = updateModel(m, bracketedPaste("one\ntwo\nthree"))
	if len(st.saved) != 0 {
		t.Errorf("a three-line paste was saved to a file")
	}
	if got := m.textarea.Value(); got != "one\ntwo\nthree" {
		t.Errorf("textarea = %q, want the pasted text", got)
	}
	// One line over the character limit is long, however few lines it has.
	m = updateModel(m, bracketedPaste(strings.Repeat("x", longPasteChars+1)))
	if len(st.saved) != 1 {
		t.Errorf("a %d-character line was not saved to a file", longPasteChars+1)
	}
}

// When the file cannot be written, the paste goes in as text and says why.
func TestPasteThatCannotBeSavedGoesInAsText(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetTextPasteHandler((&stager{err: errors.New("disk full")}).stage)
	m = updateModel(m, bracketedPaste(logLines(30, "\n")))
	if got := m.textarea.Value(); got != logLines(30, "\n") {
		t.Errorf("textarea holds %d characters, want the pasted text", len(got))
	}
	if !strings.Contains(m.View(), "disk full") {
		t.Errorf("View hides why the paste was not saved:\n%s", m.View())
	}
}

// On submit only the tags still in the text are sent, numbered again from 1
// so they match the order the files go to the agent in.
func TestSubmitSendsRemainingTagsRenumbered(t *testing.T) {
	st := &stager{}
	m := NewInputModel(">", nil)
	m.SetTextPasteHandler(st.stage)
	m = updateModel(m, bracketedPaste(logLines(30, "\n")))
	m = updateModel(m, bracketedPaste(logLines(40, "\n")))
	m = updateModel(m, bracketedPaste(logLines(50, "\n")))
	if got := m.textarea.Value(); got != "[paste1] [paste2] [paste3] " {
		t.Fatalf("textarea = %q", got)
	}
	m.textarea.SetValue("compare [paste2] with [paste3], then [paste2] again")
	m = updateModel(m, enterKey())
	if !m.Submitted() {
		t.Fatal("Enter did not submit")
	}
	if got := m.Value(); got != "compare [paste1] with [paste2], then [paste1] again" {
		t.Errorf("submitted %q, want the tags renumbered", got)
	}
	if got := m.PastedTexts(); len(got) != 2 || got[0] != "/tmp/paste-2.txt" || got[1] != "/tmp/paste-3.txt" {
		t.Errorf("PastedTexts() = %v, want the second and third pastes", got)
	}
}

// Esc clears the draft and the pastes with it.
func TestEscDropsPastes(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetTextPasteHandler((&stager{}).stage)
	m = updateModel(m, bracketedPaste(logLines(30, "\n")))
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	if strings.Contains(m.View(), "[paste1:") {
		t.Errorf("View still lists the paste after Esc:\n%s", m.View())
	}
	m = typeText(m, "[paste1]")
	m = updateModel(m, enterKey())
	if got := m.PastedTexts(); len(got) != 0 {
		t.Errorf("PastedTexts() = %v after Esc, want none", got)
	}
}

// Where the terminal passes Ctrl+V through, the composer reads the clipboard
// itself, so a long clipboard text becomes a tag too.
func TestCtrlVReadsTheClipboard(t *testing.T) {
	st := &stager{}
	clip := logLines(30, "\r\n")
	m := NewInputModel(">", nil)
	m.SetTextPasteHandler(st.stage)
	m.SetClipboardTextReader(func() (string, error) { return clip, nil })
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlV})
	if got := m.textarea.Value(); got != "[paste1] " {
		t.Errorf("textarea = %q after Ctrl+V of a long text", got)
	}
	clip = "short"
	m = updateModel(m, tea.KeyMsg{Type: tea.KeyCtrlV})
	if got := m.textarea.Value(); got != "[paste1] short" {
		t.Errorf("textarea = %q after Ctrl+V of a short text", got)
	}
}

// A Windows paste that grows past the limit stops going into the textarea and
// becomes a tag when it ends, in place of the part that had gone in.
func TestLongWindowsPasteBecomesTag(t *testing.T) {
	withBurstKeys(t)
	st := &stager{}
	m := NewInputModel(">", nil)
	m.SetTextPasteHandler(st.stage)
	m = typeText(m, "why:")
	m = pause(m)
	m = typeText(m, " ")
	m = pause(m)
	text := logLines(30, "\n")
	for i, r := range text {
		if r == '\n' {
			m = updateModel(m, enterKey())
		} else {
			m = updateModel(m, runeKey(r))
		}
		if i%100 == 99 {
			m = updateModel(m, pasteTickMsg{})
		}
	}
	if got := strings.Count(m.textarea.Value(), "\n"); got > longPasteLines {
		t.Errorf("the textarea took %d lines of a paste that is becoming a tag", got)
	}
	if view := m.View(); !strings.Contains(view, "[pasting: 30 lines so far]") {
		t.Errorf("View does not say the paste is arriving:\n%s", view)
	}
	m = settle(m)
	if got := m.textarea.Value(); got != "why: [paste1] " {
		t.Errorf("textarea = %q, want the tag after the typed text", got)
	}
	if len(st.saved) != 1 || st.saved[0] != text {
		t.Errorf("saved %d texts, want the whole paste once", len(st.saved))
	}
}

// The characters a Windows paste lost come back before the paste is saved.
func TestLongWindowsPasteSavesRepairedText(t *testing.T) {
	withBurstKeys(t)
	st := &stager{}
	text := strings.Replace(logLines(30, "\n"), "INFO", "INFO —", 1)
	m := NewInputModel(">", nil)
	m.SetTextPasteHandler(st.stage)
	m.SetClipboardTextReader(func() (string, error) { return text, nil })
	m = pasteKeys(m, strings.Replace(text, "—", "\x00", 1), 3)
	m = settle(m)
	if len(st.saved) != 1 || st.saved[0] != text {
		t.Errorf("saved %q, want the text with its dash back", st.saved)
	}
	if got := m.textarea.Value(); got != "[paste1] " {
		t.Errorf("textarea = %q", got)
	}
}

func TestDescribeText(t *testing.T) {
	cases := map[string]string{
		"":                           "0 lines, 0 bytes",
		"one":                        "1 line, 3 bytes",
		"a\nb\n":                     "2 lines, 4 bytes",
		strings.Repeat("x", 3<<10):   "1 line, 3.0 KB",
		strings.Repeat("x\n", 1<<19): "524288 lines, 1.0 MB",
	}
	for in, want := range cases {
		if got := DescribeText(in); got != want {
			t.Errorf("DescribeText(%d bytes) = %q, want %q", len(in), got, want)
		}
	}
}

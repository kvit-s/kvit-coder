package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// withBurstKeys enables the Windows paste-burst path for one test, whatever
// platform the test runs on.
func withBurstKeys(t *testing.T) {
	t.Helper()
	old := pasteBurstKeys
	pasteBurstKeys = true
	t.Cleanup(func() { pasteBurstKeys = old })
}

func withoutBurstKeys(t *testing.T) {
	t.Helper()
	old := pasteBurstKeys
	pasteBurstKeys = false
	t.Cleanup(func() { pasteBurstKeys = old })
}

func ctrlJKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyCtrlJ} }

func updateModel(m InputModel, msg tea.Msg) InputModel {
	mod, _ := m.Update(msg)
	return mod.(InputModel)
}

func typeText(m InputModel, s string) InputModel {
	for _, r := range s {
		m = updateModel(m, runeKey(r))
	}
	return m
}

// A paste burst is runes with no human gap between them: on Windows the
// console delivers a paste as one key event per character, so every pasted
// line break arrives as its own Enter. Those Enters must become newlines,
// not submits.
func TestBurstEnterInsertsNewline(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "line1")
	m = updateModel(m, enterKey())
	if m.Submitted() {
		t.Fatal("an Enter inside a paste burst submitted instead of a newline")
	}
	m = typeText(m, "line2")
	m = updateModel(m, enterKey())
	if m.Submitted() {
		t.Fatal("a second burst Enter submitted instead of a newline")
	}
	if got := m.textarea.Value(); got != "line1\nline2\n" {
		t.Errorf("textarea = %q, want the pasted lines", got)
	}
}

// A real submit follows thinking: an Enter after a pause still submits.
func TestSlowEnterSubmits(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "fix")
	m.lastKeyTime = time.Now().Add(-time.Second)
	m = updateModel(m, enterKey())
	if !m.Submitted() {
		t.Fatal("an Enter after a pause did not submit")
	}
	if got := m.Value(); got != "fix" {
		t.Errorf("submitted %q, want the typed text", got)
	}
}

// Recalling history and submitting is not a burst: navigation keys are not
// text, so the Enter right after them submits.
func TestEnterAfterHistoryNavSubmits(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", []string{"older", "newer"})
	m = updateModel(m, upKey())
	m = updateModel(m, enterKey())
	if !m.Submitted() {
		t.Fatal("an Enter right after history recall did not submit")
	}
	if got := m.Value(); got != "newer" {
		t.Errorf("submitted %q, want the recalled history", got)
	}
}

// Windows pastes carry CRLF: the CR half becomes a newline above, and the LF
// half lands as Ctrl+J right after. Swallow it so one pasted line break is
// one newline, not two.
func TestBurstCRLFIsOneNewline(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "a")
	m = updateModel(m, enterKey())
	m = updateModel(m, ctrlJKey())
	m = typeText(m, "b")
	if m.Submitted() {
		t.Fatal("a CRLF paste submitted")
	}
	if got := m.textarea.Value(); got != "a\nb" {
		t.Errorf("textarea = %q, want one newline per pasted line break", got)
	}
}

// A blank line inside a paste is two burst Enters in a row: both stay
// newlines.
func TestBurstBlankLinePreserved(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "a")
	m = updateModel(m, enterKey())
	m = updateModel(m, enterKey())
	m = typeText(m, "b")
	if m.Submitted() {
		t.Fatal("a paste with a blank line submitted")
	}
	if got := m.textarea.Value(); got != "a\n\nb" {
		t.Errorf("textarea = %q, want the blank line kept", got)
	}
}

// A lone Ctrl+J still inserts a newline, burst keys or not.
func TestCtrlJStillInsertsNewline(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "a")
	m = updateModel(m, ctrlJKey())
	if m.Submitted() {
		t.Fatal("Ctrl+J submitted")
	}
	if got := m.textarea.Value(); got != "a\n" {
		t.Errorf("textarea = %q, want the newline", got)
	}
}

// Where burst keys are off (unix, where bracketed paste never reads as
// "enter"), rapid typing plus Enter still submits exactly as before.
func TestRapidEnterSubmitsWhenBurstKeysOff(t *testing.T) {
	withoutBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "fix")
	m = updateModel(m, enterKey())
	if !m.Submitted() {
		t.Fatal("a rapid Enter did not submit with burst keys off")
	}
	if got := m.Value(); got != "fix" {
		t.Errorf("submitted %q, want the typed text", got)
	}
}

package ui

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func altV() tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'v'}, Alt: true}
}

func TestAltVPasteStagesImage(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetImagePasteHandler(func() (string, error) { return "/tmp/paste-1.png", nil })
	mod, _ := m.Update(altV())
	m = mod.(InputModel)
	if len(m.PastedImages()) != 1 || m.PastedImages()[0] != "/tmp/paste-1.png" {
		t.Fatalf("PastedImages() = %v", m.PastedImages())
	}
	if view := m.View(); !strings.Contains(view, "staged image 1") {
		t.Errorf("View() hides the staged image:\n%s", view)
	}
	// The key must not leak into the text.
	if m.textarea.Value() != "" {
		t.Errorf("paste key inserted text %q", m.textarea.Value())
	}
}

func TestAltVPasteFailureIsVisible(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetImagePasteHandler(func() (string, error) { return "", errors.New("no image in clipboard") })
	mod, _ := m.Update(altV())
	m = mod.(InputModel)
	if len(m.PastedImages()) != 0 {
		t.Fatalf("PastedImages() = %v, want none", m.PastedImages())
	}
	if view := m.View(); !strings.Contains(view, "no image in clipboard") {
		t.Errorf("View() hides the failure:\n%s", view)
	}
}

func TestAltVWithoutHandlerFallsThrough(t *testing.T) {
	m := NewInputModel(">", nil)
	mod, _ := m.Update(altV())
	m = mod.(InputModel)
	if len(m.PastedImages()) != 0 || m.pasteNotice != "" {
		t.Errorf("no handler: PastedImages() = %v, notice = %q", m.PastedImages(), m.pasteNotice)
	}
}

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
	// The label lands in the text at the cursor, so the prompt can refer to
	// it: "look at [image1] and compare with [image2]".
	if got := m.textarea.Value(); got != "[image1] " {
		t.Errorf("textarea = %q, want %q", got, "[image1] ")
	}
	if view := m.View(); !strings.Contains(view, "[image1: /tmp/paste-1.png]") {
		t.Errorf("View() hides the image list:\n%s", view)
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

func TestAltVPasteAddsSpacing(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetImagePasteHandler(func() (string, error) { return "/tmp/a.png", nil })
	m.textarea.SetValue("look at")
	m.textarea.CursorEnd()
	mod, _ := m.Update(altV())
	m = mod.(InputModel)
	if got := m.textarea.Value(); got != "look at [image1] " {
		t.Errorf("textarea = %q, want %q", got, "look at [image1] ")
	}
}

func TestAltVPasteTwoImagesNumbered(t *testing.T) {
	m := NewInputModel(">", nil)
	n := 0
	m.SetImagePasteHandler(func() (string, error) {
		n++
		return "/tmp/paste-" + itoa(n) + ".png", nil
	})
	mod, _ := m.Update(altV())
	m = mod.(InputModel)
	mod, _ = m.Update(altV())
	m = mod.(InputModel)
	if got := m.textarea.Value(); !strings.Contains(got, "[image1]") || !strings.Contains(got, "[image2]") {
		t.Errorf("textarea = %q, want both [image1] and [image2]", got)
	}
	view := m.View()
	if !strings.Contains(view, "[image1: /tmp/paste-1.png]") || !strings.Contains(view, "[image2: /tmp/paste-2.png]") {
		t.Errorf("View() hides the image list:\n%s", view)
	}
}

func TestSetStagedImagesContinuesNumbering(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetStagedImages([]string{"/tmp/staged.png"})
	m.SetImagePasteHandler(func() (string, error) { return "/tmp/paste.png", nil })
	mod, _ := m.Update(altV())
	m = mod.(InputModel)
	if got := m.textarea.Value(); got != "[image2] " {
		t.Errorf("textarea = %q, want %q (continues after staged [image1])", got, "[image2] ")
	}
	view := m.View()
	if !strings.Contains(view, "[image1: /tmp/staged.png]") || !strings.Contains(view, "[image2: /tmp/paste.png]") {
		t.Errorf("View() hides the seeded list:\n%s", view)
	}
}

func TestEscKeepsSeededClearsPasted(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetStagedImages([]string{"/tmp/staged.png"})
	m.SetImagePasteHandler(func() (string, error) { return "/tmp/paste.png", nil })
	mod, _ := m.Update(altV())
	m = mod.(InputModel)
	esc := tea.KeyMsg{Type: tea.KeyEsc}
	mod, _ = m.Update(esc)
	m = mod.(InputModel)
	if m.textarea.Value() != "" {
		t.Errorf("textarea = %q, want cleared", m.textarea.Value())
	}
	if got := m.PastedImages(); len(got) != 1 || got[0] != "/tmp/staged.png" {
		t.Errorf("PastedImages() = %v, want only the seeded image", got)
	}
}

func TestImageLabel(t *testing.T) {
	if ImageLabel(1) != "[image1]" || ImageLabel(12) != "[image12]" {
		t.Errorf("ImageLabel wrong: %q %q", ImageLabel(1), ImageLabel(12))
	}
}

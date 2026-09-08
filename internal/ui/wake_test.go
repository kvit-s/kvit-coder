package ui

import (
	"strings"
	"testing"
)

// TestWakeFiresWhenEmpty: inbox pending while the composer is still empty
// submits itself as an inbox-only turn.
func TestWakeFiresWhenEmpty(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetWakePoll(func() (int, string) { return 2, "" })

	mod, _ := m.Update(wakeTickMsg{})
	m = mod.(InputModel)
	if !m.Submitted() || !m.WakeFired() {
		t.Fatalf("submitted=%v wake=%v, want the composer to fire", m.Submitted(), m.WakeFired())
	}
	if got := m.Value(); got != "" {
		t.Errorf("Value() = %q, want empty for an inbox-only turn", got)
	}
}

// TestWakeBadgesWhenTyping: inbox pending while the user is composing does
// not steal the composer — it badges, and the messages ride along on submit.
func TestWakeBadgesWhenTyping(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetWakePoll(func() (int, string) { return 3, "" })
	m.textarea.SetValue("half-written thought")
	m.textarea.CursorEnd()

	mod, cmd := m.Update(wakeTickMsg{})
	m = mod.(InputModel)
	if m.Submitted() || m.WakeFired() {
		t.Fatal("the composer fired while the user was typing")
	}
	if cmd == nil {
		t.Error("no follow-up tick scheduled after badging")
	}
	if view := m.View(); !strings.Contains(view, "[inbox: 3 pending") {
		t.Errorf("View() hides the pending badge:\n%s", view)
	}
}

// TestWakeQuietWhenNone: nothing pending means no badge, no fire, and the
// tick reschedules.
func TestWakeQuietWhenNone(t *testing.T) {
	m := NewInputModel(">", nil)
	m.SetWakePoll(func() (int, string) { return 0, "" })

	mod, cmd := m.Update(wakeTickMsg{})
	m = mod.(InputModel)
	if m.Submitted() || m.WakeFired() {
		t.Fatal("the composer fired with nothing pending")
	}
	if cmd == nil {
		t.Error("no follow-up tick scheduled while idle")
	}
	if view := m.View(); strings.Contains(view, "inbox:") {
		t.Errorf("View() shows a badge with nothing pending:\n%s", view)
	}
}

// TestWakeDisabledWithoutPoll: no waiter means the tick is a no-op —
// existing callers that never set a poll see no behavior change.
func TestWakeDisabledWithoutPoll(t *testing.T) {
	m := NewInputModel(">", nil)
	mod, cmd := m.Update(wakeTickMsg{})
	m = mod.(InputModel)
	if m.Submitted() || m.WakeFired() {
		t.Fatal("the composer fired with no waiter installed")
	}
	if cmd != nil {
		t.Error("a tick was scheduled with no waiter installed")
	}
}

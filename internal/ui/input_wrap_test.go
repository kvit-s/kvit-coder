package ui

import (
	"strings"
	"testing"
)

// A long dictation-style single line must occupy several visual rows and grow
// the input instead of staying at height 1 and scrolling the start away.
func TestSoftWrappedHeightSingleLongLine(t *testing.T) {
	longLine := strings.TrimSpace(strings.Repeat("word ", 60)) // ~300 chars, no newlines
	if strings.Contains(longLine, "\n") {
		t.Fatal("test input must be a single hard line")
	}
	got := softWrappedHeight(longLine, 80)
	if got <= 1 {
		t.Fatalf("softWrappedHeight(%d chars, 80) = %d, want > 1", len(longLine), got)
	}
}

func TestAdjustHeightGrowsForLongSingleLine(t *testing.T) {
	m := NewInputModel(">", nil)
	m.textarea.SetWidth(60)
	m.maxHeight = 20

	longLine := strings.TrimSpace(strings.Repeat("word ", 60))
	m.textarea.SetValue(longLine)
	m.adjustHeight()

	want := softWrappedHeight(longLine, m.inputWidth())
	if want <= 1 {
		t.Fatalf("test setup: want wrapped height > 1, got %d", want)
	}
	if got := m.textarea.Height(); got != want {
		t.Fatalf("Height() = %d, want %d (wrapped rows)", got, want)
	}
}

func TestAdjustHeightCapsAtMaxHeight(t *testing.T) {
	m := NewInputModel(">", nil)
	m.textarea.SetWidth(40)
	m.maxHeight = 5

	longLine := strings.TrimSpace(strings.Repeat("word ", 200))
	m.textarea.SetValue(longLine)
	m.adjustHeight()

	if got := m.textarea.Height(); got != 5 {
		t.Fatalf("Height() = %d, want capped maxHeight 5", got)
	}
	if total := m.totalSoftLines(); total <= 5 {
		t.Fatalf("totalSoftLines() = %d, want > maxHeight for this input", total)
	}
}

func TestCursorSoftRowTracksWrappedRows(t *testing.T) {
	m := NewInputModel(">", nil)
	m.textarea.SetWidth(40)
	m.maxHeight = 20

	longLine := strings.TrimSpace(strings.Repeat("word ", 40))
	m.textarea.SetValue(longLine)
	m.textarea.CursorEnd()
	m.adjustHeight()

	lastRow := m.totalSoftLines() - 1
	if got := m.cursorSoftRow(); got != lastRow {
		t.Fatalf("cursorSoftRow() at end = %d, want last visual row %d", got, lastRow)
	}
}

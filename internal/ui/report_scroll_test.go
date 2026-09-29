package ui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/kvit-s/kvit-coder/internal/report"
)

// tallReport is a full-app-shaped report: several things done plus something
// to answer, with details long enough that expanding it overflows a short
// terminal.
func tallReport() *report.Report {
	blocks := []report.Block{
		{Type: report.BlockCheck, ID: "bench", Summary: "Fresh bench: open ~0.16s, scroll ~1.55ms, keystroke ~7ms, heap 81MB.",
			Status: report.CheckPassed, Evidence: "two --bench runs on current HEAD"},
		{Type: report.BlockCheck, ID: "build", Summary: "Cold build 11.8s; unpacked 32MB/146 files vs Qt 138MB/1,581 files.",
			Status: report.CheckPassed, Evidence: "empty GOCACHE build plus four-platform cross build",
			Details: "GOCACHE=$(mktemp -d) go build -o /tmp/kvit-notes-cold ./cmd/kvit-notes takes 11.78s wall " +
				"at 689% CPU with a 533MB peak single-process RSS, matching the logged figures within noise."},
		{Type: report.BlockCheck, ID: "tests", Summary: "858 Go tests pass in 21s; Qt suite takes 14.1 min.",
			Status: report.CheckPassed, Evidence: "go test ./... all packages ok in 21.4s elapsed",
			Details: "858 func Test counted via git ls-files; the Qt side runs 113 test programs in 14.1 minutes, " +
				"so the Go suite passing in seconds is the number that matters for the loop."},
		{Type: report.BlockFinding, ID: "keystroke", Summary: "Keystroke 6.5-7.7ms vs logged 3.2-3.7ms needs a quiet-machine retest.",
			Impact: "Two runs agree with each other but not with the logged editor-stage figure.",
			Details: "Both fresh runs land at 6.51ms and 7.74ms against the logged 3.2-3.7ms, so one clean retest " +
				"on a quiet machine settles whether the finished app regressed or the labeller moved."},
		{Type: report.BlockNext, ID: "log-it", Summary: "Record these full-app figures in migration-log.md's Before-and-after table?",
			Options: []report.Option{
				{ID: "write", Label: "Write log entry", Effect: report.EffectDispatch,
					Instruction: "Add a 2026-09-29 migration-log entry with the fresh bench, build, size, memory and test figures."},
				{ID: "review", Label: "Review numbers first", Effect: report.EffectCollect},
				{ID: "skip", Label: "Skip for now", Effect: report.EffectResolve},
			},
			Recommendation: "write", RecommendationReason: "The log's Go column is still prototype-only.",
			ResponseType: report.ResponseSingle},
	}
	return &report.Report{
		TaskStatus: report.StatusNeedsAction,
		Headline:   "Fresh full-app numbers: 32MB/146 files vs 138MB/1,581, 858 tests in 21s, bench rerun done",
		Blocks:     blocks,
	}
}

func sizedCard(rep *report.Report, width, height int) InputModel {
	m := withCard(rep)
	mod, _ := m.Update(tea.WindowSizeMsg{Width: width, Height: height})
	return mod.(InputModel)
}

func pgKey(t tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: t} }

// An expanded report taller than the terminal must fit on screen: the frame
// gets a window plus a scroll indicator instead of pushing its top away.
func TestTallCardFitsTheTerminal(t *testing.T) {
	m := press(sizedCard(tallReport(), 80, 20), "0")
	view := m.View()
	if rows := strings.Count(view, "\n") + 1; rows > 20 {
		t.Errorf("expanded view is %d rows in a 20-row terminal:\n%s", rows, view)
	}
	if !strings.Contains(view, "pgup/pgdn to scroll") {
		t.Errorf("no scroll indicator on a truncated card:\n%s", view)
	}
	if !strings.Contains(view, "pgup/pgdn scroll") {
		t.Errorf("the footer does not offer the scroll keys:\n%s", view)
	}
	// The top stays reachable: the headline is what shows first.
	if !strings.Contains(view, "Fresh full-app numbers") {
		t.Errorf("the headline is not in the first window:\n%s", view)
	}
}

// pgdown pages the window down past the headline; pgup brings it back.
func TestCardPgDownAndPgUp(t *testing.T) {
	m := press(sizedCard(tallReport(), 80, 20), "0")
	if m.cardOffset != 0 {
		t.Fatalf("offset is %d after expanding, want 0", m.cardOffset)
	}
	m = pressKey(t, m, pgKey(tea.KeyPgDown))
	if m.cardOffset <= 0 {
		t.Fatalf("pgdown did not move the window: offset %d", m.cardOffset)
	}
	if view := m.View(); strings.Contains(view, "Fresh full-app numbers") {
		t.Errorf("the headline is still visible after paging down:\n%s", view)
	}
	if view := m.View(); !strings.Contains(view, "↑") || !strings.Contains(view, "pgup/pgdn to scroll") {
		t.Errorf("no above-indicator after paging down:\n%s", view)
	}
	m = pressKey(t, m, pgKey(tea.KeyPgUp))
	if m.cardOffset != 0 {
		t.Errorf("pgup did not return to the top: offset %d", m.cardOffset)
	}
	if view := m.View(); !strings.Contains(view, "Fresh full-app numbers") {
		t.Errorf("the headline is gone after paging back up:\n%s", view)
	}
}

// The offset clamps at both ends, and home/end jump there.
func TestCardScrollClampsAtTheEnds(t *testing.T) {
	m := press(sizedCard(tallReport(), 80, 20), "0")
	m = pressKey(t, m, pgKey(tea.KeyPgUp))
	if m.cardOffset != 0 {
		t.Errorf("pgup at the top moved to %d", m.cardOffset)
	}
	for i := 0; i < 10; i++ {
		m = pressKey(t, m, pgKey(tea.KeyPgDown))
	}
	rows, _ := cardRows(m.card, m.cardOpts(), m.cardWidth(), true)
	vis, cut := m.cardVisibleBudget(len(rows))
	if !cut {
		t.Fatal("the tall card is not truncated at height 20")
	}
	if want := len(rows) - vis; m.cardOffset != want {
		t.Errorf("offset is %d at the bottom, want %d", m.cardOffset, want)
	}
	if view := m.View(); !strings.Contains(view, "Skip for now") {
		t.Errorf("the last option is not visible at the bottom:\n%s", view)
	}
	m = pressKey(t, m, pgKey(tea.KeyHome))
	if m.cardOffset != 0 {
		t.Errorf("home did not jump to the top: offset %d", m.cardOffset)
	}
	m = pressKey(t, m, pgKey(tea.KeyEnd))
	rows, _ = cardRows(m.card, m.cardOpts(), m.cardWidth(), true)
	vis, _ = m.cardVisibleBudget(len(rows))
	if want := len(rows) - vis; m.cardOffset != want {
		t.Errorf("end did not jump to the bottom: offset %d, want %d", m.cardOffset, want)
	}
}

// A card that fits shows everything with no indicator and no scroll keys:
// pgup keeps its normal job instead of moving anything.
func TestShortCardDoesNotScroll(t *testing.T) {
	m := sizedCard(decisionReport(), 100, 40)
	if m.cardScrollable() {
		t.Error("a fitting card reports itself scrollable")
	}
	if view := m.View(); strings.Contains(view, "pgup/pgdn") {
		t.Errorf("scroll keys advertised on a fitting card:\n%s", view)
	}
	before := m.View()
	m = pressKey(t, m, pgKey(tea.KeyPgDown))
	if m.cardOffset != 0 {
		t.Errorf("pgdown moved an untruncated card: offset %d", m.cardOffset)
	}
	if got := m.View(); got != before {
		t.Error("pgdown changed the view of an untruncated card")
	}
}

// Without a resize the card is uncapped, exactly as before: every test that
// never sets a size keeps seeing the whole report.
func TestCardUncappedWithoutResize(t *testing.T) {
	m := press(withCard(tallReport()), "0")
	if strings.Contains(m.View(), "pgup/pgdn") {
		t.Errorf("scroll UI without a known terminal size:\n%s", m.View())
	}
	if flat := strings.Join(strings.Fields(m.View()), " "); !strings.Contains(flat, "settles whether the finished app regressed") {
		t.Error("uncapped card lost details it used to show")
	}
}

// Closing the details cannot strand the offset past the shorter card, and
// opening starts back at the top.
func TestToggleDetailsResetsTheOffset(t *testing.T) {
	m := press(sizedCard(tallReport(), 80, 20), "0")
	m = pressKey(t, m, pgKey(tea.KeyPgDown))
	if m.cardOffset == 0 {
		t.Fatal("setup did not scroll: offset still 0")
	}
	m = press(m, "0") // close
	if m.cardOffset != 0 {
		t.Errorf("closing left offset at %d", m.cardOffset)
	}
	m = press(m, "0") // open again
	if m.cardOffset != 0 {
		t.Errorf("reopening left offset at %d", m.cardOffset)
	}
	if view := m.View(); !strings.Contains(view, "Fresh full-app numbers") {
		t.Errorf("reopened card does not start at the top:\n%s", view)
	}
}

// home/end stay cursor keys while a draft is being typed: the card keeps its
// position instead of jumping out from under the text.
func TestHomeKeepsDraftEditing(t *testing.T) {
	m := press(sizedCard(tallReport(), 80, 20), "0")
	m = pressKey(t, m, pgKey(tea.KeyPgDown))
	off := m.cardOffset
	if off == 0 {
		t.Fatal("setup did not scroll: offset still 0")
	}
	m = press(m, "x")
	m = pressKey(t, m, pgKey(tea.KeyHome))
	if m.cardOffset != off {
		t.Errorf("home moved the card to %d while typing, was %d", m.cardOffset, off)
	}
	if got := m.textarea.Value(); got != "x" {
		t.Errorf("textarea = %q, want the draft kept", got)
	}
}

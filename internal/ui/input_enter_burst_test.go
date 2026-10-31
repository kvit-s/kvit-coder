package ui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

// lostKey is one key event of an Alt+numpad sequence, the way bubbletea
// passes it on: an Alt key with a NUL rune and no character.
func lostKey() tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{0}, Alt: true} }

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

// pause makes the next key arrive after a human gap.
func pause(m InputModel) InputModel {
	m.lastKeyTime = m.lastKeyTime.Add(-time.Second)
	return m
}

// settle lets the time the paste path waits for pass: a held key nothing
// followed acts, and the collected paste goes into the textarea.
func settle(m InputModel) InputModel {
	if m.paste.held != nil {
		m = updateModel(m, heldKeyMsg{gen: m.paste.heldGen})
	}
	return updateModel(pause(m), pasteTickMsg{})
}

// pasteKeys is how a Windows paste reaches the composer: one key per
// character, Enter for each line break, and the given number of
// character-less Alt+numpad key events for each '\x00' in the text.
func pasteKeys(m InputModel, s string, lostPerChar int) InputModel {
	for _, r := range s {
		switch r {
		case '\n':
			m = updateModel(m, enterKey())
		case '\t':
			m = updateModel(m, tea.KeyMsg{Type: tea.KeyTab})
		case 0:
			for i := 0; i < lostPerChar; i++ {
				m = updateModel(m, lostKey())
			}
		default:
			m = updateModel(m, runeKey(r))
		}
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
	m = settle(m)
	if m.Submitted() {
		t.Fatal("a second burst Enter submitted instead of a newline")
	}
	if got := m.textarea.Value(); got != "line1\nline2\n" {
		t.Errorf("textarea = %q, want the pasted lines", got)
	}
}

// A real submit follows thinking: an Enter after a pause submits once the
// hold has passed with nothing after it.
func TestSlowEnterSubmits(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "fix")
	m = updateModel(pause(m), enterKey())
	if m.Submitted() {
		t.Fatal("the Enter submitted before its hold passed")
	}
	m = settle(m)
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
	m = settle(m)
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
	m = settle(m)
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
	m = settle(m)
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
	m = settle(m)
	if m.Submitted() {
		t.Fatal("Ctrl+J submitted")
	}
	if got := m.textarea.Value(); got != "a\n" {
		t.Errorf("textarea = %q, want the newline", got)
	}
	m = updateModel(pause(m), ctrlJKey())
	if got := m.textarea.Value(); got != "a\n\n" {
		t.Errorf("textarea = %q after a typed Ctrl+J, want a second newline", got)
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

// Text copied from the end of a line starts with a line break. Typed text, a
// pause, then the paste: the first Enter is held, the keys after it show it
// was pasted, and it becomes a newline instead of sending the typed text.
func TestPasteStartingWithLineBreakDoesNotSubmit(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "look:")
	m = pause(m)
	staleGen := 0
	m = updateModel(m, enterKey())
	staleGen = m.paste.heldGen
	m = pasteKeys(m, "line1\nline2", 0)
	// The hold's own tick arrives late and must do nothing.
	m = updateModel(m, heldKeyMsg{gen: staleGen})
	m = settle(m)
	if m.Submitted() {
		t.Fatalf("the paste's leading line break submitted %q", m.Value())
	}
	if got := m.textarea.Value(); got != "look:\nline1\nline2" {
		t.Errorf("textarea = %q, want the typed text then the pasted lines", got)
	}
}

// A pasted log starting with a date must not pick report option 2 on an empty
// composer: the digit is held, and the rest of the paste makes it text.
func TestPasteOnEmptyComposerDoesNotPickOption(t *testing.T) {
	withBurstKeys(t)
	m := withCard(decisionReport())
	m = pasteKeys(m, "2026-10-31 INFO started\nnext line", 0)
	m = settle(m)
	if m.Submitted() || m.CardAnswerPicked() != nil {
		t.Fatalf("the paste picked an option: submitted=%v value=%q", m.Submitted(), m.Value())
	}
	if got := m.textarea.Value(); got != "2026-10-31 INFO started\nnext line" {
		t.Errorf("textarea = %q, want the pasted text", got)
	}
}

// A digit pressed on its own still picks, once its hold passes.
func TestTypedDigitStillPicksOption(t *testing.T) {
	withBurstKeys(t)
	m := withCard(decisionReport())
	m = updateModel(m, runeKey('1'))
	if m.Submitted() {
		t.Fatal("the digit acted before its hold passed")
	}
	m = settle(m)
	if !m.Submitted() || m.Value() != "Implement reordering while preserving hidden-row positions." {
		t.Fatalf("the digit did not pick the dispatch option: submitted=%v value=%q", m.Submitted(), m.Value())
	}
}

// A line ending in a character the keyboard layout cannot type ends in
// key events with no character. The Enter after them is still pasted.
func TestLineEndingInUntypeableCharacterStaysOneMessage(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = pasteKeys(m, "tests pass \x00\nnext", 3)
	m = settle(m)
	if m.Submitted() {
		t.Fatalf("the paste submitted %q at a line ending in a lost character", m.Value())
	}
	if got := m.textarea.Value(); got != "tests pass \nnext" {
		t.Errorf("textarea = %q, want both lines", got)
	}
}

// A pasted tab is indentation, not @path completion.
func TestPastedTabIsText(t *testing.T) {
	withBurstKeys(t)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "input.go"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m = pasteKeys(m, "see @in\tnow", 0)
	m = settle(m)
	if got := m.textarea.Value(); got != "see @in    now" {
		t.Errorf("textarea = %q, want the tab kept as text", got)
	}
}

// The reason for collecting: a paste of a few hundred lines used to take
// minutes, because every key redrew and re-measured the whole text. It must
// now take well under a second; the bound leaves room for a slow runner.
func TestLongPasteIsFast(t *testing.T) {
	withBurstKeys(t)
	var lines []string
	for i := 0; i < 250; i++ {
		lines = append(lines, fmt.Sprintf("2026-10-31 08:40:%02d DEBUG request id=%05d path=/api/v1/items/%d status=200", i%60, i, i))
	}
	text := strings.Join(lines, "\n")
	m := NewInputModel(">", nil)
	m = updateModel(m, tea.WindowSizeMsg{Width: 120, Height: 40})
	start := time.Now()
	for i, r := range text {
		if r == '\n' {
			m = updateModel(m, enterKey())
		} else {
			m = updateModel(m, runeKey(r))
		}
		_ = m.View()
		// The flush tick lands among the keys while they arrive.
		if i%500 == 499 {
			m = updateModel(m, pasteTickMsg{})
			_ = m.View()
		}
		if time.Since(start) > 10*time.Second {
			t.Fatalf("pasting %d characters took over 10s", len(text))
		}
	}
	m = settle(m)
	if m.Submitted() {
		t.Fatal("the long paste submitted")
	}
	if got := m.textarea.Value(); got != text {
		t.Errorf("textarea holds %d lines, want all %d", strings.Count(got, "\n")+1, len(lines))
	}
	t.Logf("%d keys in %v", len(text), time.Since(start))
}

// While a paste is collected nothing on screen changes, so View returns the
// last frame; once it is inserted the frame shows it.
func TestViewStandsWhileCollecting(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m = typeText(m, "a")
	before := m.View()
	m = typeText(m, "bc")
	if got := m.View(); got != before {
		t.Error("View changed while the paste was only being collected")
	}
	m = settle(m)
	if got := m.View(); !strings.Contains(got, "abc") {
		t.Errorf("View after the paste went in:\n%s", got)
	}

	// A frame older than the last update is never reused, even when nobody
	// called View in between.
	m = typeText(pause(m), "d")
	m = typeText(m, "e")
	if got := m.View(); !strings.Contains(got, "abcd") {
		t.Errorf("View reused a frame from before the last update:\n%s", got)
	}
}

// A paste still being collected is composing: the inbox wake must not fire
// and throw it away just because the textarea is still empty.
func TestWakeWaitsForCollectedPaste(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m.SetWakePoll(func() (int, string) { return 1, "" })
	m = updateModel(m, lostKey())
	m = updateModel(m, enterKey())
	m = updateModel(m, wakeTickMsg{})
	if m.WakeFired() {
		t.Fatal("the wake fired over a paste being collected")
	}

	m = NewInputModel(">", nil)
	m.SetWakePoll(func() (int, string) { return 1, "" })
	m = updateModel(m, enterKey())
	if m.paste.held == nil {
		t.Fatal("test setup: the Enter was not held")
	}
	m = updateModel(m, wakeTickMsg{})
	if m.WakeFired() {
		t.Fatal("the wake fired over a held Enter")
	}
}

// The console drops characters it sends as Alt+numpad sequences. When the
// paste ends, the clipboard holding the same text with them puts them back.
func TestPasteRecoversLostCharactersFromClipboard(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m.SetClipboardTextReader(func() (string, error) { return "a — b\r\nc ✓\tok", nil })
	m = pasteKeys(m, "a \x00 b\nc \x00\tok", 3)
	m = settle(m)
	if got := m.textarea.Value(); got != "a — b\nc ✓    ok" {
		t.Errorf("textarea = %q, want the clipboard's characters back", got)
	}
	m = typeText(pause(m), "!")
	if got := m.textarea.Value(); !strings.HasSuffix(got, "ok!") {
		t.Errorf("textarea = %q, want the cursor left after the paste", got)
	}
}

// The repair touches only the pasted text: what was there before and after
// the cursor stays, and the cursor ends after the paste.
func TestRecoveryKeepsSurroundingText(t *testing.T) {
	withBurstKeys(t)
	m := NewInputModel(">", nil)
	m.SetClipboardTextReader(func() (string, error) { return "x→y", nil })
	m.textarea.SetValue("first\nAB")
	m.textarea.SetCursor(1)
	m = pasteKeys(m, "x\x00y", 3)
	m = settle(m)
	m = typeText(pause(m), "z")
	if got := m.textarea.Value(); got != "first\nAx→yzB" {
		t.Errorf("textarea = %q, want the paste repaired in place", got)
	}
}

// Nothing is replaced when the clipboard does not hold what was pasted, or
// cannot be read, or nothing was lost.
func TestRecoveryNeedsMatchingClipboard(t *testing.T) {
	withBurstKeys(t)
	for name, clip := range map[string]func() (string, error){
		"different": func() (string, error) { return "something else entirely", nil },
		"error":     func() (string, error) { return "", errors.New("clipboard busy") },
	} {
		m := NewInputModel(">", nil)
		m.SetClipboardTextReader(clip)
		m = pasteKeys(m, "a \x00 b", 3)
		m = settle(m)
		if got := m.textarea.Value(); got != "a  b" {
			t.Errorf("%s: textarea = %q, want the typed text left alone", name, got)
		}
	}
	read := false
	m := NewInputModel(">", nil)
	m.SetClipboardTextReader(func() (string, error) { read = true; return "abc", nil })
	m = settle(pasteKeys(m, "abc", 0))
	if read {
		t.Error("the clipboard was read for a paste that lost nothing")
	}
}

func TestRecoverPastedText(t *testing.T) {
	cases := []struct {
		name  string
		typed string
		clip  string
		lost  int
		want  string
		ok    bool
	}{
		{"one lost character", "a  b", "a — b", 3, "a — b", true},
		{"crlf becomes newline", "x\ny", "x\r\n→y", 2, "x\n→y", true},
		{"lone alt press", "ab", "a✓b", 1, "", false},
		{"more missing than lost", "ab", "a✓✓b", 2, "", false},
		{"missing newline", "ab", "a\nb", 4, "", false},
		{"typed text not in clipboard", "abz", "a✓b", 4, "", false},
		{"nothing missing", "ab", "ab", 4, "", false},
		{"missing ascii", "ab", "axb", 4, "", false},
		{"nothing typed", "", "✓", 4, "", false},
	}
	for _, c := range cases {
		got, ok := recoverPastedText([]rune(c.typed), c.clip, c.lost)
		if ok != c.ok || string(got) != c.want {
			t.Errorf("%s: recoverPastedText(%q, %q, %d) = %q, %v; want %q, %v", c.name, c.typed, c.clip, c.lost, string(got), ok, c.want, c.ok)
		}
	}
}

// Typing one accented letter inside a draft keeps the cursor where it is:
// the paste check counts characters, not bytes.
func TestTypedAccentKeepsCursor(t *testing.T) {
	withoutBurstKeys(t)
	m := NewInputModel(">", nil)
	m.textarea.SetValue("ab\ncd")
	m.textarea.CursorUp()
	m.textarea.SetCursor(1)
	m = typeText(m, "éx")
	if got := m.textarea.Value(); got != "aéxb\ncd" {
		t.Errorf("textarea = %q, want the letters typed at the cursor", got)
	}
}

package ui

import (
	"runtime"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/bubbles/runeutil"
	tea "github.com/charmbracelet/bubbletea"
)

// Pasting into the composer on Windows.
//
// On Windows bubbletea reads the console input buffer, and a paste from
// Windows Terminal reaches it as one key event per character. The console
// host swallows the bracketed-paste markers the terminal wraps around the
// text, so every pasted line break is an Enter keypress. A character the
// keyboard layout cannot type (—, →, ✓, “ ”, emoji) arrives as an Alt+numpad
// sequence whose character is only on the Alt release, which bubbletea drops,
// leaving key events that carry no character at all.
//
// Applied one at a time through the textarea, each of those keys costs time
// in proportion to everything already in the composer: a few hundred pasted
// lines take minutes to appear, and once a single key takes longer than
// pasteEnterWindow the pasted Enters no longer look like a paste and submit
// the text a line at a time.
//
// So the composer collects a paste instead of applying it key by key:
//
//   - A key that arrives within pasteEnterWindow of a text key, or of another
//     pasted key, is part of a paste. Text, Tab, Enter and Ctrl+J are
//     buffered in pasteState.buf, and a pasteTickMsg every
//     pasteFlushInterval inserts the buffer with one InsertString. Nothing
//     on screen changes between flushes, so View returns the previous frame
//     instead of drawing the whole text again.
//   - A key that acts instead of typing (Enter, or a digit that would pick a
//     report option on an empty composer) is held for pasteHoldDelay. If
//     another key arrives first, the held key was the first character of a
//     paste and goes in as text; otherwise it acts.
//   - When a paste ends having lost characters to Alt+numpad sequences, the
//     clipboard is read. If it holds the pasted text with those characters
//     in it, the clipboard's copy replaces what was typed.
//   - A paste that grows past the long-paste limit (input_text_paste.go)
//     stops going into the textarea and becomes a [pasteN] tag when it ends.

// pasteBurstKeys reports whether pastes arrive as individual keypresses, so
// that the handling above is needed. On Windows bubbletea reads the console
// input buffer, where a paste is one key event per character. On unix the
// same paste arrives as one bracketed message, so the handling stays off and
// keys are applied as they come. Tests set it directly to drive the Windows
// path on any platform.
var pasteBurstKeys = runtime.GOOS == "windows"

// pasteEnterWindow is how soon after a text key another key must arrive to
// count as part of the same paste. Pasted keys are already queued in the
// console input buffer when the first one is read, so they arrive with gaps of
// microseconds, at most bubbletea's 16 ms poll when the buffer briefly runs
// dry, whereas a typed Enter comes after a human pause. An Enter typed inside
// the window costs one extra Enter: it inserts a newline, and the next Enter
// submits.
const pasteEnterWindow = 100 * time.Millisecond

// pasteHoldDelay is how long a key that acts is held to see whether more keys
// follow it. It covers bubbletea's 16 ms poll with room to spare, and delays
// a typed submit by less than anyone notices.
const pasteHoldDelay = 50 * time.Millisecond

// pasteFlushInterval is how often collected paste text is inserted while a
// paste is still arriving: about one frame, so a long paste is seen growing.
const pasteFlushInterval = 16 * time.Millisecond

// pasteTickMsg flushes the collected paste text and checks whether the paste
// has ended.
type pasteTickMsg struct{}

// heldKeyMsg releases a held key if nothing followed it. gen matches the hold
// it was scheduled for, so a tick left over from an earlier hold does nothing.
type heldKeyMsg struct{ gen int }

// pasteState is the composer's record of a paste in progress.
type pasteState struct {
	// buf is pasted text collected since the last flush.
	buf []rune
	// tickArmed says a pasteTickMsg is on its way; there is never more than
	// one.
	tickArmed bool
	// held is a key that acts, waiting to see whether a paste follows it.
	held    *tea.KeyMsg
	heldGen int
	// releasing is set while a held key that nothing followed is handled, so
	// it acts instead of being held again.
	releasing bool
	// burst is the paste in progress, kept so that characters lost to
	// Alt+numpad sequences can be put back when it ends. Nil between pastes.
	burst *pasteBurst
	// prevInserted is what the last key handled the ordinary way inserted. A
	// paste's first character is handled that way, because nothing before it
	// shows it is a paste, so the paste record starts from it.
	prevInserted []rune
}

// pasteBurst is one paste as it was typed into the composer.
type pasteBurst struct {
	// start is the rune offset in the composer text where the paste begins.
	start int
	// text is every character the paste inserted, before the textarea turns
	// tabs into spaces.
	text []rune
	// lines counts the line breaks in text.
	lines int
	// lost counts key events that carried no character: the Alt press and
	// numpad digits of the sequences that stand in for untypeable characters.
	lost int
}

// SetClipboardTextReader installs the clipboard reader used to put back
// characters a Windows paste lost. Nil leaves them lost. The driver sets it;
// the model itself never touches the clipboard.
func (m *InputModel) SetClipboardTextReader(fn func() (string, error)) {
	m.onClipboardText = fn
}

// textKey reports whether a key types its runes. Alt+letter is a shortcut and
// a bracketed paste is handled whole by the textarea, so neither counts.
func textKey(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeySpace {
		return true
	}
	if msg.Type != tea.KeyRunes || msg.Alt || msg.Paste || len(msg.Runes) == 0 {
		return false
	}
	for _, r := range msg.Runes {
		if r == 0 {
			return false
		}
	}
	return true
}

// lostCharKey reports whether a key is part of an Alt+numpad sequence: the
// console sends the Alt press and each numpad digit with no character, and
// bubbletea turns each into an Alt key with a NUL rune. A lone Alt press looks
// the same.
func lostCharKey(msg tea.KeyMsg) bool {
	if msg.Type != tea.KeyRunes || !msg.Alt || len(msg.Runes) == 0 {
		return false
	}
	for _, r := range msg.Runes {
		if r != 0 {
			return false
		}
	}
	return true
}

// keyRunes is what a text key types.
func keyRunes(msg tea.KeyMsg) []rune {
	if msg.Type == tea.KeySpace {
		return []rune{' '}
	}
	return msg.Runes
}

// pasteKey handles a key on the Windows path. It reports whether the key was
// dealt with; if not, Update handles it the ordinary way.
func (m *InputModel) pasteKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	now := time.Now()
	inPaste := m.lastKeyBurstable && !m.lastKeyTime.IsZero() && now.Sub(m.lastKeyTime) < pasteEnterWindow
	m.lastKeyTime = now
	afterCR := m.lastWasBurstCR
	m.lastWasBurstCR = false

	if m.paste.releasing {
		// A held key nothing followed: it acts now.
		m.lastKeyBurstable = false
		m.paste.prevInserted = nil
		return false, nil
	}

	if m.paste.held != nil {
		// Another key came while one was held, so the held key was the
		// first character of a paste and is text after all.
		held := *m.paste.held
		m.paste.held = nil
		m.beginBurst(false)
		if held.Type == tea.KeyEnter {
			m.bufferRunes('\n')
			afterCR = true
		} else {
			m.bufferRunes(keyRunes(held)...)
		}
		inPaste = true
	}

	if inPaste && m.bufferPasteKey(msg, afterCR) {
		m.lastKeyBurstable = true
		m.viewFresh = true
		return true, m.armPasteTick()
	}

	// Not part of a paste: whatever was collected goes in first, so this key
	// acts on the text as it now stands.
	m.flushPaste()
	m.endBurst()

	if m.holdsKey(msg) {
		m.paste.heldGen++
		gen := m.paste.heldGen
		held := msg
		m.paste.held = &held
		m.lastKeyBurstable = true
		return true, tea.Tick(pasteHoldDelay, func(time.Time) tea.Msg { return heldKeyMsg{gen: gen} })
	}
	if lostCharKey(msg) {
		// A paste can begin with an untypeable character.
		m.beginBurst(false)
		m.paste.burst.lost++
		m.lastKeyBurstable = true
		return true, m.armPasteTick()
	}

	m.lastKeyBurstable = textKey(msg)
	m.paste.prevInserted = nil
	if m.lastKeyBurstable {
		m.paste.prevInserted = append([]rune(nil), keyRunes(msg)...)
	}
	return false, nil
}

// bufferPasteKey collects one key of a paste. It reports false for a key a
// paste cannot produce, which ends the paste.
func (m *InputModel) bufferPasteKey(msg tea.KeyMsg, afterCR bool) bool {
	switch {
	case lostCharKey(msg):
		m.beginBurst(true)
		m.paste.burst.lost++
	case textKey(msg):
		m.beginBurst(true)
		m.bufferRunes(keyRunes(msg)...)
	case msg.Type == tea.KeyTab:
		// Pasted indentation, not completion.
		m.beginBurst(true)
		m.bufferRunes('\t')
	case msg.Type == tea.KeyEnter:
		m.beginBurst(true)
		m.bufferRunes('\n')
		m.lastWasBurstCR = true
	case msg.Type == tea.KeyCtrlJ:
		// The LF half of a CRLF arrives as Ctrl+J right after the CR half
		// became a newline: one line break per pasted line, not two.
		m.beginBurst(true)
		if !afterCR {
			m.bufferRunes('\n')
		}
	default:
		return false
	}
	return true
}

// holdsKey reports whether a key acts rather than types, so that it is held
// to see whether a paste follows: Enter submits, and on an empty composer a
// digit picks a report option.
func (m *InputModel) holdsKey(msg tea.KeyMsg) bool {
	if msg.Type == tea.KeyEnter {
		return true
	}
	if !textKey(msg) || len(msg.Runes) != 1 || msg.Runes[0] < '0' || msg.Runes[0] > '9' {
		return false
	}
	return m.cardShowing() && m.textarea.Value() == ""
}

// releaseHeld lets a held key act when nothing followed it.
func (m InputModel) releaseHeld(msg heldKeyMsg) (tea.Model, tea.Cmd) {
	if m.paste.held == nil || msg.gen != m.paste.heldGen {
		return m, nil
	}
	held := *m.paste.held
	m.paste.held = nil
	m.paste.releasing = true
	mod, cmd := m.Update(held)
	out := mod.(InputModel)
	out.paste.releasing = false
	return out, cmd
}

// pasteInFlight reports whether a paste has keys not yet in the textarea.
func (m *InputModel) pasteInFlight() bool {
	return len(m.paste.buf) > 0 || m.paste.held != nil
}

// beginBurst starts the record of a paste unless one is open. withPrev says
// the key handled the ordinary way just before this one belongs to the paste.
func (m *InputModel) beginBurst(withPrev bool) {
	if m.paste.burst != nil {
		return
	}
	b := &pasteBurst{start: m.cursorOffset()}
	if withPrev {
		b.text = append(b.text, m.paste.prevInserted...)
		b.start -= len(m.paste.prevInserted)
	}
	m.paste.burst = b
	m.paste.prevInserted = nil
}

// bufferRunes adds pasted characters to the buffer and the paste record.
func (m *InputModel) bufferRunes(rs ...rune) {
	m.paste.buf = append(m.paste.buf, rs...)
	if b := m.paste.burst; b != nil {
		b.text = append(b.text, rs...)
		for _, r := range rs {
			if r == '\n' {
				b.lines++
			}
		}
	}
}

// collectingLongPaste reports whether the paste in progress has grown past
// the long-paste limit, so that it will become a tag and its text no longer
// goes into the textarea.
func (m *InputModel) collectingLongPaste() bool {
	b := m.paste.burst
	if b == nil || m.onTextPaste == nil {
		return false
	}
	return b.lines+1 > longPasteLines || len(b.text) > longPasteChars
}

// armPasteTick schedules the next flush unless one is already scheduled.
func (m *InputModel) armPasteTick() tea.Cmd {
	if m.paste.tickArmed {
		return nil
	}
	m.paste.tickArmed = true
	return tea.Tick(pasteFlushInterval, func(time.Time) tea.Msg { return pasteTickMsg{} })
}

// onPasteTick inserts what the paste has brought so far. While keys are still
// arriving it schedules another tick; once they have stopped for
// pasteEnterWindow the paste is over.
func (m *InputModel) onPasteTick() tea.Cmd {
	m.paste.tickArmed = false
	m.flushPaste()
	if m.paste.burst == nil {
		return nil
	}
	if time.Since(m.lastKeyTime) < pasteEnterWindow {
		return m.armPasteTick()
	}
	m.endBurst()
	return nil
}

// flushPaste inserts the collected paste text at the cursor in one go, unless
// the paste is long enough to become a tag, which endBurst makes.
func (m *InputModel) flushPaste() {
	if m.collectingLongPaste() {
		return
	}
	m.insertBuffered()
}

// insertBuffered inserts the collected paste text at the cursor.
func (m *InputModel) insertBuffered() {
	if len(m.paste.buf) == 0 {
		return
	}
	text := string(m.paste.buf)
	m.paste.buf = m.paste.buf[:0]
	m.clearCompletion()
	m.textarea.InsertString(text)
	m.historyIdx = -1
	m.afterBulkEdit()
}

// afterBulkEdit settles the composer after text went in without passing
// through textarea.Update: height, the textarea's own scroll position (which
// only Update moves, so an empty message is sent through it), the tracked
// viewport, and live @path completion.
func (m *InputModel) afterBulkEdit() {
	m.adjustHeight()
	m.textarea, _ = m.textarea.Update(nil)
	m.ensureCursorVisible()
	m.refreshAutoCompletion()
}

// endBurst closes the paste record. If the paste lost characters to
// Alt+numpad sequences and the clipboard holds the same text with them, the
// clipboard's copy is what was pasted. A long paste then becomes a tag in
// place of whatever part of it went into the textarea; anything shorter goes
// in as text.
func (m *InputModel) endBurst() {
	b := m.paste.burst
	if b == nil {
		return
	}
	text := b.text
	if b.lost >= 2 && m.onClipboardText != nil {
		if clip, err := m.onClipboardText(); err == nil {
			if want, ok := recoverPastedText(b.text, clip, b.lost); ok {
				text = want
			}
		}
	}
	if m.onTextPaste != nil && isLongPaste(string(text)) {
		if start, end, ok := m.burstRegion(b); ok {
			path, err := m.onTextPaste(string(text))
			if err == nil {
				m.paste.burst = nil
				m.paste.buf = m.paste.buf[:0]
				m.replaceRange(start, end, "")
				m.addPastedText(path, string(text))
				return
			}
			m.pasteNotice = "paste: " + err.Error() + "; pasted as text instead"
		}
	}
	m.paste.burst = nil
	m.insertBuffered()
	if string(text) != string(b.text) {
		if start, end, ok := m.burstRegion(b); ok {
			m.replaceRange(start, end, string(text))
		}
	}
}

// burstRegion finds what a paste has put into the textarea so far: the runes
// from b.start to the cursor, which must be the paste's text up to what is
// still buffered, as the textarea stores it. ok is false when the text there
// is something else, and then nothing is replaced.
func (m *InputModel) burstRegion(b *pasteBurst) (start, end int, ok bool) {
	flushed := len(b.text) - len(m.paste.buf)
	if flushed < 0 {
		return 0, 0, false
	}
	typed := runeutil.NewSanitizer().Sanitize(append([]rune(nil), b.text[:flushed]...))
	end = m.cursorOffset()
	runes := []rune(m.textarea.Value())
	if b.start < 0 || end > len(runes) || end-b.start != len(typed) || string(runes[b.start:end]) != string(typed) {
		return 0, 0, false
	}
	return b.start, end, true
}

// recoverPastedText matches what a paste typed against the clipboard text. The
// typed text must be the clipboard text with some characters missing, and each
// missing character must be accounted for by the key events with no character:
// an Alt+numpad sequence is at least two of them, the Alt press and a digit.
// The console only sends characters outside ASCII that way, so an ASCII
// character, a line break or a tab is never missing. The paste must also have
// typed something itself: a few quick Alt+numpad digits typed by hand would
// otherwise pass for a paste of whatever short text the clipboard holds. It
// returns the clipboard text with its line breaks made the composer's.
func recoverPastedText(typed []rune, clip string, lost int) ([]rune, bool) {
	if len(typed) == 0 {
		return nil, false
	}
	clip = strings.ReplaceAll(clip, "\r\n", "\n")
	clip = strings.ReplaceAll(clip, "\r", "\n")
	want := make([]rune, 0, len(clip))
	for _, r := range clip {
		if r == '\n' || r == '\t' || !unicode.IsControl(r) {
			want = append(want, r)
		}
	}
	i, missing := 0, 0
	for _, r := range want {
		if i < len(typed) && r == typed[i] {
			i++
			continue
		}
		if r < utf8.RuneSelf {
			return nil, false
		}
		missing++
	}
	if i != len(typed) || missing == 0 || 2*missing > lost {
		return nil, false
	}
	return want, true
}

// cursorOffset returns the cursor's position as a rune offset into the
// composer text.
func (m *InputModel) cursorOffset() int {
	lines := m.hardLines()
	row := m.textarea.Line()
	off := 0
	for i := 0; i < row && i < len(lines); i++ {
		off += len([]rune(lines[i])) + 1
	}
	return off + m.cursorHardCol()
}

// moveCursorToOffset puts the cursor at a rune offset into the composer text.
// SetValue leaves the cursor at the end of the text, so this walks back up to
// the target line the way replaceToken does.
func (m *InputModel) moveCursorToOffset(off int) {
	lines := m.hardLines()
	row, col := 0, off
	for row < len(lines)-1 && col > len([]rune(lines[row])) {
		col -= len([]rune(lines[row])) + 1
		row++
	}
	m.textarea.CursorStart()
	for i := 0; i < 1_000_000 && m.textarea.Line() > row; i++ {
		m.textarea.CursorUp()
	}
	m.textarea.SetCursor(col)
}

package ui

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path/filepath"

	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/kvit-s/kvit-coder/internal/report"
	rw "github.com/mattn/go-runewidth"
	"github.com/rivo/uniseg"
	"os"
	"strings"
	"time"
	"unicode"
)

// InputModel wraps the textarea component for multi-line input
type InputModel struct {
	textarea      textarea.Model
	history       []string
	historyIdx    int
	submitted     bool
	cancelled     bool
	value         string
	prompt        string
	width         int
	maxHeight     int
	quitting      bool
	lastValueLen  int // Track previous value length to detect paste
	viewportStart int // Track which line is at the top of the viewport
	// onImagePaste stages a clipboard image and returns its staged path.
	// Nil when the host offers no image paste (set by the driver; the model
	// itself never touches the clipboard).
	onImagePaste func() (string, error)
	// pastedImages are the images staged this session, in label order:
	// pastedImages[0] is [image1], pastedImages[1] is [image2], and so on.
	// The first imageBase entries were staged before the composer opened
	// (:image, :paste); the rest came from the paste key below.
	pastedImages []string
	imageBase    int
	// pasteNotice is the one-line outcome of the last failed paste attempt,
	// shown under the input so a failed paste is visible, not silent.
	// Success needs no notice: the new token in the text plus the image
	// list below already say what happened.
	pasteNotice string
	// completionBase is the directory relative path completion resolves
	// against (the workspace root, set by the driver; empty falls back
	// to the working directory).
	completionBase string
	// compCandidates is the active completion list: full replacements
	// for the path part of the current token, directories with a
	// trailing slash, sorted dirs-first. Nil when no completion is
	// showing. The list appears automatically while typing @ plus two
	// characters, or on Tab for shorter/noisy tokens; Tab and up/down
	// then select from it.
	compCandidates []string
	// compTotal is the uncapped match count, for the "…and N more" line.
	compTotal int
	// compIndex selects compCandidates[compIndex] once Tab or up/down has
	// cycled one in; -1 while the list only shows.
	compIndex int
	// compRow and compStart locate the token the list was built for:
	// hard-line index plus rune offset of the token start.
	compRow, compStart int
	// compToken is the full token text the list was built for. Typing
	// narrows it (the list refreshes from the filesystem); a Tab with
	// unchanged text cycles instead of re-listing.
	compToken string
	// compNotice is a one-line dimmed note under the input ("no match").
	// Only an explicit Tab sets it; live typing stays quiet when nothing
	// matches so the composer does not flicker while narrowing.
	compNotice string
	// wakePoll reports how many inbox files are waiting for the next turn.
	// It is set by the driver (kvit-coder-ui), which polls the session's
	// proc registry into the inbox directory and counts what is there. Nil
	// disables wake: the composer never fires on its own.
	wakePoll func() (pending int, summary string)
	// wakePending is the last polled count, shown as a badge while the user
	// types so nothing arrives silently.
	wakePending int
	// wakeFired reports that this composer submitted itself: the inbox was
	// non-empty while the text was still empty, so the driver should run an
	// inbox-only turn rather than treat the empty submit as a no-op.
	wakeFired bool

	// card is the report the last turn ended with, drawn above the input.
	// Nil when the conversation's last message was not a report. See
	// report_card.go and docs/structured-reports.md.
	card *report.Report
	// cardDismissed is set by esc. It hides the card and lifts the hold it
	// puts on an inbox-only turn, and lasts only as long as this composer:
	// the next one reads the same report again, and :report names that.
	cardDismissed bool
	// termCols is the terminal's width as the last resize reported it, which
	// is what the report card wraps to. m.width is the textarea's width and
	// has a margin taken off it already, so it is too narrow for the card.
	termCols int
	// cardExpanded holds the ids of blocks showing their details.
	cardExpanded map[string]bool
	// cardAnswer is what the user picked, for the driver to act on.
	cardAnswer *CardAnswer
}

// SetImagePasteHandler installs the clipboard-image stager behind the paste
// key. Passing nil disables image paste.
func (m *InputModel) SetImagePasteHandler(fn func() (string, error)) {
	m.onImagePaste = fn
}

// SetStagedImages seeds the composer with images staged before it opened
// (:image, :paste). They take the first labels, so a paste below continues
// the numbering instead of restarting at [image1].
func (m *InputModel) SetStagedImages(paths []string) {
	m.pastedImages = append([]string(nil), paths...)
	m.imageBase = len(m.pastedImages)
}

// SetCompletionBaseDir tells path completion which directory relative
// paths resolve against (the workspace root). Empty falls back to the
// working directory at completion time.
func (m *InputModel) SetCompletionBaseDir(dir string) {
	m.completionBase = dir
}

// clearCompletion drops the completion list and notice. Tab, Shift+Tab
// and the completion arrow keys keep the list alive (cycling); typing
// refreshes it via refreshAutoCompletion instead, so a list never
// outlives the token it was built for.
func (m *InputModel) clearCompletion() {
	m.compCandidates = nil
	m.compTotal = 0
	m.compIndex = -1
	m.compToken = ""
	m.compNotice = ""
}

// completionListActive reports whether a candidate list is showing.
// The "no match" notice alone is not a list: arrows have nothing to
// select there.
func (m *InputModel) completionListActive() bool {
	return len(m.compCandidates) > 0
}

// PastedImages returns the images staged by the paste key plus any seeded
// with SetStagedImages, in label order.
func (m InputModel) PastedImages() []string {
	return append([]string(nil), m.pastedImages...)
}

// ImageLabel returns the in-text label for the nth staged image (1-based),
// e.g. "[image1]".
func ImageLabel(n int) string {
	return "[" + "image" + itoa(n) + "]"
}

// pasteImage stages one clipboard image behind the paste key and inserts its
// [imageN] label at the cursor, so the text says what the list below maps:
// "look at [image1] and compare with [image2]". A failure leaves a one-line
// notice; a paste that fails silently reads as a broken key, which is the
// report that started this.
func (m *InputModel) pasteImage() {
	if m.onImagePaste == nil {
		return
	}
	path, err := m.onImagePaste()
	if err != nil {
		m.pasteNotice = "paste: " + err.Error()
		return
	}
	m.pastedImages = append(m.pastedImages, path)
	m.pasteNotice = ""
	m.insertImageToken(len(m.pastedImages))
}

// insertImageToken inserts the [imageN] label at the cursor, adding spaces
// where needed so "look at[image1]and" becomes "look at [image1] and".
// The cursor ends up after the token's trailing space, ready to keep typing.
func (m *InputModel) insertImageToken(n int) {
	token := ImageLabel(n)
	v := m.textarea.Value()
	row := m.textarea.Line()
	col := m.textarea.LineInfo().CharOffset
	lines := strings.Split(v, "\n")
	var before, after rune
	if row >= 0 && row < len(lines) {
		runes := []rune(lines[row])
		if col-1 >= 0 && col-1 < len(runes) {
			before = runes[col-1]
		}
		if col >= 0 && col < len(runes) {
			after = runes[col]
		}
	}
	prefix := ""
	if before != 0 && before != ' ' && before != '\t' && before != '\n' {
		prefix = " "
	}
	suffix := " "
	if after == ' ' || after == '\t' || after == '\n' {
		suffix = ""
	}
	m.textarea.InsertString(prefix + token + suffix)
	m.adjustHeight()
}

// cursorHardCol returns the cursor's rune offset within its hard line.
// LineInfo reports the position inside the soft-wrapped grid, so the
// absolute column is the segment start plus the in-segment offset.
func (m *InputModel) cursorHardCol() int {
	li := m.textarea.LineInfo()
	col := li.StartColumn + li.ColumnOffset
	if col < 0 {
		return 0
	}
	return col
}

// hardLines splits the composer text into hard lines.
func (m *InputModel) hardLines() []string {
	if v := m.textarea.Value(); v != "" {
		return strings.Split(v, "\n")
	}
	return []string{""}
}

// replaceToken swaps the token at [tokenStart, cursorCol) on the given
// hard line for newToken, leaving every other line and the text after
// the cursor untouched, and parks the cursor right after the insertion.
func (m *InputModel) replaceToken(row, tokenStart, cursorCol int, newToken string) {
	lines := m.hardLines()
	if row < 0 || row >= len(lines) {
		return
	}
	runes := []rune(lines[row])
	if tokenStart < 0 {
		tokenStart = 0
	}
	if tokenStart > len(runes) {
		tokenStart = len(runes)
	}
	if cursorCol < tokenStart {
		cursorCol = tokenStart
	}
	if cursorCol > len(runes) {
		cursorCol = len(runes)
	}
	combined := make([]rune, 0, len(runes)+len([]rune(newToken))-(cursorCol-tokenStart))
	combined = append(combined, runes[:tokenStart]...)
	combined = append(combined, []rune(newToken)...)
	combined = append(combined, runes[cursorCol:]...)
	lines[row] = string(combined)
	m.textarea.SetValue(strings.Join(lines, "\n"))
	// SetValue leaves the cursor at the end of the text, so walk back up
	// to the edited row (bounded, like history navigation) and set the
	// column exactly: SetCursor clamps, so drift on the way is harmless.
	target := tokenStart + len([]rune(newToken))
	m.textarea.CursorStart()
	for i := 0; i < 10000 && m.textarea.Line() > row; i++ {
		m.textarea.CursorUp()
	}
	m.textarea.SetCursor(target)
}

// ensureCursorVisible keeps the tracked viewport on the cursor's visual
// row after an edit that bypasses textarea.Update (Tab completion,
// image paste).
func (m *InputModel) ensureCursorVisible() {
	currentVisual := m.cursorSoftRow()
	visibleHeight := m.textarea.Height()
	totalVisual := m.totalSoftLines()
	if currentVisual < m.viewportStart {
		m.viewportStart = currentVisual
	} else if currentVisual >= m.viewportStart+visibleHeight {
		m.viewportStart = currentVisual - visibleHeight + 1
	}
	if m.viewportStart < 0 {
		m.viewportStart = 0
	}
	maxViewportStart := totalVisual - visibleHeight
	if maxViewportStart < 0 {
		maxViewportStart = 0
	}
	if m.viewportStart > maxViewportStart {
		m.viewportStart = maxViewportStart
	}
}

// handleTab completes the @path in the token left of the cursor. A live
// list (from live typing or an earlier Tab) with untouched text cycles
// forward through the list (Shift+Tab backwards, down/up likewise).
// Otherwise one match replaces it in place; several list under the
// input, extending the token only when every candidate shares the typed
// text as a prefix (a fuzzy abbreviation like "@sn-p" is left alone for
// Tab to cycle). Tokens without @ never complete: an empty token
// indents, anything else is left alone.
func (m *InputModel) handleTab(forward bool) {
	row, tokenStart, col, token, inQuotes := m.completionTarget()

	// A live list for this same token position with the text untouched
	// cycles instead of re-reading the directory.
	if m.compCandidates != nil && row == m.compRow && tokenStart == m.compStart && token == m.compToken {
		m.cycleCompletion(forward, row, tokenStart, col, token, inQuotes)
		return
	}

	if token == "" {
		// No path under the cursor: indent for code in the prompt (the
		// textarea sanitizer turns the tab into spaces).
		m.clearCompletion()
		m.textarea.InsertString("\t")
		m.historyIdx = -1
		m.adjustHeight()
		m.ensureCursorVisible()
		return
	}

	if _, at, _, _ := splitCompletionToken(token); at != "@" {
		// Free text, not a file reference: leave it alone rather
		// than completing or indenting mid-word.
		m.clearCompletion()
		return
	}

	m.completeFresh(row, tokenStart, col, token, inQuotes)
}

// completionTarget locates the token under the cursor: its hard-line
// row, rune offset of the token start, rune cursor column, token text,
// and whether the cursor sits inside quotes.
func (m *InputModel) completionTarget() (row, tokenStart, col int, token string, inQuotes bool) {
	lines := m.hardLines()
	row = m.textarea.Line()
	if row < 0 {
		row = 0
	}
	if row >= len(lines) {
		row = len(lines) - 1
	}
	lineRunes := []rune(lines[row])
	col = m.cursorHardCol()
	if col > len(lineRunes) {
		col = len(lineRunes)
	}
	tokenStart, inQuotes = tokenStartForLine(lineRunes, col)
	token = string(lineRunes[tokenStart:col])

	if token == "" && m.compCandidates != nil && row == m.compRow && m.compToken != "" &&
		strings.HasSuffix(string(lineRunes[:col]), m.compToken) {
		// Sitting right after a quoted insertion ("my dir/"): the
		// closing quote hides the token, so re-anchor onto it and
		// carry on cycling instead of indenting.
		token = m.compToken
		tokenStart = col - len([]rune(m.compToken))
	}
	return row, tokenStart, col, token, inQuotes
}

// tryCycleCompletion moves the selection one step through the live list
// (down/Tab forward, up/Shift+Tab back) and reports whether it did.
// It fails when no list is showing or the text moved off the token the
// list was built for; the caller then falls through to the key's normal
// job instead.
func (m *InputModel) tryCycleCompletion(forward bool) bool {
	if !m.completionListActive() {
		return false
	}
	row, tokenStart, col, token, inQuotes := m.completionTarget()
	if m.compCandidates == nil || row != m.compRow || tokenStart != m.compStart || token != m.compToken {
		return false
	}
	m.cycleCompletion(forward, row, tokenStart, col, token, inQuotes)
	return true
}

// completeFresh lists the filesystem for a new @ token and either
// replaces it (a single match, or the common prefix of several) or just
// shows the list when there is nothing to extend. The common prefix
// only extends when it keeps the typed core as a prefix: fuzzy matches
// share a score, not a spelling, so rewriting "@sn-p" to the
// candidates' longest common path would discard the abbreviation the
// list was built for.
func (m *InputModel) completeFresh(row, tokenStart, col int, token string, inQuotes bool) {
	lead, at, core, trailing := splitCompletionToken(token)
	if at != "@" {
		m.clearCompletion()
		return
	}
	candidates, total := listPathCompletions(m.completionBase, core)
	switch {
	case len(candidates) == 0:
		m.compCandidates = nil
		m.compTotal = 0
		m.compIndex = -1
		m.compToken = ""
		m.compNotice = "no match"
	case len(candidates) == 1:
		m.clearCompletion()
		m.replaceToken(row, tokenStart, col, buildCompletionToken(lead, at, candidates[0], trailing, inQuotes))
		m.historyIdx = -1
		m.adjustHeight()
		m.ensureCursorVisible()
	default:
		m.historyIdx = -1
		if lcp := commonPathPrefix(candidates); len(lcp) > len(core) && strings.HasPrefix(lcp, core) {
			newToken := buildCompletionToken(lead, at, lcp, trailing, inQuotes)
			m.replaceToken(row, tokenStart, col, newToken)
			m.compToken = newToken
		} else {
			m.compToken = token
		}
		m.compCandidates = candidates
		m.compTotal = total
		m.compIndex = -1
		m.compRow = row
		m.compStart = tokenStart
		m.compNotice = ""
		m.adjustHeight()
		m.ensureCursorVisible()
	}
}

// refreshAutoCompletion lists the filesystem for the @ token under the
// cursor after typing, without rewriting anything. It is the live-typing
// counterpart to completeFresh: Tab still owns replacing (single match,
// common-prefix extension, cycling), while typing only shows or narrows
// the list. Tokens without @, or with fewer than minAutoCompletionRunes
// in the core, clear the list; zero matches clear quietly (no "no match"
// flicker while narrowing). An unchanged token keeps its list, so a
// cursor move without an edit does not pay for another walk.
func (m *InputModel) refreshAutoCompletion() {
	row, tokenStart, _, token, _ := m.completionTarget()
	_, at, core, _ := splitCompletionToken(token)
	if at != "@" || len([]rune(core)) < minAutoCompletionRunes {
		m.clearCompletion()
		return
	}
	if m.compCandidates != nil && row == m.compRow && tokenStart == m.compStart && token == m.compToken {
		return
	}
	candidates, total := listPathCompletions(m.completionBase, core)
	if len(candidates) == 0 {
		m.clearCompletion()
		return
	}
	m.compCandidates = candidates
	m.compTotal = total
	m.compIndex = -1
	m.compRow = row
	m.compStart = tokenStart
	m.compToken = token
	m.compNotice = ""
	m.adjustHeight()
	m.ensureCursorVisible()
}

// cycleCompletion swaps the current token for the next (or previous)
// candidate in the live list, wrapping around.
func (m *InputModel) cycleCompletion(forward bool, row, tokenStart, col int, token string, inQuotes bool) {
	n := len(m.compCandidates)
	if n == 0 {
		m.clearCompletion()
		return
	}
	idx := m.compIndex
	if forward {
		if idx < 0 {
			idx = 0
		} else {
			idx = (idx + 1) % n
		}
	} else {
		if idx < 0 {
			idx = n - 1
		} else {
			idx = (idx - 1 + n) % n
		}
	}
	lead, at, _, trailing := splitCompletionToken(token)
	newToken := buildCompletionToken(lead, at, m.compCandidates[idx], trailing, inQuotes)
	m.replaceToken(row, tokenStart, col, newToken)
	m.compIndex = idx
	m.compToken = newToken
	m.historyIdx = -1
	m.adjustHeight()
	m.ensureCursorVisible()
}

// adjustHeight adjusts the textarea height to fit content, up to maxHeight.
// It counts soft-wrapped visual rows, not just hard lines, so a long single
// line without newlines still grows the input instead of scrolling the start
// of the text out of a 1-row viewport.
func (m *InputModel) adjustHeight() {
	total := m.totalSoftLines()
	newHeight := total
	if newHeight > m.maxHeight {
		newHeight = m.maxHeight
	}
	if newHeight < 1 {
		newHeight = 1
	}
	m.textarea.SetHeight(newHeight)
}

// inputWidth returns the inner width the textarea wraps at.
func (m *InputModel) inputWidth() int {
	if w := m.textarea.Width(); w > 0 {
		return w
	}
	if m.width > 0 {
		return m.width
	}
	return 80
}

// totalSoftLines returns the number of visual rows the current value occupies
// once soft-wrapped at the input width.
func (m *InputModel) totalSoftLines() int {
	return softWrappedHeight(m.textarea.Value(), m.inputWidth())
}

// cursorSoftRow returns the 0-based visual row the cursor is on, accounting
// for soft-wrapped rows in all hard lines above it.
func (m *InputModel) cursorSoftRow() int {
	width := m.inputWidth()
	hardRow := m.textarea.Line()
	rowsBefore := 0
	if v := m.textarea.Value(); v != "" {
		lines := strings.Split(v, "\n")
		for i := 0; i < hardRow && i < len(lines); i++ {
			rowsBefore += wrapLineCount([]rune(lines[i]), width)
		}
	}
	return rowsBefore + m.textarea.LineInfo().RowOffset
}

// softWrappedHeight counts visual rows for a value at the given width.
func softWrappedHeight(value string, width int) int {
	if width <= 0 {
		width = 80
	}
	if value == "" {
		return 1
	}
	total := 0
	for _, line := range strings.Split(value, "\n") {
		total += wrapLineCount([]rune(line), width)
	}
	if total < 1 {
		total = 1
	}
	return total
}

// wrapLineCount mirrors the word-wrap used by bubbles/textarea: it reports how
// many visual rows one hard line occupies at the given width.
func wrapLineCount(runes []rune, width int) int {
	return len(wrapRunes(runes, width))
}

// wrapRunes is a copy of the wrapping algorithm in bubbles/textarea, kept here
// so height and viewport accounting match exactly what the component renders.
func wrapRunes(runes []rune, width int) [][]rune {
	var (
		lines  = [][]rune{{}}
		word   = []rune{}
		row    int
		spaces int
	)

	// Word wrap the runes
	for _, r := range runes {
		if unicode.IsSpace(r) {
			spaces++
		} else {
			word = append(word, r)
		}

		if spaces > 0 { //nolint:nestif
			if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces > width {
				row++
				lines = append(lines, []rune{})
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			} else {
				lines[row] = append(lines[row], word...)
				lines[row] = append(lines[row], repeatSpaces(spaces)...)
				spaces = 0
				word = nil
			}
		} else {
			// If the last character is a double-width rune, then we may not be able to add it to this line
			// as it might cause us to go past the width.
			lastCharLen := rw.RuneWidth(word[len(word)-1])
			if uniseg.StringWidth(string(word))+lastCharLen > width {
				// If the current line has any content, let's move to the next
				// line because the current word fills up the entire line.
				if len(lines[row]) > 0 {
					row++
					lines = append(lines, []rune{})
				}
				lines[row] = append(lines[row], word...)
				word = nil
			}
		}
	}

	if uniseg.StringWidth(string(lines[row]))+uniseg.StringWidth(string(word))+spaces >= width {
		lines = append(lines, []rune{})
		lines[row+1] = append(lines[row+1], word...)
		// We add an extra space at the end of the line to account for the
		// trailing space at the end of the previous soft-wrapped lines so that
		// behaviour when navigating is consistent and so that we don't need to
		// continually add edges to handle the last line of the wrapped input.
		spaces++
		lines[row+1] = append(lines[row+1], repeatSpaces(spaces)...)
	} else {
		lines[row] = append(lines[row], word...)
		spaces++
		lines[row] = append(lines[row], repeatSpaces(spaces)...)
	}

	return lines
}

func repeatSpaces(n int) []rune {
	return []rune(strings.Repeat(string(' '), n))
}

// NewInputModel creates a new input model with the given prompt
func NewInputModel(prompt string, history []string) InputModel {
	ta := textarea.New()
	ta.Prompt = "" // We'll show the prompt separately
	ta.Placeholder = "(Enter to submit, Ctrl+J newline, @path completes live, Tab select, Alt+V paste image)"
	ta.ShowLineNumbers = false
	ta.CharLimit = 0 // No limit

	// Start with 1 row, will grow automatically
	ta.SetHeight(1)
	ta.SetWidth(80) // Default width

	// Configure styling - remove cursor line highlighting
	ta.FocusedStyle.CursorLine = lipgloss.NewStyle()
	ta.BlurredStyle.CursorLine = lipgloss.NewStyle()
	ta.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	ta.FocusedStyle.Text = lipgloss.NewStyle()

	// Disable the default Enter → newline binding
	// We'll handle Enter for submission and Ctrl+J for newlines
	ta.KeyMap.InsertNewline.SetEnabled(false)

	ta.Focus()

	return InputModel{
		textarea:     ta,
		history:      history,
		historyIdx:   -1,
		submitted:    false,
		cancelled:    false,
		prompt:       prompt,
		width:        80,
		maxHeight:    20, // Max height before scrolling
		lastValueLen: 0,
	}
}

// wakeTickInterval is how often an idle composer re-checks the inbox while
// it is open. One second keeps a wake within a tick of the arrival without
// any file watcher.
const wakeTickInterval = time.Second

// wakeTickMsg is the BubbleTea tick that re-checks the inbox.
type wakeTickMsg struct{}

// wakeTickCmd schedules the next inbox re-check.
func wakeTickCmd() tea.Cmd {
	return tea.Tick(wakeTickInterval, func(time.Time) tea.Msg { return wakeTickMsg{} })
}

// SetWakePoll installs the inbox waiter behind the composer. A nil poll
// disables wake.
func (m *InputModel) SetWakePoll(fn func() (int, string)) {
	m.wakePoll = fn
}

// WakeFired reports whether the composer submitted itself on inbox activity.
func (m InputModel) WakeFired() bool {
	return m.wakeFired
}

// Init initializes the input model
func (m InputModel) Init() tea.Cmd {
	if m.wakePoll == nil {
		return textarea.Blink
	}
	return tea.Batch(textarea.Blink, wakeTickCmd())
}

// Update handles input events
func (m InputModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case wakeTickMsg:
		// No waiter installed (a test driving the model, say): stay quiet.
		if m.wakePoll == nil {
			return m, nil
		}
		pending, _ := m.wakePoll()
		m.wakePending = pending
		// Fire only while the text is still empty: the user is idling at
		// the prompt, not composing. Anything typed means they will send
		// soon, and the turn they send picks the inbox up at its first
		// iteration — so show a badge instead and keep waiting.
		if pending > 0 && strings.TrimSpace(m.textarea.Value()) == "" && !m.cardHolds() {
			m.value = ""
			m.submitted = true
			m.wakeFired = true
			m.quitting = true
			return m, tea.Quit
		}
		return m, wakeTickCmd()
	case tea.WindowSizeMsg:
		m.termCols = msg.Width
		// Adjust width based on terminal size
		m.width = msg.Width - 10 // Leave some margin
		if m.width < 40 {
			m.width = 40
		}
		m.textarea.SetWidth(m.width)

		// Adjust max height based on terminal size (leave room for prompt and other UI)
		m.maxHeight = msg.Height - 5
		if m.maxHeight < 5 {
			m.maxHeight = 5
		}
		// Re-wrap at the new width: a narrower terminal can turn one hard
		// line into several visual rows.
		m.adjustHeight()

	case tea.KeyMsg:
		// A completion list belongs to the token it was built for. Tab
		// and Shift+Tab cycle it, up/down select from it while it
		// shows, and esc dismisses it; typing narrows it via
		// refreshAutoCompletion below, and anything else (cursor moves,
		// submit) discards it before the key is handled.
		switch key := msg.String(); {
		case key == "tab" || key == "shift+tab":
			// Kept alive for cycling.
		case (key == "up" || key == "down") && m.completionListActive():
			// Kept alive for arrow selection below.
		case key == "esc" && (m.completionListActive() || m.compNotice != ""):
			// Kept so the esc case can dismiss just the
			// completion UI instead of the whole input.
		default:
			m.clearCompletion()
		}
		if handled, cmd := m.handleCardKey(msg); handled {
			return m, cmd
		}

		switch msg.String() {
		// Submit on Enter. On an empty line with no staged images, Enter
		// picks the report's primary recommendation instead of submitting
		// empty, so the highlighted action needs no digit.
		case "enter":
			if strings.TrimSpace(m.textarea.Value()) == "" && len(m.pastedImages) == 0 {
				if handled, cmd := m.pickRecommended(); handled {
					return m, cmd
				}
			}
			m.value = m.textarea.Value()
			m.submitted = true
			m.quitting = true
			return m, tea.Quit

		// Insert newline on Ctrl+J
		case "ctrl+j":
			m.textarea.InsertString("\n")
			m.adjustHeight()
			return m, nil

		// Smart history navigation. While a completion list shows, up
		// selects the previous candidate instead (down the next one):
		// the list owns the arrows until it is dismissed.
		case "up":
			if m.tryCycleCompletion(false) {
				return m, nil
			}
			m.clearCompletion()
			if len(m.history) > 0 {
				// Navigate history if:
				// 1. Textarea is empty OR
				// 2. We're in history mode (historyIdx >= 0) and on the first
				//    visual (soft-wrapped) row, so up inside a long wrapped
				//    line still moves the cursor instead of leaving history.
				isEmpty := m.textarea.Value() == ""
				inHistoryMode := m.historyIdx >= 0
				onFirstVisual := m.cursorSoftRow() == 0

				if isEmpty || (inHistoryMode && onFirstVisual) {
					// Navigate to older history
					if m.historyIdx < len(m.history)-1 {
						m.historyIdx++
						m.textarea.SetValue(m.history[len(m.history)-1-m.historyIdx])
						m.adjustHeight()
						// Move cursor to beginning: first move to start of current line
						m.textarea.CursorStart()
						// Then move up to line 0
						for m.textarea.Line() > 0 {
							m.textarea.CursorUp()
						}
						// Finally ensure we're at column 0
						m.textarea.CursorStart()
						m.viewportStart = 0
					}
					return m, nil
				}
			}
			// Otherwise, let textarea handle it for line navigation

		case "down":
			if m.tryCycleCompletion(true) {
				return m, nil
			}
			m.clearCompletion()
			if len(m.history) > 0 {
				// Navigate history if:
				// 1. Textarea is empty OR
				// 2. We're in history mode (historyIdx >= 0) and on the last
				//    visual (soft-wrapped) row.
				isEmpty := m.textarea.Value() == ""
				inHistoryMode := m.historyIdx >= 0
				onLastVisual := m.cursorSoftRow() == m.totalSoftLines()-1

				if isEmpty || (inHistoryMode && onLastVisual) {
					// Navigate to newer history
					if m.historyIdx > 0 {
						m.historyIdx--
						m.textarea.SetValue(m.history[len(m.history)-1-m.historyIdx])
						m.adjustHeight()
						// Move cursor to beginning: first move to start of current line
						m.textarea.CursorStart()
						// Then move up to line 0
						for m.textarea.Line() > 0 {
							m.textarea.CursorUp()
						}
						// Finally ensure we're at column 0
						m.textarea.CursorStart()
						m.viewportStart = 0
					} else if m.historyIdx == 0 {
						m.historyIdx = -1
						m.textarea.SetValue("")
						m.adjustHeight()
						m.viewportStart = 0
					}
					return m, nil
				}
			}
			// Otherwise, let textarea handle it for line navigation

		case "ctrl+c":
			m.cancelled = true
			m.quitting = true
			return m, tea.Quit

		// Paste a clipboard image as a staged attachment. Alt+V is the
		// escape hatch for terminals that intercept Ctrl+V with a text-only
		// paste (Windows Terminal, the VS Code terminal on WSL): Ctrl+V
		// never arrives as a keypress there, so it cannot be handled here.
		case "alt+v":
			if m.onImagePaste != nil {
				m.pasteImage()
				return m, nil
			}
			// No handler: let the textarea see the key.
		// Select from the live @path list (typing @ plus two characters
		// already shows it; a bare Tab lists from the filesystem when
		// nothing shows yet). Shift+Tab cycles the same list backwards;
		// while the list shows, down/up select from it (handled in their
		// cases above).
		case "tab":
			m.handleTab(true)
			return m, nil

		case "shift+tab":
			m.handleTab(false)
			return m, nil
		case "esc":
			// ESC with a completion list showing dismisses just the
			// list, so a stray keypress doesn't eat the draft. A
			// second ESC with nothing showing clears the input as
			// before (images pasted in this composer go with it;
			// images staged before it opened stay for next turn).
			if m.completionListActive() || m.compNotice != "" {
				m.clearCompletion()
				return m, nil
			}
			// ESC just clears the current input, doesn't exit. Images pasted
			// in this composer go with it; images staged before it opened
			// (:image, :paste) stay staged for the next turn.
			m.textarea.SetValue("")
			m.adjustHeight()
			m.viewportStart = 0
			m.pasteNotice = ""
			if len(m.pastedImages) > m.imageBase {
				m.pastedImages = append([]string(nil), m.pastedImages[:m.imageBase]...)
			}
			return m, nil
		}
	}

	// Track value length before update to detect paste and edits
	beforeValueLen := len(m.textarea.Value())
	beforeValue := m.textarea.Value()
	wasInHistoryMode := m.historyIdx >= 0

	// Pre-grow the viewport by one visual row before the textarea sees the
	// key: it lays out and scrolls to keep the cursor visible using the old
	// height, so the keystroke that wraps onto a new row would scroll the
	// first row out of view (seen as the line shifting left instead of
	// wrapping) and the shrink-to-fit below would leave that offset
	// stranded. A single keystroke adds at most one visual row; at the cap
	// the height is already correct and scrolling is genuinely needed.
	if total := m.totalSoftLines(); total+1 <= m.maxHeight && m.textarea.Height() < total+1 {
		m.textarea.SetHeight(total + 1)
	}

	m.textarea, cmd = m.textarea.Update(msg)

	// If user edited the content while in history mode, exit history mode
	if m.historyIdx >= 0 && m.textarea.Value() != beforeValue {
		// Check if it's an actual edit (not just navigation)
		if msg, ok := msg.(tea.KeyMsg); ok {
			// Only reset if it's a typing key (not arrow keys, page up/down, etc.)
			key := msg.String()
			if len(key) == 1 || key == "backspace" || key == "delete" || key == "ctrl+u" || key == "ctrl+k" {
				m.historyIdx = -1
			}
		}
	}

	// Always adjust height to fit content, up to maxHeight
	m.adjustHeight()

	// Detect paste: if value length increased significantly in one update (>1 char),
	// it's likely a paste operation (but not history navigation)
	afterValueLen := len(m.textarea.Value())
	isPaste := afterValueLen > beforeValueLen+1 && !wasInHistoryMode

	// If we detected a paste whose wrapped content spans multiple visual
	// rows, reset the viewport to show from the beginning and move cursor
	// to end (the tracking below then keeps the cursor visible).
	if isPaste && m.totalSoftLines() > 1 {
		currentValue := m.textarea.Value()
		m.textarea.SetValue(currentValue)
		m.textarea.CursorEnd()
		m.viewportStart = 0
	}

	// Update viewport tracking to keep cursor visible.
	// This mimics the textarea's internal viewport logic, but in visual
	// (soft-wrapped) rows so long single lines track correctly.
	m.ensureCursorVisible()

	// Live @path completion: @ plus two characters lists matches without
	// needing Tab. Only re-list when the text actually changed — cursor
	// moves alone keep the dismissal (esc included) until the next edit —
	// and never while cycling (those keys return early above).
	if m.textarea.Value() != beforeValue {
		m.refreshAutoCompletion()
	}

	return m, cmd
}

// View renders the input model
func (m InputModel) View() string {
	// If quitting, return empty string to clear the display
	if m.quitting {
		return ""
	}

	// Calculate scroll indicator based on tracked viewport position.
	// Use soft-wrapped visual rows so a long single line reports scrolling.
	totalVisual := softWrappedHeight(m.textarea.Value(), m.inputWidth())
	visibleHeight := m.textarea.Height()

	scrollInfo := ""
	if totalVisual > visibleHeight {
		// Calculate what's hidden above and below the viewport
		viewportEnd := m.viewportStart + visibleHeight
		if viewportEnd > totalVisual {
			viewportEnd = totalVisual
		}

		hiddenAbove := m.viewportStart
		hiddenBelow := totalVisual - viewportEnd

		if hiddenAbove > 0 || hiddenBelow > 0 {
			scrollInfo = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
				" [" + formatScrollInfo(hiddenAbove, hiddenBelow, totalVisual, visibleHeight) + "]")
		}
	}

	// Simplified view without borders for better performance
	// Just show prompt and textarea with scroll indicator
	out := m.cardView() + m.prompt + scrollInfo + "\n" + m.textarea.View()
	if m.pasteNotice != "" {
		out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(m.pasteNotice)
	}
	// Pending inbox while composing: say so, so nothing arrives silently.
	// The messages ride along when this is submitted; clearing the text
	// lets the next tick fire an inbox-only turn instead.
	if m.wakePending > 0 {
		out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
			"[inbox: "+itoa(m.wakePending)+" pending — submit to include]")
	}

	out += m.completionView()
	// The staged-image list maps the [imageN] labels in the text above to
	// their files, so "compare [image1] with [image2]" is unambiguous.
	for i, p := range m.pastedImages {
		out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
			fmt.Sprintf("[%s: %s]", "image"+itoa(i+1), p))
	}
	return out + m.cardFooter()
}

// completionView renders the completion list (or the "no match"
// notice) under the input. The selected candidate gets a marker and a
// brighter color; the rest stay dim. The list is capped at
// maxCompletionDisplay rows with an "…and N more" tail, so typing @
// plus two characters in a big workspace cannot flood the composer.
func (m InputModel) completionView() string {
	out := ""
	if m.compNotice != "" {
		out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(m.compNotice)
	}
	if len(m.compCandidates) == 0 {
		return out
	}
	shown := m.compCandidates
	remaining := m.compTotal - len(shown)
	if len(shown) > maxCompletionDisplay {
		remaining += len(shown) - maxCompletionDisplay
		shown = shown[:maxCompletionDisplay]
	}
	for i, c := range shown {
		if i == m.compIndex {
			out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("136")).Render("> "+c)
		} else {
			out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("  "+c)
		}
	}
	if remaining > 0 {
		out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
			"  …and "+itoa(remaining)+" more (keep typing to narrow)")
	}
	out += "\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(
		"  (↑↓/tab select, esc dismiss)")
	return out
}

// formatScrollInfo creates a compact scroll indicator
func formatScrollInfo(hiddenAbove, hiddenBelow, total, visible int) string {
	if hiddenAbove > 0 && hiddenBelow > 0 {
		return lipgloss.NewStyle().Render("↑" + itoa(hiddenAbove) + " ↓" + itoa(hiddenBelow))
	} else if hiddenAbove > 0 {
		return lipgloss.NewStyle().Render("↑" + itoa(hiddenAbove))
	} else if hiddenBelow > 0 {
		return lipgloss.NewStyle().Render("↓" + itoa(hiddenBelow))
	}
	return lipgloss.NewStyle().Render(itoa(visible) + "/" + itoa(total))
}

// itoa is a simple int to string converter
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	negative := i < 0
	if negative {
		i = -i
	}

	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if negative {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}

// Value returns the submitted value
func (m InputModel) Value() string {
	return m.value
}

// Submitted returns whether the input was submitted
func (m InputModel) Submitted() bool {
	return m.submitted
}

// Cancelled returns whether the input was cancelled
func (m InputModel) Cancelled() bool {
	return m.cancelled
}

// LoadHistory loads history from a file
func LoadHistory(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	// Use null byte as delimiter to preserve multi-line entries
	entries := strings.Split(string(data), "\x00")
	// Filter out empty entries
	var history []string
	for _, entry := range entries {
		if strings.TrimSpace(entry) != "" {
			history = append(history, entry)
		}
	}
	return history, nil
}

// SaveHistory saves history to a file
func SaveHistory(path string, history []string) error {
	// Limit history size to last 1000 entries
	const maxHistory = 1000
	if len(history) > maxHistory {
		history = history[len(history)-maxHistory:]
	}

	if dir := filepath.Dir(path); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}

	// Use null byte as delimiter to preserve multi-line entries
	return os.WriteFile(path, []byte(strings.Join(history, "\x00")), 0644)
}

// LegacyHistoryPath is the pre-per-directory input history file: a single
// global file in the user's home. It is kept as the seed for per-directory
// histories, not as the live path.
func LegacyHistoryPath(home string) string {
	return filepath.Join(home, ".kvit-coder-history")
}

// HistoryPathForWorkspace returns the input history file for one workspace
// directory. The path is under ~/.kvit-coder/history/ keyed by the sha256 of
// the absolute workspace path (same scheme as permission stores), so every
// directory gets its own arrow-up history and listing the directory shows
// every stored workspace. An empty workspaceDir resolves against the current
// working directory; when that also fails the legacy global path is returned
// so the caller still has somewhere to read and write.
func HistoryPathForWorkspace(home, workspaceDir string) string {
	abs := workspaceDir
	if abs == "" {
		var err error
		abs, err = os.Getwd()
		if err != nil {
			return LegacyHistoryPath(home)
		}
	} else if absResolved, err := filepath.Abs(abs); err == nil {
		abs = absResolved
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(home, ".kvit-coder", "history", hex.EncodeToString(sum[:6])+".history")
}

// LoadWorkspaceHistory loads the history for one workspace directory. When
// the per-directory file does not exist yet but the legacy global file does,
// the global entries seed the workspace so a first run in a new directory
// starts from the familiar history and then diverges on its own.
func LoadWorkspaceHistory(home, workspaceDir string) ([]string, string) {
	path := HistoryPathForWorkspace(home, workspaceDir)
	history, _ := LoadHistory(path)
	if len(history) > 0 {
		return history, path
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		if legacy, _ := LoadHistory(LegacyHistoryPath(home)); len(legacy) > 0 {
			return legacy, path
		}
	}
	return history, path
}

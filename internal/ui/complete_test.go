package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func tabKey() tea.KeyMsg        { return tea.KeyMsg{Type: tea.KeyTab} }
func shiftTabKey() tea.KeyMsg   { return tea.KeyMsg{Type: tea.KeyShiftTab} }
func runeKey(r rune) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}} }
func upKey() tea.KeyMsg         { return tea.KeyMsg{Type: tea.KeyUp} }
func downKey() tea.KeyMsg       { return tea.KeyMsg{Type: tea.KeyDown} }
func escKey() tea.KeyMsg        { return tea.KeyMsg{Type: tea.KeyEsc} }
func writeFile(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// pressTab drives one Tab (or Shift+Tab) through Update and returns the
// model as an InputModel.
func pressTab(t *testing.T, m InputModel, forward bool) InputModel {
	t.Helper()
	key := tabKey()
	if !forward {
		key = shiftTabKey()
	}
	mod, _ := m.Update(key)
	return mod.(InputModel)
}

// pressKey drives one arbitrary key through Update and returns the model
// as an InputModel.
func pressKey(t *testing.T, m InputModel, key tea.KeyMsg) InputModel {
	t.Helper()
	mod, _ := m.Update(key)
	return mod.(InputModel)
}

func TestTokenStartForLine(t *testing.T) {
	cases := []struct {
		line     string
		cursor   int
		want     int
		inQuotes bool
	}{
		{"read mai", 8, 5, false},
		{"read mai", 0, 0, false},
		{"mai", 3, 0, false},
		{`read "my dir/fi`, 15, 6, true}, // spaces count inside quotes
		{`a "b c" d`, 9, 8, false},       // balanced quotes: plain token
		{`"ab"`, 4, 4, false},            // after closing quote: empty token
		{"one\ntwo th", 6, 4, false},     // newline delimits like a space
		{"  spaced", 8, 2, false},
		{"it's fine", 4, 0, false},   // apostrophe is literal mid-word text
		{"don't stop", 10, 6, false}, // ...so the token after it completes
		{`'my dir/fi`, 10, 1, true},  // boundary quote still opens
		{`--file="my dir/fi`, 17, 8, true},
	}
	for _, c := range cases {
		line := []rune(c.line)
		cursor := c.cursor
		if cursor > len(line) {
			cursor = len(line)
		}
		got, q := tokenStartForLine(line, cursor)
		if got != c.want || q != c.inQuotes {
			t.Errorf("tokenStartForLine(%q, %d) = (%d, %v), want (%d, %v)",
				c.line, c.cursor, got, q, c.want, c.inQuotes)
		}
	}
}

func TestSplitCompletionToken(t *testing.T) {
	cases := []struct {
		token                    string
		lead, at, core, trailing string
	}{
		{"@src/ma", "", "@", "src/ma", ""},
		{"(@src/ma,", "(", "@", "src/ma", ","},
		{"src/ma", "", "", "src/ma", ""},
		{`"@a b/"`, "", "@", "a b/", ""},
		{"@", "", "@", "", ""},
		{"foo)", "", "", "foo", ")"},
		{"", "", "", "", ""},
	}
	for _, c := range cases {
		lead, at, core, trailing := splitCompletionToken(c.token)
		if lead != c.lead || at != c.at || core != c.core || trailing != c.trailing {
			t.Errorf("splitCompletionToken(%q) = (%q,%q,%q,%q), want (%q,%q,%q,%q)",
				c.token, lead, at, core, trailing, c.lead, c.at, c.core, c.trailing)
		}
	}
}

func TestSplitDirPrefix(t *testing.T) {
	if d, f := splitDirPrefix("src/ma"); d != "src/" || f != "ma" {
		t.Errorf("splitDirPrefix(src/ma) = (%q,%q)", d, f)
	}
	if d, f := splitDirPrefix("ma"); d != "" || f != "ma" {
		t.Errorf("splitDirPrefix(ma) = (%q,%q)", d, f)
	}
	if d, f := splitDirPrefix("a/b/"); d != "a/b/" || f != "" {
		t.Errorf("splitDirPrefix(a/b/) = (%q,%q)", d, f)
	}
}

func TestCommonPathPrefix(t *testing.T) {
	if got := commonPathPrefix(nil); got != "" {
		t.Errorf("empty = %q", got)
	}
	if got := commonPathPrefix([]string{"src/main.go"}); got != "src/main.go" {
		t.Errorf("single = %q", got)
	}
	if got := commonPathPrefix([]string{"src/main.go", "src/magic/"}); got != "src/ma" {
		t.Errorf("lcp = %q", got)
	}
}

func TestListPathCompletions(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "zebra"))
	writeFile(t, filepath.Join(dir, "alpha.go"))
	writeFile(t, filepath.Join(dir, ".hidden"))
	if err := os.Mkdir(filepath.Join(dir, "apple"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "sub", "main.go"))

	// Prefix match, dirs first: apple/ and sub/ before the files.
	got, total := listPathCompletions(dir, "")
	if total != 4 || len(got) != 4 { // .hidden excluded
		t.Fatalf("root list = %v (total %d), want 4 non-hidden", got, total)
	}
	if got[0] != "apple/" || got[1] != "sub/" || got[2] != "alpha.go" || got[3] != "zebra" {
		t.Errorf("root order = %v, want dirs-first alphabetical", got)
	}

	// Dot-prefix reveals hidden files.
	got, _ = listPathCompletions(dir, ".h")
	if len(got) != 1 || got[0] != ".hidden" {
		t.Errorf("hidden list = %v", got)
	}

	// Subdirectory keeps its prefix in the replacement.
	got, _ = listPathCompletions(dir, "sub/m")
	if len(got) != 1 || got[0] != "sub/main.go" {
		t.Errorf("subdir list = %v", got)
	}

	// Missing directory reads as no match, not an error.
	if got, total := listPathCompletions(dir, "nope/n"); got != nil || total != 0 {
		t.Errorf("missing dir = %v, %d", got, total)
	}
}

func TestListPathCompletionsCap(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < maxCompletionCandidates+10; i++ {
		writeFile(t, filepath.Join(dir, "f"+itoa(i)))
	}
	got, total := listPathCompletions(dir, "f")
	if total != maxCompletionCandidates+10 {
		t.Errorf("total = %d, want %d", total, maxCompletionCandidates+10)
	}
	if len(got) != maxCompletionCandidates {
		t.Errorf("capped at %d, got %d", maxCompletionCandidates, len(got))
	}
}

func TestBuildCompletionTokenQuotesSpaces(t *testing.T) {
	if got := buildCompletionToken("", "@", "my dir/", "", false); got != `"@my dir/"` {
		t.Errorf("quoted = %q", got)
	}
	if got := buildCompletionToken("", "@", "my dir/", "", true); got != "@my dir/" {
		t.Errorf("already quoted = %q", got)
	}
	if got := buildCompletionToken("(", "@", "src/ma", ",", false); got != "(@src/ma," {
		t.Errorf("affixes = %q", got)
	}
}

func TestTabSingleCompletion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @mai")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @main.go" {
		t.Errorf("value = %q, want %q", got, "read @main.go")
	}
	if len(m.compCandidates) != 0 {
		t.Errorf("single match keeps no list, got %v", m.compCandidates)
	}

	// Directories complete with a trailing slash.
	m.textarea.SetValue("cd @su")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "cd @sub/" {
		t.Errorf("dir value = %q", got)
	}

	// Trailing sentence punctuation survives the replacement.
	m.textarea.SetValue("see @mai,")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "see @main.go," {
		t.Errorf("affix value = %q", got)
	}
}

func TestTabListsAndCycles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.go"))
	writeFile(t, filepath.Join(dir, "alpine.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @a")

	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @alp" {
		t.Fatalf("common prefix = %q, want %q", got, "read @alp")
	}
	if view := m.View(); !strings.Contains(view, "alpha.go") || !strings.Contains(view, "alpine.go") {
		t.Fatalf("View() hides the list:\n%s", view)
	}

	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @alpha.go" {
		t.Errorf("first cycle = %q", got)
	}
	if view := m.View(); !strings.Contains(view, "> alpha.go") {
		t.Errorf("View() marks nothing selected:\n%s", view)
	}

	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @alpine.go" {
		t.Errorf("second cycle = %q", got)
	}

	// Shift+Tab walks back, wrapping around.
	m = pressTab(t, m, false)
	if got := m.textarea.Value(); got != "read @alpha.go" {
		t.Errorf("backward cycle = %q", got)
	}
}

func TestTabWithoutAtDoesNothing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read mai")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read mai" {
		t.Errorf("non-@ Tab rewrote the text: %q", got)
	}
	if len(m.compCandidates) != 0 {
		t.Errorf("non-@ Tab built a list: %v", m.compCandidates)
	}
	if view := m.View(); strings.Contains(view, "main.go") || strings.Contains(view, "no match") {
		t.Errorf("non-@ Tab shows completion UI:\n%s", view)
	}
}

func TestTabNoMatchNotice(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @zzz")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @zzz" {
		t.Errorf("no match rewrote the text: %q", got)
	}
	if view := m.View(); !strings.Contains(view, "no match") {
		t.Errorf("View() shows no notice:\n%s", view)
	}
}

func TestTabEmptyIndents(t *testing.T) {
	m := NewInputModel(">", nil)
	m.textarea.SetValue("")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "    " {
		t.Errorf("empty Tab = %q, want four spaces", got)
	}
}

func TestTypingClearsCompletionList(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.go"))
	writeFile(t, filepath.Join(dir, "alpine.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @a")
	m = pressTab(t, m, true)
	if len(m.compCandidates) != 2 {
		t.Fatalf("setup: want live list, got %v", m.compCandidates)
	}
	mod, _ := m.Update(runeKey('x'))
	m = mod.(InputModel)
	if len(m.compCandidates) != 0 {
		t.Errorf("typing kept the list: %v", m.compCandidates)
	}
	if view := m.View(); strings.Contains(view, "  alpha.go") {
		t.Errorf("View() still shows the list:\n%s", view)
	}
}

func TestTabMidLinePreservesSuffix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("use @mai here")
	m.textarea.SetCursor(8) // right after "@mai"
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "use @main.go here" {
		t.Fatalf("value = %q", got)
	}
	if col := m.cursorHardCol(); col != len([]rune("use @main.go")) {
		t.Errorf("cursor at %d, want after the insertion", col)
	}
}

func TestTabQuotedPathWithSpace(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "my dir"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(dir, "my dir", "f.txt"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue(`read "@my `)
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != `read "@my dir/` {
		t.Errorf("quoted value = %q", got)
	}
}

func TestTabSubdirPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sub", "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @sub/m")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @sub/main.go" {
		t.Errorf("value = %q", got)
	}
}

func TestTabFirstLineOfTwo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @mai\nsecond line")
	// Back up to the end of the first line: this is the path that
	// walks the cursor up after SetValue resets it to the end.
	for m.textarea.Line() > 0 {
		m.textarea.CursorUp()
	}
	m.textarea.CursorEnd()
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @main.go\nsecond line" {
		t.Fatalf("value = %q", got)
	}
	if row := m.textarea.Line(); row != 0 {
		t.Errorf("cursor row = %d, want 0", row)
	}
	if col := m.cursorHardCol(); col != len([]rune("read @main.go")) {
		t.Errorf("cursor col = %d, want after the insertion", col)
	}
}

func TestTildeCompletesToHome(t *testing.T) {
	dir := t.TempDir()
	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @~")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @~/" {
		t.Errorf("value = %q, want %q", got, "read @~/")
	}
}

func TestFuzzyScore(t *testing.T) {
	if _, ok := fuzzyScore("sn-p", "docs/redesign-mcp.md"); !ok {
		t.Errorf("sn-p should fuzzy-match redesign-mcp.md")
	}
	if _, ok := fuzzyScore("sn-p", "docs/redesign-plan.md"); !ok {
		t.Errorf("sn-p should fuzzy-match redesign-plan.md")
	}
	if _, ok := fuzzyScore("sn-p", "docs/review.md"); ok {
		t.Errorf("sn-p should not match review.md")
	}
	if _, ok := fuzzyScore("XYZ", "abc"); ok {
		t.Errorf("XYZ should not match abc")
	}
	// A true substring outranks a sparse subsequence.
	sub, _ := fuzzyScore("plan", "docs/plan.md")
	sparse, _ := fuzzyScore("plan", "docs/redesign-plan.md")
	if !(sub < sparse) {
		t.Errorf("substring score %d should beat subsequence %d", sub, sparse)
	}
	// Case-insensitive.
	if _, ok := fuzzyScore("SN-P", "docs/redesign-mcp.md"); !ok {
		t.Errorf("matching should be case-insensitive")
	}
}

func TestTabFuzzyFindsNestedFiles(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docs", "redesign-mcp.md"))
	writeFile(t, filepath.Join(dir, "docs", "redesign-plan.md"))
	writeFile(t, filepath.Join(dir, "docs", "review.md"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @sn-p")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @sn-p" {
		t.Fatalf("fuzzy Tab rewrote the abbreviation: %q", got)
	}
	if len(m.compCandidates) != 2 {
		t.Fatalf("candidates = %v, want the two redesign files", m.compCandidates)
	}
	view := m.View()
	if !strings.Contains(view, "docs/redesign-mcp.md") || !strings.Contains(view, "docs/redesign-plan.md") {
		t.Fatalf("View() hides the fuzzy matches:\n%s", view)
	}
	if strings.Contains(view, "review.md") {
		t.Errorf("View() shows a non-match:\n%s", view)
	}

	// Cycling replaces the abbreviation with the full relative path.
	m = pressTab(t, m, true)
	got := m.textarea.Value()
	if got != "read @docs/redesign-mcp.md" && got != "read @docs/redesign-plan.md" {
		t.Errorf("cycle = %q, want a full redesign path", got)
	}
}

func TestTabFuzzySubstring(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "docs", "redesign-mcp.md"))
	writeFile(t, filepath.Join(dir, "src", "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @esign-m")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @docs/redesign-mcp.md" {
		t.Errorf("substring single = %q", got)
	}
}

func TestTabAtAloneBrowsesRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"))
	if err := os.Mkdir(filepath.Join(dir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @")
	m = pressTab(t, m, true)
	if len(m.compCandidates) != 2 {
		t.Fatalf("candidates = %v, want root listing", m.compCandidates)
	}
	view := m.View()
	if !strings.Contains(view, "main.go") || !strings.Contains(view, "sub/") {
		t.Errorf("View() hides the root listing:\n%s", view)
	}
}

func TestDownArrowSelectsCompletion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.go"))
	writeFile(t, filepath.Join(dir, "alpine.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @a")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read @alp" {
		t.Fatalf("setup: Tab extended to %q, want %q", got, "read @alp")
	}

	m = pressKey(t, m, downKey())
	if got := m.textarea.Value(); got != "read @alpha.go" {
		t.Errorf("down first = %q, want first candidate", got)
	}
	m = pressKey(t, m, downKey())
	if got := m.textarea.Value(); got != "read @alpine.go" {
		t.Errorf("down second = %q, want second candidate", got)
	}
	// Wraps around.
	m = pressKey(t, m, downKey())
	if got := m.textarea.Value(); got != "read @alpha.go" {
		t.Errorf("down wrap = %q", got)
	}
}

func TestUpArrowSelectsCompletionBackwards(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.go"))
	writeFile(t, filepath.Join(dir, "alpine.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @a")
	m = pressTab(t, m, true)

	// Nothing selected yet: up starts from the bottom, like Shift+Tab.
	m = pressKey(t, m, upKey())
	if got := m.textarea.Value(); got != "read @alpine.go" {
		t.Errorf("up first = %q, want last candidate", got)
	}
	m = pressKey(t, m, upKey())
	if got := m.textarea.Value(); got != "read @alpha.go" {
		t.Errorf("up second = %q", got)
	}
}

func TestArrowsBeatHistoryWhileCompleting(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.go"))
	writeFile(t, filepath.Join(dir, "alpine.go"))

	m := NewInputModel(">", []string{"older prompt"})
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @a")
	m = pressTab(t, m, true)
	if len(m.compCandidates) != 2 {
		t.Fatalf("setup: want live list, got %v", m.compCandidates)
	}

	// With a list showing, up selects instead of pulling history.
	m = pressKey(t, m, upKey())
	if got := m.textarea.Value(); got != "read @alpine.go" {
		t.Errorf("up with list = %q, want a candidate, not history", got)
	}
}

func TestUpDownWithoutCompletionStillHistory(t *testing.T) {
	m := NewInputModel(">", []string{"older prompt"})
	m.textarea.SetValue("")
	m = pressKey(t, m, upKey())
	if got := m.textarea.Value(); got != "older prompt" {
		t.Errorf("up without list = %q, want history", got)
	}
	m = pressKey(t, m, downKey())
	if got := m.textarea.Value(); got != "" {
		t.Errorf("down without list = %q, want back to empty", got)
	}
}

func TestEscDismissesCompletionFirst(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "alpha.go"))
	writeFile(t, filepath.Join(dir, "alpine.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @a")
	m = pressTab(t, m, true)
	if len(m.compCandidates) != 2 {
		t.Fatalf("setup: want live list, got %v", m.compCandidates)
	}

	// First esc drops only the list; the draft survives.
	m = pressKey(t, m, escKey())
	if len(m.compCandidates) != 0 {
		t.Fatalf("esc kept the list: %v", m.compCandidates)
	}
	if got := m.textarea.Value(); got != "read @alp" {
		t.Errorf("esc cleared the draft: %q", got)
	}

	// Second esc, with nothing showing, clears the input as before.
	m = pressKey(t, m, escKey())
	if got := m.textarea.Value(); got != "" {
		t.Errorf("second esc = %q, want cleared input", got)
	}
}

func TestEscDismissesNoMatchNotice(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read @zzz")
	m = pressTab(t, m, true)
	if m.compNotice == "" {
		t.Fatalf("setup: want a no-match notice")
	}
	m = pressKey(t, m, escKey())
	if m.compNotice != "" {
		t.Errorf("esc kept the notice: %q", m.compNotice)
	}
	if got := m.textarea.Value(); got != "read @zzz" {
		t.Errorf("esc cleared the draft: %q", got)
	}
}

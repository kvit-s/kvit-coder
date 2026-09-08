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
	m.textarea.SetValue("cd su")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "cd sub/" {
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
	m.textarea.SetValue("read a")

	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read alp" {
		t.Fatalf("common prefix = %q, want %q", got, "read alp")
	}
	if view := m.View(); !strings.Contains(view, "alpha.go") || !strings.Contains(view, "alpine.go") {
		t.Fatalf("View() hides the list:\n%s", view)
	}

	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read alpha.go" {
		t.Errorf("first cycle = %q", got)
	}
	if view := m.View(); !strings.Contains(view, "> alpha.go") {
		t.Errorf("View() marks nothing selected:\n%s", view)
	}

	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read alpine.go" {
		t.Errorf("second cycle = %q", got)
	}

	// Shift+Tab walks back, wrapping around.
	m = pressTab(t, m, false)
	if got := m.textarea.Value(); got != "read alpha.go" {
		t.Errorf("backward cycle = %q", got)
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
	m.textarea.SetValue("read a")
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
	m.textarea.SetValue("use mai here")
	m.textarea.SetCursor(7) // right after "mai"
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "use main.go here" {
		t.Fatalf("value = %q", got)
	}
	if col := m.cursorHardCol(); col != len([]rune("use main.go")) {
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
	m.textarea.SetValue(`read "my `)
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != `read "my dir/` {
		t.Errorf("quoted value = %q", got)
	}
}

func TestTabSubdirPrefix(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "sub", "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read sub/m")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read sub/main.go" {
		t.Errorf("value = %q", got)
	}
}

func TestTabFirstLineOfTwo(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "main.go"))

	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read mai\nsecond line")
	// Back up to the end of the first line: this is the path that
	// walks the cursor up after SetValue resets it to the end.
	for m.textarea.Line() > 0 {
		m.textarea.CursorUp()
	}
	m.textarea.CursorEnd()
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read main.go\nsecond line" {
		t.Fatalf("value = %q", got)
	}
	if row := m.textarea.Line(); row != 0 {
		t.Errorf("cursor row = %d, want 0", row)
	}
	if col := m.cursorHardCol(); col != len([]rune("read main.go")) {
		t.Errorf("cursor col = %d, want after the insertion", col)
	}
}

func TestTildeCompletesToHome(t *testing.T) {
	dir := t.TempDir()
	m := NewInputModel(">", nil)
	m.SetCompletionBaseDir(dir)
	m.textarea.SetValue("read ~")
	m = pressTab(t, m, true)
	if got := m.textarea.Value(); got != "read ~/" {
		t.Errorf("value = %q, want %q", got, "read ~/")
	}
}

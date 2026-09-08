package session

import (
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

func TestFirstWordsTitle(t *testing.T) {
	for prompt, want := range map[string]string{
		"fix the login bug now please":   "fix the login bug now",
		"  fix   the login   bug  now  ": "fix the login bug now",
		"one two three":                  "one two three",
		"one":                            "one",
		"add dark mode\nwith tests":      "add dark mode with tests",
		"":                               "",
		"   \n\t  ":                      "",
	} {
		if got := FirstWordsTitle(prompt, 5); got != want {
			t.Errorf("FirstWordsTitle(%q) = %q, want %q", prompt, got, want)
		}
	}
	if got := FirstWordsTitle("one two three four five six", 0); got != "one two three four five" {
		t.Errorf("n<=0 should select the 5-word default, got %q", got)
	}
}

func TestNormalizeTitle(t *testing.T) {
	for raw, want := range map[string]string{
		`  Fix login bug  `:       "Fix login bug",
		`"Fix login bug"`:         "Fix login bug",
		"'Fix login bug'":         "Fix login bug",
		"`Fix login bug`":         "Fix login bug",
		"Fix login bug.":          "Fix login bug.",
		"First line\nSecond line": "First line",
		"\n\nReal title\n":        "Real title",
		"":                        "",
		"   ":                     "",
		`"""`:                     "",
	} {
		if got := NormalizeTitle(raw); got != want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", raw, got, want)
		}
	}
	long := strings.Repeat("word ", 30)
	got := NormalizeTitle(long)
	if len(got) > MaxTitleChars {
		t.Errorf("NormalizeTitle did not cap at %d chars: %q", MaxTitleChars, got)
	}
	if strings.HasSuffix(got, " ") || strings.HasPrefix(got, " ") {
		t.Errorf("capped title has stray spaces: %q", got)
	}
}

// TestTitleRoundTrip: a title written to meta.json survives reopening, which
// is what listings and --session-show read.
func TestTitleRoundTrip(t *testing.T) {
	mgr := setupTestManager(t)
	sess, err := mgr.Open("titled")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	sess.Meta().Title = "Fix login bug"
	if err := sess.SaveMeta(); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	reopened, err := mgr.Open("titled")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := reopened.Meta().Title; got != "Fix login bug" {
		t.Errorf("title did not survive reopening: %q", got)
	}
}

// TestListSessionsCarriesTitle: the --sessions listing shows the title, not
// just the YYYY-MM-DD-random6 label.
func TestListSessionsCarriesTitle(t *testing.T) {
	mgr := setupTestManager(t)
	sess := writeSession(t, mgr, "2026-09-08-abc123", []llm.Message{{Role: llm.RoleUser, Content: "hi"}})
	sess.Meta().Title = "Fix login bug"
	if err := sess.SaveMeta(); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	sessions, err := mgr.ListSessions()
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 1 {
		t.Fatalf("want 1 session, got %d", len(sessions))
	}
	if sessions[0].Title != "Fix login bug" {
		t.Errorf("listing title = %q, want %q", sessions[0].Title, "Fix login bug")
	}
}

// TestShowSessionIncludesTitle: --session-show names the title.
func TestShowSessionIncludesTitle(t *testing.T) {
	mgr := setupTestManager(t)
	sess := writeSession(t, mgr, "shown", []llm.Message{{Role: llm.RoleUser, Content: "hi"}})
	sess.Meta().Title = "Fix login bug"
	if err := sess.SaveMeta(); err != nil {
		t.Fatalf("SaveMeta: %v", err)
	}
	content, err := mgr.ShowSession("shown")
	if err != nil {
		t.Fatalf("ShowSession: %v", err)
	}
	if !strings.Contains(content, "Title: Fix login bug") {
		t.Errorf("show output has no title line:\n%s", content)
	}
}

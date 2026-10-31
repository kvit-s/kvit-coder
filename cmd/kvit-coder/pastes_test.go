package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// Each -paste file is copied into the session's pastes/, and the prompt gets a
// line per paste naming its tag, its size and where the copy is, which is all
// the model receives of it.
func TestAttachPastesNamesEachCopy(t *testing.T) {
	sess, err := session.OpenDir(filepath.Join(t.TempDir(), "s"))
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	a := filepath.Join(src, "a.txt")
	b := filepath.Join(src, "b.txt")
	os.WriteFile(a, []byte("first\nsecond\n"), 0o644)
	os.WriteFile(b, []byte("only"), 0o644)
	missing := filepath.Join(src, "gone.txt")

	got := attachPastes("compare [paste1] with [paste3]\n", []string{a, missing, b}, sess, ui.NewWriter(0))

	if !strings.HasPrefix(got, "compare [paste1] with [paste3]\n\n") {
		t.Errorf("the prompt itself changed:\n%s", got)
	}
	notes := strings.Split(strings.SplitN(got, "\n\n", 2)[1], "\n")
	if len(notes) != 3 {
		t.Fatalf("want one line per paste, got:\n%s", got)
	}
	path := regexp.MustCompile(`saved as (.*)\. Read that file`)
	for i, want := range []struct{ label, about, content string }{
		{"[paste1]", "(2 lines, 13 bytes)", "first\nsecond\n"},
		{"[paste3]", "(1 line, 4 bytes)", "only"},
	} {
		note := notes[2*i]
		if !strings.HasPrefix(note, want.label+" is text the user pasted "+want.about) {
			t.Errorf("note %d = %q", 2*i, note)
			continue
		}
		m := path.FindStringSubmatch(note)
		if m == nil || filepath.Dir(m[1]) != sess.PastesDir() {
			t.Errorf("note %q does not name a file in %s", note, sess.PastesDir())
			continue
		}
		if data, err := os.ReadFile(m[1]); err != nil || string(data) != want.content {
			t.Errorf("copy of %s holds %q, %v", want.label, data, err)
		}
	}
	if notes[1] != "[paste2] was pasted but could not be saved, so its text is not available." {
		t.Errorf("a missing paste was reported as %q", notes[1])
	}
}

// No pastes, no change: a turn without them sends the prompt as it was.
func TestAttachPastesWithoutPastes(t *testing.T) {
	if got := attachPastes("hello", nil, nil, ui.NewWriter(0)); got != "hello" {
		t.Errorf("prompt = %q", got)
	}
}

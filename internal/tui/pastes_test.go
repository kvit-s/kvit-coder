package tui

import (
	"os"
	"strings"
	"testing"
)

// Long pastes go to the agent as files, one -paste per file, in the order of
// their [pasteN] tags.
func TestTurnPassesPastesInTagOrder(t *testing.T) {
	u := newTestUI(t, "pastes", nil)
	args := strings.Join(u.agentArgs("compare [paste1] with [paste2]", nil, []string{"/tmp/a.txt", "/tmp/b.txt"}), " ")
	if !strings.Contains(args, "-paste /tmp/a.txt -paste /tmp/b.txt") {
		t.Errorf("args = %s, want both pastes in order", args)
	}
	if !strings.Contains(args, "compare [paste1] with [paste2]") {
		t.Errorf("args = %s, want the prompt with its tags", args)
	}
}

// The staged file holds the text exactly, and removing it after the turn
// leaves nothing behind.
func TestStagePastedText(t *testing.T) {
	t.Setenv("TMPDIR", t.TempDir())
	path, err := StagePastedText("line one\nline two\n")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "line one\nline two\n" {
		t.Fatalf("staged %q, %v", data, err)
	}
	removePastes([]string{path})
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("the staged paste is still there after the turn: %v", err)
	}
}

// Only the pastes whose tags were sent are kept for the turn.
func TestUnsentPastes(t *testing.T) {
	got := unsentPastes([]string{"a", "b", "c"}, []string{"c", "a"})
	if len(got) != 1 || got[0] != "b" {
		t.Errorf("unsentPastes = %v, want [b]", got)
	}
	if got := unsentPastes([]string{"a"}, nil); len(got) != 1 {
		t.Errorf("nothing sent: unsentPastes = %v, want every staged paste", got)
	}
}

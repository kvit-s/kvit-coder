package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// sessionDir creates a session directory under base, with a meta.json
// holding the given display title (none when title is empty).
func sessionDir(t *testing.T, base, name, title string) {
	t.Helper()
	dir := filepath.Join(base, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	meta := `{"name":` + quote(name) + `}`
	if title != "" {
		meta = `{"name":` + quote(name) + `,"title":` + quote(title) + `}`
	}
	if err := os.WriteFile(filepath.Join(dir, "meta.json"), []byte(meta), 0o644); err != nil {
		t.Fatalf("write meta: %v", err)
	}
}

func quote(s string) string { return `"` + s + `"` }

func TestSessionLabelPrefersTheSessionTitle(t *testing.T) {
	base := t.TempDir()
	sessionDir(t, base, "2026-09-08-aabbcc", "add the retry loop")
	sessionDir(t, base, "2026-09-08-ddeeff", "")

	mgr, err := session.NewManagerIn(base)
	if err != nil {
		t.Fatalf("NewManagerIn: %v", err)
	}
	u := New(Options{Config: testCatalogConfig(), SessionMgr: mgr})

	if got := u.sessionLabel(); got != "" {
		t.Fatalf("with no session, label = %q, want empty", got)
	}

	// A titled session shows its title.
	u.currentSession = "2026-09-08-aabbcc"
	if got, want := u.sessionLabel(), "add the retry loop"; got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}

	// Switching sessions drops the cached title rather than showing the
	// previous session's; an untitled session falls back to its name.
	u.currentSession = "2026-09-08-ddeeff"
	if got, want := u.sessionLabel(), "2026-09-08-ddeeff"; got != want {
		t.Fatalf("label = %q, want %q", got, want)
	}

	// A title written while the session is open (the first turn is what
	// creates one) is picked up at the next read.
	sessionDir(t, base, "2026-09-08-ddeeff", "rename the flag")
	if got, want := u.sessionLabel(), "rename the flag"; got != want {
		t.Fatalf("label after titling = %q, want %q", got, want)
	}
}

// TestRunAgentMovesTheTitleThroughTheTurn drives a turn against a stand-in
// agent binary and checks the window title says "running" while it runs and
// goes back to "waiting" when it is done, with the session's title picked up
// from the meta.json the turn wrote.
func TestRunAgentMovesTheTitleThroughTheTurn(t *testing.T) {
	base := t.TempDir()
	sessionDir(t, base, "2026-09-08-aabbcc", "")
	mgr, err := session.NewManagerIn(base)
	if err != nil {
		t.Fatalf("NewManagerIn: %v", err)
	}

	// The stand-in agent titles the session the way a real first turn does,
	// so the waiting title has to be read after the turn, not before it.
	agent := filepath.Join(t.TempDir(), "agent")
	script := "#!/bin/sh\nprintf '{\"name\":\"x\",\"title\":\"add the retry loop\"}' > " +
		filepath.Join(base, "2026-09-08-aabbcc", "meta.json") + "\n"
	if err := os.WriteFile(agent, []byte(script), 0o755); err != nil {
		t.Fatalf("write agent: %v", err)
	}

	u := New(Options{Config: testCatalogConfig(), SessionMgr: mgr, AgentPath: agent})
	u.currentSession = "2026-09-08-aabbcc"
	var buf strings.Builder
	u.title = ui.NewTerminalTitle(&buf)

	u.runAgent("hello", nil)

	want := "\033]0;" + ui.DefaultTitleRunningIcon + " 2026-09-08-aabbcc\007" +
		"\033]0;" + ui.DefaultTitleWaitingIcon + " add the retry loop\007"
	if got := buf.String(); got != want {
		t.Fatalf("title stream = %q, want %q", got, want)
	}
}

package repl

import (
	"testing"

	"github.com/kvit-s/kvit-coder/internal/procs"
	"github.com/kvit-s/kvit-coder/internal/session"
)

// backgroundSuffix is empty without a session or without running processes,
// " ★N" for ephemeral blues and " ★N!" for persistent reds — the same
// registry the next turn reads.
func TestBackgroundSuffix(t *testing.T) {
	if got := backgroundSuffix(nil); got != "" {
		t.Errorf("nil session = %q, want empty", got)
	}

	sess, err := session.OpenDir(t.TempDir())
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	if got := backgroundSuffix(sess); got != "" {
		t.Errorf("no processes = %q, want empty", got)
	}

	reg, err := procs.New(sess.ProcDir())
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	id, err := reg.Start("sleep 30", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = reg.Kill(id) }()

	if got := backgroundSuffix(sess); got != " ★1" {
		t.Errorf("one running = %q, want %q", got, " ★1")
	}
	if err := reg.Kill(id); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	if got := backgroundSuffix(sess); got != "" {
		t.Errorf("after kill = %q, want empty", got)
	}
}

// backgroundSuffix marks persistent survivors red with a bang, so the
// footing tells ephemeral blues from what needs stopping.
func TestBackgroundSuffixPersistent(t *testing.T) {
	sess, err := session.OpenDir(t.TempDir())
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	reg, err := procs.New(sess.ProcDir())
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	keep, err := reg.StartWithOptions("sleep 30", t.TempDir(), "server", "exit", "", 0, true)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = reg.Kill(keep) }()

	if got := backgroundSuffix(sess); got != " ★1!" {
		t.Errorf("one persistent = %q, want %q", got, " ★1!")
	}

	ephem, err := reg.Start("sleep 30", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() { _ = reg.Kill(ephem) }()

	if got := backgroundSuffix(sess); got != " ★1 ★1!" {
		t.Errorf("one of each = %q, want %q", got, " ★1 ★1!")
	}
}

// endTurnProcs stops the ephemeral defaults and returns the persistent
// survivors: abandoned sessions leak nothing, dev servers live on.
func TestEndTurnProcs(t *testing.T) {
	sess, err := session.OpenDir(t.TempDir())
	if err != nil {
		t.Fatalf("OpenDir: %v", err)
	}
	reg, err := procs.New(sess.ProcDir())
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	ephem, err := reg.Start("sleep 30", t.TempDir(), "scratch")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	keep, err := reg.StartWithOptions("sleep 30", t.TempDir(), "server", "exit", "", 0, true)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = reg.Kill(keep) }()

	killed, persistent := endTurnProcs(sess)
	if len(killed) != 1 || killed[0] != ephem {
		t.Errorf("killed = %v, want [%s]", killed, ephem)
	}
	if len(persistent) != 1 || persistent[0].ID != keep {
		t.Errorf("persistent = %v, want [%s]", persistent, keep)
	}
	if info, _ := reg.Status(ephem); info.Running() {
		t.Error("the ephemeral process is still running after the turn end")
	}
	if info, _ := reg.Status(keep); !info.Running() {
		t.Errorf("the persistent process is %s, want it surviving", info.State)
	}
	// The footing after cleanup shows only the red survivor.
	if got := backgroundSuffix(sess); got != " ★1!" {
		t.Errorf("footing after cleanup = %q, want %q", got, " ★1!")
	}
}

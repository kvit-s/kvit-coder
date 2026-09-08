package tui

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/procs"
	"github.com/kvit-s/kvit-coder/internal/session"
)

// TestPollProcsToInboxDeliversExitOnce: a process that ended between turns is
// materialized as one inbox file, and a second poll finds nothing new.
func TestPollProcsToInboxDeliversExitOnce(t *testing.T) {
	procDir := t.TempDir()
	inboxDir := t.TempDir()

	reg, err := procs.New(procDir)
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	id, err := reg.Start("echo all done; exit 2", t.TempDir(), "build")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		info, err := reg.Status(id)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if !info.Running() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the process to finish")
		}
		time.Sleep(20 * time.Millisecond)
	}

	n, err := PollProcsToInbox(procDir, inboxDir)
	if err != nil {
		t.Fatalf("PollProcsToInbox: %v", err)
	}
	if n != 1 {
		t.Fatalf("delivered %d events, want 1", n)
	}
	if got := inbox.PendingCount(inboxDir); got != 1 {
		t.Fatalf("PendingCount = %d, want 1", got)
	}

	got := inbox.New(inboxDir).Drain()
	if len(got) != 1 || got[0].Kind != inbox.KindProcessEvent {
		t.Fatalf("drained %+v, want the one process event", got)
	}
	if got[0].Text == "" || !contains(got[0].Text, "exited with status 2") {
		t.Errorf("event text %q does not say how it ended", got[0].Text)
	}

	// The ending was marked reported, so a second poll is quiet.
	if n, err := PollProcsToInbox(procDir, inboxDir); err != nil || n != 0 {
		t.Errorf("second poll delivered %d (%v), want 0", n, err)
	}
	if got := inbox.PendingCount(inboxDir); got != 0 {
		t.Errorf("PendingCount after second poll = %d, want 0", got)
	}
}

// TestPollProcsToInboxTickDelivers: a due still-running reminder is
// materialized the same way, carrying the new output.
func TestPollProcsToInboxTickDelivers(t *testing.T) {
	procDir := t.TempDir()
	inboxDir := t.TempDir()

	reg, err := procs.New(procDir)
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	id, err := reg.StartWithOptions("echo first words; sleep 30", t.TempDir(), "", "exit", "", 1, true)
	if err != nil {
		t.Fatalf("StartWithOptions: %v", err)
	}
	defer func() { _ = reg.Kill(id) }()

	var n int
	deadline := time.Now().Add(10 * time.Second)
	for {
		var err error
		n, err = PollProcsToInbox(procDir, inboxDir)
		if err != nil {
			t.Fatalf("PollProcsToInbox: %v", err)
		}
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the tick")
		}
		time.Sleep(50 * time.Millisecond)
	}
	if got := inbox.PendingCount(inboxDir); got != 1 {
		t.Fatalf("PendingCount = %d, want 1", got)
	}
	got := inbox.New(inboxDir).Drain()
	if len(got) != 1 || !contains(got[0].Text, "first words") {
		t.Errorf("drained %+v, want the tick carrying the output", got)
	}
}

// TestPollProcsToInboxEmptyRegistry: a session that never started a
// background process is zero events, not an error. (New creates the proc
// directory on open; the point is the empty registry is quiet.)
func TestPollProcsToInboxEmptyRegistry(t *testing.T) {
	procDir := filepath.Join(t.TempDir(), "no-such-proc")
	inboxDir := t.TempDir()
	n, err := PollProcsToInbox(procDir, inboxDir)
	if err != nil {
		t.Fatalf("PollProcsToInbox: %v", err)
	}
	if n != 0 {
		t.Errorf("delivered %d events, want 0", n)
	}
	if got := inbox.PendingCount(inboxDir); got != 0 {
		t.Errorf("PendingCount = %d, want 0", got)
	}
}
func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// wakeTestUI roots a session manager at a temp HOME so the boundary check
// runs against a throwaway session instead of the user's own.
func wakeTestUI(t *testing.T, sessionName string) (*UI, *session.Session) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	mgr, err := session.NewManager()
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	sess, err := mgr.Open(sessionName)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return &UI{sessionMgr: mgr, currentSession: sessionName}, sess
}

// TestCheckWakeQuietWhenIdle: no session, no files, no procs — nothing to
// fire for.
func TestCheckWakeQuietWhenIdle(t *testing.T) {
	if got := (&UI{}).checkWake(); got != 0 {
		t.Errorf("checkWake = %d, want 0", got)
	}
	u, _ := wakeTestUI(t, "quiet-sess")
	if got := u.checkWake(); got != 0 {
		t.Errorf("checkWake = %d, want 0 for a fresh session", got)
	}
}

// TestCheckWakeFiresForSteeringAndProcEvents: a steered line waiting at the
// boundary fires the next turn, and a proc exit materializes alongside it —
// the startup case and the post-turn case take the same path.
func TestCheckWakeFiresForSteeringAndProcEvents(t *testing.T) {
	u, sess := wakeTestUI(t, "wake-sess")

	if _, err := inbox.Deliver(sess.InboxDir(), inbox.Message{
		Kind: inbox.KindUserLine,
		Text: "also update the tests",
	}); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if got := u.checkWake(); got != 1 {
		t.Fatalf("checkWake = %d, want 1 for the steered line", got)
	}

	reg, err := procs.New(sess.ProcDir())
	if err != nil {
		t.Fatalf("procs.New: %v", err)
	}
	id, err := reg.Start("echo all done; exit 0", t.TempDir(), "")
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		info, err := reg.Status(id)
		if err != nil {
			t.Fatalf("Status: %v", err)
		}
		if !info.Running() {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for the process to finish")
		}
		time.Sleep(20 * time.Millisecond)
	}

	if got := u.checkWake(); got != 2 {
		t.Fatalf("checkWake = %d, want the steered line plus the exit", got)
	}
	// Nothing consumed: the next turn's drain still sees both.
	if got := inbox.New(sess.InboxDir()).Drain(); len(got) != 2 {
		t.Errorf("drain after check drained %d, want 2", len(got))
	}
}

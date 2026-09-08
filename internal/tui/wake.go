package tui

import (
	"path/filepath"

	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/procs"
	"github.com/kvit-s/kvit-coder/internal/session"
)

// PollProcsToInbox moves due background-process events into the session inbox
// directory, so they wait for the next turn the same way steering does. It is
// the turn-boundary half of what the agent does every iteration: the agent
// polls the registry and pushes into memory (which dies with the turn), while
// the UI — the only thing alive between turns — polls and delivers into files
// (which survive until the next turn drains them).
//
// Calling Events marks what it returns as reported (ReportedOffset,
// LastEventAt), so an event materialized here is not reported again by the
// next turn's first poll, and one the agent already reported in-turn is not
// repeated here. A missing proc directory means no processes, not an error.
func PollProcsToInbox(procDir, inboxDir string) (int, error) {
	reg, err := procs.New(procDir)
	if err != nil {
		return 0, err
	}
	reg.Reconcile()
	events := reg.Events()
	for _, text := range events {
		if _, err := inbox.Deliver(inboxDir, inbox.Message{
			Kind: inbox.KindProcessEvent,
			Text: text,
		}); err != nil {
			return 0, err
		}
	}
	return len(events), nil
}

// sessionDirs locates the current session's inbox and proc directories: the
// same directories the agent turn reads and writes.
func (u *UI) sessionDirs() (inboxDir, procDir string) {
	if u.sessionMgr == nil || u.currentSession == "" {
		return "", ""
	}
	base := filepath.Join(u.sessionMgr.BaseDir(), u.currentSession)
	return filepath.Join(base, session.InboxSubdir), filepath.Join(base, session.ProcSubdir)
}

// wakePoll materializes due proc events into the inbox and reports how many
// inbox files wait. It is the composer's tick callback and the turn-boundary
// check alike: one function, so the badge, the auto-fire, and the boundary
// all agree on what "pending" means. A delivery error is ignored — the events
// stay unreported in the registry and the next poll retries them.
func (u *UI) wakePoll() (int, string) {
	inboxDir, procDir := u.sessionDirs()
	if inboxDir == "" {
		return 0, ""
	}
	if procDir != "" {
		_, _ = PollProcsToInbox(procDir, inboxDir)
	}
	return inbox.PendingCount(inboxDir), ""
}

// checkWake is the turn-boundary form of the wake poll: how many inbox files
// wait for the next turn right now.
func (u *UI) checkWake() int {
	n, _ := u.wakePoll()
	return n
}

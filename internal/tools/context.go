package tools

import (
	"strings"
	"sync"
	"time"

	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/permissions"
)

// ToolContext holds shared mutable state for all tools in a session.
// This replaces the global variables (globalReadTracker, globalPendingEdit, globalPendingWrite)
// and enables proper testing and concurrent session isolation.
type ToolContext struct {
	ReadTracker *FileReadTracker

	pendingEditMu sync.Mutex
	pendingEdit   *pendingEdit

	pendingWriteMu sync.Mutex
	pendingWrite   *pendingWrite

	// inbox is where a tool that waits for a person reads the answer. It is
	// the same inbox the loop drains between iterations, so there is one input
	// path rather than two contending for stdin.
	inboxMu sync.Mutex
	inbox   *inbox.Inbox

	// dismissedQuestions remembers questions the person ended the turn on
	// rather than answering, so the model cannot ask the same one again.
	dismissedMu       sync.Mutex
	dismissedQuestion map[string]bool

	// grantor holds what has been allowed for this command, this session, this
	// project and this machine, and can add to it.
	grantorMu sync.Mutex
	grantor   *permissions.Grantor

	// promptWait is how long this process has spent, in total, waiting for a
	// person to answer a prompt. The agent loop subtracts it from a tool's
	// elapsed time before deciding the tool timed out: a read that waited two
	// minutes for you to allow a path did not take two minutes.
	promptWaitMu sync.Mutex
	promptWait   time.Duration

	// interactive says whether someone is at the terminal and able to answer.
	// It is set once, by whatever started reading stdin, so a tool that waits
	// for a person and the reader that would feed it never disagree.
	interactiveMu sync.Mutex
	interactive   bool
}

// NewToolContext creates a new ToolContext with initialized state.
func NewToolContext() *ToolContext {
	return &ToolContext{
		ReadTracker: &FileReadTracker{maxEntries: 10},
	}
}

// AddPromptWait records time spent waiting for a person to answer.
func (tc *ToolContext) AddPromptWait(d time.Duration) {
	if tc == nil || d <= 0 {
		return
	}
	tc.promptWaitMu.Lock()
	defer tc.promptWaitMu.Unlock()
	tc.promptWait += d
}

// PromptWait is the total time spent waiting for a person so far.
func (tc *ToolContext) PromptWait() time.Duration {
	if tc == nil {
		return 0
	}
	tc.promptWaitMu.Lock()
	defer tc.promptWaitMu.Unlock()
	return tc.promptWait
}

// SetGrantor gives tools the permission grants that apply to this run.
func (tc *ToolContext) SetGrantor(g *permissions.Grantor) {
	tc.grantorMu.Lock()
	defer tc.grantorMu.Unlock()
	tc.grantor = g
}

// Grantor returns the permission grants that apply to this run, or nil.
func (tc *ToolContext) Grantor() *permissions.Grantor {
	if tc == nil {
		return nil
	}
	tc.grantorMu.Lock()
	defer tc.grantorMu.Unlock()
	return tc.grantor
}

// SetInteractive records whether someone is at the terminal to answer a tool
// that asks. It is set by whatever started reading stdin.
func (tc *ToolContext) SetInteractive(interactive bool) {
	tc.interactiveMu.Lock()
	defer tc.interactiveMu.Unlock()
	tc.interactive = interactive
}

// Interactive reports whether a tool that waits for a person can expect one.
func (tc *ToolContext) Interactive() bool {
	if tc == nil {
		return false
	}
	tc.interactiveMu.Lock()
	defer tc.interactiveMu.Unlock()
	return tc.interactive
}

// SetInbox gives tools access to the turn's inbox.
func (tc *ToolContext) SetInbox(box *inbox.Inbox) {
	tc.inboxMu.Lock()
	defer tc.inboxMu.Unlock()
	tc.inbox = box
}

// Inbox returns the turn's inbox, or nil when there is none.
func (tc *ToolContext) Inbox() *inbox.Inbox {
	if tc == nil {
		return nil
	}
	tc.inboxMu.Lock()
	defer tc.inboxMu.Unlock()
	return tc.inbox
}

// RecordDismissedQuestion notes that a question was put and not answered.
func (tc *ToolContext) RecordDismissedQuestion(question string) {
	if tc == nil {
		return
	}
	tc.dismissedMu.Lock()
	defer tc.dismissedMu.Unlock()
	if tc.dismissedQuestion == nil {
		tc.dismissedQuestion = map[string]bool{}
	}
	tc.dismissedQuestion[normalizeQuestion(question)] = true
}

// WasQuestionDismissed reports whether this exact question was already put and
// dismissed.
func (tc *ToolContext) WasQuestionDismissed(question string) bool {
	if tc == nil {
		return false
	}
	tc.dismissedMu.Lock()
	defer tc.dismissedMu.Unlock()
	return tc.dismissedQuestion[normalizeQuestion(question)]
}

func normalizeQuestion(q string) string {
	return strings.ToLower(strings.Join(strings.Fields(q), " "))
}

// SetPendingEdit stores a pending edit operation.
func (tc *ToolContext) SetPendingEdit(p *pendingEdit) {
	tc.pendingEditMu.Lock()
	defer tc.pendingEditMu.Unlock()
	tc.pendingEdit = p
}

// GetPendingEdit returns the current pending edit without clearing it.
func (tc *ToolContext) GetPendingEdit() *pendingEdit {
	tc.pendingEditMu.Lock()
	defer tc.pendingEditMu.Unlock()
	return tc.pendingEdit
}

// GetAndClearPendingEdit returns and clears the pending edit.
func (tc *ToolContext) GetAndClearPendingEdit() *pendingEdit {
	tc.pendingEditMu.Lock()
	defer tc.pendingEditMu.Unlock()
	p := tc.pendingEdit
	tc.pendingEdit = nil
	return p
}

// ClearPendingEdit clears any pending edit.
func (tc *ToolContext) ClearPendingEdit() {
	tc.pendingEditMu.Lock()
	defer tc.pendingEditMu.Unlock()
	tc.pendingEdit = nil
}

// HasPendingEdit returns true if there's a pending edit.
func (tc *ToolContext) HasPendingEdit() bool {
	tc.pendingEditMu.Lock()
	defer tc.pendingEditMu.Unlock()
	return tc.pendingEdit != nil
}

// GetPendingEditPath returns the path of the pending edit, or empty if none.
func (tc *ToolContext) GetPendingEditPath() string {
	tc.pendingEditMu.Lock()
	defer tc.pendingEditMu.Unlock()
	if tc.pendingEdit == nil {
		return ""
	}
	return tc.pendingEdit.path
}

// GetPendingEditDiff returns the diff of the pending edit, or empty if none.
func (tc *ToolContext) GetPendingEditDiff() string {
	tc.pendingEditMu.Lock()
	defer tc.pendingEditMu.Unlock()
	if tc.pendingEdit == nil {
		return ""
	}
	return tc.pendingEdit.diff
}

// ClearPendingEditIfPath clears pending edit only if it matches the given path.
func (tc *ToolContext) ClearPendingEditIfPath(path string) {
	tc.pendingEditMu.Lock()
	defer tc.pendingEditMu.Unlock()
	if tc.pendingEdit != nil && tc.pendingEdit.path == path {
		tc.pendingEdit = nil
	}
}

// SetPendingWrite stores a pending write operation.
func (tc *ToolContext) SetPendingWrite(p *pendingWrite) {
	tc.pendingWriteMu.Lock()
	defer tc.pendingWriteMu.Unlock()
	tc.pendingWrite = p
}

// GetAndClearPendingWrite returns and clears the pending write.
func (tc *ToolContext) GetAndClearPendingWrite() *pendingWrite {
	tc.pendingWriteMu.Lock()
	defer tc.pendingWriteMu.Unlock()
	p := tc.pendingWrite
	tc.pendingWrite = nil
	return p
}

// GetPendingWritePath returns the path of the pending write, or empty if none.
// HasPendingWrite reports whether this process is holding a staged overwrite.
func (tc *ToolContext) HasPendingWrite() bool {
	return tc.GetPendingWritePath() != ""
}

func (tc *ToolContext) GetPendingWritePath() string {
	tc.pendingWriteMu.Lock()
	defer tc.pendingWriteMu.Unlock()
	if tc.pendingWrite == nil {
		return ""
	}
	return tc.pendingWrite.path
}

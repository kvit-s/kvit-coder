package tools

import "sync"

// ToolContext holds shared mutable state for all tools in a session.
// This replaces the global variables (globalReadTracker, globalPendingEdit, globalPendingWrite)
// and enables proper testing and concurrent session isolation.
type ToolContext struct {
	ReadTracker *FileReadTracker

	pendingEditMu sync.Mutex
	pendingEdit   *pendingEdit

	pendingWriteMu sync.Mutex
	pendingWrite   *pendingWrite
}

// NewToolContext creates a new ToolContext with initialized state.
func NewToolContext() *ToolContext {
	return &ToolContext{
		ReadTracker: &FileReadTracker{maxEntries: 10},
	}
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
func (tc *ToolContext) GetPendingWritePath() string {
	tc.pendingWriteMu.Lock()
	defer tc.pendingWriteMu.Unlock()
	if tc.pendingWrite == nil {
		return ""
	}
	return tc.pendingWrite.path
}

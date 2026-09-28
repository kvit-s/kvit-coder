package context

import (
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// openTestTask appends a Tasks.Start pair with "task started", so the
// manager reports an unfinished task.
func openTestTask(t *testing.T, m *Manager) {
	t.Helper()
	startArgs := `{"task":"explore"}`
	if err := m.AppendMessage(llm.Message{
		Role:    llm.RoleAssistant,
		Content: "Starting task: explore",
		ToolCalls: []llm.ToolCall{{
			ID:   "tasks_start",
			Type: "function",
			Function: llm.ToolCallFunction{
				Name:      "Tasks.Start",
				Arguments: startArgs,
			},
		}},
	}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
	if err := m.AppendMessage(llm.Message{
		Role:       llm.RoleTool,
		Name:       "Tasks.Start",
		ToolCallID: "tasks_start",
		Content:    `{"result":"task started","_internal":{"checkpoint_id":"checkpoint_1"}}`,
	}); err != nil {
		t.Fatalf("AppendMessage: %v", err)
	}
}

func newTestMiddleware(t *testing.T, cfg RuntimeNoticeConfig) (*Manager, *Middleware) {
	t.Helper()
	m, err := NewManager(t.TempDir(), nil)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize: %v", err)
	}
	return m, NewMiddleware(m, cfg)
}

// Outside a task there is no notice, however many responses pass.
func TestNoNoticeOutsideTask(t *testing.T) {
	_, mw := newTestMiddleware(t, RuntimeNoticeConfig{TaskWarnTurns: 5, TaskCriticalTurns: 10})
	for i := 0; i < 12; i++ {
		if got := mw.ProcessAssistantMessage("work"); strings.Contains(got, "<runtime-notice>") {
			t.Fatalf("notice outside task on response %d:\n%s", i+1, got)
		}
	}
}

// The warn/critical thresholds count real responses inside the task, not
// Tasks.* pairs in the repo (which stay near zero while the model works).
func TestTaskLengthWarningsCountResponses(t *testing.T) {
	_, mw := newTestMiddleware(t, RuntimeNoticeConfig{TaskWarnTurns: 5, TaskCriticalTurns: 10})
	_ = mw.manager // open task on the middleware's manager
	m := mw.manager
	openTestTask(t, m)

	var got string
	for i := 0; i < 6; i++ {
		got = mw.ProcessAssistantMessage("work")
	}
	if !strings.Contains(got, "Task running for 6 turns") {
		t.Errorf("warn threshold did not fire after 6 responses:\n%s", got)
	}
	for i := 0; i < 5; i++ {
		got = mw.ProcessAssistantMessage("work")
	}
	if !strings.Contains(got, "Long-running task") {
		t.Errorf("critical threshold did not fire after 11 responses:\n%s", got)
	}
}

// Near the hard turn limit the notice names the remaining budget so the
// model finishes instead of being cut off mid-task.
func TestHardLimitBudgetLine(t *testing.T) {
	_, mw := newTestMiddleware(t, RuntimeNoticeConfig{
		TaskWarnTurns: 5, TaskCriticalTurns: 100, MaxIterations: 20,
	})
	openTestTask(t, mw.manager)

	// Early in the run with a short task: no budget line yet.
	mw.SetIteration(2, 20)
	if got := mw.ProcessAssistantMessage("work"); strings.Contains(got, "hard turn limit") {
		t.Errorf("budget line too early:\n%s", got)
	}
	// Same position, but the task is now long: budget is named.
	for i := 0; i < 6; i++ {
		mw.ProcessAssistantMessage("work")
	}
	mw.SetIteration(10, 20)
	if got := mw.ProcessAssistantMessage("work"); !strings.Contains(got, "Iteration 11 of 20 (hard turn limit in 9)") {
		t.Errorf("long task misses budget line:\n%s", got)
	}
	// Short task but nearly spent budget: budget is named anyway.
	_, mw2 := newTestMiddleware(t, RuntimeNoticeConfig{
		TaskWarnTurns: 100, TaskCriticalTurns: 1000, MaxIterations: 20,
	})
	openTestTask(t, mw2.manager)
	mw2.SetIteration(18, 20)
	if got := mw2.ProcessAssistantMessage("work"); !strings.Contains(got, "hard turn limit in 1") {
		t.Errorf("low budget misses budget line:\n%s", got)
	}
}

// Unknown budget disables the line without affecting the other warnings.
func TestUnknownBudgetDisablesBudgetLine(t *testing.T) {
	_, mw := newTestMiddleware(t, RuntimeNoticeConfig{TaskWarnTurns: 5, TaskCriticalTurns: 100})
	openTestTask(t, mw.manager)
	var got string
	for i := 0; i < 7; i++ {
		got = mw.ProcessAssistantMessage("work")
	}
	if !strings.Contains(got, "Task running for") {
		t.Errorf("warn threshold missing:\n%s", got)
	}
	if strings.Contains(got, "hard turn limit") {
		t.Errorf("budget line without known budget:\n%s", got)
	}
}

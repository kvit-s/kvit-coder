package context

import (
	"fmt"
	"strings"
)

// RuntimeNoticeConfig configures when runtime notices are injected
type RuntimeNoticeConfig struct {
	TaskWarnTurns       int  // Warn after N turns in task (default: 5)
	TaskCriticalTurns   int  // Critical warning after N turns (default: 10)
	ContextCapacityWarn int  // Warn at N% context capacity (default: 80)
	MaxNestedDepth      int  // Max task nesting depth warning (default: 2)
	NotifyFileChanges   bool // Notify about file changes in task
	// MaxIterations is the agent loop hard turn limit
	// (agent.max_tool_iterations). When known, task warnings name the
	// remaining budget so the model finishes before the turn is cut off.
	MaxIterations int // Hard turn limit for the whole run (0 = unknown)
}

// Middleware processes LLM responses before storage
type Middleware struct {
	manager *Manager
	config  RuntimeNoticeConfig

	// In-process counters for real task length. The tasks repo only holds
	// Tasks.* pairs, so CountTurnsSinceTaskStart on it stays near zero while
	// the model does dozens of reads/edits inside the task (the 7xmo86
	// failure: ~70 responses, counter stuck at 1, no warning ever fired).
	// Counting ProcessAssistantMessage calls while a task is open measures
	// what the model actually did. Counters live per OS process, which is
	// per turn; a task carried into a new turn restarts its count there.
	totalResponses int
	taskResponses  int
	wasInTask      bool
	// Latest loop position for hard-limit math, set by the runner each
	// iteration. iter is 0-based, maxIters the loop cap; -1/0 = unknown.
	iter     int
	maxIters int
}

// NewMiddleware creates a new context tools middleware
func NewMiddleware(manager *Manager, config RuntimeNoticeConfig) *Middleware {
	return &Middleware{
		manager:  manager,
		config:   config,
		iter:     -1,
		maxIters: config.MaxIterations,
	}
}

// ProcessAssistantMessage injects turn number and optional runtime notice
// into an assistant message before it's stored to history
func (m *Middleware) ProcessAssistantMessage(content string) string {
	if content == "" {
		return content
	}

	// Get current turn number
	turnNumber := m.manager.GetTurnCount() + 1

	// Inject turn number at START
	result := fmt.Sprintf("[turn_%d] %s", turnNumber, content)

	// Track real task length: the tasks repo only records Tasks.* pairs,
	// so repo-derived counts never grow while the model works inside the
	// task. Counting responses while a task is open is what fires the
	// warn/critical thresholds below.
	inTask := m.unfinishedTask()
	m.totalResponses++
	if inTask {
		if !m.wasInTask {
			m.taskResponses = 0
		}
		m.taskResponses++
	} else {
		m.taskResponses = 0
	}
	m.wasInTask = inTask

	// Inject runtime notice at END (if applicable)
	if notice := m.generateRuntimeNotice(); notice != "" {
		result = fmt.Sprintf("%s\n\n<runtime-notice>\n%s\n</runtime-notice>", result, notice)
	}

	return result
}

// SetIteration records the runner loop position for hard-limit warnings.
// iter is 0-based, maxIters the loop cap (agent.max_tool_iterations).
// Call once per iteration; unknown values are -1/0 and disable the budget line.
func (m *Middleware) SetIteration(iter, maxIters int) {
	m.iter = iter
	m.maxIters = maxIters
}

// unfinishedTask reports whether a task is currently open, tolerating a
// missing manager read.
func (m *Middleware) unfinishedTask() bool {
	turns, err := m.manager.ReadTurnsForLLM()
	if err != nil {
		return false
	}
	return HasUnfinishedTaskInHistory(turns)
}

// taskTurns returns the real number of responses produced inside the open
// task in this process, falling back to the repo-derived count (which only
// sees Tasks.* pairs) when the in-process counter has not started yet.
func (m *Middleware) taskTurns(repoTurns int) int {
	if m.taskResponses > 0 {
		return m.taskResponses
	}
	return repoTurns
}

// generateRuntimeNotice generates a runtime notice based on current state
func (m *Middleware) generateRuntimeNotice() string {
	turns, err := m.manager.ReadTurnsForLLM()
	if err != nil {
		return ""
	}

	// Check if in task
	if !HasUnfinishedTaskInHistory(turns) {
		return ""
	}

	// Count turns since task started. Prefer the in-process response count:
	// the repo only holds Tasks.* pairs, so its count stays near zero while
	// the model does real work inside the task.
	taskTurns := m.taskTurns(CountTurnsSinceTaskStart(turns))

	var guidance []string

	// Long-running task warning
	if m.config.TaskCriticalTurns > 0 && taskTurns > m.config.TaskCriticalTurns {
		guidance = append(guidance, "Long-running task. Strongly consider finishing or breaking into sub-tasks.")
	} else if m.config.TaskWarnTurns > 0 && taskTurns > m.config.TaskWarnTurns {
		guidance = append(guidance, fmt.Sprintf("Task running for %d turns. Consider summarizing if complete.", taskTurns))
	}

	// Hard turn limit: name the remaining budget when the task is getting
	// long or the loop is nearly spent, so the model finishes with
	// Tasks.Finish instead of being cut off mid-task.
	if m.maxIters > 0 && m.iter >= 0 {
		remaining := m.maxIters - m.iter - 1
		if remaining < 0 {
			remaining = 0
		}
		longTask := (m.config.TaskWarnTurns > 0 && taskTurns >= m.config.TaskWarnTurns) ||
			(m.config.TaskCriticalTurns > 0 && taskTurns >= m.config.TaskCriticalTurns)
		lowBudget := remaining <= 10
		if longTask || lowBudget {
			guidance = append(guidance, fmt.Sprintf("Iteration %d of %d (hard turn limit in %d). Finish the task with Tasks.Finish before the budget runs out.", m.iter+1, m.maxIters, remaining))
		}
	}

	// Nesting depth warning
	depth := GetTaskDepth(turns)
	if m.config.MaxNestedDepth > 0 && depth > m.config.MaxNestedDepth {
		guidance = append(guidance, fmt.Sprintf("Nested task depth is %d. Consider simplifying.", depth))
	}

	// File changes notification (if enabled)
	if m.config.NotifyFileChanges {
		// Count file changes from checkpoint manager
		if m.manager.checkpointMgr != nil {
			modifiedFiles := m.manager.checkpointMgr.GetModifiedFiles()
			if len(modifiedFiles) > 0 {
				guidance = append(guidance, fmt.Sprintf("%d files modified in this task.", len(modifiedFiles)))
			}
		}
	}

	return strings.Join(guidance, "\n")
}

// InjectTurnNumber is a simpler version that only injects turn number
// Use this when you don't need runtime notices
func (m *Middleware) InjectTurnNumber(content string) string {
	if content == "" {
		return content
	}

	turnNumber := m.manager.GetTurnCount() + 1
	return fmt.Sprintf("[turn_%d] %s", turnNumber, content)
}

// ExtractTurnNumber extracts the turn number from a message content
// Returns 0 if no turn number found
func ExtractTurnNumber(content string) int {
	if !strings.HasPrefix(content, "[turn_") {
		return 0
	}

	// Find closing bracket
	endIdx := strings.Index(content, "]")
	if endIdx == -1 {
		return 0
	}

	// Parse the number
	numStr := content[6:endIdx] // Skip "[turn_"
	var num int
	_, err := fmt.Sscanf(numStr, "%d", &num)
	if err != nil {
		return 0
	}

	return num
}

// StripTurnNumber removes the turn number prefix from content
func StripTurnNumber(content string) string {
	if !strings.HasPrefix(content, "[turn_") {
		return content
	}

	// Find closing bracket and space
	endIdx := strings.Index(content, "] ")
	if endIdx == -1 {
		return content
	}

	return content[endIdx+2:]
}

// StripRuntimeNotice removes the runtime notice suffix from content
func StripRuntimeNotice(content string) string {
	noticeStart := strings.Index(content, "\n\n<runtime-notice>")
	if noticeStart == -1 {
		return content
	}

	return content[:noticeStart]
}

// CleanContent removes both turn number and runtime notice from content
func CleanContent(content string) string {
	content = StripTurnNumber(content)
	content = StripRuntimeNotice(content)
	return content
}

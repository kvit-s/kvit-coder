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
}

// Middleware processes LLM responses before storage
type Middleware struct {
	manager *Manager
	config  RuntimeNoticeConfig
}

// NewMiddleware creates a new context tools middleware
func NewMiddleware(manager *Manager, config RuntimeNoticeConfig) *Middleware {
	return &Middleware{
		manager: manager,
		config:  config,
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

	// Inject runtime notice at END (if applicable)
	if notice := m.generateRuntimeNotice(); notice != "" {
		result = fmt.Sprintf("%s\n\n<runtime-notice>\n%s\n</runtime-notice>", result, notice)
	}

	return result
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

	// Count turns since task started
	taskTurns := CountTurnsSinceTaskStart(turns)

	var guidance []string

	// Long-running task warning
	if m.config.TaskCriticalTurns > 0 && taskTurns > m.config.TaskCriticalTurns {
		guidance = append(guidance, "Long-running task. Strongly consider finishing or breaking into sub-tasks.")
	} else if m.config.TaskWarnTurns > 0 && taskTurns > m.config.TaskWarnTurns {
		guidance = append(guidance, fmt.Sprintf("Task running for %d turns. Consider summarizing if complete.", taskTurns))
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

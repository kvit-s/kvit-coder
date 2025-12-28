package agent

import (
	"sync"
)

// BacktrackConfig holds configuration for backtrack mode
type BacktrackConfig struct {
	Enabled           bool // Enable backtrack mode
	MaxRetries        int  // Max retries at same history point (default: 5)
	InjectUserMessage bool // On limit reached: backtrack + inject user message instead of error-in-history
}

// DefaultBacktrackConfig returns the default backtrack configuration
func DefaultBacktrackConfig() BacktrackConfig {
	return BacktrackConfig{
		Enabled:    true,
		MaxRetries: 5,
	}
}

// DiscardedStats tracks costs for discarded (backtracked) requests
type DiscardedStats struct {
	TotalPromptTokens     int
	TotalCompletionTokens int
	TotalCost             float64
	DiscardCount          int
}

// BacktrackEvent records a single backtrack occurrence for reporting
type BacktrackEvent struct {
	ToolName string
	Reason   string
}

// BacktrackTracker tracks backtrack state and costs
type BacktrackTracker struct {
	mu     sync.Mutex
	config BacktrackConfig

	// Cost tracking for discarded requests
	discardedStats DiscardedStats

	// Retry tracking at current history point (identified by message count)
	currentHistoryLen int
	retriesAtPoint    int

	// Events for reporting
	lastEvent *BacktrackEvent
}

// NewBacktrackTracker creates a new backtrack tracker with the given config
func NewBacktrackTracker(config BacktrackConfig) *BacktrackTracker {
	return &BacktrackTracker{
		config: config,
	}
}

// RecordDiscarded records costs for a discarded request
func (bt *BacktrackTracker) RecordDiscarded(promptTokens, completionTokens int, cost float64) {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	bt.discardedStats.TotalPromptTokens += promptTokens
	bt.discardedStats.TotalCompletionTokens += completionTokens
	bt.discardedStats.TotalCost += cost
	bt.discardedStats.DiscardCount++
}

// ShouldBacktrack returns true if should backtrack, false if should fall back to error-in-history.
// It also increments the retry counter for the current history point.
func (bt *BacktrackTracker) ShouldBacktrack(historyLen int) bool {
	bt.mu.Lock()
	defer bt.mu.Unlock()

	if !bt.config.Enabled {
		return false
	}

	if historyLen != bt.currentHistoryLen {
		// New history point, reset counter
		bt.currentHistoryLen = historyLen
		bt.retriesAtPoint = 0
	}

	bt.retriesAtPoint++
	return bt.retriesAtPoint <= bt.config.MaxRetries
}

// GetRetryCount returns the current retry count at this history point
func (bt *BacktrackTracker) GetRetryCount() int {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	return bt.retriesAtPoint
}

// GetMaxRetries returns the maximum retries configured
func (bt *BacktrackTracker) GetMaxRetries() int {
	return bt.config.MaxRetries
}

// ResetAtPoint resets the retry counter (called after fallback to error-in-history)
func (bt *BacktrackTracker) ResetAtPoint() {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.retriesAtPoint = 0
}

// GetDiscardedStats returns the accumulated stats for discarded requests
func (bt *BacktrackTracker) GetDiscardedStats() DiscardedStats {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	return bt.discardedStats
}

// SetLastEvent records the last backtrack event for reporting
func (bt *BacktrackTracker) SetLastEvent(toolName, reason string) {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.lastEvent = &BacktrackEvent{
		ToolName: toolName,
		Reason:   reason,
	}
}

// GetLastEvent returns and clears the last backtrack event
func (bt *BacktrackTracker) GetLastEvent() *BacktrackEvent {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	event := bt.lastEvent
	bt.lastEvent = nil
	return event
}

// IsEnabled returns whether backtrack mode is enabled
func (bt *BacktrackTracker) IsEnabled() bool {
	return bt.config.Enabled
}

// ShouldInjectUserMessage returns whether user message should be injected on limit
func (bt *BacktrackTracker) ShouldInjectUserMessage() bool {
	return bt.config.InjectUserMessage
}

// Reset resets the tracker state (but not accumulated stats)
func (bt *BacktrackTracker) Reset() {
	bt.mu.Lock()
	defer bt.mu.Unlock()
	bt.currentHistoryLen = 0
	bt.retriesAtPoint = 0
	bt.lastEvent = nil
}

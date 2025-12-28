package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"sync"
)

// LoopDetector tracks tool calls and detects when the LLM is stuck in a loop
type LoopDetector struct {
	mu        sync.Mutex
	history   []callRecord
	maxHistory int
}

type callRecord struct {
	hash      string // hash of tool+args+error
	toolName  string
	isError   bool
}

// LoopInfo contains information about a detected loop
type LoopInfo struct {
	ToolName  string
	Count     int
	IsError   bool // All calls in the loop were errors
	IsSuccess bool // All calls in the loop were successes (still a loop!)
}

// NewLoopDetector creates a new loop detector
func NewLoopDetector() *LoopDetector {
	return &LoopDetector{
		history:    make([]callRecord, 0),
		maxHistory: 20, // Keep track of last 20 calls
	}
}

// normalizeArgs normalizes JSON args so that different key orders produce the same string
// This ensures {"a":1,"b":2} and {"b":2,"a":1} hash to the same value
func normalizeArgs(args string) string {
	var data map[string]any
	if err := json.Unmarshal([]byte(args), &data); err != nil {
		// Not valid JSON, return as-is
		return args
	}

	// json.Marshal produces deterministic output with sorted keys
	normalized, err := json.Marshal(sortedMap(data))
	if err != nil {
		return args
	}
	return string(normalized)
}

// sortedMap recursively processes a map to ensure consistent ordering
func sortedMap(m map[string]any) map[string]any {
	result := make(map[string]any)
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		v := m[k]
		if nested, ok := v.(map[string]any); ok {
			result[k] = sortedMap(nested)
		} else {
			result[k] = v
		}
	}
	return result
}

// Record records a tool call result
func (ld *LoopDetector) Record(toolName, args, result string, isError bool) {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	// Create hash of tool+args+result (first 200 chars of result to normalize)
	resultPrefix := result
	if len(resultPrefix) > 200 {
		resultPrefix = resultPrefix[:200]
	}

	// Normalize args to handle different key orderings
	normalizedArgs := normalizeArgs(args)

	h := sha256.New()
	h.Write([]byte(toolName))
	h.Write([]byte(normalizedArgs))
	h.Write([]byte(resultPrefix))
	hash := hex.EncodeToString(h.Sum(nil))[:16]

	ld.history = append(ld.history, callRecord{
		hash:     hash,
		toolName: toolName,
		isError:  isError,
	})

	// Trim old entries
	if len(ld.history) > ld.maxHistory {
		ld.history = ld.history[len(ld.history)-ld.maxHistory:]
	}
}

// DetectLoop checks if the last N calls are identical (same tool, args, and result)
// Returns loop info if detected, nil otherwise
// Detects both error loops AND success loops (LLM repeating same successful call)
func (ld *LoopDetector) DetectLoop(threshold int) *LoopInfo {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	if len(ld.history) < threshold {
		return nil
	}

	// Check last N entries
	recent := ld.history[len(ld.history)-threshold:]

	// All must have the same hash
	firstHash := recent[0].hash
	firstTool := recent[0].toolName
	allErrors := true
	allSuccesses := true

	for _, r := range recent {
		if r.hash != firstHash {
			return nil
		}
		if !r.isError {
			allErrors = false
		} else {
			allSuccesses = false
		}
	}

	return &LoopInfo{
		ToolName:   firstTool,
		Count:      threshold,
		IsError:    allErrors,
		IsSuccess:  allSuccesses,
	}
}

// DetectErrorLoop specifically checks for repeated errors (any errors, not necessarily identical)
// This catches cases where LLM keeps hitting the same error with slightly different args
func (ld *LoopDetector) DetectErrorLoop(threshold int) *LoopInfo {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	if len(ld.history) < threshold {
		return nil
	}

	// Check last N entries - all must be errors for the same tool
	recent := ld.history[len(ld.history)-threshold:]
	firstTool := recent[0].toolName

	for _, r := range recent {
		if r.toolName != firstTool || !r.isError {
			return nil
		}
	}

	return &LoopInfo{
		ToolName: firstTool,
		Count:    threshold,
		IsError:  true,
	}
}

// Reset clears the history (call when user provides new input)
func (ld *LoopDetector) Reset() {
	ld.mu.Lock()
	defer ld.mu.Unlock()
	ld.history = make([]callRecord, 0)
}

// DetectAlternatingLoop detects patterns like A→B→A→B→A→B where two calls repeat
// This catches loops like Edit→Edit.cancel→Edit→Edit.cancel
// threshold is the number of complete A→B cycles required (e.g., 3 means A→B→A→B→A→B)
func (ld *LoopDetector) DetectAlternatingLoop(threshold int) *LoopInfo {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	// Need at least threshold*2 entries for threshold cycles
	needed := threshold * 2
	if len(ld.history) < needed {
		return nil
	}

	recent := ld.history[len(ld.history)-needed:]

	// Check for alternating pattern: entries at even indices should match,
	// entries at odd indices should match, and the two patterns should be different
	evenHash := recent[0].hash
	oddHash := recent[1].hash

	// The two hashes must be different (otherwise it's just consecutive identical calls)
	if evenHash == oddHash {
		return nil
	}

	for i, r := range recent {
		if i%2 == 0 {
			if r.hash != evenHash {
				return nil
			}
		} else {
			if r.hash != oddHash {
				return nil
			}
		}
	}

	// Found alternating pattern - return info about the primary tool (first in cycle)
	return &LoopInfo{
		ToolName:  recent[0].toolName,
		Count:     threshold,
		IsError:   false,
		IsSuccess: false,
	}
}

// GetRecentErrorCount returns the count of consecutive recent errors for a specific tool
func (ld *LoopDetector) GetRecentErrorCount(toolName string) int {
	ld.mu.Lock()
	defer ld.mu.Unlock()

	count := 0
	for i := len(ld.history) - 1; i >= 0; i-- {
		if ld.history[i].toolName == toolName && ld.history[i].isError {
			count++
		} else {
			break
		}
	}
	return count
}

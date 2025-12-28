// Package stats provides statistics tracking for agent operations.
package stats

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// AgentStats tracks cumulative statistics across all agent iterations
type AgentStats struct {
	TotalPromptTokens     int
	TotalCompletionTokens int
	TotalCacheReadTokens  int
	TotalAgentTime        time.Duration
	TotalLLMTime          time.Duration
	TotalToolTime         time.Duration
	Steps                 int // Number of LLM iterations
	MaxContextUsed        int
	TotalCost             float64
	CacheDiscount         float64

	// Timing stats from LLM server (in milliseconds)
	TotalPromptMS     float64
	TotalGenerationMS float64

	// Backtrack stats (discarded requests)
	DiscardedPromptTokens     int
	DiscardedCompletionTokens int
	DiscardedCost             float64
	BacktrackCount            int
}

// AgentStatsJSON is the JSON output format for agent stats
type AgentStatsJSON struct {
	Tokens struct {
		Prompt     int `json:"prompt"`
		Completion int `json:"completion"`
		CacheRead  int `json:"cache_read"`
	} `json:"tokens"`
	Timing struct {
		TotalAgentSeconds float64 `json:"total_agent_seconds"`
		LLMSeconds        float64 `json:"llm_seconds"`
		ToolSeconds       float64 `json:"tool_seconds"`
		PromptMS          float64 `json:"prompt_ms"`
		GenerationMS      float64 `json:"generation_ms"`
	} `json:"timing"`
	Steps          int `json:"steps"`
	MaxContextUsed int `json:"max_context_used"`
	Cost           struct {
		CacheDiscountUSD float64 `json:"cache_discount_usd"`
		TotalCostUSD     float64 `json:"total_cost_usd"`
	} `json:"cost"`
	Backtrack struct {
		DiscardedPromptTokens     int     `json:"discarded_prompt_tokens,omitempty"`
		DiscardedCompletionTokens int     `json:"discarded_completion_tokens,omitempty"`
		DiscardedCostUSD          float64 `json:"discarded_cost_usd,omitempty"`
		BacktrackCount            int     `json:"backtrack_count,omitempty"`
	} `json:"backtrack,omitempty"`
}

// ToJSON converts AgentStats to its JSON representation
func (s *AgentStats) ToJSON() AgentStatsJSON {
	var j AgentStatsJSON
	j.Tokens.Prompt = s.TotalPromptTokens
	j.Tokens.Completion = s.TotalCompletionTokens
	j.Tokens.CacheRead = s.TotalCacheReadTokens
	j.Timing.TotalAgentSeconds = s.TotalAgentTime.Seconds()
	j.Timing.LLMSeconds = s.TotalLLMTime.Seconds()
	j.Timing.ToolSeconds = s.TotalToolTime.Seconds()
	j.Timing.PromptMS = s.TotalPromptMS
	j.Timing.GenerationMS = s.TotalGenerationMS
	j.Steps = s.Steps
	j.MaxContextUsed = s.MaxContextUsed
	j.Cost.CacheDiscountUSD = s.CacheDiscount
	j.Cost.TotalCostUSD = s.TotalCost
	j.Backtrack.DiscardedPromptTokens = s.DiscardedPromptTokens
	j.Backtrack.DiscardedCompletionTokens = s.DiscardedCompletionTokens
	j.Backtrack.DiscardedCostUSD = s.DiscardedCost
	j.Backtrack.BacktrackCount = s.BacktrackCount
	return j
}

// Print outputs the agent stats in a formatted JSON block to stdout
func (s *AgentStats) Print() {
	s.PrintTo(os.Stdout)
}

// PrintTo outputs the agent stats in a formatted JSON block to the given writer
func (s *AgentStats) PrintTo(w io.Writer) {
	j := s.ToJSON()
	jsonBytes, _ := json.MarshalIndent(j, "", "  ")
	fmt.Fprintln(w, "=== AGENT STATS START ===")
	fmt.Fprintln(w, string(jsonBytes))
	fmt.Fprintln(w, "=== AGENT STATS END ===")
}

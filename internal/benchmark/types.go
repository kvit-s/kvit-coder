// Package benchmark provides benchmarking infrastructure for testing LLM tool usage.
package benchmark

import (
	"encoding/json"
	"time"
)

// BenchmarkDef defines a single benchmark test case.
type BenchmarkDef struct {
	ID         string            `yaml:"id"`
	Name       string            `yaml:"name"`
	Category   string            `yaml:"category"`
	Goal       string            `yaml:"goal"`
	Setup      []SetupFile       `yaml:"setup"`
	Task       string            `yaml:"task"`
	Validation []ValidationCheck `yaml:"validation"`
	Tags       []string          `yaml:"tags"`
}

// SetupFile defines a file to create for benchmark setup.
type SetupFile struct {
	File    string `yaml:"file"`
	Content string `yaml:"content"`
	Binary  []byte `yaml:"binary,omitempty"` // For binary file tests
	Dir     bool   `yaml:"dir,omitempty"`    // Create directory instead of file
}

// ValidationCheck defines a validation condition for benchmark success.
type ValidationCheck struct {
	Type     string `yaml:"type"`     // "file_contains", "file_equals", "file_exists", "file_not_exists", "file_line_count", "tool_called", "tool_called_with", "output_contains", "output_not_contains", "multi_tool_calls", "run_command"
	Target   string `yaml:"target"`   // File path or "output"
	Expected string `yaml:"expected"` // Expected value, pattern, or tool name
	Args     string `yaml:"args"`     // Expected args pattern (for tool_called_with)
	Command  string `yaml:"command"`  // Command to run (for run_command)
	Count    int    `yaml:"count"`    // Expected count (for file_line_count, multi_tool_calls)
	Line     int    `yaml:"line"`     // Specific line number (for file_line_equals)
	Negate   bool   `yaml:"negate"`   // Check for absence instead of presence
}

// RunResult captures metrics from a single benchmark run.
type RunResult struct {
	BenchmarkID     string        `json:"benchmark_id" csv:"benchmark_id"`
	Run             int           `json:"run" csv:"run"`
	Success         bool          `json:"success" csv:"success"`
	LLMCalls        int           `json:"llm_calls" csv:"llm_calls"`
	Tokens          int           `json:"tokens" csv:"tokens"`
	PromptTokens    int           `json:"prompt_tokens" csv:"prompt_tokens"`
	GeneratedTokens int           `json:"generated_tokens" csv:"generated_tokens"`
	CachedTokens    int           `json:"cached_tokens" csv:"cached_tokens"`
	ContextUsed     int           `json:"context_used" csv:"context_used"`
	Cost            float64       `json:"cost" csv:"cost"`
	DurationMS      int64         `json:"duration_ms" csv:"duration_ms"`
	PromptMS        float64       `json:"prompt_ms" csv:"prompt_ms"`
	GenerationMS    float64       `json:"generation_ms" csv:"generation_ms"`
	ToolCalls       []ToolCallLog `json:"tool_calls" csv:"-"`
	Errors          []string      `json:"errors" csv:"-"`
	StartedAt       time.Time     `json:"started_at" csv:"started_at"`
	CompletedAt     time.Time     `json:"completed_at" csv:"completed_at"`

	// For CSV serialization
	ToolCallsJSON string `json:"-" csv:"tool_calls"`
	ErrorsJSON    string `json:"-" csv:"errors"`
}

// ToolCallLog captures a single tool call made during a benchmark run.
type ToolCallLog struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args,omitempty"`
}

// BenchmarkConfig holds configuration for benchmark runs.
type BenchmarkConfig struct {
	Enabled         bool     `yaml:"enabled"`
	OutputDir       string   `yaml:"output_dir"`
	RunsPerTask     int      `yaml:"runs_per_task"`
	TimeoutPerRun   int      `yaml:"timeout_per_run"` // seconds
	ReportFormat    string   `yaml:"report_format"`   // "markdown" or "json"
	Categories      []string `yaml:"categories"`      // Which categories to run
	BenchmarkIDs    []string `yaml:"benchmark_ids"`   // Specific benchmarks to run
	WarmupTask      string   `yaml:"warmup_task"`     // Warmup task (default: "Say hello")
	NoResume        bool     `yaml:"no_resume"`       // Force fresh start
}

// DefaultBenchmarkConfig returns default benchmark configuration.
func DefaultBenchmarkConfig() *BenchmarkConfig {
	return &BenchmarkConfig{
		Enabled:       true,
		OutputDir:     ".kvit-coder-benchmark",
		RunsPerTask:   10,
		TimeoutPerRun: 120,
		ReportFormat:  "markdown",
		WarmupTask:    "Say hello",
		NoResume:      false,
	}
}

// AggregatedStats holds aggregated statistics for a benchmark across all runs.
type AggregatedStats struct {
	BenchmarkID string  `json:"benchmark_id"`
	TotalRuns   int     `json:"total_runs"`
	Successes   int     `json:"successes"`
	Failures    int     `json:"failures"`
	SuccessRate float64 `json:"success_rate"`

	// LLM Calls
	LLMCallsMin    int     `json:"llm_calls_min"`
	LLMCallsMax    int     `json:"llm_calls_max"`
	LLMCallsMean   float64 `json:"llm_calls_mean"`
	LLMCallsMedian float64 `json:"llm_calls_median"`
	LLMCallsP5     float64 `json:"llm_calls_p5"`
	LLMCallsP95    float64 `json:"llm_calls_p95"`
	LLMCallsStdDev float64 `json:"llm_calls_stddev"`

	// Tokens (total: prompt + completion)
	TokensMin    int     `json:"tokens_min"`
	TokensMax    int     `json:"tokens_max"`
	TokensMean   float64 `json:"tokens_mean"`
	TokensMedian float64 `json:"tokens_median"`
	TokensP5     float64 `json:"tokens_p5"`
	TokensP95    float64 `json:"tokens_p95"`
	TokensStdDev float64 `json:"tokens_stddev"`

	// Generated Tokens (completion tokens only)
	GeneratedTokensMin    int     `json:"generated_tokens_min"`
	GeneratedTokensMax    int     `json:"generated_tokens_max"`
	GeneratedTokensMean   float64 `json:"generated_tokens_mean"`
	GeneratedTokensMedian float64 `json:"generated_tokens_median"`
	GeneratedTokensP5     float64 `json:"generated_tokens_p5"`
	GeneratedTokensP95    float64 `json:"generated_tokens_p95"`
	GeneratedTokensStdDev float64 `json:"generated_tokens_stddev"`

	// Context Used (max context window used)
	ContextUsedMin    int     `json:"context_used_min"`
	ContextUsedMax    int     `json:"context_used_max"`
	ContextUsedMean   float64 `json:"context_used_mean"`
	ContextUsedMedian float64 `json:"context_used_median"`
	ContextUsedP5     float64 `json:"context_used_p5"`
	ContextUsedP95    float64 `json:"context_used_p95"`

	// Processed Tokens (prompt - cached)
	ProcessedTokensMean float64 `json:"processed_tokens_mean"`

	// Speed metrics (tokens/sec, weighted averages across all runs)
	PromptSpeed     float64 `json:"prompt_speed"`     // processed tokens / prompt time
	GenerationSpeed float64 `json:"generation_speed"` // generated tokens / generation time

	// Cost
	CostMin    float64 `json:"cost_min"`
	CostMax    float64 `json:"cost_max"`
	CostMean   float64 `json:"cost_mean"`
	CostTotal  float64 `json:"cost_total"`
	CostStdDev float64 `json:"cost_stddev"`

	// Duration
	DurationMinMS    int64   `json:"duration_min_ms"`
	DurationMaxMS    int64   `json:"duration_max_ms"`
	DurationMeanMS   float64 `json:"duration_mean_ms"`
	DurationMedianMS float64 `json:"duration_median_ms"`
	DurationP5MS     float64 `json:"duration_p5_ms"`
	DurationP95MS    float64 `json:"duration_p95_ms"`
	DurationStdDevMS float64 `json:"duration_stddev_ms"`

	// Tool usage
	ToolCallCounts map[string]int `json:"tool_call_counts"`

	// Errors
	ErrorCounts map[string]int `json:"error_counts"`
}

// Report holds the complete benchmark report.
type Report struct {
	Version    string            `json:"version"`
	Date       time.Time         `json:"date"`
	Config     string            `json:"config"` // Full config.yaml contents
	Summary    []AggregatedStats `json:"summary"`
	Failures   []FailureDetail   `json:"failures"`
	RawResults []RunResult       `json:"raw_results,omitempty"` // Optional detailed results
}

// FailureDetail captures information about a failed benchmark run.
type FailureDetail struct {
	BenchmarkID  string   `json:"benchmark_id"`
	Run          int      `json:"run"`
	Errors       []string `json:"errors"`
	LastToolCall string   `json:"last_tool_call,omitempty"`
}

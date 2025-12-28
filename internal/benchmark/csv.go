package benchmark

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// CSVWriter handles writing benchmark results to CSV.
type CSVWriter struct {
	path   string
	file   *os.File
	writer *csv.Writer
}

// CSVHeaders are the column headers for the benchmark CSV.
var CSVHeaders = []string{
	"benchmark_id",
	"run",
	"success",
	"llm_calls",
	"tokens",
	"prompt_tokens",
	"generated_tokens",
	"cached_tokens",
	"context_used",
	"cost",
	"duration_ms",
	"prompt_ms",
	"generation_ms",
	"tool_calls",
	"errors",
	"started_at",
	"completed_at",
}

// NewCSVWriter creates a new CSV writer for benchmark results.
func NewCSVWriter(path string, resume bool) (*CSVWriter, error) {
	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, fmt.Errorf("failed to create CSV directory: %w", err)
	}

	var file *os.File
	var err error

	if resume {
		// Check if file exists and open for append
		if _, statErr := os.Stat(path); statErr == nil {
			file, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to open CSV for append: %w", err)
			}
		} else {
			// File doesn't exist, create new
			file, err = os.Create(path)
			if err != nil {
				return nil, fmt.Errorf("failed to create CSV file: %w", err)
			}
		}
	} else {
		// Always create new file
		file, err = os.Create(path)
		if err != nil {
			return nil, fmt.Errorf("failed to create CSV file: %w", err)
		}
	}

	w := &CSVWriter{
		path:   path,
		file:   file,
		writer: csv.NewWriter(file),
	}

	// Write headers if new file (check file size)
	info, _ := file.Stat()
	if info.Size() == 0 {
		if err := w.writer.Write(CSVHeaders); err != nil {
			return nil, fmt.Errorf("failed to write CSV headers: %w", err)
		}
		w.writer.Flush()
	}

	return w, nil
}

// WriteResult writes a single benchmark result to CSV.
func (w *CSVWriter) WriteResult(result *RunResult) error {
	// Serialize tool calls and errors to JSON
	toolCallsJSON, _ := json.Marshal(result.ToolCalls)
	errorsJSON, _ := json.Marshal(result.Errors)

	row := []string{
		result.BenchmarkID,
		strconv.Itoa(result.Run),
		strconv.FormatBool(result.Success),
		strconv.Itoa(result.LLMCalls),
		strconv.Itoa(result.Tokens),
		strconv.Itoa(result.PromptTokens),
		strconv.Itoa(result.GeneratedTokens),
		strconv.Itoa(result.CachedTokens),
		strconv.Itoa(result.ContextUsed),
		fmt.Sprintf("%.6f", result.Cost),
		strconv.FormatInt(result.DurationMS, 10),
		fmt.Sprintf("%.2f", result.PromptMS),
		fmt.Sprintf("%.2f", result.GenerationMS),
		string(toolCallsJSON),
		string(errorsJSON),
		result.StartedAt.Format(time.RFC3339),
		result.CompletedAt.Format(time.RFC3339),
	}

	if err := w.writer.Write(row); err != nil {
		return fmt.Errorf("failed to write CSV row: %w", err)
	}

	w.writer.Flush()
	return w.writer.Error()
}

// Close closes the CSV writer.
func (w *CSVWriter) Close() error {
	w.writer.Flush()
	return w.file.Close()
}

// LoadResults loads all benchmark results from a CSV file.
func LoadResults(path string) ([]RunResult, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No results yet
		}
		return nil, fmt.Errorf("failed to open CSV file: %w", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read CSV: %w", err)
	}

	if len(records) < 2 {
		return nil, nil // Only headers, no data
	}

	var results []RunResult
	for i, row := range records[1:] { // Skip header
		// Support multiple CSV formats:
		// - v1 (11 cols): benchmark_id,run,success,llm_calls,tokens,cost,duration_ms,tool_calls,errors,started_at,completed_at
		// - v2 (13 cols): + generated_tokens, context_used
		// - v3 (17 cols): + prompt_tokens, cached_tokens, prompt_ms, generation_ms
		if len(row) < 11 {
			return nil, fmt.Errorf("invalid CSV row %d: expected at least 11 columns, got %d", i+1, len(row))
		}

		run, _ := strconv.Atoi(row[1])
		llmCalls, _ := strconv.Atoi(row[3])
		tokens, _ := strconv.Atoi(row[4])

		var promptTokens, generatedTokens, cachedTokens, contextUsed int
		var cost float64
		var durationMS int64
		var promptMS, generationMS float64
		var toolCallsIdx, errorsIdx, startedAtIdx, completedAtIdx int

		if len(row) >= 17 {
			// v3 format with all fields
			promptTokens, _ = strconv.Atoi(row[5])
			generatedTokens, _ = strconv.Atoi(row[6])
			cachedTokens, _ = strconv.Atoi(row[7])
			contextUsed, _ = strconv.Atoi(row[8])
			cost, _ = strconv.ParseFloat(row[9], 64)
			durationMS, _ = strconv.ParseInt(row[10], 10, 64)
			promptMS, _ = strconv.ParseFloat(row[11], 64)
			generationMS, _ = strconv.ParseFloat(row[12], 64)
			toolCallsIdx, errorsIdx, startedAtIdx, completedAtIdx = 13, 14, 15, 16
		} else if len(row) >= 13 {
			// v2 format with generated_tokens and context_used
			generatedTokens, _ = strconv.Atoi(row[5])
			contextUsed, _ = strconv.Atoi(row[6])
			cost, _ = strconv.ParseFloat(row[7], 64)
			durationMS, _ = strconv.ParseInt(row[8], 10, 64)
			toolCallsIdx, errorsIdx, startedAtIdx, completedAtIdx = 9, 10, 11, 12
		} else {
			// v1 format without generated_tokens and context_used
			cost, _ = strconv.ParseFloat(row[5], 64)
			durationMS, _ = strconv.ParseInt(row[6], 10, 64)
			toolCallsIdx, errorsIdx, startedAtIdx, completedAtIdx = 7, 8, 9, 10
		}

		startedAt, _ := time.Parse(time.RFC3339, row[startedAtIdx])
		completedAt, _ := time.Parse(time.RFC3339, row[completedAtIdx])

		var toolCalls []ToolCallLog
		if row[toolCallsIdx] != "" && row[toolCallsIdx] != "null" {
			json.Unmarshal([]byte(row[toolCallsIdx]), &toolCalls)
		}

		var errors []string
		if row[errorsIdx] != "" && row[errorsIdx] != "null" {
			json.Unmarshal([]byte(row[errorsIdx]), &errors)
		}

		results = append(results, RunResult{
			BenchmarkID:     row[0],
			Run:             run,
			Success:         strings.ToLower(row[2]) == "true",
			LLMCalls:        llmCalls,
			Tokens:          tokens,
			PromptTokens:    promptTokens,
			GeneratedTokens: generatedTokens,
			CachedTokens:    cachedTokens,
			ContextUsed:     contextUsed,
			Cost:            cost,
			DurationMS:      durationMS,
			PromptMS:        promptMS,
			GenerationMS:    generationMS,
			ToolCalls:       toolCalls,
			Errors:          errors,
			StartedAt:       startedAt,
			CompletedAt:     completedAt,
		})
	}

	return results, nil
}

// ResumeInfo contains information about where to resume benchmark runs.
type ResumeInfo struct {
	CompletedRuns map[string]map[int]bool // map[benchmarkID]map[runNumber]completed
	FirstStarted  time.Time               // When benchmarking started
	LastCompleted time.Time               // Last completion time
}

// GetResumeInfo analyzes existing results to determine resume point.
func GetResumeInfo(results []RunResult) *ResumeInfo {
	info := &ResumeInfo{
		CompletedRuns: make(map[string]map[int]bool),
	}

	if len(results) == 0 {
		return info
	}

	for _, r := range results {
		if info.CompletedRuns[r.BenchmarkID] == nil {
			info.CompletedRuns[r.BenchmarkID] = make(map[int]bool)
		}
		info.CompletedRuns[r.BenchmarkID][r.Run] = true

		if info.FirstStarted.IsZero() || r.StartedAt.Before(info.FirstStarted) {
			info.FirstStarted = r.StartedAt
		}
		if r.CompletedAt.After(info.LastCompleted) {
			info.LastCompleted = r.CompletedAt
		}
	}

	return info
}

// IsCompleted checks if a specific benchmark run is already completed.
func (r *ResumeInfo) IsCompleted(benchmarkID string, run int) bool {
	if r.CompletedRuns[benchmarkID] == nil {
		return false
	}
	return r.CompletedRuns[benchmarkID][run]
}

// CountCompleted returns the total number of completed runs.
func (r *ResumeInfo) CountCompleted() int {
	count := 0
	for _, runs := range r.CompletedRuns {
		count += len(runs)
	}
	return count
}

// GetCSVPath returns the CSV path for a given output path.
// If output is benchmark-results.md, returns benchmark-results.csv
func GetCSVPath(outputPath string) string {
	ext := filepath.Ext(outputPath)
	if ext != "" {
		return strings.TrimSuffix(outputPath, ext) + ".csv"
	}
	return outputPath + ".csv"
}

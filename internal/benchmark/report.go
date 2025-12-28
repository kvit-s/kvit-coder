package benchmark

import (
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

// ReportGenerator generates benchmark reports.
type ReportGenerator struct {
	results    []RunResult
	benchmarks []BenchmarkDef
	version    string
	configYAML string
	numRuns    int // Number of runs per benchmark (the -n flag)
}

// NewReportGenerator creates a new report generator.
func NewReportGenerator(results []RunResult, benchmarks []BenchmarkDef, version, configYAML string, numRuns int) *ReportGenerator {
	return &ReportGenerator{
		results:    results,
		benchmarks: benchmarks,
		version:    version,
		configYAML: configYAML,
		numRuns:    numRuns,
	}
}

// GenerateMarkdown generates a markdown report.
func (g *ReportGenerator) GenerateMarkdown() string {
	var sb strings.Builder

	// Header
	sb.WriteString("# LLM Tool Usage Benchmark Report\n\n")

	// Metadata
	sb.WriteString("## Metadata\n\n")
	sb.WriteString(fmt.Sprintf("- **Version**: %s\n", g.version))
	sb.WriteString(fmt.Sprintf("- **Date**: %s\n", time.Now().Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("- **Total Benchmarks**: %d\n", len(g.benchmarks)))
	sb.WriteString(fmt.Sprintf("- **Total Runs**: %d\n", len(g.results)))
	sb.WriteString("\n")

	// Calculate aggregated stats for each benchmark
	statsMap := g.calculateAggregatedStats()

	// Sort by benchmark ID (natural sort to handle E1, E2, ..., E10 correctly)
	var ids []string
	for id := range statsMap {
		ids = append(ids, id)
	}
	sortNatural(ids)

	// Summary Table - grouped by benchmark class
	sb.WriteString("## Summary\n\n")
	sb.WriteString("| Class | Success Rate | Avg Time/Run |\n")
	sb.WriteString("|-------|--------------|--------------|")
	sb.WriteString("\n")

	// Group by class (first letter of benchmark ID)
	classStats := g.calculateClassStats(statsMap)
	var classes []string
	for class := range classStats {
		classes = append(classes, class)
	}
	sort.Strings(classes)

	var totalSuccesses, totalRuns int
	var totalDurationMS int64
	for _, class := range classes {
		cs := classStats[class]
		totalSuccesses += cs.Successes
		totalRuns += cs.TotalRuns
		totalDurationMS += cs.TotalDurationMS
		avgTimePerRun := 0.0
		if g.numRuns > 0 {
			avgTimePerRun = float64(cs.TotalDurationMS) / float64(g.numRuns) / 1000
		}
		sb.WriteString(fmt.Sprintf("| %s | %.0f%% (%d/%d) | %.1fs |\n",
			class,
			cs.SuccessRate*100,
			cs.Successes,
			cs.TotalRuns,
			avgTimePerRun,
		))
	}

	// Total row
	totalSuccessRate := 0.0
	totalAvgTimePerRun := 0.0
	if totalRuns > 0 {
		totalSuccessRate = float64(totalSuccesses) / float64(totalRuns)
	}
	if g.numRuns > 0 {
		totalAvgTimePerRun = float64(totalDurationMS) / float64(g.numRuns) / 1000
	}
	sb.WriteString(fmt.Sprintf("| **Total** | **%.0f%% (%d/%d)** | **%.1fs** |\n",
		totalSuccessRate*100,
		totalSuccesses,
		totalRuns,
		totalAvgTimePerRun,
	))
	sb.WriteString("\n")

	// Detailed Statistics
	sb.WriteString("## Detailed Statistics\n\n")

	// Per-benchmark summary table (with stddev in parentheses)
	sb.WriteString("### Per-Benchmark Summary\n\n")
	sb.WriteString("| Benchmark | Success | LLM Calls | Tokens | Generated | Context | Prompt Speed | Gen Speed | Cost | Duration |\n")
	sb.WriteString("|-----------|---------|-----------|--------|-----------|---------|--------------|-----------|------|----------|\n")

	for _, id := range ids {
		stats := statsMap[id]
		// Format values with stddev in parentheses (only show stddev if > 0)
		llmCalls := formatWithStdDev(stats.LLMCallsMean, stats.LLMCallsStdDev, "%.1f")
		tokens := formatWithStdDev(stats.TokensMean, stats.TokensStdDev, "%.0f")
		generated := formatWithStdDev(stats.GeneratedTokensMean, stats.GeneratedTokensStdDev, "%.0f")
		duration := formatDurationWithStdDev(stats.DurationMeanMS, stats.DurationStdDevMS)
		cost := formatCostWithStdDev(stats.CostMean, stats.CostStdDev)

		sb.WriteString(fmt.Sprintf("| %s | %.0f%% | %s | %s | %s | %.0f | %.1f t/s | %.1f t/s | %s | %s |\n",
			stats.BenchmarkID,
			stats.SuccessRate*100,
			llmCalls,
			tokens,
			generated,
			stats.ContextUsedMean,
			stats.PromptSpeed,
			stats.GenerationSpeed,
			cost,
			duration,
		))
	}
	sb.WriteString("\n")

	// Group results by benchmark ID for detail tables
	groupedResults := make(map[string][]RunResult)
	for _, r := range g.results {
		groupedResults[r.BenchmarkID] = append(groupedResults[r.BenchmarkID], r)
	}

	// Detailed breakdown for each benchmark
	sb.WriteString("### Per-Benchmark Details\n\n")

	for _, id := range ids {
		stats := statsMap[id]
		sb.WriteString(fmt.Sprintf("#### %s\n\n", id))

		// Find benchmark definition for name/goal
		var benchName, benchGoal string
		for _, b := range g.benchmarks {
			if b.ID == id {
				benchName = b.Name
				benchGoal = b.Goal
				break
			}
		}

		// Summary line at top (from Per-Benchmark Summary table)
		resultIcon := "✓"
		if stats.Failures > 0 {
			resultIcon = "✗"
		}
		sb.WriteString(fmt.Sprintf("**%s %s** | %s | %.0f%% (%d/%d) | %.1f calls | %.0f tokens | %.1fs\n\n",
			resultIcon, benchName, benchGoal,
			stats.SuccessRate*100, stats.Successes, stats.TotalRuns,
			stats.LLMCallsMean, stats.TokensMean, float64(stats.DurationMeanMS)/1000))

		// Individual runs table
		runs := groupedResults[id]
		if len(runs) > 0 {
			// Sort runs by run number
			sort.Slice(runs, func(i, j int) bool { return runs[i].Run < runs[j].Run })

			sb.WriteString("| Run | Result | Duration | LLM Calls | Tokens | Generated | Context | Cost | Tools |\n")
			sb.WriteString("|-----|--------|----------|-----------|--------|-----------|---------|------|-------|\n")

			for _, r := range runs {
				result := "✓ PASS"
				if !r.Success {
					result = "✗ FAIL"
				}

				// Format tool calls as compact list
				toolsSummary := formatToolsSummary(r.ToolCalls)

				sb.WriteString(fmt.Sprintf("| %d | %s | %.1fs | %d | %d | %d | %d | $%.4f | %s |\n",
					r.Run,
					result,
					float64(r.DurationMS)/1000,
					r.LLMCalls,
					r.Tokens,
					r.GeneratedTokens,
					r.ContextUsed,
					r.Cost,
					toolsSummary,
				))
			}
			sb.WriteString("\n")

			// Show failure details with full error messages
			var failures []RunResult
			for _, r := range runs {
				if !r.Success {
					failures = append(failures, r)
				}
			}

			if len(failures) > 0 {
				sb.WriteString("**Failures:**\n\n")
				for _, f := range failures {
					sb.WriteString(fmt.Sprintf("**Run %d**: ", f.Run))
					for i, err := range f.Errors {
						if i > 0 {
							sb.WriteString("\n")
						}
						// Check if error contains a diff (has --- expected or +++ actual)
						if strings.Contains(err, "--- expected") || strings.Contains(err, "+++ actual") {
							sb.WriteString("\n```diff\n")
							sb.WriteString(err)
							sb.WriteString("\n```\n")
						} else {
							sb.WriteString(err)
						}
					}
					sb.WriteString("\n\n")
				}
			}
		}

		sb.WriteString("---\n\n")
	}

	// Failure Analysis
	failures := g.getFailures()
	if len(failures) > 0 {
		sb.WriteString("## Failure Analysis\n\n")
		sb.WriteString("| Benchmark | Run | Errors | Last Tool Call |\n")
		sb.WriteString("|-----------|-----|--------|----------------|\n")

		for _, f := range failures {
			errorsStr := strings.Join(f.Errors, "; ")
			if len(errorsStr) > 50 {
				errorsStr = errorsStr[:47] + "..."
			}
			sb.WriteString(fmt.Sprintf("| %s | %d | %s | %s |\n",
				f.BenchmarkID, f.Run, errorsStr, f.LastToolCall))
		}
		sb.WriteString("\n")
	}

	// Appendix: Configuration
	sb.WriteString("## Appendix A: Configuration\n\n")
	sb.WriteString("### Version\n\n")
	sb.WriteString(fmt.Sprintf("```\n%s\n```\n\n", g.version))
	sb.WriteString("### config.yaml\n\n")
	sb.WriteString(fmt.Sprintf("```yaml\n%s\n```\n\n", g.configYAML))

	return sb.String()
}

// WriteMarkdown writes the markdown report to a file.
func (g *ReportGenerator) WriteMarkdown(path string) error {
	content := g.GenerateMarkdown()
	return os.WriteFile(path, []byte(content), 0644)
}

// calculateAggregatedStats calculates aggregated statistics for all benchmarks.
func (g *ReportGenerator) calculateAggregatedStats() map[string]*AggregatedStats {
	// Group results by benchmark ID
	grouped := make(map[string][]RunResult)
	for _, r := range g.results {
		grouped[r.BenchmarkID] = append(grouped[r.BenchmarkID], r)
	}

	// Calculate stats for each benchmark
	statsMap := make(map[string]*AggregatedStats)
	for id, results := range grouped {
		statsMap[id] = calculateStats(id, results)
	}

	return statsMap
}

// calculateStats calculates aggregated statistics for a single benchmark.
func calculateStats(id string, results []RunResult) *AggregatedStats {
	if len(results) == 0 {
		return &AggregatedStats{BenchmarkID: id}
	}

	stats := &AggregatedStats{
		BenchmarkID:    id,
		TotalRuns:      len(results),
		ToolCallCounts: make(map[string]int),
		ErrorCounts:    make(map[string]int),
	}

	var llmCalls, tokens, generatedTokens, contextUsed []int
	var costs []float64
	var durations []int64

	// For weighted speed calculations
	var totalProcessedTokens, totalGeneratedTokens int
	var totalPromptMS, totalGenerationMS float64

	for _, r := range results {
		if r.Success {
			stats.Successes++
		} else {
			stats.Failures++
			for _, e := range r.Errors {
				stats.ErrorCounts[e]++
			}
		}

		llmCalls = append(llmCalls, r.LLMCalls)
		tokens = append(tokens, r.Tokens)
		generatedTokens = append(generatedTokens, r.GeneratedTokens)
		contextUsed = append(contextUsed, r.ContextUsed)
		costs = append(costs, r.Cost)
		durations = append(durations, r.DurationMS)
		stats.CostTotal += r.Cost

		// Accumulate for speed calculations
		processedTokens := r.PromptTokens - r.CachedTokens
		if processedTokens < 0 {
			processedTokens = 0
		}
		totalProcessedTokens += processedTokens
		totalGeneratedTokens += r.GeneratedTokens
		totalPromptMS += r.PromptMS
		totalGenerationMS += r.GenerationMS

		for _, tc := range r.ToolCalls {
			stats.ToolCallCounts[tc.Tool]++
		}
	}

	stats.SuccessRate = float64(stats.Successes) / float64(stats.TotalRuns)

	// LLM Calls stats
	sort.Ints(llmCalls)
	stats.LLMCallsMin = llmCalls[0]
	stats.LLMCallsMax = llmCalls[len(llmCalls)-1]
	stats.LLMCallsMean = mean(llmCalls)
	stats.LLMCallsMedian = median(llmCalls)
	stats.LLMCallsP5 = percentile(llmCalls, 5)
	stats.LLMCallsP95 = percentile(llmCalls, 95)
	stats.LLMCallsStdDev = stddev(llmCalls)

	// Tokens stats
	sort.Ints(tokens)
	stats.TokensMin = tokens[0]
	stats.TokensMax = tokens[len(tokens)-1]
	stats.TokensMean = mean(tokens)
	stats.TokensMedian = median(tokens)
	stats.TokensP5 = percentile(tokens, 5)
	stats.TokensP95 = percentile(tokens, 95)
	stats.TokensStdDev = stddev(tokens)

	// Generated Tokens stats
	sort.Ints(generatedTokens)
	stats.GeneratedTokensMin = generatedTokens[0]
	stats.GeneratedTokensMax = generatedTokens[len(generatedTokens)-1]
	stats.GeneratedTokensMean = mean(generatedTokens)
	stats.GeneratedTokensMedian = median(generatedTokens)
	stats.GeneratedTokensP5 = percentile(generatedTokens, 5)
	stats.GeneratedTokensP95 = percentile(generatedTokens, 95)
	stats.GeneratedTokensStdDev = stddev(generatedTokens)

	// Context Used stats
	sort.Ints(contextUsed)
	stats.ContextUsedMin = contextUsed[0]
	stats.ContextUsedMax = contextUsed[len(contextUsed)-1]
	stats.ContextUsedMean = mean(contextUsed)
	stats.ContextUsedMedian = median(contextUsed)
	stats.ContextUsedP5 = percentile(contextUsed, 5)
	stats.ContextUsedP95 = percentile(contextUsed, 95)

	// Processed tokens and speed stats
	stats.ProcessedTokensMean = float64(totalProcessedTokens) / float64(len(results))
	if totalPromptMS > 0 {
		stats.PromptSpeed = float64(totalProcessedTokens) / totalPromptMS * 1000 // tokens/sec
	}
	if totalGenerationMS > 0 {
		stats.GenerationSpeed = float64(totalGeneratedTokens) / totalGenerationMS * 1000 // tokens/sec
	}

	// Cost stats
	sort.Float64s(costs)
	stats.CostMin = costs[0]
	stats.CostMax = costs[len(costs)-1]
	stats.CostMean = meanFloat(costs)
	stats.CostStdDev = stddevFloat(costs)

	// Duration stats
	sort.Slice(durations, func(i, j int) bool { return durations[i] < durations[j] })
	stats.DurationMinMS = durations[0]
	stats.DurationMaxMS = durations[len(durations)-1]
	stats.DurationMeanMS = meanInt64(durations)
	stats.DurationMedianMS = medianInt64(durations)
	stats.DurationP5MS = percentileInt64(durations, 5)
	stats.DurationP95MS = percentileInt64(durations, 95)
	stats.DurationStdDevMS = stddevInt64(durations)

	return stats
}

// getFailures returns details about failed runs.
func (g *ReportGenerator) getFailures() []FailureDetail {
	var failures []FailureDetail

	for _, r := range g.results {
		if !r.Success {
			lastTool := ""
			if len(r.ToolCalls) > 0 {
				lastTool = r.ToolCalls[len(r.ToolCalls)-1].Tool
			}

			failures = append(failures, FailureDetail{
				BenchmarkID:  r.BenchmarkID,
				Run:          r.Run,
				Errors:       r.Errors,
				LastToolCall: lastTool,
			})
		}
	}

	return failures
}

// formatWithStdDev formats a mean value with stddev in parentheses
func formatWithStdDev(mean, stddev float64, format string) string {
	if stddev == 0 {
		return fmt.Sprintf(format, mean)
	}
	return fmt.Sprintf(format+"(±"+format+")", mean, stddev)
}

// formatDurationWithStdDev formats duration in seconds with stddev
func formatDurationWithStdDev(meanMS, stddevMS float64) string {
	mean := meanMS / 1000
	stddev := stddevMS / 1000
	if stddev == 0 {
		return fmt.Sprintf("%.1fs", mean)
	}
	return fmt.Sprintf("%.1fs(±%.1f)", mean, stddev)
}

// formatCostWithStdDev formats cost with stddev
func formatCostWithStdDev(mean, stddev float64) string {
	if stddev == 0 {
		return fmt.Sprintf("$%.4f", mean)
	}
	return fmt.Sprintf("$%.4f(±%.4f)", mean, stddev)
}

// formatToolsSummary creates a compact summary of tool calls for a run
func formatToolsSummary(calls []ToolCallLog) string {
	if len(calls) == 0 {
		return "-"
	}

	// Count occurrences of each tool
	counts := make(map[string]int)
	for _, tc := range calls {
		counts[tc.Tool]++
	}

	// Sort tools alphabetically
	var tools []string
	for tool := range counts {
		tools = append(tools, tool)
	}
	sort.Strings(tools)

	// Format as "Tool:N, Tool2:M" or just "Tool, Tool2" if count is 1
	var parts []string
	for _, tool := range tools {
		if counts[tool] > 1 {
			parts = append(parts, fmt.Sprintf("%s×%d", tool, counts[tool]))
		} else {
			parts = append(parts, tool)
		}
	}

	result := strings.Join(parts, ", ")
	// Truncate if too long for table
	if len(result) > 40 {
		result = result[:37] + "..."
	}
	return result
}

// Helper functions for statistics

func mean(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

func meanFloat(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range values {
		sum += v
	}
	return sum / float64(len(values))
}

func meanInt64(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	var sum int64
	for _, v := range values {
		sum += v
	}
	return float64(sum) / float64(len(values))
}

func median(values []int) float64 {
	if len(values) == 0 {
		return 0
	}
	n := len(values)
	if n%2 == 0 {
		return float64(values[n/2-1]+values[n/2]) / 2
	}
	return float64(values[n/2])
}

func medianInt64(values []int64) float64 {
	if len(values) == 0 {
		return 0
	}
	n := len(values)
	if n%2 == 0 {
		return float64(values[n/2-1]+values[n/2]) / 2
	}
	return float64(values[n/2])
}

func percentile(values []int, p int) float64 {
	if len(values) == 0 {
		return 0
	}
	index := (p * len(values)) / 100
	if index >= len(values) {
		index = len(values) - 1
	}
	return float64(values[index])
}

func percentileInt64(values []int64, p int) float64 {
	if len(values) == 0 {
		return 0
	}
	index := (p * len(values)) / 100
	if index >= len(values) {
		index = len(values) - 1
	}
	return float64(values[index])
}

func stddev(values []int) float64 {
	if len(values) < 2 {
		return 0
	}
	m := mean(values)
	var sumSquares float64
	for _, v := range values {
		diff := float64(v) - m
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}

func stddevFloat(values []float64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := meanFloat(values)
	var sumSquares float64
	for _, v := range values {
		diff := v - m
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}

func stddevInt64(values []int64) float64 {
	if len(values) < 2 {
		return 0
	}
	m := meanInt64(values)
	var sumSquares float64
	for _, v := range values {
		diff := float64(v) - m
		sumSquares += diff * diff
	}
	return math.Sqrt(sumSquares / float64(len(values)))
}

// ClassStats holds aggregated statistics for a benchmark class (e.g., C, E, R, S, W)
type ClassStats struct {
	Class           string
	TotalRuns       int
	Successes       int
	Failures        int
	SuccessRate     float64
	TotalDurationMS int64
}

// sortNatural sorts strings naturally (E1, E2, ..., E10 instead of E1, E10, E2)
func sortNatural(ids []string) {
	re := regexp.MustCompile(`^([A-Za-z]+)(\d+)$`)
	sort.Slice(ids, func(i, j int) bool {
		matchI := re.FindStringSubmatch(ids[i])
		matchJ := re.FindStringSubmatch(ids[j])

		// If both match the pattern (prefix + number)
		if matchI != nil && matchJ != nil {
			// Compare prefixes first
			if matchI[1] != matchJ[1] {
				return matchI[1] < matchJ[1]
			}
			// Same prefix, compare numbers
			numI, _ := strconv.Atoi(matchI[2])
			numJ, _ := strconv.Atoi(matchJ[2])
			return numI < numJ
		}
		// Fall back to string comparison
		return ids[i] < ids[j]
	})
}

// calculateClassStats groups benchmark stats by class (first letter of ID)
func (g *ReportGenerator) calculateClassStats(statsMap map[string]*AggregatedStats) map[string]*ClassStats {
	classMap := make(map[string]*ClassStats)

	for id, stats := range statsMap {
		// Get class from first character of benchmark ID
		class := string(id[0])

		if _, exists := classMap[class]; !exists {
			classMap[class] = &ClassStats{Class: class}
		}

		cs := classMap[class]
		cs.TotalRuns += stats.TotalRuns
		cs.Successes += stats.Successes
		cs.Failures += stats.Failures
		// Sum total duration (mean * runs gives approximate total for this benchmark)
		cs.TotalDurationMS += int64(stats.DurationMeanMS * float64(stats.TotalRuns))
	}

	// Calculate success rates
	for _, cs := range classMap {
		if cs.TotalRuns > 0 {
			cs.SuccessRate = float64(cs.Successes) / float64(cs.TotalRuns)
		}
	}

	return classMap
}

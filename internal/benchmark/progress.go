package benchmark

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// Progress tracks and displays benchmark progress.
type Progress struct {
	writer       io.Writer
	totalRuns    int
	completedRuns int
	passedRuns   int
	startTime    time.Time
	currentBenchmark string
	currentRun   int
	runsPerBenchmark int
	durations    []time.Duration // Track recent run durations for ETA
}

// NewProgress creates a new progress tracker.
func NewProgress(writer io.Writer, totalBenchmarks, runsPerBenchmark int) *Progress {
	return &Progress{
		writer:           writer,
		totalRuns:        totalBenchmarks * runsPerBenchmark,
		completedRuns:    0,
		startTime:        time.Now(),
		runsPerBenchmark: runsPerBenchmark,
		durations:        make([]time.Duration, 0, 100),
	}
}

// SetResumePoint sets the starting point when resuming.
func (p *Progress) SetResumePoint(completedRuns int, originalStartTime time.Time) {
	p.completedRuns = completedRuns
	if !originalStartTime.IsZero() {
		p.startTime = originalStartTime
	}
}

// StartRun marks the start of a new benchmark run.
func (p *Progress) StartRun(benchmarkID string, run int) {
	p.currentBenchmark = benchmarkID
	p.currentRun = run
	p.Display()
}

// CompleteRun marks a run as complete and shows pass/fail status with statistics.
func (p *Progress) CompleteRun(duration time.Duration, result *RunResult) {
	p.completedRuns++
	if result != nil && result.Success {
		p.passedRuns++
	}
	p.durations = append(p.durations, duration)
	// Keep only last 50 durations for ETA calculation
	if len(p.durations) > 50 {
		p.durations = p.durations[len(p.durations)-50:]
	}

	// Print pass/fail status with statistics for the completed run
	if result == nil || !result.Success {
		status := "✗ FAIL"
		if result == nil {
			fmt.Fprintf(p.writer, "\r[%s run %d/%d] %s (%s)                                              \n",
				p.currentBenchmark,
				p.currentRun,
				p.runsPerBenchmark,
				status,
				formatDuration(duration),
			)
		} else {
			fmt.Fprintf(p.writer, "\r[%s run %d/%d] %s | %s | turns: %d | tokens: %d\n",
				p.currentBenchmark,
				p.currentRun,
				p.runsPerBenchmark,
				status,
				formatDuration(duration),
				result.LLMCalls,
				result.Tokens,
			)
			// Print failure reasons
			for _, err := range result.Errors {
				fmt.Fprintf(p.writer, "    → %s\n", err)
			}
		}
	} else {
		fmt.Fprintf(p.writer, "\r[%s run %d/%d] ✓ PASS | %s | turns: %d | tokens: %d\n",
			p.currentBenchmark,
			p.currentRun,
			p.runsPerBenchmark,
			formatDuration(duration),
			result.LLMCalls,
			result.Tokens,
		)
	}

	p.Display()
}

// Display shows the current progress.
func (p *Progress) Display() {
	// Format: [S3 run 2/10] ████░░░░░░ 12% (42/350) | Pass: 85% | Elapsed: 5m 30s | ETA: 40m 15s

	percent := 0
	if p.totalRuns > 0 {
		percent = (p.completedRuns * 100) / p.totalRuns
	}

	// Calculate pass rate
	passRate := 0.0
	if p.completedRuns > 0 {
		passRate = float64(p.passedRuns) * 100 / float64(p.completedRuns)
	}

	// Progress bar (10 characters wide)
	filled := percent / 10
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 10-filled)

	elapsed := time.Since(p.startTime)
	eta := p.calculateETA()

	// Clear line and write progress
	fmt.Fprintf(p.writer, "\r[%s run %d/%d] %s %d%% (%d/%d) | Pass: %.0f%% | Elapsed: %s | ETA: %s    ",
		p.currentBenchmark,
		p.currentRun,
		p.runsPerBenchmark,
		bar,
		percent,
		p.completedRuns,
		p.totalRuns,
		passRate,
		formatDuration(elapsed),
		formatDuration(eta),
	)
}

// calculateETA calculates estimated time remaining.
func (p *Progress) calculateETA() time.Duration {
	if p.completedRuns == 0 || len(p.durations) == 0 {
		return 0
	}

	// Calculate average duration from recent runs
	var total time.Duration
	for _, d := range p.durations {
		total += d
	}
	avg := total / time.Duration(len(p.durations))

	remaining := p.totalRuns - p.completedRuns
	return avg * time.Duration(remaining)
}

// Finish displays the final progress state.
func (p *Progress) Finish() {
	elapsed := time.Since(p.startTime)
	fmt.Fprintf(p.writer, "\r[Complete] ██████████ 100%% (%d/%d) | Total time: %s              \n",
		p.completedRuns,
		p.totalRuns,
		formatDuration(elapsed),
	)
}

// PrintSummary prints a summary after all benchmarks complete.
func (p *Progress) PrintSummary(results []RunResult) {
	successes := 0
	failures := 0
	var totalCost float64
	var totalDuration time.Duration

	for _, r := range results {
		if r.Success {
			successes++
		} else {
			failures++
		}
		totalCost += r.Cost
		totalDuration += time.Duration(r.DurationMS) * time.Millisecond
	}

	fmt.Fprintf(p.writer, "\n=== Benchmark Summary ===\n")
	fmt.Fprintf(p.writer, "Total runs:     %d\n", len(results))
	fmt.Fprintf(p.writer, "Successes:      %d (%.1f%%)\n", successes, float64(successes)*100/float64(len(results)))
	fmt.Fprintf(p.writer, "Failures:       %d (%.1f%%)\n", failures, float64(failures)*100/float64(len(results)))
	fmt.Fprintf(p.writer, "Total cost:     $%.4f\n", totalCost)
	fmt.Fprintf(p.writer, "Total duration: %s\n", formatDuration(totalDuration))
}

// formatDuration formats a duration in a human-readable way.
func formatDuration(d time.Duration) string {
	if d <= 0 {
		return "--"
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	}
	if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
}

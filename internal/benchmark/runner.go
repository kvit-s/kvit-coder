package benchmark

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Runner orchestrates the execution of all benchmarks.
type Runner struct {
	executor   *Executor
	env        *Environment
	config     *BenchmarkConfig
	benchmarks []BenchmarkDef
	writer     io.Writer
	csvPath    string
}

// NewRunner creates a new benchmark runner.
func NewRunner(executor *Executor, env *Environment, config *BenchmarkConfig, benchmarks []BenchmarkDef, writer io.Writer, csvPath string) *Runner {
	return &Runner{
		executor:   executor,
		env:        env,
		config:     config,
		benchmarks: benchmarks,
		writer:     writer,
		csvPath:    csvPath,
	}
}

// RunAll executes all benchmarks according to configuration.
// Press Esc to skip current benchmark, Ctrl+C to exit.
func (r *Runner) RunAll(ctx context.Context) ([]RunResult, error) {
	// Initialize environment
	if err := r.env.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize environment: %w", err)
	}

	// Check for resume
	var existingResults []RunResult
	var resumeInfo *ResumeInfo

	if !r.config.NoResume {
		var err error
		existingResults, err = LoadResults(r.csvPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing results: %w", err)
		}
		resumeInfo = GetResumeInfo(existingResults)

		if resumeInfo.CountCompleted() > 0 {
			fmt.Fprintf(r.writer, "Resuming from %d completed runs\n", resumeInfo.CountCompleted())
		}
	} else {
		resumeInfo = &ResumeInfo{CompletedRuns: make(map[string]map[int]bool)}
	}

	// Open CSV writer
	csvWriter, err := NewCSVWriter(r.csvPath, !r.config.NoResume)
	if err != nil {
		return nil, fmt.Errorf("failed to create CSV writer: %w", err)
	}
	defer csvWriter.Close()

	// Initialize progress tracker
	totalBenchmarks := len(r.benchmarks)
	progress := NewProgress(r.writer, totalBenchmarks, r.config.RunsPerTask)
	if resumeInfo != nil && resumeInfo.CountCompleted() > 0 {
		progress.SetResumePoint(resumeInfo.CountCompleted(), resumeInfo.FirstStarted)
	}

	// Run warmup if this is a fresh start
	if resumeInfo.CountCompleted() == 0 && r.config.WarmupTask != "" {
		fmt.Fprintf(r.writer, "Running warmup task...\n")
		if err := r.executor.Warmup(ctx, r.config.WarmupTask); err != nil {
			fmt.Fprintf(r.writer, "Warmup failed: %v (continuing anyway)\n", err)
		}
		fmt.Fprintf(r.writer, "Warmup complete\n\n")
	}

	// Collect all results (existing + new)
	allResults := make([]RunResult, 0, len(existingResults)+totalBenchmarks*r.config.RunsPerTask)
	allResults = append(allResults, existingResults...)

	// Run benchmarks
	for _, benchmark := range r.benchmarks {
		for run := 1; run <= r.config.RunsPerTask; run++ {
			// Check if cancelled
			select {
			case <-ctx.Done():
				fmt.Fprintf(r.writer, "\nBenchmark run cancelled\n")
				progress.Finish()
				return allResults, nil
			default:
			}

			// Skip if already completed
			if resumeInfo.IsCompleted(benchmark.ID, run) {
				continue
			}

			// Update progress
			progress.StartRun(benchmark.ID, run)

			// Execute benchmark
			runStart := time.Now()
			result, err := r.executor.Execute(ctx, benchmark, run)
			runDuration := time.Since(runStart)

			if err != nil {
				// Create error result
				errorResult := &RunResult{
					BenchmarkID: benchmark.ID,
					Run:         run,
					Success:     false,
					Errors:      []string{err.Error()},
					StartedAt:   runStart,
					CompletedAt: time.Now(),
					DurationMS:  runDuration.Milliseconds(),
				}
				allResults = append(allResults, *errorResult)

				// Write to CSV
				if csvErr := csvWriter.WriteResult(errorResult); csvErr != nil {
					fmt.Fprintf(r.writer, "\nWarning: failed to write result to CSV: %v\n", csvErr)
				}
			} else {
				allResults = append(allResults, *result.RunResult)

				// Write to CSV
				if csvErr := csvWriter.WriteResult(result.RunResult); csvErr != nil {
					fmt.Fprintf(r.writer, "\nWarning: failed to write result to CSV: %v\n", csvErr)
				}

				// Cleanup workspace if successful (keep on failure for debugging)
				if result.RunResult.Success {
					_ = r.env.CleanupBenchmark(benchmark, run)
				}
			}

			// Update progress with pass/fail status and statistics
			var runResult *RunResult
			if err != nil {
				runResult = &allResults[len(allResults)-1] // Get the error result we just added
			} else {
				runResult = result.RunResult
			}
			progress.CompleteRun(runDuration, runResult)
		}
	}

	progress.Finish()
	progress.PrintSummary(allResults)

	return allResults, nil
}

// RunSingle executes a single benchmark for debugging.
func (r *Runner) RunSingle(ctx context.Context, benchmarkID string) (*ExecuteResult, error) {
	// Find the benchmark
	var benchmark *BenchmarkDef
	for _, b := range r.benchmarks {
		if b.ID == benchmarkID {
			benchmark = &b
			break
		}
	}

	if benchmark == nil {
		return nil, fmt.Errorf("benchmark %s not found", benchmarkID)
	}

	// Initialize environment
	if err := r.env.Initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize environment: %w", err)
	}

	// Execute single run
	return r.executor.Execute(ctx, *benchmark, 1)
}

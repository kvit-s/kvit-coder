package benchmark

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/config"
)

// restoreTerminal resets terminal to sane state
func restoreTerminal() {
	cmd := exec.Command("stty", "sane")
	cmd.Stdin = os.Stdin
	_ = cmd.Run()
}

// CLIFlags holds the command-line flags for benchmarks.
type CLIFlags struct {
	Enabled     bool
	Runs        int
	Category    string
	BenchmarkID string
	OutputFile  string
	NoResume    bool
	Suffix      string
}

// Run executes the benchmark CLI with the given configuration.
// originalWorkspaceRoot is the workspace before benchmark override (for finding benchmarks.yaml)
func Run(ctx context.Context, flags CLIFlags, runner *agent.Runner, cfg *config.Config, systemPrompt string, version string, originalWorkspaceRoot string) error {
	// Ensure terminal is restored on exit (including Ctrl+C)
	defer restoreTerminal()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		restoreTerminal()
		os.Exit(1)
	}()

	// Determine base directory (use original workspace, not the overridden one)
	baseDir := filepath.Join(originalWorkspaceRoot, ".kvit-coder-benchmark")

	// Find benchmarks.yaml file (use original workspace)
	benchmarksFile := FindBenchmarksFile(originalWorkspaceRoot)
	if benchmarksFile == "" {
		return fmt.Errorf("benchmarks.yaml not found in workspace")
	}

	// Load benchmark definitions
	benchmarks, err := LoadBenchmarks(benchmarksFile)
	if err != nil {
		return fmt.Errorf("failed to load benchmarks: %w", err)
	}

	fmt.Printf("Loaded %d benchmarks from %s\n", len(benchmarks), benchmarksFile)

	// Filter by category/ID
	var categories, ids []string
	if flags.Category != "" {
		categories = strings.Split(flags.Category, ",")
	}
	if flags.BenchmarkID != "" {
		ids = strings.Split(flags.BenchmarkID, ",")
	}

	benchmarks = FilterBenchmarks(benchmarks, categories, ids)
	if len(benchmarks) == 0 {
		return fmt.Errorf("no benchmarks match the specified filters")
	}

	fmt.Printf("Running %d benchmarks with %d runs each\n", len(benchmarks), flags.Runs)

	// Indicate if using external command mode
	if cfg.LLM.BenchmarkCmd != "" {
		fmt.Printf("Using external command: %s\n", cfg.LLM.BenchmarkCmd)
	}

	// Determine output path - report goes in same directory as config.yaml
	outputPath := flags.OutputFile
	timestamp := time.Now().Format("20060102-150405")
	if outputPath == "" {
		if flags.Suffix != "" {
			outputPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("benchmark-%s-%s.md", flags.Suffix, timestamp))
		} else {
			outputPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("benchmark-%s.md", timestamp))
		}
	}

	// CSV goes in .kvit-coder-benchmark directory
	csvPath := filepath.Join(baseDir, fmt.Sprintf("benchmark-%s.csv", timestamp))

	// Ensure output directories exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create benchmark directory: %w", err)
	}

	// Create benchmark configuration
	benchConfig := &BenchmarkConfig{
		Enabled:       true,
		OutputDir:     baseDir,
		RunsPerTask:   flags.Runs,
		TimeoutPerRun: 120,
		ReportFormat:  "markdown",
		WarmupTask:    "Say hello",
		NoResume:      flags.NoResume,
	}

	// Create environment - use cfg.Workspace.Root which is set per --benchmark suffix
	env := NewEnvironment(baseDir)
	env.WorkspaceDir = cfg.Workspace.Root

	// Create executor
	executor := NewExecutor(runner, cfg, systemPrompt, env, time.Duration(benchConfig.TimeoutPerRun)*time.Second)

	// Create runner
	benchRunner := NewRunner(executor, env, benchConfig, benchmarks, os.Stdout, csvPath)

	// Run benchmarks
	results, err := benchRunner.RunAll(ctx)
	if err != nil {
		return fmt.Errorf("benchmark run failed: %w", err)
	}

	// Load config.yaml for report (use original workspace)
	configYAML := ""
	configPath := filepath.Join(originalWorkspaceRoot, "config.yaml")
	if data, err := os.ReadFile(configPath); err == nil {
		configYAML = string(data)
	}

	// Generate report
	generator := NewReportGenerator(results, benchmarks, version, configYAML, flags.Runs)
	if err := generator.WriteMarkdown(outputPath); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	fmt.Printf("\nReport written to: %s\n", outputPath)
	fmt.Printf("CSV data saved to: %s\n", csvPath)

	return nil
}

// ListBenchmarks prints available benchmarks.
func ListBenchmarks(cfg *config.Config) error {
	// Find benchmarks.yaml file
	benchmarksFile := FindBenchmarksFile(cfg.Workspace.Root)
	if benchmarksFile == "" {
		return fmt.Errorf("benchmarks.yaml not found in workspace")
	}

	// Load benchmark definitions
	benchmarks, err := LoadBenchmarks(benchmarksFile)
	if err != nil {
		return fmt.Errorf("failed to load benchmarks: %w", err)
	}

	// Group by category
	byCategory := GetBenchmarksByCategory(benchmarks)

	fmt.Printf("Available benchmarks (%d total):\n\n", len(benchmarks))

	// Sort categories
	categories := ListCategories(benchmarks)
	for _, cat := range categories {
		fmt.Printf("## %s\n", strings.ToUpper(cat))
		for _, b := range byCategory[cat] {
			name := b.Name
			if name == "" {
				name = b.Goal
			}
			fmt.Printf("  %s - %s\n", b.ID, name)
		}
		fmt.Println()
	}

	return nil
}

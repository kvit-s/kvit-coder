package benchmark

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
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

	// Determine output paths early so we can create the terminal log file
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

	// Terminal output file - same directory and naming pattern as markdown report
	var terminalPath string
	if flags.Suffix != "" {
		terminalPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("terminal-%s-%s.txt", flags.Suffix, timestamp))
	} else {
		terminalPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("terminal-%s.txt", timestamp))
	}

	// Ensure output directories exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create benchmark directory: %w", err)
	}

	// Create terminal output file and MultiWriter
	terminalFile, err := os.Create(terminalPath)
	if err != nil {
		return fmt.Errorf("failed to create terminal output file: %w", err)
	}
	defer terminalFile.Close()

	// Write to both terminal streams and the file
	stdoutWriter := io.MultiWriter(os.Stdout, terminalFile)
	stderrWriter := io.MultiWriter(os.Stderr, terminalFile)

	// Configure the runner's writer to capture output to terminal file
	// Use non-headless mode to preserve colors, but route color output through MultiWriter
	writer := runner.Writer()
	writer.SetHeadless(false)
	writer.SetQuiet(false)
	writer.SetStdout(stdoutWriter)
	writer.SetStderr(stderrWriter)
	writer.SetColorOutput(stdoutWriter) // Route colored output to terminal file too

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

	fmt.Fprintf(stdoutWriter, "Loaded %d benchmarks from %s\n", len(benchmarks), benchmarksFile)

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

	fmt.Fprintf(stdoutWriter, "Running %d benchmarks with %d runs each\n", len(benchmarks), flags.Runs)

	// Indicate if using external command mode
	if cfg.LLM.BenchmarkCmd != "" {
		fmt.Fprintf(stdoutWriter, "Using external command: %s\n", cfg.LLM.BenchmarkCmd)
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
	executor := NewExecutor(runner, cfg, systemPrompt, env, time.Duration(benchConfig.TimeoutPerRun)*time.Second, stdoutWriter, stderrWriter)

	// Create runner
	benchRunner := NewRunner(executor, env, benchConfig, benchmarks, stdoutWriter, csvPath)

	// Run benchmarks
	results, err := benchRunner.RunAll(ctx)
	if err != nil {
		return fmt.Errorf("benchmark run failed: %w", err)
	}

	// Load config file for report (use original workspace)
	// If benchmark suffix is set, use config-{suffix}.yaml, otherwise config.yaml
	configYAML := ""
	configFileName := "config.yaml"
	if flags.Suffix != "" {
		configFileName = fmt.Sprintf("config-%s.yaml", flags.Suffix)
	}
	configPath := filepath.Join(originalWorkspaceRoot, configFileName)
	if data, err := os.ReadFile(configPath); err == nil {
		configYAML = string(data)
	}

	// Generate report
	generator := NewReportGenerator(results, benchmarks, version, configYAML, flags.Runs)
	if err := generator.WriteMarkdown(outputPath); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	fmt.Fprintf(stdoutWriter, "\nReport written to: %s\n", outputPath)
	fmt.Fprintf(stdoutWriter, "CSV data saved to: %s\n", csvPath)
	fmt.Fprintf(stdoutWriter, "Terminal log saved to: %s\n", terminalPath)

	return nil
}

// HaystackCLIFlags holds the command-line flags for haystack benchmarks.
type HaystackCLIFlags struct {
	Enabled     bool
	Runs        int
	BenchmarkID string
	OutputFile  string
	NoResume    bool
	Suffix      string
}

// RunHaystack executes haystack (needle retrieval) benchmarks.
// These benchmarks inject large context into the prompt and test retrieval without tools.
func RunHaystack(ctx context.Context, flags HaystackCLIFlags, cfg *config.Config, version string, originalWorkspaceRoot string) error {
	// Ensure terminal is restored on exit
	defer restoreTerminal()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		restoreTerminal()
		os.Exit(1)
	}()

	// Determine base directory
	baseDir := filepath.Join(originalWorkspaceRoot, ".kvit-coder-benchmark")

	// Determine output paths
	timestamp := time.Now().Format("20060102-150405")
	outputPath := flags.OutputFile
	if outputPath == "" {
		if flags.Suffix != "" {
			outputPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("haystack-%s-%s.md", flags.Suffix, timestamp))
		} else {
			outputPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("haystack-%s.md", timestamp))
		}
	}

	csvPath := filepath.Join(baseDir, fmt.Sprintf("haystack-%s.csv", timestamp))

	// Terminal output file - don't include haystack content
	var terminalPath string
	if flags.Suffix != "" {
		terminalPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("terminal-haystack-%s-%s.txt", flags.Suffix, timestamp))
	} else {
		terminalPath = filepath.Join(originalWorkspaceRoot, fmt.Sprintf("terminal-haystack-%s.txt", timestamp))
	}

	// Ensure directories exist
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		return fmt.Errorf("failed to create benchmark directory: %w", err)
	}

	// Create terminal output file
	terminalFile, err := os.Create(terminalPath)
	if err != nil {
		return fmt.Errorf("failed to create terminal output file: %w", err)
	}
	defer terminalFile.Close()

	// Write to both terminal and file (but we'll be careful what we write)
	stdoutWriter := io.MultiWriter(os.Stdout, terminalFile)

	// Find haystack.yaml
	haystackFile := FindHaystackFile(originalWorkspaceRoot)
	if haystackFile == "" {
		return fmt.Errorf("haystack.yaml not found in workspace")
	}

	// Load haystack benchmarks
	haystackData, err := LoadHaystackBenchmarks(haystackFile)
	if err != nil {
		return fmt.Errorf("failed to load haystack benchmarks: %w", err)
	}

	fmt.Fprintf(stdoutWriter, "Loaded %d haystack benchmarks from %s\n", len(haystackData.Benchmarks), haystackFile)
	fmt.Fprintf(stdoutWriter, "Haystacks: %d defined\n", len(haystackData.Haystacks))

	// Filter by ID if specified
	benchmarks := haystackData.Benchmarks
	if flags.BenchmarkID != "" {
		ids := strings.Split(flags.BenchmarkID, ",")
		idSet := make(map[string]bool)
		for _, id := range ids {
			idSet[strings.ToUpper(id)] = true
		}
		var filtered []HaystackBenchmarkDef
		for _, b := range benchmarks {
			if idSet[strings.ToUpper(b.ID)] {
				filtered = append(filtered, b)
			}
		}
		benchmarks = filtered
	}

	if len(benchmarks) == 0 {
		return fmt.Errorf("no haystack benchmarks match the specified filters")
	}

	fmt.Fprintf(stdoutWriter, "Running %d haystack benchmarks with %d runs each\n", len(benchmarks), flags.Runs)

	// Group benchmarks by haystack for prompt caching
	groups := make(map[string][]HaystackBenchmarkDef)
	for _, b := range benchmarks {
		groups[b.Haystack] = append(groups[b.Haystack], b)
	}

	fmt.Fprintf(stdoutWriter, "Grouped into %d haystack(s) for prompt caching\n\n", len(groups))

	// Load haystack contents (do this once per haystack)
	haystackDir := filepath.Dir(haystackFile)
	haystackContents := make(map[string]string)
	for haystackID, haystackDef := range haystackData.Haystacks {
		// Only load if we have benchmarks using it
		if _, ok := groups[haystackID]; !ok {
			continue
		}

		fmt.Fprintf(stdoutWriter, "Loading haystack '%s'...\n", haystackID)
		content, err := LoadHaystackContent(haystackDir, haystackDef)
		if err != nil {
			return fmt.Errorf("failed to load haystack '%s': %w", haystackID, err)
		}
		haystackContents[haystackID] = content
		// Show size but NOT the content
		fmt.Fprintf(stdoutWriter, "  Loaded: %d bytes, %d lines\n", len(content), strings.Count(content, "\n"))
	}

	fmt.Fprintf(stdoutWriter, "\nStarting benchmark runs...\n\n")

	// Open CSV writer
	csvWriter, err := NewCSVWriter(csvPath, !flags.NoResume)
	if err != nil {
		return fmt.Errorf("failed to create CSV writer: %w", err)
	}
	defer csvWriter.Close()

	// Create LLM client
	llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey)

	// Simple system prompt - no agent behavior, just Q&A
	systemPrompt := "You are a helpful assistant. Answer questions about the provided source code concisely and accurately."

	// Collect results
	var allResults []RunResult
	totalBenchmarks := 0
	for _, group := range groups {
		totalBenchmarks += len(group)
	}

	// Run benchmarks grouped by haystack for prompt caching
	for haystackID, benchmarkGroup := range groups {
		haystackContent := haystackContents[haystackID]
		haystackDef := haystackData.Haystacks[haystackID]

		fmt.Fprintf(stdoutWriter, "Running %d benchmarks with haystack '%s'\n", len(benchmarkGroup), haystackID)

		// Build the context message (this stays constant for caching)
		contextMessage := fmt.Sprintf("<source_code file=\"%s\">\n%s\n</source_code>", haystackDef.File, haystackContent)

		for run := 1; run <= flags.Runs; run++ {
			for _, b := range benchmarkGroup {
				// Check if cancelled
				select {
				case <-ctx.Done():
					fmt.Fprintf(stdoutWriter, "\nBenchmark run cancelled\n")
					return nil
				default:
				}

				startTime := time.Now()

				// Build messages: system + context + question
				// The context is in the user message to enable prompt caching
				userMessage := fmt.Sprintf("%s\n\nQuestion: %s", contextMessage, b.Task)

				messages := []llm.Message{
					{Role: llm.RoleSystem, Content: systemPrompt},
					{Role: llm.RoleUser, Content: userMessage},
				}

				// Call LLM with no tools
				req := llm.ChatRequest{
					Model:       cfg.LLM.Model,
					Messages:    messages,
					Temperature: cfg.LLM.Temperature,
					MaxTokens:   cfg.LLM.MaxTokens,
					// No Tools - pure text completion
				}

				// Show progress (not the full prompt!)
				fmt.Fprintf(stdoutWriter, "  [%s] run %d: %s... ", b.ID, run, truncateString(b.Name, 40))

				resp, err := llmClient.Chat(ctx, req)
				completedAt := time.Now()
				durationMS := completedAt.Sub(startTime).Milliseconds()

				result := RunResult{
					BenchmarkID: b.ID,
					Run:         run,
					StartedAt:   startTime,
					CompletedAt: completedAt,
					DurationMS:  durationMS,
				}

				if err != nil {
					result.Success = false
					result.Errors = []string{err.Error()}
					fmt.Fprintf(stdoutWriter, "ERROR: %v\n", err)
				} else if len(resp.Choices) == 0 {
					result.Success = false
					result.Errors = []string{"no response from LLM"}
					fmt.Fprintf(stdoutWriter, "ERROR: no response\n")
				} else {
					// Extract response
					finalOutput := resp.Choices[0].Message.Content
					if finalOutput == "" {
						finalOutput = resp.Choices[0].Message.ReasoningContent
					}

					// Validate
					validator := NewValidator("", finalOutput, nil)
					success, validationErrors := validator.Validate(b.Validation)

					result.Success = success
					result.Errors = validationErrors
					result.LLMCalls = 1
					result.Tokens = resp.Usage.TotalTokens
					result.PromptTokens = resp.Usage.PromptTokens
					result.GeneratedTokens = resp.Usage.CompletionTokens
					// Note: CachedTokens not available in standard ChatResponse

					if success {
						fmt.Fprintf(stdoutWriter, "PASS (%d tokens)\n", result.Tokens)
					} else {
						fmt.Fprintf(stdoutWriter, "FAIL: %v\n", validationErrors)
					}
				}

				allResults = append(allResults, result)

				// Write to CSV
				if csvErr := csvWriter.WriteResult(&result); csvErr != nil {
					fmt.Fprintf(stdoutWriter, "Warning: failed to write result to CSV: %v\n", csvErr)
				}
			}
		}
	}

	// Print summary
	fmt.Fprintf(stdoutWriter, "\n--- Summary ---\n")
	passed := 0
	failed := 0
	totalTokens := 0
	for _, r := range allResults {
		if r.Success {
			passed++
		} else {
			failed++
		}
		totalTokens += r.Tokens
	}
	fmt.Fprintf(stdoutWriter, "Total: %d, Passed: %d, Failed: %d\n", len(allResults), passed, failed)
	fmt.Fprintf(stdoutWriter, "Total tokens used: %d\n", totalTokens)

	// Convert HaystackBenchmarkDef to BenchmarkDef for report generation
	var benchmarkDefs []BenchmarkDef
	for _, b := range benchmarks {
		benchmarkDefs = append(benchmarkDefs, BenchmarkDef{
			ID:         b.ID,
			Name:       b.Name,
			Category:   "needle_retrieval",
			Haystack:   b.Haystack,
			Task:       b.Task,
			Validation: b.Validation,
			Tags:       b.Tags,
		})
	}

	// Load config file for report
	configYAML := ""
	configFileName := "config.yaml"
	if flags.Suffix != "" {
		configFileName = fmt.Sprintf("config-%s.yaml", flags.Suffix)
	}
	configPath := filepath.Join(originalWorkspaceRoot, configFileName)
	if data, err := os.ReadFile(configPath); err == nil {
		configYAML = string(data)
	}

	// Generate markdown report
	generator := NewReportGenerator(allResults, benchmarkDefs, version, configYAML, flags.Runs)
	if err := generator.WriteMarkdown(outputPath); err != nil {
		return fmt.Errorf("failed to write report: %w", err)
	}

	fmt.Fprintf(stdoutWriter, "\nReport saved to: %s\n", outputPath)
	fmt.Fprintf(stdoutWriter, "CSV results saved to: %s\n", csvPath)
	fmt.Fprintf(stdoutWriter, "Terminal log saved to: %s\n", terminalPath)

	return nil
}

// truncateString truncates a string to maxLen with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
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

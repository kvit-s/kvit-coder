package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
)

// Environment manages the benchmark test environment.
type Environment struct {
	BaseDir      string // .kvit-coder-benchmark
	SetupDir     string // .kvit-coder-benchmark/setup
	WorkspaceDir string // .kvit-coder-benchmark/workspace
	ExpectedDir  string // .kvit-coder-benchmark/expected
	ResultsDir   string // .kvit-coder-benchmark/results
}

// NewEnvironment creates a new benchmark environment.
func NewEnvironment(baseDir string) *Environment {
	return &Environment{
		BaseDir:      baseDir,
		SetupDir:     filepath.Join(baseDir, "setup"),
		WorkspaceDir: filepath.Join(baseDir, "workspace"),
		ExpectedDir:  filepath.Join(baseDir, "expected"),
		ResultsDir:   filepath.Join(baseDir, "results"),
	}
}

// Initialize creates the benchmark directory structure.
func (e *Environment) Initialize() error {
	dirs := []string{
		e.BaseDir,
		e.SetupDir,
		e.WorkspaceDir,
		e.ExpectedDir,
		e.ResultsDir,
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}

	return nil
}

// SetupBenchmark prepares the workspace for a specific benchmark run.
// All benchmarks share the same workspace directory, which is cleaned before each run.
func (e *Environment) SetupBenchmark(benchmark BenchmarkDef, runID int) (string, error) {
	// All benchmarks use the same workspace directory (set per --benchmark suffix)
	runDir := e.WorkspaceDir

	// Clean workspace before each run
	if err := os.RemoveAll(runDir); err != nil {
		return "", fmt.Errorf("failed to clean workspace: %w", err)
	}

	if err := os.MkdirAll(runDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create workspace directory: %w", err)
	}

	// Create setup files
	for _, setup := range benchmark.Setup {
		filePath := filepath.Join(runDir, setup.File)

		// Create parent directories if needed
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			return "", fmt.Errorf("failed to create parent directory for %s: %w", setup.File, err)
		}

		if setup.Dir {
			// Create directory
			if err := os.MkdirAll(filePath, 0755); err != nil {
				return "", fmt.Errorf("failed to create directory %s: %w", setup.File, err)
			}
			continue
		}

		// Create file
		var content []byte
		if setup.Binary != nil {
			content = setup.Binary
		} else {
			content = []byte(setup.Content)
		}

		if err := os.WriteFile(filePath, content, 0644); err != nil {
			return "", fmt.Errorf("failed to create file %s: %w", setup.File, err)
		}
	}

	return runDir, nil
}

// CleanupBenchmark cleans the workspace after a benchmark run.
func (e *Environment) CleanupBenchmark(benchmark BenchmarkDef, runID int) error {
	// Workspace is cleaned before each run, so this is a no-op
	return nil
}

// CleanupAll removes all benchmark workspaces.
func (e *Environment) CleanupAll() error {
	return os.RemoveAll(e.WorkspaceDir)
}

// GetWorkspaceDir returns the workspace directory for a specific run.
func (e *Environment) GetWorkspaceDir(benchmarkID string, runID int) string {
	return e.WorkspaceDir
}

// SaveExpected saves expected results for validation.
func (e *Environment) SaveExpected(benchmarkID string, content []byte) error {
	path := filepath.Join(e.ExpectedDir, benchmarkID+".txt")
	return os.WriteFile(path, content, 0644)
}

// LoadExpected loads expected results for validation.
func (e *Environment) LoadExpected(benchmarkID string) ([]byte, error) {
	path := filepath.Join(e.ExpectedDir, benchmarkID+".txt")
	return os.ReadFile(path)
}

// ReadFileFromWorkspace reads a file from the benchmark workspace.
func (e *Environment) ReadFileFromWorkspace(benchmarkID string, runID int, relativePath string) ([]byte, error) {
	runDir := e.GetWorkspaceDir(benchmarkID, runID)
	path := filepath.Join(runDir, relativePath)
	return os.ReadFile(path)
}

// FileExistsInWorkspace checks if a file exists in the benchmark workspace.
func (e *Environment) FileExistsInWorkspace(benchmarkID string, runID int, relativePath string) bool {
	runDir := e.GetWorkspaceDir(benchmarkID, runID)
	path := filepath.Join(runDir, relativePath)
	_, err := os.Stat(path)
	return err == nil
}

// ListFilesInWorkspace lists all files in the benchmark workspace.
func (e *Environment) ListFilesInWorkspace(benchmarkID string, runID int) ([]string, error) {
	runDir := e.GetWorkspaceDir(benchmarkID, runID)
	var files []string

	err := filepath.Walk(runDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(runDir, path)
			files = append(files, relPath)
		}
		return nil
	})

	return files, err
}

package benchmark

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// BenchmarksFile represents the structure of benchmarks.yaml
type BenchmarksFile struct {
	Haystacks  map[string]HaystackDef `yaml:"haystacks"`
	Benchmarks []BenchmarkDef         `yaml:"benchmarks"`
}

// LoadBenchmarks loads benchmark definitions from a YAML file.
func LoadBenchmarks(path string) ([]BenchmarkDef, error) {
	_, benchmarks, err := LoadBenchmarksWithHaystacks(path)
	return benchmarks, err
}

// LoadBenchmarksWithHaystacks loads benchmark definitions and haystack configs from a YAML file.
func LoadBenchmarksWithHaystacks(path string) (map[string]HaystackDef, []BenchmarkDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read benchmarks file: %w", err)
	}

	var file BenchmarksFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, nil, fmt.Errorf("failed to parse benchmarks file: %w", err)
	}

	// Validate benchmarks
	for i, b := range file.Benchmarks {
		if b.ID == "" {
			return nil, nil, fmt.Errorf("benchmark %d missing required field 'id'", i)
		}
		if b.Task == "" {
			return nil, nil, fmt.Errorf("benchmark %s missing required field 'task'", b.ID)
		}
		if b.Category == "" {
			return nil, nil, fmt.Errorf("benchmark %s missing required field 'category'", b.ID)
		}
		// Validate haystack reference for needle_retrieval benchmarks
		if b.Category == "needle_retrieval" {
			if b.Haystack == "" {
				return nil, nil, fmt.Errorf("benchmark %s (needle_retrieval) missing required field 'haystack'", b.ID)
			}
			if file.Haystacks != nil {
				if _, ok := file.Haystacks[b.Haystack]; !ok {
					return nil, nil, fmt.Errorf("benchmark %s references unknown haystack '%s'", b.ID, b.Haystack)
				}
			}
		}
	}

	return file.Haystacks, file.Benchmarks, nil
}

// LoadHaystackContent loads the content of a haystack file.
// If the file doesn't exist and generate script is specified, runs the script first.
func LoadHaystackContent(benchmarksDir string, haystack HaystackDef) (string, error) {
	haystackPath := filepath.Join(benchmarksDir, haystack.File)

	// Check if file exists
	if _, err := os.Stat(haystackPath); os.IsNotExist(err) {
		// Try to generate if script is specified
		if haystack.Generate != "" {
			scriptPath := filepath.Join(benchmarksDir, "..", haystack.Generate)
			fmt.Printf("Generating haystack using %s...\n", haystack.Generate)

			cmd := exec.Command("sh", "-c", scriptPath)
			cmd.Dir = filepath.Join(benchmarksDir, "..")
			if output, err := cmd.CombinedOutput(); err != nil {
				return "", fmt.Errorf("failed to generate haystack: %w\nOutput: %s", err, output)
			}
		} else if haystack.Source != "" {
			return "", fmt.Errorf("haystack file %s not found (download from %s)", haystackPath, haystack.Source)
		} else {
			return "", fmt.Errorf("haystack file %s not found", haystackPath)
		}
	}

	content, err := os.ReadFile(haystackPath)
	if err != nil {
		return "", fmt.Errorf("failed to read haystack file: %w", err)
	}

	return string(content), nil
}

// GroupBenchmarksByHaystack groups needle_retrieval benchmarks by their haystack.
// Returns a map of haystack ID -> benchmarks, preserving order within each group.
func GroupBenchmarksByHaystack(benchmarks []BenchmarkDef) map[string][]BenchmarkDef {
	groups := make(map[string][]BenchmarkDef)
	for _, b := range benchmarks {
		if b.Category == "needle_retrieval" && b.Haystack != "" {
			groups[b.Haystack] = append(groups[b.Haystack], b)
		}
	}
	return groups
}

// IsNeedleRetrieval returns true if the benchmark is a needle retrieval type.
func IsNeedleRetrieval(b BenchmarkDef) bool {
	return b.Category == "needle_retrieval" && b.Haystack != ""
}

// HaystackFile represents the structure of haystack.yaml
type HaystackFile struct {
	Haystacks  map[string]HaystackDef  `yaml:"haystacks"`
	Benchmarks []HaystackBenchmarkDef  `yaml:"benchmarks"`
}

// HaystackBenchmarkDef defines a haystack benchmark (simpler than tool benchmarks)
type HaystackBenchmarkDef struct {
	ID         string            `yaml:"id"`
	Name       string            `yaml:"name"`
	Haystack   string            `yaml:"haystack"`
	Task       string            `yaml:"task"`
	Validation []ValidationCheck `yaml:"validation"`
	Tags       []string          `yaml:"tags"`
}

// LoadHaystackBenchmarks loads haystack benchmark definitions from haystack.yaml
func LoadHaystackBenchmarks(path string) (*HaystackFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read haystack file: %w", err)
	}

	var file HaystackFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse haystack file: %w", err)
	}

	// Validate benchmarks
	for i, b := range file.Benchmarks {
		if b.ID == "" {
			return nil, fmt.Errorf("haystack benchmark %d missing required field 'id'", i)
		}
		if b.Task == "" {
			return nil, fmt.Errorf("haystack benchmark %s missing required field 'task'", b.ID)
		}
		if b.Haystack == "" {
			return nil, fmt.Errorf("haystack benchmark %s missing required field 'haystack'", b.ID)
		}
		if _, ok := file.Haystacks[b.Haystack]; !ok {
			return nil, fmt.Errorf("haystack benchmark %s references unknown haystack '%s'", b.ID, b.Haystack)
		}
	}

	return &file, nil
}

// FindHaystackFile looks for haystack.yaml in standard locations.
func FindHaystackFile(workspaceRoot string) string {
	locations := []string{
		filepath.Join(workspaceRoot, "haystack.yaml"),
		filepath.Join(workspaceRoot, "benchmarks", "haystack.yaml"),
		filepath.Join(workspaceRoot, ".kvit-coder-benchmark", "haystack.yaml"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

// FilterBenchmarks filters benchmarks by category and/or IDs.
func FilterBenchmarks(benchmarks []BenchmarkDef, categories []string, ids []string) []BenchmarkDef {
	if len(categories) == 0 && len(ids) == 0 {
		return benchmarks
	}

	categorySet := make(map[string]bool)
	for _, c := range categories {
		categorySet[strings.ToLower(c)] = true
	}

	idSet := make(map[string]bool)
	for _, id := range ids {
		idSet[strings.ToUpper(id)] = true
	}

	var filtered []BenchmarkDef
	for _, b := range benchmarks {
		if len(idSet) > 0 && idSet[strings.ToUpper(b.ID)] {
			filtered = append(filtered, b)
			continue
		}
		if len(categorySet) > 0 && categorySet[strings.ToLower(b.Category)] {
			filtered = append(filtered, b)
		}
	}

	return filtered
}

// FindBenchmarksFile looks for benchmarks.yaml in standard locations.
func FindBenchmarksFile(workspaceRoot string) string {
	locations := []string{
		filepath.Join(workspaceRoot, "benchmarks.yaml"),
		filepath.Join(workspaceRoot, ".kvit-coder-benchmark", "benchmarks.yaml"),
		filepath.Join(workspaceRoot, "config", "benchmarks.yaml"),
	}

	for _, loc := range locations {
		if _, err := os.Stat(loc); err == nil {
			return loc
		}
	}

	return ""
}

// GetBenchmarksByCategory groups benchmarks by category.
func GetBenchmarksByCategory(benchmarks []BenchmarkDef) map[string][]BenchmarkDef {
	result := make(map[string][]BenchmarkDef)
	for _, b := range benchmarks {
		result[b.Category] = append(result[b.Category], b)
	}
	return result
}

// ListCategories returns unique categories from benchmarks.
func ListCategories(benchmarks []BenchmarkDef) []string {
	seen := make(map[string]bool)
	var categories []string
	for _, b := range benchmarks {
		if !seen[b.Category] {
			seen[b.Category] = true
			categories = append(categories, b.Category)
		}
	}
	return categories
}

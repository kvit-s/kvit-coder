package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// BenchmarksFile represents the structure of benchmarks.yaml
type BenchmarksFile struct {
	Benchmarks []BenchmarkDef `yaml:"benchmarks"`
}

// LoadBenchmarks loads benchmark definitions from a YAML file.
func LoadBenchmarks(path string) ([]BenchmarkDef, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read benchmarks file: %w", err)
	}

	var file BenchmarksFile
	if err := yaml.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("failed to parse benchmarks file: %w", err)
	}

	// Validate benchmarks
	for i, b := range file.Benchmarks {
		if b.ID == "" {
			return nil, fmt.Errorf("benchmark %d missing required field 'id'", i)
		}
		if b.Task == "" {
			return nil, fmt.Errorf("benchmark %s missing required field 'task'", b.ID)
		}
		if b.Category == "" {
			return nil, fmt.Errorf("benchmark %s missing required field 'category'", b.ID)
		}
	}

	return file.Benchmarks, nil
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

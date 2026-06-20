package benchmark

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FindThinkbenchSuite locates the in-repo thinkbench suite directory. It checks
// benchmarks/thinkbench/suite under the given root and a couple of parents, so
// it works whether kvit-coder is invoked from the repo root or the benchmarks
// directory. Returns "" if not found.
func FindThinkbenchSuite(root string) string {
	candidates := []string{
		filepath.Join(root, "benchmarks", "thinkbench", "suite"),
		filepath.Join(root, "thinkbench", "suite"),
		filepath.Join(root, "suite"),
	}
	// Also walk a few parents of root.
	cur := root
	for i := 0; i < 3; i++ {
		candidates = append(candidates, filepath.Join(cur, "benchmarks", "thinkbench", "suite"))
		cur = filepath.Dir(cur)
	}
	for _, c := range candidates {
		if info, err := os.Stat(filepath.Join(c, "manifest.json")); err == nil && !info.IsDir() {
			abs, err := filepath.Abs(c)
			if err == nil {
				return abs
			}
			return c
		}
	}
	return ""
}

// LoadThinkbenchSuite reads the suite manifest and, per task, resolves the
// on-disk brief, setup tree, and held-out grader path. It prefers the on-disk
// brief.txt over the manifest copy to avoid drift.
func LoadThinkbenchSuite(suiteDir string) ([]TBTask, error) {
	manifestPath := filepath.Join(suiteDir, "manifest.json")
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var entries []TBManifestEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	tasks := make([]TBTask, 0, len(entries))
	for _, e := range entries {
		taskDir := filepath.Join(suiteDir, "tasks", e.Slug)

		brief := e.Brief
		if onDisk, err := os.ReadFile(filepath.Join(taskDir, "brief.txt")); err == nil {
			brief = string(onDisk)
		}

		setupDir := filepath.Join(taskDir, "setup")
		hasSetup := e.HasSetup
		if info, err := os.Stat(setupDir); err != nil || !info.IsDir() {
			hasSetup = false
		}

		graderPath := filepath.Join(taskDir, "grade.py")
		if !e.Observed {
			if _, err := os.Stat(graderPath); err != nil {
				return nil, fmt.Errorf("graded task %q is missing grade.py: %w", e.Slug, err)
			}
		} else {
			graderPath = ""
		}

		// Sanity: graded tasks must declare checks; observed must not be graded.
		if !e.Observed && e.NumChecks <= 0 {
			return nil, fmt.Errorf("graded task %q declares num_checks=%d", e.Slug, e.NumChecks)
		}

		tasks = append(tasks, TBTask{
			Slug:       e.Slug,
			Type:       TBType(e.Type),
			Brief:      brief,
			Observed:   e.Observed,
			NumChecks:  e.NumChecks,
			HasSetup:   hasSetup,
			SetupDir:   setupDir,
			GraderPath: graderPath,
		})
	}

	return tasks, nil
}

// FilterThinkbench narrows tasks by slug (ids) and/or type, and applies the
// observed-task policy. When ids is non-empty, only matching slugs are kept
// (and observed tasks named explicitly are included regardless of policy).
// When types is non-empty, only matching types are kept. includeObserved
// controls whether ungraded ambiguous-spec tasks are run when not named
// explicitly.
func FilterThinkbench(tasks []TBTask, ids, types []string, includeObserved bool) []TBTask {
	idSet := toLowerSet(ids)
	typeSet := toLowerSet(types)

	var out []TBTask
	for _, t := range tasks {
		// Explicit id selection wins over all policy.
		if len(idSet) > 0 {
			if idSet[strings.ToLower(t.Slug)] {
				out = append(out, t)
			}
			continue
		}

		if t.Observed && !includeObserved {
			continue
		}

		if len(typeSet) > 0 && !typeSet[strings.ToLower(string(t.Type))] {
			continue
		}

		out = append(out, t)
	}
	return out
}

func toLowerSet(items []string) map[string]bool {
	if len(items) == 0 {
		return nil
	}
	m := make(map[string]bool, len(items))
	for _, s := range items {
		s = strings.TrimSpace(s)
		if s != "" {
			m[strings.ToLower(s)] = true
		}
	}
	return m
}

// copyTree recursively copies the directory tree at src into dst, creating dst
// (and parents) as needed. File permissions are preserved (best-effort).
func copyTree(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return copyFile(src, dst, info.Mode())
	}

	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := copyTree(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}
		fi, err := entry.Info()
		if err != nil {
			return err
		}
		if err := copyFile(srcPath, dstPath, fi.Mode()); err != nil {
			return err
		}
	}
	return nil
}

func copyFile(src, dst string, mode os.FileMode) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}
	return os.WriteFile(dst, data, mode.Perm())
}

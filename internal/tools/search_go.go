package tools

import (
	"bufio"
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// searchExcludeDirs mirrors the grep fallback's exclusions, so the pure-Go
// fallback searches the same tree the external tools would.
var searchExcludeDirs = map[string]bool{
	".git": true, "node_modules": true, "__pycache__": true,
	".venv": true, "venv": true, ".tox": true, ".mypy_cache": true,
}

// searchWithGo is the pure-Go search fallback (decision 7). rg works on
// Windows when installed; neither the grep nor the find|xargs grep fallback
// does. A filepath.WalkDir walk respecting the same excludeDirs removes the
// 2>/dev/null|xargs quoting surface on the platform where quoting differs
// most. Result parsing (parseSearchOutput) is untouched: this builds the same
// searchMatch values directly.
func (t *SearchTool) searchWithGo(ctx context.Context, pattern, searchPath, filePattern string, contextLines int) ([]searchMatch, error) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, fmt.Errorf("invalid pattern: %w", err)
	}

	var files []string
	walkErr := filepath.WalkDir(searchPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if searchExcludeDirs[d.Name()] {
				return filepath.SkipDir
			}
			return nil
		}
		if filePattern != "" {
			matched, err := filepath.Match(filePattern, d.Name())
			if err != nil || !matched {
				// filepath.Match only handles globs; fall back to suffix
				// comparison for patterns it rejects.
				if err != nil && !strings.HasSuffix(d.Name(), strings.TrimPrefix(filePattern, "*")) {
					return nil
				}
				if err == nil && !matched {
					return nil
				}
			}
		}
		files = append(files, path)
		return nil
	})
	if walkErr != nil {
		return nil, walkErr
	}
	sort.Strings(files)

	var matches []searchMatch
	for _, file := range files {
		if ctx.Err() != nil {
			return matches, ctx.Err()
		}
		fm, err := searchFileGo(file, re, contextLines)
		if err != nil {
			continue
		}
		matches = append(matches, fm...)
	}
	return matches, nil
}

func searchFileGo(path string, re *regexp.Regexp, contextLines int) ([]searchMatch, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	// Skip large files (binaries, bundles) rather than loading them whole.
	if fi.Size() > 5*1024*1024 {
		return nil, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		// NUL byte means binary; skip the file.
		if strings.IndexByte(line, 0) >= 0 {
			return nil, nil
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var matches []searchMatch
	for i, line := range lines {
		if !re.MatchString(line) {
			continue
		}
		m := searchMatch{File: path, Line: i + 1, Match: line}
		// Before: up to contextLines preceding lines.
		start := i - contextLines
		if start < 0 {
			start = 0
		}
		m.Before = append([]string(nil), lines[start:i]...)
		// After: up to contextLines following lines.
		end := i + 1 + contextLines
		if end > len(lines) {
			end = len(lines)
		}
		m.After = append([]string(nil), lines[i+1:end]...)
		// Snippet mirrors parseSearchOutput's format.
		var snippetLines []string
		for j, b := range m.Before {
			snippetLines = append(snippetLines, fmt.Sprintf("%4d│%s", start+j+1, b))
		}
		snippetLines = append(snippetLines, fmt.Sprintf("%4d│%s", m.Line, m.Match))
		for j, a := range m.After {
			snippetLines = append(snippetLines, fmt.Sprintf("%4d│%s", m.Line+j+1, a))
		}
		m.Snippet = strings.Join(snippetLines, "\n")
		matches = append(matches, m)
	}
	return matches, nil
}

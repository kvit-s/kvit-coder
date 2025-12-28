package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// searchBackend represents the available search command
type searchBackend int

const (
	backendRipgrep searchBackend = iota
	backendGrep
	backendNone
)

var (
	detectedBackend     searchBackend
	backendDetectedOnce sync.Once
)

// detectSearchBackend checks which search command is available
func detectSearchBackend() searchBackend {
	backendDetectedOnce.Do(func() {
		// Try ripgrep first
		if _, err := exec.LookPath("rg"); err == nil {
			detectedBackend = backendRipgrep
			return
		}
		// Fall back to grep
		if _, err := exec.LookPath("grep"); err == nil {
			detectedBackend = backendGrep
			return
		}
		detectedBackend = backendNone
	})
	return detectedBackend
}

// IsRipgrepAvailable returns true if ripgrep (rg) is available on the system
func IsRipgrepAvailable() bool {
	return detectSearchBackend() == backendRipgrep
}

// SearchTool searches for code patterns and returns snippets with line numbers
type SearchTool struct {
	config        *config.Config
	workspaceRoot string
	tempFileMgr   *TempFileManager
}

func NewSearchTool(cfg *config.Config, tempFileMgr *TempFileManager) *SearchTool {
	// Trigger backend detection at initialization
	detectSearchBackend()
	return &SearchTool{
		config:        cfg,
		workspaceRoot: cfg.Workspace.Root,
		tempFileMgr:   tempFileMgr,
	}
}

func (t *SearchTool) Name() string {
	return "Search"
}

func (t *SearchTool) Description() string {
	return "Search for code patterns and return snippets with line numbers. Results can be directly used with edit. Use before editing to find exact locations."
}

func (t *SearchTool) Check(ctx context.Context, args json.RawMessage) error {
	return nil
}

func (t *SearchTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Text or regex pattern to search for",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Path to search in (default: workspace root)",
			},
			"file_pattern": map[string]any{
				"type":        "string",
				"description": "File glob pattern, e.g., '*.py', '*.go'",
			},
			"context_lines": map[string]any{
				"type":        "integer",
				"description": "Lines of context around match (default: 3)",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *SearchTool) PromptCategory() string { return "filesystem" }
func (t *SearchTool) PromptOrder() int        { return 5 } // Before read
func (t *SearchTool) PromptSection() string {
	return `### Search - Find Code Patterns

**Usage:** ` + "`" + `Search {"pattern": "<text or regex>"}` + "`" + `

Examples:
- ` + "`" + `Search {"pattern": "class AuthForm", "file_pattern": "*.py"}` + "`" + `
- ` + "`" + `Search {"pattern": "def authenticate", "path": "src/"}` + "`" + `
- ` + "`" + `Search {"pattern": "func.*Handler", "file_pattern": "*.go"}` + "`" + `

**Parameters:**
- ` + "`pattern`" + ` (required): Text or regex to search for
- ` + "`path`" + ` (optional): Directory to search in (default: workspace root)
- ` + "`file_pattern`" + ` (optional): File glob, e.g., "*.py", "*.go"
- ` + "`context_lines`" + ` (optional): Lines of context around match (default: 3)`
}

// searchMatch represents a single search match with context
type searchMatch struct {
	File    string   `json:"file"`
	Line    int      `json:"line"`
	Match   string   `json:"match"`
	Snippet string   `json:"snippet"`
	Before  []string `json:"context_before,omitempty"`
	After   []string `json:"context_after,omitempty"`
}

func (t *SearchTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Pattern      string `json:"pattern"`
		Path         string `json:"path"`
		FilePattern  string `json:"file_pattern"`
		ContextLines int    `json:"context_lines"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	if params.Pattern == "" {
		return map[string]any{
			"success": false,
			"error":   "missing_pattern",
			"message": "Pattern is required",
		}, nil
	}

	if params.ContextLines == 0 {
		params.ContextLines = 3
	}

	searchPath := t.workspaceRoot
	if params.Path != "" {
		// Check path permissions (for denied_paths)
		result, err := t.config.CheckPathPermission(params.Path, config.AccessRead)
		if err != nil && result == config.PermissionDenied {
			return map[string]any{
				"success": false,
				"error":   "access_denied",
				"message": fmt.Sprintf("Access denied: %v", err),
			}, nil
		}

		// Normalize and validate path using shared utility
		fullPath, outside, err := NormalizeAndValidatePath(t.workspaceRoot, params.Path)
		if err != nil {
			return map[string]any{
				"success": false,
				"error":   "invalid_path",
				"message": fmt.Sprintf("Invalid path: %v", err),
			}, nil
		}

		// For outside-workspace paths, use CheckPathSafety which respects path_safety_mode
		if outside {
			if err := t.config.CheckPathSafety("search", params.Path); err != nil {
				return map[string]any{
					"success": false,
					"error":   "path_outside_workspace",
					"message": err.Error(),
				}, nil
			}
		}
		searchPath = fullPath
	}

	backend := detectSearchBackend()
	if backend == backendNone {
		return map[string]any{
			"success": false,
			"error":   "no_search_command",
			"message": "Neither 'rg' (ripgrep) nor 'grep' found in PATH. Please install one of them.",
		}, nil
	}

	var output []byte
	var err error

	if backend == backendRipgrep {
		output, err = t.searchWithRipgrep(ctx, params.Pattern, searchPath, params.FilePattern, params.ContextLines)
	} else {
		output, err = t.searchWithGrep(ctx, params.Pattern, searchPath, params.FilePattern, params.ContextLines)
	}

	// Both rg and grep exit with 1 if no matches found, which is not an error
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				// No matches found
				return map[string]any{
					"success":       true,
					"matches":       []searchMatch{},
					"total_matches": 0,
					"message":       "No matches found",
				}, nil
			}
		}
		return nil, fmt.Errorf("search failed: %w", err)
	}

	// Parse output (format is the same for both rg and grep with our options)
	matches := parseSearchOutput(string(output), params.ContextLines)
	totalMatches := len(matches)

	// Get thresholds from config (with defaults)
	maxSnippet := t.config.Tools.Search.MaxSnippetResults
	if maxSnippet == 0 {
		maxSnippet = 20
	}
	maxCompact := t.config.Tools.Search.MaxCompactResults
	if maxCompact == 0 {
		maxCompact = 100
	}

	result := map[string]any{
		"success":       true,
		"total_matches": totalMatches,
	}

	if totalMatches == 0 {
		result["matches"] = []searchMatch{}
		result["message"] = "No matches found"
		return result, nil
	}

	// Helper to generate read hint
	readHint := func(file string, line int) string {
		startLine := line - params.ContextLines
		if startLine < 1 {
			startLine = 1
		}
		if t.config.Tools.Read.Enabled {
			return fmt.Sprintf("Read {\"path\": \"%s\", \"start\": %d, \"limit\": 20}", file, startLine)
		}
		return fmt.Sprintf("Shell {\"command\": \"cat -n %s | head -%d\"}", file, startLine+20)
	}

	// Tiered output based on match count
	if totalMatches <= maxSnippet {
		// Tier 1: Full snippets
		result["matches"] = matches
		first := matches[0]
		result["hint"] = "To read more context: " + readHint(first.File, first.Line)
	} else if totalMatches <= maxCompact {
		// Tier 2: Compact format (file:line:match trimmed)
		compact := make([]map[string]any, len(matches))
		for i, m := range matches {
			match := m.Match
			if len(match) > 50 {
				match = match[:50] + "..."
			}
			compact[i] = map[string]any{
				"file":  m.File,
				"line":  m.Line,
				"match": match,
			}
		}
		result["matches"] = compact
		result["format"] = "compact"
		result["message"] = fmt.Sprintf("Showing %d matches in compact format.", totalMatches)
		first := matches[0]
		result["hint"] = "To read match context: " + readHint(first.File, first.Line)
	} else {
		// Tier 3: Too many matches - save to temp file within workspace (if available)
		showFirst := 10
		showLast := 10
		trimMatch := func(s string) string {
			if len(s) > 50 {
				return s[:50] + "..."
			}
			return s
		}

		var preview []map[string]any
		for i := 0; i < showFirst && i < len(matches); i++ {
			m := matches[i]
			preview = append(preview, map[string]any{"file": m.File, "line": m.Line, "match": trimMatch(m.Match)})
		}

		result["matches_preview"] = preview
		result["format"] = "truncated"
		result["message"] = fmt.Sprintf("Too many matches (%d). First %d shown.", totalMatches, showFirst)

		// Add last few matches
		if len(matches) > showFirst+showLast {
			var lastPreview []map[string]any
			for i := len(matches) - showLast; i < len(matches); i++ {
				m := matches[i]
				lastPreview = append(lastPreview, map[string]any{"file": m.File, "line": m.Line, "match": trimMatch(m.Match)})
			}
			result["matches_last"] = lastPreview
		}

		// Try to save to temp file if TempFileManager is available
		if t.tempFileMgr != nil {
			tempFile, err := t.tempFileMgr.CreateTempFile()
			if err == nil {
				defer tempFile.Close()
				// Write all matches to temp file
				for _, m := range matches {
					fmt.Fprintf(tempFile, "%s:%d: %s\n", m.File, m.Line, m.Match)
				}
				result["results_file"] = tempFile.Name()
				if t.config.Tools.Read.Enabled {
					result["hint"] = fmt.Sprintf("Full results: Read {\"path\": \"%s\"}", tempFile.Name())
				} else {
					result["hint"] = fmt.Sprintf("Full results: %s", tempFile.Name())
				}
			}
		}

		// Hints for reading match context
		first := matches[0]
		result["hint_context"] = "To read match context: " + readHint(first.File, first.Line)
	}

	return result, nil
}

// searchWithRipgrep executes search using ripgrep
func (t *SearchTool) searchWithRipgrep(ctx context.Context, pattern, searchPath, filePattern string, contextLines int) ([]byte, error) {
	cmdArgs := []string{
		"-n",                                 // line numbers
		fmt.Sprintf("-C%d", contextLines),   // context
		"--no-heading",                       // each line is its own result
		"--with-filename",                    // include filename
	}

	if filePattern != "" {
		cmdArgs = append(cmdArgs, "-g", filePattern)
	}

	cmdArgs = append(cmdArgs, "--", pattern, searchPath)

	cmd := exec.CommandContext(ctx, "rg", cmdArgs...)
	return cmd.Output()
}

// searchWithGrep executes search using grep (with find for file patterns)
func (t *SearchTool) searchWithGrep(ctx context.Context, pattern, searchPath, filePattern string, contextLines int) ([]byte, error) {
	// Directories to exclude from search
	excludeDirs := []string{".git", "node_modules", "__pycache__", ".venv", "venv", ".tox", ".mypy_cache"}

	// For grep, we need to use find + grep for file patterns, or grep -r directly
	if filePattern != "" {
		// Build exclusion part for find
		var excludeParts []string
		for _, dir := range excludeDirs {
			excludeParts = append(excludeParts, fmt.Sprintf("-name %q -prune -o", dir))
		}
		excludeStr := strings.Join(excludeParts, " ")

		// Build find | xargs grep command
		findCmd := fmt.Sprintf("find %s %s -type f -name %q -print 2>/dev/null | xargs grep -n -H -C%d -e %q 2>/dev/null || true",
			searchPath, excludeStr, filePattern, contextLines, pattern)

		cmd := exec.CommandContext(ctx, "sh", "-c", findCmd)
		return cmd.Output()
	}

	// Build exclusion arguments for grep
	var excludeArgs []string
	for _, dir := range excludeDirs {
		excludeArgs = append(excludeArgs, fmt.Sprintf("--exclude-dir=%s", dir))
	}

	// Simple recursive grep
	cmdArgs := []string{
		"-r",                                // recursive
		"-n",                                // line numbers
		"-H",                                // include filename
		fmt.Sprintf("-C%d", contextLines),   // context
	}
	cmdArgs = append(cmdArgs, excludeArgs...)
	cmdArgs = append(cmdArgs, "-e", pattern, searchPath)

	cmd := exec.CommandContext(ctx, "grep", cmdArgs...)
	return cmd.Output()
}

// parseSearchOutput parses grep/ripgrep output with context
// Both tools use similar output format with -n -H -C options
func parseSearchOutput(output string, contextLines int) []searchMatch {
	var matches []searchMatch
	var currentMatch *searchMatch
	var contextBuffer []string
	var afterCount int

	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()

		// Parse line format: file:line:content or file-line-content (context)
		// Actual match uses : as separator, context uses -
		var file string
		var lineNum int
		var content string
		var isMatch bool

		// Try to parse as match line (file:line:content)
		parts := strings.SplitN(line, ":", 3)
		if len(parts) >= 3 {
			file = parts[0]
			if n, err := strconv.Atoi(parts[1]); err == nil {
				lineNum = n
				content = parts[2]
				isMatch = true
			}
		}

		// If not a match, try context line (file-line-content)
		if !isMatch && len(parts) < 3 {
			dashParts := strings.SplitN(line, "-", 3)
			if len(dashParts) >= 3 {
				file = dashParts[0]
				if n, err := strconv.Atoi(dashParts[1]); err == nil {
					lineNum = n
					content = dashParts[2]
					isMatch = false
				}
			}
		}

		// Skip separator lines (--)
		if line == "--" {
			if currentMatch != nil {
				matches = append(matches, *currentMatch)
				currentMatch = nil
			}
			contextBuffer = nil
			afterCount = 0
			continue
		}

		if lineNum == 0 {
			continue // Couldn't parse line
		}

		if isMatch {
			// This is an actual match
			if currentMatch != nil {
				matches = append(matches, *currentMatch)
			}

			currentMatch = &searchMatch{
				File:   file,
				Line:   lineNum,
				Match:  content,
				Before: contextBuffer,
			}
			contextBuffer = nil
			afterCount = 0
		} else if currentMatch != nil {
			// This is a context line after match
			currentMatch.After = append(currentMatch.After, content)
			afterCount++
			if afterCount >= contextLines {
				matches = append(matches, *currentMatch)
				currentMatch = nil
				afterCount = 0
			}
		} else {
			// Context line before match
			contextBuffer = append(contextBuffer, content)
			if len(contextBuffer) > contextLines {
				contextBuffer = contextBuffer[1:]
			}
		}
	}

	// Don't forget the last match
	if currentMatch != nil {
		matches = append(matches, *currentMatch)
	}

	// Build snippets
	for i := range matches {
		m := &matches[i]
		var snippetLines []string
		startLine := m.Line - len(m.Before)
		for j, line := range m.Before {
			snippetLines = append(snippetLines, fmt.Sprintf("%4d│%s", startLine+j, line))
		}
		snippetLines = append(snippetLines, fmt.Sprintf("%4d│%s", m.Line, m.Match))
		for j, line := range m.After {
			snippetLines = append(snippetLines, fmt.Sprintf("%4d│%s", m.Line+j+1, line))
		}
		m.Snippet = strings.Join(snippetLines, "\n")
	}

	return matches
}

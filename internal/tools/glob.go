package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// DefaultGlobMaxResults caps one Glob call. Past it the result carries a
// truncation flag instead of more paths, the way Search degrades to compact
// output: a flat list of thousands of files is never what the next Read wants.
const DefaultGlobMaxResults = 100

// GlobTool finds files by glob pattern and lists directory contents. It owns
// every "which files exist" question: Read reads bytes, Search reads inside
// files, Glob names them.
type GlobTool struct {
	config        *config.Config
	workspaceRoot string
	// toolCtx carries the subagent-child mark for the path-prompt deny.
	// Nil in a test, which leaves only the config rules in force.
	toolCtx *ToolContext
}

func NewGlobTool(cfg *config.Config) *GlobTool {
	return &GlobTool{
		config:        cfg,
		workspaceRoot: cfg.Workspace.Root,
	}
}

// SetToolContext gives the tool the context it needs to deny prompts inside
// a subagent child.
func (t *GlobTool) SetToolContext(toolCtx *ToolContext) { t.toolCtx = toolCtx }

// ParallelSafe says Glob can run alongside other reads in a Batch: it changes
// nothing, so nothing else in the batch can be affected by when it runs.
func (t *GlobTool) ParallelSafe() bool { return true }

func (t *GlobTool) Name() string {
	return "Glob"
}

func (t *GlobTool) Description() string {
	return "Find files by glob pattern or list a directory's contents. Use before Search/Read to narrow the candidate set."
}

func (t *GlobTool) Check(ctx context.Context, args json.RawMessage) error {
	// No pre-execution checks needed for glob
	return nil
}

func (t *GlobTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Glob pattern, e.g. '**/*.go', 'src/**/*.ts'. Empty lists the directory at path instead.",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Directory to search in (default: workspace root)",
			},
		},
	}
}

func (t *GlobTool) PromptCategory() string     { return "filesystem" }
func (t *GlobTool) PromptOrder() int           { return 4 } // Before Search (5): discovery narrows what Search reads
func (t *GlobTool) PromptTemplateName() string { return "glob" }
func (t *GlobTool) PromptSection() string {
	return `### Glob - Find Files / List Directories

**Usage:** ` + "`" + `Glob {"pattern": "<glob>"}` + "`" + `

Examples:
- ` + "`" + `Glob {"pattern": "**/*.go"}` + "`" + ` - every Go file under the workspace
- ` + "`" + `Glob {"pattern": "src/**/*.ts", "path": "web/"}` + "`" + ` - scoped to a directory
- ` + "`" + `Glob {"path": "src/"}` + "`" + ` - list directory contents (no pattern)

**Parameters:**
- ` + "`pattern`" + ` (optional): Glob with '*' (within a directory), '**' (across directories), '?', '[...]'. Empty lists the directory at ` + "`path`" + `.
- ` + "`path`" + ` (optional): Directory to search in (default: workspace root)

Results are sorted alphabetically and capped; past the cap a truncation flag
says to narrow the pattern. Excluded directories (.git, node_modules, ...) are
never walked. To read what is inside a file, use Search or Read next.
Always use Glob — not Read — to see what a directory contains.`
}

func (t *GlobTool) maxResults() int {
	if t.config.Tools.Glob.MaxResults > 0 {
		return t.config.Tools.Glob.MaxResults
	}
	return DefaultGlobMaxResults
}

func (t *GlobTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Pattern string `json:"pattern"`
		Path    string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	// Resolve the search root the same way Search does: the workspace root by
	// default, with denied-paths and outside-workspace safety when set.
	root := t.workspaceRoot
	displayPath := params.Path
	if params.Path != "" {
		result, err := t.config.CheckPathPermission(params.Path, config.AccessRead)
		if err != nil && result == config.PermissionDenied {
			return map[string]any{
				"success": false,
				"error":   "access_denied",
				"message": fmt.Sprintf("Access denied: %v", err),
			}, nil
		}

		fullPath, outside, err := NormalizeAndValidatePath(t.workspaceRoot, params.Path)
		if err != nil {
			return map[string]any{
				"success": false,
				"error":   "invalid_path",
				"message": fmt.Sprintf("Invalid path: %v", err),
			}, nil
		}

		if outside {
			if err := subagentPathPromptDeny(t.toolCtx, t.config, "glob", params.Path); err != nil {
				return nil, err
			}
			if err := t.config.CheckPathSafety("glob", params.Path); err != nil {
				return map[string]any{
					"success": false,
					"error":   "path_outside_workspace",
					"message": err.Error(),
				}, nil
			}
		}

		root = fullPath
	} else {
		displayPath = "."
	}

	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]any{
				"success": false,
				"error":   "path_not_found",
				"path":    params.Path,
				"message": fmt.Sprintf("Path not found: %s", params.Path),
			}, nil
		}
		return nil, fmt.Errorf("stat path: %w", err)
	}
	if !info.IsDir() {
		return map[string]any{
			"success": false,
			"error":   "not_a_directory",
			"path":    params.Path,
			"message": fmt.Sprintf("%s is a file. Use Read to read it.", params.Path),
		}, nil
	}

	pattern := normalizeGlobPattern(params.Pattern)
	if pattern == "" {
		return t.listDirectory(root, displayPath)
	}
	return t.findPattern(ctx, root, displayPath, pattern)
}

// normalizeGlobPattern cleans a pattern for matching: slashes throughout, no
// leading anchor, a trailing slash meaning "everything under here".
func normalizeGlobPattern(pattern string) string {
	p := strings.TrimSpace(pattern)
	p = strings.ReplaceAll(p, "\\", "/")
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "./")
	if p == "" || p == "." {
		return ""
	}
	if strings.HasSuffix(p, "/") {
		p += "*"
	}
	return p
}

// listDirectory answers a pattern-less Glob: one level of the directory,
// directories first with a trailing slash, the shape Read used to return.
func (t *GlobTool) listDirectory(fullPath, displayPath string) (any, error) {
	entries, err := os.ReadDir(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read directory: %w", err)
	}

	max := t.maxResults()
	var dirs, files []map[string]any
	for _, entry := range entries {
		item := map[string]any{
			"name": entry.Name(),
			"type": "file",
		}
		if entry.IsDir() {
			item["name"] = entry.Name() + "/"
			item["type"] = "dir"
			dirs = append(dirs, item)
		} else {
			if info, err := entry.Info(); err == nil {
				item["size"] = info.Size()
			}
			files = append(files, item)
		}
	}

	result := append(dirs, files...)
	total := len(result)
	shown := total
	if shown > max {
		result = result[:max]
		shown = max
	}

	response := map[string]any{
		"success":       true,
		"path":          displayPath,
		"type":          "directory",
		"entries":       result,
		"shown_entries": shown,
		"total_entries": total,
	}

	if shown < total {
		response["hint"] = fmt.Sprintf("Showing %d of %d entries. Pass a pattern to narrow it.", shown, total)
	}

	return response, nil
}

// findPattern walks root and returns the paths matching pattern, relative to
// the workspace so they can go straight into Read. Excluded build and VCS
// directories are never descended into, mirroring the Search fallback.
func (t *GlobTool) findPattern(ctx context.Context, root, displayPath, pattern string) (any, error) {
	patSegments := strings.Split(pattern, "/")

	var matches []string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if d.IsDir() {
			if path != root && searchExcludeDirs[d.Name()] {
				return filepath.SkipDir
			}
			// The root itself is the search scope, never a match.
			if path == root {
				return nil
			}
		}

		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		relSlash := filepath.ToSlash(rel)
		if !matchGlobSegments(patSegments, strings.Split(relSlash, "/")) {
			return nil
		}

		display := t.displayPath(path)
		if d.IsDir() {
			display += "/"
		}
		matches = append(matches, display)
		return nil
	})
	if walkErr != nil {
		return nil, fmt.Errorf("glob failed: %w", walkErr)
	}

	sort.Strings(matches)

	max := t.maxResults()
	total := len(matches)
	truncated := total > max
	shown := matches
	if truncated {
		shown = matches[:max]
	}

	result := map[string]any{
		"success":       true,
		"pattern":       pattern,
		"path":          displayPath,
		"files":         shown,
		"total_matches": total,
	}
	if total == 0 {
		result["message"] = "No files found"
	} else if truncated {
		result["truncated"] = true
		result["message"] = fmt.Sprintf("Showing first %d of %d matches. Narrow the pattern.", max, total)
	}
	return result, nil
}

// displayPath renders an absolute match the way the model reuses it: relative
// to the workspace with slashes, absolute only outside the workspace.
func (t *GlobTool) displayPath(abs string) string {
	workspaceAbs, err := filepath.Abs(t.workspaceRoot)
	if err != nil {
		return filepath.ToSlash(abs)
	}
	rel, err := filepath.Rel(workspaceAbs, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return filepath.ToSlash(abs)
	}
	return filepath.ToSlash(rel)
}

// matchGlobSegments reports whether path segments match pattern segments.
// A "**" segment crosses any number of segments (including none); every other
// segment matches exactly one with path.Match syntax ('*', '?', '[...]').
func matchGlobSegments(pat, rel []string) bool {
	if len(pat) == 0 {
		return len(rel) == 0
	}
	if pat[0] == "**" {
		for i := 0; i <= len(rel); i++ {
			if matchGlobSegments(pat[1:], rel[i:]) {
				return true
			}
		}
		return false
	}
	if len(rel) == 0 {
		return false
	}
	ok, err := path.Match(pat[0], rel[0])
	if err != nil || !ok {
		return false
	}
	return matchGlobSegments(pat[1:], rel[1:])
}

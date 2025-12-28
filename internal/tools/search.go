package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
)

type GrepTool struct {
	config        *config.Config
	workspaceRoot string
}

func NewGrepTool(cfg *config.Config) *GrepTool {
	return &GrepTool{
		config:        cfg,
		workspaceRoot: cfg.Workspace.Root,
	}
}

func (t *GrepTool) Name() string {
	return "grep"
}

func (t *GrepTool) Description() string {
	return "Search for text patterns in files (supports regex)"
}

func (t *GrepTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"pattern": map[string]any{
				"type":        "string",
				"description": "Text/regex pattern to search for",
			},
			"path": map[string]any{
				"type":        "string",
				"description": "Optional: Path to search in (default: workspace root)",
			},
			"file_pattern": map[string]any{
				"type":        "string",
				"description": "Optional: File glob pattern to filter (e.g., '*.go')",
			},
			"context_lines": map[string]any{
				"type":        "integer",
				"description": "Optional: Lines of context around matches",
			},
		},
		"required": []string{"pattern"},
	}
}

func (t *GrepTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Pattern      string `json:"pattern"`
		Path         string `json:"path"`
		FilePattern  string `json:"file_pattern"`
		ContextLines int    `json:"context_lines"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	searchPath := t.workspaceRoot
	if params.Path != "" {
		// Check permissions
		result, err := t.config.CheckPathPermission(params.Path, config.AccessRead)
		if err != nil && result == config.PermissionDenied {
			return nil, fmt.Errorf("access denied: %w", err)
		}

		if !filepath.IsAbs(params.Path) {
			searchPath = filepath.Join(t.workspaceRoot, params.Path)
		} else {
			searchPath = params.Path
		}
	}

	// Build command args
	var cmdArgs []string
	cmdArgs = append(cmdArgs, "-n") // line numbers

	if params.ContextLines > 0 {
		cmdArgs = append(cmdArgs, fmt.Sprintf("-C%d", params.ContextLines))
	}

	if params.FilePattern != "" {
		cmdArgs = append(cmdArgs, "--include", params.FilePattern)
	}

	cmdArgs = append(cmdArgs, "-r", "--", params.Pattern, searchPath)

	// Try ripgrep first, fall back to grep
	cmd := exec.CommandContext(ctx, "rg", cmdArgs...)
	output, err := cmd.CombinedOutput()

	if err != nil {
		// Try regular grep
		cmd = exec.CommandContext(ctx, "grep", cmdArgs...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return map[string]any{
				"matches": []string{},
				"count":   0,
			}, nil
		}
	}

	// Parse results
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	matches := []string{}
	for _, line := range lines {
		if line != "" {
			matches = append(matches, line)
		}
	}

	return map[string]any{
		"matches": matches,
		"count":   len(matches),
	}, nil
}

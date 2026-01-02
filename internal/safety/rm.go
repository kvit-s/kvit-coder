package safety

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// RmRule checks for dangerous rm operations
type RmRule struct{}

// Name returns the rule identifier
func (r *RmRule) Name() string {
	return "rm"
}

// Applies returns true if this is an rm command
func (r *RmRule) Applies(parsed *ParsedCommand) bool {
	return parsed.Binary == "rm"
}

// Check evaluates rm commands for dangerous operations
func (r *RmRule) Check(parsed *ParsedCommand, ctx *Context) *RuleResult {
	recursive, force := parsed.HasAnyDestructiveRmFlags()

	// Only check recursive + force combinations
	if !recursive || !force {
		return AllowResult()
	}

	cfg := ctx.Config.Rm

	// Check each target path
	for _, target := range parsed.Args {
		// Skip flags
		if strings.HasPrefix(target, "-") {
			continue
		}

		absPath := resolvePath(target, ctx.WorkingDir)

		// Always block system-critical paths
		if IsSystemCritical(absPath) {
			return BlockResult("rm_critical",
				fmt.Sprintf("rm -rf on system critical path: %s. "+
					"This operation is blocked for safety.", absPath))
		}

		// Block workspace root if configured
		if cfg.BlockWorkspaceRoot && ctx.IsWorkspaceRoot(absPath) {
			return BlockResult("rm_workspace_root",
				"rm -rf on workspace root directory is blocked. "+
					"Remove contents individually or run manually if you're sure.")
		}

		// Allow in temp directories if configured
		if cfg.AllowInTemp && ctx.IsTempPath(absPath) {
			continue
		}

		// Allow within workspace subdirectories if configured
		if cfg.AllowInWorkspaceCwd && ctx.IsWithinWorkspace(absPath) && !ctx.IsWorkspaceRoot(absPath) {
			continue
		}

		// Outside workspace - prompt or block based on paranoid mode
		if !ctx.IsWithinWorkspace(absPath) {
			if ctx.Config.ParanoidMode {
				return BlockResult("rm_paranoid",
					fmt.Sprintf("paranoid mode: rm -rf outside safe paths blocked: %s", absPath))
			}

			return PromptResult("rm_outside_workspace",
				fmt.Sprintf("rm -rf targets path outside workspace: %s", absPath),
				[]string{absPath})
		}
	}

	return AllowResult()
}

// resolvePath resolves a path to an absolute path
func resolvePath(path string, workingDir string) string {
	// Handle home directory expansion
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			path = filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			path = home
		}
	}

	// Convert to absolute path
	if !filepath.IsAbs(path) {
		path = filepath.Join(workingDir, path)
	}

	return filepath.Clean(path)
}

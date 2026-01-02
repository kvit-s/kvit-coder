package safety

import (
	"fmt"
	"strings"
)

// FindRule checks for dangerous find operations
type FindRule struct{}

// Name returns the rule identifier
func (r *FindRule) Name() string {
	return "find"
}

// Applies returns true if this is a find command
func (r *FindRule) Applies(parsed *ParsedCommand) bool {
	return parsed.Binary == "find"
}

// Check evaluates find commands for dangerous operations
func (r *FindRule) Check(parsed *ParsedCommand, ctx *Context) *RuleResult {
	hasDelete := parsed.HasFlag("delete", "-delete", "--delete")
	hasExecRm := r.hasExecRm(parsed.Args)

	if !hasDelete && !hasExecRm {
		return AllowResult()
	}

	// Find the search path (first non-flag argument)
	searchPath := r.extractSearchPath(parsed.Args)
	if searchPath == "" {
		searchPath = "."
	}

	absPath := resolvePath(searchPath, ctx.WorkingDir)

	// Allow in temp directories
	if ctx.IsTempPath(absPath) {
		return AllowResult()
	}

	// Warn for workspace paths (but allow)
	if ctx.IsWithinWorkspace(absPath) {
		actionDesc := "-delete"
		if hasExecRm {
			actionDesc = "-exec rm"
		}
		return WarnResult("find_delete_workspace",
			fmt.Sprintf("find with %s in workspace: %s", actionDesc, absPath))
	}

	// Block outside workspace
	return BlockResult("find_delete_outside",
		fmt.Sprintf("find with -delete/-exec rm outside safe directories: %s. "+
			"Run manually if you're sure.", absPath))
}

// hasExecRm checks if the find command contains -exec rm or -execdir rm
func (r *FindRule) hasExecRm(args []string) bool {
	for i, arg := range args {
		if (arg == "-exec" || arg == "-execdir" || arg == "-ok" || arg == "-okdir") && i+1 < len(args) {
			nextArg := args[i+1]
			if strings.HasPrefix(nextArg, "rm") || nextArg == "rm" {
				return true
			}
			// Check for full path like /bin/rm
			if strings.HasSuffix(nextArg, "/rm") {
				return true
			}
		}
	}
	return false
}

// extractSearchPath finds the search path in find arguments
// In find, the path comes before any -options
func (r *FindRule) extractSearchPath(args []string) string {
	for _, arg := range args {
		// Skip the "find" subcommand itself if present
		if arg == "find" {
			continue
		}
		// First non-flag argument is typically the path
		if !strings.HasPrefix(arg, "-") && arg != "!" && arg != "(" && arg != ")" {
			return arg
		}
	}
	return ""
}

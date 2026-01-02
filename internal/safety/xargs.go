package safety

import (
	"fmt"
	"strings"
)

// XargsRule checks for dangerous xargs/parallel operations
type XargsRule struct{}

// Name returns the rule identifier
func (r *XargsRule) Name() string {
	return "xargs"
}

// Applies returns true if this is an xargs or parallel command
// Also applies if a pipeline contains xargs/parallel
func (r *XargsRule) Applies(parsed *ParsedCommand) bool {
	if r.isXargsOrParallel(parsed.Binary) {
		return true
	}
	for _, pipe := range parsed.Pipes {
		if r.isXargsOrParallel(pipe.Binary) {
			return true
		}
	}
	return false
}

// isXargsOrParallel checks if the binary is xargs or parallel
func (r *XargsRule) isXargsOrParallel(binary string) bool {
	return binary == "xargs" || binary == "parallel"
}

// Check evaluates xargs/parallel commands for dangerous operations
func (r *XargsRule) Check(parsed *ParsedCommand, ctx *Context) *RuleResult {
	// Check the main command if it's xargs/parallel
	if r.isXargsOrParallel(parsed.Binary) {
		if result := r.checkXargsCommand(parsed); result.Action != Allow {
			return result
		}
	}

	// Check pipeline segments
	for _, pipe := range parsed.Pipes {
		if r.isXargsOrParallel(pipe.Binary) {
			if result := r.checkXargsCommand(pipe); result.Action != Allow {
				return result
			}
		}
	}

	return AllowResult()
}

// checkXargsCommand checks a single xargs/parallel command
func (r *XargsRule) checkXargsCommand(parsed *ParsedCommand) *RuleResult {
	innerCmd := r.extractInnerCommand(parsed)
	if innerCmd == "" {
		return AllowResult()
	}

	// Check if inner command is destructive
	dangerousCmds := []string{"rm", "rmdir", "shred", "unlink", "mv", "dd"}
	for _, danger := range dangerousCmds {
		if innerCmd == danger || strings.HasPrefix(innerCmd, danger+" ") {
			return BlockResult("xargs_destructive",
				fmt.Sprintf("destructive command '%s' in %s pipeline. "+
					"Run manually if you're sure.", innerCmd, parsed.Binary))
		}
		// Check for full path
		if strings.HasSuffix(innerCmd, "/"+danger) {
			return BlockResult("xargs_destructive",
				fmt.Sprintf("destructive command '%s' in %s pipeline. "+
					"Run manually if you're sure.", innerCmd, parsed.Binary))
		}
	}

	return AllowResult()
}

// extractInnerCommand extracts the command that xargs will execute
func (r *XargsRule) extractInnerCommand(parsed *ParsedCommand) string {
	// Common xargs patterns:
	// xargs rm -rf
	// xargs -I {} rm -rf {}
	// xargs -0 rm

	// Skip known xargs flags and find the command
	skipNext := false
	for _, arg := range parsed.Args {
		if skipNext {
			skipNext = false
			continue
		}

		// Skip xargs-specific flags
		if strings.HasPrefix(arg, "-") {
			// Flags that take an argument
			switch arg {
			case "-I", "-i", "-L", "-l", "-n", "-P", "-s", "-E", "-a", "-R":
				skipNext = true
			case "--replace", "--max-lines", "--max-args", "--max-procs", "--max-chars":
				skipNext = true
			}
			continue
		}

		// This is the command to execute
		if arg != "{}" && arg != "\\{\\}" {
			return arg
		}
	}

	return ""
}

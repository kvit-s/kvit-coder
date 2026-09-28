package safety

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// ShellBinaries are recognized shell binaries that can wrap commands
var ShellBinaries = map[string]bool{
	"bash":          true,
	"sh":            true,
	"zsh":           true,
	"ksh":           true,
	"dash":          true,
	"/bin/bash":     true,
	"/usr/bin/bash": true,
	"/bin/sh":       true,
	"/usr/bin/sh":   true,
	"/bin/zsh":      true,
	"/usr/bin/zsh":  true,
	// Windows shells: a `powershell -Command "rm ..."` must unwrap like
	// `bash -c` does, or the rm rules never see the inner command. Matched
	// case-insensitively on Windows (see IsShellBinary).
	"powershell": true,
	"pwsh":       true,
	"cmd":        true,
}

// IsShellBinary returns true if the binary is a recognized shell
func IsShellBinary(binary string) bool {
	// Check exact match
	if ShellBinaries[binary] {
		return true
	}
	// Check base name (for full paths)
	base := filepath.Base(binary)
	if ShellBinaries[base] {
		return true
	}
	if runtime.GOOS == "windows" {
		return IsWindowsShellBinary(binary)
	}
	return false
}

// IsWindowsShellBinary reports whether binary is a recognized shell on
// Windows, folding case and stripping .exe/.bat/.cmd/.ps1 so
// `powershell.exe -Command ...` unwraps like `bash -c` does. It is a separate
// function so tests can exercise Windows matching on any OS.
func IsWindowsShellBinary(binary string) bool {
	base := binary
	if i := strings.LastIndexAny(base, `/\`); i >= 0 {
		base = base[i+1:]
	}
	lower := strings.ToLower(base)
	for _, suffix := range []string{".exe", ".bat", ".cmd", ".ps1"} {
		if strings.HasSuffix(lower, suffix) {
			lower = lower[:len(lower)-len(suffix)]
			break
		}
	}
	if ShellBinaries[lower] {
		return true
	}
	// Full-path keys are unix-only; compare their bases too.
	return ShellBinaries[filepath.Base(lower)]
}

// UnwrapShellCommand recursively unwraps shell invocations like:
// - bash -c 'dangerous command'
// - sh -c "dangerous command"
// - env bash -c 'command'
// Returns a list of inner commands that need to be checked
func UnwrapShellCommand(parsed *ParsedCommand, maxDepth int) ([]*ParsedCommand, error) {
	if maxDepth <= 0 {
		return nil, fmt.Errorf("maximum unwrap depth exceeded")
	}

	// Handle 'env' prefix: env bash -c 'command'
	if parsed.Binary == "env" && len(parsed.Args) > 0 {
		// Find the actual binary in args
		for i, arg := range parsed.Args {
			if IsShellBinary(arg) {
				// Reconstruct the command without 'env'
				newCmd := &ParsedCommand{
					Binary:   arg,
					Flags:    parsed.Flags,
					FlagArgs: parsed.FlagArgs,
					Args:     parsed.Args[i+1:],
					Raw:      parsed.Raw,
				}
				return UnwrapShellCommand(newCmd, maxDepth-1)
			}
		}
		// Not a shell wrapper, return as-is
		return []*ParsedCommand{parsed}, nil
	}

	// Not a shell binary, return as-is
	if !IsShellBinary(parsed.Binary) {
		return []*ParsedCommand{parsed}, nil
	}

	// Check for -c flag which executes a command string
	innerCmd, hasC := parsed.GetFlagArg("c")
	if !hasC || innerCmd == "" {
		// No -c flag, it's just a shell invocation (not wrapping)
		return []*ParsedCommand{parsed}, nil
	}

	// Parse the inner command
	innerParsed, err := ParseCommand(innerCmd)
	if err != nil {
		// Can't parse inner command, return both the wrapper and
		// a minimal representation of the inner command for safety
		return []*ParsedCommand{parsed}, nil
	}

	// Recursively unwrap
	return UnwrapShellCommand(innerParsed, maxDepth-1)
}

// UnwrapAll unwraps the main command and all pipe segments
func UnwrapAll(parsed *ParsedCommand, maxDepth int) ([]*ParsedCommand, error) {
	var results []*ParsedCommand

	// Unwrap the main command
	unwrapped, err := UnwrapShellCommand(parsed, maxDepth)
	if err != nil {
		return nil, err
	}
	results = append(results, unwrapped...)

	// Unwrap each pipe segment
	for _, pipe := range parsed.Pipes {
		pipeUnwrapped, err := UnwrapShellCommand(pipe, maxDepth)
		if err != nil {
			return nil, err
		}
		results = append(results, pipeUnwrapped...)
	}

	return results, nil
}

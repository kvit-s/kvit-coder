//go:build windows

package procutil

import (
	"os/exec"
	"strings"
)

// MediateMCPCommand returns the command and args to spawn for an MCP stdio
// server. On Windows `npx` is `npx.cmd`, which Go's LookPath finds via PATHEXT
// but CreateProcess cannot run directly. When the resolved server command has
// a .cmd/.bat extension (or resolves only via PATHEXT), spawn it as
// `cmd.exe /d /s /c …` with the argument list preserved. The JSON-RPC-over-
// pipes transport itself is platform-neutral.
func MediateMCPCommand(command string, args []string) (string, []string) {
	lower := strings.ToLower(command)
	if strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat") {
		full := append([]string{"/d", "/s", "/c", command}, args...)
		return "cmd.exe", full
	}
	// LookPath without extension may still resolve via PATHEXT (npx -> npx.cmd).
	if !strings.Contains(lower, ".") {
		if resolved, err := exec.LookPath(command); err == nil {
			rl := strings.ToLower(resolved)
			if strings.HasSuffix(rl, ".cmd") || strings.HasSuffix(rl, ".bat") {
				full := append([]string{"/d", "/s", "/c", resolved}, args...)
				return "cmd.exe", full
			}
		}
	}
	return command, args
}

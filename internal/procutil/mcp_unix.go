//go:build !windows

package procutil

// MediateMCPCommand returns the command and args to spawn for an MCP stdio
// server. On unix the resolved command runs directly.
func MediateMCPCommand(command string, args []string) (string, []string) {
	return command, args
}

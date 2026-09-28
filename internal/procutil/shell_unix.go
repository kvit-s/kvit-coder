//go:build !windows

package procutil

// ResolveShell returns the POSIX shell to run agent commands through. On
// unix it is always "sh" (exec.LookPath resolution happens at spawn time so
// the error surfaces where the command runs).
func ResolveShell() (string, error) {
	return "sh", nil
}

// ShellNameForPrompt reports the shell for the environment prompt block.
func ShellNameForPrompt() string {
	return ""
}

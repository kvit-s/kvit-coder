//go:build windows

package procutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// ResolveShell finds the POSIX shell agent commands run through on Windows.
// Git for Windows ships sh.exe (plus grep, find, and git itself, which
// checkpoints already require), so: sh.exe on PATH first, else the standard
// Git install locations. When none is found it fails fast naming Git for
// Windows rather than surfacing a bare exec "sh not found".
func ResolveShell() (string, error) {
	if path, err := exec.LookPath("sh.exe"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("sh"); err == nil {
		return path, nil
	}
	candidates := []string{}
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		base := os.Getenv(env)
		if base == "" {
			continue
		}
		candidates = append(candidates, filepath.Join(base, "Git", "usr", "bin", "sh.exe"))
		candidates = append(candidates, filepath.Join(base, "Git", "bin", "sh.exe"))
	}
	// Also try the default install roots directly in case the env vars are
	// missing (e.g. 32-bit process on 64-bit Windows).
	candidates = append(candidates,
		`C:\Program Files\Git\usr\bin\sh.exe`,
		`C:\Program Files\Git\bin\sh.exe`,
		`C:\Program Files (x86)\Git\usr\bin\sh.exe`,
		`C:\Program Files (x86)\Git\bin\sh.exe`,
	)
	seen := map[string]bool{}
	for _, c := range candidates {
		if seen[c] {
			continue
		}
		seen[c] = true
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("no POSIX shell found: install Git for Windows (https://git-scm.com/download/win) so sh.exe is on PATH or under %%ProgramFiles%%\\Git")
}

// ShellNameForPrompt reports the resolved shell for the environment prompt
// block, so transcripts say what ran. Empty when no shell resolves.
func ShellNameForPrompt() string {
	if sh, err := ResolveShell(); err == nil {
		return sh
	}
	return ""
}

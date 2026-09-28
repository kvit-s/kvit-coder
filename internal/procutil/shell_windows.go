//go:build windows

package procutil

import (
	"fmt"
	"os"
	"os/exec"

	"golang.org/x/sys/windows/registry"
)

// ResolveShell finds the POSIX shell agent commands run through on Windows.
// Git for Windows ships sh.exe (plus grep and find), so: sh.exe on PATH
// first, then sh.exe derived from git.exe's location — a default install
// puts only Git\cmd (git.exe) on PATH, not Git\usr\bin (sh.exe), so "git
// works but sh.exe is not on PATH" is the common case, not a broken
// install — then the registry's install path, then the standard and
// per-user install locations. When none is found it fails fast naming Git
// for Windows rather than surfacing a bare exec "sh not found".
func ResolveShell() (string, error) {
	if path, err := exec.LookPath("sh.exe"); err == nil {
		return path, nil
	}
	if path, err := exec.LookPath("sh"); err == nil {
		return path, nil
	}
	candidates := []string{}
	if git, err := exec.LookPath("git.exe"); err == nil {
		candidates = append(candidates, gitShCandidates(git)...)
	}
	if root := gitInstallPathFromRegistry(); root != "" {
		candidates = append(candidates, root+"\\usr\\bin\\sh.exe", root+"\\bin\\sh.exe")
	}
	for _, base := range shellSearchBases(os.Getenv) {
		candidates = append(candidates, base+"\\Git\\usr\\bin\\sh.exe", base+"\\Git\\bin\\sh.exe")
	}
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

// gitInstallPathFromRegistry reads the install root Git for Windows records
// at setup time. Per-user installs land under HKCU, machine-wide under
// HKLM; either may be absent, in which case this returns "".
func gitInstallPathFromRegistry() string {
	for _, hive := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		k, err := registry.OpenKey(hive, `SOFTWARE\GitForWindows`, registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		path, _, err := k.GetStringValue("InstallPath")
		k.Close()
		if err == nil && path != "" {
			return path
		}
	}
	return ""
}

// ShellNameForPrompt reports the resolved shell for the environment prompt
// block, so transcripts say what ran. Empty when no shell resolves.
func ShellNameForPrompt() string {
	if sh, err := ResolveShell(); err == nil {
		return sh
	}
	return ""
}

package procutil

import "strings"

// Windows shell discovery helpers, kept free of build tags and OS-specific
// calls so they are unit-testable on any platform. shell_windows.go feeds
// them the real environment; the installer script
// (scripts/portable-install.ps1) mirrors the same search order.
//
// Background: a default Git for Windows install puts only Git\cmd (git.exe)
// on PATH, not Git\usr\bin (sh.exe). So "git works, sh.exe not found" is the
// common case, not a broken install — and the fix is to derive sh.exe from
// git.exe's location rather than demand sh.exe itself be on PATH.

// gitShCandidates derives sh.exe locations from the path of a git.exe. It
// walks up to two levels above git.exe's directory trying the usr\bin, bin
// and same-dir layouts, which covers Git\cmd\git.exe, Git\bin\git.exe and
// Git\mingw64\bin\git.exe. A bare name with no directory yields nothing.
func gitShCandidates(gitExe string) []string {
	p := strings.ReplaceAll(gitExe, "/", "\\")
	dir, ok := splitDir(p)
	if !ok {
		return nil
	}
	var out []string
	add := func(d string) {
		out = append(out, d+"\\usr\\bin\\sh.exe", d+"\\bin\\sh.exe", d+"\\sh.exe")
	}
	add(dir)
	if parent, ok := splitDir(dir); ok {
		add(parent)
		if grand, ok := splitDir(parent); ok {
			add(grand)
		}
	}
	return out
}

func splitDir(p string) (string, bool) {
	i := strings.LastIndex(p, "\\")
	if i <= 0 {
		return "", false
	}
	return p[:i], true
}

// shellSearchBases returns the install-root bases to look under for a Git
// directory, in order: the ProgramFiles variants (ProgramW6432 matters to a
// 32-bit process, where ProgramFiles points at the x86 tree), then the
// per-user install root (no admin rights needed), then hardcoded defaults
// for when the environment itself is missing entries.
func shellSearchBases(getenv func(string) string) []string {
	var bases []string
	for _, env := range []string{"ProgramFiles", "ProgramFiles(x86)", "ProgramW6432"} {
		if b := getenv(env); b != "" {
			bases = append(bases, b)
		}
	}
	if lad := getenv("LocalAppData"); lad != "" {
		bases = append(bases, lad+"\\Programs")
	} else if profile := getenv("USERPROFILE"); profile != "" {
		bases = append(bases, profile+"\\AppData\\Local\\Programs")
	}
	bases = append(bases, `C:\Program Files`, `C:\Program Files (x86)`)
	return bases
}

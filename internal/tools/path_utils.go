package tools

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// NormalizeAndValidatePath normalizes a path and checks if it's outside workspace
// Returns: (normalizedPath, isOutside, error)
func NormalizeAndValidatePath(workspaceRoot, inputPath string) (string, bool, error) {
	// Step 1: Expand ~ to home directory
	path := inputPath
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, err
		}
		path = filepath.Join(home, path[2:])
	}

	// Step 2: Convert to absolute path
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(workspaceRoot, path)
	}

	// Step 3: Clean the path to resolve .. and .
	absPath = filepath.Clean(absPath)
	workspaceAbs := filepath.Clean(workspaceRoot)

	if runtime.GOOS == "windows" {
		return absPath, pathOutsideWorkspaceWindows(workspaceAbs, absPath), nil
	}
	outside, err := pathOutsideWorkspace(workspaceAbs, absPath)
	if err != nil {
		return "", false, err
	}
	return absPath, outside, nil
}

// pathOutsideWorkspace reports whether absPath escapes workspaceAbs on unix.
// An unrelatable pair returns an error; callers skip such paths, preserving
// the long-standing contract the shell tests rely on (a relative workspace
// root like "." against an absolute path). Windows cross-volume and
// case-folded checks live in pathOutsideWorkspaceWindows, used on Windows.
func pathOutsideWorkspace(workspaceAbs, absPath string) (bool, error) {
	relPath, err := filepath.Rel(workspaceAbs, absPath)
	if err != nil {
		return false, err
	}
	return relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)), nil
}

// pathOutsideWorkspaceWindows is the case-folded containment check, split out
// so tests can exercise Windows semantics on any OS. It parses Windows paths
// explicitly (drive letters, UNC shares, extended \\?\ prefixes, both
// separators) rather than via filepath, whose VolumeName/Rel follow the host
// OS and cannot evaluate C:\ paths on linux. C:\Proj and c:\proj\..\proj are
// the same directory; a case-sensitive check can be walked around
// (..\PROJ\.kvit-coder), so every comparison here folds case.
func pathOutsideWorkspaceWindows(workspaceAbs, absPath string) bool {
	volW, compsW, okW := parseWindowsPath(workspaceAbs)
	volA, compsA, okA := parseWindowsPath(absPath)
	if !okW || !okA {
		// Drive-relative (C:foo) or otherwise unparseable: at least do not
		// panic. Fail closed — outside, never inside on a guess.
		return true
	}
	if !strings.EqualFold(volW, volA) {
		// Another volume (C:\ vs D:\), another UNC share, or extended
		// \\?\... against a plain path: outside.
		return true
	}
	if len(compsA) < len(compsW) {
		return true
	}
	for i := range compsW {
		if !strings.EqualFold(compsW[i], compsA[i]) {
			return true
		}
	}
	return false
}

// parseWindowsPath splits an absolute Windows path into its volume and its
// cleaned components. Both separators count; "." drops out and ".." pops,
// without ever popping past the root. It reports false for drive-relative
// (C:foo) and other forms with no volume, which callers treat as outside.
func parseWindowsPath(p string) (vol string, comps []string, ok bool) {
	// Normalise separators first so UNC, extended, and drive parsing each see
	// one form.
	s := strings.ReplaceAll(p, "/", "\\")
	lower := strings.ToLower(s)
	// Extended-length \\?\... is its own volume: \\?\C:\x vs C:\x must not
	// compare equal, or autocrlf-style rewrites are not the only thing that
	// round-trips wrong. Keep the \\?\ prefix in the volume so extended and
	// plain never fold together; \\?\UNC\host\share is its own share too.
	if strings.HasPrefix(lower, "\\\\?\\") {
		after := s[4:]
		afterLower := strings.ToLower(after)
		if strings.HasPrefix(afterLower, "unc\\") {
			// \\?\UNC\host\share\...
			rem := after[4:]
			host, rem2, found := cutWindowsComponent(rem)
			if !found {
				return "", nil, false
			}
			share, rem3, found := cutWindowsComponent(rem2)
			if !found {
				// Host without share: volume is host, no components.
				vol = "\\\\?\\UNC\\" + host
				return vol, cleanWindowsComps(""), true
			}
			vol = "\\\\?\\UNC\\" + host + "\\" + share
			return vol, cleanWindowsComps(rem3), true
		}
		// \\?\C:\... : volume is \\?\ + drive.
		if len(after) >= 2 && after[1] == ':' && isDriveLetter(after[0]) {
			vol = "\\\\?\\" + after[:2]
			return vol, cleanWindowsComps(after[2:]), true
		}
		return "", nil, false
	}

	// UNC \\host\share\...
	if strings.HasPrefix(s, "\\\\") {
		rem := s[2:]
		host, rem2, found := cutWindowsComponent(rem)
		if !found || host == "" {
			return "", nil, false
		}
		share, rem3, found := cutWindowsComponent(rem2)
		if !found {
			return "", nil, false
		}
		vol = "\\\\" + host + "\\" + share
		return vol, cleanWindowsComps(rem3), true
	}

	// Drive C:\... (absolute) — C:foo (drive-relative) has no slash and fails.
	if len(s) >= 3 && s[1] == ':' && isDriveLetter(s[0]) && s[2] == '\\' {
		vol = s[:2]
		return vol, cleanWindowsComps(s[2:]), true
	}
	if len(s) == 2 && s[1] == ':' && isDriveLetter(s[0]) {
		// Bare drive root "C:" — treat as its root.
		return s[:2], nil, true
	}
	return "", nil, false
}

func isDriveLetter(c byte) bool {
	return (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

// cutWindowsComponent splits the next \-separated component off rem. It
// reports false when no separator follows (last component).
func cutWindowsComponent(rem string) (head, tail string, found bool) {
	if i := strings.IndexByte(rem, '\\'); i >= 0 {
		return rem[:i], rem[i+1:], true
	}
	return rem, "", false
}

// cleanWindowsComps resolves "." and ".." over \-separated components,
// dropping empties (duplicate separators, trailing slash). ".." past the root
// stays (it escapes), so containment sees the walk-out.
func cleanWindowsComps(rest string) []string {
	var out []string
	escaped := false
	for _, part := range strings.Split(rest, "\\") {
		switch part {
		case "", ".":
			continue
		case "..":
			if len(out) > 0 {
				out = out[:len(out)-1]
			} else {
				escaped = true
			}
		default:
			out = append(out, part)
		}
	}
	if escaped {
		// Mark the escape so a prefix comparison cannot hide it: prepend a
		// ".." the workspace (which never escapes) cannot share.
		out = append([]string{".."}, out...)
	}
	return out
}

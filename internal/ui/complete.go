package ui

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"unicode"
)

// Path completion for the input composer (Tab key).
//
// The composer is a free-text box, not a shell, so completion is
// deliberately small: take the whitespace-delimited token left of the
// cursor, list directory entries that extend it, and complete the
// longest common prefix. Repeated Tab cycles through the candidates.
// A second Tab press with untouched text cycles; any other key restarts.

// maxCompletionCandidates caps how many filesystem matches one Tab keeps
// for cycling. The list is sorted dirs-first, so truncating drops files
// before directories.
const maxCompletionCandidates = 50

// maxCompletionDisplay caps how many candidates View renders under the
// input. The rest fold into an "…and N more" line.
const maxCompletionDisplay = 10

// tokenStartForLine finds the token being completed: the run of non-space
// characters left of cursor on this hard line. A token inside quotes may
// contain spaces ("my dir/fi" completes "my dir/fi"), so quoted regions
// are scanned forward for first: an opening quote without its close means
// the token runs back to just after the quote. A quote only opens after a
// non-word character, so the apostrophe in "it's" stays literal text
// instead of quoting the rest of the line. Escaped quotes are not
// special-cased; a path containing \" is rare enough to complete wrong.
func tokenStartForLine(line []rune, cursor int) (start int, inQuotes bool) {
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(line) {
		cursor = len(line)
	}
	var quote rune
	openPos := -1
	lastBreak := 0 // offset after the last delimiter outside quotes
	for i := 0; i < cursor; i++ {
		r := line[i]
		if quote != 0 {
			if r == quote {
				quote = 0
				openPos = -1
				lastBreak = i + 1
			}
		} else if r == '"' || r == '\'' || r == '`' {
			if i == 0 || (!unicode.IsLetter(line[i-1]) && !unicode.IsDigit(line[i-1])) {
				quote = r
				openPos = i
			}
			// A mid-word quote (the apostrophe in "it's") is literal
			// text: no region opens and the token runs through it.
		} else if unicode.IsSpace(r) {
			lastBreak = i + 1
		}
	}
	if quote != 0 {
		return openPos + 1, true
	}
	return lastBreak, false
}

// splitCompletionToken separates a raw token into the parts completion
// preserves verbatim (lead, at, trailing) and the filesystem path it
// looks up (core):
//
//	"(@src/ma," -> lead="(", at="@", core="src/ma", trailing=","
//
// The @ prefix is kvit-coder's file-reference marker; it stays in the
// text but is not part of the directory lookup. Leading openers and one
// run of trailing closers/punctuation survive a chat sentence around a
// path ("see (src/ma), ok") without becoming part of the lookup.
func splitCompletionToken(token string) (lead, at, core, trailing string) {
	runes := []rune(token)
	i := 0
	for i < len(runes) {
		r := runes[i]
		if r == '(' || r == '[' || r == '{' || r == '<' {
			i++
		} else {
			break
		}
	}
	lead = string(runes[:i])
	rest := runes[i:]
	// Trailing sentence punctuation first, so a quoted insertion with a
	// comma after it ("@a b/",) still exposes its closing quote.
	end := len(rest)
	for end > 0 {
		r := rest[end-1]
		if r == ',' || r == ';' || r == ':' || r == '!' || r == '?' ||
			r == ')' || r == ']' || r == '}' || r == '>' {
			end--
		} else {
			break
		}
	}
	trailing = string(rest[end:])
	rest = rest[:end]
	// A quoted insertion ("@a b/") re-enters as one token when Tab
	// follows the closing quote: drop that wrapping pair so the lookup
	// sees @a b/ again rather than the quotes.
	if len(rest) >= 2 && (rest[0] == '"' || rest[0] == '\'' || rest[0] == '`') &&
		rest[len(rest)-1] == rest[0] {
		rest = rest[1 : len(rest)-1]
	}
	if len(rest) > 0 && rest[0] == '@' {
		at = "@"
		rest = rest[1:]
	}
	core = string(rest)
	return lead, at, core, trailing
}

// splitDirPrefix cuts a path at its last separator: "src/ma" gives
// dirPart "src/" and filePrefix "ma"; "ma" gives "" and "ma". Only "/"
// separates, except on Windows where "\\" does too ('\\' is a valid
// filename character on unix, so it must not split there).
func splitDirPrefix(pathPart string) (dirPart, filePrefix string) {
	idx := strings.LastIndex(pathPart, "/")
	if runtime.GOOS == "windows" {
		if b := strings.LastIndex(pathPart, "\\"); b > idx {
			idx = b
		}
	}
	if idx < 0 {
		return "", pathPart
	}
	return pathPart[:idx+1], pathPart[idx+1:]
}

// resolveCompletionDir maps a dirPart back to the filesystem. Empty means
// the completion base (the workspace root); "~/" expands the home
// directory; absolute stays; anything else joins onto the base. An empty
// base falls back to the process working directory.
func resolveCompletionDir(baseDir, dirPart string) string {
	base := baseDir
	if base == "" {
		if cwd, err := os.Getwd(); err == nil {
			base = cwd
		}
	}
	if dirPart == "" {
		return base
	}
	if dirPart == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
		return base
	}
	if strings.HasPrefix(dirPart, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(dirPart, "~/"))
		}
		return base
	}
	if filepath.IsAbs(dirPart) {
		return dirPart
	}
	return filepath.Join(base, dirPart)
}

// listPathCompletions returns the full replacements for pathPart: dirPart
// plus each matching entry name, directories with a trailing slash,
// sorted dirs-first then alphabetically, capped at
// maxCompletionCandidates. total is the uncapped match count, for the
// "…and N more" line. A nil slice with zero total means the directory
// could not be read or nothing matched.
func listPathCompletions(baseDir, pathPart string) (candidates []string, total int) {
	if pathPart == "~" {
		// A bare tilde completes to ~/ so the next Tab lists home.
		return []string{"~/"}, 1
	}
	dirPart, filePrefix := splitDirPrefix(pathPart)
	dir := resolveCompletionDir(baseDir, dirPart)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0
	}
	type match struct {
		name  string
		isDir bool
	}
	var matches []match
	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, filePrefix) {
			continue
		}
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(filePrefix, ".") {
			continue
		}
		isDir := e.IsDir()
		if !isDir && e.Type()&os.ModeSymlink != 0 {
			// A symlink to a directory completes with a slash like
			// one, so Tab can descend through it.
			if info, err := e.Info(); err == nil && info.IsDir() {
				isDir = true
			}
		}
		matches = append(matches, match{name, isDir})
	}
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].isDir != matches[j].isDir {
			return matches[i].isDir
		}
		return matches[i].name < matches[j].name
	})
	total = len(matches)
	if len(matches) > maxCompletionCandidates {
		matches = matches[:maxCompletionCandidates]
	}
	for _, m := range matches {
		c := dirPart + m.name
		if m.isDir {
			c += "/"
		}
		candidates = append(candidates, c)
	}
	return candidates, total
}

// commonPathPrefix is the longest run of leading runes every candidate
// shares (rune-wise, so a multi-byte name cannot be cut mid-rune).
func commonPathPrefix(candidates []string) string {
	if len(candidates) == 0 {
		return ""
	}
	prefix := []rune(candidates[0])
	for _, c := range candidates[1:] {
		r := []rune(c)
		i := 0
		for i < len(prefix) && i < len(r) && prefix[i] == r[i] {
			i++
		}
		prefix = prefix[:i]
		if len(prefix) == 0 {
			break
		}
	}
	return string(prefix)
}

// buildCompletionToken reassembles a token after its core was replaced,
// quoting when the completed path holds a space the user did not quote:
// @"my dir/" stays one token for splitImageFields either way, bare
// @my dir/ would not. The quote wraps the @ marker too ("@a b/"), which
// the image-reference parser accepts.
func buildCompletionToken(lead, at, core, trailing string, inQuotes bool) string {
	tok := at + core
	if !inQuotes && strings.Contains(core, " ") && core != "" {
		return lead + `"` + tok + `"` + trailing
	}
	return lead + tok + trailing
}

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
// Completion is @-triggered file search, not shell prefix expansion: the
// token left of the cursor must hold an @path reference, and the path
// part fuzzy-matches workspace files. A token without @ never completes;
// Tab there indents (empty token) or does nothing. Matching is an ordered,
// case-insensitive subsequence over the relative path, so "@sn-p" finds
// "docs/redesign-mcp.md" as well as true substrings. One match replaces
// in place; several list under the input. Repeated Tab cycles through the
// candidates. A second Tab press with untouched text cycles; any other
// key restarts.

// maxCompletionCandidates caps how many filesystem matches one Tab keeps
// for cycling. The list is ranked best-first, so truncating drops the
// worst matches.
const maxCompletionCandidates = 50

// maxCompletionDisplay caps how many candidates View renders under the
// input. The rest fold into an "…and N more" line.
const maxCompletionDisplay = 10

// maxCompletionWalk caps how many files and directories the recursive
// search visits per Tab press, so a huge workspace stays interactive.
// The walk still reports every match up to the candidate cap; beyond the
// walk cap the "…and N more" line only counts what was seen.
const maxCompletionWalk = 10000

// completionSkipDirs are directory names the recursive search never
// descends into. .git would flood every short pattern with objects;
// the rest are the usual build-dependency forests.
var completionSkipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"__pycache__":  true,
	".venv":        true,
	"venv":         true,
	"target":       true,
	"dist":         true,
	"build":        true,
	".hg":          true,
	".svn":         true,
}

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

// listPathCompletions returns the full replacements for pathPart, ranked
// best match first and capped at maxCompletionCandidates. total is the
// uncapped match count, for the "…and N more" line. A nil slice with zero
// total means nothing matched.
//
// pathPart is the @-stripped core: absolute and ~/ paths list one
// directory with fuzzy name matching (walking / or $HOME recursively
// would be absurd). A relative path with a slash browses its directory
// when the directory exists ("docs/re" searches inside docs/); anything
// else — a bare word like "sn-p" or a slash into a missing directory —
// searches the whole workspace recursively, so "@sn-p" finds
// "docs/redesign-mcp.md".
func listPathCompletions(baseDir, pathPart string) (candidates []string, total int) {
	if pathPart == "~" {
		// A bare tilde completes to ~/ so the next Tab lists home.
		return []string{"~/"}, 1
	}
	if strings.HasPrefix(pathPart, "~/") || filepath.IsAbs(pathPart) {
		dirPart, filePattern := splitDirPrefix(pathPart)
		return matchEntriesInDir(resolveCompletionDir(baseDir, dirPart), dirPart, filePattern)
	}
	if strings.Contains(pathPart, "/") {
		dirPart, filePattern := splitDirPrefix(pathPart)
		if info, err := os.Stat(resolveCompletionDir(baseDir, dirPart)); err == nil && info.IsDir() {
			return matchEntriesInDir(resolveCompletionDir(baseDir, dirPart), dirPart, filePattern)
		}
		return recursiveFuzzyCompletions(baseDir, pathPart)
	}
	if pathPart == "" {
		// Bare "@": browse the workspace root instead of matching
		// every file in it.
		return matchEntriesInDir(resolveCompletionDir(baseDir, ""), "", "")
	}
	return recursiveFuzzyCompletions(baseDir, pathPart)
}

// matchEntriesInDir fuzzy-matches pattern against the names in one
// directory. dir is the filesystem path; dirPart is its prefix in the
// replacement text. An empty pattern lists everything dirs-first.
func matchEntriesInDir(dir, dirPart, pattern string) (candidates []string, total int) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, 0
	}
	showHidden := strings.HasPrefix(pattern, ".")
	type match struct {
		name  string
		isDir bool
		score int
	}
	var matches []match
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") && !showHidden {
			continue
		}
		if pattern != "" {
			score, ok := fuzzyScore(pattern, name)
			if !ok {
				continue
			}
			isDir := e.IsDir()
			if !isDir && e.Type()&os.ModeSymlink != 0 {
				if info, err := e.Info(); err == nil && info.IsDir() {
					isDir = true
				}
			}
			matches = append(matches, match{name, isDir, score})
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
		matches = append(matches, match{name, isDir, 0})
	}
	if pattern == "" {
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].isDir != matches[j].isDir {
				return matches[i].isDir
			}
			return matches[i].name < matches[j].name
		})
	} else {
		sort.Slice(matches, func(i, j int) bool {
			if matches[i].score != matches[j].score {
				return matches[i].score < matches[j].score
			}
			if matches[i].isDir != matches[j].isDir {
				return matches[i].isDir
			}
			return matches[i].name < matches[j].name
		})
	}
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

// recursiveFuzzyCompletions walks the workspace and fuzzy-matches
// pattern against each relative path, so a bare "@sn-p" finds
// "docs/redesign-mcp.md" without its directory. Results rank by score,
// then alphabetically; directories keep a trailing slash so Tab can
// descend into them.
func recursiveFuzzyCompletions(baseDir, pattern string) (candidates []string, total int) {
	base := baseDir
	if base == "" {
		if cwd, err := os.Getwd(); err == nil {
			base = cwd
		}
	}
	if base == "" {
		return nil, 0
	}
	showHidden := strings.HasPrefix(pattern, ".")
	type match struct {
		rel   string
		isDir bool
		score int
	}
	var matches []match
	visited := 0
	_ = filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if visited >= maxCompletionWalk {
			return filepath.SkipAll
		}
		visited++
		if path == base {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			if completionSkipDirs[name] {
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") && !showHidden {
				return filepath.SkipDir
			}
		} else if strings.HasPrefix(name, ".") && !showHidden {
			// Dotfiles still need a hidden-parent check below;
			// skip the file itself here.
			if !strings.Contains(pattern, "/") {
				return nil
			}
		}
		rel, err := filepath.Rel(base, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if !showHidden {
			hidden := false
			for _, part := range strings.Split(rel, "/") {
				if strings.HasPrefix(part, ".") {
					hidden = true
					break
				}
			}
			if hidden {
				return nil
			}
		}
		score, ok := fuzzyScore(pattern, rel)
		if !ok {
			return nil
		}
		isDir := d.IsDir()
		if !isDir && d.Type()&os.ModeSymlink != 0 {
			if info, err := d.Info(); err == nil && info.IsDir() {
				isDir = true
			}
		}
		matches = append(matches, match{rel, isDir, score})
		return nil
	})
	sort.Slice(matches, func(i, j int) bool {
		if matches[i].score != matches[j].score {
			return matches[i].score < matches[j].score
		}
		return matches[i].rel < matches[j].rel
	})
	total = len(matches)
	if len(matches) > maxCompletionCandidates {
		matches = matches[:maxCompletionCandidates]
	}
	for _, m := range matches {
		c := m.rel
		if m.isDir {
			c += "/"
		}
		candidates = append(candidates, c)
	}
	return candidates, total
}

// fuzzyScore reports whether every rune of pattern appears in candidate
// in order (case-insensitive), with a score where lower ranks better.
// Gaps cost, word-boundary and consecutive hits discount, and longer
// candidates pay their length — so a true substring outranks a sparse
// subsequence, and "plan" outranks "redesign-plan" for "plan". An empty
// pattern matches everything at zero.
func fuzzyScore(pattern, candidate string) (int, bool) {
	if pattern == "" {
		return 0, true
	}
	p := []rune(strings.ToLower(pattern))
	c := []rune(strings.ToLower(candidate))
	score := 0
	prev := -1
	for _, pc := range p {
		found := -1
		for i := prev + 1; i < len(c); i++ {
			if c[i] == pc {
				found = i
				break
			}
		}
		if found < 0 {
			return 0, false
		}
		gap := found - prev - 1
		score += gap * 10
		if found == 0 || c[found-1] == '/' || c[found-1] == '-' ||
			c[found-1] == '_' || c[found-1] == '.' || c[found-1] == ' ' {
			score -= 5
		}
		if gap == 0 {
			score -= 3
		}
		prev = found
	}
	score += len(c)
	return score, true
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

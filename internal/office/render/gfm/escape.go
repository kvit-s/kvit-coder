package gfm

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

type context int

const (
	blockContext context = iota
	headingContext
	cellContext
)

type escapeOpts struct {
	atLineStart      bool
	styled           bool
	trailingActive   bool
	trailingNonspace bool
	trailingDelims   delims
	inLabel          bool
}

const (
	slotStar = iota
	slotUnderscore
	slotTilde
	slotBacktick
	slotBracket
	slotDollar
)

// delims is the set of pairable delimiters * _ ~ ` ] $ that later output on
// the same line will emit.
type delims [6]bool

func delimSlot(c rune) int {
	switch c {
	case '*':
		return slotStar
	case '_':
		return slotUnderscore
	case '~':
		return slotTilde
	case '`':
		return slotBacktick
	case ']':
		return slotBracket
	case '$':
		return slotDollar
	}
	return -1
}

func (d *delims) insert(c rune) {
	if slot := delimSlot(c); slot >= 0 {
		d[slot] = true
	}
}

func (d *delims) insertClosers(text string) {
	if !strings.ContainsAny(text, "*_~`]$") {
		return
	}
	chars := []rune(text)
	for j := 0; j < len(chars); {
		end := runEnd(chars, j)
		if slot := partnerSlot(chars, j, end); slot >= 0 {
			d[slot] = true
		}
		j = end
	}
}

func (d *delims) union(o delims) {
	for i, set := range o {
		d[i] = d[i] || set
	}
}

func runEnd(chars []rune, j int) int {
	end := j + 1
	for end < len(chars) && chars[end] == chars[j] {
		end++
	}
	return end
}

// partnerSlot reports the slot of the delimiter run j..end when it can pair:
// backticks and ] always can (code spans ignore escapes, brackets pair as
// link structure); * _ ~ only where flanking lets them close; $ only after a
// non-space and before a non-digit.
func partnerSlot(chars []rune, j, end int) int {
	slot := delimSlot(chars[j])
	if slot < 0 {
		return -1
	}
	var closes bool
	switch chars[j] {
	case '`', ']':
		closes = true
	case '$':
		closes = canCloseMath(chars, j, end)
	default:
		closes = canClose(chars, j, end)
	}
	if !closes {
		return -1
	}
	return slot
}

func neighbours(chars []rune, j, end int) (prev, next rune, hasPrev, hasNext bool) {
	if j > 0 {
		prev, hasPrev = chars[j-1], true
	}
	if end < len(chars) {
		next, hasNext = chars[end], true
	}
	return
}

func canCloseMath(chars []rune, j, end int) bool {
	prev, next, hasPrev, hasNext := neighbours(chars, j, end)
	return !(hasPrev && unicode.IsSpace(prev)) && !(hasNext && isASCIIDigit(next))
}

// canClose approximates right-flanking with the intraword exclusion for _;
// unknown neighbours at the edges assume the worst, and the punctuation test
// stays ASCII so an unclassified character never suppresses a real closer.
func canClose(chars []rune, j, end int) bool {
	prev, next, hasPrev, hasNext := neighbours(chars, j, end)
	if hasPrev && unicode.IsSpace(prev) {
		return false
	}
	if hasPrev && isASCIIPunct(prev) && hasNext && isAlnum(next) {
		return false
	}
	return chars[j] != '_' || !(hasPrev && isAlnum(prev) && hasNext && isAlnum(next))
}

// escapable holds every character escapeText may escape or rewrite; digits
// matter only where a block line can start.
const escapable = `\$]` + "`" + `*_~[<!|&#-+>=`

func escapeText(text string, ctx context, o escapeOpts) string {
	lineStarts := ctx == blockContext && (o.atLineStart || strings.Contains(text, "\n"))
	if !strings.ContainsAny(text, escapable) && !(lineStarts && strings.ContainsAny(text, "0123456789")) && utf8.ValidString(text) {
		return text
	}
	return escapeRunes(text, ctx, o)
}

func escapeRunes(text string, ctx context, o escapeOpts) string {
	chars := []rune(text)
	last := [6]int{-1, -1, -1, -1, -1, -1}
	for j := 0; j < len(chars); {
		end := runEnd(chars, j)
		if slot := partnerSlot(chars, j, end); slot >= 0 {
			last[slot] = end - 1
		}
		j = end
	}
	var out strings.Builder
	out.Grow(len(text) + 8)
	lineHasContent := !(o.atLineStart && ctx == blockContext)
	for i := 0; i < len(chars); i++ {
		c := chars[i]
		if c == '\n' {
			out.WriteByte('\n')
			if ctx == blockContext {
				lineHasContent = false
			}
			continue
		}
		startOfLine := !lineHasContent
		if !unicode.IsSpace(c) {
			lineHasContent = true
		}
		var next rune
		hasNext := i+1 < len(chars)
		if hasNext {
			next = chars[i+1]
		}
		nextNonspace := o.trailingActive || o.trailingNonspace
		if hasNext {
			nextNonspace = !unicode.IsSpace(next)
		}
		paired := func(slot int) bool {
			return o.trailingActive || o.trailingDelims[slot] || last[slot] > i
		}
		var escape bool
		switch {
		case c == '\\':
			escape = true
		case c == '$':
			escape = nextNonspace && paired(slotDollar)
		case c == ']' && o.inLabel:
			escape = true
		case c == '`':
			escape = o.styled || paired(slotBacktick)
		case c == '*':
			escape = o.styled || startOfLine || nextNonspace && paired(slotStar)
		case c == '_':
			intraword := i > 0 && isAlnum(chars[i-1]) && hasNext && isAlnum(next)
			escape = o.styled || nextNonspace && !intraword && paired(slotUnderscore)
		case c == '~':
			escape = o.styled || nextNonspace && paired(slotTilde)
		case c == '[':
			escape = o.inLabel || paired(slotBracket)
		case c == '<':
			escape = hasNext && (isASCIILetter(next) || next == '/' || next == '!' || next == '?')
		case c == '!':
			escape = !hasNext && o.trailingActive
		case c == '|' && ctx == cellContext:
			escape = true
		case c == '&' && entityAhead(chars[i:]):
			out.WriteString("&amp;")
			continue
		case c == '#' && startOfLine:
			j := i
			for j < len(chars) && chars[j] == '#' {
				j++
			}
			escape = j == len(chars) || unicode.IsSpace(chars[j])
		case c == '-' && startOfLine:
			escape = !nextNonspace || lineIsOnly(chars[i:], '-')
		case c == '+' && startOfLine:
			escape = !nextNonspace
		case c == '>' && startOfLine:
			escape = true
		case c == '=' && startOfLine:
			escape = lineIsOnly(chars[i:], '=')
		case isASCIIDigit(c) && startOfLine:
			j := i
			for j < len(chars) && isASCIIDigit(chars[j]) {
				j++
			}
			if j < len(chars) && (chars[j] == '.' || chars[j] == ')') && (j+1 == len(chars) || unicode.IsSpace(chars[j+1])) {
				out.WriteString(string(chars[i:j]))
				out.WriteByte('\\')
				out.WriteRune(chars[j])
				i = j
				continue
			}
		}
		if escape {
			out.WriteByte('\\')
		}
		out.WriteRune(c)
	}
	return out.String()
}

func lineIsOnly(chars []rune, c rune) bool {
	for _, ch := range chars {
		if ch == '\n' {
			break
		}
		if ch != c && ch != ' ' && ch != '\t' {
			return false
		}
	}
	return true
}

func entityAhead(chars []rune) bool {
	if len(chars) > 1 && chars[1] == '#' {
		return true
	}
	i := 1
	for i < len(chars) && (isASCIILetter(chars[i]) || isASCIIDigit(chars[i])) {
		i++
	}
	return i > 1 && i < len(chars) && chars[i] == ';'
}

// escapeMarkerLabel neutralizes a source-derived composite list label so it
// cannot alter document structure.
func escapeMarkerLabel(label string, ctx context) string {
	return escapeText(spaceControls(label), ctx, escapeOpts{atLineStart: ctx == blockContext, trailingActive: true})
}

// headingText keeps rendered heading content on one line, so no frontend can
// break out of the heading, and escapes a trailing '#' run that CommonMark
// would read as the closing sequence.
func headingText(text string) string {
	text = strings.TrimSpace(strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, text))
	at := len(strings.TrimRight(text, "#"))
	if at == len(text) {
		return text
	}
	if at == 0 || text[at-1] == ' ' || text[at-1] == '\t' {
		return text[:at] + `\` + text[at:]
	}
	return text
}

func formatURL(url string) string {
	const hex = "0123456789ABCDEF"
	var sb strings.Builder
	bracket := false
	for _, c := range url {
		switch {
		case c == '<':
			sb.WriteString("%3C")
		case c == '>':
			sb.WriteString("%3E")
		case c == '|':
			sb.WriteString("%7C")
		case unicode.IsControl(c):
			for _, b := range []byte(string(c)) {
				sb.WriteByte('%')
				sb.WriteByte(hex[b>>4])
				sb.WriteByte(hex[b&0x0F])
			}
		default:
			bracket = bracket || unicode.IsSpace(c) || c == '(' || c == ')'
			sb.WriteRune(c)
		}
	}
	if bracket {
		return "<" + sb.String() + ">"
	}
	return sb.String()
}

func spaceControls(s string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, s)
}

// escapeCellCodeSpan escapes the pipes of a code span inside a table cell.
// A backslash run already before a pipe is doubled so the escape survives;
// GFM cannot encode a code span holding a backslash right before a pipe, and
// an intact row is worth more than the exact backslash count.
func escapeCellCodeSpan(text string) string {
	var sb strings.Builder
	backslashes := 0
	for _, c := range text {
		switch c {
		case '|':
			sb.WriteString(strings.Repeat(`\`, backslashes+1))
			backslashes = 0
		case '\\':
			backslashes++
		default:
			backslashes = 0
		}
		sb.WriteRune(c)
	}
	return sb.String()
}

func escapeUnescaped(text string, c rune) string {
	var sb strings.Builder
	backslashes := 0
	for _, r := range text {
		if r == c && backslashes%2 == 0 {
			sb.WriteByte('\\')
		}
		sb.WriteRune(r)
		if r == '\\' {
			backslashes++
		} else {
			backslashes = 0
		}
	}
	return sb.String()
}

func backtickFence(text string, minLen int) string {
	longest, run := 0, 0
	for _, c := range text {
		if c == '`' {
			run++
			longest = max(longest, run)
		} else {
			run = 0
		}
	}
	return strings.Repeat("`", max(longest+1, minLen))
}

func isAlnum(r rune) bool { return unicode.IsLetter(r) || unicode.IsNumber(r) }

func isASCIIDigit(r rune) bool { return r >= '0' && r <= '9' }

func isASCIILetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }

func isASCIIPunct(r rune) bool {
	return r < 0x80 && unicode.IsPunct(r) || strings.ContainsRune("$+<=>^`|~", r)
}

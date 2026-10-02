package xlsx

import (
	"math"
	"math/big"
	"math/bits"
	"slices"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

// builtinCode is the implied code for a built-in numFmtId, "" when none. Ids
// 5-8 are left to the file's own formatCode and ids 27-36 and 50-81 are
// locale-specific, so they resolve to General rather than a guess.
func builtinCode(id int) string {
	switch id {
	case 1:
		return "0"
	case 2:
		return "0.00"
	case 3:
		return "#,##0"
	case 4:
		return "#,##0.00"
	case 9:
		return "0%"
	case 10:
		return "0.00%"
	case 11:
		return "0.00E+00"
	case 12:
		return "# ?/?"
	case 13:
		return "# ??/??"
	case 14:
		return "mm-dd-yy"
	case 15:
		return "d-mmm-yy"
	case 16:
		return "d-mmm"
	case 17:
		return "mmm-yy"
	case 18:
		return "h:mm AM/PM"
	case 19:
		return "h:mm:ss AM/PM"
	case 20:
		return "h:mm"
	case 21:
		return "h:mm:ss"
	case 22:
		return "m/d/yy h:mm"
	case 37:
		return "#,##0 ;(#,##0)"
	case 38:
		return "#,##0 ;[Red](#,##0)"
	case 39:
		return "#,##0.00;(#,##0.00)"
	case 40:
		return "#,##0.00;[Red](#,##0.00)"
	case 45:
		return "mm:ss"
	case 46:
		return "[h]:mm:ss"
	case 47:
		return "mmss.0"
	case 48:
		return "##0.0E+0"
	case 49:
		return "@"
	}
	return ""
}

type renderedKind int

const (
	renderedGeneral renderedKind = iota
	renderedDateTime
	renderedText
)

// rendered is what a format asks the caller to do with a number: render
// value as an unformatted cell between prefix and suffix, render the serial
// as the date parts, or emit text.
type rendered struct {
	kind   renderedKind
	value  float64
	prefix string
	suffix string
	parts  dateParts
	text   string
}

// dateParts names the parts a date/time section shows, so a date-only code
// never gains a time and an hours-only code never gains a date.
type dateParts struct {
	date    bool
	time    bool
	elapsed bool
}

type region int

const (
	regionInt region = iota
	regionFrac
	regionExp
	regionNum
	regionDen
)

type tokKind int

const (
	tokDigit tokKind = iota
	tokDecimal
	tokPercent
	tokLiteral
	tokBareDigits
	tokComma
	tokExp
	tokSlash
	tokAt
	tokSkip
)

type tok struct {
	kind   tokKind
	place  rune
	region region
	text   string
	plus   bool
}

type condition struct {
	op      string
	operand float64
}

func (c condition) matches(v float64) bool {
	switch c.op {
	case "<":
		return v < c.operand
	case "<=":
		return v <= c.operand
	case ">":
		return v > c.operand
	case ">=":
		return v >= c.operand
	case "<>":
		return v != c.operand
	}
	return v == c.operand
}

type numSpec struct {
	toks       []tok
	grouping   bool
	scale      int
	percents   int
	intPlaces  int
	fracPlaces int
	exp        bool
	numPlaces  int
	denPlaces  int
	fixedDen   uint64
	hasFixed   bool
}

type bodyKind int

const (
	bodyGeneral bodyKind = iota
	bodyDateTime
	bodyNumber
	bodyText
)

type section struct {
	cond     *condition
	kind     bodyKind
	prefix   string
	suffix   string
	parts    dateParts
	number   numSpec
	textToks []tok
}

type numberFormat struct {
	sections []section
}

// maxFormatCodeLen is the spreadsheet's own cap on a format code, in
// characters; longer codes render General.
const maxFormatCodeLen = 255

// parseNumberFormat returns nil for any code outside the implemented grammar:
// the caller then renders General, since approximating a format would be
// worse than not applying it.
func parseNumberFormat(code string) *numberFormat {
	if code == "" || utf8.RuneCountInString(code) > maxFormatCodeLen {
		return nil
	}
	parts, ok := splitSections(code)
	if !ok || len(parts) > 4 {
		return nil
	}
	f := &numberFormat{}
	conditions := 0
	for _, p := range parts {
		s, ok := parseSection(p)
		if !ok {
			return nil
		}
		if s.cond != nil {
			conditions++
		}
		f.sections = append(f.sections, s)
	}
	if conditions > 2 {
		return nil
	}
	last := len(f.sections) - 1
	for i, s := range f.sections {
		isText := s.kind == bodyText
		if isText && (i != last || s.cond != nil) {
			return nil
		}
		if len(f.sections) == 4 && i == 3 && !isText && (s.kind != bodyNumber || len(s.number.toks) > 0) {
			return nil
		}
	}
	return f
}

func (f *numberFormat) numericSections() []section {
	n := len(f.sections)
	switch {
	case f.sections[n-1].kind == bodyText:
		return f.sections[:n-1]
	case n == 4:
		return f.sections[:3]
	}
	return f.sections
}

func (f *numberFormat) formatNumber(v float64) rendered {
	general := rendered{kind: renderedGeneral, value: v}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return general
	}
	s, value, autoMinus, ok := selectSection(f.numericSections(), v)
	if !ok {
		return general
	}
	switch s.kind {
	case bodyGeneral:
		return rendered{kind: renderedGeneral, value: value, prefix: s.prefix, suffix: s.suffix}
	case bodyDateTime:
		return rendered{kind: renderedDateTime, parts: s.parts}
	case bodyNumber:
		if text, ok := renderNumberSpec(&s.number, math.Abs(value), autoMinus && value < 0); ok {
			return rendered{kind: renderedText, text: text}
		}
	}
	return general
}

// formatText applies the text section; false means the format has none and
// the value stays as it is.
func (f *numberFormat) formatText(text string) (string, bool) {
	n := len(f.sections)
	var s section
	switch {
	case f.sections[n-1].kind == bodyText:
		s = f.sections[n-1]
	case n == 4:
		s = f.sections[3]
	default:
		return "", false
	}
	var sb strings.Builder
	for _, t := range s.textToks {
		switch t.kind {
		case tokLiteral:
			sb.WriteString(t.text)
		case tokSkip:
			sb.WriteByte(' ')
		case tokAt:
			sb.WriteString(text)
		}
	}
	return sb.String(), true
}

// selectSection returns the section for v, the value to render (the
// magnitude for the positional negative section, whose code supplies the
// sign) and whether negative values still need a leading minus.
func selectSection(sections []section, v float64) (section, float64, bool, bool) {
	if len(sections) == 0 {
		return section{}, 0, false, false
	}
	if slices.ContainsFunc(sections, func(s section) bool { return s.cond != nil }) {
		for _, s := range sections {
			if s.cond == nil || s.cond.matches(v) {
				return s, v, true, true
			}
		}
		return section{}, 0, false, false
	}
	idx := 0
	switch {
	case len(sections) >= 2 && v < 0:
		idx = 1
	case len(sections) >= 3 && v == 0:
		idx = 2
	}
	if idx == 1 {
		return sections[1], math.Abs(v), false, true
	}
	return sections[idx], v, true, true
}

func splitSections(code string) ([]string, bool) {
	var parts []string
	chars := []rune(code)
	start := 0
	for i := 0; i < len(chars); i++ {
		switch c := chars[i]; c {
		case ';':
			parts = append(parts, string(chars[start:i]))
			start = i + 1
		case '"', '[':
			closer := '"'
			if c == '[' {
				closer = ']'
			}
			end := slices.Index(chars[i+1:], closer)
			if end < 0 {
				return nil, false
			}
			i += end + 1
		case '\\', '_', '*':
			if i+1 >= len(chars) {
				return nil, false
			}
			i++
		}
	}
	return append(parts, string(chars[start:])), true
}

func decoration(toks []tok) string {
	var sb strings.Builder
	for _, t := range toks {
		if t.kind == tokLiteral {
			sb.WriteString(t.text)
		} else {
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

// datePartsOf reads m as minutes when an hour run precedes it or a seconds
// run follows it, and as months otherwise.
func datePartsOf(runs []rune, elapsed bool) dateParts {
	parts := dateParts{elapsed: elapsed}
	for i, run := range runs {
		switch run {
		case 'y', 'd':
			parts.date = true
		case 'h', 's', 'a':
			parts.time = true
		case 'm':
			if i > 0 && runs[i-1] == 'h' || i+1 < len(runs) && runs[i+1] == 's' {
				parts.time = true
			} else {
				parts.date = true
			}
		}
	}
	if elapsed || !parts.date && !parts.time {
		parts.date, parts.time = false, true
	}
	return parts
}

func pushLiteral(toks []tok, s string) []tok {
	if n := len(toks); n > 0 && toks[n-1].kind == tokLiteral {
		toks[n-1].text += s
		return toks
	}
	return append(toks, tok{kind: tokLiteral, text: s})
}

func hasToken(toks []tok, kinds ...tokKind) bool {
	return slices.ContainsFunc(toks, func(t tok) bool { return slices.Contains(kinds, t.kind) })
}

func parseSection(s string) (section, bool) {
	chars := []rune(s)
	var (
		raw        []tok
		generalAt  int
		cond       *condition
		hasDate    bool
		elapsed    bool
		runs       []rune
		hasGeneral bool
	)
	bracket := func(inner string) bool {
		if inner == "" {
			return false
		}
		first := []rune(inner)[0]
		switch {
		case first == '<' || first == '>' || first == '=':
			if cond != nil {
				return false
			}
			op := inner[:1]
			for _, two := range []string{">=", "<=", "<>"} {
				if strings.HasPrefix(inner, two) {
					op = two
					break
				}
			}
			operand, ok := parseFloat(strings.TrimSpace(inner[len(op):]))
			if !ok {
				return false
			}
			cond = &condition{op: op, operand: operand}
		case first == '$':
			if sym, _, _ := strings.Cut(inner[1:], "-"); sym != "" {
				raw = append(raw, tok{kind: tokLiteral, text: sym})
			}
		case strings.ContainsRune("hHmMsS", first) && strings.Trim(inner, string([]rune{first, first ^ 0x20})) == "":
			hasDate, elapsed = true, true
			runs = append(runs, asciiLower(first))
		default:
			lower := strings.Map(asciiLower, inner)
			if slices.Contains(colorNames, lower) {
				return true
			}
			rest, ok := strings.CutPrefix(lower, "color")
			n, isNum := parseUint(strings.TrimSpace(rest), 32)
			return ok && isNum && n >= 1 && n <= 56
		}
		return true
	}
	for i := 0; i < len(chars); {
		c := chars[i]
		switch {
		case c == '[':
			end := slices.Index(chars[i:], ']')
			if end < 0 {
				return section{}, false
			}
			inner := string(chars[i+1 : i+end])
			i += end + 1
			if !bracket(inner) {
				return section{}, false
			}
		case c == '"':
			end := slices.Index(chars[i+1:], '"')
			if end < 0 {
				return section{}, false
			}
			if end > 0 {
				raw = pushLiteral(raw, string(chars[i+1:i+1+end]))
			}
			i += end + 2
		case c == '\\':
			if i+1 >= len(chars) {
				return section{}, false
			}
			raw = pushLiteral(raw, string(chars[i+1]))
			i += 2
		case c == '_':
			if i+1 >= len(chars) {
				return section{}, false
			}
			raw = append(raw, tok{kind: tokSkip})
			i += 2
		case c == '*':
			if i+1 >= len(chars) {
				return section{}, false
			}
			i += 2
		case c == '0' || c == '#' || c == '?':
			raw = append(raw, tok{kind: tokDigit, place: c, region: regionInt})
			i++
		case c == '.':
			raw = append(raw, tok{kind: tokDecimal})
			i++
		case c == ',':
			raw = append(raw, tok{kind: tokComma})
			i++
		case c == '%':
			raw = append(raw, tok{kind: tokPercent})
			i++
		case c == '@':
			raw = append(raw, tok{kind: tokAt})
			i++
		case (c == 'E' || c == 'e') && i+1 < len(chars) && (chars[i+1] == '+' || chars[i+1] == '-'):
			raw = append(raw, tok{kind: tokExp, plus: chars[i+1] == '+'})
			i += 2
		case strings.ContainsRune("yYdDhHsSmM", c):
			hasDate = true
			lower := asciiLower(c)
			runs = append(runs, lower)
			for i < len(chars) && asciiLower(chars[i]) == lower {
				i++
			}
		case c == 'g' || c == 'G':
			if !strings.EqualFold(string(chars[i:min(len(chars), i+7)]), "general") {
				return section{}, false
			}
			hasGeneral = true
			generalAt = len([]rune(decoration(raw)))
			i += 7
		case c == 'a' || c == 'A':
			n := 0
			for _, word := range []string{"AM/PM", "A/P"} {
				if len(chars)-i >= len(word) && strings.EqualFold(string(chars[i:i+len(word)]), word) {
					n = len(word)
					break
				}
			}
			if n == 0 {
				return section{}, false
			}
			hasDate = true
			runs = append(runs, 'a')
			i += n
		case c >= '1' && c <= '9':
			end := i
			for end < len(chars) && chars[end] >= '0' && chars[end] <= '9' {
				end++
			}
			raw = append(raw, tok{kind: tokBareDigits, text: string(chars[i:end])})
			i = end
		case strings.ContainsRune("$-+(): ", c):
			raw = pushLiteral(raw, string(c))
			i++
		case c == '/':
			raw = append(raw, tok{kind: tokSlash})
			i++
		default:
			return section{}, false
		}
	}
	sec := section{cond: cond}
	switch {
	case hasGeneral:
		if slices.ContainsFunc(raw, func(t tok) bool { return t.kind != tokLiteral && t.kind != tokSkip }) {
			return section{}, false
		}
		text := []rune(decoration(raw))
		sec.kind = bodyGeneral
		sec.prefix, sec.suffix = string(text[:generalAt]), string(text[generalAt:])
	case hasDate:
		if hasToken(raw, tokAt, tokExp, tokBareDigits) {
			return section{}, false
		}
		sec.kind = bodyDateTime
		sec.parts = datePartsOf(runs, elapsed)
	case hasToken(raw, tokAt):
		if hasToken(raw, tokDigit, tokDecimal, tokExp, tokSlash, tokBareDigits) {
			return section{}, false
		}
		sec.kind = bodyText
		for _, t := range raw {
			switch t.kind {
			case tokPercent:
				t = tok{kind: tokLiteral, text: "%"}
			case tokComma:
				t = tok{kind: tokLiteral, text: ","}
			}
			sec.textToks = append(sec.textToks, t)
		}
	default:
		spec, ok := resolveNumber(raw)
		if !ok {
			return section{}, false
		}
		sec.kind = bodyNumber
		sec.number = spec
	}
	return sec, true
}

func asciiLower(c rune) rune {
	if c >= 'A' && c <= 'Z' {
		return c + 'a' - 'A'
	}
	return c
}

var colorNames = []string{"black", "blue", "cyan", "green", "magenta", "red", "white", "yellow"}

func resolveNumber(raw []tok) (numSpec, bool) {
	var spec numSpec
	var toks []tok
	reg := regionInt
	for i := 0; i < len(raw); i++ {
		t := raw[i]
		switch t.kind {
		case tokDigit:
			if spec.hasFixed && reg == regionDen {
				return spec, false
			}
			t.region = reg
			toks = append(toks, t)
		case tokDecimal:
			if reg != regionInt {
				return spec, false
			}
			reg = regionFrac
			toks = append(toks, t)
		case tokExp:
			if spec.exp || reg == regionNum || reg == regionDen {
				return spec, false
			}
			spec.exp = true
			reg = regionExp
			toks = append(toks, t)
		case tokSlash:
			run := 0
			for j := len(toks) - 1; j >= 0 && toks[j].kind == tokDigit && toks[j].region == regionInt; j-- {
				run++
			}
			if run == 0 || reg != regionInt || spec.hasFixed {
				toks = pushLiteral(toks, "/")
				continue
			}
			for j := len(toks) - run; j < len(toks); j++ {
				toks[j].region = regionNum
			}
			toks = append(toks, t)
			reg = regionDen
			if i+1 < len(raw) && raw[i+1].kind == tokBareDigits {
				i++
				d, ok := parseUint(raw[i].text, 64)
				if !ok {
					return spec, false
				}
				spec.fixedDen, spec.hasFixed = d, true
				toks = append(toks, tok{kind: tokLiteral, text: raw[i].text})
			}
		case tokPercent:
			spec.percents++
			toks = append(toks, t)
		case tokBareDigits:
			return spec, false
		default:
			toks = append(toks, t)
		}
	}
	isDigit := func(t tok) bool { return t.kind == tokDigit }
	first, last := slices.IndexFunc(toks, isDigit), -1
	for j := len(toks) - 1; j >= 0; j-- {
		if isDigit(toks[j]) {
			last = j
			break
		}
	}
	var out []tok
	for j, t := range toks {
		switch {
		case t.kind != tokComma:
			out = append(out, t)
		case last >= 0 && j > last:
			spec.scale++
		case first >= 0 && j > first && j < last:
			spec.grouping = true
		default:
			out = pushLiteral(out, ",")
		}
	}
	hasExpDigit := false
	for _, t := range out {
		if t.kind != tokDigit {
			continue
		}
		switch t.region {
		case regionInt:
			spec.intPlaces++
		case regionFrac:
			spec.fracPlaces++
		case regionExp:
			hasExpDigit = true
		case regionNum:
			spec.numPlaces++
		case regionDen:
			spec.denPlaces++
		}
	}
	if spec.exp && !hasExpDigit || spec.numPlaces > 0 && spec.denPlaces == 0 && !spec.hasFixed {
		return spec, false
	}
	spec.toks = out
	return spec, true
}

func renderNumberSpec(spec *numSpec, v float64, minus bool) (string, bool) {
	for range spec.percents {
		v *= 100
	}
	for range spec.scale {
		v /= 1000
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return "", false
	}
	var body string
	var ok bool
	switch {
	case spec.exp:
		body, ok = renderScientific(spec, v)
	case spec.numPlaces > 0:
		body, ok = renderFraction(spec, v)
	default:
		var intDigits, fracDigits string
		intDigits, fracDigits, ok = splitDigits(v, spec.fracPlaces)
		body = emit(spec, intDigits, fracDigits, "", 0)
	}
	if !ok {
		return "", false
	}
	if minus {
		body = "-" + body
	}
	return body, true
}

// splitDigits returns the integer digits of v rounded to dp decimals (empty
// when zero, so # can drop it) and exactly dp fractional digits. Rounding
// runs half away from zero on the 15-significant-digit decimal form, the way
// a spreadsheet displays: binary rounding would turn 5.255 into 5.25.
func splitDigits(v float64, dp int) (string, string, bool) {
	if math.IsInf(v, 0) || math.IsNaN(v) || v < 0 || dp > 512 {
		return "", "", false
	}
	if v == 0 {
		return "", strings.Repeat("0", dp), true
	}
	mantissa, expText, _ := strings.Cut(strconv.FormatFloat(v, 'e', 14, 64), "e")
	e, _ := strconv.Atoi(expText)
	digits := strings.Replace(mantissa, ".", "", 1)
	shift := e - 14 + dp
	var scaled string
	switch {
	case shift >= 0:
		scaled = digits + strings.Repeat("0", shift)
	case -shift <= len(digits):
		kept := []byte(digits[:len(digits)+shift])
		if digits[len(digits)+shift] >= '5' {
			kept = carry(kept)
		}
		scaled = string(kept)
	}
	s := strings.TrimLeft(scaled, "0")
	if len(s) < dp {
		s = strings.Repeat("0", dp-len(s)) + s
	}
	return s[:len(s)-dp], s[len(s)-dp:], true
}

func carry(digits []byte) []byte {
	for i := len(digits) - 1; i >= 0; i-- {
		if digits[i] != '9' {
			digits[i]++
			return digits
		}
		digits[i] = '0'
	}
	return append([]byte{'1'}, digits...)
}

func places(toks []tok, r region) []rune {
	var out []rune
	for _, t := range toks {
		if t.kind == tokDigit && t.region == r {
			out = append(out, t.place)
		}
	}
	return out
}

// assign spreads digits over placeholders: the first takes any excess, and
// placeholders past the digits pad per kind (0 a zero, ? a space, # nothing).
func assign(digits string, ps []rune) []string {
	k, n := len(ps), len(digits)
	if k == 0 {
		return nil
	}
	out := make([]string, 0, k)
	if n > k {
		out = append(out, digits[:n-k+1])
		digits = digits[n-k+1:]
	} else {
		for _, p := range ps[:k-n] {
			switch p {
			case '0':
				out = append(out, "0")
			case '?':
				out = append(out, " ")
			default:
				out = append(out, "")
			}
		}
	}
	for _, c := range digits {
		out = append(out, string(c))
	}
	return out
}

func countDigits(s string) int {
	n := 0
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n++
		}
	}
	return n
}

func emit(spec *numSpec, intDigits, fracDigits, expDigits string, expSign int) string {
	intAssigned := assign(intDigits, places(spec.toks, regionInt))
	expAssigned := assign(expDigits, places(spec.toks, regionExp))
	intRemaining := 0
	for _, s := range intAssigned {
		intRemaining += countDigits(s)
	}
	fracPlaces := places(spec.toks, regionFrac)
	minFrac := 0
	for i, p := range fracPlaces {
		if p == '0' {
			minFrac = i + 1
		}
	}
	keepFrac := max(len(strings.TrimRight(fracDigits, "0")), minFrac)
	showDecimal := keepFrac > 0 || slices.Contains(fracPlaces, '?') || len(fracPlaces) == 0

	var sb strings.Builder
	intI, fracI, expI := 0, 0, 0
	for _, t := range spec.toks {
		switch {
		case t.kind == tokDigit && t.region == regionInt:
			for _, c := range intAssigned[intI] {
				sb.WriteRune(c)
				if c >= '0' && c <= '9' {
					intRemaining--
					if spec.grouping && intRemaining > 0 && intRemaining%3 == 0 {
						sb.WriteByte(',')
					}
				}
			}
			intI++
		case t.kind == tokDigit && t.region == regionFrac:
			if fracI < keepFrac {
				sb.WriteByte(fracDigits[fracI])
			} else if t.place == '?' {
				sb.WriteByte(' ')
			}
			fracI++
		case t.kind == tokDigit && t.region == regionExp:
			sb.WriteString(expAssigned[expI])
			expI++
		case t.kind == tokDecimal:
			if showDecimal {
				sb.WriteByte('.')
			}
		case t.kind == tokPercent:
			sb.WriteByte('%')
		case t.kind == tokLiteral:
			sb.WriteString(t.text)
		case t.kind == tokExp:
			sb.WriteByte('E')
			if expSign < 0 {
				sb.WriteByte('-')
			} else if t.plus {
				sb.WriteByte('+')
			}
		case t.kind == tokSkip:
			sb.WriteByte(' ')
		}
	}
	return sb.String()
}

func renderScientific(spec *numSpec, v float64) (string, bool) {
	nInt := max(spec.intPlaces, 1)
	intDigits, fracDigits, exp10 := "", "", 0
	if v == 0 {
		var ok bool
		if intDigits, fracDigits, ok = splitDigits(0, spec.fracPlaces); !ok {
			return "", false
		}
	} else {
		_, expText, _ := strings.Cut(strconv.FormatFloat(v, 'e', -1, 64), "e")
		e, _ := strconv.Atoi(expText)
		e = floorDiv(e, nInt) * nInt
		var ok bool
		if intDigits, fracDigits, ok = splitDigits(v/powi(10, e), spec.fracPlaces); !ok {
			return "", false
		}
		if len(intDigits) > nInt {
			e += nInt
			if intDigits, fracDigits, ok = splitDigits(v/powi(10, e), spec.fracPlaces); !ok {
				return "", false
			}
		}
		exp10 = e
	}
	bare := strconv.Itoa(max(exp10, -exp10))
	pad := max(len(places(spec.toks, regionExp))-len(bare), 0)
	sign := 1
	if exp10 < 0 {
		sign = -1
	}
	return emit(spec, intDigits, fracDigits, strings.Repeat("0", pad)+bare, sign), true
}

func floorDiv[T int | int64](a, b T) T {
	q := a / b
	if a%b != 0 && a < 0 {
		q--
	}
	return q
}

// powi raises by repeated squaring and inverts negative powers, matching the
// integer-power routine the reference values were computed with; math.Pow10
// rounds differently past 1e22.
func powi(x float64, n int) float64 {
	r, a, m := 1.0, x, max(n, -n)
	for {
		if m&1 != 0 {
			r *= a
		}
		m /= 2
		if m == 0 {
			break
		}
		a *= a
	}
	if n < 0 {
		return 1 / r
	}
	return r
}

func renderFraction(spec *numSpec, v float64) (string, bool) {
	if v >= 1e15 {
		return "", false
	}
	hasInt := spec.intPlaces > 0
	whole, target := 0.0, v
	if hasInt {
		whole, target = math.Modf(v)
	}
	num, den, ok := bestFraction(target, spec)
	if !ok {
		return "", false
	}
	if hasInt && num == den && den > 0 {
		whole++
		num = 0
	}
	var intDigits string
	switch {
	case whole != 0:
		intDigits, _, _ = splitDigits(whole, 0)
	case num == 0:
		intDigits = "0"
	}
	hide := hasInt && num == 0
	intAssigned := assign(intDigits, places(spec.toks, regionInt))
	numAssigned := assign(strconv.FormatUint(num, 10), places(spec.toks, regionNum))
	denAssigned := assign(strconv.FormatUint(den, 10), places(spec.toks, regionDen))
	next := func(list *[]string) string {
		if len(*list) == 0 {
			return ""
		}
		s := (*list)[0]
		*list = (*list)[1:]
		return s
	}
	fixedText := strconv.FormatUint(spec.fixedDen, 10)
	var sb strings.Builder
	for _, t := range spec.toks {
		switch {
		case t.kind == tokDigit && t.region == regionInt:
			sb.WriteString(next(&intAssigned))
		case t.kind == tokDigit && t.region == regionNum && !hide:
			sb.WriteString(next(&numAssigned))
		case t.kind == tokDigit && t.region == regionDen && !hide:
			sb.WriteString(next(&denAssigned))
		case t.kind == tokSlash && !hide:
			sb.WriteByte('/')
		case t.kind == tokLiteral:
			if !hide || !spec.hasFixed || fixedText != t.text {
				sb.WriteString(t.text)
			}
		case t.kind == tokPercent:
			sb.WriteByte('%')
		case t.kind == tokSkip:
			sb.WriteByte(' ')
		}
	}
	return strings.TrimRightFunc(sb.String(), unicode.IsSpace), true
}

func bestFraction(x float64, spec *numSpec) (uint64, uint64, bool) {
	if x < 0 || math.IsInf(x, 0) || math.IsNaN(x) {
		return 0, 0, false
	}
	if spec.hasFixed {
		n := math.Round(x * float64(spec.fixedDen))
		return uint64(n), spec.fixedDen, n < 1e18
	}
	maxDen := uint64(math.MaxUint64)
	if spec.denPlaces < 20 {
		maxDen = max(pow10u(spec.denPlaces)-1, 1)
	}
	n, d := closestRational(x, maxDen)
	return n, d, true
}

func pow10u(n int) uint64 {
	p := uint64(1)
	for range n {
		p *= 10
	}
	return p
}

// closestRational is the closest fraction to x with a denominator at most
// maxDen. It walks the continued fraction of the exact ratio the float
// stores; inverting float remainders instead compounds rounding until large
// bounds pick a non-closest fraction. When the next term overshoots the
// bound, the best semiconvergent is the only other candidate, and the exact
// tail decides between them.
func closestRational(x float64, maxDen uint64) (uint64, uint64) {
	p, q, ok := dyadic(x)
	if !ok {
		return 0, 1
	}
	var pn, pd, n, d uint64 = 0, 1, 1, 0
	a, rem := new(big.Int), new(big.Int)
	for q.Sign() > 0 {
		a.QuoRem(p, q, rem)
		nn, nd, fits := convergent(a, n, d, pn, pd)
		if !fits || nd > maxDen {
			if d == 0 {
				return 0, 1
			}
			k := (maxDen - pd) / d
			hi, kn := bits.Mul64(k, n)
			sn, overflow := bits.Add64(kn, pn, 0)
			twoK := new(big.Int).Lsh(new(big.Int).SetUint64(k), 1)
			better := false
			switch twoK.Cmp(a) {
			case 1:
				better = true
			case 0:
				lhs := new(big.Int).Mul(rem, new(big.Int).SetUint64(d))
				rhs := new(big.Int).Mul(q, new(big.Int).SetUint64(pd))
				better = lhs.Cmp(rhs) < 0
			}
			if better && hi == 0 && overflow == 0 {
				return sn, k*d + pd
			}
			return n, d
		}
		pn, pd, n, d = n, d, nn, nd
		p, q = q, new(big.Int).Set(rem)
	}
	if d == 0 {
		return 0, 1
	}
	return n, d
}

func convergent(a *big.Int, n, d, pn, pd uint64) (uint64, uint64, bool) {
	if !a.IsUint64() {
		return 0, 0, false
	}
	av := a.Uint64()
	hi1, an := bits.Mul64(av, n)
	nn, c1 := bits.Add64(an, pn, 0)
	hi2, ad := bits.Mul64(av, d)
	nd, c2 := bits.Add64(ad, pd, 0)
	return nn, nd, hi1|c1|hi2|c2 == 0
}

// dyadic is the exact ratio a finite non-negative float stores; false when no
// bounded fraction can tell it from zero or from an integer.
func dyadic(x float64) (*big.Int, *big.Int, bool) {
	b := math.Float64bits(x)
	exp := int(b >> 52)
	frac := b & (1<<52 - 1)
	m, e := frac|1<<52, exp-1075
	if exp == 0 {
		m, e = frac, -1074
	}
	if m == 0 {
		return big.NewInt(0), big.NewInt(1), true
	}
	if e >= 0 {
		if e > 75 {
			return nil, nil, false
		}
		return new(big.Int).Lsh(new(big.Int).SetUint64(m), uint(e)), big.NewInt(1), true
	}
	shift := min(bits.TrailingZeros64(m), -e)
	k := -e - shift
	if k > 127 {
		return nil, nil, false
	}
	return new(big.Int).SetUint64(m >> shift), new(big.Int).Lsh(big.NewInt(1), uint(k)), true
}

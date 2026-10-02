package model

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Header detection tuning: body rows sampled for column types, the share of
// a column's non-empty values that must agree on one type before it votes,
// and the longest text that still reads as a label when only row shape is
// left to decide.
const (
	headerSampleRows = 50
	dominanceNum     = 9
	dominanceDen     = 10
	maxLabelRunes    = 64
)

type valueKind int

const (
	kindBlank valueKind = iota
	kindNumber
	kindBool
	kindDate
	kindText
)

// ResolveHeaderRows returns declared (capped at the row count) when the
// format declared header rows, else 1 when the first row labels the columns
// and 0 otherwise. The first row is a header when the columns below it are
// consistently typed and the row itself is not.
func ResolveHeaderRows(t Table, declared int) int {
	grid := t.grid
	if declared > 0 {
		return min(declared, len(grid))
	}
	if len(grid) < 2 {
		return 0
	}
	for _, s := range grid[0] {
		if s.Covered || max(s.Cell.ColSpan, 1) != 1 || max(s.Cell.RowSpan, 1) != 1 {
			return 0
		}
	}
	sample := grid[1:min(len(grid), headerSampleRows+1)]
	if len(grid[0]) != modalWidth(sample) {
		return 0
	}
	head := slotTexts(grid[0])
	body := make([][]string, len(sample))
	for i, row := range sample {
		body[i] = slotTexts(row)
	}
	width := contentWidth(head)
	for _, row := range body {
		width = max(width, contentWidth(row))
	}
	if width == 0 || width > len(head) {
		return 0
	}

	seen := map[string]bool{}
	for c, label := range head[:width] {
		if strings.TrimSpace(label) == "" {
			if c == 0 {
				continue
			}
			return 0
		}
		if strings.Contains(label, "\n") || seen[fold(label)] {
			return 0
		}
		seen[fold(label)] = true
	}

	headerVotes, dataVotes := 0, 0
	for c, label := range head[:width] {
		var values []string
		for _, row := range body {
			if c < len(row) {
				if v := strings.TrimSpace(row[c]); v != "" {
					values = append(values, v)
				}
			}
		}
		if len(values) == 0 {
			continue
		}
		if kind := dominantKind(values); kind != kindBlank && kind != kindText {
			if classify(label) == kindText {
				headerVotes++
			} else {
				dataVotes++
			}
			continue
		}
		for _, v := range values {
			if fold(v) == fold(label) {
				dataVotes++
				break
			}
		}
	}
	if headerVotes == 0 && dataVotes == 0 {
		for _, label := range head[:width] {
			if utf8.RuneCountInString(label) > maxLabelRunes {
				return 0
			}
		}
		return 1
	}
	if headerVotes > dataVotes {
		return 1
	}
	return 0
}

// modalWidth is the row length most sample rows share, ties broken toward
// the wider shape.
func modalWidth(rows [][]Slot) int {
	tally := map[int]int{}
	for _, row := range rows {
		tally[len(row)]++
	}
	best, bestCount := 0, 0
	for width, n := range tally {
		if n > bestCount || n == bestCount && width > best {
			best, bestCount = width, n
		}
	}
	return best
}

// slotTexts reads a row's plain values; covered slots and cells holding
// anything but paragraphs read as empty.
func slotTexts(row []Slot) []string {
	out := make([]string, len(row))
	for i, s := range row {
		if s.Covered {
			continue
		}
		var parts []string
		for _, b := range s.Cell.Blocks {
			p, ok := b.(Paragraph)
			if !ok {
				parts = nil
				break
			}
			parts = append(parts, PlainText(p))
		}
		out[i] = strings.Join(parts, "\n")
	}
	return out
}

func contentWidth(row []string) int {
	for i := len(row) - 1; i >= 0; i-- {
		if strings.TrimSpace(row[i]) != "" {
			return i + 1
		}
	}
	return 0
}

func fold(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

func dominantKind(values []string) valueKind {
	for _, kind := range []valueKind{kindNumber, kindBool, kindDate, kindText} {
		n := 0
		for _, v := range values {
			if classify(v) == kind {
				n++
			}
		}
		if n*dominanceDen >= len(values)*dominanceNum {
			return kind
		}
	}
	return kindBlank
}

func classify(value string) valueKind {
	value = strings.TrimSpace(value)
	switch {
	case value == "":
		return kindBlank
	case isNumber(value):
		return kindNumber
	}
	switch fold(value) {
	case "true", "false", "yes", "no":
		return kindBool
	}
	if isTemporal(value) {
		return kindDate
	}
	return kindText
}

// isNumber accepts sign, digit grouping, either decimal convention, an
// exponent and a percent suffix.
func isNumber(value string) bool {
	value = strings.TrimSuffix(value, "%")
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ',', ' ', '_', '\u00a0':
			return -1
		}
		return r
	}, value)
	if !strings.ContainsAny(cleaned, "0123456789") || strings.ContainsAny(cleaned, "xX") {
		return false
	}
	_, err := strconv.ParseFloat(cleaned, 64)
	return err == nil || errors.Is(err, strconv.ErrRange)
}

// isTemporal matches a numeric date triple in any field order, optionally
// followed by a clock time, or a bare clock time.
func isTemporal(value string) bool {
	date, clock := value, ""
	if i := strings.IndexAny(value, "T "); i >= 0 {
		date, clock = value[:i], value[i+1:]
	}
	clock, _, _ = strings.Cut(strings.TrimRight(clock, "Z"), ".")
	isClock := func(s string) bool {
		n := digitGroups(s, ":")
		return n == 2 || n == 3
	}
	if digitGroups(date, "-/.") == 3 {
		return clock == "" || isClock(clock)
	}
	return clock == "" && isClock(date)
}

// digitGroups splits value on the first of seps it contains and returns the
// component count, or 0 unless every component is one to four ASCII digits.
func digitGroups(value, seps string) int {
	for _, sep := range seps {
		if !strings.ContainsRune(value, sep) {
			continue
		}
		parts := strings.Split(value, string(sep))
		for _, p := range parts {
			if len(p) < 1 || len(p) > 4 || strings.Trim(p, "0123456789") != "" {
				return 0
			}
		}
		return len(parts)
	}
	return 0
}

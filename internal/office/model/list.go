package model

import (
	"strconv"
	"strings"
)

type MarkerKind int

const (
	Bullet MarkerKind = iota
	Decimal
	LowerAlpha
	UpperAlpha
	LowerRoman
	UpperRoman
)

func (k MarkerKind) Ordered() bool { return k != Bullet }

// Label is the marker text for 1-based ordinal n without trailing space:
// "3.", "c.", "iv.", or "-" for bullets.
func (k MarkerKind) Label(n int) string {
	if k == Bullet {
		return "-"
	}
	return k.Ordinal(n) + "."
}

func (k MarkerKind) Ordinal(n int) string {
	switch k {
	case Bullet:
		return "-"
	case LowerAlpha:
		return alpha(n)
	case UpperAlpha:
		return strings.ToUpper(alpha(n))
	case LowerRoman:
		return roman(n)
	case UpperRoman:
		return strings.ToUpper(roman(n))
	default:
		return strconv.Itoa(n)
	}
}

func alpha(n int) string {
	if n <= 0 {
		return strconv.Itoa(n)
	}
	var out []byte
	for n > 0 {
		n--
		out = append(out, byte('a'+n%26))
		n /= 26
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

var numerals = []struct {
	value   int
	numeral string
}{
	{1000, "m"}, {900, "cm"}, {500, "d"}, {400, "cd"},
	{100, "c"}, {90, "xc"}, {50, "l"}, {40, "xl"},
	{10, "x"}, {9, "ix"}, {5, "v"}, {4, "iv"}, {1, "i"},
}

func roman(n int) string {
	if n <= 0 || n > 3999 {
		return strconv.Itoa(n)
	}
	var sb strings.Builder
	for _, r := range numerals {
		for n >= r.value {
			sb.WriteString(r.numeral)
			n -= r.value
		}
	}
	return sb.String()
}

// List is fully resolved: frontends split runs wherever the list instance or
// marker kind changes, and Start is the source's own ordinal for the first
// item.
type List struct {
	Marker MarkerKind
	Start  int
	Items  []ListItem
}

// ListItem is one list entry. Label overrides the level marker with literal
// source number text when marker kind and position cannot reproduce it
// (composite labels such as "1-a)"); "" means none.
type ListItem struct {
	Blocks []Block
	Label  string
}

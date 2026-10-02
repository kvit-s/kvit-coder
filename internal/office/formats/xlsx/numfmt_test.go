package xlsx

import (
	"math"
	"math/rand/v2"
	"reflect"
	"strings"
	"testing"
	"time"
)

func mustFormat(t *testing.T, code string, v float64) string {
	t.Helper()
	f := parseNumberFormat(code)
	if f == nil {
		t.Fatalf("code %q must parse", code)
	}
	r := f.formatNumber(v)
	if r.kind != renderedText {
		t.Fatalf("expected text for %q on %v, got %+v", code, v, r)
	}
	return r.text
}

func TestNumberFormatRendering(t *testing.T) {
	cases := []struct {
		code string
		v    float64
		want string
	}{
		{"0.0%", 0.075, "7.5%"},
		{"0%", 0.155, "16%"},
		{"0.00%", -0.5, "-50.00%"},
		{"#,##0", 9876543, "9,876,543"},
		{"#,##0.00", 1234.5, "1,234.50"},
		{"#,##0", 0, "0"},
		{"#,##0", 999, "999"},
		{`"$"#,##0.00`, 1234.5, "$1,234.50"},
		{"[$$-409]#,##0.00", 1234.5, "$1,234.50"},
		{`#,##0.00\ "kr"`, 1234.5, "1,234.50 kr"},
		{"00000", 42, "00042"},
		{"#", 0, ""},
		{"0.##", 5, "5"},
		{"0.??", 5, "5.  "},
		{"0.0#", 5.25, "5.25"},
		{"0.00", 5.255, "5.26"},
		{"???", 42, " 42"},
		{"0.", 5, "5."},
		{`0."kg"`, 5, "5.kg"},
		{"0.00;(0.00)", -3.5, "(3.50)"},
		{"0.00;(0.00)", 3.5, "3.50"},
		{`0;-0;"zero"`, 0, "zero"},
		{"0", -3, "-3"},
		{"#,##0;[Red](#,##0)", -1234, "(1,234)"},
		{"[>=100]0.0;0.00", 250, "250.0"},
		{"[>=100]0.0;0.00", 3, "3.00"},
		{"0.0,,", 12_345_678, "12.3"},
		{"#,##0,", 12_345_678, "12,346"},
		{"0.00E+00", 12345, "1.23E+04"},
		{"0.00E+00", 0.0001234, "1.23E-04"},
		{"##0.0E+0", 0.0000123, "12.3E-6"},
		{"0.00E+00", 0, "0.00E+00"},
		{"# ?/?", 5.25, "5 1/4"},
		{"# ??/??", 2.675, "2 27/40"},
		{"# ?/?", 5, "5"},
		{"?/?", 0.5, "1/2"},
		{"# ?/8", 5.25, "5 2/8"},
		{"?/" + strings.Repeat("?", 20), 0.5, "1/" + strings.Repeat(" ", 19) + "2"},
		{"0.00_);(0.00)", 3.5, "3.50 "},
		{"$* 0.00", 3.5, "$3.50"},
		{`0.0\ "m/s"`, 3.51, "3.5 m/s"},
		{`0"d"`, 3, "3d"},
		{`"yes";"yes";"no"`, 1, "yes"},
		{`"yes";"yes";"no"`, 0, "no"},
		{"0", 2.5, "3"},
	}
	for _, c := range cases {
		if got := mustFormat(t, c.code, c.v); got != c.want {
			t.Errorf("format %q on %v = %q, want %q", c.code, c.v, got, c.want)
		}
	}
}

func TestDateSectionsClassifyWithoutRendering(t *testing.T) {
	cases := []struct {
		code string
		want dateParts
	}{
		{`yyyy\-mm\-dd`, dateParts{date: true}},
		{"[hh]:mm:ss", dateParts{time: true, elapsed: true}},
		{"h:mm AM/PM", dateParts{time: true}},
		{"yyyy-mm-dd", dateParts{date: true}},
		{"d mmm yyyy", dateParts{date: true}},
		{"h:mm", dateParts{time: true}},
		{"yyyy-mm-dd hh:mm:ss", dateParts{date: true, time: true}},
		{"[h]:mm:ss", dateParts{time: true, elapsed: true}},
		{"[m]", dateParts{time: true, elapsed: true}},
		{"mm:ss", dateParts{time: true}},
		{"mm/dd/yyyy", dateParts{date: true}},
	}
	for _, c := range cases {
		r := parseNumberFormat(c.code).formatNumber(45000.5)
		if r.kind != renderedDateTime || r.parts != c.want {
			t.Errorf("%q: got %+v, want parts %+v", c.code, r, c.want)
		}
	}
}

func TestGeneralSections(t *testing.T) {
	r := parseNumberFormat(`"~"General" kg"`).formatNumber(1234.5)
	if r != (rendered{kind: renderedGeneral, value: 1234.5, prefix: "~", suffix: " kg"}) {
		t.Errorf("decorated General: %+v", r)
	}
	r = parseNumberFormat("General").formatNumber(3.5)
	if r != (rendered{kind: renderedGeneral, value: 3.5}) {
		t.Errorf("General: %+v", r)
	}
	r = parseNumberFormat("General;General").formatNumber(-3.5)
	if r != (rendered{kind: renderedGeneral, value: 3.5}) {
		t.Errorf("negative General section receives the magnitude: %+v", r)
	}
}

func TestUnsupportedConstructsRefuseToParse(t *testing.T) {
	for _, code := range []string{"", "[DBNum1]0", "0.0.0", "abc0", "0;0;0;0;0", "€0.00", `"open`, "0;0;0;0.0"} {
		if parseNumberFormat(code) != nil {
			t.Errorf("%q must not parse", code)
		}
	}
}

func TestSplitSections(t *testing.T) {
	tests := []struct {
		name string
		code string
		want []string
		ok   bool
	}{
		{"single", "0.00", []string{"0.00"}, true},
		{"four with empty last", "0;(0);-;", []string{"0", "(0)", "-", ""}, true},
		{"separators inside quotes and brackets", `"a;b"0;[<0;x]0`, []string{`"a;b"0`, "[<0;x]0"}, true},
		{"escaped separator", `0\;0`, []string{`0\;0`}, true},
		{"skip and fill take the separator", "_;*;0", []string{"_;*;0"}, true},
		{"unterminated quote", `0;"open`, nil, false},
		{"unterminated bracket", "[Red0", nil, false},
		{"dangling escape", `0\`, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := splitSections(tt.code)
			if ok != tt.ok || !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitSections(%q) = %q, %v; want %q, %v", tt.code, got, ok, tt.want, tt.ok)
			}
		})
	}
}

func TestFormatCodeLengthCap(t *testing.T) {
	quoted := func(n int) string { return `"` + strings.Repeat("a", n-3) + `"0` }
	if f := parseNumberFormat(quoted(maxFormatCodeLen)); f == nil {
		t.Errorf("a %d-character code must parse", maxFormatCodeLen)
	}
	long := []struct {
		name string
		code string
	}{
		{"one past the cap", quoted(maxFormatCodeLen + 1)},
		{"huge quoted literal", quoted(200_000)},
		{"huge escape run", strings.Repeat(`\a`, 100_000) + "0"},
		{"huge section soup", strings.Repeat(`"x;`, 70_000)},
	}
	for _, tt := range long {
		t.Run(tt.name, func(t *testing.T) {
			start := time.Now()
			if parseNumberFormat(tt.code) != nil {
				t.Errorf("%d-character code must render General", len(tt.code))
			}
			if d := time.Since(start); d > 100*time.Millisecond {
				t.Errorf("took %v, want well under a second", d)
			}
		})
	}
}

func TestLongSectionsSplitInLinearTime(t *testing.T) {
	code := `"` + strings.Repeat("a", 200_000) + `";` + strings.Repeat(`\a`, 100_000)
	start := time.Now()
	parts, ok := splitSections(code)
	if d := time.Since(start); d > 100*time.Millisecond {
		t.Errorf("took %v, want well under a second", d)
	}
	if !ok || len(parts) != 2 || len(parts[0]) != 200_002 || len(parts[1]) != 200_000 {
		t.Errorf("got %d parts, ok %v", len(parts), ok)
	}
}

func TestTextSectionAppliesToTextOnly(t *testing.T) {
	if got, ok := parseNumberFormat(`0.00;(0.00);"-";"* "@" *"`).formatText("hi"); !ok || got != "* hi *" {
		t.Errorf("text section: %q %v", got, ok)
	}
	at := parseNumberFormat("@")
	if got, ok := at.formatText("hi"); !ok || got != "hi" {
		t.Errorf("@: %q %v", got, ok)
	}
	if r := at.formatNumber(3.5); r != (rendered{kind: renderedGeneral, value: 3.5}) {
		t.Errorf("@ on a number: %+v", r)
	}
	if _, ok := parseNumberFormat("0.00").formatText("hi"); ok {
		t.Error("a numeric-only format has no text section")
	}
	if got, ok := parseNumberFormat("0;0;0;").formatText("hi"); !ok || got != "" {
		t.Errorf("an empty fourth section hides text: %q %v", got, ok)
	}
}

func TestBuiltinCodes(t *testing.T) {
	for id, want := range map[int]string{0: "", 5: "", 30: "", 14: "mm-dd-yy", 46: "[h]:mm:ss", 49: "@"} {
		if got := builtinCode(id); got != want {
			t.Errorf("builtin %d = %q, want %q", id, got, want)
		}
	}
}

func TestClosestRationalMatchesExhaustiveSearch(t *testing.T) {
	brute := func(x float64, maxDen uint64) (uint64, uint64) {
		best, bestDen, bestErr := uint64(0), uint64(1), math.Inf(1)
		for d := uint64(1); d <= maxDen; d++ {
			n := math.Round(x * float64(d))
			if e := math.Abs(x - n/float64(d)); e < bestErr-1e-15 {
				best, bestDen, bestErr = uint64(n), d, e
			}
		}
		return best, bestDen
	}
	rng := rand.New(rand.NewPCG(12345, 1))
	for range 2000 {
		x := rng.Float64() * 10
		for places := 1; places <= 4; places++ {
			maxDen := uint64(math.Pow10(places)) - 1
			n, d := closestRational(x, maxDen)
			bn, bd := brute(x, maxDen)
			got := math.Abs(x - float64(n)/float64(d))
			want := math.Abs(x - float64(bn)/float64(bd))
			if got > want+1e-12 {
				t.Fatalf("x=%v maxDen=%d: %d/%d vs %d/%d", x, maxDen, n, d, bn, bd)
			}
		}
	}
}

func TestClosestRationalStaysExactPastFloatPrecision(t *testing.T) {
	if n, d := closestRational(1.0/3.0, math.MaxUint64); n != 6004799503160661 || d != 18014398509481984 {
		t.Errorf("got %d/%d", n, d)
	}
}

func TestFormatFloat(t *testing.T) {
	cases := map[float64]string{
		0.0000004:             "0.0000004",
		12:                    "12",
		1.5:                   "1.5",
		3554.7000000000003:    "3554.7",
		5649.5599999999995:    "5649.56",
		346_289_529.491_800_1: "346289529.4918",
		1:                     "1",
		math.Inf(1):           "inf",
		math.Inf(-1):          "-inf",
	}
	for v, want := range cases {
		if got := formatFloat(v); got != want {
			t.Errorf("formatFloat(%v) = %q, want %q", v, got, want)
		}
	}
	if got := formatFloat(math.NaN()); got != "NaN" {
		t.Errorf("NaN = %q", got)
	}
}

func TestClockFormatting(t *testing.T) {
	if got := formatTimeOfDay(32_694.184 / 86_400.0); got != "09:04:54" {
		t.Errorf("time of day = %q", got)
	}
	if got := formatTimeOfDay(0); got != "00:00:00" {
		t.Errorf("midnight = %q", got)
	}
	if got := formatDurationDays((26*3600 + 30*60 + 15) / 86_400.0); got != "26:30:15" {
		t.Errorf("duration = %q", got)
	}
	if got := formatDurationDays(-0.5); got != "-12:00:00" {
		t.Errorf("negative duration = %q", got)
	}
}

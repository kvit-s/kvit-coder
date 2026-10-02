package gfm

import (
	"testing"
	"unicode/utf8"
)

var escapeOptions = []escapeOpts{
	{},
	{atLineStart: true},
	{styled: true},
	{trailingActive: true},
	{trailingNonspace: true, inLabel: true},
	{atLineStart: true, trailingDelims: delims{true, true, true, true, true, true}},
}

func sameEscape(t *testing.T, text string) {
	t.Helper()
	for _, ctx := range []context{blockContext, headingContext, cellContext} {
		for _, o := range escapeOptions {
			if got, want := escapeText(text, ctx, o), escapeRunes(text, ctx, o); got != want {
				t.Errorf("escapeText(%q, %v, %+v) = %q, full scan %q", text, ctx, o, got, want)
			}
		}
	}
	var fast, full delims
	fast.insertClosers(text)
	if utf8.ValidString(text) {
		for j, chars := 0, []rune(text); j < len(chars); {
			end := runEnd(chars, j)
			if slot := partnerSlot(chars, j, end); slot >= 0 {
				full[slot] = true
			}
			j = end
		}
		if fast != full {
			t.Errorf("insertClosers(%q) = %v, full scan %v", text, fast, full)
		}
	}
}

func TestEscapeFastPathsMatchFullScan(t *testing.T) {
	for _, text := range []string{
		"", "plain words", "12. not a list", "1) x", "Owner 12", "3.25", "\n12. after break",
		"a*b", "_x_", "~", "`code`", "[link]", "$5", "<b>", "!", "|", "&amp;", "# h", "- item", "+ x", "> q", "==",
		"café \u00a0 \u2028", "bad \xff utf8", "tab\tsep", "line\nbreak\r\n",
	} {
		sameEscape(t, text)
	}
	for c := range 128 {
		sameEscape(t, string(rune(c)))
		sameEscape(t, "x"+string(rune(c))+"1 y")
		sameEscape(t, "1"+string(rune(c))+" ")
	}
}

func FuzzEscapeFastPathsMatchFullScan(f *testing.F) {
	f.Add("12. plain")
	f.Add("a_b*c")
	f.Fuzz(sameEscape)
}

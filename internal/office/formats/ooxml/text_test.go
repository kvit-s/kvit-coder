package ooxml

import "testing"

func TestCleanText(t *testing.T) {
	cases := map[string]string{
		"a\u00adb\u200bc\ufeffd\u00a0e": "abcd e",
		"line\r\nbreak\nhere\rend":      "line break here end",
		"tab\there\x01":                 "tab\there",
		"\u0645\u06cc\u200c\u062e":      "\u0645\u06cc\u200c\u062e",
	}
	for in, want := range cases {
		if got := CleanText(in); got != want {
			t.Errorf("CleanText(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsAbsoluteURI(t *testing.T) {
	cases := map[string]bool{
		"https://e.com":         true,
		"mailto:x@y":            true,
		"a:relative-scheme-uri": true,
		"1http:x":               false,
		"no scheme here":        false,
		":empty":                false,
		"path/with:colon":       false,
		`C:\docs\a.doc`:         false,
		"c:/docs/a.doc":         false,
	}
	for in, want := range cases {
		if got := IsAbsoluteURI(in); got != want {
			t.Errorf("IsAbsoluteURI(%q) = %v, want %v", in, got, want)
		}
	}
}

package ui

import (
	"strings"
	"testing"
)

func TestTerminalTitleWritesStateAndSession(t *testing.T) {
	var buf strings.Builder
	title := NewTerminalTitle(&buf)

	title.Running("fix the parser")
	title.Asking("fix the parser")
	title.Waiting("fix the parser")

	want := "\033]0;" + DefaultTitleRunningIcon + " fix the parser\007" +
		"\033]0;" + DefaultTitleAskingIcon + " fix the parser\007" +
		"\033]0;" + DefaultTitleWaitingIcon + " fix the parser\007"
	if got := buf.String(); got != want {
		t.Fatalf("title stream = %q, want %q", got, want)
	}
}

func TestTerminalTitleSkipsRepeats(t *testing.T) {
	var buf strings.Builder
	title := NewTerminalTitle(&buf)

	title.Running("a session")
	before := buf.Len()
	title.Running("a session")
	if buf.Len() != before {
		t.Fatalf("repeating a title wrote %q", buf.String()[before:])
	}
}

func TestTerminalTitleSilentWhenOff(t *testing.T) {
	var buf strings.Builder
	title := NewTerminalTitle(&buf)
	title.Disable()
	title.Running("a session")
	title.Waiting("a session")
	title.Clear()
	if buf.Len() != 0 {
		t.Fatalf("an off title setter wrote %q", buf.String())
	}

	buf.Reset()
	title = NewTerminalTitle(&buf)
	title.Disable()
	title.Running("a session")
	if buf.Len() != 0 {
		t.Fatalf("a disabled title setter wrote %q", buf.String())
	}
}

func TestTerminalTitleCustomAndEmptyIcons(t *testing.T) {
	var buf strings.Builder
	title := NewTerminalTitle(&buf)
	title.SetIcons("*", "", "?")

	title.Running("build")
	title.Waiting("build")
	title.Asking("build")

	want := "\033]0;* build\007" + "\033]0;build\007" + "\033]0;? build\007"
	if got := buf.String(); got != want {
		t.Fatalf("title stream = %q, want %q", got, want)
	}
}

func TestTerminalTitleClearEmptiesIt(t *testing.T) {
	var buf strings.Builder
	title := NewTerminalTitle(&buf)
	title.Running("build")
	buf.Reset()

	title.Clear()
	if got, want := buf.String(), "\033]0;\007"; got != want {
		t.Fatalf("clear wrote %q, want %q", got, want)
	}
}

func TestSanitizeTitle(t *testing.T) {
	long := strings.Repeat("word ", 40)
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"plain", "add the retry loop", "add the retry loop"},
		{"strips control characters", "add\033]0;evil\007 the loop", "add ]0;evil the loop"},
		{"collapses whitespace", "add\tthe   loop\n", "add the loop"},
		{"keeps non-ascii", "убрать цикл", "убрать цикл"},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sanitizeTitle(tc.in); got != tc.want {
				t.Fatalf("sanitizeTitle(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	got := sanitizeTitle(long)
	if n := len([]rune(got)); n != titleMaxRunes {
		t.Fatalf("a long title came back %d runes, want %d", n, titleMaxRunes)
	}
	if !strings.HasSuffix(got, "…") {
		t.Fatalf("a cut title should end in an ellipsis, got %q", got)
	}
}

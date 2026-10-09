package main

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/inbox"
)

// byteReader dribbles out one byte per Read, so a "\r\n" split across reads
// exercises the splitter's wait-for-more path.
type byteReader struct{ s string }

func (b *byteReader) Read(p []byte) (int, error) {
	if len(b.s) == 0 {
		return 0, io.EOF
	}
	p[0] = b.s[0]
	b.s = b.s[1:]
	return 1, nil
}

func drainTexts(t *testing.T, box *inbox.Inbox) []string {
	t.Helper()
	var out []string
	for _, m := range box.Drain() {
		out = append(out, m.Text)
	}
	return out
}

// TestReadStdinLinesHandlesBareCR: steering split by lone carriage returns —
// what Enter looks like with the console in raw mode — still queues.
func TestReadStdinLinesHandlesBareCR(t *testing.T) {
	box := inbox.New("")
	var out bytes.Buffer
	w := testWriter(&out)
	readStdinLines(strings.NewReader("take the stairs\rcheck the tests\r\nand again\n"), box, w)
	if got, want := drainTexts(t, box),
		[]string{"take the stairs", "check the tests", "and again"}; !equalStrings(got, want) {
		t.Fatalf("drained %q, want %q", got, want)
	}
	if n := strings.Count(out.String(), "queued"); n != 3 {
		t.Errorf("reported %d queued lines, want 3: %q", n, out.String())
	}
}

// TestReadStdinLinesBareCREmptyRequestsPause: a lone "\r" with nothing on it
// is an empty Enter, so it asks for a pause rather than vanishing the way it
// would in a splitter that only ends lines on "\n".
func TestReadStdinLinesBareCREmptyRequestsPause(t *testing.T) {
	box := inbox.New("")
	var out bytes.Buffer
	w := testWriter(&out)
	readStdinLines(strings.NewReader("\r"), box, w)
	if !box.PauseRequested() {
		t.Fatal("a bare-CR Enter did not request a pause")
	}
	if !strings.Contains(out.String(), "pause requested") {
		t.Errorf("no pause notice: %q", out.String())
	}
}

// TestReadStdinLinesSplitCRLF: a "\r\n" divided between reads is one line
// break, not two, and a trailing bare "\r" still ends its line.
func TestReadStdinLinesSplitCRLF(t *testing.T) {
	box := inbox.New("")
	var out bytes.Buffer
	w := testWriter(&out)
	readStdinLines(&byteReader{s: "a\r\nb\rc\n"}, box, w)
	if got, want := drainTexts(t, box), []string{"a", "b", "c"}; !equalStrings(got, want) {
		t.Fatalf("drained %q, want %q", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

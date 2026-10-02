package model

import "testing"

func TestMarkerLabels(t *testing.T) {
	cases := []struct {
		kind MarkerKind
		n    int
		want string
	}{
		{Bullet, 4, "-"},
		{Decimal, 7, "7."},
		{LowerAlpha, 3, "c."},
		{LowerAlpha, 27, "aa."},
		{UpperAlpha, 2, "B."},
		{LowerRoman, 4, "iv."},
		{LowerRoman, 1994, "mcmxciv."},
		{UpperRoman, 9, "IX."},
		{LowerRoman, 4000, "4000."},
		{LowerAlpha, 0, "0."},
	}
	for _, c := range cases {
		if got := c.kind.Label(c.n); got != c.want {
			t.Errorf("%v.Label(%d) = %q, want %q", c.kind, c.n, got, c.want)
		}
	}
	if Bullet.Ordered() || !UpperRoman.Ordered() {
		t.Error("only bullets are unordered")
	}
}

func TestPlainTextAndEmptiness(t *testing.T) {
	inlines := []Inline{
		Text{Text: "a", Style: Style{Bold: true}},
		Link{Content: []Inline{Text{Text: "b"}}, Target: Target{Kind: TargetExternal, Ref: "https://e.test"}},
		Image{Alt: "c"},
		Anchor("x"),
		NoteRef("n"),
		LineBreak{},
		Math("d"),
		Checkbox(true),
	}
	if got := PlainText(inlines); got != "abc\nd[x]" {
		t.Fatalf("PlainText = %q", got)
	}
	if IsEmpty(inlines) {
		t.Fatal("content is not empty")
	}
	blank := []Inline{Text{Text: "  "}, Anchor("x"), LineBreak{}, Link{Content: []Inline{Text{Text: " "}}}, Math(" ")}
	if !IsEmpty(blank) {
		t.Fatal("whitespace, anchors, breaks and empty links are empty")
	}
	for _, in := range []Inline{Image{}, NoteRef("n"), Checkbox(false)} {
		if IsEmpty([]Inline{in}) {
			t.Fatalf("%T always counts as content", in)
		}
	}
	if !(Cell{Blocks: []Block{Paragraph{}}}).IsEmpty() || (Cell{Blocks: []Block{Rule{}}}).IsEmpty() {
		t.Fatal("only paragraphs count toward cell emptiness")
	}
}

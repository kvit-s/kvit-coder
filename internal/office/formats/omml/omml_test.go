package omml

import (
	"testing"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

func omml(t *testing.T, inner string) string {
	t.Helper()
	root, err := opc.ParseXML([]byte(`<m:oMath xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math">` + inner + `</m:oMath>`))
	if err != nil {
		t.Fatal(err)
	}
	return string(Math(root))
}

func TestOMMLToLaTeX(t *testing.T) {
	cases := []struct{ xml, want string }{
		{`<m:f><m:num><m:r><m:t>a</m:t></m:r></m:num><m:den><m:r><m:t>b</m:t></m:r></m:den></m:f>` +
			`<m:sSup><m:e><m:r><m:t>x</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup>` +
			`<m:rad><m:radPr><m:degHide m:val="1"/></m:radPr><m:deg/><m:e><m:r><m:t>y</m:t></m:r></m:e></m:rad>` +
			`<m:nary><m:naryPr><m:chr m:val="∑"/><m:limLoc m:val="undOvr"/></m:naryPr>` +
			`<m:sub><m:r><m:t>i=1</m:t></m:r></m:sub><m:sup><m:r><m:t>n</m:t></m:r></m:sup><m:e><m:r><m:t>i</m:t></m:r></m:e></m:nary>` +
			`<m:d><m:dPr><m:begChr m:val="⟨"/><m:endChr m:val="⟩"/></m:dPr><m:e><m:r><m:t>α</m:t></m:r></m:e></m:d>` +
			`<m:func><m:fName><m:r><m:rPr><m:sty m:val="p"/></m:rPr><m:t>sin</m:t></m:r></m:fName><m:e><m:r><m:t>θ</m:t></m:r></m:e></m:func>`,
			`\frac{a}{b}x^{2}\sqrt{y}\sum_{i=1}^{n}{i}\left\langle\alpha\right\rangle\sin{\theta}`},
		{`<m:r><m:rPr><m:nor/></m:rPr><m:t>for all </m:t></m:r><m:r><m:t>𝐱 ∈ ℝ</m:t></m:r>`,
			`\text{for all }\mathbf{x} \in \mathbb{R}`},
		{`<m:f><m:fPr><m:type>lin</m:type></m:fPr><m:num><m:r>a</m:r></m:num><m:den><m:r><m:sty>0</m:sty>b</m:r></m:den></m:f>`,
			`{a}/{\mathrm{b}}`},
		{`<m:m><m:mr><m:e><m:r><m:t>1</m:t></m:r></m:e><m:e><m:r><m:t>0</m:t></m:r></m:e></m:mr><m:mr><m:e><m:r><m:t>0</m:t></m:r></m:e><m:e><m:r><m:t>1</m:t></m:r></m:e></m:mr></m:m>`,
			`\begin{matrix} 1 & 0 \\ 0 & 1 \end{matrix}`},
		{`<m:eqArr><m:e><m:r><m:t>x&amp;=1</m:t></m:r></m:e><m:e><m:r><m:t>y&amp;=2</m:t></m:r></m:e></m:eqArr>`,
			`\begin{aligned} x & =1 \\ y & =2 \end{aligned}`},
		{`<m:func><m:fName><m:limLow><m:e><m:r><m:t>lim</m:t></m:r></m:e><m:lim><m:r><m:t>x→0</m:t></m:r></m:lim></m:limLow></m:fName><m:e><m:r><m:t>f</m:t></m:r></m:e></m:func>`,
			`\lim_{x\to0}{f}`},
	}
	for _, c := range cases {
		if got := omml(t, c.xml); got != c.want {
			t.Errorf("got  %s\nwant %s", got, c.want)
		}
	}
}

func TestBlocksSplitEquationLines(t *testing.T) {
	root, err := opc.ParseXML([]byte(`<m:oMathPara xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><m:oMath><m:r><m:t>a</m:t></m:r></m:oMath><m:oMath/><m:oMath><m:r><m:t>b</m:t></m:r></m:oMath></m:oMathPara>`))
	if err != nil {
		t.Fatal(err)
	}
	got := Blocks(root)
	if len(got) != 2 || got[0] != model.MathBlock("a") || got[1] != model.MathBlock("b") {
		t.Fatalf("blocks %#v", got)
	}
}

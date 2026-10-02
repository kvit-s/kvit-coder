// Package omml converts Office Math (m:oMath) trees, shared by
// WordprocessingML and PresentationML, into LaTeX math for the model.
package omml

import (
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

const nsM = opc.NSMath

type mathMode int

const (
	mathNormal mathMode = iota
	// mathFuncName is inside m:fName, where run text names a function.
	mathFuncName
	// mathArray is inside an equation-array row, where & marks alignment.
	mathArray
)

// Math converts one m:oMath to inline math; "" when it has no content.
func Math(omath *opc.Element) model.Math {
	var t texBuf
	mathChildren(omath, &t, mathNormal)
	return model.Math(t.finish())
}

// Blocks converts an m:oMathPara, or a lone m:oMath, to displayed math, one
// block per equation line.
func Blocks(para *opc.Element) []model.Block {
	var blocks []model.Block
	for omath := range para.Children(nsM, "oMath") {
		if tex := Math(omath); tex != "" {
			blocks = append(blocks, model.MathBlock(tex))
		}
	}
	if len(blocks) == 0 {
		if whole := Math(para); whole != "" {
			blocks = []model.Block{model.MathBlock(whole)}
		}
	}
	return blocks
}

func mathChildren(e *opc.Element, t *texBuf, mode mathMode) {
	for c := range e.Elements() {
		mathElem(c, t, mode)
	}
}

func mathElem(e *opc.Element, t *texBuf, mode mathMode) {
	if e.Space != nsM {
		switch e.Local {
		case "del", "moveFrom", "rPr", "pPr":
		default:
			mathChildren(e, t, mode)
		}
		return
	}
	switch e.Local {
	case "r":
		mathRun(e, t, mode)
	case "t":
		t.mathText(e.Text())
	case "f":
		fraction(e, t)
	case "sSup":
		t.base(mathArg(e, "e", mathNormal))
		t.char('^')
		t.group(mathArg(e, "sup", mathNormal))
	case "sSub":
		t.base(mathArg(e, "e", mathNormal))
		t.char('_')
		t.group(mathArg(e, "sub", mathNormal))
	case "sSubSup":
		t.base(mathArg(e, "e", mathNormal))
		t.char('_')
		t.group(mathArg(e, "sub", mathNormal))
		t.char('^')
		t.group(mathArg(e, "sup", mathNormal))
	case "sPre":
		t.str("{}_")
		t.group(mathArg(e, "sub", mathNormal))
		t.char('^')
		t.group(mathArg(e, "sup", mathNormal))
		t.group(mathArg(e, "e", mathNormal))
	case "rad":
		deg := mathArg(e, "deg", mathNormal)
		t.macro(`\sqrt`)
		if hide, ok := mathFlag(e, "radPr", "degHide"); !(ok && hide) && !deg.empty() {
			t.char('[')
			t.tex(deg)
			t.char(']')
		}
		t.group(mathArg(e, "e", mathNormal))
	case "d":
		delimited(e, t)
	case "nary":
		nary(e, t)
	case "func":
		if name := e.Child(nsM, "fName"); name != nil {
			mathChildren(name, t, mathFuncName)
		}
		t.group(mathArg(e, "e", mathNormal))
	case "acc":
		cmd := `\hat`
		if chr, ok := firstRune(mathProp(e, "accPr", "chr")); ok {
			if a, ok := accent(chr); ok {
				cmd = a
			}
		}
		t.command(cmd, mathArg(e, "e", mathNormal))
	case "bar":
		cmd := `\underline`
		if pos, _ := mathProp(e, "barPr", "pos"); pos == "top" {
			cmd = `\overline`
		}
		t.command(cmd, mathArg(e, "e", mathNormal))
	case "borderBox":
		t.command(`\boxed`, mathArg(e, "e", mathNormal))
	case "phant":
		if show, ok := mathFlag(e, "phantPr", "show"); ok && !show {
			t.command(`\phantom`, mathArg(e, "e", mathNormal))
		} else {
			t.group(mathArg(e, "e", mode))
		}
	case "box":
		t.group(mathArg(e, "e", mode))
	case "groupChr":
		groupChar(e, t)
	case "limLow", "limUpp":
		limit(e, t, mode)
	case "m":
		matrix(e, t)
	case "eqArr":
		equationArray(e, t)
	default:
		if !strings.HasSuffix(e.Local, "Pr") {
			mathChildren(e, t, mode)
		}
	}
}

func mathArg(parent *opc.Element, name string, mode mathMode) *texBuf {
	var t texBuf
	if c := parent.Child(nsM, name); c != nil {
		mathChildren(c, &t, mode)
	}
	return &t
}

// mathProp reads parent/pr/name/@m:val, or the element's own text where the
// tree came from rtf.
func mathProp(parent *opc.Element, pr, name string) (string, bool) {
	prop := parent.Child(nsM, pr)
	if prop == nil {
		return "", false
	}
	return propValue(prop.Child(nsM, name))
}

func propValue(e *opc.Element) (string, bool) {
	if e == nil {
		return "", false
	}
	if v, ok := e.Attr(nsM, "val"); ok {
		return v, true
	}
	return strings.TrimSpace(directText(e)), true
}

// mathFlag reads an on/off property: present with no value is on.
func mathFlag(parent *opc.Element, pr, name string) (bool, bool) {
	v, ok := mathProp(parent, pr, name)
	if !ok {
		return false, false
	}
	return v != "0" && v != "off" && v != "false", true
}

func firstRune(s string, ok bool) (rune, bool) {
	for _, r := range s {
		return r, ok
	}
	return 0, false
}

func directText(e *opc.Element) string {
	var sb strings.Builder
	for _, n := range e.Nodes {
		if n.Elem == nil {
			sb.WriteString(n.Text)
		}
	}
	return sb.String()
}

func mathRun(r *opc.Element, t *texBuf, mode mathMode) {
	var text string
	hasT := false
	for tt := range r.Children(nsM, "t") {
		hasT = true
		text += tt.Text()
	}
	if !hasT {
		if direct := directText(r); strings.TrimSpace(direct) != "" {
			text = direct
		}
	}
	if text == "" {
		return
	}
	rpr := r.Child(nsM, "rPr")
	value := func(name string) (string, bool) {
		e := r.Child(nsM, name)
		if rpr != nil {
			if fromPr := rpr.Child(nsM, name); fromPr != nil {
				e = fromPr
			}
		}
		return propValue(e)
	}
	if nor, ok := value("nor"); ok && nor != "0" && nor != "off" && nor != "false" {
		t.textMode(text)
		return
	}
	if mode == mathFuncName {
		if name, ok := functionName(text); ok {
			t.macro(name)
			return
		}
	}
	var font string
	scr, _ := value("scr")
	switch scr {
	case "script", "1":
		font = `\mathcal`
	case "fraktur", "2":
		font = `\mathfrak`
	case "double-struck", "3":
		font = `\mathbb`
	case "sans-serif", "4":
		font = `\mathsf`
	case "monospace", "5":
		font = `\mathtt`
	default:
		switch sty, _ := value("sty"); sty {
		case "p", "0":
			font = `\mathrm`
		case "b", "1":
			font = `\mathbf`
		case "bi", "3":
			font = `\boldsymbol`
		}
	}
	var inner texBuf
	if mode == mathArray {
		for i, piece := range strings.Split(text, "&") {
			if i > 0 {
				inner.str(" & ")
			}
			inner.mathText(piece)
		}
	} else {
		inner.mathText(text)
	}
	if font != "" {
		t.command(font, &inner)
	} else {
		t.tex(&inner)
	}
}

func fraction(e *opc.Element, t *texBuf) {
	num := mathArg(e, "num", mathNormal)
	den := mathArg(e, "den", mathNormal)
	switch typ, _ := mathProp(e, "fPr", "type"); typ {
	case "lin":
		t.group(num)
		t.char('/')
		t.group(den)
	case "skw":
		t.str("{}^")
		t.group(num)
		t.str("/_")
		t.group(den)
	case "noBar":
		t.char('{')
		t.tex(num)
		t.macro(`\atop`)
		t.tex(den)
		t.char('}')
	default:
		t.macro(`\frac`)
		t.group(num)
		t.group(den)
	}
}

func delimited(e *opc.Element, t *texBuf) {
	// An explicitly empty character means no delimiter on that side.
	chr := func(name string, def rune) (rune, bool) {
		v, ok := mathProp(e, "dPr", name)
		if !ok {
			return def, true
		}
		return firstRune(v, true)
	}
	side := func(name string, def rune) string {
		if c, ok := chr(name, def); ok {
			return delimiter(c)
		}
		return "."
	}
	t.macro(`\left`)
	t.str(side("begChr", '('))
	sep, hasSep := chr("sepChr", '|')
	i := 0
	for part := range e.Children(nsM, "e") {
		if i > 0 {
			if hasSep {
				t.macro(`\middle`)
				t.str(delimiter(sep))
			} else {
				t.char(',')
			}
		}
		i++
		var inner texBuf
		mathChildren(part, &inner, mathNormal)
		t.tex(&inner)
	}
	t.macro(`\right`)
	t.str(side("endChr", ')'))
}

func nary(e *opc.Element, t *texBuf) {
	chr, ok := firstRune(mathProp(e, "naryPr", "chr"))
	if !ok {
		chr = '∫'
	}
	if op, ok := texSymbols[chr]; ok && strings.HasPrefix(op, `\`) {
		t.macro(op)
	} else {
		t.macro(`\operatorname*`)
		var inner texBuf
		inner.mathChar(chr)
		t.group(&inner)
	}
	integral := strings.ContainsRune("∫∬∭⨌∮∯∰", chr)
	switch loc, _ := mathProp(e, "naryPr", "limLoc"); {
	case loc == "undOvr" && integral:
		t.macro(`\limits`)
	case loc == "subSup" && !integral:
		t.macro(`\nolimits`)
	}
	if hide, ok := mathFlag(e, "naryPr", "subHide"); !(ok && hide) {
		if sub := mathArg(e, "sub", mathNormal); !sub.empty() {
			t.char('_')
			t.group(sub)
		}
	}
	if hide, ok := mathFlag(e, "naryPr", "supHide"); !(ok && hide) {
		if sup := mathArg(e, "sup", mathNormal); !sup.empty() {
			t.char('^')
			t.group(sup)
		}
	}
	t.group(mathArg(e, "e", mathNormal))
}

func groupChar(e *opc.Element, t *texBuf) {
	chr, ok := firstRune(mathProp(e, "groupChrPr", "chr"))
	if !ok {
		chr = '⏟'
	}
	pos, _ := mathProp(e, "groupChrPr", "pos")
	body := mathArg(e, "e", mathNormal)
	if cmd, ok := accent(chr); ok {
		t.command(cmd, body)
		return
	}
	var mark texBuf
	mark.mathChar(chr)
	if pos == "top" {
		t.macro(`\overset`)
	} else {
		t.macro(`\underset`)
	}
	t.group(&mark)
	t.group(body)
}

func limit(e *opc.Element, t *texBuf, mode mathMode) {
	upper := e.Local == "limUpp"
	base := mathArg(e, "e", mode)
	lim := mathArg(e, "lim", mathNormal)
	if mode == mathFuncName {
		t.tex(base)
		if upper {
			t.char('^')
		} else {
			t.char('_')
		}
		t.group(lim)
		return
	}
	if upper {
		t.macro(`\overset`)
	} else {
		t.macro(`\underset`)
	}
	t.group(lim)
	t.group(base)
}

func matrix(e *opc.Element, t *texBuf) {
	t.str(`\begin{matrix}`)
	i := 0
	for row := range e.Children(nsM, "mr") {
		if i > 0 {
			t.str(` \\`)
		}
		i++
		j := 0
		for cell := range row.Children(nsM, "e") {
			if j > 0 {
				t.str(" & ")
			} else {
				t.str(" ")
			}
			j++
			var inner texBuf
			mathChildren(cell, &inner, mathNormal)
			t.tex(&inner)
		}
	}
	t.str(` \end{matrix}`)
}

func equationArray(e *opc.Element, t *texBuf) {
	var rows []*texBuf
	env := "gathered"
	for row := range e.Children(nsM, "e") {
		var inner texBuf
		mathChildren(row, &inner, mathArray)
		if strings.Contains(inner.String(), "&") {
			env = "aligned"
		}
		rows = append(rows, &inner)
	}
	t.str(`\begin{` + env + `}`)
	for i, row := range rows {
		if i > 0 {
			t.str(` \\`)
		}
		t.char(' ')
		t.tex(row)
	}
	t.str(` \end{` + env + `}`)
}

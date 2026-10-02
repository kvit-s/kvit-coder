package omml

import (
	"strings"
	"unicode"
)

// texBuf accumulates LaTeX, keeping a letter-named control word apart from a
// letter written right after it.
type texBuf struct {
	out        strings.Builder
	afterMacro bool
}

func (t *texBuf) empty() bool { return t.out.Len() == 0 }

func (t *texBuf) String() string { return t.out.String() }

func (t *texBuf) str(s string) {
	if s == "" {
		return
	}
	if t.afterMacro && isASCIILetter(s[0]) {
		t.out.WriteByte(' ')
	}
	t.out.WriteString(s)
	t.afterMacro = endsWithControlWord(s)
}

func (t *texBuf) char(c rune) {
	if t.afterMacro && c < 0x80 && isASCIILetter(byte(c)) {
		t.out.WriteByte(' ')
	}
	t.out.WriteRune(c)
	t.afterMacro = false
}

// macro writes a control word with its backslash; the backslash ends any
// control word before it.
func (t *texBuf) macro(name string) {
	t.out.WriteString(name)
	t.afterMacro = endsWithControlWord(name)
}

func (t *texBuf) tex(other *texBuf) {
	if other.empty() {
		return
	}
	t.str(other.String())
	t.afterMacro = other.afterMacro
}

func (t *texBuf) group(inner *texBuf) {
	t.char('{')
	t.tex(inner)
	t.char('}')
}

// base writes a script base bare when it is one atom (a single character or
// control word) and grouped otherwise.
func (t *texBuf) base(inner *texBuf) {
	s := inner.String()
	name, isMacro := strings.CutPrefix(s, `\`)
	atom := len([]rune(s)) == 1 || isMacro && name != "" && strings.Trim(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") == ""
	if atom {
		t.tex(inner)
	} else {
		t.group(inner)
	}
}

func (t *texBuf) command(name string, inner *texBuf) {
	t.macro(name)
	t.group(inner)
}

// mathText writes text in math mode: symbols become control words, styled
// mathematical alphanumerics fold to their base letters inside the matching
// font command, and TeX specials are escaped.
func (t *texBuf) mathText(text string) {
	var run []rune
	runVariant := variantPlain
	flush := func() {
		if len(run) == 0 {
			return
		}
		var inner texBuf
		inner.mathText(string(run))
		t.command(runVariant.command(), &inner)
		run = nil
	}
	for _, c := range text {
		base, variant, ok := foldAlnum(c)
		switch {
		case ok && variant != variantPlain:
			if len(run) > 0 && variant != runVariant {
				flush()
			}
			runVariant = variant
			run = append(run, base)
		case ok:
			flush()
			t.mathChar(base)
		default:
			flush()
			t.mathChar(c)
		}
	}
	flush()
}

func (t *texBuf) mathChar(c rune) {
	if mapped, ok := texSymbols[c]; ok {
		if strings.HasPrefix(mapped, `\`) {
			t.macro(mapped)
		} else {
			t.str(mapped)
		}
		return
	}
	switch {
	case strings.ContainsRune("{}$%&#_", c):
		t.char('\\')
		t.char(c)
	case c == '\\':
		t.macro(`\backslash`)
	case c == '^':
		t.str("\\char`^")
	case c == '~':
		t.macro(`\sim`)
	case c == '\u00a0':
		t.str(`\ `)
	case unicode.IsSpace(c):
		t.char(' ')
	case unicode.IsControl(c):
	default:
		t.char(c)
	}
}

// textMode writes \text{...}: literal text inside a formula.
func (t *texBuf) textMode(text string) {
	var inner texBuf
	for _, c := range text {
		switch {
		case strings.ContainsRune("{}$%&#_", c):
			inner.char('\\')
			inner.char(c)
		case c == '\\':
			inner.str(`\textbackslash{}`)
		case c == '^':
			inner.str(`\textasciicircum{}`)
		case c == '~':
			inner.str(`\textasciitilde{}`)
		case c == '\u00a0':
			inner.char('~')
		case unicode.IsSpace(c):
			inner.char(' ')
		case unicode.IsControl(c):
		default:
			inner.char(c)
		}
	}
	t.command(`\text`, &inner)
}

// finish trims the source and collapses space runs.
func (t *texBuf) finish() string {
	var sb strings.Builder
	prevSpace := true
	for _, c := range strings.TrimSpace(t.String()) {
		space := c == ' '
		if !space || !prevSpace {
			sb.WriteRune(c)
		}
		prevSpace = space
	}
	return sb.String()
}

func endsWithControlWord(s string) bool {
	trimmed := strings.TrimRight(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	return len(trimmed) < len(s) && strings.HasSuffix(trimmed, `\`)
}

type mathVariant int

const (
	variantPlain mathVariant = iota
	variantBold
	variantBoldItalic
	variantScript
	variantFraktur
	variantDoubleStruck
	variantSansSerif
	variantMonospace
)

func (v mathVariant) command() string {
	return [...]string{`\mathit`, `\mathbf`, `\boldsymbol`, `\mathcal`, `\mathfrak`, `\mathbb`, `\mathsf`, `\mathtt`}[v]
}

var letterlike = map[rune]struct {
	base    rune
	variant mathVariant
}{
	'ℎ': {'h', variantPlain}, 'ℬ': {'B', variantScript}, 'ℰ': {'E', variantScript},
	'ℱ': {'F', variantScript}, 'ℋ': {'H', variantScript}, 'ℐ': {'I', variantScript},
	'ℒ': {'L', variantScript}, 'ℳ': {'M', variantScript}, 'ℛ': {'R', variantScript},
	'ℯ': {'e', variantScript}, 'ℊ': {'g', variantScript}, 'ℴ': {'o', variantScript},
	'ℭ': {'C', variantFraktur}, 'ℌ': {'H', variantFraktur}, 'ℑ': {'I', variantFraktur},
	'ℜ': {'R', variantFraktur}, 'ℨ': {'Z', variantFraktur}, 'ℂ': {'C', variantDoubleStruck},
	'ℍ': {'H', variantDoubleStruck}, 'ℕ': {'N', variantDoubleStruck}, 'ℙ': {'P', variantDoubleStruck},
	'ℚ': {'Q', variantDoubleStruck}, 'ℝ': {'R', variantDoubleStruck}, 'ℤ': {'Z', variantDoubleStruck},
}

var (
	greekUpper = []rune("ΑΒΓΔΕΖΗΘΙΚΛΜΝΞΟΠΡϴΣΤΥΦΧΨΩ")
	greekLower = []rune("αβγδεζηθικλμνξοπρςστυφχψω")
	greekExtra = []rune("∂ϵϑϰϕϱϖ")
)

// foldAlnum folds a Mathematical Alphanumeric Symbol or letterlike symbol to
// its base character and font variant; italic is the default math style and
// folds to plain.
func foldAlnum(c rune) (rune, mathVariant, bool) {
	if l, ok := letterlike[c]; ok {
		return l.base, l.variant, true
	}
	if c < 0x1D400 || c >= 0x1D800 {
		return 0, 0, false
	}
	if offset := c - 0x1D400; offset < 13*52 {
		latin := [13]mathVariant{variantBold, variantPlain, variantBoldItalic, variantScript, variantScript, variantFraktur,
			variantDoubleStruck, variantFraktur, variantSansSerif, variantSansSerif, variantSansSerif, variantSansSerif, variantMonospace}
		i := offset % 52
		base := 'A' + i
		if i >= 26 {
			base = 'a' + i - 26
		}
		return base, latin[offset/52], true
	}
	if greek := c - 0x1D6A8; greek >= 0 && greek < 5*58 {
		styles := [5]mathVariant{variantBold, variantPlain, variantBoldItalic, variantSansSerif, variantSansSerif}
		var base rune
		switch i := int(greek % 58); {
		case i <= 24:
			base = greekUpper[i]
		case i == 25:
			base = '∇'
		case i <= 50:
			base = greekLower[i-26]
		default:
			base = greekExtra[i-51]
		}
		return base, styles[greek/58], true
	}
	if digit := c - 0x1D7CE; digit >= 0 && digit < 5*10 {
		styles := [5]mathVariant{variantBold, variantDoubleStruck, variantSansSerif, variantSansSerif, variantMonospace}
		return '0' + digit%10, styles[digit/10], true
	}
	return 0, 0, false
}

func delimiter(c rune) string {
	switch c {
	case '(', '⟮':
		return "("
	case ')', '⟯':
		return ")"
	case '[':
		return "["
	case ']':
		return "]"
	case '{':
		return `\{`
	case '}':
		return `\}`
	case '|', '∣':
		return "|"
	case '‖', '∥':
		return `\Vert`
	case '⟨', '〈':
		return `\langle`
	case '⟩', '〉':
		return `\rangle`
	case '⌈':
		return `\lceil`
	case '⌉':
		return `\rceil`
	case '⌊':
		return `\lfloor`
	case '⌋':
		return `\rfloor`
	case '/':
		return "/"
	case '\\':
		return `\backslash`
	case '↑':
		return `\uparrow`
	case '↓':
		return `\downarrow`
	case '↕':
		return `\updownarrow`
	case '⇑':
		return `\Uparrow`
	case '⇓':
		return `\Downarrow`
	case '⇕':
		return `\Updownarrow`
	}
	return "."
}

func accent(c rune) (string, bool) {
	switch c {
	case '\u0300', '`':
		return `\grave`, true
	case '\u0301', '´':
		return `\acute`, true
	case '\u0302', '^', 'ˆ':
		return `\hat`, true
	case '\u0303', '~', '˜':
		return `\tilde`, true
	case '\u0304', '¯', 'ˉ':
		return `\bar`, true
	case '\u0305', '‾', '⎴':
		return `\overline`, true
	case '\u0306', '˘':
		return `\breve`, true
	case '\u0307', '˙':
		return `\dot`, true
	case '\u0308', '¨':
		return `\ddot`, true
	case '\u030a', '˚':
		return `\mathring`, true
	case '\u030c', 'ˇ':
		return `\check`, true
	case '\u0332', '_', '⎵':
		return `\underline`, true
	case '\u20d6', '←':
		return `\overleftarrow`, true
	case '\u20d7', '→':
		return `\vec`, true
	case '\u20e1', '↔':
		return `\overleftrightarrow`, true
	case '\u20db':
		return `\dddot`, true
	case '\u20dc':
		return `\ddddot`, true
	case '\u20ee':
		return `\underleftarrow`, true
	case '\u20ef':
		return `\underrightarrow`, true
	case '⏞', '︷':
		return `\overbrace`, true
	case '⏟', '︸':
		return `\underbrace`, true
	}
	return "", false
}

var knownFunctions = map[string]bool{
	"arccos": true, "arcsin": true, "arctan": true, "arg": true, "cos": true, "cosh": true, "cot": true,
	"coth": true, "csc": true, "deg": true, "det": true, "dim": true, "exp": true, "gcd": true, "hom": true,
	"inf": true, "ker": true, "lg": true, "lim": true, "liminf": true, "limsup": true, "ln": true, "log": true,
	"max": true, "min": true, "Pr": true, "sec": true, "sin": true, "sinh": true, "sup": true, "tan": true, "tanh": true,
}

// functionName is the control word for a predefined function name, or an
// \operatorname for any other alphabetic name.
func functionName(name string) (string, bool) {
	name = strings.TrimSpace(name)
	if name == "" || strings.Trim(name, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") != "" {
		return "", false
	}
	if knownFunctions[name] {
		return `\` + name, true
	}
	return `\operatorname{` + name + `}`, true
}

var texSymbols = map[rune]string{
	'α': `\alpha`, 'β': `\beta`, 'γ': `\gamma`, 'δ': `\delta`, 'ε': `\varepsilon`, 'ϵ': `\epsilon`,
	'ζ': `\zeta`, 'η': `\eta`, 'θ': `\theta`, 'ϑ': `\vartheta`, 'ι': `\iota`, 'κ': `\kappa`,
	'ϰ': `\varkappa`, 'λ': `\lambda`, 'μ': `\mu`, 'ν': `\nu`, 'ξ': `\xi`, 'ο': "o", 'π': `\pi`,
	'ϖ': `\varpi`, 'ρ': `\rho`, 'ϱ': `\varrho`, 'σ': `\sigma`, 'ς': `\varsigma`, 'τ': `\tau`,
	'υ': `\upsilon`, 'φ': `\varphi`, 'ϕ': `\phi`, 'χ': `\chi`, 'ψ': `\psi`, 'ω': `\omega`,
	'Α': "A", 'Β': "B", 'Γ': `\Gamma`, 'Δ': `\Delta`, 'Ε': "E", 'Ζ': "Z", 'Η': "H",
	'Θ': `\Theta`, 'ϴ': `\Theta`, 'Ι': "I", 'Κ': "K", 'Λ': `\Lambda`, 'Μ': "M", 'Ν': "N",
	'Ξ': `\Xi`, 'Ο': "O", 'Π': `\Pi`, 'Ρ': "P", 'Σ': `\Sigma`, 'Τ': "T", 'Υ': `\Upsilon`,
	'Φ': `\Phi`, 'Χ': "X", 'Ψ': `\Psi`, 'Ω': `\Omega`,
	'∑': `\sum`, '∏': `\prod`, '∐': `\coprod`, '∫': `\int`, '∬': `\iint`, '∭': `\iiint`,
	'⨌': `\iiiint`, '∮': `\oint`, '∯': `\oiint`, '∰': `\oiiint`, '⋃': `\bigcup`, '⋂': `\bigcap`,
	'⋁': `\bigvee`, '⋀': `\bigwedge`, '⨁': `\bigoplus`, '⨂': `\bigotimes`, '⨀': `\bigodot`,
	'⨄': `\biguplus`, '⨆': `\bigsqcup`,
	'×': `\times`, '÷': `\div`, '±': `\pm`, '∓': `\mp`, '⋅': `\cdot`, '·': `\cdot`, '∙': `\cdot`,
	'∗': `\ast`, '∘': `\circ`, '∖': `\setminus`, '⊕': `\oplus`, '⊖': `\ominus`, '⊗': `\otimes`,
	'⊘': `\oslash`, '⊙': `\odot`, '∪': `\cup`, '∩': `\cap`, '⊎': `\uplus`, '⊓': `\sqcap`,
	'⊔': `\sqcup`, '∧': `\wedge`, '∨': `\vee`, '†': `\dagger`, '‡': `\ddagger`, '⋆': `\star`,
	'≤': `\le`, '⩽': `\le`, '≥': `\ge`, '⩾': `\ge`, '≠': `\ne`, '≈': `\approx`, '≡': `\equiv`,
	'≢': `\not\equiv`, '≅': `\cong`, '≃': `\simeq`, '∼': `\sim`, '≁': `\nsim`, '∝': `\propto`,
	'≪': `\ll`, '≫': `\gg`, '≺': `\prec`, '≻': `\succ`, '⪯': `\preceq`, '≼': `\preceq`,
	'⪰': `\succeq`, '≽': `\succeq`, '⊂': `\subset`, '⊃': `\supset`, '⊆': `\subseteq`,
	'⊇': `\supseteq`, '⊄': `\not\subset`, '⊈': `\nsubseteq`, '⊊': `\subsetneq`, '⊋': `\supsetneq`,
	'⊏': `\sqsubset`, '⊐': `\sqsupset`, '⊑': `\sqsubseteq`, '⊒': `\sqsupseteq`, '∈': `\in`,
	'∉': `\notin`, '∋': `\ni`, '∌': `\not\ni`, '≐': `\doteq`, '≜': `\triangleq`,
	'≝': `\overset{\mathrm{def}}{=}`, '≔': `\coloneqq`, '⊥': `\perp`, '∥': `\parallel`,
	'∦': `\nparallel`, '⊢': `\vdash`, '⊣': `\dashv`, '⊨': `\models`, '⊤': `\top`, '≍': `\asymp`,
	'≀': `\wr`, '⋈': `\bowtie`, '∣': `\mid`, '∤': `\nmid`,
	'→': `\to`, '←': `\leftarrow`, '↔': `\leftrightarrow`, '⇒': `\Rightarrow`, '⇐': `\Leftarrow`,
	'⇔': `\Leftrightarrow`, '↑': `\uparrow`, '↓': `\downarrow`, '↕': `\updownarrow`, '⇑': `\Uparrow`,
	'⇓': `\Downarrow`, '⇕': `\Updownarrow`, '↦': `\mapsto`, '⟶': `\longrightarrow`,
	'⟵': `\longleftarrow`, '⟷': `\longleftrightarrow`, '⟹': `\Longrightarrow`, '⟸': `\Longleftarrow`,
	'⟺': `\Longleftrightarrow`, '⟼': `\longmapsto`, '↗': `\nearrow`, '↘': `\searrow`,
	'↙': `\swarrow`, '↖': `\nwarrow`, '↩': `\hookleftarrow`, '↪': `\hookrightarrow`,
	'⇀': `\rightharpoonup`, '↼': `\leftharpoonup`, '⇌': `\rightleftharpoons`,
	'↶': `\curvearrowleft`, '↷': `\curvearrowright`,
	'∀': `\forall`, '∃': `\exists`, '∄': `\nexists`, '¬': `\neg`, '∅': `\emptyset`, '⌀': `\emptyset`,
	'∞': `\infty`, '∂': `\partial`, '∇': `\nabla`, '∴': `\therefore`, '∵': `\because`, 'ℵ': `\aleph`,
	'ℶ': `\beth`, 'ℏ': `\hbar`, 'ℓ': `\ell`, '℘': `\wp`, '℧': `\mho`, '√': `\surd`, '∠': `\angle`,
	'∡': `\measuredangle`, '△': `\triangle`, '□': `\square`, '◻': `\square`, '◊': `\lozenge`,
	'°': `^{\circ}`, '′': "'", '″': "''", '‴': "'''", '⁗': "''''", '…': `\ldots`, '⋯': `\cdots`,
	'⋮': `\vdots`, '⋱': `\ddots`, '⋰': `\iddots`,
	'⟨': `\langle`, '〈': `\langle`, '⟩': `\rangle`, '〉': `\rangle`, '⌈': `\lceil`, '⌉': `\rceil`,
	'⌊': `\lfloor`, '⌋': `\rfloor`, '‖': `\Vert`,
	'−': "-", '‐': "-", '‑': "-", '‒': "-", '–': "-",
	'\u2009': `\,`, '\u200a': `\,`, '\u2006': `\,`, '\u2005': `\:`, '\u2004': `\:`,
	'\u2003': `\;`, '\u2002': `\;`,
	'\u2061': "", '\u2062': "", '\u2063': "", '\u2064': "", '\u200b': "", '\ufeff': "",
}

func isASCIILetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

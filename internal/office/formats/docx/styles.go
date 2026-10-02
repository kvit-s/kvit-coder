package docx

import (
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

// toggles is the parity of true bold/italic/strike specifications along a
// style chain. These are toggle properties (ECMA-376 §17.7.3): within the
// style hierarchy a true specification flips the inherited value and a false
// one leaves it unchanged, so the parity is XORed over the docDefaults base.
// Direct run formatting is absolute on/off instead.
type toggles struct {
	bold, italic, strike bool
}

func (t toggles) over(base model.Style) model.Style {
	base.Bold = base.Bold != t.bold
	base.Italic = base.Italic != t.italic
	base.Strike = base.Strike != t.strike
	return base
}

type styleDef struct {
	elem   *opc.Element
	parent string
}

// styleProps are a style's effective properties through its basedOn chain.
type styleProps struct {
	toggles    toggles
	heading    int
	hasHeading bool
	kind       blockKind
	numID      int
	hasNumID   bool
}

// resolvedStyle memoizes a style's properties, or the cycle error every
// lookup through it reports.
type resolvedStyle struct {
	props styleProps
	err   error
}

type numberingLevelKey struct {
	inst *instance
	id   string
}

type numberingLevel struct {
	level int
	found bool
}

type styles struct {
	defs        map[string]styleDef
	docDefaults model.Style
	resolved    map[string]resolvedStyle
	levels      map[numberingLevelKey]numberingLevel
}

func parseStyles(root *opc.Element) *styles {
	s := &styles{
		defs:     map[string]styleDef{},
		resolved: map[string]resolvedStyle{},
		levels:   map[numberingLevelKey]numberingLevel{},
	}
	if root == nil {
		return s
	}
	for style := range root.Children(nsW, "style") {
		if id, ok := style.Attr(nsW, "styleId"); ok {
			parent, _ := val(style, "basedOn")
			s.defs[id] = styleDef{elem: style, parent: parent}
		}
	}
	if rpr := child(child(child(root, "docDefaults"), "rPrDefault"), "rPr"); rpr != nil {
		s.docDefaults = applyDirect(rpr, model.Style{})
	}
	return s
}

// resolve returns a style's effective properties, resolving each style of
// the basedOn chain at most once per document. A dangling reference ends
// the chain; a cycle is malformed, reported at the first style the chain
// revisits.
func (s *styles) resolve(id string) (styleProps, error) {
	if r, ok := s.resolved[id]; ok {
		return r.props, r.err
	}
	var chain []string
	onChain := map[string]int{}
	var base resolvedStyle
	for cur := id; ; {
		if r, ok := s.resolved[cur]; ok {
			base = r
			break
		}
		def, ok := s.defs[cur]
		if !ok {
			break
		}
		if at, ok := onChain[cur]; ok {
			for _, member := range chain[at:] {
				s.resolved[member] = resolvedStyle{err: styleCycle(member)}
			}
			base = resolvedStyle{err: styleCycle(cur)}
			chain = chain[:at]
			break
		}
		onChain[cur] = len(chain)
		chain = append(chain, cur)
		cur = def.parent
	}
	for i := len(chain) - 1; i >= 0; i-- {
		if base.err == nil {
			base.props = ownProps(s.defs[chain[i]].elem, base.props)
		}
		s.resolved[chain[i]] = base
	}
	if r, ok := s.resolved[id]; ok {
		return r.props, r.err
	}
	return styleProps{}, nil
}

func styleCycle(id string) error {
	return &opc.MalformedError{Detail: "style inheritance cycle at " + strconv.Quote(id)}
}

// ownProps overlays a style's own specification on its parent's effective
// properties: toggles XOR along the chain, everything else is inherited
// unless the style specifies it.
func ownProps(style *opc.Element, parent styleProps) styleProps {
	p := parent
	if rpr := child(style, "rPr"); rpr != nil {
		b, _ := onOff(rpr, "b")
		i, _ := onOff(rpr, "i")
		st, _ := onOff(rpr, "strike")
		ds, _ := onOff(rpr, "dstrike")
		p.toggles.bold = p.toggles.bold != b
		p.toggles.italic = p.toggles.italic != i
		p.toggles.strike = p.toggles.strike != (st || ds)
	}
	if level, ok := ownHeadingLevel(style); ok {
		p.heading, p.hasHeading = level, true
	}
	if name, ok := val(style, "name"); ok {
		if kind := blockKindOf(name); kind != noBlock {
			p.kind = kind
		}
	}
	if numID, ok := parseNumID(child(child(style, "pPr"), "numPr")); ok {
		p.numID, p.hasNumID = numID, true
	}
	return p
}

// ownHeadingLevel reads a style's heading level from its name ("heading
// N", "Title") or outlineLvl; level 0 is the explicit off value (outlineLvl
// 9), which stops inheritance.
func ownHeadingLevel(style *opc.Element) (int, bool) {
	name, _ := val(style, "name")
	name = strings.ToLower(name)
	if rest, ok := strings.CutPrefix(name, "heading "); ok {
		if n, err := strconv.ParseUint(strings.TrimSpace(rest), 10, 8); err == nil {
			return int(n), true
		}
	}
	if name == "title" {
		return 1, true
	}
	return outlineLevel(child(style, "pPr"))
}

func (s *styles) runToggles(id string) (toggles, error) {
	p, err := s.resolve(id)
	return p.toggles, err
}

// headingLevel resolves a paragraph style's heading level through basedOn.
// found reports the nearest specification.
func (s *styles) headingLevel(id string) (level int, found bool, err error) {
	p, err := s.resolve(id)
	return p.heading, p.hasHeading, err
}

// outlineLevel reads w:outlineLvl as a 1-based heading level, 0 for the
// explicit "no outline level" value 9.
func outlineLevel(ppr *opc.Element) (int, bool) {
	v, ok := val(ppr, "outlineLvl")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		return 0, false
	}
	if n < 9 {
		return int(n) + 1, true
	}
	return 0, true
}

func (s *styles) blockStyle(id string) (blockKind, error) {
	p, err := s.resolve(id)
	return p.kind, err
}

// styleNumID is the numId a paragraph style contributes through basedOn. An
// ilvl in a style's numPr is ignored (ECMA-376 §17.3.1.19); the level comes
// from the abstract levels' pStyle bindings.
func (s *styles) styleNumID(id string) (numID int, found bool, err error) {
	p, err := s.resolve(id)
	return p.numID, p.hasNumID, err
}

// styleNumberingLevel is the level of the nearest style in the basedOn
// chain that one of the instance's levels binds through pStyle. Results are
// memoized per instance and style, so each chain link is visited once per
// instance.
func (s *styles) styleNumberingLevel(id string, inst *instance) (level int, found bool, err error) {
	if _, err := s.resolve(id); err != nil {
		return 0, false, err
	}
	var chain []string
	var result numberingLevel
	for cur := id; ; {
		if r, ok := s.levels[numberingLevelKey{inst: inst, id: cur}]; ok {
			result = r
			break
		}
		def, ok := s.defs[cur]
		if !ok {
			break
		}
		chain = append(chain, cur)
		if level, ok := inst.styleLevel(cur); ok {
			result = numberingLevel{level: level, found: true}
			break
		}
		cur = def.parent
	}
	for _, member := range chain {
		s.levels[numberingLevelKey{inst: inst, id: member}] = result
	}
	return result.level, result.found, nil
}

// directNumID is the numId a numbering style's own numPr references, without
// inheritance (the numStyleLink contract).
func (s *styles) directNumID(id string) (int, bool) {
	def, ok := s.defs[id]
	if !ok {
		return 0, false
	}
	return parseNumID(child(child(def.elem, "pPr"), "numPr"))
}

func parseNumID(numPr *opc.Element) (int, bool) {
	v, ok := val(numPr, "numId")
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseUint(v, 10, 31)
	return int(n), err == nil
}

// applyDirect overlays direct w:rPr formatting, which is absolute on/off.
func applyDirect(rpr *opc.Element, base model.Style) model.Style {
	if on, ok := onOff(rpr, "b"); ok {
		base.Bold = on
	}
	if on, ok := onOff(rpr, "i"); ok {
		base.Italic = on
	}
	s, sok := onOff(rpr, "strike")
	d, dok := onOff(rpr, "dstrike")
	if sok || dok {
		base.Strike = s || d
	}
	return base
}

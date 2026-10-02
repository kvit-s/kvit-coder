package pptx

import (
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

const levels = 9

type toggle int8

const (
	inherit toggle = iota
	off
	on
)

func toggleOf(set, value bool) toggle {
	switch {
	case !set:
		return inherit
	case value:
		return on
	}
	return off
}

func (t toggle) over(base toggle) toggle {
	if t == inherit {
		return base
	}
	return t
}

func (t toggle) apply(base bool) bool {
	if t == inherit {
		return base
	}
	return t == on
}

type styleDelta struct {
	bold, italic, strike toggle
}

func (d styleDelta) merge(child styleDelta) styleDelta {
	return styleDelta{
		bold:   child.bold.over(d.bold),
		italic: child.italic.over(d.italic),
		strike: child.strike.over(d.strike),
	}
}

func (d styleDelta) apply(base model.Style) model.Style {
	base.Bold = d.bold.apply(base.Bold)
	base.Italic = d.italic.apply(base.Italic)
	base.Strike = d.strike.apply(base.Strike)
	return base
}

func runDelta(rpr *opc.Element) styleDelta {
	onOff := func(name string) toggle {
		v, ok := rpr.Attr(opc.NSDrawingML, name)
		return toggleOf(ok, v == "1" || v == "true" || v == "on")
	}
	strike, ok := rpr.Attr(opc.NSDrawingML, "strike")
	return styleDelta{
		bold:   onOff("b"),
		italic: onOff("i"),
		strike: toggleOf(ok, strike == "sngStrike" || strike == "dblStrike"),
	}
}

type bulletKind int

const (
	bulletInherit bulletKind = iota
	bulletNone
	bulletChar
	bulletAutoNum
)

type numWrap int

const (
	wrapPeriod numWrap = iota
	wrapParenR
	wrapParenBoth
	wrapPlain
)

type bullet struct {
	kind   bulletKind
	marker model.MarkerKind
	start  int
	wrap   numWrap
}

// label is the literal marker for ordinal n, "" when the renderer's default
// "n." label is already faithful.
func (b bullet) label(n int) string {
	ordinal := b.marker.Ordinal(n)
	switch b.wrap {
	case wrapParenR:
		return ordinal + ")"
	case wrapParenBoth:
		return "(" + ordinal + ")"
	case wrapPlain:
		return ordinal
	}
	return ""
}

type textProps struct {
	delta  styleDelta
	bullet bullet
}

func (p textProps) merge(over textProps) textProps {
	out := textProps{delta: p.delta.merge(over.delta), bullet: over.bullet}
	if over.bullet.kind == bulletInherit {
		out.bullet = p.bullet
	}
	return out
}

type levelStyle [levels]textProps

func (s *levelStyle) level(lvl int) textProps {
	return s[min(lvl, levels-1)]
}

// parseLevelStyles reads the lvl1pPr..lvl9pPr children of a lstStyle-shaped
// element; nil yields an all-inherit style.
func parseLevelStyles(e *opc.Element) levelStyle {
	var out levelStyle
	if e == nil {
		return out
	}
	for i := range out {
		if ppr := e.Child(opc.NSDrawingML, "lvl"+strconv.Itoa(i+1)+"pPr"); ppr != nil {
			out[i] = paragraphProps(ppr)
		}
	}
	return out
}

func paragraphProps(ppr *opc.Element) textProps {
	var props textProps
	if rpr := ppr.Child(opc.NSDrawingML, "defRPr"); rpr != nil {
		props.delta = runDelta(rpr)
	}
	if ppr.Child(opc.NSDrawingML, "buNone") != nil {
		props.bullet.kind = bulletNone
	} else if auto := ppr.Child(opc.NSDrawingML, "buAutoNum"); auto != nil {
		props.bullet = autoNum(auto)
	} else if ppr.Child(opc.NSDrawingML, "buChar") != nil {
		props.bullet.kind = bulletChar
	}
	return props
}

func autoNum(auto *opc.Element) bullet {
	scheme, _ := auto.Attr(opc.NSDrawingML, "type")
	b := bullet{kind: bulletAutoNum, marker: model.Decimal, start: 1}
	switch {
	case strings.HasPrefix(scheme, "alphaLc"):
		b.marker = model.LowerAlpha
	case strings.HasPrefix(scheme, "alphaUc"):
		b.marker = model.UpperAlpha
	case strings.HasPrefix(scheme, "romanLc"):
		b.marker = model.LowerRoman
	case strings.HasPrefix(scheme, "romanUc"):
		b.marker = model.UpperRoman
	}
	switch {
	case strings.HasSuffix(scheme, "ParenBoth"):
		b.wrap = wrapParenBoth
	case strings.HasSuffix(scheme, "ParenR"):
		b.wrap = wrapParenR
	case strings.HasSuffix(scheme, "Plain"):
		b.wrap = wrapPlain
	}
	// ST_TextBulletStartAtNum is 1..32767; untrusted values are clamped so
	// counters cannot overflow.
	if v, ok := auto.Attr(opc.NSDrawingML, "startAt"); ok {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil {
			b.start = int(min(max(n, 1), 32767))
		}
	}
	return b
}

type placeholder struct {
	phType string
	idx    string
	styles levelStyle
}

// placeholderOf reads the p:ph below a shape, nil when it is not a
// placeholder; an absent type means body.
func placeholderOf(sp *opc.Element) *placeholder {
	ph := sp.FirstDescendant(nsP, "ph")
	if ph == nil {
		return nil
	}
	p := &placeholder{phType: "body"}
	if t, ok := ph.Attr(nsP, "type"); ok {
		p.phType = t
	}
	p.idx, _ = ph.Attr(nsP, "idx")
	return p
}

func collectPlaceholders(spTree *opc.Element) []placeholder {
	var out []placeholder
	for sp := range spTree.Descendants(nsP, "sp") {
		p := placeholderOf(sp)
		if p == nil {
			continue
		}
		if tx := sp.Child(nsP, "txBody"); tx != nil {
			p.styles = parseLevelStyles(tx.Child(opc.NSDrawingML, "lstStyle"))
		}
		out = append(out, *p)
	}
	return out
}

func isTitle(phType string) bool {
	return phType == "title" || phType == "ctrTitle"
}

// matchPlaceholder finds the placeholder a shape inherits from: by idx
// first, then by type, with the title variants unified.
func matchPlaceholder(candidates []placeholder, ph *placeholder) *placeholder {
	if ph.idx != "" {
		for i := range candidates {
			if candidates[i].idx == ph.idx {
				return &candidates[i]
			}
		}
	}
	title := isTitle(ph.phType)
	for i := range candidates {
		if isTitle(candidates[i].phType) == title && (title || candidates[i].phType == ph.phType) {
			return &candidates[i]
		}
	}
	for i := range candidates {
		if isTitle(candidates[i].phType) == title {
			return &candidates[i]
		}
	}
	return nil
}

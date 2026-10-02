package pptx

import (
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/formats/omml"
	"github.com/kvit-s/kvit-coder/internal/office/formats/ooxml"
	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

// supportedNS are the namespaces this frontend understands; mc:Choice
// branches requiring anything else fall back to mc:Fallback.
var supportedNS = []string{nsP, opc.NSDrawingML, opc.NSRelationships, opc.NSMarkupCompatibility, nsA14}

// slide converts the content of one slide or notes part. Notes parts carry
// no layout or master.
type slide struct {
	c      *converter
	rels   opc.Relationships
	part   string
	layout *layout
	master *master
}

func (s *slide) shapes(parent *opc.Element, blocks []model.Block) ([]model.Block, error) {
	var err error
	for child := range parent.Elements() {
		if child.Is(opc.NSMarkupCompatibility, "AlternateContent") {
			if branch := alternateBranch(child); branch != nil {
				if blocks, err = s.shapes(branch, blocks); err != nil {
					return nil, err
				}
			}
			continue
		}
		if child.Space != nsP {
			continue
		}
		switch child.Local {
		case "sp", "cxnSp":
			blocks = s.shape(child, blocks)
		case "grpSp":
			blocks, err = s.shapes(child, blocks)
		case "graphicFrame":
			blocks, err = s.graphicFrame(child, blocks)
		case "pic":
			blocks, err = s.picture(child, blocks)
		}
		if err != nil {
			return nil, err
		}
	}
	return blocks, nil
}

func alternateBranch(alt *opc.Element) *opc.Element {
	for choice := range alt.Children(opc.NSMarkupCompatibility, "Choice") {
		requires, _ := choice.Attr(opc.NSMarkupCompatibility, "Requires")
		supported := true
		for _, uri := range strings.Fields(requires) {
			if !slices.Contains(supportedNS, uri) {
				supported = false
				break
			}
		}
		if supported {
			return choice
		}
	}
	return alt.Child(opc.NSMarkupCompatibility, "Fallback")
}

func (s *slide) shape(sp *opc.Element, blocks []model.Block) []model.Block {
	ph := placeholderOf(sp)
	if ph != nil && (ph.phType == "sldNum" || ph.phType == "dt" || ph.phType == "ftr") {
		return blocks
	}
	tx := sp.Child(nsP, "txBody")
	switch {
	case tx == nil:
		return blocks
	case ph != nil && isTitle(ph.phType):
		return s.titleHeading(tx, ph, blocks)
	}
	return s.textBody(tx, ph, blocks)
}

// baseProps folds the cascade for one paragraph level, outermost first:
// presentation defaults, master text styles, master placeholder, layout
// placeholder, shape list style.
func (s *slide) baseProps(ph *placeholder, shapeStyles *levelStyle, lvl int) textProps {
	props := s.c.defaultText.level(lvl)
	if s.master != nil {
		classStyle := &s.master.other
		if ph != nil {
			classStyle = &s.master.body
			if isTitle(ph.phType) {
				classStyle = &s.master.title
			}
		}
		props = props.merge(classStyle.level(lvl))
		if ph != nil {
			if hit := matchPlaceholder(s.master.placeholders, ph); hit != nil {
				props = props.merge(hit.styles.level(lvl))
			}
		}
	}
	if s.layout != nil && ph != nil {
		if hit := matchPlaceholder(s.layout.placeholders, ph); hit != nil {
			props = props.merge(hit.styles.level(lvl))
		}
	}
	return props.merge(shapeStyles.level(lvl))
}

func (s *slide) paragraphProps(p *opc.Element, ph *placeholder, shapeStyles *levelStyle) (textProps, int) {
	ppr := p.Child(opc.NSDrawingML, "pPr")
	lvl := 0
	if ppr != nil {
		if v, ok := ppr.Attr(opc.NSDrawingML, "lvl"); ok {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				lvl = n
			}
		}
	}
	props := s.baseProps(ph, shapeStyles, lvl)
	if ppr != nil {
		props = props.merge(paragraphProps(ppr))
	}
	return props, lvl
}

// titleHeading collapses a title placeholder's paragraphs into one heading
// that keeps its shape-order position. The title style's own emphasis is
// dropped from the runs, so only what they add beyond it is marked.
func (s *slide) titleHeading(tx *opc.Element, ph *placeholder, blocks []model.Block) []model.Block {
	shapeStyles := parseLevelStyles(tx.Child(opc.NSDrawingML, "lstStyle"))
	var content []model.Inline
	for p := range tx.Children(opc.NSDrawingML, "p") {
		props, _ := s.paragraphProps(p, ph, &shapeStyles)
		base := props.delta.apply(model.Style{})
		para := s.inlines(p, base)
		if model.IsEmpty(para) {
			continue
		}
		rebaseEmphasis(para, base)
		if len(content) > 0 {
			content = append(content, model.LineBreak{})
		}
		content = append(content, para...)
	}
	if model.IsEmpty(content) {
		return blocks
	}
	return append(blocks, model.Heading{Level: 2, Anchor: model.PlainText(content), Content: content})
}

func rebaseEmphasis(inlines []model.Inline, base model.Style) {
	for i, in := range inlines {
		switch in := in.(type) {
		case model.Text:
			in.Style.Bold = in.Style.Bold && !base.Bold
			in.Style.Italic = in.Style.Italic && !base.Italic
			in.Style.Strike = in.Style.Strike && !base.Strike
			inlines[i] = in
		case model.Link:
			rebaseEmphasis(in.Content, base)
		}
	}
}

func (s *slide) textBody(tx *opc.Element, ph *placeholder, blocks []model.Block) []model.Block {
	shapeStyles := parseLevelStyles(tx.Child(opc.NSDrawingML, "lstStyle"))
	s.c.instances++
	instance := s.c.instances
	var (
		counters [levels]int
		started  [levels]bool
		run      []listEntry
	)
	flush := func() {
		blocks = append(blocks, buildLists(run)...)
		run = nil
	}
	for p := range tx.Children(opc.NSDrawingML, "p") {
		props, lvl := s.paragraphProps(p, ph, &shapeStyles)
		inlines := s.inlines(p, props.delta.apply(model.Style{}))
		if model.IsEmpty(inlines) {
			flush()
			continue
		}
		entry := listEntry{level: lvl, blocks: []model.Block{model.Paragraph(inlines)}}
		switch b := props.bullet; b.kind {
		case bulletAutoNum:
			at := min(lvl, levels-1)
			number := b.start
			if started[at] {
				number = counters[at] + 1
			}
			started[at], counters[at] = true, number
			for deeper := at + 1; deeper < levels; deeper++ {
				started[deeper] = false
			}
			entry.key = listKey{instance: instance, marker: b.marker}
			entry.number, entry.label = number, b.label(number)
			run = append(run, entry)
		case bulletChar:
			entry.key = listKey{instance: instance, marker: model.Bullet}
			run = append(run, entry)
		default:
			flush()
			if lines := mathLines(inlines); lines != nil {
				blocks = append(blocks, lines...)
			} else {
				blocks = append(blocks, model.Paragraph(inlines))
			}
		}
	}
	flush()
	return blocks
}

// mathLines turns a paragraph holding only formulas, line breaks and blank
// text into display math, one block per formula; nil for anything else.
func mathLines(inlines []model.Inline) []model.Block {
	var lines []model.Block
	for _, in := range inlines {
		switch in := in.(type) {
		case model.Math:
			lines = append(lines, model.MathBlock(in))
		case model.LineBreak:
		case model.Text:
			if strings.TrimSpace(in.Text) != "" {
				return nil
			}
		default:
			return nil
		}
	}
	return lines
}

func (s *slide) inlines(p *opc.Element, base model.Style) []model.Inline {
	var out []model.Inline
	for child := range p.Elements() {
		if child.Is(nsA14, "m") {
			para := child.Child(opc.NSMath, "oMathPara")
			if para == nil {
				para = child
			}
			for i, line := range omml.Blocks(para) {
				if i > 0 {
					out = append(out, model.LineBreak{})
				}
				out = append(out, model.Math(line.(model.MathBlock)))
			}
			continue
		}
		if child.Space != opc.NSDrawingML {
			continue
		}
		switch child.Local {
		case "r", "fld":
			var text string
			if t := child.Child(opc.NSDrawingML, "t"); t != nil {
				text = ooxml.CleanText(t.Text())
			}
			if text == "" {
				continue
			}
			style := base
			rpr := child.Child(opc.NSDrawingML, "rPr")
			if rpr != nil {
				style = runDelta(rpr).apply(base)
			}
			var inline model.Inline = model.Text{Text: text, Style: style}
			if target, ok := s.hyperlink(rpr); ok {
				inline = model.Link{Content: []model.Inline{inline}, Target: target}
			}
			out = append(out, inline)
		case "br":
			out = append(out, model.LineBreak{})
		}
	}
	return out
}

func (s *slide) hyperlink(rpr *opc.Element) (model.Target, bool) {
	if rpr == nil {
		return model.Target{}, false
	}
	click := rpr.Child(opc.NSDrawingML, "hlinkClick")
	if click == nil {
		return model.Target{}, false
	}
	id, ok := click.QualifiedAttr(opc.NSRelationships, "id")
	if !ok {
		return model.Target{}, false
	}
	rel, ok := s.rels.ByID(id)
	if !ok {
		return model.Target{}, false
	}
	return s.linkTarget(rel), true
}

// linkTarget maps slide-to-slide relationships onto the target slide's
// anchor; other internal targets stay package-relative.
func (s *slide) linkTarget(rel opc.Relationship) model.Target {
	if !rel.External {
		if t, err := opc.Resolve(s.part, rel.Target); err == nil {
			if anchor, ok := s.c.slideAnchors[t.Path]; ok {
				return model.Target{Kind: model.TargetAnchor, Ref: anchor}
			}
		}
		return model.Target{Kind: model.TargetRelative, Ref: rel.Target}
	}
	if anchor, ok := strings.CutPrefix(rel.Target, "#"); ok {
		return model.Target{Kind: model.TargetAnchor, Ref: anchor}
	}
	if ooxml.IsAbsoluteURI(rel.Target) {
		return model.Target{Kind: model.TargetExternal, Ref: rel.Target}
	}
	return model.Target{Kind: model.TargetRelative, Ref: rel.Target}
}

// relPart loads the internal part a relationship id targets; unusable
// targets degrade to not found with a warning, limit errors propagate.
func (s *slide) relPart(id string) (string, []byte, error) {
	path, ok := s.rels.PartPath(id)
	if !ok {
		return "", nil, nil
	}
	data, err := s.c.pkg.OptionalPart(path)
	if err != nil {
		return "", nil, err
	}
	if data == nil {
		model.Log.Warn("relationship target is missing", "part", path)
		return "", nil, nil
	}
	return path, data, nil
}

func (s *slide) picture(pic *opc.Element, blocks []model.Block) ([]model.Block, error) {
	var descr string
	if nv := pic.FirstDescendant(nsP, "cNvPr"); nv != nil {
		v, _ := nv.Attr(nsP, "descr")
		descr = ooxml.CleanText(v)
	}
	source := model.ImageSource{Kind: model.SourceUnavailable}
	if blip := pic.FirstDescendant(opc.NSDrawingML, "blip"); blip != nil {
		id, ok := blip.QualifiedAttr(opc.NSRelationships, "embed")
		if !ok {
			id, ok = blip.QualifiedAttr(opc.NSRelationships, "link")
		}
		if ok {
			var err error
			if source, err = s.imageSource(id); err != nil {
				return nil, err
			}
		}
	}
	if source.Kind == model.SourceUnavailable && strings.TrimSpace(descr) == "" {
		return blocks, nil
	}
	return append(blocks, model.Paragraph{model.Image{Alt: descr, Source: source}}), nil
}

// imageSource resolves an image relationship: external targets carry their
// URL, internal ones are retained as assets.
func (s *slide) imageSource(id string) (model.ImageSource, error) {
	unavailable := model.ImageSource{Kind: model.SourceUnavailable}
	rel, ok := s.rels.ByID(id)
	if !ok {
		return unavailable, nil
	}
	if rel.External {
		if rel.Target == "" {
			return unavailable, nil
		}
		return model.ImageSource{Kind: model.SourceExternal, URL: rel.Target}, nil
	}
	part, data, err := s.relPart(id)
	if err != nil || data == nil {
		return unavailable, err
	}
	asset, err := s.c.asset(ooxml.MediaType(part), part, data)
	if err != nil {
		return unavailable, err
	}
	return model.ImageSource{Kind: model.SourceAsset, Asset: asset}, nil
}

func (s *slide) graphicFrame(frame *opc.Element, blocks []model.Block) ([]model.Block, error) {
	if tbl := frame.FirstDescendant(opc.NSDrawingML, "tbl"); tbl != nil {
		return s.table(tbl, blocks)
	}
	if ole := frame.FirstDescendant(nsP, "oleObj"); ole != nil {
		return s.oleObject(ole, blocks)
	}
	if chart := frame.FirstDescendant(opc.NSChart, "chart"); chart != nil {
		if id, ok := chart.QualifiedAttr(opc.NSRelationships, "id"); ok {
			root, err := s.relXML(id)
			if err != nil {
				return nil, err
			}
			if root != nil {
				return append(blocks, ooxml.ChartBlocks(root)...), nil
			}
		}
	}
	if ids := frame.FirstDescendant(opc.NSDiagram, "relIds"); ids != nil {
		if id, ok := ids.QualifiedAttr(opc.NSRelationships, "dm"); ok {
			root, err := s.relXML(id)
			if err != nil {
				return nil, err
			}
			if root != nil {
				return append(blocks, ooxml.DiagramBlocks(root)...), nil
			}
		}
	}
	return blocks, nil
}

func (s *slide) relXML(id string) (*opc.Element, error) {
	part, data, err := s.relPart(id)
	if err != nil || data == nil {
		return nil, err
	}
	root, err := opc.ParseXML(data)
	if err != nil {
		var limit *model.LimitError
		if errors.As(err, &limit) {
			return nil, err
		}
		model.Log.Warn("skipping corrupt part", "part", part, "err", err)
		return nil, nil
	}
	return root, nil
}

func (s *slide) oleObject(ole *opc.Element, blocks []model.Block) ([]model.Block, error) {
	name, _ := ole.Attr(nsP, "name")
	alt := strings.TrimSpace(name)
	if alt == "" {
		progID, ok := ole.Attr(nsP, "progId")
		if !ok {
			progID = "object"
		}
		alt = "Embedded object: " + progID
	}
	source := model.ImageSource{Kind: model.SourceUnavailable}
	if id, ok := ole.QualifiedAttr(opc.NSRelationships, "id"); ok {
		part, data, err := s.relPart(id)
		if err != nil {
			return nil, err
		}
		if data != nil {
			asset, err := s.c.asset("application/vnd.ms-ole-object", part, data)
			if err != nil {
				return nil, err
			}
			source = model.ImageSource{Kind: model.SourceAsset, Asset: asset}
		}
	}
	return append(blocks, model.Paragraph{model.Image{Alt: alt, Source: source}}), nil
}

// table converts a DrawingML table: origins carry gridSpan/rowSpan and
// hMerge/vMerge continuation cells consume covered positions. Column spans
// clamp to the declared grid (or the row's cell count when wider), row spans
// to the remaining rows, and every span area is charged against the
// presentation-wide expansion budget.
func (s *slide) table(tbl *opc.Element, blocks []model.Block) ([]model.Block, error) {
	declared := 0
	if pr := tbl.Child(opc.NSDrawingML, "tblPr"); pr != nil && isTrue(pr, "firstRow") {
		declared = 1
	}
	gridCols := 0
	if grid := tbl.Child(opc.NSDrawingML, "tblGrid"); grid != nil {
		for range grid.Children(opc.NSDrawingML, "gridCol") {
			gridCols++
		}
	}
	rows := slices.Collect(tbl.Children(opc.NSDrawingML, "tr"))
	var b model.GridBuilder
	for r, tr := range rows {
		b.NextRow()
		cells := slices.Collect(tr.Children(opc.NSDrawingML, "tc"))
		maxCols := max(gridCols, len(cells))
		for _, tc := range cells {
			if isTrue(tc, "hMerge") || isTrue(tc, "vMerge") {
				b.Covered()
				continue
			}
			cell := model.Cell{
				ColSpan: min(span(tc, "gridSpan"), maxCols),
				RowSpan: min(span(tc, "rowSpan"), len(rows)-r),
			}
			s.c.expansion += uint64(cell.ColSpan)*uint64(cell.RowSpan) - 1
			if s.c.expansion > model.MaxExpansion {
				return nil, &model.LimitError{Limit: "max_expansion", Detail: "table span expansion exceeds the presentation budget"}
			}
			if tx := tc.Child(opc.NSDrawingML, "txBody"); tx != nil {
				cell.Blocks = s.textBody(tx, nil, nil)
			}
			if err := b.Place(cell); err != nil {
				return nil, err
			}
		}
	}
	t := b.Finish(model.DataTable)
	if len(t.Grid()) == 0 {
		return blocks, nil
	}
	t.HeaderRows = model.ResolveHeaderRows(t, declared)
	return append(blocks, t), nil
}

func isTrue(e *opc.Element, name string) bool {
	v, _ := e.Attr(opc.NSDrawingML, name)
	return v == "1" || v == "true"
}

func span(tc *opc.Element, name string) int {
	v, _ := tc.Attr(opc.NSDrawingML, name)
	n, err := strconv.ParseUint(v, 10, 32)
	if err != nil {
		return 1
	}
	return max(int(n), 1)
}

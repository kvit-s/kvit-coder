package docx

import (
	"errors"
	"slices"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/formats/omml"
	"github.com/kvit-s/kvit-coder/internal/office/formats/ooxml"
	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

// supportedNS are the vocabularies this frontend reads; mc:Choice branches
// requiring anything else fall back to mc:Fallback.
var supportedNS = map[string]bool{
	nsW: true, nsA: true, nsPic: true, nsWP: true, nsMC: true, nsChart: true,
	nsDgm: true, nsVML: true, nsOVML: true, nsWPS: true, nsWPG: true,
}

func alternateBranch(alt *opc.Element) *opc.Element {
	for choice := range alt.Children(nsMC, "Choice") {
		requires, ok := choice.Attr(nsMC, "Requires")
		supported := true
		if ok {
			for _, uri := range strings.Fields(requires) {
				supported = supported && supportedNS[uri]
			}
		}
		if supported {
			return choice
		}
	}
	return alt.Child(nsMC, "Fallback")
}

// piece is paragraph content in source order: inline runs, or blocks (text
// boxes, charts) attached at their anchor position.
type piece struct {
	inlines []model.Inline
	blocks  []model.Block
}

type paraKind int

const (
	plainPara paraKind = iota
	headingPara
	listPara
	styledPara
)

type paragraph struct {
	kind paraKind
	// heading: level, the visible number of a numbered heading, and the
	// style's own emphasis to subtract from its runs.
	level int
	label string
	base  model.Style
	entry listEntry
	block blockKind
}

// blockBuilder collects a container's blocks, holding the open list run or
// styled container a following paragraph may extend; only one is ever open.
type blockBuilder struct {
	c      *converter
	blocks []model.Block
	list   []listEntry
	styled styledRun
}

func (c *converter) parseBlocks(parent *opc.Element) ([]model.Block, error) {
	b := &blockBuilder{c: c}
	if err := b.collect(parent); err != nil {
		return nil, err
	}
	b.flush()
	return b.blocks, nil
}

func (b *blockBuilder) flushList() {
	b.blocks = append(b.blocks, buildLists(b.list)...)
	b.list = nil
}

func (b *blockBuilder) flush() {
	b.styled.flush(&b.blocks)
	b.flushList()
}

func (b *blockBuilder) collect(parent *opc.Element) error {
	for e := range parent.Elements() {
		switch {
		case e.Is(nsMC, "AlternateContent"):
			if branch := alternateBranch(e); branch != nil {
				if err := b.collect(branch); err != nil {
					return err
				}
			}
			continue
		case e.Is(nsM, "oMathPara") || e.Is(nsM, "oMath"):
			b.flush()
			b.blocks = append(b.blocks, omml.Blocks(e)...)
			continue
		case e.Space != nsW:
			continue
		}
		switch e.Local {
		case "p":
			para, pieces, err := b.c.parseParagraph(e)
			if err != nil {
				return err
			}
			b.emit(para, pieces)
		case "tbl":
			b.flush()
			table, err := b.c.parseTable(e)
			if err != nil {
				return err
			}
			b.blocks = append(b.blocks, table...)
		case "sdt", "customXml":
			content := e
			if e.Local == "sdt" {
				content = child(e, "sdtContent")
			}
			if content == nil {
				continue
			}
			if err := b.collect(content); err != nil {
				return err
			}
		}
	}
	return nil
}

func (b *blockBuilder) emit(para paragraph, pieces []piece) {
	switch para.kind {
	case listPara:
		b.styled.flush(&b.blocks)
		para.entry.blocks = piecesToBlocks(pieces)
		b.list = append(b.list, para.entry)
	case styledPara:
		b.flushList()
		for _, p := range pieces {
			if p.blocks != nil {
				b.styled.flush(&b.blocks)
				b.blocks = append(b.blocks, p.blocks...)
			} else {
				b.styled.push(para.block, p.inlines, &b.blocks)
			}
		}
	case headingPara:
		b.flush()
		label, emitted := para.label, false
		for _, p := range pieces {
			switch {
			case p.blocks != nil:
				b.blocks = append(b.blocks, p.blocks...)
			case model.IsEmpty(p.inlines):
			case !emitted:
				content := rebaseEmphasis(p.inlines, para.base)
				if label != "" {
					content = append([]model.Inline{model.Text{Text: label}}, content...)
				}
				b.blocks = append(b.blocks, model.Heading{Level: para.level, Content: content})
				emitted = true
			default:
				b.blocks = append(b.blocks, model.Paragraph(rebaseEmphasis(p.inlines, para.base)))
			}
		}
	default:
		b.flush()
		b.blocks = append(b.blocks, piecesToBlocks(pieces)...)
	}
}

// rebaseEmphasis drops the emphasis a heading style already carries, so
// runs keep only what they add beyond the style's own typography.
func rebaseEmphasis(inlines []model.Inline, base model.Style) []model.Inline {
	if base == (model.Style{}) {
		return inlines
	}
	out := make([]model.Inline, len(inlines))
	for i, in := range inlines {
		switch in := in.(type) {
		case model.Text:
			in.Style.Bold = in.Style.Bold && !base.Bold
			in.Style.Italic = in.Style.Italic && !base.Italic
			in.Style.Strike = in.Style.Strike && !base.Strike
			out[i] = in
		case model.Link:
			in.Content = rebaseEmphasis(in.Content, base)
			out[i] = in
		default:
			out[i] = in
		}
	}
	return out
}

// piecesToBlocks keeps visually empty inline runs as paragraphs: a
// bookmark-only paragraph carries the anchor a link resolves against.
func piecesToBlocks(pieces []piece) []model.Block {
	var blocks []model.Block
	for _, p := range pieces {
		if p.blocks != nil {
			blocks = append(blocks, p.blocks...)
		} else {
			blocks = append(blocks, model.Paragraph(p.inlines))
		}
	}
	return blocks
}

func (c *converter) parseParagraph(p *opc.Element) (paragraph, []piece, error) {
	ppr := child(p, "pPr")
	styleID, hasStyle := val(ppr, "pStyle")

	level, hasHeading := outlineLevel(ppr)
	if !hasHeading && hasStyle {
		var err error
		if level, hasHeading, err = c.styles.headingLevel(styleID); err != nil {
			return paragraph{}, nil, err
		}
	}
	block := noBlock
	var t toggles
	if hasStyle {
		var err error
		if block, err = c.styles.blockStyle(styleID); err != nil {
			return paragraph{}, nil, err
		}
		if t, err = c.styles.runToggles(styleID); err != nil {
			return paragraph{}, nil, err
		}
	}
	entry, numbered, err := c.resolveNumbering(ppr, styleID, hasStyle)
	if err != nil {
		return paragraph{}, nil, err
	}
	para := paragraph{base: t.over(c.styles.docDefaults), block: block}
	switch {
	case hasHeading && level > 0:
		para.kind, para.level = headingPara, level
		if numbered && entry.key.marker.Ordered() {
			label := entry.label
			if label == "" {
				label = entry.key.marker.Label(entry.number)
			}
			para.label = label + " "
		}
	case numbered:
		para.kind, para.entry = listPara, entry
	case block != noBlock:
		para.kind = styledPara
	}

	w := &inlineWalker{c: c, base: para.base}
	if err := w.walk(p); err != nil {
		return paragraph{}, nil, err
	}
	return para, w.finish(), nil
}

// resolveNumbering merges the direct numPr with the style's property by
// property (ECMA-376): a missing numId or ilvl inherits, numId 0 suppresses,
// and a numFmt of none suppresses. Ordered levels advance their counters.
func (c *converter) resolveNumbering(ppr *opc.Element, styleID string, hasStyle bool) (listEntry, bool, error) {
	numPr := child(ppr, "numPr")
	numID, hasNumID := parseNumID(numPr)
	if !hasNumID && hasStyle {
		var err error
		if numID, hasNumID, err = c.styles.styleNumID(styleID); err != nil {
			return listEntry{}, false, err
		}
	}
	if !hasNumID || numID == 0 {
		return listEntry{}, false, nil
	}
	inst, ok := c.numbering[numID]
	if !ok {
		model.Log.Debug("paragraph references an undefined numbering instance", "numId", numID)
		return listEntry{}, false, nil
	}
	ilvl, hasLevel := 0, false
	if v, ok := val(numPr, "ilvl"); ok {
		ilvl, hasLevel = levelIndexValue(v)
	}
	if !hasLevel && hasStyle {
		level, found, err := c.styles.styleNumberingLevel(styleID, inst)
		if err != nil {
			return listEntry{}, false, err
		}
		if found {
			ilvl = level
		}
	}
	ilvl = min(ilvl, numLevels-1)
	def := inst.levels[ilvl]
	if def.suppressed {
		return listEntry{}, false, nil
	}
	entry := listEntry{level: ilvl, key: listKey{instance: numID, marker: def.marker}}
	if def.marker.Ordered() {
		entry.number, entry.label = c.nextNumber(numID, ilvl, inst)
	}
	return entry, true, nil
}

type inlineWalker struct {
	c       *converter
	base    model.Style
	pieces  []piece
	current []model.Inline
	fields  []*fieldFrame
}

func (w *inlineWalker) push(in model.Inline) {
	if n := len(w.fields); n > 0 {
		if f := w.fields[n-1]; f.inResult {
			f.inlines = append(f.inlines, in)
		}
		return
	}
	w.current = append(w.current, in)
}

func (w *inlineWalker) pushBlocks(blocks []model.Block) {
	if len(blocks) == 0 {
		return
	}
	if len(w.current) > 0 {
		w.pieces = append(w.pieces, piece{inlines: w.current})
		w.current = nil
	}
	w.pieces = append(w.pieces, piece{blocks: blocks})
}

func (w *inlineWalker) finish() []piece {
	for len(w.fields) > 0 {
		frame := w.fields[len(w.fields)-1]
		w.fields = w.fields[:len(w.fields)-1]
		for _, in := range frame.inlines {
			w.push(in)
		}
	}
	if len(w.current) > 0 {
		w.pieces = append(w.pieces, piece{inlines: w.current})
	}
	return w.pieces
}

// sub walks e in a nested walker and splits its content into inlines and
// attachments.
func (w *inlineWalker) sub(e *opc.Element) ([]model.Inline, []model.Block, error) {
	inner := &inlineWalker{c: w.c, base: w.base}
	if err := inner.walk(e); err != nil {
		return nil, nil, err
	}
	var inlines []model.Inline
	var blocks []model.Block
	for _, p := range inner.finish() {
		inlines = append(inlines, p.inlines...)
		blocks = append(blocks, p.blocks...)
	}
	return inlines, blocks, nil
}

func (w *inlineWalker) walk(e *opc.Element) error {
	for ch := range e.Elements() {
		switch {
		case ch.Is(nsMC, "AlternateContent"):
			if branch := alternateBranch(ch); branch != nil {
				if err := w.walk(branch); err != nil {
					return err
				}
			}
			continue
		case ch.Is(nsM, "oMathPara"):
			w.pushBlocks(omml.Blocks(ch))
			continue
		case ch.Is(nsM, "oMath"):
			if math := omml.Math(ch); math != "" {
				w.push(math)
			}
			continue
		case ch.Space != nsW:
			continue
		}
		var err error
		switch ch.Local {
		case "r":
			err = w.walkRun(ch)
		case "hyperlink":
			err = w.hyperlink(ch)
		case "fldSimple":
			err = w.simpleField(ch)
		case "bookmarkStart":
			if name, ok := ch.Attr(nsW, "name"); ok && name != "_GoBack" {
				w.push(model.Anchor(name))
			}
		case "sdt":
			if content := child(ch, "sdtContent"); content != nil {
				err = w.walk(content)
			}
		case "smartTag", "ins", "bdo", "dir", "moveTo", "customXml":
			err = w.walk(ch)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *inlineWalker) simpleField(field *opc.Element) error {
	instr, _ := field.Attr(nsW, "instr")
	content, blocks, err := w.sub(field)
	if err != nil {
		return err
	}
	for _, in := range fieldResult(instr, content) {
		w.push(in)
	}
	w.pushBlocks(blocks)
	return nil
}

func (w *inlineWalker) hyperlink(link *opc.Element) error {
	var target model.Target
	resolved := false
	if id, ok := link.QualifiedAttr(nsR, "id"); ok {
		if rel, ok := w.c.rels.ByID(id); ok {
			target, resolved = model.Target{Kind: model.TargetRelative, Ref: rel.Target}, true
			if rel.External {
				target = classifyTarget(rel.Target)
			}
		}
	}
	if !resolved {
		if anchor, ok := link.Attr(nsW, "anchor"); ok {
			target, resolved = model.Target{Kind: model.TargetAnchor, Ref: anchor}, true
		}
	}
	content, blocks, err := w.sub(link)
	if err != nil {
		return err
	}
	if resolved {
		w.push(model.Link{Content: content, Target: target})
	} else {
		for _, in := range content {
			w.push(in)
		}
	}
	w.pushBlocks(blocks)
	return nil
}

func (w *inlineWalker) walkRun(run *opc.Element) error {
	style := w.base
	if rpr := child(run, "rPr"); rpr != nil {
		if id, ok := val(rpr, "rStyle"); ok {
			t, err := w.c.styles.runToggles(id)
			if err != nil {
				return err
			}
			style = t.over(style)
		}
		style = applyDirect(rpr, style)
	}
	return w.walkRunContent(run, style)
}

func (w *inlineWalker) walkRunContent(run *opc.Element, style model.Style) error {
	for ch := range run.Elements() {
		if ch.Is(nsMC, "AlternateContent") {
			if branch := alternateBranch(ch); branch != nil {
				if err := w.walkRunContent(branch, style); err != nil {
					return err
				}
			}
			continue
		}
		if ch.Space != nsW {
			continue
		}
		switch ch.Local {
		case "t":
			// Run edges carry the spacing between words in documents that
			// never mark xml:space, so unmarked whitespace is kept.
			if text := ooxml.CleanText(ch.Text()); text != "" {
				w.push(model.Text{Text: text, Style: style})
			}
		case "tab", "ptab":
			w.push(model.Text{Text: " "})
		case "br", "cr":
			// Every break separates the runs around it, page breaks
			// included; one ending a paragraph is trimmed at render time.
			w.push(model.LineBreak{})
		case "footnoteReference":
			if id, ok := ch.Attr(nsW, "id"); ok {
				w.push(model.NoteRef("fn" + id))
			}
		case "endnoteReference":
			if id, ok := ch.Attr(nsW, "id"); ok {
				w.push(model.NoteRef("en" + id))
			}
		case "drawing", "pict", "object":
			if err := w.drawing(ch); err != nil {
				return err
			}
		case "fldChar":
			switch typ, _ := ch.Attr(nsW, "fldCharType"); typ {
			case "begin":
				w.fields = append(w.fields, &fieldFrame{})
			case "separate":
				if n := len(w.fields); n > 0 {
					w.fields[n-1].inResult = true
				}
			case "end":
				if n := len(w.fields); n > 0 {
					frame := w.fields[n-1]
					w.fields = w.fields[:n-1]
					for _, in := range fieldResult(frame.instr.String(), frame.inlines) {
						w.push(in)
					}
				}
			}
		case "instrText":
			if n := len(w.fields); n > 0 {
				w.fields[n-1].instr.WriteString(ch.Text())
			}
		}
	}
	return nil
}

// drawing handles drawings, VML picts and embedded objects: text boxes
// become attachments at this position; charts, diagrams, objects and images
// resolve through relationships.
func (w *inlineWalker) drawing(e *opc.Element) error {
	var boxes []*opc.Element
	collectTextBoxes(e, &boxes)
	if len(boxes) > 0 {
		var blocks []model.Block
		for _, box := range boxes {
			inner, err := w.c.parseBlocks(box)
			if err != nil {
				return err
			}
			blocks = append(blocks, inner...)
		}
		w.pushBlocks(blocks)
		return nil
	}

	var descr string
	if docPr := e.FirstDescendant(nsWP, "docPr"); docPr != nil {
		d, _ := docPr.Attr(nsWP, "descr")
		descr = ooxml.CleanText(d)
	}

	if chart := e.FirstDescendant(nsChart, "chart"); chart != nil {
		if id, ok := chart.QualifiedAttr(nsR, "id"); ok {
			blocks, err := w.c.relXMLBlocks(id, w.c.charts, ooxml.ChartBlocks)
			w.pushBlocks(blocks)
			return err
		}
	}
	if relIDs := e.FirstDescendant(nsDgm, "relIds"); relIDs != nil {
		if id, ok := relIDs.QualifiedAttr(nsR, "dm"); ok {
			blocks, err := w.c.relXMLBlocks(id, w.c.diagrams, ooxml.DiagramBlocks)
			w.pushBlocks(blocks)
			return err
		}
	}

	// An OLE object wins over the VML preview image Word places next to it.
	if ole := e.FirstDescendant(nsOVML, "OLEObject"); ole != nil {
		alt := descr
		if strings.TrimSpace(alt) == "" {
			progID, ok := ole.Attr(nsOVML, "ProgID")
			if !ok {
				progID = "object"
			}
			alt = "Embedded object: " + progID
		}
		img := model.Image{Alt: alt}
		if id, ok := ole.QualifiedAttr(nsR, "id"); ok {
			part, data, err := w.c.relPart(id)
			if err != nil {
				return err
			}
			if data != nil {
				asset, err := w.c.assets.add("application/vnd.ms-ole-object", part, data)
				if err != nil {
					return err
				}
				img.Source = model.ImageSource{Kind: model.SourceAsset, Asset: asset}
			}
		}
		w.push(img)
		return nil
	}

	relID, hasImage := "", false
	if blip := e.FirstDescendant(nsA, "blip"); blip != nil {
		if relID, hasImage = blip.QualifiedAttr(nsR, "embed"); !hasImage {
			relID, hasImage = blip.QualifiedAttr(nsR, "link")
		}
	}
	if !hasImage {
		if data := e.FirstDescendant(nsVML, "imagedata"); data != nil {
			relID, hasImage = data.QualifiedAttr(nsR, "id")
		}
	}
	if hasImage {
		source, err := w.c.imageSource(relID)
		if err != nil {
			return err
		}
		if source.Kind != model.SourceUnavailable || strings.TrimSpace(descr) != "" {
			w.push(model.Image{Alt: descr, Source: source})
		}
		return nil
	}
	if strings.TrimSpace(descr) != "" {
		w.push(model.Image{Alt: descr})
	}
	return nil
}

// imageSource resolves an image relationship: external targets carry their
// URL, internal ones are retained as assets.
func (c *converter) imageSource(id string) (model.ImageSource, error) {
	rel, ok := c.rels.ByID(id)
	if !ok {
		return model.ImageSource{}, nil
	}
	if rel.External {
		if rel.Target == "" {
			return model.ImageSource{}, nil
		}
		return model.ImageSource{Kind: model.SourceExternal, URL: rel.Target}, nil
	}
	part, data, err := c.relPart(id)
	if err != nil || data == nil {
		return model.ImageSource{}, err
	}
	asset, err := c.assets.add(ooxml.MediaType(part), part, data)
	if err != nil {
		return model.ImageSource{}, err
	}
	return model.ImageSource{Kind: model.SourceAsset, Asset: asset}, nil
}

// relXMLBlocks parses a related XML part into blocks once per document,
// memoized in cache by part; a corrupt part is skipped with a warning.
func (c *converter) relXMLBlocks(id string, cache map[string][]model.Block, convert func(*opc.Element) []model.Block) ([]model.Block, error) {
	if path, ok := c.rels.PartPath(id); ok {
		if blocks, ok := cache[path]; ok {
			return slices.Clip(blocks), nil
		}
	}
	part, data, err := c.relPart(id)
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
		cache[part] = nil
		return nil, nil
	}
	blocks := convert(root)
	cache[part] = blocks
	return slices.Clip(blocks), nil
}

// collectTextBoxes finds w:txbxContent in a drawing, skipping mc:Fallback so
// AlternateContent shapes are not collected twice.
func collectTextBoxes(e *opc.Element, out *[]*opc.Element) {
	for ch := range e.Elements() {
		switch {
		case ch.Is(nsMC, "Fallback"):
		case ch.Is(nsW, "txbxContent"):
			*out = append(*out, ch)
		default:
			collectTextBoxes(ch, out)
		}
	}
}

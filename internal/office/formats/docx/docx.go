// Package docx converts WordprocessingML packages into the document model:
// package parts, then the style and numbering models, then spec-order
// property resolution over the body, notes and drawings.
package docx

import (
	"strconv"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

const (
	nsW     = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	nsR     = opc.NSRelationships
	nsA     = opc.NSDrawingML
	nsM     = opc.NSMath
	nsMC    = opc.NSMarkupCompatibility
	nsPic   = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	nsWP    = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsChart = opc.NSChart
	nsDgm   = opc.NSDiagram
	nsVML   = "urn:schemas-microsoft-com:vml"
	nsOVML  = "urn:schemas-microsoft-com:office:office"
	nsWPS   = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
	nsWPG   = "http://schemas.microsoft.com/office/word/2010/wordprocessingGroup"
)

// maxAssetTotalBytes is the fixed cap on embedded asset bytes one document
// retains.
const maxAssetTotalBytes = 128 << 20

type converter struct {
	pkg       *opc.Package
	rels      opc.Relationships
	styles    *styles
	numbering map[int]*instance
	counters  map[int]*counterState
	assets    *assetSink
	charts    map[string][]model.Block
	diagrams  map[string][]model.Block
}

// Parse converts a .docx/.docm package. Errors are opc.MalformedError,
// opc.MissingPartError, opc.ErrEncrypted or *model.LimitError; unreadable
// optional parts are skipped with a warning.
func Parse(data []byte) (*model.Document, error) {
	pkg, err := opc.Open(data)
	if err != nil {
		return nil, err
	}
	mainPart, err := pkg.MainPart("word/document.xml")
	if err != nil {
		return nil, err
	}
	docRels, err := pkg.Rels(mainPart)
	if err != nil {
		return nil, err
	}

	stylesRoot, err := pkg.OptionalXML(docRels.PartOfType(opc.RelStyles, "styles.xml"))
	if err != nil {
		return nil, err
	}
	if stylesRoot != nil && !stylesRoot.Is(nsW, "styles") {
		stylesRoot = nil
	}
	st := parseStyles(stylesRoot)

	numberingRoot, err := pkg.OptionalXML(docRels.PartOfType(opc.RelNumbering, "numbering.xml"))
	if err != nil {
		return nil, err
	}
	var numbering map[int]*instance
	if numberingRoot != nil && numberingRoot.Is(nsW, "numbering") {
		numbering, err = parseNumbering(numberingRoot, st.directNumID)
		if err != nil {
			return nil, err
		}
	}

	docRoot, err := pkg.XML(mainPart)
	if err != nil {
		return nil, err
	}
	body := docRoot.Child(nsW, "body")
	if !docRoot.Is(nsW, "document") || body == nil {
		return nil, &opc.MalformedError{Part: mainPart, Detail: "no document body"}
	}

	c := &converter{
		pkg:       pkg,
		rels:      docRels,
		styles:    st,
		numbering: numbering,
		counters:  map[int]*counterState{},
		assets:    &assetSink{byPart: map[string]model.AssetID{}},
		charts:    map[string][]model.Block{},
		diagrams:  map[string][]model.Block{},
	}
	blocks, err := c.parseBlocks(body)
	if err != nil {
		return nil, err
	}

	var notes []model.Note
	for _, n := range []struct {
		relType, conventional, root, elem, prefix string
		kind                                      model.NoteKind
	}{
		{opc.RelFootnotes, "footnotes.xml", "footnotes", "footnote", "fn", model.Footnote},
		{opc.RelEndnotes, "endnotes.xml", "endnotes", "endnote", "en", model.Endnote},
	} {
		part := docRels.PartOfType(n.relType, n.conventional)
		root, err := pkg.OptionalXML(part)
		if err != nil {
			return nil, err
		}
		if root == nil || !root.Is(nsW, n.root) {
			continue
		}
		noteRels, err := pkg.Rels(part)
		if err != nil {
			return nil, err
		}
		nc := *c
		nc.rels = noteRels
		for note := range root.Children(nsW, n.elem) {
			switch typ, _ := note.Attr(nsW, "type"); typ {
			case "separator", "continuationSeparator", "continuationNotice":
				continue
			}
			id, ok := note.Attr(nsW, "id")
			if !ok {
				continue
			}
			noteBlocks, err := nc.parseBlocks(note)
			if err != nil {
				return nil, err
			}
			notes = append(notes, model.Note{ID: n.prefix + id, Kind: n.kind, Blocks: noteBlocks})
		}
	}
	return &model.Document{Blocks: blocks, Notes: notes, Assets: c.assets.assets}, nil
}

// relPart loads the internal target of a relationship; unknown, external,
// unresolvable and missing targets yield nothing.
func (c *converter) relPart(id string) (string, []byte, error) {
	path, ok := c.rels.PartPath(id)
	if !ok {
		return "", nil, nil
	}
	data, err := c.pkg.OptionalPart(path)
	if err != nil {
		return "", nil, err
	}
	if data == nil {
		model.Log.Warn("relationship target is missing", "part", path)
		return "", nil, nil
	}
	return path, data, nil
}

type assetSink struct {
	assets []model.Asset
	byPart map[string]model.AssetID
	total  int
}

// add retains an asset once per origin part, so repeated references share
// one asset and are charged against the cap once.
func (s *assetSink) add(mediaType, part string, data []byte) (model.AssetID, error) {
	if id, ok := s.byPart[part]; ok {
		return id, nil
	}
	s.total += len(data)
	if s.total > maxAssetTotalBytes {
		return 0, &model.LimitError{Limit: "max_asset_total_bytes", Detail: "embedded assets exceed the retained-bytes cap"}
	}
	id := model.AssetID(len(s.assets))
	s.byPart[part] = id
	s.assets = append(s.assets, model.Asset{ID: id, MediaType: mediaType, OriginPart: part, Bytes: data})
	return id, nil
}

func child(e *opc.Element, local string) *opc.Element {
	if e == nil {
		return nil
	}
	return e.Child(nsW, local)
}

// val is the w:val of the w:local child of e.
func val(e *opc.Element, local string) (string, bool) {
	c := child(e, local)
	if c == nil {
		return "", false
	}
	return c.Attr(nsW, "val")
}

func intVal(e *opc.Element, local string) (int, bool) {
	v, ok := val(e, local)
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(v)
	return n, err == nil
}

// onOff reads an ST_OnOff child: present without a value, or with any value
// but 0/false/off/none, is on.
func onOff(parent *opc.Element, local string) (on, specified bool) {
	c := child(parent, local)
	if c == nil {
		return false, false
	}
	v, _ := c.Attr(nsW, "val")
	switch v {
	case "0", "false", "off", "none":
		return false, true
	}
	return true, true
}

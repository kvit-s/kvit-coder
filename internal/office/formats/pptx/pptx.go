// Package pptx converts PresentationML packages to the document model:
// slides in presentation order through the full text cascade, with speaker
// notes quoted after each slide.
package pptx

import (
	"strconv"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

const (
	nsP   = "http://schemas.openxmlformats.org/presentationml/2006/main"
	nsA14 = "http://schemas.microsoft.com/office/drawing/2010/main"
)

// maxAssetBytes caps the embedded bytes one document retains.
const maxAssetBytes = 128 << 20

type layout struct {
	placeholders []placeholder
	master       string
}

type master struct {
	title, body, other levelStyle
	placeholders       []placeholder
}

type converter struct {
	pkg          *opc.Package
	defaultText  levelStyle
	layouts      map[string]*layout
	masters      map[string]*master
	slideAnchors map[string]string
	instances    int
	assets       []model.Asset
	assetByPart  map[string]model.AssetID
	assetBytes   int
	expansion    uint64
}

func Parse(data []byte) (*model.Document, error) {
	pkg, err := opc.Open(data)
	if err != nil {
		return nil, err
	}
	presPart, err := pkg.MainPart("ppt/presentation.xml")
	if err != nil {
		return nil, err
	}
	pres, err := pkg.XML(presPart)
	if err != nil {
		return nil, err
	}
	presRels, err := pkg.Rels(presPart)
	if err != nil {
		return nil, err
	}
	c := &converter{
		pkg:          pkg,
		defaultText:  parseLevelStyles(pres.FirstDescendant(nsP, "defaultTextStyle")),
		layouts:      map[string]*layout{},
		masters:      map[string]*master{},
		slideAnchors: map[string]string{},
		assetByPart:  map[string]model.AssetID{},
	}
	var slides []string
	if list := pres.FirstDescendant(nsP, "sldIdLst"); list != nil {
		for sld := range list.Children(nsP, "sldId") {
			id, ok := sld.QualifiedAttr(opc.NSRelationships, "id")
			if !ok {
				continue
			}
			if path, ok := presRels.PartPath(id); ok {
				slides = append(slides, path)
			}
		}
	}
	if len(slides) == 0 {
		return nil, &opc.MalformedError{Part: presPart, Detail: "presentation has no slide list"}
	}
	for i, path := range slides {
		c.slideAnchors[path] = "slide-" + strconv.Itoa(i+1)
	}
	rels := make([]opc.Relationships, len(slides))
	targeted := map[string]bool{}
	for i, path := range slides {
		if rels[i], err = pkg.Rels(path); err != nil {
			return nil, err
		}
		for _, rel := range rels[i].List {
			if rel.Type != opc.RelSlide || rel.External {
				continue
			}
			if t, err := opc.Resolve(path, rel.Target); err == nil && c.slideAnchors[t.Path] != "" {
				targeted[t.Path] = true
			}
		}
	}

	var blocks []model.Block
	failed := 0
	for i, path := range slides {
		root, err := pkg.OptionalXML(path)
		if err != nil {
			return nil, err
		}
		spTree := slideShapeTree(root)
		if spTree == nil {
			model.Log.Warn("skipping unusable slide", "part", path)
			failed++
			continue
		}
		s := &slide{c: c, rels: rels[i], part: path}
		if s.layout, s.master, err = c.loadLayout(rels[i]); err != nil {
			return nil, err
		}
		if targeted[path] {
			blocks = append(blocks, model.Paragraph{model.Anchor(c.slideAnchors[path])})
		}
		if blocks, err = s.shapes(spTree, blocks); err != nil {
			return nil, err
		}
		if blocks, err = c.notes(rels[i], blocks); err != nil {
			return nil, err
		}
	}
	if failed == len(slides) {
		return nil, &opc.MalformedError{Detail: "no slide in the presentation could be read"}
	}
	return &model.Document{Blocks: blocks, Assets: c.assets}, nil
}

func slideShapeTree(root *opc.Element) *opc.Element {
	if root == nil || !root.Is(nsP, "sld") {
		return nil
	}
	if cSld := root.Child(nsP, "cSld"); cSld != nil {
		return cSld.Child(nsP, "spTree")
	}
	return nil
}

func relatedPart(rels opc.Relationships, relType string) (string, bool) {
	rel, ok := rels.FirstOfType(relType)
	if !ok {
		return "", false
	}
	t, err := opc.Resolve(rels.Source, rel.Target)
	return t.Path, err == nil
}

func (c *converter) loadLayout(slideRels opc.Relationships) (*layout, *master, error) {
	path, ok := relatedPart(slideRels, opc.RelSlideLayout)
	if !ok {
		return nil, nil, nil
	}
	l, ok := c.layouts[path]
	if !ok {
		l = &layout{}
		root, err := c.pkg.OptionalXML(path)
		if err != nil {
			return nil, nil, err
		}
		if root != nil {
			if spTree := root.FirstDescendant(nsP, "spTree"); spTree != nil {
				l.placeholders = collectPlaceholders(spTree)
			}
		}
		rels, err := c.pkg.Rels(path)
		if err != nil {
			return nil, nil, err
		}
		l.master, _ = relatedPart(rels, opc.RelSlideMaster)
		if err := c.loadMaster(l.master); err != nil {
			return nil, nil, err
		}
		c.layouts[path] = l
	}
	return l, c.masters[l.master], nil
}

func (c *converter) loadMaster(path string) error {
	if _, done := c.masters[path]; done || path == "" {
		return nil
	}
	m := &master{}
	root, err := c.pkg.OptionalXML(path)
	if err != nil {
		return err
	}
	if root != nil {
		if styles := root.FirstDescendant(nsP, "txStyles"); styles != nil {
			m.title = parseLevelStyles(styles.Child(nsP, "titleStyle"))
			m.body = parseLevelStyles(styles.Child(nsP, "bodyStyle"))
			m.other = parseLevelStyles(styles.Child(nsP, "otherStyle"))
		}
		if spTree := root.FirstDescendant(nsP, "spTree"); spTree != nil {
			m.placeholders = collectPlaceholders(spTree)
		}
	}
	c.masters[path] = m
	return nil
}

// notes appends the slide's speaker notes as a quote. Real producers use a
// body placeholder and some write plain text boxes, so every text body is
// kept except the slide image and chrome placeholders.
func (c *converter) notes(slideRels opc.Relationships, blocks []model.Block) ([]model.Block, error) {
	path, ok := relatedPart(slideRels, opc.RelNotesSlide)
	if !ok {
		return blocks, nil
	}
	root, err := c.pkg.OptionalXML(path)
	if err != nil || root == nil {
		return blocks, err
	}
	rels, err := c.pkg.Rels(path)
	if err != nil {
		return nil, err
	}
	s := &slide{c: c, rels: rels, part: path}
	var quote []model.Block
	for sp := range root.Descendants(nsP, "sp") {
		if ph := placeholderOf(sp); ph != nil {
			switch ph.phType {
			case "sldImg", "sldNum", "hdr", "ftr", "dt":
				continue
			}
		}
		if tx := sp.Child(nsP, "txBody"); tx != nil {
			quote = s.textBody(tx, nil, quote)
		}
	}
	if len(quote) > 0 {
		blocks = append(blocks, model.Quote(quote))
	}
	return blocks, nil
}

// asset retains an embedded part once per origin part under maxAssetBytes.
func (c *converter) asset(mediaType, part string, data []byte) (model.AssetID, error) {
	if id, ok := c.assetByPart[part]; ok {
		return id, nil
	}
	c.assetBytes += len(data)
	if c.assetBytes > maxAssetBytes {
		return 0, &model.LimitError{Limit: "max_asset_total_bytes", Detail: "embedded assets exceed the retained-bytes cap"}
	}
	id := model.AssetID(len(c.assets))
	c.assetByPart[part] = id
	c.assets = append(c.assets, model.Asset{ID: id, MediaType: mediaType, OriginPart: part, Bytes: data})
	return id, nil
}

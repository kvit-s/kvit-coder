package opc

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"iter"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

const (
	NSRelationships        = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	NSPackageRelationships = "http://schemas.openxmlformats.org/package/2006/relationships"
	NSMarkupCompatibility  = "http://schemas.openxmlformats.org/markup-compatibility/2006"
	NSDrawingML            = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NSMath                 = "http://schemas.openxmlformats.org/officeDocument/2006/math"
	NSChart                = "http://schemas.openxmlformats.org/drawingml/2006/chart"
	NSDiagram              = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
	nsXML                  = "http://www.w3.org/XML/1998/namespace"
)

// Element is a namespace-resolved XML element. Space holds the namespace URI
// with ISO/IEC 29500 Strict URIs normalized to their Transitional form, so
// callers match one set of constants.
type Element struct {
	Space string
	Local string
	Attrs []Attr
	Nodes []Node
}

type Attr struct {
	Space string
	Local string
	Value string
}

// Node is a child element, or a text run when Elem is nil.
type Node struct {
	Elem *Element
	Text string
}

func (e *Element) Is(space, local string) bool {
	return e.Local == local && e.Space == space
}

// Attr looks up a same-vocabulary attribute: the qualified one wins, and an
// unqualified one with the same local name is accepted because schemas often
// leave their own attributes unqualified. Cross-vocabulary lookups (r:id)
// must use QualifiedAttr.
func (e *Element) Attr(space, local string) (string, bool) {
	if v, ok := e.QualifiedAttr(space, local); ok {
		return v, true
	}
	return e.QualifiedAttr("", local)
}

func (e *Element) QualifiedAttr(space, local string) (string, bool) {
	for _, a := range e.Attrs {
		if a.Local == local && a.Space == space {
			return a.Value, true
		}
	}
	return "", false
}

func (e *Element) AnyAttr(local string) (string, bool) {
	for _, a := range e.Attrs {
		if a.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

func (e *Element) Elements() iter.Seq[*Element] {
	return func(yield func(*Element) bool) {
		for _, n := range e.Nodes {
			if n.Elem != nil && !yield(n.Elem) {
				return
			}
		}
	}
}

func (e *Element) Child(space, local string) *Element {
	for c := range e.Children(space, local) {
		return c
	}
	return nil
}

func (e *Element) Children(space, local string) iter.Seq[*Element] {
	return func(yield func(*Element) bool) {
		for c := range e.Elements() {
			if c.Is(space, local) && !yield(c) {
				return
			}
		}
	}
}

// Descendants yields matching elements below e, depth-first in document
// order.
func (e *Element) Descendants(space, local string) iter.Seq[*Element] {
	return func(yield func(*Element) bool) {
		e.walk(func(d *Element) bool { return !d.Is(space, local) || yield(d) })
	}
}

// FirstDescendant returns the first matching element below e in document
// order, or nil.
func (e *Element) FirstDescendant(space, local string) *Element {
	for d := range e.Descendants(space, local) {
		return d
	}
	return nil
}

func (e *Element) walk(visit func(*Element) bool) bool {
	for c := range e.Elements() {
		if !visit(c) || !c.walk(visit) {
			return false
		}
	}
	return true
}

func (e *Element) Text() string {
	var sb strings.Builder
	e.writeText(&sb)
	return sb.String()
}

func (e *Element) writeText(sb *strings.Builder) {
	for _, n := range e.Nodes {
		if n.Elem != nil {
			n.Elem.writeText(sb)
		} else {
			sb.WriteString(n.Text)
		}
	}
}

// ParseXML parses a part into its root element, enforcing MaxXMLDepth and
// MaxXMLNodes. The encoding comes from the BOM or the declaration. Unclosed
// and mismatched tags are repaired: an end tag closes the innermost open
// element whatever its name.
func ParseXML(data []byte) (*Element, error) {
	return parseXML(data, MaxXMLNodes)
}

func parseXML(data []byte, maxNodes int) (*Element, error) {
	data = toUTF8(data)
	if root, err := scanXML(data, maxNodes); err != errNotScannable {
		return root, err
	}
	return decodeXML(data, maxNodes)
}

func decodeXML(data []byte, maxNodes int) (*Element, error) {
	b := &treeBuilder{maxNodes: maxNodes}
	d := xml.NewDecoder(bytes.NewReader(data))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = charsetReader
	for {
		tok, err := d.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, &MalformedError{Detail: "unparseable xml: " + err.Error()}
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			err = b.start(tok.Name, tok.Attr)
		case xml.EndElement:
			b.end()
		case xml.CharData:
			err = b.text(string(tok))
		}
		if err != nil {
			return nil, err
		}
	}
	return b.finish()
}

// treeBuilder assembles the element tree from raw tokens, resolving
// namespaces and enforcing the depth and node caps. Children collect on a
// shared pending stack and are copied into an exact-size slice when their
// element closes; elements, nodes and attributes come from slabs, so a
// large part costs a handful of allocations per thousand elements.
type treeBuilder struct {
	maxNodes int
	nodes    int
	root     *Element
	stack    []openElement
	pending  []Node
	elems    []Element
	nodeSlab []Node
	attrSlab []Attr
	// merging marks the last pending node as a text run whose content is
	// being accumulated in textBuf; it is written back by flushText.
	merging bool
	textBuf []byte
}

type openElement struct {
	elem  *Element
	scope map[string]string
	first int
}

// slabLen grows slabs with the part, so small parts stay small and large
// ones allocate a few big chunks.
func slabLen(nodes int) int {
	return min(max(nodes, 8), 1024)
}

func (b *treeBuilder) lookup(prefix string) (string, bool) {
	for i := len(b.stack) - 1; i >= 0; i-- {
		if uri, ok := b.stack[i].scope[prefix]; ok {
			return uri, true
		}
	}
	return "", false
}

func isNamespaceDecl(a xml.Attr) bool {
	return a.Name.Space == "xmlns" || a.Name.Space == "" && a.Name.Local == "xmlns"
}

func (b *treeBuilder) start(name xml.Name, attrs []xml.Attr) error {
	if len(b.stack) >= MaxXMLDepth {
		return &model.LimitError{Limit: "max_xml_depth", Detail: "element nesting exceeds the depth cap"}
	}
	if b.nodes++; b.nodes > b.maxNodes {
		return nodeLimit()
	}
	b.flushText()
	var scope map[string]string
	kept := 0
	for _, a := range attrs {
		if !isNamespaceDecl(a) {
			kept++
			continue
		}
		if scope == nil {
			scope = map[string]string{}
		}
		if a.Name.Space == "xmlns" {
			scope[a.Name.Local] = a.Value
		} else {
			scope[""] = a.Value
		}
	}
	if len(b.elems) == 0 {
		b.elems = make([]Element, slabLen(b.nodes))
	}
	elem := &b.elems[0]
	b.elems = b.elems[1:]
	elem.Local = name.Local
	if len(b.stack) > 0 {
		b.pending = append(b.pending, Node{Elem: elem})
	} else if b.root == nil {
		b.root = elem
	}
	b.stack = append(b.stack, openElement{elem: elem, scope: scope, first: len(b.pending)})
	switch name.Space {
	case "xml":
		elem.Space = nsXML
	default:
		uri, _ := b.lookup(name.Space)
		elem.Space = normalizeURI(uri)
	}
	if kept > 0 {
		elem.Attrs = b.takeAttrs(kept)
		for _, a := range attrs {
			if isNamespaceDecl(a) {
				continue
			}
			attr := Attr{Local: a.Name.Local, Value: a.Value}
			switch a.Name.Space {
			case "":
			case "xml":
				attr.Space = nsXML
			default:
				if uri, ok := b.lookup(a.Name.Space); ok {
					attr.Space = normalizeURI(uri)
				}
			}
			elem.Attrs = append(elem.Attrs, attr)
		}
		if elem.Is(NSMarkupCompatibility, "Choice") {
			resolveRequires(elem, b.lookup)
		}
	}
	return nil
}

func (b *treeBuilder) takeAttrs(n int) []Attr {
	if n > len(b.attrSlab) {
		b.attrSlab = make([]Attr, max(n, slabLen(b.nodes)))
	}
	attrs := b.attrSlab[:0:n]
	b.attrSlab = b.attrSlab[n:]
	return attrs
}

// end closes the innermost open element whatever the end tag names.
func (b *treeBuilder) end() {
	if len(b.stack) == 0 {
		return
	}
	b.flushText()
	top := b.stack[len(b.stack)-1]
	b.stack = b.stack[:len(b.stack)-1]
	if n := len(b.pending) - top.first; n > 0 {
		if n > len(b.nodeSlab) {
			b.nodeSlab = make([]Node, max(n, slabLen(b.nodes)))
		}
		top.elem.Nodes = b.nodeSlab[:n:n]
		b.nodeSlab = b.nodeSlab[n:]
		copy(top.elem.Nodes, b.pending[top.first:])
		clear(b.pending[top.first:])
		b.pending = b.pending[:top.first]
	}
}

func (b *treeBuilder) text(s string) error {
	if len(b.stack) == 0 || s == "" {
		return nil
	}
	if last := len(b.pending) - 1; last >= b.stack[len(b.stack)-1].first && b.pending[last].Elem == nil {
		if !b.merging {
			b.textBuf = append(b.textBuf[:0], b.pending[last].Text...)
			b.merging = true
		}
		b.textBuf = append(b.textBuf, s...)
		return nil
	}
	if b.nodes++; b.nodes > b.maxNodes {
		return nodeLimit()
	}
	b.pending = append(b.pending, Node{Text: s})
	return nil
}

func (b *treeBuilder) flushText() {
	if !b.merging {
		return
	}
	b.pending[len(b.pending)-1].Text = string(b.textBuf)
	b.merging = false
}

func (b *treeBuilder) finish() (*Element, error) {
	for len(b.stack) > 0 {
		b.end()
	}
	if b.root == nil {
		return nil, &MalformedError{Detail: "xml has no root element"}
	}
	return b.root, nil
}

func nodeLimit() error {
	return &model.LimitError{Limit: "max_xml_nodes", Detail: "part exceeds the xml node cap"}
}

// resolveRequires rewrites mc:Choice/@Requires prefixes to namespace URIs in
// the element's own lexical scope, where local rebinding is visible;
// unresolvable prefixes stay literal and match no requirement.
func resolveRequires(e *Element, lookup func(string) (string, bool)) {
	for i, a := range e.Attrs {
		if a.Local != "Requires" {
			continue
		}
		prefixes := strings.Fields(a.Value)
		for j, prefix := range prefixes {
			if uri, ok := lookup(prefix); ok {
				prefixes[j] = normalizeURI(uri)
			}
		}
		e.Attrs[i].Value = strings.Join(prefixes, " ")
	}
}

// normalizeURI maps a Strict OOXML URI (re-rooted under purl.oclc.org with
// the 2006 segment dropped) onto its Transitional form.
func normalizeURI(uri string) string {
	rest, ok := strings.CutPrefix(uri, "http://purl.oclc.org/ooxml/")
	if !ok {
		return uri
	}
	family, tail, ok := strings.Cut(rest, "/")
	if !ok {
		return uri
	}
	return "http://schemas.openxmlformats.org/" + family + "/2006/" + tail
}

func toUTF8(data []byte) []byte {
	switch {
	case bytes.HasPrefix(data, []byte{0xEF, 0xBB, 0xBF}):
		return data[3:]
	case bytes.HasPrefix(data, []byte{0xFF, 0xFE}):
		return utf16ToUTF8(data[2:], func(b []byte) uint16 { return uint16(b[0]) | uint16(b[1])<<8 })
	case bytes.HasPrefix(data, []byte{0xFE, 0xFF}):
		return utf16ToUTF8(data[2:], func(b []byte) uint16 { return uint16(b[1]) | uint16(b[0])<<8 })
	}
	return data
}

func utf16ToUTF8(data []byte, unit func([]byte) uint16) []byte {
	units := make([]uint16, len(data)/2)
	for i := range units {
		units[i] = unit(data[2*i:])
	}
	return []byte(string(utf16.Decode(units)))
}

// charsetReader accepts declarations the input already satisfies after BOM
// transcoding, and decodes Latin-1 family declarations.
func charsetReader(label string, input io.Reader) (io.Reader, error) {
	switch strings.ToLower(label) {
	case "utf-16", "utf-16le", "utf-16be", "us-ascii", "ascii":
		return input, nil
	case "iso-8859-1", "latin1", "windows-1252", "cp1252":
		data, err := io.ReadAll(input)
		if err != nil {
			return nil, err
		}
		out := make([]byte, 0, len(data))
		for _, b := range data {
			out = utf8.AppendRune(out, rune(b))
		}
		return bytes.NewReader(out), nil
	}
	return nil, errors.New("unsupported xml encoding " + label)
}

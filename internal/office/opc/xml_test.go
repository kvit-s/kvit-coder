package opc

import (
	"errors"
	"runtime"
	"slices"
	"strings"
	"testing"
	"unicode/utf16"
)

func mustParse(t *testing.T, s string) *Element {
	t.Helper()
	root, err := ParseXML([]byte(s))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func TestNamespacesResolveRegardlessOfPrefix(t *testing.T) {
	root := mustParse(t, `<x:root xmlns:x="urn:a" xmlns:y="urn:b"><y:kid x:id="1" plain="p"/></x:root>`)
	if !root.Is("urn:a", "root") {
		t.Fatalf("root %s %s", root.Space, root.Local)
	}
	kid := root.Child("urn:b", "kid")
	if kid == nil || root.Child("urn:a", "kid") != nil {
		t.Fatal("child lookup must match namespace URIs, not prefixes")
	}
	if v, ok := kid.Attr("urn:a", "id"); !ok || v != "1" {
		t.Fatalf("qualified attr %q %v", v, ok)
	}
	if v, ok := kid.Attr("urn:whatever", "plain"); !ok || v != "p" {
		t.Fatalf("lenient attr falls back to unqualified, got %q %v", v, ok)
	}
}

func TestQualifiedAndUnqualifiedAttrs(t *testing.T) {
	root := mustParse(t, `<x:root xmlns:x="urn:a" xmlns:r="urn:r"><x:kid r:id="rel" id="plain"/></x:root>`)
	kid := root.Child("urn:a", "kid")
	if v, _ := kid.QualifiedAttr("urn:r", "id"); v != "rel" {
		t.Fatalf("qualified %q", v)
	}
	if v, _ := kid.Attr("urn:r", "id"); v != "rel" {
		t.Fatalf("lenient prefers qualified, got %q", v)
	}
	if _, ok := kid.QualifiedAttr("urn:other", "id"); ok {
		t.Fatal("strict lookup never falls back")
	}
	if v, _ := kid.AnyAttr("id"); v != "rel" && v != "plain" {
		t.Fatalf("any attr %q", v)
	}
}

func TestStrictNamespacesNormalize(t *testing.T) {
	root := mustParse(t, `<w:document xmlns:w="http://purl.oclc.org/ooxml/wordprocessingml/main" xmlns:r="http://purl.oclc.org/ooxml/officeDocument/relationships"><w:body r:id="x"/></w:document>`)
	if !root.Is("http://schemas.openxmlformats.org/wordprocessingml/2006/main", "document") {
		t.Fatalf("root space %q", root.Space)
	}
	body := root.Child("http://schemas.openxmlformats.org/wordprocessingml/2006/main", "body")
	if v, ok := body.QualifiedAttr(NSRelationships, "id"); !ok || v != "x" {
		t.Fatalf("strict attr namespace must normalize, got %q %v", v, ok)
	}
}

func TestDefaultNamespaceAppliesToElementsOnly(t *testing.T) {
	root := mustParse(t, `<root xmlns="urn:d"><kid id="1"/></root>`)
	kid := root.Child("urn:d", "kid")
	if v, ok := kid.QualifiedAttr("", "id"); !ok || v != "1" {
		t.Fatalf("unprefixed attribute has no namespace, got %q %v", v, ok)
	}
}

func TestRequiresPrefixesResolveInScope(t *testing.T) {
	root := mustParse(t, `<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006">
		<mc:Choice xmlns:z="urn:one" Requires="z"/>
		<mc:Choice xmlns:z="urn:two" Requires="z unknownprefix"/>
		<mc:Choice xmlns:s="http://purl.oclc.org/ooxml/drawingml/main" Requires="s"/>
	</mc:AlternateContent>`)
	var got []string
	for c := range root.Children(NSMarkupCompatibility, "Choice") {
		v, _ := c.AnyAttr("Requires")
		got = append(got, v)
	}
	want := []string{"urn:one", "urn:two unknownprefix", NSDrawingML}
	if !slices.Equal(got, want) {
		t.Fatalf("Requires = %q, want %q", got, want)
	}
}

func TestTextEntitiesAndTraversal(t *testing.T) {
	root := mustParse(t, `<?xml version="1.0"?><a>x &amp; &#x41;&nbsp;<b>y<c>z</c></b><![CDATA[<w>]]><c>q</c></a>`)
	if got := root.Text(); got != "x & A yz<w>q" {
		t.Fatalf("Text = %q", got)
	}
	var cs []string
	for c := range root.Descendants("", "c") {
		cs = append(cs, c.Text())
	}
	if !slices.Equal(cs, []string{"z", "q"}) {
		t.Fatalf("descendants %q", cs)
	}
	var names []string
	for e := range root.Elements() {
		names = append(names, e.Local)
	}
	if !slices.Equal(names, []string{"b", "c"}) {
		t.Fatalf("elements %q", names)
	}
	if len(root.Nodes) != 4 || root.Nodes[0].Text != "x & A " || root.Nodes[2].Text != "<w>" {
		t.Fatalf("nodes %+v", root.Nodes)
	}
}

func TestUTF16WithBOM(t *testing.T) {
	text := `<?xml version="1.0" encoding="UTF-16"?><r a="café">你好</r>`
	data := []byte{0xFF, 0xFE}
	for _, u := range utf16.Encode([]rune(text)) {
		data = append(data, byte(u), byte(u>>8))
	}
	root := mustParse(t, string(data))
	if v, _ := root.AnyAttr("a"); v != "café" || root.Text() != "你好" {
		t.Fatalf("attr %q text %q", v, root.Text())
	}
	be := []byte{0xFE, 0xFF}
	for _, u := range utf16.Encode([]rune("<r>é</r>")) {
		be = append(be, byte(u>>8), byte(u))
	}
	if got := mustParse(t, string(be)).Text(); got != "é" {
		t.Fatalf("utf-16be text %q", got)
	}
}

func TestLatin1Declaration(t *testing.T) {
	root := mustParse(t, "<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><r>caf\xe9</r>")
	if root.Text() != "café" {
		t.Fatalf("text %q", root.Text())
	}
}

func TestUTF8BOM(t *testing.T) {
	if got := mustParse(t, "\xEF\xBB\xBF<r>x</r>").Text(); got != "x" {
		t.Fatalf("text %q", got)
	}
}

func TestMalformedXMLRecovery(t *testing.T) {
	root := mustParse(t, `<a><b>text`)
	if root.Local != "a" || root.Text() != "text" || root.Child("", "b") == nil {
		t.Fatalf("unclosed elements must be recovered, got %+v", root)
	}
	root = mustParse(t, `<a><b>one</c>two</a>`)
	if root.Text() != "onetwo" {
		t.Fatalf("mismatched end tag must be recovered, got %q", root.Text())
	}
	var me *MalformedError
	for _, bad := range []string{"", "just text", "<a><<<>"} {
		if _, err := ParseXML([]byte(bad)); !errors.As(err, &me) {
			t.Errorf("ParseXML(%q) want malformed, got %v", bad, err)
		}
	}
}

func TestXMLDepthLimit(t *testing.T) {
	deep := strings.Repeat("<d>", MaxXMLDepth+1)
	_, err := ParseXML([]byte(deep))
	wantLimit(t, err, "max_xml_depth")
	ok := strings.Repeat("<d>", MaxXMLDepth) + "leaf" + strings.Repeat("</d>", MaxXMLDepth)
	if got := mustParse(t, ok).Text(); got != "leaf" {
		t.Fatalf("depth at the cap must parse, got %q", got)
	}
}

func TestXMLNodeLimit(t *testing.T) {
	doc := []byte("<r>" + strings.Repeat("<e>t</e>", 5) + "</r>")
	if _, err := parseXML(doc, 11); err != nil {
		t.Fatalf("11 nodes fit an 11-node cap: %v", err)
	}
	_, err := parseXML(doc, 10)
	wantLimit(t, err, "max_xml_nodes")
}

func TestPackageXMLParts(t *testing.T) {
	p := open(t, part{"good.xml", "<r>x</r>"}, part{"bad.xml", "not xml"})
	if root, err := p.XML("good.xml"); err != nil || root.Text() != "x" {
		t.Fatalf("XML = %v, %v", root, err)
	}
	var me *MalformedError
	if _, err := p.XML("bad.xml"); !errors.As(err, &me) || me.Part != "bad.xml" {
		t.Fatalf("required corrupt xml must be malformed with its part, got %v", err)
	}
	var missing *MissingPartError
	if _, err := p.XML("gone.xml"); !errors.As(err, &missing) {
		t.Fatalf("required absent xml must be missing, got %v", err)
	}
	for _, name := range []string{"bad.xml", "gone.xml"} {
		if root, err := p.OptionalXML(name); root != nil || err != nil {
			t.Fatalf("OptionalXML(%q) = %v, %v", name, root, err)
		}
	}
}

func TestSplitTextMergesInLinearSpace(t *testing.T) {
	const runs = 100_000
	data := []byte("<r>" + strings.Repeat("x<!---->", runs) + "y</r>")
	want := strings.Repeat("x", runs) + "y"
	parsers := []struct {
		name  string
		parse func([]byte, int) (*Element, error)
	}{
		{"scanner", scanXML},
		{"decoder", decodeXML},
	}
	for _, p := range parsers {
		t.Run(p.name, func(t *testing.T) {
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			root, err := p.parse(data, MaxXMLNodes)
			runtime.ReadMemStats(&after)
			if err != nil {
				t.Fatal(err)
			}
			if len(root.Nodes) != 1 || root.Nodes[0].Text != want {
				t.Fatalf("got %d nodes, text length %d; want one text node of length %d", len(root.Nodes), len(root.Text()), len(want))
			}
			if allocated := after.TotalAlloc - before.TotalAlloc; allocated > 64<<20 {
				t.Fatalf("allocated %d bytes merging %d text runs", allocated, runs)
			}
		})
	}
}

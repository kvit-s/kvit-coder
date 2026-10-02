package docx

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
	"github.com/kvit-s/kvit-coder/internal/office/render/gfm"
)

const wNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`

func pkg(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, parts[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func body(inner string) string {
	return `<w:document ` + wNS + ` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"><w:body>` + inner + `</w:body></w:document>`
}

func parse(t *testing.T, parts map[string]string) *model.Document {
	t.Helper()
	doc, err := Parse(pkg(t, parts))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}

func markdown(t *testing.T, parts map[string]string) string {
	t.Helper()
	return gfm.Render(parse(t, parts))
}

func TestLinkedImageBecomesExternalSource(t *testing.T) {
	doc := parse(t, map[string]string{
		"word/document.xml": body(`<w:p><w:r><w:drawing><a:blip r:link="rId9"/></w:drawing></w:r></w:p>`),
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
			<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
				Target="https://e.com/pic.png" TargetMode="External"/></Relationships>`,
	})
	p := doc.Blocks[0].(model.Paragraph)
	img := p[0].(model.Image)
	if img.Source.Kind != model.SourceExternal || img.Source.URL != "https://e.com/pic.png" || len(doc.Assets) != 0 {
		t.Fatalf("image %+v, assets %d", img, len(doc.Assets))
	}
}

func numbering(levels string) string {
	return `<w:numbering ` + wNS + `><w:abstractNum w:abstractNumId="0">` + levels +
		`</w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num></w:numbering>`
}

const numberedPara = `<w:p><w:pPr><w:numPr><w:ilvl w:val="0"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>item</w:t></w:r></w:p>`

func TestHugeNumberingStartsClamp(t *testing.T) {
	for _, start := range []string{"18446744073709551615", "-5", "2147483647"} {
		md := markdown(t, map[string]string{
			"word/document.xml":  body(numberedPara + numberedPara),
			"word/numbering.xml": numbering(`<w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/><w:start w:val="` + start + `"/></w:lvl>`),
		})
		if !strings.Contains(md, "item") {
			t.Fatalf("start %s: %q", start, md)
		}
	}
}

func TestPartialNumPrInheritsFromStyle(t *testing.T) {
	md := markdown(t, map[string]string{
		"word/document.xml": body(`<w:p><w:pPr><w:pStyle w:val="Listy"/><w:numPr><w:ilvl w:val="1"/></w:numPr></w:pPr><w:r><w:t>second level</w:t></w:r></w:p>`),
		"word/styles.xml": `<w:styles ` + wNS + `><w:style w:type="paragraph" w:styleId="Listy">
			<w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr></w:style></w:styles>`,
		"word/numbering.xml": numbering(`<w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/></w:lvl><w:lvl w:ilvl="1"><w:numFmt w:val="lowerLetter"/></w:lvl>`),
	})
	if md != "- a. second level\n" {
		t.Fatalf("got %q", md)
	}
}

func TestRestartedAndCompositeNumbering(t *testing.T) {
	levels := `<w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/></w:lvl>
		<w:lvl w:ilvl="1"><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%1-%2)"/></w:lvl>`
	para := func(ilvl, text string) string {
		return `<w:p><w:pPr><w:numPr><w:ilvl w:val="` + ilvl + `"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>` + text + `</w:t></w:r></w:p>`
	}
	md := markdown(t, map[string]string{
		"word/document.xml":  body(para("0", "one") + para("1", "sub") + para("1", "sub2") + para("0", "two") + para("1", "restarted")),
		"word/numbering.xml": numbering(levels),
	})
	want := "1. one\n\n   - 1-a) sub\n   - 1-b) sub2\n\n2. two\n\n   - 2-a) restarted\n"
	if md != want {
		t.Fatalf("got %q\nwant %q", md, want)
	}
}

func TestNumberingCycleIsMalformed(t *testing.T) {
	_, err := Parse(pkg(t, map[string]string{
		"word/document.xml": body(numberedPara),
		"word/styles.xml": `<w:styles ` + wNS + `><w:style w:type="numbering" w:styleId="Loop">
			<w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr></w:style></w:styles>`,
		"word/numbering.xml": `<w:numbering ` + wNS + `><w:abstractNum w:abstractNumId="0"><w:numStyleLink w:val="Loop"/></w:abstractNum>
			<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num></w:numbering>`,
	}))
	var me *opc.MalformedError
	if !errors.As(err, &me) {
		t.Fatalf("want malformed, got %v", err)
	}
}

func TestStyleCycleIsMalformed(t *testing.T) {
	_, err := Parse(pkg(t, map[string]string{
		"word/document.xml": body(`<w:p><w:pPr><w:pStyle w:val="A"/></w:pPr><w:r><w:t>x</w:t></w:r></w:p>`),
		"word/styles.xml": `<w:styles ` + wNS + `>
			<w:style w:styleId="A"><w:basedOn w:val="B"/></w:style>
			<w:style w:styleId="B"><w:basedOn w:val="A"/></w:style></w:styles>`,
	}))
	var me *opc.MalformedError
	if !errors.As(err, &me) {
		t.Fatalf("want malformed, got %v", err)
	}
}

func TestRunEdgeWhitespaceAndBreaks(t *testing.T) {
	md := markdown(t, map[string]string{"word/document.xml": body(
		`<w:p><w:r><w:t>This</w:t></w:r><w:r><w:t> by-law</w:t></w:r><w:r><w:t> grants</w:t></w:r></w:p>` +
			`<w:p><w:r><w:t>Alfa</w:t><w:br w:type="page"/><w:t>Beta</w:t></w:r></w:p>` +
			`<w:p><w:r><w:t>Gamma</w:t><w:br w:type="page"/></w:r></w:p>`)})
	if md != "This by-law grants\n\nAlfa\\\nBeta\n\nGamma\n" {
		t.Fatalf("got %q", md)
	}
}

func TestNumberedHeadingKeepsItsNumberWithoutDoubleEmphasis(t *testing.T) {
	heading := func(text string) string {
		return `<w:p><w:pPr><w:pStyle w:val="H1"/></w:pPr><w:r><w:rPr><w:b/></w:rPr><w:t>` + text + `</w:t></w:r></w:p>`
	}
	md := markdown(t, map[string]string{
		"word/document.xml": body(heading("Intro") + heading("Details")),
		"word/styles.xml": `<w:styles ` + wNS + `><w:style w:type="paragraph" w:styleId="H1"><w:name w:val="heading 1"/>
			<w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr><w:rPr><w:b/></w:rPr></w:style></w:styles>`,
		"word/numbering.xml": numbering(`<w:lvl w:ilvl="0"><w:numFmt w:val="decimal"/><w:lvlText w:val="%1."/><w:pStyle w:val="H1"/></w:lvl>`),
	})
	if md != "# 1. Intro\n\n# 2. Details\n" {
		t.Fatalf("got %q", md)
	}
}

func TestHyperlinkTargets(t *testing.T) {
	md := markdown(t, map[string]string{
		"word/document.xml": body(`<w:p><w:bookmarkStart w:name="sec"/><w:r><w:t>Target</w:t></w:r></w:p>
			<w:p><w:hyperlink r:id="rId1"><w:r><w:t>ext</w:t></w:r></w:hyperlink>
			<w:hyperlink r:id="rId2"><w:r><w:t>rel</w:t></w:r></w:hyperlink>
			<w:hyperlink w:anchor="sec"><w:r><w:t>anchor</w:t></w:r></w:hyperlink>
			<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText> HYPERLINK \l "sec" </w:instrText></w:r>
			<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>field</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r>
			<w:fldSimple w:instr="HYPERLINK &quot;https://e.com/s&quot;"><w:r><w:t>simple</w:t></w:r></w:fldSimple></w:p>`),
		"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
			<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://e.com/a" TargetMode="External"/>
			<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="../other.docx" TargetMode="External"/>
		</Relationships>`,
	})
	want := "<a id=\"sec\"></a>Target\n\n[ext](https://e.com/a)[rel](../other.docx)[anchor](#sec)[field](#sec)[simple](https://e.com/s)\n"
	if md != want {
		t.Fatalf("got %q\nwant %q", md, want)
	}
}

func TestHyperlinkFieldInstructions(t *testing.T) {
	cases := []struct {
		instr string
		kind  model.TargetKind
		ref   string
	}{
		{` HYPERLINK "https://e.com/a b" `, model.TargetExternal, "https://e.com/a b"},
		{`hyperlink "https://e.com"`, model.TargetExternal, "https://e.com"},
		{`HYPERLINK \l "sec2"`, model.TargetAnchor, "sec2"},
		{`HYPERLINK "https://e.com/p" \l "frag"`, model.TargetExternal, "https://e.com/p#frag"},
		{`HYPERLINK \o "tooltip text" "https://e.com"`, model.TargetExternal, "https://e.com"},
		{`HYPERLINK "https://e.com/\"q\""`, model.TargetExternal, `https://e.com/"q"`},
		{`HYPERLINK "docs/readme.docx"`, model.TargetRelative, "docs/readme.docx"},
		{`HYPERLINK "C:\docs\a.doc"`, model.TargetRelative, `C:\docs\a.doc`},
		{`HYPERLINK "https://e.com/p" \o \l "frag"`, model.TargetExternal, "https://e.com/p#frag"},
	}
	for _, c := range cases {
		got, ok := hyperlinkTarget(c.instr)
		if !ok || got.Kind != c.kind || got.Ref != c.ref {
			t.Errorf("hyperlinkTarget(%q) = %+v, %v", c.instr, got, ok)
		}
	}
	if _, ok := hyperlinkTarget(`PAGEREF _Toc123 \h`); ok {
		t.Error("non-hyperlink fields are not links")
	}
}

func TestNotesRenumberByFirstReference(t *testing.T) {
	notes := func(root, elem string) string {
		return `<w:` + root + ` ` + wNS + `><w:` + elem + ` w:type="separator" w:id="-1"><w:p/></w:` + elem + `>
			<w:` + elem + ` w:id="1"><w:p><w:r><w:t>first ` + elem + `</w:t></w:r></w:p></w:` + elem + `>
			<w:` + elem + ` w:id="2"><w:p><w:r><w:t>second ` + elem + `</w:t></w:r></w:p></w:` + elem + `></w:` + root + `>`
	}
	md := markdown(t, map[string]string{
		"word/document.xml":  body(`<w:p><w:r><w:t>a</w:t><w:endnoteReference w:id="1"/><w:t>b</w:t><w:footnoteReference w:id="2"/><w:t>c</w:t><w:footnoteReference w:id="2"/></w:r></w:p>`),
		"word/footnotes.xml": notes("footnotes", "footnote"),
		"word/endnotes.xml":  notes("endnotes", "endnote"),
	})
	want := "a[^1]b[^2]c[^2]\n\n[^1]: first endnote\n\n[^2]: second footnote\n\n[^3]: first footnote\n\n[^4]: second endnote\n"
	if md != want {
		t.Fatalf("got %q\nwant %q", md, want)
	}
}

func TestMissingMainPartIsMissingPart(t *testing.T) {
	_, err := Parse(pkg(t, map[string]string{"word/styles.xml": "<x/>"}))
	var missing *opc.MissingPartError
	if !errors.As(err, &missing) || missing.Part != "word/document.xml" {
		t.Fatalf("want missing part, got %v", err)
	}
	_, err = Parse(pkg(t, map[string]string{"word/document.xml": `<w:document ` + wNS + `/>`}))
	var me *opc.MalformedError
	if !errors.As(err, &me) {
		t.Fatalf("want malformed without a body, got %v", err)
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestAssetsAreRetainedOncePerPart(t *testing.T) {
	doc, err := Parse(fixture(t, "handmade-manyrefs.docx"))
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Assets) != 1 {
		t.Fatalf("seventy references share one asset, got %d", len(doc.Assets))
	}
	doc, err = Parse(fixture(t, "handmade-ole.docx"))
	if err != nil {
		t.Fatal(err)
	}
	var ole *model.Asset
	for i := range doc.Assets {
		if doc.Assets[i].MediaType == "application/vnd.ms-ole-object" {
			ole = &doc.Assets[i]
		}
	}
	if ole == nil || !bytes.Equal(ole.Bytes, bytes.Repeat([]byte("DOCX-OLE-PAYLOAD"), 4)) {
		t.Fatalf("OLE payload must win over its preview image, assets %+v", doc.Assets)
	}
}

func TestAssetCapIsFatal(t *testing.T) {
	sink := &assetSink{byPart: map[string]model.AssetID{}, total: maxAssetTotalBytes - 10}
	for range 3 {
		if _, err := sink.add("image/png", "media/a.png", make([]byte, 8)); err != nil {
			t.Fatal(err)
		}
	}
	_, err := sink.add("image/png", "media/b.png", make([]byte, 8))
	var limit *model.LimitError
	if !errors.As(err, &limit) || limit.Limit != "max_asset_total_bytes" {
		t.Fatalf("want asset cap, got %v", err)
	}
}

func TestListAssemblySplitsAndNests(t *testing.T) {
	entry := func(level, inst int, marker model.MarkerKind, number int) listEntry {
		return listEntry{level: level, key: listKey{inst, marker}, number: number, blocks: []model.Block{model.Paragraph{model.Text{Text: "x"}}}}
	}
	lists := func(entries ...listEntry) []model.Block { return buildLists(entries) }
	if got := lists(entry(0, 1, model.Decimal, 1), entry(0, 1, model.Decimal, 2)); len(got) != 1 {
		t.Fatalf("contiguous numbers stay one list: %d", len(got))
	}
	if got := lists(entry(0, 1, model.Decimal, 1), entry(0, 1, model.Decimal, 10)); len(got) != 2 || got[1].(model.List).Start != 10 {
		t.Fatalf("restart splits with the new start: %+v", got)
	}
	if got := lists(entry(0, 1, model.Decimal, 1), entry(0, 2, model.Decimal, 1)); len(got) != 2 {
		t.Fatal("distinct instances split")
	}
	got := lists(entry(1, 1, model.Bullet, 0), entry(0, 1, model.Decimal, 1), entry(1, 1, model.LowerRoman, 1), entry(0, 1, model.Decimal, 2))
	if len(got) != 2 {
		t.Fatalf("orphan sub-level hosts in an anonymous item: %+v", got)
	}
	outer := got[1].(model.List)
	if len(outer.Items) != 2 || outer.Items[0].Blocks[1].(model.List).Marker != model.LowerRoman {
		t.Fatalf("nesting preserved: %+v", outer)
	}
}

func TestAttachmentsKeepSourceOrder(t *testing.T) {
	textBox := `<w:r><w:pict><w:txbxContent><w:p><w:r><w:t>boxed</w:t></w:r></w:p></w:txbxContent></w:pict></w:r>`
	md := markdown(t, map[string]string{
		"word/document.xml": body(`<w:p><w:pPr><w:pStyle w:val="Code"/></w:pPr><w:r><w:t>before</w:t></w:r>` + textBox + `<w:r><w:t>after</w:t></w:r></w:p>`),
		"word/styles.xml":   `<w:styles ` + wNS + `><w:style w:styleId="Code"><w:name w:val="Source Code"/></w:style></w:styles>`,
	})
	if md != "```\nbefore\n```\n\nboxed\n\n```\nafter\n```\n" {
		t.Fatalf("got %q", md)
	}
}

func allocatedBytes(f func()) uint64 {
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	f()
	runtime.ReadMemStats(&after)
	return after.TotalAlloc - before.TotalAlloc
}

func TestSplitFieldInstructionAccumulatesLinearly(t *testing.T) {
	const pieces = 20_000
	url := "https://e.com/" + strings.Repeat("a", pieces)
	field := func(instr string) string {
		return `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r>` + instr +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>link</w:t></w:r>` +
			`<w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`
	}
	whole := `<w:r><w:instrText>HYPERLINK "` + url + `"</w:instrText></w:r>`
	var split strings.Builder
	split.WriteString(`<w:r><w:instrText>HYPERLINK "https://e.com/</w:instrText>`)
	for range pieces {
		split.WriteString(`<w:instrText>a</w:instrText>`)
	}
	split.WriteString(`<w:instrText>"</w:instrText></w:r>`)

	want := markdown(t, map[string]string{"word/document.xml": body(field(whole))})
	if !strings.Contains(want, url) {
		t.Fatalf("unsplit field is not a link: %.80q", want)
	}
	data := pkg(t, map[string]string{"word/document.xml": body(field(split.String()))})
	var doc *model.Document
	var err error
	allocated := allocatedBytes(func() { doc, err = Parse(data) })
	if err != nil {
		t.Fatal(err)
	}
	if got := gfm.Render(doc); got != want {
		t.Fatalf("split field differs from unsplit:\ngot  %.120q\nwant %.120q", got, want)
	}
	if allocated > 64<<20 {
		t.Fatalf("allocated %d bytes for %d instruction pieces", allocated, pieces)
	}
}

func TestListLevelClampsToDefinedDepth(t *testing.T) {
	para := func(ilvl int) string {
		return `<w:p><w:pPr><w:numPr><w:ilvl w:val="` + strconv.Itoa(ilvl) + `"/><w:numId w:val="1"/></w:numPr></w:pPr><w:r><w:t>item</w:t></w:r></w:p>`
	}
	levels := numbering(`<w:lvl w:ilvl="0"><w:numFmt w:val="bullet"/></w:lvl>`)
	var deepest strings.Builder
	for ilvl := range numLevels {
		deepest.WriteString(para(ilvl))
	}
	capped := maxIndent(markdown(t, map[string]string{"word/document.xml": body(deepest.String()), "word/numbering.xml": levels}))

	var hostile strings.Builder
	for ilvl := range 4001 {
		hostile.WriteString(para(ilvl))
	}
	data := pkg(t, map[string]string{"word/document.xml": body(hostile.String()), "word/numbering.xml": levels})
	var md string
	allocated := allocatedBytes(func() {
		doc, err := Parse(data)
		if err != nil {
			t.Fatal(err)
		}
		md = gfm.Render(doc)
	})
	if got := maxIndent(md); got != capped {
		t.Fatalf("deepest indent %d, want the level-%d indent %d", got, numLevels-1, capped)
	}
	if got := strings.Count(md, "item"); got != 4001 {
		t.Fatalf("rendered %d items, want 4001", got)
	}
	if allocated > 64<<20 {
		t.Fatalf("allocated %d bytes for 4001 list levels", allocated)
	}
}

func maxIndent(md string) int {
	deepest := 0
	for line := range strings.Lines(md) {
		deepest = max(deepest, len(line)-len(strings.TrimLeft(line, " ")))
	}
	return deepest
}

func TestWideVMergeContinuationIsClamped(t *testing.T) {
	const rows = 4000
	var tbl strings.Builder
	tbl.WriteString(`<w:tbl><w:tr><w:tc><w:tcPr><w:vMerge w:val="restart"/></w:tcPr><w:p><w:r><w:t>origin</w:t></w:r></w:p></w:tc></w:tr>`)
	for range rows {
		tbl.WriteString(`<w:tr><w:tc><w:tcPr><w:gridSpan w:val="1000"/><w:vMerge/></w:tcPr><w:p/></w:tc></w:tr>`)
	}
	tbl.WriteString(`</w:tbl>`)
	data := pkg(t, map[string]string{"word/document.xml": body(tbl.String())})
	var doc *model.Document
	var err error
	allocated := allocatedBytes(func() { doc, err = Parse(data) })
	if err != nil {
		t.Fatal(err)
	}
	table := doc.Blocks[0].(model.Table)
	grid := table.Grid()
	for r, row := range grid {
		if len(row) != 1 {
			t.Fatalf("row %d has %d slots, want 1", r, len(row))
		}
	}
	if origin := grid[0][0]; origin.Covered || origin.Cell.RowSpan != len(grid) {
		t.Fatalf("origin %+v, want a row span of %d", origin, len(grid))
	}
	if allocated > 32<<20 {
		t.Fatalf("allocated %d bytes for %d continuation rows", allocated, rows)
	}
}

func TestDeepStyleChainResolvesOnce(t *testing.T) {
	const depth, runs = 2000, 2000
	var styles strings.Builder
	styles.WriteString(`<w:styles ` + wNS + `>`)
	for i := range depth {
		styles.WriteString(`<w:style w:styleId="S` + strconv.Itoa(i) + `">`)
		if i+1 < depth {
			styles.WriteString(`<w:basedOn w:val="S` + strconv.Itoa(i+1) + `"/>`)
		} else {
			styles.WriteString(`<w:rPr><w:b/></w:rPr>`)
		}
		styles.WriteString(`</w:style>`)
	}
	styles.WriteString(`</w:styles>`)
	var paras strings.Builder
	for range runs {
		paras.WriteString(`<w:p><w:pPr><w:pStyle w:val="S1"/></w:pPr><w:r><w:rPr><w:rStyle w:val="S0"/></w:rPr><w:t>x</w:t></w:r></w:p>`)
	}
	data := pkg(t, map[string]string{"word/document.xml": body(paras.String()), "word/styles.xml": styles.String()})
	var doc *model.Document
	var err error
	allocated := allocatedBytes(func() { doc, err = Parse(data) })
	if err != nil {
		t.Fatal(err)
	}
	if got := gfm.Render(doc); !strings.HasPrefix(got, "x\n") {
		t.Fatalf("paragraph bold and run bold toggle off, got %.40q", got)
	}
	if allocated > 64<<20 {
		t.Fatalf("allocated %d bytes for %d runs over a %d-style chain", allocated, runs, depth)
	}
}

func TestStyleResolutionFollowsChain(t *testing.T) {
	styles := `<w:styles ` + wNS + `>
		<w:style w:styleId="Base"><w:name w:val="Quote"/><w:rPr><w:b/><w:i/></w:rPr></w:style>
		<w:style w:styleId="Mid"><w:basedOn w:val="Base"/><w:pPr><w:outlineLvl w:val="1"/></w:pPr><w:rPr><w:b/></w:rPr></w:style>
		<w:style w:styleId="Leaf"><w:basedOn w:val="Mid"/><w:name w:val="Leafy"/></w:style>
		<w:style w:styleId="Dangling"><w:basedOn w:val="Nowhere"/><w:rPr><w:strike/></w:rPr></w:style></w:styles>`
	root, err := opc.ParseXML([]byte(styles))
	if err != nil {
		t.Fatal(err)
	}
	s := parseStyles(root)
	cases := []struct {
		id      string
		toggles toggles
		heading int
		found   bool
		kind    blockKind
	}{
		{"Leaf", toggles{italic: true}, 2, true, quoteBlock},
		{"Mid", toggles{italic: true}, 2, true, quoteBlock},
		{"Base", toggles{bold: true, italic: true}, 0, false, quoteBlock},
		{"Dangling", toggles{strike: true}, 0, false, noBlock},
		{"Unknown", toggles{}, 0, false, noBlock},
	}
	for _, c := range cases {
		t.Run(c.id, func(t *testing.T) {
			tg, err := s.runToggles(c.id)
			if err != nil || tg != c.toggles {
				t.Errorf("toggles %+v %v, want %+v", tg, err, c.toggles)
			}
			level, found, err := s.headingLevel(c.id)
			if err != nil || level != c.heading || found != c.found {
				t.Errorf("heading %d %v %v, want %d %v", level, found, err, c.heading, c.found)
			}
			if kind, err := s.blockStyle(c.id); err != nil || kind != c.kind {
				t.Errorf("block kind %v %v, want %v", kind, err, c.kind)
			}
		})
	}
}

func TestStyleCycleNamesFirstRevisitedStyle(t *testing.T) {
	root, err := opc.ParseXML([]byte(`<w:styles ` + wNS + `>
		<w:style w:styleId="Entry"><w:basedOn w:val="A"/></w:style>
		<w:style w:styleId="A"><w:basedOn w:val="B"/></w:style>
		<w:style w:styleId="B"><w:basedOn w:val="A"/></w:style></w:styles>`))
	if err != nil {
		t.Fatal(err)
	}
	s := parseStyles(root)
	for _, c := range []struct{ id, at string }{{"Entry", "A"}, {"A", "A"}, {"B", "B"}} {
		_, err := s.runToggles(c.id)
		var me *opc.MalformedError
		if !errors.As(err, &me) || !strings.Contains(me.Detail, strconv.Quote(c.at)) {
			t.Errorf("%s: got %v, want a cycle at %q", c.id, err, c.at)
		}
	}
}

func TestChartReferencedManyTimesParsesOnce(t *testing.T) {
	var chart strings.Builder
	chart.WriteString(`<c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart"><c:chart><c:plotArea><c:barChart><c:ser><c:tx><c:v>S</c:v></c:tx><c:cat><c:strCache>`)
	for i := range 5000 {
		chart.WriteString(`<c:pt idx="` + strconv.Itoa(i) + `"><c:v>c` + strconv.Itoa(i) + `</c:v></c:pt>`)
	}
	chart.WriteString(`</c:strCache></c:cat><c:val><c:numCache><c:pt idx="0"><c:v>1</c:v></c:pt></c:numCache></c:val></c:ser></c:barChart></c:plotArea></c:chart></c:chartSpace>`)
	ref := `<w:p><w:r><w:drawing><c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rId1"/></w:drawing></w:r></w:p>`
	docWith := func(refs int) []byte {
		return pkg(t, map[string]string{
			"word/document.xml": body(strings.Repeat(ref, refs)),
			"word/_rels/document.xml.rels": `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
				<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chart" Target="charts/chart1.xml"/></Relationships>`,
			"word/charts/chart1.xml": chart.String(),
		})
	}
	parseCounting := func(data []byte) (*model.Document, uint64) {
		var doc *model.Document
		var err error
		allocated := allocatedBytes(func() { doc, err = Parse(data) })
		if err != nil {
			t.Fatal(err)
		}
		return doc, allocated
	}
	one, oneAlloc := parseCounting(docWith(1))
	const refs = 100
	many, manyAlloc := parseCounting(docWith(refs))
	if len(one.Blocks) != 1 || len(many.Blocks) != refs {
		t.Fatalf("got %d and %d blocks, want 1 and %d", len(one.Blocks), len(many.Blocks), refs)
	}
	if manyAlloc > 2*oneAlloc {
		t.Fatalf("%d references allocated %d bytes, one allocated %d: the chart is parsed per reference", refs, manyAlloc, oneAlloc)
	}
}

func TestStyleNumberingLevelFindsNearestBoundStyle(t *testing.T) {
	root, err := opc.ParseXML([]byte(`<w:styles ` + wNS + `>
		<w:style w:styleId="Top"/>
		<w:style w:styleId="Bound"><w:basedOn w:val="Top"/></w:style>
		<w:style w:styleId="Leaf"><w:basedOn w:val="Bound"/></w:style>
		<w:style w:styleId="Other"><w:basedOn w:val="Top"/></w:style></w:styles>`))
	if err != nil {
		t.Fatal(err)
	}
	s := parseStyles(root)
	inst := &instance{}
	inst.pstyles[3] = "Bound"
	cases := []struct {
		id    string
		level int
		found bool
	}{
		{"Leaf", 3, true},
		{"Bound", 3, true},
		{"Other", 0, false},
		{"Top", 0, false},
		{"Leaf", 3, true},
		{"Unknown", 0, false},
	}
	for _, c := range cases {
		level, found, err := s.styleNumberingLevel(c.id, inst)
		if err != nil || level != c.level || found != c.found {
			t.Errorf("%s: got %d %v %v, want %d %v", c.id, level, found, err, c.level, c.found)
		}
	}
}

package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
	"github.com/kvit-s/kvit-coder/internal/office/render/gfm"
)

const (
	relsNS       = "http://schemas.openxmlformats.org/package/2006/relationships"
	relWorksheet = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"
)

type sheetSpec struct{ name, state, body string }

type workbook struct {
	sheets   []sheetSpec
	styles   string
	shared   string
	date1904 bool
	extra    map[string]string
}

func oneSheet(body string) workbook {
	return workbook{sheets: []sheetSpec{{"S", "", body}}}
}

func (w workbook) build(t *testing.T) []byte {
	t.Helper()
	var sheets, rels strings.Builder
	for i, s := range w.sheets {
		state := ""
		if s.state != "" {
			state = fmt.Sprintf(` state="%s"`, s.state)
		}
		fmt.Fprintf(&sheets, `<sheet name="%s" sheetId="%d"%s r:id="rId%d"/>`, s.name, i+1, state, i+1)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="%s" Target="worksheets/sheet%d.xml"/>`, i+1, relWorksheet, i+1)
	}
	if w.styles != "" {
		fmt.Fprintf(&rels, `<Relationship Id="rId90" Type="%s" Target="styles.xml"/>`, opc.RelStyles)
	}
	if w.shared != "" {
		fmt.Fprintf(&rels, `<Relationship Id="rId91" Type="%s" Target="sharedStrings.xml"/>`, opc.RelSharedStrings)
	}
	pr := ""
	if w.date1904 {
		pr = `<workbookPr date1904="1"/>`
	}
	parts := map[string]string{
		"xl/workbook.xml": fmt.Sprintf(`<?xml version="1.0"?><workbook xmlns="%s" xmlns:r="%s">%s<sheets>%s</sheets></workbook>`,
			NSSpreadsheetML, opc.NSRelationships, pr, sheets.String()),
		"xl/_rels/workbook.xml.rels": fmt.Sprintf(`<?xml version="1.0"?><Relationships xmlns="%s">%s</Relationships>`, relsNS, rels.String()),
	}
	for i, s := range w.sheets {
		parts[fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)] = fmt.Sprintf(`<?xml version="1.0"?><worksheet xmlns="%s">%s</worksheet>`, NSSpreadsheetML, s.body)
	}
	if w.styles != "" {
		parts["xl/styles.xml"] = fmt.Sprintf(`<?xml version="1.0"?><styleSheet xmlns="%s">%s</styleSheet>`, NSSpreadsheetML, w.styles)
	}
	if w.shared != "" {
		parts["xl/sharedStrings.xml"] = fmt.Sprintf(`<?xml version="1.0"?><sst xmlns="%s">%s</sst>`, NSSpreadsheetML, w.shared)
	}
	for name, body := range w.extra {
		parts[name] = body
	}
	return zipParts(t, parts)
}

func zipParts(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, parts[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func parse(t *testing.T, w workbook) *model.Document {
	t.Helper()
	doc, err := Parse(w.build(t))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	return doc
}

func firstTable(t *testing.T, doc *model.Document) model.Table {
	t.Helper()
	for _, b := range doc.Blocks {
		if table, ok := b.(model.Table); ok {
			return table
		}
	}
	t.Fatalf("expected a table, got %#v", doc.Blocks)
	return model.Table{}
}

func texts(table model.Table) [][]string {
	var out [][]string
	for _, row := range table.Grid() {
		var cells []string
		for _, s := range row {
			if s.Covered {
				cells = append(cells, "<covered>")
				continue
			}
			var sb strings.Builder
			for _, b := range s.Cell.Blocks {
				if p, ok := b.(model.Paragraph); ok {
					sb.WriteString(model.PlainText(p))
				}
			}
			cells = append(cells, sb.String())
		}
		out = append(out, cells)
	}
	return out
}

func assertTexts(t *testing.T, table model.Table, want [][]string) {
	t.Helper()
	if got := texts(table); fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("cells = %q, want %q", got, want)
	}
}

func coveredCount(table model.Table) int {
	n := 0
	for _, row := range table.Grid() {
		for _, s := range row {
			if s.Covered {
				n++
			}
		}
	}
	return n
}

func inline(ref, text string) string {
	return fmt.Sprintf(`<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, text)
}

func TestNumberFormatsApplyToStoredValues(t *testing.T) {
	w := oneSheet(`<sheetData><row r="1"><c r="A1" s="1"><v>0.075</v></c><c r="B1" s="2"><v>1234.5</v></c><c r="C1" s="3"><v>46096</v></c></row></sheetData>`)
	w.styles = `<numFmts><numFmt numFmtId="164" formatCode="0.0%"/><numFmt numFmtId="165" formatCode="&quot;$&quot;#,##0.00"/><numFmt numFmtId="166" formatCode="mm/dd/yyyy"/></numFmts><cellXfs><xf numFmtId="0"/><xf numFmtId="164"/><xf numFmtId="165"/><xf numFmtId="166"/></cellXfs>`
	assertTexts(t, firstTable(t, parse(t, w)), [][]string{{"7.5%", "$1,234.50", "2026-03-15"}})
}

func TestUnresolvableNumFmtIDsRenderGeneral(t *testing.T) {
	w := oneSheet(`<sheetData><row r="1"><c r="A1" s="0"><v>1234.5</v></c><c r="B1" s="1"><v>1234.5</v></c></row></sheetData>`)
	w.styles = `<cellXfs><xf numFmtId="5"/><xf numFmtId="30"/></cellXfs>`
	assertTexts(t, firstTable(t, parse(t, w)), [][]string{{"1234.5", "1234.5"}})
}

func TestValueTypesRenderByTheirTAttribute(t *testing.T) {
	w := oneSheet(`<sheetData><row r="1"><c r="A1" t="b"><v>1</v></c><c r="B1" t="e"><v>#DIV/0!</v></c><c r="C1" t="str"><v>=sum</v></c><c r="D1" t="d"><v>2026-03-15</v></c><c r="E1" t="inlineStr"><is><r><t>in</t></r><r><t>line</t></r></is></c><c r="F1" t="e"><v>#N/A</v></c><c r="G1"><v>3554.7000000000003</v></c></row></sheetData>`)
	assertTexts(t, firstTable(t, parse(t, w)), [][]string{{"TRUE", "#DIV/0!", "=sum", "2026-03-15", "inline", "#N/A", "3554.7"}})
}

func TestSharedStringsResolveIncludingRichTextRuns(t *testing.T) {
	w := oneSheet(`<sheetData><row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c><c r="C1" t="s"><v>9</v></c></row></sheetData>`)
	w.shared = `<si><t>plain</t></si><si><r><t>ri</t></r><r><t>ch</t></r><rPh sb="0" eb="1"><t>ignored</t></rPh></si>`
	assertTexts(t, firstTable(t, parse(t, w)), [][]string{{"plain", "rich"}})
}

func TestDate1904SerialsShiftEpoch(t *testing.T) {
	w := oneSheet(`<sheetData><row r="1"><c r="A1" s="0"><v>100</v></c></row></sheetData>`)
	w.styles = `<numFmts><numFmt numFmtId="164" formatCode="yyyy-mm-dd"/></numFmts><cellXfs><xf numFmtId="164"/></cellXfs>`
	w.date1904 = true
	assertTexts(t, firstTable(t, parse(t, w)), [][]string{{"1904-04-10"}})
}

func TestDateTimeAndDurationFormats(t *testing.T) {
	w := oneSheet(`<sheetData><row r="1"><c r="A1" s="1"><v>46096.5</v></c><c r="B1" s="2"><v>1.1043402777777778</v></c><c r="C1" s="3"><v>0.3784027777777778</v></c><c r="D1" s="4"><v>46096.75</v></c><c r="E1" s="5"><v>46096</v></c></row></sheetData>`)
	w.styles = `<numFmts><numFmt numFmtId="164" formatCode="yyyy-mm-dd"/></numFmts><cellXfs><xf numFmtId="0"/><xf numFmtId="164"/><xf numFmtId="46"/><xf numFmtId="21"/><xf numFmtId="22"/><xf numFmtId="14"/></cellXfs>`
	assertTexts(t, firstTable(t, parse(t, w)), [][]string{{"2026-03-15", "26:30:15", "09:04:54", "2026-03-15 18:00:00", "2026-03-15"}})
}

func TestMergeExtendsPastThePopulatedRange(t *testing.T) {
	table := firstTable(t, parse(t, oneSheet(`<sheetData><row r="1">`+inline("F1", "wide")+`</row></sheetData><mergeCells count="1"><mergeCell ref="F1:O3"/></mergeCells>`)))
	grid := table.Grid()
	if len(grid) != 3 || len(grid[1]) != 10 {
		t.Fatalf("grid %dx%d, want 3x10", len(grid), len(grid[1]))
	}
	if c := grid[0][0].Cell; grid[0][0].Covered || c.ColSpan != 10 || c.RowSpan != 3 {
		t.Errorf("origin = %+v", grid[0][0])
	}
	if n := coveredCount(table); n != 29 {
		t.Errorf("covered = %d, want 29", n)
	}
}

func sheetWithMerge(ref string) workbook {
	return oneSheet(`<sheetData><row r="11">` + inline("D11", "x") + inline("E11", "y") + `</row><row r="12">` + inline("D12", "z") + inline("E12", "w") + `</row></sheetData><mergeCells count="1"><mergeCell ref="` + ref + `"/></mergeCells>`)
}

func TestMergesAgainstTheUsedRange(t *testing.T) {
	if n := coveredCount(firstTable(t, parse(t, sheetWithMerge("D11:E11")))); n != 1 {
		t.Errorf("inside merge covered = %d, want 1", n)
	}
	table := firstTable(t, parse(t, sheetWithMerge("A1:B12")))
	if n := coveredCount(table); n != 0 || len(table.Grid()[0]) != 2 {
		t.Errorf("outside merge: covered %d, width %d", n, len(table.Grid()[0]))
	}
}

func TestHiddenRowsColumnsAndSheetsAreOmitted(t *testing.T) {
	visible := `<cols><col min="2" max="2" hidden="1"/></cols><sheetData><row r="1">` + inline("A1", "a") + inline("B1", "hidden col") + inline("C1", "c") + `</row><row r="2" hidden="1">` + inline("A2", "hidden row") + `</row><row r="3">` + inline("A3", "d") + `</row></sheetData>`
	secret := `<sheetData><row r="1">` + inline("A1", "secret") + `</row></sheetData>`
	doc := parse(t, workbook{sheets: []sheetSpec{{"Shown", "", visible}, {"Secret", "hidden", secret}, {"Deep", "veryHidden", secret}}})
	if len(doc.Blocks) != 1 {
		t.Fatalf("blocks = %d, want only the visible sheet's table", len(doc.Blocks))
	}
	assertTexts(t, firstTable(t, doc), [][]string{{"a", "c"}, {"d", ""}})
}

func TestMergesRemapAcrossHiddenColumns(t *testing.T) {
	table := firstTable(t, parse(t, oneSheet(`<cols><col min="2" max="2" hidden="1"/></cols><sheetData><row r="1">`+inline("A1", "m")+inline("D1", "x")+`</row></sheetData><mergeCells count="1"><mergeCell ref="A1:C1"/></mergeCells>`)))
	if c := table.Grid()[0][0].Cell; c.ColSpan != 2 || c.RowSpan != 1 || len(table.Grid()[0]) != 3 {
		t.Errorf("origin %+v, width %d", c, len(table.Grid()[0]))
	}
}

func TestMergeOriginInAHiddenRowKeepsItsContent(t *testing.T) {
	table := firstTable(t, parse(t, oneSheet(`<sheetData><row r="1" hidden="1">`+inline("A1", "kept")+`</row><row r="2"/><row r="3">`+inline("B3", "x")+`</row></sheetData><mergeCells count="1"><mergeCell ref="A1:A3"/></mergeCells>`)))
	if c := table.Grid()[0][0].Cell; c.RowSpan != 2 || texts(table)[0][0] != "kept" {
		t.Errorf("origin %+v, text %q", c, texts(table)[0][0])
	}
}

func TestGridBudgetSpansTheWholeWorkbook(t *testing.T) {
	var limit *model.LimitError
	corners := oneSheet(`<sheetData><row r="1">` + inline("A1", "a") + `</row><row r="1048576">` + inline("XFD1048576", "b") + `</row></sheetData>`)
	if _, err := Parse(corners.build(t)); !errors.As(err, &limit) || limit.Limit != "max_grid_slots" {
		t.Errorf("corners: %v", err)
	}
	half := `<sheetData><row r="1">` + inline("A1", "a") + `</row><row r="150000">` + inline("P150000", "b") + `</row></sheetData>`
	if _, err := Parse(workbook{sheets: []sheetSpec{{"A", "", half}, {"B", "", half}}}.build(t)); !errors.As(err, &limit) || limit.Limit != "max_grid_slots" {
		t.Errorf("accumulated: %v", err)
	}
	if _, err := Parse(workbook{sheets: []sheetSpec{{"A", "", half}}}.build(t)); err != nil {
		t.Errorf("one sheet under the cap: %v", err)
	}
}

func TestSheetNamesCannotForgeHeadings(t *testing.T) {
	body := `<sheetData><row r="1">` + inline("A1", "v") + `</row></sheetData>`
	w := workbook{sheets: []sheetSpec{{"A&#10;# Injected&#13;&#10;[x](http://evil)", "", body}, {"B", "", body}}}
	md := gfm.Render(parse(t, w))
	var headings []string
	for line := range strings.SplitSeq(md, "\n") {
		if strings.HasPrefix(line, "#") {
			headings = append(headings, line)
		}
	}
	want := []string{`## A # Injected \[x](http://evil)`, "## B"}
	if !slices.Equal(headings, want) {
		t.Errorf("headings = %q, want %q\n%s", headings, want, md)
	}
}

func TestFictitiousLeapDayKeepsItsOwnValue(t *testing.T) {
	dateOnly := dateParts{date: true}
	cases := []struct {
		serial   float64
		date1904 bool
		want     string
	}{
		{59, false, "1900-02-28"},
		{60, false, "60"},
		{61, false, "1900-03-01"},
		{59.9999999, false, "1900-02-28"},
		{60, true, "1904-03-01"},
	}
	for _, c := range cases {
		if got := renderSerial(c.serial, dateOnly, c.date1904); got != c.want {
			t.Errorf("renderSerial(%v, 1904=%v) = %q, want %q", c.serial, c.date1904, got, c.want)
		}
	}
	if got := renderSerial(0.5, dateParts{date: true, time: true}, false); got != "12:00:00" {
		t.Errorf("sub-day combined = %q", got)
	}
	if got := renderSerial(0.5, dateOnly, false); got != "0.5" {
		t.Errorf("sub-day date-only = %q", got)
	}
	if got := renderSerial(3e6, dateOnly, false); got != "3000000" {
		t.Errorf("out of range = %q", got)
	}
}

const vmlDrawing = `<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:x="urn:schemas-microsoft-com:office:excel">
<v:shape id="_x0000_s1025" type="#_x0000_t201" style="position:absolute;margin-left:1pt">
  <v:textbox><div style="text-align:left"><font face="Tahoma">Roof</font></div></v:textbox>
  <x:ClientData ObjectType="Checkbox"><x:Anchor>1, 5, 0, 2, 2, 10, 1, 1</x:Anchor><x:Checked>1</x:Checked></x:ClientData>
</v:shape>
<v:shape id="_x0000_s1026" type="#_x0000_t201" style="position:absolute">
  <v:textbox><div><font></font></div></v:textbox>
  <x:ClientData ObjectType="Checkbox"><x:Anchor>2, 5, 0, 2, 3, 10, 1, 1</x:Anchor></x:ClientData>
</v:shape>
<v:shape id="_x0000_s1027" type="#_x0000_t201" style="position:absolute;visibility:hidden">
  <x:ClientData ObjectType="Checkbox"><x:Anchor>3, 5, 0, 2, 4, 10, 1, 1</x:Anchor><x:Checked>1</x:Checked></x:ClientData>
</v:shape>
<v:shape id="_x0000_s1028" type="#_x0000_t202" style="position:absolute">
  <v:textbox><div>a note</div></v:textbox>
  <x:ClientData ObjectType="Note"><x:Anchor>4, 5, 0, 2, 5, 10, 1, 1</x:Anchor></x:ClientData>
</v:shape>
</xml>`

func TestFormControlCheckboxesLandInTheirAnchorCell(t *testing.T) {
	w := oneSheet(`<sheetData><row r="1"><c r="A1" t="str"><v>14</v></c><c r="B1" t="str"><v>L/R</v></c></row></sheetData><legacyDrawing r:id="rId1"/>`)
	w.extra = map[string]string{
		"xl/worksheets/_rels/sheet1.xml.rels": fmt.Sprintf(`<?xml version="1.0"?><Relationships xmlns="%s"><Relationship Id="rId1" Type="%s" Target="../drawings/vmlDrawing1.vml"/></Relationships>`, relsNS, relVMLDrawing),
		"xl/drawings/vmlDrawing1.vml":         vmlDrawing,
	}
	assertTexts(t, firstTable(t, parse(t, w)), [][]string{{"14", "L/R [x] Roof", "[ ]"}})
}

func TestNonWorkbookPackagesAreMalformed(t *testing.T) {
	document := zipParts(t, map[string]string{
		"_rels/.rels":       fmt.Sprintf(`<?xml version="1.0"?><Relationships xmlns="%s"><Relationship Id="rId1" Type="%s" Target="word/document.xml"/></Relationships>`, relsNS, opc.RelOfficeDocument),
		"word/document.xml": `<?xml version="1.0"?><w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body/></w:document>`,
	})
	binary := zipParts(t, map[string]string{"xl/workbook.bin": "\x83\x01\x00"})
	for name, data := range map[string][]byte{"document": document, "binary": binary, "not a zip": []byte("plain text"), "empty package": zipParts(t, map[string]string{"a.txt": "x"})} {
		var bad *opc.MalformedError
		if _, err := Parse(data); !errors.As(err, &bad) {
			t.Errorf("%s: err = %v, want malformed", name, err)
		}
	}
}

func TestUnreadableSheets(t *testing.T) {
	var bad *opc.MalformedError
	w := oneSheet("")
	w.extra = map[string]string{"xl/worksheets/sheet1.xml": "<notaworksheet/>"}
	if _, err := Parse(w.build(t)); !errors.As(err, &bad) {
		t.Errorf("only sheet unreadable: %v", err)
	}
	w = workbook{sheets: []sheetSpec{{"Bad", "", ""}, {"Good", "", `<sheetData><row r="1">` + inline("A1", "ok") + `</row></sheetData>`}}}
	w.extra = map[string]string{"xl/worksheets/sheet1.xml": "<notaworksheet/>"}
	doc := parse(t, w)
	if got := gfm.Render(doc); got != "## Good\n\n|  |\n| --- |\n| ok |\n" {
		t.Errorf("one sheet unreadable: %q", got)
	}
}

func TestSheetOrderAndTrailingTrim(t *testing.T) {
	w := workbook{sheets: []sheetSpec{
		{"Zeta", "", `<sheetData><row r="2">` + inline("B2", "first") + `<c r="C2" t="inlineStr"><is><t></t></is></c></row><row r="5"><c r="B5"/></row></sheetData>`},
		{"Empty", "", `<sheetData/>`},
		{"Alpha", "", `<sheetData><row r="1">` + inline("A1", "second") + `</row></sheetData>`},
	}}
	want := "## Zeta\n\n|  |\n| --- |\n| first |\n\n## Alpha\n\n|  |\n| --- |\n| second |\n"
	if got := gfm.Render(parse(t, w)); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestParseRef(t *testing.T) {
	cases := map[string]cellPos{"A1": {0, 0}, "c3": {2, 2}, "AA10": {9, 26}, "XFD1048576": {1_048_575, 16_383}}
	for ref, want := range cases {
		if got, ok := parseRef(ref); !ok || got != want {
			t.Errorf("parseRef(%q) = %v %v, want %v", ref, got, ok, want)
		}
	}
	for _, ref := range []string{"", "1", "A", "A0", "XFE1", "A1048577", "A1B", "É1"} {
		if _, ok := parseRef(ref); ok {
			t.Errorf("parseRef(%q) must fail", ref)
		}
	}
}

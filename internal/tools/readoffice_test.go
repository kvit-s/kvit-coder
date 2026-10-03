package tools

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

const (
	testWordNS  = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	testRelsNS  = "http://schemas.openxmlformats.org/package/2006/relationships"
	testRNS     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	testSheetNS = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	testPresNS  = "http://schemas.openxmlformats.org/presentationml/2006/main"
	testDrawNS  = "http://schemas.openxmlformats.org/drawingml/2006/main"
)

// writeOfficePackage writes a zip laid out as an Office package: the content
// types and root relationships every real one has, pointing at mainPart, plus
// the parts given. Like writeTestPDF, it builds the file in the test so the
// test data is visible where it is used.
func writeOfficePackage(t *testing.T, dir, name, mainPart string, parts map[string]string) string {
	t.Helper()
	all := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/></Types>`,
		"_rels/.rels": fmt.Sprintf(`<?xml version="1.0"?><Relationships xmlns="%s"><Relationship Id="rId1" Type="%s/officeDocument" Target="%s"/></Relationships>`,
			testRelsNS, testRNS, mainPart),
	}
	for k, v := range parts {
		all[k] = v
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(all))
	for n := range all {
		names = append(names, n)
	}
	slices.Sort(names)
	for _, n := range names {
		f, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(all[n])); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func wordBody(inner string) string {
	return `<?xml version="1.0"?><w:document ` + testWordNS + `><w:body>` + inner + `</w:body></w:document>`
}

func wordPara(text string) string {
	return `<w:p><w:r><w:t>` + text + `</w:t></w:r></w:p>`
}

func wordHeading(text string) string {
	return `<w:p><w:pPr><w:outlineLvl w:val="0"/></w:pPr><w:r><w:t>` + text + `</w:t></w:r></w:p>`
}

func wordTable(rows ...[]string) string {
	var b strings.Builder
	b.WriteString("<w:tbl>")
	for _, row := range rows {
		b.WriteString("<w:tr>")
		for _, cell := range row {
			b.WriteString("<w:tc>" + wordPara(cell) + "</w:tc>")
		}
		b.WriteString("</w:tr>")
	}
	b.WriteString("</w:tbl>")
	return b.String()
}

func writeTestDocx(t *testing.T, dir, name, inner string) string {
	t.Helper()
	return writeOfficePackage(t, dir, name, "word/document.xml", map[string]string{"word/document.xml": wordBody(inner)})
}

// writeTestXlsx writes a workbook with one sheet per entry, each holding its
// rows as inline strings.
func writeTestXlsx(t *testing.T, dir, name string, sheets map[string][][]string) string {
	t.Helper()
	sheetNames := make([]string, 0, len(sheets))
	for n := range sheets {
		sheetNames = append(sheetNames, n)
	}
	slices.Sort(sheetNames)

	var list, rels strings.Builder
	parts := map[string]string{}
	for i, sheet := range sheetNames {
		fmt.Fprintf(&list, `<sheet name="%s" sheetId="%d" r:id="rId%d"/>`, sheet, i+1, i+1)
		fmt.Fprintf(&rels, `<Relationship Id="rId%d" Type="%s/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, testRNS, i+1)

		var data strings.Builder
		for r, row := range sheets[sheet] {
			fmt.Fprintf(&data, `<row r="%d">`, r+1)
			for c, v := range row {
				fmt.Fprintf(&data, `<c r="%c%d" t="inlineStr"><is><t>%s</t></is></c>`, 'A'+c, r+1, v)
			}
			data.WriteString(`</row>`)
		}
		parts[fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1)] = fmt.Sprintf(`<?xml version="1.0"?><worksheet xmlns="%s"><sheetData>%s</sheetData></worksheet>`, testSheetNS, data.String())
	}
	parts["xl/workbook.xml"] = fmt.Sprintf(`<?xml version="1.0"?><workbook xmlns="%s" xmlns:r="%s"><sheets>%s</sheets></workbook>`, testSheetNS, testRNS, list.String())
	parts["xl/_rels/workbook.xml.rels"] = fmt.Sprintf(`<?xml version="1.0"?><Relationships xmlns="%s">%s</Relationships>`, testRelsNS, rels.String())
	return writeOfficePackage(t, dir, name, "xl/workbook.xml", parts)
}

func writeTestPptx(t *testing.T, dir, name, slideText string) string {
	t.Helper()
	return writeOfficePackage(t, dir, name, "ppt/presentation.xml", map[string]string{
		"ppt/presentation.xml": fmt.Sprintf(`<?xml version="1.0"?><p:presentation xmlns:p="%s" xmlns:r="%s"><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst></p:presentation>`,
			testPresNS, testRNS),
		"ppt/_rels/presentation.xml.rels": fmt.Sprintf(`<?xml version="1.0"?><Relationships xmlns="%s"><Relationship Id="rId1" Type="%s/slide" Target="slides/slide1.xml"/></Relationships>`,
			testRelsNS, testRNS),
		"ppt/slides/slide1.xml": fmt.Sprintf(`<?xml version="1.0"?><p:sld xmlns:p="%s" xmlns:a="%s"><p:cSld><p:spTree><p:sp><p:txBody><a:p><a:r><a:t>%s</a:t></a:r></a:p></p:txBody></p:sp></p:spTree></p:cSld></p:sld>`,
			testPresNS, testDrawNS, slideText),
	})
}

// readOfficeThroughTool calls Read the way the model does, with the session's
// tmp/ folder set as it is during a turn.
func readOfficeThroughTool(t *testing.T, dir, sessionTmp string, args map[string]any) map[string]any {
	t.Helper()
	cfg := newTestConfig()
	cfg.Workspace.Root = dir
	toolCtx := NewToolContext()
	toolCtx.SetSessionTmp(sessionTmp)
	tool := NewReadFileTool(cfg, toolCtx)

	encoded, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}
	result, err := tool.Call(context.Background(), encoded)
	if err != nil {
		t.Fatalf("Read %v: %v", args, err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Read returned %T, want map", result)
	}
	return m
}

func TestReadConvertsWordDocument(t *testing.T) {
	dir, tmp := t.TempDir(), t.TempDir()
	writeTestDocx(t, dir, "report.docx",
		wordHeading("Quarterly Report")+wordPara("Revenue grew in every region.")+wordTable([]string{"Region", "Total"}, []string{"North", "42"}))

	got := readOfficeThroughTool(t, dir, tmp, map[string]any{"path": "report.docx"})

	if got["success"] != true {
		t.Fatalf("success = %v: %v", got["success"], got["message"])
	}
	content, _ := got["content"].(string)
	for _, want := range []string{"# Quarterly Report", "Revenue grew in every region.", "| Region | Total |", "| North | 42 |"} {
		if !strings.Contains(content, want) {
			t.Errorf("content missing %q:\n%s", want, content)
		}
	}
	if got["converted_from"] != "Word document" {
		t.Errorf("converted_from = %v", got["converted_from"])
	}
	converted, _ := got["converted_path"].(string)
	if filepath.Dir(converted) != tmp {
		t.Errorf("converted_path = %q, want it in the session tmp folder %q", converted, tmp)
	}
	if _, err := os.Stat(converted); err != nil {
		t.Errorf("converted text was not saved: %v", err)
	}
	if got["path"] != "report.docx" {
		t.Errorf("path = %v, want the document's own path", got["path"])
	}
}

func TestReadConvertsWorkbookOneTablePerSheet(t *testing.T) {
	dir := t.TempDir()
	writeTestXlsx(t, dir, "sales.xlsx", map[string][][]string{
		"North": {{"Month", "Units"}, {"January", "17"}},
		"South": {{"Month", "Units"}, {"February", "23"}},
	})

	got := readOfficeThroughTool(t, dir, t.TempDir(), map[string]any{"path": "sales.xlsx"})

	content, _ := got["content"].(string)
	for _, want := range []string{"North", "South", "| January | 17 |", "| February | 23 |"} {
		if !strings.Contains(content, want) {
			t.Errorf("content missing %q:\n%s", want, content)
		}
	}
	if got["converted_from"] != "Excel workbook" {
		t.Errorf("converted_from = %v", got["converted_from"])
	}
}

func TestReadConvertsPresentation(t *testing.T) {
	dir := t.TempDir()
	writeTestPptx(t, dir, "launch.pptx", "Launch plan for October")

	got := readOfficeThroughTool(t, dir, t.TempDir(), map[string]any{"path": "launch.pptx"})

	content, _ := got["content"].(string)
	if !strings.Contains(content, "Launch plan for October") {
		t.Errorf("content missing the slide text:\n%s", content)
	}
	if got["converted_from"] != "PowerPoint presentation" {
		t.Errorf("converted_from = %v", got["converted_from"])
	}
}

// A document longer than one response is continued with start on the
// document's own path, and the second read reuses the first one's conversion.
func TestReadPagesThroughALongOfficeDocument(t *testing.T) {
	dir, tmp := t.TempDir(), t.TempDir()
	var body strings.Builder
	for section := 1; section <= 3; section++ {
		body.WriteString(wordHeading(fmt.Sprintf("Section %d", section)))
		for i := 1; i <= 100; i++ {
			body.WriteString(wordPara(fmt.Sprintf("Paragraph %d.%d", section, i)))
		}
	}
	writeTestDocx(t, dir, "long.docx", body.String())

	first := readOfficeThroughTool(t, dir, tmp, map[string]any{"path": "long.docx"})

	hint, _ := first["hint"].(string)
	if !strings.Contains(hint, `"path": "long.docx"`) || !strings.Contains(hint, `"start"`) {
		t.Fatalf("hint does not continue on the document's path: %q", hint)
	}
	outline, ok := first["outline"].([]outlineEntry)
	if !ok || len(outline) != 3 || outline[2].Heading != "Section 3" {
		t.Fatalf("outline = %#v, want the three section headings", first["outline"])
	}

	second := readOfficeThroughTool(t, dir, tmp, map[string]any{"path": "long.docx", "start": outline[2].Line})
	content, _ := second["content"].(string)
	if !strings.Contains(content, "Section 3") || !strings.Contains(content, "Paragraph 3.1") {
		t.Errorf("reading from the outline's line did not reach section 3:\n%.300s", content)
	}
	if second["converted_path"] != first["converted_path"] {
		t.Errorf("second read converted again: %v then %v", first["converted_path"], second["converted_path"])
	}
	if _, has := second["outline"]; has {
		t.Error("a read that names its start repeated the outline")
	}
}

func TestReadConvertsAgainAfterTheDocumentChanges(t *testing.T) {
	dir, tmp := t.TempDir(), t.TempDir()
	path := writeTestDocx(t, dir, "notes.docx", wordPara("First draft"))
	first := readOfficeThroughTool(t, dir, tmp, map[string]any{"path": "notes.docx"})

	writeTestDocx(t, dir, "notes.docx", wordPara("Second draft"))
	later := time.Now().Add(time.Minute)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	second := readOfficeThroughTool(t, dir, tmp, map[string]any{"path": "notes.docx"})

	if content, _ := second["content"].(string); !strings.Contains(content, "Second draft") {
		t.Errorf("read after the change returned the old text:\n%s", content)
	}
	if second["converted_path"] == first["converted_path"] {
		t.Error("the changed document reused the old conversion")
	}
}

// Like a PDF, a converted document must not satisfy the Read-before-Edit gate:
// the converted text appears nowhere in the file's bytes.
func TestReadOfficeDoesNotSatisfyReadBeforeEdit(t *testing.T) {
	dir := t.TempDir()
	path := writeTestDocx(t, dir, "doc.docx", wordPara("Some text"))

	cfg := newTestConfig()
	cfg.Workspace.Root = dir
	toolCtx := NewToolContext()
	toolCtx.SetSessionTmp(t.TempDir())
	tool := NewReadFileTool(cfg, toolCtx)
	args, _ := json.Marshal(map[string]any{"path": "doc.docx"})
	if _, err := tool.Call(context.Background(), args); err != nil {
		t.Fatal(err)
	}

	if toolCtx.ReadTracker.WasReadRecently(path, toolCtx.ReadTracker.CurrentMessageID(), 5) {
		t.Error("reading a Word document recorded a read, so Edit would accept the file as text")
	}
}

// The converter recovers from a part whose XML stops early, so the damage here
// is a package whose main part is missing altogether, which nothing recovers.
func TestReadReportsADamagedOfficeDocument(t *testing.T) {
	dir := t.TempDir()
	writeOfficePackage(t, dir, "broken.docx", "word/document.xml", map[string]string{
		"word/styles.xml": `<?xml version="1.0"?><w:styles ` + testWordNS + `/>`,
	})

	got := readOfficeThroughTool(t, dir, t.TempDir(), map[string]any{"path": "broken.docx"})

	if got["success"] != false || got["error"] != "office_unreadable" {
		t.Fatalf("got success=%v error=%v, want office_unreadable", got["success"], got["error"])
	}
	if hint, _ := got["hint"].(string); !strings.Contains(hint, "word/document.xml") {
		t.Errorf("hint = %q, want the unzip fallback", hint)
	}
}

// Two limits keep a conversion's memory bounded: one on the XML it has to
// parse, which is what memory follows, and one on the file read into memory.
func TestReadRefusesAnOversizedOfficeDocument(t *testing.T) {
	for _, tc := range []struct {
		name  string
		limit *int64
	}{
		{"unpacked XML", &maxOfficeXMLBytes},
		{"file size", &maxOfficeFileBytes},
	} {
		t.Run(tc.name, func(t *testing.T) {
			saved := *tc.limit
			*tc.limit = 100
			defer func() { *tc.limit = saved }()

			dir := t.TempDir()
			writeTestDocx(t, dir, "big.docx", wordPara(strings.Repeat("word ", 200)))

			got := readOfficeThroughTool(t, dir, t.TempDir(), map[string]any{"path": "big.docx"})

			if got["error"] != "office_unreadable" {
				t.Fatalf("error = %v, want office_unreadable", got["error"])
			}
			if msg, _ := got["message"].(string); !strings.Contains(msg, "limit") {
				t.Errorf("message = %q, want it to name the limit", msg)
			}
		})
	}
}

// Zip files that are not Office documents keep the advice Read gave before:
// one without the Office layout is never read past its directory, and one with
// the layout but another kind of content is handed back by the converter.
func TestReadLeavesOtherZipFilesToTheGuard(t *testing.T) {
	dir := t.TempDir()

	var plain bytes.Buffer
	zw := zip.NewWriter(&plain)
	f, _ := zw.Create("META-INF/MANIFEST.MF")
	f.Write([]byte("Manifest-Version: 1.0\n"))
	zw.Close()
	writeFileForGuard(t, dir, "lib.jar", plain.Bytes())
	writeOfficePackage(t, dir, "drawing.zip", "visio/document.xml", map[string]string{"visio/document.xml": `<VisioDocument/>`})

	for name, wantType := range map[string]string{"lib.jar": "Java archive", "drawing.zip": "ZIP archive"} {
		got := readOfficeThroughTool(t, dir, t.TempDir(), map[string]any{"path": name})
		if got["error"] != "not_text" || got["file_type"] != wantType {
			t.Errorf("%s: error=%v file_type=%v, want not_text %q", name, got["error"], got["file_type"], wantType)
		}
	}
}

// A password-protected document is stored in the older compound-file container
// with an EncryptedPackage stream, and is named as such rather than as an
// unknown binary file.
func TestReadNamesAPasswordProtectedDocument(t *testing.T) {
	data := make([]byte, 1024)
	copy(data, oleMagic)
	entry := data[512 : 512+128]
	name := utf16.Encode([]rune("EncryptedPackage"))
	for i, u := range name {
		binary.LittleEndian.PutUint16(entry[2*i:], u)
	}
	binary.LittleEndian.PutUint16(entry[64:], uint16(2*(len(name)+1)))
	entry[66] = 2 // a stream

	dir := t.TempDir()
	writeFileForGuard(t, dir, "secret.docx", data)

	got := readOfficeThroughTool(t, dir, t.TempDir(), map[string]any{"path": "secret.docx"})

	if got["error"] != "office_encrypted" {
		t.Fatalf("error = %v (%v), want office_encrypted", got["error"], got["message"])
	}
}

// The pre-2007 formats and OpenDocument are not converted. Each is named, and
// the hint converts it to the newer format inside the session's tmp/ folder,
// which Read opens without asking, with a way at the text as the fallback.
func TestReadNamesConvertibleOfficeFormats(t *testing.T) {
	compound := append(append([]byte{}, oleMagic...), make([]byte, 1016)...)
	var odf bytes.Buffer
	zw := zip.NewWriter(&odf)
	f, _ := zw.Create("mimetype")
	f.Write([]byte("application/vnd.oasis.opendocument.text"))
	zw.Close()
	dir, tmp := t.TempDir(), t.TempDir()

	for _, tc := range []struct {
		name                                               string
		content                                            []byte
		wantType, wantConvert, wantFallback, wantConverted string
	}{
		{"minutes.doc", compound, "Word 97-2003 document", "--convert-to docx", "catdoc", "minutes.docx"},
		{"budget.xls", compound, "Excel 97-2003 workbook", "--convert-to xlsx", "xls2csv", "budget.xlsx"},
		{"pitch.ppt", compound, "PowerPoint 97-2003 presentation", "--convert-to pptx", "catppt", "pitch.pptx"},
		{"letter.odt", odf.Bytes(), "OpenDocument text", "--convert-to docx", "content.xml", "letter.docx"},
		{"ledger.ods", odf.Bytes(), "OpenDocument spreadsheet", "--convert-to xlsx", "content.xml", "ledger.xlsx"},
		{"deck.odp", odf.Bytes(), "OpenDocument presentation", "--convert-to pptx", "content.xml", "deck.pptx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			writeFileForGuard(t, dir, tc.name, tc.content)

			got := readOfficeThroughTool(t, dir, tmp, map[string]any{"path": tc.name})

			if got["error"] != "not_text" || got["file_type"] != tc.wantType {
				t.Fatalf("error=%v file_type=%v, want not_text %q", got["error"], got["file_type"], tc.wantType)
			}
			hint, _ := got["hint"].(string)
			converted := filepath.Join(tmp, tc.wantConverted)
			for _, want := range []string{tc.wantConvert, fmt.Sprintf("--outdir %q", tmp), fmt.Sprintf(`Read {"path": %q}`, converted), tc.wantFallback} {
				if !strings.Contains(hint, want) {
					t.Errorf("hint missing %q:\n%s", want, hint)
				}
			}
		})
	}

	// The container alone is not enough, since other programs use it too, and
	// the extension alone is not either.
	writeFileForGuard(t, dir, "thumbs.bin", compound)
	if got := readOfficeThroughTool(t, dir, tmp, map[string]any{"path": "thumbs.bin"}); got["file_type"] != "binary file" {
		t.Errorf("a compound file without an Office extension was named %v", got["file_type"])
	}
	writeFileForGuard(t, dir, "fake.odt", compound)
	if got := readOfficeThroughTool(t, dir, tmp, map[string]any{"path": "fake.odt"}); got["file_type"] == "OpenDocument text" {
		t.Error("a compound file named .odt was taken for OpenDocument")
	}
}

func TestCharModeStillReturnsAnOfficeDocumentsBytes(t *testing.T) {
	dir := t.TempDir()
	writeTestDocx(t, dir, "raw.docx", wordPara("Some text"))

	got := readOfficeThroughTool(t, dir, t.TempDir(), map[string]any{"path": "raw.docx", "char_mode": true})

	if _, converted := got["converted_from"]; converted {
		t.Fatal("char_mode converted the document instead of returning its bytes")
	}
	if content, _ := got["content"].(string); !strings.HasPrefix(content, "PK") {
		t.Errorf("char_mode content does not start with the zip signature: %.20q", content)
	}
}

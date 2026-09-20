package tools

import (
	"bytes"
	"compress/zlib"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTestPDF builds a PDF with one page per string, each drawn as a single
// line of text, and returns its path. It is written by hand rather than
// checked in so the test data is visible in the test, the way writeTestPNG
// does it for images.
func writeTestPDF(t *testing.T, dir, name string, pages []string) string {
	t.Helper()

	var objects [][]byte
	// 1: catalog, 2: page tree, 3..: one page and one content stream each,
	// then the font as the last object.
	pageObjStart := 3
	fontObj := pageObjStart + len(pages)*2

	kids := make([]string, len(pages))
	for i := range pages {
		kids[i] = fmt.Sprintf("%d 0 R", pageObjStart+i*2)
	}

	objects = append(objects, []byte("<< /Type /Catalog /Pages 2 0 R >>"))
	objects = append(objects, fmt.Appendf(nil, "<< /Type /Pages /Kids [%s] /Count %d >>",
		strings.Join(kids, " "), len(pages)))

	for i, text := range pages {
		contentObj := pageObjStart + i*2 + 1
		objects = append(objects, fmt.Appendf(nil,
			"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] /Contents %d 0 R /Resources << /Font << /F1 %d 0 R >> >> >>",
			contentObj, fontObj))

		stream := fmt.Sprintf("BT /F1 24 Tf 72 720 Td (%s) Tj ET", text)
		var compressed bytes.Buffer
		zw := zlib.NewWriter(&compressed)
		if _, err := zw.Write([]byte(stream)); err != nil {
			t.Fatalf("compress page %d: %v", i+1, err)
		}
		zw.Close()

		obj := fmt.Appendf(nil, "<< /Length %d /Filter /FlateDecode >>\nstream\n", compressed.Len())
		obj = append(obj, compressed.Bytes()...)
		obj = append(obj, []byte("\nendstream")...)
		objects = append(objects, obj)
	}

	objects = append(objects, []byte("<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>"))

	out := []byte("%PDF-1.4\n")
	offsets := make([]int, len(objects))
	for i, obj := range objects {
		offsets[i] = len(out)
		out = fmt.Appendf(out, "%d 0 obj\n", i+1)
		out = append(out, obj...)
		out = append(out, []byte("\nendobj\n")...)
	}

	xref := len(out)
	out = fmt.Appendf(out, "xref\n0 %d\n0000000000 65535 f \n", len(objects)+1)
	for _, off := range offsets {
		out = fmt.Appendf(out, "%010d 00000 n \n", off)
	}
	out = fmt.Appendf(out, "trailer\n<< /Size %d /Root 1 0 R >>\nstartxref\n%d\n%%%%EOF\n",
		len(objects)+1, xref)

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// readPDFThroughTool calls Read the way the model does, with the workspace
// rooted at dir.
func readPDFThroughTool(t *testing.T, dir, path, pages string) map[string]any {
	t.Helper()

	cfg := newTestConfig()
	cfg.Workspace.Root = dir
	cfg.Tools.Read.PDF.MaxPages = 10
	cfg.Tools.Read.PDF.CacheDir = filepath.Join(t.TempDir(), "wasm-cache")

	tool := NewReadFileTool(cfg, NewToolContext())
	args, err := json.Marshal(map[string]any{"path": path, "pages": pages})
	if err != nil {
		t.Fatalf("marshal args: %v", err)
	}
	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("Read %s: %v", path, err)
	}
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Read returned %T, want map", result)
	}
	return m
}

func TestReadExtractsPDFText(t *testing.T) {
	dir := t.TempDir()
	writeTestPDF(t, dir, "notes.pdf", []string{"First page text", "Second page text"})

	got := readPDFThroughTool(t, dir, "notes.pdf", "")

	if got["success"] != true {
		t.Fatalf("success = %v, message = %v", got["success"], got["message"])
	}
	content, _ := got["content"].(string)
	for _, want := range []string{"First page text", "Second page text"} {
		if !strings.Contains(content, want) {
			t.Errorf("content missing %q:\n%s", want, content)
		}
	}
	if total, _ := got["total_pages"].(int); total != 2 {
		t.Errorf("total_pages = %v, want 2", got["total_pages"])
	}
	// The line-mode branch must not have run: its output carries line numbers
	// and a note about them.
	if strings.Contains(content, "│") {
		t.Errorf("PDF content has line-number prefixes:\n%s", content)
	}
}

func TestReadPDFHonorsPageRange(t *testing.T) {
	dir := t.TempDir()
	writeTestPDF(t, dir, "many.pdf", []string{"Alpha one", "Bravo two", "Charlie three", "Delta four"})

	got := readPDFThroughTool(t, dir, "many.pdf", "2-3")

	content, _ := got["content"].(string)
	if !strings.Contains(content, "Bravo two") || !strings.Contains(content, "Charlie three") {
		t.Errorf("content missing the requested pages:\n%s", content)
	}
	if strings.Contains(content, "Alpha one") || strings.Contains(content, "Delta four") {
		t.Errorf("content includes pages outside the range:\n%s", content)
	}
	if first, _ := got["first_read_page"].(int); first != 2 {
		t.Errorf("first_read_page = %v, want 2", got["first_read_page"])
	}
	if last, _ := got["last_read_page"].(int); last != 3 {
		t.Errorf("last_read_page = %v, want 3", got["last_read_page"])
	}
	hint, _ := got["hint"].(string)
	if !strings.Contains(hint, "4-") {
		t.Errorf("hint does not say how to continue: %q", hint)
	}
}

// A PDF read must not satisfy the Read-before-Edit gate. The gate exists so the
// model quotes back text it has seen, and extracted text appears nowhere in the
// file's bytes.
func TestReadPDFDoesNotSatisfyReadBeforeEdit(t *testing.T) {
	dir := t.TempDir()
	path := writeTestPDF(t, dir, "doc.pdf", []string{"Some text"})

	cfg := newTestConfig()
	cfg.Workspace.Root = dir
	cfg.Tools.Read.PDF.CacheDir = filepath.Join(t.TempDir(), "wasm-cache")
	toolCtx := NewToolContext()

	tool := NewReadFileTool(cfg, toolCtx)
	args, _ := json.Marshal(map[string]any{"path": "doc.pdf"})
	if _, err := tool.Call(context.Background(), args); err != nil {
		t.Fatalf("Read: %v", err)
	}

	if toolCtx.ReadTracker.WasReadRecently(path, toolCtx.ReadTracker.CurrentMessageID(), 5) {
		t.Error("reading a PDF recorded a read, so Edit would accept the file as text")
	}
}

// A file named .pdf that is not one falls through to the text branches rather
// than producing a PDF error.
func TestReadFallsThroughForNonPDFNamedPDF(t *testing.T) {
	dir := t.TempDir()
	html := "<!DOCTYPE html>\n<html><body>not a pdf</body></html>\n"
	if err := os.WriteFile(filepath.Join(dir, "fake.pdf"), []byte(html), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got := readPDFThroughTool(t, dir, "fake.pdf", "")

	if got["success"] != true {
		t.Fatalf("success = %v, want the text branch to read it", got["success"])
	}
	content, _ := got["content"].(string)
	if !strings.Contains(content, "not a pdf") {
		t.Errorf("content does not look like the file's text:\n%s", content)
	}
}

// A page with no text is what a scanned document looks like, and saying so is
// the difference between the model concluding the page is blank and the model
// knowing it needs optical character recognition.
func TestReadPDFReportsPagesWithoutText(t *testing.T) {
	dir := t.TempDir()
	writeTestPDF(t, dir, "scan.pdf", []string{""})

	got := readPDFThroughTool(t, dir, "scan.pdf", "")

	if got["success"] != true {
		t.Fatalf("success = %v, message = %v", got["success"], got["message"])
	}
	content, _ := got["content"].(string)
	if !strings.Contains(content, "no text") {
		t.Errorf("content does not say the page has no text:\n%s", content)
	}
	hint, _ := got["hint"].(string)
	if !strings.Contains(hint, "scanned") {
		t.Errorf("hint does not explain why: %q", hint)
	}
}

func TestParsePDFPages(t *testing.T) {
	cases := []struct {
		spec      string
		want      pdfPageRange
		wantError bool
	}{
		{spec: "", want: pdfPageRange{First: 1}},
		{spec: "3", want: pdfPageRange{First: 3, Last: 3}},
		{spec: "2-6", want: pdfPageRange{First: 2, Last: 6}},
		{spec: " 2 - 6 ", want: pdfPageRange{First: 2, Last: 6}},
		{spec: "5-", want: pdfPageRange{First: 5}},
		{spec: "6-2", wantError: true},
		{spec: "0", wantError: true},
		{spec: "-3", wantError: true},
		{spec: "two", wantError: true},
	}

	for _, tc := range cases {
		got, err := parsePDFPages(tc.spec)
		switch {
		case tc.wantError && err == nil:
			t.Errorf("parsePDFPages(%q) = %+v, want an error", tc.spec, got)
		case !tc.wantError && err != nil:
			t.Errorf("parsePDFPages(%q): %v", tc.spec, err)
		case !tc.wantError && got != tc.want:
			t.Errorf("parsePDFPages(%q) = %+v, want %+v", tc.spec, got, tc.want)
		}
	}
}

func TestLooksLikePDF(t *testing.T) {
	dir := t.TempDir()
	real := writeTestPDF(t, dir, "real.pdf", []string{"text"})

	text := filepath.Join(dir, "plain.txt")
	if err := os.WriteFile(text, []byte("just words\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	short := filepath.Join(dir, "tiny.txt")
	if err := os.WriteFile(short, []byte("%PD"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	if !looksLikePDF(real) {
		t.Error("a real PDF was not recognized")
	}
	if looksLikePDF(text) {
		t.Error("a text file was taken for a PDF")
	}
	if looksLikePDF(short) {
		t.Error("a file shorter than the header was taken for a PDF")
	}
	if looksLikePDF(filepath.Join(dir, "missing.pdf")) {
		t.Error("a missing file was taken for a PDF")
	}
}

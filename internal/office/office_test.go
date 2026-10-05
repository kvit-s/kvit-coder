package office

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

const (
	wordNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"`
	relsNS = "http://schemas.openxmlformats.org/package/2006/relationships"
	relNS  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

func packageOf(t *testing.T, parts map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range parts {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func wordPackage(t *testing.T, body string, extra map[string]string) []byte {
	parts := map[string]string{
		"[Content_Types].xml": `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"/>`,
		"_rels/.rels":         `<Relationships xmlns="` + relsNS + `"><Relationship Id="rId1" Type="` + relNS + `/officeDocument" Target="word/document.xml"/></Relationships>`,
		"word/document.xml":   `<w:document ` + wordNS + `><w:body>` + body + `</w:body></w:document>`,
	}
	for k, v := range extra {
		parts[k] = v
	}
	return packageOf(t, parts)
}

func TestConvertNamesTheFormat(t *testing.T) {
	got, err := Convert(wordPackage(t, `<w:p><w:r><w:t>hello</w:t></w:r></w:p>`, nil), "a.docx")
	if err != nil {
		t.Fatal(err)
	}
	if got.Kind != "Word document" || strings.TrimSpace(got.Markdown) != "hello" {
		t.Errorf("got kind %q markdown %q", got.Kind, got.Markdown)
	}
}

func TestConvertRefusesWhatIsNotOffice(t *testing.T) {
	for name, data := range map[string][]byte{
		"plain.txt": []byte("just text"),
		"lib.jar":   packageOf(t, map[string]string{"META-INF/MANIFEST.MF": "Manifest-Version: 1.0\n"}),
	} {
		if _, err := Convert(data, name); !errors.Is(err, ErrNotOffice) {
			t.Errorf("%s: err = %v, want ErrNotOffice", name, err)
		}
	}
}

// A picture whose image part is missing is skipped with a warning, and the
// warning belongs to the conversion that produced it.
func TestConvertCollectsWarningsAboutSkippedParts(t *testing.T) {
	data := wordPackage(t,
		`<w:p><w:r><w:t>before</w:t></w:r><w:r><w:drawing><a:blip r:embed="rId5"/></w:drawing></w:r></w:p>`,
		map[string]string{
			"word/_rels/document.xml.rels": `<Relationships xmlns="` + relsNS + `"><Relationship Id="rId5" Type="` + relNS + `/image" Target="media/gone.png"/></Relationships>`,
		})

	got, err := Convert(data, "a.docx")
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "word/media/gone.png") {
		t.Fatalf("warnings = %q, want one naming the missing image", got.Warnings)
	}
	if !strings.Contains(got.Markdown, "before") {
		t.Errorf("the rest of the document was lost: %q", got.Markdown)
	}

	clean, err := Convert(wordPackage(t, `<w:p><w:r><w:t>clean</w:t></w:r></w:p>`, nil), "b.docx")
	if err != nil {
		t.Fatal(err)
	}
	if len(clean.Warnings) != 0 {
		t.Errorf("a clean conversion inherited warnings: %q", clean.Warnings)
	}
}

// The logger is swapped only for the length of a conversion, so the parsers go
// back to logging nowhere afterwards.
func TestConvertRestoresTheLogger(t *testing.T) {
	before := model.Log
	if _, err := Convert(wordPackage(t, `<w:p/>`, nil), "a.docx"); err != nil {
		t.Fatal(err)
	}
	if model.Log != before {
		t.Error("model.Log was left pointing at a conversion's collector")
	}
}

func TestInspectPackage(t *testing.T) {
	dir := t.TempDir()
	office := filepath.Join(dir, "a.docx")
	other := filepath.Join(dir, "b.zip")
	notZip := filepath.Join(dir, "c.txt")
	for path, data := range map[string][]byte{
		office: wordPackage(t, `<w:p/>`, map[string]string{"word/media/photo.png": strings.Repeat("x", 5000)}),
		other:  packageOf(t, map[string]string{"x.txt": "x"}),
		notZip: []byte("text"),
	} {
		if err := os.WriteFile(path, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	isPackage, xmlBytes := InspectPackage(office)
	if !isPackage {
		t.Error("an Office package was not recognised")
	}
	if xmlBytes == 0 || xmlBytes >= 5000 {
		t.Errorf("xmlBytes = %d, want the XML parts counted and the picture left out", xmlBytes)
	}
	for _, path := range []string{other, notZip} {
		if isPackage, _ := InspectPackage(path); isPackage {
			t.Errorf("%s was taken for an Office package", filepath.Base(path))
		}
	}
}

func TestReadFileRefusesBeforeReading(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.docx")
	if err := os.WriteFile(path, make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadFile(path, 1024); err == nil || !strings.Contains(err.Error(), "limit") {
		t.Errorf("err = %v, want the size limit", err)
	}
}

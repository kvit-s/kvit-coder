package opc

import (
	"archive/zip"
	"bytes"
	"compress/flate"
	"encoding/binary"
	"errors"
	"io"
	"testing"
	"unicode/utf16"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

type part struct{ name, body string }

func zipOf(t *testing.T, parts ...part) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, p := range parts {
		f, err := w.Create(p.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, p.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func open(t *testing.T, parts ...part) *Package {
	t.Helper()
	p, err := Open(zipOf(t, parts...))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func wantLimit(t *testing.T, err error, limit string) {
	t.Helper()
	var le *model.LimitError
	if !errors.As(err, &le) || le.Limit != limit {
		t.Fatalf("want %s limit error, got %v", limit, err)
	}
}

func TestOpenRejectsNonZip(t *testing.T) {
	_, err := Open([]byte("not a zip at all"))
	var me *MalformedError
	if !errors.As(err, &me) {
		t.Fatalf("want malformed, got %v", err)
	}
}

func cfbWithStream(name string) []byte {
	data := make([]byte, 1024)
	copy(data, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	entry := data[512+128:]
	units := utf16.Encode([]rune(name))
	for i, u := range units {
		binary.LittleEndian.PutUint16(entry[2*i:], u)
	}
	binary.LittleEndian.PutUint16(entry[64:], uint16(2*(len(units)+1)))
	entry[66] = 2
	return data
}

func TestOpenNamesEncryptedPackages(t *testing.T) {
	if _, err := Open(cfbWithStream("EncryptedPackage")); !errors.Is(err, ErrEncrypted) {
		t.Fatalf("want ErrEncrypted, got %v", err)
	}
	var me *MalformedError
	if _, err := Open(cfbWithStream("WordDocument")); !errors.As(err, &me) {
		t.Fatalf("legacy OLE document must be malformed, got %v", err)
	}
	if !IsEncrypted(cfbWithStream("EncryptionInfo")) || IsEncrypted(cfbWithStream("Workbook")) || IsEncrypted([]byte("PK")) {
		t.Fatal("IsEncrypted must detect only encryption streams")
	}
}

func TestPartLookup(t *testing.T) {
	p := open(t, part{"word/document.xml", "<x/>"})
	if !p.Has("/word/document.xml") || p.Has("word/missing.xml") {
		t.Fatal("Has must normalize the leading slash and report absence")
	}
	data, err := p.Part("/word/document.xml")
	if err != nil || string(data) != "<x/>" {
		t.Fatalf("Part = %q, %v", data, err)
	}
	var missing *MissingPartError
	if _, err := p.Part("word/styles.xml"); !errors.As(err, &missing) || missing.Part != "word/styles.xml" {
		t.Fatalf("want missing part, got %v", err)
	}
	if data, err := p.OptionalPart("word/styles.xml"); data != nil || err != nil {
		t.Fatalf("absent optional part must be nil, nil; got %q, %v", data, err)
	}
}

func TestRepeatedReadsAreChargedOnce(t *testing.T) {
	p := open(t, part{"media/a.bin", string(make([]byte, 4096))})
	for range 5 {
		if data, err := p.Part("media/a.bin"); err != nil || len(data) != 4096 {
			t.Fatalf("read %d bytes, %v", len(data), err)
		}
	}
	if p.totalRead != 4096 {
		t.Fatalf("budget charged %d bytes, want 4096", p.totalRead)
	}
}

func TestTotalBudgetExhaustion(t *testing.T) {
	p := open(t, part{"a.bin", string(make([]byte, 4096))}, part{"b.bin", string(make([]byte, 4096))})
	if _, err := p.Part("a.bin"); err != nil {
		t.Fatal(err)
	}
	p.totalRead = MaxTotalBytes - 100
	_, err := p.Part("b.bin")
	wantLimit(t, err, "max_total_bytes")
	if _, err := p.OptionalPart("b.bin"); err == nil {
		t.Fatal("limit errors must propagate from optional reads")
	}
}

func rawZip(t *testing.T, name string, declared uint64, compressed []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.CreateRaw(&zip.FileHeader{
		Name: name, Method: zip.Deflate,
		CompressedSize64: uint64(len(compressed)), UncompressedSize64: declared,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(compressed); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func deflateZeros(t *testing.T, n int64) []byte {
	t.Helper()
	var buf bytes.Buffer
	fw, _ := flate.NewWriter(&buf, flate.BestSpeed)
	if _, err := io.CopyN(fw, zeroReader{}, n); err != nil {
		t.Fatal(err)
	}
	if err := fw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

type zeroReader struct{}

func (zeroReader) Read(b []byte) (int, error) {
	clear(b)
	return len(b), nil
}

func TestEntryBytesLimit(t *testing.T) {
	small := deflateZeros(t, 10)
	p, err := Open(rawZip(t, "declared.bin", MaxEntryBytes+1, small))
	if err != nil {
		t.Fatal(err)
	}
	_, err = p.Part("declared.bin")
	wantLimit(t, err, "max_entry_bytes")

	p, err = Open(rawZip(t, "lying.bin", 10, deflateZeros(t, 1<<20)))
	if err != nil {
		t.Fatal(err)
	}
	var me *MalformedError
	if _, err := p.Part("lying.bin"); !errors.As(err, &me) {
		t.Fatalf("an entry inflating past its declared size must not be read, got %v", err)
	}
	if p.totalRead != 0 {
		t.Fatalf("rejected entry charged %d bytes", p.totalRead)
	}
}

func TestEntryCountLimit(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for i := range MaxEntryCount + 1 {
		if _, err := w.CreateHeader(&zip.FileHeader{Name: string(rune('a'+i%26)) + string(rune(i)), Method: zip.Store}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	_, err := Open(buf.Bytes())
	wantLimit(t, err, "max_entry_count")
}

func TestCorruptOptionalPartIsSkipped(t *testing.T) {
	p, err := Open(rawZip(t, "broken.xml", 5, []byte{0xFF, 0xFF, 0xFF}))
	if err != nil {
		t.Fatal(err)
	}
	var me *MalformedError
	if _, err := p.Part("broken.xml"); !errors.As(err, &me) || me.Part != "broken.xml" {
		t.Fatalf("want malformed part error, got %v", err)
	}
	if data, err := p.OptionalPart("broken.xml"); data != nil || err != nil {
		t.Fatalf("corrupt optional part must be skipped, got %q, %v", data, err)
	}
}

func TestContentTypes(t *testing.T) {
	p := open(t,
		part{"[Content_Types].xml", `<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
			<Default Extension="xml" ContentType="application/xml"/>
			<Default Extension="PNG" ContentType="image/png"/>
			<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>
		</Types>`},
		part{"word/document.xml", "<x/>"},
	)
	cases := map[string]string{
		"word/document.xml":  "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
		"/WORD/Document.xml": "application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml",
		"word/styles.xml":    "application/xml",
		"word/media/a.png":   "image/png",
		"word/media/a.emf":   "",
	}
	for name, want := range cases {
		if got, err := p.ContentType(name); err != nil || got != want {
			t.Errorf("ContentType(%q) = %q, %v; want %q", name, got, err, want)
		}
	}
	if got, err := open(t).ContentType("a.xml"); got != "" || err != nil {
		t.Fatalf("no content types part: %q, %v", got, err)
	}
}

const rootRels = `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
	<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="custom/main.xml"/>
	<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="other/main.xml"/>
	<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
</Relationships>`

func TestMainPart(t *testing.T) {
	p := open(t, part{"_rels/.rels", rootRels})
	if got, err := p.MainPart("word/document.xml"); err != nil || got != "other/main.xml" {
		t.Fatalf("MainPart = %q, %v; want the lowest-id officeDocument target", got, err)
	}
	if got, err := open(t).MainPart("word/document.xml"); err != nil || got != "word/document.xml" {
		t.Fatalf("MainPart without rels = %q, %v", got, err)
	}
	if got, err := open(t, part{"_rels/.rels", "<not xml"}).MainPart("ppt/presentation.xml"); err != nil || got != "ppt/presentation.xml" {
		t.Fatalf("corrupt rels must fall back, got %q, %v", got, err)
	}
}

func TestRootName(t *testing.T) {
	p := open(t,
		part{"strict.xml", `<?xml version="1.0"?><!-- c --><x:workbook xmlns:x="http://purl.oclc.org/ooxml/spreadsheetml/main"><x:sheets/></x:workbook>`},
		part{"late.xml", "<!--" + string(bytes.Repeat([]byte("x"), maxRootSniffBytes)) + "--><root/>"},
		part{"empty.xml", ""},
	)
	space, local, err := p.RootName("/strict.xml")
	if err != nil || space != "http://schemas.openxmlformats.org/spreadsheetml/2006/main" || local != "workbook" {
		t.Fatalf("RootName = %q %q %v", space, local, err)
	}
	var me *MalformedError
	for _, name := range []string{"late.xml", "empty.xml"} {
		if _, _, err := p.RootName(name); !errors.As(err, &me) {
			t.Errorf("%s: want malformed, got %v", name, err)
		}
	}
	var missing *MissingPartError
	if _, _, err := p.RootName("absent.xml"); !errors.As(err, &missing) {
		t.Errorf("absent part: want missing, got %v", err)
	}
}

func TestRelationships(t *testing.T) {
	p := open(t, part{"word/_rels/document.xml.rels", `<Relationships xmlns="http://purl.oclc.org/ooxml/package/relationships">
		<Relationship Id="rId1" Type="http://purl.oclc.org/ooxml/officeDocument/relationships/styles" Target="styles.xml"/>
		<Relationship Id="rId2" Type="http://purl.oclc.org/ooxml/officeDocument/relationships/image" Target="../media/image1.png"/>
		<Relationship Id="rId3" Type="http://purl.oclc.org/ooxml/officeDocument/relationships/hyperlink" Target="https://example.com/a" TargetMode="External"/>
		<Relationship Id="rId4" Type="http://purl.oclc.org/ooxml/officeDocument/relationships/numbering" Target="/word/numbering2.xml"/>
		<Relationship Type="broken" Target="no-id.xml"/>
	</Relationships>`})
	rels, err := p.Rels("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if rels.Source != "word/document.xml" || len(rels.List) != 4 {
		t.Fatalf("rels %+v", rels)
	}
	link, ok := rels.ByID("rId3")
	if !ok || !link.External || link.Target != "https://example.com/a" || link.Type != RelHyperlink {
		t.Fatalf("hyperlink %+v", link)
	}
	if got, ok := rels.PartPath("rId2"); !ok || got != "media/image1.png" {
		t.Fatalf("PartPath(rId2) = %q, %v", got, ok)
	}
	if _, ok := rels.PartPath("rId3"); ok {
		t.Fatal("external targets are not parts")
	}
	if _, ok := rels.PartPath("rId404"); ok {
		t.Fatal("unknown ids resolve to nothing")
	}
	if got := rels.PartOfType(RelStyles, "styles.xml"); got != "word/styles.xml" {
		t.Fatalf("styles part %q", got)
	}
	if got := rels.PartOfType(RelNumbering, "numbering.xml"); got != "word/numbering2.xml" {
		t.Fatalf("numbering part %q", got)
	}
	if got := rels.PartOfType(RelFootnotes, "footnotes.xml"); got != "word/footnotes.xml" {
		t.Fatalf("conventional footnotes part %q", got)
	}
	if empty, err := p.Rels("word/footnotes.xml"); err != nil || len(empty.List) != 0 || empty.Source != "word/footnotes.xml" {
		t.Fatalf("absent rels part must be empty, got %+v, %v", empty, err)
	}
}

func TestResolve(t *testing.T) {
	cases := []struct{ base, ref, path, fragment string }{
		{"word/document.xml", "media/image1.png", "word/media/image1.png", ""},
		{"word/document.xml", "/docProps/core.xml", "docProps/core.xml", ""},
		{"", "word/document.xml", "word/document.xml", ""},
		{"a/b/c.xml", "../images/i.png", "a/images/i.png", ""},
		{"a/b.xml", "./c.xml", "a/c.xml", ""},
		{"a/b.xml", "../../../x.xml", "x.xml", ""},
		{"ppt/slides/slide1.xml", "slide2.xml?x=1#caf%C3%A9%20menu", "ppt/slides/slide2.xml", "café menu"},
		{"ppt/slides/slide1.xml", "#local", "ppt/slides/slide1.xml", "local"},
		{"x/y.xml", "my%20file.xml", "x/my file.xml", ""},
		{"x/y.xml", "bad%zzname.xml", "x/bad%zzname.xml", ""},
	}
	for _, c := range cases {
		got, err := Resolve(c.base, c.ref)
		if err != nil || got.Path != c.path || got.Fragment != c.fragment {
			t.Errorf("Resolve(%q, %q) = %+v, %v; want %q #%q", c.base, c.ref, got, err, c.path, c.fragment)
		}
	}
	for _, ref := range []string{"x%2Fy.xml", "x%5Cy.xml", "%2E%2E/secret.xml", "%2e%2e/secret.xml"} {
		if _, err := Resolve("a/b.xml", ref); err == nil {
			t.Errorf("Resolve(%q) must reject encoded structure", ref)
		}
	}
}

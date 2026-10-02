// Package formats identifies document formats from content bytes.
package formats

import (
	"bytes"
	"path"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

type Format int

const (
	Unknown Format = iota
	PDF
	Docx
	Xlsx
	Pptx
	Rtf
	Ole
)

func (f Format) String() string {
	switch f {
	case PDF:
		return "pdf"
	case Docx:
		return "docx"
	case Xlsx:
		return "xlsx"
	case Pptx:
		return "pptx"
	case Rtf:
		return "rtf"
	case Ole:
		return "ole"
	default:
		return "unknown"
	}
}

func (f Format) MarshalText() ([]byte, error) {
	return []byte(f.String()), nil
}

var (
	oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}
	rtfMagic = []byte("{\\rtf")
	pdfMagic = []byte("%PDF-")
)

type officeKind struct {
	dir      string
	entry    string
	exts     []string
	format   Format
	ctFamily []string
	rootNS   string
	root     string
}

var officeKinds = []officeKind{
	{"word/", "word/document.xml", []string{".docx", ".docm"}, Docx,
		[]string{"application/vnd.openxmlformats-officedocument.wordprocessingml.", "application/vnd.ms-word."},
		"http://schemas.openxmlformats.org/wordprocessingml/2006/main", "document"},
	{"xl/", "xl/workbook.xml", []string{".xlsx", ".xlsm"}, Xlsx,
		[]string{"application/vnd.openxmlformats-officedocument.spreadsheetml.", "application/vnd.ms-excel."},
		"http://schemas.openxmlformats.org/spreadsheetml/2006/main", "workbook"},
	{"ppt/", "ppt/presentation.xml", []string{".pptx", ".pptm", ".ppsx", ".ppsm"}, Pptx,
		[]string{"application/vnd.openxmlformats-officedocument.presentationml.", "application/vnd.ms-powerpoint."},
		"http://schemas.openxmlformats.org/presentationml/2006/main", "presentation"},
}

func Detect(data []byte, name string) Format {
	switch {
	case bytes.HasPrefix(data, rtfMagic):
		return Rtf
	case bytes.HasPrefix(data, oleMagic):
		return Ole
	case isZip(data):
		pkg, err := opc.Open(data)
		if err != nil {
			return fromExtension(name)
		}
		return inspectOPC(pkg)
	case bytes.Contains(data[:min(len(data), 1024)], pdfMagic):
		return PDF
	}
	return fromExtension(name)
}

func isZip(data []byte) bool {
	return bytes.HasPrefix(data, []byte("PK\x03\x04")) || bytes.HasPrefix(data, []byte("PK\x05\x06"))
}

// inspectOPC classifies a package by its officeDocument main part, falling
// back to the conventional main part paths.
func inspectOPC(pkg *opc.Package) Format {
	if rels, err := pkg.Rels(""); err == nil {
		if rel, ok := rels.FirstOfType(opc.RelOfficeDocument); ok {
			if t, err := opc.Resolve("", rel.Target); err == nil {
				if f := mainPartFormat(pkg, t.Path); f != Unknown {
					return f
				}
			}
		}
	}
	for _, kind := range officeKinds {
		if pkg.Has(kind.entry) {
			return kind.format
		}
	}
	return Unknown
}

// mainPartFormat reads a main part's format from its conventional
// directory, then its declared content type, then its root element, so
// relocated main parts classify too.
func mainPartFormat(pkg *opc.Package, main string) Format {
	for _, kind := range officeKinds {
		if strings.HasPrefix(main, kind.dir) {
			return kind.format
		}
	}
	if ct, err := pkg.ContentType(main); err == nil && strings.HasSuffix(strings.ToLower(ct), ".main+xml") {
		ct = strings.ToLower(ct)
		for _, kind := range officeKinds {
			for _, family := range kind.ctFamily {
				if strings.HasPrefix(ct, family) {
					return kind.format
				}
			}
		}
	}
	space, local, err := pkg.RootName(main)
	if err != nil {
		return Unknown
	}
	for _, kind := range officeKinds {
		if space == kind.rootNS && local == kind.root {
			return kind.format
		}
	}
	return Unknown
}

func fromExtension(name string) Format {
	ext := strings.ToLower(path.Ext(name))
	for _, kind := range officeKinds {
		for _, e := range kind.exts {
			if ext == e {
				return kind.format
			}
		}
	}
	switch ext {
	case ".pdf":
		return PDF
	case ".rtf":
		return Rtf
	case ".doc", ".xls", ".ppt", ".pps", ".pot":
		return Ole
	}
	return Unknown
}

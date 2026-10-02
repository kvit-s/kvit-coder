// Package opc reads ZIP-based OOXML packages under fixed resource limits:
// part access, content types, relationships with OPC target resolution, and
// a namespace-aware XML tree.
package opc

import (
	"archive/zip"
	"bytes"
	"encoding/binary"
	"encoding/xml"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

// Fixed, non-configurable package limits. Crossing one returns a
// *model.LimitError, which is always fatal.
const (
	MaxEntryCount = 100_000
	MaxEntryBytes = 128 << 20
	MaxTotalBytes = 512 << 20
	MaxXMLDepth   = 256
	MaxXMLNodes   = 2_000_000
)

var ErrEncrypted = errors.New("opc: package is encrypted")

type MalformedError struct {
	Part   string
	Detail string
}

func (e *MalformedError) Error() string {
	if e.Part == "" {
		return "opc: malformed: " + e.Detail
	}
	return "opc: malformed " + e.Part + ": " + e.Detail
}

type MissingPartError struct {
	Part string
}

func (e *MissingPartError) Error() string {
	return "opc: missing required part " + e.Part
}

type Package struct {
	files     map[string]*zip.File
	cache     map[string][]byte
	totalRead int64
	types     *contentTypes
}

var oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// Open reads the archive directory. An OLE compound file in place of a
// package is either an encrypted OOXML package (ErrEncrypted) or a legacy
// binary document (malformed).
func Open(data []byte) (*Package, error) {
	if bytes.HasPrefix(data, oleMagic) {
		if IsEncrypted(data) {
			return nil, ErrEncrypted
		}
		return nil, &MalformedError{Detail: "OLE compound document where an OOXML package was expected"}
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil && !errors.Is(err, zip.ErrInsecurePath) {
		return nil, &MalformedError{Detail: "not a readable zip archive: " + err.Error()}
	}
	if len(zr.File) > MaxEntryCount {
		return nil, &model.LimitError{Limit: "max_entry_count", Detail: "archive contains " + strconv.Itoa(len(zr.File)) + " entries"}
	}
	p := &Package{files: make(map[string]*zip.File, len(zr.File)), cache: map[string][]byte{}}
	for _, f := range zr.File {
		if _, dup := p.files[f.Name]; !dup {
			p.files[f.Name] = f
		}
	}
	return p, nil
}

// IsEncrypted reports whether data is an OLE compound file holding an
// encrypted OOXML package. Directory entries sit on 128-byte boundaries after
// the 512-byte header whatever the sector size, so the stream names are
// matched there without walking the allocation table.
func IsEncrypted(data []byte) bool {
	if !bytes.HasPrefix(data, oleMagic) {
		return false
	}
	for off := 512; off+128 <= len(data); off += 128 {
		entry := data[off : off+128]
		nameBytes := int(binary.LittleEndian.Uint16(entry[64:]))
		if entry[66] != 2 || nameBytes < 2 || nameBytes > 64 || nameBytes%2 != 0 {
			continue
		}
		units := make([]uint16, nameBytes/2-1)
		for i := range units {
			units[i] = binary.LittleEndian.Uint16(entry[2*i:])
		}
		if name := string(utf16.Decode(units)); name == "EncryptedPackage" || name == "EncryptionInfo" {
			return true
		}
	}
	return false
}

func (p *Package) Has(name string) bool {
	_, ok := p.files[strings.TrimPrefix(name, "/")]
	return ok
}

// Part reads a part's bytes, charging the entry and total decompression
// budgets once per part. An absent part is a *MissingPartError.
func (p *Package) Part(name string) ([]byte, error) {
	name = strings.TrimPrefix(name, "/")
	if data, ok := p.cache[name]; ok {
		return data, nil
	}
	f, ok := p.files[name]
	if !ok {
		return nil, &MissingPartError{Part: name}
	}
	if f.UncompressedSize64 > MaxEntryBytes {
		return nil, &model.LimitError{Limit: "max_entry_bytes", Detail: name + " declares " + strconv.FormatUint(f.UncompressedSize64, 10) + " decompressed bytes"}
	}
	rc, err := f.Open()
	if err != nil {
		return nil, &MalformedError{Part: name, Detail: "unreadable archive entry: " + err.Error()}
	}
	defer func() { _ = rc.Close() }()
	remaining := MaxTotalBytes - p.totalRead
	limit := min(int64(MaxEntryBytes), remaining)
	data, err := io.ReadAll(io.LimitReader(rc, limit+1))
	if int64(len(data)) > limit {
		if remaining < MaxEntryBytes {
			return nil, &model.LimitError{Limit: "max_total_bytes", Detail: name + " exceeds the archive's remaining decompression budget"}
		}
		return nil, &model.LimitError{Limit: "max_entry_bytes", Detail: name + " exceeds the decompression cap"}
	}
	if err != nil {
		return nil, &MalformedError{Part: name, Detail: "corrupt archive entry: " + err.Error()}
	}
	p.totalRead += int64(len(data))
	p.cache[name] = data
	return data, nil
}

// maxRootSniffBytes bounds how much of a part RootName inflates.
const maxRootSniffBytes = 64 << 10

// RootName returns the namespace URI (Strict-normalized) and local name of a
// part's root element, decoding no further than its start tag and at most
// maxRootSniffBytes, outside the decompression budget.
func (p *Package) RootName(name string) (space, local string, err error) {
	name = strings.TrimPrefix(name, "/")
	f, ok := p.files[name]
	if !ok {
		return "", "", &MissingPartError{Part: name}
	}
	rc, err := f.Open()
	if err != nil {
		return "", "", &MalformedError{Part: name, Detail: "unreadable archive entry: " + err.Error()}
	}
	defer func() { _ = rc.Close() }()
	d := xml.NewDecoder(io.LimitReader(rc, maxRootSniffBytes))
	d.Strict = false
	d.CharsetReader = charsetReader
	for {
		tok, err := d.Token()
		if err != nil {
			return "", "", &MalformedError{Part: name, Detail: "no root element within the sniff window: " + err.Error()}
		}
		if start, ok := tok.(xml.StartElement); ok {
			return normalizeURI(start.Name.Space), start.Name.Local, nil
		}
	}
}

// OptionalPart reads a part under the recovery policy: absent yields nil, an
// unreadable part is skipped with a warning, and limit errors propagate.
func (p *Package) OptionalPart(name string) ([]byte, error) {
	data, err := p.Part(name)
	var missing *MissingPartError
	switch {
	case err == nil:
		return data, nil
	case errors.As(err, &missing):
		return nil, nil
	case isFatal(err):
		return nil, err
	}
	model.Log.Warn("skipping unreadable part", "part", name, "err", err)
	return nil, nil
}

// XML reads and parses a part that must exist and parse.
func (p *Package) XML(name string) (*Element, error) {
	data, err := p.Part(name)
	if err != nil {
		return nil, err
	}
	root, err := ParseXML(data)
	var malformed *MalformedError
	if errors.As(err, &malformed) {
		malformed.Part = strings.TrimPrefix(name, "/")
	}
	return root, err
}

// OptionalXML reads and parses a part under the recovery policy.
func (p *Package) OptionalXML(name string) (*Element, error) {
	root, err := p.XML(name)
	var missing *MissingPartError
	switch {
	case err == nil:
		return root, nil
	case errors.As(err, &missing):
		return nil, nil
	case isFatal(err):
		return nil, err
	}
	model.Log.Warn("skipping corrupt part", "part", name, "err", err)
	return nil, nil
}

func isFatal(err error) bool {
	var limit *model.LimitError
	return errors.As(err, &limit)
}

type contentTypes struct {
	defaults  map[string]string
	overrides map[string]string
}

// ContentType returns a part's media type from [Content_Types].xml: an
// override by part name wins over a default by extension, both matched
// case-insensitively. "" when the package declares none.
func (p *Package) ContentType(name string) (string, error) {
	if p.types == nil {
		root, err := p.OptionalXML("[Content_Types].xml")
		if err != nil {
			return "", err
		}
		p.types = &contentTypes{defaults: map[string]string{}, overrides: map[string]string{}}
		if root != nil {
			for e := range root.Elements() {
				ct, _ := e.AnyAttr("ContentType")
				switch e.Local {
				case "Default":
					ext, _ := e.AnyAttr("Extension")
					p.types.defaults[strings.ToLower(ext)] = ct
				case "Override":
					part, _ := e.AnyAttr("PartName")
					p.types.overrides[strings.ToLower(strings.TrimPrefix(part, "/"))] = ct
				}
			}
		}
	}
	name = strings.ToLower(strings.TrimPrefix(name, "/"))
	if ct, ok := p.types.overrides[name]; ok {
		return ct, nil
	}
	return p.types.defaults[strings.TrimPrefix(path.Ext(name), ".")], nil
}

const (
	RelOfficeDocument = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument"
	RelStyles         = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles"
	RelNumbering      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering"
	RelFootnotes      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes"
	RelEndnotes       = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes"
	RelHyperlink      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink"
	RelImage          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	RelSharedStrings  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings"
	RelWorksheet      = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"
	RelSlide          = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slide"
	RelSlideLayout    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout"
	RelSlideMaster    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideMaster"
	RelNotesSlide     = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/notesSlide"
)

// Relationship types are Strict-normalized onto the Rel* constants.
type Relationship struct {
	ID       string
	Type     string
	Target   string
	External bool
}

// Relationships are the relationships of the Source part ("" for the
// package), in document order.
type Relationships struct {
	Source string
	List   []Relationship
}

// Rels reads the relationships of part, or of the package when part is "".
// An absent, unreadable or corrupt rels part yields none; limit errors
// propagate.
func (p *Package) Rels(part string) (Relationships, error) {
	part = strings.TrimPrefix(part, "/")
	rels := Relationships{Source: part}
	dir, file := path.Split(part)
	root, err := p.OptionalXML(dir + "_rels/" + file + ".rels")
	if root == nil {
		return rels, err
	}
	for e := range root.Descendants(NSPackageRelationships, "Relationship") {
		id, hasID := e.AnyAttr("Id")
		target, hasTarget := e.AnyAttr("Target")
		if !hasID || !hasTarget {
			continue
		}
		relType, _ := e.AnyAttr("Type")
		mode, _ := e.AnyAttr("TargetMode")
		rels.List = append(rels.List, Relationship{
			ID:       id,
			Type:     normalizeURI(relType),
			Target:   target,
			External: strings.EqualFold(mode, "External"),
		})
	}
	return rels, nil
}

// ByID returns the relationship with id; the last one wins a duplicated id.
func (r Relationships) ByID(id string) (Relationship, bool) {
	for i := len(r.List) - 1; i >= 0; i-- {
		if r.List[i].ID == id {
			return r.List[i], true
		}
	}
	return Relationship{}, false
}

// FirstOfType returns the internal relationship of relType with the lowest
// id, so duplicates resolve deterministically.
func (r Relationships) FirstOfType(relType string) (Relationship, bool) {
	var best Relationship
	found := false
	for _, rel := range r.List {
		if rel.Type == relType && !rel.External && (!found || rel.ID < best.ID) {
			best, found = rel, true
		}
	}
	return best, found
}

// PartPath resolves the internal target of id against the source part.
func (r Relationships) PartPath(id string) (string, bool) {
	rel, ok := r.ByID(id)
	if !ok || rel.External {
		return "", false
	}
	t, err := Resolve(r.Source, rel.Target)
	if err != nil {
		model.Log.Warn("skipping unresolvable relationship target", "target", rel.Target, "err", err)
		return "", false
	}
	return t.Path, true
}

// PartOfType is the path of the related part of relType, falling back to the
// conventional name next to the source part when the relationship is absent
// or unresolvable.
func (r Relationships) PartOfType(relType, conventional string) string {
	if rel, ok := r.FirstOfType(relType); ok {
		if t, err := Resolve(r.Source, rel.Target); err == nil {
			return t.Path
		}
	}
	t, err := Resolve(r.Source, conventional)
	if err != nil {
		return conventional
	}
	return t.Path
}

// MainPart is the target of the package's officeDocument relationship, or
// conventional when the package has no usable one.
func (p *Package) MainPart(conventional string) (string, error) {
	rels, err := p.Rels("")
	if err != nil {
		return "", err
	}
	return rels.PartOfType(RelOfficeDocument, conventional), nil
}

// Target is a resolved package reference: an archive path without a leading
// slash, and the percent-decoded fragment.
type Target struct {
	Path     string
	Fragment string
}

// Resolve resolves a relative or package-absolute reference against the part
// it appears in. The query is dropped, dot segments clamp at the package
// root, and segments are percent-decoded after splitting; a segment whose
// decoded form is a separator or dot segment is rejected, so encoded
// traversal never becomes path structure.
func Resolve(base, ref string) (Target, error) {
	ref, fragment, _ := strings.Cut(ref, "#")
	t := Target{Fragment: percentDecode(fragment)}
	ref, _, _ = strings.Cut(ref, "?")
	if ref == "" {
		t.Path = base
		return t, nil
	}
	var segments []string
	if !strings.HasPrefix(ref, "/") {
		dir, _ := path.Split(base)
		segments = strings.FieldsFunc(dir, func(r rune) bool { return r == '/' })
	}
	for _, raw := range strings.Split(ref, "/") {
		switch raw {
		case "", ".":
		case "..":
			if len(segments) > 0 {
				segments = segments[:len(segments)-1]
			}
		default:
			seg := percentDecode(raw)
			if strings.ContainsAny(seg, `/\`) || seg == "." || seg == ".." {
				return Target{}, &MalformedError{Detail: "percent-encoded structure in package reference segment " + strconv.Quote(raw)}
			}
			segments = append(segments, seg)
		}
	}
	t.Path = strings.Join(segments, "/")
	return t, nil
}

// percentDecode decodes %XX escapes; a % not followed by two hex digits stays
// literal because producers emit such names.
func percentDecode(s string) string {
	if !strings.Contains(s, "%") {
		return s
	}
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) {
			if b, err := strconv.ParseUint(s[i+1:i+3], 16, 8); err == nil {
				out = append(out, byte(b))
				i += 2
				continue
			}
		}
		out = append(out, s[i])
	}
	return strings.ToValidUTF8(string(out), "�")
}

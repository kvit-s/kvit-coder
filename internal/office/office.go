// Package office converts Word, Excel and PowerPoint files to GitHub-flavored
// Markdown, so that Read can return them as text. It covers the zip-based
// formats Office has written since 2007: .docx, .xlsx and .pptx, their
// macro-enabled and template variants, and .ppsx.
//
// Everything under this directory except this file, office_test.go,
// model/log.go and NOTICE.md is copied from docstomd-go
// (https://github.com/m7medVision/docstomd-go, commit e81e9a43bdde,
// 28 September 2026), which is a Go port of the office converters in
// Firecrawl's anydoc. On the 94 documents both were tried on, the output was
// byte-for-byte identical to anydoc's. Only the office packages were taken;
// docstomd-go's PDF and OCR code, about 40,000 lines, was not.
//
// The changes made to the copy:
//
//   - import paths point here;
//   - the parsers log through model.Log rather than slog's default logger;
//   - the docx tests read the two fixture files they need from
//     formats/docx/testdata rather than upstream's top-level fixture
//     directory, and the opc test that walks that directory skips without it.
//
// To take a newer upstream version, copy internal/{formats,opc,model,render}
// over these directories (leaving out formats/office_bench_test.go), rewrite
// the import prefix github.com/m7medVision/docstomd-go/internal/ to
// github.com/kvit-s/kvit-coder/internal/office/, and reapply the changes above.
package office

import (
	"archive/zip"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"

	"github.com/kvit-s/kvit-coder/internal/office/formats"
	"github.com/kvit-s/kvit-coder/internal/office/formats/docx"
	"github.com/kvit-s/kvit-coder/internal/office/formats/pptx"
	"github.com/kvit-s/kvit-coder/internal/office/formats/xlsx"
	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
	"github.com/kvit-s/kvit-coder/internal/office/render/gfm"
)

// ErrNotOffice is returned for a file that is none of the formats this package
// reads, including zip-based formats that share the Office container, such as
// .vsdx, .xps and NuGet packages.
var ErrNotOffice = errors.New("not a Word, Excel or PowerPoint file")

// ErrEncrypted is returned for a password-protected document. Office stores
// these in a different container from the unprotected kind, so they are
// recognised but cannot be read.
var ErrEncrypted = errors.New("the document is password-protected")

// Result is one converted document.
type Result struct {
	Markdown string
	// Kind names the format for a reader, such as "Word document".
	Kind string
	// Warnings lists parts of the document the parser skipped because they
	// were damaged or referred to something missing. The Markdown is complete
	// apart from those parts.
	Warnings []string
}

// maxWarnings bounds how many skipped parts a result lists. A badly damaged
// file can report one per part, and past a handful the list says nothing more.
const maxWarnings = 10

// convertMu serialises conversions, because the parsers report skipped parts
// through the one package-level logger in model.Log and each conversion needs
// those reports to itself. A conversion takes milliseconds, so two reads in
// one Batch waiting on each other costs nothing noticeable.
var convertMu sync.Mutex

// InspectPackage reports whether the file at path is a zip archive laid out as
// an Office package, and how many bytes of XML it holds once unpacked. It
// reads only the archive's directory, so a large zip that is something else
// is turned away without being read.
//
// The XML size is what predicts the memory a conversion needs: measured on
// spreadsheets, peak memory ran at 11 to 19 times it, against up to 126 times
// the file's size. Pictures and other media are left out of the count, since
// a presentation is often mostly images and costs little to convert.
func InspectPackage(path string) (isPackage bool, xmlBytes int64) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return false, 0
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == "[Content_Types].xml" {
			isPackage = true
		}
		name := strings.ToLower(f.Name)
		if strings.HasSuffix(name, ".xml") || strings.HasSuffix(name, ".rels") {
			xmlBytes += int64(f.UncompressedSize64)
		}
	}
	return isPackage, xmlBytes
}

// IsEncrypted reports whether data is a password-protected Office document.
func IsEncrypted(data []byte) bool {
	return opc.IsEncrypted(data)
}

// Convert turns an Office document into Markdown. The format is decided from
// the content; name is consulted for its extension only when the content does
// not settle it.
func Convert(data []byte, name string) (result Result, err error) {
	if opc.IsEncrypted(data) {
		return Result{}, ErrEncrypted
	}

	var parse func([]byte) (*model.Document, error)
	switch formats.Detect(data, name) {
	case formats.Docx:
		parse, result.Kind = docx.Parse, "Word document"
	case formats.Xlsx:
		parse, result.Kind = xlsx.Parse, "Excel workbook"
	case formats.Pptx:
		parse, result.Kind = pptx.Parse, "PowerPoint presentation"
	default:
		return Result{}, ErrNotOffice
	}

	convertMu.Lock()
	defer convertMu.Unlock()

	warnings := &warningHandler{}
	previous := model.Log
	model.Log = slog.New(warnings)
	defer func() { model.Log = previous }()

	// The parsers are copied code run on files from anywhere. A panic in one
	// would end the whole turn, since the agent is one process per turn, so it
	// is turned into an error about this file instead.
	defer func() {
		if r := recover(); r != nil {
			result, err = Result{}, fmt.Errorf("the converter failed on this file: %v", r)
		}
	}()

	doc, err := parse(data)
	if err != nil {
		return Result{}, describe(err)
	}
	result.Markdown = gfm.Render(doc)
	result.Warnings = warnings.list()
	return result, nil
}

// describe turns the parsers' typed failures into a sentence, because which of
// them happened decides whether there is anything worth trying next.
func describe(err error) error {
	var (
		limit     *model.LimitError
		malformed *opc.MalformedError
		missing   *opc.MissingPartError
	)
	switch {
	case errors.Is(err, opc.ErrEncrypted):
		return ErrEncrypted
	case errors.As(err, &limit):
		return fmt.Errorf("the document is too large to convert safely: %s", limit.Detail)
	case errors.As(err, &missing):
		return fmt.Errorf("the document is incomplete: its %s part is missing", missing.Part)
	case errors.As(err, &malformed):
		if malformed.Part == "" {
			return fmt.Errorf("the file is damaged: %s", malformed.Detail)
		}
		return fmt.Errorf("the file is damaged: %s: %s", malformed.Part, malformed.Detail)
	}
	return err
}

// warningHandler collects warnings as sentences and drops everything quieter.
type warningHandler struct {
	mu    sync.Mutex
	seen  map[string]bool
	items []string
}

func (h *warningHandler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelWarn
}

func (h *warningHandler) Handle(_ context.Context, r slog.Record) error {
	var b strings.Builder
	b.WriteString(r.Message)
	r.Attrs(func(a slog.Attr) bool {
		fmt.Fprintf(&b, " %s=%s", a.Key, a.Value)
		return true
	})
	line := b.String()

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.seen == nil {
		h.seen = map[string]bool{}
	}
	if !h.seen[line] && len(h.items) < maxWarnings {
		h.seen[line] = true
		h.items = append(h.items, line)
	}
	return nil
}

func (h *warningHandler) WithAttrs([]slog.Attr) slog.Handler { return h }
func (h *warningHandler) WithGroup(string) slog.Handler      { return h }

func (h *warningHandler) list() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.items
}

// ReadFile reads a file for Convert, refusing one larger than maxBytes before
// reading it, since the conversion holds the whole file in memory.
func ReadFile(path string, maxBytes int64) ([]byte, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.Size() > maxBytes {
		return nil, fmt.Errorf("the file is %.1f MB, over the %.0f MB limit for conversion", float64(info.Size())/(1<<20), float64(maxBytes)/(1<<20))
	}
	return os.ReadFile(path)
}

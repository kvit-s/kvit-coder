package tools

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/klippa-app/go-pdfium"
	pdfiumerrors "github.com/klippa-app/go-pdfium/errors"
	"github.com/klippa-app/go-pdfium/requests"
	"github.com/klippa-app/go-pdfium/webassembly"
	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/experimental"
)

// Reading a PDF goes through PDFium, the same library Chrome uses to display
// one, compiled to WebAssembly and executed by wazero inside this process.
// Nothing is spawned and nothing is dialed: the compiled module is a byte slice
// embedded in the go-pdfium package, and wazero turns it into machine code in
// memory.
//
// That translation is the only slow part, at about 1.8 seconds. Since the agent
// runs one process per turn it would be paid on every turn that touches a PDF,
// so wazero is given a directory to keep the translated code in and the cost
// drops to about 50ms. Extraction itself is 3-8ms per page.

const (
	// pdfMagic is the header every PDF file starts with.
	pdfMagic = "%PDF-"

	// defaultPDFMaxPages caps how many pages one Read call will extract. Output
	// size binds before this does — a dense page is 4-5KB of text against the
	// 24KB output cap — but the cap keeps a 400-page document from being asked
	// for in one call.
	defaultPDFMaxPages = 10

	// pdfPageWait bounds how long a call waits for the one PDFium instance when
	// another read holds it, as happens when Batch runs two PDF reads together.
	pdfPageWait = 10 * time.Second
)

// looksLikePDF reports whether the file's opening bytes are the PDF header.
//
// The check is on the bytes rather than the extension because the extension
// lies in both directions: of 50 files ending in .pdf used to test this, five
// were HTML error pages saved under that name.
func looksLikePDF(head []byte) bool {
	return len(head) >= len(pdfMagic) && string(head[:len(pdfMagic)]) == pdfMagic
}

// pdfPageRange is the pages one Read call asked for, 1-based and inclusive.
type pdfPageRange struct {
	First int
	Last  int
}

// parsePDFPages reads the pages parameter: "4" for one page, "2-6" for a span,
// empty for "start at the beginning". Last is left at 0 when the caller did not
// name an end, which means "as many as the cap allows".
func parsePDFPages(spec string) (pdfPageRange, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return pdfPageRange{First: 1}, nil
	}

	first, last, hasDash := strings.Cut(spec, "-")
	start, err := strconv.Atoi(strings.TrimSpace(first))
	if err != nil || start < 1 {
		return pdfPageRange{}, fmt.Errorf("pages %q is not a page or page range: use \"3\" or \"2-6\", counting from 1", spec)
	}
	if !hasDash {
		return pdfPageRange{First: start, Last: start}, nil
	}

	// "5-" means from page five onwards, which is what the continuation hint
	// on a truncated read tells the model to send back.
	if strings.TrimSpace(last) == "" {
		return pdfPageRange{First: start}, nil
	}

	end, err := strconv.Atoi(strings.TrimSpace(last))
	if err != nil || end < 1 {
		return pdfPageRange{}, fmt.Errorf("pages %q is not a page range: use \"2-6\", counting from 1", spec)
	}
	if end < start {
		return pdfPageRange{}, fmt.Errorf("pages %q ends before it starts", spec)
	}
	return pdfPageRange{First: start, Last: end}, nil
}

// pdfEngine holds the one PDFium instance this process uses. It is built on
// first use, because a turn that reads no PDF should not pay for it, and it is
// never torn down, because the process exits at the end of the turn.
var pdfEngine struct {
	once sync.Once
	pool pdfium.Pool
	err  error
}

// pdfiumPool starts PDFium, or returns the failure from the first attempt.
func pdfiumPool(cacheDir string) (pdfium.Pool, error) {
	pdfEngine.once.Do(func() {
		config := webassembly.Config{MinIdle: 1, MaxIdle: 1, MaxTotal: 1}

		// The runtime configuration has to be built here rather than left to
		// go-pdfium's default, because passing one replaces that default and
		// the module needs the exception-handling feature to compile at all.
		runtime := wazero.NewRuntimeConfig().
			WithCoreFeatures(api.CoreFeaturesV2 | experimental.CoreFeaturesExceptionHandling)
		if cache, err := pdfCompilationCache(cacheDir); err == nil {
			runtime = runtime.WithCompilationCache(cache)
		}
		config.RuntimeConfig = runtime

		pdfEngine.pool, pdfEngine.err = webassembly.Init(config)
	})
	return pdfEngine.pool, pdfEngine.err
}

// pdfCompilationCache opens the directory wazero keeps translated code in. A
// failure here is not fatal: without it every turn spends about 1.8 seconds
// translating the module instead of about 50 milliseconds reading it back.
func pdfCompilationCache(dir string) (wazero.CompilationCache, error) {
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dir = filepath.Join(home, ".kvit-coder", "wasm-cache")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	return wazero.NewCompilationCacheWithDir(dir)
}

// pdfPage is the text of one page, with the page number it came from.
type pdfPage struct {
	Number int
	Text   string
}

// extractPDFText returns the text of the requested pages, plus how many pages
// the document has. The caller's context bounds the wait for the instance; the
// extraction itself is milliseconds per page.
func extractPDFText(ctx context.Context, pool pdfium.Pool, path string, want pdfPageRange, maxPages int) (pages []pdfPage, totalPages int, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}

	instance, err := pool.GetInstanceWithContext(ctx)
	if err != nil {
		return nil, 0, fmt.Errorf("PDF reader busy: %w", err)
	}
	defer instance.Close()

	doc, err := instance.FPDF_LoadMemDocument(&requests.FPDF_LoadMemDocument{Data: &data})
	if err != nil {
		return nil, 0, describePDFLoadError(err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	count, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return nil, 0, fmt.Errorf("read page count: %w", err)
	}
	totalPages = count.PageCount

	first := want.First
	if first > totalPages {
		return nil, totalPages, fmt.Errorf("page %d requested but the document has %d", first, totalPages)
	}
	last := want.Last
	if last == 0 || last > totalPages {
		last = totalPages
	}
	if last-first+1 > maxPages {
		last = first + maxPages - 1
	}

	for n := first; n <= last; n++ {
		if err := ctx.Err(); err != nil {
			break
		}
		text, err := instance.GetPageText(&requests.GetPageText{
			Page: requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: n - 1}},
		})
		if err != nil {
			return pages, totalPages, fmt.Errorf("read page %d: %w", n, err)
		}
		pages = append(pages, pdfPage{Number: n, Text: text.Text})
	}
	return pages, totalPages, nil
}

// renderPDFPage draws one page into a PNG file under destDir and returns its
// path along with the document's page count.
//
// The page is rendered straight to the size the model will see. Rendering at a
// high resolution and downscaling afterwards would resample twice for nothing,
// because the long-edge cap exists precisely because detail above it buys no
// accuracy.
func renderPDFPage(ctx context.Context, pool pdfium.Pool, srcPath, destDir string, page, maxDim int) (path string, totalPages int, err error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", 0, err
	}

	instance, err := pool.GetInstanceWithContext(ctx)
	if err != nil {
		return "", 0, fmt.Errorf("PDF reader busy: %w", err)
	}
	defer instance.Close()

	doc, err := instance.FPDF_LoadMemDocument(&requests.FPDF_LoadMemDocument{Data: &data})
	if err != nil {
		return "", 0, describePDFLoadError(err)
	}
	defer instance.FPDF_CloseDocument(&requests.FPDF_CloseDocument{Document: doc.Document})

	count, err := instance.FPDF_GetPageCount(&requests.FPDF_GetPageCount{Document: doc.Document})
	if err != nil {
		return "", 0, fmt.Errorf("read page count: %w", err)
	}
	totalPages = count.PageCount
	if page < 1 || page > totalPages {
		return "", totalPages, fmt.Errorf("page %d requested but the document has %d", page, totalPages)
	}

	rendered, err := instance.RenderPageInPixels(&requests.RenderPageInPixels{
		Page:   requests.Page{ByIndex: &requests.PageByIndex{Document: doc.Document, Index: page - 1}},
		Width:  maxDim,
		Height: maxDim,
	})
	if err != nil {
		return "", totalPages, fmt.Errorf("render page %d: %w", page, err)
	}
	// Under WebAssembly the pixels live in the instance's memory and are only
	// valid until this runs, so the encode below has to happen first and this
	// has to happen on every path out.
	defer rendered.Cleanup()

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", totalPages, fmt.Errorf("create image store: %w", err)
	}
	out, err := os.CreateTemp(destDir, fmt.Sprintf("pdfpage-%d-*.png", page))
	if err != nil {
		return "", totalPages, fmt.Errorf("create page image: %w", err)
	}
	defer out.Close()

	if err := png.Encode(out, rendered.Result.RenderedImage); err != nil {
		os.Remove(out.Name())
		return "", totalPages, fmt.Errorf("encode page %d: %w", page, err)
	}
	return out.Name(), totalPages, nil
}

// describePDFLoadError turns PDFium's typed failures into something a model can
// act on, since the difference between a password and a damaged file decides
// whether there is anything worth trying next.
func describePDFLoadError(err error) error {
	switch err {
	case pdfiumerrors.ErrPassword:
		return fmt.Errorf("this PDF is encrypted and needs a password, which Read cannot supply")
	case pdfiumerrors.ErrFormat:
		return fmt.Errorf("this file begins like a PDF but its structure is damaged and cannot be read")
	case pdfiumerrors.ErrFile:
		return fmt.Errorf("this PDF could not be opened: the file is truncated or unreadable")
	default:
		return fmt.Errorf("open PDF: %w", err)
	}
}

// readPDF is the PDF branch of Read. It returns the same shape as the text
// branches — success, path, content, a range and a total — counted in pages
// rather than lines.
func (t *ReadFileTool) readPDF(ctx context.Context, fullPath, path, pagesSpec string) (any, error) {
	want, err := parsePDFPages(pagesSpec)
	if err != nil {
		return map[string]any{
			"success": false,
			"error":   "invalid_pages",
			"path":    path,
			"message": err.Error(),
		}, nil
	}

	maxPages := t.config.Tools.Read.PDF.MaxPages
	if maxPages <= 0 {
		maxPages = defaultPDFMaxPages
	}

	pool, err := pdfiumPool(t.config.Tools.Read.PDF.CacheDir)
	if err != nil {
		return nil, fmt.Errorf("start PDF reader: %w", err)
	}

	waitCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, pdfPageWait)
		defer cancel()
	}

	pages, totalPages, err := extractPDFText(waitCtx, pool, fullPath, want, maxPages)
	if err != nil {
		return map[string]any{
			"success":     false,
			"error":       "pdf_unreadable",
			"path":        path,
			"total_pages": totalPages,
			"message":     err.Error(),
		}, nil
	}
	if len(pages) == 0 {
		return map[string]any{
			"success":     false,
			"error":       "pdf_no_pages_read",
			"path":        path,
			"total_pages": totalPages,
			"message":     "no pages were read before the tool ran out of time",
		}, nil
	}

	return t.formatPDFResult(pages, totalPages, path), nil
}

// formatPDFResult assembles the pages into one string, stopping at the same
// output cap the text branches use, and says how to continue.
func (t *ReadFileTool) formatPDFResult(pages []pdfPage, totalPages int, path string) map[string]any {
	var content strings.Builder
	var withText, lastIncluded int
	truncated := false

	for _, page := range pages {
		text := strings.TrimSpace(page.Text)
		block := fmt.Sprintf("── page %d of %d ──\n%s\n\n", page.Number, totalPages, text)
		if text == "" {
			block = fmt.Sprintf("── page %d of %d ── (no text: this page is an image)\n\n", page.Number, totalPages)
		}
		if content.Len() > 0 && content.Len()+len(block) > t.maxBytes {
			truncated = true
			break
		}
		content.WriteString(block)
		lastIncluded = page.Number
		if text != "" {
			withText++
		}
	}

	response := map[string]any{
		"success":         true,
		"path":            path,
		"content":         strings.TrimRight(content.String(), "\n") + "\n",
		"first_read_page": pages[0].Number,
		"last_read_page":  lastIncluded,
		"total_pages":     totalPages,
		"format_note":     "Text extracted from a PDF. Page markers are added by Read, not part of the document. This file cannot be edited as text.",
	}

	switch {
	case withText == 0:
		// No extractable text does not mean an empty page. A scan holds an
		// image of the text, and a label or a form often draws its lettering as
		// shapes rather than characters. Either way the page can be looked at.
		response["hint"] = fmt.Sprintf("No text to extract from %s: the page is a picture, or its lettering is drawn as shapes. Look at it instead with ReadImage {\"path\": %q, \"page\": %d}.",
			pluralPages(len(pages)), path, pages[0].Number)
	case truncated:
		response["hint"] = fmt.Sprintf("Output limit reached. Read {\"path\": %q, \"pages\": \"%d-\"} to continue.", path, lastIncluded+1)
	case lastIncluded < totalPages:
		response["hint"] = fmt.Sprintf("%d more pages. Read {\"path\": %q, \"pages\": \"%d-\"} to continue.", totalPages-lastIncluded, path, lastIncluded+1)
	}
	return response
}

func pluralPages(n int) string {
	if n == 1 {
		return "this page"
	}
	return fmt.Sprintf("any of these %d pages", n)
}

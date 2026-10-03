package tools

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Read given a file that is not text used to read it as lines anyway, which
// produced a screenful of replacement characters, a byte offset to continue
// from, and nothing the model could act on. There is no reader for most of
// these formats and there should not be: the shell already opens an archive,
// a database or a compiled object, and a tool for each would be a tool
// specification in every request standing in for a one-line command.
//
// So Read names the file instead of reading it, and says which of the tools
// already at hand opens it. The cases that shell cannot cover, images and
// PDFs, are pointed at the tool that can.
//
// Two tiers. A file that is not text at all is refused, because the bytes are
// no use to the model. A file that is text but expensive to read raw, such as
// a notebook whose images are inlined as base64, is read as usual with a note
// about the cheaper way.

// guardHeadBytes is how much of the file the guard looks at. Every signature
// below sits in the first 512 bytes except the tar one at offset 257.
const guardHeadBytes = 512

// readFileHead returns the first n bytes of a file, or fewer if it is shorter.
// Every decision about how to read a file is made from these bytes, so they are
// fetched once and passed along.
func readFileHead(path string, n int) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	head := make([]byte, n)
	read, err := f.Read(head)
	if err != nil && read == 0 {
		if err == io.EOF {
			return nil, nil
		}
		return nil, err
	}
	return head[:read], nil
}

// fileKind is what a file turned out to be, and what opens it.
type fileKind struct {
	// Name is how the file is described to the model, such as "ZIP archive".
	Name string
	// Advice says what to do instead, already carrying the path.
	Advice string
}

// magicSignature is a byte pattern at a known offset that identifies a format.
type magicSignature struct {
	offset int
	prefix []byte
	name   string
	advice func(path string) string
}

// shellHint phrases a suggestion as the Shell call that runs it.
func shellHint(command, path string) string {
	return fmt.Sprintf("Open it with Shell: %s %q", command, path)
}

// magicSignatures are checked in order, so a longer or more specific pattern
// comes before a shorter one that would also match.
var magicSignatures = []magicSignature{
	// Images. No shell command puts pixels into the conversation, so these go
	// to the tool that attaches them to the result.
	{0, []byte("\x89PNG\r\n\x1a\n"), "PNG image", readImageHint},
	{0, []byte("\xff\xd8\xff"), "JPEG image", readImageHint},
	{0, []byte("GIF87a"), "GIF image", readImageHint},
	{0, []byte("GIF89a"), "GIF image", readImageHint},
	{0, []byte("BM"), "BMP image", convertImageHint},
	{0, []byte("II*\x00"), "TIFF image", convertImageHint},
	{0, []byte("MM\x00*"), "TIFF image", convertImageHint},

	// Compressed and archived. Tar is checked before gzip would matter because
	// its signature sits deeper in the file.
	{257, []byte("ustar"), "tar archive", func(p string) string { return shellHint("tar -tf", p) }},
	{0, []byte("\x1f\x8b"), "gzip file", func(p string) string { return shellHint("tar -tzf", p) + ", or gunzip -c for a single compressed file" }},
	{0, []byte("BZh"), "bzip2 file", func(p string) string { return shellHint("tar -tjf", p) }},
	{0, []byte("\xfd7zXZ\x00"), "xz file", func(p string) string { return shellHint("tar -tJf", p) }},
	{0, []byte("\x28\xb5\x2f\xfd"), "zstandard file", func(p string) string { return shellHint("zstd -l", p) }},
	{0, []byte("7z\xbc\xaf\x27\x1c"), "7-Zip archive", func(p string) string { return shellHint("7z l", p) }},
	{0, []byte("Rar!\x1a\x07"), "RAR archive", func(p string) string { return shellHint("unrar l", p) }},

	// Databases and compiled output.
	{0, []byte("SQLite format 3\x00"), "SQLite database", sqliteHint},
	{0, []byte("\x7fELF"), "compiled object or executable (ELF)", compiledHint},
	{0, []byte("\xca\xfe\xba\xbe"), "compiled Java class", func(p string) string { return shellHint("javap -c", p) }},
	{0, []byte("\x00asm"), "WebAssembly module", func(p string) string { return shellHint("wasm-objdump -x", p) }},
	{0, []byte("\xcf\xfa\xed\xfe"), "compiled object or executable (Mach-O)", compiledHint},
	{0, []byte("\xce\xfa\xed\xfe"), "compiled object or executable (Mach-O)", compiledHint},
	{0, []byte("MZ"), "Windows executable or library", compiledHint},

	// Fonts and media, where the useful answer is just the name.
	{0, []byte("wOFF"), "WOFF font", nothingToReadHint},
	{0, []byte("wOF2"), "WOFF2 font", nothingToReadHint},
	{0, []byte("OTTO"), "OpenType font", nothingToReadHint},
	{0, []byte("OggS"), "Ogg media file", nothingToReadHint},
	{0, []byte("ID3"), "MP3 audio file", nothingToReadHint},
	{4, []byte("ftyp"), "MP4 or QuickTime media file", nothingToReadHint},

	// Text in an encoding the line reader cannot make sense of. These have to
	// be named, because a byte-order mark plus NUL padding otherwise reaches
	// the generic binary case and the model is told nothing useful.
	{0, []byte("\xff\xfe\x00\x00"), "UTF-32 text (little endian)", iconvHint},
	{0, []byte("\x00\x00\xfe\xff"), "UTF-32 text (big endian)", iconvHint},
	{0, []byte("\xff\xfe"), "UTF-16 text (little endian)", iconvHint},
	{0, []byte("\xfe\xff"), "UTF-16 text (big endian)", iconvHint},
}

func readImageHint(path string) string {
	return fmt.Sprintf("Use ReadImage to see it: ReadImage {\"path\": %q}", path)
}

// convertImageHint is for the image formats ReadImage does not accept. It
// gives the conversion as a command, because "convert it first" without one
// leaves the model to guess which of several tools is installed.
func convertImageHint(path string) string {
	converted := strings.TrimSuffix(path, filepath.Ext(path)) + ".png"
	return fmt.Sprintf("ReadImage accepts PNG, JPEG and GIF. Convert it with Shell first — %s — then ReadImage {\"path\": %q}",
		fmt.Sprintf("ffmpeg -loglevel error -i %q %q, or magick %q %q", path, converted, path, converted),
		converted)
}

func sqliteHint(path string) string {
	return fmt.Sprintf("List its tables with Shell: sqlite3 %q .schema", path)
}

func compiledHint(path string) string {
	return fmt.Sprintf("It is build output rather than source, so read the source it was built from. To inspect the file itself, Shell has nm, objdump and strings %q.", path)
}

// pycHint covers compiled Python, which is worth naming because the source it
// came from is almost always sitting next to it.
func pycHint(path string) string {
	return fmt.Sprintf("It is compiled Python. Read the .py file it was built from, or disassemble it with Shell: python3 -m dis %q", path)
}

func nothingToReadHint(path string) string {
	return "There is no text in this file to read."
}

func iconvHint(path string) string {
	return fmt.Sprintf("Convert it to UTF-8 first with Shell: iconv -f UTF-16 -t UTF-8 %q", path)
}

// zipMembers describes the formats that are a ZIP file underneath, which share
// one signature and are told apart by their extension. Naming the real format
// matters because the way in differs: a wheel is listed, a Word document has
// one member worth extracting.
var zipMembers = map[string]fileKind{
	".docx": {"Word document (a ZIP container)", "Extract its text with Shell: unzip -p %q word/document.xml | sed -e 's/<[^>]*>/ /g'"},
	".xlsx": {"Excel workbook (a ZIP container)", "Its sheets are XML inside the archive. Start with Shell: unzip -l %q"},
	".pptx": {"PowerPoint deck (a ZIP container)", "Its slides are XML inside the archive. Start with Shell: unzip -l %q"},
	".odt":  {"OpenDocument text (a ZIP container)", "Extract its text with Shell: unzip -p %q content.xml | sed -e 's/<[^>]*>/ /g'"},
	".ods":  {"OpenDocument spreadsheet (a ZIP container)", "Start with Shell: unzip -p %q content.xml"},
	".jar":  {"Java archive", "List its contents with Shell: unzip -l %q"},
	".war":  {"Java web archive", "List its contents with Shell: unzip -l %q"},
	".whl":  {"Python wheel", "List its contents with Shell: unzip -l %q"},
	".apk":  {"Android package", "List its contents with Shell: unzip -l %q"},
	".epub": {"EPUB book", "List its contents with Shell: unzip -l %q"},
}

// legacyOffice describes the binary formats Office used before 2007. They all
// share one container, the compound file, so the extension is what tells them
// apart. Each is named with the newer format Read converts and the catdoc
// program that prints the same kind of file as text.
var legacyOffice = map[string]struct{ name, modern, textCmd string }{
	".doc": {"Word 97-2003 document", "docx", "catdoc"},
	".dot": {"Word 97-2003 template", "docx", "catdoc"},
	".xls": {"Excel 97-2003 workbook", "xlsx", "xls2csv"},
	".xlt": {"Excel 97-2003 template", "xlsx", "xls2csv"},
	".ppt": {"PowerPoint 97-2003 presentation", "pptx", "catppt"},
	".pps": {"PowerPoint 97-2003 slide show", "pptx", "catppt"},
	".pot": {"PowerPoint 97-2003 template", "pptx", "catppt"},
}

// classifyLegacyOffice names a pre-2007 Office file, which classifyNotText
// would otherwise call an unknown binary file. Read cannot convert these, so
// the advice is a conversion to the newer format, written into outDir where
// Read can open it, with catdoc's text dump for a machine without LibreOffice.
func classifyLegacyOffice(path string, head []byte, outDir string) (fileKind, bool) {
	if !hasMagicAt(head, 0, oleMagic) {
		return fileKind{}, false
	}
	format, ok := legacyOffice[strings.ToLower(filepath.Ext(path))]
	if !ok {
		return fileKind{}, false
	}
	base := filepath.Base(path)
	converted := filepath.Join(outDir, strings.TrimSuffix(base, filepath.Ext(base))+"."+format.modern)
	return fileKind{
		Name: format.name,
		Advice: fmt.Sprintf("Read converts the newer .%s format. Convert this file with Shell: soffice --headless --convert-to %s --outdir %q %q, then Read {\"path\": %q}. Without LibreOffice, %s %q prints its text.",
			format.modern, format.modern, outDir, path, converted, format.textCmd, path),
	}, true
}

// classifyNotText says what a file is when it is not text Read can return. The
// second result is false for anything that should be read as usual.
func classifyNotText(path string, head []byte) (fileKind, bool) {
	// ZIP comes first: several document formats share its signature and are
	// separated by extension, so the generic answer is the fallback.
	if hasMagicAt(head, 0, []byte("PK\x03\x04")) || hasMagicAt(head, 0, []byte("PK\x05\x06")) {
		if kind, ok := zipMembers[strings.ToLower(filepath.Ext(path))]; ok {
			return fileKind{Name: kind.Name, Advice: fmt.Sprintf(kind.Advice, path)}, true
		}
		return fileKind{Name: "ZIP archive", Advice: shellHint("unzip -l", path)}, true
	}

	// WebP is RIFF with a tag four bytes further in, so it cannot be one
	// prefix match like the formats above.
	if hasMagicAt(head, 0, []byte("RIFF")) && hasMagicAt(head, 8, []byte("WEBP")) {
		return fileKind{
			Name:   "WebP image",
			Advice: readImageHint(path),
		}, true
	}

	// Compiled Python has a magic number that changes with the interpreter
	// version, so the stable part is the carriage return and newline in its
	// third and fourth bytes. The extension keeps that weak pattern from
	// matching anything else.
	if strings.EqualFold(filepath.Ext(path), ".pyc") && len(head) >= 4 && head[2] == '\r' && head[3] == '\n' {
		return fileKind{Name: "compiled Python bytecode", Advice: pycHint(path)}, true
	}

	for _, sig := range magicSignatures {
		if hasMagicAt(head, sig.offset, sig.prefix) {
			return fileKind{Name: sig.name, Advice: sig.advice(path)}, true
		}
	}

	// Anything left holding a NUL byte is not text, whatever it is. Saying so
	// is still better than returning the bytes.
	if bytes.IndexByte(head, 0) >= 0 {
		return fileKind{
			Name:   "binary file",
			Advice: fmt.Sprintf("If you need its bytes, ask for them: Read {\"path\": %q, \"char_mode\": true}", path),
		}, true
	}

	return fileKind{}, false
}

func hasMagicAt(head []byte, offset int, prefix []byte) bool {
	if len(head) < offset+len(prefix) {
		return false
	}
	return bytes.Equal(head[offset:offset+len(prefix)], prefix)
}

// notTextResult is what Read returns instead of a file it cannot usefully read.
func notTextResult(path string, size int64, kind fileKind) map[string]any {
	return map[string]any{
		"success":   false,
		"error":     "not_text",
		"path":      path,
		"file_type": kind.Name,
		"size":      size,
		"message":   fmt.Sprintf("%s is a %s (%s), so there is nothing to read as text.", path, kind.Name, formatFileSize(size)),
		"hint":      kind.Advice,
	}
}

// notebookHint is the advice attached to a Jupyter notebook. The file is JSON,
// so Read can return it, but the images an executed notebook carries are
// inlined as base64 and dwarf the code: one 262KB notebook measured here held
// 3,795 characters of source against 246,920 characters of encoded images.
// Read returns it as usual and says how to get the code on its own.
func notebookHint(path string) string {
	return fmt.Sprintf("This is a Jupyter notebook, and most of its bytes are usually base64 images rather than code. For the code alone, use Shell: jq -r '.cells[] | select(.cell_type==\"code\") | .source | join(\"\")' %q", path)
}

// isNotebook reports whether the file is a Jupyter notebook, by extension and
// by the key every notebook has, so a file merely named .ipynb is not annotated
// with advice that would not work on it.
func isNotebook(path string, head []byte) bool {
	if !strings.EqualFold(filepath.Ext(path), ".ipynb") {
		return false
	}
	return bytes.Contains(head, []byte("\"cells\""))
}

// formatFileSize renders a size the way a person would write it.
func formatFileSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1f GB", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1f MB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1f KB", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%d bytes", n)
	}
}

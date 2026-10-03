package tools

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office"
)

// A Word, Excel or PowerPoint file is a zip archive of XML, so reading it line
// by line returns compressed bytes. Read converts it to Markdown instead,
// through internal/office, and saves the Markdown in the session's tmp/ folder.
// Every read of the document, including the continuation of a long one, is
// then an ordinary line read of that saved text, so start, limit and the
// truncation hints behave as they do for any text file.
//
// The saved file is named from the document's path, size and modification
// time, which makes paging through a long spreadsheet one conversion rather
// than one per page, and makes an edited document convert again.

// maxOfficeXMLBytes is the most XML, once unpacked, that Read will convert.
// Peak memory ran at 11 to 19 times this figure on real spreadsheets (a 2 MB
// workbook unpacks to 14 MB and peaked at 261 MB), so the cap keeps the worst
// case near 300 MB. Every spreadsheet on the machine it was measured on fits.
var maxOfficeXMLBytes int64 = 16 << 20

// maxOfficeFileBytes bounds the file itself, which is read into memory whole.
// It is far above maxOfficeXMLBytes because a presentation is mostly pictures:
// a 5.4 MB deck peaked at 35 MB.
var maxOfficeFileBytes int64 = 64 << 20

// maxOutlineEntries bounds the heading list returned with a long document. A
// workbook with hundreds of sheets would otherwise spend the output on names.
const maxOutlineEntries = 60

var oleMagic = []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// officeExtensions are the names a password-protected document is recognised
// by. Its container is the same one the older binary formats use, so for
// these, unlike unprotected ones, the extension is the only way to tell.
var officeExtensions = map[string]bool{
	".docx": true, ".docm": true, ".dotx": true, ".dotm": true,
	".xlsx": true, ".xlsm": true, ".xltx": true, ".xltm": true,
	".pptx": true, ".pptm": true, ".potx": true, ".potm": true, ".ppsx": true, ".ppsm": true,
}

// readOffice is the Office branch of Read. handled is false when the file is
// not an Office document, and Read goes on to treat it as it otherwise would.
func (t *ReadFileTool) readOffice(fullPath, path string, info os.FileInfo, head []byte, start, limit *int) (result any, handled bool, err error) {
	isZip := bytes.HasPrefix(head, []byte("PK\x03\x04")) || bytes.HasPrefix(head, []byte("PK\x05\x06"))
	if bytes.HasPrefix(head, oleMagic) && officeExtensions[strings.ToLower(filepath.Ext(path))] {
		return t.readProtectedOffice(fullPath, path, info)
	}
	if !isZip {
		return nil, false, nil
	}
	isPackage, xmlBytes := office.InspectPackage(fullPath)
	if !isPackage {
		return nil, false, nil
	}

	convertedPath, kind, warnings, failure := t.convertOffice(fullPath, path, info, xmlBytes)
	if failure != nil {
		return failure, true, nil
	}
	if convertedPath == "" {
		return nil, false, nil
	}

	read, err := t.readLineMode(convertedPath, start, limit, path)
	if err != nil {
		return nil, true, err
	}
	response, ok := read.(map[string]any)
	if !ok {
		return read, true, nil
	}

	response["converted_from"] = kind
	response["converted_path"] = convertedPath
	note := fmt.Sprintf("Read converted this %s to Markdown, so the content is not the file's bytes and line numbers count lines of the converted text. The document cannot be edited as text. The converted text is saved at converted_path, where Search and Shell can use it.", kind)
	if existing, _ := response["format_note"].(string); existing != "" {
		note += " " + existing
	}
	response["format_note"] = note

	// A line too long for one response is continued in char_mode, and
	// char_mode on the document itself returns its zip bytes. The converted
	// text is the file that hint has to name.
	if _, partial := response["first_read_byte"]; partial {
		next, _ := response["last_read_byte"].(int64)
		response["hint"] = fmt.Sprintf("A line here is too long to return whole. Continue in the converted text: Read {\"path\": %q, \"char_mode\": true, \"start\": %d}", convertedPath, next+1)
	}

	if len(warnings) > 0 {
		response["warnings"] = warnings
	}
	last, _ := response["last_read_line"].(int)
	if total, _ := response["total_lines"].(int); start == nil && last < total {
		if outline := t.officeOutline(convertedPath); len(outline) > 0 {
			response["outline"] = outline
		}
	}
	return response, true, nil
}

// convertOffice returns the path of the document's converted text, converting
// it first unless an earlier read already did. failure is set when the file is
// an Office document that cannot be converted; an empty path with no failure
// means it was not an Office document after all.
func (t *ReadFileTool) convertOffice(fullPath, path string, info os.FileInfo, xmlBytes int64) (convertedPath, kind string, warnings []string, failure map[string]any) {
	dir := t.officeDir()
	key := sha256.Sum256(fmt.Appendf(nil, "%s\x00%d\x00%d", fullPath, info.Size(), info.ModTime().UnixNano()))
	base := "office-" + hex.EncodeToString(key[:])[:12]

	for _, k := range []string{"Word document", "Excel workbook", "PowerPoint presentation"} {
		candidate := filepath.Join(dir, base+"-"+kindSlug(k)+".md")
		if _, err := os.Stat(candidate); err == nil {
			return candidate, k, nil, nil
		}
	}

	if xmlBytes > maxOfficeXMLBytes {
		return "", "", nil, officeFailure(path, info, "", fmt.Errorf("its text is %.1f MB of XML once unpacked, over the %.0f MB limit for conversion",
			float64(xmlBytes)/(1<<20), float64(maxOfficeXMLBytes)/(1<<20)))
	}
	data, err := office.ReadFile(fullPath, maxOfficeFileBytes)
	if err != nil {
		return "", "", nil, officeFailure(path, info, "", err)
	}
	converted, err := office.Convert(data, path)
	switch {
	case errors.Is(err, office.ErrNotOffice):
		return "", "", nil, nil
	case err != nil:
		return "", "", nil, officeFailure(path, info, "", err)
	}

	convertedPath = filepath.Join(dir, base+"-"+kindSlug(converted.Kind)+".md")
	if err := writeFileAtomic(convertedPath, []byte(converted.Markdown)); err != nil {
		return "", "", nil, officeFailure(path, info, converted.Kind, fmt.Errorf("converted, but could not save the result: %w", err))
	}
	return convertedPath, converted.Kind, converted.Warnings, nil
}

// officeDir is where converted documents go: the session's tmp/ folder, which
// the model may read without being asked about, or a fixed folder under the
// system's temporary directory when there is no session.
func (t *ReadFileTool) officeDir() string {
	if dir := t.toolCtx.SessionTmp(); dir != "" {
		return dir
	}
	return filepath.Join(os.TempDir(), "kvit-coder-office")
}

// readProtectedOffice answers for a password-protected document, which would
// otherwise be described as an unknown binary file.
func (t *ReadFileTool) readProtectedOffice(fullPath, path string, info os.FileInfo) (any, bool, error) {
	data, err := office.ReadFile(fullPath, maxOfficeFileBytes)
	if err != nil || !office.IsEncrypted(data) {
		return nil, false, nil
	}
	return map[string]any{
		"success": false,
		"error":   "office_encrypted",
		"path":    path,
		"size":    info.Size(),
		"message": fmt.Sprintf("%s is password-protected. Read cannot open it, and no command at hand can either without the password.", path),
	}, true, nil
}

// officeFailure describes a document that is an Office file but could not be
// converted. The hint is the one Read gave before it converted these files,
// which still gets at the text of a damaged one.
func officeFailure(path string, info os.FileInfo, kind string, err error) map[string]any {
	if errors.Is(err, office.ErrEncrypted) {
		return map[string]any{
			"success": false,
			"error":   "office_encrypted",
			"path":    path,
			"size":    info.Size(),
			"message": fmt.Sprintf("%s is password-protected. Read cannot open it.", path),
		}
	}
	if kind == "" {
		kind = "Office document"
	}
	result := map[string]any{
		"success":   false,
		"error":     "office_unreadable",
		"path":      path,
		"file_type": kind,
		"size":      info.Size(),
		"message":   fmt.Sprintf("%s could not be converted to text: %v", path, err),
	}
	if fallback, ok := zipMembers[strings.ToLower(filepath.Ext(path))]; ok {
		result["hint"] = fmt.Sprintf(fallback.Advice, path)
	} else {
		result["hint"] = shellHint("unzip -l", path)
	}
	return result
}

// officeOutline lists the converted text's headings with their line numbers,
// so that the model can go straight to a section or a worksheet of a document
// too long to return at once.
func (t *ReadFileTool) officeOutline(convertedPath string) []outlineEntry {
	data, err := os.ReadFile(convertedPath)
	if err != nil {
		return nil
	}
	outline := buildOutline(string(data))
	if len(outline) > maxOutlineEntries {
		outline = outline[:maxOutlineEntries]
	}
	return outline
}

func kindSlug(kind string) string {
	switch kind {
	case "Excel workbook":
		return "xlsx"
	case "PowerPoint presentation":
		return "pptx"
	default:
		return "docx"
	}
}

// writeFileAtomic writes through a temporary file and a rename, so a reader in
// the same Batch never finds a half-written conversion under the final name.
func writeFileAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".office-*.tmp")
	if err != nil {
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmp.Name())
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	if err := os.Rename(tmp.Name(), path); err != nil {
		os.Remove(tmp.Name())
		return err
	}
	return nil
}

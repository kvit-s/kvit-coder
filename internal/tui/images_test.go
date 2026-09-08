package tui

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExtractImageRefs(t *testing.T) {
	dir := t.TempDir()
	shot := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(shot, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "note.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got := ExtractImageRefs("look at @"+shot+" please", nil)
	if len(got) != 1 || got[0] != shot {
		t.Errorf("@ref = %v", got)
	}
	// Bare drag-drop path, quoted.
	got = ExtractImageRefs(`see "`+shot+`" now`, nil)
	if len(got) != 1 {
		t.Errorf("quoted = %v", got)
	}
	// Non-images and missing files are ignored.
	got = ExtractImageRefs("read @"+filepath.Join(dir, "note.txt")+" and @/tmp/does-not-exist.png", nil)
	if len(got) != 0 {
		t.Errorf("non-images = %v", got)
	}
	// Duplicates collapse.
	got = ExtractImageRefs("@"+shot+" @"+shot, nil)
	if len(got) != 1 {
		t.Errorf("dedup = %v", got)
	}
}

func TestSplitImageFieldsQuotes(t *testing.T) {
	fields := splitImageFields(`:image "C:\My Photos\shot.png" a.png`)
	if len(fields) != 3 || fields[1] != `C:\My Photos\shot.png` {
		t.Errorf("fields = %q", fields)
	}
}

func TestReadClipboardImageNone(t *testing.T) {
	old := cmdRunner
	defer func() { cmdRunner = old }()
	oldWSL := wslDetect
	defer func() { wslDetect = oldWSL }()
	wslDetect = func() bool { return true }
	cmdRunner = func(string, ...string) ([]byte, error) {
		return nil, errors.New("no such helper")
	}
	if _, _, err := ReadClipboardImage(); err == nil {
		t.Error("expected an error with no helpers")
	} else if !strings.Contains(err.Error(), "no image in clipboard") {
		t.Errorf("error = %q", err)
	}
}

func TestReadClipboardImageWayland(t *testing.T) {
	old := cmdRunner
	defer func() { cmdRunner = old }()
	pngHead := append([]byte("PNG"), make([]byte, 600)...)
	// Make DetectContentType agree: real PNG magic.
	pngHead[0], pngHead[1], pngHead[2], pngHead[3] = 0x89, 'P', 'N', 'G'
	pngHead[4], pngHead[5], pngHead[6], pngHead[7] = 0x0d, 0x0a, 0x1a, 0x0a
	cmdRunner = func(name string, args ...string) ([]byte, error) {
		if name == "wl-paste" && len(args) == 1 {
			return []byte("image/png\ntext/plain"), nil
		}
		if name == "wl-paste" {
			return pngHead, nil
		}
		return nil, errors.New("unused")
	}
	data, ext, err := ReadClipboardImage()
	if err != nil {
		t.Fatalf("clipboard: %v", err)
	}
	if ext != ".png" || len(data) == 0 {
		t.Errorf("got ext=%q len=%d", ext, len(data))
	}
}

func TestStageClipboardImageRejectsText(t *testing.T) {
	old := cmdRunner
	defer func() { cmdRunner = old }()
	cmdRunner = func(string, ...string) ([]byte, error) {
		out := make([]byte, 600)
		copy(out, "hello world, this is plain text from the clipboard helper")
		return out, nil
	}
	// wl-paste list-types fails, xclip returns text: not image bytes, so the
	// Windows branch runs powershell which also fails here.
	if _, err := StageClipboardImage(); err == nil {
		t.Error("expected an error for non-image clipboard")
	}
}

func TestReadWindowsClipboardImageBase64(t *testing.T) {
	old := cmdRunner
	defer func() { cmdRunner = old }()
	oldWSL := wslDetect
	defer func() { wslDetect = oldWSL }()
	wslDetect = func() bool { return true }
	// 1x1 PNG with PowerShell-style CRLF around the payload.
	const tinyB64 = "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=="
	cmdRunner = func(name string, args ...string) ([]byte, error) {
		if name != "powershell.exe" {
			return nil, errors.New("no such helper")
		}
		return []byte(tinyB64 + "\r\n"), nil
	}
	data, ext, err := ReadClipboardImage()
	if err != nil {
		t.Fatalf("clipboard: %v", err)
	}
	if ext != ".png" {
		t.Errorf("ext = %q, want .png", ext)
	}
	if len(data) < 8 || string(data[:8]) != "\x89PNG\r\n\x1a\n" {
		t.Errorf("not a PNG: % x", data[:8])
	}
}

func TestReadWindowsClipboardImageEmpty(t *testing.T) {
	old := cmdRunner
	defer func() { cmdRunner = old }()
	oldWSL := wslDetect
	defer func() { wslDetect = oldWSL }()
	wslDetect = func() bool { return true }
	cmdRunner = func(name string, args ...string) ([]byte, error) {
		if name != "powershell.exe" && name != "pwsh.exe" {
			return nil, errors.New("no such helper")
		}
		return []byte("\r\n"), nil // no image: helper prints nothing
	}
	if _, _, err := ReadClipboardImage(); err == nil {
		t.Error("expected an error for an empty clipboard")
	}
}

func TestDecodeClipboardPayloadRejectsText(t *testing.T) {
	if _, ok := decodeClipboardPayload([]byte("hello world")); ok {
		t.Error("plain text decoded as an image")
	}
	if _, ok := decodeClipboardPayload([]byte("")); ok {
		t.Error("empty payload decoded as an image")
	}
}

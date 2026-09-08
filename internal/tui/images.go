package tui

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// splitImageFields splits input on whitespace, keeping quoted sections
// together, so a drag-dropped "C:\My Photos\shot.png" stays one token.
func splitImageFields(input string) []string {
	var fields []string
	var b strings.Builder
	var quote rune
	flush := func() {
		if b.Len() > 0 {
			fields = append(fields, b.String())
			b.Reset()
		}
	}
	for _, r := range input {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				b.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote = r
		case r == ' ' || r == '\t' || r == '\n':
			flush()
		default:
			b.WriteRune(r)
		}
	}
	flush()
	return fields
}

// ExtractImageRefs returns the image files an input line refers to: @paths,
// quoted drag-drop paths, and bare paths that exist on disk with an image
// extension. The text is left untouched — the model sees "@shot.png" and the
// pixels together. Paths are deduplicated, order kept.
func ExtractImageRefs(input string, exists func(string) bool) []string {
	if exists == nil {
		exists = func(p string) bool {
			_, err := os.Stat(p)
			return err == nil
		}
	}
	var out []string
	seen := map[string]bool{}
	for _, field := range splitImageFields(input) {
		ref := strings.TrimLeft(field, "@")
		if ref == "" {
			continue
		}
		clean := llm.NormalizeImagePath(ref)
		if !llm.IsImagePath(clean) || !exists(clean) {
			continue
		}
		if !seen[clean] {
			seen[clean] = true
			out = append(out, clean)
		}
	}
	return out
}

// cmdRunner runs an external clipboard helper. A variable (not a plain func)
// so tests can stub it. The default bounds every helper, so a hung reader
// (a PowerShell cold start, a Wayland prompt) cannot freeze paste.
var cmdRunner = func(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// wslDetect reports whether this process runs under WSL. A variable so tests
// can drive the Windows fallback without Windows.
var wslDetect = llm.IsWSL

// ReadClipboardImage returns raw image bytes from the system clipboard, plus
// a file extension for them. It probes the platform helpers in order —
// Wayland, X11, then (on WSL) Windows via interop — and reports plainly when
// none has an image, which is the common case over SSH.
func ReadClipboardImage() (data []byte, ext string, err error) {
	if out, err := cmdRunner("wl-paste", "--list-types"); err == nil {
		for _, mime := range []string{"image/png", "image/jpeg"} {
			if strings.Contains(string(out), mime) {
				if data, err := cmdRunner("wl-paste", "--type", mime); err == nil && len(data) > 0 {
					return data, mimeExt(mime), nil
				}
			}
		}
	}
	if data, err := cmdRunner("xclip", "-selection", "clipboard", "-t", "image/png", "-o"); err == nil && isImageBytes(data) {
		return data, ".png", nil
	}
	if wslDetect() {
		if data, err := readWindowsClipboardImage(); err == nil {
			return data, ".png", nil
		}
	}
	return nil, "", fmt.Errorf("no image in clipboard (tried wl-paste, xclip, Windows clipboard)")
}
func mimeExt(mime string) string {
	if mime == "image/jpeg" {
		return ".jpg"
	}
	return ".png"
}

func isImageBytes(data []byte) bool {
	if len(data) < 512 {
		return false
	}
	ct := http.DetectContentType(data[:512])
	return ct == "image/png" || ct == "image/jpeg" || ct == "image/gif"
}

// windowsImagePS prints the Windows clipboard image as one Base64 PNG line,
// or nothing when the clipboard holds no image. The pixels travel over the
// helper's stdout, so there is no temp file to map through /mnt and clean up.
const windowsImagePS = "Add-Type -AssemblyName System.Windows.Forms; Add-Type -AssemblyName System.Drawing; " +
	"$img=[System.Windows.Forms.Clipboard]::GetImage(); if($null -eq $img){exit 0}; " +
	"$ms=New-Object System.IO.MemoryStream; $img.Save($ms,[System.Drawing.Imaging.ImageFormat]::Png); " +
	"[Convert]::ToBase64String($ms.ToArray())"

// maxClipboardPayload caps the Base64 payload decoded from the Windows
// clipboard, so a hostile paste cannot grow the process without limit.
const maxClipboardPayload = 64 << 20

// windowsShells are the Windows shells reachable from WSL via interop, in
// probe order.
var windowsShells = []string{"powershell.exe", "pwsh.exe"}

// readWindowsClipboardImage fetches the Windows clipboard image through
// interop. Under WSL the Linux clipboard tools see only the Linux side (and
// the terminal never delivers image bytes on Ctrl+V at all), so the pixels
// are pulled from Windows instead. Only called after the Linux helpers miss.
func readWindowsClipboardImage() ([]byte, error) {
	for _, shell := range windowsShells {
		out, err := cmdRunner(shell, "-NoProfile", "-NonInteractive", "-Command", windowsImagePS)
		if err != nil {
			continue // shell missing or failed: try the next one
		}
		if data, ok := decodeClipboardPayload(out); ok {
			return data, nil
		}
		// Empty output means no image, not a broken shell: stop probing.
		if len(trimSpace(out)) == 0 {
			return nil, fmt.Errorf("no image in Windows clipboard")
		}
	}
	return nil, fmt.Errorf("no image in Windows clipboard")
}

// decodeClipboardPayload turns the helper's stdout into image bytes:
// surrounding whitespace (PowerShell line endings) is stripped, the rest
// must be Base64 decoding to sniffable image bytes.
func decodeClipboardPayload(out []byte) ([]byte, bool) {
	compact := make([]byte, 0, len(out))
	for _, b := range out {
		if b != ' ' && b != '\t' && b != '\r' && b != '\n' {
			compact = append(compact, b)
		}
	}
	if len(compact) == 0 || len(compact) > maxClipboardPayload {
		return nil, false
	}
	data, err := base64.StdEncoding.DecodeString(string(compact))
	if err != nil || len(data) == 0 {
		return nil, false
	}
	if !isImageBytes(padForSniff(data)) {
		return nil, false
	}
	return data, true
}

func trimSpace(b []byte) []byte {
	return []byte(strings.TrimSpace(string(b)))
}

// padForSniff extends short payloads so DetectContentType can sniff them;
// isImageBytes otherwise refuses anything under 512 bytes.
func padForSniff(data []byte) []byte {
	if len(data) >= 512 {
		return data
	}
	padded := make([]byte, 512)
	copy(padded, data)
	return padded
}

// SaveClipboardTemp stores pasted bytes where the agent will find them. The
// agent copies -image files into the session on arrival, so plain temp is
// enough here — the UI does not need the session directory.
func SaveClipboardTemp(data []byte, ext string) (string, error) {
	if ext == "" {
		ext = ".png"
	}
	f, err := os.CreateTemp("", "kvit-coder-paste-*"+ext)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := f.Write(data); err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// StageClipboardImage reads the clipboard and stages it as a file, returning
// the staged path for the UI to attach to the next turn.
func StageClipboardImage() (string, error) {
	data, ext, err := ReadClipboardImage()
	if err != nil {
		return "", err
	}
	path, err := SaveClipboardTemp(data, ext)
	if err != nil {
		return "", err
	}
	// Guard against a helper handing back text: only keep real images.
	head, _ := os.ReadFile(path)
	if len(head) >= 512 {
		head = head[:512]
	}
	if ct := http.DetectContentType(head); ct != "image/png" && ct != "image/jpeg" && ct != "image/gif" {
		os.Remove(path)
		return "", fmt.Errorf("clipboard contents are %s, not an image", ct)
	}
	return path, nil
}

package tui

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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
// so tests can stub it.
var cmdRunner = func(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// ReadClipboardImage returns raw image bytes from the system clipboard, plus
// a file extension for them. It probes the platform helpers in order —
// Wayland, X11, then Windows via interop — and reports plainly when none has
// an image, which is the common case over SSH.
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
	if data, err := readWindowsClipboardImage(); err == nil {
		return data, ".png", nil
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

// readWindowsClipboardImage fetches the Windows clipboard image through
// interop. Under WSL the Linux clipboard tools see only the Linux side, so
// this shells out to powershell.exe: save the bitmap to a Windows temp file,
// read it back through the /mnt mapping, and remove both copies.
func readWindowsClipboardImage() ([]byte, error) {
	name := fmt.Sprintf("kvit-clipboard-%d.png", os.Getpid())
	winTempOut, err := cmdRunner("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", "$env:TEMP")
	if err != nil {
		return nil, err
	}
	winTemp := strings.TrimSpace(string(winTempOut)) + "\\" + name
	save := fmt.Sprintf(
		`Add-Type -AssemblyName System.Windows.Forms; `+
			`$i=[System.Windows.Forms.Clipboard]::GetImage(); `+
			`if($null -eq $i){exit 42}; `+
			`$i.Save('%s','Png')`, winTemp)
	if _, err := cmdRunner("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", save); err != nil {
		return nil, err
	}
	wslPath := llm.NormalizeImagePath(winTemp)
	defer os.Remove(wslPath)
	data, err := os.ReadFile(wslPath)
	if err != nil {
		return nil, err
	}
	if !isImageBytes(data) {
		return nil, fmt.Errorf("clipboard did not yield an image")
	}
	return data, nil
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
	_ = filepath.Clean(path)
	return path, nil
}

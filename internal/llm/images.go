package llm

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Defaults for image ingestion. A full-resolution screenshot costs thousands
// of tokens for no additional accuracy on code-adjacent images, so anything
// larger than the long edge is downscaled once, at ingest, and the normalized
// copy is what every later turn re-encodes.
const (
	DefaultImageMaxSizeMB = 15
	DefaultImageMaxDim    = 1568
	maxHydrateBytes       = 30 << 20
)

// ImagePart is one image attached to a message. The pixels live on disk — in
// the session's tmp/ once normalized — and Path points at that copy; Data
// carries the bytes in memory only (json:"-"), filled by HydrateImages before
// a request is built. History therefore stores references, never base64.
type ImagePart struct {
	Path      string `json:"path,omitempty"`
	MediaType string `json:"media_type,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Size      int64  `json:"size,omitempty"`
	Data      []byte `json:"-"`
}

// Name is the file name for display, never the full path.
func (p ImagePart) Name() string {
	if p.Path == "" {
		return "image"
	}
	return filepath.Base(p.Path)
}

// Summary is the one-line redaction used in logs, transcripts and echoes:
// dimensions and size, never pixels.
func (p ImagePart) Summary() string {
	dims := ""
	if p.Width > 0 && p.Height > 0 {
		dims = fmt.Sprintf(" %dx%d", p.Width, p.Height)
	}
	size := ""
	if p.Size > 0 {
		size = " " + formatImageBytes(p.Size)
	}
	return fmt.Sprintf("[image %s%s%s]", p.Name(), dims, size)
}

func formatImageBytes(n int64) string {
	switch {
	case n >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%dKB", n/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// EstimateImageTokens approximates the tokens a provider charges for an
// image: a base cost plus a per-tile cost over 512px tiles. It is the rule
// OpenAI publishes; other providers differ, but only in the constants, so it
// is good enough for a client-side context guard.
func EstimateImageTokens(width, height int) int {
	if width <= 0 || height <= 0 {
		return 1000
	}
	tiles := (width + 511) / 512 * ((height + 511) / 512)
	return 85 + 170*tiles
}

// ImageTokensForMessages sums the estimate over every attached image.
func ImageTokensForMessages(msgs []Message) int {
	total := 0
	for _, m := range msgs {
		for _, img := range m.Images {
			total += EstimateImageTokens(img.Width, img.Height)
		}
	}
	return total
}

// DataURL renders the in-memory bytes as a data: URL for the wire.
func (p ImagePart) DataURL() string {
	return "data:" + p.MediaType + ";base64," + base64.StdEncoding.EncodeToString(p.Data)
}

// withData reports whether the part is ready to send.
func (p ImagePart) withData() bool { return len(p.Data) > 0 }

// imagesWithData filters a message's attachments down to the sendable ones.
func imagesWithData(imgs []ImagePart) []ImagePart {
	out := imgs[:0:0]
	for _, img := range imgs {
		if img.withData() {
			out = append(out, img)
		}
	}
	return out
}

// HydrateImages loads every attachment's bytes from disk, returning a copy.
// A file that is gone (tmp/ cleaned out from under the session, a hand-edited
// history) degrades to a text notice on that message rather than failing the
// turn. Messages without attachments are returned untouched.
func HydrateImages(msgs []Message) []Message {
	out := make([]Message, len(msgs))
	for i, m := range msgs {
		out[i] = m
		if len(m.Images) == 0 {
			continue
		}
		kept := make([]ImagePart, 0, len(m.Images))
		for _, img := range m.Images {
			if img.withData() {
				kept = append(kept, img)
				continue
			}
			if img.Path == "" {
				continue
			}
			data, err := readImageFile(img.Path)
			if err != nil {
				out[i].Content += "\n" + (&ImagePart{Path: img.Path}).Summary() + " unavailable: " + err.Error()
				continue
			}
			img.Data = data
			kept = append(kept, img)
		}
		out[i].Images = kept
	}
	return out
}

func readImageFile(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxHydrateBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxHydrateBytes {
		return nil, fmt.Errorf("file too large")
	}
	return data, nil
}

// imageExts are the extensions accepted as image references in the composer
// and by the ReadImage tool. WebP is listed so it is recognized as an image
// and refused with a clear error, rather than misread as something else.
var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true,
}

// IsImagePath reports whether path names an image by extension. It says
// nothing about whether the file exists or what it contains; PrepareImage
// sniffs the content rather than trusting this.
func IsImagePath(path string) bool {
	return imageExts[strings.ToLower(filepath.Ext(strings.TrimSpace(path)))]
}

// NormalizeImagePath cleans a raw image reference from the composer: quoted
// drag-drop paths, file:// URLs, and — on WSL — Windows paths from Windows
// Terminal. It is pure string work plus wslpath when one is needed.
func NormalizeImagePath(path string) string {
	p := strings.TrimSpace(path)
	if len(p) >= 2 {
		if (p[0] == '"' && p[len(p)-1] == '"') || (p[0] == '\'' && p[len(p)-1] == '\'') {
			p = p[1 : len(p)-1]
		}
	}
	p = strings.TrimSpace(p)
	if strings.HasPrefix(strings.ToLower(p), "file://") {
		u := p[len("file://"):]
		u = strings.TrimPrefix(u, "localhost")
		if un, err := url.PathUnescape(u); err == nil {
			u = un
		}
		// file:///C:/... carries a leading slash before the drive.
		if len(u) > 3 && u[0] == '/' && u[2] == ':' && (u[1] >= 'A' && u[1] <= 'Z' || u[1] >= 'a' && u[1] <= 'z') {
			u = u[1:]
		}
		p = u
	}
	if windowsDrive.MatchString(p) {
		p = convertWindowsPath(p, defaultCmdRunner)
	}
	return p
}

var windowsDrive = regexp.MustCompile(`^[A-Za-z]:[\\/]`)

func defaultCmdRunner(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).Output()
}

// convertWindowsPath maps C:\... to its WSL path via wslpath, falling back to
// the /mnt/<drive>/ convention when wslpath is unavailable (not WSL, or a
// minimal PATH). run is injectable for tests.
func convertWindowsPath(win string, run func(string, ...string) ([]byte, error)) string {
	if out, err := run("wslpath", "-u", win); err == nil {
		if s := strings.TrimSpace(string(out)); s != "" {
			return s
		}
	}
	rest := win[2:]
	rest = strings.ReplaceAll(rest, "\\", "/")
	rest = strings.TrimPrefix(rest, "/")
	return "/mnt/" + strings.ToLower(string(win[0])) + "/" + rest
}

// IsWSL reports whether this process runs under Windows Subsystem for Linux,
// where the clipboard lives on the Windows side (see internal/tui).
func IsWSL() bool {
	if os.Getenv("WSL_DISTRO_NAME") != "" || os.Getenv("WSL_INTEROP") != "" {
		return true
	}
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// PrepareImage validates srcPath as an image, normalizes it into destDir, and
// returns the attachment pointing at the normalized copy with bytes loaded.
// Oversized images are downscaled (long edge to maxDim) and re-encoded as
// JPEG; anything else is stored byte-identical. Non-positive caps select the
// defaults.
func PrepareImage(srcPath, destDir string, maxSizeMB, maxDim int) (ImagePart, error) {
	if maxSizeMB <= 0 {
		maxSizeMB = DefaultImageMaxSizeMB
	}
	if maxDim <= 0 {
		maxDim = DefaultImageMaxDim
	}
	clean := NormalizeImagePath(srcPath)
	info, err := os.Stat(clean)
	if err != nil {
		return ImagePart{}, fmt.Errorf("read image %q: %w", srcPath, err)
	}
	if info.IsDir() {
		return ImagePart{}, fmt.Errorf("%q is a directory, not an image", srcPath)
	}
	if info.Size() > int64(maxSizeMB)<<20 {
		return ImagePart{}, fmt.Errorf("%q is %s, over the %dMB limit", srcPath, formatImageBytes(info.Size()), maxSizeMB)
	}
	data, err := os.ReadFile(clean)
	if err != nil {
		return ImagePart{}, fmt.Errorf("read image %q: %w", srcPath, err)
	}
	media := http.DetectContentType(head(data, 512))
	switch media {
	case "image/png", "image/jpeg", "image/gif":
	default:
		if strings.EqualFold(filepath.Ext(clean), ".webp") || media == "image/webp" {
			return ImagePart{}, fmt.Errorf("%q is WebP, which is not supported yet: convert it to PNG or JPEG first", srcPath)
		}
		return ImagePart{}, fmt.Errorf("%q is not an image (%s)", srcPath, media)
	}
	img, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return ImagePart{}, fmt.Errorf("decode image %q: %w", srcPath, err)
	}
	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	stored := data
	storedMedia := media
	ext := strings.ToLower(filepath.Ext(clean))
	if ext == ".jpeg" {
		ext = ".jpg"
	}
	if max(w, h) > maxDim {
		stored, w, h = downscaleJPEG(img, w, h, maxDim)
		storedMedia = "image/jpeg"
		ext = ".jpg"
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return ImagePart{}, fmt.Errorf("create image store: %w", err)
	}
	name := fmt.Sprintf("img-%s-%s%s", time.Now().UTC().Format("20060102-150405"), randHex(4), ext)
	dst := filepath.Join(destDir, name)
	if err := os.WriteFile(dst, stored, 0644); err != nil {
		return ImagePart{}, fmt.Errorf("store image: %w", err)
	}
	return ImagePart{
		Path:      dst,
		MediaType: storedMedia,
		Width:     w,
		Height:    h,
		Size:      int64(len(stored)),
		Data:      stored,
	}, nil
}

func head(data []byte, n int) []byte {
	if len(data) < n {
		return data
	}
	return data[:n]
}

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// downscaleJPEG scales img to fit the long edge in maxDim with nearest
// neighbor and re-encodes at quality 85. It is stdlib-only on purpose: no
// cgo, no new dependency for scripts/build.sh to carry.
func downscaleJPEG(img image.Image, w, h, maxDim int) ([]byte, int, int) {
	longest := max(w, h)
	nw := max(1, w*maxDim/longest)
	nh := max(1, h*maxDim/longest)
	dst := image.NewRGBA(image.Rect(0, 0, nw, nh))
	bounds := img.Bounds()
	for y := 0; y < nh; y++ {
		sy := bounds.Min.Y + y*h/nh
		for x := 0; x < nw; x++ {
			sx := bounds.Min.X + x*w/nw
			dst.Set(x, y, img.At(sx, sy))
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, dst, &jpeg.Options{Quality: 85})
	return buf.Bytes(), nw, nh
}

// UnmarshalJSON accepts content as the string every writer stores, or as the
// array of typed parts some endpoints return, so a multimodal answer decodes
// without error. Image parts on the wire carry no bytes back, so only their
// text survives; attachments kvit-coder sent are already in Images.
func (m *Message) UnmarshalJSON(data []byte) error {
	type plain Message
	var p plain
	if err := json.Unmarshal(data, &p); err != nil {
		// The only field that legitimately varies in shape is content, so
		// retry with it decoded leniently.
		var raw struct {
			Role             MessageRole     `json:"role"`
			Content          json.RawMessage `json:"content"`
			ReasoningContent string          `json:"reasoning_content"`
			Name             string          `json:"name"`
			ToolCalls        []ToolCall      `json:"tool_calls"`
			ToolCallID       string          `json:"tool_call_id"`
			Images           []ImagePart     `json:"images"`
		}
		if err2 := json.Unmarshal(data, &raw); err2 != nil {
			return err
		}
		*m = Message{
			Role:             raw.Role,
			Content:          messageTextFromContent(raw.Content),
			ReasoningContent: raw.ReasoningContent,
			Name:             raw.Name,
			ToolCalls:        raw.ToolCalls,
			ToolCallID:       raw.ToolCallID,
			Images:           raw.Images,
		}
		return nil
	}
	*m = Message(p)
	return nil
}

// messageTextFromContent pulls the readable text out of a wire content value
// in any of its shapes: a bare string, a list of typed parts, or null.
func messageTextFromContent(raw json.RawMessage) string {
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	var parts []struct {
		Type     string `json:"type"`
		Text     string `json:"text"`
		ImageURL any    `json:"image_url"`
	}
	if err := json.Unmarshal(raw, &parts); err != nil {
		return ""
	}
	var b strings.Builder
	for _, part := range parts {
		switch part.Type {
		case "text", "input_text", "output_text":
			b.WriteString(part.Text)
		}
	}
	return b.String()
}

// chatContentPart is one element of a chat-completions content array.
type chatContentPart struct {
	Type     string        `json:"type"`
	Text     string        `json:"text,omitempty"`
	ImageURL *chatImageURL `json:"image_url,omitempty"`
}

type chatImageURL struct {
	URL string `json:"url"`
}

// chatWireMessage is Message as the /chat/completions endpoint sees it:
// identical fields in identical order, except content may be a part array and
// the images reference list is omitted (the pixels ride in the parts).
type chatWireMessage struct {
	Role             MessageRole `json:"role"`
	Content          any         `json:"content,omitempty"`
	ReasoningContent string      `json:"reasoning_content,omitempty"`
	Name             string      `json:"name,omitempty"`
	ToolCalls        []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID       string      `json:"tool_call_id,omitempty"`
}

func toChatWireMessage(m Message) chatWireMessage {
	out := chatWireMessage{
		Role:             m.Role,
		ReasoningContent: m.ReasoningContent,
		Name:             m.Name,
		ToolCalls:        m.ToolCalls,
		ToolCallID:       m.ToolCallID,
	}
	imgs := imagesWithData(m.Images)
	if len(imgs) == 0 {
		// Bare strings, exactly as before: the prompt prefix must stay
		// byte-identical between requests so the server-side cache hits.
		if m.Content != "" {
			out.Content = m.Content
		}
		return out
	}
	parts := make([]chatContentPart, 0, len(imgs)+1)
	if m.Content != "" {
		parts = append(parts, chatContentPart{Type: "text", Text: m.Content})
	}
	for _, img := range imgs {
		parts = append(parts, chatContentPart{
			Type:     "image_url",
			ImageURL: &chatImageURL{URL: img.DataURL()},
		})
	}
	out.Content = parts
	return out
}

// toChatWireMessages converts a history for the chat-completions body.
func toChatWireMessages(msgs []Message) []chatWireMessage {
	out := make([]chatWireMessage, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, toChatWireMessage(m))
	}
	return out
}

// chatWireRequest mirrors ChatRequest with wire messages.
type chatWireRequest struct {
	Model              string            `json:"model"`
	Messages           []chatWireMessage `json:"messages"`
	Temperature        float32           `json:"temperature,omitempty"`
	MaxTokens          int               `json:"max_tokens,omitempty"`
	Tools              []ToolSpec        `json:"tools,omitempty"`
	ToolChoice         string            `json:"tool_choice,omitempty"`
	Stream             bool              `json:"stream,omitempty"`
	ChatTemplateKwargs map[string]any    `json:"chat_template_kwargs,omitempty"`
}

func toChatWireRequest(req ChatRequest) chatWireRequest {
	return chatWireRequest{
		Model:              req.Model,
		Messages:           toChatWireMessages(req.Messages),
		Temperature:        req.Temperature,
		MaxTokens:          req.MaxTokens,
		Tools:              req.Tools,
		ToolChoice:         req.ToolChoice,
		Stream:             req.Stream,
		ChatTemplateKwargs: req.ChatTemplateKwargs,
	}
}

// responsesInputImagePart is one element of a Responses message content list.
type responsesInputImagePart struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

// toResponsesContent renders a user/system message body: a bare string when
// text-only (byte-identical to before), or input_text plus input_image parts
// once images are attached.
func toResponsesContent(m Message) any {
	imgs := imagesWithData(m.Images)
	if len(imgs) == 0 {
		return m.Content
	}
	parts := make([]responsesInputImagePart, 0, len(imgs)+1)
	if m.Content != "" {
		parts = append(parts, responsesInputImagePart{Type: "input_text", Text: m.Content})
	}
	for _, img := range imgs {
		parts = append(parts, responsesInputImagePart{Type: "input_image", ImageURL: img.DataURL()})
	}
	return parts
}

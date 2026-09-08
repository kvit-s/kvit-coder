package llm

import (
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestPNG(t *testing.T, dir string, w, h int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x % 256), uint8(y % 256), 128, 255})
		}
	}
	p := filepath.Join(dir, "shot.png")
	f, err := os.Create(p)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPrepareImageRoundTrip(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPNG(t, dir, 64, 48)
	part, err := PrepareImage(src, filepath.Join(dir, "store"), 0, 0)
	if err != nil {
		t.Fatalf("PrepareImage: %v", err)
	}
	if part.MediaType != "image/png" {
		t.Errorf("media = %q, want image/png", part.MediaType)
	}
	if part.Width != 64 || part.Height != 48 {
		t.Errorf("dims = %dx%d, want 64x48", part.Width, part.Height)
	}
	if len(part.Data) == 0 {
		t.Error("Data not loaded")
	}
	if _, err := os.Stat(part.Path); err != nil {
		t.Errorf("stored file missing: %v", err)
	}
	if part.Path == src {
		t.Error("normalized copy should not be the source file")
	}
	// History form carries references, never pixels.
	body, _ := json.Marshal(part)
	if strings.Contains(string(body), "iVBOR") {
		t.Error("ImagePart JSON contains base64 pixels")
	}
}

func TestPrepareImageDownscales(t *testing.T) {
	dir := t.TempDir()
	src := writeTestPNG(t, dir, 2000, 1000)
	part, err := PrepareImage(src, filepath.Join(dir, "store"), 0, 500)
	if err != nil {
		t.Fatalf("PrepareImage: %v", err)
	}
	if part.Width != 500 || part.Height != 250 {
		t.Errorf("dims = %dx%d, want 500x250", part.Width, part.Height)
	}
	if part.MediaType != "image/jpeg" {
		t.Errorf("media = %q, want image/jpeg after downscale", part.MediaType)
	}
}

func TestPrepareImageRejectsText(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "note.txt")
	if err := os.WriteFile(p, []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareImage(p, filepath.Join(dir, "store"), 0, 0); err == nil {
		t.Error("expected error for a text file")
	} else if !strings.Contains(err.Error(), "not an image") {
		t.Errorf("error = %q, want a not-an-image message", err)
	}
}

func TestNormalizeImagePath(t *testing.T) {
	for in, want := range map[string]string{
		`"/tmp/shot.png"`:   "/tmp/shot.png",
		`'/tmp/shot.png'`:   "/tmp/shot.png",
		"file:///tmp/a.png": "/tmp/a.png",
	} {
		if got := NormalizeImagePath(in); got != want {
			t.Errorf("NormalizeImagePath(%q) = %q, want %q", in, got, want)
		}
	}
	// Windows path without wslpath on PATH falls back to /mnt/<drive>/.
	got := convertWindowsPath(`C:\Users\me\shot.png`, func(string, ...string) ([]byte, error) {
		return nil, os.ErrNotExist
	})
	if got != "/mnt/c/Users/me/shot.png" {
		t.Errorf("windows fallback = %q", got)
	}
	if !IsImagePath("/tmp/a.PNG") || IsImagePath("/tmp/a.txt") {
		t.Error("IsImagePath extension check wrong")
	}
}

func TestHydrateImagesMissingFile(t *testing.T) {
	msgs := []Message{{
		Role:    RoleUser,
		Content: "look",
		Images:  []ImagePart{{Path: filepath.Join(t.TempDir(), "gone.png"), MediaType: "image/png", Width: 10, Height: 10}},
	}}
	got := HydrateImages(msgs)
	if len(got[0].Images) != 0 {
		t.Errorf("missing file should drop the image, got %+v", got[0].Images)
	}
	if !strings.Contains(got[0].Content, "unavailable") {
		t.Errorf("missing file should leave a notice, got %q", got[0].Content)
	}
	// The caller's slice is untouched.
	if len(msgs[0].Images) != 1 {
		t.Error("HydrateImages mutated its input")
	}
}

func TestChatWireTextOnlyDecodesAsStrings(t *testing.T) {
	req := ChatRequest{
		Model: "m",
		Messages: []Message{
			{Role: RoleSystem, Content: "sys"},
			{Role: RoleUser, Content: "hi"},
		},
		Temperature: 0.5,
	}
	newBody, err := json.Marshal(toChatWireRequest(req))
	if err != nil {
		t.Fatal(err)
	}
	var decoded struct {
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(newBody, &decoded); err != nil {
		t.Fatalf("wire body should decode with string content: %v\n%s", err, newBody)
	}
	if len(decoded.Messages) != 2 || decoded.Messages[1].Content != "hi" {
		t.Errorf("wire messages changed: %s", newBody)
	}
}

func TestChatWireImageParts(t *testing.T) {
	dir := t.TempDir()
	part, err := PrepareImage(writeTestPNG(t, dir, 8, 8), filepath.Join(dir, "s"), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	m := toChatWireMessage(Message{Role: RoleUser, Content: "see", Images: []ImagePart{part}})
	parts, ok := m.Content.([]chatContentPart)
	if !ok {
		t.Fatalf("content type = %T, want parts", m.Content)
	}
	if len(parts) != 2 || parts[0].Type != "text" || parts[1].Type != "image_url" {
		t.Fatalf("parts = %+v", parts)
	}
	if !strings.HasPrefix(parts[1].ImageURL.URL, "data:image/png;base64,") {
		t.Errorf("data URL = %.40q", parts[1].ImageURL.URL)
	}
	// No images field leaks to the server.
	body, _ := json.Marshal(m)
	if strings.Contains(string(body), `"images"`) {
		t.Errorf("wire message leaks images field: %s", body)
	}
}

func TestResponsesInputImageParts(t *testing.T) {
	dir := t.TempDir()
	part, err := PrepareImage(writeTestPNG(t, dir, 8, 8), filepath.Join(dir, "s"), 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	items := messagesToResponsesInput([]Message{{Role: RoleUser, Content: "see", Images: []ImagePart{part}}})
	if len(items) != 1 {
		t.Fatalf("got %d items", len(items))
	}
	parts, ok := items[0].Content.([]responsesInputImagePart)
	if !ok {
		t.Fatalf("content type = %T", items[0].Content)
	}
	if len(parts) != 2 || parts[0].Type != "input_text" || parts[1].Type != "input_image" {
		t.Fatalf("parts = %+v", parts)
	}
	// Text-only still a bare string.
	textOnly := messagesToResponsesInput([]Message{{Role: RoleUser, Content: "hi"}})
	if s, ok := textOnly[0].Content.(string); !ok || s != "hi" {
		t.Errorf("text-only content = %#v", textOnly[0].Content)
	}
}

func TestMessageUnmarshalArrayContent(t *testing.T) {
	raw := `{"role":"assistant","content":[{"type":"output_text","text":"hello"},{"type":"image_url","image_url":{"url":"data:..."}}]}`
	var m Message
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if m.Content != "hello" {
		t.Errorf("content = %q, want hello", m.Content)
	}
}

func TestEstimateImageTokens(t *testing.T) {
	if got := EstimateImageTokens(512, 512); got != 85+170 {
		t.Errorf("512x512 = %d", got)
	}
	if EstimateImageTokens(0, 0) <= 0 {
		t.Error("unknown dims should still estimate something")
	}
}

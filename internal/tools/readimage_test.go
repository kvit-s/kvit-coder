package tools

import (
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

func testConfig(root string) *config.Config {
	cfg := &config.Config{}
	cfg.Workspace.Root = root
	cfg.Tools.Read.Enabled = true
	cfg.Tools.Images.Enabled = true
	cfg.Tools.Images.MaxSizeMB = 15
	cfg.Tools.Images.MaxDimension = 1568
	return cfg
}

func writePNG(t *testing.T, path string, w, h int) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{200, 100, 50, 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func callReadImage(t *testing.T, tool *ReadImageTool, path string) (any, error) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{"path": path})
	return tool.Call(context.Background(), raw)
}

func TestReadImageSuccess(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "shot.png")
	writePNG(t, src, 40, 30)
	toolCtx := NewToolContext()
	toolCtx.SetSessionTmp(filepath.Join(root, "session-tmp"))
	tool := NewReadImageTool(testConfig(root), toolCtx)

	result, err := callReadImage(t, tool, "shot.png")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	carrier, ok := result.(ImageCarrier)
	if !ok {
		t.Fatalf("result %T does not carry images", result)
	}
	if len(carrier.ToolImages()) != 1 {
		t.Fatalf("got %d images", len(carrier.ToolImages()))
	}
	part := carrier.ToolImages()[0]
	if part.Width != 40 || part.Height != 30 || part.MediaType != "image/png" {
		t.Errorf("part = %+v", part)
	}
	// The normalized copy lives in the session tmp, not the workspace.
	if !strings.HasPrefix(part.Path, filepath.Join(root, "session-tmp")) {
		t.Errorf("stored at %q, want session tmp", part.Path)
	}
	// The summary marshals without pixels.
	body, _ := json.Marshal(result)
	if strings.Contains(string(body), "iVBOR") {
		t.Error("result JSON contains pixels")
	}
}

func TestReadImageRejectsText(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	tool := NewReadImageTool(testConfig(root), NewToolContext())
	if _, err := callReadImage(t, tool, "a.txt"); err == nil {
		t.Error("expected an error for a text file")
	}
}

func TestReadImageMissing(t *testing.T) {
	root := t.TempDir()
	tool := NewReadImageTool(testConfig(root), NewToolContext())
	if _, err := callReadImage(t, tool, "gone.png"); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestReadImageDownscales(t *testing.T) {
	root := t.TempDir()
	src := filepath.Join(root, "big.png")
	writePNG(t, src, 1200, 600)
	cfg := testConfig(root)
	cfg.Tools.Images.MaxDimension = 600
	tool := NewReadImageTool(cfg, NewToolContext())
	result, err := callReadImage(t, tool, "big.png")
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	part := result.(ImageCarrier).ToolImages()[0]
	if part.Width != 600 || part.Height != 300 {
		t.Errorf("dims = %dx%d", part.Width, part.Height)
	}
}

func TestBatchPropagatesReadImage(t *testing.T) {
	root := t.TempDir()
	writePNG(t, filepath.Join(root, "a.png"), 10, 10)
	cfg := testConfig(root)
	registry := NewRegistry()
	registry.Enable(NewReadImageTool(cfg, NewToolContext()))
	batch := NewBatchTool(cfg)
	registry.Enable(batch)
	batch.SetRegistry(registry)

	raw, _ := json.Marshal(map[string]any{"calls": []map[string]any{
		{"tool": "ReadImage", "args": map[string]any{"path": "a.png"}},
	}})
	out, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Batch: %v", err)
	}
	m := out.(map[string]any)
	results := m["results"].([]BatchResult)
	if !results[0].OK {
		t.Fatalf("call failed: %q", results[0].Error)
	}
	if len(results[0].Images) != 1 {
		t.Errorf("batch dropped the image: %+v", results[0])
	}
}

func TestReadImagePromptDocs(t *testing.T) {
	tool := NewReadImageTool(testConfig(t.TempDir()), NewToolContext())
	if tool.Name() != "ReadImage" || tool.PromptCategory() != "filesystem" {
		t.Errorf("registration fields wrong: %q %q", tool.Name(), tool.PromptCategory())
	}
	if !tool.ParallelSafe() {
		t.Error("ReadImage should be batch-parallel-safe")
	}
	if !strings.Contains(tool.PromptSection(), "ReadImage") {
		t.Error("PromptSection should document the tool")
	}
}

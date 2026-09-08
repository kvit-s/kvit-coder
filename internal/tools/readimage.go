package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// ImageReadResult is what ReadImage returns: a small JSON summary for the
// conversation plus the attachments, which the runner places on a follower
// user message. Only the summary is serialized; pixels travel in memory and
// on disk, never in JSON.
type ImageReadResult struct {
	Success   bool   `json:"success"`
	Path      string `json:"path"`
	MediaType string `json:"media_type,omitempty"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	Size      int64  `json:"size,omitempty"`

	images []llm.ImagePart
}

// ToolImages hands the attachments to the runner.
func (r *ImageReadResult) ToolImages() []llm.ImagePart {
	if r == nil {
		return nil
	}
	return r.images
}

// ReadImageTool reads an image file so the model can see it. Text stays with
// Read; this tool sniffs the content, normalizes a copy into the session's
// tmp/, and returns the reference. An image read never satisfies the
// read-before-edit gate, which is about text the model will quote back.
type ReadImageTool struct {
	config        *config.Config
	workspaceRoot string
	maxSizeMB     int
	maxDim        int
	toolCtx       *ToolContext
}

func NewReadImageTool(cfg *config.Config, toolCtx *ToolContext) *ReadImageTool {
	maxSizeMB := cfg.Tools.Images.MaxSizeMB
	if maxSizeMB <= 0 {
		maxSizeMB = llm.DefaultImageMaxSizeMB
	}
	maxDim := cfg.Tools.Images.MaxDimension
	if maxDim <= 0 {
		maxDim = llm.DefaultImageMaxDim
	}
	return &ReadImageTool{
		config:        cfg,
		workspaceRoot: cfg.Workspace.Root,
		maxSizeMB:     maxSizeMB,
		maxDim:        maxDim,
		toolCtx:       toolCtx,
	}
}

// ParallelSafe says ReadImage can run alongside other reads in a Batch: it
// writes only its own normalized copy under a unique name.
func (t *ReadImageTool) ParallelSafe() bool { return true }

func (t *ReadImageTool) Name() string {
	return "ReadImage"
}

func (t *ReadImageTool) Description() string {
	return "Read an image file (PNG, JPEG, GIF) so the model can see it. Returns a summary; the image itself is attached to the result. Large images are downscaled automatically."
}

func (t *ReadImageTool) Check(ctx context.Context, args json.RawMessage) error {
	return nil
}

func (t *ReadImageTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the image file (relative to workspace or absolute)",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadImageTool) PromptCategory() string     { return "filesystem" }
func (t *ReadImageTool) PromptOrder() int           { return 11 }
func (t *ReadImageTool) PromptTemplateName() string { return "read-image" }
func (t *ReadImageTool) PromptSection() string {
	return `### ReadImage - Read an Image File

**Usage:** ` + "`" + `ReadImage {"path": "<image file>"}` + "`" + `

Reads a PNG, JPEG or GIF so you can see it: a screenshot, a photo of a
whiteboard, a diagram. The image arrives with the result; the JSON summary
says which file it was. Large images are downscaled automatically. WebP is
not supported — convert it to PNG or JPEG first.

An image read does not count as reading a file for editing: quote paths, not
pixels, and Read the file before editing it.`
}

func (t *ReadImageTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}
	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	clean := llm.NormalizeImagePath(params.Path)

	// Same 3-tier permission path as Read.
	permResult, err := t.config.CheckPathPermission(clean, config.AccessRead)
	if err != nil && permResult == config.PermissionDenied {
		return nil, fmt.Errorf("access denied: %w", err)
	}
	fullPath, outside, err := NormalizeAndValidatePath(t.workspaceRoot, clean)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}
	if outside {
		if err := t.config.CheckPathSafety("ReadImage", clean); err != nil {
			return nil, err
		}
	}
	if info, err := os.Stat(fullPath); err == nil && info.IsDir() {
		return nil, fmt.Errorf("%q is a directory: ReadImage reads image files, use Read to list it", params.Path)
	}

	destDir := ""
	if t.toolCtx != nil {
		destDir = t.toolCtx.SessionTmp()
	}
	if destDir == "" {
		destDir = filepath.Join(os.TempDir(), "kvit-coder-images")
	}

	part, err := llm.PrepareImage(fullPath, destDir, t.maxSizeMB, t.maxDim)
	if err != nil {
		return nil, err
	}
	return &ImageReadResult{
		Success:   true,
		Path:      params.Path,
		MediaType: part.MediaType,
		Width:     part.Width,
		Height:    part.Height,
		Size:      part.Size,
		images:    []llm.ImagePart{part},
	}, nil
}

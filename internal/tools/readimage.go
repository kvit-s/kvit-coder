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

	// Page and TotalPages are filled when the source was a PDF, so the model
	// knows which page it is looking at and what else is there.
	Page       int    `json:"page,omitempty"`
	TotalPages int    `json:"total_pages,omitempty"`
	Hint       string `json:"hint,omitempty"`

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
	return "Read an image file (PNG, JPEG, GIF, WebP), or one page of a PDF drawn as a picture, so the model can see it. Returns a summary; the image itself is attached to the result. Large images are downscaled automatically."
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
				"description": "Path to the image or PDF file (relative to workspace or absolute)",
			},
			"page": map[string]any{
				"type":        "integer",
				"description": "PDF files only: which page to draw, counting from 1. Default: 1. One page per call.",
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

Reads a PNG, JPEG, GIF or WebP so you can see it: a screenshot, a photo of a
whiteboard, a diagram. The image arrives with the result; the JSON summary
says which file it was. Large images are downscaled automatically.

**PDF files** are drawn as a picture, one page per call:
` + "`" + `ReadImage {"path": "scan.pdf", "page": 3}` + "`" + `, page 1 if you do not say.
Use this when Read found no text in the PDF, and when the layout itself is
the point: a diagram, a chart, a form. Read gets you the text far more
cheaply, so reach for this second.

An image read does not count as reading a file for editing: quote paths, not
pixels, and Read the file before editing it.`
}

func (t *ReadImageTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Path string `json:"path"`
		Page int    `json:"page"`
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

	// A PDF holds no pixels to read, so a page is drawn into one first and the
	// picture is what goes through the usual image path. This is the answer for
	// a page whose text will not extract, which happens to a scan and equally to
	// a page whose lettering is drawn as shapes rather than characters.
	if head, err := readFileHead(fullPath, guardHeadBytes); err == nil && looksLikePDF(head) {
		return t.readPDFPage(ctx, fullPath, params.Path, params.Page, destDir)
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

// readPDFPage draws one page of a PDF and returns it as the attached image.
func (t *ReadImageTool) readPDFPage(ctx context.Context, fullPath, path string, page int, destDir string) (any, error) {
	if page == 0 {
		page = 1
	}
	if page < 0 {
		return nil, fmt.Errorf("page %d is not a page number: pages count from 1", page)
	}

	cacheDir := ""
	if t.config != nil {
		cacheDir = t.config.Tools.Read.PDF.CacheDir
	}
	pool, err := pdfiumPool(cacheDir)
	if err != nil {
		return nil, fmt.Errorf("start PDF reader: %w", err)
	}

	waitCtx := ctx
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		waitCtx, cancel = context.WithTimeout(ctx, pdfPageWait)
		defer cancel()
	}

	imagePath, totalPages, err := renderPDFPage(waitCtx, pool, fullPath, destDir, page, t.maxDim)
	if err != nil {
		return nil, err
	}
	// The rendered page goes through the same preparation as any other image,
	// so the size caps and the normalized copy apply to it unchanged.
	part, err := llm.PrepareImage(imagePath, destDir, t.maxSizeMB, t.maxDim)
	if err != nil {
		return nil, err
	}

	result := &ImageReadResult{
		Success:    true,
		Path:       path,
		MediaType:  part.MediaType,
		Width:      part.Width,
		Height:     part.Height,
		Size:       part.Size,
		Page:       page,
		TotalPages: totalPages,
		images:     []llm.ImagePart{part},
	}
	if totalPages > 1 {
		result.Hint = fmt.Sprintf("Page %d of %d. Ask for another with ReadImage {\"path\": %q, \"page\": N}, one page per call.",
			page, totalPages, path)
	}
	return result, nil
}

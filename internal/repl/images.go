package repl

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// preparePromptImages normalizes each -image path into the session's tmp/
// and returns the attachments for the prompt's user message. A file that
// cannot be read is skipped with a warning rather than failing the turn:
// the text prompt is still worth running. Nothing is recorded in the
// read tracker — naming a file is the authorization, and an image read must
// never satisfy the read-before-edit gate.
func preparePromptImages(paths []string, cfg *config.Config, sess *session.Session, writer *ui.Writer, quietMode bool) []llm.ImagePart {
	if len(paths) == 0 {
		return nil
	}
	destDir := filepath.Join(os.TempDir(), "kvit-coder-images")
	if sess != nil {
		destDir = sess.TmpDir()
	}
	maxSizeMB, maxDim := 0, 0
	if cfg != nil {
		maxSizeMB = cfg.Tools.Images.MaxSizeMB
		maxDim = cfg.Tools.Images.MaxDimension
	}
	var out []llm.ImagePart
	for _, p := range paths {
		part, err := llm.PrepareImage(p, destDir, maxSizeMB, maxDim)
		if err != nil {
			writer.Warn(fmt.Sprintf("skip image %q: %v", p, err))
			continue
		}
		out = append(out, part)
		if !quietMode && !session.FromUI() {
			fmt.Fprintf(os.Stderr, "[image %d: %s]\n", len(out), part.Summary())
		}
	}
	return out
}

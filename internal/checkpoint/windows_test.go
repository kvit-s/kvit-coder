package checkpoint

import (
	"strings"
	"testing"
)

// Every shadow-repo git invocation must carry the autocrlf/eol overrides so
// a user's global git config (notably core.autocrlf=true on Git for Windows)
// cannot change checkpoint bytes.
func TestGitCmdCarriesLineEndingConfig(t *testing.T) {
	cmd := gitCmd("--git-dir=/tmp/x", "tag", "--list", "turn-*")
	joined := strings.Join(cmd.Args, " ")
	if !strings.Contains(joined, "core.autocrlf=false") {
		t.Errorf("gitCmd missing core.autocrlf=false: %q", joined)
	}
	if !strings.Contains(joined, "core.eol=lf") {
		t.Errorf("gitCmd missing core.eol=lf: %q", joined)
	}
	// Overrides sort before the subcommand's global flags.
	autoIdx := strings.Index(joined, "core.autocrlf")
	gitDirIdx := strings.Index(joined, "--git-dir")
	if autoIdx < 0 || gitDirIdx < 0 || autoIdx > gitDirIdx {
		t.Errorf("overrides must precede --git-dir: %q", joined)
	}
}

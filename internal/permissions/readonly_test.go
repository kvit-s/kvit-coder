package permissions

import "testing"

func TestReadOnlyCommands(t *testing.T) {
	for _, cmd := range []string{
		"ls -la",
		"cat go.mod | head -n 20",
		`grep -rn "Report" internal/ | wc -l`,
		"cd internal/report && ls",
		"git status --short",
		"git -C /home/x/repo log --oneline -5",
		"git --no-pager diff HEAD~1 -- internal/",
		"git branch -a",
		"git stash list",
		"git config --get user.name",
		"go vet ./...",
		"go env GOMODCACHE",
		"gofmt -l internal/",
		"sed -n '1,40p' main.go",
		"sort -u names.txt",
		`find . -name "*.go" -type f`,
		"ls missing 2>/dev/null || echo none",
		"go vet ./... 2>&1 | tail -5",
		"FOO=1 printenv FOO",
		"for f in *.go; do wc -l $f; done",
		"echo $(git rev-parse HEAD)",
		"cat <<EOF\nhello\nEOF",
	} {
		if !ReadOnly(cmd) {
			t.Errorf("%q counted as a change", cmd)
		}
	}
}

// Everything that writes, and everything that cannot be judged from the text,
// counts as a change.
func TestCommandsThatChangeSomething(t *testing.T) {
	for _, cmd := range []string{
		"",
		"rm -rf build",
		"echo hi > notes.txt",
		"echo hi >> notes.txt",
		"go vet ./... &> log.txt",
		"ls >&listing.txt",
		"cat a | tee b",
		"sed -i 's/a/b/' main.go",
		"sed -ni 's/a/b/p' main.go",
		"sed --in-place=.bak 's/a/b/' main.go",
		"sort -o out.txt names.txt",
		"gofmt -w internal/",
		"find . -name '*.tmp' -delete",
		"find . -name '*.go' -exec rm {} +",
		"git commit -m wip",
		"git branch feature",
		"git stash",
		"git config user.name x",
		"git diff --output=patch.diff",
		"go build ./cmd/kvit-coder",
		"go test ./...",
		"go env -w GOFLAGS=-mod=mod",
		"python3 -c 'print(1)'",
		"$CMD --version",
		"sed $FLAGS main.go",
		"ls && touch marker",
		"cat $(rm -f x)",
		"xargs rm < files.txt",
		"echo 'unterminated",
	} {
		if ReadOnly(cmd) {
			t.Errorf("%q counted as read-only", cmd)
		}
	}
}

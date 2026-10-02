package gfm

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestDependsOnStandardLibraryOnly(t *testing.T) {
	const modelPath = "github.com/kvit-s/kvit-coder/internal/office/model"
	for dir, allowed := range map[string]string{".": modelPath, "../../model": ""} {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), filepath.Join(dir, name), nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				if strings.Contains(strings.Split(path, "/")[0], ".") && path != allowed {
					t.Errorf("%s imports non-standard package %s", filepath.Join(dir, name), path)
				}
			}
		}
	}
}

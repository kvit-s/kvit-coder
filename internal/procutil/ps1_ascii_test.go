package procutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Windows PowerShell 5.1 reads a .ps1 file without a BOM using the system
// ANSI codepage, so a UTF-8 em dash (E2 80 94) decodes as â€ť — whose last
// byte is a closing curly quote that terminates the string literal early
// and breaks parsing. Keep every shipped script pure ASCII so the encoding
// never matters.
func TestPowerShellScriptsAreASCII(t *testing.T) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Dir(filepath.Dir(filepath.Dir(thisFile)))
	files := []string{
		filepath.Join(root, "scripts", "portable-install.ps1"),
		filepath.Join(root, "install.ps1"),
	}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Errorf("read %s: %v", f, err)
			continue
		}
		for i, b := range data {
			if b > 127 {
				t.Errorf("%s: non-ASCII byte 0x%02X at offset %d (use ASCII -- instead of em dashes)", f, b, i)
				break
			}
		}
	}
}

package config

import (
	"fmt"
	"os"
	"testing"
)

// TestMain gives the package's tests an empty home directory. Load reads
// ~/.kvit-coder/models.yaml and credentials.json, and a test must not see the
// models and keys saved on the machine that happens to run it. Tests that need
// those files point HOME at a directory of their own with useHome.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "kvit-config-home-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	os.Setenv("HOME", home)
	os.Setenv("USERPROFILE", home)
	code := m.Run()
	os.RemoveAll(home)
	os.Exit(code)
}

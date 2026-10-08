package tui

import (
	"fmt"
	"os"
	"testing"
)

// TestMain gives the package's tests an empty home directory. Looking up a
// row's key reads ~/.kvit-coder/credentials.json, and the setup screens write
// models.yaml beside it; a test must never see or change the files of the
// machine running it. Tests that need them point HOME at a directory of their
// own.
func TestMain(m *testing.M) {
	home, err := os.MkdirTemp("", "kvit-tui-home-")
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

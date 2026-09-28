package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHistoryPathForWorkspaceIsPerDirectory(t *testing.T) {
	home := t.TempDir()
	a := filepath.Join(t.TempDir(), "proj-a")
	b := filepath.Join(t.TempDir(), "proj-b")
	if err := os.MkdirAll(a, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(b, 0755); err != nil {
		t.Fatal(err)
	}

	pa := HistoryPathForWorkspace(home, a)
	pb := HistoryPathForWorkspace(home, b)
	if pa == pb {
		t.Fatalf("different workspaces share history path %q", pa)
	}
	if !strings.HasPrefix(pa, filepath.Join(home, ".kvit-coder", "history")) {
		t.Fatalf("history path %q is not under ~/.kvit-coder/history", pa)
	}
	// Same directory resolves to the same file, relative or absolute.
	if again := HistoryPathForWorkspace(home, a); again != pa {
		t.Fatalf("same workspace gave %q and %q", pa, again)
	}
}

func TestSaveHistoryCreatesParentDirs(t *testing.T) {
	home := t.TempDir()
	ws := filepath.Join(t.TempDir(), "proj")
	path := HistoryPathForWorkspace(home, ws)
	if err := SaveHistory(path, []string{"hello"}); err != nil {
		t.Fatalf("SaveHistory: %v", err)
	}
	loaded, err := LoadHistory(path)
	if err != nil {
		t.Fatalf("LoadHistory: %v", err)
	}
	if len(loaded) != 1 || loaded[0] != "hello" {
		t.Fatalf("loaded = %q", loaded)
	}
}

func TestLoadWorkspaceHistorySeedsFromLegacy(t *testing.T) {
	home := t.TempDir()
	ws := filepath.Join(t.TempDir(), "proj")
	if err := SaveHistory(LegacyHistoryPath(home), []string{"old one", "old two"}); err != nil {
		t.Fatal(err)
	}

	history, path := LoadWorkspaceHistory(home, ws)
	if len(history) != 2 || history[0] != "old one" {
		t.Fatalf("seeded history = %q", history)
	}
	if want := HistoryPathForWorkspace(home, ws); path != want {
		t.Fatalf("path = %q, want %q", path, want)
	}
	// Seeding is in-memory only: the per-directory file is written on the
	// next save, so the legacy file stays the only one on disk.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("per-directory file should not exist yet: err=%v", err)
	}
}

func TestLoadWorkspaceHistoryPrefersPerDirectory(t *testing.T) {
	home := t.TempDir()
	ws := filepath.Join(t.TempDir(), "proj")
	path := HistoryPathForWorkspace(home, ws)
	if err := SaveHistory(path, []string{"per-dir"}); err != nil {
		t.Fatal(err)
	}
	if err := SaveHistory(LegacyHistoryPath(home), []string{"global"}); err != nil {
		t.Fatal(err)
	}

	history, _ := LoadWorkspaceHistory(home, ws)
	if len(history) != 1 || history[0] != "per-dir" {
		t.Fatalf("history = %q, want per-directory entries", history)
	}
}

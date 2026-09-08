package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// kvitConfig is the smallest file the discovery treats as one of ours.
const kvitConfig = "llm:\n  base_url: \"https://example.invalid/v1\"\n"

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestResolvePathPrefersExplicit(t *testing.T) {
	t.Setenv(ConfigPathEnv, "/from/env.yaml")
	got, note, err := ResolvePath("/named/on/the/command-line.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got != "/named/on/the/command-line.yaml" {
		t.Errorf("got %q, want the explicit path", got)
	}
	if note != "" {
		t.Errorf("unexpected note %q", note)
	}
}

func TestResolvePathUsesEnvironment(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "somewhere.yaml")
	writeFile(t, cfg, kvitConfig)
	t.Setenv(ConfigPathEnv, cfg)

	got, _, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != cfg {
		t.Errorf("got %q, want %q", got, cfg)
	}
}

func TestResolvePathPrefersWorkingDirectory(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv(ConfigPathEnv, "")
	t.Setenv("HOME", home)
	t.Chdir(work)
	writeFile(t, filepath.Join(home, ".kvit-coder", "config.yaml"), kvitConfig)
	writeFile(t, filepath.Join(work, "config.yaml"), kvitConfig)

	got, note, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(work, "config.yaml"); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if note != "" {
		t.Errorf("unexpected note %q", note)
	}
}

func TestResolvePathSkipsForeignConfigYAML(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv(ConfigPathEnv, "")
	t.Setenv("HOME", home)
	t.Chdir(work)
	// A Hugo site's config.yaml, say: right name, nothing to do with us.
	writeFile(t, filepath.Join(work, "config.yaml"), "baseURL: https://example.com/\ntitle: A Site\n")
	userConfig := filepath.Join(home, ".kvit-coder", "config.yaml")
	writeFile(t, userConfig, kvitConfig)

	got, note, err := ResolvePath("")
	if err != nil {
		t.Fatal(err)
	}
	if got != userConfig {
		t.Errorf("got %q, want the config under HOME (%q)", got, userConfig)
	}
	if !strings.Contains(note, "ignoring") {
		t.Errorf("expected a note about the skipped file, got %q", note)
	}
}

func TestResolveNamedTakesAnyBenchmarkConfigAtFaceValue(t *testing.T) {
	work := t.TempDir()
	t.Setenv(ConfigPathEnv, "")
	t.Chdir(work)
	bench := filepath.Join(work, "config-bench.yaml")
	// No llm: section: a file named for a benchmark is not being confused
	// with another program's, so it is used as it is.
	writeFile(t, bench, "agent:\n  profile: strong\n")

	got, _, err := ResolveNamed("", "config-bench.yaml")
	if err != nil {
		t.Fatal(err)
	}
	if got != bench {
		t.Errorf("got %q, want %q", got, bench)
	}
}

func TestResolvePathReportsWhereItLooked(t *testing.T) {
	home := t.TempDir()
	work := t.TempDir()
	t.Setenv(ConfigPathEnv, "")
	t.Setenv("HOME", home)
	t.Chdir(work)

	_, _, err := ResolvePath("")
	if err == nil {
		t.Fatal("expected an error when there is no config anywhere")
	}
	for _, want := range []string{work, filepath.Join(home, ".kvit-coder"), ConfigPathEnv} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not mention %q", err, want)
		}
	}
}

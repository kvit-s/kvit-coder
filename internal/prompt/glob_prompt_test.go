package prompt

import (
	"strings"
	"testing"
)

// The Glob tool must be advertised with discovery-first steering, and Read
// must no longer offer directory listing, under both profiles.
func TestPromptAdvertisesGlob(t *testing.T) {
	cfg := promptTestConfig("weak")
	cfg.Tools.Glob.Enabled = true
	weak := generate(t, cfg)

	for _, wanted := range []string{
		"### Glob - Find Files",
		"1. Find relevant files using Glob tool",
		"finding files",
		`Glob {"pattern": "**/*.go"}`,
	} {
		if !strings.Contains(weak, wanted) {
			t.Errorf("the weak prompt does not mention %q", wanted)
		}
	}
	if strings.Contains(weak, `Read {"path": "src/"}`) {
		t.Error("the weak prompt still offers Read for directory listing")
	}

	strongCfg := promptTestConfig("strong")
	strongCfg.Tools.Glob.Enabled = true
	strong := generate(t, strongCfg)

	for _, wanted := range []string{
		"### Glob",
		"use Glob to list it",
	} {
		if !strings.Contains(strong, wanted) {
			t.Errorf("the strong prompt does not mention %q", wanted)
		}
	}
}

// Without Glob, discovery falls back to the shell explicitly rather than
// being silently skipped.
func TestPromptWithoutGlobFallsBackToShell(t *testing.T) {
	weak := generate(t, promptTestConfig("weak"))
	if !strings.Contains(weak, "Find relevant files using Shell tool (ls, find)") {
		t.Error("the weak prompt has no discovery fallback without Glob")
	}
}

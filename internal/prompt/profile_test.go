package prompt

import (
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/tools"
)

func promptTestConfig(profile string) *config.Config {
	cfg := &config.Config{}
	cfg.Agent.Profile = profile
	cfg.Workspace.Root = "."
	cfg.Tools.Read.Enabled = true
	cfg.Tools.Read.MaxPartialLines = 150
	cfg.Tools.Read.MaxReadSizeKB = 24
	cfg.Tools.Edit.Enabled = true
	cfg.Tools.Edit.Mode = "searchreplace"
	cfg.Tools.Search.Enabled = true
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.Shell.DefaultTimeout = 120
	cfg.Tools.Shell.MaxTimeout = 600
	cfg.Tools.Procs.Enabled = true
	cfg.Tools.Question.Enabled = true
	cfg.Tools.Batch.Enabled = true
	cfg.Tools.SafetyConfirmations = map[string]config.SafetyConfirmation{}
	return cfg
}

func generate(t *testing.T, cfg *config.Config) string {
	t.Helper()
	tempMgr := tools.NewTempFileManager(t.TempDir())
	registry := tools.SetupRegistry(tools.SetupConfig{Cfg: cfg, TempFileMgr: tempMgr})
	gen, err := NewGenerator(registry, cfg)
	if err != nil {
		t.Fatalf("NewGenerator: %v", err)
	}
	out, err := gen.GenerateSystemPrompt()
	if err != nil {
		t.Fatalf("GenerateSystemPrompt: %v", err)
	}
	return out
}

// TestStrongPromptDropsTheInstructions: the numbered workflow, the worked
// example and the list of what the tools are for are instructions to a model
// that needs them, and are not in the strong prompt.
func TestStrongPromptDropsTheInstructions(t *testing.T) {
	strong := generate(t, promptTestConfig("strong"))

	for _, unwanted := range []string{"# WORKFLOW", "# EXAMPLE", "# MAIN TASKS", "Key Rules"} {
		if strings.Contains(strong, unwanted) {
			t.Errorf("the strong prompt still has %q", unwanted)
		}
	}
	for _, wanted := range []string{"# ROLE", "# ENVIRONMENT", "# HOW THIS SESSION WORKS", "# TOOLS"} {
		if !strings.Contains(strong, wanted) {
			t.Errorf("the strong prompt is missing %q", wanted)
		}
	}
}

// TestWeakPromptIsUnchanged: profile: weak has to keep the long prompts, or a
// comparison between the two is measuring two things at once.
func TestWeakPromptIsUnchanged(t *testing.T) {
	weak := generate(t, promptTestConfig("weak"))
	for _, wanted := range []string{"# WORKFLOW", "# EXAMPLE", "# MAIN TASKS", "**Key Rules:**"} {
		if !strings.Contains(weak, wanted) {
			t.Errorf("the weak prompt is missing %q", wanted)
		}
	}
}

// TestStrongPromptIsShorter states the size difference as a fact rather than a
// hope, and fails if the strong prompt ever grows back.
func TestStrongPromptIsShorter(t *testing.T) {
	strong := generate(t, promptTestConfig("strong"))
	weak := generate(t, promptTestConfig("weak"))

	t.Logf("strong prompt: %d bytes, weak prompt: %d bytes", len(strong), len(weak))
	if len(strong) >= len(weak) {
		t.Errorf("the strong prompt is %d bytes and the weak one %d: it is not shorter",
			len(strong), len(weak))
	}
}

// TestStrongPromptSaysWhatTheSchemaCannot: what is left after the instructions
// are cut has to be the things a tool schema has no way to express.
func TestStrongPromptSaysWhatTheSchemaCannot(t *testing.T) {
	strong := generate(t, promptTestConfig("strong"))

	for _, wanted := range []string{
		"byte for byte",     // Edit's matching rule
		"fresh shell",       // the shell's statelessness
		"temp file",         // where large output goes
		"Truncates at",      // Read's limits
		"Results degrade",   // Search's
		"<user-steering>",   // a message arriving mid-turn
		"<system-reminder>", // and what one is
		"Observe.wait",      // waiting for background work
		"Question",          // when to ask
		"Batch",             // sending several calls at once
	} {
		if !strings.Contains(strong, wanted) {
			t.Errorf("the strong prompt does not mention %q", wanted)
		}
	}
}

// TestEnvironmentBlockDescribesThisSession: the part of the prompt that cannot
// come from a template because it is different every time.
func TestEnvironmentBlockDescribesThisSession(t *testing.T) {
	env := Environment(".")
	for _, wanted := range []string{"Working directory:", "Platform:", "Today:"} {
		if !strings.Contains(env, wanted) {
			t.Errorf("the environment block is missing %q:\n%s", wanted, env)
		}
	}
	// This repository is a git checkout, so the block should say so.
	if !strings.Contains(env, "Git branch:") {
		t.Errorf("the environment block does not name the git branch:\n%s", env)
	}
	if !strings.Contains(env, "Top level:") {
		t.Errorf("the environment block does not list the workspace root:\n%s", env)
	}
}

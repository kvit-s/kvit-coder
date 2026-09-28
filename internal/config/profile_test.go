package config

import (
	"os"
	"path/filepath"
	"testing"
)

// perModelConfig writes a config with a global profile and two catalog
// entries (one strong, one weak) plus weak-machinery settings the strong
// profile forces off. It returns the loaded config.
func perModelConfig(t *testing.T, globalProfile string) *Config {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `
agent:
  profile: ` + globalProfile + `
llm:
  model: wire-strong
  base_url: "https://strong/v1"
workspace:
  root: "."
backtrack:
  enabled: true
diagnostics:
  interrogate_on_anomaly: true
tools:
  edit:
    enabled: true
    preview_mode: true
    fuzzy_threshold: 0.8
models:
  - id: "strong-entry"
    name: "Strong"
    model: "wire-strong"
    base_url: "https://strong/v1"
    api_backend: "responses"
    profile: strong
  - id: "weak-entry"
    name: "Weak"
    model: "wire-weak"
    base_url: "https://weak/v1"
    api_backend: "responses"
    profile: weak
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	return cfg
}

// TestProfileForFallback: an entry without its own profile inherits the
// global agent.profile; an explicit entry profile wins.
func TestProfileForFallback(t *testing.T) {
	cfg := &Config{}
	cfg.Agent.Profile = "weak"

	plain := ModelEntry{ID: "plain", Model: "wire"}
	if got := cfg.ProfileFor(plain); got != "weak" {
		t.Errorf("ProfileFor(inherit) = %q, want global %q", got, "weak")
	}
	if cfg.IsStrongFor(plain) {
		t.Error("IsStrongFor(inherit weak) = true, want false")
	}

	strong := ModelEntry{ID: "s", Model: "wire", Profile: "strong"}
	if got := cfg.ProfileFor(strong); got != "strong" {
		t.Errorf("ProfileFor(override) = %q, want %q", got, "strong")
	}
	if !cfg.IsStrongFor(strong) {
		t.Error("IsStrongFor(override strong) = false, want true")
	}

	// Case-insensitive, like agent.profile itself.
	weak := ModelEntry{ID: "w", Model: "wire", Profile: "Weak"}
	if cfg.IsStrongFor(weak) {
		t.Error("IsStrongFor(Weak) = true, want false")
	}
}

// TestValidateModelsRejectsBadProfile: a typo in an entry's profile fails at
// load with the file path, not mid-session at the API.
func TestValidateModelsRejectsBadProfile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `
llm:
  model: wire
workspace:
  root: "."
models:
  - id: "bad"
    name: "Bad"
    model: "wire"
    profile: medium
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if _, err := Load(path); err == nil {
		t.Fatal("Load with profile: medium: want error, got nil")
	}
}

// TestLoadAppliesDefaultModelProfile: with a mixed catalog, Load applies the
// default entry's effective profile — not just the global one.
func TestLoadAppliesDefaultModelProfile(t *testing.T) {
	// Default row (wire-strong) is strong: the file's weak settings are off.
	cfg := perModelConfig(t, "strong")
	if !cfg.Agent.IsStrong() {
		t.Fatal("default strong entry loaded as weak")
	}
	if cfg.Backtrack.Enabled || cfg.Tools.Edit.PreviewMode ||
		cfg.Tools.Edit.FuzzyThreshold != 0 || !cfg.Tools.Edit.ExactMatchOnly ||
		cfg.Diagnostics.InterrogateOnAnomaly {
		t.Errorf("strong default entry left weak machinery on: %+v", cfg.Tools.Edit)
	}

	// Same file but defaulting to the weak entry: the file comes through as
	// written.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := `
agent:
  profile: strong
llm:
  model: wire-weak
  base_url: "https://weak/v1"
workspace:
  root: "."
backtrack:
  enabled: true
diagnostics:
  interrogate_on_anomaly: true
tools:
  edit:
    enabled: true
    preview_mode: true
    fuzzy_threshold: 0.8
models:
  - id: "strong-entry"
    name: "Strong"
    model: "wire-strong"
    base_url: "https://strong/v1"
    api_backend: "responses"
    profile: strong
  - id: "weak-entry"
    name: "Weak"
    model: "wire-weak"
    base_url: "https://weak/v1"
    api_backend: "responses"
    profile: weak
`
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	weakCfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if weakCfg.Agent.IsStrong() {
		t.Fatal("default weak entry loaded as strong")
	}
	if !weakCfg.Backtrack.Enabled || !weakCfg.Tools.Edit.PreviewMode ||
		weakCfg.Tools.Edit.FuzzyThreshold != 0.8 ||
		!weakCfg.Diagnostics.InterrogateOnAnomaly {
		t.Errorf("weak default entry did not keep the file's settings: %+v", weakCfg.Tools.Edit)
	}
}

// TestApplyModelSwitchesProfileReversibly: selecting a weak entry restores
// the file's settings, switching back to strong forces them off again, and
// an entry without its own profile falls back to the global default.
func TestApplyModelSwitchesProfileReversibly(t *testing.T) {
	cfg := perModelConfig(t, "strong")

	weak := cfg.Models[1]
	cfg.ApplyModel(weak, "")
	if cfg.Agent.IsStrong() {
		t.Fatal("ApplyModel(weak) stayed strong")
	}
	if !cfg.Backtrack.Enabled || !cfg.Tools.Edit.PreviewMode ||
		cfg.Tools.Edit.FuzzyThreshold != 0.8 ||
		!cfg.Diagnostics.InterrogateOnAnomaly {
		t.Errorf("ApplyModel(weak) did not restore the file: backtrack=%v preview=%v fuzzy=%v interrogate=%v",
			cfg.Backtrack.Enabled, cfg.Tools.Edit.PreviewMode,
			cfg.Tools.Edit.FuzzyThreshold, cfg.Diagnostics.InterrogateOnAnomaly)
	}

	strong := cfg.Models[0]
	cfg.ApplyModel(strong, "")
	if !cfg.Agent.IsStrong() {
		t.Fatal("ApplyModel(strong) stayed weak")
	}
	if cfg.Backtrack.Enabled || cfg.Tools.Edit.PreviewMode ||
		cfg.Tools.Edit.FuzzyThreshold != 0 || !cfg.Tools.Edit.ExactMatchOnly ||
		cfg.Diagnostics.InterrogateOnAnomaly {
		t.Error("ApplyModel(strong) left weak machinery on")
	}

	// Back to weak: the snapshot restores, it does not keep the forced-off
	// values from the strong turn.
	cfg.ApplyModel(weak, "")
	if cfg.Agent.IsStrong() || !cfg.Tools.Edit.PreviewMode ||
		cfg.Tools.Edit.FuzzyThreshold != 0.8 {
		t.Error("second ApplyModel(weak) did not restore the file's settings")
	}

	// An entry without its own profile follows the global default (strong
	// here), including the forced-off machinery.
	plain := ModelEntry{ID: "plain", Name: "Plain", Model: "wire-plain",
		BaseURL: "https://plain/v1", APIBackend: "responses"}
	cfg.ApplyModel(plain, "")
	if !cfg.Agent.IsStrong() {
		t.Error("ApplyModel(entry without profile) did not fall back to the global strong default")
	}
	if cfg.Tools.Edit.PreviewMode {
		t.Error("fallback to global strong left preview_mode on")
	}
}

// TestApplyProfileWeakGlobal: a globally weak config whose default entry is
// explicitly strong still starts strong; switching to the weak entry
// restores the file.
func TestApplyProfileWeakGlobal(t *testing.T) {

	cfg := perModelConfig(t, "weak")

	if !cfg.Agent.IsStrong() {
		t.Fatal("strong default entry loaded as weak under a weak global")
	}

	// Default row is wire-strong (strong entry), so Load applied strong even
	// though the global default is weak.
	if cfg.Tools.Edit.PreviewMode {
		t.Fatal("strong default entry left preview_mode on under a weak global")
	}

	weak := cfg.Models[1]
	cfg.ApplyModel(weak, "")
	if cfg.Agent.IsStrong() || !cfg.Tools.Edit.PreviewMode {
		t.Error("ApplyModel(weak) under a weak global did not restore the file")
	}
}

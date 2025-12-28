package tools

import "github.com/kvit-s/kvit-coder/internal/config"

// newTestConfig creates a minimal config for tool tests.
func newTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.Workspace.Root = "."
	cfg.Tools.Shell.Enabled = true
	cfg.Tools.SafetyConfirmations = make(map[string]config.SafetyConfirmation)
	return cfg
}

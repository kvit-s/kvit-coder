package main

import (
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// selectionTestConfig is a three-entry catalog: two reasoning models and a
// Qwen-shaped chat_completions entry with its own menu.
func selectionTestConfig() *config.Config {
	cfg := &config.Config{}
	cfg.LLM.Model = "wire-go"
	cfg.Models = []config.ModelEntry{
		{ID: "go", Name: "Go", Model: "wire-go", BaseURL: "https://go/v1", APIBackend: "responses",
			Efforts: []config.EffortOption{{Value: "low"}, {Value: "high", Default: true}, {Value: "xhigh"}}},
		{ID: "zen", Name: "Zen", Model: "wire-zen", BaseURL: "https://zen/v1", APIBackend: "responses",
			Efforts: []config.EffortOption{{Value: "low"}, {Value: "high", Default: true}}},
		{ID: "qwen", Name: "Qwen", Model: "wire-qwen", BaseURL: "http://local/v1", APIBackend: "chat_completions",
			Efforts: []config.EffortOption{{Value: "low", Default: true}, {Value: "medium"}, {Value: "high"}, {Value: "xhigh"}}},
	}
	return cfg
}

// TestResolveModelSelectionInlineEffort: -m m3:xhigh resolves the row and
// the effort in one flag; an explicit -e wins over the inline one.
func TestResolveModelSelectionInlineEffort(t *testing.T) {
	cfg := selectionTestConfig()
	resolveModelSelection(cfg, "m3:xhigh", &stringFlag{}, "", "")
	if cfg.LLM.Model != "wire-qwen" || cfg.LLM.BaseURL != "http://local/v1" {
		t.Fatalf("inline selection reached %+v, want qwen entry", cfg.LLM)
	}
	if cfg.LLM.ReasoningEffort != "xhigh" {
		t.Fatalf("inline effort = %q, want xhigh", cfg.LLM.ReasoningEffort)
	}
	if got := cfg.ModelDisplay(); got != "wire-qwen:xhigh" {
		t.Fatalf("display = %q, want wire-qwen:xhigh", got)
	}

	cfg = selectionTestConfig()
	resolveModelSelection(cfg, "m1:xhigh", &stringFlag{set: true, val: "low"}, "", "")
	if cfg.LLM.Model != "wire-go" || cfg.LLM.ReasoningEffort != "low" {
		t.Fatalf("explicit effort lost to inline: %+v", cfg.LLM)
	}

	cfg = selectionTestConfig()
	resolveModelSelection(cfg, "qwen", &stringFlag{}, "", "")
	if cfg.LLM.Model != "wire-qwen" || cfg.LLM.ReasoningEffort != "low" {
		t.Fatalf("bare id did not take entry default: %+v", cfg.LLM)
	}
}

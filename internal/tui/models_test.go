package tui

import (
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// testCatalogUI builds a UI over a three-entry catalog: two reasoning models
// and one local non-reasoning model. Command output goes to stdout, which
// tests ignore; only the selection state is asserted.
func testCatalogUI(t *testing.T) *UI {
	t.Helper()
	cfg := &config.Config{}
	cfg.LLM.Model = "wire-go"
	cfg.LLM.BaseURL = "https://go/v1"
	cfg.LLM.APIBackend = "responses"
	cfg.LLM.ReasoningEffort = "xhigh"
	cfg.Models = []config.ModelEntry{
		{ID: "go", Name: "Go", Model: "wire-go", BaseURL: "https://go/v1", APIBackend: "responses",
			Efforts: []config.EffortOption{{Value: "low"}, {Value: "high", Default: true}}},
		{ID: "zen", Name: "Zen", Model: "wire-zen", BaseURL: "https://zen/v1", APIBackend: "responses",
			Efforts: []config.EffortOption{{Value: "low"}, {Value: "high", Default: true}}},
		{ID: "qwen", Name: "Qwen", Model: "wire-qwen", BaseURL: "http://local/v1", APIBackend: "chat_completions"},
	}
	u := New(Options{Config: cfg})
	if u.currentModel != 0 {
		t.Fatalf("default row = %d, want 0 (the llm.model row)", u.currentModel)
	}
	return u
}

// TestModelSwitchResetsEffort: :mN selects the row and resets the effort to
// the new entry's default; switching to a model with no menu clears it.
func TestModelSwitchResetsEffort(t *testing.T) {
	u := testCatalogUI(t)
	if got := u.activeDisplay(); got != "wire-go:high" {
		t.Fatalf("initial display = %q, want wire-go:high", got)
	}
	u.handleCommand(":e1")
	if got := u.effectiveEffort(); got != "low" {
		t.Fatalf("after :e1 effort = %q, want low", got)
	}
	u.handleCommand(":m3")
	if u.currentModel != 2 {
		t.Fatalf("after :m3 row = %d, want 2", u.currentModel)
	}
	if got := u.effectiveEffort(); got != "" {
		t.Fatalf("after :m3 effort = %q, want cleared", got)
	}
	if got := u.activeDisplay(); got != "wire-qwen" {
		t.Fatalf("after :m3 display = %q, want plain wire id", got)
	}
	if got := u.activeBaseURL(); got != "http://local/v1" {
		t.Fatalf("after :m3 base = %q", got)
	}
	u.handleCommand(":m1")
	if got := u.activeDisplay(); got != "wire-go:high" {
		t.Fatalf("after :m1 display = %q, want entry default back", got)
	}
}

// TestModelSwitchOutOfRange: :m9 changes nothing and does not exit.
func TestModelSwitchOutOfRange(t *testing.T) {
	u := testCatalogUI(t)
	if exit := u.handleCommand(":m9"); exit {
		t.Fatal(":m9 exited the UI")
	}
	if u.currentModel != 0 {
		t.Fatalf("after :m9 row = %d, want unchanged 0", u.currentModel)
	}
}

// TestEffortCommands: :eN follows the current menu, :e takes a value, and
// both reject what the model does not offer without changing state.
func TestEffortCommands(t *testing.T) {
	u := testCatalogUI(t)
	u.handleCommand(":e low")
	if got := u.effectiveEffort(); got != "low" {
		t.Fatalf("after ':e low' effort = %q", got)
	}
	u.handleCommand(":e bogus")
	if got := u.effectiveEffort(); got != "low" {
		t.Fatalf("after ':e bogus' effort = %q, want unchanged low", got)
	}
	u.handleCommand(":e9")
	if got := u.effectiveEffort(); got != "low" {
		t.Fatalf("after :e9 effort = %q, want unchanged low", got)
	}
	// No menu: :eN and :e both refuse.
	u.handleCommand(":m3")
	u.handleCommand(":e1")
	u.handleCommand(":e low")
	if got := u.effectiveEffort(); got != "" {
		t.Fatalf("effort on menu-less model = %q, want empty", got)
	}
}

// TestLegacySingleModelUI: without models: the catalog is the one
// synthesized row and :m1 still addresses it.
func TestLegacySingleModelUI(t *testing.T) {
	cfg := &config.Config{}
	cfg.LLM.Model = "wire-go"
	cfg.LLM.BaseURL = "https://go/v1"
	cfg.LLM.APIBackend = "responses"
	cfg.LLM.ReasoningEffort = "xhigh"
	u := New(Options{Config: cfg})
	if len(u.models) != 1 {
		t.Fatalf("catalog has %d rows, want 1", len(u.models))
	}
	if got := u.activeDisplay(); got != "wire-go:xhigh" {
		t.Fatalf("display = %q, want wire-go:xhigh", got)
	}
	u.handleCommand(":m1")
	if u.currentModel != 0 || u.activeDisplay() != "wire-go:xhigh" {
		t.Fatalf("after :m1 display = %q", u.activeDisplay())
	}
}

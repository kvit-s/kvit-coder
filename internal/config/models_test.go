package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestModelListSynthesizesLegacy: an old config with only llm: yields a
// one-entry catalog whose display is byte-identical to ModelDisplay, so old
// configs behave exactly as before.
func TestModelListSynthesizesLegacy(t *testing.T) {
	cfg := &Config{}
	cfg.LLM.Model = "muse-spark-1.3-contributor"
	cfg.LLM.BaseURL = "https://opencode.ai/zen/go/v1"
	cfg.LLM.APIBackend = "responses"
	cfg.LLM.ReasoningEffort = "xhigh"
	cfg.LLM.Context = 1048576

	list := cfg.ModelList()
	if len(list) != 1 {
		t.Fatalf("ModelList() has %d entries, want 1", len(list))
	}
	e := list[0]
	if e.Model != cfg.LLM.Model || e.BaseURL != cfg.LLM.BaseURL || e.Context != cfg.LLM.Context {
		t.Errorf("synthesized entry %+v does not mirror llm: block", e)
	}
	if got := EntryDisplay(e, cfg.DefaultEffort(e)); got != cfg.ModelDisplay() {
		t.Errorf("EntryDisplay = %q, ModelDisplay = %q, want identical", got, cfg.ModelDisplay())
	}
	menu := cfg.EffortOptions(e)
	if len(menu) != len(LegacyEfforts) {
		t.Errorf("synthesized responses entry menu = %v, want legacy %v", menu, LegacyEfforts)
	}
	if got := cfg.DefaultEffort(e); got != "xhigh" {
		t.Errorf("DefaultEffort = %q, want llm.reasoning_effort %q", got, "xhigh")
	}
}

// TestModelListSynthesizesChatCompletions: a legacy local endpoint has no
// effort menu.
func TestModelListSynthesizesChatCompletions(t *testing.T) {
	cfg := &Config{}
	cfg.LLM.Model = "ministral-3-14b"
	cfg.LLM.BaseURL = "http://192.168.8.20:8080/v1"
	if menu := cfg.EffortOptions(cfg.ModelList()[0]); len(menu) != 0 {
		t.Errorf("chat_completions entry menu = %v, want empty", menu)
	}
}

func multiModelConfig() *Config {
	cfg := &Config{}
	cfg.LLM.ReasoningEffort = "xhigh"
	cfg.Models = []ModelEntry{
		{ID: "spark-go", Name: "Muse Spark 1.3 Go", Model: "muse-spark-1.3-contributor",
			BaseURL: "https://opencode.ai/zen/go/v1", APIBackend: "responses",
			Efforts: []EffortOption{{Value: "minimal"}, {Value: "low"}, {Value: "medium"}, {Value: "high", Default: true}, {Value: "xhigh"}}},
		{ID: "spark-zen", Name: "Muse Spark 1.3 Zen Free", Model: "muse-spark-1.3-contributor-free",
			BaseURL: "https://opencode.ai/zen/v1", APIBackend: "responses",
			Efforts: []EffortOption{{Value: "low"}, {Value: "high", Default: true}}},
		{ID: "qwen-local", Name: "Qwen3.8-27B (local)", Model: "qwen3.8-27b-nvfp4-dflash2",
			BaseURL: "http://192.168.138.186:8090/v1", APIBackend: "chat_completions"},
	}
	return cfg
}

// TestResolveModel: index, id, display name and wire id all find the row.
func TestResolveModel(t *testing.T) {
	cfg := multiModelConfig()
	for ref, wantID := range map[string]string{
		"1": "spark-go", "2": "spark-zen", "3": "qwen-local",
		"spark-go":                   "spark-go",
		"SPARK-ZEN":                  "spark-zen", // ids match case-insensitively
		"Muse Spark 1.3 Go":          "spark-go",
		"muse spark 1.3 zen free":    "spark-zen", // names match case-insensitively
		"muse-spark-1.3-contributor": "spark-go",  // wire id
		"qwen3.8-27b-nvfp4-dflash2":  "qwen-local",
		"  spark-go  ":               "spark-go", // surrounding space is trimmed
	} {
		e, idx, err := cfg.ResolveModel(ref)
		if err != nil {
			t.Errorf("ResolveModel(%q): %v", ref, err)
			continue
		}
		if e.ID != wantID {
			t.Errorf("ResolveModel(%q).ID = %q, want %q", ref, e.ID, wantID)
		}
		if cfg.ModelList()[idx].ID != wantID {
			t.Errorf("ResolveModel(%q) index %d does not point at %q", ref, idx, wantID)
		}
	}
}

// TestResolveModelUnknown: the error lists what is offered, for :m9 and for
// a headless --model alike.
func TestResolveModelUnknown(t *testing.T) {
	cfg := multiModelConfig()
	for _, ref := range []string{"9", "0", "nope", ""} {
		_, _, err := cfg.ResolveModel(ref)
		if err == nil {
			t.Errorf("ResolveModel(%q): want error, got nil", ref)
			continue
		}
		if !strings.Contains(err.Error(), "use one of:") {
			t.Errorf("ResolveModel(%q) error %q does not list offered models", ref, err)
		}
	}
}

// TestDefaultEffort: entry default wins, then llm.reasoning_effort, then
// default_effort, then "". A model with no menu always yields "".
func TestDefaultEffort(t *testing.T) {
	cfg := multiModelConfig()
	goEntry, _, _ := cfg.ResolveModel("spark-go")
	if got := cfg.DefaultEffort(goEntry); got != "high" {
		t.Errorf("DefaultEffort(spark-go) = %q, want entry default %q", got, "high")
	}
	qwen, _, _ := cfg.ResolveModel("qwen-local")
	if got := cfg.DefaultEffort(qwen); got != "" {
		t.Errorf("DefaultEffort(qwen-local) = %q, want empty (no menu clears)", got)
	}
	// The fallback chain past the entry default, on a responses entry with
	// a menu but no default row.
	bare := ModelEntry{ID: "bare", Name: "Bare", Model: "b", APIBackend: "responses"}
	if got := cfg.DefaultEffort(bare); got != "xhigh" {
		t.Errorf("DefaultEffort(bare) = %q, want llm.reasoning_effort fallback", got)
	}
	cfg.LLM.ReasoningEffort = ""
	cfg.DefaultEffortValue = "medium"
	if got := cfg.DefaultEffort(bare); got != "medium" {
		t.Errorf("DefaultEffort(bare) = %q, want default_effort fallback", got)
	}
	if got := cfg.DefaultEffort(qwen); got != "" {
		t.Errorf("DefaultEffort(qwen-local) = %q, want empty even with default_effort set", got)
	}
	cfg.DefaultEffortValue = ""
	if got := cfg.DefaultEffort(bare); got != "" {
		t.Errorf("DefaultEffort(bare) = %q, want empty (omit reasoning:)", got)
	}
}

// TestResolveEffortGate: only values the model offers resolve; "" clears;
// a model with no menu rejects everything but "".
func TestResolveEffortGate(t *testing.T) {
	cfg := multiModelConfig()
	goEntry, _, _ := cfg.ResolveModel("spark-go")
	if v, err := cfg.ResolveEffort(goEntry, "HIGH"); err != nil || v != "high" {
		t.Errorf("ResolveEffort(spark-go, HIGH) = %q, %v; want high, nil", v, err)
	}
	if v, err := cfg.ResolveEffort(goEntry, ""); err != nil || v != "" {
		t.Errorf("ResolveEffort(spark-go, empty) = %q, %v; want empty, nil", v, err)
	}
	if _, err := cfg.ResolveEffort(goEntry, "max"); err == nil {
		t.Error("ResolveEffort(spark-go, max): want rejection (not in menu), got nil")
	} else if !strings.Contains(err.Error(), "use one of:") {
		t.Errorf("ResolveEffort(spark-go, max) error %q does not list offered levels", err)
	}
	qwen, _, _ := cfg.ResolveModel("qwen-local")
	if _, err := cfg.ResolveEffort(qwen, "high"); err == nil {
		t.Error("ResolveEffort(qwen-local, high): want unsupported error, got nil")
	} else if !strings.Contains(err.Error(), "does not support reasoning effort") {
		t.Errorf("ResolveEffort(qwen-local, high) error %q, want unsupported message", err)
	}
	if v, err := cfg.ResolveEffort(qwen, ""); err != nil || v != "" {
		t.Errorf("ResolveEffort(qwen-local, empty) = %q, %v; want empty, nil", v, err)
	}
}

// TestApplyModel: the entry's endpoint fields overwrite llm:, the rest stays.
func TestApplyModel(t *testing.T) {
	cfg := &Config{}
	cfg.LLM.Model = "old"
	cfg.LLM.BaseURL = "https://old/v1"
	cfg.LLM.APIBackend = "responses"
	cfg.LLM.APIKey = "old-key"
	cfg.LLM.Headers = []string{"x-opencode-session=kvit-coder-${KVIT_RUN_ID}"}
	cfg.LLM.MergeThinking = true
	cfg.LLM.Context = 8

	qwen := ModelEntry{ID: "qwen-local", Name: "Q", Model: "qwen-wire",
		BaseURL: "http://local/v1", APIBackend: "chat_completions", Context: 262144}
	cfg.ApplyModel(qwen, "")
	if cfg.LLM.Model != "qwen-wire" || cfg.LLM.BaseURL != "http://local/v1" ||
		cfg.LLM.APIBackend != "chat_completions" || cfg.LLM.Context != 262144 {
		t.Errorf("ApplyModel did not take the entry endpoint: %+v", cfg.LLM)
	}
	if cfg.LLM.ReasoningEffort != "" || cfg.LLM.APIKey != "" {
		t.Errorf("ApplyModel did not clear effort/key: %+v", cfg.LLM)
	}
	if len(cfg.LLM.Headers) != 1 || !cfg.LLM.MergeThinking {
		t.Errorf("ApplyModel touched non-endpoint settings: %+v", cfg.LLM)
	}
	if got := cfg.ModelDisplay(); got != "qwen-wire" {
		t.Errorf("ModelDisplay after switch = %q, want plain wire id", got)
	}

	goEntry := ModelEntry{ID: "spark-go", Model: "spark-wire", BaseURL: "https://go/v1",
		APIBackend: "responses", APIKeyEnv: "MODELS_TEST_KEY", Context: 1048576}
	t.Setenv("MODELS_TEST_KEY", "sekret")
	cfg.ApplyModel(goEntry, "low")
	if cfg.LLM.APIKey != "sekret" || cfg.LLM.APIKeyEnv != "MODELS_TEST_KEY" {
		t.Errorf("ApplyModel did not resolve api_key_env: %+v", cfg.LLM)
	}
	if got := cfg.ModelDisplay(); got != "spark-wire:low" {
		t.Errorf("ModelDisplay after switch = %q, want spark-wire:low", got)
	}
}

// TestWireID: strips a trailing :effort, leaves other colons alone.
func TestWireID(t *testing.T) {
	if got := WireID("muse-spark-1.3-contributor:xhigh"); got != "muse-spark-1.3-contributor" {
		t.Errorf("WireID = %q", got)
	}
	if got := WireID("muse-spark-1.3-contributor"); got != "muse-spark-1.3-contributor" {
		t.Errorf("WireID = %q", got)
	}
	if got := WireID("openai:gpt-4"); got != "openai:gpt-4" {
		t.Errorf("WireID stripped a non-effort suffix: %q", got)
	}
}

// TestParseIndexedCommand: m/e plus digits only.
func TestParseIndexedCommand(t *testing.T) {
	for _, tc := range []struct {
		in   string
		kind byte
		n    int
		ok   bool
	}{
		{"m1", 'm', 1, true},
		{"e12", 'e', 12, true},
		{"m", 0, 0, false},
		{"e", 0, 0, false},
		{"model", 0, 0, false},
		{"memory", 0, 0, false},
		{"m1x", 0, 0, false},
		{"switch", 0, 0, false},
		{"", 0, 0, false},
	} {
		kind, n, ok := ParseIndexedCommand(tc.in)
		if kind != tc.kind || n != tc.n || ok != tc.ok {
			t.Errorf("ParseIndexedCommand(%q) = (%q, %d, %v), want (%q, %d, %v)",
				tc.in, kind, n, ok, tc.kind, tc.n, tc.ok)
		}
	}
}

func writeTempConfig(t *testing.T, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

const legacyConfigBody = "llm:\n  model: \"muse-spark-1.3-contributor\"\n  reasoning_effort: \"xhigh\"\n"

// TestLoadLegacyUnchanged: an old config loads with a nil catalog and an
// identical display.
func TestLoadLegacyUnchanged(t *testing.T) {
	cfg, err := Load(writeTempConfig(t, legacyConfigBody))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(cfg.Models) != 0 {
		t.Errorf("legacy config Models = %v, want nil", cfg.Models)
	}
	if got := cfg.ModelDisplay(); got != "muse-spark-1.3-contributor:xhigh" {
		t.Errorf("ModelDisplay = %q", got)
	}
}

// TestLoadValidatesModels: bad catalogs fail at load with the entry number.
func TestLoadValidatesModels(t *testing.T) {
	bodies := map[string]string{
		"bad backend": "llm:\n  model: a\nmodels:\n  - id: x\n    model: a\n    api_backend: carrier-pigeon\n",
		"bad effort":  "llm:\n  model: a\nmodels:\n  - id: x\n    model: a\n    efforts:\n      - value: turbo\n",
		"dup id":      "llm:\n  model: a\nmodels:\n  - id: x\n    model: a\n  - id: X\n    model: b\n",
		"two default": "llm:\n  model: a\nmodels:\n  - id: x\n    model: a\n    efforts:\n      - value: low\n        default: true\n      - value: high\n        default: true\n",
		"no wire":     "llm:\n  model: a\nmodels:\n  - id: x\n",
		"bad default": "llm:\n  model: a\ndefault_model: ghost\nmodels:\n  - id: x\n    model: a\n",
	}
	for name, body := range bodies {
		if _, err := Load(writeTempConfig(t, body)); err == nil {
			t.Errorf("%s: want load error, got nil", name)
		} else if !strings.Contains(err.Error(), "config.yaml") {
			t.Errorf("%s: error %q does not name the file", name, err)
		}
	}
}

// TestLoadModelsEffortCase: effort values are normalized to lowercase.
func TestLoadModelsEffortCase(t *testing.T) {
	cfg, err := Load(writeTempConfig(t, "llm:\n  model: a\nmodels:\n  - id: x\n    name: X\n    model: a\n    efforts:\n      - value: XHigh\n        default: true\n"))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := cfg.DefaultEffort(cfg.Models[0]); got != "xhigh" {
		t.Errorf("DefaultEffort = %q, want normalized xhigh", got)
	}
}

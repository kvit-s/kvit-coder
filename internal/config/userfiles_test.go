package config

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// useHome points HOME at a fresh directory and returns its .kvit-coder, the
// directory models.yaml and credentials.json are read from.
func useHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	return filepath.Join(home, ".kvit-coder")
}

// loadConfig writes body as a config file in its own directory and loads it.
func loadConfig(t *testing.T, body string, opts LoadOptions) (*Config, error) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.yaml")
	writeFile(t, path, body)
	return LoadWith(path, opts)
}

func modelIDs(cfg *Config) []string {
	var ids []string
	for _, e := range cfg.ModelList() {
		ids = append(ids, e.ID)
	}
	return ids
}

const twoRowConfig = `
models:
  - id: a
    model: wire-a
    base_url: https://a.invalid/v1
  - id: b
    model: wire-b
    base_url: https://b.invalid/v1
`

const savedRows = `
models:
  - id: c
    model: wire-c
    base_url: https://c.invalid/v1
  - id: B
    model: saved-b
    base_url: https://saved.invalid/v1
`

// TestLoadAppendsSavedModels: saved rows follow the config file's rows, so
// :m1 and :m2 keep their meaning, and a saved row reusing an id the config
// file has is left out in favour of the hand-written one.
func TestLoadAppendsSavedModels(t *testing.T) {
	dir := useHome(t)
	writeFile(t, filepath.Join(dir, SavedModelsName), savedRows)

	cfg, err := loadConfig(t, twoRowConfig, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := modelIDs(cfg), []string{"a", "b", "c"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("model ids = %v, want %v", got, want)
	}
	if cfg.Models[1].Model != "wire-b" {
		t.Errorf("row b = %+v, want the config file's row", cfg.Models[1])
	}
	for i, want := range []bool{false, false, true} {
		if got := cfg.FromSavedModels(i); got != want {
			t.Errorf("FromSavedModels(%d) = %v, want %v", i, got, want)
		}
	}
}

// TestLoadKeepsLLMRowBeforeSavedModels: a config with only an llm: block has
// one model; saving another must add to it, not replace it.
func TestLoadKeepsLLMRowBeforeSavedModels(t *testing.T) {
	dir := useHome(t)
	writeFile(t, filepath.Join(dir, SavedModelsName), savedRows)

	cfg, err := loadConfig(t, `
llm:
  base_url: http://localhost:8080/v1
  model: local-model
`, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	list := cfg.ModelList()
	if got, want := modelIDs(cfg), []string{"default", "c", "B"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("model ids = %v, want %v", got, want)
	}
	if list[0].Model != "local-model" || list[0].BaseURL != "http://localhost:8080/v1" {
		t.Errorf("first row = %+v, want the llm: block", list[0])
	}
	if idx := cfg.DefaultModelIndex(); idx != 0 {
		t.Errorf("DefaultModelIndex = %d, want the llm: row", idx)
	}
}

// TestLoadSavedModelsAlone: a config file that names no model at all, which
// is what kvit-coder-ui's setup starts from, gets its models from the saved
// rows only.
func TestLoadSavedModelsAlone(t *testing.T) {
	dir := useHome(t)
	writeFile(t, filepath.Join(dir, SavedModelsName), savedRows)

	cfg, err := loadConfig(t, "llm:\n  request_timeout: 300\n", LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := modelIDs(cfg), []string{"c", "B"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("model ids = %v, want %v", got, want)
	}
}

// TestLoadDefaultModelNamesSavedRow: default_model in config.yaml may name a
// row that only models.yaml has.
func TestLoadDefaultModelNamesSavedRow(t *testing.T) {
	dir := useHome(t)
	writeFile(t, filepath.Join(dir, SavedModelsName), savedRows)

	cfg, err := loadConfig(t, twoRowConfig+"default_model: c\n", LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if idx := cfg.DefaultModelIndex(); idx != 2 {
		t.Errorf("DefaultModelIndex = %d, want 2 (row c from models.yaml)", idx)
	}
}

// TestLoadSkipSavedModels: the benchmark modes load without the saved rows.
func TestLoadSkipSavedModels(t *testing.T) {
	dir := useHome(t)
	writeFile(t, filepath.Join(dir, SavedModelsName), savedRows)

	cfg, err := loadConfig(t, twoRowConfig, LoadOptions{SkipSavedModels: true})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := modelIDs(cfg), []string{"a", "b"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("model ids = %v, want %v", got, want)
	}
	if cfg.FromSavedModels(0) || cfg.FromSavedModels(1) {
		t.Error("FromSavedModels true with the saved rows skipped")
	}
}

// TestLoadSavedModelsErrorsNameTheFile: a mistake in models.yaml is reported
// against models.yaml and its own row number, not against config.yaml.
func TestLoadSavedModelsErrorsNameTheFile(t *testing.T) {
	cases := []struct {
		name, saved, want string
	}{
		{"missing wire id", "models:\n  - id: c\n", "models entry 1 (c): missing model wire id"},
		{"duplicate within the file", "models:\n  - id: c\n    model: x\n  - id: c\n    model: y\n", "models entry 2: duplicate id"},
		{"bad backend", "models:\n  - id: c\n    model: x\n    api_backend: grpc\n", "unknown api_backend"},
		{"config keys copied in", "llm:\n  model: x\n", "field llm not found"},
		{"bad header", "models:\n  - id: c\n    model: x\n    headers: [\"no-equals-sign\"]\n", "is not Key=Value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := useHome(t)
			saved := filepath.Join(dir, SavedModelsName)
			writeFile(t, saved, tc.saved)

			_, err := loadConfig(t, twoRowConfig, LoadOptions{})
			if err == nil {
				t.Fatal("Load accepted a broken models.yaml")
			}
			if !strings.Contains(err.Error(), saved) || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("error %q, want it to name %s and say %q", err, saved, tc.want)
			}
		})
	}
}

// TestEntryAPIKeyOrder: the environment variable, then the key saved under
// its name, then the row's literal key.
func TestEntryAPIKeyOrder(t *testing.T) {
	dir := useHome(t)
	entry := ModelEntry{ID: "go", Model: "kimi-k3", APIKeyEnv: "KVIT_TEST_SAVED_KEY", APIKey: "literal"}
	t.Setenv("KVIT_TEST_SAVED_KEY", "")

	if got := EntryAPIKey(entry); got != "literal" {
		t.Errorf("no variable, nothing saved: key = %q, want the literal", got)
	}

	writeFile(t, filepath.Join(dir, CredentialsName), `{"KVIT_TEST_SAVED_KEY": "saved"}`)
	if got := EntryAPIKey(entry); got != "saved" {
		t.Errorf("variable unset: key = %q, want the saved one", got)
	}

	t.Setenv("KVIT_TEST_SAVED_KEY", "from-env")
	if got := EntryAPIKey(entry); got != "from-env" {
		t.Errorf("variable set: key = %q, want the environment's", got)
	}
}

// TestLoadUsesSavedKeyForLLMBlock: llm.api_key_env finds a saved key too, so
// a single-model config needs no exported variable either.
func TestLoadUsesSavedKeyForLLMBlock(t *testing.T) {
	dir := useHome(t)
	t.Setenv("KVIT_TEST_LLM_KEY", "")
	writeFile(t, filepath.Join(dir, CredentialsName), `{"KVIT_TEST_LLM_KEY": "saved-llm"}`)

	cfg, err := loadConfig(t, `
llm:
  base_url: https://example.invalid/v1
  model: m
  api_key_env: KVIT_TEST_LLM_KEY
`, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.APIKey != "saved-llm" {
		t.Errorf("LLM.APIKey = %q, want the saved key", cfg.LLM.APIKey)
	}
}

// TestLoadRejectsMalformedCredentials: a broken credentials.json stops the
// load with the file's name, rather than reading as "no key" and turning
// into a refused request later.
func TestLoadRejectsMalformedCredentials(t *testing.T) {
	dir := useHome(t)
	path := filepath.Join(dir, CredentialsName)
	writeFile(t, path, `{"KVIT_TEST_LLM_KEY": `)

	_, err := loadConfig(t, twoRowConfig, LoadOptions{})
	if err == nil || !strings.Contains(err.Error(), path) {
		t.Fatalf("error = %v, want one naming %s", err, path)
	}
}

// TestEntryHeadersOverLLMHeaders: an entry's headers are sent on top of
// llm.headers, replace one with the same name whatever its case, and go away
// again when the next ApplyModel selects an entry without them.
func TestEntryHeadersOverLLMHeaders(t *testing.T) {
	t.Setenv("KVIT_RUN_ID", "run1")
	cfg := &Config{}
	cfg.LLM.Headers = []string{"User-Agent=kvit-coder", "X-Base=1"}
	withHeaders := ModelEntry{ID: "go", Model: "kimi-k3",
		Headers: []string{"user-agent=kvit-coder-go", "x-opencode-session=kvit-coder-${KVIT_RUN_ID}"}}
	plain := ModelEntry{ID: "local", Model: "qwen"}

	cfg.ApplyModel(withHeaders, "")
	want := map[string]string{"user-agent": "kvit-coder-go", "X-Base": "1", "x-opencode-session": "kvit-coder-run1"}
	if got := cfg.LLMHeaders(); !reflect.DeepEqual(got, want) {
		t.Errorf("headers for an entry with its own = %v, want %v", got, want)
	}
	if got := cfg.HeadersFor(withHeaders); !reflect.DeepEqual(got, want) {
		t.Errorf("HeadersFor = %v, want %v", got, want)
	}

	cfg.ApplyModel(plain, "")
	want = map[string]string{"User-Agent": "kvit-coder", "X-Base": "1"}
	if got := cfg.LLMHeaders(); !reflect.DeepEqual(got, want) {
		t.Errorf("headers after switching to an entry without any = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(cfg.LLM.Headers, []string{"User-Agent=kvit-coder", "X-Base=1"}) {
		t.Errorf("llm.headers changed to %v", cfg.LLM.Headers)
	}
}

// TestWriteSavedModelsRoundTrip: rows written by kvit-coder-ui read back the
// same, without the empty keys, and appending and removing keep the rest.
func TestWriteSavedModelsRoundTrip(t *testing.T) {
	dir := useHome(t)
	row := ModelEntry{ID: "kimi-k3-go", Name: "Kimi K3 (Go)", Model: "kimi-k3",
		BaseURL: "https://opencode.ai/zen/go/v1", APIBackend: "chat_completions",
		EffortField: "reasoning_effort", APIKeyEnv: "OPENCODE_API_KEY", Context: 1048576,
		Efforts: []EffortOption{{Value: "max", Default: true}},
		Headers: []string{"x-opencode-session=kvit-coder-${KVIT_RUN_ID}"}}
	if err := AppendSavedModels(row); err != nil {
		t.Fatal(err)
	}
	if err := AppendSavedModels(ModelEntry{ID: "local", Model: "qwen", BaseURL: "http://localhost:8080/v1"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, SavedModelsName))
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	if !strings.HasPrefix(text, "# Written by kvit-coder-ui") || strings.Contains(text, "api_key:") || strings.Contains(text, "summarizer") {
		t.Errorf("models.yaml:\n%s", text)
	}
	rows, err := ReadSavedModels()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || !reflect.DeepEqual(rows[0], row) {
		t.Fatalf("read back %+v", rows)
	}
	if removed, err := RemoveSavedModel("KIMI-K3-GO"); err != nil || !removed {
		t.Fatalf("remove: %v %v", removed, err)
	}
	if removed, _ := RemoveSavedModel("kimi-k3-go"); removed {
		t.Error("removed a row twice")
	}
	rows, _ = ReadSavedModels()
	if len(rows) != 1 || rows[0].ID != "local" {
		t.Errorf("after removing: %+v", rows)
	}
}

func TestSaveCredential(t *testing.T) {
	dir := useHome(t)
	if err := SaveCredential("A_KEY", "one"); err != nil {
		t.Fatal(err)
	}
	if err := SaveCredential("B_KEY", "two"); err != nil {
		t.Fatal(err)
	}
	if SavedKey("A_KEY") != "one" || SavedKey("B_KEY") != "two" {
		t.Errorf("saved keys read back as %q %q", SavedKey("A_KEY"), SavedKey("B_KEY"))
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(filepath.Join(dir, CredentialsName))
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("credentials.json mode %o, want 600", perm)
		}
	}
	if removed, err := DeleteCredential("A_KEY"); err != nil || !removed {
		t.Fatalf("delete: %v %v", removed, err)
	}
	if SavedKey("A_KEY") != "" || SavedKey("B_KEY") != "two" {
		t.Error("delete removed the wrong key")
	}
}

// TestLoadDropsExamplePlaceholder: the example config installs seeded until
// October 2026 names a placeholder model nothing serves. It counts as no
// model, so a saved one comes first and :setup is offered.
func TestLoadDropsExamplePlaceholder(t *testing.T) {
	dir := useHome(t)
	seeded := "llm:\n  base_url: \"http://localhost:8080/v1\"\n  model: \"your-model\"\n  api_key_env: \"OPENAI_API_KEY\"\n"
	cfg, err := loadConfig(t, seeded, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if cfg.LLM.Model != "" {
		t.Errorf("LLM.Model = %q, want the placeholder dropped", cfg.LLM.Model)
	}
	writeFile(t, filepath.Join(dir, SavedModelsName), savedRows)
	cfg, err = loadConfig(t, seeded, LoadOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := modelIDs(cfg), []string{"c", "B"}; !reflect.DeepEqual(got, want) {
		t.Errorf("model ids = %v, want %v", got, want)
	}
	cfg, err = loadConfig(t, "llm:\n  base_url: \"http://localhost:9000/v1\"\n  model: \"your-model\"\n", LoadOptions{})
	if err != nil || cfg.LLM.Model != "your-model" {
		t.Errorf("a your-model at another address was dropped: %v %q", err, cfg.LLM.Model)
	}
}

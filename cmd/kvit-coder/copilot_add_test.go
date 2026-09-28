package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/copilot"
)

func TestIsEmbeddingModel(t *testing.T) {
	embed := []copilot.ListedModel{
		{ID: "text-embedding-3-small", Name: "Embedding V3 small"},
		{ID: "text-embedding-ada-002", Name: "Embedding V2 Ada"},
		{ID: "text-embedding-3-small-inference", Name: "Embedding V3 small (Inference)"},
		{ID: "some-future-embed-model", Name: "Something Else"},
		{ID: "chat-model", Name: "Embedding Helper"},
		{ID: "chat-model", Endpoints: []string{"/embeddings"}},
	}
	for _, m := range embed {
		if !isEmbeddingModel(m) {
			t.Errorf("isEmbeddingModel(%+v) = false, want true", m)
		}
	}
	chat := []copilot.ListedModel{
		{ID: "gpt-5-mini", Name: "GPT-5 mini", Endpoints: []string{"/chat/completions"}},
		{ID: "claude-haiku-4.5", Name: "Claude Haiku 4.5", Endpoints: []string{"/v1/messages"}},
		{ID: "gpt-41-copilot", Name: "GPT-4.1 Copilot", Endpoints: []string{"/responses"}},
		{ID: "kimi-k3", Name: "Kimi K3"},
	}
	for _, m := range chat {
		if isEmbeddingModel(m) {
			t.Errorf("isEmbeddingModel(%+v) = true, want false", m)
		}
	}
}

func TestPlanCopilotAdditions(t *testing.T) {
	models := []copilot.ListedModel{
		{ID: "gpt-5-mini", Name: "GPT-5 mini", Protocol: "chat_completions", Context: 264000, Efforts: []string{"low", "medium", "high"}},
		{ID: "claude-haiku-4.5", Name: "Claude Haiku 4.5", Protocol: "messages", Context: 144000},
		{ID: "text-embedding-3-small", Name: "Embedding V3 small", Protocol: "chat_completions"},
		{ID: "off-model", Name: "Off", Disabled: true},
		{ID: "gpt-4.1", Name: "GPT-4.1", Protocol: "chat_completions", Context: 128000},
	}
	existing := []existingModelRef{
		{ID: "copilot-gpt-4.1", Provider: copilot.Provider, Model: "gpt-4.1"},
		{ID: "local", Provider: "", Model: "qwen"},
	}
	res := planCopilotAdditions(models, existing, "")
	if len(res.Add) != 2 {
		t.Fatalf("add = %d entries %+v, want 2", len(res.Add), res.Add)
	}
	if res.SkippedExisting != 1 || res.SkippedEmbedding != 1 || res.SkippedDisabled != 1 {
		t.Fatalf("plan = %+v, want 1 existing, 1 embedding, 1 disabled skipped", res)
	}

	mini := res.Add[0]
	if mini.ID != "copilot-gpt-5-mini" || mini.Model != "gpt-5-mini" {
		t.Errorf("mini identity = %q/%q", mini.ID, mini.Model)
	}
	if mini.Name != "GPT-5 mini (Copilot)" {
		t.Errorf("mini name = %q, want %q", mini.Name, "GPT-5 mini (Copilot)")
	}
	if mini.Context != 264000 {
		t.Errorf("mini context = %d, want 264000", mini.Context)
	}
	if len(mini.Efforts) != 3 || !mini.Efforts[1].Default || mini.Efforts[0].Default || mini.Efforts[2].Default {
		t.Errorf("mini efforts = %+v, want default on medium", mini.Efforts)
	}

	haiku := res.Add[1]
	if haiku.ID != "copilot-claude-haiku-4.5" {
		t.Errorf("haiku id = %q", haiku.ID)
	}
	if len(haiku.Efforts) != 0 {
		t.Errorf("haiku efforts = %+v, want none", haiku.Efforts)
	}
	if haiku.Context != 144000 {
		t.Errorf("haiku context = %d, want 144000", haiku.Context)
	}
}

// An id collision with a non-copilot row gets a numeric suffix; a name that
// already mentions Copilot is left alone; -host is recorded on new rows.
func TestPlanCopilotAdditionsCollisionAndHost(t *testing.T) {
	models := []copilot.ListedModel{
		{ID: "luna", Name: "GPT-6 Luna", Protocol: "responses", Context: 1000000, Efforts: []string{"low", "high"}},
		{ID: "go-model", Name: "GPT-4.1 Copilot", Protocol: "responses"},
	}
	existing := []existingModelRef{{ID: "copilot-luna", Provider: "", Model: "other"}}
	res := planCopilotAdditions(models, existing, "company.ghe.com")
	if len(res.Add) != 2 {
		t.Fatalf("add = %+v, want 2", res.Add)
	}
	if res.Add[0].ID != "copilot-luna-2" {
		t.Errorf("collided id = %q, want copilot-luna-2", res.Add[0].ID)
	}
	if res.Add[0].Name != "GPT-6 Luna (Copilot)" {
		t.Errorf("name = %q", res.Add[0].Name)
	}
	// No medium offered, so the first level is the default.
	if !res.Add[0].Efforts[0].Default || res.Add[0].Efforts[1].Default {
		t.Errorf("efforts = %+v, want default on first", res.Add[0].Efforts)
	}
	if res.Add[1].Name != "GPT-4.1 Copilot" {
		t.Errorf("name = %q, want it unchanged", res.Add[1].Name)
	}
	for _, e := range res.Add {
		if e.Host != "company.ghe.com" {
			t.Errorf("host = %q, want company.ghe.com", e.Host)
		}
	}
}

func TestAppendModelsToYAML(t *testing.T) {
	raw := []byte(`# a comment that must survive
llm:
  base_url: "http://localhost:8080/v1"
  model: "qwen"
  api_backend: "chat_completions"
models:
  - id: local
    name: "Local"
    model: "qwen"
`)
	add := []copilotConfigEntry{{
		ID: "copilot-gpt-5-mini", Name: "GPT-5 mini (Copilot)",
		Model: "gpt-5-mini", Context: 264000,
		Efforts: []copilotEffort{{Value: "low"}, {Value: "medium", Default: true}},
	}}
	out, err := appendModelsToYAML(raw, add)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "# a comment that must survive") {
		t.Errorf("comment lost:\n%s", text)
	}
	if !strings.Contains(text, "id: copilot-gpt-5-mini") {
		t.Errorf("new entry missing:\n%s", text)
	}
	if strings.Count(text, "- id:") != 2 {
		t.Errorf("want 2 entries:\n%s", text)
	}

	// The rewritten file must load, and planning against it must be a no-op.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatalf("Load(rewritten) = %v\n%s", err, text)
	}
	if len(cfg.Models) != 2 || cfg.Models[1].Model != "gpt-5-mini" {
		t.Fatalf("models = %+v", cfg.Models)
	}
	// api_backend stays empty: the protocol comes from the live model list.
	if cfg.Models[1].APIBackend != "" || cfg.Models[1].Context != 264000 {
		t.Fatalf("entry = %+v", cfg.Models[1])
	}
	if strings.Count(text, "api_backend:") != 1 {
		t.Errorf("generated row should not pin api_backend (want only the llm: one):\n%s", text)
	}
	existing, err := existingModelRefs(out)
	if err != nil {
		t.Fatal(err)
	}
	res := planCopilotAdditions([]copilot.ListedModel{
		{ID: "gpt-5-mini", Name: "GPT-5 mini", Protocol: "chat_completions", Context: 264000},
	}, existing, "")
	if len(res.Add) != 0 || res.SkippedExisting != 1 {
		t.Fatalf("re-plan = %+v, want one skip", res)
	}
}

// A config with no models: section gains one; context 0 and empty efforts
// stay out of the generated row.
func TestAppendModelsToYAMLCreatesSection(t *testing.T) {
	raw := []byte("llm:\n  base_url: \"http://localhost:8080/v1\"\n  model: \"qwen\"\n")
	add := []copilotConfigEntry{{
		ID: "copilot-gpt-41-copilot", Name: "GPT-4.1 Copilot",
		Model: "gpt-41-copilot",
	}}
	out, err := appendModelsToYAML(raw, add)
	if err != nil {
		t.Fatal(err)
	}
	text := string(out)
	if !strings.Contains(text, "models:") || !strings.Contains(text, "id: copilot-gpt-41-copilot") {
		t.Errorf("section not created:\n%s", text)
	}
	if strings.Contains(text, "context:") || strings.Contains(text, "efforts:") {
		t.Errorf("empty fields should be omitted:\n%s", text)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, out, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := config.Load(path); err != nil {
		t.Fatalf("Load(rewritten) = %v\n%s", err, text)
	}
}

func TestExistingModelRefsErrors(t *testing.T) {
	if _, err := existingModelRefs([]byte(":\n\tbad")); err == nil {
		t.Error("bad YAML should fail")
	}
	if _, err := existingModelRefs([]byte("models: {}\n")); err == nil {
		t.Error("non-list models: should fail")
	}
	if _, err := appendModelsToYAML([]byte("- just\n- a\n- list\n"), nil); err == nil {
		t.Error("non-mapping top level should fail")
	}
}

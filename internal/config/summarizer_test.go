package config

import (
	"strings"
	"testing"
)

const summarizerBase = `
llm:
  model: main-wire
  base_url: https://example.com/v1
models:
  - id: main
    name: Main
    model: main-wire
    base_url: https://example.com/v1
`

func TestSummarizerEntryAbsent(t *testing.T) {
	cfg, err := writeConfig(t, summarizerBase)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := cfg.SummarizerEntry(); ok {
		t.Error("SummarizerEntry found one with none flagged")
	}
}

func TestSummarizerEntryFound(t *testing.T) {
	cfg, err := writeConfig(t, summarizerBase+`
  - id: cheap
    name: Cheap
    model: cheap-wire
    base_url: https://cheap.example.com/v1
    summarizer: true
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	entry, ok := cfg.SummarizerEntry()
	if !ok {
		t.Fatal("SummarizerEntry found none with one flagged")
	}
	if entry.ID != "cheap" || entry.Model != "cheap-wire" {
		t.Errorf("SummarizerEntry = %+v, want the cheap entry", entry)
	}
}

func TestSummarizerEntryRejectsTwo(t *testing.T) {
	_, err := writeConfig(t, summarizerBase+`
  - id: cheap
    name: Cheap
    model: cheap-wire
    base_url: https://cheap.example.com/v1
    summarizer: true
  - id: cheaper
    name: Cheaper
    model: cheaper-wire
    base_url: https://cheaper.example.com/v1
    summarizer: true
`)
	if err == nil {
		t.Fatal("want an error with two summarizer entries, got nil")
	}
	if !strings.Contains(err.Error(), "summarizer") {
		t.Errorf("error %q does not name the summarizer flag", err)
	}
}

func TestSummarizerEntryLegacyConfig(t *testing.T) {
	cfg, err := writeConfig(t, `
llm:
  model: solo
  base_url: https://example.com/v1
`)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if _, ok := cfg.SummarizerEntry(); ok {
		t.Error("a legacy single-model config has nowhere to flag, want none")
	}
}

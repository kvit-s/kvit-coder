package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestCompactionSectionDefaultsOn: no compaction: section means compaction on,
// and the file's values are read.
func TestCompactionSectionDefaultsOn(t *testing.T) {
	if !(CompactionConfig{}).AutoEnabled() {
		t.Errorf("an absent compaction section should leave compaction on")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	body := "llm:\n  base_url: http://example.test/v1\n  model: m\n" +
		"compaction:\n  auto: false\n  keep_recent_tokens: 30000\n  turn_headroom_ratio: 0.75\n  model: m2\n"
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	c := cfg.Compaction
	if c.AutoEnabled() || c.KeepRecentTokens != 30000 || c.TurnHeadroomRatio != 0.75 || c.Model != "m2" {
		t.Errorf("compaction section = %+v", c)
	}
}

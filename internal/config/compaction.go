package config

// CompactionConfig is the compaction: section. Every number has a default, so
// an absent section means compaction on with the defaults of
// docs/compaction.md; a zero value in the file also means the default.
type CompactionConfig struct {
	// Auto rewrites the context at the tier 1 point without being asked. Off,
	// the session runs until the window is full as it always did, and
	// :compact still works. Default on.
	Auto *bool `yaml:"auto"`
	// ThinkingCutoffFraction is the share of the window at which a turn stops
	// merging the model's readable thinking into its answers (tier 0), unless
	// tier 1 comes first. Default 0.8.
	ThinkingCutoffFraction float64 `yaml:"thinking_cutoff_fraction"`
	// KeepRecentTokens is how much of the turn in progress, counted back from
	// its end, is kept word for word. Default 20000, at most an eighth of the
	// window.
	KeepRecentTokens int `yaml:"keep_recent_tokens"`
	// CompactedTurnTokens caps the model's compacted version of the turn in
	// progress (S). Default 0: a 32nd of the window, kept within 4k–32k.
	CompactedTurnTokens int `yaml:"compacted_turn_tokens"`
	// StepTokens is the least room kept for one more step (G). Default 16000,
	// at most a 16th of the window.
	StepTokens int `yaml:"step_tokens"`
	// TurnHeadroomRatio (α) is the share of the turn's own length that is
	// kept free: a turn that has added 500k tokens compacts once fewer than
	// 250k are left. Default 0.5.
	TurnHeadroomRatio float64 `yaml:"turn_headroom_ratio"`
	// OlderTurnsFraction is the share of the window above which a rewritten
	// context also has its oldest turns summarized by the model. Default 0.2.
	OlderTurnsFraction float64 `yaml:"older_turns_fraction"`
	// Instructions is appended to every compaction and summary request.
	Instructions string `yaml:"instructions"`
	// Model names the models: entry that writes compacted text, by anything
	// --model accepts. Empty uses the session's own model and client, whose
	// prompt cache the compaction request reuses.
	Model string `yaml:"model"`
}

// AutoEnabled reports whether compaction runs without being asked.
func (c CompactionConfig) AutoEnabled() bool { return c.Auto == nil || *c.Auto }

// Package compaction shrinks the conversation sent to the model as it nears
// the model's context window, as docs/compaction.md describes: older turns
// are reduced by code to what the user said and the answer they ended with,
// the turn in progress is compacted by the model up to a word-for-word tail,
// and the result is recorded in history.jsonl so the next process sends the
// same thing.
package compaction

import "github.com/kvit-s/kvit-coder/internal/config"

// Defaults, from docs/compaction.md.
const (
	defaultKeepRecent       = 20000
	defaultStepTokens       = 16000
	defaultHeadroomRatio    = 0.5
	defaultOlderFraction    = 0.2
	defaultThinkingCutoff   = 0.8
	compactedMin            = 4000
	compactedMax            = 32000
	compactedNoWindow       = 8000
	compactedWindowDivisor  = 32
	keepWindowDivisor       = 8
	stepWindowDivisor       = 16
	headroomWindowCapDivide = 2
)

// Settings are the compaction: section resolved against one model's context
// window: every number the tier points and the rewrite use.
type Settings struct {
	// Window is the model's context window (llm.context), W. Zero means it
	// is not known, and nothing fires automatically.
	Window int
	// Auto is compaction.auto.
	Auto bool
	// ThinkingCutoff is the share of the window at which tier 0 fires.
	ThinkingCutoff float64
	// Keep is how much of the turn in progress is kept word for word.
	Keep int
	// Compacted is S: the budget for the model's compacted text, and for the
	// summary of older turns.
	Compacted int
	// StepMin is the least room kept for one more step, G's minimum.
	StepMin int
	// HeadroomRatio is α.
	HeadroomRatio float64
	// OlderFraction is the share of the window above which a rewrite also
	// summarizes the oldest turns.
	OlderFraction float64
	// Instructions is appended to every compaction and summary request.
	Instructions string
}

// Resolve applies the defaults and the window to the configured section.
func Resolve(c config.CompactionConfig, window int) Settings {
	s := Settings{
		Window:         max(window, 0),
		Auto:           c.AutoEnabled(),
		ThinkingCutoff: orFloat(c.ThinkingCutoffFraction, defaultThinkingCutoff),
		Keep:           orInt(c.KeepRecentTokens, defaultKeepRecent),
		Compacted:      c.CompactedTurnTokens,
		StepMin:        orInt(c.StepTokens, defaultStepTokens),
		HeadroomRatio:  orFloat(c.TurnHeadroomRatio, defaultHeadroomRatio),
		OlderFraction:  orFloat(c.OlderTurnsFraction, defaultOlderFraction),
		Instructions:   c.Instructions,
	}
	if s.Window > 0 {
		// On a small window the fixed numbers are more than the window can
		// hold: a 20k tail and 16k of step room do not fit in 32k.
		s.Keep = min(s.Keep, s.Window/keepWindowDivisor)
		s.StepMin = min(s.StepMin, s.Window/stepWindowDivisor)
	}
	if s.Compacted <= 0 {
		if s.Window > 0 {
			s.Compacted = min(max(s.Window/compactedWindowDivisor, compactedMin), compactedMax)
		} else {
			s.Compacted = compactedNoWindow
		}
	}
	return s
}

// Margin is the free space a turn keeps before tier 1 fires:
//
//	max(S + G, min(α × T, (W − F) / 2))
//
// turnTokens is T, how much the turn in progress has added since it began;
// step is the largest growth from one request to the next seen in the turn,
// which raises G above its minimum; floor is F, the size of the context right
// after the last rewrite in this turn, 0 before the first. S + G is what the
// compaction request itself needs. α × T is room for the turn to keep going,
// capped at half the space a rewrite leaves free: a rewrite cannot make the
// context smaller than F, so asking for more room than (W − F) / 2 would
// compact again a step or two after every rewrite.
func (s Settings) Margin(turnTokens, step, floor int) int {
	g := max(s.StepMin, step)
	headroom := min(int(s.HeadroomRatio*float64(turnTokens)), max(s.Window-floor, 0)/headroomWindowCapDivide)
	return max(s.Compacted+g, headroom)
}

// Tier1Due reports whether a request of prompt tokens would leave less free
// space than Margin.
func (s Settings) Tier1Due(prompt, turnTokens, step, floor int) bool {
	return s.Window > 0 && s.Window-prompt < s.Margin(turnTokens, step, floor)
}

// Tier0Due reports whether a request of prompt tokens is past the point where
// the turn stops merging readable thinking: the thinking cutoff, or the tier
// 1 point when that comes first.
func (s Settings) Tier0Due(prompt, turnTokens, step, floor int) bool {
	if s.Window <= 0 {
		return false
	}
	point := min(int(s.ThinkingCutoff*float64(s.Window)), s.Window-s.Margin(turnTokens, step, floor))
	return prompt >= point
}

// Tier1Point is the request size at which tier 1 fires.
func (s Settings) Tier1Point(turnTokens, step, floor int) int {
	return s.Window - s.Margin(turnTokens, step, floor)
}

func orInt(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

func orFloat(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

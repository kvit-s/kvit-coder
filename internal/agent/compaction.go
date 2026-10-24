package agent

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/kvit-s/kvit-coder/internal/compaction"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// Compactor rewrites the context a turn sends when it nears the model's
// window. *compaction.Compactor is the one a session turn uses; a test can
// supply its own.
type Compactor interface {
	Settings() compaction.Settings
	Compact(context.Context, compaction.Request) (*compaction.Result, error)
}

// CompactionStart is where the messages a run starts with stand for
// compaction, as the caller loaded them from the session.
type CompactionStart struct {
	// TailIndex is the index in Messages of the first message that is the
	// conversation's own, word for word; TailStart is its position in the
	// session's conversation.
	TailIndex int
	TailStart int
	// PromptTokens estimates the first request.
	PromptTokens int
	// TurnTokens is how much the turn in progress has added before this run:
	// the prompt, and any earlier turns that ended without an answer.
	TurnTokens int
	// PromptPrefix is what the turn's prompt has in front of it on the wire
	// but not in the history (the project instructions).
	PromptPrefix string
}

// compactionState follows a run's requests for compaction: where the
// word-for-word tail starts, how large the last request was, and how much the
// turn has added.
type compactionState struct {
	tailIndex int
	tailStart int
	// lastPrompt is the size of the last request: the server's count, or an
	// estimate at the start of the run and after a rewrite. lastPromptLen is
	// how many messages it covered. measured says lastPrompt is the server's,
	// so the next count can be compared with it.
	lastPrompt    int
	lastPromptLen int
	measured      bool
	// turnTokens is T and maxStep the largest growth seen between two
	// consecutive requests, which raises G above its minimum. floor is the
	// size right after the last rewrite in this run, which the next rewrite
	// cannot get below.
	turnTokens int
	maxStep    int
	floor      int
	// overflowTried stops a request the endpoint keeps refusing as too long
	// from being compacted again until one gets through.
	overflowTried bool
	// quietBelow holds off another attempt after one that could do nothing,
	// until the request has grown by another step.
	quietBelow int
	// thinkingStopped is set once tier 0 has fired in this run.
	thinkingStopped bool
	promptPrefix    string
}

func newCompactionState(start *CompactionStart, messages int) *compactionState {
	if start == nil {
		return nil
	}
	return &compactionState{
		tailIndex:     start.TailIndex,
		tailStart:     start.TailStart,
		lastPrompt:    start.PromptTokens,
		lastPromptLen: messages,
		turnTokens:    start.TurnTokens,
		promptPrefix:  start.PromptPrefix,
	}
}

// SetCompactor installs what rewrites the context. A run compacts only when it
// also has a CompactionStart.
func (r *Runner) SetCompactor(c Compactor) { r.compactor = c }

// Compactor is what SetCompactor installed, or nil.
func (r *Runner) Compactor() Compactor { return r.compactor }

// SetNotice installs a function that records something that happened to the
// turn, in the session's history.
func (r *Runner) SetNotice(fn func(string) error) { r.notice = fn }

// compactionActive reports whether this run compacts automatically.
func (r *Runner) compactionActive(state *runState) bool {
	return r.compactor != nil && state.comp != nil && r.compactor.Settings().Auto
}

// nextPromptSize estimates the request about to be sent: the last request's
// size and what has been added since.
func (r *Runner) nextPromptSize(state *runState) int {
	cs := state.comp
	start := min(cs.lastPromptLen, len(state.messages))
	return cs.lastPrompt + compaction.EstimateTokens(state.messages[start:])
}

// checkCompaction runs before each request: tier 1 rewrites the context when
// the request would leave less of the window free than the turn needs, and
// tier 0 stops merging the model's readable thinking.
func (r *Runner) checkCompaction(ctx context.Context, state *runState) {
	if !r.compactionActive(state) {
		return
	}
	cs := state.comp
	limits := r.compactor.Settings()
	prompt := r.nextPromptSize(state)
	// Tier 0 is judged before any rewrite: its point is the thinking cutoff
	// or the tier 1 point, whichever comes first, so reaching tier 1 stops
	// the merging too, for the rest of the turn.
	if !cs.thinkingStopped && limits.Tier0Due(prompt, cs.turnTokens, cs.maxStep, cs.floor) {
		r.stopMergingThinking(state, prompt)
	}
	if prompt >= cs.quietBelow && limits.Tier1Due(prompt, cs.turnTokens, cs.maxStep, cs.floor) {
		if !r.compactNow(ctx, state, prompt, false) {
			cs.quietBelow = prompt + max(limits.StepMin, cs.maxStep)
		}
	}
}

// compactAfterOverflow compacts after the endpoint refused a request as longer
// than the window, and reports whether the request should be sent again.
func (r *Runner) compactAfterOverflow(ctx context.Context, err error, state *runState) bool {
	if !r.compactionActive(state) || state.comp.overflowTried || !llm.IsContextOverflow(err) {
		return false
	}
	state.comp.overflowTried = true
	r.writer.Warn("the request was too long for the model's context window, compacting")
	return r.compactNow(ctx, state, r.nextPromptSize(state), true)
}

// compactNow rewrites the context. Everything is written to the session
// first, so the history the compactor reads matches the messages it is handed;
// afterwards the rewritten context replaces the run's messages, all of which
// are already on disk.
func (r *Runner) compactNow(ctx context.Context, state *runState, prompt int, overflow bool) bool {
	cs := state.comp
	r.flush(state)
	if r.persist != nil && state.persistedUpTo != len(state.messages) {
		r.writer.Warn("cannot compact: the session has not recorded every message")
		return false
	}
	// What the last step added has not been measured yet. It is counted once
	// the rewrite succeeds, as the first request after it cannot be compared
	// with the last; if the rewrite fails, the next count measures it.
	unmeasured := 0
	if cs.measured {
		unmeasured = compaction.EstimateTokens(state.messages[min(cs.lastPromptLen, len(state.messages)):])
	}

	res, err := r.compactor.Compact(ctx, compaction.Request{
		Messages:     state.messages,
		TailIndex:    cs.tailIndex,
		TailStart:    cs.tailStart,
		TokensBefore: prompt,
		Overflow:     overflow,
		PromptPrefix: cs.promptPrefix,
	})
	if err != nil {
		if !errors.Is(err, compaction.ErrNothingToCompact) && ctx.Err() == nil {
			r.writer.Warn(fmt.Sprintf("compaction failed: %v", err))
		}
		return false
	}

	if unmeasured > 0 {
		cs.turnTokens += unmeasured
		cs.maxStep = max(cs.maxStep, unmeasured)
	}
	state.messages = res.Messages
	state.persistedUpTo = len(state.messages)
	cs.tailIndex, cs.tailStart = res.TailIndex, res.TailStart
	cs.lastPrompt, cs.lastPromptLen, cs.measured = res.TokensAfter, len(state.messages), false
	cs.floor = res.TokensAfter
	cs.quietBelow = 0
	// A backtrack must not reach behind the rewrite; its checkpoints were
	// positions in the messages that were just replaced.
	state.backtracker = NewBacktrackTracker(BacktrackConfig{
		Enabled:           r.cfg.Backtrack.Enabled,
		MaxRetries:        r.cfg.Backtrack.MaxRetries,
		InjectUserMessage: r.cfg.Backtrack.InjectUserMessage,
	})

	r.writer.Info(fmt.Sprintf("context compacted: %s → %s tokens", ui.FormatTokens(res.TokensBefore), ui.FormatTokens(res.TokensAfter)))
	limits := r.compactor.Settings()
	if limits.Window > 0 && limits.Window-res.TokensAfter < limits.Compacted+max(limits.StepMin, cs.maxStep) {
		r.writer.Warn("the context is still nearly full after compacting; start a new session")
	}
	return true
}

// stopMergingThinking is tier 0: from here on the turn drops the model's
// readable thinking instead of merging it into its answers, and stops asking
// for it.
func (r *Runner) stopMergingThinking(state *runState, prompt int) {
	state.comp.thinkingStopped = true
	if !state.normalizer.MergeThinking() {
		return
	}
	state.normalizer.SetMergeThinking(false)
	if c, ok := r.llmClient.(interface{ SetReasoningSummary(string) }); ok {
		c.SetReasoningSummary("")
	}
	text := fmt.Sprintf("context at %s tokens: no longer merging the model's thinking into its answers", ui.FormatTokens(prompt))
	r.writer.Info(text)
	if r.notice != nil {
		if err := r.notice(text); err != nil {
			r.writer.Debug(fmt.Sprintf("cannot record notice: %v", err))
		}
	}
}

// recordPromptSize follows the server's count of each request: how much the
// turn grew since the last one, and the largest single step.
func (r *Runner) recordPromptSize(promptTokens int, state *runState) {
	cs := state.comp
	if cs == nil {
		return
	}
	cs.overflowTried = false
	if promptTokens <= 0 {
		return
	}
	if cs.measured {
		if g := promptTokens - cs.lastPrompt; g > 0 {
			cs.turnTokens += g
			cs.maxStep = max(cs.maxStep, g)
		}
	}
	cs.lastPrompt, cs.lastPromptLen, cs.measured = promptTokens, len(state.messages), true
}

// logReasoningBlocks writes to the agent log how much encrypted thinking a
// response brought back. The blocks are sent with every later request of the
// turn, so this is what tells whether they, rather than tool output, are what
// fills the window (docs/compaction.md, "Encrypted thinking").
func (r *Runner) logReasoningBlocks(msg *llm.Message, promptTokens int) {
	if len(msg.ReasoningBlocks) == 0 {
		return
	}
	size := 0
	for _, b := range msg.ReasoningBlocks {
		size += len(b.EncryptedContent)
	}
	r.logger.Info("encrypted thinking",
		zap.Int("blocks", len(msg.ReasoningBlocks)),
		zap.Int("encrypted_bytes", size),
		zap.Int("prompt_tokens", promptTokens))
}

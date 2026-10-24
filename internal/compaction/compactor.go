package compaction

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
)

// ErrNothingToCompact is returned when a rewrite would change nothing: the
// context is already as short as compaction can make it.
var ErrNothingToCompact = errors.New("nothing to compact")

// Chatter is the part of the model client compaction uses.
type Chatter interface {
	Chat(context.Context, llm.ChatRequest) (*llm.ChatResponse, error)
}

// Compactor rewrites one session's context. It is built once per turn, with
// the client and tool specs the turn's own requests use, so that a compaction
// request starts exactly as an earlier request of the turn did and the server
// has it cached.
type Compactor struct {
	Sess *session.Session
	// Client and Model send the compaction requests: the session's own unless
	// compaction.model names another entry.
	Client Chatter
	Model  string
	// Tools returns the tool specs the turn's requests carry. They are sent
	// unchanged, because a request without them would start differently from
	// the ones the server cached.
	Tools       func() []llm.ToolSpec
	Temperature float32
	Limits      Settings
	// Warn, when set, is told about a compaction that fell short: a model
	// request that failed, so a fallback was used.
	Warn func(string)
}

// Settings are the resolved limits the compactor works to.
func (c *Compactor) Settings() Settings { return c.Limits }

// SpecTokens estimates the tool specs every request carries, which a size
// estimated from messages alone leaves out.
func (c *Compactor) SpecTokens() int {
	if c.Tools == nil {
		return 0
	}
	return EstimateSpecs(c.Tools())
}

// Request is a context to compact, as the next request would send it.
type Request struct {
	// Messages is the context with the system prompt first. Messages from
	// TailIndex on are the conversation's own, starting at position
	// TailStart; before that, whatever an earlier compaction built.
	Messages  []llm.Message
	TailIndex int
	TailStart int
	// TokensBefore is the size of the request about to be sent, recorded.
	TokensBefore int
	// Overflow says the endpoint refused the request as too long, so the
	// compaction request is cut down before it is sent.
	Overflow bool
	// Force summarizes the oldest turns whatever the size of the result, as
	// :compact does.
	Force bool
	// Focus is what :compact was asked to concentrate on.
	Focus string
	// PromptPrefix is what the turn put in front of its prompt on the wire
	// and not in the history: the project instructions. When the rewrite
	// folds the current turn's prompt into the compacted part, it keeps the
	// prefix in front of it, as the turn's requests had it.
	PromptPrefix string
}

// Result is the rewritten context, to send from now on.
type Result struct {
	Messages     []llm.Message
	TailIndex    int
	TailStart    int
	TokensBefore int
	TokensAfter  int
	// Summarized says the model wrote the compacted text of the turn in
	// progress; false with Fallback means it failed and the turn's tool
	// outputs were removed instead.
	Summarized bool
	Fallback   bool
	// OlderSummarized says the oldest turns were summarized by the model.
	OlderSummarized bool
}

// Compact rewrites the context (docs/compaction.md, tier 1): every turn before
// the turn in progress becomes a turn summary, the turn in progress up to its
// last Keep tokens becomes the model's compacted text, and, when the result is
// still above OlderFraction of the window, the oldest turns become one
// summary. The record goes to history.jsonl before Compact returns, and the
// returned messages are what the next process would build from it, with the
// tail's in-memory messages kept so they keep their encrypted thinking.
func (c *Compactor) Compact(ctx context.Context, req Request) (*Result, error) {
	if len(req.Messages) == 0 || req.TailIndex < 1 || req.TailIndex > len(req.Messages) {
		return nil, fmt.Errorf("compaction: the context has no system prompt in front of it")
	}
	tl, err := c.Sess.Timeline()
	if err != nil {
		return nil, err
	}
	n := len(tl.Messages)
	if req.TailStart+(len(req.Messages)-req.TailIndex) != n {
		return nil, fmt.Errorf("compaction: the context has %d conversation messages from position %d, the history has %d",
			len(req.Messages)-req.TailIndex, req.TailStart, n)
	}

	from := tl.InProgressFrom()
	floor := max(from, req.TailStart)
	floorIdx := req.TailIndex + (floor - req.TailStart)
	cutIdx := chooseCut(req.Messages, floorIdx, c.Limits.Keep)
	cut := req.TailStart + (cutIdx - req.TailIndex)
	if cut <= req.TailStart && !req.Force {
		return nil, ErrNothingToCompact
	}

	rec := session.Compaction{From: from, Cut: cut, TokensBefore: req.TokensBefore, Model: c.Model}
	res := &Result{TailStart: cut, TokensBefore: req.TokensBefore}

	// The turn in progress, up to the cut. When the cut is where the turn
	// began, it has nothing yet to compact; when it is where the last
	// compaction of this same turn put it, that compaction's text still
	// covers everything before it.
	prev := tl.LatestCompaction()
	if cut > from && prev != nil && prev.From == from && prev.Cut == cut {
		rec.Compacted = prev.Compacted
	} else if cut > from {
		spanIdx := max(req.TailIndex, req.TailIndex+(from-req.TailStart))
		text, err := c.compactTurn(ctx, req, spanIdx, cutIdx)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			c.warn(fmt.Sprintf("compacting the turn failed (%v); its tool outputs were removed instead", err))
			res.Fallback = true
		} else {
			rec.Compacted = text
			res.Summarized = true
		}
	}

	view := session.BuildContext(tl.WithCompaction(rec))
	messages, tailIndex := assemble(req, view, cutIdx)
	keepPromptPrefix(messages, tailIndex, tl, from, cut, rec.Compacted != "", req.PromptPrefix)
	specs := c.SpecTokens()
	after := EstimateTokens(messages) + specs

	if req.Force || (c.Limits.Window > 0 && float64(after) > c.Limits.OlderFraction*float64(c.Limits.Window)) {
		summary, upTo, err := c.summarizeOlder(ctx, tl, req, from)
		switch {
		case err == nil && summary != "":
			rec.OlderSummary, rec.OlderUpTo = summary, upTo
			res.OlderSummarized = true
			view = session.BuildContext(tl.WithCompaction(rec))
			messages, tailIndex = assemble(req, view, cutIdx)
			keepPromptPrefix(messages, tailIndex, tl, from, cut, rec.Compacted != "", req.PromptPrefix)
			after = EstimateTokens(messages) + specs
		case err != nil:
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			c.warn(fmt.Sprintf("summarizing older turns failed (%v); their turn summaries stay", err))
		}
	}
	if cut <= req.TailStart && !res.OlderSummarized {
		return nil, ErrNothingToCompact
	}

	rec.TokensAfter = after
	if err := c.Sess.AppendCompaction(rec); err != nil {
		return nil, err
	}
	res.Messages, res.TailIndex, res.TokensAfter = messages, tailIndex, after
	return res, nil
}

// assemble is the system prompt, the part of view that compaction built, and
// the request's own messages from the cut on — the same messages the view has
// there, but the in-memory ones, which still have their encrypted thinking.
func assemble(req Request, view *session.Context, cutIdx int) ([]llm.Message, int) {
	out := make([]llm.Message, 0, 1+view.TailIndex+len(req.Messages)-cutIdx)
	out = append(out, req.Messages[0])
	out = append(out, view.Messages[:view.TailIndex]...)
	tailIndex := len(out)
	out = append(out, req.Messages[cutIdx:]...)
	return out, tailIndex
}

// keepPromptPrefix puts prefix back in front of the current turn's prompt when
// the rewrite moved that prompt out of the tail: into the compacted message,
// which starts with the turn's words, or, when the model could not compact,
// into the turn's own messages copied from the history without it.
func keepPromptPrefix(messages []llm.Message, tailIndex int, tl *session.Timeline, from, cut int, compacted bool, prefix string) {
	if prefix == "" {
		return
	}
	spans := tl.TurnSpans()
	if len(spans) == 0 {
		return
	}
	current := spans[len(spans)-1]
	if current.Start < from || current.Start >= cut || current.Start >= len(tl.Messages) ||
		tl.Messages[current.Start].Role != llm.RoleUser {
		return
	}
	if compacted {
		i := tailIndex - 1
		if i >= 1 && messages[i].Role == llm.RoleUser {
			messages[i].Content = prefix + messages[i].Content
		}
		return
	}
	prompt := tl.Messages[current.Start].Content
	for i := 1; i < tailIndex; i++ {
		if messages[i].Role == llm.RoleUser && messages[i].Content == prompt {
			messages[i].Content = prefix + prompt
			return
		}
	}
}

// chooseCut is the index where the word-for-word tail begins: walk back from
// the end until keep tokens are reached, then back to the start of the step
// that point is in, so a tool call is never separated from its result. It
// never goes below floor, and is floor when everything above floor is shorter
// than keep.
func chooseCut(msgs []llm.Message, floor, keep int) int {
	i := len(msgs)
	acc := 0
	for i > floor {
		i--
		acc += EstimateTokens(msgs[i : i+1])
		if acc >= keep {
			break
		}
	}
	if acc < keep {
		return floor
	}
	for j := i; j > floor; j-- {
		if msgs[j].Role == llm.RoleAssistant {
			return j
		}
	}
	return floor
}

// compactTurn asks the model for the compacted text of the turn in progress:
// the request's messages before the cut, which is an earlier request of the
// turn and so already cached, followed by the instruction. It is tried twice.
// When the request would not fit — or the endpoint has just said the turn's
// request did not — the oldest tool outputs of the turn are removed from it
// first; after a second refusal for length, all of them are.
func (c *Compactor) compactTurn(ctx context.Context, req Request, spanIdx, cutIdx int) (string, error) {
	msgs := append([]llm.Message(nil), req.Messages[:cutIdx]...)
	if limit := c.Limits.Window - c.Limits.Compacted - c.SpecTokens(); c.Limits.Window > 0 && (req.Overflow || EstimateTokens(msgs) > limit) {
		if req.Overflow {
			// The server's count was over the window while the estimate may
			// not be, so aim well inside it.
			limit = limit * 3 / 4
		}
		msgs = removeOutputs(msgs, spanIdx, cutIdx, limit)
	} else if req.Overflow {
		msgs = removeOutputs(msgs, spanIdx, cutIdx, 0)
	}
	instruction := withFocus(turnInstruction, c.Limits.Instructions, req.Focus)

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		text, err := c.ask(ctx, append(msgs, llm.Message{Role: llm.RoleUser, Content: instruction}))
		if err == nil {
			return text, nil
		}
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		lastErr = err
		if llm.IsContextOverflow(err) {
			msgs = removeOutputs(msgs, spanIdx, cutIdx, 0)
		}
	}
	return "", lastErr
}

// removeOutputs replaces tool outputs between from and to with the removal
// note, oldest first, until msgs is estimated at limit tokens or fewer; with
// limit 0, all of them. msgs is a copy the caller owns.
func removeOutputs(msgs []llm.Message, from, to, limit int) []llm.Message {
	total := EstimateTokens(msgs)
	for i := from; i < to && i < len(msgs); i++ {
		if limit > 0 && total <= limit {
			break
		}
		if msgs[i].Role != llm.RoleTool || msgs[i].Content == session.RemovedOutputNote {
			continue
		}
		before := EstimateTokens(msgs[i : i+1])
		msgs[i].Content = session.RemovedOutputNote
		total -= before - EstimateTokens(msgs[i:i+1])
	}
	return msgs
}

// summarizeOlder asks the model for one summary of the turns before the last
// finished one (turns 1 … N-2), from their turn summaries rebuilt from the
// history, so it never summarizes an earlier summary. Only when those would
// take more than half the window does it start from the previous summary and
// the turn summaries after it. It returns the summary and the position of the
// first message it does not cover, or "" when there is nothing new to cover.
func (c *Compactor) summarizeOlder(ctx context.Context, tl *session.Timeline, req Request, from int) (string, int, error) {
	var finished []session.TurnSpan
	for _, sp := range tl.TurnSpans() {
		if sp.Start < from && sp.End > sp.Start {
			sp.End = min(sp.End, from)
			finished = append(finished, sp)
		}
	}
	if len(finished) < 2 {
		return "", 0, nil
	}
	older := finished[:len(finished)-1]
	upTo := finished[len(finished)-1].Start

	var prev *session.Compaction
	for i := len(tl.Compactions) - 1; i >= 0; i-- {
		if tl.Compactions[i].OlderSummary != "" {
			prev = &tl.Compactions[i]
			break
		}
	}
	if prev != nil && prev.OlderUpTo >= upTo {
		return "", 0, nil
	}

	var turns []llm.Message
	for _, sp := range older {
		turns = append(turns, tl.TurnSummary(sp)...)
	}
	if prev != nil && c.Limits.Window > 0 && EstimateTokens(turns) > c.Limits.Window/2 {
		turns = []llm.Message{
			{Role: llm.RoleUser, Content: session.OlderTurnsQuestion},
			{Role: llm.RoleAssistant, Content: prev.OlderSummary},
		}
		for _, sp := range older {
			if sp.Start >= prev.OlderUpTo {
				turns = append(turns, tl.TurnSummary(sp)...)
			}
		}
	}

	msgs := append([]llm.Message{req.Messages[0]}, turns...)
	msgs = append(msgs, llm.Message{Role: llm.RoleUser, Content: withFocus(olderInstruction, c.Limits.Instructions, req.Focus)})
	text, err := c.ask(ctx, msgs)
	if err != nil {
		return "", 0, err
	}
	return text, upTo, nil
}

// ask sends one compaction request and returns its answer, or why it is not
// usable: an error, a tool call, an answer cut off at the token limit, or an
// empty one.
func (c *Compactor) ask(ctx context.Context, msgs []llm.Message) (string, error) {
	var specs []llm.ToolSpec
	if c.Tools != nil {
		specs = c.Tools()
	}
	toolChoice := ""
	if len(specs) > 0 {
		toolChoice = "auto"
	}
	resp, err := c.Client.Chat(ctx, llm.ChatRequest{
		Model:       c.Model,
		Messages:    msgs,
		Tools:       specs,
		ToolChoice:  toolChoice,
		Temperature: c.Temperature,
		MaxTokens:   c.Limits.Compacted,
	})
	if err != nil {
		return "", err
	}
	if resp == nil || len(resp.Choices) == 0 {
		return "", errors.New("no answer")
	}
	choice := resp.Choices[0]
	if choice.Error != nil {
		return "", fmt.Errorf("provider error %d: %s", choice.Error.Code, choice.Error.Message)
	}
	if len(choice.Message.ToolCalls) > 0 {
		return "", errors.New("the model called a tool instead of answering")
	}
	if choice.FinishReason == "length" {
		return "", errors.New("the answer was cut off at the token limit")
	}
	text := strings.TrimSpace(choice.Message.Content)
	if text == "" {
		return "", errors.New("the answer was empty")
	}
	return text, nil
}

func (c *Compactor) warn(msg string) {
	if c.Warn != nil {
		c.Warn(msg)
	}
}

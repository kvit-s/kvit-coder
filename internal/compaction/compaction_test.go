package compaction

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
)

// --- the formula -------------------------------------------------------------

func TestResolveScalesWithTheWindow(t *testing.T) {
	for _, tc := range []struct {
		window                   int
		keep, compacted, stepMin int
	}{
		{1_000_000, 20000, 31250, 16000},
		{200_000, 20000, 6250, 12500},
		{32_000, 4000, 4000, 2000},
		{0, 20000, 8000, 16000},
	} {
		s := Resolve(config.CompactionConfig{}, tc.window)
		if s.Keep != tc.keep || s.Compacted != tc.compacted || s.StepMin != tc.stepMin {
			t.Errorf("window %d: keep %d compacted %d step %d, want %d %d %d",
				tc.window, s.Keep, s.Compacted, s.StepMin, tc.keep, tc.compacted, tc.stepMin)
		}
		if !s.Auto || s.HeadroomRatio != 0.5 || s.OlderFraction != 0.2 || s.ThinkingCutoff != 0.8 {
			t.Errorf("window %d: defaults not applied: %+v", tc.window, s)
		}
	}
	off := false
	if s := Resolve(config.CompactionConfig{Auto: &off, CompactedTurnTokens: 9000}, 1_000_000); s.Auto || s.Compacted != 9000 {
		t.Errorf("configured values not kept: %+v", s)
	}
}

// TestTierPointsMatchTheDesign checks the worked examples of
// docs/compaction.md, "When each tier fires".
func TestTierPointsMatchTheDesign(t *testing.T) {
	s := Resolve(config.CompactionConfig{}, 1_000_000)

	// A turn that has added 30k keeps 47k free: tier 1 at about 953k.
	if p := s.Tier1Point(30_000, 0, 0); p != 952_750 {
		t.Errorf("short turn: tier 1 point %d, want 952750", p)
	}
	if !s.Tier0Due(800_000, 30_000, 0, 0) || s.Tier0Due(799_999, 30_000, 0, 0) {
		t.Errorf("short turn: tier 0 should fire at 800k")
	}

	// A turn of 500k with 480k left needs 250k: nothing happens.
	if s.Tier1Due(520_000, 500_000, 0, 0) {
		t.Errorf("a 500k turn with 480k left should not compact")
	}

	// One long turn (20k of system prompt) compacts once it passes ~653k.
	if s.Tier1Due(20_000+650_000, 650_000, 0, 0) {
		t.Errorf("a 650k turn should not compact yet")
	}
	if !s.Tier1Due(20_000+660_000, 660_000, 0, 0) {
		t.Errorf("a 660k turn should compact")
	}
	if !s.Tier0Due(20_000+660_000, 660_000, 0, 0) {
		t.Errorf("tier 0 should fire with tier 1 for the long turn")
	}

	// 600k of earlier turns and 100k in this one: margin 50k, tier 1 at 950k.
	if p := s.Tier1Point(100_000, 0, 0); p != 950_000 {
		t.Errorf("tier 1 point %d, want 950000", p)
	}

	// The cap at W/2: a very long turn still leaves room after a rewrite.
	if m := s.Margin(5_000_000, 0, 0); m != 500_000 {
		t.Errorf("margin for a 5M turn = %d, want the W/2 cap", m)
	}
	// After a rewrite to F the cap is half of what is left above F, so the
	// next rewrite waits until the context has grown by that much.
	if m := s.Margin(5_000_000, 0, 200_000); m != 400_000 {
		t.Errorf("margin after a rewrite to 200k = %d, want (W-F)/2", m)
	}
	small := Resolve(config.CompactionConfig{}, 40_000)
	if p := small.Tier1Point(60_000, 0, 17_000); p != 40_000-11_500 {
		t.Errorf("40k window after a rewrite to 17k: tier 1 at %d, want 28500", p)
	}

	// A large step raises G.
	if m := s.Margin(0, 40_000, 0); m != 31_250+40_000 {
		t.Errorf("margin after a 40k step = %d", m)
	}

	// 200k window, short turn: tier 1 at about 181k.
	if p := Resolve(config.CompactionConfig{}, 200_000).Tier1Point(10_000, 0, 0); p != 181_250 {
		t.Errorf("200k window: tier 1 point %d, want 181250", p)
	}

	// No window known: nothing fires on its own.
	none := Resolve(config.CompactionConfig{}, 0)
	if none.Tier1Due(1<<30, 1<<30, 0, 0) || none.Tier0Due(1<<30, 1<<30, 0, 0) {
		t.Errorf("with no window nothing should fire")
	}
}

// --- the cut -----------------------------------------------------------------

func TestChooseCutStartsAStepAndKeepsAtLeastKeep(t *testing.T) {
	msgs := []llm.Message{{Role: llm.RoleSystem, Content: "sys"}, {Role: llm.RoleUser, Content: "go"}}
	msgs = append(msgs, steps("s", 5, 3000)...)
	cut := chooseCut(msgs, 1, 2500)
	if msgs[cut].Role != llm.RoleAssistant {
		t.Fatalf("cut at a %s message, want the start of a step", msgs[cut].Role)
	}
	if got := EstimateTokens(msgs[cut:]); got < 2500 {
		t.Errorf("tail is %d tokens, want at least 2500", got)
	}
	if got := EstimateTokens(msgs[cut+2:]); got >= 2500 {
		t.Errorf("the cut is a step earlier than it needs to be")
	}
	if c := chooseCut(msgs, 1, 1_000_000); c != 1 {
		t.Errorf("a conversation shorter than keep should cut at the floor, got %d", c)
	}
}

// --- a session to compact ----------------------------------------------------

type reply struct {
	text     string
	err      error
	toolCall bool
	finish   string
}

// fakeChat answers compaction requests from a script and records them.
type fakeChat struct {
	replies  []reply
	requests []llm.ChatRequest
}

func (f *fakeChat) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	req.Messages = append([]llm.Message(nil), req.Messages...)
	f.requests = append(f.requests, req)
	if len(f.replies) == 0 {
		return nil, errors.New("fake chat: no reply scripted")
	}
	r := f.replies[0]
	f.replies = f.replies[1:]
	if r.err != nil {
		return nil, r.err
	}
	msg := llm.Message{Role: llm.RoleAssistant, Content: r.text}
	if r.toolCall {
		msg.ToolCalls = []llm.ToolCall{{ID: "x", Function: llm.ToolCallFunction{Name: "Read", Arguments: "{}"}}}
	}
	finish := r.finish
	if finish == "" {
		finish = "stop"
	}
	return &llm.ChatResponse{Choices: []llm.Choice{{Message: msg, FinishReason: finish}}}, nil
}

var testTools = []llm.ToolSpec{{Type: "function", Function: llm.ToolSpecFunction{Name: "Read"}}}

type fixture struct {
	t     *testing.T
	sess  *session.Session
	chat  *fakeChat
	comp  *Compactor
	warns []string
}

func newFixture(t *testing.T, window int, cfg config.CompactionConfig, replies ...reply) *fixture {
	t.Helper()
	sess, err := session.OpenDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	f := &fixture{t: t, sess: sess, chat: &fakeChat{replies: replies}}
	f.comp = &Compactor{
		Sess:   sess,
		Client: f.chat,
		Model:  "test-model",
		Tools:  func() []llm.ToolSpec { return testTools },
		Limits: Resolve(cfg, window),
		Warn:   func(s string) { f.warns = append(f.warns, s) },
	}
	return f
}

// turn writes a turn's settings line and messages and returns where it began.
func (f *fixture) turn(msgs ...llm.Message) int {
	f.t.Helper()
	tl, err := f.sess.Timeline()
	if err != nil {
		f.t.Fatal(err)
	}
	at := len(tl.Messages)
	if err := f.sess.Settings("test-model", false, nil); err != nil {
		f.t.Fatal(err)
	}
	f.add(msgs...)
	return at
}

func (f *fixture) add(msgs ...llm.Message) {
	f.t.Helper()
	if err := f.sess.AppendMessages(msgs); err != nil {
		f.t.Fatal(err)
	}
}

// request is the context as a turn would send it now: the system prompt and
// what LoadContext builds.
func (f *fixture) request() Request {
	f.t.Helper()
	ctx, _, err := f.sess.LoadContext()
	if err != nil {
		f.t.Fatal(err)
	}
	msgs := append([]llm.Message{{Role: llm.RoleSystem, Content: "system prompt"}}, ctx.Messages...)
	return Request{Messages: msgs, TailIndex: 1 + ctx.TailIndex, TailStart: ctx.TailStart, TokensBefore: EstimateTokens(msgs)}
}

func (f *fixture) timeline() *session.Timeline {
	f.t.Helper()
	tl, err := f.sess.Timeline()
	if err != nil {
		f.t.Fatal(err)
	}
	return tl
}

func steps(prefix string, n, size int) []llm.Message {
	var out []llm.Message
	for i := 0; i < n; i++ {
		id := prefix + string(rune('a'+i))
		out = append(out,
			llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{ID: id, Type: "function",
				Function: llm.ToolCallFunction{Name: "Read", Arguments: `{"path":"` + id + `.go"}`}}}},
			llm.Message{Role: llm.RoleTool, Name: "Read", ToolCallID: id, Content: strings.Repeat("x", size)})
	}
	return out
}

func user(s string) llm.Message { return llm.Message{Role: llm.RoleUser, Content: s} }
func say(s string) llm.Message  { return llm.Message{Role: llm.RoleAssistant, Content: s} }

var smallTail = config.CompactionConfig{KeepRecentTokens: 2500}

// --- compacting the turn in progress -----------------------------------------

// TestCompactTheTurnInProgress: the request is the context up to the cut plus
// the instruction; the finished turn becomes its summary; the turn in progress
// becomes its words and the compacted text; and the record on disk rebuilds
// the same context.
func TestCompactTheTurnInProgress(t *testing.T) {
	f := newFixture(t, 100_000, smallTail, reply{text: "read sa.go and sb.go"})
	f.turn(user("one"), say("done one"))
	from := f.turn(append([]llm.Message{user("two")}, steps("s", 6, 3000)...)...)
	req := f.request()

	res, err := f.comp.Compact(context.Background(), req)
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if !res.Summarized || res.Fallback {
		t.Errorf("result = %+v, want the model's compacted text", res)
	}
	if len(f.chat.requests) != 1 {
		t.Fatalf("%d model requests, want 1", len(f.chat.requests))
	}
	sent := f.chat.requests[0]
	cutIdx := req.TailIndex + (res.TailStart - req.TailStart)
	if !reflect.DeepEqual(sent.Messages[:len(sent.Messages)-1], req.Messages[:cutIdx]) {
		t.Errorf("the compaction request is not the context up to the cut")
	}
	last := sent.Messages[len(sent.Messages)-1]
	if last.Role != llm.RoleUser || !strings.HasPrefix(last.Content, turnInstruction[:40]) {
		t.Errorf("the request should end with the instruction, got %q", last.Content)
	}
	if res.TokensAfter != EstimateTokens(res.Messages)+EstimateSpecs(testTools) {
		t.Errorf("TokensAfter %d should count the messages and the tool specs", res.TokensAfter)
	}
	if !reflect.DeepEqual(sent.Tools, testTools) || sent.MaxTokens != f.comp.Limits.Compacted {
		t.Errorf("tools %v max tokens %d; want the turn's tools and S", sent.Tools, sent.MaxTokens)
	}

	if req.Messages[cutIdx].Role != llm.RoleAssistant {
		t.Errorf("the tail starts with a %s message", req.Messages[cutIdx].Role)
	}
	if got := EstimateTokens(req.Messages[cutIdx:]); got < 2500 {
		t.Errorf("tail of %d tokens, want at least 2500", got)
	}
	m := res.Messages
	if m[0].Role != llm.RoleSystem || m[1].Content != "one" || m[2].Content != "done one" {
		t.Errorf("finished turn not reduced to its summary: %q / %q", m[1].Content, m[2].Content)
	}
	if m[3].Content != "two\n\n"+session.CompactedLabel+"\n\nread sa.go and sb.go" || res.TailIndex != 4 {
		t.Errorf("turn in progress = %q at tail index %d", m[3].Content, res.TailIndex)
	}
	if !reflect.DeepEqual(m[res.TailIndex:], req.Messages[cutIdx:]) {
		t.Errorf("the tail is not the request's own messages from the cut")
	}

	rec := f.timeline().LatestCompaction()
	if rec == nil || rec.From != from || rec.Cut != res.TailStart || rec.Compacted != "read sa.go and sb.go" ||
		rec.TokensBefore != req.TokensBefore || rec.TokensAfter != res.TokensAfter || rec.TokensAfter >= rec.TokensBefore {
		t.Errorf("record = %+v", rec)
	}
	disk, _, err := f.sess.LoadContext()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(disk.Messages, m[1:]) || disk.TailIndex+1 != res.TailIndex || disk.TailStart != res.TailStart {
		t.Errorf("the context the next process loads differs from the one returned")
	}
}

// TestCompactFallsBackWhenTheModelFails: two failures leave the turn's
// messages with their outputs removed, and say so.
func TestCompactFallsBackWhenTheModelFails(t *testing.T) {
	f := newFixture(t, 100_000, smallTail, reply{err: errors.New("boom")}, reply{err: errors.New("boom again")})
	from := f.turn(append([]llm.Message{user("go")}, steps("s", 6, 3000)...)...)
	res, err := f.comp.Compact(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if !res.Fallback || res.Summarized || len(f.chat.requests) != 2 || len(f.warns) != 1 {
		t.Fatalf("result %+v after %d requests, warnings %v", res, len(f.chat.requests), f.warns)
	}
	prefix := res.Messages[1:res.TailIndex]
	if prefix[0].Content != "go" {
		t.Errorf("the turn's prompt should stay, got %q", prefix[0].Content)
	}
	for _, m := range prefix {
		if m.Role == llm.RoleTool && m.Content != session.RemovedOutputNote {
			t.Errorf("a tool output before the cut was kept: %d chars", len(m.Content))
		}
	}
	if rec := f.timeline().LatestCompaction(); rec.Compacted != "" || rec.From != from {
		t.Errorf("record = %+v, want no compacted text", rec)
	}
}

// TestCompactRejectsUnusableAnswers: a tool call or an answer cut off at the
// token limit is not compacted text; the retry is.
func TestCompactRejectsUnusableAnswers(t *testing.T) {
	for _, bad := range []reply{{toolCall: true, text: "x"}, {text: "half", finish: "length"}, {text: "  "}} {
		f := newFixture(t, 100_000, smallTail, bad, reply{text: "good"})
		f.turn(append([]llm.Message{user("go")}, steps("s", 6, 3000)...)...)
		res, err := f.comp.Compact(context.Background(), f.request())
		if err != nil || !res.Summarized || len(f.chat.requests) != 2 {
			t.Errorf("after %+v: result %+v, err %v, %d requests", bad, res, err, len(f.chat.requests))
		}
	}
}

// TestCompactAfterOverflowRemovesOutputsFromTheRequest: after a refusal for
// length the compaction request is cut down, and the caller's messages are
// left alone.
func TestCompactAfterOverflowRemovesOutputsFromTheRequest(t *testing.T) {
	f := newFixture(t, 20_000, config.CompactionConfig{KeepRecentTokens: 2000}, reply{text: "ok"})
	// About 24k tokens: more than three quarters of what the window leaves
	// once the answer's room is set aside, so outputs have to go.
	f.turn(append([]llm.Message{user("go")}, steps("s", 24, 3000)...)...)
	req := f.request()
	before := append([]llm.Message(nil), req.Messages...)
	req.Overflow = true
	if _, err := f.comp.Compact(context.Background(), req); err != nil {
		t.Fatalf("Compact: %v", err)
	}
	removed := 0
	for _, m := range f.chat.requests[0].Messages {
		if m.Role == llm.RoleTool && m.Content == session.RemovedOutputNote {
			removed++
		}
	}
	if removed == 0 {
		t.Errorf("no tool output was removed from the compaction request")
	}
	if est := EstimateTokens(f.chat.requests[0].Messages); est > (20_000-f.comp.Limits.Compacted)*3/4+500 {
		t.Errorf("compaction request still %d tokens", est)
	}
	if !reflect.DeepEqual(req.Messages, before) {
		t.Errorf("Compact changed the caller's messages")
	}
}

// TestCompactAtTurnStartIsCode: when the last turn ended with an answer there
// is no turn in progress yet, so finished turns become summaries and no model
// is asked.
func TestCompactAtTurnStartIsCode(t *testing.T) {
	f := newFixture(t, 100_000, smallTail)
	f.turn(append([]llm.Message{user("one")}, append(steps("a", 3, 3000), say("done one"))...)...)
	f.turn(append([]llm.Message{user("two")}, append(steps("b", 3, 3000), say("done two"))...)...)
	res, err := f.comp.Compact(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if len(f.chat.requests) != 0 {
		t.Errorf("%d model requests, want none", len(f.chat.requests))
	}
	n := len(f.timeline().Messages)
	if res.TailStart != n || res.TailIndex != len(res.Messages) || len(res.Messages) != 5 {
		t.Errorf("result: %d messages, tail %d at %d; want system and two summaries, tail at %d", len(res.Messages), res.TailIndex, res.TailStart, n)
	}
	if rec := f.timeline().LatestCompaction(); rec.From != n || rec.Cut != n {
		t.Errorf("record = %+v", rec)
	}
}

// TestCompactAtTurnStartAfterAnUnansweredTurn: a turn that was cancelled is the
// turn in progress, so the model compacts it.
func TestCompactAtTurnStartAfterAnUnansweredTurn(t *testing.T) {
	f := newFixture(t, 100_000, smallTail, reply{text: "half done"})
	f.turn(user("one"), say("done one"))
	from := f.turn(append([]llm.Message{user("two")}, steps("b", 6, 3000)...)...)
	if err := f.sess.Notice(session.NoticeCancelled); err != nil {
		t.Fatal(err)
	}
	res, err := f.comp.Compact(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Compact: %v", err)
	}
	if !res.Summarized || len(f.chat.requests) != 1 {
		t.Fatalf("result %+v, %d requests", res, len(f.chat.requests))
	}
	if rec := f.timeline().LatestCompaction(); rec.From != from || rec.Cut <= from {
		t.Errorf("record = %+v, want the cancelled turn compacted", rec)
	}
}

// TestNothingToCompact: a context already as short as compaction makes it is
// left alone and no record is written.
func TestNothingToCompact(t *testing.T) {
	f := newFixture(t, 100_000, smallTail)
	f.turn(user("go"), say("done"))
	f.turn(user("again"))
	if _, err := f.comp.Compact(context.Background(), f.request()); err != nil {
		t.Fatalf("first: %v", err)
	}
	if _, err := f.comp.Compact(context.Background(), f.request()); !errors.Is(err, ErrNothingToCompact) {
		t.Errorf("second Compact: err = %v, want ErrNothingToCompact", err)
	}
	if got := len(f.timeline().Compactions); got != 1 {
		t.Errorf("%d records, want 1", got)
	}
}

// TestSecondCompactionReplacesTheFirst: the second request carries the first
// compacted text, and the result keeps only the new one.
func TestSecondCompactionReplacesTheFirst(t *testing.T) {
	f := newFixture(t, 100_000, smallTail, reply{text: "first"}, reply{text: "second"})
	f.turn(append([]llm.Message{user("go")}, steps("s", 6, 3000)...)...)
	first, err := f.comp.Compact(context.Background(), f.request())
	if err != nil {
		t.Fatal(err)
	}
	more := steps("t", 6, 3000)
	f.add(more...)
	req := Request{Messages: append(append([]llm.Message(nil), first.Messages...), more...), TailIndex: first.TailIndex, TailStart: first.TailStart}
	second, err := f.comp.Compact(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	sent := f.chat.requests[1].Messages
	if !strings.HasSuffix(sent[1].Content, "first") {
		t.Errorf("the second request should carry the first compacted text, got %q", sent[1].Content)
	}
	if got := second.Messages[1].Content; !strings.HasSuffix(got, "second") || strings.Contains(got, "first") {
		t.Errorf("turn in progress after the second rewrite = %q", got)
	}
	if second.TailStart <= first.TailStart {
		t.Errorf("the cut did not move: %d then %d", first.TailStart, second.TailStart)
	}
}

// TestHistoryMismatchIsRefused: messages that do not line up with the history
// are not compacted.
func TestHistoryMismatchIsRefused(t *testing.T) {
	f := newFixture(t, 100_000, smallTail)
	f.turn(append([]llm.Message{user("go")}, steps("s", 6, 3000)...)...)
	req := f.request()
	req.Messages = req.Messages[:len(req.Messages)-1]
	if _, err := f.comp.Compact(context.Background(), req); err == nil || errors.Is(err, ErrNothingToCompact) {
		t.Errorf("err = %v, want a mismatch error", err)
	}
}

// TestPromptPrefixIsKeptWhenThePromptIsFolded: the project instructions the
// turn's prompt had on the wire stay in front of it in the compacted message.
func TestPromptPrefixIsKeptWhenThePromptIsFolded(t *testing.T) {
	f := newFixture(t, 100_000, smallTail, reply{text: "c"})
	f.turn(append([]llm.Message{user("go")}, steps("s", 6, 3000)...)...)
	req := f.request()
	req.PromptPrefix = "PROJECT RULES\nUser request:\n"
	res, err := f.comp.Compact(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got := res.Messages[1].Content; !strings.HasPrefix(got, "PROJECT RULES\nUser request:\ngo") {
		t.Errorf("compacted message = %q", got)
	}
}

// --- summarizing older turns -------------------------------------------------

// answeredTurns writes n turns, each answered in about size characters.
func answeredTurns(f *fixture, n, size int) {
	for i := 0; i < n; i++ {
		f.turn(user("task "+string(rune('a'+i))), say(strings.Repeat("y", size)))
	}
}

// TestOlderTurnsAreSummarizedAboveTheThreshold: when the rewritten context is
// still above the threshold, turns 1 … N-2 become one summary written from
// their turn summaries; the last finished turn stays.
func TestOlderTurnsAreSummarizedAboveTheThreshold(t *testing.T) {
	f := newFixture(t, 10_000, config.CompactionConfig{KeepRecentTokens: 1000}, reply{text: "older work"})
	answeredTurns(f, 4, 1500)
	res, err := f.comp.Compact(context.Background(), f.request())
	if err != nil {
		t.Fatal(err)
	}
	if !res.OlderSummarized || len(f.chat.requests) != 1 {
		t.Fatalf("result %+v, %d requests", res, len(f.chat.requests))
	}
	sent := f.chat.requests[0].Messages
	if sent[0].Role != llm.RoleSystem || sent[1].Content != "task a" || sent[5].Content != "task c" || len(sent) != 8 {
		t.Errorf("summary request = %v", shape(sent))
	}
	if !strings.HasPrefix(sent[len(sent)-1].Content, olderInstruction[:40]) {
		t.Errorf("the summary request should end with its instruction")
	}
	m := res.Messages
	if m[1].Content != session.OlderTurnsQuestion || m[2].Content != "older work" || m[3].Content != "task d" {
		t.Errorf("context = %v / %q / %q / %q", shape(m), m[1].Content, m[2].Content, m[3].Content)
	}
	tl := f.timeline()
	if rec := tl.LatestCompaction(); rec.OlderSummary != "older work" || rec.OlderUpTo != tl.Turns[3].At {
		t.Errorf("record = %+v", rec)
	}
}

// TestSmallContextIsNotSummarized: below the threshold the turn summaries stay.
func TestSmallContextIsNotSummarized(t *testing.T) {
	f := newFixture(t, 1_000_000, smallTail)
	answeredTurns(f, 4, 100)
	res, err := f.comp.Compact(context.Background(), f.request())
	if err != nil {
		t.Fatal(err)
	}
	if res.OlderSummarized || len(f.chat.requests) != 0 {
		t.Errorf("summarized a small context: %+v", res)
	}
}

// TestForceSummarizesOlderTurns: :compact summarizes the oldest turns however
// small the result, and passes on its focus.
func TestForceSummarizesOlderTurns(t *testing.T) {
	f := newFixture(t, 1_000_000, smallTail, reply{text: "s"})
	answeredTurns(f, 3, 100)
	req := f.request()
	req.Force, req.Focus = true, "the parser"
	res, err := f.comp.Compact(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OlderSummarized {
		t.Fatalf("result %+v", res)
	}
	last := f.chat.requests[0].Messages
	if !strings.HasSuffix(last[len(last)-1].Content, "Additional focus: the parser") {
		t.Errorf("focus not passed: %q", last[len(last)-1].Content)
	}
}

// TestOlderSummaryStartsFromThePreviousWhenTooLarge: turn summaries that would
// take more than half the window are replaced by the previous summary and the
// turns after it.
func TestOlderSummaryStartsFromThePreviousWhenTooLarge(t *testing.T) {
	f := newFixture(t, 6_000, config.CompactionConfig{KeepRecentTokens: 500}, reply{text: "first summary"}, reply{text: "second summary"})
	answeredTurns(f, 4, 4000)
	if _, err := f.comp.Compact(context.Background(), f.request()); err != nil {
		t.Fatal(err)
	}
	answeredTurns(f, 2, 4000)
	if _, err := f.comp.Compact(context.Background(), f.request()); err != nil {
		t.Fatal(err)
	}
	if len(f.chat.requests) != 2 {
		t.Fatalf("%d requests, want 2", len(f.chat.requests))
	}
	sent := f.chat.requests[1].Messages
	if sent[1].Content != session.OlderTurnsQuestion || sent[2].Content != "first summary" {
		t.Errorf("second summary request should start from the first summary, got %v", shape(sent))
	}
}

func shape(msgs []llm.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		c := m.Content
		if len(c) > 12 {
			c = c[:12] + "…"
		}
		out[i] = string(m.Role) + ":" + c
	}
	return out
}

// --- estimates ---------------------------------------------------------------

// TestStartEstimateUsesTheRecordedCount: the last recorded count is used when
// it came from the same model after the last compaction.
func TestStartEstimateUsesTheRecordedCount(t *testing.T) {
	f := newFixture(t, 100_000, smallTail)
	answer := say("done")
	answer.PromptTokens = 50_000
	f.turn(user("go"), answer)
	tl := f.timeline()
	req := f.request()
	prompt, turn := StartEstimate(tl, req.Messages, 0, func(string) bool { return true })
	if prompt != 50_000+EstimateTokens(tl.Messages[1:]) {
		t.Errorf("prompt = %d, want the recorded count plus the answer", prompt)
	}
	if turn != 0 {
		t.Errorf("turn = %d, want 0 after an answered turn", turn)
	}
	if p, _ := StartEstimate(tl, req.Messages, 700, func(string) bool { return false }); p != EstimateTokens(req.Messages)+700 {
		t.Errorf("after a model switch the whole context and the specs should be estimated, got %d", p)
	}
}

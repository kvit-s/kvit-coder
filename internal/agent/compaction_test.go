package agent

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/compaction"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
)

// fakeCompactor records what the loop asks it and answers with a scripted
// rewrite.
type fakeCompactor struct {
	limits   compaction.Settings
	requests []compaction.Request
	rewrite  func(compaction.Request) (*compaction.Result, error)
}

func (f *fakeCompactor) Settings() compaction.Settings { return f.limits }

func (f *fakeCompactor) Compact(ctx context.Context, req compaction.Request) (*compaction.Result, error) {
	req.Messages = append([]llm.Message(nil), req.Messages...)
	f.requests = append(f.requests, req)
	return f.rewrite(req)
}

// shrinkToTail rewrites a context to the system prompt, one summary message
// and the last step.
func shrinkToTail(req compaction.Request) (*compaction.Result, error) {
	n := len(req.Messages)
	msgs := []llm.Message{req.Messages[0], {Role: llm.RoleUser, Content: "summary"}}
	msgs = append(msgs, req.Messages[n-2:]...)
	return &compaction.Result{
		Messages:     msgs,
		TailIndex:    2,
		TailStart:    req.TailStart + (n - 2 - req.TailIndex),
		TokensBefore: req.TokensBefore,
		TokensAfter:  1000,
	}, nil
}

func withUsage(step scriptStep, prompt int) scriptStep {
	step.resp.Usage = llm.Usage{PromptTokens: prompt, CompletionTokens: 10}
	return step
}

func compactionStart(messages []llm.Message, prompt int) *CompactionStart {
	return &CompactionStart{TailIndex: 1, TailStart: 0, PromptTokens: prompt, TurnTokens: 100}
}

func systemAndPrompt(text string) []llm.Message {
	return []llm.Message{{Role: llm.RoleSystem, Content: "system"}, {Role: llm.RoleUser, Content: text}}
}

// TestCompactionRunsBeforeTheRequestThatWouldNotFit: the loop compacts before
// sending a request that would leave too little of the window free, sends the
// rewritten context from then on, and writes every message to the session
// exactly once.
func TestCompactionRunsBeforeTheRequestThatWouldNotFit(t *testing.T) {
	client := newFakeClient(
		withUsage(calls(toolCall("c1", "echo", map[string]string{"arg": "a"})), 85_000),
		withUsage(calls(toolCall("c2", "echo", map[string]string{"arg": "b"})), 90_000),
		withUsage(answer("done"), 1_200),
	)
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})
	comp := &fakeCompactor{limits: compaction.Resolve(config.CompactionConfig{}, 100_000), rewrite: shrinkToTail}
	runner.SetCompactor(comp)
	var written []llm.Message
	runner.SetPersist(func(batch []llm.Message) error {
		written = append(written, batch...)
		return nil
	}, nil)

	start := systemAndPrompt("go")
	if _, err := runner.Run(context.Background(), RunConfig{Messages: start, Compaction: compactionStart(start, 2_000)}); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(comp.requests) != 1 {
		t.Fatalf("compacted %d times, want once", len(comp.requests))
	}
	req := comp.requests[0]
	assertShape(t, req.Messages, "system", "user", "assistant+tool_calls", "tool:echo", "assistant+tool_calls", "tool:echo")
	if req.TokensBefore < 90_000 || req.Overflow {
		t.Errorf("compaction request: tokens %d overflow %v", req.TokensBefore, req.Overflow)
	}
	assertShape(t, client.requests[2].Messages, "system", "user", "assistant+tool_calls", "tool:echo")
	if client.requests[2].Messages[1].Content != "summary" {
		t.Errorf("the request after compacting was not the rewritten context")
	}

	assertShape(t, written, "assistant+tool_calls", "tool:echo", "assistant+tool_calls", "tool:echo", "assistant")
	for i, want := range map[int]int{0: 85_000, 2: 90_000, 4: 1_200} {
		if written[i].PromptTokens != want {
			t.Errorf("message %d recorded %d prompt tokens, want %d", i, written[i].PromptTokens, want)
		}
	}
}

// TestNoCompactionWithoutAStart: a run with no CompactionStart — a benchmark,
// a subagent — never compacts.
func TestNoCompactionWithoutAStart(t *testing.T) {
	client := newFakeClient(withUsage(calls(toolCall("c1", "echo", nil)), 99_000), answer("done"))
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})
	comp := &fakeCompactor{limits: compaction.Resolve(config.CompactionConfig{}, 100_000), rewrite: shrinkToTail}
	runner.SetCompactor(comp)
	if _, err := runner.Run(context.Background(), RunConfig{Messages: systemAndPrompt("go")}); err != nil {
		t.Fatal(err)
	}
	if len(comp.requests) != 0 {
		t.Errorf("compacted %d times without a CompactionStart", len(comp.requests))
	}
}

// TestCompactionOffWhenAutoIsOff: compaction.auto: false leaves the turn alone.
func TestCompactionOffWhenAutoIsOff(t *testing.T) {
	off := false
	client := newFakeClient(withUsage(calls(toolCall("c1", "echo", nil)), 99_000), answer("done"))
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})
	comp := &fakeCompactor{limits: compaction.Resolve(config.CompactionConfig{Auto: &off}, 100_000), rewrite: shrinkToTail}
	runner.SetCompactor(comp)
	start := systemAndPrompt("go")
	if _, err := runner.Run(context.Background(), RunConfig{Messages: start, Compaction: compactionStart(start, 2_000)}); err != nil {
		t.Fatal(err)
	}
	if len(comp.requests) != 0 {
		t.Errorf("compacted %d times with auto off", len(comp.requests))
	}
}

// TestOverflowErrorCompactsAndRetries: a request the endpoint refuses as too
// long is compacted and sent again.
func TestOverflowErrorCompactsAndRetries(t *testing.T) {
	overflow := &llm.StatusError{Code: 400, Body: []byte(`{"error":{"message":"This model's maximum context length is 100000 tokens","code":"context_length_exceeded"}}`)}
	client := newFakeClient(
		withUsage(calls(toolCall("c1", "echo", nil)), 10_000),
		scriptStep{err: overflow},
		withUsage(answer("done"), 1_000),
	)
	runner, out := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})
	comp := &fakeCompactor{limits: compaction.Resolve(config.CompactionConfig{}, 100_000), rewrite: shrinkToTail}
	runner.SetCompactor(comp)
	start := systemAndPrompt("go")
	res, err := runner.Run(context.Background(), RunConfig{Messages: start, Compaction: compactionStart(start, 2_000)})
	if err != nil || res.Failure != nil {
		t.Fatalf("Run: %v / %v", err, res.Failure)
	}
	if len(comp.requests) != 1 || !comp.requests[0].Overflow {
		t.Fatalf("compaction requests: %+v", comp.requests)
	}
	if client.requests[2].Messages[1].Content != "summary" {
		t.Errorf("the retried request was not the rewritten context")
	}
	if strings.Contains(out.String(), "server error, retrying") {
		t.Errorf("the overflow went down the blank-and-retry path too:\n%s", out.String())
	}
}

// TestOtherBadRequestsDoNotCompact: a 400 that is not about length keeps its
// old handling.
func TestOtherBadRequestsDoNotCompact(t *testing.T) {
	bad := &llm.StatusError{Code: 400, Body: []byte(`{"error":{"message":"Invalid schema for function 'echo'"}}`)}
	client := newFakeClient(scriptStep{err: bad}, answer("done"))
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})
	comp := &fakeCompactor{limits: compaction.Resolve(config.CompactionConfig{}, 100_000), rewrite: shrinkToTail}
	runner.SetCompactor(comp)
	start := systemAndPrompt("go")
	if _, err := runner.Run(context.Background(), RunConfig{Messages: start, Compaction: compactionStart(start, 2_000)}); err != nil {
		t.Fatal(err)
	}
	if len(comp.requests) != 0 {
		t.Errorf("a schema error was treated as an overflow")
	}
}

// summaryClient is a fakeClient that also takes SetReasoningSummary, as the
// real client does.
type summaryClient struct {
	*fakeClient
	summaries []string
}

func (s *summaryClient) SetReasoningSummary(v string) { s.summaries = append(s.summaries, v) }

func thinking(step scriptStep, text string) scriptStep {
	step.resp.Choices[0].Message.ReasoningContent = text
	return step
}

// TestTierZeroStopsMergingThinking: past the tier 0 point new thinking is
// dropped rather than merged, the client stops asking for it, and a notice is
// recorded.
func TestTierZeroStopsMergingThinking(t *testing.T) {
	inner := newFakeClient(
		withUsage(thinking(calls(toolCall("c1", "echo", nil)), "first thought"), 70_000),
		withUsage(thinking(calls(toolCall("c2", "echo", nil)), "second thought"), 81_000),
		withUsage(thinking(answer("done"), "third thought"), 82_000),
	)
	client := &summaryClient{fakeClient: inner}
	cfg := testConfig()
	cfg.LLM.MergeThinking = true
	runner, _ := newTestRunner(t, cfg, client, &scriptedTool{name: "echo"})
	runner.SetCompactor(&fakeCompactor{limits: compaction.Resolve(config.CompactionConfig{}, 100_000), rewrite: shrinkToTail})
	var notices []string
	runner.SetNotice(func(s string) error { notices = append(notices, s); return nil })

	start := systemAndPrompt("go")
	res, err := runner.Run(context.Background(), RunConfig{Messages: start, Compaction: compactionStart(start, 2_000)})
	if err != nil {
		t.Fatal(err)
	}
	var assistants []llm.Message
	for _, m := range res.FinalMessages {
		if m.Role == llm.RoleAssistant {
			assistants = append(assistants, m)
		}
	}
	if !strings.Contains(assistants[0].Content, "first thought") || !strings.Contains(assistants[1].Content, "second thought") {
		t.Errorf("thinking before the tier 0 point should be merged: %q / %q", assistants[0].Content, assistants[1].Content)
	}
	if strings.Contains(assistants[2].Content, "third thought") {
		t.Errorf("thinking after the tier 0 point was merged: %q", assistants[2].Content)
	}
	if !reflect.DeepEqual(client.summaries, []string{""}) {
		t.Errorf("reasoning summaries set to %v, want one call turning them off", client.summaries)
	}
	if len(notices) != 1 || !strings.Contains(notices[0], "no longer merging") {
		t.Errorf("notices = %v", notices)
	}
}

// TestCompactionInATurnAgainstASession runs a turn through a real session and
// compactor: the context the loop goes on with after compacting is the one the
// next process loads from the history.
func TestCompactionInATurnAgainstASession(t *testing.T) {
	sess, err := session.OpenDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := sess.Settings("test-model", false, nil); err != nil {
		t.Fatal(err)
	}
	if err := sess.AppendMessages([]llm.Message{{Role: llm.RoleUser, Content: "go"}}); err != nil {
		t.Fatal(err)
	}

	big := strings.Repeat("x", 6000)
	var script []scriptStep
	for i := 0; i < 8; i++ {
		// Different arguments each time, or the loop detector adds its
		// reminder to the results in memory, which the history never has.
		id := "c" + string(rune('a'+i))
		script = append(script, withUsage(calls(toolCall(id, "echo", map[string]string{"arg": id})), 20_000+i*9_000))
	}
	script = append(script, withUsage(answer("done"), 30_000))
	client := newFakeClient(script...)
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo",
		call: func(_ context.Context, args json.RawMessage) (any, error) { return string(args) + big, nil }})
	chat := &compactionChat{}
	runner.SetCompactor(&compaction.Compactor{
		Sess:   sess,
		Client: chat,
		Model:  "test-model",
		Limits: compaction.Resolve(config.CompactionConfig{KeepRecentTokens: 3000}, 100_000),
	})
	runner.SetPersist(func(batch []llm.Message) error { return sess.AppendMessages(batch) }, sess.Rollback)

	start := systemAndPrompt("go")
	res, err := runner.Run(context.Background(), RunConfig{Messages: start, Compaction: compactionStart(start, 50)})
	if err != nil {
		t.Fatal(err)
	}
	tl, err := sess.Timeline()
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Compactions) == 0 || chat.calls == 0 {
		t.Fatalf("no compaction happened (records %d, model calls %d)", len(tl.Compactions), chat.calls)
	}
	ctx := session.BuildContext(tl)
	if !reflect.DeepEqual(ctx.Messages, res.FinalMessages[1:]) {
		t.Errorf("the next process would load a different context:\n disk   %v\n memory %v",
			shape(ctx.Messages), shape(res.FinalMessages[1:]))
		for i := range ctx.Messages {
			if !reflect.DeepEqual(ctx.Messages[i], res.FinalMessages[1+i]) {
				t.Logf("message %d differs:\n disk   %.200q\n memory %.200q", i, ctx.Messages[i].Content, res.FinalMessages[1+i].Content)
			}
		}
	}
	loaded, err := sess.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 1+8*2+1 {
		t.Errorf("history has %d messages, want every message written once (18)", len(loaded))
	}
}

// compactionChat answers compaction requests with a fixed text.
type compactionChat struct{ calls int }

func (c *compactionChat) Chat(ctx context.Context, req llm.ChatRequest) (*llm.ChatResponse, error) {
	c.calls++
	return &llm.ChatResponse{Choices: []llm.Choice{{
		Message:      llm.Message{Role: llm.RoleAssistant, Content: "compacted"},
		FinishReason: "stop",
	}}}, nil
}

// TestTierZeroFiresWithTierOne: when the tier 1 point comes before the
// thinking cutoff, reaching it stops the merging as well as compacting.
func TestTierZeroFiresWithTierOne(t *testing.T) {
	inner := newFakeClient(
		withUsage(thinking(calls(toolCall("c1", "echo", nil)), "first thought"), 72_000),
		withUsage(thinking(answer("done"), "second thought"), 5_000),
	)
	client := &summaryClient{fakeClient: inner}
	cfg := testConfig()
	cfg.LLM.MergeThinking = true
	runner, _ := newTestRunner(t, cfg, client, &scriptedTool{name: "echo"})
	comp := &fakeCompactor{limits: compaction.Resolve(config.CompactionConfig{}, 100_000), rewrite: shrinkToTail}
	runner.SetCompactor(comp)

	start := systemAndPrompt("go")
	cs := compactionStart(start, 2_000)
	cs.TurnTokens = 60_000 // margin 30k: tier 1 at 70k, before the 80k cutoff
	res, err := runner.Run(context.Background(), RunConfig{Messages: start, Compaction: cs})
	if err != nil {
		t.Fatal(err)
	}
	if len(comp.requests) != 1 {
		t.Fatalf("compacted %d times, want once", len(comp.requests))
	}
	last := res.FinalMessages[len(res.FinalMessages)-1]
	if strings.Contains(last.Content, "second thought") {
		t.Errorf("thinking was merged after the tier 1 point: %q", last.Content)
	}
}

// TestNoCompactionStraightAfterARewrite: on a small window the context right
// after a rewrite is a large part of it; the next rewrite waits until the
// context has grown by half of what is left, rather than firing every step.
func TestNoCompactionStraightAfterARewrite(t *testing.T) {
	client := newFakeClient(
		withUsage(calls(toolCall("c1", "echo", map[string]string{"arg": "1"})), 31_000),
		withUsage(calls(toolCall("c2", "echo", map[string]string{"arg": "2"})), 18_000),
		withUsage(calls(toolCall("c3", "echo", map[string]string{"arg": "3"})), 20_300),
		withUsage(calls(toolCall("c4", "echo", map[string]string{"arg": "4"})), 22_600),
		withUsage(calls(toolCall("c5", "echo", map[string]string{"arg": "5"})), 24_900),
		withUsage(answer("done"), 26_000),
	)
	runner, _ := newTestRunner(t, testConfig(), client, &scriptedTool{name: "echo"})
	comp := &fakeCompactor{limits: compaction.Resolve(config.CompactionConfig{}, 40_000),
		rewrite: func(req compaction.Request) (*compaction.Result, error) {
			res, err := shrinkToTail(req)
			res.TokensAfter = 17_000
			return res, err
		}}
	runner.SetCompactor(comp)
	start := systemAndPrompt("go")
	cs := compactionStart(start, 2_000)
	cs.TurnTokens = 60_000
	if _, err := runner.Run(context.Background(), RunConfig{Messages: start, Compaction: cs}); err != nil {
		t.Fatal(err)
	}
	if len(comp.requests) != 1 {
		t.Errorf("compacted %d times, want once: the context never grew 11.5k past the 17k floor", len(comp.requests))
	}
}

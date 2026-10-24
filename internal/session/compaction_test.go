package session

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// --- building histories ------------------------------------------------------

func newCompactionSession(t *testing.T) *Session {
	t.Helper()
	s, err := setupTestManager(t).Open("compaction")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

// startTurn writes a turn's settings line and its messages, the way RunTurn
// and the loop do, and returns the position of its first message.
func startTurn(t *testing.T, s *Session, msgs ...llm.Message) int {
	t.Helper()
	tl, err := s.Timeline()
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	at := len(tl.Messages)
	if err := s.Settings("test-model", false, nil); err != nil {
		t.Fatalf("Settings: %v", err)
	}
	appendMsgs(t, s, msgs...)
	return at
}

func appendMsgs(t *testing.T, s *Session, msgs ...llm.Message) {
	t.Helper()
	if len(msgs) == 0 {
		return
	}
	if err := s.AppendMessages(msgs); err != nil {
		t.Fatalf("AppendMessages: %v", err)
	}
}

func userMsg(text string) llm.Message { return llm.Message{Role: llm.RoleUser, Content: text} }
func sayMsg(text string) llm.Message  { return llm.Message{Role: llm.RoleAssistant, Content: text} }

func callMsg(id, name, args string) llm.Message {
	return llm.Message{Role: llm.RoleAssistant, ToolCalls: []llm.ToolCall{{
		ID: id, Type: "function", Function: llm.ToolCallFunction{Name: name, Arguments: args},
	}}}
}

func resultMsg(id, name, content string) llm.Message {
	return llm.Message{Role: llm.RoleTool, Name: name, ToolCallID: id, Content: content}
}

func timeline(t *testing.T, s *Session) *Timeline {
	t.Helper()
	tl, err := s.Timeline()
	if err != nil {
		t.Fatalf("Timeline: %v", err)
	}
	return tl
}

// --- the timeline ------------------------------------------------------------

// TestTimelinePlacesEverythingByPosition: turns, notices and compaction
// records are placed by the number of messages before them, and the record's
// fields survive the round trip through the file.
func TestTimelinePlacesEverythingByPosition(t *testing.T) {
	s := newCompactionSession(t)
	startTurn(t, s, userMsg("one"), sayMsg("done"))
	if err := s.Notice(NoticeCancelled); err != nil {
		t.Fatal(err)
	}
	second := startTurn(t, s, userMsg("two"))
	rec := Compaction{From: 2, Cut: 2, Compacted: "c", OlderSummary: "o", OlderUpTo: 0, TokensBefore: 900, TokensAfter: 90, Model: "m"}
	if err := s.AppendCompaction(rec); err != nil {
		t.Fatal(err)
	}

	tl := timeline(t, s)
	if len(tl.Messages) != 3 {
		t.Fatalf("messages = %d, want 3", len(tl.Messages))
	}
	if len(tl.Turns) != 2 || tl.Turns[0].At != 0 || tl.Turns[1].At != second || tl.Turns[1].Model != "test-model" {
		t.Errorf("turns = %+v, want starts at 0 and %d", tl.Turns, second)
	}
	if len(tl.Notices) != 1 || tl.Notices[0].At != 2 || tl.Notices[0].Turn != 0 || tl.Notices[0].Text != NoticeCancelled {
		t.Errorf("notices = %+v, want the cancel notice at 2 in turn 0", tl.Notices)
	}
	got := tl.LatestCompaction()
	rec.At = 3
	if got == nil || *got != rec {
		t.Errorf("compaction = %+v, want %+v", got, rec)
	}
}

// TestPromptTokensRoundTrip: the server's count on an assistant message goes
// to its history line and comes back on the next load.
func TestPromptTokensRoundTrip(t *testing.T) {
	s := newCompactionSession(t)
	answer := sayMsg("done")
	answer.PromptTokens = 1234
	startTurn(t, s, userMsg("go"), answer)

	raw, err := os.ReadFile(s.HistoryPath())
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"prompt_tokens":1234`) {
		t.Errorf("history has no prompt_tokens line:\n%s", raw)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if loaded[1].PromptTokens != 1234 || loaded[0].PromptTokens != 0 {
		t.Errorf("prompt tokens after load = %d/%d, want 0/1234", loaded[0].PromptTokens, loaded[1].PromptTokens)
	}
}

// TestLoadIgnoresCompactionRecords: every reader of the full conversation —
// --session-show, the editor replay, the report card, migration — still gets
// all of it after a compaction.
func TestLoadIgnoresCompactionRecords(t *testing.T) {
	s := newCompactionSession(t)
	startTurn(t, s, userMsg("one"), callMsg("c1", "Read", `{"path":"a.go"}`), resultMsg("c1", "Read", "package a"), sayMsg("done"))
	before, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AppendCompaction(Compaction{From: 0, Cut: 3, Compacted: "x"}); err != nil {
		t.Fatal(err)
	}
	after, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("Load changed after a compaction record:\n before %+v\n after  %+v", before, after)
	}
}

// TestRollbackStopsAtTheCut: a rollback after a compaction record cannot take
// back messages before its cut, which the record's positions depend on.
func TestRollbackStopsAtTheCut(t *testing.T) {
	s := newCompactionSession(t)
	startTurn(t, s, userMsg("go"),
		callMsg("c1", "Read", `{}`), resultMsg("c1", "Read", "r1"),
		callMsg("c2", "Read", `{}`), resultMsg("c2", "Read", "r2"))
	if err := s.AppendCompaction(Compaction{From: 0, Cut: 3}); err != nil {
		t.Fatal(err)
	}
	if err := s.Rollback(4); err != nil {
		t.Fatal(err)
	}
	loaded, err := s.Load()
	if err != nil {
		t.Fatal(err)
	}
	if len(loaded) != 3 {
		t.Errorf("after the rollback %d messages remain, want 3 (the cut)", len(loaded))
	}
}

// --- the context -------------------------------------------------------------

// TestBuildContextWithoutRecordIsTheConversation: before any compaction the
// model is sent the whole conversation.
func TestBuildContextWithoutRecordIsTheConversation(t *testing.T) {
	s := newCompactionSession(t)
	startTurn(t, s, userMsg("go"), sayMsg("done"))
	ctx, tl, err := s.LoadContext()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ctx.Messages, tl.Messages) || ctx.TailIndex != 0 || ctx.TailStart != 0 {
		t.Errorf("context = %+v, want the conversation from position 0", ctx)
	}
}

// threeTurns writes the history most tests below compact: a turn answered in
// prose, a turn answered by an accepted report, and a turn in progress with
// three steps. It returns the start of the turn in progress and the position
// of its second step.
func threeTurns(t *testing.T, s *Session) (third, secondStep int) {
	t.Helper()
	startTurn(t, s,
		userMsg("one"),
		callMsg("a1", "Edit", `{"path":"a.go","start_line":1,"new_text":"x"}`),
		resultMsg("a1", "Edit", "ok"),
		sayMsg("done one"))
	startTurn(t, s,
		userMsg("two"),
		callMsg("r1", ReportToolName, `{"headline":"Two is done","task_status":"done","blocks":[]}`),
		resultMsg("r1", ReportToolName, "{\n  \"accepted\": true\n}"))
	third = startTurn(t, s,
		userMsg("three"),
		callMsg("t1", "Read", `{"path":"b.go"}`), resultMsg("t1", "Read", "package b"),
		callMsg("t2", "Shell", `{"command":"go test ./..."}`), resultMsg("t2", "Shell", "FAIL"),
		callMsg("t3", "Read", `{"path":"c.go"}`), resultMsg("t3", "Read", "package c"))
	return third, third + 3
}

func shapeOf(msgs []llm.Message) []string {
	out := make([]string, len(msgs))
	for i, m := range msgs {
		out[i] = string(m.Role)
		if len(m.ToolCalls) > 0 {
			out[i] += "+call"
		}
	}
	return out
}

// TestBuildContextShrinksFinishedTurnsAndCompactsTheTurnInProgress: finished
// turns become their words and answer, the turn in progress becomes its words
// and the compacted text, and the tail follows word for word.
func TestBuildContextShrinksFinishedTurnsAndCompactsTheTurnInProgress(t *testing.T) {
	s := newCompactionSession(t)
	third, cut := threeTurns(t, s)
	if err := s.AppendCompaction(Compaction{From: third, Cut: cut, Compacted: "read b.go"}); err != nil {
		t.Fatal(err)
	}
	ctx, tl, err := s.LoadContext()
	if err != nil {
		t.Fatal(err)
	}

	want := []string{"user", "assistant", "user", "assistant", "user", "assistant+call", "tool", "assistant+call", "tool"}
	if got := shapeOf(ctx.Messages); !reflect.DeepEqual(got, want) {
		t.Fatalf("shape = %v, want %v", got, want)
	}
	if ctx.TailIndex != 5 || ctx.TailStart != cut {
		t.Errorf("tail = index %d position %d, want 5 and %d", ctx.TailIndex, ctx.TailStart, cut)
	}
	if !reflect.DeepEqual(ctx.Messages[ctx.TailIndex:], tl.Messages[cut:]) {
		t.Errorf("the tail is not the conversation from the cut, word for word")
	}
	if ctx.Messages[0].Content != "one" || !strings.HasPrefix(ctx.Messages[1].Content, "done one") ||
		!strings.Contains(ctx.Messages[1].Content, "changed a.go") {
		t.Errorf("first turn = %q / %q", ctx.Messages[0].Content, ctx.Messages[1].Content)
	}
	if !strings.Contains(ctx.Messages[3].Content, "Two is done") {
		t.Errorf("the second turn's answer should be its accepted report, got %q", ctx.Messages[3].Content)
	}
	if got := ctx.Messages[4].Content; got != "three\n\n"+CompactedLabel+"\n\nread b.go" {
		t.Errorf("turn in progress = %q", got)
	}
}

// TestBuildContextWithoutCompactedTextRemovesOutputs: when the model could not
// compact the turn, its messages stay with their tool outputs removed.
func TestBuildContextWithoutCompactedTextRemovesOutputs(t *testing.T) {
	s := newCompactionSession(t)
	third, cut := threeTurns(t, s)
	if err := s.AppendCompaction(Compaction{From: third, Cut: cut}); err != nil {
		t.Fatal(err)
	}
	ctx, _, err := s.LoadContext()
	if err != nil {
		t.Fatal(err)
	}
	prefix := ctx.Messages[:ctx.TailIndex]
	want := []string{"user", "assistant", "user", "assistant", "user", "assistant+call", "tool"}
	if got := shapeOf(prefix); !reflect.DeepEqual(got, want) {
		t.Fatalf("prefix shape = %v, want %v", got, want)
	}
	if prefix[4].Content != "three" || prefix[6].Content != RemovedOutputNote {
		t.Errorf("turn in progress = %q, %q; want the prompt and a removed output", prefix[4].Content, prefix[6].Content)
	}
}

// TestBuildContextOlderSummary: the summary stands in for the turns before
// OlderUpTo, and stays in force when a later record has none.
func TestBuildContextOlderSummary(t *testing.T) {
	s := newCompactionSession(t)
	third, cut := threeTurns(t, s)
	second := timeline(t, s).Turns[1].At
	if err := s.AppendCompaction(Compaction{From: third, Cut: cut, Compacted: "c", OlderSummary: "turn one edited a.go", OlderUpTo: second}); err != nil {
		t.Fatal(err)
	}
	appendMsgs(t, s, callMsg("t4", "Read", `{}`), resultMsg("t4", "Read", "more"))
	if err := s.AppendCompaction(Compaction{From: third, Cut: cut + 2, Compacted: "c2"}); err != nil {
		t.Fatal(err)
	}
	ctx, _, err := s.LoadContext()
	if err != nil {
		t.Fatal(err)
	}
	if ctx.Messages[0].Content != OlderTurnsQuestion || ctx.Messages[1].Content != "turn one edited a.go" {
		t.Fatalf("context should open with the older summary, got %q / %q", ctx.Messages[0].Content, ctx.Messages[1].Content)
	}
	if ctx.Messages[2].Content != "two" {
		t.Errorf("the second turn should follow the summary, got %q", ctx.Messages[2].Content)
	}
	if !strings.HasSuffix(ctx.Messages[4].Content, "c2") {
		t.Errorf("the latest compacted text should be used, got %q", ctx.Messages[4].Content)
	}
}

// --- turn summaries ----------------------------------------------------------

func spanOf(tl *Timeline, i int) TurnSpan { return tl.TurnSpans()[i] }

// TestTurnSummaryForATurnWithoutAnAnswer: the reason comes from the notice.
func TestTurnSummaryForATurnWithoutAnAnswer(t *testing.T) {
	s := newCompactionSession(t)
	startTurn(t, s, userMsg("fix it"), callMsg("c1", "Read", `{"path":"x.go"}`), resultMsg("c1", "Read", "x"))
	if err := s.Notice(NoticeBudgetExhausted); err != nil {
		t.Fatal(err)
	}
	tl := timeline(t, s)
	sum := tl.TurnSummary(spanOf(tl, 0))
	if sum[0].Content != "fix it" {
		t.Errorf("words = %q", sum[0].Content)
	}
	if !strings.HasPrefix(sum[1].Content, "[the turn ended without an answer: "+NoticeBudgetExhausted+"]") {
		t.Errorf("answer = %q", sum[1].Content)
	}
	if !strings.Contains(sum[1].Content, "read x.go") {
		t.Errorf("activity line missing from %q", sum[1].Content)
	}
}

// TestTurnSummaryKeepsSteeringAndDropsAgentMessages: words typed during the
// turn stay; reminders the agent inserted do not; pictures become paths.
func TestTurnSummaryKeepsSteeringAndDropsAgentMessages(t *testing.T) {
	s := newCompactionSession(t)
	prompt := userMsg("look at this")
	prompt.Images = []llm.ImagePart{{Path: "/tmp/shot.png"}}
	startTurn(t, s,
		prompt,
		callMsg("c1", "Read", `{}`), resultMsg("c1", "Read", "x"),
		userMsg("<user-steering>\nalso check y\n</user-steering>"),
		sayMsg("interim"),
		userMsg("<system-reminder>\nThis turn has not submitted a report.\n</system-reminder>"),
		sayMsg("final"))
	tl := timeline(t, s)
	sum := tl.TurnSummary(spanOf(tl, 0))
	words := sum[0].Content
	if !strings.Contains(words, "look at this\n[image: /tmp/shot.png]") || !strings.Contains(words, "also check y") {
		t.Errorf("words = %q", words)
	}
	if strings.Contains(words, "has not submitted a report") {
		t.Errorf("an agent-inserted message reached the words: %q", words)
	}
	if !strings.HasPrefix(sum[1].Content, "final") {
		t.Errorf("answer = %q, want the last prose answer", sum[1].Content)
	}
}

// TestTurnSummaryForAWakeTurn: a turn the inbox started has no prompt.
func TestTurnSummaryForAWakeTurn(t *testing.T) {
	s := newCompactionSession(t)
	startTurn(t, s, sayMsg("the build finished"))
	tl := timeline(t, s)
	sum := tl.TurnSummary(spanOf(tl, 0))
	if sum[0].Content != InboxTurnInput || sum[1].Content != "the build finished" {
		t.Errorf("summary = %q / %q", sum[0].Content, sum[1].Content)
	}
}

// TestRejectedReportIsNotAnAnswer: a report the tool refused was something to
// repair, so the turn did not end with it.
func TestRejectedReportIsNotAnAnswer(t *testing.T) {
	s := newCompactionSession(t)
	startTurn(t, s,
		userMsg("go"),
		callMsg("r1", ReportToolName, `{"headline":"Done","blocks":[]}`),
		resultMsg("r1", ReportToolName, "Error: REPORT_REJECTED: 1 problem."))
	tl := timeline(t, s)
	if _, ok, _ := tl.FinalAnswer(spanOf(tl, 0)); ok {
		t.Errorf("a rejected report counted as the turn's answer")
	}
}

// TestInProgressFromSpansTurnsWithoutAnAnswer: the turn in progress reaches
// back over turns that ended without an answer to the last one that had one.
func TestInProgressFromSpansTurnsWithoutAnAnswer(t *testing.T) {
	s := newCompactionSession(t)
	startTurn(t, s, userMsg("one"), sayMsg("done"))
	second := startTurn(t, s, userMsg("two"), callMsg("c1", "Read", `{}`), resultMsg("c1", "Read", "x"))
	if err := s.Notice(NoticeCancelled); err != nil {
		t.Fatal(err)
	}
	if got := timeline(t, s).InProgressFrom(); got != second {
		t.Errorf("between turns after a cancelled one: InProgressFrom = %d, want %d", got, second)
	}
	startTurn(t, s, userMsg("continue"), callMsg("c2", "Read", `{}`))
	if got := timeline(t, s).InProgressFrom(); got != second {
		t.Errorf("during the next turn: InProgressFrom = %d, want %d", got, second)
	}
	appendMsgs(t, s, resultMsg("c2", "Read", "y"), sayMsg("all done"))
	tl := timeline(t, s)
	if got := tl.InProgressFrom(); got != len(tl.Messages) {
		t.Errorf("after an answered turn: InProgressFrom = %d, want %d", got, len(tl.Messages))
	}
}

// TestInProgressInputListsEachSpannedPrompt: the turn in progress keeps the
// prompt of every turn it spans, with their pictures still attached.
func TestInProgressInputListsEachSpannedPrompt(t *testing.T) {
	s := newCompactionSession(t)
	first := userMsg("fix the tests")
	first.Images = []llm.ImagePart{{Path: "/tmp/a.png"}}
	from := startTurn(t, s, first, callMsg("c1", "Read", `{}`), resultMsg("c1", "Read", "x"))
	if err := s.Notice(NoticeCancelled); err != nil {
		t.Fatal(err)
	}
	startTurn(t, s, userMsg("continue"), callMsg("c2", "Read", `{}`), resultMsg("c2", "Read", "y"))
	tl := timeline(t, s)
	words, images := tl.InProgressInput(from, len(tl.Messages))
	if words != "fix the tests\n\ncontinue" || len(images) != 1 {
		t.Errorf("words = %q, images = %d", words, len(images))
	}
}

// --- the activity line -------------------------------------------------------

func TestActivityLine(t *testing.T) {
	msgs := []llm.Message{
		callMsg("1", "Edit", `{"path":"a.go"}`),
		callMsg("2", "Write", `{"path":"b.go","text":"x"}`),
		callMsg("3", "Read", `{"path":"a.go"}`),
		callMsg("4", "Shell", `{"command":"go test ./...\necho done"}`),
		callMsg("5", "Shell_start", `{"command":"npm run dev"}`),
		callMsg("6", "Batch", `{"calls":[{"tool":"Read","args":{"path":"c.go"}},{"tool":"Edit","args":{"path":"a.go"}}]}`),
		callMsg("7", "Edit", `{"patch":"*** Begin Patch\n*** Update File: d.go\n*** End Patch"}`),
		callMsg("8", "Search", `{"pattern":"x"}`),
	}
	got := ActivityLine(msgs)
	want := "[In this turn: changed a.go, b.go, d.go; read a.go, c.go; ran `go test ./...`, `npm run dev`]"
	if got != want {
		t.Errorf("ActivityLine =\n %s\nwant\n %s", got, want)
	}
	if ActivityLine([]llm.Message{sayMsg("hi")}) != "" {
		t.Errorf("a turn that touched nothing should have no activity line")
	}
}

func TestActivityLineCapsEachList(t *testing.T) {
	var msgs []llm.Message
	for i := 0; i < activityCap+3; i++ {
		msgs = append(msgs, callMsg("r", "Read", `{"path":"f`+string(rune('a'+i))+`.go"}`))
	}
	if got := ActivityLine(msgs); !strings.HasSuffix(got, "and 3 more]") {
		t.Errorf("ActivityLine = %q, want the list capped with a count of the rest", got)
	}
}

// TestShowSessionMarksEachCompaction: --session-show lists every message and
// a line where each compaction happened.
func TestShowSessionMarksEachCompaction(t *testing.T) {
	mgr := setupTestManager(t)
	s, err := mgr.Open("shown")
	if err != nil {
		t.Fatal(err)
	}
	startTurn(t, s, userMsg("one"), sayMsg("done"))
	if err := s.AppendCompaction(Compaction{From: 2, Cut: 2, TokensBefore: 952750, TokensAfter: 61204}); err != nil {
		t.Fatal(err)
	}
	startTurn(t, s, userMsg("two"), sayMsg("done again"))
	out, err := mgr.ShowSession("shown")
	if err != nil {
		t.Fatal(err)
	}
	line := "── context compacted: 953k → 61.2k tokens ──"
	if !strings.Contains(out, line) || !strings.Contains(out, "done again") {
		t.Fatalf("ShowSession output:\n%s", out)
	}
	if strings.Index(out, line) > strings.Index(out, "two") || strings.Index(out, line) < strings.Index(out, "done\n") {
		t.Errorf("the compaction line is not between the turns:\n%s", out)
	}
}

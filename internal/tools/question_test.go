package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
)

func questionTestTool(t *testing.T, timeout int, interactive bool) (*QuestionTool, *inbox.Inbox, *bytes.Buffer) {
	t.Helper()
	cfg := &config.Config{}
	cfg.Tools.Question.Enabled = true
	cfg.Tools.Question.Timeout = timeout

	box := inbox.New("")
	toolCtx := NewToolContext()
	toolCtx.SetInbox(box)

	var out bytes.Buffer
	tool := NewQuestionTool(cfg, toolCtx)
	tool.out = &out
	tool.interactive = func() bool { return interactive }
	return tool, box, &out
}

func askArgs(t *testing.T, spec questionSpec) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(questionArgs{Questions: []questionSpec{spec}})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return raw
}

func answersFrom(t *testing.T, result any) []QuestionAnswer {
	t.Helper()
	m, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("result is %T, want a map", result)
	}
	answers, ok := m["answers"].([]QuestionAnswer)
	if !ok {
		t.Fatalf("result has no answers: %+v", m)
	}
	return answers
}

// TestNumberedAnswerSelectsAnOption: a bare number picks the option it names.
func TestNumberedAnswerSelectsAnOption(t *testing.T) {
	tool, box, out := questionTestTool(t, 0, true)
	spec := questionSpec{
		Question: "Reuse the retry wrapper, or write a new one?",
		Header:   "retry strategy",
		Options: []questionOption{
			{Label: "Reuse internal/http.Retry", Description: "same backoff"},
			{Label: "New wrapper", Description: "no shared state"},
		},
	}

	go func() {
		time.Sleep(50 * time.Millisecond)
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "2"})
	}()

	result, err := tool.Call(context.Background(), askArgs(t, spec))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	answers := answersFrom(t, result)
	if len(answers) != 1 {
		t.Fatalf("got %d answers, want 1", len(answers))
	}
	if len(answers[0].Answers) != 1 || answers[0].Answers[0] != "New wrapper" {
		t.Errorf("answer is %+v, want the second option's label", answers[0])
	}

	shown := out.String()
	if !strings.Contains(shown, "retry strategy") || !strings.Contains(shown, "1) Reuse internal/http.Retry") {
		t.Errorf("the question was not drawn with its header and numbered options:\n%s", shown)
	}
	if !strings.Contains(shown, "[1-2, or type]") {
		t.Errorf("the input hint does not say which mode the line is in:\n%s", shown)
	}
}

// TestMultiSelect: "1,3" picks several when the question allows it.
func TestMultiSelect(t *testing.T) {
	tool, box, _ := questionTestTool(t, 0, true)
	spec := questionSpec{
		Question: "Which suites should run?",
		Multi:    true,
		Options: []questionOption{
			{Label: "unit"}, {Label: "integration"}, {Label: "e2e"},
		},
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "1,3"})
	}()

	result, err := tool.Call(context.Background(), askArgs(t, spec))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	got := answersFrom(t, result)[0].Answers
	if len(got) != 2 || got[0] != "unit" || got[1] != "e2e" {
		t.Errorf("selected %v, want [unit e2e]", got)
	}
}

// TestFreeTextAnswer: anything that is not an option number comes back as
// free text, which is the escape when the choice has gone stale.
func TestFreeTextAnswer(t *testing.T) {
	tool, box, _ := questionTestTool(t, 0, true)
	spec := questionSpec{
		Question: "Which one?",
		Options:  []questionOption{{Label: "a"}, {Label: "b"}},
	}
	go func() {
		time.Sleep(20 * time.Millisecond)
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "neither, the build is broken"})
	}()

	result, err := tool.Call(context.Background(), askArgs(t, spec))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	a := answersFrom(t, result)[0]
	if a.FreeText != "neither, the build is broken" {
		t.Errorf("free text is %q, want what was typed", a.FreeText)
	}
	if len(a.Answers) != 0 {
		t.Errorf("free text was also read as an option: %v", a.Answers)
	}
}

// TestStaleLineIsNotTheAnswer: a line typed before the question appeared is
// steering, not an answer. It goes back in the inbox for the loop, and the
// question keeps waiting.
func TestStaleLineIsNotTheAnswer(t *testing.T) {
	tool, box, _ := questionTestTool(t, 0, true)
	box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "1"}) // typed before the question

	spec := questionSpec{
		Question: "Which one?",
		Options:  []questionOption{{Label: "first"}, {Label: "second"}},
	}
	go func() {
		time.Sleep(80 * time.Millisecond)
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "2"})
	}()

	result, err := tool.Call(context.Background(), askArgs(t, spec))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	a := answersFrom(t, result)[0]
	if len(a.Answers) != 1 || a.Answers[0] != "second" {
		t.Errorf("answered %+v, want the line typed after the question was shown", a)
	}

	// The stale line is still waiting for the loop to pick up as steering.
	left := box.Drain()
	if len(left) != 1 || left[0].Text != "1" {
		t.Errorf("the inbox holds %+v, want the stale line put back as steering", left)
	}
}

// TestHeadlessFallsBackAtOnce: with no terminal and the default timeout of
// zero, the tool proceeds immediately rather than hanging a scripted run.
func TestHeadlessFallsBackAtOnce(t *testing.T) {
	tool, _, out := questionTestTool(t, 0, false)
	spec := questionSpec{Question: "Which one?", Options: []questionOption{{Label: "a"}}}

	start := time.Now()
	result, err := tool.Call(context.Background(), askArgs(t, spec))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("the call took %s, want an immediate fallback", elapsed)
	}
	a := answersFrom(t, result)[0]
	if !a.Unanswered {
		t.Errorf("answer is %+v, want it marked unanswered", a)
	}
	if !strings.Contains(out.String(), "No one answered") {
		t.Errorf("nothing was said about nobody answering:\n%s", out.String())
	}

	m := result.(map[string]any)
	note, _ := m["note"].(string)
	if !strings.Contains(note, "state what you assumed") {
		t.Errorf("the result does not tell the model what to do instead: %q", note)
	}
}

// TestHeadlessWaitsForItsTimeout: with a timeout set, an answer dropped in the
// inbox within it is still picked up.
func TestHeadlessWaitsForItsTimeout(t *testing.T) {
	tool, box, _ := questionTestTool(t, 5, false)
	spec := questionSpec{Question: "Which one?", Options: []questionOption{{Label: "a"}, {Label: "b"}}}

	go func() {
		time.Sleep(100 * time.Millisecond)
		box.Push(inbox.Message{Kind: inbox.KindUserLine, Text: "1"})
	}()

	result, err := tool.Call(context.Background(), askArgs(t, spec))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	a := answersFrom(t, result)[0]
	if len(a.Answers) != 1 || a.Answers[0] != "a" {
		t.Errorf("answered %+v, want the option that was chosen inside the timeout", a)
	}
}

// TestDismissalIsRecordedAndRefusesARepeat: ending the turn instead of
// answering is a decision, and asking the same question again is refused.
func TestDismissalIsRecordedAndRefusesARepeat(t *testing.T) {
	tool, _, _ := questionTestTool(t, 0, true)
	spec := questionSpec{Question: "Should I drop the column?", Options: []questionOption{{Label: "yes"}, {Label: "no"}}}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := tool.Call(ctx, askArgs(t, spec))
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if a := answersFrom(t, result)[0]; !a.Dismissed {
		t.Fatalf("answer is %+v, want it marked dismissed", a)
	}

	// Asking again, word for word, is refused before it reaches a person.
	err = tool.Check(context.Background(), askArgs(t, spec))
	if err == nil {
		t.Fatal("repeating a dismissed question was allowed")
	}
	if !IsBacktrackable(err) {
		t.Errorf("the refusal is not a semantic error the model can recover from: %v", err)
	}
	if !strings.Contains(err.Error(), "dismissed") {
		t.Errorf("the refusal does not say why: %v", err)
	}
}

// TestRemainingQuestionsAreNotPutAfterDismissal: once the turn is being ended,
// the rest of a batch is not put to someone who has gone.
func TestRemainingQuestionsAreNotPutAfterDismissal(t *testing.T) {
	tool, _, _ := questionTestTool(t, 0, true)
	raw, _ := json.Marshal(questionArgs{Questions: []questionSpec{
		{Question: "First?"},
		{Question: "Second?"},
	}})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result, err := tool.Call(ctx, raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	answers := answersFrom(t, result)
	if len(answers) != 2 {
		t.Fatalf("got %d answers, want one per question", len(answers))
	}
	for i, a := range answers {
		if !a.Dismissed {
			t.Errorf("answer %d is %+v, want it marked dismissed", i, a)
		}
	}
}

// TestCheckRejectsAnEmptyCall: calling with no questions is a mistake worth
// saying so about rather than a silent no-op.
func TestCheckRejectsAnEmptyCall(t *testing.T) {
	tool, _, _ := questionTestTool(t, 0, true)
	if err := tool.Check(context.Background(), json.RawMessage(`{"questions":[]}`)); err == nil {
		t.Error("a call with no questions was accepted")
	}
	if err := tool.Check(context.Background(), json.RawMessage(`{"questions":[{"question":"  "}]}`)); err == nil {
		t.Error("a question with no text was accepted")
	}
}

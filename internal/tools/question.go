package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/inbox"
)

// QuestionTool lets the model ask a person something and get the answer as the
// tool's result, so no new input path is needed: the answer arrives through the
// inbox like everything else that reaches a running turn.
//
// It blocks. That costs nothing, because a background process is not in the
// loop — a build started earlier keeps building while you decide — and the only
// thing that stops is the model's turn, which had stopped anyway: it asked
// because it could not proceed.
type QuestionTool struct {
	cfg     *config.Config
	toolCtx *ToolContext

	// out is where the question is drawn. Stderr, so a headless run's answer on
	// stdout stays clean.
	out io.Writer
	// interactive reports whether a person is at the terminal. With no
	// terminal the tool waits tools.question.timeout for an answer to appear
	// in the session inbox and then proceeds on its own judgement.
	interactive func() bool
	// now is the clock, so a test can control which lines count as answers.
	now func() time.Time
}

// NewQuestionTool builds the tool. The inbox it reads comes from the tool
// context, which the agent shares with the loop.
func NewQuestionTool(cfg *config.Config, toolCtx *ToolContext) *QuestionTool {
	return &QuestionTool{
		cfg:         cfg,
		toolCtx:     toolCtx,
		out:         os.Stderr,
		interactive: toolCtx.Interactive,
		now:         time.Now,
	}
}

func (t *QuestionTool) Name() string { return "Question" }

func (t *QuestionTool) Description() string {
	return "Ask the person you are working for a question and wait for the answer. " +
		"Put every question you have in one call: each separate call costs a full replay of " +
		"the conversation and another round of thinking. Ask when the answer changes what you " +
		"build and the code cannot tell you; do not ask what reading a file would answer, and " +
		"do not ask permission to do the thing you were asked to do."
}

func (t *QuestionTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"questions": map[string]any{
				"type":        "array",
				"description": "The questions to ask, in the order they should be put.",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"question": map[string]any{
							"type":        "string",
							"description": "The question, as one sentence.",
						},
						"header": map[string]any{
							"type":        "string",
							"description": "A few words naming what is being decided, shown above the question.",
						},
						"options": map[string]any{
							"type":        "array",
							"description": "The choices you see. Enumerating them is what makes the question answerable; a question with no options is usually one that has not been thought through. A free-text answer is always possible regardless.",
							"items": map[string]any{
								"type": "object",
								"properties": map[string]any{
									"label":       map[string]any{"type": "string", "description": "The choice, in a few words."},
									"description": map[string]any{"type": "string", "description": "What choosing it means."},
								},
								"required": []string{"label"},
							},
						},
						"multi": map[string]any{
							"type":        "boolean",
							"description": "True when more than one option may be chosen.",
						},
					},
					"required": []string{"question"},
				},
			},
		},
		"required": []string{"questions"},
	}
}

func (t *QuestionTool) PromptCategory() string     { return "shell" }
func (t *QuestionTool) PromptOrder() int           { return 20 }
func (t *QuestionTool) PromptTemplateName() string { return "" }
func (t *QuestionTool) PromptSection() string {
	return `### Question - Ask the person you are working for

Question({"questions": [{"question": "Reuse the existing retry wrapper, or write a new one?",
  "header": "retry strategy",
  "options": [{"label": "Reuse internal/http.Retry", "description": "same backoff, already tested"},
              {"label": "New wrapper in this package", "description": "no shared state"}]}]})

The call blocks until it is answered, and the answer comes back as the tool
result. Put every question you have in one call. Ask when the answer changes
what you build and the code cannot tell you; do not ask what reading would
answer, and do not ask permission to do what you were asked to do. When no one
answers you are told so, and are expected to proceed on your own judgement and
say what you assumed.`
}

// SelfTimeout opts out of the loop's blanket 15-second tool timeout: waiting
// for a person is the whole point of this tool.
func (t *QuestionTool) SelfTimeout() bool { return true }

type questionOption struct {
	Label       string `json:"label"`
	Description string `json:"description,omitempty"`
}

type questionSpec struct {
	Question string           `json:"question"`
	Header   string           `json:"header,omitempty"`
	Options  []questionOption `json:"options,omitempty"`
	Multi    bool             `json:"multi,omitempty"`
}

type questionArgs struct {
	Questions []questionSpec `json:"questions"`
}

// QuestionAnswer is one question and what came back.
type QuestionAnswer struct {
	Question string `json:"question"`
	// Answers holds the labels of the options that were chosen.
	Answers []string `json:"answers,omitempty"`
	// FreeText is what was typed when it was not an option number.
	FreeText string `json:"free_text,omitempty"`
	// Dismissed is set when the person ended the turn instead of answering.
	Dismissed bool `json:"dismissed,omitempty"`
	// Unanswered is set when nobody answered within the time allowed.
	Unanswered bool `json:"unanswered,omitempty"`
	// Declined is set when the person chose not to answer.
	Declined bool `json:"declined,omitempty"`
}

// unansweredNote is what the model is told when nobody answered. It says what
// to do next, because otherwise the model asks the same question again.
const unansweredNote = "No one answered. Use your judgement, proceed, and state what you assumed."

// declinedNote is what the model is told when the person chose not to answer.
const declinedNote = "The person chose not to answer. Use your judgement, proceed, and state what you assumed."

func (t *QuestionTool) Check(ctx context.Context, args json.RawMessage) error {
	var parsed questionArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return SemanticErrorf("Question: arguments are not valid JSON: %v", err)
	}
	if len(parsed.Questions) == 0 {
		return SemanticErrorf("Question: 'questions' is empty. Ask at least one question, or do not call this tool.")
	}
	for i, q := range parsed.Questions {
		if strings.TrimSpace(q.Question) == "" {
			return SemanticErrorf("Question: question %d has no text.", i+1)
		}
		// Asking again, word for word, after it was dismissed is the one thing
		// this tool must refuse: the person said no by ending the turn, and
		// repeating it turns a decision into a loop.
		if t.toolCtx != nil && t.toolCtx.WasQuestionDismissed(q.Question) {
			return SemanticErrorf(
				"Question: %q was already asked and dismissed. Do not ask it again — "+
					"proceed on your own judgement and say what you assumed.", q.Question)
		}
	}
	return nil
}

func (t *QuestionTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var parsed questionArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		return nil, SemanticErrorf("Question: arguments are not valid JSON: %v", err)
	}

	// A program driving this process shows every question of the call as
	// one form.
	if ask := t.toolCtx.FormAsker(); ask != nil {
		return t.askForm(ctx, ask, parsed.Questions)
	}

	box := t.toolCtx.Inbox()
	answers := make([]QuestionAnswer, 0, len(parsed.Questions))

	for _, spec := range parsed.Questions {
		answer, err := t.ask(ctx, box, spec)
		if err != nil {
			return nil, err
		}
		answers = append(answers, answer)
		if answer.Dismissed {
			// The turn is ending; do not put the remaining questions to
			// someone who is no longer there.
			for _, rest := range parsed.Questions[len(answers):] {
				t.toolCtx.RecordDismissedQuestion(rest.Question)
				answers = append(answers, QuestionAnswer{Question: rest.Question, Dismissed: true})
			}
			break
		}
	}

	result := map[string]any{"answers": answers}
	for _, a := range answers {
		if a.Unanswered {
			result["note"] = unansweredNote
			break
		}
	}
	return result, nil
}

// askForm puts the call's questions to the program driving this process as
// one form, and waits as long as it takes: someone is there to answer, or the
// program says the turn is being stopped.
func (t *QuestionTool) askForm(ctx context.Context, ask FormAsker, specs []questionSpec) (any, error) {
	questions := make([]FormQuestion, len(specs))
	for i, spec := range specs {
		q := FormQuestion{Question: spec.Question, Header: spec.Header, Multi: spec.Multi}
		for _, opt := range spec.Options {
			q.Options = append(q.Options, FormOption(opt))
		}
		questions[i] = q
	}
	started := time.Now()
	reply, err := ask(ctx, questions)
	t.toolCtx.AddPromptWait(time.Since(started))
	if err != nil && ctx.Err() == nil {
		reply = FormReply{}
	} else if err != nil {
		reply = FormReply{Action: "cancel"}
	}

	answers := make([]QuestionAnswer, len(specs))
	result := map[string]any{}
	for i, spec := range specs {
		answer := QuestionAnswer{Question: spec.Question}
		switch reply.Action {
		case "accept":
			if i < len(reply.Chosen) {
				answer.Answers = reply.Chosen[i]
			}
			if i < len(reply.Typed) {
				answer.FreeText = strings.TrimSpace(reply.Typed[i])
			}
			if len(answer.Answers) == 0 && answer.FreeText == "" {
				answer.Unanswered = true
				result["note"] = unansweredNote
			}
		case "decline":
			answer.Declined = true
			result["note"] = declinedNote
		case "cancel":
			// The turn is being stopped, as ctrl-c at the question stops it.
			answer.Dismissed = true
			t.toolCtx.RecordDismissedQuestion(spec.Question)
		default:
			answer.Unanswered = true
			result["note"] = unansweredNote
		}
		answers[i] = answer
	}
	result["answers"] = answers
	return result, nil
}

// ask puts one question and waits for the line that answers it.
func (t *QuestionTool) ask(ctx context.Context, box *inbox.Inbox, spec questionSpec) (QuestionAnswer, error) {
	answer := QuestionAnswer{Question: spec.Question}

	if box == nil {
		t.render(spec)
		answer.Unanswered = true
		t.say(unansweredNote)
		return answer, nil
	}

	// With a person at the terminal, wait as long as it takes: a timeout
	// firing while you are away is worse than waiting. With no terminal, wait
	// tools.question.timeout — zero by default, so a scripted or benchmark run
	// falls back at once rather than hanging.
	var wait time.Duration
	if !t.interactive() {
		wait = time.Duration(t.cfg.Tools.Question.Timeout) * time.Second
		if wait <= 0 {
			t.render(spec)
			answer.Unanswered = true
			t.say(unansweredNote)
			return answer, nil
		}
	}

	// Ask through the inbox, which is the process's only reader of the
	// terminal. A line typed before the question was drawn was not answering
	// it, and Ask hands those back to the loop as ordinary steering.
	text, outcome := box.Ask(ctx, t.out, t.renderText(spec), wait)
	switch outcome {
	case inbox.AskAnswered:
		t.fill(&answer, spec, text)
	case inbox.AskCancelled:
		// Ctrl-C is dismissal. Recording it is what stops the model asking
		// the same thing on the next turn.
		answer.Dismissed = true
		t.toolCtx.RecordDismissedQuestion(spec.Question)
		t.say("question dismissed")
	default:
		answer.Unanswered = true
		t.say(unansweredNote)
	}
	return answer, nil
}

// fill turns what was typed into an answer. A bare number picks an option,
// "1,3" picks several, and anything else is free text — so you never have to
// decide which you are doing before you type.
func (t *QuestionTool) fill(answer *QuestionAnswer, spec questionSpec, text string) {
	text = strings.TrimSpace(text)
	if labels, ok := selectOptions(spec, text); ok {
		answer.Answers = labels
		return
	}
	answer.FreeText = text
}

func selectOptions(spec questionSpec, text string) ([]string, bool) {
	if len(spec.Options) == 0 || text == "" {
		return nil, false
	}
	fields := strings.FieldsFunc(text, func(r rune) bool { return r == ',' || r == ' ' })
	if len(fields) == 0 {
		return nil, false
	}
	var labels []string
	seen := map[int]bool{}
	for _, f := range fields {
		n, err := strconv.Atoi(strings.TrimSpace(f))
		if err != nil || n < 1 || n > len(spec.Options) {
			return nil, false
		}
		if seen[n] {
			continue
		}
		seen[n] = true
		labels = append(labels, spec.Options[n-1].Label)
	}
	if len(labels) > 1 && !spec.Multi {
		// Several numbers for a single-choice question: the first is the answer.
		labels = labels[:1]
	}
	return labels, true
}

// render draws the question above the input line.
func (t *QuestionTool) render(spec questionSpec) {
	fmt.Fprint(t.out, t.renderText(spec))
}

// questionHeaderCode paints the header blue on a gray background so the
// waiting prompt stands out from the regular white text around it. It mirrors
// fatih/color's FgBlue+BgWhite, emitted manually so a headless run whose
// stderr is a terminal still gets colors even when stdout is piped and that
// package has switched itself off (see internal/ui).
const questionHeaderCode = "\x1b[34;47m"

const questionResetCode = "\x1b[0m"

// useColor reports whether the question header should carry ANSI colors.
// It respects NO_COLOR and a dumb terminal, and otherwise colors when the
// output looks like a terminal. A non-file writer (a test buffer, say) is
// treated as color-capable so the marker stays testable; a file that is not
// a character device (piped logs) stays plain.
func (t *QuestionTool) useColor() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	if os.Getenv("TERM") == "dumb" {
		return false
	}
	if t == nil || t.out == nil {
		return true
	}
	if f, ok := t.out.(*os.File); ok {
		if info, err := f.Stat(); err == nil {
			return info.Mode()&os.ModeCharDevice != 0
		}
		return false
	}
	return true
}

// paintHeader wraps s in the header colors when colors are on.
func (t *QuestionTool) paintHeader(s string) string {
	if !t.useColor() {
		return s
	}
	return questionHeaderCode + s + questionResetCode
}

// renderText is the question as it appears above the input line. The first
// visible character is always '?', and the header line (or the '?' marker
// when there is no header) is blue on gray.
func (t *QuestionTool) renderText(spec questionSpec) string {
	var sb strings.Builder
	sb.WriteString("\n")
	if spec.Header != "" {
		sb.WriteString(t.paintHeader("? ── "+spec.Header+" ──") + "\n")
		sb.WriteString(spec.Question + "\n")
	} else {
		sb.WriteString(t.paintHeader("?") + " " + spec.Question + "\n")
	}
	for i, opt := range spec.Options {
		sb.WriteString(fmt.Sprintf("  %d) %s", i+1, opt.Label))
		if opt.Description != "" {
			sb.WriteString("  — " + opt.Description)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("  " + inputHint(spec) + "\n")
	return sb.String()
}

// inputHint says which mode the input line is in.
func inputHint(spec questionSpec) string {
	switch n := len(spec.Options); {
	case n == 0:
		return "[type your answer]"
	case n == 1:
		return "[1, or type]"
	default:
		return fmt.Sprintf("[1-%d, or type]", n)
	}
}

func (t *QuestionTool) say(msg string) {
	fmt.Fprintf(t.out, "  %s\n", msg)
}

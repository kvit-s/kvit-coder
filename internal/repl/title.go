package repl

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// titleTimeout bounds the summarizer call end to end. A title must never
// stall a turn: any failure — timeout included — falls back to the prompt's
// first words.
//
// Nothing is printed while the call runs, and it runs before the turn's first
// model call, so every second of it is a new session sitting silent at what
// looks like a dead prompt. Eight seconds is long enough for a small model to
// answer with three words and short enough that giving up and using the
// prompt's own first words barely registers.
const titleTimeout = 8 * time.Second

// titleMaxTokens caps the summarizer answer. A 3-6 word title fits in a
// fraction of it; the cap only trims a model that answers in sentences.
const titleMaxTokens = 32

// titlePromptChars caps how much of the prompt reaches the summarizer. A
// pasted file or stack trace titles no better for its tail, and sending it
// all would make the cheapest call of the session cost like a real one.
const titlePromptChars = 2000

// EnsureSessionTitle sets the session's display title once, at the start of
// a session, and reports what it set ("" when there was nothing to title).
// An existing title is left alone, so this is a no-op after the first turn.
//
// The title is a 3-6 word summary of the first prompt from the
// `summarizer: true` models: entry (see Config.SummarizerEntry). When no
// entry is flagged, or the call fails or returns nothing usable, the title
// falls back to the prompt's first words.
//
// It sets meta.Title but does not save: the RunExec caller saves meta.json
// once with FirstPrompt and Title together. Tests can read sess.Meta().Title
// without saving.
func EnsureSessionTitle(ctx context.Context, cfg *config.Config, sess *session.Session, promptText string, haveUserMsg bool, writer *ui.Writer) string {
	if sess == nil || cfg == nil || !haveUserMsg {
		return ""
	}
	meta := sess.Meta()
	if meta.Title != "" {
		return meta.Title
	}
	// Prefer the recorded first prompt so a session that predates titles
	// is backfilled from what started it, not from whatever the next turn
	// happens to ask. New sessions set FirstPrompt to this turn's prompt
	// before calling here, so both point at the same text.
	source := strings.TrimSpace(meta.FirstPrompt)
	if source == "" {
		source = strings.TrimSpace(promptText)
	}
	if source == "" {
		return ""
	}
	title := generateTitle(ctx, cfg, source, writer)
	if title == "" {
		title = session.FirstWordsTitle(source, session.TitleFallbackWords)
	}
	if title = session.NormalizeTitle(title); title == "" {
		return ""
	}
	meta.Title = title
	return title
}

// generateTitle asks the summarizer model for a title. It returns "" when
// no summarizer is configured or the answer is unusable, letting the caller
// fall back — never an error, so titling cannot fail a turn.
func generateTitle(ctx context.Context, cfg *config.Config, prompt string, writer *ui.Writer) string {
	entry, ok := cfg.SummarizerEntry()
	if !ok {
		return ""
	}
	raw, err := summarizerTitle(ctx, cfg, entry, prompt)
	if err != nil {
		debugf(writer, fmt.Sprintf("session title: summarizer %q failed, falling back: %v", entry.ID, err))
		return ""
	}
	if title := session.NormalizeTitle(raw); title != "" {
		return title
	}
	debugf(writer, fmt.Sprintf("session title: summarizer %q returned nothing usable, falling back", entry.ID))
	return ""
}

// summarizerTitle runs one tool-free chat completion against the summarizer
// entry and returns the raw answer text.
func summarizerTitle(ctx context.Context, cfg *config.Config, entry config.ModelEntry, prompt string) (string, error) {
	callCtx, cancel := context.WithTimeout(ctx, titleTimeout)
	client := summarizerClientFor(cfg, entry)
	defer cancel()
	if truncated := truncatePrompt(prompt); truncated != prompt {
		prompt = truncated
	}
	req := llm.ChatRequest{
		Model: entry.Model,
		Messages: []llm.Message{
			{Role: llm.RoleSystem, Content: "You generate short session titles."},
			{Role: llm.RoleUser, Content: "Generate a concise 3-6 word title for the following coding request. " +
				"Return only the title, with no quotes and no trailing period.\n\nRequest:\n" + prompt},
		},
		Temperature: 0,
		MaxTokens:   titleMaxTokens,
		Stream:      false,
	}
	// Answer directly: reasoning would only slow a 3-word answer.
	// Ignored by servers/templates without the key (cf. interrogate).
	req.ChatTemplateKwargs = map[string]any{"enable_thinking": false}
	resp, err := client.Chat(callCtx, req)
	if err != nil {
		return "", err
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no choices in summarizer response")
	}
	msg := resp.Choices[0].Message
	if content := strings.TrimSpace(msg.Content); content != "" {
		return content, nil
	}
	if content := strings.TrimSpace(msg.ReasoningContent); content != "" {
		return content, nil
	}
	return "", fmt.Errorf("empty summarizer response")
}

// summarizerClientFor builds the lightweight client the title call runs on.
// Endpoint fields come from the entry, falling back to the `llm:` block when
// the entry leaves them empty. Headers are the `llm:` block's with the
// entry's own laid over them, which is what ApplyModel would send for it;
// anything else (timeouts) always comes from `llm:`.
func summarizerClientFor(cfg *config.Config, entry config.ModelEntry) *llm.Client {
	baseURL := entry.BaseURL
	if baseURL == "" {
		baseURL = cfg.LLM.BaseURL
	}
	apiKey := config.EntryAPIKey(entry)
	if apiKey == "" {
		apiKey = cfg.LLM.APIKey
	}
	backend := entry.APIBackend
	if backend == "" {
		backend = cfg.LLM.APIBackend
	}
	effortField := entry.EffortField
	if effortField == "" {
		effortField = cfg.LLM.EffortField
	}
	effort := summarizerEffort(cfg, entry)

	return llm.NewClient(
		baseURL,
		apiKey,
		llm.WithBackend(backend),
		llm.WithHeaders(cfg.HeadersFor(entry)),
		llm.WithReasoningEffort(effort),
		llm.WithEffortField(effortField),
		llm.WithTimeout(titleTimeout),
		// One attempt. The default ladder retries a 5xx ten times, waiting
		// 1s, 2s, 4s, 8s, 16s and so on in between, which is how a local
		// endpoint answering 502 in three milliseconds — llama-swap with no
		// model loaded — still held the start of every new session for the
		// full titleTimeout with nothing on screen. The title is worth one
		// try; the fallback is the prompt's own first words.
		llm.WithMaxRetries(0),
	)
}

// summarizerEffort picks the cheapest thinking the entry offers for a
// 3-word answer: the earliest CanonicalEfforts value on its menu ("none"
// first — a title needs no reasoning), or "" for a non-reasoning model.
// It never takes the entry default: that default serves real turns, where
// thinking pays, and would make the title the most expensive few words of
// the session.
func summarizerEffort(cfg *config.Config, entry config.ModelEntry) string {
	menu := cfg.EffortOptions(entry)
	if len(menu) == 0 {
		return ""
	}
	offered := make(map[string]bool, len(menu))
	for _, o := range menu {
		offered[strings.ToLower(o.Value)] = true
	}
	for _, v := range config.CanonicalEfforts {
		if offered[v] {
			return v
		}
	}
	return cfg.DefaultEffort(entry)
}

// truncatePrompt caps the prompt text sent to the summarizer.
func truncatePrompt(prompt string) string {
	if len(prompt) <= titlePromptChars {
		return prompt
	}
	cut := strings.LastIndex(prompt[:titlePromptChars], " ")
	if cut <= 0 {
		return prompt[:titlePromptChars]
	}
	return prompt[:cut]
}

func debugf(writer *ui.Writer, msg string) {
	if writer != nil {
		writer.Debug(msg)
	}
}

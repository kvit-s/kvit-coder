// Package repl provides the exec mode runner for the agent.
package repl

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/agent"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/procs"
	"github.com/kvit-s/kvit-coder/internal/session"
	"github.com/kvit-s/kvit-coder/internal/ui"
)

// RunExec runs one turn: load the session's conversation, put the prompt to
// the model, and append what the turn produces to the session's history.
//
// History is appended to rather than rewritten, and written as the turn goes
// rather than at the end of it: the prompt up front, then the model's turn and
// each tool result as each happens. So a crash costs one message rather than a
// turn, and "tail -f" on history.jsonl follows a turn while it runs. When the
// loop backtracks over messages that are already written, the discard is
// recorded as its own line, which keeps the file append-only and leaves the
// abandoned attempt visible.
// RunExec returns the error that ended the turn, or nil when the turn
// finished. The caller turns that into the process's exit status: a turn whose
// model call failed used to exit 0, which left a wrapper or a CI job unable to
// tell a failed turn from a successful one.
func RunExec(ctx context.Context, runner *agent.Runner, writer *ui.Writer, cfg *config.Config, systemPrompt string, promptText string, quietMode bool, sess *session.Session, projectInstructions *ProjectInstructions, imagePaths []string) error {
	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: systemPrompt},
	}

	sessionName := ""
	isNewSession := true
	if sess != nil {
		sessionName = sess.Name()
		previous, err := sess.Load()
		if err != nil {
			writer.Error(fmt.Sprintf("cannot load session: %v", err))
		}
		// The system prompt is regenerated each turn (tools and config may have
		// changed), so any system message in the record is dropped.
		for _, msg := range previous {
			if msg.Role != llm.RoleSystem {
				messages = append(messages, msg)
			}
		}
		// A mid-session model switch must not replay one model's opaque
		// Responses state to another: encrypted thinking blocks and item ids
		// belong to the model that produced them, and at best waste tokens,
		// at worst fail the request. The readable text stays.
		// (File-loaded history currently carries neither — both fields are
		// json:"-" — so this is a guard rather than a live path.)
		if lastModel, lerr := sess.LastSettingsModel(); lerr == nil && lastModel != "" &&
			config.WireID(lastModel) != cfg.LLM.Model {
			stripReplayState(messages)
		}
		isNewSession = len(messages) == 1
		// The UI already announced the session and echoed the prompt, so a
		// child it spawned stays quiet about both rather than printing them
		// a second time.
		if !quietMode && !session.FromUI() {
			if isNewSession {
				fmt.Fprintf(os.Stderr, "Starting new session: %s\n\n", sessionName)
			} else if title := strings.TrimSpace(sess.Meta().Title); title != "" {
				fmt.Fprintf(os.Stderr, "Continuing session: %s — %s (%d messages)\n\n", sessionName, title, len(previous))
			} else {
				fmt.Fprintf(os.Stderr, "Continuing session: %s (%d messages)\n\n", sessionName, len(previous))
			}

		}
	}

	// Display the prompt (unless in quiet mode, or spawned by the UI which
	// already echoed what was typed). A wake turn carries no prompt: the
	// inbox is what it is for.
	wake := isWakeTurn(promptText)
	if !wake && !quietMode && !session.FromUI() {
		colorStart := "\033[97;100m"
		colorEnd := "\033[0m"
		inputLines := strings.Split(promptText, "\n")
		for _, line := range inputLines {
			fmt.Fprintf(os.Stderr, "%s%s%s\n", colorStart, line, colorEnd)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Add user message — unless this is a wake turn, where the pending
	// inbox is the prompt and an empty message would only pollute history.
	// Staged images still ride along: a wake fired with an empty composer
	// but pre-staged :image attachments must not drop them.
	var userMsg llm.Message
	haveUserMsg := false
	if !wake || len(imagePaths) > 0 {
		userMsg = llm.Message{
			Role:    llm.RoleUser,
			Content: promptWithProjectInstructions(promptText, projectInstructions),
		}
		userMsg.Images = preparePromptImages(imagePaths, cfg, sess, writer, quietMode)
		messages = append(messages, userMsg)
		haveUserMsg = true
	}

	if sess != nil {
		meta := sess.Meta()
		meta.Workspace = cfg.Workspace.Root
		// The resolved selection, effort included, so reopening the session
		// lands on the model that produced it and the transcript says what
		// did.
		meta.Model = cfg.ModelDisplay()
		if meta.FirstPrompt == "" && haveUserMsg {
			meta.FirstPrompt = promptText
		}
		// The title is set once, on the first titled turn: a 3-6 word
		// summary from the summarizer model, or the prompt's first words
		// when none is configured or its call fails. It lands in the same
		// SaveMeta below, so a crashed first turn still leaves a titled
		// session behind.
		if title := EnsureSessionTitle(ctx, cfg, sess, promptText, haveUserMsg, writer); title != "" {
			writer.Debug(fmt.Sprintf("Session title: %s", title))
		}

		if err := sess.SaveMeta(); err != nil {
			writer.Debug(fmt.Sprintf("Failed to write session metadata: %v", err))
		}

		if err := sess.Settings(cfg.ModelDisplay(), cfg.LLM.MergeThinking, runner.ToolNames()); err != nil {
			writer.Debug(fmt.Sprintf("Failed to record session settings: %v", err))
		}

		if haveUserMsg {
			if err := sess.AppendMessages(stripProjectInstructions([]llm.Message{userMsg}, projectInstructions)); err != nil {
				writer.Error(fmt.Sprintf("cannot record prompt: %v", err))
			}
		}
	}

	// Record each message as the loop produces it, and record a discarded
	// attempt as a discard rather than by rewriting what was already written.
	if sess != nil {
		runner.SetPersist(
			func(batch []llm.Message) error {
				return sess.AppendMessages(stripProjectInstructions(batch, projectInstructions))
			},
			sess.Rollback,
		)
		defer runner.SetPersist(nil, nil)
	}

	// Run agent loop
	result, err := runner.Run(ctx, agent.RunConfig{
		Messages:     messages,
		UseFileFirst: false,
		QuietMode:    quietMode,
	})

	// Ephemeral background processes die with the turn, so an abandoned
	// session leaves nothing behind; persistent ones survive for the next
	// turn and are reported below. Cleanup runs even when the loop errored,
	// otherwise the error path would be the leak.
	var turnKilled []string
	var turnPersistent []procs.Info
	if sess != nil {
		turnKilled, turnPersistent = endTurnProcs(sess)
	}

	if err != nil {
		writer.Error(fmt.Sprintf("agent error: %v", err))
		reportPersistent(writer, sess, turnPersistent, quietMode)
		return err
	}

	// The loop has already written its messages; what is left is why it stopped.
	if sess != nil {
		if result.BudgetExhausted {
			_ = sess.Notice("iteration budget reached without a final answer")
		}
		if result.Cancelled {
			_ = sess.Notice("cancelled by user")
		}
		if result.TimedOut {
			_ = sess.Notice("time budget for this run expired")
		}
	}

	// In JSON mode the stats ride along in the JSON document on stdout.
	// Otherwise the turn ends with the report itself: the divider printed
	// before it already carries the timing, so no stats block follows.
	if writer.IsJSONMode() {
		writer.WriteJSONOutput(&ui.JSONStats{
			Session:          sessionName,
			PromptTokens:     result.Stats.TotalPromptTokens,
			CompletionTokens: result.Stats.TotalCompletionTokens,
			TotalTokens:      result.Stats.TotalPromptTokens + result.Stats.TotalCompletionTokens,
			CacheReadTokens:  result.Stats.TotalCacheReadTokens,
			TotalCost:        result.Stats.TotalCost,
			CacheDiscount:    result.Stats.CacheDiscount,
			ContextUsed:      result.Stats.MaxContextUsed,
			ContextWindow:    cfg.LLM.Context,
			DurationMs:       result.Stats.TotalAgentTime.Milliseconds(),
			Steps:            result.Stats.Steps,
		})
	}
	// Print session info (skip in JSON mode - it's included in the JSON output)
	if !writer.IsJSONMode() && sessionName != "" {
		if quietMode {
			// In quiet mode, print minimal session info to stderr
			if isNewSession {
				fmt.Fprintf(os.Stderr, "[new] %s\n", sessionName)
			} else {
				fmt.Fprintf(os.Stderr, "[continued] %s\n", sessionName)
			}
		} else {
			fmt.Fprintln(os.Stderr)
			fmt.Fprintln(os.Stderr, strings.Repeat("─", 50))
			sessionLabel := sessionName
			if sess != nil {
				if title := strings.TrimSpace(sess.Meta().Title); title != "" {
					sessionLabel += " — " + title
				}
			}
			fmt.Fprintf(os.Stderr, "Session: %s%s\n", sessionLabel, backgroundSuffix(sess))
			// The window is the selected model's, so the share is the same
			// number the per-step status lines were counting toward.
			if used := result.Stats.MaxContextUsed; used > 0 {
				fmt.Fprintf(os.Stderr, "Context: %s\n", ui.FormatContextUsed(used, cfg.LLM.Context))
			}
		}
	}
	reportPersistent(writer, sess, turnPersistent, quietMode)
	if len(turnKilled) > 0 {
		writer.Debug(fmt.Sprintf("Stopped %d ephemeral background process(es) at turn end: %s", len(turnKilled), strings.Join(turnKilled, ", ")))
	}
	// A turn the loop could not finish has already said why on screen. It is
	// returned rather than reprinted, so the caller can exit non-zero.
	return result.Failure
}

// isWakeTurn reports whether this turn carries no user prompt: the pending
// inbox is the prompt. The UI fires such turns when the inbox waits while no
// turn runs (or `kvit-coder --wake` by hand); RunExec then skips the user
// message so history holds no empty prompt.
func isWakeTurn(promptText string) bool {
	return strings.TrimSpace(promptText) == ""
}

// endTurnProcs stops the turn's ephemeral processes and returns what was
// stopped plus the persistent processes still running. Ephemeral die here so
// abandoned sessions leak nothing; persistent survive for the next turn.
func endTurnProcs(sess *session.Session) (killed []string, persistent []procs.Info) {
	reg, err := procs.New(sess.ProcDir())
	if err != nil {
		return nil, nil
	}
	killed = reg.KillEphemeral()
	for _, info := range reg.List() {
		if info.Running() && info.Persistent {
			persistent = append(persistent, info)
		}
	}
	return killed, persistent
}

// reportPersistent lists the persistent processes that survived the turn and
// tells the user how to stop them, on stderr and in the session history so
// the next turn sees it too. Quiet keeps the history note but skips stderr;
// JSON mode skips both (the JSON document owns stdout and stderr stays clean
// for it).
func reportPersistent(writer *ui.Writer, sess *session.Session, persistent []procs.Info, quietMode bool) {
	if len(persistent) == 0 || sess == nil {
		return
	}
	var names []string
	for _, info := range persistent {
		label := info.ID
		if info.Name != "" {
			label = fmt.Sprintf("%s (%s)", info.ID, info.Name)
		} else if info.Command != "" {
			cmd := strings.TrimSpace(info.Command)
			if len(cmd) > 60 {
				cmd = cmd[:57] + "..."
			}
			label = fmt.Sprintf("%s (%s)", info.ID, cmd)
		}
		names = append(names, label)
	}
	sessionName := sess.Name()
	notice := fmt.Sprintf("persistent background processes still running: %s — stop with Shell.kill, or kvit-coder --kill-background %s (all sessions: --kill-background all)",
		strings.Join(names, ", "), sessionName)
	_ = sess.Notice(notice)
	if writer.IsJSONMode() || quietMode {
		return
	}
	fmt.Fprintf(os.Stderr, "Background (persistent, red): %s still running.\n", strings.Join(names, ", "))
	fmt.Fprintf(os.Stderr, "Stop with Shell.kill, or: kvit-coder --kill-background %s  (all sessions: --kill-background all)\n", sessionName)
}

// stripReplayState drops the model-specific replay state from loaded
// messages: encrypted thinking blocks and Responses item ids. The readable
// text (Content, merged thinking) stays.
func stripReplayState(messages []llm.Message) {
	for i := range messages {
		messages[i].ReasoningBlocks = nil
		messages[i].ToolCallItemIDs = nil
	}
}

// backgroundSuffix reports the running background processes as blue " ★N"
// for ephemeral and red " ★N!" for persistent, or "" when none. It reads the
// session's proc registry the same way the next turn will, so the footing
// never claims a process the next turn cannot see. Any error (no session, no
// registry) means no suffix rather than a wrong one. After the turn-end
// cleanup ephemeral should already be gone, so the footing normally shows
// only the red survivors — but a failed stop still shows blue, honestly.
func backgroundSuffix(sess *session.Session) string {
	if sess == nil {
		return ""
	}
	reg, err := procs.New(sess.ProcDir())
	if err != nil {
		return ""
	}
	ephemeral, persistent := reg.RunningCounts()
	var parts []string
	if ephemeral > 0 {
		parts = append(parts, fmt.Sprintf("★%d", ephemeral))
	}
	if persistent > 0 {
		parts = append(parts, fmt.Sprintf("★%d!", persistent))
	}
	if len(parts) == 0 {
		return ""
	}
	return " " + strings.Join(parts, " ")
}

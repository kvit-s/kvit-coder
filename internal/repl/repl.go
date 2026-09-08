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
func RunExec(ctx context.Context, runner *agent.Runner, writer *ui.Writer, cfg *config.Config, systemPrompt string, promptText string, quietMode bool, sess *session.Session, projectInstructions *ProjectInstructions, imagePaths []string) {
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
		isNewSession = len(messages) == 1
		// The UI already announced the session and echoed the prompt, so a
		// child it spawned stays quiet about both rather than printing them
		// a second time.
		if !quietMode && !session.FromUI() {
			if isNewSession {
				fmt.Fprintf(os.Stderr, "Starting new session: %s\n\n", sessionName)
			} else {
				fmt.Fprintf(os.Stderr, "Continuing session: %s (%d messages)\n\n", sessionName, len(previous))
			}

		}
	}

	// Display the prompt (unless in quiet mode, or spawned by the UI which
	// already echoed what was typed).
	if !quietMode && !session.FromUI() {
		colorStart := "\033[97;100m"
		colorEnd := "\033[0m"
		inputLines := strings.Split(promptText, "\n")
		for _, line := range inputLines {
			fmt.Fprintf(os.Stderr, "%s%s%s\n", colorStart, line, colorEnd)
		}
		fmt.Fprintln(os.Stderr)
	}

	// Add user message
	userMsg := llm.Message{
		Role:    llm.RoleUser,
		Content: promptWithProjectInstructions(promptText, projectInstructions),
	}
	userMsg.Images = preparePromptImages(imagePaths, cfg, sess, writer, quietMode)
	messages = append(messages, userMsg)

	if sess != nil {
		meta := sess.Meta()
		meta.Workspace = cfg.Workspace.Root
		meta.Model = cfg.LLM.Model
		if meta.FirstPrompt == "" {
			meta.FirstPrompt = promptText
		}
		if err := sess.SaveMeta(); err != nil {
			writer.Debug(fmt.Sprintf("Failed to write session metadata: %v", err))
		}
		if err := sess.Settings(cfg.LLM.Model, cfg.LLM.MergeThinking, runner.ToolNames()); err != nil {
			writer.Debug(fmt.Sprintf("Failed to record session settings: %v", err))
		}
		if err := sess.AppendMessages(stripProjectInstructions([]llm.Message{userMsg}, projectInstructions)); err != nil {
			writer.Error(fmt.Sprintf("cannot record prompt: %v", err))
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
		return
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
			fmt.Fprintf(os.Stderr, "Session: %s%s\n", sessionName, backgroundSuffix(sess))
		}
	}
	reportPersistent(writer, sess, turnPersistent, quietMode)
	if len(turnKilled) > 0 {
		writer.Debug(fmt.Sprintf("Stopped %d ephemeral background process(es) at turn end: %s", len(turnKilled), strings.Join(turnKilled, ", ")))
	}
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

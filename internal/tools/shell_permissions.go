package tools

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/inbox"
	"github.com/kvit-s/kvit-coder/internal/permissions"
)

// checkPermissions decides whether a command may run, and — when someone is
// there to say so — asks.
//
// A command that is refused outright is refused. A command that merely needs
// permission is where the old blocklist ran out of options: `curl` to a local
// port and `dd` inside a container were walls, because a regular expression can
// only say no. Here it is a question with four answers, so the wall becomes a
// decision you make once.
func (t *ShellAdvancedTool) checkPermissions(command string) error {
	policy := t.policy()
	verdicts, err := policy.Decide(command)
	if err != nil {
		// A command the parser cannot read is a command whose scopes are
		// unknown, and guessing is worse than saying so.
		return fmt.Errorf("could not check this command for safety: %v", err)
	}

	worst, needsAttention := permissions.Worst(verdicts)
	if !needsAttention {
		return nil
	}
	if worst.Effect == permissions.EffectDeny {
		return fmt.Errorf("%s", worst.Reason())
	}

	grantor := t.grantor()
	if grantor == nil || !t.interactive() {
		return fmt.Errorf("%s.\nNo one is here to allow it. Add %q to tools.shell.allowed_commands, "+
			"or run the command yourself, or work another way",
			worst.Reason(), worst.Scope.Pattern())
	}

	granted, err := t.askToGrant(grantor, worst)
	if err != nil {
		return err
	}
	if !granted {
		return fmt.Errorf("%s.\nPermission was refused. Work another way, or say what you need it for",
			worst.Reason())
	}

	// A grant may have covered only the command that was asked about; anything
	// else in the line still has to pass.
	verdicts, err = policy.Decide(command)
	if err != nil {
		return fmt.Errorf("could not check this command for safety: %v", err)
	}
	if again, still := permissions.Worst(verdicts); still {
		if again.Effect == permissions.EffectDeny {
			return fmt.Errorf("%s", again.Reason())
		}
		return t.checkPermissions(command)
	}
	return nil
}

// policy assembles the rules that apply to this run: kvit-coder's own, then the
// config's allow and deny lists, then whatever has been granted.
func (t *ShellAdvancedTool) policy() *permissions.Policy {
	return &permissions.Policy{
		Builtin: permissions.Builtin(permissions.Options{
			AllowInterpreters: t.cfg.Tools.Shell.AllowInterpreters,
			EditToolAvailable: t.cfg.Tools.Edit.Enabled,
		}),
		Config: permissions.FromConfig(
			t.cfg.Tools.Shell.AllowedCommands,
			t.cfg.Tools.Shell.DisallowedCommands,
		),
		Session:       t.grantor().Rules(),
		AllowlistOnly: len(t.cfg.Tools.Shell.AllowedCommands) > 0,
	}
}

func (t *ShellAdvancedTool) grantor() *permissions.Grantor {
	if t.toolCtx == nil {
		return nil
	}
	return t.toolCtx.Grantor()
}

func (t *ShellAdvancedTool) interactive() bool {
	return t.toolCtx != nil && t.toolCtx.Interactive() && t.toolCtx.Inbox() != nil
}

// grantChoices are the four answers, in the order they are offered: narrowest
// first, so the easy key is also the least committal.
var grantChoices = []struct {
	scope permissions.GrantScope
	label string
}{
	{permissions.GrantOnce, "just this once"},
	{permissions.GrantSession, "for the rest of this session"},
	{permissions.GrantProject, "always, for this project"},
	{permissions.GrantGlobal, "always, everywhere"},
}

// askToGrant puts the decision to whoever is at the terminal. It reads the
// answer from the inbox, like every other tool that waits for a person, so
// there is one input path rather than two contending for stdin.
func (t *ShellAdvancedTool) askToGrant(grantor *permissions.Grantor, verdict permissions.Verdict) (bool, error) {
	box := t.toolCtx.Inbox()
	out := io.Writer(os.Stderr)

	var sb strings.Builder
	sb.WriteString("\n── permission ──\n")
	sb.WriteString(verdict.Reason() + "\n")
	sb.WriteString(fmt.Sprintf("Allow %q?\n", verdict.Scope.Pattern()))
	for i, choice := range grantChoices {
		sb.WriteString(fmt.Sprintf("  %d) %s", i+1, choice.label))
		if where := grantor.Where(choice.scope); where != "" {
			sb.WriteString("  — recorded in " + where)
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("  [1-%d, or anything else to refuse]\n", len(grantChoices)))

	// Ask through the inbox, the process's only reader of the terminal. A line
	// typed before the question was drawn was not answering it, and Ask hands
	// those back to the loop as ordinary steering.
	answer, outcome := box.Ask(context.Background(), out, sb.String(), 0)
	if outcome != inbox.AskAnswered {
		fmt.Fprintln(out, "  refused")
		return false, nil
	}
	return t.applyGrant(grantor, verdict, answer, out)
}

func (t *ShellAdvancedTool) applyGrant(grantor *permissions.Grantor, verdict permissions.Verdict, answer string, out io.Writer) (bool, error) {
	for i, choice := range grantChoices {
		if answer != fmt.Sprintf("%d", i+1) {
			continue
		}
		if err := grantor.Grant(choice.scope, verdict.Scope.Pattern()); err != nil {
			fmt.Fprintf(out, "  could not record the grant: %v\n", err)
			return false, nil
		}
		fmt.Fprintf(out, "  allowed %s\n", choice.label)
		return true, nil
	}
	fmt.Fprintln(out, "  refused")
	return false, nil
}

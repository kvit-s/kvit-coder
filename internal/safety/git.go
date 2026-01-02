package safety

import (
	"fmt"
)

// GitRule checks for dangerous git operations
type GitRule struct{}

// Name returns the rule identifier
func (r *GitRule) Name() string {
	return "git"
}

// Applies returns true if this is a git command
func (r *GitRule) Applies(parsed *ParsedCommand) bool {
	return parsed.Binary == "git"
}

// Check evaluates git commands for dangerous operations
func (r *GitRule) Check(parsed *ParsedCommand, ctx *Context) *RuleResult {
	cfg := ctx.Config.Git

	switch parsed.Subcommand {
	case "push":
		return r.checkPush(parsed, cfg)
	case "reset":
		return r.checkReset(parsed, cfg)
	case "checkout":
		return r.checkCheckout(parsed, cfg)
	case "stash":
		return r.checkStash(parsed, cfg)
	case "clean":
		return r.checkClean(parsed, cfg)
	case "branch":
		return r.checkBranch(parsed, cfg)
	}

	return AllowResult()
}

// checkPush blocks git push if configured
func (r *GitRule) checkPush(parsed *ParsedCommand, cfg GitSafetyConfig) *RuleResult {
	if !cfg.BlockPush {
		return AllowResult()
	}

	return BlockResult("git_push",
		"git push should be done by user directly for safety. "+
			"Run the command manually if you're sure.")
}

// checkReset blocks git reset --hard
func (r *GitRule) checkReset(parsed *ParsedCommand, cfg GitSafetyConfig) *RuleResult {
	if !cfg.BlockHardReset {
		return AllowResult()
	}

	if parsed.HasFlag("hard", "-hard", "--hard") {
		return BlockResult("git_reset_hard",
			"git reset --hard destroys uncommitted work. "+
				"Consider using 'git stash' first or run manually if you're sure.")
	}

	return AllowResult()
}

// checkCheckout blocks git checkout -- <file> which discards changes
func (r *GitRule) checkCheckout(parsed *ParsedCommand, cfg GitSafetyConfig) *RuleResult {
	if !cfg.BlockCheckoutDiscard {
		return AllowResult()
	}

	// Check for "git checkout -- <file>" pattern
	if parsed.HasDoubleDash() {
		argsAfter := parsed.ArgsAfterDoubleDash()
		if len(argsAfter) > 0 {
			return BlockResult("git_checkout_discard",
				fmt.Sprintf("git checkout -- %s discards uncommitted changes. "+
					"Use 'git stash' first or run manually if you're sure.", argsAfter[0]))
		}
	}

	return AllowResult()
}

// checkStash blocks stash drop/clear operations
func (r *GitRule) checkStash(parsed *ParsedCommand, cfg GitSafetyConfig) *RuleResult {
	if !cfg.BlockStashDrop {
		return AllowResult()
	}

	// Look for stash subcommand in args
	for _, arg := range parsed.Args {
		if arg == "stash" {
			continue
		}
		switch arg {
		case "drop":
			return BlockResult("git_stash_drop",
				"git stash drop permanently deletes a stash. "+
					"Run manually if you're sure.")
		case "clear":
			return BlockResult("git_stash_clear",
				"git stash clear deletes ALL stashes permanently. "+
					"Run manually if you're sure.")
		}
	}

	return AllowResult()
}

// checkClean blocks git clean -f (but allows --dry-run)
func (r *GitRule) checkClean(parsed *ParsedCommand, cfg GitSafetyConfig) *RuleResult {
	if !cfg.BlockCleanForce {
		return AllowResult()
	}

	hasForce := parsed.HasFlag("f", "force", "-f", "--force")
	hasDryRun := parsed.HasFlag("n", "dry-run", "-n", "--dry-run")

	if hasForce && !hasDryRun {
		return BlockResult("git_clean_force",
			"git clean -f removes untracked files permanently. "+
				"Try 'git clean -n' (dry-run) first or run manually if you're sure.")
	}

	return AllowResult()
}

// checkBranch warns about git branch -D (force delete)
func (r *GitRule) checkBranch(parsed *ParsedCommand, cfg GitSafetyConfig) *RuleResult {
	if !cfg.WarnBranchForceDelete {
		return AllowResult()
	}

	// -D is force delete (unlike -d which checks merge status)
	if parsed.HasFlag("D") {
		return WarnResult("git_branch_force_delete",
			"git branch -D force-deletes branch without checking merge status. "+
				"Use -d for safe delete.")
	}

	return AllowResult()
}

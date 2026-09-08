package permissions

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Effect is what a rule says about a command.
type Effect string

const (
	// EffectAllow lets the command run.
	EffectAllow Effect = "allow"
	// EffectDeny refuses it outright. A denial cannot be granted away.
	EffectDeny Effect = "deny"
	// EffectAsk refuses it unless someone says yes. This is the answer for
	// things that are dangerous in general and ordinary in context — a curl to
	// localhost, a package installed inside a container.
	EffectAsk Effect = "ask"
)

// Rule names a set of commands and says what to do about them.
//
// A pattern is a list of words. Ending it with "*" matches any command whose
// first words are the rest of the pattern, so "curl *" is every curl and
// "python3 -c *" is only the one-liner form. Without a trailing "*" the whole
// command must match word for word.
//
// The program itself may also end in "*", which matches every program whose
// name starts with what comes before it: "mkfs.* *" is mkfs.ext4, mkfs.xfs and
// every other member of that family. Those are separate binaries rather than
// mkfs with an argument, so no pattern over arguments can reach them, and there
// are too many to list. Only the program accepts a partial word; the arguments
// are compared whole.
type Rule struct {
	Pattern string
	Effect  Effect
	Reason  string
	// Source says where the rule came from, for a message that explains
	// itself: "builtin", "config", "session", "project", "global".
	Source string
	// Irreversible says the command destroys something that cannot be got
	// back — a disk overwritten, a filesystem written over what was there.
	// Everything else asked about can be undone or lived with: a fetch can be
	// deleted, a listening port closed. It is the one thing --yolo does not
	// answer for you, because the reason an ask is safe to answer blind is
	// that a wrong answer can be corrected.
	Irreversible bool
}

// Matches reports whether this rule covers the given command.
func (r Rule) Matches(s Scope) bool {
	pattern := strings.Fields(r.Pattern)
	if len(pattern) == 0 {
		return false
	}
	words := append([]string{s.Program}, s.Args...)

	wildcard := pattern[len(pattern)-1] == "*"
	if wildcard {
		pattern = pattern[:len(pattern)-1]
	}
	if len(pattern) == 0 {
		return wildcard
	}
	if len(words) < len(pattern) {
		return false
	}
	if !wildcard && len(words) != len(pattern) {
		return false
	}
	for i, want := range pattern {
		got := words[i]
		if i == 0 {
			// The program is compared without its directory, so /usr/bin/curl
			// and ./curl are the same rule's business.
			got = programName(got)
			want = programName(want)
			if prefix, partial := strings.CutSuffix(want, "*"); partial {
				if !strings.HasPrefix(got, prefix) {
					return false
				}
				continue
			}
		}
		if got != want {
			return false
		}
	}
	return true
}

func programName(word string) string {
	if base := filepath.Base(word); base != "" && base != "." {
		return base
	}
	return word
}

// Verdict is what the policy decided about one command.
type Verdict struct {
	Effect Effect
	Scope  Scope
	Rule   Rule
	// AutoAllowed says this was a command someone would normally be asked
	// about, allowed because AllowAsks is set. The caller says so on the
	// terminal, so a run started with --yolo leaves a record of what it
	// decided rather than only of having been started.
	AutoAllowed bool
}

// Reason explains the verdict in a sentence, naming the command it is about.
func (v Verdict) Reason() string {
	switch v.Effect {
	case EffectDeny:
		return fmt.Sprintf("%q is not allowed: %s", v.Scope.Literal, v.Rule.Reason)
	case EffectAsk:
		return fmt.Sprintf("%q needs permission: %s", v.Scope.Literal, v.Rule.Reason)
	default:
		return fmt.Sprintf("%q is allowed", v.Scope.Literal)
	}
}

// Policy is the rules in the order they are consulted. Denials are looked for
// first across every tier, so a later allow cannot open something an earlier
// deny closed — a session grant for "git *" does not make "sudo git" acceptable.
type Policy struct {
	// Builtin rules are kvit-coder's own and cannot be removed by config.
	Builtin []Rule
	// Config rules come from tools.shell.allowed_commands and
	// disallowed_commands.
	Config []Rule
	// Session rules were granted during this conversation, or in an earlier
	// one for this project or this machine.
	Session []Rule

	// AllowlistOnly makes anything not explicitly allowed a denial. It is set
	// when tools.shell.allowed_commands is non-empty, which is how that option
	// has always behaved.
	AllowlistOnly bool

	// AllowAsks answers yes to every command that would need permission,
	// except the ones marked Irreversible. --yolo sets it. A refusal stays a
	// refusal: this answers questions, and a denial was never a question.
	AllowAsks bool

	// HomeDir is used to recognise a command that would delete the home
	// directory. Empty falls back to the process's own home.
	HomeDir string
}

// Decide parses a command and returns the verdict for every simple command in
// it. A command line is only as safe as its worst segment, so a caller that
// wants one answer takes the strongest verdict — Worst does that.
func (p *Policy) Decide(command string) ([]Verdict, error) {
	scopes, err := Parse(command)
	if err != nil {
		return nil, fmt.Errorf("could not parse the command: %w", err)
	}
	verdicts := make([]Verdict, 0, len(scopes))
	for _, s := range scopes {
		verdicts = append(verdicts, p.decideScope(s))
	}
	return verdicts, nil
}

// Worst returns the verdict that decides the command line: a denial if there is
// one, otherwise the first thing that needs permission, otherwise allowed.
func Worst(verdicts []Verdict) (Verdict, bool) {
	var ask *Verdict
	for i := range verdicts {
		switch verdicts[i].Effect {
		case EffectDeny:
			return verdicts[i], true
		case EffectAsk:
			if ask == nil {
				ask = &verdicts[i]
			}
		}
	}
	if ask != nil {
		return *ask, true
	}
	return Verdict{Effect: EffectAllow}, false
}

func (p *Policy) decideScope(s Scope) Verdict {
	tiers := [][]Rule{p.Builtin, p.Config, p.Session}

	// Denials first, in every tier. Nothing granted later can undo one.
	for _, tier := range tiers {
		for _, rule := range tier {
			if rule.Effect == EffectDeny && rule.Matches(s) {
				return Verdict{Effect: EffectDeny, Scope: s, Rule: rule}
			}
		}
	}
	if rule, ok := p.builtinPathDenial(s); ok {
		return Verdict{Effect: EffectDeny, Scope: s, Rule: rule}
	}

	// Then anything explicitly allowed, nearest tier first: a session grant is
	// the most recent thing anyone said about this command.
	for _, tier := range [][]Rule{p.Session, p.Config, p.Builtin} {
		for _, rule := range tier {
			if rule.Effect == EffectAllow && rule.Matches(s) {
				return Verdict{Effect: EffectAllow, Scope: s, Rule: rule}
			}
		}
	}

	// Then the things that need someone to say yes.
	for _, tier := range tiers {
		for _, rule := range tier {
			if rule.Effect == EffectAsk && rule.Matches(s) {
				if p.AllowAsks && !rule.Irreversible {
					return Verdict{Effect: EffectAllow, Scope: s, Rule: rule, AutoAllowed: true}
				}
				return Verdict{Effect: EffectAsk, Scope: s, Rule: rule}
			}
		}
	}

	if p.AllowlistOnly {
		return Verdict{Effect: EffectDeny, Scope: s, Rule: Rule{
			Pattern: s.Pattern(),
			Effect:  EffectDeny,
			Source:  "config",
			Reason:  "tools.shell.allowed_commands is set and this command is not on it",
		}}
	}
	return Verdict{Effect: EffectAllow, Scope: s}
}

// builtinPathDenial catches the things a pattern cannot say: deleting the
// filesystem root or the home directory itself. Deleting something under them
// is ordinary cleanup and is not caught here.
func (p *Policy) builtinPathDenial(s Scope) (Rule, bool) {
	if s.Program != "rm" {
		return Rule{}, false
	}
	home := p.HomeDir
	if home == "" {
		home, _ = os.UserHomeDir()
	}
	for _, raw := range s.Paths {
		path := strings.TrimSuffix(raw, "/*")
		path = strings.TrimSuffix(path, "/")
		if path == "" {
			path = "/"
		}
		if path == "/" || path == "~" || (home != "" && filepath.Clean(path) == filepath.Clean(home)) {
			return Rule{
				Pattern: "rm " + raw,
				Effect:  EffectDeny,
				Source:  "builtin",
				Reason:  "it would delete your home directory or the filesystem root",
			}, true
		}
	}
	return Rule{}, false
}

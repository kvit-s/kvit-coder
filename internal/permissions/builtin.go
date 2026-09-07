package permissions

// Options are the few things about the surrounding configuration that change
// which builtin rules apply.
type Options struct {
	// AllowInterpreters lifts the rules on `eval` and on interpreter
	// one-liners (python -c, node -e). The benchmark harnesses set it: their
	// tasks are interpreter-driven and run inside an OS sandbox, where a
	// one-liner grants nothing the task did not already have.
	AllowInterpreters bool
	// EditToolAvailable adds the rule against editing a file with sed -i,
	// which only makes sense when there is an Edit tool to use instead.
	EditToolAvailable bool
}

// Builtin returns kvit-coder's own rules. Denials here are things no grant
// should open. The rest are asks: dangerous in general and ordinary in
// context, which is exactly the case a grant exists for.
func Builtin(opts Options) []Rule {
	deny := func(pattern, reason string) Rule {
		return Rule{Pattern: pattern, Effect: EffectDeny, Reason: reason, Source: "builtin"}
	}
	ask := func(pattern, reason string) Rule {
		return Rule{Pattern: pattern, Effect: EffectAsk, Reason: reason, Source: "builtin"}
	}

	rules := []Rule{
		deny("sudo *", "it runs as another user, which puts the whole machine in reach"),
		deny("su *", "it runs as another user, which puts the whole machine in reach"),
		deny("doas *", "it runs as another user, which puts the whole machine in reach"),
		deny("chroot *", "it changes what the filesystem looks like to everything below it"),

		deny("apt *", "it changes the packages installed on this machine"),
		deny("apt-get *", "it changes the packages installed on this machine"),
		deny("yum *", "it changes the packages installed on this machine"),
		deny("dnf *", "it changes the packages installed on this machine"),
		deny("pacman *", "it changes the packages installed on this machine"),
		deny("brew *", "it changes the packages installed on this machine"),

		deny("shutdown *", "it would stop the machine"),
		deny("reboot *", "it would restart the machine"),
		deny("halt *", "it would stop the machine"),
		deny("poweroff *", "it would stop the machine"),

		deny("mkfs *", "it would format a filesystem"),

		ask("dd *", "it writes to a device or a file byte for byte, which can destroy a disk"),

		ask("curl *", "it reaches the network, which can send this project's contents somewhere else"),
		ask("wget *", "it reaches the network, which can send this project's contents somewhere else"),
		ask("nc *", "it opens a network connection, which can send this project's contents somewhere else"),
		ask("ncat *", "it opens a network connection, which can send this project's contents somewhere else"),
		ask("netcat *", "it opens a network connection, which can send this project's contents somewhere else"),
	}

	if !opts.AllowInterpreters {
		rules = append(rules,
			ask("eval *", "it runs text as a command, so what it does cannot be read off the line"),
			ask("eval", "it runs text as a command, so what it does cannot be read off the line"),
			ask("python -c *", "an interpreter one-liner can edit files without going through the Edit tool"),
			ask("python2 -c *", "an interpreter one-liner can edit files without going through the Edit tool"),
			ask("python3 -c *", "an interpreter one-liner can edit files without going through the Edit tool"),
			ask("perl -e *", "an interpreter one-liner can edit files without going through the Edit tool"),
			ask("perl -E *", "an interpreter one-liner can edit files without going through the Edit tool"),
			ask("ruby -e *", "an interpreter one-liner can edit files without going through the Edit tool"),
			ask("node -e *", "an interpreter one-liner can edit files without going through the Edit tool"),
			ask("node --eval *", "an interpreter one-liner can edit files without going through the Edit tool"),
			ask("php -r *", "an interpreter one-liner can edit files without going through the Edit tool"),
		)
	}

	if opts.EditToolAvailable {
		rules = append(rules, deny("sed -i *",
			"it edits a file in place. Use the Edit tool, which shows the change and can undo it"))
	}

	return rules
}

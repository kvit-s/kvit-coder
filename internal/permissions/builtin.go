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
	// Strong says agent.profile is "strong". Rules written to stop a model
	// from working around the Edit tool do not apply to a model that reaches
	// for the right tool on its own, and refusing it only costs iterations.
	Strong bool
}

// The reasons a rule gives are read in two places: by whoever is at the
// terminal deciding whether to allow the command, and by the model when there
// is nobody there and the refusal comes back as the tool's result. Each says
// what the command would do, in one line — a person choosing between four
// answers is not going to read a paragraph, and the category the rule falls
// into is not what they need. The ones used by more than one rule live here so
// the copies cannot drift apart.
const (
	elevateReason = "it runs as root, so nothing checked here still applies"
	packageReason = "it changes the software installed on the whole machine"
	stopReason    = "it would stop the machine, and this session with it"
	mkfsReason    = "it erases the target and writes an empty filesystem over it"
	fetchReason   = "it reaches the network and can send this project's contents out"
	listenReason  = "it opens a network connection to or from this machine"
	opaqueReason  = "it runs code that cannot be read off the command line"
)

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
	// askIrreversibly is for the commands where a wrong answer cannot be taken
	// back. They are the ones --yolo keeps asking about, and with no terminal
	// they are refused rather than run.
	askIrreversibly := func(pattern, reason string) Rule {
		r := ask(pattern, reason)
		r.Irreversible = true
		return r
	}

	rules := []Rule{
		deny("sudo *", elevateReason),
		deny("su *", elevateReason),
		deny("doas *", elevateReason),
		deny("chroot *", "it moves the filesystem root out from under every path check"),

		deny("apt *", packageReason),
		deny("apt-get *", packageReason),
		deny("yum *", packageReason),
		deny("dnf *", packageReason),
		deny("pacman *", packageReason),
		deny("brew *", packageReason),

		deny("shutdown *", stopReason),
		deny("reboot *", stopReason),
		deny("halt *", stopReason),
		deny("poweroff *", stopReason),

		// mkfs is the front end; mkfs.ext4, mkfs.xfs and the rest are separate
		// binaries, and mke2fs is the one in common use that the family name
		// does not cover. These ask rather than refuse: formatting a loop image
		// or a scratch device is ordinary work, and only the target tells you
		// which case you are in.
		askIrreversibly("mkfs *", mkfsReason),
		askIrreversibly("mkfs.* *", mkfsReason),
		askIrreversibly("mke2fs *", mkfsReason),

		askIrreversibly("dd *", "it writes raw bytes to a device and can destroy a disk"),

		ask("curl *", fetchReason),
		ask("wget *", fetchReason),
		ask("nc *", listenReason),
		ask("ncat *", listenReason),
		ask("netcat *", listenReason),
	}

	// `eval` and the interpreter one-liners are asked about only on the weak
	// profile. The reason they were on the list is in the reason they used to
	// give — a one-liner edits files without going through the Edit tool —
	// which is the same concern as the sed -i rule below and the same answer:
	// a model that reaches for the right tool does not need the wrong one shut.
	// AllowInterpreters lifts them for the benchmark harnesses as well, whose
	// tasks are interpreter-driven and which run inside an OS sandbox.
	//
	// This is not a hole that lifting them opens. Anything reachable through
	// `python3 -c` is reachable by writing the same code to a file and running
	// it, which no rule here refuses. The rules catch a command that would do
	// damage by accident; none of them survives a determined route around.
	if !opts.AllowInterpreters && !opts.Strong {
		rules = append(rules,
			ask("eval *", opaqueReason),
			ask("eval", opaqueReason),
			ask("python -c *", opaqueReason),
			ask("python2 -c *", opaqueReason),
			ask("python3 -c *", opaqueReason),
			ask("perl -e *", opaqueReason),
			ask("perl -E *", opaqueReason),
			ask("ruby -e *", opaqueReason),
			ask("node -e *", opaqueReason),
			ask("node --eval *", opaqueReason),
			ask("php -r *", opaqueReason),
		)
	}

	// A local model would answer a whole task in `sed -i` rather than editing,
	// so the weak profile refuses it. Under the strong profile the refusal
	// bought nothing: it caught only this one spelling — `sed --in-place`,
	// `sed -i.bak` and `perl -pi -e` all did the same thing and all passed —
	// while a model that uses sed as a filter, or edits a scratch file outside
	// the workspace, spent iterations working around a deny no grant could
	// lift.
	if opts.EditToolAvailable && !opts.Strong {
		rules = append(rules, deny("sed -i *",
			"it edits a file in place; use the Edit tool instead"))
	}

	return rules
}

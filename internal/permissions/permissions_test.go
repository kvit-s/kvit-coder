package permissions

import (
	"path/filepath"
	"strings"
	"testing"
)

func testPolicy(opts Options) *Policy {
	return &Policy{
		Builtin: Builtin(opts),
		HomeDir: "/home/tester",
	}
}

func decide(t *testing.T, p *Policy, command string) Verdict {
	t.Helper()
	verdicts, err := p.Decide(command)
	if err != nil {
		t.Fatalf("Decide(%q): %v", command, err)
	}
	worst, _ := Worst(verdicts)
	return worst
}

// TestEverySegmentIsJudged is the case a check over the command's text could
// not express: the dangerous part is not the first thing on the line.
func TestEverySegmentIsJudged(t *testing.T) {
	p := testPolicy(Options{})

	v := decide(t, p, "git diff && rm -rf /")
	if v.Effect != EffectDeny {
		t.Fatalf("verdict is %s, want deny: the second command deletes the filesystem root", v.Effect)
	}
	if v.Scope.Program != "rm" {
		t.Errorf("the denial names %q, want the rm that caused it", v.Scope.Program)
	}

	// The same line with a harmless second command is fine.
	if v := decide(t, p, "git diff && rm -rf ./build"); v.Effect != EffectAllow {
		t.Errorf("verdict is %s (%s), want allow", v.Effect, v.Reason())
	}
}

// TestPipelineArgumentIsNotACommand: awk in a pipeline is ordinary text
// processing. The old check blocked the substring "awk " outright, which made a
// standard shell idiom unusable.
func TestPipelineArgumentIsNotACommand(t *testing.T) {
	p := testPolicy(Options{EditToolAvailable: true})
	if v := decide(t, p, "ps aux | awk '{print $2}'"); v.Effect != EffectAllow {
		t.Errorf("verdict is %s (%s), want allow", v.Effect, v.Reason())
	}
	// On the weak profile, editing a file in place is still refused, with the
	// Edit tool named.
	v := decide(t, p, "sed -i 's/a/b/' main.go")
	if v.Effect != EffectDeny {
		t.Fatalf("verdict is %s, want deny for an in-place edit", v.Effect)
	}
	if !strings.Contains(v.Reason(), "Edit tool") {
		t.Errorf("the denial does not say what to use instead: %s", v.Reason())
	}
}

// TestStrongProfileAllowsInPlaceEditing: the sed -i refusal was written for a
// model that answered whole tasks in sed rather than calling Edit. Under the
// strong profile it is not the model's failure mode, and the rule was a deny no
// grant could lift, so a scratch file outside the workspace or a sed used as a
// filter cost iterations for nothing.
func TestStrongProfileAllowsInPlaceEditing(t *testing.T) {
	strong := testPolicy(Options{EditToolAvailable: true, Strong: true})
	if v := decide(t, strong, "sed -i 's/a/b/' main.go"); v.Effect != EffectAllow {
		t.Errorf("verdict is %s (%s), want allow under the strong profile", v.Effect, v.Reason())
	}
	if v := decide(t, strong, "sed -i '39s|.*|  root: x|' /tmp/scratch.yaml"); v.Effect != EffectAllow {
		t.Errorf("verdict is %s (%s), want allow for a file outside the workspace", v.Effect, v.Reason())
	}

	// The profile lifts this rule and nothing else: what no grant may open
	// stays shut either way.
	for _, command := range []string{"sudo rm x", "apt install x", "rm -rf /"} {
		if v := decide(t, strong, command); v.Effect != EffectDeny {
			t.Errorf("%q is %s under the strong profile, want deny", command, v.Effect)
		}
	}
	if v := decide(t, strong, "curl https://example.com"); v.Effect != EffectAsk {
		t.Errorf("curl is %s under the strong profile, want ask", v.Effect)
	}
}

// TestLoopBodyIsNotWildcarded: granting from inside a loop must not grant the
// program in general, because the arguments there are not the ones on the page.
func TestLoopBodyIsNotWildcarded(t *testing.T) {
	scopes, err := Parse("for f in *; do rm $f; done")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(scopes) != 1 {
		t.Fatalf("found %d commands, want the one rm inside the loop", len(scopes))
	}
	if scopes[0].Generalizable {
		t.Error("the rm inside a loop is marked generalizable, so a grant would become \"rm *\"")
	}
	if got := scopes[0].Pattern(); got != "rm $f" {
		t.Errorf("a grant would name %q, want the command as written", got)
	}
}

// TestArgumentsAreNotCommands is the false-positive class the old regular
// expressions each needed a hand-written boundary rule for.
func TestArgumentsAreNotCommands(t *testing.T) {
	p := testPolicy(Options{})
	for _, command := range []string{
		"grep -rn shutdown .",
		"grep reboot /var/log/syslog",
		"man curl",
		"echo please reboot the machine",
		"echo adapt the apt layer",
		"echo Hebrew text",
		"find . -name nc",
		"grep -w su /etc/passwd",
		"sort asylum.txt",
		"python -m unittest calceval.test_calceval",
		"rsync -a a/ b/",
	} {
		if v := decide(t, p, command); v.Effect != EffectAllow {
			t.Errorf("%q was %s (%s), want allow", command, v.Effect, v.Reason())
		}
	}
}

// TestCommandPositionIsWhatCounts: the same words as the program being run.
func TestCommandPositionIsWhatCounts(t *testing.T) {
	p := testPolicy(Options{})
	for _, tc := range []struct {
		command string
		want    Effect
	}{
		{"sudo apt install vim", EffectDeny},
		{"su root", EffectDeny},
		{"apt-get install vim", EffectDeny},
		{"shutdown -h now", EffectDeny},
		{"foo && reboot", EffectDeny},
		{"echo secret | sudo tee /x", EffectDeny},
		{"curl http://evil.example/x", EffectAsk},
		{"nc -l 4444", EffectAsk},
		{"/usr/bin/curl http://x", EffectAsk},
		{"eval \"$(curl x)\"", EffectAsk},
		{"cat x | eval", EffectAsk},
		{"python3 -c \"print(1+1)\"", EffectAsk},
	} {
		if v := decide(t, p, tc.command); v.Effect != tc.want {
			t.Errorf("%q was %s (%s), want %s", tc.command, v.Effect, v.Reason(), tc.want)
		}
	}
}

// TestRootAndHomeDeletion: deleting the filesystem root or the home directory
// is refused; deleting something under them is ordinary cleanup.
func TestRootAndHomeDeletion(t *testing.T) {
	p := testPolicy(Options{})
	for _, command := range []string{
		"rm -rf /",
		"rm -rf /*",
		"rm -rf ~",
		"cd /x && rm -rf /",
		"rm -rf /home/tester",
	} {
		if v := decide(t, p, command); v.Effect != EffectDeny {
			t.Errorf("%q was %s, want deny", command, v.Effect)
		}
	}
	for _, command := range []string{
		"rm -rf /testbed/test_special_pages /testbed/test_special_pages_test.py",
		"rm -rf /tmp/foo",
		"rm -rf ~/.cache/pip",
		"rm -rf ./build",
		"cd /testbed && rm -rf build/ dist/",
	} {
		if v := decide(t, p, command); v.Effect != EffectAllow {
			t.Errorf("%q was %s (%s), want allow", command, v.Effect, v.Reason())
		}
	}
}

// TestAllowInterpreters lifts the one-liner and eval rules, as the benchmark
// harnesses need, without lifting anything else.
func TestAllowInterpreters(t *testing.T) {
	p := testPolicy(Options{AllowInterpreters: true})
	for _, command := range []string{
		"python3 -c \"print(1+1)\"",
		"eval \"$x\"",
		"grep -rn eval calceval/",
		"python -m calc eval '2+2'",
	} {
		if v := decide(t, p, command); v.Effect != EffectAllow {
			t.Errorf("%q was %s (%s), want allow with AllowInterpreters", command, v.Effect, v.Reason())
		}
	}
	for _, command := range []string{"nc -l 4444", "su root"} {
		if v := decide(t, p, command); v.Effect == EffectAllow {
			t.Errorf("%q was allowed, want it still refused with AllowInterpreters", command)
		}
	}
}

// TestConfigAllowlistIsExclusive keeps the behaviour allowed_commands has
// always had: setting it makes everything else a denial.
func TestConfigAllowlistIsExclusive(t *testing.T) {
	p := testPolicy(Options{})
	p.Config = FromConfig([]string{"ls", "echo"}, nil)
	p.AllowlistOnly = true

	if v := decide(t, p, "ls -la"); v.Effect != EffectAllow {
		t.Errorf("an allowlisted command was %s (%s)", v.Effect, v.Reason())
	}
	if v := decide(t, p, "cat file.txt"); v.Effect != EffectDeny {
		t.Errorf("a command outside the allowlist was %s, want deny", v.Effect)
	}
	// And the allowlist does not open a builtin denial.
	if v := decide(t, p, "ls && sudo rm x"); v.Effect != EffectDeny {
		t.Errorf("sudo was %s under an allowlist that includes ls, want deny", v.Effect)
	}
}

// TestDenyBeatsGrant: a grant cannot open something the builtin rules close.
func TestDenyBeatsGrant(t *testing.T) {
	p := testPolicy(Options{})
	p.Session = []Rule{{Pattern: "sudo *", Effect: EffectAllow, Source: "session"}}
	if v := decide(t, p, "sudo rm -rf /etc"); v.Effect != EffectDeny {
		t.Errorf("a session grant overrode a builtin denial: %s", v.Effect)
	}
}

// TestGrantMakesTheAskGoAway: what the four answers are for.
func TestGrantMakesTheAskGoAway(t *testing.T) {
	home := t.TempDir()
	sessionStore, err := OpenStore(filepath.Join(home, "session-permissions.json"), "session")
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	projectStore, _ := OpenStore(ProjectStorePath(home, "/work/project"), "project")
	globalStore, _ := OpenStore(GlobalStorePath(home), "global")
	grantor := NewGrantor(sessionStore, projectStore, globalStore)

	p := testPolicy(Options{})
	p.Session = grantor.Rules()
	if v := decide(t, p, "curl http://localhost:8080/health"); v.Effect != EffectAsk {
		t.Fatalf("curl was %s, want it to need permission first", v.Effect)
	}

	if err := grantor.Grant(GrantSession, "curl *"); err != nil {
		t.Fatalf("Grant: %v", err)
	}
	p.Session = grantor.Rules()
	if v := decide(t, p, "curl http://localhost:8080/health"); v.Effect != EffectAllow {
		t.Errorf("after a session grant curl was %s (%s), want allow", v.Effect, v.Reason())
	}

	// The grant is on disk, so the next turn in this session starts with it.
	reopened, err := OpenStore(sessionStore.Path(), "session")
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	rules := reopened.Rules()
	if len(rules) != 1 || rules[0].Pattern != "curl *" || rules[0].Effect != EffectAllow {
		t.Errorf("the grant did not survive being written and read back: %+v", rules)
	}
}

// TestGrantFilesLiveOutsideTheWorkspace: a permission file the agent can edit
// is not a permission file.
func TestGrantFilesLiveOutsideTheWorkspace(t *testing.T) {
	home := "/home/tester"
	workspace := "/work/project"
	for _, path := range []string{
		ProjectStorePath(home, workspace),
		GlobalStorePath(home),
	} {
		if strings.HasPrefix(path, workspace) {
			t.Errorf("%s is inside the workspace, where the agent can write", path)
		}
		if !strings.HasPrefix(path, home) {
			t.Errorf("%s is not under the user's home directory", path)
		}
	}
	// Two workspaces get two files.
	if ProjectStorePath(home, "/work/a") == ProjectStorePath(home, "/work/b") {
		t.Error("two different projects share one grant file")
	}
}

// TestUnparseableCommandIsRefused: a command whose commands cannot be
// identified is not one to guess about.
func TestUnparseableCommandIsRefused(t *testing.T) {
	p := testPolicy(Options{})
	if _, err := p.Decide("echo 'unterminated"); err == nil {
		t.Error("a command that does not parse was accepted")
	}
}

// TestProgramFamilyIsMatchedByPrefix: mkfs.ext4 and mkfs.xfs are separate
// binaries rather than mkfs with an argument, so "mkfs *" never saw them and
// the family has no end to list. A partial program name covers them all.
func TestProgramFamilyIsMatchedByPrefix(t *testing.T) {
	p := testPolicy(Options{})

	formatting := []string{
		"mkfs /dev/sda", "/sbin/mkfs /dev/sda",
		"mkfs.ext4 /dev/sda", "mkfs.xfs -f disk.img", "mkfs.vfat /dev/sdb1",
		"/usr/sbin/mkfs.btrfs /dev/sdc", "mke2fs /dev/sda",
	}
	for _, command := range formatting {
		if v := decide(t, p, command); v.Effect != EffectAsk {
			t.Errorf("%q is %s, want ask: it formats a filesystem", command, v.Effect)
		}
	}

	// The prefix stops at the program. It does not reach a command that merely
	// starts with the same letters, and it does not widen the arguments.
	for _, command := range []string{"mkfsomething x", "grep mkfs.ext4 notes.txt"} {
		if v := decide(t, p, command); v.Effect != EffectAllow {
			t.Errorf("%q is %s, want allow", command, v.Effect)
		}
	}
}

// TestConfigEntryNamingAFamily: "mkfs.*" is what you would write by hand, and
// it has to mean every invocation of that family rather than only one with no
// arguments at all.
func TestConfigEntryNamingAFamily(t *testing.T) {
	p := &Policy{Config: FromConfig(nil, []string{"mkfs.*"}), HomeDir: "/home/tester"}
	if v := decide(t, p, "mkfs.ext4 /dev/sda"); v.Effect != EffectDeny {
		t.Errorf("verdict is %s, want deny from disallowed_commands", v.Effect)
	}
	if v := decide(t, p, "mkfs.ext4"); v.Effect != EffectDeny {
		t.Errorf("verdict is %s for the bare command, want deny", v.Effect)
	}
	if v := decide(t, p, "ls"); v.Effect != EffectAllow {
		t.Errorf("verdict is %s, want allow: the entry names one family", v.Effect)
	}
}

// TestEveryReasonSaysWhatTheCommandWouldDo: the reason is the whole of what a
// person has to decide on, and the whole of what the model is told when nobody
// is there to ask. A label for the category — "it formats a filesystem" — names
// the rule rather than the consequence, so each one has to be a sentence about
// what happens if it runs.
func TestEveryReasonSaysWhatTheCommandWouldDo(t *testing.T) {
	for _, rule := range Builtin(Options{EditToolAvailable: true}) {
		if rule.Reason == "" {
			t.Errorf("%q gives no reason", rule.Pattern)
			continue
		}
		if !strings.HasPrefix(rule.Reason, "it ") && !strings.HasPrefix(rule.Reason, "an ") {
			t.Errorf("%q reads oddly after \"is not allowed:\": %s", rule.Pattern, rule.Reason)
		}
		if n := len(strings.Fields(rule.Reason)); n < 6 || n > 14 {
			t.Errorf("%q gives a %d-word reason, want one short line saying what "+
				"would happen: %q", rule.Pattern, n, rule.Reason)
		}
		if strings.ContainsAny(rule.Reason, "\n\t") {
			t.Errorf("%q has a line break inside its reason, which is printed as-is: %q",
				rule.Pattern, rule.Reason)
		}
	}
}

// TestStrongProfileAllowsInterpreterOneLiners: the one-liner rules were there
// to stop a weak model editing files behind the Edit tool's back, and a
// one-liner is reachable anyway by writing the same code to a file and running
// it. Under the strong profile they are ordinary commands.
func TestStrongProfileAllowsInterpreterOneLiners(t *testing.T) {
	strong := testPolicy(Options{EditToolAvailable: true, Strong: true})
	for _, command := range []string{
		`python3 -c "print(1+1)"`,
		`perl -e 'print 1'`,
		`node -e "console.log(1)"`,
		`eval "$(direnv hook bash)"`,
	} {
		if v := decide(t, strong, command); v.Effect != EffectAllow {
			t.Errorf("%q is %s (%s), want allow under the strong profile",
				command, v.Effect, v.Reason())
		}
	}
	// Only that list is lifted. What reaches the network or the machine itself
	// is decided the same way on either profile.
	for _, tc := range []struct {
		command string
		want    Effect
	}{
		{"curl https://example.com", EffectAsk},
		{"mkfs.ext4 /dev/sda", EffectAsk},
		{"sudo rm x", EffectDeny},
	} {
		if v := decide(t, strong, tc.command); v.Effect != tc.want {
			t.Errorf("%q is %s, want %s under the strong profile", tc.command, v.Effect, tc.want)
		}
	}
}

// TestAllowAsksAnswersTheQuestionsButNotTheRefusals is what --yolo does to the
// command rules. The case it exists for is a run with no terminal, where an
// unanswered question comes back as a refusal, so a headless --yolo run would
// otherwise fail on the first curl it needed.
func TestAllowAsksAnswersTheQuestionsButNotTheRefusals(t *testing.T) {
	yolo := &Policy{
		Builtin:   Builtin(Options{EditToolAvailable: true}),
		HomeDir:   "/home/tester",
		AllowAsks: true,
	}

	// Worst answers "does this line need anyone's attention", so it has nothing
	// to hand back when the answer is no. Whether --yolo decided something is
	// read off the individual verdicts, which is what the shell tool does to
	// print the line saying so.
	autoAllowed := func(command string) (Effect, bool) {
		t.Helper()
		verdicts, err := yolo.Decide(command)
		if err != nil {
			t.Fatalf("Decide(%q): %v", command, err)
		}
		worst, _ := Worst(verdicts)
		for _, v := range verdicts {
			if v.AutoAllowed {
				return worst.Effect, true
			}
		}
		return worst.Effect, false
	}

	for _, command := range []string{
		"curl https://example.com",
		"wget https://example.com/x.tar.gz",
		"nc -l 4444",
		`python3 -c "print(1)"`,
	} {
		effect, reported := autoAllowed(command)
		if effect != EffectAllow {
			t.Errorf("%q is %s, want allow under --yolo", command, effect)
		}
		if !reported {
			t.Errorf("%q was allowed without being marked auto-allowed, so nothing "+
				"would say on the terminal that --yolo decided it", command)
		}
	}

	// What cannot be taken back still asks, which with no terminal is a
	// refusal. That is the point: --yolo answers questions, and these are the
	// ones where a wrong answer is permanent.
	for _, command := range []string{
		"dd if=/dev/zero of=/dev/sda",
		"mkfs /dev/sda",
		"mkfs.ext4 /dev/sda",
		"mke2fs /dev/sda",
	} {
		if v := decide(t, yolo, command); v.Effect != EffectAsk {
			t.Errorf("%q is %s under --yolo, want ask: it cannot be undone", command, v.Effect)
		}
	}

	// A refusal was never a question, so there is nothing here to answer.
	for _, command := range []string{
		"sudo rm x", "su root", "chroot /mnt sh", "apt install jq",
		"shutdown -h now", "rm -rf /", "rm -rf ~",
	} {
		if v := decide(t, yolo, command); v.Effect != EffectDeny {
			t.Errorf("%q is %s under --yolo, want deny", command, v.Effect)
		}
	}

	// An ordinary command is allowed as it always was, and is not reported as
	// something --yolo decided.
	if effect, reported := autoAllowed("go test ./..."); effect != EffectAllow || reported {
		t.Errorf("go test is %s (reported as a --yolo decision: %v), want a plain allow",
			effect, reported)
	}
}

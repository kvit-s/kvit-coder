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
	// Editing a file in place is still refused, with the Edit tool named.
	v := decide(t, p, "sed -i 's/a/b/' main.go")
	if v.Effect != EffectDeny {
		t.Fatalf("verdict is %s, want deny for an in-place edit", v.Effect)
	}
	if !strings.Contains(v.Reason(), "Edit tool") {
		t.Errorf("the denial does not say what to use instead: %s", v.Reason())
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

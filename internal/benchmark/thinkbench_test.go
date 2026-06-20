package benchmark

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestParseScorecard checks the scorecard parser against full-pass, partial,
// import-fail, and non-JSON grader outputs.
func TestParseScorecard(t *testing.T) {
	cases := []struct {
		name      string
		stdout    string
		wantOK    bool
		wantScore float64
		wantTotal int
	}{
		{
			name:      "full pass",
			stdout:    `{"task":"fix_base62","import_ok":true,"passed":16,"total":16,"score":1.0,"checks":[]}`,
			wantOK:    true,
			wantScore: 1.0,
			wantTotal: 16,
		},
		{
			name:      "partial",
			stdout:    "noise line\n{\"task\":\"t\",\"import_ok\":true,\"passed\":8,\"total\":16,\"score\":0.5,\"checks\":[]}\n",
			wantOK:    true,
			wantScore: 0.5,
			wantTotal: 16,
		},
		{
			name:      "import fail",
			stdout:    `{"task":"t","import_ok":false,"passed":0,"total":12,"score":0.0,"checks":[]}`,
			wantOK:    true,
			wantScore: 0.0,
			wantTotal: 12,
		},
		{
			name:   "non json",
			stdout: "Traceback (most recent call last):\n  ValueError: boom\n",
			wantOK: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			card, err := parseScorecard(tc.stdout)
			if tc.wantOK {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if card.Score != tc.wantScore || card.Total != tc.wantTotal {
					t.Fatalf("got score=%v total=%v, want %v/%v", card.Score, card.Total, tc.wantScore, tc.wantTotal)
				}
			} else if err == nil {
				t.Fatalf("expected parse error, got card %+v", card)
			}
		})
	}
}

// TestFilterThinkbench checks slug/type filtering and observed policy.
func TestFilterThinkbench(t *testing.T) {
	tasks := []TBTask{
		{Slug: "a", Type: TBBugFix},
		{Slug: "b", Type: TBImplement},
		{Slug: "c", Type: TBAmbiguousSpec, Observed: true},
	}

	if got := FilterThinkbench(tasks, nil, nil, false); len(got) != 2 {
		t.Fatalf("default policy: want 2 graded, got %d", len(got))
	}
	if got := FilterThinkbench(tasks, nil, nil, true); len(got) != 3 {
		t.Fatalf("includeObserved: want 3, got %d", len(got))
	}
	if got := FilterThinkbench(tasks, nil, []string{"bug-fix"}, false); len(got) != 1 || got[0].Slug != "a" {
		t.Fatalf("type filter: want [a], got %+v", got)
	}
	// Explicit id selects an observed task despite policy.
	if got := FilterThinkbench(tasks, []string{"c"}, nil, false); len(got) != 1 || got[0].Slug != "c" {
		t.Fatalf("id filter: want [c], got %+v", got)
	}
}

// TestSandboxReadConfinement provisions a uv env and verifies the OS sandbox
// denies reads outside the workspace while allowing reads inside it. Skips when
// uv or bwrap/firejail are unavailable (e.g. CI).
func TestSandboxReadConfinement(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not available")
	}
	if _, err := exec.LookPath("bwrap"); err != nil {
		if _, err := exec.LookPath("firejail"); err != nil {
			t.Skip("no OS sandbox (bwrap/firejail) available")
		}
	}

	tmp := t.TempDir()
	uvDir := filepath.Join(tmp, ".uv")
	workspace := filepath.Join(tmp, "runs", "workspace")
	outside := filepath.Join(tmp, "secret")
	if err := os.MkdirAll(workspace, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0755); err != nil {
		t.Fatal(err)
	}
	secretFile := filepath.Join(outside, "grade.py")
	if err := os.WriteFile(secretFile, []byte(thinkbenchSandboxSentinel+"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	// A file inside the workspace (positive control).
	if err := os.WriteFile(filepath.Join(workspace, "inside.txt"), []byte("VISIBLE_INSIDE\n"), 0644); err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	env, err := provisionUV(ctx, uvDir, "3.11", io_Discard{})
	if err != nil {
		t.Fatalf("provisionUV: %v", err)
	}

	sb, err := resolveSandbox("auto", env.UVDir, workspace, io_Discard{})
	if err != nil {
		t.Fatalf("resolveSandbox: %v", err)
	}
	if sb.Tool == "none" {
		t.Skip("sandbox resolved to none")
	}

	// Read outside the workspace must be denied.
	if err := smokeTestSandbox(ctx, sb, env, workspace, secretFile, io_Discard{}); err != nil {
		t.Fatalf("smoke test (outside read should be denied): %v", err)
	}

	// Positive control: a read INSIDE the workspace must succeed.
	args := make([]string, 0, len(sb.ExecPrefix)+3)
	for _, a := range sb.ExecPrefix {
		args = append(args, strings.ReplaceAll(a, "{workdir}", workspace))
	}
	args = append(args, "sh", "-c", "cat inside.txt")
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = workspace
	cmd.Env = append(os.Environ(), env.InjectEnv...)
	gotOut, err := cmd.CombinedOutput()
	if err != nil || !strings.Contains(string(gotOut), "VISIBLE_INSIDE") {
		t.Fatalf("inside read should succeed: err=%v out=%q", err, string(gotOut))
	}
}

// TestUVInterpreterResolves verifies python3 resolves into the uv env via the
// injected PATH (no outside path named in the command). Skips without uv.
func TestUVInterpreterResolves(t *testing.T) {
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not available")
	}
	tmp := t.TempDir()
	env, err := provisionUV(context.Background(), filepath.Join(tmp, ".uv"), "3.11", io_Discard{})
	if err != nil {
		t.Fatalf("provisionUV: %v", err)
	}
	cmd := exec.Command("sh", "-c", "command -v python3")
	cmd.Env = append(os.Environ(), env.InjectEnv...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("which python3: %v (%s)", err, out)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(out)), env.UVBin) {
		t.Fatalf("python3 resolved to %q, want under %q", strings.TrimSpace(string(out)), env.UVBin)
	}
}

// io_Discard is a tiny io.Writer that drops output, avoiding an import alias.
type io_Discard struct{}

func (io_Discard) Write(p []byte) (int, error) { return len(p), nil }

package copilot

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type fakeRun struct {
	paths map[string]bool
	out   map[string]string
	err   map[string]error
	calls []string
}

func (f *fakeRun) LookPath(name string) (string, error) {
	if f.paths[name] {
		return name, nil
	}
	return "", os.ErrNotExist
}

func (f *fakeRun) Output(_ context.Context, name string, args ...string) ([]byte, error) {
	key := name + " " + strings.Join(args, " ")
	f.calls = append(f.calls, key)
	if err := f.err[key]; err != nil {
		return nil, err
	}
	return []byte(f.out[key]), nil
}

func TestTokenOrder(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".copilot", "config.json"), []byte(`{"oauth_token":"gho_fromfile"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	run := &fakeRun{
		paths: map[string]bool{"gh": true, "secret-tool": true},
		out: map[string]string{
			"secret-tool lookup service copilot-cli": "gho_fromkeychain",
			"gh auth token":                          "gho_fromgh",
		},
	}
	f := Finder{HomeDir: home, Run: run, Getenv: func(k string) string {
		if k == "GITHUB_TOKEN" {
			return "gho_fromenv"
		}
		return ""
	}}
	tok, src, err := f.Token(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "gho_fromenv" || src != "$GITHUB_TOKEN" {
		t.Fatalf("got %s from %s", tok, src)
	}
	if len(run.calls) != 0 {
		t.Fatalf("env token should win before commands run, calls = %v", run.calls)
	}

	f.Getenv = func(string) string { return "" }
	tok, src, err = f.Token(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "gho_fromfile" || !strings.HasSuffix(src, "config.json") {
		t.Fatalf("file token = %s from %s", tok, src)
	}
}

func TestTokenSkipsClassicPATAndWrongHost(t *testing.T) {
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".copilot"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{
	  "last_logged_in_user": "work",
	  "users": {
	    "home": {"github_token": "gho_personal", "host": "github.com"},
	    "work": {"oauth_token": "gho_work", "host": "company.ghe.com"}
	  }
	}`
	if err := os.WriteFile(filepath.Join(home, ".copilot", "config.json"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	run := &fakeRun{paths: map[string]bool{}, out: map[string]string{}}
	f := Finder{
		HomeDir: home,
		Run:     run,
		Getenv: func(k string) string {
			if k == "GH_TOKEN" {
				return "ghp_classic"
			}
			return ""
		},
	}
	tok, _, err := f.Token(context.Background(), "company.ghe.com")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "gho_work" {
		t.Fatalf("token = %s, want the enterprise user", tok)
	}

	tok, _, err = f.Token(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "gho_personal" {
		t.Fatalf("public token = %s, want the github.com user", tok)
	}
}

func TestTokenKeychainThenGH(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("keychain lookup in this test is the Linux secret-tool command")
	}
	run := &fakeRun{
		paths: map[string]bool{"secret-tool": true, "gh": true},
		out: map[string]string{
			"secret-tool lookup service copilot-cli": "not-a-token",
			"secret-tool search service copilot-cli": "secret = gho_searched\n",
		},
	}
	f := Finder{HomeDir: t.TempDir(), Run: run, Getenv: func(string) string { return "" }}
	tok, src, err := f.Token(context.Background(), "")
	if err != nil {
		t.Fatal(err)
	}
	if tok != "gho_searched" || src != "the copilot-cli keychain entry" {
		t.Fatalf("got %s from %s", tok, src)
	}
}

func TestNoTokenExplainsWhereToLook(t *testing.T) {
	f := Finder{HomeDir: t.TempDir(), Run: &fakeRun{}, Getenv: func(string) string { return "" }}
	_, _, err := f.Token(context.Background(), "")
	if err == nil || !strings.Contains(err.Error(), "COPILOT_GITHUB_TOKEN") {
		t.Fatalf("error = %v", err)
	}
}

func TestNormalizeHost(t *testing.T) {
	got, err := NormalizeHost("https://company.ghe.com/")
	if err != nil || got != "company.ghe.com" {
		t.Fatalf("got %q %v", got, err)
	}
	if _, err := NormalizeHost("https://company.ghe.com/extra"); err == nil {
		t.Fatal("path should be rejected")
	}
}

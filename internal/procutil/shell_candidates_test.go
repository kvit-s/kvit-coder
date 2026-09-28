package procutil

import (
	"slices"
	"testing"
)

func TestGitShCandidatesCmdLayout(t *testing.T) {
	got := gitShCandidates(`C:\Program Files\Git\cmd\git.exe`)
	for _, want := range []string{
		`C:\Program Files\Git\usr\bin\sh.exe`,
		`C:\Program Files\Git\bin\sh.exe`,
	} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %q in %q", want, got)
		}
	}
}

func TestGitShCandidatesPerUserLayout(t *testing.T) {
	got := gitShCandidates(`C:\Users\amy\AppData\Local\Programs\Git\cmd\git.exe`)
	if !slices.Contains(got, `C:\Users\amy\AppData\Local\Programs\Git\usr\bin\sh.exe`) {
		t.Errorf("missing per-user sh.exe in %q", got)
	}
}

func TestGitShCandidatesBareName(t *testing.T) {
	if got := gitShCandidates(`git.exe`); len(got) != 0 {
		t.Errorf("bare name yielded %q, want nothing", got)
	}
}

func TestShellSearchBasesOrder(t *testing.T) {
	env := map[string]string{
		"ProgramFiles":      `C:\Program Files`,
		"ProgramFiles(x86)": `C:\Program Files (x86)`,
		"ProgramW6432":      `C:\Program Files`,
		"LocalAppData":      `C:\Users\amy\AppData\Local`,
	}
	got := shellSearchBases(func(k string) string { return env[k] })
	want := []string{
		`C:\Program Files`,
		`C:\Program Files (x86)`,
		`C:\Program Files`,
		`C:\Users\amy\AppData\Local\Programs`,
		`C:\Program Files`,
		`C:\Program Files (x86)`,
	}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShellSearchBasesMissingEnv(t *testing.T) {
	got := shellSearchBases(func(string) string { return "" })
	want := []string{`C:\Program Files`, `C:\Program Files (x86)`}
	if !slices.Equal(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestShellSearchBasesProfileFallback(t *testing.T) {
	got := shellSearchBases(func(k string) string {
		if k == "USERPROFILE" {
			return `C:\Users\amy`
		}
		return ""
	})
	if !slices.Contains(got, `C:\Users\amy\AppData\Local\Programs`) {
		t.Errorf("missing profile-derived base in %q", got)
	}
}

package permissions

import "testing"

// Windows program matching strips .exe/.bat/.cmd/.ps1 and folds case, so
// curl.exe matches a curl rule. Tested via the OS-independent helper.
func TestCanonicalWindowsProgramName(t *testing.T) {
	tests := []struct{ in, want string }{
		{`C:\tools\CURL.EXE`, "curl"},
		{`curl.exe`, "curl"},
		{`C:\Windows\System32\curl.EXE`, "curl"},
		{`npx.cmd`, "npx"},
		{`run.bat`, "run"},
		{`script.ps1`, "script"},
		{`C:\a\b\cURL`, "curl"},
		{`./curl`, "curl"},
		{`/usr/bin/curl`, "curl"},
	}
	for _, tt := range tests {
		if got := canonicalWindowsProgramName(tt.in); got != tt.want {
			t.Errorf("canonicalWindowsProgramName(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

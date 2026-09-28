package safety

import "testing"

// Windows shells must unwrap like bash -c does: a powershell -Command "rm …"
// the rm rules never see is a safety hole. Tested via the OS-independent
// helper so it runs on any OS.
func TestIsWindowsShellBinary(t *testing.T) {
	yes := []string{
		"powershell", "POWERSHELL", "powershell.exe", "Powershell.EXE",
		"pwsh", "pwsh.exe", "cmd", "cmd.exe", "CMD.EXE",
		`C:\Windows\System32\WindowsPowerShell\v1.0\powershell.exe`,
		`C:\Program Files\PowerShell\7\pwsh.exe`,
	}
	for _, b := range yes {
		if !IsWindowsShellBinary(b) {
			t.Errorf("IsWindowsShellBinary(%q) = false, want true", b)
		}
	}
	no := []string{"curl", "curl.exe", "python", "git", "rm"}
	for _, b := range no {
		if IsWindowsShellBinary(b) {
			t.Errorf("IsWindowsShellBinary(%q) = true, want false", b)
		}
	}
}

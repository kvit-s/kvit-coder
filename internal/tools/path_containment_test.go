package tools

import "testing"

// Windows containment cases, exercised via the OS-independent helper so they
// run on any OS. C:\Proj and c:\proj\..\proj are the same directory; a
// case-sensitive prefix/Rel check can be walked around.
func TestPathOutsideWorkspaceWindows(t *testing.T) {
	tests := []struct {
		name      string
		workspace string
		path      string
		outside   bool
	}{
		{"exact match folds case", `C:\Proj`, `c:\proj`, false},
		{"child folds case", `C:\Proj`, `C:\PROJ\src\main.go`, false},
		{"dotdot resolves then folds", `C:\Proj`, `c:\proj\..\proj\src\x.go`, false},
		{"case-variant dotdot escape is outside", `C:\proj`, `C:\proj\..\PROJ2\secret`, true},
		{"parent escape", `C:\proj`, `C:\proj\..\other`, true},
		{"sibling prefix is outside", `C:\proj`, `C:\proj2\file`, true},
		{"sibling case-variant prefix is outside", `C:\proj`, `C:\PROJ2\file`, true},
		{"other volume is outside", `C:\proj`, `D:\proj\file`, true},
		{"volume letter folds case", `C:\proj`, `c:\proj\file`, false},
		{"UNC share mismatch is outside", `\\server\share\proj`, `\\server\other\file`, true},
		{"UNC same share folds case", `\\SERVER\share\proj`, `\\server\SHARE\proj\file`, false},
		{"extended prefix is its own volume", `C:\proj`, `\\?\C:\proj\file`, true},
		{"drive-relative never panics, counts as outside", `C:\proj`, `C:foo`, true},
		{"UNC root without share never panics", `C:\proj`, `\\server`, true},
		{"drive-relative joins workspace", `C:\proj`, `C:\proj\sub\..\file`, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := pathOutsideWorkspaceWindows(tt.workspace, tt.path); got != tt.outside {
				t.Errorf("pathOutsideWorkspaceWindows(%q, %q) = %v, want %v", tt.workspace, tt.path, got, tt.outside)
			}
		})
	}
}

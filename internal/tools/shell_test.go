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
		{"cd '/path with spaces' && ls", "/path"},  // stops at space outside quotes for now
		{"ls -la", ""},                              // no cd
		{"echo cd /tmp", ""},                        // cd not at start
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


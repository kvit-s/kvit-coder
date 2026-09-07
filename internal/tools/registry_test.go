package tools

import (
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestRegistry_EnableAndGet(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	registry := NewRegistry()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)

	registry.Enable(tool)

	retrieved := registry.Get("Shell")
	if retrieved == nil {
		t.Fatal("Expected to retrieve enabled tool")
	}

	if retrieved.Name() != "Shell" {
		t.Errorf("Expected tool name 'Shell', got '%s'", retrieved.Name())
	}
}

func TestRegistry_GetNonExistent(t *testing.T) {
	registry := NewRegistry()

	retrieved := registry.Get("nonexistent")
	if retrieved != nil {
		t.Error("Expected nil for non-existent tool")
	}
}

func TestRegistry_Specs(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	registry := NewRegistry()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)
	registry.Enable(tool)

	specs := registry.Specs()
	if len(specs) != 1 {
		t.Errorf("Expected 1 spec, got %d", len(specs))
	}

	if specs[0].Type != "function" {
		t.Errorf("Expected type 'function', got '%s'", specs[0].Type)
	}

	if specs[0].Function.Name != "Shell" {
		t.Errorf("Expected function name 'Shell', got '%s'", specs[0].Function.Name)
	}
}

func TestRegistry_ModelToolNameAliases(t *testing.T) {
	registry := NewRegistry()
	tool := fakeCategoryTool{name: "Shell.advanced", category: "shell"}
	registry.Enable(tool)

	specs := registry.Specs()
	if len(specs) != 1 {
		t.Fatalf("Expected 1 spec, got %d", len(specs))
	}
	if specs[0].Function.Name != "Shell_advanced" {
		t.Fatalf("Expected provider-safe function name Shell_advanced, got %q", specs[0].Function.Name)
	}
	if registry.Get("Shell_advanced") == nil {
		t.Fatal("Expected provider-facing alias to resolve to internal tool")
	}
	if got := registry.InternalName("Shell_advanced"); got != "Shell.advanced" {
		t.Fatalf("Expected alias to resolve to Shell.advanced, got %q", got)
	}
	if got := registry.ModelName("Shell.advanced"); got != "Shell_advanced" {
		t.Fatalf("Expected internal name to map to Shell_advanced, got %q", got)
	}

	prompt := registry.RewriteToolNamesForPrompt("Use Shell.advanced for timeout options.")
	if prompt != "Use Shell_advanced for timeout options." {
		t.Fatalf("Expected prompt tool name rewrite, got %q", prompt)
	}
}

func TestRegistry_ModelToolNameAliasCollision(t *testing.T) {
	registry := NewRegistry()
	registry.Enable(fakeCategoryTool{name: "mcp.srv.tool", category: "mcp"})
	registry.Enable(fakeCategoryTool{name: "mcp_srv_tool", category: "mcp"})

	if got := registry.ModelName("mcp_srv_tool"); got != "mcp_srv_tool" {
		t.Fatalf("Expected valid internal name to keep exact provider name, got %q", got)
	}
	if got := registry.ModelName("mcp.srv.tool"); got != "mcp_srv_tool_2" {
		t.Fatalf("Expected colliding sanitized alias to get suffix, got %q", got)
	}
}

func TestRegistry_All(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	registry := NewRegistry()
	tool1 := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)
	registry.Enable(tool1)

	all := registry.All()
	if len(all) != 1 {
		t.Errorf("Expected 1 tool, got %d", len(all))
	}
}

func TestRegistry_ExtractToolCallsFromText_AnthropicFormat(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	registry := NewRegistry()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)
	registry.Enable(tool)

	tests := []struct {
		name         string
		content      string
		wantCount    int
		wantToolName string
		wantArgs     map[string]interface{}
	}{
		{
			name:         "basic anthropic format",
			content:      "<function_calls>\n<invoke name=\"Shell\">\n<parameter name=\"command\">ls -la</parameter>\n</invoke>\n</function_calls>",
			wantCount:    1,
			wantToolName: "Shell",
			wantArgs:     map[string]interface{}{"command": "ls -la"},
		},
		{
			name:         "multiple parameters",
			content:      "<function_calls>\n<invoke name=\"Shell\">\n<parameter name=\"command\">echo test</parameter>\n<parameter name=\"timeout\">30</parameter>\n</invoke>\n</function_calls>",
			wantCount:    1,
			wantToolName: "Shell",
			wantArgs:     map[string]interface{}{"command": "echo test", "timeout": float64(30)},
		},
		{
			name:         "multiple invoke blocks",
			content:      "<function_calls>\n<invoke name=\"Shell\">\n<parameter name=\"command\">cmd1</parameter>\n</invoke>\n<invoke name=\"Shell\">\n<parameter name=\"command\">cmd2</parameter>\n</invoke>\n</function_calls>",
			wantCount:    2,
			wantToolName: "Shell",
			wantArgs:     map[string]interface{}{"command": "cmd1"},
		},
		{
			name:         "with surrounding text",
			content:      "Let me run a command:\n<function_calls>\n<invoke name=\"Shell\">\n<parameter name=\"command\">pwd</parameter>\n</invoke>\n</function_calls>\nDone.",
			wantCount:    1,
			wantToolName: "Shell",
			wantArgs:     map[string]interface{}{"command": "pwd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolCalls := registry.ExtractToolCallsFromText(tt.content)

			if len(toolCalls) != tt.wantCount {
				t.Errorf("Expected %d tool calls, got %d", tt.wantCount, len(toolCalls))
				return
			}

			if tt.wantCount > 0 {
				tc := toolCalls[0]
				if tc.Function.Name != tt.wantToolName {
					t.Errorf("Expected tool name %q, got %q", tt.wantToolName, tc.Function.Name)
				}

				if tc.Type != "function" {
					t.Errorf("Expected type 'function', got %q", tc.Type)
				}

				if tc.ID == "" {
					t.Error("Expected non-empty tool call ID")
				}

				// Verify arguments
				var gotArgs map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &gotArgs); err != nil {
					t.Errorf("Failed to unmarshal arguments: %v", err)
					return
				}

				for key, wantVal := range tt.wantArgs {
					gotVal, ok := gotArgs[key]
					if !ok {
						t.Errorf("Missing expected argument %q", key)
						continue
					}
					if gotVal != wantVal {
						t.Errorf("Argument %q: expected %v (%T), got %v (%T)", key, wantVal, wantVal, gotVal, gotVal)
					}
				}
			}
		})
	}
}

func TestRegistry_ExtractToolCallsFromText_JSONInToolCall(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()
	registry := NewRegistry()
	tool := NewShellTool(newTestConfig(), 10*time.Second, tempMgr)
	registry.Enable(tool)

	// Also enable a Search tool if available, or just test with Shell
	tests := []struct {
		name         string
		content      string
		wantCount    int
		wantToolName string
		wantArgs     map[string]interface{}
	}{
		{
			name:         "json in tool_call tags",
			content:      `<tool_call> {"name": "Shell", "arguments": {"command": "ls -la"}}</tool_call>`,
			wantCount:    1,
			wantToolName: "Shell",
			wantArgs:     map[string]interface{}{"command": "ls -la"},
		},
		{
			name:         "json in tool_call with regex pattern",
			content:      `<tool_call> {"name": "Shell", "arguments": {"command": "grep -E 'Config\\s*=\\s*\\w+'"}}</tool_call>`,
			wantCount:    1,
			wantToolName: "Shell",
			wantArgs:     map[string]interface{}{"command": "grep -E 'Config\\s*=\\s*\\w+'"},
		},
		{
			name:         "json in tool_call with surrounding text",
			content:      "Let me search for that:\n<tool_call> {\"name\": \"Shell\", \"arguments\": {\"command\": \"find . -name '*.go'\"}}</tool_call>\nDone.",
			wantCount:    1,
			wantToolName: "Shell",
			wantArgs:     map[string]interface{}{"command": "find . -name '*.go'"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			toolCalls := registry.ExtractToolCallsFromText(tt.content)

			if len(toolCalls) != tt.wantCount {
				t.Errorf("Expected %d tool calls, got %d", tt.wantCount, len(toolCalls))
				return
			}

			if tt.wantCount > 0 {
				tc := toolCalls[0]
				if tc.Function.Name != tt.wantToolName {
					t.Errorf("Expected tool name %q, got %q", tt.wantToolName, tc.Function.Name)
				}

				if tc.Type != "function" {
					t.Errorf("Expected type 'function', got %q", tc.Type)
				}

				// Verify arguments
				var gotArgs map[string]interface{}
				if err := json.Unmarshal([]byte(tc.Function.Arguments), &gotArgs); err != nil {
					t.Errorf("Failed to unmarshal arguments: %v", err)
					return
				}

				for key, wantVal := range tt.wantArgs {
					gotVal, ok := gotArgs[key]
					if !ok {
						t.Errorf("Missing expected argument %q", key)
						continue
					}
					if gotVal != wantVal {
						t.Errorf("Argument %q: expected %v, got %v", key, wantVal, gotVal)
					}
				}
			}
		})
	}
}

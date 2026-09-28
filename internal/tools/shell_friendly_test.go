package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestParseShellArgsTimeoutForms(t *testing.T) {
	cases := []struct {
		name    string
		args    string
		timeout int
	}{
		{"int", `{"command":"echo hi","timeout":120}`, 120},
		{"numeric string", `{"command":"echo hi","timeout":"120"}`, 120},
		{"string with s suffix", `{"command":"echo hi","timeout":"120s"}`, 120},
		{"string with spaces", `{"command":"echo hi","timeout":" 120 "}`, 120},
		{"float", `{"command":"echo hi","timeout":120.0}`, 120},
		{"float string", `{"command":"echo hi","timeout":"120.0"}`, 120},
		{"empty string means default", `{"command":"echo hi","timeout":""}`, 0},
		{"null means default", `{"command":"echo hi","timeout":null}`, 0},
		{"missing means default", `{"command":"echo hi"}`, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := parseShellArgs("Shell", json.RawMessage(tc.args))
			if err != nil {
				t.Fatalf("parseShellArgs(%s) error: %v", tc.args, err)
			}
			if got.Timeout != tc.timeout {
				t.Errorf("timeout = %d, want %d", got.Timeout, tc.timeout)
			}
			if got.Command != "echo hi" {
				t.Errorf("command = %q, want %q", got.Command, "echo hi")
			}
		})
	}
}

func TestParseShellArgsBareString(t *testing.T) {
	got, err := parseShellArgs("Shell", json.RawMessage(`"go vet ./..."`))
	if err != nil {
		t.Fatalf("bare string error: %v", err)
	}
	if got.Command != "go vet ./..." {
		t.Errorf("command = %q", got.Command)
	}
}

func TestParseShellArgsFriendlyErrors(t *testing.T) {
	cases := []struct {
		name     string
		args     string
		contains []string
	}{
		{
			"string timeout that is not a number",
			`{"command":"echo hi","timeout":"soon"}`,
			[]string{`"timeout"`, "seconds", "Example:"},
		},
		{
			"missing command",
			`{"timeout":120}`,
			[]string{`"command"`, "required", "Example:"},
		},
		{
			"empty object",
			`{}`,
			[]string{`"command"`, "Example:"},
		},
		{
			"array instead of object",
			`["echo hi"]`,
			[]string{"JSON object", `"command"`, "Example:"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseShellArgs("Shell", json.RawMessage(tc.args))
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			msg := err.Error()
			if strings.Contains(msg, "cannot unmarshal") || strings.Contains(msg, "Go struct") {
				t.Errorf("error leaks Go internals: %q", msg)
			}
			for _, want := range tc.contains {
				if !strings.Contains(msg, want) {
					t.Errorf("error %q does not contain %q", msg, want)
				}
			}
		})
	}
}

// The reported failure: Shell with a string timeout must not fail with
// "json: cannot unmarshal string into Go struct field .timeout of type int".
func TestShellCallStringTimeout(t *testing.T) {
	tool := &ShellAdvancedTool{workspaceRoot: t.TempDir(), timeout: 30000000000}
	// Bypass config-dependent validation by calling parse directly and via
	// NormalizeToolCallArguments + Call path shapes.
	normalized, err := NormalizeToolCallArguments(tool, json.RawMessage(`{"command":"echo hi","timeout":"60"}`))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(normalized, &decoded); err != nil {
		t.Fatalf("normalized is not valid JSON: %v", err)
	}
	if v, ok := decoded["timeout"].(float64); !ok || int(v) != 60 {
		t.Errorf("normalized timeout = %v (%T), want 60", decoded["timeout"], decoded["timeout"])
	}
	if _, err := parseShellArgs("Shell", normalized); err != nil {
		t.Errorf("parseShellArgs after normalize error: %v", err)
	}
}

func TestShellToolAcceptsTimeoutWithoutRetry(t *testing.T) {
	// Shell's schema must declare timeout/working_dir so the normalizer
	// converts "60" -> 60 for Shell too, not just Shell.advanced.
	tool := &ShellTool{advanced: &ShellAdvancedTool{workspaceRoot: t.TempDir()}}
	schema := tool.JSONSchema()
	props, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatal("Shell schema has no properties")
	}
	for _, field := range []string{"command", "timeout", "working_dir"} {
		if _, ok := props[field]; !ok {
			t.Errorf("Shell schema missing %q; string timeouts would fail to normalize", field)
		}
	}
	normalized, err := NormalizeToolCallArguments(tool, json.RawMessage(`{"command":"echo hi","timeout":"60"}`))
	if err != nil {
		t.Fatalf("normalize error: %v", err)
	}
	if _, err := parseShellArgs("Shell", normalized); err != nil {
		t.Errorf("Shell should accept a string timeout after normalization: %v", err)
	}
	// Even without normalization, the tolerant parser accepts it directly.
	if _, err := parseShellArgs("Shell", json.RawMessage(`{"command":"echo hi","timeout":"60"}`)); err != nil {
		t.Errorf("Shell should accept a string timeout directly: %v", err)
	}
	ctx := context.Background()
	if err := tool.Check(ctx, json.RawMessage(`{"command":"echo hi","timeout":"soon"}`)); err == nil {
		t.Error("expected friendly Check error for bad timeout, got nil")
	} else if strings.Contains(err.Error(), "cannot unmarshal") {
		t.Errorf("Check leaks Go internals: %v", err)
	}
}

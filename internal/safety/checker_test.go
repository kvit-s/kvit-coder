package safety

import (
	"strings"
	"testing"
)

func TestChecker_GitRules(t *testing.T) {
	cfg := &SafetyConfig{
		Git: GitSafetyConfig{
			BlockPush:             true,
			BlockHardReset:        true,
			BlockCheckoutDiscard:  true,
			BlockStashDrop:        true,
			BlockCleanForce:       true,
			WarnBranchForceDelete: true,
		},
	}
	checker := NewChecker(cfg)
	ctx := NewContext(cfg, "/workspace", "/workspace")

	tests := []struct {
		name       string
		cmd        string
		wantAction Action
		wantRule   string
	}{
		{"git push blocked", "git push origin main", Block, "git_push"},
		{"git push -f blocked", "git push -f origin main", Block, "git_push"},
		{"git reset --hard blocked", "git reset --hard HEAD", Block, "git_reset_hard"},
		{"git reset (soft) allowed", "git reset HEAD~1", Allow, ""},
		{"git checkout -- file blocked", "git checkout -- file.txt", Block, "git_checkout_discard"},
		{"git checkout branch allowed", "git checkout main", Allow, ""},
		{"git stash drop blocked", "git stash drop", Block, "git_stash_drop"},
		{"git stash clear blocked", "git stash clear", Block, "git_stash_clear"},
		{"git stash save allowed", "git stash save 'work'", Allow, ""},
		{"git clean -f blocked", "git clean -f", Block, "git_clean_force"},
		{"git clean -n allowed", "git clean -n", Allow, ""},
		{"git clean -fn allowed (dry-run)", "git clean -fn", Allow, ""},
		{"git branch -D warns", "git branch -D feature", Warn, "git_branch_force_delete"},
		{"git branch -d allowed", "git branch -d feature", Allow, ""},
		{"git status allowed", "git status", Allow, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.Check(tt.cmd, ctx)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}

			if result.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v (message: %s)", result.Action, tt.wantAction, result.Message)
			}

			if tt.wantRule != "" && result.Rule != tt.wantRule {
				t.Errorf("Rule = %q, want %q", result.Rule, tt.wantRule)
			}
		})
	}
}

func TestChecker_RmRules(t *testing.T) {
	cfg := &SafetyConfig{
		Rm: RmSafetyConfig{
			AllowInTemp:         true,
			AllowInWorkspaceCwd: true,
			BlockWorkspaceRoot:  true,
		},
	}
	checker := NewChecker(cfg)
	ctx := NewContext(cfg, "/workspace", "/workspace")

	tests := []struct {
		name       string
		cmd        string
		wantAction Action
	}{
		{"rm file allowed", "rm file.txt", Allow},
		{"rm -r dir allowed", "rm -r dir", Allow},
		{"rm -f file allowed", "rm -f file.txt", Allow},
		{"rm -rf / blocked", "rm -rf /", Block},
		{"rm -rf ~ blocked", "rm -rf ~", Block},
		{"rm -rf /usr blocked", "rm -rf /usr", Block},
		{"rm -rf /tmp allowed", "rm -rf /tmp/test", Allow},
		{"rm -rf workspace subdir allowed", "rm -rf subdir", Allow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.Check(tt.cmd, ctx)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}

			if result.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v (rule: %s, message: %s)",
					result.Action, tt.wantAction, result.Rule, result.Message)
			}
		})
	}
}

func TestChecker_XargsRules(t *testing.T) {
	cfg := &SafetyConfig{}
	checker := NewChecker(cfg)
	ctx := NewContext(cfg, "/workspace", "/workspace")

	tests := []struct {
		name       string
		cmd        string
		wantAction Action
	}{
		{"xargs rm blocked", "find . | xargs rm", Block},
		{"xargs rm -rf blocked", "find . | xargs rm -rf", Block},
		{"xargs echo allowed", "find . | xargs echo", Allow},
		{"xargs cat allowed", "find . | xargs cat", Allow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.Check(tt.cmd, ctx)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}

			if result.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v (rule: %s, message: %s)",
					result.Action, tt.wantAction, result.Rule, result.Message)
			}
		})
	}
}

func TestChecker_ShellWrapper(t *testing.T) {
	cfg := &SafetyConfig{
		Git: GitSafetyConfig{
			BlockPush: true,
		},
	}
	checker := NewChecker(cfg)
	ctx := NewContext(cfg, "/workspace", "/workspace")

	tests := []struct {
		name       string
		cmd        string
		wantAction Action
	}{
		{"bash -c git push blocked", "bash -c 'git push'", Block},
		{"sh -c git push blocked", "sh -c 'git push origin main'", Block},
		{"bash -c echo allowed", "bash -c 'echo hello'", Allow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := checker.Check(tt.cmd, ctx)
			if err != nil {
				t.Fatalf("Check() error = %v", err)
			}

			if result.Action != tt.wantAction {
				t.Errorf("Action = %v, want %v (rule: %s)", result.Action, tt.wantAction, result.Rule)
			}
		})
	}
}

func TestChecker_StrictMode(t *testing.T) {
	cfg := &SafetyConfig{
		StrictMode: true,
	}
	checker := NewChecker(cfg)
	ctx := NewContext(cfg, "/workspace", "/workspace")

	// Test that unparseable commands are blocked in strict mode
	result, err := checker.Check("echo 'unterminated", ctx)
	if err != nil {
		t.Fatalf("Check() error = %v", err)
	}

	if result.Action != Block {
		t.Errorf("Expected Block for unparseable command in strict mode, got %v", result.Action)
	}

	if !strings.Contains(result.Rule, "strict") {
		t.Errorf("Expected strict rule, got %q", result.Rule)
	}
}

func TestChecker_Disabled(t *testing.T) {
	// Empty config - all features disabled
	cfg := &SafetyConfig{}
	checker := NewChecker(cfg)

	if checker.IsEnabled() {
		t.Error("Expected checker to be disabled with empty config")
	}

	// Enable one feature
	cfg.Git.BlockPush = true
	checker2 := NewChecker(cfg)

	if !checker2.IsEnabled() {
		t.Error("Expected checker to be enabled with BlockPush=true")
	}
}

func TestRedactor(t *testing.T) {
	redactor := NewRedactor()

	tests := []struct {
		input    string
		contains string // what should be replaced
	}{
		{"API_KEY=sk-abc123def456", "sk-"},
		{"ghp_abcdefghijklmnopqrstuvwxyz1234567890", "ghp_"},
		{"AKIAIOSFODNN7EXAMPLE", "AKIA"},
		{"Authorization: Bearer token123", "Bearer"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := redactor.Redact(tt.input)
			if strings.Contains(result, tt.contains) {
				t.Errorf("Expected %q to be redacted from %q", tt.contains, result)
			}
			if !strings.Contains(result, "[REDACTED]") {
				t.Errorf("Expected [REDACTED] in result: %q", result)
			}
		})
	}
}

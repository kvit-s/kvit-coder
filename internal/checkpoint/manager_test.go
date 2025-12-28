package checkpoint

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewManager(t *testing.T) {
	// Create temp workdir
	workdir, err := os.MkdirTemp("", "checkpoint-test-workdir-")
	if err != nil {
		t.Fatalf("Failed to create temp workdir: %v", err)
	}
	defer os.RemoveAll(workdir)

	mgr, err := NewManager("test-session", workdir, nil, 0)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	if mgr.Workdir() != workdir {
		t.Errorf("Expected workdir %s, got %s", workdir, mgr.Workdir())
	}

	if !mgr.Enabled() {
		t.Error("Expected manager to be enabled by default")
	}

	if mgr.CurrentTurn() != 0 {
		t.Errorf("Expected initial turn 0, got %d", mgr.CurrentTurn())
	}
}

func TestManagerInitialize(t *testing.T) {
	// Create temp workdir
	workdir, err := os.MkdirTemp("", "checkpoint-test-workdir-")
	if err != nil {
		t.Fatalf("Failed to create temp workdir: %v", err)
	}
	defer os.RemoveAll(workdir)

	// Create a test file
	testFile := filepath.Join(workdir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mgr, err := NewManager("test-init", workdir, nil, 0)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Cleanup()

	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Check that checkpoint directory was created
	checkpointDir := filepath.Join(os.TempDir(), "go-coder-checkpoints-test-init")
	if _, err := os.Stat(checkpointDir); os.IsNotExist(err) {
		t.Error("Checkpoint directory was not created")
	}

	// Check that .git was created
	gitDir := filepath.Join(checkpointDir, ".git")
	if _, err := os.Stat(gitDir); os.IsNotExist(err) {
		t.Error("Git directory was not created")
	}
}

func TestManagerTurnLifecycle(t *testing.T) {
	// Create temp workdir
	workdir, err := os.MkdirTemp("", "checkpoint-test-workdir-")
	if err != nil {
		t.Fatalf("Failed to create temp workdir: %v", err)
	}
	defer os.RemoveAll(workdir)

	// Create a test file
	testFile := filepath.Join(workdir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mgr, err := NewManager("test-turns", workdir, nil, 0)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Cleanup()

	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Start turn 1
	turn := mgr.StartTurn()
	if turn != 1 {
		t.Errorf("Expected turn 1, got %d", turn)
	}

	// Make a change
	if err := os.WriteFile(testFile, []byte("turn 1 content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// End turn 1
	if err := mgr.EndTurn(); err != nil {
		t.Fatalf("Failed to end turn: %v", err)
	}

	// Start turn 2
	turn = mgr.StartTurn()
	if turn != 2 {
		t.Errorf("Expected turn 2, got %d", turn)
	}

	// Make another change
	if err := os.WriteFile(testFile, []byte("turn 2 content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// End turn 2
	if err := mgr.EndTurn(); err != nil {
		t.Fatalf("Failed to end turn: %v", err)
	}

	// Verify current turn
	if mgr.CurrentTurn() != 2 {
		t.Errorf("Expected current turn 2, got %d", mgr.CurrentTurn())
	}
}

func TestManagerRestore(t *testing.T) {
	// Create temp workdir
	workdir, err := os.MkdirTemp("", "checkpoint-test-workdir-")
	if err != nil {
		t.Fatalf("Failed to create temp workdir: %v", err)
	}
	defer os.RemoveAll(workdir)

	// Create a test file
	testFile := filepath.Join(workdir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mgr, err := NewManager("test-restore", workdir, nil, 0)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Cleanup()

	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Turn 1: modify file
	mgr.StartTurn()
	if err := os.WriteFile(testFile, []byte("turn 1 content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}
	if err := mgr.EndTurn(); err != nil {
		t.Fatalf("Failed to end turn 1: %v", err)
	}

	// Turn 2: modify file again
	mgr.StartTurn()
	if err := os.WriteFile(testFile, []byte("turn 2 content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}
	if err := mgr.EndTurn(); err != nil {
		t.Fatalf("Failed to end turn 2: %v", err)
	}

	// Verify current content
	content, _ := os.ReadFile(testFile)
	if string(content) != "turn 2 content" {
		t.Errorf("Expected 'turn 2 content', got '%s'", string(content))
	}

	// Restore to turn 1
	changedFiles, err := mgr.Restore(1)
	if err != nil {
		t.Fatalf("Failed to restore: %v", err)
	}

	// Verify file was changed
	if len(changedFiles) == 0 {
		t.Error("Expected at least one changed file")
	}

	// Verify content was restored
	content, _ = os.ReadFile(testFile)
	if string(content) != "turn 1 content" {
		t.Errorf("Expected 'turn 1 content', got '%s'", string(content))
	}

	// Verify turn was incremented
	if mgr.CurrentTurn() != 3 {
		t.Errorf("Expected turn 3 after restore, got %d", mgr.CurrentTurn())
	}
}

func TestManagerDiff(t *testing.T) {
	// Create temp workdir
	workdir, err := os.MkdirTemp("", "checkpoint-test-workdir-")
	if err != nil {
		t.Fatalf("Failed to create temp workdir: %v", err)
	}
	defer os.RemoveAll(workdir)

	// Create a test file
	testFile := filepath.Join(workdir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mgr, err := NewManager("test-diff", workdir, nil, 0)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Cleanup()

	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Turn 1: modify file
	mgr.StartTurn()
	if err := os.WriteFile(testFile, []byte("modified content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}
	if err := mgr.EndTurn(); err != nil {
		t.Fatalf("Failed to end turn 1: %v", err)
	}

	// Modify file without committing (current state differs from turn 1)
	if err := os.WriteFile(testFile, []byte("uncommitted content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}

	// Get diff
	diff, err := mgr.Diff(1, "")
	if err != nil {
		t.Fatalf("Failed to get diff: %v", err)
	}

	// Diff should show the change
	if diff == "" {
		t.Error("Expected non-empty diff")
	}
}

func TestManagerList(t *testing.T) {
	// Create temp workdir
	workdir, err := os.MkdirTemp("", "checkpoint-test-workdir-")
	if err != nil {
		t.Fatalf("Failed to create temp workdir: %v", err)
	}
	defer os.RemoveAll(workdir)

	// Create a test file
	testFile := filepath.Join(workdir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	mgr, err := NewManager("test-list", workdir, nil, 0)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}
	defer mgr.Cleanup()

	if err := mgr.Initialize(); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Turn 1
	mgr.StartTurn()
	if err := os.WriteFile(testFile, []byte("turn 1 content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}
	if err := mgr.EndTurn(); err != nil {
		t.Fatalf("Failed to end turn 1: %v", err)
	}

	// Turn 2
	mgr.StartTurn()
	if err := os.WriteFile(testFile, []byte("turn 2 content"), 0644); err != nil {
		t.Fatalf("Failed to modify test file: %v", err)
	}
	if err := mgr.EndTurn(); err != nil {
		t.Fatalf("Failed to end turn 2: %v", err)
	}

	// List turns
	turns, err := mgr.List()
	if err != nil {
		t.Fatalf("Failed to list turns: %v", err)
	}

	if len(turns) != 2 {
		t.Errorf("Expected 2 turns, got %d", len(turns))
	}

	if turns[0].Turn != 1 {
		t.Errorf("Expected first turn to be 1, got %d", turns[0].Turn)
	}

	if turns[1].Turn != 2 {
		t.Errorf("Expected second turn to be 2, got %d", turns[1].Turn)
	}
}

func TestManagerIsExternalPath(t *testing.T) {
	workdir, err := os.MkdirTemp("", "checkpoint-test-workdir-")
	if err != nil {
		t.Fatalf("Failed to create temp workdir: %v", err)
	}
	defer os.RemoveAll(workdir)

	mgr, err := NewManager("test-external", workdir, nil, 0)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	// Internal path
	internalPath := filepath.Join(workdir, "file.txt")
	if mgr.IsExternalPath(internalPath) {
		t.Error("Expected internal path to not be external")
	}

	// External path
	externalPath := "/etc/hosts"
	if !mgr.IsExternalPath(externalPath) {
		t.Error("Expected /etc/hosts to be external")
	}
}

func TestManagerDisabled(t *testing.T) {
	workdir, err := os.MkdirTemp("", "checkpoint-test-workdir-")
	if err != nil {
		t.Fatalf("Failed to create temp workdir: %v", err)
	}
	defer os.RemoveAll(workdir)

	mgr, err := NewManager("test-disabled", workdir, nil, 0)
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	mgr.SetEnabled(false)

	if mgr.Enabled() {
		t.Error("Expected manager to be disabled")
	}

	// Operations should be no-ops when disabled
	if err := mgr.EndTurn(); err != nil {
		t.Errorf("EndTurn should not error when disabled: %v", err)
	}

	_, err = mgr.Restore(0)
	if err == nil {
		t.Error("Expected Restore to error when disabled")
	}
}

package session

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

func TestNewManager(t *testing.T) {
	// Use temp dir as home
	tempHome, err := os.MkdirTemp("", "session-test-home-")
	if err != nil {
		t.Fatalf("Failed to create temp home: %v", err)
	}
	defer os.RemoveAll(tempHome)

	// Override HOME for this test
	oldHome := os.Getenv("HOME")
	os.Setenv("HOME", tempHome)
	defer os.Setenv("HOME", oldHome)

	mgr, err := NewManager()
	if err != nil {
		t.Fatalf("Failed to create manager: %v", err)
	}

	expectedDir := filepath.Join(tempHome, ".kvit-coder", "sessions")
	if mgr.baseDir != expectedDir {
		t.Errorf("Expected baseDir %s, got %s", expectedDir, mgr.baseDir)
	}

	// Check that directory was created
	if _, err := os.Stat(expectedDir); os.IsNotExist(err) {
		t.Error("Sessions directory was not created")
	}
}

func TestSessionExists(t *testing.T) {
	mgr := setupTestManager(t)

	// Non-existent session
	if mgr.SessionExists("nonexistent") {
		t.Error("Expected SessionExists to return false for non-existent session")
	}

	// Create a session file
	sessionPath := filepath.Join(mgr.baseDir, "test-session.jsonl")
	if err := os.WriteFile(sessionPath, []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to create test session file: %v", err)
	}

	// Existing session
	if !mgr.SessionExists("test-session") {
		t.Error("Expected SessionExists to return true for existing session")
	}
}

func TestSaveAndLoadSession(t *testing.T) {
	mgr := setupTestManager(t)

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "You are a helpful assistant."},
		{Role: llm.RoleUser, Content: "Hello"},
		{Role: llm.RoleAssistant, Content: "Hi there!"},
	}

	// Save session
	if err := mgr.SaveSession("test-save", messages); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	// Verify file exists
	if !mgr.SessionExists("test-save") {
		t.Error("Session file was not created")
	}

	// Load session
	loaded, err := mgr.LoadSession("test-save")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	if len(loaded) != len(messages) {
		t.Errorf("Expected %d messages, got %d", len(messages), len(loaded))
	}

	for i, msg := range loaded {
		if msg.Role != messages[i].Role {
			t.Errorf("Message %d: expected role %s, got %s", i, messages[i].Role, msg.Role)
		}
		if msg.Content != messages[i].Content {
			t.Errorf("Message %d: expected content %q, got %q", i, messages[i].Content, msg.Content)
		}
	}
}

func TestSaveSessionWithToolCalls(t *testing.T) {
	mgr := setupTestManager(t)

	messages := []llm.Message{
		{Role: llm.RoleUser, Content: "List files"},
		{
			Role:    llm.RoleAssistant,
			Content: "I'll list the files.",
			ToolCalls: []llm.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: struct {
						Name      string `json:"name"`
						Arguments string `json:"arguments"`
					}{
						Name:      "shell",
						Arguments: `{"command": "ls -la"}`,
					},
				},
			},
		},
		{
			Role:       llm.RoleTool,
			Name:       "shell",
			Content:    "file1.txt\nfile2.txt",
			ToolCallID: "call_123",
		},
	}

	// Save and reload
	if err := mgr.SaveSession("test-tools", messages); err != nil {
		t.Fatalf("Failed to save session: %v", err)
	}

	loaded, err := mgr.LoadSession("test-tools")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(loaded))
	}

	// Check tool call was preserved
	if len(loaded[1].ToolCalls) != 1 {
		t.Fatalf("Expected 1 tool call, got %d", len(loaded[1].ToolCalls))
	}
	if loaded[1].ToolCalls[0].ID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got %q", loaded[1].ToolCalls[0].ID)
	}

	// Check tool result was preserved
	if loaded[2].ToolCallID != "call_123" {
		t.Errorf("Expected tool call ID 'call_123', got %q", loaded[2].ToolCallID)
	}
}

func TestAppendToSession(t *testing.T) {
	mgr := setupTestManager(t)

	// Initial messages
	initial := []llm.Message{
		{Role: llm.RoleUser, Content: "First message"},
	}
	if err := mgr.SaveSession("test-append", initial); err != nil {
		t.Fatalf("Failed to save initial session: %v", err)
	}

	// Append more messages
	additional := []llm.Message{
		{Role: llm.RoleAssistant, Content: "Response"},
		{Role: llm.RoleUser, Content: "Second message"},
	}
	if err := mgr.AppendToSession("test-append", additional); err != nil {
		t.Fatalf("Failed to append to session: %v", err)
	}

	// Load and verify
	loaded, err := mgr.LoadSession("test-append")
	if err != nil {
		t.Fatalf("Failed to load session: %v", err)
	}

	if len(loaded) != 3 {
		t.Errorf("Expected 3 messages, got %d", len(loaded))
	}
}

func TestGenerateSessionName(t *testing.T) {
	mgr := setupTestManager(t)

	name := mgr.GenerateSessionName()

	// Check format: YYYY-MM-DD-random6
	parts := strings.Split(name, "-")
	if len(parts) != 4 {
		t.Errorf("Expected 4 parts in session name, got %d: %s", len(parts), name)
	}

	// Check date prefix
	today := time.Now().Format("2006-01-02")
	if !strings.HasPrefix(name, today) {
		t.Errorf("Expected session name to start with %s, got %s", today, name)
	}

	// Check suffix length
	suffix := parts[3]
	if len(suffix) != 6 {
		t.Errorf("Expected 6-char suffix, got %d chars: %s", len(suffix), suffix)
	}

	// Generate another and verify uniqueness
	name2 := mgr.GenerateSessionName()
	if name == name2 {
		t.Error("Generated session names should be unique")
	}
}

func TestListSessions(t *testing.T) {
	mgr := setupTestManager(t)

	// Empty list
	sessions, err := mgr.ListSessions()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}
	if len(sessions) != 0 {
		t.Errorf("Expected 0 sessions, got %d", len(sessions))
	}

	// Create some sessions
	_ = mgr.SaveSession("session-a", []llm.Message{{Role: llm.RoleUser, Content: "a"}})
	time.Sleep(10 * time.Millisecond) // Ensure different mod times
	_ = mgr.SaveSession("session-b", []llm.Message{
		{Role: llm.RoleUser, Content: "b1"},
		{Role: llm.RoleAssistant, Content: "b2"},
	})

	sessions, err = mgr.ListSessions()
	if err != nil {
		t.Fatalf("Failed to list sessions: %v", err)
	}

	if len(sessions) != 2 {
		t.Fatalf("Expected 2 sessions, got %d", len(sessions))
	}

	// Should be sorted by mod time (newest first)
	if sessions[0].Name != "session-b" {
		t.Errorf("Expected newest session first, got %s", sessions[0].Name)
	}

	// Check message counts
	if sessions[0].MessageCount != 2 {
		t.Errorf("Expected session-b to have 2 messages, got %d", sessions[0].MessageCount)
	}
	if sessions[1].MessageCount != 1 {
		t.Errorf("Expected session-a to have 1 message, got %d", sessions[1].MessageCount)
	}
}

func TestDeleteSession(t *testing.T) {
	mgr := setupTestManager(t)

	// Create a session
	_ = mgr.SaveSession("to-delete", []llm.Message{{Role: llm.RoleUser, Content: "test"}})

	if !mgr.SessionExists("to-delete") {
		t.Fatal("Session was not created")
	}

	// Delete it
	if err := mgr.DeleteSession("to-delete"); err != nil {
		t.Fatalf("Failed to delete session: %v", err)
	}

	if mgr.SessionExists("to-delete") {
		t.Error("Session still exists after deletion")
	}

	// Deleting non-existent session should fail
	if err := mgr.DeleteSession("nonexistent"); err == nil {
		t.Error("Expected error when deleting non-existent session")
	}
}

func TestShowSession(t *testing.T) {
	mgr := setupTestManager(t)

	messages := []llm.Message{
		{Role: llm.RoleSystem, Content: "System prompt"},
		{Role: llm.RoleUser, Content: "Hello there"},
		{Role: llm.RoleAssistant, Content: "Hi! How can I help?"},
	}
	_ = mgr.SaveSession("test-show", messages)

	content, err := mgr.ShowSession("test-show")
	if err != nil {
		t.Fatalf("Failed to show session: %v", err)
	}

	// Check content includes expected parts
	if !strings.Contains(content, "Session: test-show") {
		t.Error("Output should contain session name")
	}
	if !strings.Contains(content, "[user]") {
		t.Error("Output should contain user role")
	}
	if !strings.Contains(content, "Hello there") {
		t.Error("Output should contain user message")
	}
	if !strings.Contains(content, "[assistant]") {
		t.Error("Output should contain assistant role")
	}
	if !strings.Contains(content, "[system] (omitted)") {
		t.Error("System message should be shown as omitted")
	}
}

func TestAcquireLock(t *testing.T) {
	mgr := setupTestManager(t)

	// Acquire lock
	unlock, err := mgr.AcquireLock("test-lock")
	if err != nil {
		t.Fatalf("Failed to acquire lock: %v", err)
	}

	// Try to acquire same lock again (should fail)
	_, err = mgr.AcquireLock("test-lock")
	if err == nil {
		t.Error("Expected error when acquiring lock twice")
	}
	if !strings.Contains(err.Error(), "already in use") {
		t.Errorf("Expected 'already in use' error, got: %v", err)
	}

	// Release lock
	unlock()

	// Should be able to acquire again
	unlock2, err := mgr.AcquireLock("test-lock")
	if err != nil {
		t.Fatalf("Failed to acquire lock after release: %v", err)
	}
	unlock2()
}

func TestLoadNonExistentSession(t *testing.T) {
	mgr := setupTestManager(t)

	_, err := mgr.LoadSession("nonexistent")
	if err == nil {
		t.Error("Expected error when loading non-existent session")
	}
}

func TestLargeMessage(t *testing.T) {
	mgr := setupTestManager(t)

	// Create a message with large content (> 64KB default buffer)
	largeContent := strings.Repeat("x", 100*1024) // 100KB
	messages := []llm.Message{
		{Role: llm.RoleUser, Content: largeContent},
	}

	if err := mgr.SaveSession("large-msg", messages); err != nil {
		t.Fatalf("Failed to save large message: %v", err)
	}

	loaded, err := mgr.LoadSession("large-msg")
	if err != nil {
		t.Fatalf("Failed to load large message: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(loaded))
	}
	if len(loaded[0].Content) != len(largeContent) {
		t.Errorf("Expected content length %d, got %d", len(largeContent), len(loaded[0].Content))
	}
}

// setupTestManager creates a Manager with a temp directory for testing
func setupTestManager(t *testing.T) *Manager {
	t.Helper()

	tempDir, err := os.MkdirTemp("", "session-test-")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	return &Manager{baseDir: tempDir}
}

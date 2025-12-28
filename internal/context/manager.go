package context

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"github.com/kvit-s/kvit-coder/internal/checkpoint"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// Manager handles context tools git repo for conversation branch tracking
type Manager struct {
	mu            sync.RWMutex
	sessionID     string
	contextDir    string // temp dir for context tools git
	enabled       bool
	branchCounter int // sequential branch naming
	checkpointMgr *checkpoint.Manager

	// System prompt (kept in RAM, prepended to file contents)
	systemPrompt string

	// Cached state (derived from file on read)
	cachedLastMessage *llm.Message
	cachedTurnCount   int
}

// ToolContent represents the parsed content of a tool result
type ToolContent struct {
	Result   string        `json:"result"`
	Diff     string        `json:"diff,omitempty"`    // for completed task
	Success  *bool         `json:"success,omitempty"` // for completed task (nil = true)
	Internal *InternalMeta `json:"_internal,omitempty"`
}

// InternalMeta contains metadata for context tool operations
type InternalMeta struct {
	CheckpointID string `json:"checkpoint_id,omitempty"` // checkpoint at task start (for rollback)
	HasChanges   bool   `json:"has_changes,omitempty"`   // true if task made file changes
}

// NewManager creates a new context tools manager
func NewManager(sessionID string, checkpointMgr *checkpoint.Manager) (*Manager, error) {
	if sessionID == "" {
		return nil, fmt.Errorf("sessionID cannot be empty")
	}

	contextDir := filepath.Join(os.TempDir(), fmt.Sprintf("go-coder-context-%s", sessionID))

	m := &Manager{
		sessionID:     sessionID,
		contextDir:    contextDir,
		enabled:       true,
		branchCounter: 0,
		checkpointMgr: checkpointMgr,
	}

	return m, nil
}

// Initialize sets up the context tools git repository
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.enabled {
		return nil
	}

	// Create context directory
	if err := os.MkdirAll(m.contextDir, 0755); err != nil {
		return fmt.Errorf("failed to create context directory: %w", err)
	}

	// Initialize git repo (not bare, we need a working tree for turns.jsonl)
	cmd := exec.Command("git", "init")
	cmd.Dir = m.contextDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to init context git: %w\nOutput: %s", err, output)
	}

	// Configure git user for this repo
	cmd = exec.Command("git", "config", "user.email", "go-coder@local")
	cmd.Dir = m.contextDir
	cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "go-coder")
	cmd.Dir = m.contextDir
	cmd.Run()

	// Create empty turns.jsonl
	turnsFile := filepath.Join(m.contextDir, "turns.jsonl")
	if err := os.WriteFile(turnsFile, []byte{}, 0644); err != nil {
		return fmt.Errorf("failed to create turns.jsonl: %w", err)
	}

	// Initial commit on branch "0" (the starting branch)
	cmd = exec.Command("git", "add", "turns.jsonl")
	cmd.Dir = m.contextDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to add turns.jsonl: %w\nOutput: %s", err, output)
	}

	cmd = exec.Command("git", "commit", "-m", "Initial context")
	cmd.Dir = m.contextDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create initial commit: %w\nOutput: %s", err, output)
	}

	// Rename branch to "0"
	cmd = exec.Command("git", "branch", "-m", "0")
	cmd.Dir = m.contextDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to rename branch to 0: %w\nOutput: %s", err, output)
	}

	return nil
}

// =============================================================================
// Core Read/Write Operations
// =============================================================================

// ReadTurnsForLLM reads all messages from current branch's turns.jsonl
// This is the primary way to get message history for LLM calls
func (m *Manager) ReadTurnsForLLM() ([]llm.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.readTurnsFile()
}

// readTurnsFile reads turns without locking (internal use)
func (m *Manager) readTurnsFile() ([]llm.Message, error) {
	turnsFile := filepath.Join(m.contextDir, "turns.jsonl")
	data, err := os.ReadFile(turnsFile)
	if err != nil {
		if os.IsNotExist(err) {
			return []llm.Message{}, nil
		}
		return nil, fmt.Errorf("failed to read turns.jsonl: %w", err)
	}

	if len(data) == 0 {
		return []llm.Message{}, nil
	}

	var turns []llm.Message
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		var msg llm.Message
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			return nil, fmt.Errorf("failed to parse turn %d: %w", i+1, err)
		}
		turns = append(turns, msg)
	}

	// Update cache
	if len(turns) > 0 {
		m.cachedLastMessage = &turns[len(turns)-1]
	}
	m.cachedTurnCount = countAssistantMessages(turns)

	return turns, nil
}

// AppendMessages appends multiple messages to turns.jsonl (batch write)
// Used at the end of a successful iteration to persist the buffer
func (m *Manager) AppendMessages(messages []llm.Message) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	turnsFile := filepath.Join(m.contextDir, "turns.jsonl")

	f, err := os.OpenFile(turnsFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
	if err != nil {
		return fmt.Errorf("failed to open turns.jsonl: %w", err)
	}
	defer f.Close()

	for _, msg := range messages {
		data, err := json.Marshal(msg)
		if err != nil {
			return fmt.Errorf("failed to marshal message: %w", err)
		}
		if _, err := f.WriteString(string(data) + "\n"); err != nil {
			return fmt.Errorf("failed to write message: %w", err)
		}
		// Update cache
		msgCopy := msg
		m.cachedLastMessage = &msgCopy
		if msg.Role == llm.RoleAssistant {
			m.cachedTurnCount++
		}
	}

	return nil
}

// AppendMessage appends a single message to turns.jsonl
func (m *Manager) AppendMessage(msg llm.Message) error {
	return m.AppendMessages([]llm.Message{msg})
}

// ModifyLastMessage modifies the last message in turns.jsonl
// Used for patterns like appending intervention hints to last tool result
func (m *Manager) ModifyLastMessage(modifier func(*llm.Message)) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	turns, err := m.readTurnsFile()
	if err != nil {
		return err
	}

	if len(turns) == 0 {
		return fmt.Errorf("no messages to modify")
	}

	// Modify the last message
	modifier(&turns[len(turns)-1])

	// Rewrite the file
	return m.writeTurnsFile(turns)
}

// writeTurnsFile writes all turns to file (internal use)
func (m *Manager) writeTurnsFile(turns []llm.Message) error {
	turnsFile := filepath.Join(m.contextDir, "turns.jsonl")

	var lines []string
	for _, t := range turns {
		data, err := json.Marshal(t)
		if err != nil {
			return fmt.Errorf("failed to marshal turn: %w", err)
		}
		lines = append(lines, string(data))
	}

	content := strings.Join(lines, "\n")
	if len(lines) > 0 {
		content += "\n"
	}

	// Update cache
	if len(turns) > 0 {
		m.cachedLastMessage = &turns[len(turns)-1]
	}
	m.cachedTurnCount = countAssistantMessages(turns)

	return os.WriteFile(turnsFile, []byte(content), 0644)
}

// =============================================================================
// Cache Accessors
// =============================================================================

// GetLastMessage returns the cached last message (nil if empty)
func (m *Manager) GetLastMessage() *llm.Message {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cachedLastMessage
}

// GetTurnCount returns the number of assistant messages (turns)
func (m *Manager) GetTurnCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.cachedTurnCount
}

// countAssistantMessages counts assistant messages in turns
func countAssistantMessages(turns []llm.Message) int {
	count := 0
	for _, t := range turns {
		if t.Role == llm.RoleAssistant {
			count++
		}
	}
	return count
}

// =============================================================================
// System Prompt Management
// =============================================================================

// SetSystemPrompt stores the system prompt (kept in RAM, prepended when reading)
func (m *Manager) SetSystemPrompt(prompt string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.systemPrompt = prompt
}

// GetSystemPrompt returns the stored system prompt
func (m *Manager) GetSystemPrompt() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.systemPrompt
}

// ReadMessagesForLLM reads turns from file and prepends system prompt
// This is the primary method for getting messages to send to the LLM
func (m *Manager) ReadMessagesForLLM() ([]llm.Message, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	turns, err := m.readTurnsFile()
	if err != nil {
		return nil, err
	}

	// Prepend system prompt if set
	if m.systemPrompt != "" {
		messages := make([]llm.Message, 0, len(turns)+1)
		messages = append(messages, llm.Message{
			Role:    llm.RoleSystem,
			Content: m.systemPrompt,
		})
		messages = append(messages, turns...)
		return messages, nil
	}

	return turns, nil
}

// =============================================================================
// Branch Operations
// =============================================================================

// CurrentBranch returns the current branch name
func (m *Manager) CurrentBranch() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cmd := exec.Command("git", "branch", "--show-current")
	cmd.Dir = m.contextDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "0"
	}
	return strings.TrimSpace(string(output))
}

// CreateBranch creates a new branch and switches to it
func (m *Manager) CreateBranch() (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.branchCounter++
	branchName := strconv.Itoa(m.branchCounter)

	cmd := exec.Command("git", "checkout", "-b", branchName)
	cmd.Dir = m.contextDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("failed to create branch %s: %w\nOutput: %s", branchName, err, output)
	}

	return branchName, nil
}

// SwitchBranch switches to an existing branch
func (m *Manager) SwitchBranch(branchName string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = m.contextDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to switch to branch %s: %w\nOutput: %s", branchName, err, output)
	}

	// Invalidate cache on branch switch
	m.cachedLastMessage = nil
	m.cachedTurnCount = 0

	return nil
}

// ParentBranch returns the parent branch of the given branch
func (m *Manager) ParentBranch(branchName string) string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// For our simple model: parent is branch with number - 1
	if branchNum, err := strconv.Atoi(branchName); err == nil && branchNum > 0 {
		return strconv.Itoa(branchNum - 1)
	}

	return "0"
}

// CommitTurns commits the current turns.jsonl state
func (m *Manager) CommitTurns(message string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cmd := exec.Command("git", "add", "turns.jsonl")
	cmd.Dir = m.contextDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to stage turns.jsonl: %w\nOutput: %s", err, output)
	}

	cmd = exec.Command("git", "commit", "-m", message)
	cmd.Dir = m.contextDir
	if output, err := cmd.CombinedOutput(); err != nil {
		// Ignore "nothing to commit"
		if !strings.Contains(string(output), "nothing to commit") {
			return fmt.Errorf("failed to commit: %w\nOutput: %s", err, output)
		}
	}

	return nil
}

// ReplaceLastToolResult replaces the content of the last tool result message
func (m *Manager) ReplaceLastToolResult(content ToolContent) error {
	return m.ModifyLastMessage(func(msg *llm.Message) {
		if msg.Role == llm.RoleTool {
			contentJSON, _ := json.Marshal(content)
			msg.Content = string(contentJSON)
		}
	})
}

// =============================================================================
// State Derivation Functions
// =============================================================================

// ParseToolContent parses the content field of a tool message
func ParseToolContent(msg llm.Message) *ToolContent {
	if msg.Role != llm.RoleTool {
		return nil
	}
	var content ToolContent
	if err := json.Unmarshal([]byte(msg.Content), &content); err != nil {
		return nil
	}
	return &content
}

// HasUnfinishedTask checks if there's an unfinished Tasks.Start in history
func (m *Manager) HasUnfinishedTask() bool {
	turns, err := m.ReadTurnsForLLM()
	if err != nil {
		return false
	}
	return HasUnfinishedTaskInHistory(turns)
}

// HasUnfinishedTaskInHistory checks if the most recent Tasks.Start tool result
// has "task started" (meaning we're still working on it)
func HasUnfinishedTaskInHistory(turns []llm.Message) bool {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Name == "Tasks.Start" && turns[i].Role == llm.RoleTool {
			content := ParseToolContent(turns[i])
			if content != nil && content.Result == "task started" {
				return true // found unfinished task
			}
			return false // most recent task is completed
		}
	}
	return false // no task found
}

// GetTaskCheckpoint returns the checkpoint ID for the current task
func GetTaskCheckpoint(turns []llm.Message) *string {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Name == "Tasks.Start" && turns[i].Role == llm.RoleTool {
			content := ParseToolContent(turns[i])
			if content != nil && content.Internal != nil {
				if content.Result == "task started" {
					return &content.Internal.CheckpointID
				}
				return nil // task completed
			}
		}
	}
	return nil
}

// GetPendingDiff returns the checkpoint ID if there's a pending diff awaiting accept/decline
func GetPendingDiff(turns []llm.Message) *string {
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Name == "Tasks.AcceptDiff" || turns[i].Name == "Tasks.DeclineDiff" {
			return nil // diff already resolved
		}
		if turns[i].Name == "Tasks.Start" && turns[i].Role == llm.RoleTool {
			content := ParseToolContent(turns[i])
			if content != nil && content.Internal != nil {
				if content.Result != "task started" && content.Internal.HasChanges {
					return &content.Internal.CheckpointID
				}
			}
			return nil
		}
	}
	return nil
}

// GetTaskDepth returns the current task nesting depth
func GetTaskDepth(turns []llm.Message) int {
	depth := 0
	for _, turn := range turns {
		if turn.Name == "Tasks.Start" && turn.Role == llm.RoleTool {
			content := ParseToolContent(turn)
			if content != nil {
				if content.Result == "task started" {
					depth++
				} else {
					depth--
				}
			}
		}
	}
	if depth < 0 {
		depth = 0
	}
	return depth
}

// CountTurnsSinceTaskStart counts turns since the last Tasks.Start
func CountTurnsSinceTaskStart(turns []llm.Message) int {
	count := 0
	for i := len(turns) - 1; i >= 0; i-- {
		if turns[i].Role == llm.RoleAssistant {
			count++
		}
		if turns[i].Name == "Tasks.Start" && turns[i].Role == llm.RoleTool {
			content := ParseToolContent(turns[i])
			if content != nil && content.Result == "task started" {
				return count
			}
		}
	}
	return count
}

// CanExecuteTool checks if a tool can be executed given current state
func CanExecuteTool(turns []llm.Message, toolName string) bool {
	pendingDiff := GetPendingDiff(turns)
	if pendingDiff == nil {
		return true
	}
	return toolName == "Tasks.AcceptDiff" || toolName == "Tasks.DeclineDiff"
}

// =============================================================================
// Checkpoint Integration
// =============================================================================

// CheckpointManager returns the associated checkpoint manager
func (m *Manager) CheckpointManager() *checkpoint.Manager {
	return m.checkpointMgr
}

// CreateCheckpoint creates a named checkpoint at the current file state
func (m *Manager) CreateCheckpoint() (string, error) {
	if m.checkpointMgr == nil {
		return "", fmt.Errorf("checkpoint manager not available")
	}

	turn := m.checkpointMgr.CurrentTurn()
	checkpointName := fmt.Sprintf("checkpoint_%d", turn)
	return checkpointName, nil
}

// GetDiffSinceCheckpoint gets the diff of file changes since a checkpoint
func (m *Manager) GetDiffSinceCheckpoint(checkpointID string) (string, error) {
	if m.checkpointMgr == nil {
		return "", fmt.Errorf("checkpoint manager not available")
	}

	parts := strings.Split(checkpointID, "_")
	if len(parts) != 2 {
		return "", fmt.Errorf("invalid checkpoint ID: %s", checkpointID)
	}

	turnNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid checkpoint number: %s", parts[1])
	}

	return m.checkpointMgr.Diff(turnNum, "")
}

// RollbackToCheckpoint rolls back file system to a checkpoint state
func (m *Manager) RollbackToCheckpoint(checkpointID string) error {
	if m.checkpointMgr == nil {
		return fmt.Errorf("checkpoint manager not available")
	}

	parts := strings.Split(checkpointID, "_")
	if len(parts) != 2 {
		return fmt.Errorf("invalid checkpoint ID: %s", checkpointID)
	}

	turnNum, err := strconv.Atoi(parts[1])
	if err != nil {
		return fmt.Errorf("invalid checkpoint number: %s", parts[1])
	}

	_, err = m.checkpointMgr.Restore(turnNum)
	return err
}

// RestoreFileToStart restores a file to its state at conversation start
func (m *Manager) RestoreFileToStart(path string) ([]byte, error) {
	if m.checkpointMgr == nil {
		return nil, fmt.Errorf("checkpoint manager not available")
	}

	return m.checkpointMgr.RestoreFile(path)
}

// RestoreFileToTaskStart restores a file to its state at current task start
func (m *Manager) RestoreFileToTaskStart(path string) ([]byte, error) {
	if m.checkpointMgr == nil {
		return nil, fmt.Errorf("checkpoint manager not available")
	}

	turns, err := m.ReadTurnsForLLM()
	if err != nil {
		return nil, err
	}

	checkpointID := GetTaskCheckpoint(turns)
	if checkpointID == nil {
		return nil, fmt.Errorf("not in a task")
	}

	// TODO: Implement per-file restore to specific turn
	return nil, fmt.Errorf("RestoreFileToTaskStart not yet implemented")
}

// =============================================================================
// Utility
// =============================================================================

// Enabled returns whether context tools are enabled
func (m *Manager) Enabled() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.enabled
}

// SetEnabled enables or disables context tools
func (m *Manager) SetEnabled(enabled bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.enabled = enabled
}

// Cleanup removes the context directory
func (m *Manager) Cleanup() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.contextDir == "" {
		return nil
	}

	return os.RemoveAll(m.contextDir)
}

package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	ctxtools "github.com/kvit-s/kvit-coder/internal/context"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// =============================================================================
// Tasks.Start - Start a task for context compression
// =============================================================================

type TasksStartTool struct {
	manager *ctxtools.Manager
}

func NewTasksStartTool(manager *ctxtools.Manager) *TasksStartTool {
	return &TasksStartTool{manager: manager}
}

func (t *TasksStartTool) Name() string {
	return "Tasks.Start"
}

func (t *TasksStartTool) Description() string {
	return "Start a task. Intermediate work will be hidden when finished. Use for exploration, debugging, or any multi-step work where only the final result matters."
}

func (t *TasksStartTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"task": map[string]any{
				"type":        "string",
				"description": "Description of the task goal",
			},
		},
		"required": []string{"task"},
	}
}

func (t *TasksStartTool) PromptCategory() string { return "context" }
func (t *TasksStartTool) PromptOrder() int       { return 10 }
func (t *TasksStartTool) PromptSection() string {
	return `## Tasks

You have tools for managing tasks and compressing context:

### Task Lifecycle
- **Tasks.Start[description]**: Start a task when planning multiple steps where intermediate results are not important for progress on the user's query (code exploration, trial tests, log analysis, etc.). Intermediate work will be compressed when finished.
- **Tasks.Finish[summary]**: Complete current task with a summary of findings/results. Use success=false if goal not achieved.

### File Changes
- **Tasks.AcceptDiff**: Accept file changes from completed task
- **Tasks.DeclineDiff**: Discard file changes, rollback to pre-task state
- **Tasks.RevertFile[path]**: Revert file to conversation start
- **Tasks.RevertToTaskStart[path]**: Revert file to task start (only inside task)

### When to use Tasks.Start:
- Searching through logs or data (many filter steps → final findings)
- Exploring codebase to answer a question (many reads → final answer)
- Debugging (many checks → root cause)
- Any multi-step exploration where intermediate steps don't need to persist

### File Changes:
- Tasks.Finish shows a diff of all file changes made in the task
- If there are changes, you MUST call Tasks.AcceptDiff or Tasks.DeclineDiff (other tools blocked)
- If no changes, continue normally`
}

type tasksStartArgs struct {
	Task string `json:"task"`
}

func (t *TasksStartTool) Check(ctx context.Context, args json.RawMessage) error {
	var params tasksStartArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	if strings.TrimSpace(params.Task) == "" {
		return SemanticError("task description cannot be empty")
	}

	// Check if there's a pending diff that needs to be resolved
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	if !ctxtools.CanExecuteTool(turns, "Tasks.Start") {
		return SemanticError("cannot start new task: pending diff must be resolved first. Use Tasks.AcceptDiff or Tasks.DeclineDiff.")
	}

	return nil
}

func (t *TasksStartTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params tasksStartArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	task := strings.TrimSpace(params.Task)

	// 1. Create checkpoint BEFORE task (for rollback if declined)
	checkpointID, err := t.manager.CreateCheckpoint()
	if err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to create checkpoint: %v", err))
	}

	// 2. Get current branch (will be parent)
	parentBranch := t.manager.CurrentBranch()

	// 3. Record Tasks.Start turn on PARENT branch first
	// The tool result will be replaced when task finishes
	assistantMsg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: fmt.Sprintf("Starting task: %s", task),
		ToolCalls: []llm.ToolCall{{
			ID:   "tasks_start",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "Tasks.Start",
				Arguments: string(args),
			},
		}},
	}
	if err := t.manager.AppendMessage(assistantMsg); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to append assistant turn: %v", err))
	}

	toolContent := ctxtools.ToolContent{
		Result: "task started",
		Internal: &ctxtools.InternalMeta{
			CheckpointID: checkpointID,
		},
	}
	contentJSON, _ := json.Marshal(toolContent)
	toolMsg := llm.Message{
		Role:       llm.RoleTool,
		Name:       "Tasks.Start",
		ToolCallID: "tasks_start",
		Content:    string(contentJSON),
	}
	if err := t.manager.AppendMessage(toolMsg); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to append tool turn: %v", err))
	}

	if err := t.manager.CommitTurns("Tasks.Start: " + task); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to commit: %v", err))
	}

	// 4. Create new branch for task work
	branchName, err := t.manager.CreateBranch()
	if err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to create task branch: %v", err))
	}

	return map[string]any{
		"status":        "task started",
		"task":          task,
		"branch":        branchName,
		"parent_branch": parentBranch,
		"checkpoint_id": checkpointID,
	}, nil
}

// =============================================================================
// Tasks.Finish - Complete the current task
// =============================================================================

type TasksFinishTool struct {
	manager *ctxtools.Manager
}

func NewTasksFinishTool(manager *ctxtools.Manager) *TasksFinishTool {
	return &TasksFinishTool{manager: manager}
}

func (t *TasksFinishTool) Name() string {
	return "Tasks.Finish"
}

func (t *TasksFinishTool) Description() string {
	return "Complete the current task. Shows diff of all file changes. If changes exist, must call AcceptDiff or DeclineDiff next."
}

func (t *TasksFinishTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"summary": map[string]any{
				"type":        "string",
				"description": "Condensed summary of findings/results (required even for failed tasks)",
			},
			"success": map[string]any{
				"type":        "boolean",
				"description": "Whether the task goal was accomplished (default: true)",
			},
		},
		"required": []string{"summary"},
	}
}

func (t *TasksFinishTool) PromptCategory() string { return "context" }
func (t *TasksFinishTool) PromptOrder() int       { return 20 }
func (t *TasksFinishTool) PromptSection() string  { return "" } // Docs in Tasks.Start

type tasksFinishArgs struct {
	Summary string `json:"summary"`
	Success *bool  `json:"success,omitempty"`
}

func (t *TasksFinishTool) Check(ctx context.Context, args json.RawMessage) error {
	var params tasksFinishArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	if strings.TrimSpace(params.Summary) == "" {
		return SemanticError("summary cannot be empty")
	}

	// Check if we're actually in a task
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	if !ctxtools.HasUnfinishedTaskInHistory(turns) {
		return SemanticError("not in a task. Use Tasks.Start first, or use Tasks.Collapse to retroactively compress past work.")
	}

	return nil
}

func (t *TasksFinishTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params tasksFinishArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	summary := strings.TrimSpace(params.Summary)
	success := true
	if params.Success != nil {
		success = *params.Success
	}

	// Get checkpoint_id from history (stateless)
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	preTaskCheckpoint := ctxtools.GetTaskCheckpoint(turns)
	if preTaskCheckpoint == nil {
		return nil, SemanticError("not in a task")
	}

	// Get current branch from context tools git
	taskBranch := t.manager.CurrentBranch()

	// 1. Get diff from pre-task checkpoint
	diffOutput, err := t.manager.GetDiffSinceCheckpoint(*preTaskCheckpoint)
	if err != nil {
		// Non-fatal, just no diff
		diffOutput = ""
	}
	hasChanges := diffOutput != ""

	// 2. Record Tasks.Finish on TASK branch (preserves detailed history)
	assistantMsg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: fmt.Sprintf("Finishing task: %s", summary),
		ToolCalls: []llm.ToolCall{{
			ID:   "tasks_finish",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "Tasks.Finish",
				Arguments: string(args),
			},
		}},
	}
	if err := t.manager.AppendMessage(assistantMsg); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to append assistant turn: %v", err))
	}

	finishedContent := ctxtools.ToolContent{
		Result: summary,
		Diff:   diffOutput,
		Internal: &ctxtools.InternalMeta{
			CheckpointID: *preTaskCheckpoint,
			HasChanges:   hasChanges,
		},
	}
	if !success {
		finishedContent.Success = &success
	}
	finishedJSON, _ := json.Marshal(finishedContent)
	toolMsg := llm.Message{
		Role:       llm.RoleTool,
		Name:       "Tasks.Finish",
		ToolCallID: "tasks_finish",
		Content:    string(finishedJSON),
	}
	if err := t.manager.AppendMessage(toolMsg); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to append tool turn: %v", err))
	}

	if err := t.manager.CommitTurns("Tasks.Finish: " + summary); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to commit: %v", err))
	}

	// 3. Switch to parent branch
	parentBranch := t.manager.ParentBranch(taskBranch)
	if err := t.manager.SwitchBranch(parentBranch); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to switch to parent branch: %v", err))
	}

	// 4. REPLACE the Tasks.Start tool result on parent branch with final result
	replaceContent := ctxtools.ToolContent{
		Result: summary,
		Diff:   diffOutput,
		Internal: &ctxtools.InternalMeta{
			CheckpointID: *preTaskCheckpoint,
			HasChanges:   hasChanges,
		},
	}
	if !success {
		replaceContent.Success = &success
	}
	if err := t.manager.ReplaceLastToolResult(replaceContent); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to replace tool result: %v", err))
	}

	if err := t.manager.CommitTurns("Task completed: " + summary); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to commit: %v", err))
	}

	result := map[string]any{
		"status":      "Task completed.",
		"summary":     summary,
		"success":     success,
		"has_changes": hasChanges,
	}

	if hasChanges {
		result["diff"] = diffOutput
		result["next_action"] = "Call Tasks.AcceptDiff to keep changes or Tasks.DeclineDiff to discard them."
	}

	return result, nil
}

// =============================================================================
// Tasks.AcceptDiff - Accept pending task diff
// =============================================================================

type TasksAcceptDiffTool struct {
	manager *ctxtools.Manager
}

func NewTasksAcceptDiffTool(manager *ctxtools.Manager) *TasksAcceptDiffTool {
	return &TasksAcceptDiffTool{manager: manager}
}

func (t *TasksAcceptDiffTool) Name() string {
	return "Tasks.AcceptDiff"
}

func (t *TasksAcceptDiffTool) Description() string {
	return "Accept pending task diff, applying all file changes to parent. Only available after Tasks.Finish with changes."
}

func (t *TasksAcceptDiffTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
}

func (t *TasksAcceptDiffTool) PromptCategory() string { return "context" }
func (t *TasksAcceptDiffTool) PromptOrder() int       { return 30 }
func (t *TasksAcceptDiffTool) PromptSection() string  { return "" } // Docs in Tasks.Start

func (t *TasksAcceptDiffTool) Check(ctx context.Context, args json.RawMessage) error {
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	pendingCheckpoint := ctxtools.GetPendingDiff(turns)
	if pendingCheckpoint == nil {
		return SemanticError("no pending diff to accept. This tool is only available after Tasks.Finish when there are file changes.")
	}

	return nil
}

func (t *TasksAcceptDiffTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	pendingCheckpoint := ctxtools.GetPendingDiff(turns)
	if pendingCheckpoint == nil {
		return nil, SemanticError("no pending diff to accept")
	}

	// Do nothing to file system - it already has the changes
	// Just record in context tools git
	assistantMsg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: "Accepting file changes.",
		ToolCalls: []llm.ToolCall{{
			ID:   "tasks_accept",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "Tasks.AcceptDiff",
				Arguments: "{}",
			},
		}},
	}
	if err := t.manager.AppendMessage(assistantMsg); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to append assistant turn: %v", err))
	}

	toolContent := ctxtools.ToolContent{Result: "Changes accepted."}
	contentJSON, _ := json.Marshal(toolContent)
	toolMsg := llm.Message{
		Role:       llm.RoleTool,
		Name:       "Tasks.AcceptDiff",
		ToolCallID: "tasks_accept",
		Content:    string(contentJSON),
	}
	if err := t.manager.AppendMessage(toolMsg); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to append tool turn: %v", err))
	}

	if err := t.manager.CommitTurns("Tasks.AcceptDiff"); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to commit: %v", err))
	}

	return map[string]any{
		"status":  "accepted",
		"message": "Changes accepted. Files remain as modified.",
	}, nil
}

// =============================================================================
// Tasks.DeclineDiff - Decline pending task diff
// =============================================================================

type TasksDeclineDiffTool struct {
	manager *ctxtools.Manager
}

func NewTasksDeclineDiffTool(manager *ctxtools.Manager) *TasksDeclineDiffTool {
	return &TasksDeclineDiffTool{manager: manager}
}

func (t *TasksDeclineDiffTool) Name() string {
	return "Tasks.DeclineDiff"
}

func (t *TasksDeclineDiffTool) Description() string {
	return "Decline pending task diff, discarding all file changes. Only available after Tasks.Finish with changes."
}

func (t *TasksDeclineDiffTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
}

func (t *TasksDeclineDiffTool) PromptCategory() string { return "context" }
func (t *TasksDeclineDiffTool) PromptOrder() int       { return 40 }
func (t *TasksDeclineDiffTool) PromptSection() string  { return "" } // Docs in Tasks.Start

func (t *TasksDeclineDiffTool) Check(ctx context.Context, args json.RawMessage) error {
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	pendingCheckpoint := ctxtools.GetPendingDiff(turns)
	if pendingCheckpoint == nil {
		return SemanticError("no pending diff to decline. This tool is only available after Tasks.Finish when there are file changes.")
	}

	return nil
}

func (t *TasksDeclineDiffTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	pendingCheckpoint := ctxtools.GetPendingDiff(turns)
	if pendingCheckpoint == nil {
		return nil, SemanticError("no pending diff to decline")
	}

	// Rollback to pre-task checkpoint (in checkpoint system - affects file system)
	if err := t.manager.RollbackToCheckpoint(*pendingCheckpoint); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to rollback: %v", err))
	}

	// Record in context tools git
	assistantMsg := llm.Message{
		Role:    llm.RoleAssistant,
		Content: "Declining file changes.",
		ToolCalls: []llm.ToolCall{{
			ID:   "tasks_decline",
			Type: "function",
			Function: struct {
				Name      string `json:"name"`
				Arguments string `json:"arguments"`
			}{
				Name:      "Tasks.DeclineDiff",
				Arguments: "{}",
			},
		}},
	}
	if err := t.manager.AppendMessage(assistantMsg); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to append assistant turn: %v", err))
	}

	toolContent := ctxtools.ToolContent{Result: "Changes discarded."}
	contentJSON, _ := json.Marshal(toolContent)
	toolMsg := llm.Message{
		Role:       llm.RoleTool,
		Name:       "Tasks.DeclineDiff",
		ToolCallID: "tasks_decline",
		Content:    string(contentJSON),
	}
	if err := t.manager.AppendMessage(toolMsg); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to append tool turn: %v", err))
	}

	if err := t.manager.CommitTurns("Tasks.DeclineDiff"); err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to commit: %v", err))
	}

	return map[string]any{
		"status":  "declined",
		"message": "Changes discarded. Files rolled back to pre-task state.",
	}, nil
}

// =============================================================================
// Tasks.RevertFile - Revert a file to conversation start
// =============================================================================

type TasksRevertFileTool struct {
	manager *ctxtools.Manager
}

func NewTasksRevertFileTool(manager *ctxtools.Manager) *TasksRevertFileTool {
	return &TasksRevertFileTool{manager: manager}
}

func (t *TasksRevertFileTool) Name() string {
	return "Tasks.RevertFile"
}

func (t *TasksRevertFileTool) Description() string {
	return "Revert a file to its state at the start of the conversation. Use when you want to undo all changes made to a file."
}

func (t *TasksRevertFileTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to revert",
			},
		},
		"required": []string{"path"},
	}
}

func (t *TasksRevertFileTool) PromptCategory() string { return "context" }
func (t *TasksRevertFileTool) PromptOrder() int       { return 50 }
func (t *TasksRevertFileTool) PromptSection() string  { return "" } // Docs in Tasks.Start

type tasksRevertFileArgs struct {
	Path string `json:"path"`
}

func (t *TasksRevertFileTool) Check(ctx context.Context, args json.RawMessage) error {
	var params tasksRevertFileArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	if strings.TrimSpace(params.Path) == "" {
		return SemanticError("path cannot be empty")
	}

	// Check if there's a pending diff that needs to be resolved
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	if !ctxtools.CanExecuteTool(turns, "Tasks.RevertFile") {
		return SemanticError("cannot revert file: pending diff must be resolved first. Use Tasks.AcceptDiff or Tasks.DeclineDiff.")
	}

	return nil
}

func (t *TasksRevertFileTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params tasksRevertFileArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	path := strings.TrimSpace(params.Path)

	content, err := t.manager.RestoreFileToStart(path)
	if err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to restore file: %v", err))
	}

	return map[string]any{
		"status":       "reverted",
		"path":         path,
		"message":      fmt.Sprintf("File '%s' reverted to conversation start state.", path),
		"content_size": len(content),
	}, nil
}

// =============================================================================
// Tasks.RevertToTaskStart - Revert a file to task start
// =============================================================================

type TasksRevertToTaskStartTool struct {
	manager *ctxtools.Manager
}

func NewTasksRevertToTaskStartTool(manager *ctxtools.Manager) *TasksRevertToTaskStartTool {
	return &TasksRevertToTaskStartTool{manager: manager}
}

func (t *TasksRevertToTaskStartTool) Name() string {
	return "Tasks.RevertToTaskStart"
}

func (t *TasksRevertToTaskStartTool) Description() string {
	return "Revert a file to its state when the current task began. Only available inside a task."
}

func (t *TasksRevertToTaskStartTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to the file to revert",
			},
		},
		"required": []string{"path"},
	}
}

func (t *TasksRevertToTaskStartTool) PromptCategory() string { return "context" }
func (t *TasksRevertToTaskStartTool) PromptOrder() int       { return 60 }
func (t *TasksRevertToTaskStartTool) PromptSection() string  { return "" } // Docs in Tasks.Start

type tasksRevertToTaskStartArgs struct {
	Path string `json:"path"`
}

func (t *TasksRevertToTaskStartTool) Check(ctx context.Context, args json.RawMessage) error {
	var params tasksRevertToTaskStartArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	if strings.TrimSpace(params.Path) == "" {
		return SemanticError("path cannot be empty")
	}

	// Check if we're actually in a task
	turns, err := t.manager.ReadTurnsForLLM()
	if err != nil {
		return RuntimeError(fmt.Sprintf("failed to read turns: %v", err))
	}

	if !ctxtools.HasUnfinishedTaskInHistory(turns) {
		return SemanticError("not in a task. Use Tasks.RevertFile to revert to conversation start instead.")
	}

	return nil
}

func (t *TasksRevertToTaskStartTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params tasksRevertToTaskStartArgs
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	path := strings.TrimSpace(params.Path)

	content, err := t.manager.RestoreFileToTaskStart(path)
	if err != nil {
		return nil, RuntimeError(fmt.Sprintf("failed to restore file: %v", err))
	}

	return map[string]any{
		"status":       "reverted",
		"path":         path,
		"message":      fmt.Sprintf("File '%s' reverted to task start state.", path),
		"content_size": len(content),
	}, nil
}

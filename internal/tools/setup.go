package tools

import (
	"fmt"
	"time"

	"github.com/kvit-s/kvit-coder/internal/checkpoint"
	"github.com/kvit-s/kvit-coder/internal/config"
	ctxtools "github.com/kvit-s/kvit-coder/internal/context"
)

// DebugLogger is an interface for debug logging to avoid import cycles
type DebugLogger interface {
	Debug(msg string)
}

// SetupConfig contains all dependencies needed to set up the tool registry
type SetupConfig struct {
	Cfg           *config.Config
	CheckpointMgr *checkpoint.Manager
	ContextMgr    *ctxtools.Manager
	Logger        DebugLogger // Optional debug logger (can be nil)
	TempFileMgr   *TempFileManager
	PlanManager   *PlanManager
}

// SetupRegistry creates and configures the tool registry based on config.
// It enables all tools according to the configuration and returns the populated registry.
func SetupRegistry(sc SetupConfig) *Registry {
	registry := NewRegistry()
	cfg := sc.Cfg

	// Helper for conditional debug logging
	debug := func(msg string) {
		if sc.Logger != nil {
			sc.Logger.Debug(msg)
		}
	}

	// Enable tools based on config
	if cfg.Tools.Read.Enabled {
		readFileTool := NewReadFileTool(cfg)
		registry.Enable(readFileTool)
		debug(fmt.Sprintf("Enabled tool: %s", readFileTool.Name()))
	}

	if cfg.Tools.Edit.Enabled {
		// Select edit tool based on configured mode
		var editTool Tool
		editMode := cfg.Tools.Edit.GetEditMode()
		switch editMode {
		case "searchreplace":
			editTool = NewSearchReplaceEditTool(cfg)
			debug(fmt.Sprintf("Enabled tool: %s (mode: searchreplace)", editTool.Name()))
		case "patch":
			editTool = NewPatchEditTool(cfg)
			debug(fmt.Sprintf("Enabled tool: %s (mode: patch)", editTool.Name()))
		default: // "unified" or empty - unified is the default
			editTool = NewUnifiedEditTool(cfg)
			debug(fmt.Sprintf("Enabled tool: %s (mode: unified)", editTool.Name()))
		}
		registry.Enable(editTool)

		// Write tool is enabled alongside Edit (uses same permission checks)
		writeFileTool := NewWriteFileTool(cfg)
		registry.Enable(writeFileTool)
		debug(fmt.Sprintf("Enabled tool: %s", writeFileTool.Name()))

		// Write.confirm and Write.cancel for overwrite confirmation
		confirmWriteTool := NewConfirmWriteTool(cfg)
		registry.Enable(confirmWriteTool)
		debug(fmt.Sprintf("Enabled tool: %s", confirmWriteTool.Name()))

		cancelWriteTool := NewCancelWriteTool(cfg)
		registry.Enable(cancelWriteTool)
		debug(fmt.Sprintf("Enabled tool: %s", cancelWriteTool.Name()))

		// edit.confirm and edit.cancel only available when edit is enabled AND preview_mode is true
		if cfg.Tools.Edit.PreviewMode {
			confirmEditTool := NewConfirmEditTool(cfg)
			registry.Enable(confirmEditTool)
			debug(fmt.Sprintf("Enabled tool: %s", confirmEditTool.Name()))

			cancelEditTool := NewCancelEditTool(cfg)
			registry.Enable(cancelEditTool)
			debug(fmt.Sprintf("Enabled tool: %s", cancelEditTool.Name()))
		}
	}

	if cfg.Tools.RestoreFile.Enabled && sc.CheckpointMgr != nil && sc.CheckpointMgr.Enabled() {
		restoreFileTool := NewRestoreFileTool(cfg, sc.CheckpointMgr)
		registry.Enable(restoreFileTool)
		debug(fmt.Sprintf("Enabled tool: %s", restoreFileTool.Name()))
	}

	if cfg.Tools.Search.Enabled && sc.TempFileMgr != nil {
		searchTool := NewSearchTool(cfg, sc.TempFileMgr)
		registry.Enable(searchTool)
		debug(fmt.Sprintf("Enabled tool: %s", searchTool.Name()))
	}

	if cfg.Tools.Shell.Enabled && sc.TempFileMgr != nil {
		shellTool := NewShellTool(cfg, 30*time.Second, sc.TempFileMgr)
		registry.Enable(shellTool)
		debug(fmt.Sprintf("Enabled tool: %s", shellTool.Name()))

		shellAdvancedTool := NewShellAdvancedTool(cfg, 30*time.Second, sc.TempFileMgr)
		registry.Enable(shellAdvancedTool)
		debug(fmt.Sprintf("Enabled tool: %s", shellAdvancedTool.Name()))
	}

	// Tasks.* tools - mutually exclusive with Plan.* and Checkpoint.* tools
	if cfg.Tools.Tasks.Enabled && sc.ContextMgr != nil {
		tasksStartTool := NewTasksStartTool(sc.ContextMgr)
		registry.Enable(tasksStartTool)
		debug(fmt.Sprintf("Enabled tool: %s", tasksStartTool.Name()))

		tasksFinishTool := NewTasksFinishTool(sc.ContextMgr)
		registry.Enable(tasksFinishTool)
		debug(fmt.Sprintf("Enabled tool: %s", tasksFinishTool.Name()))

		tasksAcceptDiffTool := NewTasksAcceptDiffTool(sc.ContextMgr)
		registry.Enable(tasksAcceptDiffTool)
		debug(fmt.Sprintf("Enabled tool: %s", tasksAcceptDiffTool.Name()))

		tasksDeclineDiffTool := NewTasksDeclineDiffTool(sc.ContextMgr)
		registry.Enable(tasksDeclineDiffTool)
		debug(fmt.Sprintf("Enabled tool: %s", tasksDeclineDiffTool.Name()))

		tasksRevertFileTool := NewTasksRevertFileTool(sc.ContextMgr)
		registry.Enable(tasksRevertFileTool)
		debug(fmt.Sprintf("Enabled tool: %s", tasksRevertFileTool.Name()))

		tasksRevertToTaskStartTool := NewTasksRevertToTaskStartTool(sc.ContextMgr)
		registry.Enable(tasksRevertToTaskStartTool)
		debug(fmt.Sprintf("Enabled tool: %s", tasksRevertToTaskStartTool.Name()))
	}

	// Plan tools - disabled when Tasks tools are enabled
	if cfg.Tools.Plan.Enabled && !cfg.Tools.Tasks.Enabled && sc.PlanManager != nil {
		planCreateTool := NewPlanCreateTool(sc.PlanManager)
		registry.Enable(planCreateTool)
		debug(fmt.Sprintf("Enabled tool: %s", planCreateTool.Name()))

		planAddStepTool := NewPlanAddStepTool(sc.PlanManager)
		registry.Enable(planAddStepTool)
		debug(fmt.Sprintf("Enabled tool: %s", planAddStepTool.Name()))

		planCompleteStepTool := NewPlanCompleteStepTool(sc.PlanManager)
		registry.Enable(planCompleteStepTool)
		debug(fmt.Sprintf("Enabled tool: %s", planCompleteStepTool.Name()))

		planRemoveStepTool := NewPlanRemoveStepTool(sc.PlanManager)
		registry.Enable(planRemoveStepTool)
		debug(fmt.Sprintf("Enabled tool: %s", planRemoveStepTool.Name()))

		planMoveStepTool := NewPlanMoveStepTool(sc.PlanManager)
		registry.Enable(planMoveStepTool)
		debug(fmt.Sprintf("Enabled tool: %s", planMoveStepTool.Name()))
	}

	// Checkpoint tools - disabled when Tasks tools are enabled
	if cfg.Tools.Checkpoint.Enabled && sc.CheckpointMgr != nil && sc.CheckpointMgr.Enabled() && !cfg.Tools.Tasks.Enabled {
		checkpointListTool := NewCheckpointListTool(sc.CheckpointMgr)
		registry.Enable(checkpointListTool)
		debug(fmt.Sprintf("Enabled tool: %s", checkpointListTool.Name()))

		checkpointRestoreTool := NewCheckpointRestoreTool(sc.CheckpointMgr)
		registry.Enable(checkpointRestoreTool)
		debug(fmt.Sprintf("Enabled tool: %s", checkpointRestoreTool.Name()))

		checkpointDiffTool := NewCheckpointDiffTool(sc.CheckpointMgr)
		registry.Enable(checkpointDiffTool)
		debug(fmt.Sprintf("Enabled tool: %s", checkpointDiffTool.Name()))

		checkpointUndoTool := NewCheckpointUndoTool(sc.CheckpointMgr)
		registry.Enable(checkpointUndoTool)
		debug(fmt.Sprintf("Enabled tool: %s", checkpointUndoTool.Name()))
	}

	return registry
}

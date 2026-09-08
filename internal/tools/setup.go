package tools

import (
	"fmt"
	"time"

	"github.com/kvit-s/kvit-coder/internal/checkpoint"
	"github.com/kvit-s/kvit-coder/internal/config"
	ctxtools "github.com/kvit-s/kvit-coder/internal/context"
	"github.com/kvit-s/kvit-coder/internal/procs"
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
	// ProcRegistry owns the processes that outlive a turn. Nil disables the
	// Shell.start / Observe.* tools.
	ProcRegistry *procs.Registry
	ToolCtx      *ToolContext // Shared mutable state for tools (created if nil)

	// MCPTools are adapters for tools discovered on configured MCP servers,
	// already built by the MCP manager. Passing built []Tool (rather than the
	// manager) keeps internal/tools free of any dependency on internal/mcp,
	// avoiding an import cycle. Nil/empty when MCP is disabled.
	MCPTools []Tool
}

// SetupRegistry creates and configures the tool registry based on config.
// It enables all tools according to the configuration and returns the populated registry.
func SetupRegistry(sc SetupConfig) *Registry {
	registry := NewRegistry()
	cfg := sc.Cfg

	// Create ToolContext if not provided
	toolCtx := sc.ToolCtx
	if toolCtx == nil {
		toolCtx = NewToolContext()
	}

	// Helper for conditional debug logging
	debug := func(msg string) {
		if sc.Logger != nil {
			sc.Logger.Debug(msg)
		}
	}

	// Enable tools based on config
	if cfg.Tools.Read.Enabled {
		readFileTool := NewReadFileTool(cfg, toolCtx)
		registry.Enable(readFileTool)
		debug(fmt.Sprintf("Enabled tool: %s", readFileTool.Name()))
	}

	if cfg.Tools.Images.Enabled {
		readImageTool := NewReadImageTool(cfg, toolCtx)
		registry.Enable(readImageTool)
		debug(fmt.Sprintf("Enabled tool: %s", readImageTool.Name()))
	}

	if cfg.Tools.Edit.Enabled {
		// Select edit tool based on configured mode
		var editTool Tool
		editMode := cfg.Tools.Edit.GetEditMode()
		switch editMode {
		case "searchreplace":
			editTool = NewSearchReplaceEditTool(cfg, toolCtx)
			debug(fmt.Sprintf("Enabled tool: %s (mode: searchreplace)", editTool.Name()))
		case "patch":
			editTool = NewPatchEditTool(cfg, toolCtx)
			debug(fmt.Sprintf("Enabled tool: %s (mode: patch)", editTool.Name()))
		default: // "unified" or empty - unified is the default
			editTool = NewUnifiedEditTool(cfg, toolCtx)
			debug(fmt.Sprintf("Enabled tool: %s (mode: unified)", editTool.Name()))
		}
		registry.Enable(editTool)

		// Write tool is enabled alongside Edit (uses same permission checks)
		writeFileTool := NewWriteFileTool(cfg, toolCtx)
		registry.Enable(writeFileTool)
		debug(fmt.Sprintf("Enabled tool: %s", writeFileTool.Name()))

		// The confirm handshake, for Write as much as for Edit, exists only
		// when preview_mode asks for it. Registering Write.confirm and
		// Write.cancel unconditionally offered a model two tools that had
		// nothing to confirm, and under the strong profile — where the
		// handshake is off — they were the tools it reached for when a stale
		// pending edit appeared in the history.
		if cfg.Tools.Edit.PreviewMode {
			confirmWriteTool := NewConfirmWriteTool(cfg, toolCtx)
			registry.Enable(confirmWriteTool)
			debug(fmt.Sprintf("Enabled tool: %s", confirmWriteTool.Name()))

			cancelWriteTool := NewCancelWriteTool(cfg, toolCtx)
			registry.Enable(cancelWriteTool)
			debug(fmt.Sprintf("Enabled tool: %s", cancelWriteTool.Name()))

			confirmEditTool := NewConfirmEditTool(cfg, toolCtx)
			registry.Enable(confirmEditTool)
			debug(fmt.Sprintf("Enabled tool: %s", confirmEditTool.Name()))

			cancelEditTool := NewCancelEditTool(cfg, toolCtx)
			registry.Enable(cancelEditTool)
			debug(fmt.Sprintf("Enabled tool: %s", cancelEditTool.Name()))

			// Edit.undo_autoindent reverts a first-line indent auto-correction.
			// Only meaningful when smart_first_line_indent is on, but it is a safe
			// no-op otherwise, so register it whenever preview mode is available.
			if cfg.Tools.Edit.SmartFirstLineIndent {
				undoAutoindentTool := NewUndoAutoindentTool(cfg, toolCtx)
				registry.Enable(undoAutoindentTool)
				debug(fmt.Sprintf("Enabled tool: %s", undoAutoindentTool.Name()))
			}
		}

		// DeleteLines: explicit, first-class line removal tool (Improvement 3).
		// Edit-mode independent; only registered when explicit_delete is on.
		if cfg.Tools.Edit.ExplicitDelete {
			deleteLinesTool := NewDeleteLinesTool(cfg, toolCtx)
			registry.Enable(deleteLinesTool)
			debug(fmt.Sprintf("Enabled tool: %s", deleteLinesTool.Name()))
		}
	}

	if cfg.Tools.RestoreFile.Enabled && sc.CheckpointMgr != nil && sc.CheckpointMgr.Enabled() {
		restoreFileTool := NewRestoreFileTool(cfg, sc.CheckpointMgr, toolCtx)
		registry.Enable(restoreFileTool)
		debug(fmt.Sprintf("Enabled tool: %s", restoreFileTool.Name()))
	}

	if cfg.Tools.Search.Enabled && sc.TempFileMgr != nil {
		searchTool := NewSearchTool(cfg, sc.TempFileMgr)
		registry.Enable(searchTool)
		debug(fmt.Sprintf("Enabled tool: %s", searchTool.Name()))
	}

	if cfg.Tools.Shell.Enabled && sc.TempFileMgr != nil {
		shellTimeout := time.Duration(cfg.Tools.Shell.DefaultTimeout) * time.Second
		shellTool := NewShellTool(cfg, shellTimeout, sc.TempFileMgr)
		shellTool.SetToolContext(toolCtx)
		registry.Enable(shellTool)
		debug(fmt.Sprintf("Enabled tool: %s", shellTool.Name()))

		shellAdvancedTool := NewShellAdvancedTool(cfg, shellTimeout, sc.TempFileMgr)
		shellAdvancedTool.SetToolContext(toolCtx)
		registry.Enable(shellAdvancedTool)
		debug(fmt.Sprintf("Enabled tool: %s", shellAdvancedTool.Name()))
	}

	if cfg.Tools.Procs.Enabled && sc.ProcRegistry != nil {
		for _, tool := range []Tool{
			NewShellStartTool(cfg, sc.ProcRegistry, toolCtx),
			NewShellOutputTool(cfg, sc.ProcRegistry, toolCtx),
			NewShellStatusTool(cfg, sc.ProcRegistry, toolCtx),
			NewShellListTool(cfg, sc.ProcRegistry, toolCtx),
			NewShellKillTool(cfg, sc.ProcRegistry, toolCtx),
			NewShellTuneTool(cfg, sc.ProcRegistry, toolCtx),
			NewObserveWaitTool(cfg, sc.ProcRegistry, toolCtx),
			NewObserveAddTool(cfg, sc.ProcRegistry, toolCtx),
		} {
			registry.Enable(tool)
			debug(fmt.Sprintf("Enabled tool: %s", tool.Name()))
		}
	}

	// Web tools. Enabling one in the config is the decision to let the agent
	// reach the network -- unlike a shell curl, which needs approving per
	// command because the shell can run anything, these two can only search
	// and fetch.
	if cfg.Tools.Web.Search.Enabled {
		webSearchTool := NewWebSearchTool(cfg)
		registry.Enable(webSearchTool)
		debug(fmt.Sprintf("Enabled tool: %s", webSearchTool.Name()))
	}

	if cfg.Tools.Web.Fetch.Enabled {
		webFetchTool := NewWebFetchTool(cfg, sc.TempFileMgr)
		registry.Enable(webFetchTool)
		debug(fmt.Sprintf("Enabled tool: %s", webFetchTool.Name()))
	}

	// Batch dispatches through the registry that holds it, so it is registered
	// first and given the registry once everything else is in.
	var batchTool *BatchTool
	if cfg.Tools.Batch.Enabled {
		batchTool = NewBatchTool(cfg)
		registry.Enable(batchTool)
		debug(fmt.Sprintf("Enabled tool: %s", batchTool.Name()))
	}

	if cfg.Tools.Question.Enabled {
		questionTool := NewQuestionTool(cfg, toolCtx)
		registry.Enable(questionTool)
		debug(fmt.Sprintf("Enabled tool: %s", questionTool.Name()))
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

	// Plan tools - disabled when Tasks tools are enabled.
	// Mode "write" (default) registers a single idempotent rewrite tool;
	// "incremental" registers the legacy 5-tool family.
	if cfg.Tools.Plan.Enabled && !cfg.Tools.Tasks.Enabled && sc.PlanManager != nil {
		if cfg.Tools.Plan.GetPlanMode() == "incremental" {
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
		} else {
			planWriteTool := NewPlanWriteTool(sc.PlanManager)
			registry.Enable(planWriteTool)
			debug(fmt.Sprintf("Enabled tool: %s", planWriteTool.Name()))
		}
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

	// MCP tools (from configured external servers). Registered last; they are
	// indistinguishable from built-in tools to the registry and agent loop.
	for _, t := range sc.MCPTools {
		registry.Enable(t)
		debug(fmt.Sprintf("Enabled MCP tool: %s", t.Name()))
	}

	// Batch can only dispatch once everything it might call is registered.
	if batchTool != nil {
		batchTool.SetRegistry(registry)
	}

	return registry
}

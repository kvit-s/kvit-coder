package tools

import (
	"fmt"
	"sort"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/procs"
)

// DebugLogger is an interface for debug logging to avoid import cycles
type DebugLogger interface {
	Debug(msg string)
}

// SetupConfig contains all dependencies needed to set up the tool registry
type SetupConfig struct {
	Cfg         *config.Config
	Logger      DebugLogger // Optional debug logger (can be nil)
	TempFileMgr *TempFileManager
	// ProcRegistry owns the processes that outlive a turn. Nil disables the
	// Shell.start / Observe.* tools.
	ProcRegistry *procs.Registry
	ToolCtx      *ToolContext // Shared mutable state for tools (created if nil)

	// MCPTools are adapters for tools discovered on configured MCP servers,
	// already built by the MCP manager. Passing built []Tool (rather than the
	// manager) keeps internal/tools free of any dependency on internal/mcp,
	// avoiding an import cycle. Nil/empty when MCP is disabled.
	MCPTools []Tool

	// ToolGroups are the groups from cfg.ToolGroups, already built with their
	// member sources bound (see group.go). Each is registered as one tool, and
	// any built-in tool the configuration moved into a group is handed to it
	// here instead of being registered on its own.
	ToolGroups []*GroupTool

	// SubRunner drives the Subagent child loop. It is the parent Runner,
	// injected after both exist (turn setup rebinds it), breaking the
	// tools->agent import cycle the same way MCPTools does. Nil disables
	// child runs: Subagent validates args but fails at Call time.
	SubRunner SubRunner
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

		}

		// DeleteLines: explicit, first-class line removal tool (Improvement 3).
		// Edit-mode independent; only registered when explicit_delete is on.
		if cfg.Tools.Edit.ExplicitDelete {
			deleteLinesTool := NewDeleteLinesTool(cfg, toolCtx)
			registry.Enable(deleteLinesTool)
			debug(fmt.Sprintf("Enabled tool: %s", deleteLinesTool.Name()))
		}
	}

	if cfg.Tools.Search.Enabled && sc.TempFileMgr != nil {
		searchTool := NewSearchTool(cfg, sc.TempFileMgr)
		registry.Enable(searchTool)
		debug(fmt.Sprintf("Enabled tool: %s", searchTool.Name()))
	}

	if cfg.Tools.Glob.Enabled {
		globTool := NewGlobTool(cfg)
		registry.Enable(globTool)
		debug(fmt.Sprintf("Enabled tool: %s", globTool.Name()))
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

	if cfg.Tools.Report.Enabled {
		reportTool := NewReportTool(cfg, toolCtx)
		registry.Enable(reportTool)
		debug(fmt.Sprintf("Enabled tool: %s", reportTool.Name()))
	}

	// Subagent delegates to a child loop with a filtered registry. Like
	// Batch it dispatches through the registry that holds it, so it is
	// registered first and given the registry once everything else is in.
	var subagentTool *SubagentTool
	if cfg.Tools.Subagent.Enabled {
		subagentTool = NewSubagentTool(cfg, toolCtx, sc.TempFileMgr)
		subagentTool.SetRunner(sc.SubRunner)
		registry.Enable(subagentTool)
		debug(fmt.Sprintf("Enabled tool: %s", subagentTool.Name()))
	}

	// MCP tools (from configured external servers). Registered last; they are
	// indistinguishable from built-in tools to the registry and agent loop.
	for _, t := range sc.MCPTools {
		registry.Enable(t)
		debug(fmt.Sprintf("Enabled MCP tool: %s", t.Name()))
	}

	// Tool groups. Registered last, after every built-in tool exists, because
	// a group can claim built-in tools: those are taken back out of the
	// registry and handed to their group, so they stop being advertised on
	// every request and are reachable only through it.
	if len(sc.ToolGroups) > 0 {
		byName := make(map[string]*GroupTool, len(sc.ToolGroups))
		for _, g := range sc.ToolGroups {
			byName[g.Name()] = g
		}

		claimed := cfg.DeferredBuiltinTools()
		names := make([]string, 0, len(claimed))
		for name := range claimed {
			names = append(names, name)
		}
		sort.Strings(names) // deterministic member order, and a stable log

		for _, name := range names {
			group := byName[claimed[name]]
			if group == nil {
				continue
			}
			tool := registry.Get(name)
			if tool == nil {
				debug(fmt.Sprintf("Tool group %s: no tool named %s to claim", group.Name(), name))
				continue
			}
			registry.Disable(tool.Name())
			group.AddLocalMembers(tool)
			debug(fmt.Sprintf("Tool group %s claimed: %s", group.Name(), tool.Name()))
		}

		for _, g := range sc.ToolGroups {
			registry.Enable(g)
			debug(fmt.Sprintf("Enabled tool group: %s", g.Name()))
		}
	}

	// Batch can only dispatch once everything it might call is registered.
	if batchTool != nil {
		batchTool.SetRegistry(registry)
		batchTool.SetToolContext(toolCtx)
	}

	// Subagent filters the parent registry at Call time, so it needs the
	// same late binding.
	if subagentTool != nil {
		subagentTool.SetRegistry(registry)
	}

	return registry
}

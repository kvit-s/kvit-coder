package config

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kvit-s/kvit-coder/internal/llm"
	"gopkg.in/yaml.v3"
)

type Config struct {
	LLM struct {
		BaseURL   string `yaml:"base_url"`
		APIKey    string `yaml:"api_key"`
		APIKeyEnv string `yaml:"api_key_env"`
		Model     string `yaml:"model"`
		// APIBackend picks the wire protocol: "chat_completions" (default) or
		// "responses". Some hosted models are served only on /responses and
		// return an error on /chat/completions.
		APIBackend string `yaml:"api_backend"`
		// Headers are extra "Key=Value" request headers sent with every call
		// (supports ${VAR}). Needed by endpoints that demand a routing or
		// session header of their own.
		Headers []string `yaml:"headers"`
		// ReasoningEffort asks a reasoning model for more or less thinking.
		// Only the "responses" backend sends it; values are the provider's
		// (commonly minimal, low, medium, high).
		ReasoningEffort string `yaml:"reasoning_effort"`
		// ReasoningSummary asks a reasoning model for readable text describing
		// its thinking. Only the "responses" backend sends it; values are the
		// provider's (auto, concise, detailed), and "off" suppresses it.
		// Empty follows merge_thinking, because thinking that is never
		// requested cannot be merged.
		ReasoningSummary string `yaml:"reasoning_summary"`
		// RequestTimeout bounds one HTTP request to the model, in seconds.
		// A request that exceeds it is retried, so this times the retry count
		// is how long a turn can stall on an endpoint that never answers.
		// Default 600.
		RequestTimeout int     `yaml:"request_timeout"`
		Temperature    float32 `yaml:"temperature"`
		MaxTokens      int     `yaml:"max_output_tokens"`
		Context        int     `yaml:"context"`        // Max context size for display (0 = don't show)
		MergeThinking  bool    `yaml:"merge_thinking"` // Merge reasoning_content into content (default: false, discard thinking)
		Verbose        int     `yaml:"verbose"`        // 0 = off, >0 = show tool output up to N lines
		// GenerationStats asks the endpoint for per-request cost and native
		// token counts after each answer. It is an OpenRouter endpoint
		// (/generation) and returns 404 everywhere else, so it is off by
		// default: leaving it on doubles the HTTP requests per iteration.
		GenerationStats bool   `yaml:"generation_stats"`
		BenchmarkCmd    string `yaml:"benchmark_cmd"` // External command for benchmarks (use {prompt} placeholder)
	} `yaml:"llm"`

	Workspace struct {
		Root                  string   `yaml:"root"`
		Lock                  bool     `yaml:"lock"`             // Refuse to start when another agent is working in this directory (default: false)
		PathSafetyMode        string   `yaml:"path_safety_mode"` // "allow", "block", "warn", "ask_once", "ask_always"
		AllowOutsideWorkspace bool     `yaml:"allow_outside_workspace"`
		AllowedPaths          []string `yaml:"allowed_paths"`
		AllowedReadPaths      []string `yaml:"allowed_read_paths"`
		DeniedPaths           []string `yaml:"denied_paths"`
	} `yaml:"workspace"`

	Agent AgentConfig `yaml:"agent"`

	Backtrack BacktrackConfig `yaml:"backtrack"`

	Tools ToolsConfig `yaml:"tools"`

	Prompts PromptsConfig `yaml:"prompts"`

	Safety SafetyConfig `yaml:"safety"`

	Diagnostics DiagnosticsConfig `yaml:"diagnostics"`

	Thinkbench ThinkbenchConfig `yaml:"thinkbench"`

	MCP MCPConfig `yaml:"mcp"`

	UI UIConfig `yaml:"ui"`

	// Models is the optional multi-model catalog (docs/model-selection.md).
	// Order is stable: :m1 is the first entry, :m2 the second. When absent,
	// ModelList synthesizes a one-entry catalog from the llm: block, so old
	// configs behave exactly as before.
	Models []ModelEntry `yaml:"models"`
	// DefaultModel selects the catalog row a fresh session starts on: a
	// 1-based index, id, display name or wire model id (see ResolveModel).
	// Empty means the row matching llm.model, else the first row.
	DefaultModel string `yaml:"default_model"`
	// DefaultEffortValue is the effort fallback after an entry's own
	// `default: true` row and llm.reasoning_effort.
	DefaultEffortValue string `yaml:"default_effort"`
}

// UIConfig controls terminal output styling.
type UIConfig struct {
	// Markdown selects when the final answer is styled as markdown:
	// "auto" (the default), "always", or "never". Auto styles only on a
	// terminal; piped output and --json stay raw markdown. Empty and
	// unknown values are auto.
	Markdown string `yaml:"markdown"`
}

// AgentConfig configures the agent loop and startup instruction sources.
type AgentConfig struct {
	// Profile says how much the loop should compensate for the model.
	//
	// "weak" keeps every mechanism that exists to catch a model getting
	// confused: backtracking away from a bad tool call, the duplicate-call
	// kill switch, the confirm handshake before an edit is applied, scraping
	// tool calls out of prose, fuzzy matching and indentation repair, asking
	// the model to explain itself after an anomaly, and retrying an empty
	// answer. Each of those was written for a model that needed it.
	//
	// "strong", the default, skips all of it. On a model that does not make
	// those mistakes, each mechanism is a tax: a retry that discards good
	// work, a handshake that costs two round trips per edit, a fuzzy match
	// that silently edits the wrong lines.
	Profile             string                    `yaml:"profile"`
	MaxIterations       int                       `yaml:"max_tool_iterations"`
	AgentFile           string                    `yaml:"agent_file"`
	ProjectInstructions ProjectInstructionsConfig `yaml:"project_instructions"`
}

// IsStrong reports whether the loop should skip the machinery that exists to
// compensate for a weak model. It is the default.
func (a AgentConfig) IsStrong() bool { return !strings.EqualFold(a.Profile, "weak") }

// ProjectInstructionsConfig controls the Claude-Code-style project instruction
// file loaded for headless -p runs.
type ProjectInstructionsConfig struct {
	// Enabled defaults to true when unset, so an existing CLAUDE.md is picked up.
	Enabled *bool `yaml:"enabled"`
	// Path is resolved relative to the directory kvit-coder was launched from.
	// Defaults to "CLAUDE.md".
	Path string `yaml:"path"`
}

// IsEnabled reports whether project instructions should be loaded.
func (p ProjectInstructionsConfig) IsEnabled() bool {
	return p.Enabled == nil || *p.Enabled
}

// PathOrDefault returns the configured project instructions path.
func (p ProjectInstructionsConfig) PathOrDefault() string {
	if p.Path == "" {
		return "CLAUDE.md"
	}
	return p.Path
}

// MCPConfig configures the Model Context Protocol (MCP) client. MCP lets
// kvit-coder connect to external tool servers (over stdio or HTTP) and surface
// each server's tools to the model as ordinary agent tools. The section is
// absent by default, so existing configs are unaffected and the feature is
// zero-cost when unused. See docs/archive/mcp-plan.md.
type MCPConfig struct {
	// Enabled is the group toggle. When false (default) no servers are dialed.
	Enabled bool `yaml:"enabled"`
	// StartupTimeout is the per-server connect + tools/list deadline, in seconds
	// (default 20). A server that exceeds it is logged and skipped, never fatal.
	StartupTimeout int `yaml:"startup_timeout"`
	// CallTimeout is the default per-tool-call deadline, in seconds (default 120),
	// overridable per server. MCP calls are exempt from the agent loop's blanket
	// 15s tool timeout and use this deadline instead.
	CallTimeout int `yaml:"call_timeout"`
	// Confirm is the default trust policy for MCP tool calls, overridable per
	// server: "block" | "ask_once" | "ask_always" | "trust" (default "ask_once").
	Confirm string `yaml:"confirm"`
	// SanitizeSchemas flattens/strips JSON Schema constructs ($ref, oneOf, allOf,
	// ...) that some grammar-constrained tool-calling templates reject. Opt-in so
	// faithful schemas are the default.
	SanitizeSchemas bool `yaml:"sanitize_schemas"`
	// Servers lists the configured MCP servers.
	Servers []MCPServerConfig `yaml:"servers"`
}

// MCPServerConfig configures a single MCP server.
type MCPServerConfig struct {
	// Name namespaces the server's tools (mcp.<name>.<tool>); must be unique.
	Name string `yaml:"name"`
	// Enabled opts this server in. Zero value false = skipped (opt-in).
	Enabled bool `yaml:"enabled"`
	// Transport selects the wire protocol: "stdio" (default) or "http".
	Transport string `yaml:"transport"`

	// stdio transport
	Command string   `yaml:"command"` // executable to spawn
	Args    []string `yaml:"args"`    // arguments
	Env     []string `yaml:"env"`     // extra "KEY=VALUE" entries appended to inherited env
	// Cwd is the working directory for the stdio subprocess. Empty defaults to
	// the workspace root, so a server that keys off its own working directory
	// (e.g. a codebase indexer) targets the project kvit-coder operates on rather
	// than wherever kvit-coder happened to be launched. A relative path is
	// resolved against the workspace root.
	Cwd string `yaml:"cwd"`

	// http transport
	URL     string   `yaml:"url"`     // streamable-HTTP endpoint
	Headers []string `yaml:"headers"` // extra "Key=Value" request headers (supports ${VAR})

	// CallTimeout optionally overrides MCPConfig.CallTimeout for this server, in seconds.
	CallTimeout int `yaml:"call_timeout"`
	// Confirm optionally overrides MCPConfig.Confirm for this server.
	Confirm string `yaml:"confirm"`
	// Tools optionally filters which of the server's tools are exposed.
	Tools MCPToolFilter `yaml:"tools"`
}

// MCPToolFilter optionally restricts which tools a server exposes. Names are
// matched against the raw (server-side) tool name. An empty Allow list allows
// all tools; Deny is applied after Allow.
type MCPToolFilter struct {
	Allow []string `yaml:"allow"`
	Deny  []string `yaml:"deny"`
}

// GetTransport returns the transport, defaulting to "stdio".
func (s *MCPServerConfig) GetTransport() string {
	if s.Transport == "" {
		return "stdio"
	}
	return s.Transport
}

// GetCallTimeout returns the per-call deadline for this server in seconds,
// preferring the server override, then the group default, then 120.
func (s *MCPServerConfig) GetCallTimeout(group *MCPConfig) int {
	if s.CallTimeout > 0 {
		return s.CallTimeout
	}
	return group.GetCallTimeout()
}

// GetConfirm returns the confirm policy for this server, preferring the server
// override, then the group default.
func (s *MCPServerConfig) GetConfirm(group *MCPConfig) string {
	if s.Confirm != "" {
		return s.Confirm
	}
	return group.GetConfirm()
}

// GetStartupTimeout returns the per-server connect deadline in seconds, default 20.
func (m *MCPConfig) GetStartupTimeout() int {
	if m.StartupTimeout <= 0 {
		return 20
	}
	return m.StartupTimeout
}

// GetCallTimeout returns the default per-call deadline in seconds, default 120.
func (m *MCPConfig) GetCallTimeout() int {
	if m.CallTimeout <= 0 {
		return 120
	}
	return m.CallTimeout
}

// GetConfirm returns the default confirm policy, default "ask_once".
func (m *MCPConfig) GetConfirm() string {
	if m.Confirm == "" {
		return "ask_once"
	}
	return m.Confirm
}

// ThinkbenchConfig configures the thinkbench benchmark family (autonomous coding
// agent tasks graded by a held-out python grader). See thinkbench-plan.md.
type ThinkbenchConfig struct {
	// SuiteDir overrides the auto-discovered suite directory
	// (default: benchmarks/thinkbench/suite, discovered from the repo root).
	SuiteDir string `yaml:"suite_dir"`
	// TimeoutPerRun is the per-task agent-loop timeout in seconds (default: 600).
	TimeoutPerRun int `yaml:"timeout_per_run"`
	// UVPython pins the python version for the uv env (default: "3.11").
	UVPython string `yaml:"uv_python"`
	// Sandbox selects the OS sandbox for agent run_command:
	// "auto" (default; bwrap→firejail→none), "bwrap", "firejail", "none", or
	// "require" (auto, but hard-fail if no sandbox tool is available).
	Sandbox string `yaml:"sandbox"`
	// Types restricts which task types run (default: all graded types, excluding observed).
	Types []string `yaml:"types"`
	// Trials is the default trials-per-task (overridden by -n on the command line).
	Trials int `yaml:"trials"`
	// PipPackages are installed into the shared uv env at preflight so agents can
	// use common test deps without reaching outside the sandbox to find them.
	// nil (unset) defaults to ["pytest"]; an explicit empty list installs nothing.
	PipPackages []string `yaml:"pip_packages"`
	// IncludeObserved runs the ungraded ambiguous-spec tasks too (persist-only).
	IncludeObserved bool `yaml:"include_observed"`
}

// GetTimeoutPerRun returns the per-run timeout in seconds, defaulting to 600.
func (t *ThinkbenchConfig) GetTimeoutPerRun() int {
	if t.TimeoutPerRun <= 0 {
		return 600
	}
	return t.TimeoutPerRun
}

// GetUVPython returns the pinned python version, defaulting to "3.11".
func (t *ThinkbenchConfig) GetUVPython() string {
	if t.UVPython == "" {
		return "3.11"
	}
	return t.UVPython
}

// GetSandbox returns the sandbox mode, defaulting to "auto".
func (t *ThinkbenchConfig) GetSandbox() string {
	if t.Sandbox == "" {
		return "auto"
	}
	return t.Sandbox
}

// GetPipPackages returns the packages to pre-install into the uv env. An unset
// (nil) list defaults to ["pytest"]; an explicit empty list installs nothing.
func (t *ThinkbenchConfig) GetPipPackages() []string {
	if t.PipPackages == nil {
		return []string{"pytest"}
	}
	return t.PipPackages
}

// DiagnosticsConfig configures anomaly interrogation/logging (Improvement 2).
// When enabled, the agent loop asks the model to explain "odd" behavior (loops,
// repeated identical calls, ignored confirm/cancel) and logs the Q&A to a side
// channel for offline analysis. Observability only: it does not alter recovery.
type DiagnosticsConfig struct {
	// InterrogateOnAnomaly enables interrogation. Default off.
	InterrogateOnAnomaly bool `yaml:"interrogate_on_anomaly"`
	// InterrogateTriggers selects which anomaly triggers fire interrogation. Empty = all.
	// Valid values: duplicate_call, pending_blocked, alternating_loop, fatal, backtrack.
	InterrogateTriggers []string `yaml:"interrogate_triggers"`
	// InterrogateIdenticalThreshold is the consecutive-identical-call count that triggers
	// interrogation. Default 2.
	InterrogateIdenticalThreshold int `yaml:"interrogate_identical_threshold"`
	// InterrogateMaxPerTask hard-caps interrogations per task to avoid interrogation loops.
	// Default 3.
	InterrogateMaxPerTask int `yaml:"interrogate_max_per_task"`
	// InterrogateLogDir is where JSONL episode logs are written. Default
	// "benchmarks/.kvit-coder-benchmark/interrogations".
	InterrogateLogDir string `yaml:"interrogate_log_dir"`
	// InterrogateThenInject is reserved for the later recovery variant; no-op in v1.
	InterrogateThenInject bool `yaml:"interrogate_then_inject"`
}

// GetInterrogateIdenticalThreshold returns the identical-call threshold, default 2.
func (d *DiagnosticsConfig) GetInterrogateIdenticalThreshold() int {
	if d.InterrogateIdenticalThreshold <= 0 {
		return 2
	}
	return d.InterrogateIdenticalThreshold
}

// GetInterrogateMaxPerTask returns the per-task interrogation cap, default 3.
func (d *DiagnosticsConfig) GetInterrogateMaxPerTask() int {
	if d.InterrogateMaxPerTask <= 0 {
		return 3
	}
	return d.InterrogateMaxPerTask
}

// GetInterrogateLogDir returns the log directory, with a default under benchmarks/.
func (d *DiagnosticsConfig) GetInterrogateLogDir() string {
	if d.InterrogateLogDir == "" {
		return "benchmarks/.kvit-coder-benchmark/interrogations"
	}
	return d.InterrogateLogDir
}

// TriggerEnabled reports whether the given anomaly trigger should fire interrogation.
// An empty trigger list means all triggers are enabled.
func (d *DiagnosticsConfig) TriggerEnabled(trigger string) bool {
	if len(d.InterrogateTriggers) == 0 {
		return true
	}
	for _, t := range d.InterrogateTriggers {
		if t == trigger {
			return true
		}
	}
	return false
}

// SafetyConfig holds enhanced safety mode configuration
type SafetyConfig struct {
	StrictMode   bool              `yaml:"strict_mode"`   // Fail-closed on parse errors
	ParanoidMode bool              `yaml:"paranoid_mode"` // Aggressive restrictions
	Audit        AuditConfig       `yaml:"audit"`
	Git          GitSafetyConfig   `yaml:"git"`
	Rm           RmSafetyConfig    `yaml:"rm"`
	Interpreters InterpreterConfig `yaml:"interpreters"`
}

// AuditConfig configures safety audit logging
type AuditConfig struct {
	Enabled       bool   `yaml:"enabled"`
	LogDir        string `yaml:"log_dir"`
	RedactSecrets bool   `yaml:"redact_secrets"`
	RetentionDays int    `yaml:"retention_days"`
}

// GitSafetyConfig configures git operation safety
type GitSafetyConfig struct {
	BlockPush             bool `yaml:"block_push"`
	BlockHardReset        bool `yaml:"block_hard_reset"`
	BlockCheckoutDiscard  bool `yaml:"block_checkout_discard"`
	BlockStashDrop        bool `yaml:"block_stash_drop"`
	BlockCleanForce       bool `yaml:"block_clean_force"`
	WarnBranchForceDelete bool `yaml:"warn_branch_force_delete"`
}

// RmSafetyConfig configures rm command safety
type RmSafetyConfig struct {
	AllowInTemp         bool `yaml:"allow_in_temp"`
	AllowInWorkspaceCwd bool `yaml:"allow_in_workspace_cwd"`
	BlockWorkspaceRoot  bool `yaml:"block_workspace_root"`
}

// InterpreterConfig configures interpreter one-liner blocking
type InterpreterConfig struct {
	BlockOneLiners bool     `yaml:"block_one_liners"`
	Allowed        []string `yaml:"allowed"`
}

// PromptsConfig configures prompt template system
type PromptsConfig struct {
	// UseTemplates enables template-based prompt generation (default: false)
	UseTemplates bool `yaml:"use_templates"`
	// TemplatesDir overrides embedded templates with filesystem directory
	TemplatesDir string `yaml:"templates_dir"`
	// HotReload enables template reloading on each request (dev mode)
	HotReload bool `yaml:"hot_reload"`
}

// ToolsConfig holds per-tool configuration with explicit enable/disable
type ToolsConfig struct {
	Read        ReadToolConfig        `yaml:"read"`
	Images      ImagesConfig          `yaml:"images"`
	Edit        EditToolConfig        `yaml:"edit"`
	RestoreFile RestoreFileToolConfig `yaml:"restore_file"`
	Search      SearchToolConfig      `yaml:"search"`
	Shell       ShellToolConfig       `yaml:"shell"`
	Question    QuestionToolConfig    `yaml:"question"`
	Procs       ProcsToolsConfig      `yaml:"procs"`
	Batch       BatchToolConfig       `yaml:"batch"`
	Plan        PlanToolsConfig       `yaml:"plan"`
	Checkpoint  CheckpointToolsConfig `yaml:"checkpoint"`
	Tasks       TasksToolsConfig      `yaml:"tasks"`
	Web         WebToolsConfig        `yaml:"web"`

	// Safety confirmations (runtime only, not persisted)
	SafetyConfirmations map[string]SafetyConfirmation `yaml:"-"`
}

// WebToolsConfig configures the tools that reach the network: Web.search, which
// queries the Brave Search API, and Web.fetch, which retrieves one page and
// converts it to markdown. The key is named rather than written here, the way
// llm.api_key_env does it, because the same key is usually shared with other
// programs and this file is checked in.
type WebToolsConfig struct {
	APIKeyEnv string `yaml:"api_key_env"`
	BaseURL   string `yaml:"base_url"`
	UsageLog  string `yaml:"usage_log"`

	Search WebSearchToolConfig `yaml:"search"`
	Fetch  WebFetchToolConfig  `yaml:"fetch"`
}

// WebSearchToolConfig configures Web.search.
//
// MaxAttempts is a setting rather than a constant because the right value
// depends on a rate limit this program does not control. Four suits one request
// per second, where a collision clears in about a second and retrying quietly
// beats handing the model a failure to reason about; a backend without a
// per-second limit wants one.
type WebSearchToolConfig struct {
	Enabled     bool `yaml:"enabled"`
	Count       int  `yaml:"count"`
	MaxAttempts int  `yaml:"max_attempts"`
	Timeout     int  `yaml:"timeout"`
}

// WebFetchToolConfig configures Web.fetch.
type WebFetchToolConfig struct {
	Enabled   bool   `yaml:"enabled"`
	Timeout   int    `yaml:"timeout"`
	MaxBytes  int64  `yaml:"max_bytes"`
	UserAgent string `yaml:"user_agent"`
}

// ReadToolConfig configures the read tool
type ReadToolConfig struct {
	Enabled         bool  `yaml:"enabled"`
	MaxFileSizeKB   int   `yaml:"max_file_size_kb"`
	MaxReadSizeKB   int   `yaml:"max_read_size_kb"`
	MaxPartialLines int   `yaml:"max_partial_lines"`
	ShowLineNumbers *bool `yaml:"show_line_numbers"` // nil = default true, for backward compat
}

// ImagesConfig configures image ingestion: the ReadImage tool and the -image
// flag share the caps. The tool is opt-in like every other tool; -image works
// whenever it is passed, because naming a file is its own authorization.
type ImagesConfig struct {
	Enabled      bool `yaml:"enabled"`
	MaxSizeMB    int  `yaml:"max_size_mb"`
	MaxDimension int  `yaml:"max_dimension"`
}

// EditToolConfig configures the edit tool
type EditToolConfig struct {
	Enabled               bool    `yaml:"enabled"`
	Mode                  string  `yaml:"mode"` // "lines" (default), "searchreplace", or "patch"
	MaxFileSizeKB         int     `yaml:"max_file_size_kb"`
	PreviewMode           bool    `yaml:"preview_mode"`            // enables edit.confirm/edit.cancel
	ReadBeforeEditMsgs    int     `yaml:"read_before_edit_msgs"`   // require read within N messages before edit (0 = disabled)
	PendingConfirmRetries int     `yaml:"pending_confirm_retries"` // max retries when LLM ignores confirm/cancel (0 = disabled, default 5)
	FuzzyThreshold        float64 `yaml:"fuzzy_threshold"`         // searchreplace mode: gates ONLY the fuzzy level (0 = fuzzy off). Whitespace-normalization levels still run unless ExactMatchOnly is set.

	// ExactMatchOnly forces searchreplace matching to byte-exact (Level 0) only:
	// no rstrip, no leading/trailing whitespace normalization, no fuzzy fallback.
	// A non-exact search returns a clean no-match instead of being silently
	// re-anchored to a normalized (line-start) position. Default off = today's
	// cascade behavior for all existing configs.
	ExactMatchOnly bool `yaml:"exact_match_only"`

	// SmartFirstLineIndent enables first-line indentation auto-correction in lines mode
	// (the "autoindent" feature). Only active when PreviewMode is also true, since the
	// correction is only reversible via Edit.undo_autoindent during preview. Default off.
	SmartFirstLineIndent bool `yaml:"smart_first_line_indent"`
	// MaxAutoindentFix is the dedent-guard threshold: the maximum under-indent deficit (in
	// chars) that will be auto-corrected. Default 1 = fix only exact off-by-one under-indents.
	MaxAutoindentFix int `yaml:"max_autoindent_fix"`

	// ExplicitDelete enables the DeleteLines tool and flips lines-mode empty new_text
	// semantics: with this on, an Edit replace with new_text="" blanks the addressed
	// line(s) (keeps them) instead of deleting, and deletes go through DeleteLines.
	// Default off = today's behavior exactly ("" deletes, no DeleteLines tool).
	ExplicitDelete bool `yaml:"explicit_delete"`
}

// RestoreFileToolConfig configures the restore_file tool
type RestoreFileToolConfig struct {
	Enabled bool `yaml:"enabled"`
}

// SearchToolConfig configures the search tool
type SearchToolConfig struct {
	Enabled           bool `yaml:"enabled"`
	MaxSnippetResults int  `yaml:"max_snippet_results"` // Show full snippets up to this many (default: 20)
	MaxCompactResults int  `yaml:"max_compact_results"` // Show file:line:char up to this many (default: 100)
	// Above max_compact_results: save to temp file, show truncated
}

// QuestionToolConfig configures the Question tool, which asks the person
// running kvit-coder something and waits for the answer.
type QuestionToolConfig struct {
	Enabled bool `yaml:"enabled"`
	// Timeout is how long a run with no terminal waits for an answer to appear
	// in the session inbox, in seconds. Zero, the default, falls back at once,
	// so a benchmark or scripted run never hangs on a question. A supervised
	// headless run sets it to a few minutes.
	Timeout int `yaml:"timeout"`
}

// BatchToolConfig configures the Batch tool, which runs several independent
// tool calls in one request.
type BatchToolConfig struct {
	Enabled bool `yaml:"enabled"`
}

// ProcsToolsConfig configures the tools for work that outlives a turn's
// iterations: Shell.start, Shell.output, Shell.status, Shell.list,
// Shell.kill, Shell.tune, Observe.wait and Observe.add. Ephemeral processes
// die with the turn; persistent ones survive it.
type ProcsToolsConfig struct {
	Enabled bool `yaml:"enabled"`
	// KillOnExit stops everything this session started when the turn is
	// interrupted. On by default: an interrupt that leaves a dev server
	// running is a surprise. Turn it off to keep long-lived services alive
	// across interrupts.
	KillOnExit *bool `yaml:"kill_on_exit"`
}

// ShouldKillOnExit reports whether an interrupt stops background processes.
func (p *ProcsToolsConfig) ShouldKillOnExit() bool {
	return p.KillOnExit == nil || *p.KillOnExit
}

// ShellToolConfig configures the shell tool
type ShellToolConfig struct {
	Enabled            bool     `yaml:"enabled"`
	AllowedCommands    []string `yaml:"allowed_commands"`    // allowlist (empty = allow all)
	DisallowedCommands []string `yaml:"disallowed_commands"` // blocklist (checked after allowlist)

	// InjectEnv, when non-empty, replaces the child process environment for shell
	// commands with os.Environ() plus these "KEY=VALUE" entries. Used by the
	// thinkbench harness to point python3/pip at the uv environment without naming
	// an outside path in the command string. Empty = inherit parent env (default).
	InjectEnv []string `yaml:"-"`
	// ExecPrefix, when non-empty, is prepended (as an argv) before "sh -c <command>"
	// when running shell commands. Used by the thinkbench harness to wrap agent
	// commands in an OS sandbox (e.g. bwrap). Empty = run "sh -c" directly (default).
	ExecPrefix []string `yaml:"-"`
	// AllowInterpreters, when true, lifts the default block on interpreter
	// one-liners (python -c, node -e, perl -e, ...). Set by the thinkbench harness,
	// whose tasks are interpreter-driven and sandboxed, and opt-in via benchmark
	// configs (e.g. SWE-bench, which runs in a Docker container and needs python -c
	// to introspect the codebase). Default false (blocked) for normal use.
	AllowInterpreters bool `yaml:"allow_interpreters"`

	// AllowWithoutAsking answers yes to every command that would otherwise
	// stop and ask, except the few that destroy something unrecoverably
	// (dd, mkfs and its family). --yolo sets it. What is refused outright
	// stays refused: this answers questions, and a denial was never one.
	//
	// It matters most with no terminal, where an unanswered question is a
	// refusal rather than a pause: without this, a headless --yolo run fails
	// on the first curl it needs.
	AllowWithoutAsking bool `yaml:"allow_without_asking"`

	// DefaultTimeout is how long a shell command may run when the call does
	// not ask for a timeout, in seconds. Default 120.
	DefaultTimeout int `yaml:"default_timeout"`
	// MaxTimeout caps what a call may ask for, in seconds. Default 600.
	MaxTimeout int `yaml:"max_timeout"`
}

// PlanToolsConfig configures all plan.* tools as a group
type PlanToolsConfig struct {
	Enabled       bool   `yaml:"enabled"`        // group toggle for all plan.* tools
	Mode          string `yaml:"mode"`           // "write" (default, full-list rewrite) or "incremental" (legacy 5-tool)
	InjectionMode string `yaml:"injection_mode"` // "none" or "every_step"
}

// GetPlanMode returns the plan tool mode, defaulting to "write" (the
// OpenCode/Codex-style full-list rewrite tool) when unset.
func (p *PlanToolsConfig) GetPlanMode() string {
	if p.Mode == "" {
		return "write"
	}
	return p.Mode
}

// CheckpointToolsConfig configures all checkpoint.* tools as a group
type CheckpointToolsConfig struct {
	Enabled          bool     `yaml:"enabled"`           // group toggle for all checkpoint.* tools
	MaxTurns         int      `yaml:"max_turns"`         // max checkpoints before rotating (default: 100)
	TempDir          string   `yaml:"temp_dir"`          // base directory for checkpoint storage
	MaxFileSizeKB    int      `yaml:"max_file_size_kb"`  // skip files larger than this (default: 1024)
	ExcludedPatterns []string `yaml:"excluded_patterns"` // don't track these files
}

// TasksToolsConfig configures Tasks.* tools for context compression
type TasksToolsConfig struct {
	Enabled  bool `yaml:"enabled"`  // Enable Tasks tools (disables Plan.* and Checkpoint.* tools)
	Collapse bool `yaml:"collapse"` // Stage 2: Enable Tasks.Collapse (requires enabled=true)
	Plan     bool `yaml:"plan"`     // Stage 3: Enable plan-based tools (requires enabled=true)

	// Runtime notice thresholds
	TaskWarnTurns       int  `yaml:"task_warn_turns"`       // Warn after N turns in task (default: 5)
	TaskCriticalTurns   int  `yaml:"task_critical_turns"`   // Critical warning after N turns (default: 10)
	ContextCapacityWarn int  `yaml:"context_capacity_warn"` // Warn at N% context capacity (default: 80)
	MaxNestedDepth      int  `yaml:"max_nested_depth"`      // Max task nesting depth (default: 2)
	NotifyFileChanges   bool `yaml:"notify_file_changes"`   // Notify about file changes in task (default: true)
}

// BacktrackConfig configures the backtrack error handling mode
type BacktrackConfig struct {
	Enabled           bool `yaml:"enabled"`             // Enable backtrack mode (default: true)
	MaxRetries        int  `yaml:"max_retries"`         // Max retries at same history point (default: 5)
	InjectUserMessage bool `yaml:"inject_user_message"` // On limit reached: backtrack + inject user message instead of error-in-history
}

// GetShowLineNumbers returns whether line numbers should be shown in Read output.
// Defaults to true for backward compatibility.
func (r *ReadToolConfig) GetShowLineNumbers() bool {
	if r.ShowLineNumbers == nil {
		return true // Default: show line numbers
	}
	return *r.ShowLineNumbers
}

// GetEditMode returns the edit mode, defaulting to "lines" for backward compatibility.
func (e *EditToolConfig) GetEditMode() string {
	if e.Mode == "" {
		return "lines"
	}
	return e.Mode
}

// GetMaxAutoindentFix returns the autoindent dedent-guard threshold, defaulting to 1
// (fix only exact off-by-one under-indents) when unset or non-positive.
func (e *EditToolConfig) GetMaxAutoindentFix() int {
	if e.MaxAutoindentFix <= 0 {
		return 1
	}
	return e.MaxAutoindentFix
}

// SafetyConfirmation tracks user confirmations for path access
// This is memory-only and does not persist across sessions
type SafetyConfirmation struct {
	ToolName  string    `yaml:"-"`
	Path      string    `yaml:"-"`
	Timestamp time.Time `yaml:"-"`
}

// runIDVar is the environment variable llm.headers can reference as
// ${KVIT_RUN_ID} to get a value unique to this kvit-coder process.
const runIDVar = "KVIT_RUN_ID"

var runIDOnce sync.Once

// ensureRunID gives this process an ID that no other kvit-coder running at the
// same time will have. Endpoints that route by a session header put every
// request with the same header value on one backend, so two agents sharing an
// ID compete for the same prompt cache instead of each keeping their own. An
// ID already in the environment is left alone, which lets a caller pin one
// across several runs on purpose.
func ensureRunID() {
	runIDOnce.Do(func() {
		if os.Getenv(runIDVar) != "" {
			return
		}
		var buf [6]byte
		if _, err := rand.Read(buf[:]); err != nil {
			os.Setenv(runIDVar, strconv.FormatInt(time.Now().UnixNano(), 36))
			return
		}
		os.Setenv(runIDVar, hex.EncodeToString(buf[:]))
	})
}

// ModelDisplay is the model name as shown in startup banners and :config:
// "model:effort" when llm.reasoning_effort is set, plain "model" otherwise,
// so the banner reflects what the Responses backend is actually asked for.
func (c *Config) ModelDisplay() string {
	if effort := c.LLM.ReasoningEffort; effort != "" {
		return c.LLM.Model + ":" + effort
	}
	return c.LLM.Model
}

// ReasoningSummaryOrDefault says what to ask the provider for. Turning on
// merge_thinking without this asks the model to think and then throws the
// readable part away, so an unset value follows merge_thinking.
func (c *Config) ReasoningSummaryOrDefault() string {
	switch strings.ToLower(c.LLM.ReasoningSummary) {
	case "":
		if c.LLM.MergeThinking {
			return "auto"
		}
		return ""
	case "off", "none", "false":
		return ""
	default:
		return c.LLM.ReasoningSummary
	}
}

// LLMHeaders parses llm.headers into the header map the LLM client wants,
// expanding ${VAR} references against the environment so a session or routing
// header can come from a secret the config does not contain. ${KVIT_RUN_ID}
// expands to an ID unique to this process unless the environment already sets
// one.
func (c *Config) LLMHeaders() map[string]string {
	if len(c.LLM.Headers) == 0 {
		return nil
	}
	ensureRunID()
	headers := make(map[string]string, len(c.LLM.Headers))
	for _, h := range c.LLM.Headers {
		k, v, ok := strings.Cut(h, "=")
		if !ok {
			continue
		}
		headers[strings.TrimSpace(k)] = os.ExpandEnv(strings.TrimSpace(v))
	}
	return headers
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Apply environment overrides
	if cfg.LLM.APIKeyEnv != "" {
		if key := os.Getenv(cfg.LLM.APIKeyEnv); key != "" {
			cfg.LLM.APIKey = key
		}
	}

	// Convert workspace root to absolute path
	if cfg.Workspace.Root != "" {
		absRoot, err := filepath.Abs(cfg.Workspace.Root)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve workspace root: %w", err)
		}
		cfg.Workspace.Root = absRoot
	}

	// The models: catalog is validated here so a typo fails at startup with
	// the file path and entry number, not mid-session at the API.
	if err := cfg.validateModels(path); err != nil {
		return nil, err
	}

	// Initialize runtime fields
	cfg.Tools.SafetyConfirmations = make(map[string]SafetyConfirmation)

	// Set default file size limits for read tool
	if cfg.Tools.Read.MaxFileSizeKB == 0 {
		cfg.Tools.Read.MaxFileSizeKB = 128
	}
	if cfg.Tools.Read.MaxReadSizeKB == 0 {
		cfg.Tools.Read.MaxReadSizeKB = 24
	}
	if cfg.Tools.Read.MaxPartialLines == 0 {
		cfg.Tools.Read.MaxPartialLines = 150
	}

	// Image caps shared by the ReadImage tool and the -image flag. Zero
	// selects the llm defaults, so an empty images: section just works.
	if cfg.Tools.Images.MaxSizeMB == 0 {
		cfg.Tools.Images.MaxSizeMB = llm.DefaultImageMaxSizeMB
	}
	if cfg.Tools.Images.MaxDimension == 0 {
		cfg.Tools.Images.MaxDimension = llm.DefaultImageMaxDim
	}
	// Set default file size limit for edit tool
	if cfg.Tools.Edit.MaxFileSizeKB == 0 {
		cfg.Tools.Edit.MaxFileSizeKB = 128
	}

	// Set default checkpoint settings
	if cfg.Tools.Checkpoint.MaxFileSizeKB == 0 {
		cfg.Tools.Checkpoint.MaxFileSizeKB = 1024
	}
	if cfg.Tools.Checkpoint.MaxTurns == 0 {
		cfg.Tools.Checkpoint.MaxTurns = 100
	}

	// The strong profile turns off the machinery written to compensate for a
	// weak model. Applying it here, once, rather than at each of the twenty
	// places that read these settings, means none of them can be missed — and
	// it is why "profile: weak" reproduces the old behaviour exactly: it
	// changes nothing at all.
	if cfg.Agent.IsStrong() {
		// Backtracking discards an assistant turn and retries after a semantic
		// tool error. On a model that reads the error and corrects itself, it
		// throws away work and pays for the same tokens twice.
		cfg.Backtrack.Enabled = false
		// The confirm handshake makes every edit two tool calls, and with it
		// go the indentation repair and first-line-indent guessing that only
		// run alongside it.
		cfg.Tools.Edit.PreviewMode = false
		cfg.Tools.Edit.SmartFirstLineIndent = false
		cfg.Tools.Edit.MaxAutoindentFix = 0
		// Fuzzy matching edits the closest thing it can find to what the model
		// asked for, which is a silent wrong edit when the model was right and
		// the file had moved on.
		cfg.Tools.Edit.ExactMatchOnly = true
		cfg.Tools.Edit.FuzzyThreshold = 0
		// Interrogation makes an extra model call to ask the model why it did
		// something odd. It is a diagnostic for a model that does odd things.
		cfg.Diagnostics.InterrogateOnAnomaly = false
	}

	if cfg.LLM.RequestTimeout == 0 {
		cfg.LLM.RequestTimeout = 600
	}

	// Set default shell timeouts
	if cfg.Tools.Shell.DefaultTimeout == 0 {
		cfg.Tools.Shell.DefaultTimeout = 120
	}
	if cfg.Tools.Shell.MaxTimeout == 0 {
		cfg.Tools.Shell.MaxTimeout = 600
	}

	// Set default path safety mode
	if cfg.Workspace.PathSafetyMode == "" {
		cfg.Workspace.PathSafetyMode = "ask_once"
	}

	// Set default backtrack settings
	// Backtracking is disabled by default; must be explicitly enabled in config
	if cfg.Backtrack.MaxRetries == 0 {
		cfg.Backtrack.MaxRetries = 5 // Default max retries if enabled
	}

	// Set default Tasks tools settings
	if cfg.Tools.Tasks.TaskWarnTurns == 0 {
		cfg.Tools.Tasks.TaskWarnTurns = 5
	}
	if cfg.Tools.Tasks.TaskCriticalTurns == 0 {
		cfg.Tools.Tasks.TaskCriticalTurns = 10
	}
	if cfg.Tools.Tasks.ContextCapacityWarn == 0 {
		cfg.Tools.Tasks.ContextCapacityWarn = 80
	}
	if cfg.Tools.Tasks.MaxNestedDepth == 0 {
		cfg.Tools.Tasks.MaxNestedDepth = 2
	}
	// NotifyFileChanges defaults to true (Go zero value is false, so we check if unset)
	// Since YAML unmarshals false as false, we need a different approach
	// For now, we'll leave it as the struct default behavior

	// Safety config defaults (all features disabled for backward compatibility)
	if cfg.Safety.Audit.LogDir == "" {
		cfg.Safety.Audit.LogDir = "~/.kvit-coder/safety-logs"
	}
	if cfg.Safety.Audit.RetentionDays == 0 {
		cfg.Safety.Audit.RetentionDays = 30
	}
	// All other safety features default to false (disabled)

	return &cfg, nil
}

// IsToolEnabled returns true if the tool is enabled in config
func (c *Config) IsToolEnabled(toolName string) bool {
	switch toolName {
	case "read":
		return c.Tools.Read.Enabled
	case "edit":
		return c.Tools.Edit.Enabled
	case "edit.confirm", "edit.cancel":
		return c.Tools.Edit.Enabled && c.Tools.Edit.PreviewMode
	case "restore_file":
		return c.Tools.RestoreFile.Enabled
	case "search":
		return c.Tools.Search.Enabled
	case "shell":
		return c.Tools.Shell.Enabled
	case "question", "Question":
		return c.Tools.Question.Enabled
	case "batch", "Batch":
		return c.Tools.Batch.Enabled
	case "shell.start", "shell.output", "shell.status", "shell.list", "shell.kill",
		"observe.wait", "observe.add":
		return c.Tools.Procs.Enabled
	case "plan.write", "Plan.write":
		// Default full-list rewrite tool; disabled when Tasks tools are enabled
		return c.Tools.Plan.Enabled && !c.Tools.Tasks.Enabled && c.Tools.Plan.GetPlanMode() == "write"
	case "plan.create", "plan.add_step", "plan.complete_step", "plan.remove_step", "plan.move_step":
		// Legacy incremental tools; disabled when Tasks tools are enabled
		return c.Tools.Plan.Enabled && !c.Tools.Tasks.Enabled && c.Tools.Plan.GetPlanMode() == "incremental"
	case "checkpoint.list", "checkpoint.restore", "checkpoint.diff", "checkpoint.undo":
		// User-facing checkpoint tools are disabled when Tasks tools are enabled
		return c.Tools.Checkpoint.Enabled && !c.Tools.Tasks.Enabled
	// Tasks.* tools
	case "Tasks.Start", "Tasks.Finish", "Tasks.AcceptDiff", "Tasks.DeclineDiff",
		"Tasks.RevertFile", "Tasks.RevertToTaskStart":
		return c.Tools.Tasks.Enabled
	case "Tasks.Collapse":
		return c.Tools.Tasks.Enabled && c.Tools.Tasks.Collapse
	case "Tasks.Plan", "Tasks.Add", "Tasks.Skip", "Tasks.Complete", "Tasks.Retry", "Tasks.Replace":
		return c.Tools.Tasks.Enabled && c.Tools.Tasks.Plan
	default:
		return false
	}
}

// CheckPathSafety performs unified path safety checks for all tools
// Behavior controlled by tools.safety.path_safety_mode
// readOnlyTool reports whether a tool only reads, which decides whether
// allowed_read_paths is enough to permit it.
func readOnlyTool(toolName string) bool {
	switch {
	case strings.HasPrefix(toolName, "read"), toolName == "search", toolName == "glob":
		return true
	default:
		return false
	}
}

// pathIsPermitted reports whether the configuration already grants this path,
// through allowed_paths or — for a tool that only reads — allowed_read_paths.
func (c *Config) pathIsPermitted(absPath string, readOnly bool) bool {
	for _, allowed := range c.Workspace.AllowedPaths {
		if allowedAbs, err := filepath.Abs(ExpandHome(allowed)); err == nil && pathWithin(allowedAbs, absPath) {
			return true
		}
	}
	if !readOnly {
		return false
	}
	for _, allowed := range c.Workspace.AllowedReadPaths {
		if allowedAbs, err := filepath.Abs(ExpandHome(allowed)); err == nil && pathWithin(allowedAbs, absPath) {
			return true
		}
	}
	return false
}

func (c *Config) CheckPathSafety(toolName, identifier string) error {
	// "allow" is what --yolo sets: the whole filesystem is in scope, read and
	// write, with nothing asked about and nothing warned about. Everything
	// below is about paths outside the workspace, and in this mode there is no
	// outside.
	if c.Workspace.PathSafetyMode == "allow" {
		return nil
	}

	// For filesystem tools, check if path is outside workspace
	if strings.HasPrefix(toolName, "read") || strings.HasPrefix(toolName, "edit") ||
		strings.HasPrefix(toolName, "restore") || toolName == "glob" || toolName == "shell" ||
		toolName == "search" {
		absPath, outside, err := NormalizeAndValidatePath(c.Workspace.Root, identifier)
		if err != nil || !outside {
			return nil // Not outside or invalid path
		}
		// A path the configuration already allows does not need asking about.
		// This check consulted only the workspace root, so allowed_paths and
		// allowed_read_paths bought nothing here and an explicitly permitted
		// path was queried every time it was touched — including the session's
		// own directory, where spilled tool output lives and which the model is
		// given paths into.
		if c.pathIsPermitted(absPath, readOnlyTool(toolName)) {
			return nil
		}
		identifier = absPath // Use absolute path for key
	}

	// Handle based on safety mode
	mode := c.Workspace.PathSafetyMode
	switch mode {
	case "block":
		return fmt.Errorf("access to path outside workspace blocked (path_safety_mode=block): %s", identifier)

	case "warn":
		fmt.Fprintf(os.Stderr, "⚠️  Warning: %s accesses path outside workspace: %s\n", toolName, identifier)
		return nil

	case "ask_always":
		// Always prompt user
		if confirmed := c.promptForPathAccess(toolName, identifier); !confirmed {
			return fmt.Errorf("user rejected access to path outside workspace: %s", identifier)
		}
		return nil

	case "ask_once":
		fallthrough
	default:
		// Prompt user once per path (default behavior)
		key := fmt.Sprintf("%s:%s", toolName, identifier)
		if _, exists := c.Tools.SafetyConfirmations[key]; !exists {
			if confirmed := c.promptForPathAccess(toolName, identifier); !confirmed {
				return fmt.Errorf("user rejected access to path outside workspace: %s", identifier)
			}
			c.Tools.SafetyConfirmations[key] = SafetyConfirmation{
				ToolName:  toolName,
				Path:      identifier,
				Timestamp: time.Now(),
			}
		}
		return nil
	}
}

// promptForPathAccess prompts the user to confirm path access
// linePrompter, when set, asks whoever is at the terminal a question and
// returns their answer. cmd/kvit-coder installs one that reads through the
// session inbox, which is the process's only reader of the terminal. Opening
// /dev/tty here as well would put two reads on one device, and the keystroke
// goes to whichever of them the kernel picks — the other waits forever.
var (
	linePrompterMu sync.Mutex
	linePrompter   func(prompt string) (string, bool)
)

// SetLinePrompter installs the function used to ask the person at the terminal
// a question. Passing nil restores reading the terminal directly, which is what
// a run with no line reader does.
func SetLinePrompter(fn func(prompt string) (string, bool)) {
	linePrompterMu.Lock()
	defer linePrompterMu.Unlock()
	linePrompter = fn
}

func askLine(prompt string) (string, bool, bool) {
	linePrompterMu.Lock()
	fn := linePrompter
	linePrompterMu.Unlock()
	if fn == nil {
		return "", false, false
	}
	answer, ok := fn(prompt)
	return answer, ok, true
}

func (c *Config) promptForPathAccess(toolName, path string) bool {
	question := fmt.Sprintf("\n⚠️  %s accesses path outside workspace:\n   - %s\n\nAllow this access? [y/N]: ",
		toolName, path)

	if answer, ok, handled := askLine(question); handled {
		if !ok {
			fmt.Fprintln(os.Stderr, "  no answer; access refused")
			return false
		}
		answer = strings.ToLower(strings.TrimSpace(answer))
		return answer == "y" || answer == "yes"
	}

	// No line reader running, so read the terminal directly.
	fmt.Fprint(os.Stderr, question)
	tty, err := os.Open("/dev/tty")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open /dev/tty: %v\n", err)
		return false
	}
	defer tty.Close()

	reader := bufio.NewReader(tty)
	responseByte, err := reader.ReadByte()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read confirmation: %v\n", err)
		return false
	}

	fmt.Fprintf(os.Stderr, "%c\n", responseByte)
	response := strings.ToLower(string(responseByte))
	return response == "y" || response == "yes"
}

// NormalizeAndValidatePath is a helper function to normalize and validate paths
// This should be moved to a shared utility package in a real implementation
func NormalizeAndValidatePath(workspaceRoot, path string) (string, bool, error) {
	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path is outside workspace
	absWorkspace, err := filepath.Abs(workspaceRoot)
	if err != nil {
		return "", false, fmt.Errorf("failed to resolve workspace: %w", err)
	}

	// Normalize paths for comparison
	absPath = filepath.Clean(absPath)
	absWorkspace = filepath.Clean(absWorkspace)

	// Check if path is outside workspace
	if !strings.HasPrefix(absPath, absWorkspace+string(filepath.Separator)) && absPath != absWorkspace {
		return absPath, true, nil // Outside workspace
	}

	return absPath, false, nil // Inside workspace
}

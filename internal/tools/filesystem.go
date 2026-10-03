package tools

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"

	"github.com/pmezard/go-difflib/difflib"
)

// FileReadTracker tracks which files have been read recently for read-before-edit enforcement
type FileReadTracker struct {
	mu           sync.Mutex
	readFiles    []fileReadEntry
	maxEntries   int // How many read entries to keep (corresponds to message count)
	currentMsgID int // Current message ID (incremented each agent iteration)
}

type fileReadEntry struct {
	path      string
	messageID int // Incremented each time a new message batch is processed
	// size/modTime record what the file looked like when it was read, so a
	// later edit can tell the file changed under it. hasStat is false when
	// the stat failed (fail open on stat: the mtime check is skipped for
	// that entry, never failed).
	size    int64
	modTime time.Time
	hasStat bool
}

// RecordRead records that a file was read
func (t *FileReadTracker) RecordRead(path string, messageID int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	entry := fileReadEntry{path: absPath, messageID: messageID}
	if info, statErr := os.Stat(absPath); statErr == nil {
		entry.size = info.Size()
		entry.modTime = info.ModTime()
		entry.hasStat = true
	}

	t.readFiles = append(t.readFiles, entry)

	// Trim old entries if we have too many
	if len(t.readFiles) > t.maxEntries*5 {
		t.readFiles = t.readFiles[len(t.readFiles)-t.maxEntries*5:]
	}
}

// StatChangedSinceRead reports whether the file's current size or modTime
// differs from its most recent read entry. found is false when no entry
// exists for the path or the entry was recorded without a stat — the caller
// skips the mtime check then (fail open on stat, fail closed on prompt).
func (t *FileReadTracker) StatChangedSinceRead(path string) (changed, found bool) {
	t.mu.Lock()
	defer t.mu.Unlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	for i := len(t.readFiles) - 1; i >= 0; i-- {
		entry := t.readFiles[i]
		if entry.path != absPath {
			continue
		}
		if !entry.hasStat {
			return false, false
		}
		info, statErr := os.Stat(absPath)
		if statErr != nil {
			return false, false
		}
		if info.Size() != entry.size || !info.ModTime().Equal(entry.modTime) {
			return true, true
		}
		return false, true
	}
	return false, false
}

// WasReadRecently checks if a file was read within the last N messages
func (t *FileReadTracker) WasReadRecently(path string, currentMessageID, withinMessages int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Normalize path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	minMessageID := currentMessageID - withinMessages
	for _, entry := range t.readFiles {
		if entry.path == absPath && entry.messageID >= minMessageID {
			return true
		}
	}
	return false
}

// CurrentMessageID returns the current message ID
func (t *FileReadTracker) CurrentMessageID() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.currentMsgID
}

// NextMessage increments and returns the new message ID (call for each agent loop iteration)
func (t *FileReadTracker) NextMessage() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.currentMsgID++
	return t.currentMsgID
}

// pendingEdit stores a computed edit waiting to be applied in preview mode
type pendingEdit struct {
	path          string
	fullPath      string
	oldContent    string
	newContent    string
	diff          string
	isNewFile     bool
	editStartLine int // 1-based line number where edit starts in new content
	editEndLine   int // 1-based line number where edit ends in new content
}

// pendingWrite stores a pending write operation waiting for confirmation
type pendingWrite struct {
	path     string
	fullPath string
	content  string
	oldSize  int64 // Size of existing file (for info)
	oldLines int   // Lines in existing file (for info)
}

// PendingEditAutoResolveThreshold is the number of retries before auto-cancelling pending edit
const PendingEditAutoResolveThreshold = 5

// PendingEditEscalateThreshold is the number of retries before escalating error message
const PendingEditEscalateThreshold = 3

// PendingEditMaxIgnoreCount is the maximum number of ignored responses before auto-cancel
const PendingEditMaxIgnoreCount = 5

// CheckPendingEditBlockWithState checks if a tool call should be blocked due to pending edit.
// Uses history-derived state as the source of truth for whether a pending edit exists.
// The toolCtx is used for the diff content in error messages.
func CheckPendingEditBlockWithState(toolName string, state PendingEditState, cfg *config.Config, toolCtx *ToolContext) *ToolError {
	// Use history state as source of truth
	if !state.HasPending {
		return nil // No pending edit according to history, allow all tools
	}

	// The history spans turns and the pending operation itself does not: it is
	// held in this process's memory, and the process that staged it has since
	// exited. When the history says an edit is pending and nothing here holds
	// it, there is nothing to confirm and nothing to cancel — blocking would
	// send the model to Edit.confirm, which answers no_pending_operation, and
	// cost it the turn. Let it through.
	if toolCtx != nil && !toolCtx.HasPendingEdit() && !toolCtx.HasPendingWrite() {
		return nil
	}

	// Allow all confirm/cancel tools (Edit.* and Write.* are synonyms).
	if toolName == "Edit.confirm" || toolName == "Edit.cancel" ||
		toolName == "Write.confirm" || toolName == "Write.cancel" {
		return nil
	}

	pendingPath := state.PendingPath
	blockCount := state.BlockCountSincePending

	// Try to get diff from RAM (may not exist if out of sync)
	pendingDiff := toolCtx.GetPendingEditDiff()

	// Check auto-resolve threshold (based on history count)
	maxIgnoreCount := GetMaxIgnoreCount(cfg)
	if blockCount >= maxIgnoreCount {
		// Auto-cancel the pending edit (clear RAM state)
		toolCtx.ClearPendingEdit()
		// Return RuntimeError (not backtrackable) so LLM can proceed
		return RuntimeError(fmt.Sprintf("AUTO-CANCELLED: Pending edit on '%s' was automatically cancelled after %d ignored responses. You may now proceed with your intended action.", pendingPath, blockCount))
	}

	// Helper to format blocked message with diff (if available)
	formatBlocked := func(extraMsg string) string {
		base := fmt.Sprintf("BLOCKED: Your %s call was blocked due to a pending edit on '%s'.", toolName, pendingPath)
		if extraMsg != "" {
			base += " " + extraMsg
		}
		if pendingDiff != "" {
			return fmt.Sprintf("%s\n\nPending diff:\n```diff\n%s\n```\n\nCall Edit.confirm to apply this edit, or Edit.cancel to discard it.", base, pendingDiff)
		}
		return fmt.Sprintf("%s\n\nCall Edit.confirm to apply this edit, or Edit.cancel to discard it.", base)
	}

	// Escalating error messages based on block count from history
	// All blocked states return SemanticError (backtrackable) - LLM should know to confirm/cancel first
	if toolName == "Edit" {
		if blockCount >= PendingEditEscalateThreshold {
			return SemanticError(formatBlocked(fmt.Sprintf("You have tried %d times - Edit will NOT work until you resolve the pending edit first!", blockCount)))
		}
		return SemanticError(formatBlocked(""))
	}


	// Block all other tools with escalating messages
	if blockCount >= PendingEditEscalateThreshold {
		return SemanticError(formatBlocked(fmt.Sprintf("You've been blocked %d times!", blockCount)))
	}
	return SemanticError(formatBlocked(""))
}

// CheckPendingEditBlockWithConfig is the legacy version that uses RAM state.
// Deprecated: Use CheckPendingEditBlockWithState with history-derived state instead.
func CheckPendingEditBlockWithConfig(toolName string, args json.RawMessage, blockCount int, cfg *config.Config, toolCtx *ToolContext) *ToolError {
	pendingPath := toolCtx.GetPendingEditPath()
	if pendingPath == "" {
		return nil // No pending edit, allow all tools
	}

	// Convert to state-based call
	state := PendingEditState{
		HasPending:             true,
		PendingPath:            pendingPath,
		BlockCountSincePending: blockCount,
	}
	return CheckPendingEditBlockWithState(toolName, state, cfg, toolCtx)
}

// GetMaxIgnoreCount returns the maximum ignore count from config or default
func GetMaxIgnoreCount(cfg *config.Config) int {
	if cfg != nil && cfg.Tools.Edit.PendingConfirmRetries > 0 {
		return cfg.Tools.Edit.PendingConfirmRetries
	}
	return PendingEditMaxIgnoreCount
}

// PendingEditState represents the state of pending edits derived from message history
type PendingEditState struct {
	HasPending             bool   // Whether there's an unresolved pending edit
	PendingPath            string // Path of the pending edit (empty if none)
	LastPendingIdx         int    // Index of the last pending_confirmation message
	BlockCountSincePending int    // Number of BLOCKED messages since last pending_confirmation
}

// AnalyzePendingEditState scans message history to determine pending edit state.
// This is the source of truth - RAM state may be out of sync after history manipulation.
// messageRoles, messageContents, and toolNames should be parallel arrays from the message history.
// toolNames contains the tool name for tool messages (empty for non-tool messages).
func AnalyzePendingEditState(messageRoles []string, messageContents []string, toolNames []string) PendingEditState {
	state := PendingEditState{LastPendingIdx: -1}

	// Scan for pending_confirmation and confirm/cancel tool calls
	for i, content := range messageContents {
		if messageRoles[i] != "tool" {
			continue
		}

		toolName := ""
		if i < len(toolNames) {
			toolName = toolNames[i]
		}

		// Check for pending_confirmation (new pending edit)
		if strings.Contains(content, "pending_confirmation") {
			// Extract path from the message
			// Format: "path": "some/path" in JSON
			if pathIdx := strings.Index(content, `"path"`); pathIdx != -1 {
				// Find the value after "path":
				rest := content[pathIdx+6:] // skip `"path"`
				if colonIdx := strings.Index(rest, ":"); colonIdx != -1 {
					rest = rest[colonIdx+1:]
					// Find the quoted path value
					if startQuote := strings.Index(rest, `"`); startQuote != -1 {
						rest = rest[startQuote+1:]
						if endQuote := strings.Index(rest, `"`); endQuote != -1 {
							state.PendingPath = rest[:endQuote]
						}
					}
				}
			}
			state.HasPending = true
			state.LastPendingIdx = i
			state.BlockCountSincePending = 0 // Reset count for new pending edit
		}

		// Check for confirm/cancel by tool name (more reliable than string matching)
		if toolName == "Edit.confirm" || toolName == "Write.confirm" ||
			toolName == "Edit.cancel" || toolName == "Write.cancel" {
			state.HasPending = false
			state.PendingPath = ""
			state.LastPendingIdx = -1
			state.BlockCountSincePending = 0
		}

		// Count BLOCKED messages after the pending edit was created
		if state.HasPending && strings.Contains(content, "BLOCKED:") && strings.Contains(content, "pending edit on") {
			state.BlockCountSincePending++
		}
	}

	return state
}

// ReadFileTool reads file contents with enhanced capabilities
type ReadFileTool struct {
	config        *config.Config
	workspaceRoot string
	maxFileSizeKB int // Max file size to read (default: 128KB)
	maxLines      int // Max lines before truncation (default: 150)
	maxBytes      int // Max bytes before truncation (default: 24KB)
	toolCtx       *ToolContext
}

func NewReadFileTool(cfg *config.Config, toolCtx *ToolContext) *ReadFileTool {
	maxFileSize := cfg.Tools.Read.MaxFileSizeKB
	if maxFileSize == 0 {
		maxFileSize = 128 // default 128KB
	}

	maxLines := cfg.Tools.Read.MaxPartialLines
	if maxLines == 0 {
		maxLines = DefaultMaxLines // fallback to constant
	}
	maxBytes := cfg.Tools.Read.MaxReadSizeKB * 1024
	if maxBytes == 0 {
		maxBytes = DefaultMaxBytes // fallback to constant
	}

	return &ReadFileTool{
		config:        cfg,
		workspaceRoot: cfg.Workspace.Root,
		maxFileSizeKB: maxFileSize,
		maxLines:      maxLines,
		maxBytes:      maxBytes,
		toolCtx:       toolCtx,
	}
}

// ParallelSafe says Read can run alongside other reads in a Batch: it changes
// nothing, so nothing else in the batch can be affected by when it runs.
func (t *ReadFileTool) ParallelSafe() bool { return true }

func (t *ReadFileTool) Name() string {
	return "Read"
}

func (t *ReadFileTool) Description() string {
	return "Read file contents. Line mode (default) for normal files; char_mode=true for large files (uses seek, no memory limit). Files only: use Glob to list directories or find files."
}

func (t *ReadFileTool) Check(ctx context.Context, args json.RawMessage) error {
	// No pre-execution checks needed for read
	return nil
}

func (t *ReadFileTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to a file (relative to workspace or absolute). Directories are refused: use Glob to list them.",
			},
			"char_mode": map[string]any{
				"type":        "boolean",
				"description": "Use byte/character mode instead of line mode. Required for large files (>128KB). Uses file seek - no memory limit.",
			},
			"start": map[string]any{
				"type":        "integer",
				"description": "Optional: Starting line/char. Positive values are 1-based from start (1 = first line). Negative values count from end (-1 = last line, -50 = 50 lines from end). Default: 1",
			},
			"limit": map[string]any{
				"type":        "integer",
				"description": "Optional: Maximum lines/chars to read",
			},
			"pages": map[string]any{
				"type":        "string",
				"description": "PDF files only: which pages to read, counting from 1. \"3\" for one page, \"2-6\" for a span, \"5-\" from there on. Default: from page 1.",
			},
		},
		"required": []string{"path"},
	}
}

func (t *ReadFileTool) PromptCategory() string     { return "filesystem" }
func (t *ReadFileTool) PromptOrder() int           { return 10 }
func (t *ReadFileTool) PromptTemplateName() string { return "read" }
func (t *ReadFileTool) PromptSection() string {
	return `### Read - Read Files

**Usage:** ` + "`" + `Read {"path": "<file>"}` + "`" + `

Examples:
- ` + "`" + `Read {"path": "file.py"}` + "`" + ` - read entire file
- ` + "`" + `Read {"path": "file.py", "start": 10, "limit": 20}` + "`" + ` - read lines 10-29
- ` + "`" + `Read {"path": "file.py", "start": -50}` + "`" + ` - read last 50 lines

**Parameters:**
- ` + "`path`" + ` (required): File path (directories are refused: use Glob to list them)
- ` + "`start`" + ` (optional): Starting line (1-based, default: 1). Negative = from end (-50 = last 50 lines)
- ` + "`limit`" + ` (optional): Maximum lines to read

Output is truncated at 150 lines or 24KB. For large files, use start/limit to read in chunks.
Always use Read before editing a file. To find which files exist, use Glob first.

**PDF files** are detected from their contents and their text is extracted for
you, with a marker before each page. Use ` + "`pages`" + ` to choose which pages:
` + "`" + `Read {"path": "spec.pdf", "pages": "2-6"}` + "`" + `. Without it the read starts at
page 1. ` + "`start`" + `, ` + "`limit`" + ` and ` + "`char_mode`" + ` do not apply to a PDF, and a PDF
cannot be edited as text.

**Word, Excel and PowerPoint files** (.docx, .xlsx, .pptx and their variants)
are converted to Markdown: headings, lists, tables, one table per worksheet, and
slide text with speaker notes. ` + "`start`" + ` and ` + "`limit`" + ` count lines of the converted
text, and a long document comes with an outline of its headings to jump to. The
converted text is also saved at ` + "`converted_path`" + ` for Search or Shell. These
files cannot be edited as text. The older .doc, .xls and .ppt formats and
OpenDocument files are not converted; Read names them and gives the command
that converts one to the newer format.

**Other non-text files** — images, archives, databases, compiled output — are
named rather than read, with the tool or command that opens them. That is the
answer, not a failure to retry. Pass ` + "`char_mode: true`" + ` if you want the raw
bytes anyway.`
}

func (t *ReadFileTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Path     string `json:"path"`
		CharMode bool   `json:"char_mode"`
		Start    *int   `json:"start"`
		Limit    *int   `json:"limit"`
		Pages    string `json:"pages"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, err
	}

	// Check permissions using 3-tier system
	permResult, err := t.config.CheckPathPermission(params.Path, config.AccessRead)
	if err != nil && permResult == config.PermissionDenied {
		return nil, fmt.Errorf("access denied: %w", err)
	}

	// Normalize and validate path using shared utility
	fullPath, outside, err := NormalizeAndValidatePath(t.workspaceRoot, params.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// For outside-workspace paths, use CheckPathSafety which respects path_safety_mode
	// (CheckPathPermission already ran above for denied_paths check).
	// A subagent child has no one to answer a prompt: fail closed first.
	if outside {
		if err := subagentPathPromptDeny(t.toolCtx, t.config, "read", params.Path); err != nil {
			return nil, err
		}
		if err := t.config.CheckPathSafety("read", params.Path); err != nil {
			return nil, err
		}
	}

	// Check if file exists
	info, err := os.Stat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Return structured JSON with suggestions instead of error
			result := map[string]any{
				"success": false,
				"error":   "file_not_found",
				"path":    params.Path,
				"message": fmt.Sprintf("File not found: %s", params.Path),
			}
			suggestions := findSimilarFiles(t.workspaceRoot, params.Path)
			if len(suggestions) > 0 {
				result["did_you_mean"] = suggestions
			}
			return result, nil
		}
		return nil, fmt.Errorf("stat file: %w", err)
	}

	// Directories are Glob's job. Answer in-band, the way file_not_found
	// does, so the model can act on it without losing the turn to an error.
	if info.IsDir() {
		return map[string]any{
			"success": false,
			"error":   "is_directory",
			"path":    params.Path,
			"message": fmt.Sprintf("%s is a directory. Use Glob to list it: Glob {\"path\": \"%s\"}", params.Path, params.Path),
		}, nil
	}

	fileSize := info.Size()

	// What the file starts with decides how it is read. The checks below are on
	// these bytes rather than on the extension, because the extension lies in
	// both directions: a file named .pdf is often something else, and a PDF is
	// occasionally named something else.
	head, headErr := readFileHead(fullPath, guardHeadBytes)

	// A PDF is a container, not text: reading it line by line returns the
	// object structure and the compressed streams.
	//
	// The read is deliberately not recorded: Read-before-Edit exists so the
	// model quotes back text it has seen, and the text extracted here does not
	// appear anywhere in the file's bytes.
	if headErr == nil && looksLikePDF(head) {
		return t.readPDF(ctx, fullPath, params.Path, params.Pages)
	}

	// Word, Excel and PowerPoint files are converted to Markdown, and are not
	// recorded as read for the same reason a PDF is not. char_mode still gets
	// the bytes.
	if headErr == nil && !params.CharMode {
		if result, handled, err := t.readOffice(fullPath, params.Path, info, head, params.Start, params.Limit); handled {
			return result, err
		}
	}

	// Anything else that is not text is named rather than read. char_mode is
	// the way out: a caller asking for bytes has said it means to have them.
	if headErr == nil && !params.CharMode {
		if kind, convertible := classifyConvertibleOffice(params.Path, head, t.officeDir()); convertible {
			return notTextResult(params.Path, fileSize, kind), nil
		}
		if kind, notText := classifyNotText(params.Path, head); notText {
			return notTextResult(params.Path, fileSize, kind), nil
		}
	}

	// For character mode on large files, use seek-based reading (no memory limit)
	if params.CharMode {
		// Record that this file was read (for read-before-edit enforcement)
		t.toolCtx.ReadTracker.RecordRead(fullPath, t.toolCtx.ReadTracker.CurrentMessageID())
		t.toolCtx.ClearStalePath(fullPath)
		return t.readCharModeSeek(fullPath, fileSize, params.Start, params.Limit, params.Path)
	}

	// Record that this file was read (for read-before-edit enforcement).
	// A successful read also clears the subagent re-read requirement.
	t.toolCtx.ReadTracker.RecordRead(fullPath, t.toolCtx.ReadTracker.CurrentMessageID())
	t.toolCtx.ClearStalePath(fullPath)

	result, err := t.readLineMode(fullPath, params.Start, params.Limit, params.Path)
	if err != nil {
		return nil, err
	}

	// A notebook is JSON, so it reads, but paging through the base64 images an
	// executed one carries costs many round trips for very little code. The
	// read stands and the cheaper route is offered alongside it.
	if headErr == nil && isNotebook(params.Path, head) {
		if m, ok := result.(map[string]any); ok {
			m["notebook_hint"] = notebookHint(params.Path)
		}
	}
	return result, nil
}

// readLinesResult contains the result of streaming line read
type readLinesResult struct {
	Lines            []string // The requested lines
	LineByteStarts   []int64  // Byte position where each collected line starts
	TotalLines       int      // Total lines in file
	TotalBytes       int64    // Total bytes in file
	ContentTruncated bool     // True if we stopped collecting early due to size limit
	LastByteRead     int64    // Byte position after last content read (for continuation)
}

// streamLastNLines reads the last N lines of a file in a single pass.
// Uses a circular buffer of byte positions (not content) to track where to seek.
// Returns full readLinesResult with lines, byte positions, and totals.
func streamLastNLines(path string, n int, maxBytes int) (*readLinesResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	totalBytes := info.Size()

	// Circular buffer of line start positions (just int64s, not content)
	positions := make([]int64, n)
	posIdx := 0    // Current index in circular buffer
	lineCount := 0 // Total lines seen
	bytePos := int64(0)

	buf := make([]byte, 32*1024) // 32KB read buffer
	lineStart := int64(0)        // Start position of current line

	for {
		bytesRead, err := file.Read(buf)
		if bytesRead > 0 {
			for i := 0; i < bytesRead; i++ {
				if buf[i] == '\n' {
					lineCount++
					// Record start position of this line in circular buffer
					positions[posIdx] = lineStart
					posIdx = (posIdx + 1) % n
					// Next line starts after this newline
					lineStart = bytePos + int64(i) + 1
				}
			}
			bytePos += int64(bytesRead)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
	}

	// Check if file ends without newline (last line not counted)
	if lineStart < bytePos {
		lineCount++
		positions[posIdx] = lineStart
		posIdx = (posIdx + 1) % n // Advance index so oldest position is correct
	}

	if lineCount == 0 {
		return &readLinesResult{TotalLines: 0, TotalBytes: totalBytes}, nil
	}

	// Calculate which position to seek to
	var seekPos int64
	linesToRead := n
	if lineCount <= n {
		// File has fewer lines than requested, read from start
		seekPos = 0
		linesToRead = lineCount
	} else {
		// The oldest position in circular buffer is where we need to start
		seekPos = positions[posIdx]
	}

	// Seek and read the lines, tracking byte positions
	if _, err := file.Seek(seekPos, 0); err != nil {
		return nil, err
	}

	reader := bufio.NewReader(file)
	var lines []string
	var lineByteStarts []int64
	currentBytePos := seekPos
	collectedBytes := 0
	contentTruncated := false
	lastByteRead := seekPos

	for i := 0; i < linesToRead && !contentTruncated; i++ {
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}
		if len(line) > 0 {
			cleanLine := strings.TrimSuffix(line, "\n")
			lineLen := len(cleanLine) + 1

			if collectedBytes+lineLen <= maxBytes {
				lineByteStarts = append(lineByteStarts, currentBytePos)
				lines = append(lines, cleanLine)
				collectedBytes += lineLen
				currentBytePos += int64(len(line))
				lastByteRead = currentBytePos
			} else if collectedBytes == 0 {
				// First line too long - truncate it
				remaining := maxBytes - 1
				if remaining > 0 && remaining < len(cleanLine) {
					lineByteStarts = append(lineByteStarts, currentBytePos)
					lines = append(lines, cleanLine[:remaining]+"...")
					lastByteRead = currentBytePos + int64(remaining)
				}
				contentTruncated = true
			} else {
				contentTruncated = true
			}
		}
		if err == io.EOF {
			break
		}
	}

	return &readLinesResult{
		Lines:            lines,
		LineByteStarts:   lineByteStarts,
		TotalLines:       lineCount,
		TotalBytes:       totalBytes,
		ContentTruncated: contentTruncated,
		LastByteRead:     lastByteRead,
	}, nil
}

// streamReadLines reads specific lines and counts total in a single pass
// - Streams through file, collecting lines in [startLine, endLine] range
// - Stops collecting content once maxBytes is reached (but continues counting)
// - Tracks byte positions for char_mode continuation
func streamReadLines(path string, startLine, endLine int, maxBytes int) (*readLinesResult, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	// Get file size
	info, err := file.Stat()
	if err != nil {
		return nil, err
	}
	totalBytes := info.Size()

	// Use a reader that tracks byte position
	reader := bufio.NewReader(file)
	var lines []string
	var lineByteStarts []int64
	lineNum := 0
	bytePos := int64(0)
	collectedBytes := 0
	contentTruncated := false
	lastByteRead := int64(0)

	for {
		lineStart := bytePos
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return nil, err
		}

		if len(line) > 0 {
			lineNum++
			bytePos += int64(len(line))

			if lineNum >= startLine && lineNum <= endLine {
				cleanLine := strings.TrimSuffix(line, "\n")
				lineLen := len(cleanLine) + 1 // +1 for newline

				if !contentTruncated {
					if collectedBytes+lineLen <= maxBytes {
						// Line fits, collect it
						lines = append(lines, cleanLine)
						lineByteStarts = append(lineByteStarts, lineStart)
						collectedBytes += lineLen
						lastByteRead = bytePos
					} else if collectedBytes == 0 {
						// First line is too long - truncate it
						remaining := maxBytes - 1 // leave room for newline marker
						if remaining > 0 && remaining < len(cleanLine) {
							lines = append(lines, cleanLine[:remaining]+"...")
							lineByteStarts = append(lineByteStarts, lineStart)
							collectedBytes = remaining + 4
						}
						lastByteRead = lineStart + int64(remaining)
						contentTruncated = true
					} else {
						// Hit limit, stop collecting
						contentTruncated = true
						// lastByteRead stays at previous value
					}
				}
				// Continue loop to count remaining lines
			}
		}

		if err == io.EOF {
			break
		}
	}

	return &readLinesResult{
		Lines:            lines,
		LineByteStarts:   lineByteStarts,
		TotalLines:       lineNum,
		TotalBytes:       totalBytes,
		ContentTruncated: contentTruncated,
		LastByteRead:     lastByteRead,
	}, nil
}

// readLineMode reads content in line mode with streaming (no full file load)
func (t *ReadFileTool) readLineMode(fullPath string, start, limit *int, path string) (any, error) {
	// Handle negative start with single-pass using circular buffer of positions
	if start != nil && *start < 0 {
		if *start == 0 {
			return nil, fmt.Errorf("start cannot be 0 (use 1 for first line, -1 for last line)")
		}

		// Calculate how many lines from end
		n := -(*start) // -(-50) = 50

		// Apply limit if specified
		if limit != nil && *limit > 0 && *limit < n {
			n = *limit
		}

		// Cap to maxLines
		if n > t.maxLines {
			n = t.maxLines
		}

		result, err := streamLastNLines(fullPath, n, t.maxBytes)
		if err != nil {
			return nil, fmt.Errorf("read last lines: %w", err)
		}

		if result.TotalLines == 0 {
			return map[string]any{
				"success":         true,
				"path":            path,
				"content":         "",
				"first_read_line": 0,
				"last_read_line":  0,
				"total_lines":     0,
				"hint":            "File is empty.",
			}, nil
		}

		// Calculate actual start line
		startLine := result.TotalLines - len(result.Lines) + 1
		if startLine < 1 {
			startLine = 1
		}

		return t.formatLineResult(result, startLine, path)
	}

	// Positive start: single pass
	startLine := 1
	if start != nil {
		startLine = *start
		if startLine == 0 {
			return nil, fmt.Errorf("start cannot be 0 (use 1 for first line, -1 for last line)")
		}
	}

	// Calculate end line
	endLine := startLine + t.maxLines - 1
	if limit != nil {
		if *limit < 0 {
			// Negative limit: read N lines *before* start
			// e.g., start=950, limit=-50 means lines 900-949
			endLine = startLine - 1
			startLine = startLine + *limit
			if startLine < 1 {
				startLine = 1
			}
			if endLine < startLine {
				endLine = startLine
			}
		} else if *limit == 0 {
			return map[string]any{
				"success": false,
				"error":   "invalid_limit",
				"path":    path,
				"message": "Invalid limit value 0. Limit must be non-zero.",
				"hint":    "Use positive limit to read forward, negative limit to read backward from start.",
			}, nil
		} else {
			endLine = startLine + *limit - 1
		}
	}

	// Single pass: read lines and count total
	result, err := streamReadLines(fullPath, startLine, endLine, t.maxBytes)
	if err != nil {
		return nil, fmt.Errorf("read lines: %w", err)
	}

	totalLines := result.TotalLines

	if totalLines == 0 {
		return map[string]any{
			"success":         true,
			"path":            path,
			"content":         "",
			"first_read_line": 0,
			"last_read_line":  0,
			"total_lines":     0,
			"hint":            "File is empty.",
		}, nil
	}

	// Handle start beyond EOF
	if startLine > totalLines {
		return map[string]any{
			"success":         true,
			"path":            path,
			"content":         "",
			"first_read_line": startLine,
			"last_read_line":  startLine,
			"total_lines":     totalLines,
			"hint":            fmt.Sprintf("start=%d is beyond end of file. File has %d lines. Use start=1 or start=-%d for last %d lines.", startLine, totalLines, min(totalLines, t.maxLines), min(totalLines, t.maxLines)),
		}, nil
	}

	// Handle no lines returned (e.g., range was invalid)
	if len(result.Lines) == 0 {
		return map[string]any{
			"success":         true,
			"path":            path,
			"content":         "",
			"first_read_line": startLine,
			"last_read_line":  startLine,
			"total_lines":     totalLines,
			"hint":            fmt.Sprintf("No lines in range. File has %d lines.", totalLines),
		}, nil
	}

	return t.formatLineResult(result, startLine, path)
}

// formatLineResult formats the lines into the response
func (t *ReadFileTool) formatLineResult(result *readLinesResult, startLine int, path string) (any, error) {
	selectedLines := result.Lines
	totalLines := result.TotalLines
	totalBytes := result.TotalBytes
	lineByteStarts := result.LineByteStarts

	// If content was truncated during streaming, return char_mode style output
	if result.ContentTruncated && len(lineByteStarts) > 0 {
		// Format the truncated content
		var content strings.Builder
		for _, line := range selectedLines {
			content.WriteString(line)
			content.WriteString("\n")
		}

		firstByte := lineByteStarts[0]
		linesIncluded := len(selectedLines)

		return map[string]any{
			"success":         true,
			"path":            path,
			"content":         content.String(),
			"first_read_line": startLine,
			"last_read_line":  startLine + linesIncluded - 1,
			"first_read_byte": firstByte + 1, // 1-based
			"last_read_byte":  result.LastByteRead,
			"total_lines":     totalLines,
			"total_bytes":     totalBytes,
			"hint": fmt.Sprintf("Lines in this range too large. Showing partial content. Use char_mode to continue (start is BYTE position, not line): Read {\"path\": \"%s\", \"char_mode\": true, \"start\": %d}",
				path, result.LastByteRead+1),
		}, nil
	}

	// Normal line mode output
	var resultContent string
	showLineNumbers := t.config.Tools.Read.GetShowLineNumbers()
	if showLineNumbers {
		var numberedContent strings.Builder
		for i, line := range selectedLines {
			lineNum := startLine + i
			numberedContent.WriteString(fmt.Sprintf("%4d│%s\n", lineNum, line))
		}
		resultContent = numberedContent.String()
	} else {
		var plainContent strings.Builder
		for _, line := range selectedLines {
			plainContent.WriteString(line)
			plainContent.WriteString("\n")
		}
		resultContent = plainContent.String()
	}

	actualEndLine := startLine + len(selectedLines) - 1

	response := map[string]any{
		"success":         true,
		"path":            path,
		"content":         resultContent,
		"first_read_line": startLine,
		"last_read_line":  actualEndLine,
		"total_lines":     totalLines,
	}

	// Add note about line number format when shown
	if showLineNumbers {
		response["format_note"] = "Line numbers (e.g. '  42│') are display-only prefixes - NOT part of file content. Never include them in edits."
	}

	// Add helpful hints when more content is available
	if actualEndLine < totalLines {
		remaining := totalLines - actualEndLine
		response["hint"] = fmt.Sprintf("%d more lines available. Read {\"path\": \"%s\", \"start\": %d} to continue.", remaining, path, actualEndLine+1)
	}

	return response, nil
}

// readCharModeSeek reads a portion of a file using seek - no need to load the whole file into memory
func (t *ReadFileTool) readCharModeSeek(fullPath string, fileSize int64, start, limit *int, path string) (any, error) {
	// Open file for reading
	file, err := os.Open(fullPath)
	if err != nil {
		return nil, fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	// Determine start position (1-based to 0-based byte offset)
	startByte := int64(0)
	if start != nil {
		if *start < 1 {
			return nil, fmt.Errorf("start must be >= 1")
		}
		startByte = int64(*start - 1)
	}

	// Clamp startByte to file size
	if startByte > fileSize {
		startByte = fileSize
	}

	// Determine how many bytes to read
	const defaultCharModeBytes = 12 * 1024 // Default: 12KB
	const maxCharModeBytes = 64 * 1024     // Max: 64KB

	bytesToRead := int64(defaultCharModeBytes)
	if limit != nil {
		if *limit <= 0 {
			return map[string]any{
				"success": false,
				"error":   "invalid_limit",
				"path":    path,
				"message": "Invalid limit value. Limit must be positive in char_mode.",
			}, nil
		}
		bytesToRead = int64(*limit)
	}

	// Cap bytesToRead to max allowed
	wasTruncated := false
	if bytesToRead > maxCharModeBytes {
		bytesToRead = maxCharModeBytes
		wasTruncated = true
	}

	// Don't read past end of file
	if startByte+bytesToRead > fileSize {
		bytesToRead = fileSize - startByte
	}

	// Seek to start position
	if _, err := file.Seek(startByte, 0); err != nil {
		return nil, fmt.Errorf("seek file: %w", err)
	}

	// Read the bytes
	buf := make([]byte, bytesToRead)
	n, err := file.Read(buf)
	if err != nil && err.Error() != "EOF" {
		return nil, fmt.Errorf("read file: %w", err)
	}
	buf = buf[:n]

	lastReadByte := startByte + int64(n)
	response := map[string]any{
		"success":         true,
		"path":            path,
		"content":         string(buf),
		"first_read_byte": startByte + 1, // Return 1-based
		"last_read_byte":  lastReadByte,
		"total_bytes":     fileSize,
	}

	// Add helpful hints when content is incomplete
	if wasTruncated {
		// Hit the max char_mode limit
		remaining := fileSize - lastReadByte
		response["hint"] = fmt.Sprintf("Content truncated (hit 64KB limit). %d bytes remaining. To continue, use char_mode (start is BYTE position, not line): Read {\"path\": \"%s\", \"char_mode\": true, \"start\": %d}", remaining, path, lastReadByte+1)
	} else if lastReadByte < fileSize {
		// More bytes available
		remaining := fileSize - lastReadByte
		response["hint"] = fmt.Sprintf("%d bytes remaining. To continue, use char_mode (start is BYTE position, not line): Read {\"path\": \"%s\", \"char_mode\": true, \"start\": %d}", remaining, path, lastReadByte+1)
	}

	return response, nil
}

// checkChildWriteGate enforces read-before-write for subagent children
// only. Write has no read check today and parent behavior must not change in
// phase 2, so this is a no-op for any non-child context. Overwriting an
// existing file must follow a child-local read of its current content;
// creating a new file always passes.
func checkChildWriteGate(toolCtx *ToolContext, cfg *config.Config, displayPath, fullPath string) error {
	if toolCtx == nil || !toolCtx.IsSubagentChild() {
		return nil
	}
	if _, statErr := os.Stat(fullPath); os.IsNotExist(statErr) {
		return nil
	} else if statErr != nil {
		return nil // fail open on stat; the write itself will surface it
	}
	if toolCtx.IsStalePath(fullPath) {
		return SemanticErrorWithDetails(
			fmt.Sprintf("file changed by subagent: '%s' was edited by a subagent since you last read it; read it again before editing", displayPath),
			map[string]any{
				"error":     "file_changed_by_subagent",
				"path":      displayPath,
				"next_step": fmt.Sprintf("read {\"path\": \"%s\"}", displayPath),
			},
		)
	}
	window := cfg.Tools.Edit.ReadBeforeEditMsgs
	if window <= 0 {
		return nil
	}
	tracker := toolCtx.ReadTracker
	if tracker == nil || !tracker.WasReadRecently(fullPath, tracker.CurrentMessageID(), window) {
		return SemanticErrorWithDetails(
			fmt.Sprintf("file not read recently: you must use read on '%s' before overwriting it (within last %d tool calls)", displayPath, window),
			map[string]any{
				"error":     "file_not_read",
				"path":      displayPath,
				"next_step": fmt.Sprintf("read {\"path\": \"%s\"}", displayPath),
			},
		)
	}
	if changed, found := tracker.StatChangedSinceRead(fullPath); found && changed {
		return SemanticErrorWithDetails(
			fmt.Sprintf("file changed since read: '%s' was modified after you read it; read it again before editing", displayPath),
			map[string]any{
				"error":     "file_changed_since_read",
				"path":      displayPath,
				"next_step": fmt.Sprintf("read {\"path\": \"%s\"}", displayPath),
			},
		)
	}
	return nil
}

// WriteFileTool writes entire content to a file (creates or overwrites)
type WriteFileTool struct {
	config        *config.Config
	workspaceRoot string
	toolCtx       *ToolContext
}

func NewWriteFileTool(cfg *config.Config, toolCtx *ToolContext) *WriteFileTool {
	return &WriteFileTool{
		config:        cfg,
		workspaceRoot: cfg.Workspace.Root,
		toolCtx:       toolCtx,
	}
}

func (t *WriteFileTool) Name() string {
	return "Write"
}

func (t *WriteFileTool) Description() string {
	return "Write content to a file, creating it if it doesn't exist or overwriting if it does."
}

func (t *WriteFileTool) Check(ctx context.Context, args json.RawMessage) error {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}

	// Normalize and validate path
	fullPath, _, err := NormalizeAndValidatePath(t.workspaceRoot, params.Path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}

	// Check write permission
	permResult, err := t.config.CheckPathPermission(fullPath, config.AccessWrite)
	if err != nil && permResult == config.PermissionDenied {
		return fmt.Errorf("access denied: %w", err)
	}
	if permResult == config.PermissionReadOnly {
		return fmt.Errorf("path is read-only: %w", err)
	}

	// Subagent children must read a file before overwriting it (Write has no
	// read check otherwise, and parent behavior must not change).
	if err := checkChildWriteGate(t.toolCtx, t.config, params.Path, fullPath); err != nil {
		return err
	}

	return nil
}

func (t *WriteFileTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "Path to file (relative to workspace or absolute)",
			},
			"text": map[string]any{
				"type":        "string",
				"description": "Content to write to the file",
			},
		},
		"required": []string{"path", "text"},
	}
}

func (t *WriteFileTool) PromptCategory() string     { return "filesystem" }
func (t *WriteFileTool) PromptOrder() int           { return 15 } // Between Read (10) and Edit (20)
func (t *WriteFileTool) PromptTemplateName() string { return "write" }
func (t *WriteFileTool) PromptSection() string {
	base := `### Write - Write Files

**Usage:** ` + "`" + `Write {"path": "<file>", "text": "<content>"}` + "`" + `

Creates a new file or overwrites an existing file.`
	// Only describe the handshake where it exists. Without this the prompt
	// told the model to answer a pending_confirmation with tools that are not
	// registered, and it went looking for them.
	if t.config.Tools.Edit.PreviewMode {
		base += `

**Overwrite Confirmation:**
When overwriting an existing file, returns status="pending_confirmation".
- ` + "`" + `Write.confirm {}` + "`" + ` - Apply the overwrite
- ` + "`" + `Write.cancel {}` + "`" + ` - Cancel`
	}
	return base
}

func (t *WriteFileTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	var params struct {
		Path string `json:"path"`
		Text string `json:"text"`
	}
	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	// Normalize and validate path
	fullPath, _, err := NormalizeAndValidatePath(t.workspaceRoot, params.Path)
	if err != nil {
		return nil, fmt.Errorf("invalid path: %w", err)
	}

	// Use the original path for display (or make relative to workspace)
	displayPath := params.Path

	// Check write permission
	permResult, err := t.config.CheckPathPermission(fullPath, config.AccessWrite)
	if err != nil && permResult == config.PermissionDenied {
		return nil, fmt.Errorf("access denied: %w", err)
	}
	if permResult == config.PermissionReadOnly {
		return nil, fmt.Errorf("path is read-only: %w", err)
	}

	// Subagent children must read a file before overwriting it.
	if err := checkChildWriteGate(t.toolCtx, t.config, params.Path, fullPath); err != nil {
		return nil, err
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(fullPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Check if file exists
	fileInfo, statErr := os.Stat(fullPath)
	isNewFile := os.IsNotExist(statErr)

	// Overwriting an existing file is staged for confirmation only when
	// preview_mode is on, the same switch that governs Edit's handshake. With
	// it off — the default, and what the strong profile sets — Write writes,
	// and the previous contents are lost if they are wanted back.
	if !isNewFile && t.config.Tools.Edit.PreviewMode {
		// Count lines in existing file
		oldContent, err := os.ReadFile(fullPath)
		oldLines := 0
		if err == nil {
			oldLines = strings.Count(string(oldContent), "\n")
			if len(oldContent) > 0 && !strings.HasSuffix(string(oldContent), "\n") {
				oldLines++
			}
		}

		// Store pending write
		t.toolCtx.SetPendingWrite(&pendingWrite{
			path:     displayPath,
			fullPath: fullPath,
			content:  params.Text,
			oldSize:  fileInfo.Size(),
			oldLines: oldLines,
		})

		// Calculate new file stats
		newLines := strings.Count(params.Text, "\n")
		if len(params.Text) > 0 && !strings.HasSuffix(params.Text, "\n") {
			newLines++
		}

		return map[string]any{
			"status":    "pending_confirmation",
			"path":      displayPath,
			"next_step": "File exists. Call Write.confirm to overwrite or Write.cancel to abort.",
			"old_size":  fileInfo.Size(),
			"old_lines": oldLines,
			"new_size":  len(params.Text),
			"new_lines": newLines,
		}, nil
	}

	// New file - write directly
	return t.writeFile(fullPath, displayPath, params.Text, true)
}

// writeFile performs the actual file write
func (t *WriteFileTool) writeFile(fullPath, displayPath, content string, isNewFile bool) (any, error) {
	parentDir := filepath.Dir(fullPath)

	// Write atomically - write to temp file first, then rename
	tempFile, err := os.CreateTemp(parentDir, ".write-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath) // Clean up on error

	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// Rename temp file to target (atomic on most filesystems)
	if err := os.Rename(tempPath, fullPath); err != nil {
		return nil, fmt.Errorf("rename temp file: %w", err)
	}

	t.toolCtx.RecordEditedPath(fullPath)

	// Build response
	lines := strings.Count(content, "\n")
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		lines++ // Count last line without newline
	}

	action := "updated"
	if isNewFile {
		action = "created"
	}

	return map[string]any{
		"success": true,
		"path":    displayPath,
		"action":  action,
		"lines":   lines,
		"bytes":   len(content),
	}, nil
}

// applyPendingEdit applies a pending edit operation (shared by Edit.confirm and Write.confirm)
func applyPendingEdit(toolCtx *ToolContext, pending *pendingEdit) (any, error) {
	// Verify file hasn't changed since preview (or doesn't exist for new files)
	currentContent, err := os.ReadFile(pending.fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist - this is only ok if it was a new file creation
			if !pending.isNewFile {
				return map[string]any{
					"success": false,
					"error":   "file_deleted",
					"message": "File was deleted since the preview. Please call edit again.",
				}, nil
			}
			// New file - expected to not exist
		} else {
			return nil, fmt.Errorf("read file: %w", err)
		}
	} else if string(currentContent) != pending.oldContent {
		return map[string]any{
			"success": false,
			"error":   "file_changed",
			"message": "File has been modified since the preview. Please call edit again to get a new preview.",
		}, nil
	}

	// For new files, ensure parent directory exists
	if pending.isNewFile {
		parentDir := filepath.Dir(pending.fullPath)
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return nil, fmt.Errorf("create parent directory: %w", err)
		}
	}

	// Write atomically - write to temp file first, then rename
	tempFile, err := os.CreateTemp(filepath.Dir(pending.fullPath), ".edit-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.WriteString(pending.newContent); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	// Get file permissions from original file (or use default for new files)
	info, _ := os.Stat(pending.fullPath)
	if info != nil {
		_ = os.Chmod(tempPath, info.Mode())
	} else {
		_ = os.Chmod(tempPath, 0644) // Default permissions for new files
	}

	// Atomic rename
	if err := os.Rename(tempPath, pending.fullPath); err != nil {
		return nil, fmt.Errorf("atomic rename failed: %w", err)
	}

	toolCtx.RecordEditedPath(pending.fullPath)

	return BuildEditSuccessResult(pending.path, pending.diff, pending.newContent,
		pending.editStartLine, pending.editEndLine, pending.isNewFile), nil
}

// applyPendingWrite applies a pending write operation (shared by Edit.confirm and Write.confirm)
func applyPendingWrite(toolCtx *ToolContext, pending *pendingWrite) (any, error) {
	parentDir := filepath.Dir(pending.fullPath)
	tempFile, err := os.CreateTemp(parentDir, ".write-*.tmp")
	if err != nil {
		return nil, fmt.Errorf("create temp file: %w", err)
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)

	if _, err := tempFile.WriteString(pending.content); err != nil {
		tempFile.Close()
		return nil, fmt.Errorf("write temp file: %w", err)
	}
	if err := tempFile.Close(); err != nil {
		return nil, fmt.Errorf("close temp file: %w", err)
	}

	if err := os.Rename(tempPath, pending.fullPath); err != nil {
		return nil, fmt.Errorf("rename temp file: %w", err)
	}

	toolCtx.RecordEditedPath(pending.fullPath)

	lines := strings.Count(pending.content, "\n")
	if len(pending.content) > 0 && !strings.HasSuffix(pending.content, "\n") {
		lines++
	}

	return map[string]any{
		"success": true,
		"path":    pending.path,
		"action":  "overwritten",
		"lines":   lines,
		"bytes":   len(pending.content),
	}, nil
}

// ConfirmEditTool confirms and applies the last previewed edit from edit (only used in preview mode)
type ConfirmEditTool struct {
	config  *config.Config
	toolCtx *ToolContext
}

func NewConfirmEditTool(cfg *config.Config, toolCtx *ToolContext) *ConfirmEditTool {
	return &ConfirmEditTool{config: cfg, toolCtx: toolCtx}
}

func (t *ConfirmEditTool) Name() string {
	return "Edit.confirm"
}

func (t *ConfirmEditTool) Description() string {
	return "Confirm a pending edit previewed by edit. Call this after reviewing the diff to apply the changes."
}

func (t *ConfirmEditTool) Check(ctx context.Context, args json.RawMessage) error {
	return nil
}

func (t *ConfirmEditTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
}

func (t *ConfirmEditTool) PromptCategory() string     { return "filesystem" }
func (t *ConfirmEditTool) PromptOrder() int           { return 21 }
func (t *ConfirmEditTool) PromptTemplateName() string { return "" }
func (t *ConfirmEditTool) PromptSection() string      { return "" } // Docs included in Edit tool

func (t *ConfirmEditTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Check for pending edit first
	pending := t.toolCtx.GetAndClearPendingEdit()
	if pending != nil {
		return applyPendingEdit(t.toolCtx, pending)
	}

	// If no pending edit, check for pending write (Edit.confirm and Write.confirm are synonyms)
	pendingWrite := t.toolCtx.GetAndClearPendingWrite()
	if pendingWrite != nil {
		return applyPendingWrite(t.toolCtx, pendingWrite)
	}

	return map[string]any{
		"success":    false,
		"error":      "no_pending_operation",
		"message":    "Nothing to confirm. You must call Edit or Write first, then confirm to apply it.",
		"usage_hint": "Workflow: 1) read to see content, 2) edit/write to create change, 3) confirm to apply",
	}, nil
}

// CancelEditTool cancels the pending edit from edit (only used in preview mode)
type CancelEditTool struct {
	config  *config.Config
	toolCtx *ToolContext
}

func NewCancelEditTool(cfg *config.Config, toolCtx *ToolContext) *CancelEditTool {
	return &CancelEditTool{config: cfg, toolCtx: toolCtx}
}

func (t *CancelEditTool) Name() string {
	return "Edit.cancel"
}

func (t *CancelEditTool) Description() string {
	return "Discard a pending edit preview without applying it. ONLY use this tool immediately after edit returns status 'pending_confirmation'. This does NOT undo applied edits."
}

func (t *CancelEditTool) Check(ctx context.Context, args json.RawMessage) error {
	return nil
}

func (t *CancelEditTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
}

func (t *CancelEditTool) PromptCategory() string     { return "filesystem" }
func (t *CancelEditTool) PromptOrder() int           { return 22 }
func (t *CancelEditTool) PromptTemplateName() string { return "" }
func (t *CancelEditTool) PromptSection() string      { return "" } // Docs included in ConfirmEditTool

func (t *CancelEditTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Check for pending edit first
	pending := t.toolCtx.GetAndClearPendingEdit()

	// If no pending edit, check for pending write (Edit.cancel and Write.cancel are synonyms)
	if pending == nil {
		pendingWrite := t.toolCtx.GetAndClearPendingWrite()

		if pendingWrite != nil {
			return map[string]any{
				"success": true,
				"path":    pendingWrite.path,
				"message": "Write cancelled. File was not modified.",
			}, nil
		}

		return map[string]any{
			"success":      false,
			"error":        "no_pending_operation",
			"message":      "No pending operation to cancel. This tool is only used after Edit or Write returns status='pending_confirmation'.",
			"did_you_mean": "Edit.cancel",
			"hint":         "To discard a pending edit, use Edit.cancel.",
		}, nil
	}

	return map[string]any{
		"success":   true,
		"path":      pending.path,
		"message":   "Edit cancelled. File was not modified.",
		"next_step": fmt.Sprintf("MODIFY your edit to correct for the issues you observed, then retry. Read {\"path\": \"%s\"} if needed.", pending.path),
	}, nil
}

// ConfirmWriteTool confirms and applies a pending write operation
type ConfirmWriteTool struct {
	config  *config.Config
	toolCtx *ToolContext
}

func NewConfirmWriteTool(cfg *config.Config, toolCtx *ToolContext) *ConfirmWriteTool {
	return &ConfirmWriteTool{config: cfg, toolCtx: toolCtx}
}

func (t *ConfirmWriteTool) Name() string {
	return "Write.confirm"
}

func (t *ConfirmWriteTool) Description() string {
	return "Confirm a pending write operation to overwrite an existing file."
}

func (t *ConfirmWriteTool) Check(ctx context.Context, args json.RawMessage) error {
	return nil
}

func (t *ConfirmWriteTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
}

func (t *ConfirmWriteTool) PromptCategory() string     { return "filesystem" }
func (t *ConfirmWriteTool) PromptOrder() int           { return 16 }
func (t *ConfirmWriteTool) PromptTemplateName() string { return "" }
func (t *ConfirmWriteTool) PromptSection() string      { return "" } // Docs included in WriteFileTool

func (t *ConfirmWriteTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Check for pending write first
	pending := t.toolCtx.GetAndClearPendingWrite()
	if pending != nil {
		return applyPendingWrite(t.toolCtx, pending)
	}

	// If no pending write, check for pending edit (Write.confirm and Edit.confirm are synonyms)
	pendingEdit := t.toolCtx.GetAndClearPendingEdit()
	if pendingEdit != nil {
		// Delegate to the same logic as ConfirmEditTool
		return applyPendingEdit(t.toolCtx, pendingEdit)
	}

	return map[string]any{
		"success": false,
		"error":   "no_pending_operation",
		"message": "No pending operation to confirm. This tool is only used after Edit or Write returns status='pending_confirmation'.",
	}, nil
}

// CancelWriteTool cancels a pending write operation
type CancelWriteTool struct {
	config  *config.Config
	toolCtx *ToolContext
}

func NewCancelWriteTool(cfg *config.Config, toolCtx *ToolContext) *CancelWriteTool {
	return &CancelWriteTool{config: cfg, toolCtx: toolCtx}
}

func (t *CancelWriteTool) Name() string {
	return "Write.cancel"
}

func (t *CancelWriteTool) Description() string {
	return "Cancel a pending write operation."
}

func (t *CancelWriteTool) Check(ctx context.Context, args json.RawMessage) error {
	return nil
}

func (t *CancelWriteTool) JSONSchema() map[string]any {
	return map[string]any{
		"type":       "object",
		"properties": map[string]any{},
		"required":   []string{},
	}
}

func (t *CancelWriteTool) PromptCategory() string     { return "filesystem" }
func (t *CancelWriteTool) PromptOrder() int           { return 17 }
func (t *CancelWriteTool) PromptTemplateName() string { return "" }
func (t *CancelWriteTool) PromptSection() string      { return "" } // Docs included in ConfirmWriteTool

func (t *CancelWriteTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	// Check for pending write first
	pending := t.toolCtx.GetAndClearPendingWrite()
	if pending != nil {
		return map[string]any{
			"success": true,
			"path":    pending.path,
			"message": "Write cancelled. File was not modified.",
		}, nil
	}

	// If no pending write, check for pending edit (Write.cancel and Edit.cancel are synonyms)
	pendingEdit := t.toolCtx.GetAndClearPendingEdit()
	if pendingEdit != nil {
		return map[string]any{
			"success":   true,
			"path":      pendingEdit.path,
			"message":   "Edit cancelled. File was not modified.",
			"next_step": fmt.Sprintf("MODIFY your edit to correct for the issues you observed, then retry. Read {\"path\": \"%s\"} if needed.", pendingEdit.path),
		}, nil
	}

	return map[string]any{
		"success": false,
		"error":   "no_pending_operation",
		"message": "No pending operation to cancel.",
	}, nil
}

// generateUnifiedDiff generates a unified diff between old and new content
func generateUnifiedDiff(oldContent, newContent, filename string) (string, error) {
	diff := difflib.UnifiedDiff{
		A:        difflib.SplitLines(oldContent),
		B:        difflib.SplitLines(newContent),
		FromFile: filename,
		ToFile:   filename,
		Context:  3,
	}

	return difflib.GetUnifiedDiffString(diff)
}

// findSimilarFiles searches the workspace for files with similar names to the target path
func findSimilarFiles(workspaceRoot, targetPath string) []string {
	targetBase := filepath.Base(targetPath)
	targetBaseLower := strings.ToLower(targetBase)

	// Extract name without extension for partial matching
	targetNameOnly := strings.TrimSuffix(targetBase, filepath.Ext(targetBase))
	targetNameOnlyLower := strings.ToLower(targetNameOnly)

	var exactMatches []string
	var partialMatches []string
	seen := make(map[string]bool)

	_ = filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			// Skip .git and other hidden directories
			if d != nil && d.IsDir() {
				name := d.Name()
				if strings.HasPrefix(name, ".") || name == "node_modules" || name == "__pycache__" {
					return filepath.SkipDir
				}
			}
			return nil
		}

		base := filepath.Base(path)
		baseLower := strings.ToLower(base)

		// Skip if already seen
		if seen[path] {
			return nil
		}

		// Exact match (case-insensitive)
		if baseLower == targetBaseLower {
			exactMatches = append(exactMatches, path)
			seen[path] = true
			return nil
		}

		// Partial match: target name appears in file name
		nameOnly := strings.TrimSuffix(base, filepath.Ext(base))
		nameOnlyLower := strings.ToLower(nameOnly)

		if strings.Contains(nameOnlyLower, targetNameOnlyLower) ||
			strings.Contains(targetNameOnlyLower, nameOnlyLower) {
			partialMatches = append(partialMatches, path)
			seen[path] = true
		}

		return nil
	})

	// Combine exact matches first, then partial matches
	results := append(exactMatches, partialMatches...)

	// Limit to 5 results
	if len(results) > 5 {
		results = results[:5]
	}
	return results
}

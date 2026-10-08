package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// maxBatchCalls bounds one Batch. A batch is for the handful of independent
// things you already know you need; past about ten it is a plan, and a plan
// should be made with the results of the first few in hand.
const maxBatchCalls = 10

// BatchTool runs several independent tool calls in one request.
//
// A model that wants to read four files pays for four round trips: four
// requests carrying the whole conversation, and four rounds of thinking. Doing
// them in one call costs one of each. Calls that only read are run at the same
// time, so a batch of four reads takes about as long as one.
type BatchTool struct {
	cfg *config.Config

	mu       sync.Mutex
	registry *Registry
	toolCtx  *ToolContext
}

// NewBatchTool builds the tool. The registry it dispatches through is set
// afterwards with SetRegistry, because the registry is what holds this tool.
func NewBatchTool(cfg *config.Config) *BatchTool {
	return &BatchTool{cfg: cfg}
}

// SetToolContext gives the tool the shared state its inner calls report into.
// A batch is one call to the loop, so without this a batch that edited a file
// would look like a turn that only read.
func (t *BatchTool) SetToolContext(tc *ToolContext) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.toolCtx = tc
}

func (t *BatchTool) context() *ToolContext {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.toolCtx
}

// SetRegistry gives the tool the registry it looks calls up in.
func (t *BatchTool) SetRegistry(registry *Registry) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.registry = registry
}

func (t *BatchTool) lookup() *Registry {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.registry
}

func (t *BatchTool) Name() string { return "Batch" }

func (t *BatchTool) Description() string {
	return "Run several independent tool calls in one request. Use it whenever you already know " +
		"the next few calls and none of them depends on another's result — reading four files, or " +
		"searching for three things. Calls that only read run at the same time."
}

func (t *BatchTool) JSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"calls": map[string]any{
				"type":        "array",
				"description": fmt.Sprintf("The calls to make, at most %d. Each is a tool name and the arguments you would have passed it.", maxBatchCalls),
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"tool": map[string]any{"type": "string", "description": "The tool to call."},
						"args": map[string]any{"type": "object", "description": "The arguments for that tool."},
					},
					"required": []string{"tool", "args"},
				},
			},
		},
		"required": []string{"calls"},
	}
}

func (t *BatchTool) PromptCategory() string     { return "filesystem" }
func (t *BatchTool) PromptOrder() int           { return 50 }
func (t *BatchTool) PromptTemplateName() string { return "" }
func (t *BatchTool) PromptSection() string {
	return fmt.Sprintf(`### Batch - Several calls in one request

Batch({"calls": [
  {"tool": "Read", "args": {"path": "main.go"}},
  {"tool": "Read", "args": {"path": "config.go"}},
  {"tool": "Search", "args": {"pattern": "func main"}}
]})

Use it whenever the next few calls are already decided and none depends on
another's result. One request and one round of thinking instead of one each,
and read-only calls run at the same time. At most %d calls, no Batch inside a
Batch, and one call failing does not stop the others: every call comes back
with its own result or its own error.

Shape rules: 'calls' is a raw JSON array, never a JSON-encoded string, and
every tool's parameters go inside its own 'args' object — never beside
'tool'/'args'. A Glob call looks like {"tool": "Glob", "args":
{"path": "dir", "pattern": "*.md"}}, not {"args": {"path": "dir"},
"pattern": "*.md"}.`, maxBatchCalls)
}

// SelfTimeout opts out of the loop's blanket 15-second tool timeout: a batch is
// as slow as the calls in it, and each of those has its own limits.
func (t *BatchTool) SelfTimeout() bool { return true }

type batchCall struct {
	Tool string          `json:"tool"`
	Args json.RawMessage `json:"args"`
}

type batchArgs struct {
	Calls []batchCall `json:"calls"`
}

// BatchResult is what one call in a batch produced. Every call gets one,
// whether it worked or not, so the model can tell which of the four reads
// failed rather than being told the batch failed.
type BatchResult struct {
	Index  int    `json:"index"`
	Tool   string `json:"tool"`
	OK     bool   `json:"ok"`
	Result any    `json:"result,omitempty"`
	Error  string `json:"error,omitempty"`
	// Images carries attachments from an image-bearing call (ReadImage).
	// Only metadata is serialized (bytes are json:"-"); the runner attaches
	// the pixels to a follower user message, preserving which call read
	// which image.
	Images []llm.ImagePart `json:"images,omitempty"`
}

func (t *BatchTool) Check(ctx context.Context, args json.RawMessage) error {
	parsed, err := t.parse(args)
	if err != nil {
		return err
	}
	registry := t.lookup()
	for i, call := range parsed.Calls {
		tool := registry.Get(call.Tool)
		if tool == nil {
			return SemanticErrorf("Batch: call %d names an unknown tool %q.", i+1, call.Tool)
		}
		if tool.Name() == t.Name() {
			return SemanticErrorf("Batch: call %d is another Batch. Put the calls in this one instead.", i+1)
		}
	}
	return nil
}

func (t *BatchTool) parse(args json.RawMessage) (*batchArgs, error) {
	// Lenient path, outer level: the whole payload may arrive double-encoded
	// (a JSON string holding the arguments object). There is no legitimate
	// Batch call whose arguments are a bare string, so unwrap it when it
	// parses as one rather than failing on the shape.
	args = json.RawMessage(unwrapBatchJSONString(string(args)))

	var parsed batchArgs
	if err := json.Unmarshal(args, &parsed); err != nil {
		// Lenient path: some harnesses deliver the array as a JSON-encoded
		// string ({"calls": "[{...}]"}) instead of a raw array, and a model
		// hand-writing nested JSON slips a brace the same way. There is no
		// legitimate call where 'calls' is a string, so the unambiguous
		// encodings below are decoded rather than failing on the shape.
		var outer map[string]json.RawMessage
		if oerr := json.Unmarshal(args, &outer); oerr == nil {
			if raw, ok := outer["calls"]; ok {
				var s string
				if serr := json.Unmarshal(raw, &s); serr == nil {
					if trimmed := strings.TrimSpace(s); trimmed != "" {
						calls, effective, cerr := parseBatchCallsString(trimmed)
						if cerr == nil {
							parsed.Calls = calls
							if extra := extraBatchKeysFromString(effective); len(extra) > 0 {
								// The string parsed, but a call carries fields
								// beside tool/args — usually a tool parameter
								// left outside args. Fail fast with the fix
								// instead of running with half the arguments.
								return nil, SemanticErrorf("Batch: call %d has %s outside 'args'. All of that tool's parameters go inside its args: {\"tool\": %q, \"args\": {...}}.",
									extra[0].index, quotedList(extra[0].keys), parsed.Calls[extra[0].index-1].Tool)
							}
						} else {
							return nil, SemanticErrorf("Batch: 'calls' must be an array of {tool, args} — send the raw array, not a JSON-encoded string. %d calls max, no Batch inside a Batch. The string did not parse as an array: %s. Received (%d chars total, around byte %d): %s. Every tool's parameters belong inside its args: {\"tool\": \"Glob\", \"args\": {\"path\": ..., \"pattern\": ...}}.",
								maxBatchCalls, sanitizeBatchJSONError(cerr), len(trimmed), batchErrorOffset(cerr, trimmed), batchErrorPreview(trimmed, cerr))
						}
					}
				} else if single, ok := parseBatchSingleCall(raw); ok {
					// 'calls' holds one object instead of an array of them.
					// There is no legitimate call where it is a bare object,
					// so run it as a single call rather than failing.
					parsed.Calls = []batchCall{single}
				} else {
					return nil, SemanticErrorf("Batch: 'calls' must be an array of {tool, args} — send the raw array, not a JSON-encoded string. %d calls max, no Batch inside a Batch.", maxBatchCalls)
				}
			} else {
				return nil, SemanticErrorf("Batch: 'calls' is required — send {\"calls\": [{\"tool\": ..., \"args\": {...}}]}. %d calls max, no Batch inside a Batch.", maxBatchCalls)
			}
		} else {
			return nil, SemanticErrorf("Batch: arguments are not valid JSON — send {\"calls\": [{\"tool\": ..., \"args\": {...}}]}.")
		}
	}

	// Lenient path, inner level: an individual call's args may arrive the
	// same way ({"tool": "Read", "args": "{\"path\": \"a\"}"}). Decode it in
	// place when it parses as an object; otherwise leave it for the tool's
	// own check to report.
	for i, call := range parsed.Calls {
		if len(call.Args) == 0 {
			continue
		}
		var s string
		if serr := json.Unmarshal(call.Args, &s); serr != nil {
			continue
		}
		trimmed := strings.TrimSpace(s)
		if trimmed == "" {
			continue
		}
		var obj map[string]any
		if oerr := json.Unmarshal([]byte(trimmed), &obj); oerr == nil {
			parsed.Calls[i].Args = json.RawMessage(trimmed)
		}
	}
	if len(parsed.Calls) == 0 {
		return nil, SemanticErrorf("Batch: 'calls' is empty. Call the tool directly instead.")
	}
	if len(parsed.Calls) > maxBatchCalls {
		return nil, SemanticErrorf("Batch: %d calls is more than the limit of %d. Send the first %d and decide the rest with their results.",
			len(parsed.Calls), maxBatchCalls, maxBatchCalls)
	}
	for i, call := range parsed.Calls {
		if strings.TrimSpace(call.Tool) == "" {
			if extra := extraBatchKeys(args); len(extra) > 0 {
				for _, e := range extra {
					if e.index == i+1 && len(e.keys) > 0 {
						return nil, SemanticErrorf("Batch: call %d has no tool name, and %s outside 'args'. All of that tool's parameters go inside its args: {\"tool\": \"Glob\", \"args\": {\"path\": ..., \"pattern\": ...}}.", i+1, quotedList(e.keys))
					}
				}
			}
			return nil, SemanticErrorf("Batch: call %d has no tool name. Send {\"tool\": ..., \"args\": {...}} with every tool's parameters inside args.", i+1)
		}
	}
	if extra := extraBatchKeys(args); len(extra) > 0 {
		// The call names a tool but still carries fields beside tool/args —
		// the usual slip is {"tool": "Glob", "args": {"path": ...},
		// "pattern": ...} with a parameter left outside args. Running it
		// would silently drop that parameter, so fail fast with the fix.
		return nil, SemanticErrorf("Batch: call %d has %s outside 'args'. All of that tool's parameters go inside its args: {\"tool\": %q, \"args\": {...}}.",
			extra[0].index, quotedList(extra[0].keys), parsed.Calls[extra[0].index-1].Tool)
	}
	return &parsed, nil
}

// unwrapBatchJSONString decodes s while it is a JSON string holding further
// JSON (a doubly-encoded payload). It returns s unchanged when s is not a
// string, when the string is empty, or after a few levels, so a pathological
// nesting cannot loop. Whitespace around the levels is insignificant.
func unwrapBatchJSONString(s string) string {
	for i := 0; i < 3; i++ {
		trimmed := strings.TrimSpace(s)
		if len(trimmed) < 2 || trimmed[0] != '"' {
			return s
		}
		var inner string
		if err := json.Unmarshal([]byte(trimmed), &inner); err != nil {
			return s
		}
		if strings.TrimSpace(inner) == "" {
			return s
		}
		s = inner
	}
	return s
}

// parseBatchCallsString decodes the stringified 'calls' value into calls,
// returning the array text that parsed alongside them (for the outside-args
// check). Beyond a plain array it accepts the unambiguous slips: one object
// instead of an array of them, the whole arguments object stringified instead
// of just the array, a doubly-encoded string, and objects without the outer
// brackets. Anything else comes back as the plain array parse's error, so
// genuinely broken JSON still reports where it broke.
func parseBatchCallsString(trimmed string) ([]batchCall, string, error) {
	return parseBatchCallsStringDepth(trimmed, 0)
}

func parseBatchCallsStringDepth(trimmed string, depth int) ([]batchCall, string, error) {
	s := unwrapBatchJSONString(trimmed)
	var calls []batchCall
	directErr := json.Unmarshal([]byte(s), &calls)
	if directErr == nil {
		return calls, s, nil
	}
	// One object instead of an array of them: run it as a single call.
	// The tool name must be present so a wrapper object ({"calls": ...},
	// whose unknown fields decode to a zero call) falls through to the
	// wrapper repair below instead of running as a nameless call.
	if strings.HasPrefix(s, "{") {
		var single batchCall
		if serr := json.Unmarshal([]byte(s), &single); serr == nil && strings.TrimSpace(single.Tool) != "" {
			wrapped := "[" + s + "]"
			if werr := json.Unmarshal([]byte(wrapped), &calls); werr == nil {
				return calls, wrapped, nil
			}
		}
	}
	// The whole arguments object stringified instead of just the array
	// ({"calls": [...]} inside the string). Decode the inner value with the
	// same repairs, so its own slips are accepted too.
	if depth < 3 && strings.HasPrefix(s, "{") {
		var wrapper struct {
			Calls json.RawMessage `json:"calls"`
		}
		if werr := json.Unmarshal([]byte(s), &wrapper); werr == nil && len(wrapper.Calls) != 0 {
			var innerText string
			if uerr := json.Unmarshal(wrapper.Calls, &innerText); uerr == nil {
				innerText = strings.TrimSpace(innerText)
			} else {
				innerText = strings.TrimSpace(string(wrapper.Calls))
			}
			if innerText != "" {
				if icalls, ieffective, ierr := parseBatchCallsStringDepth(innerText, depth+1); ierr == nil {
					return icalls, ieffective, nil
				}
			}
		}
	}
	// Objects without the outer brackets: {"tool": ...}, {"tool": ...}.
	if strings.HasPrefix(s, "{") {
		wrapped := "[" + s + "]"
		if werr := json.Unmarshal([]byte(wrapped), &calls); werr == nil {
			return calls, wrapped, nil
		}
	}
	return nil, "", directErr
}

// parseBatchSingleCall accepts raw (non-string) 'calls' holding one object
// instead of an array of them. There is no legitimate call where it is a
// bare object, so a clean object decode runs as a single call; the later
// checks still report a missing tool name or an unknown tool.
func parseBatchSingleCall(raw json.RawMessage) (batchCall, bool) {
	var single batchCall
	trimmed := strings.TrimSpace(string(raw))
	if !strings.HasPrefix(trimmed, "{") {
		return batchCall{}, false
	}
	if err := json.Unmarshal([]byte(trimmed), &single); err != nil {
		return batchCall{}, false
	}
	return single, true
}

// batchErrorOffset is the byte offset of a JSON syntax error in s, or zero
// when the error carries none (a type error) or points past the end.
func batchErrorOffset(err error, s string) int {
	if se, ok := err.(*json.SyntaxError); ok && se.Offset > 0 && se.Offset <= int64(len(s)) {
		return int(se.Offset)
	}
	return 0
}

// batchErrorPreview shows the received string around the parse failure, so
// the model sees the misplaced brace even in a long batch whose head is
// fine. Short payloads come back whole; longer ones are cut to max runes
// around the failure, marked where they were cut.
func batchErrorPreview(s string, err error) string {
	runes := []rune(s)
	if len(runes) <= maxBatchPreview {
		return s
	}
	offset := batchErrorOffset(err, s)
	runeOffset := len([]rune(s[:offset]))
	start := runeOffset - maxBatchPreview/2
	if start < 0 {
		start = 0
	}
	end := start + maxBatchPreview
	if end > len(runes) {
		end = len(runes)
		start = end - maxBatchPreview
	}
	out := string(runes[start:end])
	if start > 0 {
		out = "…" + out
	}
	if end < len(runes) {
		out = out + "…"
	}
	return out
}

// maxBatchPreview bounds the received-string preview in the
// stringified-calls error: enough for the model to see the nesting slip,
// not the whole payload.
const maxBatchPreview = 300

// sanitizeBatchJSONError keeps the actionable part of a JSON parse failure
// (a syntax error and its position) while leaving out Go internals. A type
// error names Go values ("cannot unmarshal object into Go value of type
// []tools.batchCall"); that tells the model nothing about how to fix the
// shape, so it is replaced with what the shape should have been.
func sanitizeBatchJSONError(err error) string {
	if err == nil {
		return "invalid JSON"
	}
	msg := err.Error()
	if strings.Contains(msg, "cannot unmarshal") || strings.Contains(msg, "Go value") || strings.Contains(msg, "batchCall") || strings.Contains(msg, "batchArgs") {
		return "it is valid JSON but not an array of {tool, args} (for example a single object, or array elements missing tool/args)"
	}
	// A syntax error knows its byte offset; that plus the preview below is
	// what lets the model find the misplaced brace.
	if se, ok := err.(*json.SyntaxError); ok && se.Offset > 0 {
		return fmt.Sprintf("%s (at byte %d)", msg, se.Offset)
	}
	return msg
}

// extraBatchCallKeys names one call's fields beside tool/args.
type extraBatchCallKeys struct {
	index int
	keys  []string
}

// extraBatchKeys reports calls in a raw-array Batch payload that carry
// fields beside "tool" and "args". It returns nil when the payload is not
// a raw array (the stringified path checks its own string instead).
func extraBatchKeys(args json.RawMessage) []extraBatchCallKeys {
	var outer struct {
		Calls []map[string]json.RawMessage `json:"calls"`
	}
	if err := json.Unmarshal(args, &outer); err != nil {
		return nil
	}
	return extraKeysFromCallMaps(outer.Calls)
}

// extraBatchKeysFromString is the same check for the lenient path, where
// the calls array arrived as a JSON-encoded string that already parsed.
func extraBatchKeysFromString(trimmed string) []extraBatchCallKeys {
	var calls []map[string]json.RawMessage
	if err := json.Unmarshal([]byte(trimmed), &calls); err != nil {
		return nil
	}
	return extraKeysFromCallMaps(calls)
}

func extraKeysFromCallMaps(calls []map[string]json.RawMessage) []extraBatchCallKeys {
	var out []extraBatchCallKeys
	for i, m := range calls {
		var keys []string
		for k := range m {
			if k != "tool" && k != "args" {
				keys = append(keys, k)
			}
		}
		if len(keys) > 0 {
			out = append(out, extraBatchCallKeys{index: i + 1, keys: keys})
		}
	}
	return out
}

// quotedList renders keys as `"pattern"` or `"path", "pattern"` for errors.
func quotedList(keys []string) string {
	sorted := append([]string(nil), keys...)
	// Deterministic order so the prompt-cache prefix stays stable and the
	// message reads the same on every retry.
	sort.Strings(sorted)
	quoted := make([]string, len(sorted))
	for i, k := range sorted {
		quoted[i] = strconv.Quote(k)
	}
	if len(quoted) == 1 {
		return "field " + quoted[0]
	}
	return "fields " + strings.Join(quoted, ", ")
}

func (t *BatchTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	parsed, err := t.parse(args)
	if err != nil {
		return nil, err
	}
	registry := t.lookup()
	if registry == nil {
		return nil, RuntimeError("Batch: no tools are available to call")
	}

	results := make([]BatchResult, len(parsed.Calls))
	tc := t.context()

	// Calls that only read cannot interfere with each other, so they run at
	// the same time. Everything else runs in order afterwards, because it
	// changes the workspace or the pending-edit state that the next call reads.
	var wg sync.WaitGroup
	var sequential []int
	for i, call := range parsed.Calls {
		tool := registry.Get(call.Tool)
		if tool == nil {
			results[i] = BatchResult{Index: i, Tool: call.Tool, Error: fmt.Sprintf("unknown tool %q", call.Tool)}
			continue
		}
		if !isParallelSafe(tool) {
			if Changes(tool, call.Args) {
				tc.NoteChange()
			} else {
				tc.NoteMutatingTool()
			}
			sequential = append(sequential, i)
			continue
		}
		wg.Add(1)
		go func(i int, call batchCall, tool Tool) {
			defer wg.Done()
			results[i] = runBatchCall(ctx, i, call, tool)
		}(i, call, tool)
	}
	wg.Wait()

	for _, i := range sequential {
		select {
		case <-ctx.Done():
			results[i] = BatchResult{Index: i, Tool: parsed.Calls[i].Tool, Error: "cancelled before this call ran"}
			continue
		default:
		}
		results[i] = runBatchCall(ctx, i, parsed.Calls[i], registry.Get(parsed.Calls[i].Tool))
	}

	failed := 0
	for _, r := range results {
		if !r.OK {
			failed++
		}
	}
	return map[string]any{
		"results": results,
		"count":   len(results),
		"failed":  failed,
	}, nil
}

// runBatchCall runs one call the way the agent loop would: normalize the
// arguments, check, then call. A failure is recorded against that call alone.
func runBatchCall(ctx context.Context, index int, call batchCall, tool Tool) BatchResult {
	out := BatchResult{Index: index, Tool: call.Tool}

	args := call.Args
	if len(args) == 0 {
		args = json.RawMessage("{}")
	}
	if normalized, err := NormalizeToolCallArguments(tool, args); err == nil {
		args = normalized
	}

	if err := tool.Check(ctx, args); err != nil {
		out.Error = err.Error()
		return out
	}
	result, err := tool.Call(ctx, args)
	if err != nil {
		out.Error = err.Error()
		return out
	}
	out.OK = true
	out.Result = result
	// An image-bearing call keeps its attachments against its own result,
	// so the runner can say which call read which image.
	if carrier, ok := result.(ImageCarrier); ok {
		out.Images = carrier.ToolImages()
	}
	return out
}
func isParallelSafe(tool Tool) bool {
	if p, ok := tool.(ParallelSafeTool); ok {
		return p.ParallelSafe()
	}
	return false
}

// Mutates reports whether running this tool can change something outside the
// conversation: the workspace or a background process.
// It is the same question isParallelSafe asks for Batch, so the two answers
// cannot drift — a tool safe to run beside another is one that only reads.
//
// Report is excluded. It records what a turn did rather than doing anything,
// and counting it would make every reported turn look like a mutating one.
func Mutates(t Tool) bool {
	if t == nil || t.Name() == "Report" {
		return false
	}
	return !isParallelSafe(t)
}

// ReadOnlyCaller is a tool that can change things in general but can tell
// from one call's arguments that this call only reads. Shell is the case:
// ls and rm are both Shell calls, and only the command tells them apart.
type ReadOnlyCaller interface {
	ReadOnlyCall(args json.RawMessage) bool
}

// Changes reports whether this particular call can change something: the
// tool Mutates, and it cannot vouch for these arguments as read-only.
func Changes(t Tool, args json.RawMessage) bool {
	if !Mutates(t) {
		return false
	}
	if r, ok := t.(ReadOnlyCaller); ok && r.ReadOnlyCall(args) {
		return false
	}
	return true
}

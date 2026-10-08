package tools

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// batchTestTool is a tool whose Call a test controls, and which says whether it
// may run alongside others.
type batchTestTool struct {
	name     string
	parallel bool
	call     func(context.Context, json.RawMessage) (any, error)
	check    func(context.Context, json.RawMessage) error
}

func (b *batchTestTool) Name() string          { return b.name }
func (b *batchTestTool) Description() string   { return "test " + b.name }
func (b *batchTestTool) ParallelSafe() bool    { return b.parallel }
func (b *batchTestTool) PromptSection() string { return "" }
func (b *batchTestTool) JSONSchema() map[string]any {
	return map[string]any{"type": "object", "properties": map[string]any{}}
}
func (b *batchTestTool) Check(ctx context.Context, args json.RawMessage) error {
	if b.check == nil {
		return nil
	}
	return b.check(ctx, args)
}
func (b *batchTestTool) Call(ctx context.Context, args json.RawMessage) (any, error) {
	if b.call == nil {
		return map[string]any{"ok": true}, nil
	}
	return b.call(ctx, args)
}
func (b *batchTestTool) PromptCategory() string     { return "filesystem" }
func (b *batchTestTool) PromptOrder() int           { return 1 }
func (b *batchTestTool) PromptTemplateName() string { return "" }

func newBatchTool(t *testing.T, tools ...Tool) *BatchTool {
	t.Helper()
	cfg := &config.Config{}
	cfg.Tools.Batch.Enabled = true
	registry := NewRegistry()
	batch := NewBatchTool(cfg)
	registry.Enable(batch)
	for _, tool := range tools {
		registry.Enable(tool)
	}
	batch.SetRegistry(registry)
	return batch
}

func runBatch(t *testing.T, batch *BatchTool, calls ...map[string]any) []BatchResult {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"calls": calls})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := batch.Check(context.Background(), raw); err != nil {
		t.Fatalf("Check: %v", err)
	}
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	m := result.(map[string]any)
	return m["results"].([]BatchResult)
}

// TestReadOnlyCallsRunAtTheSameTime: four reads should take about as long as
// one, which is the reason the tool exists.
func TestReadOnlyCallsRunAtTheSameTime(t *testing.T) {
	const each = 200 * time.Millisecond
	var running, peak int64

	slowRead := &batchTestTool{name: "Read", parallel: true, call: func(context.Context, json.RawMessage) (any, error) {
		n := atomic.AddInt64(&running, 1)
		for {
			old := atomic.LoadInt64(&peak)
			if n <= old || atomic.CompareAndSwapInt64(&peak, old, n) {
				break
			}
		}
		time.Sleep(each)
		atomic.AddInt64(&running, -1)
		return map[string]any{"ok": true}, nil
	}}
	batch := newBatchTool(t, slowRead)

	start := time.Now()
	results := runBatch(t, batch,
		map[string]any{"tool": "Read", "args": map[string]any{"path": "a"}},
		map[string]any{"tool": "Read", "args": map[string]any{"path": "b"}},
		map[string]any{"tool": "Read", "args": map[string]any{"path": "c"}},
		map[string]any{"tool": "Read", "args": map[string]any{"path": "d"}},
	)
	elapsed := time.Since(start)

	if len(results) != 4 {
		t.Fatalf("got %d results, want 4", len(results))
	}
	for _, r := range results {
		if !r.OK {
			t.Errorf("call %d failed: %s", r.Index, r.Error)
		}
	}
	if elapsed > 3*each {
		t.Errorf("four reads took %s, want about as long as one (%s): they ran in sequence", elapsed, each)
	}
	if peak < 2 {
		t.Errorf("at most %d calls were ever running at once, want them overlapping", peak)
	}
}

// TestWritesRunInOrder: a tool that changes things runs after the reads and in
// the order it was given, because the call after it reads what it did.
func TestWritesRunInOrder(t *testing.T) {
	var order []string
	record := func(name string) func(context.Context, json.RawMessage) (any, error) {
		return func(context.Context, json.RawMessage) (any, error) {
			order = append(order, name)
			return map[string]any{"ok": true}, nil
		}
	}
	batch := newBatchTool(t,
		&batchTestTool{name: "Read", parallel: true, call: func(context.Context, json.RawMessage) (any, error) {
			return map[string]any{"ok": true}, nil
		}},
		&batchTestTool{name: "Edit", call: record("Edit")},
		&batchTestTool{name: "Write", call: record("Write")},
	)

	results := runBatch(t, batch,
		map[string]any{"tool": "Edit", "args": map[string]any{}},
		map[string]any{"tool": "Read", "args": map[string]any{}},
		map[string]any{"tool": "Write", "args": map[string]any{}},
	)

	if len(order) != 2 || order[0] != "Edit" || order[1] != "Write" {
		t.Errorf("the changing calls ran as %v, want Edit then Write", order)
	}
	// Results stay in the order the calls were given, whatever order they ran in.
	for i, r := range results {
		if r.Index != i {
			t.Errorf("result %d is for call %d: results are out of order", i, r.Index)
		}
	}
	if results[0].Tool != "Edit" || results[1].Tool != "Read" || results[2].Tool != "Write" {
		t.Errorf("results are %s/%s/%s, want them against the calls that were made",
			results[0].Tool, results[1].Tool, results[2].Tool)
	}
}

// TestOneFailureDoesNotStopTheRest: the point of a labelled result array.
func TestOneFailureDoesNotStopTheRest(t *testing.T) {
	batch := newBatchTool(t,
		&batchTestTool{name: "Read", parallel: true, call: func(_ context.Context, args json.RawMessage) (any, error) {
			if strings.Contains(string(args), "missing") {
				return nil, SemanticErrorf("no such file")
			}
			return map[string]any{"ok": true}, nil
		}},
	)

	results := runBatch(t, batch,
		map[string]any{"tool": "Read", "args": map[string]any{"path": "there"}},
		map[string]any{"tool": "Read", "args": map[string]any{"path": "missing"}},
		map[string]any{"tool": "Read", "args": map[string]any{"path": "also there"}},
	)

	if !results[0].OK || results[1].OK || !results[2].OK {
		t.Fatalf("results are %v/%v/%v, want the middle one alone to have failed",
			results[0].OK, results[1].OK, results[2].OK)
	}
	if !strings.Contains(results[1].Error, "no such file") {
		t.Errorf("the failing call's error is %q, want the tool's own message", results[1].Error)
	}
}

// TestBatchRefusesNesting and the other limits: a batch inside a batch, an
// unknown tool, an empty call list, and more calls than the cap.
func TestBatchLimits(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})

	mustFail := func(name string, calls []map[string]any) {
		t.Helper()
		raw, _ := json.Marshal(map[string]any{"calls": calls})
		if err := batch.Check(context.Background(), raw); err == nil {
			t.Errorf("%s was accepted", name)
		}
	}

	mustFail("a batch inside a batch", []map[string]any{
		{"tool": "Batch", "args": map[string]any{"calls": []any{}}},
	})
	mustFail("an unknown tool", []map[string]any{
		{"tool": "Nope", "args": map[string]any{}},
	})
	mustFail("no calls", nil)

	var many []map[string]any
	for range maxBatchCalls + 1 {
		many = append(many, map[string]any{"tool": "Read", "args": map[string]any{}})
	}
	mustFail("more calls than the limit", many)
}

func TestBatchAcceptsNamespacePrefix(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})

	raw, _ := json.Marshal(map[string]any{"calls": []map[string]any{
		{"tool": "default.Read", "args": map[string]any{}},
	}})
	if err := batch.Check(context.Background(), raw); err != nil {
		t.Fatalf("Check(default.Read) = %v, want it accepted", err)
	}
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	results := result.(map[string]any)["results"].([]BatchResult)
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("results = %+v, want one OK result", results)
	}
}

// TestBatchStringifiedCallsAccepted: some harnesses deliver the array as a
// JSON-encoded string ({"calls": "[{...}]"}). There is no legitimate call
// where 'calls' is a string, so Batch accepts it when it parses as an array.
func TestBatchStringifiedCallsAccepted(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})

	inner, _ := json.Marshal([]map[string]any{
		{"tool": "Read", "args": map[string]any{"path": "a"}},
		{"tool": "Read", "args": map[string]any{"path": "b"}},
	})
	raw, _ := json.Marshal(map[string]any{"calls": string(inner)})

	if err := batch.Check(context.Background(), raw); err != nil {
		t.Fatalf("Check(stringified calls) = %v, want it accepted", err)
	}
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	results := result.(map[string]any)["results"].([]BatchResult)
	if len(results) != 2 || !results[0].OK || !results[1].OK {
		t.Fatalf("results = %+v, want two OK results", results)
	}
}

// TestBatchStringifiedCallsInvalid: a string that does not parse as an array
// is rejected with an actionable message, not Go internals.
func TestBatchStringifiedCallsInvalid(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})

	raw, _ := json.Marshal(map[string]any{"calls": "not-json["})
	err := batch.Check(context.Background(), raw)
	if err == nil {
		t.Fatal("Check(stringified garbage) = nil, want an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, "must be an array") {
		t.Errorf("error is %q, want it to say 'calls' must be an array", msg)
	}
	for _, leaked := range []string{"cannot unmarshal", "batchArgs", "Go struct"} {
		if strings.Contains(msg, leaked) {
			t.Errorf("error %q leaks Go internals %q", msg, leaked)
		}
	}
}

// TestBatchStringifiedInnerArgsAccepted: the same double-encoding one level
// down ({"tool": "Read", "args": "{\"path\": \"a\"}"}) is decoded in place.
func TestBatchStringifiedInnerArgsAccepted(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{
		name:     "Read",
		parallel: true,
		call: func(_ context.Context, args json.RawMessage) (any, error) {
			var m map[string]any
			if err := json.Unmarshal(args, &m); err != nil {
				return nil, SemanticErrorf("args are not an object: %v", err)
			}
			if m["path"] != "a" {
				return nil, SemanticErrorf("want path a, got %v", m["path"])
			}
			return map[string]any{"ok": true}, nil
		},
	})

	raw, _ := json.Marshal(map[string]any{"calls": []map[string]any{
		{"tool": "Read", "args": "{\"path\": \"a\"}"},
	}})
	if err := batch.Check(context.Background(), raw); err != nil {
		t.Fatalf("Check = %v, want it accepted", err)
	}
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	results := result.(map[string]any)["results"].([]BatchResult)
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("results = %+v, want one OK result", results)
	}
}

// TestBatchStringifiedMalformedShowsPosition: the f0vfnx failure was a
// stringified array with a misplaced brace (a tool parameter outside args),
// so the string never parsed as an array. The error must keep saying the
// array must be raw, and add where the parse broke plus what was received.
func TestBatchStringifiedMalformedShowsPosition(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true}, &batchTestTool{name: "Shell", parallel: true}, &batchTestTool{name: "Glob", parallel: true})

	inner := `[{"args": {"command": "git log --oneline"}, "tool": "Shell"}, {"args": {"path": "/tmp"}, "pattern": "*.md"}, "tool": "Glob"}]`
	raw, _ := json.Marshal(map[string]any{"calls": inner})
	err := batch.Check(context.Background(), raw)
	if err == nil {
		t.Fatal("Check(malformed stringified calls) = nil, want an error")
	}
	msg := err.Error()
	for _, want := range []string{"must be an array", "not a JSON-encoded string", "inside its args", "Received"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q wants %q", msg, want)
		}
	}
	for _, leaked := range []string{"cannot unmarshal", "batchArgs", "batchCall", "Go struct", "Go value"} {
		if strings.Contains(msg, leaked) {
			t.Errorf("error %q leaks Go internals %q", msg, leaked)
		}
	}
}

// TestBatchStringifiedObjectAccepted: a string holding a single object (not
// an array) runs as one call. There is no legitimate Batch call where
// 'calls' is a bare object, so running it beats a round-trip error.
func TestBatchStringifiedObjectAccepted(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})
	raw, _ := json.Marshal(map[string]any{"calls": `{"tool": "Read", "args": {"path": "a"}}`})
	if err := batch.Check(context.Background(), raw); err != nil {
		t.Fatalf("Check(stringified object) = %v, want it accepted", err)
	}
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	results := result.(map[string]any)["results"].([]BatchResult)
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("results = %+v, want one OK result", results)
	}
}

// TestBatchStringifiedWrapperAccepted: the whole arguments object stringified
// instead of just the array ({"calls": "{\"calls\": [...]}"}) unwraps to the
// same calls.
func TestBatchStringifiedWrapperAccepted(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})
	inner, _ := json.Marshal([]map[string]any{
		{"tool": "Read", "args": map[string]any{"path": "a"}},
		{"tool": "Read", "args": map[string]any{"path": "b"}},
	})
	wrapper, _ := json.Marshal(map[string]any{"calls": string(inner)})
	raw, _ := json.Marshal(map[string]any{"calls": string(wrapper)})
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	results := result.(map[string]any)["results"].([]BatchResult)
	if len(results) != 2 || !results[0].OK || !results[1].OK {
		t.Fatalf("results = %+v, want two OK results", results)
	}
}

// TestBatchStringifiedMissingBracketsAccepted: objects without the outer
// brackets ({"tool": ...}, {"tool": ...}) run as the calls they plainly are.
func TestBatchStringifiedMissingBracketsAccepted(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})
	inner := `{"tool": "Read", "args": {"path": "a"}}, {"tool": "Read", "args": {"path": "b"}}`
	raw, _ := json.Marshal(map[string]any{"calls": inner})
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	results := result.(map[string]any)["results"].([]BatchResult)
	if len(results) != 2 || !results[0].OK || !results[1].OK {
		t.Fatalf("results = %+v, want two OK results", results)
	}
}

// TestBatchDoubleEncodedAccepted: a doubly-encoded payload (a string holding
// a string holding the array, or the whole arguments object twice-encoded)
// unwraps to the same calls instead of failing on the shape.
func TestBatchDoubleEncodedAccepted(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})
	inner, _ := json.Marshal([]map[string]any{
		{"tool": "Read", "args": map[string]any{"path": "a"}},
	})
	once, _ := json.Marshal(string(inner))
	raw, _ := json.Marshal(map[string]any{"calls": string(once)})
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call(string holding string holding array): %v", err)
	}
	results := result.(map[string]any)["results"].([]BatchResult)
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("results = %+v, want one OK result", results)
	}

	valid, _ := json.Marshal(map[string]any{"calls": []map[string]any{
		{"tool": "Read", "args": map[string]any{"path": "a"}},
	}})
	topTwice, _ := json.Marshal(string(valid))
	if err := batch.Check(context.Background(), topTwice); err != nil {
		t.Fatalf("Check(whole payload double-encoded) = %v, want it accepted", err)
	}
}

// TestBatchRawSingleObjectAccepted: raw (non-string) 'calls' holding one
// object instead of an array runs as a single call.
func TestBatchRawSingleObjectAccepted(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})
	raw, _ := json.Marshal(map[string]any{"calls": map[string]any{
		"tool": "Read", "args": map[string]any{"path": "a"},
	}})
	if err := batch.Check(context.Background(), raw); err != nil {
		t.Fatalf("Check(raw single object) = %v, want it accepted", err)
	}
	result, err := batch.Call(context.Background(), raw)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	results := result.(map[string]any)["results"].([]BatchResult)
	if len(results) != 1 || !results[0].OK {
		t.Fatalf("results = %+v, want one OK result", results)
	}
}

// TestBatchStringifiedSyntaxErrorShowsContext: a stringified array with a
// misplaced brace (the recurring hand-written-nesting slip) still fails, but
// the error points at the break: the byte offset and the text around it,
// without Go internals.
func TestBatchStringifiedSyntaxErrorShowsContext(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})
	inner := `[{"tool": "Read", "args": {{"path": "a"}}}]`
	raw, _ := json.Marshal(map[string]any{"calls": inner})
	err := batch.Check(context.Background(), raw)
	if err == nil {
		t.Fatal("Check(doubly-braced stringified calls) = nil, want an error")
	}
	msg := err.Error()
	for _, want := range []string{"must be an array", "not a JSON-encoded string", "around byte", "Received"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error %q wants %q", msg, want)
		}
	}
	for _, leaked := range []string{"cannot unmarshal", "batchArgs", "batchCall", "Go struct", "Go value"} {
		if strings.Contains(msg, leaked) {
			t.Errorf("error %q leaks Go internals %q", msg, leaked)
		}
	}
}

// TestBatchRawExtraKeysRejected: {"tool": "Glob", "args": {"path": ...},
// "pattern": ...} leaves a parameter outside args, where it would be
// silently dropped. Fail fast naming the field and its home.
func TestBatchRawExtraKeysRejected(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Glob", parallel: true})
	raw, _ := json.Marshal(map[string]any{"calls": []map[string]any{
		{"tool": "Glob", "args": map[string]any{"path": "/tmp"}, "pattern": "*.md"},
	}})
	err := batch.Check(context.Background(), raw)
	if err == nil {
		t.Fatal("Check(extra key outside args) = nil, want an error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"pattern"`) || !strings.Contains(msg, "outside 'args'") {
		t.Errorf("error is %q, want it to name \"pattern\" outside 'args'", msg)
	}
	if !strings.Contains(msg, "inside its args") {
		t.Errorf("error is %q, want it to say parameters go inside args", msg)
	}
}

// TestBatchPromptSectionStatesShapeRules: the prompt must warn against both
// halves of the f0vfnx slip before the model makes it.
func TestBatchPromptSectionStatesShapeRules(t *testing.T) {
	batch := newBatchTool(t, &batchTestTool{name: "Read", parallel: true})
	section := batch.PromptSection()
	for _, want := range []string{"never a JSON-encoded string", "inside its own 'args'"} {
		if !strings.Contains(section, want) {
			t.Errorf("PromptSection %q wants %q", section, want)
		}
	}
}

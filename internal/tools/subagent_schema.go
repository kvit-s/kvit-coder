package tools

// output_schema enforcement (phase 2b, spec/subagents-phase2.md section 6).
//
// When a Subagent call carries output_schema, the child handle gains a
// session-local structured_output tool whose handler validates against the
// compiled schema and captures the value in a per-call closure. Invalid
// input is an inline tool error the child can fix in the same run; a
// missing call gets up to maxNudges follow-up prompts through the
// SubagentSession. Still missing after the nudges, the result is labeled
// (same convention as budget_exhausted), never a bare error.
//
// The validator is santhosh-tekuri/jsonschema v6 (draft 2020-12, offline:
// no URLLoader is set, so a remote $ref fails at compile time rather than
// reaching the network). structured_output never reaches the parent
// registry, the session directory, or history.jsonl: it is built per Call
// and dies with the turn.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"
)

// StructuredOutputToolName is the session-local tool a schema-carrying child
// must call with its final result.
const StructuredOutputToolName = "structured_output"

// Labels for results that never satisfied their contract. Same convention
// as [budget_exhausted: ...]: a labeled result, never a bare error.
const (
	schemaUnsatisfiedLabel = "[schema_unsatisfied: no valid structured_output call after %d follow-ups; what follows is the partial summary]"
	summaryMissingLabel    = "[summary_missing: the child returned no text after %d follow-ups]"
)

// hasOutputSchema reports whether the caller supplied an output_schema.
func hasOutputSchema(raw json.RawMessage) bool {
	s := strings.TrimSpace(string(raw))
	return len(raw) > 0 && s != "" && s != "null"
}

// checkOutputSchemaShape fast-validates output_schema at Check time: it
// must be a JSON object (a schema). Anything else is the model's fault and
// is rejected before spawning. Deeper problems (a schema that does not
// compile) surface at Call time, likewise before any spawn.
func checkOutputSchemaShape(raw json.RawMessage) error {
	if !hasOutputSchema(raw) {
		return nil
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return SemanticErrorf("Subagent: output_schema must be a JSON Schema object (type object): %v", err)
	}
	return nil
}

// compileOutputSchema compiles the caller's schema at Call time. A compile
// failure is the model's fault (backtrackable) and never a spawned run.
func compileOutputSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, SemanticErrorf("Subagent: output_schema is not valid JSON: %v", err)
	}
	c := jsonschema.NewCompiler()
	// No URLLoader is set, so remote $refs fail here instead of dialing
	// out: offline by construction.
	if err := c.AddResource("schema.json", doc); err != nil {
		return nil, SemanticErrorf("Subagent: output_schema cannot be registered: %v", err)
	}
	sch, err := c.Compile("schema.json")
	if err != nil {
		return nil, SemanticErrorf("Subagent: output_schema does not compile: %v", err)
	}
	return sch, nil
}

// validateAgainstSchema checks one candidate value against the compiled
// schema, returning every validation error (possibly empty). The instance
// is decoded with UnmarshalJSON so numbers keep integer semantics.
func validateAgainstSchema(sch *jsonschema.Schema, args json.RawMessage) []string {
	inst, err := jsonschema.UnmarshalJSON(bytes.NewReader(args))
	if err != nil {
		return []string{fmt.Sprintf("arguments are not valid JSON: %v", err)}
	}
	if err := sch.Validate(inst); err == nil {
		return nil
	} else if ve, ok := err.(*jsonschema.ValidationError); ok {
		return flattenValidationErrors(ve)
	} else {
		return []string{err.Error()}
	}
}

// flattenValidationErrors collects the leaf messages of a validation error
// tree, most specific first. Only the first maxSchemaErrors are ever shown
// to the child; the full list stays on the capture for the nudge prompts.
func flattenValidationErrors(ve *jsonschema.ValidationError) []string {
	var out []string
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) == 0 {
			out = append(out, e.Error())
			return
		}
		for _, c := range e.Causes {
			walk(c)
		}
	}
	walk(ve)
	return out
}

// StructuredOutputCapture is the per-Call closure a structured_output tool
// records into: whether the child filed a valid result, its canonical
// value, and the most recent validation failures (for nudge prompts).
type StructuredOutputCapture struct {
	mu         sync.Mutex
	called     bool
	value      json.RawMessage
	lastErrors []string
}

// Snapshot returns the capture's current state.
func (c *StructuredOutputCapture) Snapshot() (called bool, value json.RawMessage, lastErrors []string) {
	if c == nil {
		return false, nil, nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.called, c.value, append([]string(nil), c.lastErrors...)
}

func (c *StructuredOutputCapture) recordSuccess(value json.RawMessage) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.called = true
	c.value = value
	c.lastErrors = nil
}

func (c *StructuredOutputCapture) recordFailure(errs []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lastErrors = append([]string(nil), errs...)
}

// StructuredOutputTool is the session-local tool a schema-carrying child
// calls with its final result. It exists only in the child's registry for
// one Subagent.Call (and the child's Batch, which dispatches through the
// same handle — no smuggling, it is born there).
type StructuredOutputTool struct {
	schema   map[string]any
	compiled *jsonschema.Schema
	capture  *StructuredOutputCapture
}

// NewStructuredOutputTool builds the session-local tool for one call. The
// schema must already have passed checkOutputSchemaShape; a schema that
// fails to compile is the model's fault.
func NewStructuredOutputTool(schemaJSON json.RawMessage, capture *StructuredOutputCapture) (Tool, error) {
	compiled, err := compileOutputSchema(schemaJSON)
	if err != nil {
		return nil, err
	}
	var schema map[string]any
	if err := json.Unmarshal(schemaJSON, &schema); err != nil {
		return nil, SemanticErrorf("Subagent: output_schema is not a JSON object: %v", err)
	}
	if capture == nil {
		capture = &StructuredOutputCapture{}
	}
	return &StructuredOutputTool{schema: schema, compiled: compiled, capture: capture}, nil
}

func (t *StructuredOutputTool) Name() string { return StructuredOutputToolName }

func (t *StructuredOutputTool) Description() string {
	return "Submit your final result. Call this once, with the complete result as arguments matching the required schema, instead of ending your turn with prose. " +
		"A rejected value comes back as an error naming what failed: fix the value and call again."
}

func (t *StructuredOutputTool) JSONSchema() map[string]any { return t.schema }

func (t *StructuredOutputTool) PromptCategory() string     { return "filesystem" }
func (t *StructuredOutputTool) PromptOrder() int           { return 61 }
func (t *StructuredOutputTool) PromptTemplateName() string { return "" }
func (t *StructuredOutputTool) PromptSection() string      { return "" }

func (t *StructuredOutputTool) Check(_ context.Context, _ json.RawMessage) error { return nil }

func (t *StructuredOutputTool) Call(_ context.Context, args json.RawMessage) (any, error) {
	errs := validateAgainstSchema(t.compiled, args)
	if len(errs) > 0 {
		t.capture.recordFailure(errs)
		shown := errs
		if len(shown) > maxSchemaErrors {
			shown = shown[:maxSchemaErrors]
		}
		return nil, SemanticErrorf("structured_output: the value does not match the required schema:\n- %s\nFix the value and call structured_output again.",
			strings.Join(shown, "\n- "))
	}
	var v any
	if err := json.Unmarshal(args, &v); err != nil {
		// Unreachable in practice: validation already parsed the same
		// bytes. Fail closed rather than capture what was not checked.
		return nil, SemanticErrorf("structured_output: arguments are not valid JSON: %v", err)
	}
	canonical, err := json.Marshal(v)
	if err != nil {
		return nil, SemanticErrorf("structured_output: cannot encode the value: %v", err)
	}
	t.capture.recordSuccess(canonical)
	return map[string]any{"accepted": true}, nil
}

// schemaNudgePrompt demands the missing structured_output call, carrying
// the previous attempt's failures so the child can fix them in-run.
func schemaNudgePrompt(lastErrors []string) string {
	var sb strings.Builder
	sb.WriteString("Call the structured_output tool with your complete final result as its arguments, matching the required schema. Do not end your turn with prose instead.")
	if len(lastErrors) > 0 {
		shown := lastErrors
		if len(shown) > maxSchemaErrors {
			shown = shown[:maxSchemaErrors]
		}
		sb.WriteString(" Your previous attempt failed validation:\n- ")
		sb.WriteString(strings.Join(shown, "\n- "))
	}
	return sb.String()
}

// emptyNudgePrompt asks a silent child for the summary it owes.
const emptyNudgePrompt = "Your response was empty. Reply with a concise summary of what you found."

// accumulateSubagentResult folds one continuation run into the total the
// parent sees: the latest text wins, usage adds up, flags stick.
func accumulateSubagentResult(total *SubagentRunResult, add SubagentRunResult) {
	total.Text = add.Text
	total.PromptTokens += add.PromptTokens
	total.CompletionTokens += add.CompletionTokens
	total.Cost += add.Cost
	total.Iterations += add.Iterations
	total.BudgetExhausted = total.BudgetExhausted || add.BudgetExhausted
	total.TimedOut = total.TimedOut || add.TimedOut
	if add.Model != "" {
		total.Model = add.Model
	}
}

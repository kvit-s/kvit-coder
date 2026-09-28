package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// parseFlexibleIntString parses strings like "120", " 120 ", "120s", "120sec",
// "120.0" into an int. Empty (or whitespace-only) strings report empty=true so
// the caller can drop the field instead of erroring on it.
func parseFlexibleIntString(s string) (val int, empty bool, ok bool) {
	trimmed := strings.TrimSpace(s)
	if trimmed == "" {
		return 0, true, false
	}
	lower := strings.ToLower(trimmed)
	// Strip a trailing seconds suffix: "120s", "120sec", "120secs",
	// "120second", "120seconds". Only when something numeric remains.
	for _, suffix := range []string{"seconds", "second", "secs", "sec", "s"} {
		if strings.HasSuffix(lower, suffix) && len(lower) > len(suffix) {
			candidate := strings.TrimSpace(trimmed[:len(trimmed)-len(suffix)])
			if candidate != "" {
				trimmed = candidate
				break
			}
		}
	}
	if intVal, err := strconv.Atoi(trimmed); err == nil {
		return intVal, false, true
	}
	if floatVal, err := strconv.ParseFloat(trimmed, 64); err == nil {
		return int(floatVal), false, true
	}
	return 0, false, false
}

// coerceToInt converts the loose values LLMs produce (numeric strings,
// floats like 120.0) into an int for schema-declared integer fields.
// It reports empty=true for "" so the field can be treated as omitted.
func coerceToInt(v any) (val int, empty bool, ok bool) {
	switch t := v.(type) {
	case string:
		return parseFlexibleIntString(t)
	case float64:
		return int(t), false, true
	case bool:
		if t {
			return 1, false, true
		}
		return 0, false, true
	case int:
		return t, false, true
	case nil:
		return 0, true, false
	default:
		return 0, false, false
	}
}

func coerceToFloat(v any) (val float64, empty bool, ok bool) {
	switch t := v.(type) {
	case string:
		trimmed := strings.TrimSpace(t)
		if trimmed == "" {
			return 0, true, false
		}
		if f, err := strconv.ParseFloat(trimmed, 64); err == nil {
			return f, false, true
		}
		// Accept "120s" for numeric fields too.
		if iv, _, ok := parseFlexibleIntString(t); ok {
			return float64(iv), false, true
		}
		return 0, false, false
	case float64:
		return t, false, true
	case int:
		return float64(t), false, true
	case bool:
		if t {
			return 1, false, true
		}
		return 0, false, true
	case nil:
		return 0, true, false
	default:
		return 0, false, false
	}
}

func coerceToBool(v any) (val bool, ok bool) {
	switch t := v.(type) {
	case bool:
		return t, true
	case string:
		switch strings.ToLower(strings.TrimSpace(t)) {
		case "true", "1", "yes", "y", "on":
			return true, true
		case "false", "0", "no", "n", "off":
			return false, true
		}
		return false, false
	case float64:
		if t == 1 {
			return true, true
		}
		if t == 0 {
			return false, true
		}
		return false, false
	default:
		return false, false
	}
}

func coerceToString(v any) (val string, ok bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case float64:
		if t == float64(int(t)) {
			return strconv.Itoa(int(t)), true
		}
		return strconv.FormatFloat(t, 'f', -1, 64), true
	case bool:
		if t {
			return "true", true
		}
		return "false", true
	default:
		return "", false
	}
}

// normalizeToolArguments converts string representations of numbers to actual numbers
// in tool arguments to handle cases where LLM sends "200" instead of 200.
// It is deliberately lenient: "120s", "120.0" and 120.0 all become 120 for
// integer fields, "" is treated as omitted, and "true"/"false" become booleans.
func normalizeToolArguments(args json.RawMessage, schema map[string]any) (json.RawMessage, error) {
	// Parse the arguments into a map
	var argsMap map[string]interface{}
	if err := json.Unmarshal(args, &argsMap); err != nil {
		return args, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Get the properties from the schema
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		// No properties in schema, still apply the heuristic fallback for
		// well-known numeric fields (e.g. Shell with a timeout but a
		// command-only schema), then return.
		coerceWellKnownNumericFields(argsMap)
		normalizedArgs, err := json.Marshal(argsMap)
		if err != nil {
			return args, fmt.Errorf("failed to re-serialize normalized arguments: %w", err)
		}
		return normalizedArgs, nil
	}

	// Normalize each argument based on the schema
	for paramName, paramSchema := range properties {
		if argsMap[paramName] == nil {
			continue // Parameter not provided, skip
		}

		paramSchemaMap, ok := paramSchema.(map[string]any)
		if !ok {
			continue // Invalid schema format, skip
		}

		// Check the type in the schema
		paramType, ok := paramSchemaMap["type"].(string)
		if !ok {
			continue // No type specified, skip
		}

		raw := argsMap[paramName]
		switch paramType {
		case "integer":
			if coerced, empty, ok := coerceToInt(raw); ok {
				argsMap[paramName] = coerced
			} else if empty {
				delete(argsMap, paramName)
			}
			// Unparseable strings stay as-is so the tool can report
			// a friendly field-level error instead of a Go type error.
		case "number":
			if coerced, empty, ok := coerceToFloat(raw); ok {
				argsMap[paramName] = coerced
			} else if empty {
				delete(argsMap, paramName)
			}
		case "boolean":
			if coerced, ok := coerceToBool(raw); ok {
				argsMap[paramName] = coerced
			}
		case "string":
			if _, isStr := raw.(string); !isStr {
				if coerced, ok := coerceToString(raw); ok {
					argsMap[paramName] = coerced
				}
			}
		}
	}

	// A tool whose schema omits a field the model sent anyway (Shell with a
	// timeout, from before its schema declared one) would otherwise keep the
	// raw string and fail in json.Unmarshal with a Go internals error.
	coerceWellKnownNumericFields(argsMap)

	// Re-serialize the normalized arguments
	normalizedArgs, err := json.Marshal(argsMap)
	if err != nil {
		return args, fmt.Errorf("failed to re-serialize normalized arguments: %w", err)
	}

	return normalizedArgs, nil
}

// wellKnownIntegerFields are coerced even when the tool's schema does not
// declare them, so a model sending a plausible extra (Shell with
// {"command": ..., "timeout": "60"}) gets a conversion, not a type error.
var wellKnownIntegerFields = []string{
	"timeout", "max_wait", "poll", "cursor", "limit", "start",
	"count", "line", "start_line", "end_line",
}

func coerceWellKnownNumericFields(argsMap map[string]interface{}) {
	for _, name := range wellKnownIntegerFields {
		raw, present := argsMap[name]
		if !present || raw == nil {
			continue
		}
		// Already numeric: normalize floats like 120.0 to 120.
		if f, isFloat := raw.(float64); isFloat {
			argsMap[name] = int(f)
			continue
		}
		if _, isInt := raw.(int); isInt {
			continue
		}
		if s, isStr := raw.(string); isStr {
			if coerced, empty, ok := coerceToInt(s); ok {
				argsMap[name] = coerced
			} else if empty {
				delete(argsMap, name)
			}
		}
	}
}

// NormalizeToolCallArguments is a middleware that normalizes tool arguments
// by converting string numbers to actual numbers when the schema expects numeric types
func NormalizeToolCallArguments(tool Tool, args json.RawMessage) (json.RawMessage, error) {
	schema := tool.JSONSchema()
	if schema == nil {
		return args, nil // No schema, return original
	}

	return normalizeToolArguments(args, schema)
}

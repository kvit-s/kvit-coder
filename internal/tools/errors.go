package tools

import (
	"encoding/json"
	"fmt"
)

// ToolErrorType classifies tool errors for backtracking decisions
type ToolErrorType int

const (
	// ToolErrorRuntime - Tool executed but failed (file not found, network error, etc.)
	// NOT backtrackable - error goes to history, LLM should see and handle it
	ToolErrorRuntime ToolErrorType = iota

	// ToolErrorSemantic - LLM misused the tool (wrong sequence, invalid state, etc.)
	// Backtrackable - discard and retry, LLM should have known better from prompt
	ToolErrorSemantic
)

// ToolError is an error type that classifies errors as runtime or semantic
type ToolError struct {
	Type    ToolErrorType
	Message string
	Details map[string]any // Optional structured data for LLM
}

// Error implements the error interface
func (e *ToolError) Error() string {
	return e.Message
}

// ToJSON implements JSONError interface for structured output
func (e *ToolError) ToJSON() map[string]any {
	result := map[string]any{
		"success": false,
		"error":   e.Message,
	}
	// Merge in details if present
	for k, v := range e.Details {
		result[k] = v
	}
	return result
}

// RuntimeError creates a runtime error (not backtrackable)
// Use for: file system errors, network errors, external failures
func RuntimeError(msg string) *ToolError {
	return &ToolError{Type: ToolErrorRuntime, Message: msg}
}

// RuntimeErrorf creates a formatted runtime error
func RuntimeErrorf(format string, args ...any) *ToolError {
	return &ToolError{Type: ToolErrorRuntime, Message: fmt.Sprintf(format, args...)}
}

// RuntimeErrorWithDetails creates a runtime error with structured details
func RuntimeErrorWithDetails(msg string, details map[string]any) *ToolError {
	return &ToolError{Type: ToolErrorRuntime, Message: msg, Details: details}
}

// SemanticError creates a semantic error (backtrackable)
// Use for: LLM misuse, wrong tool sequence, invalid state, unknown tools
func SemanticError(msg string) *ToolError {
	return &ToolError{Type: ToolErrorSemantic, Message: msg}
}

// SemanticErrorf creates a formatted semantic error
func SemanticErrorf(format string, args ...any) *ToolError {
	return &ToolError{Type: ToolErrorSemantic, Message: fmt.Sprintf(format, args...)}
}

// SemanticErrorWithDetails creates a semantic error with structured details
func SemanticErrorWithDetails(msg string, details map[string]any) *ToolError {
	return &ToolError{Type: ToolErrorSemantic, Message: msg, Details: details}
}

// IsBacktrackable checks if an error should trigger backtracking
// Returns true only for semantic errors where the LLM should have known better
func IsBacktrackable(err error) bool {
	if te, ok := err.(*ToolError); ok {
		return te.Type == ToolErrorSemantic
	}
	return false // Unknown errors are not backtrackable (safe default)
}

// WrapAsRuntime wraps any error as a runtime error
func WrapAsRuntime(err error) *ToolError {
	if err == nil {
		return nil
	}
	if te, ok := err.(*ToolError); ok {
		return te // Already a ToolError, preserve its type
	}
	return RuntimeError(err.Error())
}

// WrapAsSemantic wraps any error as a semantic error
func WrapAsSemantic(err error) *ToolError {
	if err == nil {
		return nil
	}
	if te, ok := err.(*ToolError); ok {
		return te // Already a ToolError, preserve its type
	}
	return SemanticError(err.Error())
}

// FormatToolError returns a formatted string representation of a ToolError
// If the error has details, returns JSON; otherwise returns plain message
func FormatToolError(err *ToolError) string {
	if err == nil {
		return ""
	}
	if len(err.Details) > 0 {
		jsonBytes, marshalErr := json.MarshalIndent(err.ToJSON(), "", "  ")
		if marshalErr == nil {
			return string(jsonBytes)
		}
	}
	return fmt.Sprintf("Error: %s", err.Message)
}

// JSONError is an interface for errors that can provide structured JSON output
type JSONError interface {
	error
	ToJSON() map[string]any
}

// FormatError checks if an error implements JSONError and returns JSON, otherwise returns plain text
func FormatError(err error) string {
	if jsonErr, ok := err.(JSONError); ok {
		jsonBytes, marshalErr := json.MarshalIndent(jsonErr.ToJSON(), "", "  ")
		if marshalErr == nil {
			return string(jsonBytes)
		}
	}
	return fmt.Sprintf("Error: %v", err)
}

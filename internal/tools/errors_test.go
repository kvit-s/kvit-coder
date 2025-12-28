package tools

import (
	"errors"
	"testing"
)

func TestToolErrorType(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantBacktrack  bool
		wantMessage    string
	}{
		{
			name:          "semantic error is backtrackable",
			err:           SemanticError("file not read"),
			wantBacktrack: true,
			wantMessage:   "file not read",
		},
		{
			name:          "runtime error is not backtrackable",
			err:           RuntimeError("network timeout"),
			wantBacktrack: false,
			wantMessage:   "network timeout",
		},
		{
			name:          "regular error is not backtrackable",
			err:           errors.New("some error"),
			wantBacktrack: false,
			wantMessage:   "some error",
		},
		{
			name:          "nil is not backtrackable",
			err:           nil,
			wantBacktrack: false,
			wantMessage:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBacktrackable(tt.err)
			if got != tt.wantBacktrack {
				t.Errorf("IsBacktrackable() = %v, want %v", got, tt.wantBacktrack)
			}

			if tt.err != nil && tt.err.Error() != tt.wantMessage {
				t.Errorf("Error() = %v, want %v", tt.err.Error(), tt.wantMessage)
			}
		})
	}
}

func TestSemanticErrorWithDetails(t *testing.T) {
	err := SemanticErrorWithDetails("file not read", map[string]any{
		"path":      "/path/to/file",
		"next_step": "read the file first",
	})

	if !IsBacktrackable(err) {
		t.Error("SemanticErrorWithDetails should be backtrackable")
	}

	json := err.ToJSON()
	if json["path"] != "/path/to/file" {
		t.Errorf("Expected path '/path/to/file', got %v", json["path"])
	}
	if json["next_step"] != "read the file first" {
		t.Errorf("Expected next_step, got %v", json["next_step"])
	}
	if json["success"] != false {
		t.Errorf("Expected success=false, got %v", json["success"])
	}
}

func TestWrapAsSemantic(t *testing.T) {
	// Wrapping regular error
	regularErr := errors.New("some error")
	wrapped := WrapAsSemantic(regularErr)
	if !IsBacktrackable(wrapped) {
		t.Error("WrapAsSemantic should make error backtrackable")
	}

	// Wrapping existing ToolError preserves its type
	runtime := RuntimeError("runtime")
	wrappedRuntime := WrapAsSemantic(runtime)
	if IsBacktrackable(wrappedRuntime) {
		t.Error("WrapAsSemantic should preserve existing ToolError type")
	}

	// Wrapping nil returns nil
	if WrapAsSemantic(nil) != nil {
		t.Error("WrapAsSemantic(nil) should return nil")
	}
}

func TestWrapAsRuntime(t *testing.T) {
	// Wrapping regular error
	regularErr := errors.New("some error")
	wrapped := WrapAsRuntime(regularErr)
	if IsBacktrackable(wrapped) {
		t.Error("WrapAsRuntime should make error non-backtrackable")
	}

	// Wrapping existing ToolError preserves its type
	semantic := SemanticError("semantic")
	wrappedSemantic := WrapAsRuntime(semantic)
	if !IsBacktrackable(wrappedSemantic) {
		t.Error("WrapAsRuntime should preserve existing ToolError type")
	}

	// Wrapping nil returns nil
	if WrapAsRuntime(nil) != nil {
		t.Error("WrapAsRuntime(nil) should return nil")
	}
}

func TestFormatToolError(t *testing.T) {
	// Error without details
	simpleErr := SemanticError("simple error")
	output := FormatToolError(simpleErr)
	if output != "Error: simple error" {
		t.Errorf("Expected 'Error: simple error', got %v", output)
	}

	// Error with details returns JSON
	detailedErr := SemanticErrorWithDetails("detailed error", map[string]any{
		"path": "/file",
	})
	output = FormatToolError(detailedErr)
	if output == "Error: detailed error" {
		t.Error("Expected JSON output for error with details")
	}

	// Nil error
	if FormatToolError(nil) != "" {
		t.Error("FormatToolError(nil) should return empty string")
	}
}

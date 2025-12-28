package tools

import (
	"context"
	"encoding/json"
	"testing"
)

// MockTool is a simple tool for testing
type MockTool struct{}

func (t *MockTool) Name() string {
	return "test"
}

func (t *MockTool) Description() string {
	return "Test tool"
}

func (t *MockTool) JSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"string_param": map[string]interface{}{
				"type": "string",
			},
			"int_param": map[string]interface{}{
				"type": "integer",
			},
			"number_param": map[string]interface{}{
				"type": "number",
			},
			"bool_param": map[string]interface{}{
				"type": "boolean",
			},
		},
		"required": []string{"int_param"},
	}
}

func (t *MockTool) Check(ctx context.Context, args json.RawMessage) error {
	return nil
}

func (t *MockTool) Call(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return nil, nil
}

func (t *MockTool) PromptSection() string {
	return ""
}

func (t *MockTool) PromptCategory() string {
	return "test"
}

func (t *MockTool) PromptOrder() int {
	return 1
}

func TestNormalizeToolCallArguments(t *testing.T) {
	tool := &MockTool{}

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "string numbers to integers",
			input:    `{"string_param": "hello", "int_param": "42", "number_param": "3.14"}`,
			expected: `{"int_param":42,"number_param":3.14,"string_param":"hello"}`,
		},
		{
			name:     "already correct types",
			input:    `{"string_param": "hello", "int_param": 42, "number_param": 3.14}`,
			expected: `{"int_param":42,"number_param":3.14,"string_param":"hello"}`,
		},
		{
			name:     "invalid string numbers remain strings",
			input:    `{"string_param": "hello", "int_param": "not_a_number"}`,
			expected: `{"int_param":"not_a_number","string_param":"hello"}`,
		},
		{
			name:     "empty object",
			input:    `{}`,
			expected: `{}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := json.RawMessage(tc.input)
			normalized, err := NormalizeToolCallArguments(tool, args)
			if err != nil {
				t.Fatalf("NormalizeToolCallArguments failed: %v", err)
			}

			// Parse both to compare as objects (order doesn't matter)
			var normalizedMap, expectedMap map[string]interface{}
			if err := json.Unmarshal(normalized, &normalizedMap); err != nil {
				t.Fatalf("Failed to parse normalized result: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.expected), &expectedMap); err != nil {
				t.Fatalf("Failed to parse expected result: %v", err)
			}

			// Compare the maps
			if len(normalizedMap) != len(expectedMap) {
				t.Errorf("Length mismatch: got %d fields, want %d", len(normalizedMap), len(expectedMap))
			}

			for key, expectedVal := range expectedMap {
				actualVal, ok := normalizedMap[key]
				if !ok {
					t.Errorf("Missing key: %s", key)
					continue
				}

				if actualVal != expectedVal {
					t.Errorf("Value mismatch for %s: got %v (%T), want %v (%T)", key, actualVal, actualVal, expectedVal, expectedVal)
				}
			}
		})
	}
}

func TestNormalizeToolCallArgumentsWithNoSchema(t *testing.T) {
	tool := &MockToolNoSchema{}
	args := json.RawMessage(`{"param": "value"}`)

	normalized, err := NormalizeToolCallArguments(tool, args)
	if err != nil {
		t.Fatalf("NormalizeToolCallArguments failed: %v", err)
	}

	// Should return original arguments when no schema
	if string(normalized) != string(args) {
		t.Errorf("Expected original arguments, got: %s", string(normalized))
	}
}

// MockToolNoSchema is a tool without a schema for testing
type MockToolNoSchema struct{}

func (t *MockToolNoSchema) Name() string {
	return "test_no_schema"
}

func (t *MockToolNoSchema) Description() string {
	return "Test tool without schema"
}

func (t *MockToolNoSchema) JSONSchema() map[string]interface{} {
	return nil
}

func (t *MockToolNoSchema) Check(ctx context.Context, args json.RawMessage) error {
	return nil
}

func (t *MockToolNoSchema) Call(ctx context.Context, args json.RawMessage) (interface{}, error) {
	return nil, nil
}

func (t *MockToolNoSchema) PromptSection() string {
	return ""
}

func (t *MockToolNoSchema) PromptCategory() string {
	return "test"
}

func (t *MockToolNoSchema) PromptOrder() int {
	return 1
}
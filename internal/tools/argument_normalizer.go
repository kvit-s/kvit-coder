package tools

import (
	"encoding/json"
	"fmt"
	"strconv"
)

// normalizeToolArguments converts string representations of numbers to actual numbers
// in tool arguments to handle cases where LLM sends "200" instead of 200
func normalizeToolArguments(args json.RawMessage, schema map[string]any) (json.RawMessage, error) {
	// Parse the arguments into a map
	var argsMap map[string]interface{}
	if err := json.Unmarshal(args, &argsMap); err != nil {
		return args, fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	// Get the properties from the schema
	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		// No properties in schema, return original arguments
		return args, nil
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

		// Handle string values that should be numbers
		if paramType == "integer" || paramType == "number" {
			if strVal, ok := argsMap[paramName].(string); ok {
				// Try to convert string to number
				if paramType == "integer" {
					if intVal, err := strconv.Atoi(strVal); err == nil {
						argsMap[paramName] = intVal
					}
				} else { // number (float)
					if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
						argsMap[paramName] = floatVal
					}
				}
			}
		}
	}

	// Re-serialize the normalized arguments
	normalizedArgs, err := json.Marshal(argsMap)
	if err != nil {
		return args, fmt.Errorf("failed to re-serialize normalized arguments: %w", err)
	}

	return normalizedArgs, nil
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
package mcp

import (
	"reflect"
	"testing"
)

func TestSanitizeSchema_StripsUnsupportedConstructs(t *testing.T) {
	in := map[string]any{
		"type":    "object",
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$defs":   map[string]any{"X": map[string]any{"type": "string"}},
		"oneOf":   []any{map[string]any{"type": "string"}},
		"properties": map[string]any{
			"path": map[string]any{
				"type":        "string",
				"description": "a path",
				"$ref":        "#/$defs/X",
			},
			"mode": map[string]any{
				"anyOf": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "null"},
				},
			},
		},
		"required": []any{"path"},
	}
	out := SanitizeSchema(in)

	if _, ok := out["$schema"]; ok {
		t.Error("$schema should be stripped")
	}
	if _, ok := out["$defs"]; ok {
		t.Error("$defs should be stripped")
	}
	if _, ok := out["oneOf"]; ok {
		t.Error("oneOf should be stripped")
	}
	props, ok := out["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties missing: %#v", out)
	}
	pathProp := props["path"].(map[string]any)
	if _, ok := pathProp["$ref"]; ok {
		t.Error("$ref should be stripped from nested property")
	}
	if pathProp["type"] != "string" || pathProp["description"] != "a path" {
		t.Errorf("supported keys not preserved: %#v", pathProp)
	}
	// mode had only an anyOf (stripped) and no other keys -> empty object node.
	modeProp := props["mode"].(map[string]any)
	if len(modeProp) != 0 {
		t.Errorf("expected mode subschema stripped to empty, got %#v", modeProp)
	}
	if !reflect.DeepEqual(out["required"], []any{"path"}) {
		t.Errorf("required not preserved: %#v", out["required"])
	}

	// Input must not be mutated.
	if _, ok := in["oneOf"]; !ok {
		t.Error("input schema was mutated")
	}
}

func TestSanitizeSchema_EmptyAndNil(t *testing.T) {
	for _, in := range []map[string]any{nil, {}} {
		out := SanitizeSchema(in)
		if out["type"] != "object" {
			t.Errorf("expected {type:object} fallback, got %#v", out)
		}
	}
}

func TestSanitizeSchema_InfersObjectTypeFromProperties(t *testing.T) {
	in := map[string]any{
		// no "type", only a combinator + properties
		"allOf": []any{map[string]any{"type": "object"}},
		"properties": map[string]any{
			"x": map[string]any{"type": "integer"},
		},
	}
	out := SanitizeSchema(in)
	if out["type"] != "object" {
		t.Errorf("expected inferred type object, got %#v", out)
	}
}

func TestSanitizeSchema_NestedArrayItems(t *testing.T) {
	in := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
					"$ref": "#/$defs/Tag",
				},
			},
		},
	}
	out := SanitizeSchema(in)
	tags := out["properties"].(map[string]any)["tags"].(map[string]any)
	items := tags["items"].(map[string]any)
	if _, ok := items["$ref"]; ok {
		t.Error("$ref should be stripped from array items")
	}
	if items["type"] != "string" {
		t.Errorf("array item type not preserved: %#v", items)
	}
}

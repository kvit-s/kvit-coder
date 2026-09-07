package mcp

// Schema sanitization. Grammar-constrained tool calling on some llama.cpp/vLLM
// builds rejects JSON Schema constructs like $ref, oneOf, allOf, anyOf, or
// conditional (if/then/else) subschemas. SanitizeSchema rewrites a server's
// inputSchema into a permissive subset those backends accept: it keeps a
// whitelist of widely-supported keywords, recurses into nested schemas, and
// drops everything else. A node left with nothing usable becomes a bare
// {"type":"object"} so the model can still call the tool. This is opt-in
// (mcp.sanitize_schemas) so faithful schemas are the default.

// supportedKeywords are JSON Schema keywords kept verbatim (their values are
// scalars or arrays of scalars, not subschemas).
var supportedKeywords = map[string]bool{
	"type":             true,
	"description":      true,
	"title":            true,
	"enum":             true,
	"const":            true,
	"default":          true,
	"required":         true,
	"minimum":          true,
	"maximum":          true,
	"exclusiveMinimum": true,
	"exclusiveMaximum": true,
	"multipleOf":       true,
	"minLength":        true,
	"maxLength":        true,
	"pattern":          true,
	"format":           true,
	"minItems":         true,
	"maxItems":         true,
	"uniqueItems":      true,
	"minProperties":    true,
	"maxProperties":    true,
}

// SanitizeSchema returns a sanitized copy of schema, leaving the input
// untouched. A nil or empty schema yields {"type":"object"}.
func SanitizeSchema(schema map[string]any) map[string]any {
	if len(schema) == 0 {
		return map[string]any{"type": "object"}
	}
	out := sanitizeNode(schema)
	if len(out) == 0 {
		return map[string]any{"type": "object"}
	}
	return out
}

// sanitizeNode sanitizes one schema node recursively.
func sanitizeNode(node map[string]any) map[string]any {
	out := map[string]any{}

	for k, v := range node {
		if supportedKeywords[k] {
			out[k] = v
		}
	}

	// properties: map of name -> subschema
	if props, ok := node["properties"].(map[string]any); ok {
		cleanProps := map[string]any{}
		for name, sub := range props {
			if subMap, ok := sub.(map[string]any); ok {
				cleanProps[name] = sanitizeNode(subMap)
			}
		}
		if len(cleanProps) > 0 {
			out["properties"] = cleanProps
		}
	}

	// items: a subschema or an array of subschemas
	switch items := node["items"].(type) {
	case map[string]any:
		out["items"] = sanitizeNode(items)
	case []any:
		var cleaned []any
		for _, it := range items {
			if itMap, ok := it.(map[string]any); ok {
				cleaned = append(cleaned, sanitizeNode(itMap))
			}
		}
		if len(cleaned) > 0 {
			out["items"] = cleaned
		}
	}

	// additionalProperties: keep booleans; sanitize subschemas.
	switch ap := node["additionalProperties"].(type) {
	case bool:
		out["additionalProperties"] = ap
	case map[string]any:
		out["additionalProperties"] = sanitizeNode(ap)
	}

	// Infer object type when properties survived but type was dropped (e.g. the
	// node only declared a combinator we stripped).
	if _, hasType := out["type"]; !hasType {
		if _, hasProps := out["properties"]; hasProps {
			out["type"] = "object"
		}
	}

	return out
}

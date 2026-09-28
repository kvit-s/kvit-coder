package config

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/copilot"
	"gopkg.in/yaml.v3"
)

// This file is the `kvit-coder copilot models-add` command's planning and
// YAML editing. The command itself (cmd/kvit-coder/copilot.go) stays thin:
// it fetches the catalog, calls PlanCopilotAdditions, then writes the result
// with AppendModelsToYAML or previews it with FormatModelsYAML.

// CopilotAddPlan is what `models-add` will write and what it left out.
type CopilotAddPlan struct {
	Add              []ModelEntry
	SkippedExisting  int
	SkippedEmbedding int
	SkippedDisabled  int
}

// PlanCopilotAdditions turns the callable catalog models into config rows,
// skipping disabled and embedding models and rows the file already has. host
// is the normalized -host flag, recorded on new rows when non-empty.
//
// existing holds the `models:` rows already in the file; only ID, Provider
// and Model are read (see ExistingModels). New rows leave api_backend empty
// on purpose: the program reads the protocol from the account's live model
// list, so a row keeps working when Copilot moves a model to another
// protocol.
func PlanCopilotAdditions(models []copilot.ListedModel, existing []ModelEntry, host string) CopilotAddPlan {
	var res CopilotAddPlan
	byWire := map[string]bool{}
	ids := map[string]bool{}
	for _, e := range existing {
		ids[strings.ToLower(e.ID)] = true
		if e.Provider == copilot.Provider && e.Model != "" {
			byWire[e.Model] = true
		}
	}
	for _, m := range models {
		if m.Disabled {
			res.SkippedDisabled++
			continue
		}
		if copilot.IsEmbeddingModel(m) {
			res.SkippedEmbedding++
			continue
		}
		if byWire[m.ID] {
			res.SkippedExisting++
			continue
		}
		res.Add = append(res.Add, copilotEntryFor(m, host, ids))
	}
	return res
}

// copilotEntryFor builds the config row for one catalog model. ids holds the
// entry ids taken so far (lower-cased) and gains the new one, so successive
// calls in one plan never collide.
func copilotEntryFor(m copilot.ListedModel, host string, ids map[string]bool) ModelEntry {
	id := "copilot-" + m.ID
	if ids[strings.ToLower(id)] {
		for n := 2; ; n++ {
			try := fmt.Sprintf("%s-%d", id, n)
			if !ids[strings.ToLower(try)] {
				id = try
				break
			}
		}
	}
	ids[strings.ToLower(id)] = true

	name := m.Name
	if name == "" {
		name = m.ID
	}
	if !strings.Contains(strings.ToLower(name), "copilot") {
		name += " (Copilot)"
	}

	var efforts []EffortOption
	for _, v := range m.Efforts {
		efforts = append(efforts, EffortOption{Value: v})
	}
	// The default is the middle of the advertised menu when there is one,
	// else the first level offered.
	for i, e := range efforts {
		if strings.EqualFold(e.Value, "medium") {
			efforts[i].Default = true
			break
		}
	}
	if len(efforts) > 0 {
		hasDefault := false
		for _, e := range efforts {
			hasDefault = hasDefault || e.Default
		}
		if !hasDefault {
			efforts[0].Default = true
		}
	}

	return ModelEntry{
		ID: id, Name: name, Provider: copilot.Provider, Model: m.ID,
		CopilotHost: host, Context: m.Context, Efforts: efforts,
	}
}

// ExistingModels decodes the `models:` list already in the config file.
// Only the ID, Provider and Model of each row are populated; the rest is for
// PlanCopilotAdditions' skip check, not for display. A missing list is not
// an error: there is simply nothing to skip.
func ExistingModels(raw []byte) ([]ModelEntry, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}
	root := doc.Content
	if len(root) == 0 {
		return nil, nil
	}
	seq := findModelsNode(root[0], "models")
	if seq == nil {
		return nil, nil
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("the models: section is not a list")
	}
	var out []ModelEntry
	for _, item := range seq.Content {
		var e ModelEntry
		for i := 0; i+1 < len(item.Content); i += 2 {
			k := item.Content[i].Value
			v := item.Content[i+1].Value
			switch k {
			case "id":
				e.ID = v
			case "provider":
				e.Provider = v
			case "model":
				e.Model = v
			}
		}
		out = append(out, e)
	}
	return out, nil
}

// AppendModelsToYAML appends the new rows to the config file's `models:`
// list, creating the list when absent. Comments elsewhere in the file are
// kept: yaml.v3 carries them on the nodes through the round trip.
func AppendModelsToYAML(raw []byte, add []ModelEntry) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("cannot parse config: the top level is not a mapping")
	}
	root := doc.Content[0]
	seq := findModelsNode(root, "models")
	if seq == nil {
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "models"}
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, key, seq)
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("the models: section is not a list")
	}
	for _, e := range add {
		seq.Content = append(seq.Content, copilotModelNode(e))
	}
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc.Content[0]); err != nil {
		return nil, fmt.Errorf("cannot encode config: %w", err)
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

// FormatModelsYAML renders new rows as YAML documents for the --dry-run
// preview. Empty fields are omitted, as in the appended file.
func FormatModelsYAML(add []ModelEntry) ([]byte, error) {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	for _, e := range add {
		if err := enc.Encode(copilotModelNode(e)); err != nil {
			_ = enc.Close()
			return nil, err
		}
	}
	_ = enc.Close()
	return buf.Bytes(), nil
}

// copilotModelNode renders one generated row as a YAML mapping node. Only
// the identity, context, effort menu and host are written; api_backend stays
// empty so the protocol tracks the live model list.
func copilotModelNode(e ModelEntry) *yaml.Node {
	pairs := []struct {
		key   string
		value *yaml.Node
	}{
		{"id", yamlStrNode(e.ID)},
		{"name", yamlStrNode(e.Name)},
		{"provider", yamlStrNode(copilot.Provider)},
		{"model", yamlStrNode(e.Model)},
	}
	if e.Context > 0 {
		pairs = append(pairs, struct {
			key   string
			value *yaml.Node
		}{"context", yamlIntNode(e.Context)})
	}
	if len(e.Efforts) > 0 {
		items := make([]*yaml.Node, 0, len(e.Efforts))
		for _, o := range e.Efforts {
			m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			m.Content = append(m.Content, yamlStrNode("value"), yamlStrNode(o.Value))
			if o.Default {
				m.Content = append(m.Content, yamlStrNode("default"), yamlBoolNode(true))
			}
			items = append(items, m)
		}
		seq := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		seq.Content = items
		pairs = append(pairs, struct {
			key   string
			value *yaml.Node
		}{"efforts", seq})
	}
	if e.CopilotHost != "" {
		pairs = append(pairs, struct {
			key   string
			value *yaml.Node
		}{"copilot_host", yamlStrNode(e.CopilotHost)})
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, p := range pairs {
		m.Content = append(m.Content, yamlStrNode(p.key), p.value)
	}
	return m
}

func yamlStrNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

func yamlIntNode(n int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprint(n)}
}

func yamlBoolNode(b bool) *yaml.Node {
	v := "false"
	if b {
		v = "true"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
}

// findModelsNode returns the value node for key in a mapping node, or nil.
func findModelsNode(m *yaml.Node, key string) *yaml.Node {
	if m.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/copilot"
	"gopkg.in/yaml.v3"
)

// runCopilot serves `kvit-coder copilot ...`. `models` prints the models the
// signed-in Copilot account can call; `models-add` appends them to the config
// file's `models:` list, skipping embedding models and rows already there.
func runCopilot(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		copilotUsage()
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	switch args[0] {
	case "models":
		return runCopilotModels(args[1:])
	case "models-add":
		return runCopilotModelsAdd(args[1:])
	default:
		copilotUsage()
		return 2
	}
}

func copilotUsage() {
	fmt.Fprintln(os.Stderr, "usage: kvit-coder copilot models [-config path] [-host hostname]")
	fmt.Fprintln(os.Stderr, "       kvit-coder copilot models-add [-config path] [-host hostname] [-dry-run]")
}

func runCopilotModels(args []string) int {
	fs := flag.NewFlagSet("copilot models", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config file (default: the usual search)")
	hostFlag := fs.String("host", "", "GitHub Enterprise hostname, for example company.ghe.com")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	opt, code := copilotListOptions(*configPath, *hostFlag)
	if code != 0 {
		return code
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	catalog, err := copilot.List(ctx, opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}
	auth := "GitHub token sent directly"
	if catalog.Exchanged {
		auth = "exchanged session token"
	}
	fmt.Printf("API: %s\n", catalog.APIBase)
	if catalog.SKU != "" {
		fmt.Printf("Auth: %s (%s)\n", auth, catalog.SKU)
	} else {
		fmt.Printf("Auth: %s\n", auth)
	}
	fmt.Println()
	shown := 0
	for _, m := range catalog.Models {
		// GitHub returns the whole catalog, including models this account's
		// policy has turned off. The list is for choosing one to call.
		if m.Disabled {
			continue
		}
		name := m.Name
		if name == "" {
			name = m.ID
		}
		line := fmt.Sprintf("%s\t%s\tcontext %d", m.ID, m.Protocol, m.Context)
		if len(m.Efforts) > 0 {
			line += "\tefforts " + strings.Join(m.Efforts, ",")
		}
		line += "\t" + name
		fmt.Println(line)
		shown++
	}
	if shown == 0 {
		fmt.Println("No models this account can call.")
	}
	return 0
}

func runCopilotModelsAdd(args []string) int {
	fs := flag.NewFlagSet("copilot models-add", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config file (default: the usual search)")
	hostFlag := fs.String("host", "", "GitHub Enterprise hostname, for example company.ghe.com")
	dryRun := fs.Bool("dry-run", false, "print the entries that would be added without writing the config file")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	host := ""
	if *hostFlag != "" {
		h, err := copilot.NormalizeHost(*hostFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
			return 2
		}
		host = h
	}

	// The config file is resolved before the network call so a missing file
	// fails fast without waiting on the token exchange.
	path, _, err := config.ResolvePath(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
		return 1
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub Copilot: cannot read %s: %v\n", path, err)
		return 1
	}

	opt, code := copilotListOptions(*configPath, *hostFlag)
	if code != 0 {
		return code
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	catalog, err := copilot.List(ctx, opt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		return 1
	}

	existing, err := existingModelRefs(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub Copilot: cannot read models: in %s: %v\n", path, err)
		return 1
	}
	res := planCopilotAdditions(catalog.Models, existing, host)
	if len(res.Add) == 0 {
		fmt.Printf("Nothing to add: %d already in %s, %d embedding, %d disabled.\n",
			res.SkippedExisting, path, res.SkippedEmbedding, res.SkippedDisabled)
		return 0
	}

	if *dryRun {
		var buf bytes.Buffer
		enc := yaml.NewEncoder(&buf)
		enc.SetIndent(2)
		for _, e := range res.Add {
			if err := enc.Encode(copilotEntryNode(e)); err != nil {
				fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
				return 1
			}
		}
		_ = enc.Close()
		fmt.Printf("Would add %d models to %s (%d already present, %d embedding, %d disabled):\n\n",
			len(res.Add), path, res.SkippedExisting, res.SkippedEmbedding, res.SkippedDisabled)
		fmt.Print(buf.String())
		return 0
	}

	out, err := appendModelsToYAML(raw, res.Add)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
		return 1
	}
	if err := writeFileKeepMode(path, out); err != nil {
		fmt.Fprintf(os.Stderr, "GitHub Copilot: cannot write %s: %v\n", path, err)
		return 1
	}
	fmt.Printf("Added %d models to %s (%d already present, %d embedding, %d disabled).\n",
		len(res.Add), path, res.SkippedExisting, res.SkippedEmbedding, res.SkippedDisabled)
	for _, e := range res.Add {
		fmt.Printf("  %s (%s)\n", e.ID, e.Model)
	}
	return 0
}

// copilotListOptions builds the list options from the config file and the
// -host flag, shared by `models` and `models-add`. A non-zero code with no
// error means usage was already reported.
func copilotListOptions(configFlag, hostFlag string) (copilot.Options, int) {
	opt := copilot.Options{}
	if cfg, err := copilotCommandConfig(configFlag); err != nil {
		fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
		return opt, 1
	} else if cfg != nil {
		opt = copilotOptionsFromConfig(cfg)
	}
	if hostFlag != "" {
		host, err := copilot.NormalizeHost(hostFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
			return opt, 2
		}
		opt.Host = host
	}
	return opt, 0
}

// copilotConfigEntry is one `models:` row generated from a catalog model.
// api_backend is left empty on purpose: the program reads the protocol from
// the account's model list, so a row keeps working when Copilot moves a
// model to another protocol.
type copilotConfigEntry struct {
	ID      string
	Name    string
	Model   string
	Context int
	Efforts []copilotEffort
	Host    string
}

// copilotEffort is one row of a generated effort menu.
type copilotEffort struct {
	Value   string
	Default bool
}

// existingModelRef is the identity of a `models:` row already in the file.
type existingModelRef struct {
	ID       string
	Provider string
	Model    string
}

// copilotAddPlan is what `models-add` will write and what it left out.
type copilotAddPlan struct {
	Add              []copilotConfigEntry
	SkippedExisting  int
	SkippedEmbedding int
	SkippedDisabled  int
}

// isEmbeddingModel reports whether a catalog model is an embedding model.
// Those have no chat endpoint, so they are useless as agent models. GitHub
// serves them as text-embedding-*, with context 0 and no chat protocol.
func isEmbeddingModel(m copilot.ListedModel) bool {
	if strings.Contains(strings.ToLower(m.ID), "embed") {
		return true
	}
	if strings.Contains(strings.ToLower(m.Name), "embed") {
		return true
	}
	for _, e := range m.Endpoints {
		if strings.Contains(strings.ToLower(e), "embed") {
			return true
		}
	}
	return false
}

// planCopilotAdditions turns the callable catalog models into config rows,
// skipping disabled and embedding models and rows the file already has. host
// is the normalized -host flag, recorded on new rows when non-empty.
func planCopilotAdditions(models []copilot.ListedModel, existing []existingModelRef, host string) copilotAddPlan {
	var res copilotAddPlan
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
		if isEmbeddingModel(m) {
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
func copilotEntryFor(m copilot.ListedModel, host string, ids map[string]bool) copilotConfigEntry {
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

	var efforts []copilotEffort
	for _, v := range m.Efforts {
		efforts = append(efforts, copilotEffort{Value: v})
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

	return copilotConfigEntry{
		ID: id, Name: name, Model: m.ID,
		Context: m.Context, Efforts: efforts, Host: host,
	}
}

// existingModelRefs decodes the `models:` list already in the config file.
// A missing list is not an error: there is simply nothing to skip.
func existingModelRefs(raw []byte) ([]existingModelRef, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}
	root := doc.Content
	if len(root) == 0 {
		return nil, nil
	}
	seq := findMappingValue(root[0], "models")
	if seq == nil {
		return nil, nil
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("the models: section is not a list")
	}
	var out []existingModelRef
	for _, item := range seq.Content {
		var e existingModelRef
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

// appendModelsToYAML appends the new rows to the config file's `models:`
// list, creating the list when absent. Comments elsewhere in the file are
// kept: yaml.v3 carries them on the nodes through the round trip.
func appendModelsToYAML(raw []byte, add []copilotConfigEntry) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("cannot parse config: %w", err)
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("cannot parse config: the top level is not a mapping")
	}
	root := doc.Content[0]
	seq := findMappingValue(root, "models")
	if seq == nil {
		key := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "models"}
		seq = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		root.Content = append(root.Content, key, seq)
	}
	if seq.Kind != yaml.SequenceNode {
		return nil, fmt.Errorf("the models: section is not a list")
	}
	for _, e := range add {
		seq.Content = append(seq.Content, copilotEntryNode(e))
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

// copilotEntryNode renders one generated row as a YAML mapping node.
func copilotEntryNode(e copilotConfigEntry) *yaml.Node {
	pairs := []struct {
		key   string
		value *yaml.Node
	}{
		{"id", strNode(e.ID)},
		{"name", strNode(e.Name)},
		{"provider", strNode(copilot.Provider)},
		{"model", strNode(e.Model)},
	}
	if e.Context > 0 {
		pairs = append(pairs, struct {
			key   string
			value *yaml.Node
		}{"context", intNode(e.Context)})
	}
	if len(e.Efforts) > 0 {
		items := make([]*yaml.Node, 0, len(e.Efforts))
		for _, o := range e.Efforts {
			m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
			m.Content = append(m.Content, strNode("value"), strNode(o.Value))
			if o.Default {
				m.Content = append(m.Content, strNode("default"), boolNode(true))
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
	if e.Host != "" {
		pairs = append(pairs, struct {
			key   string
			value *yaml.Node
		}{"copilot_host", strNode(e.Host)})
	}
	m := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	for _, p := range pairs {
		m.Content = append(m.Content, strNode(p.key), p.value)
	}
	return m
}

func strNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s}
}

func intNode(n int) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!int", Value: fmt.Sprint(n)}
}

func boolNode(b bool) *yaml.Node {
	v := "false"
	if b {
		v = "true"
	}
	return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!bool", Value: v}
}

// findMappingValue returns the value node for key in a mapping node, or nil.
func findMappingValue(m *yaml.Node, key string) *yaml.Node {
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

// writeFileKeepMode rewrites a file that already exists, keeping its mode.
func writeFileKeepMode(path string, data []byte) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, fi.Mode().Perm())
}

// copilotCommandConfig loads a config when one can be found. A missing file
// is not an error: the command still works from the environment and the
// Copilot CLI sign-in. An explicit -config that does not load is an error.
func copilotCommandConfig(explicit string) (*config.Config, error) {
	path, _, err := config.ResolvePath(explicit)
	if err != nil {
		if explicit != "" {
			return nil, err
		}
		return nil, nil
	}
	cfg, err := config.Load(path)
	if err != nil {
		return nil, err
	}
	return cfg, nil
}

// copilotOptionsFromConfig takes the host and GitHub token from the first
// github-copilot model, or from the llm: block when that is the provider.
// The model id is left empty: this command lists every model, it does not
// call one.
func copilotOptionsFromConfig(cfg *config.Config) copilot.Options {
	opt := copilot.Options{Host: cfg.LLM.CopilotHost}
	if cfg.LLM.Provider == copilot.Provider {
		opt.GitHubToken = cfg.LLM.APIKey
		if cfg.LLM.BaseURL != "" {
			opt.BaseURL = cfg.LLM.BaseURL
		}
	}
	for _, entry := range cfg.Models {
		if entry.Provider != copilot.Provider {
			continue
		}
		if entry.CopilotHost != "" {
			opt.Host = entry.CopilotHost
		}
		if token := config.EntryAPIKey(entry); token != "" {
			opt.GitHubToken = token
		}
		if entry.BaseURL != "" {
			opt.BaseURL = entry.BaseURL
		}
		break
	}
	return opt
}

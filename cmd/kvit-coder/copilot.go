package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/copilot"
)

// runCopilot serves `kvit-coder copilot ...`. `models` prints the models the
// signed-in Copilot account can call; `models-add` appends them to the config
// file's `models:` list, skipping embedding models and rows already there.
// Planning and YAML editing live in internal/config; this file only parses
// flags, prints, and writes the result.
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

	existing, err := config.ExistingModels(raw)
	if err != nil {
		fmt.Fprintf(os.Stderr, "GitHub Copilot: cannot read models: in %s: %v\n", path, err)
		return 1
	}
	res := config.PlanCopilotAdditions(catalog.Models, existing, host)
	if len(res.Add) == 0 {
		fmt.Printf("Nothing to add: %d already in %s, %d embedding, %d disabled.\n",
			res.SkippedExisting, path, res.SkippedEmbedding, res.SkippedDisabled)
		return 0
	}

	if *dryRun {
		preview, err := config.FormatModelsYAML(res.Add)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
			return 1
		}
		fmt.Printf("Would add %d models to %s (%d already present, %d embedding, %d disabled):\n\n",
			len(res.Add), path, res.SkippedExisting, res.SkippedEmbedding, res.SkippedDisabled)
		fmt.Print(string(preview))
		return 0
	}

	out, err := config.AppendModelsToYAML(raw, res.Add)
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

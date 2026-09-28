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

// runCopilot serves `kvit-coder copilot ...`. The only subcommand is
// `models`, which prints the models the signed-in Copilot account can call
// and the API host the token exchange selected.
func runCopilot(args []string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] != "models" {
		fmt.Fprintln(os.Stderr, "usage: kvit-coder copilot models [-config path] [-host hostname]")
		if len(args) > 0 && args[0] != "models" && args[0] != "-h" && args[0] != "--help" {
			return 2
		}
		if len(args) == 0 {
			return 2
		}
		return 0
	}
	fs := flag.NewFlagSet("copilot models", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	configPath := fs.String("config", "", "config file (default: the usual search)")
	hostFlag := fs.String("host", "", "GitHub Enterprise hostname, for example company.ghe.com")
	if err := fs.Parse(args[1:]); err != nil {
		return 2
	}

	opt := copilot.Options{}
	if cfg, err := copilotCommandConfig(*configPath); err != nil {
		fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
		return 1
	} else if cfg != nil {
		opt = copilotOptionsFromConfig(cfg)
	}
	if *hostFlag != "" {
		host, err := copilot.NormalizeHost(*hostFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "GitHub Copilot: %v\n", err)
			return 2
		}
		opt.Host = host
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

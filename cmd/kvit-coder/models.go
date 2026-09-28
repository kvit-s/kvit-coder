package main

import (
	"log"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// stringFlag is a flag.Value that remembers whether it was set, so
// --effort "" (clear the effort: omit `reasoning:`) differs from no --effort
// flag at all (keep the entry default).
type stringFlag struct {
	set bool
	val string
}

func (s *stringFlag) String() string { return s.val }

func (s *stringFlag) Set(v string) error {
	s.val = v
	s.set = true
	return nil
}

// resolveModelSelection applies the --model/--effort/--api-backend/--base-url
// overrides to cfg right after Load, before the client is built and before
// the session records what this turn ran with. -m/-e are shorthand for
// --model/--effort; --model also takes an inline ":effort" ("m3:xhigh").
// With a `models:` catalog the default entry — not the `llm:` block — is the
// base, so a headless run with no flags matches what kvit-coder-ui shows for
// the same config. `llm:` stays the source of the non-endpoint settings.
// Unknown models or efforts fail on stderr with a non-zero exit and the same
// offered-list error the UI prints.
func resolveModelSelection(cfg *config.Config, modelRef string, effort *stringFlag, apiBackend, baseURL string) {
	// A -m/--model value may carry an inline ":effort" ("m3:xhigh",
	// "qwen-local:low"); an explicit --effort wins over it.
	inlineEffort := ""
	if modelRef != "" {
		if mp, ie := config.SplitModelEffort(modelRef); ie != "" && !effort.set {
			modelRef, inlineEffort = mp, ie
		} else {
			modelRef = mp
		}
	}
	if len(cfg.Models) > 0 {
		list := cfg.ModelList()
		idx := cfg.DefaultModelIndex()
		if idx < 0 || idx >= len(list) {
			idx = 0
		}
		base := list[idx]
		cfg.ApplyModel(base, cfg.DefaultEffort(base))
	}
	if modelRef != "" {
		entry, _, err := cfg.ResolveModel(modelRef)
		if err != nil {
			if len(cfg.Models) > 0 {
				log.Fatalf("Failed to select model: %v", err)
			}
			// Legacy single-model config: --model is a raw wire-id
			// override, exactly as before. It cannot resolve against a
			// catalog that does not exist, and a skewed UI/agent config
			// pair still reaches the right endpoint via the explicit
			// --base-url/--api-backend below.
			cfg.LLM.Model = modelRef
			if effort.set {
				if effort.val != "" && !config.IsCanonicalEffort(effort.val) {
					log.Fatalf("Failed to select effort: unknown effort level %q", effort.val)
				}
				cfg.LLM.ReasoningEffort = effort.val
			} else if inlineEffort != "" {
				if !config.IsCanonicalEffort(inlineEffort) {
					log.Fatalf("Failed to select effort: unknown effort level %q", inlineEffort)
				}
				cfg.LLM.ReasoningEffort = inlineEffort
			}
		} else {
			eff := cfg.DefaultEffort(entry)
			useEffort, useSet := inlineEffort, inlineEffort != ""
			if effort.set {
				useEffort, useSet = effort.val, true
			}
			if useSet {
				v, rerr := cfg.ResolveEffort(entry, useEffort)
				if rerr != nil {
					log.Fatalf("Failed to select effort: %v", rerr)
				}
				eff = v
			}
			cfg.ApplyModel(entry, eff)
		}
	} else if effort.set || inlineEffort != "" {
		list := cfg.ModelList()
		idx := cfg.DefaultModelIndex()
		if idx < 0 || idx >= len(list) {
			idx = 0
		}
		useEffort := inlineEffort
		if effort.set {
			useEffort = effort.val
		}
		v, rerr := cfg.ResolveEffort(list[idx], useEffort)
		if rerr != nil {
			log.Fatalf("Failed to select effort: %v", rerr)
		}
		cfg.LLM.ReasoningEffort = v
	}
	if baseURL != "" {
		cfg.LLM.BaseURL = baseURL
	}
	if apiBackend != "" {
		if !config.ValidBackend(apiBackend) {
			log.Fatalf("Failed to select backend: unknown api_backend %q; use \"chat_completions\", \"responses\" or \"messages\"", apiBackend)
		}
		cfg.UseAPIBackend(apiBackend)
	}
}

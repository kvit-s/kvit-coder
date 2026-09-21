package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/llm"
)

// This file is the `models:` catalog from docs/model-selection.md: several
// named endpoints in config.yaml, picked with :mN in kvit-coder-ui (or
// --model headless) and a per-model effort menu picked with :eN (or --effort).
//
// When `models:` is absent the catalog is one entry synthesized from the
// legacy single-model `llm:` block, so old configs behave exactly as before.

// CanonicalEfforts are the reasoning-effort values the Responses backend
// passes through verbatim (cf. grok's ReasoningEffort enum). The config's
// per-model menu is the authority on what is *selectable*; this set is only
// what is *spellable*.
var CanonicalEfforts = []string{"none", "minimal", "low", "medium", "high", "xhigh", "max"}

// LegacyEfforts is the effort menu for a `responses` model that lists no
// explicit menu, in presentation order.
var LegacyEfforts = []string{"xhigh", "high", "medium", "low"}

// IsCanonicalEffort reports whether s names a known effort value.
func IsCanonicalEffort(s string) bool {
	for _, e := range CanonicalEfforts {
		if strings.EqualFold(s, e) {
			return true
		}
	}
	return false
}

// ValidBackend reports whether b names a known wire protocol (or is empty,
// which means the chat-completions default).
func ValidBackend(b string) bool {
	return b == "" || b == llm.BackendChatCompletions || b == llm.BackendResponses
}

// EffortOption is one row of a model's :eN menu. Order in the file is
// presentation order; Default marks the entry's effort default.
type EffortOption struct {
	Value   string `yaml:"value"`
	Label   string `yaml:"label"`
	Default bool   `yaml:"default"`
}

// Display returns the menu text: the label when set, else the value.
func (e EffortOption) Display() string {
	if e.Label != "" {
		return e.Label
	}
	return e.Value
}

// ModelEntry is one catalog row: everything about an endpoint that switching
// models can change. Non-endpoint settings (headers, timeouts,
// merge_thinking, ...) always come from the `llm:` block — except the agent
// profile, which an entry may override per model (see ProfileFor).
type ModelEntry struct {
	ID         string         `yaml:"id"`
	Name       string         `yaml:"name"`
	Model      string         `yaml:"model"`
	BaseURL    string         `yaml:"base_url"`
	APIBackend string         `yaml:"api_backend"`
	APIKey     string         `yaml:"api_key"`
	APIKeyEnv  string         `yaml:"api_key_env"`
	Context    int            `yaml:"context"`
	Efforts    []EffortOption `yaml:"efforts"`

	// EffortField names which field of a chat-completions request carries
	// the reasoning effort for this endpoint: "chat_template_kwargs" (the
	// default, for a model whose chat template reads it) or
	// "reasoning_effort" (OpenAI's own top-level field, which hosted
	// gateways read). Empty keeps the `llm:` block's setting. An endpoint
	// that wants one spelling ignores the other silently, so a wrong value
	// means the effort picked with :eN does nothing at all. The "responses"
	// backend ignores this.
	EffortField string `yaml:"effort_field"`

	// Profile overrides agent.profile for this entry: "strong" (the default)
	// or "weak". Empty inherits the global agent.profile. A weak entry keeps
	// the compensation machinery (backtracking, the duplicate-call kill
	// switch, the edit confirm handshake, fuzzy matching, anomaly
	// interrogation, prose scraping, empty-answer retries, interpreter
	// one-liner rules); a strong one skips it.
	Profile string `yaml:"profile"`

	// Summarizer marks the entry used to generate session titles: a short
	// 3-6 word summary of the first prompt, stored in meta.json. At most
	// one entry may set it; when none does (or the call fails) the title
	// falls back to the first words of the prompt. It never affects model
	// switching — see SummarizerEntry.
	Summarizer bool `yaml:"summarizer"`
}

// IsStrongProfile reports whether a profile value selects the strong
// behavior. Empty and unknown values are strong: strong is the default, and
// only an explicit "weak" (case-insensitive) turns the compensation
// machinery on. Validation rejects unknown values at load, so unknown here
// means a Config built without Load (tests, benchmarks constructing one by
// hand), which should behave exactly like the default.
func IsStrongProfile(p string) bool { return !strings.EqualFold(p, "weak") }

// ProfileFor returns the effective profile for an entry: its own `profile:`
// when set, else the global agent.profile default. The result is normalized
// to lowercase ("strong", "weak", or "" for the default-strong), so callers
// can compare or display it directly.
func (c *Config) ProfileFor(entry ModelEntry) string {
	if entry.Profile != "" {
		return strings.ToLower(strings.TrimSpace(entry.Profile))
	}
	if c.defaultProfile != "" {
		return c.defaultProfile
	}
	return strings.ToLower(strings.TrimSpace(c.Agent.Profile))
}

// IsStrongFor reports whether the effective profile for an entry skips the
// weak-model compensation machinery.
func (c *Config) IsStrongFor(entry ModelEntry) bool { return IsStrongProfile(c.ProfileFor(entry)) }

// ModelList returns the catalog: `models:` in file order, or one entry
// synthesized from the `llm:` block when `models:` is absent. It never
// returns an empty slice, so callers can index [DefaultModelIndex()].
func (c *Config) ModelList() []ModelEntry {
	if len(c.Models) > 0 {
		return c.Models
	}
	return []ModelEntry{{
		ID:         "default",
		Name:       c.LLM.Model,
		Model:      c.LLM.Model,
		BaseURL:    c.LLM.BaseURL,
		APIBackend: c.LLM.APIBackend,
		APIKey:     c.LLM.APIKey,
		APIKeyEnv:  c.LLM.APIKeyEnv,
		Context:    c.LLM.Context,
	}}
}

// DefaultModelIndex is which catalog row a fresh UI (or a headless run with
// no --model) starts on: `default_model` when it resolves, else the row
// whose wire id matches `llm.model`, else the first row.
func (c *Config) DefaultModelIndex() int {
	list := c.ModelList()
	if c.DefaultModel != "" {
		if _, idx, err := c.ResolveModel(c.DefaultModel); err == nil {
			return idx
		}
	}
	for i, e := range list {
		if e.Model == c.LLM.Model {
			return i
		}
	}
	return 0
}

// SummarizerEntry returns the catalog row flagged `summarizer: true` for
// session-title generation. It reports false when `models:` is absent (a
// legacy single-model config has nowhere to flag) or when no entry sets
// the flag, in which case the caller falls back to the prompt's first
// words. Validation guarantees at most one entry sets it, so the first
// match is the only match.
func (c *Config) SummarizerEntry() (ModelEntry, bool) {
	for _, e := range c.Models {
		if e.Summarizer {
			return e, true
		}
	}
	return ModelEntry{}, false
}

// ResolveModel finds a catalog row by 1-based index ("1"), id, display name,
// or wire model id, in that order. Indexing is stable config-file order;
// rows are never sorted (prompt-cache determinism).
//
// The :mN spellings the UI uses are accepted too: a leading ':' is dropped
// and 'm' followed only by digits means that index, so "-m m3" on the
// command line finds what ":m3" finds in the UI. (A catalog id that is
// literally "m<N>" is shadowed by the index; ids look like "spark-go", so
// this costs nothing in practice.)
func (c *Config) ResolveModel(ref string) (ModelEntry, int, error) {
	list := c.ModelList()
	orig := strings.TrimSpace(ref)
	ref = strings.TrimPrefix(orig, ":")
	if len(ref) >= 2 && (ref[0] == 'm' || ref[0] == 'M') {
		if _, err := strconv.Atoi(ref[1:]); err == nil {
			ref = ref[1:]
		}
	}
	if n, err := strconv.Atoi(ref); err == nil {
		if n >= 1 && n <= len(list) {
			return list[n-1], n - 1, nil
		}
		return ModelEntry{}, 0, fmt.Errorf("unknown model %q; use one of: %s", orig, modelListHint(list))
	}
	for i, e := range list {
		if strings.EqualFold(e.ID, ref) {
			return e, i, nil
		}
	}
	for i, e := range list {
		if strings.EqualFold(e.Name, ref) {
			return e, i, nil
		}
	}
	for i, e := range list {
		if e.Model == ref {
			return e, i, nil
		}
	}
	for i, e := range list {
		if strings.EqualFold(e.Model, ref) {
			return e, i, nil
		}
	}
	return ModelEntry{}, 0, fmt.Errorf("unknown model %q; use one of: %s", orig, modelListHint(list))
}

// SplitModelEffort splits a -m/--model value into its model and effort
// halves: "m3:xhigh" gives ("m3", "xhigh"), "qwen-local:low" gives
// ("qwen-local", "low"), ":xhigh" gives ("", "xhigh", default model with
// that effort), and anything without a known effort suffix stays whole —
// only the last colon counts, and only when the suffix names a canonical
// effort, so a wire id with colons ("openai:gpt-4") is never cut.
func SplitModelEffort(ref string) (modelPart, effortPart string) {
	ref = strings.TrimSpace(ref)
	if i := strings.LastIndex(ref, ":"); i >= 0 {
		// Lowercased: effort values are canonical lowercase everywhere
		// (Load normalizes the catalog the same way).
		if tail := strings.ToLower(strings.TrimSpace(ref[i+1:])); IsCanonicalEffort(tail) {
			return strings.TrimSpace(ref[:i]), tail
		}
	}
	return ref, ""
}

// ResolveSelection resolves a startup/flag selection to a catalog index plus
// a resolved effort. modelRef may carry an inline ":effort" ("m3:xhigh",
// "qwen-local:low"); an explicit effort (effortSet) wins over the inline
// one. Empty modelRef means the default row; "" effort means that row's
// default ("" for a model with no menu). Errors name what is offered, for
// the UI banner path and the headless --model path alike.
func (c *Config) ResolveSelection(modelRef, effortRef string, effortSet bool) (int, string, error) {
	modelPart, inlineEff := SplitModelEffort(modelRef)
	if !effortSet && inlineEff != "" {
		effortRef, effortSet = inlineEff, true
	}
	list := c.ModelList()
	idx := c.DefaultModelIndex()
	if idx < 0 || idx >= len(list) {
		idx = 0
	}
	if strings.TrimSpace(modelPart) != "" {
		_, i, err := c.ResolveModel(modelPart)
		if err != nil {
			return 0, "", err
		}
		idx = i
	}
	eff := c.DefaultEffort(list[idx])
	if effortSet {
		v, err := c.ResolveEffort(list[idx], strings.TrimSpace(effortRef))
		if err != nil {
			return 0, "", err
		}
		eff = v
	}
	return idx, eff, nil
}

func modelListHint(list []ModelEntry) string {
	parts := make([]string, len(list))
	for i, e := range list {
		parts[i] = fmt.Sprintf(":m%d %s (%s)", i+1, e.ID, e.Name)
	}
	return strings.Join(parts, ", ")
}

// EffortOptions returns the :eN menu for an entry in config order: its own
// list, else the legacy four for a `responses` model without one, else empty
// (a non-reasoning model has no menu and switching to it clears the effort).
func (c *Config) EffortOptions(entry ModelEntry) []EffortOption {
	if len(entry.Efforts) > 0 {
		return entry.Efforts
	}
	if entry.APIBackend == llm.BackendResponses {
		opts := make([]EffortOption, len(LegacyEfforts))
		for i, v := range LegacyEfforts {
			opts[i] = EffortOption{Value: v}
		}
		return opts
	}
	return nil
}

// DefaultEffort is the effort a fresh selection of this entry runs with: ""
// when the entry has no menu (a non-reasoning model never takes an effort,
// so switching to one clears it), else its `default: true` row, else
// `llm.reasoning_effort`, else `default_effort`, else "" (no `reasoning:`
// object on the wire).
func (c *Config) DefaultEffort(entry ModelEntry) string {
	if len(c.EffortOptions(entry)) == 0 {
		return ""
	}
	for _, o := range entry.Efforts {
		if o.Default {
			return o.Value
		}
	}
	if c.LLM.ReasoningEffort != "" {
		return c.LLM.ReasoningEffort
	}
	return c.DefaultEffortValue
}

// ResolveEffort checks an --effort token (or :e value) against one entry's
// menu. "" always resolves to "" (clear: omit `reasoning:`), which is what a
// non-reasoning model needs. A value the model does not offer is rejected
// with the offered list rather than sent to the API.
func (c *Config) ResolveEffort(entry ModelEntry, token string) (string, error) {
	if token == "" {
		return "", nil
	}
	menu := c.EffortOptions(entry)
	if len(menu) == 0 {
		return "", fmt.Errorf("model %q does not support reasoning effort", entry.Name)
	}
	for _, o := range menu {
		if strings.EqualFold(o.Value, token) {
			return o.Value, nil
		}
	}
	offered := make([]string, len(menu))
	for i, o := range menu {
		offered[i] = o.Value
	}
	return "", fmt.Errorf("unknown effort level %q; use one of: %s", token, strings.Join(offered, ", "))
}

// EntryAPIKey resolves which key an entry authenticates with: its env var
// when set, else its literal, else none (a local endpoint needs no key, and
// the client omits the Authorization header then).
func EntryAPIKey(entry ModelEntry) string {
	if entry.APIKeyEnv != "" {
		return os.Getenv(entry.APIKeyEnv)
	}
	return entry.APIKey
}

// ApplyModel makes an entry plus a resolved effort the active endpoint: the
// `llm:` fields a model switch can change are overwritten, everything else
// (headers, timeouts, merge_thinking, ...) stays from the `llm:` block.
// Downstream code (client construction, session records, context accounting)
// reads cfg.LLM, so it follows the selection with no other change.
//
// It also makes the entry's effective profile (see ProfileFor) the active
// one, so a :mN switch to a weak model turns the compensation machinery
// back on for the next turn, and switching back to a strong one turns it
// off again. The agent is one process per turn, so this runs once per turn
// in resolveModelSelection; ApplyProfile restores from the pre-profile
// snapshot, so repeated calls are idempotent.
func (c *Config) ApplyModel(entry ModelEntry, effort string) {

	c.LLM.Model = entry.Model

	if entry.BaseURL != "" {
		c.LLM.BaseURL = entry.BaseURL
	}

	if entry.APIBackend != "" {
		c.LLM.APIBackend = entry.APIBackend
	}

	// Always assigned, never left alone: ApplyModel runs twice in a process
	// (the default entry, then the selected one), so an entry that names no
	// effort_field has to fall back to the file's rather than keep whatever
	// the previous entry set.
	c.LLM.EffortField = entry.EffortField
	if c.LLM.EffortField == "" {
		c.LLM.EffortField = c.fileEffortField
	}

	if entry.Context != 0 {
		c.LLM.Context = entry.Context
	}

	if entry.APIKeyEnv != "" || entry.APIKey != "" {
		c.LLM.APIKeyEnv = entry.APIKeyEnv
		c.LLM.APIKey = EntryAPIKey(entry)
	} else {
		c.LLM.APIKeyEnv = ""
		c.LLM.APIKey = ""
	}

	c.LLM.ReasoningEffort = effort

	c.ApplyProfile(c.ProfileFor(entry))
}

// EntryDisplay is ModelDisplay for a catalog row: "model:effort" when an
// effort is set, plain "model" otherwise.
func EntryDisplay(entry ModelEntry, effort string) string {
	if effort != "" {
		return entry.Model + ":" + effort
	}
	return entry.Model
}

// WireID strips a ":effort" suffix back off a display string, so a recorded
// "model:effort" can be compared against a wire id. A suffix that is not a
// known effort value is left alone (the wire id itself may contain colons).
func WireID(display string) string {
	if i := strings.LastIndex(display, ":"); i >= 0 && IsCanonicalEffort(display[i+1:]) {
		return display[:i]
	}
	return display
}

// ParseIndexedCommand parses the word after the colon of :mN / :eN: 'm' or
// 'e' followed only by digits. Anything else (including bare "m"/"e", which
// take a value argument) is not an indexed command.
func ParseIndexedCommand(cmd string) (kind byte, n int, ok bool) {
	if len(cmd) < 2 || (cmd[0] != 'm' && cmd[0] != 'e') {
		return 0, 0, false
	}
	for _, r := range cmd[1:] {
		if r < '0' || r > '9' {
			return 0, 0, false
		}
	}
	n, err := strconv.Atoi(cmd[1:])
	if err != nil {
		return 0, 0, false
	}
	return cmd[0], n, true
}

// validateModels checks the `models:` catalog at load: unknown backends or
// effort values, duplicate or empty ids, an unresolvable default_model, more
// than one summarizer entry, and more than one default effort per entry. Effort values are lowercased in
// place so later comparisons stay simple. A nil catalog (legacy single-model
// config) is valid.
func (c *Config) validateModels(configPath string) error {

	if len(c.Models) == 0 {
		if c.DefaultModel != "" {
			return fmt.Errorf("%s: default_model %q with no models: list", configPath, c.DefaultModel)
		}

		if c.DefaultEffortValue != "" && !IsCanonicalEffort(c.DefaultEffortValue) {
			return fmt.Errorf("%s: unknown default_effort %q; use one of: %s",
				configPath, c.DefaultEffortValue, strings.Join(CanonicalEfforts, ", "))
		}

		return nil
	}

	seen := map[string]int{}

	for i := range c.Models {

		e := &c.Models[i]

		where := fmt.Sprintf("%s: models entry %d", configPath, i+1)

		if e.ID == "" {
			return fmt.Errorf("%s: missing id", where)
		}

		if key := strings.ToLower(e.ID); seen[key] > 0 {
			return fmt.Errorf("%s: duplicate id %q", configPath, e.ID)
		} else {
			seen[key] = i + 1
		}

		if e.Model == "" {
			return fmt.Errorf("%s (%s): missing model wire id", where, e.ID)
		}

		if !ValidBackend(e.APIBackend) {
			return fmt.Errorf("%s (%s): unknown api_backend %q; use %q or %q",
				where, e.ID, e.APIBackend, llm.BackendChatCompletions, llm.BackendResponses)
		}

		if !llm.ValidEffortField(e.EffortField) {
			return fmt.Errorf("%s (%s): unknown effort_field %q; use %q or %q",
				where, e.ID, e.EffortField,
				llm.EffortFieldChatTemplateKwargs, llm.EffortFieldReasoningEffort)
		}

		if e.Profile != "" {
			v := strings.ToLower(strings.TrimSpace(e.Profile))
			if v != "strong" && v != "weak" {
				return fmt.Errorf("%s (%s): unknown profile %q; use \"strong\" or \"weak\"",
					where, e.ID, e.Profile)
			}

			e.Profile = v
		}

		defaults := 0

		for j := range e.Efforts {
			v := strings.ToLower(strings.TrimSpace(e.Efforts[j].Value))
			if !IsCanonicalEffort(v) {
				return fmt.Errorf("%s (%s): unknown effort value %q; use one of: %s",
					where, e.ID, e.Efforts[j].Value, strings.Join(CanonicalEfforts, ", "))
			}
			e.Efforts[j].Value = v
			if e.Efforts[j].Default {
				defaults++
			}
		}
		if defaults > 1 {
			return fmt.Errorf("%s (%s): more than one default effort", where, e.ID)
		}
	}
	summarizers := 0
	for _, e := range c.Models {
		if e.Summarizer {
			summarizers++
		}
	}
	if summarizers > 1 {
		return fmt.Errorf("%s: more than one models entry sets summarizer: true (only one may)", configPath)
	}
	if c.DefaultEffortValue != "" && !IsCanonicalEffort(c.DefaultEffortValue) {
		return fmt.Errorf("%s: unknown default_effort %q; use one of: %s",
			configPath, c.DefaultEffortValue, strings.Join(CanonicalEfforts, ", "))
	}
	if c.DefaultModel != "" {
		if _, _, err := c.ResolveModel(c.DefaultModel); err != nil {
			return fmt.Errorf("%s: bad default_model: %v", configPath, err)
		}
	}
	return nil
}

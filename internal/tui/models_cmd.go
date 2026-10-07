package tui

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/modelsetup"
)

// StateName is the file in ~/.kvit-coder where kvit-coder-ui remembers the
// model and effort last chosen, so the next start opens on them.
const StateName = "kcu-state.json"

// uiState is kcu-state.json.
type uiState struct {
	// Model is the id of the row last chosen.
	Model string `json:"model"`
	// Effort is the effort chosen for it; empty means the row's default.
	Effort string `json:"effort,omitempty"`
}

func readUIState(path string) (uiState, bool) {
	var st uiState
	data, err := os.ReadFile(path)
	if err != nil || json.Unmarshal(data, &st) != nil || st.Model == "" {
		return uiState{}, false
	}
	return st, true
}

// rememberChoice writes the current model and effort to kcu-state.json.
// Failing to is not worth interrupting anyone for: the next start opens on
// default_model instead.
func (u *UI) rememberChoice() {
	if u.statePath == "" || len(u.models) == 0 {
		return
	}
	data, err := json.MarshalIndent(uiState{Model: u.currentEntry().ID, Effort: u.currentEffort}, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(u.statePath), 0o755); err != nil {
		return
	}
	tmp := u.statePath + ".tmp"
	if os.WriteFile(tmp, append(data, '\n'), 0o644) == nil {
		_ = os.Rename(tmp, u.statePath)
	}
}

// restoreChoice starts on the row kcu-state.json names, when it still
// exists and its key can be found. Otherwise the start is default_model, or
// the first row, as before.
func (u *UI) restoreChoice() {
	st, ok := readUIState(u.statePath)
	if !ok {
		return
	}
	for i, e := range u.models {
		if !strings.EqualFold(e.ID, st.Model) || !u.hasKey(e) {
			continue
		}
		u.currentModel = i
		u.currentEffort = ""
		if st.Effort != "" {
			if v, err := u.cfg.ResolveEffort(e, st.Effort); err == nil {
				u.currentEffort = v
			}
		}
		return
	}
}

// selectModel makes row i current at its default effort and remembers it.
func (u *UI) selectModel(i int) {
	if i < 0 || i >= len(u.models) {
		return
	}
	u.currentModel = i
	u.currentEffort = ""
	u.rememberChoice()
}

// reloadConfig loads the configuration again after :setup or :models changed
// models.yaml, keeping the current row when it is still there. The agent
// reads the files itself on its next turn; this keeps the front end's list,
// banner and :mN numbers the same as what the agent will see.
func (u *UI) reloadConfig() error {
	cfg, err := config.Load(u.configPath)
	if err != nil {
		return err
	}
	current := ""
	if len(u.models) > 0 {
		current = u.currentEntry().ID
	}
	u.cfg = cfg
	u.models = cfg.ModelList()
	u.currentModel = cfg.DefaultModelIndex()
	if u.currentModel < 0 || u.currentModel >= len(u.models) {
		u.currentModel = 0
	}
	for i, e := range u.models {
		if strings.EqualFold(e.ID, current) {
			u.currentModel = i
			if u.currentEffort != "" {
				if _, err := cfg.ResolveEffort(e, u.currentEffort); err != nil {
					u.currentEffort = ""
				}
			}
			return nil
		}
	}
	u.currentEffort = ""
	return nil
}

// hasModel reports whether the configuration names any model at all. Without
// one the only row is an empty one built from an llm: block with no model.
func (u *UI) hasModel() bool {
	return u.cfg != nil && (len(u.cfg.Models) > 0 || u.cfg.LLM.Model != "")
}

// hasKey reports whether a row can find its key: it names no variable, or the
// variable is set, or a key is saved under its name, or the row has one.
func (u *UI) hasKey(e config.ModelEntry) bool {
	if e.APIKeyEnv == "" {
		return true
	}
	// The row built from llm: carries the key Load already resolved, in
	// APIKey, so this covers it too.
	return config.EntryAPIKey(e) != ""
}

// isLoopback reports whether an address is on this machine, where a server
// rarely wants a key even when a row names a variable for one.
func isLoopback(base string) bool {
	parsed, err := url.Parse(base)
	if err != nil {
		return false
	}
	host := parsed.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// startupCheck runs before the first prompt. It opens :setup when asked to
// or when no model is configured, and offers to save a key when the starting
// model names a variable that is not set and has no saved key. Nothing here
// touches the network.
func (u *UI) startupCheck(openSetup bool) {
	if openSetup {
		u.runSetup()
		return
	}
	if !u.hasModel() {
		u.ask.say("No model is set up yet.")
		res, ok := u.ask.pick(pickSpec{
			Title: "Set up a model now?",
			Items: []pickItem{
				{Label: "Set up a model provider", Detail: "choose a provider, give its key, tick the models you want"},
				{Label: "Continue without a model", Detail: ":setup opens this later"},
			},
		})
		if ok && res.Index == 0 {
			u.runSetup()
		} else {
			u.ask.say("")
		}
		return
	}
	e := u.currentEntry()
	if u.hasKey(e) || isLoopback(u.activeBaseURL()) {
		return
	}
	u.ask.say("%s at %s needs the key in %s, which is not set, and none is saved under that name.",
		e.Name, u.activeBaseURL(), e.APIKeyEnv)
	res, ok := u.ask.pick(pickSpec{
		Title: "What now?",
		Items: []pickItem{
			{Label: "Enter the key for " + e.APIKeyEnv, Detail: "saved to ~/.kvit-coder/credentials.json"},
			{Label: "Set up a model provider", Detail: "add other models with :setup"},
			{Label: "Continue anyway"},
		},
	})
	if !ok {
		u.ask.say("")
		return
	}
	switch res.Index {
	case 0:
		u.enterKey(e.APIKeyEnv)
	case 1:
		u.runSetup()
	default:
		u.ask.say("")
	}
}

// enterKey asks for a key and saves it under env.
func (u *UI) enterKey(env string) bool {
	key, ok := u.ask.text(textSpec{
		Title:  "API key for " + env,
		Hint:   "saved to ~/.kvit-coder/credentials.json · Esc to go back",
		Secret: true,
		Validate: func(v string) error {
			if v == "" {
				return fmt.Errorf("paste the key, or press Esc to go back")
			}
			return nil
		},
	})
	if !ok {
		u.ask.say("")
		return false
	}
	if err := config.SaveCredential(env, key); err != nil {
		u.ask.say("\033[31mThe key could not be saved: %v\033[0m", err)
		u.ask.say("")
		return false
	}
	path, _ := config.CredentialsPath()
	u.ask.say("\033[38;5;78mSaved the key for %s to %s.\033[0m", env, path)
	if os.Getenv(env) != "" {
		u.ask.say("%s is set in this environment, and its value is used while it is.", env)
	}
	u.ask.say("")
	return true
}

// showModelPicker is :models: every row in a list, Enter to switch to one and
// Delete to remove one that :setup added.
func (u *UI) showModelPicker() {
	for {
		items := make([]pickItem, len(u.models))
		for i, e := range u.models {
			items[i] = pickItem{Label: fmt.Sprintf(":m%d %s", i+1, e.Name), Detail: u.rowDetail(i)}
		}
		res, ok := u.ask.pick(pickSpec{
			Title:  "Models",
			Hint:   "type to filter · Enter to switch · Delete to remove one :setup added · Esc to close",
			Items:  items,
			Start:  u.currentModel,
			Delete: true,
		})
		if !ok {
			return
		}
		if !res.Deleted {
			u.switchModel(res.Index + 1)
			return
		}
		u.removeModel(res.Index)
	}
}

// rowDetail is one row's line in :models.
func (u *UI) rowDetail(i int) string {
	e := u.models[i]
	parts := []string{e.Model, modelsetup.ProtocolLabel(e.APIBackend)}
	if e.Context > 0 {
		parts = append(parts, modelsetup.Tokens(e.Context)+" context")
	}
	if !u.hasKey(e) {
		parts = append(parts, "no key ("+e.APIKeyEnv+")")
	}
	source := "config.yaml"
	if u.cfg.FromSavedModels(i) {
		source = "models.yaml"
	}
	parts = append(parts, source)
	if i == u.currentModel {
		parts = append(parts, "current")
	}
	return strings.Join(parts, " · ")
}

func (u *UI) removeModel(i int) {
	e := u.models[i]
	if !u.cfg.FromSavedModels(i) {
		u.ask.say("%s is written in %s; remove it there.", e.Name, u.configPath)
		return
	}
	res, ok := u.ask.pick(pickSpec{
		Title: "Remove " + e.Name + "?",
		Items: []pickItem{{Label: "Keep it"}, {Label: "Remove it from models.yaml"}},
	})
	if !ok || res.Index == 0 {
		return
	}
	if _, err := config.RemoveSavedModel(e.ID); err != nil {
		u.ask.say("\033[31m%s could not be removed: %v\033[0m", e.Name, err)
		return
	}
	if err := u.reloadConfig(); err != nil {
		u.ask.say("\033[31mRemoved, but the configuration did not load again: %v\033[0m", err)
		return
	}
	u.rememberChoice()
	u.ask.say("Removed %s. Now on :m%d %s.", e.Name, u.currentModel+1, u.currentEntry().Name)
}

// keyUse is one variable some row reads its key from.
type keyUse struct {
	env    string
	models []string
}

// keyUses lists the variables the rows read keys from, in row order, then
// the saved keys no row reads.
func (u *UI) keyUses() []keyUse {
	var uses []keyUse
	index := map[string]int{}
	add := func(env, model string) {
		if env == "" {
			return
		}
		i, ok := index[env]
		if !ok {
			i = len(uses)
			index[env] = i
			uses = append(uses, keyUse{env: env})
		}
		if model != "" {
			uses[i].models = append(uses[i].models, model)
		}
	}
	for _, e := range u.models {
		add(e.APIKeyEnv, e.Name)
	}
	if path, err := config.CredentialsPath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			var saved map[string]string
			if json.Unmarshal(data, &saved) == nil {
				for env := range saved {
					add(env, "")
				}
			}
		}
	}
	return uses
}

// showKeys is :keys: each variable the models read a key from, where its
// value comes from, and a way to save or forget one.
func (u *UI) showKeys() {
	for {
		uses := u.keyUses()
		if len(uses) == 0 {
			u.ask.say("No model reads a key from a variable, and no key is saved.")
			u.ask.say("")
			return
		}
		items := make([]pickItem, len(uses))
		for i, use := range uses {
			items[i] = pickItem{Label: use.env, Detail: keyDetail(use)}
		}
		res, ok := u.ask.pick(pickSpec{Title: "API keys", Hint: "Enter to save or forget a key · Esc to close", Items: items})
		if !ok {
			u.ask.say("")
			return
		}
		u.keyActions(uses[res.Index].env)
	}
}

func keyDetail(use keyUse) string {
	source := "missing"
	switch {
	case os.Getenv(use.env) != "":
		source = "from the environment"
	case config.SavedKey(use.env) != "":
		source = "saved " + maskKey(config.SavedKey(use.env))
	}
	switch len(use.models) {
	case 0:
		return source + " · no model uses it"
	case 1:
		return source + " · used by " + use.models[0]
	}
	return fmt.Sprintf("%s · used by %s and %d more", source, use.models[0], len(use.models)-1)
}

// maskKey shows enough of a key to tell two apart and not enough to use it.
func maskKey(k string) string {
	if len(k) <= 12 {
		return "…"
	}
	return k[:4] + "…" + k[len(k)-4:]
}

func (u *UI) keyActions(env string) {
	items := []pickItem{{Label: "Enter a new key", Detail: "saved to ~/.kvit-coder/credentials.json"}}
	saved := config.SavedKey(env) != ""
	if saved {
		items = append(items, pickItem{Label: "Forget the saved key"})
	}
	items = append(items, pickItem{Label: "Back"})
	res, ok := u.ask.pick(pickSpec{Title: env, Items: items})
	if !ok || res.Index == len(items)-1 {
		return
	}
	if res.Index == 0 {
		u.enterKey(env)
		return
	}
	if _, err := config.DeleteCredential(env); err != nil {
		u.ask.say("\033[31mThe key could not be removed: %v\033[0m", err)
		return
	}
	u.ask.say("Forgot the key saved for %s.", env)
}

// afterFailedTurn says what to try when a turn ended in an error. kcu only
// sees the agent's exit status, so it names a missing key when that is the
// cause and otherwise points at the two commands once per run.
func (u *UI) afterFailedTurn() {
	e := u.currentEntry()
	if !u.hasKey(e) && !isLoopback(u.activeBaseURL()) {
		fmt.Printf("\033[38;5;136m%s needs the key in %s, which is not set; :keys saves one.\033[0m\n\n", e.Name, e.APIKeyEnv)
		return
	}
	if u.failureHinted {
		return
	}
	u.failureHinted = true
	fmt.Println("\033[38;5;136mIf the model refused its key or could not be reached, :keys saves another key and :setup adds a model.\033[0m")
	fmt.Println()
}

// modelRef is how a turn names its model to the agent. With a models: list
// it is the row's id: ids are unique, where two providers often serve the
// same model id (kimi-k3 is on both OpenCode gateways) and the agent would
// take the first row with it. A config with only an llm: block has no ids
// the agent knows, and gets the model id as before. An id the agent would
// read as a row number ("3", "m3") is passed as the model id too.
func (u *UI) modelRef(e config.ModelEntry) string {
	if u.cfg == nil || len(u.cfg.Models) == 0 || e.ID == "" {
		return e.Model
	}
	if _, _, ok := config.ParseIndexedCommand(strings.ToLower(e.ID)); ok {
		return e.Model
	}
	if _, err := strconv.Atoi(e.ID); err == nil {
		return e.Model
	}
	return e.ID
}

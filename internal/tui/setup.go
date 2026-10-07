package tui

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/modelsetup"
	"gopkg.in/yaml.v3"
)

// :setup adds models without anyone editing a file: pick a provider, give
// its key, tick the models it serves, check the rows, save. The rows go to
// ~/.kvit-coder/models.yaml and a typed key to ~/.kvit-coder/credentials.json,
// which config.Load reads for kvit-coder-ui and for every agent turn alike.
// config.yaml is never written. spec/configing.md is the design.

// setupRun is one :setup, from the provider list to a save or an Esc.
type setupRun struct {
	u   *UI
	ask asker
	// catalogDir is where the models.dev catalog is cached.
	catalogDir string
	// listModels and loadCatalog are the network calls, replaced in tests.
	listModels  func(ctx context.Context, baseURL, key string, headers map[string]string) ([]modelsetup.ListedModel, error)
	loadCatalog func(ctx context.Context) (*modelsetup.Catalog, string, error)
	probe       func(ctx context.Context, p modelsetup.Provider, e config.ModelEntry, key string, headers map[string]string, tryOthers bool) modelsetup.ProbeResult
	openBrowser func(url string) bool
}

func (u *UI) newSetupRun() *setupRun {
	dir := ""
	if d, err := config.UserDir(); err == nil {
		dir = filepath.Join(d, "cache")
	}
	s := &setupRun{u: u, ask: u.ask, catalogDir: dir}
	s.listModels = func(ctx context.Context, baseURL, key string, headers map[string]string) ([]modelsetup.ListedModel, error) {
		return modelsetup.ListModels(ctx, nil, baseURL, key, headers)
	}
	s.loadCatalog = func(ctx context.Context) (*modelsetup.Catalog, string, error) {
		if s.catalogDir == "" {
			return nil, "", errors.New("no home directory to keep the catalog in")
		}
		return modelsetup.LoadCatalog(ctx, nil, s.catalogDir, time.Now())
	}
	s.probe = modelsetup.Probe
	s.openBrowser = func(url string) bool {
		return modelsetup.CanOpenBrowser() && modelsetup.OpenBrowser(url) == nil
	}
	return s
}

// runSetup is :setup. It reports whether anything was saved.
func (u *UI) runSetup() bool {
	return u.newSetupRun().run()
}

func (s *setupRun) run() bool {
	for {
		p, ok := s.chooseProvider()
		if !ok {
			s.ask.say("Setup closed; nothing was saved.")
			s.ask.say("")
			return false
		}
		if s.setUpProvider(p) {
			return true
		}
	}
}

// keySource says where a variable's key would come from.
func keySource(env string) string {
	switch {
	case env == "":
		return "no key needed"
	case os.Getenv(env) != "":
		return "key in the environment (" + env + ")"
	case config.SavedKey(env) != "":
		return "key saved (" + env + ")"
	}
	return "no key yet"
}

func (s *setupRun) chooseProvider() (modelsetup.Provider, bool) {
	items := make([]pickItem, len(modelsetup.Providers))
	for i, p := range modelsetup.Providers {
		detail := ""
		switch p.Kind {
		case modelsetup.Hosted:
			detail = keySource(p.KeyEnv)
		case modelsetup.Local:
			detail = "llama.cpp, vLLM, Ollama or LM Studio; no key needed"
		case modelsetup.Other:
			detail = "any address that speaks chat completions, responses or messages"
		}
		items[i] = pickItem{Label: p.Name, Detail: detail}
	}
	res, ok := s.ask.pick(pickSpec{
		Title: "Set up a model provider",
		Hint:  "↑↓ to move · Enter to choose · Esc to leave without saving",
		Items: items,
	})
	if !ok {
		return modelsetup.Provider{}, false
	}
	p := modelsetup.Providers[res.Index]
	s.ask.say("Provider: %s", p.Name)
	return p, true
}

// setUpProvider asks what the provider needs, then for its models. It
// reports whether models were saved; false goes back to the provider list.
func (s *setupRun) setUpProvider(p modelsetup.Provider) bool {
	var key string
	var typed bool
	switch p.Kind {
	case modelsetup.Hosted:
		var ok bool
		if key, typed, ok = s.keyFor(p); !ok {
			return false
		}
	case modelsetup.Local:
		addr, ok := s.ask.text(textSpec{
			Title:    "Address of the local server",
			Hint:     "the address that ends in /v1 · Enter to accept · Esc to go back",
			Initial:  p.BaseURL,
			Validate: validURL,
		})
		if !ok {
			return false
		}
		p = modelsetup.LocalProvider(addr)
		s.ask.say("Address: %s", p.BaseURL)
	case modelsetup.Other:
		var ok bool
		if p, key, typed, ok = s.describeOther(); !ok {
			return false
		}
	}
	return s.chooseModels(p, key, typed)
}

// keyFor finds the provider's key, or asks for it. typed reports that it was
// typed here and still has to be saved.
func (s *setupRun) keyFor(p modelsetup.Provider) (key string, typed, ok bool) {
	if v := os.Getenv(p.KeyEnv); v != "" {
		s.ask.say("Using the key in the environment variable %s.", p.KeyEnv)
		return v, false, true
	}
	if v := config.SavedKey(p.KeyEnv); v != "" {
		s.ask.say("Using the key saved for %s (:keys replaces it).", p.KeyEnv)
		return v, false, true
	}
	return s.askKey(p.Name, p.KeyEnv, p.KeyPage, false)
}

func (s *setupRun) askKey(name, env, page string, optional bool) (string, bool, bool) {
	if page != "" {
		if s.openBrowser(page) {
			s.ask.say("%s issues keys at %s, which is now open in your browser.", name, page)
		} else {
			s.ask.say("%s issues keys at %s.", name, page)
		}
	}
	hint := "it is saved to ~/.kvit-coder/credentials.json as " + env + " when you save the models · Esc to go back"
	validate := func(v string) error {
		if v == "" {
			return errors.New("paste the key, or press Esc to go back")
		}
		return nil
	}
	if optional {
		hint = "leave it empty when the endpoint needs no key · " + hint
		validate = nil
	}
	key, ok := s.ask.text(textSpec{Title: name + " API key", Hint: hint, Secret: true, Validate: validate})
	if !ok {
		return "", false, false
	}
	if key != "" {
		s.ask.say("Key entered for %s.", env)
	}
	return key, key != "", true
}

// describeOther asks for everything about an endpoint kvit-coder has no entry
// for.
func (s *setupRun) describeOther() (modelsetup.Provider, string, bool, bool) {
	name, ok := s.ask.text(textSpec{
		Title: "A name for this endpoint",
		Hint:  "shown after its models' names and used in their ids · Esc to go back",
		Validate: func(v string) error {
			if modelsetup.Slug(v) == "" {
				return errors.New("use at least one letter or digit")
			}
			return nil
		},
	})
	if !ok {
		return modelsetup.Provider{}, "", false, false
	}
	addr, ok := s.ask.text(textSpec{
		Title:    "Address of " + name,
		Hint:     "usually ending in /v1, such as https://llm.example.com/v1 · Esc to go back",
		Validate: validURL,
	})
	if !ok {
		return modelsetup.Provider{}, "", false, false
	}
	env, ok := s.ask.text(textSpec{
		Title:   "Environment variable for its API key",
		Hint:    "the key typed next is saved under this name; empty when the endpoint needs no key",
		Initial: modelsetup.KeyEnvFor(name),
		Validate: func(v string) error {
			for _, r := range v {
				if !(r == '_' || r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
					return errors.New("letters, digits and _ only")
				}
			}
			return nil
		},
	})
	if !ok {
		return modelsetup.Provider{}, "", false, false
	}
	p := modelsetup.OtherProvider(name, addr, env)
	s.ask.say("Endpoint: %s at %s", name, p.BaseURL)
	if env == "" {
		return p, "", false, true
	}
	if v := os.Getenv(env); v != "" {
		s.ask.say("Using the key in the environment variable %s.", env)
		return p, v, false, true
	}
	if v := config.SavedKey(env); v != "" {
		s.ask.say("Using the key saved for %s.", env)
		return p, v, false, true
	}
	key, typed, ok := s.askKey(name, env, "", true)
	return p, key, typed, ok
}

func validURL(v string) error {
	u, err := url.Parse(v)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return errors.New("an http:// or https:// address, such as http://localhost:8080/v1")
	}
	return nil
}

// headersFor is what a request to this provider's rows carries: llm.headers
// with the provider's own laid over them, ${VAR} expanded.
func (s *setupRun) headersFor(p modelsetup.Provider) map[string]string {
	return s.u.cfg.HeadersFor(config.ModelEntry{Headers: p.Headers})
}

// chooseModels lists what the provider serves and lets the person tick
// models. It reports whether any were saved.
func (s *setupRun) chooseModels(p modelsetup.Provider, key string, typed bool) bool {
	for {
		cands, ok := s.fetchCandidates(p, key)
		if !ok {
			return false
		}
		for {
			items := make([]pickItem, len(cands))
			for i, c := range cands {
				items[i] = pickItem{Label: c.ID, Detail: c.Detail(), Disabled: c.Unsupported != ""}
				if n := s.existingRow(p, c.ID); n > 0 {
					items[i].Detail = fmt.Sprintf("already set up as :m%d", n)
					items[i].Disabled = true
				}
			}
			res, ok := s.ask.pick(pickSpec{Title: p.Name + ": choose models", Multi: true, Items: items})
			if !ok {
				return false
			}
			var chosen []modelsetup.Candidate
			for _, i := range res.Checked {
				chosen = append(chosen, cands[i])
			}
			saved, again := s.confirm(p, chosen, key, typed)
			if saved {
				return true
			}
			if again != "" {
				key, typed = again, true
				break // list again with the new key
			}
		}
	}
}

// fetchCandidates asks the endpoint for its models and models.dev for what
// it knows about them. When the endpoint cannot list them, the person can
// try again or type a model id.
func (s *setupRun) fetchCandidates(p modelsetup.Provider, key string) ([]modelsetup.Candidate, bool) {
	headers := s.headersFor(p)
	for {
		var (
			listed  []modelsetup.ListedModel
			cat     *modelsetup.Catalog
			warn    string
			catErr  error
			wg      sync.WaitGroup
			message = "Asking " + p.Name + " which models it serves"
		)
		if p.CatalogID != "" {
			message += ", and fetching the models.dev catalog"
		}
		err := s.ask.wait(message+"…", func(ctx context.Context) error {
			if p.CatalogID != "" {
				wg.Add(1)
				go func() {
					defer wg.Done()
					cat, warn, catErr = s.loadCatalog(ctx)
				}()
			}
			var err error
			listed, err = s.listModels(ctx, p.BaseURL, key, headers)
			wg.Wait()
			if err == nil && p.Kind == modelsetup.Local {
				fillServerContext(ctx, p.BaseURL, listed)
			}
			return err
		})
		if warn != "" {
			s.ask.say("%s", warn)
		}
		if catErr != nil {
			s.ask.say("The models.dev catalog could not be loaded (%v), so protocols and context sizes are found by a test request instead.", catErr)
		}
		if err == nil && len(listed) > 0 {
			return modelsetup.Candidates(p, listed, cat), true
		}
		if err == nil {
			err = errors.New("the list was empty")
		}
		s.ask.say("%s did not list its models: %v", p.Name, err)
		res, ok := s.ask.pick(pickSpec{
			Title: "What now?",
			Items: []pickItem{
				{Label: "Try again"},
				{Label: "Type a model id", Detail: "for an endpoint that serves models without listing them"},
				{Label: "Back to the provider list"},
			},
		})
		if !ok || res.Index == 2 {
			return nil, false
		}
		if res.Index == 1 {
			id, ok := s.ask.text(textSpec{Title: "Model id", Hint: "as the endpoint expects it in a request · Esc to go back"})
			if !ok || id == "" {
				continue
			}
			return modelsetup.Candidates(p, []modelsetup.ListedModel{{ID: id}}, cat), true
		}
	}
}

// fillServerContext asks a llama.cpp server for its context size, for the
// models whose list entry did not give one.
var fillServerContext = func(ctx context.Context, baseURL string, listed []modelsetup.ListedModel) {
	n := 0
	for i := range listed {
		if listed[i].Context > 0 {
			continue
		}
		if n == 0 {
			if n = modelsetup.ServerContext(ctx, nil, baseURL); n == 0 {
				return
			}
		}
		listed[i].Context = n
	}
}

// existingRow is the :mN number of a row that already sends this model to
// this provider, or 0.
func (s *setupRun) existingRow(p modelsetup.Provider, model string) int {
	for i, e := range s.u.models {
		if e.Model != model {
			continue
		}
		base := strings.TrimRight(e.BaseURL, "/")
		if base == p.BaseURL || (p.MessagesURL != "" && base == p.MessagesURL) {
			return i + 1
		}
	}
	return 0
}

// confirm shows the rows that will be saved and saves them. saved reports
// that they were; newKey is a key typed again after the endpoint refused
// the old one, and sends the person back to the model list with it.
func (s *setupRun) confirm(p modelsetup.Provider, chosen []modelsetup.Candidate, key string, typed bool) (saved bool, newKey string) {
	taken := map[string]bool{}
	for _, e := range s.u.models {
		taken[strings.ToLower(e.ID)] = true
	}
	rows := make([]config.ModelEntry, len(chosen))
	for i, c := range chosen {
		rows[i] = modelsetup.Row(p, c, func(id string) bool { return taken[strings.ToLower(id)] })
		taken[strings.ToLower(rows[i].ID)] = true
	}
	// A model models.dev does not describe has a guessed protocol, and an
	// endpoint the person named has no record at all: testing is how both
	// are found out, so it is the first choice for them.
	needsTest := p.Kind == modelsetup.Other
	for _, c := range chosen {
		needsTest = needsTest || !c.Described()
	}
	for {
		s.showRows(rows)
		items := []pickItem{
			{Label: "Save", Detail: saveDetail(len(rows), typed, p.KeyEnv)},
			{Label: "Test each model, then save", Detail: "one short request per model; finds the protocol where it is not known"},
		}
		for _, r := range rows {
			items = append(items, pickItem{Label: "Change " + r.Name, Detail: rowSummary(r)})
		}
		items = append(items, pickItem{Label: "Back to the model list"})
		start := 0
		if needsTest {
			start = 1
		}
		res, ok := s.ask.pick(pickSpec{Title: fmt.Sprintf("Check the %s before saving", plural(len(rows), "model")), Items: items, Start: start})
		if !ok || res.Index == len(items)-1 {
			return false, ""
		}
		switch {
		case res.Index == 0:
			return s.save(p, rows, key, typed), ""
		case res.Index == 1:
			keep, again, retry := s.test(p, rows, chosen, key)
			if again != "" {
				return false, again
			}
			if retry {
				continue
			}
			if len(keep) == 0 {
				continue
			}
			return s.save(p, keep, key, typed), ""
		default:
			s.editRow(&rows[res.Index-2])
		}
	}
}

func saveDetail(n int, typed bool, env string) string {
	where := "write " + plural(n, "model") + " to ~/.kvit-coder/models.yaml"
	if typed && env != "" {
		where += " and the key to credentials.json"
	}
	return where
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return strconv.Itoa(n) + " " + word + "s"
}

// rowSummary is a row's effort levels, context size and profile in one line.
func rowSummary(e config.ModelEntry) string {
	parts := []string{modelsetup.ProtocolLabel(e.APIBackend)}
	if s := modelsetup.EffortSummary(e.Efforts); s != "" {
		parts = append(parts, "effort "+s+" (starts at "+defaultEffortOf(e)+")")
	} else {
		parts = append(parts, "no effort levels")
	}
	if e.Context > 0 {
		parts = append(parts, modelsetup.Tokens(e.Context)+" context")
	} else {
		parts = append(parts, "context size unknown")
	}
	if e.Profile == "weak" {
		parts = append(parts, "weak profile")
	}
	return strings.Join(parts, " · ")
}

func defaultEffortOf(e config.ModelEntry) string {
	for _, o := range e.Efforts {
		if o.Default {
			return o.Value
		}
	}
	if len(e.Efforts) > 0 {
		return e.Efforts[len(e.Efforts)-1].Value
	}
	return ""
}

// showRows prints the rows as they will be written to models.yaml.
func (s *setupRun) showRows(rows []config.ModelEntry) {
	var b strings.Builder
	enc := yaml.NewEncoder(&b)
	enc.SetIndent(2)
	if err := enc.Encode(rows); err != nil {
		return
	}
	enc.Close()
	s.ask.say("")
	s.ask.say("\033[38;5;136mTo be added to ~/.kvit-coder/models.yaml:\033[0m")
	for _, line := range strings.Split(strings.TrimRight(b.String(), "\n"), "\n") {
		s.ask.say("  %s", line)
	}
	s.ask.say("")
}

// editRow changes what models.dev could not settle or the person wants
// different: the effort levels, the context size, the name and the profile.
func (s *setupRun) editRow(e *config.ModelEntry) {
	for {
		profile := "strong"
		if e.Profile == "weak" {
			profile = "weak"
		}
		ctx := "unknown"
		if e.Context > 0 {
			ctx = strconv.Itoa(e.Context) + " tokens"
		}
		res, ok := s.ask.pick(pickSpec{
			Title: "Change " + e.Name,
			Items: []pickItem{
				{Label: "Effort levels", Detail: effortsText(e.Efforts)},
				{Label: "Context size", Detail: ctx},
				{Label: "Name", Detail: e.Name},
				{Label: "Profile", Detail: profile + "; weak turns on the checks for models that mis-format tool calls"},
				{Label: "Protocol", Detail: modelsetup.ProtocolLabel(e.APIBackend)},
				{Label: "Done"},
			},
		})
		if !ok || res.Index == 5 {
			return
		}
		switch res.Index {
		case 0:
			v, ok := s.ask.text(textSpec{
				Title:    "Effort levels for " + e.Name,
				Hint:     "from lowest to highest, * after the one to start at; empty for none · " + strings.Join(config.CanonicalEfforts, " "),
				Initial:  effortsText(e.Efforts),
				Validate: func(v string) error { _, err := parseEfforts(v); return err },
			})
			if ok {
				e.Efforts, _ = parseEfforts(v)
			}
		case 1:
			v, ok := s.ask.text(textSpec{
				Title:   "Context size of " + e.Name + " in tokens",
				Hint:    "0 when unknown; it is used for the context gauge and the warning near the limit",
				Initial: strconv.Itoa(e.Context),
				Validate: func(v string) error {
					if n, err := strconv.Atoi(v); err != nil || n < 0 {
						return errors.New("a whole number of tokens, such as 262144")
					}
					return nil
				},
			})
			if ok {
				e.Context, _ = strconv.Atoi(v)
			}
		case 2:
			v, ok := s.ask.text(textSpec{Title: "Name of the model", Initial: e.Name, Validate: func(v string) error {
				if v == "" {
					return errors.New("a name is needed")
				}
				return nil
			}})
			if ok {
				e.Name = v
			}
		case 3:
			if e.Profile == "weak" {
				e.Profile = ""
			} else {
				e.Profile = "weak"
			}
		case 4:
			backends := []string{llm.BackendChatCompletions, llm.BackendResponses, llm.BackendMessages}
			items := make([]pickItem, len(backends))
			start := 0
			for i, b := range backends {
				items[i] = pickItem{Label: modelsetup.ProtocolLabel(b)}
				if b == e.APIBackend {
					start = i
				}
			}
			r, ok := s.ask.pick(pickSpec{Title: "Protocol for " + e.Name, Items: items, Start: start})
			if ok {
				p := s.providerOf(*e)
				modelsetup.SetBackend(p, e, backends[r.Index])
			}
		}
	}
}

// providerOf finds the provider a row was built for from its address, so a
// protocol change can move it to the right one of the provider's addresses.
func (s *setupRun) providerOf(e config.ModelEntry) modelsetup.Provider {
	for _, p := range modelsetup.Providers {
		if p.BaseURL != "" && (e.BaseURL == p.BaseURL || e.BaseURL == p.MessagesURL) {
			return p
		}
	}
	base := strings.TrimRight(e.BaseURL, "/")
	if e.APIBackend == llm.BackendMessages {
		base += "/v1"
	}
	p := modelsetup.OtherProvider("", base, e.APIKeyEnv)
	if strings.HasPrefix(e.BaseURL, "https://opencode.ai/") {
		p.EffortField = llm.EffortFieldReasoningEffort
	}
	return p
}

// effortsText writes a menu the way the edit field takes it: "low high* max".
func effortsText(opts []config.EffortOption) string {
	if len(opts) == 0 {
		return "none"
	}
	words := make([]string, len(opts))
	for i, o := range opts {
		words[i] = o.Value
		if o.Default {
			words[i] += "*"
		}
	}
	return strings.Join(words, " ")
}

// parseEfforts reads "low high* max" back into a menu.
func parseEfforts(v string) ([]config.EffortOption, error) {
	v = strings.TrimSpace(v)
	if v == "" || strings.EqualFold(v, "none") {
		return nil, nil
	}
	var opts []config.EffortOption
	defaults := 0
	for _, w := range strings.Fields(strings.ToLower(v)) {
		def := strings.HasSuffix(w, "*")
		w = strings.TrimSuffix(w, "*")
		if !config.IsCanonicalEffort(w) {
			return nil, fmt.Errorf("%q is not an effort level; use %s", w, strings.Join(config.CanonicalEfforts, ", "))
		}
		if def {
			defaults++
		}
		opts = append(opts, config.EffortOption{Value: w, Default: def})
	}
	if defaults > 1 {
		return nil, errors.New("mark one level with *, not several")
	}
	if defaults == 0 {
		opts[len(opts)-1].Default = true
	}
	return opts, nil
}

// test sends each row one short request. It returns the rows to save, a key
// typed again after a refusal, or retry to go back to the check screen.
func (s *setupRun) test(p modelsetup.Provider, rows []config.ModelEntry, chosen []modelsetup.Candidate, key string) (keep []config.ModelEntry, newKey string, retry bool) {
	headers := s.headersFor(p)
	var failed, refused int
	answered := make([]bool, len(rows))
	for i := range rows {
		tryOthers := p.Kind == modelsetup.Other || !chosen[i].Described()
		var res modelsetup.ProbeResult
		err := s.ask.wait("Testing "+rows[i].Name+"…", func(ctx context.Context) error {
			res = s.probe(ctx, p, rows[i], key, headers, tryOthers)
			return nil
		})
		if err != nil {
			res = modelsetup.ProbeResult{Err: err}
		}
		switch {
		case res.OK() && res.Backend != rows[i].APIBackend:
			s.ask.say("✓ %s answered over %s, so the row now uses it instead of %s.", rows[i].Name,
				modelsetup.ProtocolLabel(res.Backend), modelsetup.ProtocolLabel(rows[i].APIBackend))
			modelsetup.SetBackend(p, &rows[i], res.Backend)
			answered[i] = true
		case res.OK():
			s.ask.say("✓ %s answered over %s.", rows[i].Name, modelsetup.ProtocolLabel(res.Backend))
			answered[i] = true
		default:
			failed++
			if res.KeyRefused {
				refused++
			}
			s.ask.say("✗ %s: %v", rows[i].Name, res.Err)
		}
	}
	if failed == 0 {
		return rows, "", false
	}
	var items []pickItem
	var actions []string
	if failed < len(rows) {
		items = append(items, pickItem{Label: "Save the " + plural(len(rows)-failed, "model") + " that answered"})
		actions = append(actions, "answered")
	}
	items = append(items, pickItem{Label: "Save all of them anyway"})
	actions = append(actions, "all")
	if refused > 0 && p.KeyEnv != "" {
		items = append(items, pickItem{Label: "Enter the key again", Detail: "the endpoint refused " + p.KeyEnv})
		actions = append(actions, "key")
	}
	items = append(items, pickItem{Label: "Back to the check screen"})
	actions = append(actions, "back")
	res, ok := s.ask.pick(pickSpec{Title: plural(failed, "model") + " did not answer", Items: items})
	if !ok {
		return nil, "", true
	}
	switch actions[res.Index] {
	case "answered":
		for i, r := range rows {
			if answered[i] {
				keep = append(keep, r)
			}
		}
		return keep, "", false
	case "all":
		return rows, "", false
	case "key":
		k, _, ok := s.askKey(p.Name, p.KeyEnv, p.KeyPage, false)
		if ok && k != "" {
			return nil, k, false
		}
	}
	return nil, "", true
}

// save writes the key and the rows, loads the configuration again, and
// switches to the first new row.
func (s *setupRun) save(p modelsetup.Provider, rows []config.ModelEntry, key string, typed bool) bool {
	if typed && key != "" && p.KeyEnv != "" {
		if err := config.SaveCredential(p.KeyEnv, key); err != nil {
			s.ask.say("\033[31mThe key could not be saved: %v\033[0m", err)
			return false
		}
	}
	if err := config.AppendSavedModels(rows...); err != nil {
		s.ask.say("\033[31mThe models could not be saved: %v\033[0m", err)
		return false
	}
	if err := s.u.reloadConfig(); err != nil {
		s.ask.say("\033[31mSaved, but the configuration did not load again: %v\033[0m", err)
		return true
	}
	path, _ := config.SavedModelsPath()
	line := fmt.Sprintf("Saved %s to %s", plural(len(rows), "model"), path)
	if typed && key != "" && p.KeyEnv != "" {
		cred, _ := config.CredentialsPath()
		line += fmt.Sprintf(", and the key to %s as %s", cred, p.KeyEnv)
	}
	s.ask.say("\033[38;5;78m%s.\033[0m", line)
	for i, e := range s.u.models {
		if strings.EqualFold(e.ID, rows[0].ID) {
			s.u.selectModel(i)
			s.ask.say("Now on :m%d %s (%s).", i+1, e.Name, s.u.activeDisplay())
			break
		}
	}
	s.ask.say("")
	return true
}

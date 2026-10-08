package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
	"github.com/kvit-s/kvit-coder/internal/modelsetup"
)

// answer is one scripted reply to a question from the setup screens.
type answer struct {
	// title must appear in the question's title.
	title string
	// label chooses the list row whose label contains it.
	label string
	// tick ticks the rows whose labels are these, in a list that takes
	// several.
	tick []string
	// deleteLabel ends the list with Delete on the row whose label contains it.
	deleteLabel string
	// text answers a text question.
	text string
	// cancel answers with Esc.
	cancel bool
}

// scriptAsker answers the setup screens' questions from a list, failing the
// test on a question it did not expect.
type scriptAsker struct {
	t       *testing.T
	answers []answer
	said    []string
}

func (s *scriptAsker) next(kind, title string) answer {
	s.t.Helper()
	if len(s.answers) == 0 {
		s.t.Fatalf("unexpected %s %q; said so far:\n%s", kind, title, strings.Join(s.said, "\n"))
	}
	a := s.answers[0]
	s.answers = s.answers[1:]
	if !strings.Contains(title, a.title) {
		s.t.Fatalf("%s %q, want one titled %q; said so far:\n%s", kind, title, a.title, strings.Join(s.said, "\n"))
	}
	return a
}

func (s *scriptAsker) pick(spec pickSpec) (pickResult, bool) {
	s.t.Helper()
	a := s.next("list", spec.Title)
	if a.cancel {
		return pickResult{}, false
	}
	find := func(label string) int {
		for i, it := range spec.Items {
			if strings.Contains(it.Label, label) {
				if it.Disabled {
					s.t.Fatalf("list %q: row %q is disabled", spec.Title, it.Label)
				}
				return i
			}
		}
		var labels []string
		for _, it := range spec.Items {
			labels = append(labels, it.Label)
		}
		s.t.Fatalf("list %q has no row %q; rows: %v", spec.Title, label, labels)
		return -1
	}
	switch {
	case a.deleteLabel != "":
		return pickResult{Index: find(a.deleteLabel), Deleted: true}, true
	case len(a.tick) > 0:
		var idx []int
		for _, l := range a.tick {
			idx = append(idx, find(l))
		}
		return pickResult{Index: idx[0], Checked: idx}, true
	}
	i := find(a.label)
	return pickResult{Index: i, Checked: []int{i}}, true
}

func (s *scriptAsker) text(spec textSpec) (string, bool) {
	s.t.Helper()
	a := s.next("question", spec.Title)
	if a.cancel {
		return "", false
	}
	if spec.Validate != nil {
		if err := spec.Validate(a.text); err != nil {
			s.t.Fatalf("question %q refused %q: %v", spec.Title, a.text, err)
		}
	}
	return a.text, true
}

func (s *scriptAsker) wait(message string, work func(context.Context) error) error {
	return work(context.Background())
}

func (s *scriptAsker) say(format string, args ...any) {
	s.said = append(s.said, strings.TrimSpace(fmt.Sprintf(format, args...)))
}

func (s *scriptAsker) done() {
	s.t.Helper()
	if len(s.answers) > 0 {
		s.t.Fatalf("%d answers left over, first %+v; said:\n%s", len(s.answers), s.answers[0], strings.Join(s.said, "\n"))
	}
}

func (s *scriptAsker) saidSomething(part string) bool {
	for _, line := range s.said {
		if strings.Contains(line, part) {
			return true
		}
	}
	return false
}

// setupHome points HOME at a fresh directory holding config, a copy of the
// example configuration, and returns the UI loaded from it.
func setupHome(t *testing.T, config string) (*UI, string) {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv("OPENCODE_API_KEY", "")
	t.Setenv("KVIT_RUN_ID", "run1")
	dir := filepath.Join(home, ".kvit-coder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	return loadUI(t, path, dir), dir
}

func loadUI(t *testing.T, path, dir string) *UI {
	t.Helper()
	cfg, err := config.Load(path)
	if err != nil {
		t.Fatal(err)
	}
	return New(Options{Config: cfg, ConfigPath: path, StatePath: filepath.Join(dir, StateName)})
}

// scripted is a setup run with its network calls replaced.
func scripted(u *UI, ask *scriptAsker, listed []modelsetup.ListedModel, cat *modelsetup.Catalog,
	probe func(e config.ModelEntry, key string) modelsetup.ProbeResult) *setupRun {
	u.ask = ask
	s := u.newSetupRun()
	s.ask = ask
	s.listModels = func(ctx context.Context, baseURL, key string, headers map[string]string) ([]modelsetup.ListedModel, error) {
		return listed, nil
	}
	s.loadCatalog = func(ctx context.Context) (*modelsetup.Catalog, string, error) {
		if cat == nil {
			return nil, "", errors.New("offline")
		}
		return cat, "", nil
	}
	s.probe = func(ctx context.Context, p modelsetup.Provider, e config.ModelEntry, key string, headers map[string]string, tryOthers bool) modelsetup.ProbeResult {
		if probe == nil {
			return modelsetup.ProbeResult{Backend: e.APIBackend}
		}
		return probe(e, key)
	}
	s.openBrowser = func(string) bool { return false }
	return s
}

func fixtureCatalog(t *testing.T) *modelsetup.Catalog {
	t.Helper()
	f, err := os.Open("../modelsetup/testdata/models-dev.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	c, err := modelsetup.ParseCatalog(f, time.Unix(0, 0))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

// The example configuration as it is now: no model at all.
const noModelConfig = "llm:\n  request_timeout: 600\nworkspace:\n  root: \".\"\n"

// TestSetupLocalServer: a local server's models are listed, one is ticked and
// saved, and kvit-coder-ui switches to it and remembers it.
func TestSetupLocalServer(t *testing.T) {
	u, dir := setupHome(t, noModelConfig)
	if u.hasModel() {
		t.Fatal("a config with no model reads as having one")
	}
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "Set up a model provider", label: "Local server"},
		{title: "Address of the local server", text: "http://box:8000/v1"},
		{title: "choose models", tick: []string{"qwen3-coder"}},
		{title: "Check the 1 model", label: "Save"},
	}}
	s := scripted(u, ask, []modelsetup.ListedModel{{ID: "other"}, {ID: "qwen3-coder", Context: 65536}}, nil, nil)
	if !s.run() {
		t.Fatalf("nothing saved; said:\n%s", strings.Join(ask.said, "\n"))
	}
	ask.done()

	rows, err := config.ReadSavedModels()
	if err != nil || len(rows) != 1 {
		t.Fatalf("models.yaml rows %+v, %v", rows, err)
	}
	r := rows[0]
	if r.ID != "qwen3-coder-local" || r.BaseURL != "http://box:8000/v1" || r.Context != 65536 || r.APIKeyEnv != "" {
		t.Errorf("saved row %+v", r)
	}
	if !u.hasModel() || u.currentEntry().ID != "qwen3-coder-local" {
		t.Errorf("current row %+v after saving", u.currentEntry())
	}
	if args := strings.Join(u.agentArgs("hi", nil), " "); !strings.Contains(args, "--model qwen3-coder-local") {
		t.Errorf("agent args %q do not name the row by id", args)
	}
	if st, ok := readUIState(filepath.Join(dir, StateName)); !ok || st.Model != "qwen3-coder-local" {
		t.Errorf("kcu-state.json = %+v, %v", st, ok)
	}
	if _, err := os.Stat(filepath.Join(dir, config.CredentialsName)); err == nil {
		t.Error("a local server with no key wrote credentials.json")
	}
}

// TestSetupOpenCodeGo: a typed key is saved with the rows, the rows come from
// models.dev, and the effort levels can be changed before saving.
func TestSetupOpenCodeGo(t *testing.T) {
	u, _ := setupHome(t, noModelConfig)
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "Set up a model provider", label: "OpenCode Go"},
		{title: "OpenCode Go API key", text: "sk-go-0123456789"},
		{title: "choose models", tick: []string{"muse-spark-1.3-contributor", "kimi-k3"}},
		{title: "Check the 2 models", label: "Change Kimi K3"},
		{title: "Change Kimi K3", label: "Effort levels"},
		{title: "Effort levels for Kimi K3", text: "low high* max"},
		{title: "Change Kimi K3", label: "Done"},
		{title: "Check the 2 models", label: "Save"},
	}}
	listed := []modelsetup.ListedModel{{ID: "kimi-k3"}, {ID: "minimax-m3"}, {ID: "muse-spark-1.3-contributor"}}
	s := scripted(u, ask, listed, fixtureCatalog(t), nil)
	if !s.run() {
		t.Fatalf("nothing saved; said:\n%s", strings.Join(ask.said, "\n"))
	}
	ask.done()

	if got := config.SavedKey("OPENCODE_API_KEY"); got != "sk-go-0123456789" {
		t.Errorf("saved key %q", got)
	}
	rows, _ := config.ReadSavedModels()
	if len(rows) != 2 {
		t.Fatalf("rows %+v", rows)
	}
	spark, kimi := rows[0], rows[1]
	if spark.ID != "muse-spark-1.3-contributor-go" || spark.APIBackend != llm.BackendResponses || spark.Context != 1048576 {
		t.Errorf("spark row %+v", spark)
	}
	if kimi.ID != "kimi-k3-go" || kimi.APIBackend != llm.BackendChatCompletions || kimi.EffortField != llm.EffortFieldReasoningEffort {
		t.Errorf("kimi row %+v", kimi)
	}
	if effortsText(kimi.Efforts) != "low high* max" {
		t.Errorf("kimi efforts %q, want the edited ones", effortsText(kimi.Efforts))
	}
	if len(kimi.Headers) != 2 || u.cfg.HeadersFor(kimi)["x-opencode-session"] != "kvit-coder-run1" {
		t.Errorf("kimi headers %v", kimi.Headers)
	}
	// The key is in the file, not the environment, and the loaded row finds it.
	if !u.hasKey(u.currentEntry()) || os.Getenv("OPENCODE_API_KEY") != "" {
		t.Error("the saved key is not found for the new row")
	}
}

// TestSetupTestFindsProtocol: an endpoint the person names gets a test
// request first; the protocol that answered replaces the guessed one.
func TestSetupTestFindsProtocol(t *testing.T) {
	u, _ := setupHome(t, noModelConfig)
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "Set up a model provider", label: "Other endpoint"},
		{title: "A name for this endpoint", text: "My Proxy"},
		{title: "Address of My Proxy", text: "https://llm.example.com/v1"},
		{title: "Environment variable", text: "MY_PROXY_API_KEY"},
		{title: "My Proxy API key", text: "pk-1"},
		{title: "choose models", tick: []string{"big-model"}},
		{title: "Check the 1 model", label: "Test each model"},
	}}
	probed := 0
	s := scripted(u, ask, []modelsetup.ListedModel{{ID: "big-model"}}, nil, func(e config.ModelEntry, key string) modelsetup.ProbeResult {
		probed++
		if key != "pk-1" {
			t.Errorf("probe sent key %q", key)
		}
		return modelsetup.ProbeResult{Backend: llm.BackendMessages}
	})
	if !s.run() {
		t.Fatalf("nothing saved; said:\n%s", strings.Join(ask.said, "\n"))
	}
	ask.done()
	rows, _ := config.ReadSavedModels()
	if probed != 1 || len(rows) != 1 {
		t.Fatalf("probed %d, rows %+v", probed, rows)
	}
	r := rows[0]
	if r.ID != "big-model-my-proxy" || r.APIBackend != llm.BackendMessages || r.BaseURL != "https://llm.example.com" || r.APIKeyEnv != "MY_PROXY_API_KEY" {
		t.Errorf("row %+v", r)
	}
	if config.SavedKey("MY_PROXY_API_KEY") != "pk-1" {
		t.Error("the endpoint's key was not saved")
	}
	if !ask.saidSomething("answered over messages") {
		t.Errorf("no line about the protocol found; said:\n%s", strings.Join(ask.said, "\n"))
	}
}

// TestSetupRefusedKey: a refused key can be typed again, and leaving the
// screens after that saves nothing.
func TestSetupRefusedKey(t *testing.T) {
	u, dir := setupHome(t, noModelConfig)
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "Set up a model provider", label: "OpenCode Zen"},
		{title: "OpenCode Zen API key", text: "wrong-key-0000000"},
		{title: "choose models", tick: []string{"big-pickle"}},
		{title: "Check the 1 model", label: "Test each model"},
		{title: "did not answer", label: "Enter the key again"},
		{title: "OpenCode Zen API key", text: "right-key-1111111"},
		{title: "choose models", cancel: true},
		{title: "Set up a model provider", cancel: true},
	}}
	keys := []string{}
	s := scripted(u, ask, []modelsetup.ListedModel{{ID: "big-pickle"}}, fixtureCatalog(t), func(e config.ModelEntry, key string) modelsetup.ProbeResult {
		keys = append(keys, key)
		return modelsetup.ProbeResult{Backend: e.APIBackend, KeyRefused: true, Err: errors.New("API error 401: bad key")}
	})
	if s.run() {
		t.Fatal("reported a save")
	}
	ask.done()
	if len(keys) != 1 || keys[0] != "wrong-key-0000000" {
		t.Errorf("probed with %v", keys)
	}
	for _, name := range []string{config.SavedModelsName, config.CredentialsName} {
		if _, err := os.Stat(filepath.Join(dir, name)); err == nil {
			t.Errorf("%s written although nothing was saved", name)
		}
	}
}

// TestSetupMarksModelsAlreadySetUp: a model a row already sends to the same
// provider cannot be ticked twice.
func TestSetupMarksModelsAlreadySetUp(t *testing.T) {
	u, _ := setupHome(t, noModelConfig)
	if err := config.AppendSavedModels(config.ModelEntry{ID: "kimi-k3-go", Model: "kimi-k3", BaseURL: "https://opencode.ai/zen/go/v1"}); err != nil {
		t.Fatal(err)
	}
	if err := u.reloadConfig(); err != nil {
		t.Fatal(err)
	}
	var items []pickItem
	ask := &scriptAsker{t: t}
	s := scripted(u, ask, []modelsetup.ListedModel{{ID: "kimi-k3"}, {ID: "glm-5.3"}}, fixtureCatalog(t), nil)
	s.ask = &capturingAsker{scriptAsker: ask, onPick: func(spec pickSpec) { items = spec.Items }}
	s.chooseModels(modelsetup.Providers[1], "k", false)
	if len(items) != 2 || !items[0].Disabled || !strings.Contains(items[0].Detail, "already set up as :m1") || items[1].Disabled {
		t.Errorf("model list rows %+v", items)
	}
}

// capturingAsker records the first list it is shown and closes it.
type capturingAsker struct {
	*scriptAsker
	onPick func(pickSpec)
}

func (c *capturingAsker) pick(spec pickSpec) (pickResult, bool) {
	if c.onPick != nil {
		c.onPick(spec)
		c.onPick = nil
	}
	return pickResult{}, false
}

// TestStartupCheck: no model offers setup; a starting row without its key on
// a remote address offers to save the key, and saving it ends the warning.
func TestStartupCheck(t *testing.T) {
	u, _ := setupHome(t, noModelConfig)
	ask := &scriptAsker{t: t, answers: []answer{{title: "Set up a model now?", label: "Continue without a model"}}}
	u.ask = ask
	u.startupCheck(false)
	ask.done()

	u, _ = setupHome(t, `
models:
  - id: remote
    model: m
    base_url: https://llm.example.com/v1
    api_key_env: KVIT_TUI_TEST_KEY
  - id: local
    model: q
    base_url: http://localhost:8080/v1
    api_key_env: KVIT_TUI_TEST_KEY
`)
	t.Setenv("KVIT_TUI_TEST_KEY", "")
	ask = &scriptAsker{t: t, answers: []answer{
		{title: "What now?", label: "Enter the key for KVIT_TUI_TEST_KEY"},
		{title: "API key for KVIT_TUI_TEST_KEY", text: "sk-saved-0123456789"},
	}}
	u.ask = ask
	u.startupCheck(false)
	ask.done()
	if config.SavedKey("KVIT_TUI_TEST_KEY") != "sk-saved-0123456789" || !u.hasKey(u.currentEntry()) {
		t.Error("the key was not saved or not found afterwards")
	}
	ask = &scriptAsker{t: t}
	u.ask = ask
	u.startupCheck(false) // nothing to ask now
	u.selectModel(1)
	config.DeleteCredential("KVIT_TUI_TEST_KEY")
	u.startupCheck(false) // a loopback address is not asked about
	ask.done()
}

// TestRestoreChoice: the model and effort last chosen are where the next
// start opens, unless the row is gone or -m names another.
func TestRestoreChoice(t *testing.T) {
	cfgText := `
models:
  - id: a
    model: wire-a
    base_url: http://localhost:1/v1
  - id: b
    model: wire-b
    base_url: http://localhost:2/v1
    efforts:
      - value: low
      - value: high
        default: true
`
	u, dir := setupHome(t, cfgText)
	u.switchModel(2)
	u.setEffortValue("low")
	again := loadUI(t, filepath.Join(dir, "config.yaml"), dir)
	if again.currentEntry().ID != "b" || again.effectiveEffort() != "low" {
		t.Errorf("restarted on %s at %q, want b at low", again.currentEntry().ID, again.effectiveEffort())
	}
	cfg, _ := config.Load(filepath.Join(dir, "config.yaml"))
	pinned := New(Options{Config: cfg, StatePath: filepath.Join(dir, StateName), InitialModelSet: true, InitialModel: 0})
	if pinned.currentEntry().ID != "a" {
		t.Errorf("-m lost to the saved choice: on %s", pinned.currentEntry().ID)
	}
}

// TestModelsRemove: :models removes a row :setup added, and refuses one
// written in config.yaml.
func TestModelsRemove(t *testing.T) {
	u, _ := setupHome(t, "models:\n  - id: hand\n    model: h\n    base_url: http://localhost:1/v1\n")
	if err := config.AppendSavedModels(config.ModelEntry{ID: "saved", Name: "Saved one", Model: "s", BaseURL: "http://localhost:2/v1"}); err != nil {
		t.Fatal(err)
	}
	if err := u.reloadConfig(); err != nil {
		t.Fatal(err)
	}
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "Models", deleteLabel: ":m1"},
		{title: "Models", deleteLabel: "Saved one"},
		{title: "Remove Saved one?", label: "Remove it"},
		{title: "Models", cancel: true},
	}}
	u.ask = ask
	u.showModelPicker()
	ask.done()
	rows, _ := config.ReadSavedModels()
	if len(rows) != 0 || len(u.models) != 1 {
		t.Errorf("after removing: saved %+v, list %+v", rows, u.models)
	}
	if !ask.saidSomething("remove it there") {
		t.Errorf("no refusal for the config.yaml row; said:\n%s", strings.Join(ask.said, "\n"))
	}
}

// TestKeys: :keys lists the variables the rows read, and saves and forgets a
// key.
func TestKeys(t *testing.T) {
	u, _ := setupHome(t, "models:\n  - id: r\n    name: Remote\n    model: m\n    base_url: https://x.invalid/v1\n    api_key_env: KVIT_TUI_KEYS\n")
	t.Setenv("KVIT_TUI_KEYS", "")
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "API keys", label: "KVIT_TUI_KEYS"},
		{title: "KVIT_TUI_KEYS", label: "Enter a new key"},
		{title: "API key for KVIT_TUI_KEYS", text: "sk-abcd-0000-wxyz"},
		{title: "API keys", label: "KVIT_TUI_KEYS"},
		{title: "KVIT_TUI_KEYS", label: "Forget the saved key"},
		{title: "API keys", cancel: true},
	}}
	u.ask = ask
	u.showKeys()
	ask.done()
	if config.SavedKey("KVIT_TUI_KEYS") != "" || !ask.saidSomething("Forgot the key saved for KVIT_TUI_KEYS") {
		t.Errorf("key not forgotten; said:\n%s", strings.Join(ask.said, "\n"))
	}
	if got := keyDetail(keyUse{env: "X_UNSET_KEY", models: []string{"A", "B", "C"}}); got != "missing · used by A and 2 more" {
		t.Errorf("keyDetail = %q", got)
	}
	if maskKey("sk-abcd-0000-wxyz") != "sk-a…wxyz" || maskKey("short") != "…" {
		t.Error("maskKey shows too much")
	}
}

func TestModelRef(t *testing.T) {
	u := testCatalogUI(t)
	if got := u.modelRef(u.models[0]); got != "go" {
		t.Errorf("modelRef with a models: list = %q, want the id", got)
	}
	if got := u.modelRef(config.ModelEntry{ID: "m3", Model: "wire"}); got != "wire" {
		t.Errorf("an id that reads as a row number = %q, want the model id", got)
	}
	legacy := &config.Config{}
	legacy.LLM.Model = "solo"
	lu := New(Options{Config: legacy})
	if got := lu.modelRef(lu.currentEntry()); got != "solo" {
		t.Errorf("modelRef with only llm: = %q, want the model id", got)
	}
}

func TestParseEfforts(t *testing.T) {
	opts, err := parseEfforts("Low high* max")
	if err != nil || effortsText(opts) != "low high* max" {
		t.Errorf("parse = %q, %v", effortsText(opts), err)
	}
	if opts, _ := parseEfforts("low medium"); !opts[1].Default {
		t.Error("with no * the last level should be the default")
	}
	if opts, err := parseEfforts("none"); err != nil || opts != nil {
		t.Errorf("none = %v, %v", opts, err)
	}
	for _, bad := range []string{"low* high*", "turbo"} {
		if _, err := parseEfforts(bad); err == nil {
			t.Errorf("parseEfforts(%q) accepted", bad)
		}
	}
}

// TestPicker drives the list with keys: filtering, ticking, skipping
// disabled rows, Enter, Delete and Esc.
func TestPicker(t *testing.T) {
	key := func(m *pickerModel, k tea.KeyMsg) { m.Update(k) }
	runes := func(s string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)} }
	items := []pickItem{{Label: "alpha"}, {Label: "beta", Disabled: true}, {Label: "gamma"}, {Label: "delta", Detail: "fourth"}}

	m := newPicker(pickSpec{Title: "t", Items: items})
	key(m, tea.KeyMsg{Type: tea.KeyDown})
	if idx, _ := m.current(); idx != 2 {
		t.Errorf("down from alpha landed on %d, want 2 (beta is disabled)", idx)
	}
	key(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.done || m.result.Index != 2 {
		t.Errorf("enter: done %v result %+v", m.done, m.result)
	}

	m = newPicker(pickSpec{Title: "t", Items: items})
	key(m, runes("four"))
	if idx, _ := m.current(); len(m.visible) != 1 || idx != 3 {
		t.Errorf("filter on the detail: visible %v", m.visible)
	}
	key(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.done || len(m.visible) != 4 {
		t.Error("Esc with a filter should clear it, not close")
	}
	key(m, tea.KeyMsg{Type: tea.KeyEsc})
	if !m.done || !m.cancelled {
		t.Error("Esc without a filter should close")
	}

	m = newPicker(pickSpec{Title: "t", Items: items, Multi: true})
	key(m, tea.KeyMsg{Type: tea.KeySpace}) // tick alpha, move to gamma
	key(m, tea.KeyMsg{Type: tea.KeyDown})  // to delta
	key(m, tea.KeyMsg{Type: tea.KeySpace}) // tick delta
	key(m, tea.KeyMsg{Type: tea.KeyEnter})
	if got := m.result.Checked; len(got) != 2 || got[0] != 0 || got[1] != 3 {
		t.Errorf("ticked %v, want [0 3]", got)
	}

	m = newPicker(pickSpec{Title: "t", Items: items, Delete: true, Start: 3})
	key(m, tea.KeyMsg{Type: tea.KeyDelete})
	if !m.result.Deleted || m.result.Index != 3 {
		t.Errorf("delete: %+v", m.result)
	}
	if v := newPicker(pickSpec{Title: "Title", Items: items}).View(); !strings.Contains(v, "> alpha") || !strings.Contains(v, "fourth") {
		t.Errorf("view:\n%s", v)
	}
}

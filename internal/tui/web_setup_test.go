package tui

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// agentConfig loads the configuration the way the agent's next turn will.
func agentConfig(t *testing.T, u *UI) *config.Config {
	t.Helper()
	cfg, err := config.Load(u.configPath)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

// TestSetupWebSearch: web search is listed in :setup; turning it on asks for
// the key, saves it with the switches, and turns page fetch on with it.
// Turning it off again keeps the key.
func TestSetupWebSearch(t *testing.T) {
	u, _ := setupHome(t, noModelConfig)
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "Set up a model provider or web search", label: "Web search"},
		{title: "Web search", label: "Web search"},
		{title: "Brave Search API key", text: "brave-0123456789"},
		{title: "Web search", label: "Done"},
		{title: "Set up a model provider", cancel: true},
	}}
	s := scripted(u, ask, nil, nil, nil)
	if s.run() {
		t.Error("run reported models saved when only web search was")
	}
	ask.done()

	if got := config.SavedKey(config.DefaultWebSearchKeyEnv); got != "brave-0123456789" {
		t.Errorf("saved key %q", got)
	}
	cfg := agentConfig(t, u)
	if !cfg.Tools.Web.Search.Enabled || !cfg.Tools.Web.Fetch.Enabled {
		t.Errorf("agent sees search %v fetch %v, want both on", cfg.Tools.Web.Search.Enabled, cfg.Tools.Web.Fetch.Enabled)
	}
	if u.webSummary() != "search and page fetch on" {
		t.Errorf("summary %q after turning search on", u.webSummary())
	}
	if !ask.saidSomething("Setup closed.") || ask.saidSomething("nothing was saved") {
		t.Errorf("closing line wrong; said:\n%s", strings.Join(ask.said, "\n"))
	}

	ask = &scriptAsker{t: t, answers: []answer{
		{title: "Web search", label: "Web search"},
		{title: "Web search", cancel: true},
	}}
	u.ask = ask
	if !u.setUpWeb() {
		t.Error("turning search off reported nothing saved")
	}
	ask.done()
	cfg = agentConfig(t, u)
	if cfg.Tools.Web.Search.Enabled || !cfg.Tools.Web.Fetch.Enabled {
		t.Errorf("agent sees search %v fetch %v, want search off and fetch still on", cfg.Tools.Web.Search.Enabled, cfg.Tools.Web.Fetch.Enabled)
	}
	if config.SavedKey(config.DefaultWebSearchKeyEnv) == "" || !ask.saidSomething("The key stays saved") {
		t.Errorf("the key was not kept; said:\n%s", strings.Join(ask.said, "\n"))
	}
}

// TestWebSearchUsesKeyInEnvironment: with the variable set, turning search on
// asks for nothing and saves no key.
func TestWebSearchUsesKeyInEnvironment(t *testing.T) {
	u, _ := setupHome(t, noModelConfig)
	t.Setenv(config.DefaultWebSearchKeyEnv, "from-env")
	ask := &scriptAsker{t: t}
	u.ask = ask
	if !u.turnOnWebSearch() {
		t.Fatalf("not turned on; said:\n%s", strings.Join(ask.said, "\n"))
	}
	ask.done()
	if config.SavedKey(config.DefaultWebSearchKeyEnv) != "" {
		t.Error("a key from the environment was copied into credentials.json")
	}
	if !ask.saidSomething("Using the key in the environment variable") {
		t.Errorf("said:\n%s", strings.Join(ask.said, "\n"))
	}
}

// TestWebSearchSetInConfig: a switch config.yaml sets itself cannot be
// changed from kvit-coder-ui, and is never offered; the other one can.
func TestWebSearchSetInConfig(t *testing.T) {
	u, _ := setupHome(t, noModelConfig+"tools:\n  web:\n    search:\n      enabled: false\n")
	ask := &scriptAsker{t: t}
	u.ask = ask
	u.offerWebSearch("Turn on web search too?") // asks nothing
	ask.done()

	ask = &scriptAsker{t: t, answers: []answer{
		{title: "Web search", label: "Page fetch"},
		{title: "Web search", label: "Done"},
	}}
	var searchRow pickItem
	u.ask = &peekAsker{scriptAsker: ask, peek: func(spec pickSpec) {
		if spec.Title == "Web search" {
			searchRow = spec.Items[0]
		}
	}}
	u.setUpWeb()
	ask.done()
	if !searchRow.Disabled || !strings.Contains(searchRow.Detail, "change it there") {
		t.Errorf("search row %+v, want it disabled and pointing at config.yaml", searchRow)
	}
	cfg := agentConfig(t, u)
	if cfg.Tools.Web.Search.Enabled || !cfg.Tools.Web.Fetch.Enabled {
		t.Errorf("agent sees search %v fetch %v, want fetch only", cfg.Tools.Web.Search.Enabled, cfg.Tools.Web.Fetch.Enabled)
	}
}

// peekAsker lets a test look at each list before the script answers it.
type peekAsker struct {
	*scriptAsker
	peek func(pickSpec)
}

func (p *peekAsker) pick(spec pickSpec) (pickResult, bool) {
	p.peek(spec)
	return p.scriptAsker.pick(spec)
}

// TestFirstSetupOffersWebSearch walks a first start from no model to a saved
// model and web search, against a local server that lists one model.
func TestFirstSetupOffersWebSearch(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		fmt.Fprint(w, `{"data":[{"id":"qwen3-coder"}]}`)
	}))
	defer srv.Close()

	u, _ := setupHome(t, noModelConfig)
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "Set up a model now?", label: "Set up a model provider"},
		{title: "Set up a model provider", label: "Local server"},
		{title: "Address of the local server", text: srv.URL + "/v1"},
		{title: "choose models", tick: []string{"qwen3-coder"}},
		{title: "Check the 1 model", label: "Save"},
		{title: "Turn on web search too?", label: "Turn on web search"},
		{title: "Brave Search API key", text: "brave-first-run"},
	}}
	u.ask = ask
	u.startupCheck(false)
	ask.done()

	if !u.hasModel() || u.currentEntry().ID != "qwen3-coder-local" {
		t.Errorf("current row %+v", u.currentEntry())
	}
	cfg := agentConfig(t, u)
	if !cfg.Tools.Web.Search.Enabled || config.LookupKey(cfg.WebSearchKeyEnv()) != "brave-first-run" {
		t.Errorf("web search %v with key %q after the first run", cfg.Tools.Web.Search.Enabled, config.LookupKey(cfg.WebSearchKeyEnv()))
	}
}

// TestKeysOffersWebSearch: :keys lists the web search variable while search
// is off, and entering its key there offers to turn search on.
func TestKeysOffersWebSearch(t *testing.T) {
	u, dir := setupHome(t, noModelConfig)
	ask := &scriptAsker{t: t, answers: []answer{
		{title: "API keys", label: config.DefaultWebSearchKeyEnv},
		{title: config.DefaultWebSearchKeyEnv, label: "Enter a new key"},
		{title: "API key for " + config.DefaultWebSearchKeyEnv, text: "brave-from-keys"},
		{title: "Turn on web search now?", label: "Turn on web search"},
		{title: "API keys", cancel: true},
	}}
	u.ask = ask
	u.showKeys()
	ask.done()
	if !u.webState().search {
		t.Errorf("search still off; said:\n%s", strings.Join(ask.said, "\n"))
	}
	if again := loadUI(t, filepath.Join(dir, "config.yaml"), dir); !again.webState().search {
		t.Error("a restarted kvit-coder-ui does not see web search on")
	}
	uses := u.keyUses()
	if len(uses) != 1 || keyDetail(uses[0]) != "saved brav…keys · used by web search" {
		t.Errorf("key uses %+v, detail %q", uses, keyDetail(uses[0]))
	}
}

// captureStdout returns what f prints.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	saved := os.Stdout
	os.Stdout = w
	done := make(chan string)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	defer func() { os.Stdout = saved }()
	f()
	w.Close()
	os.Stdout = saved
	return <-done
}

// The llm: block of the example configuration installs seeded until October
// 2026, which counts as naming no model.
const placeholderConfig = "llm:\n  base_url: \"http://localhost:8080/v1\"\n  model: \"your-model\"\n  api_key_env: \"OPENAI_API_KEY\"\n"

// TestNoModelDisplay: with no model set up, the commands that list or need a
// model say there is none, rather than showing the empty row built from the
// llm: block, and its key variable is not listed in :keys.
func TestNoModelDisplay(t *testing.T) {
	u, dir := setupHome(t, placeholderConfig)
	if u.hasModel() {
		t.Fatal("the example's placeholder reads as a model")
	}
	ask := &scriptAsker{t: t}
	u.ask = ask

	out := captureStdout(t, func() { u.handleCommand(":config") })
	if !strings.Contains(out, "Model: none set up yet") || strings.Contains(out, ":m1") || strings.Contains(out, "Base URL") {
		t.Errorf(":config printed:\n%s", out)
	}
	out = captureStdout(t, u.showModels)
	if strings.TrimSpace(out) != "Models: none set up yet (:setup adds one)" {
		t.Errorf("model list in :help printed:\n%s", out)
	}
	for _, cmd := range []string{":m1", ":e1", ":e high", ":models"} {
		out = captureStdout(t, func() { u.handleCommand(cmd) })
		if !strings.Contains(out, "No model is set up yet; :setup adds one.") {
			t.Errorf("%s printed:\n%s", cmd, out)
		}
	}
	ask.done()
	if _, err := os.Stat(filepath.Join(dir, StateName)); err == nil {
		t.Error("a command with no model wrote kcu-state.json")
	}
	for _, use := range u.keyUses() {
		if use.env == "OPENAI_API_KEY" {
			t.Errorf(":keys lists the empty row's variable: %+v", use)
		}
	}
}

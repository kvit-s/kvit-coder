package tui

import (
	"errors"
	"os"

	"github.com/kvit-s/kvit-coder/internal/config"
)

// Web search in kvit-coder-ui: whether the agent may search the web
// (Web.search, which queries the Brave Search API) and read pages (Web.fetch),
// and the key search needs. :setup lists it below the model providers, a first
// setup offers it once a model is saved, and :keys offers it when the search
// key is entered there. The two switches are saved to ~/.kvit-coder/tools.yaml
// and a typed key to credentials.json; config.Load reads both for every agent
// turn, so a change applies from the next prompt. A switch config.yaml sets
// itself takes precedence and is shown as decided there.

// braveKeyPage is where Brave issues Search API keys.
const braveKeyPage = "https://brave.com/search/api/"

// webState is the two web tools as the next turn will have them, and whether
// config.yaml decides each.
type webState struct {
	search, fetch                 bool
	searchInConfig, fetchInConfig bool
}

func (u *UI) webState() webState {
	st := webState{search: u.cfg.Tools.Web.Search.Enabled, fetch: u.cfg.Tools.Web.Fetch.Enabled}
	st.searchInConfig, st.fetchInConfig = u.cfg.WebSetInConfig()
	return st
}

// webSummary is the state of the web tools in a few words.
func (u *UI) webSummary() string {
	st := u.webState()
	switch {
	case st.search && st.fetch:
		return "search and page fetch on"
	case st.search:
		return "search on, page fetch off"
	case st.fetch:
		return "page fetch on, search off"
	}
	return "off"
}

func onOff(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// setUpWeb is the web search screen: a switch for each web tool and the
// search key, each change saved as it is made. It reports whether anything
// was saved.
func (u *UI) setUpWeb() bool {
	u.ask.say("Web search lets the agent look things up through the Brave Search API, and page fetch lets it read a web page as text. Both reach the network from this machine without asking first.")
	saved := false
	for {
		st := u.webState()
		env := u.cfg.WebSearchKeyEnv()
		searchAbout := "needs a Brave Search API key"
		if st.search {
			searchAbout = "searches through the Brave Search API"
		}
		res, ok := u.ask.pick(pickSpec{
			Title: "Web search",
			Hint:  "Enter to switch one on or off · Esc when done",
			Items: []pickItem{
				u.switchItem("Web search", st.search, st.searchInConfig, searchAbout),
				u.switchItem("Page fetch", st.fetch, st.fetchInConfig, "reads one page as markdown; needs no key"),
				{Label: "Brave Search API key", Detail: keySource(env)},
				{Label: "Done"},
			},
		})
		if !ok || res.Index == 3 {
			u.ask.say("")
			return saved
		}
		switch res.Index {
		case 0:
			if !st.search {
				saved = u.turnOnWebSearch() || saved
				continue
			}
			off := false
			if u.saveWeb(&off, nil) {
				saved = true
				u.sayWeb()
				if config.SavedKey(env) != "" {
					u.ask.say("The key stays saved as %s; :keys forgets it.", env)
				}
			}
		case 1:
			fetch := !st.fetch
			if u.saveWeb(nil, &fetch) {
				saved = true
				u.sayWeb()
			}
		case 2:
			u.keyActions(env)
		}
	}
}

// switchItem is one web tool's row in the web search screen. A tool
// config.yaml switches itself cannot be changed here.
func (u *UI) switchItem(label string, on, inConfig bool, about string) pickItem {
	if inConfig {
		return pickItem{Label: label, Detail: onOff(on) + ", set in " + u.configPath + "; change it there", Disabled: true}
	}
	return pickItem{Label: label, Detail: onOff(on) + " · " + about}
}

// turnOnWebSearch asks for the search key when none can be found, then turns
// search on, and page fetch with it: a search result is a title, an address
// and a sentence or two, and the agent reads the page it picks with Web.fetch.
func (u *UI) turnOnWebSearch() bool {
	st := u.webState()
	if st.searchInConfig {
		u.ask.say("Web search is switched %s in %s, which takes precedence; change it there.", onOff(st.search), u.configPath)
		return false
	}
	env := u.cfg.WebSearchKeyEnv()
	if config.LookupKey(env) == "" {
		key, ok := u.askSearchKey(env)
		if !ok {
			u.ask.say("Web search stays off.")
			return false
		}
		if err := config.SaveCredential(env, key); err != nil {
			u.ask.say("\033[31mThe key could not be saved: %v\033[0m", err)
			return false
		}
		path, _ := config.CredentialsPath()
		u.ask.say("Saved the key for %s to %s.", env, path)
	} else if os.Getenv(env) != "" {
		u.ask.say("Using the key in the environment variable %s.", env)
	} else {
		u.ask.say("Using the key saved for %s.", env)
	}
	on := true
	var fetch *bool
	if !st.fetch && !st.fetchInConfig {
		fetch = &on
	}
	if !u.saveWeb(&on, fetch) {
		return false
	}
	u.sayWeb()
	return true
}

// askSearchKey points at the page that issues keys and reads one.
func (u *UI) askSearchKey(env string) (string, bool) {
	if u.openBrowser(braveKeyPage) {
		u.ask.say("Brave issues Search API keys at %s, which is now open in your browser: sign up, choose a plan and copy the key from the dashboard.", braveKeyPage)
	} else {
		u.ask.say("Brave issues Search API keys at %s: sign up, choose a plan and copy the key from the dashboard.", braveKeyPage)
	}
	return u.ask.text(textSpec{
		Title:  "Brave Search API key",
		Hint:   "saved to ~/.kvit-coder/credentials.json as " + env + " · Esc to go back",
		Secret: true,
		Validate: func(v string) error {
			if v == "" {
				return errors.New("paste the key, or press Esc to go back")
			}
			return nil
		},
	})
}

// saveWeb writes the switches it is given to tools.yaml, a nil one left as
// the file has it, and loads the configuration again so what is shown next is
// what the next turn will have.
func (u *UI) saveWeb(search, fetch *bool) bool {
	if err := config.SaveWebTools(search, fetch); err != nil {
		u.ask.say("\033[31mThe web search setting could not be saved: %v\033[0m", err)
		return false
	}
	if err := u.reloadConfig(); err != nil {
		u.ask.say("\033[31mSaved, but the configuration did not load again: %v\033[0m", err)
	}
	return true
}

// sayWeb reports the web tools after a change, and where it was saved.
func (u *UI) sayWeb() {
	st := u.webState()
	path, _ := config.SavedToolsPath()
	u.ask.say("\033[38;5;78mFrom the next prompt web search is %s and page fetch is %s (saved to %s).\033[0m",
		onOff(st.search), onOff(st.fetch), path)
}

// offerWebSearch asks once whether to turn web search on, when it is off and
// kvit-coder-ui may change it: after a first setup, and after the search key
// is entered in :keys.
func (u *UI) offerWebSearch(title string) {
	if st := u.webState(); st.search || st.searchInConfig {
		return
	}
	res, ok := u.ask.pick(pickSpec{
		Title: title,
		Items: []pickItem{
			{Label: "Turn on web search", Detail: "the agent can search through the Brave Search API and read the pages it finds"},
			{Label: "Not now", Detail: ":setup turns it on later"},
		},
	})
	if ok && res.Index == 0 {
		u.turnOnWebSearch()
	}
	u.ask.say("")
}

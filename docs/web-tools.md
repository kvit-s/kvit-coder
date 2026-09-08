# Reaching the web: search, page retrieval, and browser control

## 1. What this document is

kvit-coder cannot reach the web. There is no tool that runs a search, no tool
that fetches a URL, and no way to drive a browser; the only route out is a
`curl` through the shell tool, which needs a permission grant and returns raw
HTML. This document works out what to build for each of those, how, and in what
order.

A note on where the idea came from, since it is easy to look in the wrong place:
`redesign.md` does not discuss web tools at all. `review.md` does, twice — line
324 lists "no web fetch" among the missing tools, and line 428 recommends "for
web fetch and git, prefer MCP servers over new code." That second recommendation
is right for one of the three capabilities below and wrong for the other two, and
section 6 says why.

**Three capabilities, not one.** They are routinely bundled under "web access"
and they have different costs, different failure modes, and different right
answers:

- **Search.** A query returns a ranked list of pages, with enough text about
  each to choose among them.
- **Retrieval.** A URL returns its readable content as text a model can use.
- **Browser control.** Navigate, click, type, wait for JavaScript to run, stay
  logged in across pages.

Search and retrieval keep nothing between calls and belong in the agent as
ordinary Go tools. Browser control keeps a great deal between calls — open
pages, cookies, a half-filled form — and is exactly the case that the per-turn
process model breaks, which makes it the trigger for the daemon proposed in
`redesign-mcp.md`.

## 2. What is already available

### 2.1 The Brave key and what it can do

There is a Brave Search API key in `c:\apps\llama-swap\config-plus.yaml`, used by
the `llama-swap-plus` front door to serve its `:online` model variants. The
client is `~/llama-swap-plus/internal/brave/client.go`, 306 lines, with per-second
rate limiting, a monthly quota counter persisted to `brave_usage.json`, and a
JSONL usage log.

Probing that key directly on 8 September 2026 gives the state precisely:

| Endpoint | Result |
|---|---|
| `GET /res/v1/web/search` | 200. Plan reported as `Free`, `rate_limit: 1`, `quota_limit: 2000`, `quota_current: 0` |
| `GET /res/v1/llm/context` | 400, `OPTION_NOT_IN_PLAN`, "The option is not included in the plan." |
| `extra_snippets=1` on web search | Accepted and ignored; results come back without the field |

So web search works today at one request per second and 2,000 per month, and the
last recorded use was December 2025, so the quota is untouched. Each result is a
title, a URL, and a description of roughly 200 characters with `<strong>` tags
marking the matched terms — enough to choose a page, not enough to answer from.

**What happens when you exceed one request per second**, measured rather than
assumed. The request is refused immediately with HTTP 429 and `code:
RATE_LIMITED`; nothing is queued, delayed, or served stale. There is no
`Retry-After` header. The signal is in headers every response carries, refusals
included:

```
x-ratelimit-policy:    1;w=1, 2000;w=2592000
x-ratelimit-limit:     1, 2000
x-ratelimit-remaining: 0, 1997
x-ratelimit-reset:     1, 1960234
```

Two policies: one request per one-second window, two thousand per thirty-day
window. `remaining` reports both, `reset` gives the seconds until each window
reopens, so the right wait after a 429 is about one second.

**Only served requests are charged.** Firing three requests concurrently into one
window, then four more spread over the following twenty seconds, gives seven
requests of which five were served and two refused, and the monthly counter moved
exactly five times. The refusals cost nothing, and neither did an earlier 400.
The numbers only read that way once you know the header reports the count
*before* charging for the request carrying it: the three concurrent requests all
reported the same monthly figure, and the single one of them that was served is
what moved it. The counter is exact rather than lagging.

That has a direct consequence for the retry: it can be bounded for wall-clock
reasons alone, since hammering a one-per-second limiter wastes seconds without
spending quota. It also means the one-per-second window admits exactly one
request, and which of several concurrent requests wins is the server's choice
rather than the order the client issued them.

**The key is shared, and not only with this machine.** The llama-swap-plus front
door uses it to serve its `:online` model variants, and other clients on other
hosts use it too. That settles a design question before it is asked: no local
quota accounting is possible, because nothing on this machine can see what a
different machine spent. The headers above are account-wide, so the server is
already counting every consumer, and reading what it reports is both simpler and
correct where a local counter would be neither.

Brave retired that free tier in February 2026 and replaced it with metered
billing: $5 per 1,000 requests on the Search plan, with $5 of credit granted each
month in exchange for attributing the API somewhere public, which for a personal
tool means a line in the README. Keys issued under the old free tier still
answer, as this one does, but they do not get the newer options — which is what
puts `llm/context` out of reach, and section 2.2 takes that decision rather than
leaving it open. The practical consequence is that the free key's limits are the
limits: one per second, two thousand a month, with nothing behind them.

### 2.2 The LLM Context endpoint, evaluated and not taken

`GET /res/v1/llm/context` takes a query and returns extracted page content rather
than a list of links: `grounding.generic` holds text chunks pulled from the pages
themselves, `sources` holds the metadata for each, and the size is governed by a
token budget — `maximum_number_of_tokens` between 1,024 and 32,768, default
8,192, alongside caps on the number of URLs and snippets. It is one request on
the Search plan, at the same $5 per 1,000.

The reason this matters more here than it would in most agents is the shape of
the interaction it removes. Search-then-read is four round trips: search, choose
from the results, fetch two or three pages, then read them. Every one of those is
a full model call, and at `reasoning_effort: xhigh` — what `config.yaml` runs —
each buys a reasoning pass measured in thousands of tokens and tens of seconds.
The LLM Context endpoint collapses all four into one call that returns the text
already extracted. That is the same argument section 10 of `redesign.md` makes
for the `Batch` tool, applied to the one workflow where the round trips are
unavoidable rather than merely likely.

Against that, it gives up control: you take the chunks Brave chose from the pages
Brave ranked, and you cannot say "not that page, this one."

**The decision is not to take it.** Reaching that endpoint means moving to the
Search plan, which is a paid subscription, and one more recurring bill is not
worth the round trips it saves. This is recorded rather than left open because
the endpoint is attractive enough on its own terms that it will look like an
oversight to a later reader, and because two things follow from the decision that
shape the rest of the document.

The first is that **search-then-read is the only path**, so the quality of
`Web.fetch` (section 4) now carries work the endpoint would otherwise have done.
Converting a page to clean text is no longer a convenience next to a better
option; it is the option.

The second is that **the free plan's ceilings are permanent**: one request per
second, two thousand a month, shared with every other client using the key. Two
thousand is roughly sixty-six a day, which for searches that are already
described as infrequent is unlikely to bind — but there is no upgrade path
behind it any more, so the exhausted-month error in section 3 is a case that can
really happen rather than a formality. If it starts happening, the answer that
costs no subscription is a self-hosted SearXNG instance on the home network,
which has no key and no quota, put behind the same `Web.search` tool as a second
backend. That is a fallback to reach for on evidence, not something to build now.

### 2.3 What kvit-coder already has that these tools would reuse

- **A permission tier that already covers fetching.** `curl` and `wget` are
  `ask` rules with a stated reason (`internal/permissions/builtin.go:85`), so
  reaching the network is a decision the user already makes once and grants for
  the session, the project, or everywhere. The web tools do **not** route
  through it, and the reason is a distinction worth stating: the shell asks per
  command because the shell can run anything, so which command matters. These
  two can only search and fetch, so enabling one in the config is the decision,
  the way it is for every other tool in the program. Making `Web.fetch` the one
  tool that also asks per call would buy nothing.
- **Spill-to-temp-file for large tool output**, via `TempFileManager`, and the
  truncation caps the Read tool uses — 150 lines or 24 KB before the rest goes
  to a file whose path comes back in the result.
- **`SelfTimeoutTool`** (`internal/tools/tool.go:68`), which lets a tool opt out
  of the loop's blanket 15-second timeout and manage its own deadline. A page
  fetch needs this; `ObserveWaitTool` and `QuestionTool` already use it.
- **`ParallelSafeTool`** and the `Batch` tool, so a read-only tool marked safe
  can run several at a time in one model call.
- **`golang.org/x/net` is already in `go.mod`** as an indirect dependency at
  v0.33.0, so `golang.org/x/net/html` costs a promotion to the direct require
  block rather than a new module — though section 4 raises the version and, with
  it, the project's Go floor. The project builds without cgo and stays that way.
- **The MCP client**, with per-server trust policy, call timeout, tool filter,
  and deterministic tool ordering for prompt caching.

## 3. Search: build it in the agent

**Recommendation: a native `Web.search` tool, not an MCP server.**

`review.md:428` prefers MCP servers over new code, and for browser control
(section 5) that is right. It is wrong here for three reasons. The Brave client
is already written and is 300 lines of Go by the same author, so "new code" is
mostly a port rather than a build. A published Brave MCP server is a Node or
Python process that must be spawned, handshaked, and killed on every single turn
under the per-turn process model, costing between half a second and two seconds
each time (`redesign-mcp.md:3.1`) for something that is one HTTPS request. And
the retry and refusal handling below is where the care goes, which is easier to
get right, test and change in this repository than in somebody else's server.

### Tool surface

```
Web.search {query, count, freshness, country}
              → [{title, url, description, age}]
```

One tool, against the Brave Web Search API, working on the key as it stands.
There is no second search tool: `llm/context` needed a paid plan and section 2.2
declines it, so the model searches and then reads what it chose, and the prompt
section should say so plainly — a result description is there to pick a page
with, not to answer from.

### Details that matter

- **Strip the HTML from descriptions.** Brave marks matched terms with
  `<strong>`. Left in, they are noise in the context and the model may echo them.
- **Do not count the quota locally.** The llama-swap-plus client reads a count
  from a file at construction and writes it back after each search, which is
  right for one long-lived process that holds the key alone and wrong here twice
  over: one process per turn means the count reloads constantly, and the key is
  shared with clients on other machines that this one cannot see. Read
  `x-ratelimit-remaining` from the response instead. It is account-wide, so it
  already includes every other consumer.
- **Handle the 429 and stop there.** Wait the seconds named by the first field of
  `x-ratelimit-reset` — one, in practice — add a little jitter so several agents
  that collided do not collide again on the retry, and give up after four
  attempts. Four rather than two because a refusal costs no quota, so the only
  price of another attempt is about a second of wall clock, whereas giving up
  hands the model a failed tool result that costs a full reasoning pass to
  interpret; at `reasoning_effort: xhigh` that is thousands of tokens and tens of
  seconds against four seconds of waiting. Four attempts also fit inside the
  20-second deadline this tool sets for itself, and the count is a config value
  rather than a constant, for the reason the configuration block below gives. No
  lock file, no shared pacer, no proxy in front of the key. Collisions need sparse searches across a few
  well-behaved clients to stay rare, which is the situation, and coordination
  machinery for a rare collision is machinery that then has to be maintained.
- **Tell the two refusals apart.** A collision in the one-second window clears in
  about a second and is worth retrying; a month with nothing left in it will not
  clear for weeks, so retrying it only wastes the turn. The second field of
  `remaining` separates them. Say which one happened in the tool result, so the
  model stops rather than trying again in a different shape.
- **Surface the monthly remaining in the result.** It costs nothing, it is the
  only honest view anyone has of a budget shared across machines, and it is what
  turns "are we near the limit" from a guess into something visible.
- **Both tools are `ParallelSafe`,** so `Batch` can run several searches at once.
  The per-second limit will serialize them anyway, which is fine — it costs wall
  clock rather than a round trip.
- **`SelfTimeout` with a 20-second deadline.** Brave answers in under a second in
  practice, but the loop's 15-second blanket timeout is the wrong thing to be
  bound by.

### Configuration

Everything with a number in it above is a config value, not a constant. The whole
tree, following the conventions the rest of `config.yaml` already uses — a
group toggle per tool, and the key read from an environment variable rather than
written into the file:

```yaml
tools:
  web:
    api_key_env: "BRAVE_API_KEY"     # the same convention llm.api_key_env
                                     # already uses, and it matters more here:
                                     # the key is shared with clients on other
                                     # machines, and config.yaml is checked in
    base_url: "https://api.search.brave.com"
    usage_log: "~/.kvit-coder/web/usage.jsonl"   # what was searched and when;
                                                 # a record, not a counter

    search:
      enabled: true
      count: 5                  # results per query
      max_attempts: 4           # tries before giving up on a 429
      timeout: 20               # seconds for one request, retries included

    fetch:
      enabled: true
      timeout: 30               # a page can be slow in a way an API is not
      max_bytes: 5242880        # refuse to download more than this
      user_agent: "kvit-coder/<version> (+https://github.com/kvit-s/kvit-coder)"
```

`max_attempts` is the one most worth having as a setting rather than a constant.
Four is right for one request per second, where a collision clears in about a
second and retrying quietly beats handing the model a failure to reason about.
What would make it wrong is the rate limit changing under it — a faster plan, or
a second backend such as the self-hosted instance section 2.2 names as the
fallback, where collisions stop happening and four attempts only delays a real
failure. Neither is planned, and that is the point: the number depends on a fact
about the far side that this program does not control, so it belongs in the file
you edit rather than in the binary you rebuild.

The same argument applies more weakly to `timeout` and `count`, which are
settings because every other tool in this program has its limits in the config
file and there is no reason for these two to be the exception.

## 4. Retrieval: a small native tool, and know where it stops

**Recommendation: a native `Web.fetch` tool that converts HTML to text, plus a
prompt section telling the model to prefer plain-text URLs where they exist.**

Shell `curl` is already possible and is not sufficient. It returns raw HTML,
which is the worst thing to put into a context window: a 400 KB page of scripts,
navigation and tracking markup wrapping 3 KB of prose. Worse, the shell tool
spills output that large to a temp file, so what the model actually receives is a
path, and it then spends a reasoning pass working out how to filter it. The
conversion is the entire value of the tool.

### Tool surface

```
Web.fetch {url, max_bytes}
    → {url, final_url, status, title, lines, path, content}
                                       # small page: the whole thing
    → {url, final_url, status, title, lines, path, head, outline}
                                       # large page: where to look instead
```

The whole conversion always goes to a file under the session's `tmp/`, keyed by
a hash of the URL so fetching the same page twice in one session costs nothing
the second time. What comes back in the message depends on size, under the same
caps the Read tool uses — 150 lines or 24 KB.

### The conversion

Use `github.com/JohannesKaufmann/html-to-markdown/v2`, and split the job in two:
prune the page yourself, then let the library generate the markdown.

Parse with `golang.org/x/net/html` and drop the `script`, `style`, `nav`,
`header`, `footer`, `form` and `svg` subtrees. Then hand the pruned tree to
`htmltomarkdown.ConvertNode(*html.Node)`, which the library exposes alongside its
string and reader entry points. That split puts each half where it belongs: which
parts of a page are content is a judgement that changes per site and wants to sit
in code you can adjust, whereas turning a cleaned tree into well-formed markdown
is tedious, well-specified work that somebody has already done properly.

**What the library gets right that matters most here.** Fenced code blocks
survive, including multi-line ones, and its table plugin handles alignment,
rowspan and colspan. Converting `go.dev/blog/context` — a code-heavy page — turns
50,758 bytes of HTML into 20,385 bytes of markdown with all fifteen code blocks
intact and correctly fenced. A documentation page whose samples were reflowed into
prose is worse than one that failed outright, because it looks like it worked, and
that failure is exactly what a hand-written converter takes several iterations to
stop doing.

**What it costs**, measured rather than assumed:

- Two new modules, `html-to-markdown/v2` (MIT, actively maintained, v2.5.2 at the
  time of writing) and the author's small `dom` helper.
- `golang.org/x/net` moves from the v0.33.0 already in the tree to v0.55.0.
- **The `go` directive has to go from 1.24.0 to 1.25.0**, which is the only part
  worth pausing on. It is x/net v0.55.0 that requires it rather than the converter
  itself. The toolchain on this machine is already 1.25.4, so nothing breaks
  today, but `CLAUDE.md` records the project as Go 1.24 and that line needs
  changing with it.
- Nothing pulls in cgo, so the project keeps building without it.

The 40% ratio above is for the whole document including navigation and footer,
which is why the pruning pass is not optional: without it the tool spends most of
its output budget on site furniture.

### When the page is too large

A converted page is often larger than a tool result should be — the `go.dev`
example above is 20 KB — so `Web.fetch` should behave the way Search and Shell
already do rather than inventing anything: write the whole thing to disk, and
return enough for the model to decide what part it wants.

**The disk half needs no new code.** Spilled tool output already goes to the
session's `tmp/` (`TempFileManager`, wired at `cmd/kvit-coder/main.go:457`), and
the session directory is added to `allowed_paths` at turn start
(`cmd/kvit-coder/main.go:425`), so the existing `Read` tool opens a spilled file
by path with no path-safety prompt and `Search` greps it. There is no need for a
`Web.fetch.range` or any other retrieval tool: the model reads a range of the
file with the same tool it reads source with, and `Read`'s own offset and limit
already do the paging.

**The message half should be an outline rather than the first 150 lines.**
Truncating at the top gives the model the navigation and the introduction and
then stops, which is the least useful slice of a long page. A heading outline
with line numbers, plus the opening few lines, tells it both what the page
contains and where each part starts:

```
Web.fetch {"url": "https://go.dev/blog/context"}

→ 50,758 bytes of HTML → 1,182 lines of markdown
  <session>/tmp/fetch-3f9a2c.md

     1  # Go Concurrency Patterns: Context
    18  ## Introduction
    47  ## Context
    52  ### The Context type
   128  ### Derived contexts
   301  ## Example: Google Web Search
   ...
```

**This is nearly free here, which it would not be elsewhere.** Appendix A.1 of
`redesign.md` proposes exactly this shape for source files — a skeleton with line
numbers, then targeted reads — and defers it because it needs per-language
parsing to be worth much. A web page needs none: the heading levels are in the
HTML as `<h1>`–`<h6>`, the converter emits them as markdown headings, and
building the outline is counting the `#` prefixes in output you already have.
The argument that appendix makes for the idea applies unchanged, and it is the
better argument now that a 1M window makes tokens the lesser concern: a skeleton
plus two targeted reads leaves less irrelevant material competing for the model's
attention than 20 KB of page does.

Two details that decide whether it works. The line numbers in the outline must be
the line numbers of the file on disk, so a `Read` at the offset the outline gave
lands where the model expected. And a page small enough to return whole should be
returned whole, with no outline at all — making the model take two steps to read
three kilobytes is the cost this is supposed to avoid.

### Where it stops, said plainly

`Web.fetch` retrieves static HTML. A page that renders its content with
JavaScript returns an empty shell, and a site behind a bot check returns the
challenge page. The tool should detect both cheaply — a successful fetch whose
converted text is under a few hundred characters, or a response that is a known
challenge — and say so in the result rather than returning an empty string that
the model interprets as "the page has nothing on it." That message is what tells
the model to reach for the browser instead, so it is worth getting right.

Much of that problem disappears without a browser, and the prompt section should
say how: prefer `raw.githubusercontent.com` over the GitHub file view, `pkg.go.dev`
over a rendered doc site, and an `llms.txt` or `.txt` variant where one is
published. These are the pages a coding agent wants most often, and they are all
served as plain text by something that will never need a browser.

One line on posture, since it will come up: this tool fetches a page a person
asked about, one at a time, which is not crawling. A truthful `User-Agent`
naming the tool, and a short delay between requests to the same host, is the
whole of what is owed.

## 5. Browser control: Stagehand, and what to use instead

### What Stagehand actually is

Stagehand is a Browserbase library with SDKs in TypeScript, Python and Go. It
drives a browser over the Chrome DevTools Protocol and offers three
natural-language primitives — `act()` to perform an action described in a
sentence, `extract()` to pull structured data from a page, and `observe()` to
list what on the page can be acted on — plus an `agent()` layer above them.

Two facts about it are commonly got wrong and both matter here. It **does** run
fully locally: `localBrowser.launch()` starts a Chrome on your machine and
`localBrowser.connect({cdpUrl})` attaches to one already running, and neither
needs a Browserbase account or key. But `act`, `extract` and `observe` are
**model calls** — they work by sending the page to an LLM and asking it what to
click — so running Stagehand means running a second agent, with its own API key,
its own token spend, and its own opinion about what the page says.

### Why that is the wrong layer for this program

The second model is buying a capability kvit-coder already has. Muse Spark 1.3
with a 1M-token context can read an accessibility tree and decide what to click
perfectly well; what it cannot do is send the click. The thing missing is the
mechanical layer — navigate, snapshot, click, type, screenshot, wait — not the
natural-language layer built on top of it.

**Playwright MCP** (Microsoft) and **Chrome DevTools MCP** (Google) expose
exactly that mechanical layer over MCP, need no API key and no model of their
own, and run against a local Chromium. Either one plugs into the MCP client that
already exists, with no new code in this repository at all. That is where
`review.md:428`'s "prefer MCP servers over new code" applies, and it applies
strongly.

Stagehand becomes interesting at a different point: when a single fuzzy
multi-step task ("log in and download last month's invoice") would otherwise cost
fifteen snapshot-and-click round trips through the primitive layer, and paying a
second model to do it in one call is cheaper in wall clock than paying the
primary model fifteen reasoning passes. That is a real case, and it is not the
case a coding agent hits first. Configure the primitive server, use it for a
week, and let the round-trip count decide.

### The WSL boundary

This machine runs under WSL, and the repository already records one place where
that boundary bites (`docs/images.md`, on clipboard access living on the Windows
side). A browser server has the same shape of problem with a simpler answer:
install a headless Chromium inside WSL and let the MCP server launch it there.
Driving the Windows-side Chrome over CDP instead means exposing a debugging port
across the WSL network boundary, which is more setup and a wider opening than the
task needs.

## 6. The MCP daemon, and what the browser case adds to it

### The proposal is already written and the trigger has arrived

`redesign-mcp.md` proposes a separate long-running executable that holds the MCP
connections, exposes each configured server as a local HTTP MCP endpoint, and
forwards to the real connection it holds. The agent still starts and exits once
per turn and connects to the daemon rather than to the servers, so
`internal/mcp/` needs no changes: it already speaks HTTP, already namespaces
tools per server, already applies a per-server trust policy and timeout. That is
precisely what was asked for — a separate executable that drives MCP servers for
multiple agents and stays up across the turn lifecycle — so the answer to "should
it be built this way" is yes, and the document to implement is that one.

Its section 9 defers the work until "the first server you actually want that
keeps state between calls — a browser driver being the likeliest." Adding a
browser server is that trigger. Its stated prerequisite, stage 6 of
`redesign.md` — detached processes with `setsid`, pidfiles, and a reconcile pass
at turn start — is done and shipping (`tools.procs.enabled: true` in
`config.yaml`, `internal/tools/procs_tools.go`), so the daemon reuses all three
rather than reinventing them.

### What the browser case changes: one browser, many tabs, and a budget

`redesign-mcp.md:5.2` names the daemon by a hash of the workspace path and the
resolved MCP configuration. Two agents working in the same checkout therefore
share one daemon and, through it, one upstream connection to each server. For a
stateless server that is connection pooling and is what you want. For a browser
it means two agents driving the same pages, which produces confusing failures
rather than clean ones.

The obvious fix — give each agent session its own browser — is affordable only
if you do not price it.

#### What a browser costs, measured

Headless Chromium from the Playwright cache on this machine, RSS across the
whole process tree:

| | RSS | processes |
|---|---:|---:|
| browser running, no page open | 416 MB | 6 |
| one page open | 565 MB | 7 |
| three pages | 844 MB | 9 |
| five pages open at once | 1,221 MB | 11 |
| one tab, seven cross-origin navigations | 495–607 MB | 6 |

Each **simultaneously open** page is its own renderer process costing 130–220 MB;
the browser itself is a fixed 416 MB however many it has. Navigating one tab is
different in kind: seven cross-origin navigations left the process count at six
throughout, with resident memory tracking whichever page was currently loaded —
607 MB on github.com, back to 503 MB on a lighter one — so the cost of browsing
is bounded by the heaviest single page rather than by how many were visited. Set against `agents-ram.md`, where a
whole kvit-coder session — interface and agent together — is 27 MB, one browser
with one page open costs about twenty-one sessions.

That settles the shape. Fifty sessions of kvit-coder is 1.4 GB. Fifty browsers
with a page each is 28 GB on a 32 GB machine that is also running several other
agents and an inference server. Even fifty isolated contexts inside one shared
browser is 416 MB plus fifty renderers, around 8.5 GB, so sharing the browser
process helps by a factor of three and still does not make the problem go away.
**A fixed pool of slots, with a session refused when none is free, is what bounds
memory** — and refusing at the door is what makes the lease below worth stating,
because a guarantee that a later arrival can revoke is not one the model can plan
against.

#### What the per-turn respawn actually costs, measured

Configuring the server made two numbers available that the argument for the
daemon had to guess at before.

**Bringing the server up costs 583 ms per turn** (574, 581 and 593 ms over three
runs: `npx`, node start, the `initialize` handshake and `tools/list` for 24
tools). That is the bottom of the half-second-to-two-seconds range
`redesign-mcp.md:3.1` estimated, and that section's own conclusion applies
unchanged: half a second at the front of an instruction is noticeable and is not,
on its own, worth a second program to avoid. Against a turn whose model calls run
to tens of seconds at `reasoning_effort: xhigh`, it is a few percent.

**Most of the state loss can be bought back without a daemon at all.** Pointing
the server at `--user-data-dir=/tmp/kvit-coder-browser/${KVIT_RUN_ID}` gives each
session its own profile directory, and Chrome persists cookies and local storage
into it. The process still dies at the turn boundary, but the login does not.
What is actually lost per turn is the open tabs and the live DOM, which costs a
re-navigation.

That weakens the case in `redesign-mcp.md:3.2` considerably, because the items on
its list were led by "a server that holds an authenticated session
re-authenticates, which in the case of an interactive sign-in means it cannot be
used." With a persistent profile that one is gone. What remains for the daemon is
the 583 ms, the re-navigation, and the supervision and per-server log from its
section 4.3 — a real list, and a shorter one than the document was written
against. The honest reading is that the daemon should now be argued for on
evidence from use rather than from this section.

#### The server already does the isolation

Playwright MCP handles multiple concurrent clients itself: `--isolated` gives
each connecting client its own in-memory context, and `--shared-browser-context`
has them reuse one. It also exposes `browser_storage_state` and
`browser_set_storage_state`, and accepts `--storage-state` to load cookies and
local storage from a file at startup.

So the daemon should run **one** server and let each agent session connect to it
as its own HTTP client, rather than spawning a browser per session as an earlier
draft of this section proposed. The per-session upstream connection stays in the
design only for a stateful server that has no client isolation of its own, which
is what the `isolation` field below selects.

Sharing should also go further than isolation-per-session by default. The use
case section 5 describes is reading a page that needs JavaScript, which wants no
cookie isolation at all, and where a login is involved one shared login is
usually what you want rather than fifty. What genuinely must not be shared is
**which page a session is driving**. One shared context with a page per session
gives that, at one renderer each and one login for everybody.

```yaml
mcp:
  servers:
    - name: playwright
      transport: stdio
      command: npx
      args: ["-y", "@playwright/mcp@latest", "--shared-browser-context"]
      isolation: shared       # shared (default) | session
      slots: 6                # tabs, and so sessions, that may browse at once
      lease: 10m              # idle time a slot is held for, refreshed on use
      max_hold: 1h            # ceiling on one session holding a slot
      idle_browser: 5m        # with no tab open: checkpoint all, then shut down
      checkpoint_tool: browser_storage_state       # what to call before dropping
      restore_tool: browser_set_storage_state      # and what to call on return
```

#### Admission, not eviction

A slot is **one tab**, which is also one session, because browsing is
navigation rather than tab-opening. The measurement above is what makes that
work: a session can visit fifty pages through one tab and never cost more than
one renderer, so there is nothing to meter and no allowance to track. A session
is admitted once and holds its tab for the life of its lease.

There is a reason beyond memory to make one tab the default, and it is worth
saying in the prompt rather than only enforcing. **The model's context is its tab
history.** A person keeps ten tabs open because human memory is poor; a model
with a 1M-token window has already read the first page into the conversation and
does not need the browser to hold it while it reads the second. A second tab is
warranted only for a page that must stay live — a sign-in flow, something with an
action in progress — and not for one that was merely read. A session that does
need two takes two slots, which keeps the accounting to counting renderers and
makes the rare case visible rather than free.

The lease is an *idle* timer refreshed by every browser call, so a session that
is actively browsing never loses its slot, and only one that stopped does. That
makes `max_hold` necessary as well: with renewal on use, a session that browses
steadily for hours would hold a slot against everyone else, and an hour's ceiling
bounds it without much cost now that losing a slot means a checkpoint rather than
a loss.

A refused session should be told what it needs to decide with — how many slots
are in use, and when the earliest lease expires — and should be able to wait
deliberately rather than only to fail. That is the shape `Observe.wait` already
has, so the model waits when it has nothing else to do and gets on with other
work when it has.

At six slots the budget is 416 MB of browser plus six renderers at their
observed worst, a little over 1.5 GB. Six concurrent browsing sessions out of
fifty is comfortable rather than tight: the number of sessions that browse at all
is small, and the number doing it inside the same ten minutes is smaller. The
value of fixing it at six is not the number but that going over it is a refusal
somebody can read rather than a machine that starts swapping.

#### Reclaiming in two steps

The question of whether to persist browser state or to promise it for ten
minutes has the same answer for both, because they address different halves of
the problem and the memory is not where you would guess. A context holds cookies
and local storage, which is kilobytes. A **page** holds the 160 MB. So the
cheapest thing to reclaim is not a session's browser but its idle tabs:

1. **Close the idle tab when the lease expires.** Reclaims about 160 MB and
   costs the session nothing but a re-navigation, because its cookies and its
   login are in the context, which survives. This is the answer to "a session
   used the browser a little and no longer needs it": its tabs close, and what
   remains is free. Note the trigger — the lease running out, not another session
   wanting the memory, which is the distinction that keeps the guarantee true.
2. **Checkpoint every context and shut the browser down** once it has held no
   tab at all for `idle_browser`. Call the server's `browser_storage_state` for
   each live context, write the JSON and the URLs that were open into the
   corresponding session folder, then stop the process. This is the only step
   that reclaims the 416 MB base, and contexts cannot outlive the browser
   anyway, so checkpointing and shutdown are one event rather than two.

On the next browser call from any session, the daemon starts the browser again
and restores that session's context with `browser_set_storage_state`, reopening
the URL it was on if it needs to be there.

An abandoned session needs no special handling: it is idle by definition and
walks down the same ladder without anybody deciding it was abandoned.

**Be aggressive about step two, because restarting is nearly free.** Measured on
this machine, a headless Chromium goes from launch to answering on its debugging
port in 65–99 milliseconds. The two to four seconds in the same runs was loading
the first page, which is paid on any navigation whether the browser was already
running or not. So an idle browser is holding 416 MB to save about a tenth of a
second, and `idle_browser` belongs in single-digit minutes rather than the tens
of minutes an expensive restart would justify.

That inverts the intuition the daemon is built on, and the inversion is worth
stating because it decides what the daemon is actually for. What is expensive
about a browser is not starting it — it is the state inside it, which is why a
session's cookies go to disk rather than being thrown away, and why the lease
exists to spare an *active* session the re-navigation. The process itself is
cheap enough to discard the moment nobody is using it.

#### What survives a checkpoint, and telling the model so

Browser state divides cleanly, and the division is what makes the checkpoint
worth building rather than merely plausible. Cookies, local storage and the URL
of each open tab serialize, and `browser_storage_state` is exactly the tool for
it. The live DOM after JavaScript has run, an open WebSocket, text typed into a
form but not submitted, and the JavaScript heap do not serialize at all.

For the workload section 5 describes, essentially all the value is in the first
group — being logged in, and being on a particular page. So a checkpoint keeps
what matters and loses what rarely does, and the tool result **must say which**,
because a model told only "the browser restarted" will assume either too much or
too little.

That is where the lease belongs too, and stating it explicitly is the point
rather than a nicety: a model that is told "your pages are held for at least ten
idle minutes, after which they may be closed and you will have to navigate
again, though logins survive" can finish a browsing task before starting a long
edit. A model told nothing will interleave them and be surprised.

The two mechanisms are not alternatives. **The lease is the fast path and the
checkpoint is the floor.** Without the lease every turn pays a browser launch
and several page loads, which is the whole reason `redesign-mcp.md` wants a
daemon. Without the checkpoint, exceeding the lease loses a login rather than
costing a re-navigation. Neither alone is sufficient, and together they cost one
tool call at eviction and a JSON file in the session directory.

This whole subsection is worth writing into `redesign-mcp.md` as an amendment to
its section 5.2, whether or not it is implemented soon: its per-workspace keying
is right for stateless servers and quietly wrong for a browser, and the failure
is invisible until two agents run at once.

### The approval-memory fix comes first and is independent

`redesign-mcp.md:3.4` notes that MCP call approvals live in a map in the
manager's memory (`internal/mcp/confirm.go:34`), so under one process per turn
the `ask_once` policy becomes ask-once-per-turn. Nothing exercises that today,
because no MCP servers are configured. A browser server will exercise it
immediately and painfully: every `browser_click` in a new turn asks again.

The fix is small and the pattern to copy already exists —
`internal/permissions/store.go` persists session grants to the session directory,
with the four scopes and the grantor already written. Point the MCP confirmer at
the same store. This is worth doing before any server is configured, and it does
not depend on the daemon.

## 7. Order of work

Each step leaves a working program, and the early ones do not depend on the
later ones. Steps 1 to 3 are built; the rest are not.

1. **`Web.fetch`** — done. Native, static HTML to structured text, spilling
   to the session `tmp/` and capped like Read. This is the single most useful
   piece and it depends on nothing.
2. **`Web.search`** — done. Port the request shaping and the JSONL usage
   log from llama-swap-plus's `internal/brave/client.go`; leave its rate limiter
   and quota counter behind, since both assume a single long-lived process that
   holds the key alone. Read the limits from the response headers, bound the
   retry, strip HTML from descriptions.
3. **Prompt sections for both** — done. Prefer plain-text URLs where they
   exist; search returns descriptions rather than content, so fetch what you
   choose; `Web.fetch` sees static HTML only and will say when a page needed a
   browser.
4. **Move MCP approval memory into the session directory** — done. Approvals go
   to `<session>/mcp-approvals.json`, so `ask_once` means once per session
   rather than once per turn.
5. **Configure a browser MCP server** — done. Playwright MCP over stdio against
   a headless Chromium inside WSL, 24 tools, with a per-session profile
   directory so the respawn costs a re-navigation rather than a login. Use it as
   is and see what the respawn actually costs in practice; the measurement above
   says the daemon is no longer the obvious next step.
6. **Build the daemon** per `redesign-mcp.md`, with the page budget and the
   three-step reclaim from section 6 above. The isolation itself comes from the
   server rather than the daemon, so what the daemon owes is the cap, the idle
   timers, and the checkpoint call before it drops a context.
7. **Stagehand, only if step 5 shows the primitive layer is too chatty**, and
   then as an MCP server wrapping it rather than as a library linked into this
   program.

## 8. Open questions

- **How often does a page actually need a browser?** The guess is that raw and
  plain-text URLs cover most of what a coding agent reads, and that the browser
  is for the occasional dashboard or JavaScript-rendered reference. If the guess
  is wrong, steps 5 and 6 move up the list.
- **Is the one-per-second limit a real constraint in practice?** The judgement it
  rests on is that searches are sparse and the clients sharing the key are
  well-behaved, so two landing in the same second is rare. The bounded retry
  covers the rare case. If the 429s turn out to be common, the answer is a second
  backend without a per-second limit rather than coordination between machines
  that cannot see each other.
- **Does two thousand a month bind?** Sixty-six a day across every client sharing
  the key, with no paid tier behind it. The guess is that it does not come close.
  What settles it is the monthly remaining figure the tool already reports, read
  after a month of use.
- **How many slots, and how often is one refused?** Six tabs is about 1.5 GB, a defensible share of this machine while several other agents
  run. It is arithmetic rather than evidence until something is observed hitting
  it, and the number worth watching is not the cap but the refusal rate: if
  nothing is ever refused the pool is larger than it needs to be, and if refusals
  are common the lease is too long rather than the pool too small.
- **Is a shared cookie jar ever the wrong default?** One login for every session
  is what you want for a documentation site and plainly wrong if two sessions
  need to be different users of the same service. The `isolation: session` escape
  covers it; whether anything ever asks for that escape is unknown.
- **Does the pruning list need to be per-site?** Dropping `nav`, `header` and
  `footer` is a blunt rule that will keep boilerplate on some pages and cut
  content on others. A measured output ratio per fetch is the cheapest way to
  notice, since a page converting at 40% is mostly furniture and one at 5% may
  have lost its content.
- **Should `Web.fetch` follow cross-host redirects?** Following silently means a
  URL the model chose is not the page it read. Returning the redirect and letting
  the model call again costs a round trip. Returning the content along with the
  final URL, and naming the redirect in the result, is probably right.
- **What does the daemon do when a stateful server's lane wedges?** A browser
  stuck on a modal dialog stays stuck, which `redesign-mcp.md:7` already names as
  the cost of holding state. Per-lane restart (`kvit-coder mcp restart
  playwright/s/my-feature`) is the narrow answer and is worth having before the
  first time it happens.

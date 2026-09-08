# Model selection: multiple models in config + `:mN` / `:eN` in the UI

Proposal, not yet built. Goal, in the requester's words:

> specify multiple models in `config.yaml`, use `:h` to see the list of
> available models, select them with `:m1`, `:m2`, etc., and select effort
> with `:e1` through `:e4` (or whatever is available for a model).
> For muse-spark the effort spec example can be seen in the settings for
> `~/krok`.

This doc records what exists today on both sides (kvit-coder and krok) and
then describes the smallest implementation that satisfies the request
without breaking the one-process-per-turn architecture or the current
single-model config.

## 1. Where kvit-coder stands today

### 1.1 Single model in `config.yaml`

`config.yaml` has exactly one model (`config.yaml:6-30`):

```yaml
llm:
  base_url: "https://opencode.ai/zen/go/v1"
  api_key_env: "OPENCODE_API_KEY"
  model: "muse-spark-1.3-contributor"
  api_backend: "responses"
  reasoning_effort: "xhigh"   # minimal | low | medium | high | xhigh
  headers:
    - "x-opencode-session=kvit-coder-${KVIT_RUN_ID}"
  context: 1048576
  merge_thinking: true
```

There is no list, no per-model effort menu, no display name. The commented-out
block below it (`config.yaml:32-36`) is the old local default
(`ministral-3-14b` on `llama-swap-plus`), kept as a comment — evidence that
"switching models" today means editing the file by hand and restarting.

### 1.2 `internal/config/config.go` — one flat `LLM` struct

`Config.LLM` (`internal/config/config.go:20-59`) is a flat struct:
`BaseURL`, `APIKey`, `APIKeyEnv`, `Model`, `APIBackend`, `Headers`,
`ReasoningEffort`, `ReasoningSummary`, `RequestTimeout`, `Temperature`,
`MaxTokens`, `Context`, `MergeThinking`, `Verbose`, `GenerationStats`,
`BenchmarkCmd`. `ReasoningEffort` is a free string; nothing validates it
against the model, and there is no notion of "which efforts does this model
offer".

Two helpers matter:

- `ModelDisplay()` (`config.go:695-703`) returns `model:effort` when
  `reasoning_effort` is set, plain `model` otherwise. It feeds the startup
  banner (`cmd/kvit-coder/main.go:606,629,666,700`,
  `internal/tui/ui.go:94`), `:config` (`internal/tui/ui.go:305`,
  `internal/agent/metacommands.go:27`), and has a test
  (`config_test.go:531-544`).
- `LLMHeaders()` (`config.go:727-741`) expands `${VAR}` in `llm.headers`,
  including `${KVIT_RUN_ID}` (see §1.5).

### 1.3 Client construction — once per turn, from `cfg.LLM`

`cmd/kvit-coder/main.go:343-354` builds the client on every turn:

```go
llmClient := llm.NewClient(cfg.LLM.BaseURL, cfg.LLM.APIKey,
    llm.WithBackend(cfg.LLM.APIBackend),
    llm.WithHeaders(cfg.LLMHeaders()),
    llm.WithReasoningEffort(cfg.LLM.ReasoningEffort),
    llm.WithReasoningSummary(cfg.ReasoningSummaryOrDefault()),
    ...)
```

Overrides are only `--model` and `--base-url` (`main.go:241-246`). There is
no `--effort` / `--reasoning-effort` flag. On the wire,
`buildResponsesRequest` (`internal/llm/responses.go:122-141`) sends
`reasoning: {effort, summary}` verbatim whenever either string is non-empty,
and omits the object otherwise. The `chat_completions` path ignores effort
entirely. So "no effort" today is encoded as the empty string, which is
exactly what a non-reasoning (local chat-completions) model needs.

### 1.4 Two command handlers, only one of them live in the UI

There are two colon-command implementations and they are not the same:

| Handler | File | Who calls it |
|---|---|---|
| `UI.handleCommand` | `internal/tui/ui.go:214-347` | `kvit-coder-ui`, the interactive front end. Commands: `:quit/:q`, `:help/:h`, `:new`, `:resume/:last`, `:switch`, `:sessions`, `:history`, `:clear`, `:config`, `:image`, `:paste`. |
| `agent.HandleMetaCommand` | `internal/agent/metacommands.go` | Legacy REPL path (`:quit/:q`, `:config`, `:tools`, `:system`, `:clear`, `:history`, `:help/:h`). Exec mode (`-p`) does not go through it. |

New model commands belong in `UI.handleCommand`. Mirroring them in
`HandleMetaCommand` is optional; note it in the change so the two helps do
not drift. The current `switch` is an exact-match `switch parts[0]`, so
`:m1`/`:e2` need a prefix branch before `default` (see §3.4).

`:h` **is already taken**: it is the alias of `:help` in both handlers
(`ui.go:225`, `metacommands.go:73`). The request "use `:h` to see the list"
therefore cannot mean "replace help". The recommendation in §3.4 is: `:h`
stays help, and help grows a models+efforts table — one keystroke still shows
the list, and nothing breaks.

### 1.5 Turn lifetime shapes the design

One OS process per turn (`CLAUDE.md`, `internal/session/session.go:17-30`).
`kvit-coder-ui` does **not** link the agent: `UI.runAgent` (`ui.go:372-443`)
spawns `kvit-coder -p <prompt> [-s <session>] [-config …] [-yolo]`
per turn, with `KVIT_CODER_FROM_UI=1` in the env. Config is loaded once at UI
startup (`cmd/kvit-coder-ui/main.go:93`) and handed to `tui.New` as `*config.Config`;
a config edit mid-session requires a UI restart (same as today).

Consequences:

- The selection (which model, which effort) must live in the long-lived
  process — the `UI` struct — and travel to the short-lived one per turn as
  argv (flags) or env. It must not live only in the agent's memory.
- `KVIT_RUN_ID` is pinned per session (`tui/ui.go:67-76`,
  `cmd/kvit-coder/main.go:394-399`, `session.RunIDFor`). Keep reusing it
  across model switches: the `x-opencode-session` header is a routing hint,
  and the prompt cache is keyed server-side per model anyway. Do not mint a
  new run ID on switch.
- Each turn already records `KindSettings{model, merge_thinking, tools}`
  (`session.go:57-59,85-92,328-340`, written from `repl.go:91`). A session
  that switches models mid-conversation stays interpretable for free, as long
  as the recorded model string includes the effort (it does via
  `ModelDisplay`, but see §3.6 about recording the resolved per-model
  endpoint too).

## 2. What krok does (reference)

All paths below are in `~/krok` (the `krok` binary) and `~/.krok/config.toml`
on this machine. kvit-coder needs only a fraction of this machinery; it is
recorded here so the mapping is explicit.

### 2.1 Effort spec in `~/.krok/config.toml`

krok's config (TOML) separates the catalog from the default:

```toml
[models]
default = "muse-spark-go"
default_reasoning_effort = "xhigh"
session_summary = "muse-spark-go"     # krok-only role overrides; NOT needed
image_description = "muse-spark-go"   #   for kvit-coder — ignore them
web_search = "muse-spark-go"

[model.muse-spark-go]
model = "muse-spark-1.3-contributor"  # wire id sent as `model:`
name = "Muse Spark 1.3 Go"            # display name
base_url = "https://opencode.ai/zen/go/v1"
api_backend = "responses"
env_key = "OPENCODE_API_KEY"
context_window = 1048576

[[model.muse-spark-go.reasoning_efforts]]
value = "minimal"
label = "Minimal"

[[model.muse-spark-go.reasoning_efforts]]
value = "low"
label = "Low"

[[model.muse-spark-go.reasoning_efforts]]
value = "medium"
label = "Medium"

[[model.muse-spark-go.reasoning_efforts]]
value = "high"
label = "High"
default = true

[[model.muse-spark-go.reasoning_efforts]]
value = "xhigh"
label = "Extra high"
```

The live file on this machine (`~/.krok/config.toml`) has three entries:
`muse-spark-go` (default, `…/zen/go/v1`), `muse-spark-zen`
(`muse-spark-1.3-contributor-free` on `…/zen/v1`), and `"qwen3.8-27b"`
(`qwen3.8-27b-nvfp4-dflash2` on `http://192.168.138.186:8090/v1`,
`api_backend = "chat_completions"`, `api_key = "none"`, no
`reasoning_efforts` at all). That last entry is the shape of a
non-reasoning model: no effort menu, effort must clear on switch.

Semantics worth copying:

- Each entry carries its **own endpoint** (`model` wire id, `base_url`,
  `api_backend`, key, `context_window`), not just a name. Switching models
  can change backend and URL — the muse-spark-go vs zen vs local-qwen split
  proves the effort menu alone is insufficient.
- `reasoning_efforts` is an **ordered list of `{value, label, default}`**.
  Order is presentation order. `default` marks the entry's effort default;
  the global `default_reasoning_effort` is the fallback when an entry marks
  none.
- Absent `reasoning_efforts` (qwen) means no menu. krok gates on
  `supportsReasoningEffort` meta before resolving any token.

### 2.2 Runtime: `ModelState` + shared resolvers

krok's runtime (`crates/codegen/xai-grok-pager/src/acp/model_state.rs`) is:

```rust
pub struct ModelState {
    pub available: IndexMap<ModelId, ModelInfo>,  // catalog, insertion order
    pub current: Option<ModelId>,
    pub reasoning_effort: Option<ReasoningEffort>,
}
```

Key methods and their kvit-coder analogues:

- `set_current(model_id, effort_override)` — sets current; effort becomes
  the override, else the catalog default parsed from meta
  (`parse_reasoning_effort_meta`). kvit-coder needs the same two-step:
  `:mN` resets effort to the entry default; `:eN` overrides after.
- `reasoning_effort_options_for(id)` — server list when the model supports
  effort, else `legacy_effort_options()` (`xhigh, high, medium, low` in
  `slash/commands/effort_levels.rs`), else empty for unsupported models.
  kvit-coder simplification: the config file **is** the server list, so
  "no list + responses backend" can fall back to the legacy four, while "no
  list + chat_completions" means no menu. No ACP catalog round-trip needed.
- `resolve_effort_token_for(id, token)` — accepts a menu option id
  (case-insensitive) **or** a canonical value, but only if that canonical
  value appears in the model's menu. Levels the model does not offer are
  rejected with the offered list (`UnknownToken { token, offered }`).
  Copy this gate: it is what makes `:e9` print "use one of: …" instead of
  sending a blocked effort to the API.
- `resolve_effort_for_model(id, token)` — support-flag gate first, then
  token resolution; one error enum (`Unsupported` / `UnknownToken` /
  `NoActiveModel`) shared by `/effort`, CLI deferred switch, and headless.
  kvit-coder should likewise share one resolver between the TUI `:eN`
  path and the headless `--effort` flag path.
- `resolve_by_name_or_id` — case-insensitive display-name-or-id match.
  Useful if kvit-coder later accepts `:m <name>` alongside `:mN`.
- Switching to a non-reasoning model **clears** `reasoning_effort`
  (`dispatch/tests/task_result.rs:1186`: "reasoning_effort must be cleared
  when switching to a non-reasoning model"). Re-dispatching the same model
  id is idempotent (no reset). Copy both rules.

### 2.3 Canonical effort values and wire mapping

`crates/codegen/xai-grok-sampling-types/src/types.rs:750-91`:

```rust
pub enum ReasoningEffort { None, Minimal, Low, Medium, High, Xhigh, Max }
// serde lowercase; FromStr accepts none|minimal|low|medium|high|xhigh|max
```

- Responses API: `to_responses_api()` passes the value through verbatim
  (all seven, including `none`/`minimal`/`max`).
- Anthropic Messages API: `to_messages_api()` returns `None` for
  `None`/`Minimal` (omitted). kvit-coder has no Messages backend, so the
  only rule to copy is: empty effort string = omit `reasoning: {}` from the
  `/responses` body (already the behavior in `responses.go:137-139`).
- CLI accepts `--reasoning-effort`/`--effort` aliases, last flag wins, plus
  remapped ids (`max`, `deep` in `app/cli.rs:1467-1501` tests). kvit-coder
  needs only one flag name; accept the seven canonical values and, if cheap,
  the `deep`→`xhigh`-family alias krok documents — but keep the config's
  per-model menu as the authority on what is *selectable*.

### 2.4 UX: `/model`, `/effort`, status line

- `/model <name> [effort]` (alias `/m`): model phase lists one row per
  catalog model with `(current)` marker; picking a reasoning model chains
  into the effort sub-menu; a trailing effort token makes a session-scoped
  switch, a bare name sets the persisted default (`set_default_model_inner`
  emits both `Effect::SwitchModel` and `Effect::PersistSetting`).
- `/effort <level>`: same shared resolver, session-scoped.
- Status line / prompt header renders `model + effort`
  (`app/agent_view/render.rs:2669`, `app_view.rs:4529`), and the TUI marks
  the active effort row `(active)` in the dropdown
  (`effort_levels.rs:build_effort_arg_items`).
- kvit-coder's equivalent under this proposal: `:h` help shows the model
  table with `*` on the active row (same marker style as `:sessions`),
  `:mN` switches, `:eN` sets effort, banner/` :config` show
  `model:effort`. No fuzzy picker, no autocomplete chaining — numeric
  indices are the whole interaction.

## 3. Implementation for kvit-coder

### 3.1 Config schema: add a `models:` list, keep `llm:` as fallback

Add a top-level `models:` list whose entries embed everything `llm:` has
today that is endpoint-specific. Keep `llm:` untouched so old configs load
unchanged; when `models:` is absent, synthesize a one-entry list from `llm:`.

```yaml
# New section. Order is stable: :m1 is the first entry, :m2 the second.
# YAML order is preserved (slice, not map) — cf. CLAUDE.md: prompt-prefix
# cache rule; do NOT sort or range over a map here.
models:
  - id: "spark-go"                        # short handle for :mN display + --model
    name: "Muse Spark 1.3 Go"             # display name (banner, :h table)
    model: "muse-spark-1.3-contributor"   # wire id
    base_url: "https://opencode.ai/zen/go/v1"
    api_backend: "responses"
    api_key_env: "OPENCODE_API_KEY"
    context: 1048576
    # Ordered effort menu for :eN. `default: true` marks the entry default.
    # Omit the whole list for a non-reasoning model (local qwen): then no
    # :eN menu exists and switching to it clears the effort.
    efforts:
      - value: "minimal"
      - value: "low"
      - value: "medium"
      - value: "high"
        default: true
      - value: "xhigh"
  - id: "spark-zen"
    name: "Muse Spark 1.3 Zen Free"
    model: "muse-spark-1.3-contributor-free"
    base_url: "https://opencode.ai/zen/v1"
    api_backend: "responses"
    api_key_env: "OPENCODE_API_KEY"
    context: 1048576
    efforts:
      - value: "minimal"
      - value: "low"
      - value: "medium"
      - value: "high"
        default: true
      - value: "xhigh"
  - id: "qwen-local"
    name: "Qwen3.8-27B (local)"
    model: "qwen3.8-27b-nvfp4-dflash2"
    base_url: "http://192.168.138.186:8090/v1"
    api_backend: "chat_completions"
    # no efforts: non-reasoning model

llm:
  # …unchanged; used when `models:` is absent, and as the source of
  # non-endpoint settings (headers, timeouts, merge_thinking, …) unless
  # overridden per entry. Document which wins: per-entry wins for the
  # fields it sets.
  model: "muse-spark-1.3-contributor"
  reasoning_effort: "xhigh"
```

Design notes:

- Field-for-field this mirrors krok's `[model.*]` + `reasoning_efforts`
  (`value`/`label`/`default`), minus krok-only role overrides
  (`session_summary`, `image_description`, `web_search`,
  `fork_secondary_model`) and minus ACP meta plumbing. `label` can be
  omitted when it equals the value; add it only if the table wants prettier
  text.
- Go shape (`internal/config/config.go`): a `ModelEntry` struct with the
  endpoint fields plus `Efforts []EffortOption{Value, Label, Default}`,
  and `Models []ModelEntry` on `Config`. Methods:
  `ActiveModel() *ModelEntry` (or `ResolveModel(ref string)` accepting
  1-based index, `id`, `name`, or wire `model`),
  `EffortOptions(entry) []EffortOption` (entry list, else legacy
  `[xhigh,high,medium,low]` for `responses` without a list, else empty),
  `DefaultEffort(entry) string` (entry `default:true`, else
  `llm.reasoning_effort`, else `models.default_effort`, else `""`).
  Keep `ModelDisplay()` working on the *resolved* entry so banner/`:config`
 /`KindSettings` need no other change.
- Validation at `Load`: unknown `api_backend`, unknown effort value
  (accept the seven canonical krok values), duplicate `id`, index-0 default
  selection (first entry unless `models.default` / `llm.model` says
  otherwise). Reject with the file path and entry number, not a bare yaml
  error.
- Backward compatibility is a hard requirement: `go test ./...` has
  `config_test.go` expectations for single-model configs; the synthesized
  one-entry list must produce byte-identical `ModelDisplay()` for them.

### 3.2 Runtime selection state (UI owns it, turns borrow it)

- `tui.UI` gains `models []config.ModelEntry` (snapshot from `cfg` at
  `New`), `currentModel int` (index into it), `currentEffort string`
  (override; `""` = entry default). `:mN` sets `currentModel` and resets
  `currentEffort` to that entry's default (clearing to `""` when the entry
  has no menu — the krok "cleared on non-reasoning" rule). `:eN` sets
  `currentEffort` within the current entry's menu only.
- `runAgent` (`ui.go:372-443`) appends the selection to every spawn:
  `--model <wire-id> --effort <effort>` (plus `--base-url`,
  `--api-backend` as needed — or a single `--model-index`/`--model-id`
  the agent resolves against the same config file; prefer explicit
  wire fields so a skewed UI/agent config pair still sends the right
  endpoint). Empty effort appends `--effort ""` / omits the flag, which the
  agent maps to "clear" (omit `reasoning:`), never to "keep previous".
- Agent side (`cmd/kvit-coder/main.go:241-246,343-354`): add the `--effort`
  flag, apply overrides to `cfg` right after `Load` (before `NewClient`,
  before `sess.Settings`/`meta.Model` in `repl.go:82-93`), resolving the
  full entry (base_url, backend, headers, context) rather than just the
  name. Headless `kvit-coder -p … --model qwen-local --effort ""` must work
  without the UI — it shares the §2.2 resolver, only the error sink differs
  (stderr + non-zero exit vs TUI line).
- Optional persistence: write `currentModel`/`currentEffort` into
  `meta.json` alongside `meta.Model` on each turn, and have the UI prefer
  it when reopening a session. This makes "reopen session lands on the same
  model" true without rewriting `config.yaml`. Do not rewrite the config
  file on switch — surprising, racy with hand edits, and unnecessary.

### 3.3 Prompt-cache and history hazards on switch

- Keep `KVIT_RUN_ID` (hence `x-opencode-session`) stable across switches
  within a session. The cache is per model server-side; a new ID buys
  nothing and loses the shared prefix.
- **Strip model-specific replay state on switch.** The Responses path
  replays `Message.ReasoningBlocks` (encrypted thinking) and
  `ToolCallItemIDs` verbatim (`internal/llm/responses.go`,
  `types.go:24-27`). Those bytes belong to the model that produced them;
  sending muse-spark-go's blocks to qwen (or zen-free) is at best waste,
  at worst a 400. Rule: when the turn's model differs from the model that
  produced the tail of `history.jsonl`, drop `ReasoningBlocks`/`ToolCallItemIDs`
  at the `repl.go` load boundary (keep the readable text — `Content` and
  merged thinking stay). This is the one place where "switching models
  mid-session" touches the agent rather than just the UI. Leave
  `KindSettings` history alone — per-turn model records are the audit trail.
- `context:` display and any context-window accounting follow the *active*
  entry, not `llm.context`.

### 3.4 TUI commands: `:h`, `:mN`, `:eN`

`:h` conflict resolution (explicit): `:h`/`:help` keeps meaning help.
Fulfilling "`:h` shows the list" means help **contains** the list. New
parsing in `UI.handleCommand`, before `default`:

| Input | Effect |
|---|---|
| `:h`, `:help` | Help as today **plus** a models table and an efforts table for the current model (see sketch). One keystroke still answers "what can I pick". |
| `:mN` (`:m1`…) | Switch to models[N-1]. `N` out of range → `unknown model :mN; use one of :m1-:mK`. Resets effort to the new entry's default (or clears when it has no menu). Prints `Switched to <name> (<wire-id>:<effort or "no effort">) @ <base_url>`. |
| `:eN` (`:e1`…) | Set effort to the Nth entry of the **current** model's menu, in config order. No menu → `current model does not support reasoning effort`. Out of range → `unknown effort level ':eN'; use one of: :e1 …`. Prints `Effort set to <value> for <model>`. |
| `:e <value>` (optional, cheap) | Same resolver by canonical value (`minimal|low|medium|high|xhigh`, plus krok's `max`/`none` passthrough) for scripts and for effort counts that exceed `:e9`. Validates against the current menu — rejects values the model does not offer with the offered list. |
| `:config` | As today plus `Models:` list with `*` on the active row (copy `:sessions` marker style) and the active entry's endpoint. |

Sketch of the `:h` models section:

```
Models (:mN to switch):
  :m1 * Muse Spark 1.3 Go (muse-spark-1.3-contributor) :xhigh @ https://opencode.ai/zen/go/v1
  :m2   Muse Spark 1.3 Zen Free (muse-spark-1.3-contributor-free) @ https://opencode.ai/zen/v1
  :m3   Qwen3.8-27B (local) (qwen3.8-27b-nvfp4-dflash2) @ http://192.168.138.186:8090/v1
Effort for Muse Spark 1.3 Go (:eN to set):
  :e1   minimal
  :e2   low
  :e3   medium
  :e4   high (default)
  :e5 * xhigh (active)
```

Implementation notes:

- Parse with `strings.HasPrefix(cmd, "m")` / `"e"` + `strconv.Atoi`,
  after the exact-match cases, guarding existing commands (`:model` does
  not exist today, so no collision; if a future `:memory` lands, match
  `m`+digits only). `metacommands.go` gets the same branch or an explicit
  "model switching lives in kvit-coder-ui" notice — decide and document;
  do not silently diverge the two helps.
- Numbering is 1-based, stable, config order. Never sort entries or menu
  rows (prompt-cache determinism per `CLAUDE.md`; also krok's `IndexMap`
  preserves insertion order for the same reason).
- Startup banner (`ui.go:93-103`) shows the resolved
  `ModelDisplay() @ BaseURL` as today — now reflecting the selection — so a
  `:m2` is visible on the next prompt without asking.
- Tests: `handleCommand` is currently untested and I/O-bound (`fmt.Println`
  directly). At minimum add resolver unit tests in `internal/config`
  (`ResolveModel` by index/id/name/wire-id, effort menu + default + gate,
  unknown-token error text listing offered ids) and a `Load` backward-compat
  test (old `config.yaml` → one synthesized entry, identical display).
  Refactor `handleCommand` to return strings (or take an `io.Writer`) if the
  diff allows; otherwise cover the parsing via the extracted
  `parseModelCommand(input) (kind, n, err)` helper.

### 3.5 Files to touch (checklist)

1. `internal/config/config.go` — `ModelEntry`, `EffortOption`,
   `Config.Models`, resolution + validation + legacy synthesis;
   `ModelDisplay` on resolved entry. Plus `config_test.go`.
2. `config.yaml` — example `models:` list (the three entries above, pointing
   at the real endpoints/ids from `~/.krok/config.toml`).
3. `cmd/kvit-coder/main.go` — `--effort` (and, if resolving by id,
   `--model` semantics extended to accept index/id/name/wire-id);
   apply-after-`Load` override block; client uses resolved entry.
4. `internal/tui/ui.go` — `UI` selection state, `:mN`/`:eN` branches,
   `:h`/`:config` tables, banner line, `runAgent` flag passing.
5. `internal/repl/repl.go` — record resolved `model:effort` in
   `meta.Model`/`KindSettings`; strip `ReasoningBlocks`/`ToolCallItemIDs`
   when the turn's model differs from history's tail producer.
6. `internal/agent/metacommands.go` — mirror or explicitly defer (see §3.4).
7. `docs/README.md` — index this file alongside the other proposals.

### 3.6 What is deliberately out of scope

- No fuzzy picker / autocomplete chaining (krok's `/model` dropdown) — `:mN`
  is the picker.
- No per-role models (krok's `session_summary`/`image_description`/
  `web_search` overrides) and no `fork_secondary_model`.
- No persisting the default back to `config.yaml` (`PersistSetting`
  equivalent) in v1; session `meta.json` restore is the persistence.
- No Messages-API effort mapping (`to_messages_api` dropping
  `none`/`minimal`) — no such backend exists here.
- No server-side model catalog: the YAML list is the catalog. If a
  `/responses`-only endpoint later needs discovery, that is a separate doc.

## 4. Acceptance

- `scripts/build.sh`, `go test ./...`, `go vet ./...` green.
- Old `config.yaml` (only `llm:`) behaves exactly as today.
- In `kvit-coder-ui`: `:h` shows models + efforts; `:m2` switches endpoint
  (banner and `:config` confirm); `:e1`..`:eN` follow the current model's
  menu; `:e9` / `:m9` print the offered list, not a stack trace;
  switching to the local qwen entry clears effort and hits
  `chat_completions`; mid-session switch does not replay stale
  `ReasoningBlocks`.
- Headless parity: `kvit-coder -p … --model <id> --effort <level>` runs the
  same resolver and fails with the same offered-list error.

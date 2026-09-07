# Evaluation Plan — Concrete Execution

Companion to `eval-design.md`. The design lays out the methodology; this plan pins
down **which corpora, which agent, which cases, which commands**. If you sit down
at the keyboard, this is the doc that tells you what to do next.

> **Architecture (verified against source).** Three facts that shape everything below:
> 1. **Slots & chunk sizing are automatic.** The rig is **2 × 100 k slots**. cag-client
>    sizes chunks to **90 % of per-slot context** (`MaxTokensPerSlot * 9/10` ≈ **90 k**
>    here), read live from `/v1/status` (`CtxSize / Parallel`). See §1.
> 2. **The agent reaches CAG over MCP.** cag-mcp ships an MCP stdio server
>    (`cmd/cag-client`) exposing `cag_query` / `cag_status`; kvit-coder (which has an
>    MCP client) registers it and calls `mcp.cag.cag_query`, with no code changes.
>    See §2.
> 3. **All CAG knobs live in cag-client (env), forwarded by kvit-coder.** A profile is
>    one `mcp.servers[]` block; kvit-coder forwards `command/args/env/cwd`. The repo
>    work is small env-knob + response-tail changes in cag-mcp. See §2.1, §6.2.

---

## 0. Definitions and paths

| Symbol | Resolves to | Notes |
|---|---|---|
| `CAG_REPO` | `D:\projects\cag-mcp` | this repo |
| `WSL_HOME` | `\\wsl.localhost\ubuntu\home\sk` | source of Tier‑2 corpora and of the agent |
| `KVIT` | `WSL_HOME\kvit-coder` | the agent binary lives here (built inside WSL) |
| `MSN1` | `WSL_HOME\msn1` | primary Tier‑2 corpus |
| `CAG_SERVER` | `http://localhost:8400` (Windows) / `http://192.168.8.20:8400` (from WSL) | the HTTP server (`cmd/cag-server`). From inside WSL use the host's LAN IP, **not** `localhost`. |
| `CAG_CLIENT` | `cmd/cag-client` | the **CAG MCP stdio server** — kvit-coder spawns this; it exposes `cag_query` / `cag_status` and talks HTTP to `CAG_SERVER`. |
| `RIG` | RTX 5080, **2 × 100 k slots** | `cag-server` started with `ctx_size 200000`, `parallel 2` → `MaxTokensPerSlot = 100 k`; chunks auto-cap at **90 k**. |
| `EVAL_ROOT` | `D:\projects\cag-mcp\eval\` | NEW — everything below is created under here |

All eval artifacts (cases, profiles, scorers, runner, results) live under
`EVAL_ROOT`. **No code goes into `experiments/`** — that directory is frozen for
the historical experiment scripts referenced in `report.md`.

---

## 1. Corpora (concrete picks)

`eval-design.md` defined three tiers. Concrete selections:

### Tier 3 — Private regression (already exists)

- `experiments/data/corpus_docs1.txt` (50 k tokens, 11 md files)
- `experiments/data/corpus_docs2.txt` (60 k tokens, 91 mixed files)

**Use it for:** validating that the eval harness reproduces the qualitative
findings of experiments 8–11 in `report.md` (cross-file trace, abstention,
multi-turn coherence at 110 k combined). If the new harness ranks Mode-3 CAG
above naive grep on docs1/docs2, the scorers are working.

### Tier 2 — Contamination‑control (the load‑bearing signal)

**Primary: `msn1`** — `\\wsl.localhost\ubuntu\home\sk\msn1`

- 313 `.cs` files under `src/` (≈ 2.3 MB), 62 `.md` files at root, plus
  `config/`, `templates/`, `examples/`, `tests/`.
- `src/core/` is a multi-project C# solution: `Microstrata.AgentManager`,
  `Microstrata.Blocks`, `Microstrata.EventBus`, `Microstrata.EventInterpreter`,
  `Microstrata.Llm`, `Microstrata.ProcessManager`, `Microstrata.TaskManager`,
  `Microstrata.ContextInjection`, `Microstrata.Buckets`, `Microstrata.Core` —
  lots of cross-component dependency chains, perfect for trace cases.
- Private. The backend model (`gpt-oss-20b`) has not seen it. ✓
- Has a `CLAUDE.md` and many `*-plan.md` / `*-progress.md` docs — gives natural
  documentation‑vs‑code cross‑reference cases.

**Token budget & chunking — automatic, not hand-sized.** Rough estimate:
313 `.cs` × ~7 KB avg = ~2.3 MB ≈ **~575 k tokens** for code alone. There is **no
need to hand-pick a subset to "fit a slot"**: on the first query, cag-client scans
the working directory, builds the import graph, and auto-chunks at **90 % of
per-slot context** — `MaxTokensPerSlot * 9/10` ≈ **90 k** on the 2 × 100 k rig (see
`internal/client/mcp.go`). ~575 k of code therefore auto-splits into **~6–7 chunks**,
which the scheduler fans across both slots — exactly the multi-chunk / multi-slot
path the KV-cache design was built for.

Consequences for corpus definition:

- **Default Tier-2 corpus = the whole `msn1` repo** (project id `msn1`). No
  hand-curated subset: cag-client auto-chunks the full repo to fit the slots, so
  there's nothing to hand-pick. Point it at `~/msn1`; cases are authored against the
  full repo.
- **Project selection is by working directory** (`project_id = sha256(cwd)`, or an
  explicit `CAG_PROJECT_ID` override — §2.1). Two logical "variants" at the **same
  path** collide on project id. If a smaller single-chunk corpus is wanted (e.g. a
  cheap calibration target), make it a **separate directory** (a pruned copy), not a
  logical subset of `~/msn1`.
- The 90 k cap leaves ~10 k of the 100 k slot for the question + reasoning + answer.
  That headroom is **fixed by the chunker regardless of think budget**, so it's
  comfortable at `reasoning_effort: low` / `think_budget: 100` (the planned profiles)
  but would tighten if a later sweep raises reasoning effort sharply.

A small script (§3.2) still measures real token counts before authoring, mainly to
predict chunk count and per-chunk descriptions.

**Secondary: pick one more from `WSL_HOME`** for variety. Candidates already
on disk (private code, mixed languages):

| Repo | Language guess | Why it's useful |
|---|---|---|
| `kvit-blog` | TS/JS web app | different language family from C# |
| `kvit-chat` | TS/Node | event‑driven, frontend+backend |
| `allocator` | Python | small, sharp domain logic |
| `montage` | Python | ML‑adjacent, very different shape |
| `chat` | unknown | small, easy to scope |

**Pick `allocator` as the second Tier‑2 corpus** unless first inspection shows
it's trivial (< 50 files). It's small enough to author cases for in an afternoon
and gives Python coverage. Fall back to `kvit-chat` if `allocator` is too thin.
Decision is made by `eval/scripts/inspect-repo.ps1` (§3.2) before authoring.

### Tier 1 — Labeled anchor (deferred)

`CodeRAG-Bench` or `SWE-bench Verified` file localization. Not built until Mode 3
on Tier 2 is producing stable signal. Sequencing in §8.

---

## 2. Agent: kvit-coder

The exploration agent in Mode 3 is **`kvit-coder`** (in `WSL_HOME\kvit-coder`),
running headless via its `-p` flag and persistent `-s <session>` flag (both verified
in `cmd/kvit-coder/main.go`: `-p` = "exec mode … exit after completion", `-s` =
"continue existing session or create new one"). Reasons: the user owns it, it's been
tested, the system prompt is fully configurable via `-agent-file`, and its
OpenAI‑compatible LLM client makes the backend a clean swappable axis. **It now also
has an MCP client** (`internal/mcp`, `internal/mcpclient`), which is how it reaches
CAG — see §2.1.

### 2.1 CAG over MCP

kvit-coder now has an MCP client, and cag-mcp ships an MCP stdio server
(`cmd/cag-client`) that exposes **`cag_query`** and **`cag_status`** and talks HTTP
to `CAG_SERVER`. So we **register cag-mcp as an MCP server**; the agent calls
**`mcp.cag.cag_query`** and kvit-coder needs no code changes. Three contract details
follow from this, each absorbed below.

**The real tool contract** (verified in `internal/client/mcp.go`):

```
tool:      mcp.cag.cag_query
argument:  { "question": string (required) }     # the ONLY argument
result:    prose text (mcp.NewToolResultText) — NOT structured metadata.
           Format: per-chunk "[chunk_id: description]\n<answer>" blocks,
           a trailing "[No relevant info from: …]" line if any chunk abstained,
           and a final "[CAG: N chunks answered, Mms]" summary line.
           All-IDK → "[CAG] No relevant information found in any chunk. [N chunks queried, Mms]".
```

What this means for the plan:

- **Project selection = working directory.** `project_id = sha256(cwd)`; cag-client
  auto-scans cwd, auto-chunks, and auto-registers on first query. kvit-coder runs
  with `workspace.root` = the corpus root, so the corpus is selected by *where the
  agent runs*. An explicit **`CAG_PROJECT_ID`** override (§6.2) is added so same-path
  variants and chunk-strategy sweeps don't collide on the cwd hash.
- **No structured metadata over the wire.** The scorers' per-call timings/tokens
  (§4.4) are **not** emitted today — only prose plus a parseable `[CAG: N chunks,
  Mms]` tail. The fix is to **enrich cag-mcp's response** (a structured tail +
  per-chunk tokens) in *this* repo (§6.2) — which serves both MCP and HTTP — rather
  than a native kvit-coder tool. `cag_action` is derivable client-side (the runner
  knows whether a file change was pending).
- **Freshness is built in.** Every `cag_query` runs `detectUpdates()`: content
  changes are sent as incremental updates with the query, and a >15 % structural
  change triggers a full re-register automatically. The runner no longer drives a
  manual re-register for `agent-cag` (see §3.4, §5.2).

**Configuration lives in cag-client, forwarded by kvit-coder.** kvit-coder's
`MCPServerConfig` already forwards `command` / `args` / `env` / `cwd` to the stdio
subprocess (env supports `${VAR}` expansion; `cwd` defaults to the workspace root),
so **kvit-coder needs zero code changes** — every CAG knob is an env entry on the
server block. A profile is therefore one `mcp.servers[]` block:

```yaml
mcp:
  servers:
    - name: cag
      enabled: true
      transport: stdio
      command: /home/sk/cag-mcp/bin/cag-client    # linux build (kvit-coder runs in WSL)
      cwd: /home/sk/msn1                           # corpus root → project selection
      env:
        - CAG_SERVER=http://192.168.8.20:8400      # CAG HTTP server (LAN IP from WSL)
        - CAG_PROJECT_ID=msn1                       # NEW knob — explicit project id
        - CAG_THINK_BUDGET=100                      # NEW knob — overrides hardcoded 100
      tools:
        allow: [cag_query]                          # isolation: hide cag_status
```

The runner generates this `config.yaml` per profile/rollout and passes it via
kvit-coder's `-config` flag. Per-profile knobs are env (process-scoped = the eval's
grain); anything genuinely per-*call* would have to become a `cag_query` argument (a
cag-mcp change) — none of S1–S3 need that.

**Sanity‑check CLI (small, separate).** For authoring cases (§3.2 step 8) and for
hand‑debugging the server outside the agent loop, also add a small standalone CLI in
*this* repo: `cmd/cag-query/main.go` — ~40 LOC, parallels `cmd/cag-server` and
`cmd/cag-client`, reuses `internal/client/cagclient.go`. Note the real API:
`ProjectQuery` takes a single `protocol.ProjectQueryRequest{ProjectID, Question,
ThinkBudget}` (no context arg), and answers come back as `resp.Answers[i].Answer`
(not `.Text`). It is **not** part of the agent path — pure convenience for the human
running the eval.

### 2.2 Three agent configurations to compare

| ID | Tools enabled in kvit-coder | Agent‑file system prompt | Purpose |
|---|---|---|---|
| `agent-cag` | `mcp.cag.cag_query`, `plan` | `eval/prompts/agent-cag.md` — "use cag_query as primary retrieval; cite files; abstain if unknown" | the system under test |
| `agent-grep` | `read`, `search`, `shell` (no MCP) | `eval/prompts/agent-grep.md` — "use ripgrep + read to explore" | naive subagent baseline |
| `agent-grep-eng` | same as `agent-grep` | `eval/prompts/agent-grep-engineered.md` — careful multi‑step exploration recipe | engineered subagent baseline (fairness control per design doc) |

The `agent-cag` config intentionally **disables** `read`, `search`, and `shell` (via
kvit-coder's own tool config) and exposes **only `cag_query`** from the CAG server
(via the MCP server block's `tools.allow: [cag_query]`), so the eval measures CAG in
isolation. A later sweep (post step‑11) can enable both `cag_query` and the file
tools together to see whether they're additive.

Each is a kvit-coder run with a different `config-<id>.yaml` and `-agent-file`.

### 2.3 Backend LLM (the model kvit-coder talks to)

Two distinct LLM consumers in this eval, **kept separate**:

- **Exploration backend** — the model kvit-coder itself drives:
  **`qwen3.6-27b-mtp`**, served on the **same host as the CAG server, port 8080**
  (`http://192.168.8.20:8080/v1`). Strong reasoning, small enough for the local rig.
  It picks tools, formulates follow‑ups, writes final answers.
- **CAG backend** — the model behind the CAG server. Default: `gpt-oss-20b`
  (the model all `report.md` experiments used; what CAG was tuned for).

This separation is the design doc's "fix the exploration agent's LLM" rule.
Swap one at a time across sweeps; never both at once.

### 2.4 Sessions and turn cap

- One kvit-coder session per `(case, profile, rollout_seed)`. Session name:
  `eval-<case_id>-<profile_id>-<seed>`. Sessions are stored under
  `~/.kvit-coder/sessions/` (inside WSL) as JSONL. The runner copies them out
  to `EVAL_ROOT\results\<run_id>\sessions\` immediately after each rollout so
  WSL state stays the source of truth but Windows has the artifacts.
- Hard turn cap = `case.turn_budget` (default 8). Enforced by the runner:
  it inspects the session JSONL after each kvit-coder invocation and stops
  invoking when the agent has answered or when budget is reached.

### 2.5 Spine smoke test (run before building anything)

Every piece of the path — kvit-coder's MCP client, `cmd/cag-client`, the
`/v1/project-query` server, the llama.cpp slots — is confirmed in source but has
never run end to end. **Validate the spine before building scorers or a runner
around it.** Cost: one config file and one command, no new code.

**Prereqs.**

- `cag-server` up on the rig (the 2 × 100 k config from §7).
- `cag-client` built for Linux inside WSL (`go build -o bin/cag-client ./cmd/cag-client`
  from the repo at `/mnt/d/projects/cag-mcp` or a WSL clone).
- A minimal agent-file (e.g. "Answer using cag_query; cite files; say so if unknown.").

**Config** — a one-server kvit-coder `config.yaml` (the §2.1 block, trimmed):

```yaml
mcp:
  servers:
    - name: cag
      enabled: true
      command: /home/sk/cag-mcp/bin/cag-client
      cwd: /home/sk/msn1                       # → project = msn1
      env: [ CAG_SERVER=http://192.168.8.20:8400 ]
      tools: { allow: [cag_query] }
```

**Run.**

```bash
wsl -e bash -c "cd ~/msn1 && ~/kvit-coder/kvit-coder \
  -config /path/to/smoke.yaml -agent-file /path/to/agent-cag.md \
  --json -p 'How does EventBus routing work?'"
```

**Pass criteria.**

1. `mcp.cag.cag_query` is actually invoked (visible in `--json` output / session
   JSONL) — not answered from the model's own weights.
2. The answer cites real `msn1` files and is not all-IDK.
3. First query pays the auto-register cost (scan + chunk + KV prime); later queries
   are fast.

**Failure modes to check first if it breaks.**

- **WSL → rig networking:** from inside WSL, `CAG_SERVER` must be the host's LAN IP
  (`192.168.8.20`), not `localhost`.
- **Wrong cwd:** cag-client scans its working directory; if `cwd` isn't the repo root
  it chunks the wrong files (or none).
- **Slot / VRAM:** watch for silent t/s halving (VRAM exhaustion) as the ~6–7 chunks
  prime across the 2 slots.

If this passes, the architecture holds and step-1 build work is safe to start. If it
fails, the gap is here — cheaper to find now than after the harness is built.

---

## 3. Case authoring

### 3.1 Schema (extends eval-design.md §"Iterative case structure")

```yaml
case_id: msn1-eventbus-routing-01
corpus: msn1                               # must match a registered project (cwd-selected)
category: cross_file_trace                 # local_fact | cross_file_trace | synthesis | unanswerable | freshness
authored_by: sk
authored_at: 2026-05-23

seed_question: "How does an event published in one component reach a handler in another?"
topic: "EventBus subscription, dispatch, and cross-component routing"

target_understanding:
  - id: T1
    fact: "EventBus.Publish enqueues events into an internal channel"
    relevant_files: ["src/core/Microstrata.EventBus/EventBus.cs"]
    nugget_strings:                         # for objective matching (per design doc Q5)
      exact: ["EventBus.Publish"]           # must match verbatim
      normalized: ["enqueues events", "internal channel", "queue"]   # case/punct-insensitive substring
  - id: T2
    fact: "Subscribers register via Subscribe<TEvent>(handler)"
    relevant_files: ["src/core/Microstrata.EventBus/EventBus.cs"]
    nugget_strings:
      exact: ["Subscribe<", "Subscribe("]
  - id: T3
    fact: "Dispatch loop runs on a dedicated background task per subscriber group"
    relevant_files: ["src/core/Microstrata.EventBus/EventBusDispatcher.cs"]
    nugget_strings:
      normalized: ["background task", "dispatch loop"]

turn_budget: 8

# Mode 1 only — used by the fixed-anchor harness. Mode 3 ignores this.
anchor_sequence:
  - turn: 1
    question: "How does an event published in one component reach a handler in another?"
  - turn: 3
    question: "Where exactly is the dispatch loop and on what thread does it run?"
  - turn: 5
    question: "What happens if a handler throws — does it stop other handlers?"

# Optional naturalness flag (design doc §"Naturalness calibration")
natural: false                              # true => seed_question was taken from a real transcript
```

`nugget_strings` carries the matching strictness with the fact, not in the
scorer. This is the design doc's open question #5 made concrete: the case
author decides what's a numeric/identifier match vs a phrase match, because
they know the answer.

### 3.2 How cases get authored — step by step

The eval is only as good as its cases. The process below is the **load‑bearing**
part of this plan; treat it as a discipline, not a vibe.

1. **Inspect the corpus.** Run `eval/scripts/inspect-repo.ps1 <corpus_root>`
   (NEW, §6.1). It prints: file count by language, top 20 largest files, top 20
   most‑referenced symbols (via tree‑sitter or rough regex), estimated tokens
   for default chunk strategy. This anchors what's worth asking about.

2. **Pick a topic.** A topic is a *real understanding goal*, not a quiz. Good
   topics: "how is auth wired", "how does this propagate errors". Bad topics:
   "what is the value of MAX_RETRIES" (too local — make it a sub‑fact, not
   a topic).

3. **Read the source for that topic.** Actually read it. Take notes on every
   concrete claim — symbol names, file paths, numeric constants, sequencing.
   These are your candidate `target_understanding` entries.

4. **Write 3–7 facts.** Each one must be (a) verifiable against source, (b)
   string‑matchable. Numbers go in `exact`. Identifiers go in `exact`. Phrases
   go in `normalized`. A fact that needs an LLM judge to decide whether it's
   present is a bad fact — split it or drop it.

5. **Write the seed question.** Phrase it the way *you* would, not the way a
   benchmark template would. If the case is `natural: true`, paste it verbatim
   from a real Claude Code / kvit-coder transcript you have.

6. **Sketch the anchor sequence (Mode 1).** 2–4 follow‑ups that a competent
   exploration agent would plausibly ask. These are stand‑ins for Mode 3
   adaptive behavior used by the cheap comparable mode.

7. **Self‑review.** Could a contamination‑style answer (recall from weights)
   pass? If yes — strengthen `nugget_strings.exact` with project‑specific
   identifiers that the model couldn't guess.

8. **Sanity‑run.** Throw the seed question at `cag-query` (the standalone CLI from §2.1) once and read the
   answer alongside your facts. If the answer trivially covers everything,
   the case is too easy — add a deeper fact or a cross‑component fact.

### 3.3 Case set targets

| Corpus | Local fact | Cross‑file trace | Synthesis | Unanswerable | Freshness | Total |
|---|---|---|---|---|---|---|
| docs1/docs2 (Tier 3) | 1 | 1 | 1 | 1 | 1 | **5** |
| msn1 (Tier 2) | 3 | 4 | 3 | 2 | 1 | **13** |
| allocator (Tier 2) | 1 | 2 | 1 | 1 | 0 | **5** |
| **TOTAL initial** | | | | | | **23** |

Authoring effort: ~30 min/case for local‑fact and unanswerable, ~60–90 min for
cross‑file trace and synthesis. **Budget two working days for initial cases.**

Cases live in `eval/cases/<corpus>/<case_id>.yaml`. Versioned in this repo.

### 3.4 Unanswerable and freshness mechanics

- **Unanswerable cases.** The seed question asks about something *not* in the
  corpus. `target_understanding` is empty; the scoring criterion flips to
  abstention correctness (§4.3). Example for `msn1`: "How does the
  Redis‑backed cache eviction policy work?" (msn1 has no Redis.)
- **Freshness cases.** Author writes a small synthetic change (e.g. add a
  constant, rename a method), captures pre‑ and post‑state file hashes. The runner
  (§5.2) writes the change into the corpus working tree between turns N and N+1 and
  re‑queries. For `agent-cag` the next `cag_query` **auto-detects** the change
  (`detectUpdates()` sends incremental updates, or a full re-register on >15 %
  structural change) — no manual trigger. **Caveat:** this gives CAG a
  harness-adjacent update that `agent-grep` only gets by re-reading files itself;
  report freshness with that asymmetry called out. Scorer checks the post‑change
  answer reflects the new value. This exercises the overlay/full‑eval path of the
  server architecture.

---

## 4. Scoring (concrete implementations)

All scorers live in `eval/scorers/` as small Python modules with a uniform
signature: `score(case: Case, rollout: Rollout) -> ScoreRecord`. The runner
calls each scorer once per rollout and writes a flat JSON line per
`(case, profile, seed, scorer)` to `EVAL_ROOT\results\<run_id>\scores.jsonl`.

### 4.1 Nugget recall (per fact, then per turn, then per case)

```python
def fact_present(fact, answer_text):
    a = normalize(answer_text)   # lower, collapse whitespace, strip punctuation
    for s in fact.nugget_strings.exact:
        if s not in answer_text:           # case-sensitive
            return False                    # all exact strings must hit
    for s in fact.nugget_strings.normalized:
        if normalize(s) not in a:
            return False                    # all normalized strings must hit
    return True
```

- **Per‑turn nugget recall** = fraction of facts present in *that turn's* answer.
- **Per‑case cumulative coverage curve** = at turn N, fraction of facts present
  in the *union of answers* up through turn N. This is the design doc's
  "coverage curve" metric.

### 4.2 Citation validity

The current CAG MCP tool returns prose, not structured citations (design doc
open question #1). Two paths:

- **Path A (immediate):** parse `file:line` or bare `path/to/file.ext` patterns
  out of the answer with a regex; validate the file exists in the corpus
  snapshot (the runner copies the corpus snapshot into
  `EVAL_ROOT\results\<run_id>\corpus-snapshot\<corpus>\` at run start so paths
  are stable). Citation = valid if file exists AND any `nugget_strings.exact`
  for the matched fact appear in that file. Brittle but unblocks.
- **Path B (later):** change the CAG tool contract to emit a small JSON tail
  block: `<<<cag-cites>>>[{"file":"...","lines":"12-40"}]<<</cag-cites>>>`.
  Strip from the answer before display; parse for scoring. This is the
  "decide before building the citation scorer" choice flagged in the design.

Start with Path A. If the regex hit rate is < 60 % of answers that *should*
cite, switch to Path B. The runner records both raw answer and parsed
citations so re‑scoring under Path B is free.

### 4.3 Abstention correctness

Two checks per answer on an `unanswerable` case:

1. **Says it doesn't know.** Regex over a small phrase set:
   `["I don't know", "not found in the (codebase|corpus|repo)", "no (such|matching) (file|class|method)", "cannot determine", "no evidence"]`.
   At least one must match.
2. **Does NOT assert fake facts.** No occurrence of fabricated symbol names —
   build a negative nugget set per unanswerable case (e.g. "must NOT mention
   `RedisCache`") and verify absence.

Both must pass for full credit. Failing only #2 (hallucinated answer) is the
worst case and is reported separately.

### 4.4 Latency, tokens

**Reality check.** Over MCP the tool result is **prose**, and the HTTP response
(`protocol.ProjectQueryResponse` / `ChunkAnswerInfo`) carries only `total_ms` and
per-chunk `wall_ms` — **no token counts, no prompt/predict split**. So until the
response-tail enrichment lands (§6.2), the only cost signal is wall time, parsed from
the `[CAG: N chunks answered, Mms]` tail of the answer text plus kvit-coder's own
per-turn timing.

The **enrichment** (§6.2, in this repo) adds a machine-parseable tail to
`formatProjectQueryResult` and surfaces per-chunk tokens, giving the target shape
below for both MCP (parsed from the tail) and HTTP. `cag_action` is filled in by the
runner (it knows whether updates were pending), not the server.

```json
{"wall_ms": 2436, "server_total_ms": 2380, "all_idk": false,
 "cag_action": "query",
 "answers": [
   {"chunk_id": "chunk_0", "idk": false, "tokens_in": 110444, "tokens_out": 412,
    "prompt_ms": 56, "predicted_ms": 2380}
 ]}
```

Until enrichment, `tokens_in/out` and `prompt_ms/predicted_ms` are absent and the
scorer records only `wall_ms` / `server_total_ms`.

### 4.5 Synthesis quality (LLM judge — deferred until needed)

Synthesis cases (e.g. "explain how event routing works end to end") don't
collapse cleanly to nuggets. For those:

- **Judge model:** Claude Sonnet (via the Anthropic API). Different family
  from `gpt-oss-20b` and from `qwen3.6-27b-mtp` — addresses design doc open
  question #6.
- **Rubric:** 1–5 on each of {correctness, completeness, citation discipline,
  conciseness}. Average is the case score.
- **Validation:** before trusting, hand‑label ~20 % of synthesis answers and
  compute judge↔human agreement. If Spearman < 0.6, fall back to "nugget +
  judged completeness only".

Don't build this until nugget‑based scoring is producing stable rankings — the
design doc's "objective‑first" rule.

---

## 5. Runner: how a rollout actually executes

`eval/runner/run.py` (NEW). Single entry point for all modes.

### 5.1 Invocation

```bash
# From repo root:
python eval/runner/run.py \
    --cases eval/cases/msn1/*.yaml \
    --profiles eval/profiles/cag-baseline.yaml,eval/profiles/grep-baseline.yaml \
    --mode 3 \
    --rollouts 3 \
    --run-id "msn1-mode3-v01"
```

Writes everything to `EVAL_ROOT\results\msn1-mode3-v01\`.

### 5.2 Per‑rollout sequence

1. **Preflight.**
   - Verify CAG server is up: `GET $CAG_SERVER/v1/health`.
   - **Registration is automatic:** the first `cag_query` makes cag-client scan the
     corpus cwd, chunk, and register. To control timing (and keep the first scored
     turn fast), the runner may **pre-warm** by issuing one throwaway `cag_query`
     (or a direct `cmd/cag-query` call) before the rollout. Explicit
     `POST /v1/register` is only needed when pre-seeding a project id that differs
     from the cwd hash — i.e. when `CAG_PROJECT_ID` is set (see §6.2).
   - Snapshot the corpus to `EVAL_ROOT\results\<run_id>\corpus-snapshot\` (file
     copies, deterministic).

2. **Start the kvit-coder session.** Build the kvit-coder invocation string:
   ```bash
   wsl -e bash -c "cd ~/msn1 && \
       ~/kvit-coder/kvit-coder \
         -config /mnt/d/projects/cag-mcp/eval/profiles/<profile>/config.yaml \
         -agent-file /mnt/d/projects/cag-mcp/eval/prompts/<agent>.md \
         -s eval-<case>-<profile>-<seed> \
         -p '<seed_question>'"
   ```
   - Working directory is the corpus root (kvit-coder's `workspace.root`).
   - The first call carries the seed question.
   - `--json` flag enabled — structured turn output to stderr.

3. **Mode 1 (fixed anchor).** Walk the `anchor_sequence`; at each `turn: N` in
   the sequence, send the predetermined question. Between anchor turns, do
   nothing (Mode 1 does not let the agent free‑run).

4. **Mode 3 (agent‑adaptive).** After the initial `-p <seed>`, the runner
   waits for kvit-coder to exit. It then:
   - Reads the session JSONL to get the final assistant message.
   - Calls the **coverage scorer** on cumulative answers so far.
   - Asks the **exploration sub‑prompt** (`eval/prompts/explorer-next.md`,
     fed to a cheap LLM call — Haiku) for the next question, given:
     facts still uncovered, what's been asked, what's been answered. If the
     sub‑prompt returns `<<<STOP>>>` or coverage is 100 % or `turn_budget` is
     reached, stop.
   - Otherwise re‑invokes kvit-coder with `-s <same session>` and `-p
     <next-question>` so conversation state persists.
   - Records turn count, latencies, tokens per turn.

   *Note:* Mode 3's "next question" generator is itself a fixed component of
   the harness, not part of the system under test. Versioned in
   `eval/prompts/explorer-next.md`.

5. **Freshness cases.** If `case.freshness` is set, between turn N and N+1 the
   runner applies the synthetic edit (file write inside the corpus working tree).
   `agent-grep` detects it by re-reading; `agent-cag` detects it automatically on
   the next `cag_query` (`detectUpdates()`), so the runner does **not** trigger a
   manual re-register. The edit is reverted at end of rollout (§6.7).

6. **Scoring.** Once the rollout terminates, run all scorers; write
   `scores.jsonl`. Also copy the session JSONL out of WSL into
   `results/<run_id>/sessions/`.

7. **Cleanup.** Delete the kvit-coder session inside WSL (so re‑runs are
   clean). The JSONL is already copied out.

### 5.3 What gets recorded per rollout

`results/<run_id>/rollouts/<case_id>/<profile_id>/seed-<n>/`:

```
session.jsonl              # raw kvit-coder transcript
cag-tool-calls.jsonl       # every cag.query tool call: args / result text / metadata / wall
turns.json                 # normalized: [{turn, question, answer, tools, tokens, wall_ms}, …]
scores.json                # all scorer outputs for this rollout
freshness-diff.patch       # only for freshness cases
```

The aggregator (`eval/runner/aggregate.py`) reads `scores.jsonl` across all
rollouts and emits the two design‑doc tables: per‑turn scorer table and
per‑case cumulative table.

---

## 6. Engineering work to land the harness

Listed in dependency order. Roughly 1.5–2 dev‑weeks of work before the first
real signal.

### 6.1 Scaffolding (~½ day)

Create:
```
eval/
  cases/{msn1,docs,allocator}/
  profiles/                        # YAML configs (see §7)
  prompts/                         # agent-file system prompts, explorer-next prompt
  scorers/                         # nugget.py, citation.py, abstention.py, latency.py
  runner/                          # run.py, aggregate.py, sessions.py, wsl_bridge.py
  scripts/
    inspect-repo.ps1               # corpus inspection (§3.2 step 1)
    register-corpus.ps1            # POST /v1/register from a corpus root
    measure-tokens.ps1             # corpus → estimated token count, chunk plan
  results/                         # gitignored
```

`.gitignore`: `eval/results/`, `eval/cases/*/*.local.yaml`.

Also (lives outside `eval/`):

```
cmd/cag-query/main.go              # NEW: standalone sanity-check CLI (§2.1)
```

### 6.2 cag-client env knobs + response-tail enrichment + standalone CLI (~1 day)

All in *this* repo — **kvit-coder needs no changes** (it forwards `command`/`args`/
`env`/`cwd` to the MCP stdio subprocess already). Three pieces:

**A. cag-client env knobs (~½ day, incl. tests).** Make the knobs the eval sweeps
read from the environment instead of being hardcoded, so a profile = an
`mcp.servers[]` block (§2.1):

- `CAG_SERVER` — already read in `cmd/cag-client/main.go`. ✓
- **`CAG_PROJECT_ID`** — override `sha256(cwd)` in `internal/client/mcp.go`
  (`initialize()` computes the project id at ~L394; `detectUpdates`/`projectQuery`
  read `m.state.ProjectID`). Needed so same-path variants and chunk-strategy sweeps
  don't collide.
- **`CAG_THINK_BUDGET`** — replace the hardcoded `ThinkBudget: 100` (`mcp.go` ~L96)
  so think-budget sweeps are reachable through the agent path.
- **`CAG_MAX_TOKENS_PER_CHUNK`** (optional) — override the auto `slot*9/10`
  (`mcp.go` ~L155, ~L358) for chunk-sizing experiments.
- Tests: a `httptest.Server` standing in for `CAG_SERVER`; assert each env var
  changes the outgoing `/v1/register` / `/v1/project-query` request.

**B. Response-tail enrichment (~½ day) — unblocks the §4.4 scorers.** The MCP tool
returns prose only and the HTTP response has no tokens. Add a machine-parseable tail
to `formatProjectQueryResult` (`mcp.go` ~L236), e.g.
`<<<cag-meta>>>{…per-chunk chunk_id, idk, wall_ms, tokens_in, tokens_out}<<</cag-meta>>>`,
stripped before display and parsed by the scorers; and surface the token/timing
fields on `protocol.ChunkAnswerInfo` from the llama.cpp timings. This serves **both**
MCP (parse the tail) and HTTP. (This is design-doc citation "Path B" generalized —
§4.2.)

**C. The standalone CLI (~40 LOC).**

```go
// cmd/cag-query/main.go (in THIS repo, parallel to cmd/cag-server, cmd/cag-client)
package main

import (
    "encoding/json"
    "flag"
    "fmt"
    "os"
    "time"

    "github.com/smolskig/cag-mcp/internal/client"
    "github.com/smolskig/cag-mcp/internal/protocol"
)

func main() {
    project := flag.String("project", "", "project_id (required)")
    server := flag.String("server", os.Getenv("CAG_SERVER"), "CAG server URL (or $CAG_SERVER)")
    flag.Parse()
    if *project == "" || flag.NArg() == 0 {
        fmt.Fprintln(os.Stderr, "usage: cag-query --project ID [--server URL] \"question\"")
        os.Exit(2)
    }
    if *server == "" {
        *server = "http://localhost:8400"
    }
    cli := client.NewCagClient(*server) // has a 10-minute HTTP timeout built in
    t0 := time.Now()
    resp, err := cli.ProjectQuery(protocol.ProjectQueryRequest{
        ProjectID:   *project,
        Question:    flag.Arg(0),
        ThinkBudget: 100,
    })
    if err != nil {
        fmt.Fprintln(os.Stderr, "error:", err)
        os.Exit(1)
    }
    for _, a := range resp.Answers {
        fmt.Println(a.Answer) // ChunkAnswerInfo.Answer, not .Text
    }
    meta, _ := json.Marshal(map[string]any{
        "wall_ms":         time.Since(t0).Milliseconds(),
        "server_total_ms": resp.TotalMs,
        "all_idk":         resp.AllIDK,
        "answers":         len(resp.Answers),
    })
    fmt.Fprintln(os.Stderr, string(meta))
}
```

Built with `go build -o bin/cag-query.exe ./cmd/cag-query`. The author of a
case runs it directly during case authoring; the agent never uses it.

### 6.3 Scorers (~1 day)

Each scorer ~50–150 lines of Python. Test against hand‑crafted
question/answer pairs (no LLM in the loop for scorer tests). Living tests in
`eval/scorers/test_*.py`.

### 6.4 Mode‑1 runner (~1 day)

The cheaper mode. Build first because it has no exploration‑agent dependency,
so the harness can be validated end‑to‑end before the Mode‑3 complexity lands.

### 6.5 Mode‑3 runner (~2 days)

Adds: the explorer‑next prompt + a Haiku client, termination logic, session
re‑entry. Re‑uses Mode‑1 scorers verbatim.

### 6.6 Aggregator + reporting (~½ day)

Two CSVs and one markdown report per run:
- `per-turn.csv` — rows are `(case, profile, seed, turn)`, cols are scorers.
- `per-case.csv` — rows are `(case, profile, seed)`, cols are cumulative
  scorers + total tokens + total wall.
- `report.md` — narrative with per‑category breakdown, win/loss matrix, paired
  A/B deltas with confidence intervals (Wilson interval on coverage proportions
  is fine — N is small).

### 6.7 Documenting freshness (~½ day, after Mode‑3 works)

Implement the synthetic‑edit path in the runner. Requires reverting edits
cleanly — use `git stash` against a known‑clean working tree inside the corpus
repo, then `git stash drop`.

---

## 7. Profiles (concrete first sweeps)

`eval/profiles/<name>.yaml`. Format extends design doc §"Config profiles":

```yaml
profile_id: cag-msn1-history-c-v01
description: "CAG, full msn1 (auto-chunked ~6–7×90k across 2 slots), agent-strategy C, qwen3.6-27b-mtp exploration, gpt-oss-20b CAG backend"

# Corpus — selected by the MCP server's cwd; auto-chunked by cag-client.
corpus: msn1                             # cwd = ~/msn1; no hand-picked subset
# (chunk strategy & sizing are owned by cag-client: import_graph, 90% of per-slot
#  ctx ≈ 90k. To experiment, set CAG_MAX_TOKENS_PER_CHUNK on the server block.)

# CAG server config — matches the 2×100k rig (used to start the server if not up).
cag_server:
  kv_type: f16
  ctx_size: 200000                       # 2 slots × 100k
  slots: 2                               # → MaxTokensPerSlot = 100k, chunk cap 90k
  reasoning_effort: low
  model: gpt-oss-20b

# Agent (kvit-coder) — CAG reached via an MCP server block, not a native tool.
agent:
  config: cag-baseline.kvit.yaml         # under eval/profiles/kvit/ (carries the mcp.servers block)
  agent_file: agent-cag.md               # under eval/prompts/
  agent_strategy: C                      # A | B | C — selected via agent_file variant
  llm_model: qwen3.6-27b-mtp
  llm_base_url: http://192.168.8.20:8080/v1   # same host as CAG server, port 8080
  mcp_server:                            # rendered into the generated kvit config.yaml
    name: cag
    command: /home/sk/cag-mcp/bin/cag-client
    cwd: /home/sk/msn1
    env:
      - CAG_SERVER=http://192.168.8.20:8400
      - CAG_PROJECT_ID=msn1
      - CAG_THINK_BUDGET=100             # the per-profile think-budget knob
    tools: { allow: [cag_query] }

# Exploration loop (Mode 3 only)
exploration:
  next_prompt: explorer-next-v01.md
  next_model: haiku
```

### 7.1 First three sweeps to actually run

| Sweep | Profiles | Holds constant | Varies | Hypothesis |
|---|---|---|---|---|
| **S1 — calibration on docs** | `cag-docs-history-c`, `grep-docs` | corpus = docs1+docs2 | system (CAG vs grep) | CAG should win cross‑file trace cases, abstain better on unanswerable |
| **S2 — CAG vs grep on msn1** | `cag-msn1-history-c`, `grep-msn1`, `grep-msn1-eng` | corpus = full msn1, backend = gpt-oss-20b | system (3 ways) | CAG wins cross‑file + synthesis; grep wins local‑fact at lower cost |
| **S3 — agent strategy on CAG** | `cag-msn1-history-A`, `…-B`, `…-C` | system = CAG, corpus = msn1 | agent strategy A/B/C | C (summarized history) dominates per design doc expectation |

S1 is the **calibration gate**. If S1 does NOT reproduce the qualitative
findings from `report.md` experiments 8–11, the harness is wrong — fix the
harness before running S2 or S3.

Each sweep: 23 cases (or relevant subset) × N profiles × 3 rollouts. With
~8 turns × ~5 s/turn that's ~15 minutes of clock per rollout, so S2 is
~ 23 × 3 × 3 × 15 min ≈ 50 hours of wall — overnight runs. Budget for it.

---

## 8. Execution sequence (Gantt‑ish)

Same shape as design doc §"Sequencing", grounded in concrete deliverables. Days are
work‑days, not calendar. **Step 0 is a hard gate: don't build the harness until the
MCP spine runs end to end.**

| # | Deliverable | Days | Depends on |
|---|---|---|---|
| 0 | **Spine smoke test (§2.5, no new code)** — wire a `cag` MCP block into kvit-coder (`cwd=~/msn1`), cag-server up, run one `cag_query`, confirm it's invoked and returns a sane cited answer. Gates everything below. | 0.5 | — |
| 1 | Scaffolding + `cmd/cag-query` CLI + cag-client env knobs (`CAG_PROJECT_ID`/`CAG_THINK_BUDGET`) + response-tail enrichment + corpus inspect script | 1.5 | 0 |
| 2 | Sanity‑check the step‑1 code: run `cag-query` against `msn1` and `docs`; confirm `CAG_PROJECT_ID`/`CAG_THINK_BUDGET` take effect and the response tail parses | 0.5 | 1 |
| 3 | Author 5 docs cases + 3 msn1 cases (one per category, smallest set that exercises every scorer) | 1.5 | 2 |
| 4 | Scorers (nugget, citation Path A, abstention, latency) + unit tests | 1 | 3 |
| 5 | Mode‑1 runner + aggregator (CSV + markdown report) | 1 | 4 |
| 6 | **Run S1 (calibration sweep on docs)** — read report, fix harness bugs | 0.5 | 5 |
| 7 | Author remaining cases: msn1 to 13, allocator to 5, docs to 5 | 2 | 6 |
| 8 | Mode‑3 runner (+ explorer‑next prompt, Haiku integration) | 2 | 5 |
| 9 | **Run S2 (CAG vs grep on msn1)** | 0.5 (compute overnight) | 7,8 |
| 10 | **Run S3 (agent strategy sweep)** | 0.5 | 9 |
| 11 | Decide next investment: synthesis judge, Tier‑1 anchor, larger corpus (msn1 + more), RAG/GraphRAG | — | 10 |

**Stop conditions before next investment.** Before adding Tier 1, Mode‑3
exploration over a different exploration LLM, or synthesis‑judge work:
S1+S2+S3 must produce *consistent rankings across rollouts* (median rank
agreement ≥ 0.7 across N=3 seeds). If rankings flap, the per‑case sample is
too small or scoring is too noisy — address that first.

---

## 9. Risks and open decisions (carry‑overs from design)

Only the ones that affect *what to build first*:

1. **Citation contract (Path A vs B).** Resolve at end of step 4. If Path A's
   regex hits < 60 %, do Path B before step 6 — re‑scoring earlier runs is
   cheap, but earlier *interpretation* will be wrong.
2. **MCP integration pinning (no fork).** The eval uses stock kvit-coder + an MCP
   server block; CAG behavior lives in cag-client. Pin **both** commit hashes
   (cag-mcp and kvit-coder) in every run's profile, since the `cag_query` contract
   (auto-chunk sizing, think-budget default, response tail) is defined by the
   cag-mcp commit. No branch to track.
3. **Multi-chunk scheduling over real content.** The default `msn1` corpus already
   auto-chunks into ~6–7 chunks across both slots, so S2 exercises the multi-chunk
   worker-pool over *real* content (not synthetic docs1+docs2) from the start —
   the architecture the server was built for. Watch the first S2 run for scheduler
   stalls / VRAM exhaustion (silent t/s halving) before scaling case count; it's
   the most expensive path to run.
4. **Exploration agent variance.** If Mode‑3 rollouts at N=3 show median
   coverage spread > 0.2, bump to N=5 for the comparison sweeps. Set the
   exploration LLM temperature to 0.3 (not 0) — the variance is the realism.

Everything else from the design doc's open questions stays open and is
explicitly deferred past step 11.

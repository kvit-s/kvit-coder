# Evaluation Design — Iterative Exploration of CAG-MCP

## Goal

Measure how well an agent + model pair, using CAG-MCP, performs at the workflow that actually matters: **iterative exploration of a codebase** (general → specific → very specific) toward a target understanding. The eval is structured so the same case set produces comparable numbers across systems — CAG, subagent baselines, static analysis + LLM, eventually RAG / GraphRAG — and across agent-side prompt strategies.

The output of this doc is methodology and artifact spec, not code. Build order in [Sequencing](#sequencing).

## Scope

In scope:
- Iterative, multi-turn exploration with an agent-adaptive loop
- Per-turn answer quality + cumulative target coverage + cost
- Cross-system comparison (CAG vs subagent vs static analysis vs …)

Deferred (constructible later from artifacts produced during iterative runs):
- A systematic case-generation pipeline (seed extractors, automated question synthesis, triviality filter). For now: **hand-author** the iterative cases. Once the iterative eval reveals what kinds of facts and questions matter, automate retroactively.
- A standalone single-shot benchmark. The per-turn scorers below produce single-shot numbers as a byproduct; a dedicated single-shot run can be reconstituted later from the same case set.
- End-to-end task success measurement (real agent completing real coding tasks).
- Fine-tuning / training-data pipelines.
- Public-benchmark / leaderboard packaging.

---

## Corpus tiers

The dominant threat to validity is **contamination**: if the backend model memorized the repo, the eval measures recall-from-weights, not retrieval. Selection rule: *ground truth is cheap AND the model has not seen the code.*

| Tier | Source | Purpose | Contamination risk |
|------|--------|---------|--------------------|
| **3 — Private regression** | `docs1` / `docs2` (apos/microstrata, in `experiments/data/`) | Build & smoke-test the harness; fast regression on config changes | None (private) |
| **2 — Contamination-control** | 2–3 recent, low-profile OSS repos past the model's training cutoff; mid-size (10k–100k LOC); mixed languages | The *trustworthy* signal | Low (chosen for it) |
| **1 — Labeled anchor** | A code-retrieval benchmark with shipped labels (e.g. **CodeRAG-Bench**; or **SWE-bench Verified** used as a file-localization source where the gold patch defines relevant files) | External comparability + free labels | Medium (note it) |

Build in the order **3 → 2 → 1**: prove the harness on the corpus we know cold, get real signal on an uncontaminated repo, add the anchor only when external comparability is wanted.

Repo selection for Tier 2 is by **recency + low star count**, never by fame.

---

## Iterative case structure

Each case is a *topic* with a target understanding to reach across a sequence of turns. Hand-authored initially; ~10–20 cases on docs1/docs2 to start.

```yaml
case_id: cag-mcp-async-redesign-01
topic: "Understand inter-component communication and async architecture"
corpus: cag-mcp
seed_question: "How does inter-component communication work in this codebase?"
target_understanding:
  - id: T1
    fact: "HTTP endpoints /v1/register and /v1/project-query"
    relevant_files: ["internal/server/api.go"]
  - id: T2
    fact: "Multi-chunk parallel query uses goroutine worker-pool in scheduler"
    relevant_files: ["internal/server/scheduler.go"]
  - id: T3
    fact: "Chunks loaded into llama.cpp slots via KV cache restore"
    relevant_files: ["internal/server/scheduler.go", "internal/server/cache.go"]
  - id: T4
    fact: "Errors propagate via Go return values, no message queue"
    relevant_files: ["internal/server/scheduler.go"]
  - id: T5
    fact: "Sync HTTP, no broker"
    relevant_files: ["internal/server/api.go"]
turn_budget: 8                # hard cap on follow-ups
optional_anchor_sequence:      # used by Mode 1 only — see operating modes
  - turn: 1
    question: "How does inter-component communication work in this codebase?"
  - turn: 3
    question: "Where in the codebase are requests distributed to multiple workers?"
```

The `target_understanding` set is the substrate for cumulative-coverage scoring. Authoring guideline: each `fact` is a discrete claim that can be matched objectively (file/symbol name, numeric value, named concept), with `relevant_files` pointing to where the fact lives.

### Naturalness calibration

Hand-authored cases can read like benchmark questions, not like how a developer/agent actually asks. Keep a small subset where the `seed_question` is taken verbatim from a real session transcript (or written deliberately in agent voice); confirm that scores on the natural subset track the synthetic subset. If they diverge, the synthetic set is mis-measuring.

---

## The iterative eval

### Three operating modes

| Mode | Follow-up source | Comparability across systems | Realism | Cost |
|---|---|---|---|---|
| **1 — Fixed anchor sequence** | predetermined Q1…QN from `optional_anchor_sequence` | apples-to-apples | low (real agents adapt) | low |
| **2 — Branching tree** | predetermined branches; system's A picks branch | medium | medium | medium |
| **3 — Agent-adaptive** | a fixed exploration-agent picks Q_{n+1} based on actual A_n | low (paths diverge) | **high** | high |

**Mode 3 is the primary methodology.** It captures the real workflow — where the agent's follow-up depends on what each system actually returned — which is where most of the practical value (or lack of it) shows up.

Mode 1 still runs on a small subset (~10 cases) as a cross-system **anchor**: identical questions, identical comparison, easy to debug regressions. Mode 2 is optional and used only if Mode 3 + Mode 1 leave a comparability gap worth closing.

### The exploration agent in the loop

Mode 3 needs an actual agent loop in the harness. Its job:

- Receive the case's `seed_question`.
- Query the system under test.
- Decide whether the target understanding is reached (judged by the cumulative-coverage scorer below).
- If not, formulate a follow-up question based on what's been returned and what's still missing.
- Repeat up to `turn_budget`.

Design choices to pin so this stays controlled:

- **Fix the exploration agent's LLM.** Don't sweep it within an experiment. A reasonable default is a mid-size model (Haiku-class or Sonnet-class); pick one and stick to it. Generalization to other agents = a separate sweep, not a confound mixed into every run.
- **Fix the exploration agent's prompt.** This is the "how the agent is instructed to use the tool" lever — sweep it across experiments, not within one. Document the prompt with each result.
- **Variance.** Mode 3 rollouts are stochastic. Run **N=3–5 rollouts per case** and report median + spread.

The exploration agent is itself part of the system being measured — intentional, since in production the agent is part of the system. The eval needs to be honest about whether a regression is the backend, the agent, or the agent prompt (see [open questions](#open-questions)).

### Agent-side strategies as a sweep axis

The conversation/history strategies established earlier run against a stateless MCP server:

- **A — full append.** Previous Q + merged A + new Q. Best quality; pays prompt-eval cost on history every turn (multi-chunk prefix cache breaks at the merged answer).
- **B — no history.** Standalone clarifying questions. Cheapest per turn; weak on references like "of those callers, which …".
- **C — summarized history.** Bounded running summary + new Q. Most of A's quality at most of B's cost.

Treat these as a first-class sweep axis. Expected (and worth confirming): C dominates A and B on the cost/quality curve, but only the eval shows it.

---

## Scoring

### Per-turn (each query's answer)

| Scorer | Definition | Objective? |
|--------|-----------|-----------|
| **Nugget recall** | fraction of relevant `target_understanding` facts present in this turn's answer | yes |
| **Citation validity** | of cited `file:line` refs, fraction that exist AND contain the claim | yes (mostly) |
| **Hallucination rate** | cited refs that don't exist or don't support the claim | yes |
| **Abstention correctness** | on questions whose answer isn't in the corpus, did it say "I don't know"? | yes |
| **Retrieval recall** | did chunk(s) covering `relevant_files` get surfaced / queried? | yes |
| **Latency** | wall time for the turn | yes |
| **Tokens** | prompt + completion tokens for the turn | yes |
| **Synthesis quality** | absolute rubric (1–5) via LLM-judge | no — validated against human on a sample |

**Objective-first.** Nugget recall + citation validity + abstention carry the bulk; LLM-judge is reserved for synthesis-style answers where no single artifact is the answer. Before trusting the judge, validate against human labels on ~20% of synthesis cases and report agreement.

### Per case (cumulative across turns)

| Scorer | Definition | Why it matters |
|---|---|---|
| **Coverage curve** | fraction of `target_understanding` IDs surfaced by the system's cumulative answers, after each turn | does the system *build up* understanding? |
| **Final coverage** | coverage at turn `turn_budget` (or at convergence) | did it get there at all? |
| **Turns to convergence** | smallest N where coverage stops increasing | iteration efficiency |
| **Total tokens** | sum over all turns (prompt + completion + per-chunk overhead) | full cost of the exploration |
| **Total wall time** | sum over all turns | latency cost |
| **Path divergence** | (cross-system only) similarity of the question sequences taken | did systems explore differently? for analysis, not a target |

The coverage **curve** matters more than per-turn quality. A system that takes 8 turns to reach 100% coverage at low cost beats one that reaches 70% in 4 turns at higher cost, *for this metric*. Whether that's the right tradeoff is a per-use-case judgment — which is why we report the curve, not just a single number.

---

## Cross-system comparison

The eval is built to compare CAG-MCP against alternative ways of doing exploration:

| System | Description | What it controls for |
|---|---|---|
| **CAG-MCP** | the system, with sweep over config + agent-strategy | reference |
| **Subagent — realistic prompts** | spawn Explore-style subagent with general prompts | the realistic alternative an agent already has |
| **Subagent — engineered prompts** | careful, multi-clause prompts per turn | best-case subagent; separates prompt skill from architecture |
| **Static analysis + LLM** | call-graph / definitions extracted from tree-sitter; LLM synthesizes | pure-syntactic baseline; we expect CAG to win on semantic categories |
| **Single-shot CAG** | no iteration, one query per case's seed_question | shows the value added by iteration itself |
| *(future) RAG, GraphRAG* | once methodology is settled | external comparison |

Report per topic/case-category — local fact, cross-file trace, synthesis, unanswerable, freshness — so we can see *where* each system wins or loses, not just an aggregate.

**Fairness note.** A frequent pitfall is engineering CAG's setup while leaving the alternatives unoptimized. Both subagent baselines (realistic *and* engineered) should be present — engineered to show the architectural ceiling, realistic to show the practical comparison.

---

## Running experiments

### Config profiles

A run is a sweep over `profiles × cases × N_rollouts`. A profile names:

```yaml
profile: f16-2slot-history-C
kv_type: f16
ctx_size: 140000
slots: 2
reasoning_effort: low
two_pass_thinking: 100
corpus_variant: source_only          # vs source+docs, +repomap, +symbolindex
prompt_variant: retrieval_v1         # backend system prompt
agent_strategy: history_C            # A | B | C
backend_model: gpt-oss-20b
exploration_agent_model: haiku
exploration_agent_prompt: explore_v1
```

`corpus_variant`, `prompt_variant`, `agent_strategy`, and the exploration-agent fields are first-class axes alongside server config — these are exactly the levers we want to A/B.

### Variance control

- Change **one variable at a time** between paired profiles.
- **Paired A/B**: same case set, same N_rollouts seeds, only the swept variable differs.
- Backend at **temp 0**; the exploration agent can be temp 0 if its prompt is rich enough — otherwise small temperature with N rollouts.
- **Version** the case set, the profiles, and the exploration-agent prompt so results stay comparable as the system evolves.

### Reporting

Per sweep:
1. **Per-turn scorer table** — attribution: where did per-turn quality move?
2. **Per-case cumulative table** — coverage curve summary, final coverage, turns-to-convergence, total tokens, total wall time.

A profile "wins" only when it improves a target metric **without** regressing faithfulness or abstention. Always show the per-category split — aggregates hide regressions that matter.

---

## Sequencing

1. **Iterative case schema** — define the YAML, write the first 3–5 cases on docs1/docs2 by hand. Sanity-check that target facts are objectively matchable.
2. **Per-turn scorers** — corpus-independent, load-bearing. Nugget recall + citation validity + abstention + latency + tokens. (Synthesis-judge later.)
3. **Cumulative scorers** — coverage curve, final coverage, turns-to-convergence.
4. **Mode 1 (fixed anchor) harness** — simplest version: hard-code the question sequence, just measure per-turn + cumulative. Reuse existing `experiments/` infrastructure (primes slots, queries, dumps JSON).
5. **Validate Mode 1** on known config differences (e.g. f16 vs q8_0, single-slot vs multi-slot). Confirm direction matches prior single-shot experiments.
6. **Expand iterative case set** to ~10–20 cases.
7. **Mode 3 (agent-adaptive) harness** — exploration agent loop, follow-up generation, termination logic. Run on the same case set.
8. **First Mode 3 sweep** — vary KV type, slots, two-pass, agent-strategy (A/B/C), backend prompt variants.
9. **Subagent baseline** (both realistic and engineered prompt variants). First real cross-system comparison.
10. **Tier 2 corpus** — first uncontaminated signal.
11. **Tier 1 anchor** — only when external comparability is wanted.

Stop before end-to-end task success, fine-tuning, and the automated case-generation pipeline. Revisit those once the iterative eval is producing stable, trustworthy signal.

---

## Open questions

1. **Output contract dependency.** Citation validity and retrieval recall assume the MCP returns structured `file:line` anchors. Today it returns prose. Either the scorer parses anchors out of prose (brittle), or the tool contract is changed to emit them. Decide before building the citation scorer — upstream of multiple scorers and of the tool-surface design lever.
2. **Exploration agent choice.** Which agent LLM to fix in the loop? Cheap (Haiku-class) reflects mass-market use but may sandbag CAG; strong (Sonnet/Opus-class) is unrealistic for a long-running cheap exploration loop. Likely pick Haiku-class as default, optionally re-run on a stronger agent as a sanity check.
3. **Coverage judge.** Determining "has the system surfaced fact T_n" needs to be objective — nugget-string match across the union of answers, ideally without an LLM judge. Define matching rules carefully; numbers exact, phrases normalized, multi-token concepts list-matched.
4. **Divergent-path comparison.** When CAG and the subagent baseline take different question sequences in Mode 3, what's the fair comparison metric? Likely: final coverage and total cost; the per-turn comparison only makes sense in Mode 1.
5. **Nugget matching strictness.** Exact string vs. normalized vs. embedding-similarity match. Start strict (exact for numbers/symbols, normalized for phrases); loosen only if it disagrees with human judgment on the calibration subset.
6. **Generator/judge/agent model overlap.** If the case-author (human now, possibly LLM-assisted later), the synthesis judge, and the exploration agent all come from the same family, they share blind spots. Prefer different families across judge/agent.
7. **Freshness case mechanics.** Mutation-pair cases need the harness to drive the overlay/update path between turns — couples the eval to the register/overlay API, not just project-query.
8. **Run-cost budget.** Mode 3 with N=5 rollouts × 20 cases × M profiles is a lot of CAG queries. Set a per-experiment token budget and cache aggressively (results are deterministic at temp 0 when case + profile + seed match).
9. **When to revisit deferred layers.** Once Mode 3 is producing stable signal: should the next investment be (a) automated case generation, (b) standalone single-shot benchmark, (c) Layer-3 end-to-end task success, or (d) Tier-2 corpus expansion? Decide based on which gap is hurting most after the first real sweeps.
10. **Tier-2 repo licensing** for storing corpus + derived cases in this repo's history.

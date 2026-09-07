# Evaluation Progress

Running log for the CAG-MCP eval. Companion to `eval-plan.md` (execution plan) and
`eval-design.md` (methodology). Step numbers (0–11) map to `eval-plan.md` §8
"Execution sequence."

**Last updated:** 2026-06-28
**Current status:** Step 0 (spine smoke test) in progress — **blocked on cag-server
being up and reachable from WSL.** Harness code (step 1+) not started.

---

## Status at a glance

| Step | Deliverable | Status |
|---|---|---|
| — | `eval-plan.md` authored + revised to the verified architecture | ✅ done (committed `556b5c6`) |
| 0 | Spine smoke test (kvit-coder → cag-client → cag-server → llama.cpp) | 🟡 in progress — **blocked** |
| 1 | Scaffolding + `cmd/cag-query` CLI + cag-client env knobs + response-tail enrichment + inspect script | ⬜ not started |
| 2 | Sanity-check step-1 code against `msn1` / `docs` | ⬜ |
| 3 | Author 5 docs + 3 msn1 cases | ⬜ |
| 4 | Scorers (nugget, citation, abstention, latency) + tests | ⬜ |
| 5 | Mode-1 runner + aggregator | ⬜ |
| 6 | Run S1 (calibration on docs) | ⬜ |
| 7 | Author remaining cases | ⬜ |
| 8 | Mode-3 runner | ⬜ |
| 9 | Run S2 (CAG vs grep on msn1) | ⬜ |
| 10 | Run S3 (agent-strategy sweep) | ⬜ |
| 11 | Decide next investment | ⬜ |

---

## Done

### Plan + architecture verification (against source)
- `eval-plan.md` written and committed (`556b5c6`), then revised to match the code:
  - kvit-coder flags `-p`, `-s`, `-agent-file`, `-config`, `--json` confirmed in
    `cmd/kvit-coder/main.go`.
  - kvit-coder has an MCP client (`internal/mcp`, `internal/mcpclient`) → CAG is
    reached over MCP, **not** a native tool; **no kvit-coder fork**.
  - `cag_query` contract confirmed in `internal/client/mcp.go`: single `question`
    arg, prose result, project = `sha256(cwd)`, auto-chunk at 90 % of per-slot ctx,
    built-in freshness via `detectUpdates()`.
  - 2 × 100 k slot sizing confirmed: chunks auto-cap at `MaxTokensPerSlot * 9/10`
    ≈ 90 k (`mcp.go` ~L155 / ~L358).
  - Config knobs forwarded by kvit-coder's `MCPServerConfig` (`command`/`args`/
    `env`/`cwd`, `${VAR}` expansion) → all CAG knobs live in cag-client env.

### Step 0 — no-code prereqs
- **cag-client built for Linux and deployed.** Cross-compiled on Windows
  (`GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o bin/cag-client ./cmd/cag-client`
  — pure Go, no cgo). Copied to `~/cag-mcp/bin/cag-client` in WSL, `chmod +x`,
  verified it starts and connects. (No Go toolchain in WSL — cross-compile is the path.)
- **Exploration LLM verified up:** `qwen3.6-27b-mtp` live on `192.168.8.20:8080`
  (llama-swap).
- **Topology pinned down:** dev box = `192.168.8.20` (there is **no** `.10`); both the
  LLM (`:8080`) and cag-server (`:8400`) live here. `eval-plan.md` IPs corrected
  `.10 → .20`; saved to memory (`rig-network-topology`).

---

## Blocked

- **cag-server is not running.** Nothing answers on `:8400` — Windows `localhost:8400`,
  `192.168.8.20:8400` from WSL, and the WSL gateway are all dead. Step 0 cannot proceed
  until it is up.
- **WSL reachability (once up):** kvit-coder + cag-client run inside WSL; WSL2 NAT can't
  reach the Windows host via `localhost`. cag-server must **bind `0.0.0.0:8400`** and
  **Windows Firewall must allow inbound 8400** from the WSL subnet. Then
  `CAG_SERVER=http://192.168.8.20:8400`.

---

## Next actions (in order)

1. **Start cag-server** on the rig (2 × 100 k config), bound to `0.0.0.0:8400`, firewall
   open. *(needs the start command/script.)*
2. **Generate kvit-coder smoke config** — `mcp.servers` block (command
   `~/cag-mcp/bin/cag-client`, cwd `~/msn1`, `CAG_SERVER=http://192.168.8.20:8400`,
   `tools.allow: [cag_query]`), file tools off for CAG isolation, llm `qwen3.6-27b-mtp`
   @ `.20:8080` — plus a one-line agent-file. *(ready to drop in.)*
3. **Run the smoke test:** `cd ~/msn1 && ~/kvit-coder/kvit-coder -config <smoke>
   -agent-file <af> --json -p "How does EventBus routing work?"`. Pass =
   `mcp.cag.cag_query` invoked, real `msn1` citations, not all-IDK.
4. On green → start **Step 1** harness code: cag-client env knobs
   (`CAG_PROJECT_ID`/`CAG_THINK_BUDGET`) → response-tail enrichment → `cmd/cag-query`
   → `eval/` scaffolding.

---

## Open issues / discrepancies

- **CAG-backend model id:** llama-swap serves `gpt-oss-20b-full`; the plan/cag-server
  config may say `gpt-oss-20b`. Reconcile before scored runs.
- **No Go in WSL** — all cag-mcp Linux builds are cross-compiled from Windows.
- Other working-tree edits (`internal/client/mcp.go`, `internal/server/{process,scheduler}.go`,
  `readme.md`, `eval-design.md`) are uncommitted, pre-session, and unrelated to the eval —
  left untouched.

---

## Session log

- **2026-06-28** — Reviewed `eval-plan.md` against source; reworked for MCP integration,
  2 × 100 k auto-chunk sizing, knobs-in-cag-client; committed (`556b5c6`). Added §2.5
  spine smoke test + §8 step-0 gate. Cross-compiled and deployed `cag-client` to WSL.
  Discovered cag-server down and wrong IPs; corrected `.10 → .20`; saved topology to
  memory.

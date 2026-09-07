# Archive

These documents describe work that is finished, superseded, or abandoned. They
are kept because they record why parts of the program are shaped the way they
are, and they are wrong about the program as it stands today. For the current
design, read `../redesign.md`.

| File | Date | Status |
|---|---|---|
| `code-review.md` | December 2025 | A review of the codebase eight months and roughly forty commits before the redesign. Superseded by `../review.md`. |
| `agent-file.md` | February 2026 | Plan for the `--agent-file` option, which appends a file's contents to the system prompt. Implemented; the root `README.md` documents the result. |
| `mcp-plan.md` | June 2026 | Plan for the Model Context Protocol client. Implemented as `internal/mcp`. Still the fullest description of how that client works, which is why `../redesign-mcp.md` refers to it. |
| `safety-mode-enhancement.md` | December 2025 | Plan for the path-safety modes (`block`, `warn`, `ask_once`, `ask_always`). Implemented as `internal/safety`. |
| `thinkbench.md`, `thinkbench-plan.md` | June 2026 | Why and how the thinkbench task suite was brought in. Implemented; `../../benchmarks/README.md` covers running it. |
| `tool-improvements.md` | June 2026 | Edit reliability and anomaly diagnostics for models that miscounted lines and mis-formatted calls. The machinery it specifies still exists but is off by default: `agent.profile: strong` skips fuzzy matching, indentation repair, backtracking, the duplicate-call guard and anomaly interrogation. Reading it as current guidance will mislead you. |
| `tool-prompts.md` | June 2026 | A probe asking three local models to describe the file-editing tools they were trained on, with their raw answers. Consolidated from eight files. Superseded by the two profiles in `../redesign.md` §11. |
| `eval-design.md`, `eval-plan.md`, `eval-progress.md` | June 2026 | Design, execution plan and progress for an evaluation of cache-augmented generation reached over MCP. The smoke-test fixtures it used are still in `../../eval/`. Not resumed. |

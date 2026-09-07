# Memory footprint of concurrent coding agents

Measured 2026-09-07 04:47 CDT on a WSL2 instance (Linux 6.18.33.2-microsoft-standard-WSL2)
with 32 GB of RAM and 16 GB of swap.

## Why this measurement exists

Several coding agents run side by side on this one machine — Claude Code, OpenAI's Codex
CLI, krok, and kvit-coder — alongside a local vLLM inference server and a VS Code remote
server. Running many sessions at once is only safe if you know what one session costs, so
this file records the per-agent cost rather than a single aggregate number.

Everything below is one snapshot of the processes that happened to be running at that
moment: eight Claude Code sessions, two Codex sessions, three krok processes, and one
kvit-coder user interface with its agent child.

## Summary: cost of one agent

| Agent | Resident (RSS) | Shared-adjusted (PSS) | Paged to swap | Processes per session | Runtime |
|---|---|---|---|---|---|
| Claude Code | 341 MB avg (195–428) | 301 MB avg | 44 MB avg | 1 | Node/V8 |
| Codex | 261 MB avg (240–281) | 261 MB avg | 456 MB avg | 2 (native + Node wrapper) | Rust, musl static |
| krok | 221 MB avg (35–435) | 192 MB avg | 16 MB avg | 1 | Rust |
| kvit-coder | 27 MB (pair) | 24 MB | 0 | 2 (interface + agent) | Go 1.25.4 |

The three columns measure different things and the gap between them matters here.
*Resident* is physical RAM the process currently occupies. *Shared-adjusted* divides each
shared page by the number of processes mapping it, so summing this column across processes
gives a total that does not double-count shared libraries; for these agents it runs 5–15%
below resident. *Paged to swap* is memory the kernel has already evicted to disk, which
counts against the machine only when the process becomes active again and faults it back in.

For capacity planning, the number that matters is resident plus swap, since an idle session
that wakes up reclaims what was evicted. On that basis: **Claude Code 384 MB, Codex 717 MB,
krok 237 MB, kvit-coder 27 MB per session.**

## Claude Code

Eight sessions, all invoked as `claude --dangerously-skip-permissions`, all a single Node
process each.

| PID | RSS | PSS | Swap | Age | Threads |
|---|---|---|---|---|---|
| 785444 | 428 MB | 388 MB | 73 MB | 29.5 h | 70 |
| 595557 | 407 MB | 346 MB | 0 | 56 min | 22 |
| 793361 | 386 MB | 352 MB | 87 MB | 29.5 h | 67 |
| 2640219 | 359 MB | 312 MB | 58 MB | 23.6 h | 52 |
| 944625 | 337 MB | 276 MB | 0 | 9 min | 21 |
| 2983297 | 312 MB | 310 MB | 58 MB | 35.6 h | 24 |
| 4069548 | 304 MB | 260 MB | 45 MB | 2.1 h | 20 |
| 900202 | 195 MB | 166 MB | 28 MB | 29.1 h | 20 |
| **total** | **2727 MB** | **2410 MB** | **349 MB** | | |

Claude Code is the heaviest of the four and also the most predictable, clustering tightly
around 300–430 MB with one outlier at the low end. The footprint tracks conversation
history rather than wall-clock age: the session started nine minutes before the snapshot
was already at 337 MB, whereas the 35-hour session sits at 312 MB. Thread count is the
clearer signal of how much work a session has done, running from 20 on quiet sessions up to
70 on the two long-lived ones, which suggests worker threads accumulate with subagent and
tool activity.

Budget 400 MB for a session and 500 MB for one expected to run for hours with a large
context.

## Codex

Two sessions, each running the native `codex` binary from the
`@openai/codex-linux-x64` package under a small Node launcher that stays resident.

| PID | Role | RSS | PSS | Swap | Age |
|---|---|---|---|---|---|
| 625767 | native binary | 281 MB | 281 MB | 869 MB | 3.1 days |
| 625754 | Node launcher | 0.1 MB | 0.1 MB | 9 MB | 3.1 days |
| 2947649 | native binary | 240 MB | 240 MB | 44 MB | 7.4 h |
| 2947641 | Node launcher | 25 MB | 16 MB | 9 MB | 7.4 h |

Reading only the resident column makes Codex look cheaper than Claude Code, and that
reading is wrong. The three-day-old session has 869 MB paged out to swap because the kernel
reclaimed it while idle, so its true working set is around 1.15 GB and it would fault most
of that back in on the next turn. Its shared-adjusted figure equals its resident figure
almost exactly, which follows from the binary being statically linked against musl — there
are effectively no shared library pages to discount.

Budget 700 MB for an active session, and note that a Codex session is two processes, not one.

## krok

Three processes, all the same Rust binary at `/home/sk/krok/krok`, a symlink to
`target/release/xai-grok-pager`.

| PID | RSS | PSS | Swap | Age | State |
|---|---|---|---|---|---|
| 973656 | 435 MB | 393 MB | 0 | 3 min | running a prompt-file task |
| 947594 | 192 MB | 150 MB | 0 | 8 min | active |
| 1774865 | 35 MB | 32 MB | 48 MB | 26.2 h | idle, mostly evicted |

krok has the widest spread of the four, from 35 MB idle to 435 MB while working a task,
so a single average is less useful here than the range. The high reading came from a process
started three minutes before the snapshot against a prompt file under
`kvit-stow/data/iterations/office-v1/`, which shows the ceiling is set by task size rather
than by session age. All three processes hold 60–64 threads regardless of how much memory
they use.

Budget 250 MB for a typical process and 450 MB for one working a large prompt.

## kvit-coder

One user interface process with one agent child, both Go binaries built from
`github.com/kvit-s/kvit-coder` with go1.25.4.

| PID | Process | RSS | PSS | Swap | Age | Threads |
|---|---|---|---|---|---|---|
| 698847 | `kvit-coder-ui` | 10.6 MB | 9.1 MB | 0 | 41 min | 14 |
| 706680 | `kvit-coder` | 16.2 MB | 14.7 MB | 0 | 39 min | 14 |
| **pair** | | **26.8 MB** | **23.8 MB** | **0** | | |

The complete pair costs about 27 MB, roughly a fifteenth of a Claude Code session and an
eighth of a krok process. Nothing has been evicted to swap, so unlike the Codex figures
these need no correction. Both processes hold 14 threads, matching the Go runtime's default
of one per available core.

A follow-up reading two minutes later showed the interface process alone at 33.7 MB, up
from 10.6 MB, after its agent child had finished and exited. The interface holds the
conversation once the agent turn ends, so its resident size grows across a session while the
child is transient — the 27 MB pair figure above is the cost during an active turn, not a
resting baseline.

Two things qualify the number. The pair had been up around 40 minutes on a short
conversation when measured, so this is closer to a floor than a steady state, and resident
memory will climb as conversation history accumulates — though Go's per-token overhead is
well below what V8 adds in the Node-based tools. The binary is also unstripped and carries
debug information, which inflates the file on disk without meaningfully affecting resident
memory, since those sections are never paged in during normal execution.

## What else is on the machine

Agents are not the largest consumer here. At snapshot time the system reported 13.7 GB used,
18.5 GB available, and 5.6 GB of the 16 GB swap already committed.

| Consumer | Resident |
|---|---|
| vLLM inference server (2 processes serving Qwen3.8-27B) | 5.7 GB |
| All 15 agent processes combined | 3.9 GB |
| VS Code server extension hosts (6 processes) | 2.8 GB |

Counting evicted pages, the agents account for roughly 5.1 GB of committed memory rather
than 3.9 GB. The vLLM server is the single largest process on the box and its engine core
alone holds 3.6 GB.

## Reproducing this

`ps` reports resident size but not the shared-page discount or evicted pages, both of which
change the conclusion for Codex. Read `/proc/<pid>/smaps_rollup` instead:

```bash
for pid in $(ps -eo pid,comm | awk '$2 ~ /^(claude|codex|krok|kvit-coder|kvit-coder-ui)$/ {print $1}'); do
  awk -v p="$pid" -v c="$(cat /proc/$pid/comm)" '
    /^Rss:/{r=$2} /^Pss:/{s=$2} /^Swap:/{w=$2}
    END{printf "%.1f\t%-14s %7s  rss=%.1fMB pss=%.1fMB swap=%.1fMB\n", r/1024, c, p, r/1024, s/1024, w/1024}
  ' /proc/$pid/smaps_rollup 2>/dev/null
done | sort -rn | cut -f2-
```

Take the snapshot in one pass, as above. Sampling tools one at a time produces figures
minutes apart, and on this machine processes start, exit, and get swapped fast enough that
the columns stop being comparable.

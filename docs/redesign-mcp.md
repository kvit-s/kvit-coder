# Holding MCP connections outside the turn

## 1. What this document is

`redesign.md` describes a rebuild of kvit-coder in which **the agent is one
operating-system process per turn**: it starts, reads the session from disk,
carries out one instruction from you, appends what happened, and exits. A
driver — a shell script or a small interactive program — starts it again for the
next instruction. Everything with a lifetime longer than a single instruction
lives in the session directory on disk rather than in the agent's memory.

That shape has a consequence for external tool servers, which `redesign.md`
notes at line 141 and treats as an acceptable cost measured in seconds. It is
larger than that, and this document says what it actually is and what to do
about it. The proposal is a separate long-running program that holds the
connections to those servers, so that the agent connecting and disconnecting
every turn stops tearing anything down.

The reader is assumed not to have `redesign.md` open, so the parts of it that
matter here are restated where they come up.

**What this changes in `redesign.md`.** Section 3's bullet on MCP servers
currently concludes that the reconnect cost is a second or two and a reason to
prefer servers reached over HTTP. That conclusion holds for servers that keep no
per-connection state and is wrong for servers that do. Section 15's line keeping
the MCP client unchanged survives intact, and the design below is chosen partly
to keep it that way.

## 2. Background

### 2.1 What an MCP server is, and what kvit-coder does with one

The Model Context Protocol is a way for an external program to offer tools to an
agent over JSON-RPC. kvit-coder has a complete client for it in `internal/mcp/`,
described in full in `mcp-plan.md`. There are two kinds of server:

- **A command-launched server** (the protocol calls this the "stdio"
  transport). kvit-coder runs the configured executable as a child process and
  talks to it over that process's standard input and output. `internal/mcp/stdio.go:47`
  is where it is spawned; line 54 puts it in its own process group, and
  `killGroup` at line 147 kills that group on close.
- **An HTTP server**, already running somewhere, that kvit-coder posts requests
  to (`internal/mcp/http.go`).

Configuration lists servers by name with a transport, a trust policy, a call
timeout, and an optional filter over which of the server's tools to expose
(`internal/config/config.go:110` and `:132`). Each server's tools reach the model
under a name that includes the server it came from — `mcp.<server>.<tool>`, built
by `NamespacedName` at `internal/mcp/client.go:54` — so two servers offering a
tool called `search` do not collide.

At startup the agent dials every enabled server at once, each under its own
20-second deadline, running the protocol's `initialize` handshake followed by
`tools/list` to discover what the server offers (`Manager.Connect`,
`internal/mcp/manager.go:71`). A server that fails is logged and skipped rather
than being fatal. The results are then sorted by server name, so the list of
tools sent to the model is byte-identical between runs and the model provider's
prompt cache keeps hitting. All of this is wired in `cmd/kvit-coder/main.go:397-401`.

**Nothing is configured today.** `mcp.enabled` defaults to false and `config.yaml`
has no `mcp:` section at all, so the cost described below is one nobody is
currently paying. That matters for the timing recommendation in section 9.

### 2.2 What one process per turn does to that

Under the current program the agent process lives for a whole working session,
so `Connect` runs once and `Close` runs when you quit. Under the redesign the
process lives for one instruction, so both run every time you say anything.

For a command-launched server, that means the server process is spawned, made to
handshake, used, and killed — once per instruction. A ten-instruction working
session starts and kills each configured server ten times.

## 3. What that costs

### 3.1 Startup time, which is small

Servers are dialed concurrently, so the delay is the slowest one rather than the
sum. A small command-launched server written in Go starts in milliseconds; one
written in Node or Python, which most of the published ones are, typically takes
between half a second and two seconds before it can answer `tools/list`. The
20-second figure in the configuration is the point at which a server is given up
on, not a typical time.

Half a second to two seconds added to the front of every instruction is
noticeable and is not, on its own, worth building a second program to avoid.

### 3.2 State that does not survive, which is the real cost

The cost that concurrency cannot reduce is that **anything the server was
holding is gone**. A few concrete cases:

- A server that drives a web browser loses the browser: the pages that were
  open, the logins, the scroll position, whatever was half-filled in a form. A
  browsing task that spans more than one instruction cannot work at all.
- A server that indexes a codebase rebuilds its index from nothing each time,
  which for a large repository is far more than the two seconds above.
- A server holding an authenticated session re-authenticates, which in the case
  of an interactive sign-in means it cannot be used.
- A server holding open database or network connections reopens them, paying
  connection setup and losing anything transaction-scoped.

The distinction that matters is not stdio against HTTP, and it is not slow
against fast. It is whether a server keeps anything between calls. A server that
answers each call from scratch — a calculator, a documentation lookup, a
stateless HTTP fetch — loses nothing when it is restarted, and per-turn
restarting costs it only the startup time in 3.1. A server that keeps something
is unusable across turns.

### 3.3 HTTP servers are partly, not wholly, protected

`redesign.md:147` recommends HTTP servers on the grounds that they are separate
programs the agent does not own, so they keep running when the agent exits. That
is true of the program. It is not true of the protocol's own session.

The protocol lets a server issue a session identifier in an `Mcp-Session-Id`
response header, which the client then sends back on every subsequent request.
kvit-coder honours this: `internal/mcp/http.go:31` holds the identifier in a
struct field, and lines 124-126 pick it up from the first response. That field is
in the agent's memory, so it dies with the turn. The next turn handshakes afresh
and is issued a new session.

Whether that costs anything depends on what the server attaches to a session. A
server that uses the identifier only for routing loses nothing. A server that
keeps per-session state — which is the reason the mechanism exists — loses it at
every turn boundary, exactly like a command-launched server, even though its
process never stopped.

### 3.4 Approvals reset every turn

A related loss, worth naming because it is visible immediately rather than only
with stateful servers. MCP tools can reach anything the server process can
reach, so kvit-coder gates them behind a trust policy per server:
`block`, `ask_once`, `ask_always`, or `trust` (`internal/mcp/confirm.go`). The
default, `ask_once`, prompts you the first time a given tool is used and
remembers your answer.

It remembers it in a map in memory, held by the manager (`confirmer.approved`).
Under per-turn processes that map is empty at the start of every instruction, so
`ask_once` becomes ask-once-per-instruction and you are prompted repeatedly for
the same tool. This is an instance of the general problem `redesign.md` section 3
already identifies for session-scoped permission grants, and its general answer —
move the record into the session directory — applies here whether or not the
daemon in section 4 is ever built. The two fixes are independent and both are
worth having.

## 4. The proposal

### 4.1 Shape

Run the MCP connections in a **daemon**: a separate executable that starts once,
connects to the configured servers, keeps them alive, and stays running while
you work. The agent process still starts and exits once per instruction, and it
talks to the daemon rather than to the servers directly.

The key decision is what the agent and the daemon say to each other, and the
answer is: **the daemon speaks MCP**. It exposes each configured server as an
HTTP MCP endpoint of its own, at a path named after that server, and forwards
requests to the real connection it holds. From the agent's point of view nothing
has changed except that every server is now an HTTP server on the local machine.

The consequence is that `internal/mcp/` needs no changes. The client already
supports HTTP servers, already namespaces tools by server, already applies a
per-server trust policy and call timeout and tool filter. Configuration entries
are rewritten in memory at startup — transport becomes `http`, the URL becomes
the daemon's endpoint for that server — and everything downstream is unaware.

```
Configured:                        What the agent connects to:

  name: playwright                   http://127.0.0.1:7391/playwright
  transport: stdio                     ↑
  command: npx                         │  daemon holds the real connection:
  args: [-y, @playwright/mcp]          └─ npx @playwright/mcp  (spawned once,
                                          kept alive across turns)
```

### 4.2 One endpoint per server, not one aggregated endpoint

An obvious simplification is to have the daemon present a single endpoint
offering every tool from every server. It should be rejected, because the agent
distinguishes servers for reasons that survive the change: the trust policy is
per server and is enforced agent-side, so it can prompt you (`newMCPTool` takes
a `confirmPolicy` per server, `internal/mcp/tool.go`); the call timeout is per
server; the tool filter is per server; and a single failed server should degrade
to that server's tools being missing rather than to a partial list from one
endpoint.

Keeping one endpoint per server preserves all of that for free. The daemon needs
only a request multiplexer keyed on path, which in Go is a few lines.

### 4.3 What the daemon adds beyond keeping connections warm

Two things fall out of having a program that owns the servers, both of which are
absent today:

- **Supervision.** A command-launched server that crashes mid-turn currently
  stays dead until the agent process exits and the next turn respawns it. A
  daemon can notice the exit, restart the server, and re-run the handshake, so
  the failure costs one call rather than the rest of the instruction.
- **A place to look.** Server error output currently goes wherever the agent's
  logging sends it, interleaved with everything else and gone when the process
  exits. A daemon keeps a log per server that `kvit-coder mcp logs <name>` can
  show.

## 5. Lifecycle and addressing

### 5.1 Starting it

Do not make starting the daemon something you have to remember. The agent should
attempt to connect; if nothing answers, it starts the daemon detached and
retries. The daemon exits on its own after a period with no connections. This is
how connection sharing in `ssh` and most editor language servers work, and it
means that a bare `kvit-coder -p "..."` typed with no preparation behaves
correctly, with one code path rather than a supervised path and an unsupervised
one.

Starting it detached is machinery that `redesign.md` stage 6 builds anyway, for
the background processes the model itself starts: `setsid`, a file recording the
process identifier, and a pass at the start of each turn that checks whether the
recorded process is still alive. See section 9 on ordering.

### 5.2 Which daemon

A daemon is identified by the workspace it serves and the configuration it was
started with. Both are needed: command-launched servers get a working directory
resolved against the workspace root (`Manager.resolveCwd`,
`internal/mcp/manager.go:262`), so a daemon serving one project cannot serve
another; and a daemon started before you edited the server list is running the
wrong servers.

Hash the workspace path together with the resolved MCP configuration, and use
the hash to name the daemon's record:

```
~/.kvit-coder/mcp/<hash>.json     { port, pid, started, workspace }
~/.kvit-coder/mcp/<hash>.log      daemon's own log
```

Editing the configuration changes the hash, so the next turn finds no daemon at
the new name, starts a fresh one, and the old one exits when its idle timer runs
out. Configuration reloading needs no code of its own, and nothing ever has to
route between workspaces.

### 5.3 Falling back

If the daemon cannot be started or cannot be reached, the agent should connect
to the configured servers directly, exactly as it does now, and warn. That keeps
the daemon an optimisation rather than a dependency, and it means a bug in it
degrades the program to today's behaviour rather than removing MCP support.

## 6. What stays in the agent

Stated explicitly, because the split is the part most likely to be got wrong
later:

| Concern | Where it lives | Why |
|---|---|---|
| Connecting to servers, keeping them alive, restarting them | daemon | It is the only thing that outlives a turn |
| Which servers exist, their names, their tool filters | both — the daemon reads the config, the agent reads it too | The agent needs the names to build endpoint URLs and apply per-server settings |
| Trust policy and prompting you to approve a call | agent | It is the only side with your terminal |
| Approval memory across turns | session directory | Section 3.4; independent of the daemon |
| Tool namespacing, schema sanitizing, result size caps | agent | Unchanged code in `internal/mcp/tool.go` |
| Deterministic tool ordering for prompt caching | agent | Already sorted by server name in `Manager.Connect` |

## 7. What it costs

The daemon is not free, and three of its costs work directly against properties
`redesign.md` values.

**It brings back a process that outlives a wedged turn.** Section 3 of
`redesign.md` lists, as an argument for per-turn processes, that a stuck turn
ends when its process does. A stuck MCP server inside a daemon does not, so the
daemon needs `kvit-coder mcp status`, `kvit-coder mcp restart`, and the idle
timeout, and the agent needs a health check on connect rather than assuming a
listening socket means a working daemon.

**It removes the guarantee that every turn starts clean.** Restarting everything
each turn is crude, and it does mean a server in a bad state is fixed by the next
instruction. Once the daemon holds servers across turns, a browser stuck on a
dialog or an index that went stale stays that way until something clears it. This
is the same trade the proposal is built to make — state surviving is the point —
but it cuts both ways and `mcp restart` is what you reach for.

**Child processes can leak.** If the daemon is killed without running its
shutdown, the servers it spawned are orphaned. The mechanism to prevent this
exists — each server is already in its own process group
(`internal/mcp/stdio.go:54`) and `killGroup` at line 147 kills the group — but
the daemon has to record what it started so that a later run can clean up after
a predecessor that died badly, in the same way section 7 of `redesign.md`
reconciles background processes at turn start.

Two smaller ones: it is a second executable to build, ship, and keep in step
with the agent's version; and the first instruction of a working session still
pays the full startup cost, since something has to start the servers once.

## 8. Access to the daemon

The daemon can invoke every configured MCP tool with no prompt, because the
prompting is agent-side (section 6). Anything on the machine that can reach it
therefore has that ability.

Two ways to close that, in increasing order of strictness and of work:

- **Loopback plus a shared secret.** Bind to `127.0.0.1`, generate a token at
  startup, write it into the daemon's record file with mode 0600, and have the
  agent send it as a request header. This needs no client changes at all: the
  per-server configuration already has a `Headers` field
  (`internal/config/config.go`) that supports `${VAR}` expansion.
- **A Unix domain socket** at mode 0600, which is not reachable by other users
  on the machine at all. This is stricter and needs a small change to
  `internal/mcp/http.go` to dial a socket path rather than a hostname.

The token is the recommended starting point on the grounds that it costs nothing
and the socket can replace it later. On a single-user machine the difference
between the two is small, and it is the same reasoning `redesign.md` applies to
the session inbox directory, which is protected by file permissions rather than
by a protocol.

## 9. When to build it

**Not yet, and here is the test for when.** No MCP servers are configured today
(section 2.1), so there is no cost currently being paid and no way to measure
whether the daemon would help. Building it now would be designing against a
guess.

The trigger is the first server you actually want that keeps state between
calls — a browser driver being the likeliest, a codebase indexer the next. When
one of those goes into the configuration, per-turn restarting stops being a
delay and starts being a reason the server does not work, and the daemon becomes
the thing that makes it usable.

Two cheap things to do before then, both worth having regardless:

1. **Measure the connect time.** Log how long `Manager.Connect` took and which
   server was slowest. One line, and it turns section 3.1 from an estimate into
   an observation after a week of use. `redesign.md` section 12 makes the same
   move for context accounting, for the same reason.
2. **Move the approval memory into the session directory** (section 3.4). This
   is a defect under per-turn processes whether or not the daemon exists.

**Where it goes in the migration.** After stage 6 of `redesign.md`, which builds
detached processes with `setsid`, process-identifier files, and the
reconcile-at-turn-start pass. Building the daemon after that stage reuses all
three; building it before means writing them twice.

## 10. Open questions

- **Which servers do you actually want?** Everything above is shaped by the
  answer, and there is currently no answer. If the servers that end up
  configured are all stateless, the daemon is not worth building and the
  recommendation in `redesign.md:147` to prefer HTTP servers stands as written.
- **Should the daemon restart a crashed server, or report it dead?** Restarting
  is friendlier and hides a server that is crash-looping. A restart count in
  `mcp status`, and giving up after a few attempts within a short window, is
  probably the right shape, but it is a guess until something crashes.
- **How long an idle timeout?** Long enough that it survives you thinking
  between instructions, short enough that a forgotten daemon does not hold a
  browser open overnight. Somewhere in tens of minutes, and worth revisiting
  once there is a daemon to observe.
- **Does a server going stale across turns become the common complaint?** If it
  does, the narrow fix is a per-server `restart_each_turn` setting that opts one
  server back into today's behaviour, rather than abandoning the daemon.
- **Does per-workspace keying multiply daemons unpleasantly?** Working in four
  repositories in one afternoon means four daemons and four sets of servers. The
  idle timeout is the intended answer; whether it is sufficient depends on how
  heavy the configured servers are.

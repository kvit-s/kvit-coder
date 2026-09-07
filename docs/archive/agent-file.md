# Implementation Plan: `agent_file` Option

## Goal

Add an `agent_file` config/CLI option that loads a file (e.g., `AGENT.md`) and appends its
content to the **system prompt** sent to the LLM. The content is invisible to the user
(not displayed in terminal) since the system prompt is never printed.

## How It Works Today

1. Config is loaded from YAML (`internal/config/config.go:217`)
2. System prompt is generated from templates or hardcoded logic (`internal/prompt/prompt.go:62`)
3. In `main.go:313`, `systemPrompt` string is produced
4. In `main.go:414`, it's passed to `repl.RunExec()`
5. In `repl.go:19-21`, the system prompt becomes the first `RoleSystem` message — never displayed
6. The `-p` prompt text is what gets displayed to the user (`repl.go:54-61`)

So appending the agent file content to the system prompt string achieves exactly what we want:
sent to the LLM, invisible to the user.

## Changes

### 1. `internal/config/config.go` — Add config field

Add `AgentFile` to the `Agent` struct (line ~38):

```go
Agent struct {
    MaxIterations int    `yaml:"max_tool_iterations"`
    AgentFile     string `yaml:"agent_file"`    // Path to file whose content is appended to system prompt
} `yaml:"agent"`
```

No defaults needed — empty string means disabled.

### 2. `cmd/kvit-coder/main.go` — Add CLI flag + apply logic

**Add flag** (after line 63, with other flags):

```go
agentFile := flag.String("agent-file", "", "path to agent file (content appended to system prompt)")
```

**Apply flag override** (after line 203, where other overrides happen):

```go
if *agentFile != "" {
    cfg.Agent.AgentFile = *agentFile
}
```

**Append agent file content to system prompt** (after line 317, after `systemPrompt` is generated):

```go
if cfg.Agent.AgentFile != "" {
    agentContent, err := os.ReadFile(cfg.Agent.AgentFile)
    if err != nil {
        log.Fatalf("Failed to read agent file %q: %v", cfg.Agent.AgentFile, err)
    }
    systemPrompt += "\n\n" + string(agentContent)
}
```

That's it. Three small edits across two files. The agent file path is resolved relative to
CWD (consistent with how `-config` works). Fatal on missing file is intentional — if you
specified it, you want it loaded.

### 3. Usage

**In config.yaml:**

```yaml
agent:
  max_tool_iterations: 25
  agent_file: AGENT.md
```

**Or via CLI flag (overrides config):**

```bash
kvit-coder -p "do something" -agent-file AGENT.md
```

**Both together (CLI wins):**

```bash
kvit-coder -p "do something" -config config.yaml -agent-file custom-agent.md
```

## What's NOT Needed

- No changes to `prompt.go` — we append after generation, keeping prompt logic clean
- No changes to `repl.go` — system prompt is already hidden from user
- No changes to `llm/` — the system message content is just a longer string
- No new packages or dependencies

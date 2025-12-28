# kvit-coder

A minimal coding agent in Go with OpenAI-compatible LLM support and pluggable tools.

## Architecture

The application is split into two binaries:

- **`kvit-coder`** - Headless agent engine for scripting and automation
- **`kvit-coder-ui`** - Interactive terminal UI that orchestrates `kvit-coder`

## Quick Start

### Build

```bash
# Build both binaries
go build -o kvit-coder ./cmd/kvit-coder
go build -o kvit-coder-ui ./cmd/kvit-coder-ui
```

Release build (optimized with embedded version info):

```bash
./scripts/release.sh
```

### Configure

Edit `config.yaml` to set your LLM endpoint:

```yaml
llm:
  base_url: "http://127.0.0.1:8080/v1"  # Your OpenAI-compatible endpoint
  api_key_env: "OPENAI_API_KEY"          # Environment variable for API key
  model: "gpt-oss-20b"                   # Model name
  temperature: 0.2
  max_output_tokens: 2048
```

### Run

**Interactive mode (kvit-coder-ui):**

```bash
./kvit-coder-ui
```

**Headless mode (kvit-coder):**

```bash
# Run a single prompt
./kvit-coder -p "fix the failing tests"

# Quiet mode (final answer only to stdout)
./kvit-coder -pq "what is 2+2"

# JSON output mode
./kvit-coder -p "list the files" --json
```

### Testing

Run the test suite:

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test ./... -cover

# Generate detailed coverage report
go test ./internal/... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

## kvit-coder (Headless Agent)

The headless agent is designed for scripting and automation. It requires either `-p` (prompt) or `--benchmark` mode.

### Output Protocol

```
stdout: Final LLM response only (the answer)
stderr: Progress, tool outputs, status, errors

Exit codes:
  0 = success
  1 = error (LLM error, tool error, etc.)
  2 = user interrupt (SIGINT)
```

This allows piping the final answer:

```bash
./kvit-coder -p "summarize this file" > summary.txt
```

### Flags

| Flag | Description |
|------|-------------|
| `-p <prompt>` | Run with this prompt and exit |
| `-pq <prompt>` | Quiet mode: only print final answer to stdout |
| `--json` | Output structured JSON messages |
| `-s <name>` | Session name: continue existing or create new |
| `--config <path>` | Config file path (default: config.yaml) |
| `--model <name>` | Override model name |
| `--base-url <url>` | Override LLM base URL |
| `--log <path>` | Log file path (default: kvit-coder.log) |
| `--benchmark` | Run benchmark mode |
| `--sessions` | List all sessions and exit |
| `--session-show <name>` | Show session history and exit |
| `--session-delete <name>` | Delete a session and exit |

### JSON Output Mode

With `--json`, all progress output is suppressed and a single JSON object is output to stdout:

```bash
./kvit-coder -p "hello" --json > result.json
```

**JSON output format:**

```json
{
  "content": "The final LLM response text...",
  "stats": {
    "session": "2024-01-15-abc123",
    "prompt_tokens": 1234,
    "completion_tokens": 567,
    "total_tokens": 1801,
    "cache_read_tokens": 500,
    "total_cost_usd": 0.0025,
    "cache_discount_usd": 0.0005,
    "duration_ms": 3500,
    "steps": 3
  }
}
```

## kvit-coder-ui (Interactive UI)

The interactive UI provides a readline-based interface with session management.

```bash
./kvit-coder-ui
```

### UI Commands

| Command | Description |
|---------|-------------|
| `:help`, `:h` | Show help |
| `:quit`, `:q` | Exit |
| `:new` | Start a new session |
| `:switch <name>` | Switch to an existing session |
| `:sessions` | List all sessions |
| `:history` | Show current session history |
| `:clear` | Clear the terminal |
| `:config` | Show configuration |

### Flags

| Flag | Description |
|------|-------------|
| `-s <name>` | Start with this session |
| `--config <path>` | Config file path |
| `--agent-path <path>` | Path to kvit-coder binary |
| `--sessions` | List all sessions and exit |
| `--session-show <name>` | Show session history and exit |
| `--session-delete <name>` | Delete a session and exit |

## Sessions

Sessions allow you to persist conversation history and continue previous conversations.

### Basic Usage

```bash
# Exec mode auto-generates a session name
./kvit-coder -p "fix the failing tests"
# Output ends with: Session: 2024-01-15-x7k9m2

# Continue the session
./kvit-coder -p "the login test is still failing" -s 2024-01-15-x7k9m2

# Use a named session from the start
./kvit-coder -p "implement user auth" -s auth-feature

# Continue named session
./kvit-coder -p "add password reset" -s auth-feature
```

### Interactive Mode with Sessions

```bash
# Start interactive mode with existing session history
./kvit-coder-ui -s auth-feature

# Start fresh interactive mode (no session)
./kvit-coder-ui
```

### Session Management

```bash
# List all sessions
./kvit-coder --sessions
# or
./kvit-coder-ui --sessions

# View session history
./kvit-coder --session-show my-feature

# Delete a session
./kvit-coder --session-delete my-feature
```

### Storage

Sessions are stored as JSONL files in `~/.kvit-coder/sessions/`:

```
~/.kvit-coder/sessions/
├── my-feature.jsonl
├── 2024-01-15-abc123.jsonl
└── 2024-01-15-def456.jsonl
```

Auto-generated session names use the format `YYYY-MM-DD-<random6>`.

## Available Tools

Tools are disabled by default and must be explicitly enabled in `config.yaml`:

**Core Tools:**
- `read` - Read file contents or list directories
- `edit` - Edit or create files by line range
- `search` - Search for code patterns with ripgrep
- `shell` - Execute shell commands from the workspace

**Group Tools (enable all at once):**
- `plan.*` - Plan management tools (plan.create, plan.complete_step, plan.add_step, plan.remove_step, plan.move_step)
- `checkpoint.*` - Checkpoint tools (checkpoint.list, checkpoint.restore, checkpoint.diff, checkpoint.undo)

**Conditional Tools:**
- `restore_file` - Requires `restore_file.enabled: true` and checkpoint infrastructure
- `edit.confirm` / `edit.cancel` - Available when `edit.enabled: true` AND `edit.preview_mode: true`

### Tool Configuration

Enable tools in `config.yaml`:

```yaml
tools:
  # All tools are disabled by default - explicitly enable the ones you want

  read:
    enabled: true
    max_file_size_kb: 128
    max_read_size_kb: 24
    max_partial_lines: 150

  edit:
    enabled: true
    max_file_size_kb: 128
    preview_mode: false         # enables edit.confirm/edit.cancel
    read_before_edit_msgs: 0

  restore_file:
    enabled: false

  search:
    enabled: true

  shell:
    enabled: true
    allowed_commands: []        # empty = allow all

  plan:
    enabled: false              # group toggle for all plan.* tools

  checkpoint:
    enabled: false              # group toggle for all checkpoint.* tools
    max_turns: 100
    max_file_size_kb: 1024
    excluded_patterns: []
```

### Adding New Tools

Implement the `Tool` interface:

```go
type Tool interface {
    Name() string                                           // Tool identifier (e.g., "shell", "read")
    Description() string                                    // For LLM
    JSONSchema() map[string]any                            // OpenAI function schema
    Check(ctx context.Context, args json.RawMessage) error // Validation
    Call(ctx context.Context, args json.RawMessage) (any, error) // Execution
    PromptSection() string                                 // System prompt docs
    PromptCategory() string                                // "filesystem", "shell", "plan", "checkpoint"
    PromptOrder() int                                      // Sort order within category
}
```

Then enable in main.go:
```go
myTool := tools.NewMyTool(cfg)
registry.Enable(myTool)
```

## Compatible LLM Providers

The agent works with any OpenAI-compatible API endpoint:

- **OpenRouter**: `https://openrouter.ai/api/v1`
- **llama.cpp server**: `http://localhost:8080/v1`
- **Local models**: Any server implementing the OpenAI chat completions API

## Benchmarks

The agent includes a benchmarking system for testing LLM tool usage precision, recovery, and consistency.

### List Available Benchmarks

```bash
./kvit-coder --benchmark-list
```

Output:
```
Available benchmarks (24 total):

## SEARCH
  S1 - Simple Pattern Search
  S2 - Multi-Pattern Search
  ...

## READ
  R1 - Simple File Read
  ...

## COMPOUND
  C5 - Needle in Haystack Search
  C6 - Multi-Step Definition Tracing
```

### Run Benchmarks

```bash
# Run all benchmarks with 10 runs each (default)
./kvit-coder --benchmark .

# Run with custom number of runs
./kvit-coder --benchmark . -n 5

# Run specific category
./kvit-coder --benchmark . -n 5 --benchmark-category search

# Run specific benchmark(s)
./kvit-coder --benchmark . -n 10 --benchmark-id S1
./kvit-coder --benchmark . -n 10 --benchmark-id S1,S2,R1

# Output to specific file
./kvit-coder --benchmark . -n 10 -o results/my-benchmark.md

# Force fresh start (ignore previous results)
./kvit-coder --benchmark . -n 10 --no-resume
```

### Benchmark Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--benchmark <suffix>` | Enable benchmark mode (use `.` for no suffix) | - |
| `-n` | Number of runs per benchmark | 10 |
| `--benchmark-category` | Filter by category (comma-separated) | all |
| `--benchmark-id` | Run specific benchmark IDs (comma-separated) | all |
| `-o` | Output file path | `.kvit-coder-benchmark/results/benchmark-{timestamp}.md` |
| `--no-resume` | Force fresh start, ignore existing CSV | false |

### Output

Benchmarks produce:
- **Markdown report** (`-o` path): Summary table, per-benchmark statistics, failure analysis
- **CSV file** (same path with `.csv`): Raw data for each run, enables resume on interrupt

Example report excerpt:
```markdown
## Summary

| Benchmark | Success Rate | Avg LLM Calls | Avg Tokens | Avg Cost | Avg Duration |
|-----------|--------------|---------------|------------|----------|--------------|
| S1        | 100%         | 1.0           | 1,234      | $0.002   | 1.2s         |
| S2        | 90%          | 1.3           | 2,456      | $0.004   | 2.1s         |
```

### Adding Custom Benchmarks

Define benchmarks in `benchmarks.yaml`:

```yaml
benchmarks:
  - id: MY1
    name: "My Custom Benchmark"
    category: custom
    goal: "Test something specific"
    readonly: true  # Optional: setup once, block write tools
    setup:
      - file: "test.go"
        content: |
          package main
          func myFunc() {}
    task: "Find the function named myFunc"
    validation:
      - type: output_contains
        expected: "test.go"
      - type: tool_called
        expected: "search"
    tags: ["custom"]
```

**Benchmark Options:**
- `readonly: true` - Setup files once (reused across runs), only allow read-only tools:
  - `search`, `read` tools
  - Shell commands: `grep`, `rg`, `find`, `ls`, `cat`, `head`, `tail`, `wc`, etc.
  - Fails if LLM calls `edit`, `write`, or destructive shell commands

**Validation types:** `file_contains`, `file_equals`, `file_exists`, `file_not_exists`, `file_line_count`, `tool_called`, `tool_called_with`, `output_contains`, `output_not_contains`, `output_matches`, `multi_tool_calls`

## License

MIT

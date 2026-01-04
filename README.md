# kvit-coder

A benchmarking framework for evaluating LLM coding agents on tool-use tasks.

## Overview

This project provides a standardized benchmark suite to measure LLM performance on coding agent tasks including file operations, code search, and multi-step problem solving. It's designed to test local/self-hosted models against a consistent set of challenges.

## Quick Start

### Build

```bash
go build -o kvit-coder ./cmd/kvit-coder
```

### Configure

Create a config file for your model (e.g., `config-mymodel.yaml`):

```yaml
llm:
  base_url: "http://127.0.0.1:8080/v1"
  api_key_env: "OPENAI_API_KEY"
  model: "my-model-name"
  temperature: 0.2
  max_output_tokens: 2048

tools:
  read:
    enabled: true
  edit:
    enabled: true
  search:
    enabled: true
  shell:
    enabled: true
```

### Run Benchmarks

```bash
# Run all benchmarks (10 runs each by default)
./kvit-coder --benchmark mymodel

# Custom number of runs
./kvit-coder --benchmark mymodel -n 5

# Run specific category
./kvit-coder --benchmark mymodel --benchmark-category search

# Run specific benchmark IDs
./kvit-coder --benchmark mymodel --benchmark-id S1,S2,R1
```

### List Available Benchmarks

```bash
./kvit-coder --benchmark-list
```

## Benchmark Categories

| Category | Description |
|----------|-------------|
| **search** | Code pattern search with ripgrep |
| **read** | File reading and directory listing |
| **edit** | File creation and modification |
| **shell** | Shell command execution |
| **compound** | Multi-step tasks combining multiple tools |

## Output Files

Each benchmark run produces:

| File | Description |
|------|-------------|
| `benchmark-{name}-{timestamp}.md` | Markdown report with summary tables and statistics |
| `terminal-{name}-{timestamp}.txt` | Full terminal output from the run |
| `.kvit-coder-benchmark/benchmark-{timestamp}.csv` | Raw CSV data (enables resume on interrupt) |

## Benchmark Flags

| Flag | Description | Default |
|------|-------------|---------|
| `--benchmark <name>` | Run benchmarks using `config-{name}.yaml` | - |
| `-n <count>` | Number of runs per benchmark | 10 |
| `--benchmark-category <cat>` | Filter by category (comma-separated) | all |
| `--benchmark-id <ids>` | Run specific benchmark IDs (comma-separated) | all |
| `-o <path>` | Output file path | auto-generated |
| `--no-resume` | Force fresh start, ignore existing CSV | false |

## External LLM Support

To benchmark an external tool (like Claude Code), use `benchmark_cmd` in your config:

```yaml
llm:
  benchmark_cmd: "claude -p {prompt} --allowedTools Edit Bash Read"
```

The `{prompt}` placeholder is replaced with the benchmark task.

## Results

Benchmark results are stored in the `benchmarks/` directory. See [benchmarks/README.md](benchmarks/README.md) for details on interpreting results.

## Adding Custom Benchmarks

Define benchmarks in `benchmarks/benchmarks.yaml`:

```yaml
benchmarks:
  - id: MY1
    name: "My Custom Benchmark"
    category: custom
    goal: "Test something specific"
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
```

**Validation types:** `file_contains`, `file_equals`, `file_exists`, `file_not_exists`, `file_line_count`, `tool_called`, `tool_called_with`, `output_contains`, `output_not_contains`, `output_matches`, `multi_tool_calls`

## Architecture

The agent consists of two binaries:

- **`kvit-coder`** - Headless agent for benchmarking and automation
- **`kvit-coder-ui`** - Interactive terminal UI (not used for benchmarking)

## License

MIT

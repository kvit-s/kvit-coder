# Benchmark Results

This directory contains benchmark results for various LLM models tested on the kvit-coder agent benchmark suite.

## File Naming Convention

| Pattern | Description |
|---------|-------------|
| `benchmark-{name}-{timestamp}.md` | Markdown report with summary and detailed statistics |
| `terminal-{name}-{timestamp}.txt` | Full terminal output captured during the benchmark run |
| `config-{name}.yaml` | Configuration for benchmark run (LLM endpoint, tools enabled) |
| `benchmarks.yaml` | Benchmark definitions (tasks, validation criteria) |

The `{name}` is a benchmark identifier that typically correlates with a model family, but the actual model used is recorded in the report's config section. A config may be updated with different model variants (e.g., different quantizations) between runs.

## Report Structure

Each markdown report contains:

1. **Metadata** - Version, date, benchmark count
2. **Summary Table** - Success rate, avg tokens, avg cost, avg duration per benchmark
3. **Detailed Statistics** - Per-benchmark breakdown with standard deviation
4. **Failure Analysis** - Common failure patterns and error messages
5. **Configuration** - Full config.yaml used for the run

## Interpreting Results

- **Success Rate**: Percentage of runs that passed all validation checks
- **Avg LLM Calls**: Number of agent turns (lower is better for efficiency)
- **Avg Tokens**: Total tokens used (prompt + completion)
- **Avg Duration**: Wall-clock time per benchmark run

## Running New Benchmarks

```bash
# From repository root
./kvit-coder --benchmark {model-name} -n 10
```

Results will be saved to this directory automatically.

# thinkbench — task catalog

72 coding-agent tasks across 5 types (60 graded by a hidden behavioral grader, 12 ambiguous-spec tasks observed without grading).


## implement (15)

_Greenfield: build a project from scratch from a spec._


### budgetrules · 17 checks

````
Start from an empty repository and implement a Python 3.11+ project named `budgetrules`.

Build a rule-based personal finance transaction categorizer. Use only the Python standard library.

Expose:

```python
def categorize(transactions: list[dict], rules: list[dict]) -> list[dict]: ...
def summarize(categorized: list[dict]) -> dict: ...
```

Transaction format:

```json
{
  "id": "txn_1",
  "date": "2026-01-01",
  "description": "ACME GROCERY",
  "amount_cents": -5234
}
```

Rules support:

```text
description_contains
description_regex
amount_min_cents
amount_max_cents
merchant_equals
set_category
set_tags
priority
```

Highest priority matching rule wins. Ties are broken by rule order. If no rule matches, category is `uncategorized`.

Summaries must total spending by category and month. Refunds should reduce spending.

Include a CLI:

```bash
python -m budgetrules categorize --transactions txns.json --rules rules.json
python -m budgetrules summarize categorized.json
```

Include tests for rule priority, regex matching, refunds, uncategorized transactions, monthly summaries, deterministic order, and malformed rules.

## Contract

This section pins the parts the prose above leaves implicit. The held-out grader
checks BEHAVIOR against this contract; it does not require any particular internal
file layout, helper names, or extra keys beyond those pinned here.

### Import path & CLI

- The package is importable as `budgetrules`, and the two functions are exposed from
  the module `budgetrules.public`:

  ```python
  from budgetrules.public import categorize, summarize
  ```

- The CLI is invoked as `python -m budgetrules ...` and MUST emit JSON to stdout:

  ```bash
  python -m budgetrules categorize --transactions txns.json --rules rules.json
  python -m budgetrules summarize categorized.json
  ```

  `categorize` prints the JSON list returned by `categorize(...)`; `summarize` reads
  a JSON file holding that list and prints the JSON dict returned by `summarize(...)`.

### `categorize(transactions, rules) -> list[dict]`

- Returns a NEW list, same length and SAME ORDER as `transactions` (deterministic;
  input is never reordered or mutated).
- Each output element preserves the input transaction's existing fields (`id`,
  `date`, `description`, `amount_cents`) and ADDS exactly these two fields:
  - `category` — a `str`. Set from the winning rule's `set_category`. If no rule
    matches, it is the literal string `"uncategorized"`.
  - `tags` — a `list[str]`. Set from the winning rule's `set_tags` (a list of
    strings). If the winning rule has no `set_tags`, or no rule matches, it is `[]`
    (empty list, never `None`).

- Rule matching: a rule matches a transaction when ALL of its present conditions
  hold (conditions absent from the rule are not constraints). Conditions:
  - `description_contains` — substring test against the transaction `description`,
    CASE-INSENSITIVE.
  - `description_regex` — `re.search` of the pattern against `description`
    (case-sensitive unless the pattern itself opts out). A malformed/uncompilable
    regex does NOT raise: such a rule simply fails to match.
  - `amount_min_cents` — matches when `amount_cents >= amount_min_cents`.
  - `amount_max_cents` — matches when `amount_cents <= amount_max_cents`.
  - `merchant_equals` — exact, case-sensitive equality against the transaction
    `description` (the merchant string).

- Winner selection: among all matching rules, the one with the highest `priority`
  wins. `priority` defaults to `0` when absent. Ties (equal priority) are broken by
  RULE ORDER — the earliest such rule in `rules` wins.

- Robustness: a malformed rule (not a dict, bad regex, non-string/non-list field
  values, unknown extra keys) must NEVER raise — it either fails to match or
  contributes no constraint, and categorization continues over the remaining rules.

### `summarize(categorized) -> dict`

Operates on the list returned by `categorize`. Returns a dict with exactly these two
top-level keys, each a dict whose VALUES are integer cent totals:

- `"by_category"` — maps each `category` string present in the input to the total
  spending for that category, in cents.
- `"by_month"` — maps each `"YYYY-MM"` month string (the first 7 chars of `date`)
  to the total spending for that month, in cents.

Spending convention (pin): a transaction's contribution to a total is the amount of
money that LEFT the account, i.e. `-amount_cents` (debits, stored as NEGATIVE
`amount_cents`, are POSITIVE spending; refunds/credits, stored as POSITIVE
`amount_cents`, are NEGATIVE spending and therefore REDUCE the relevant totals). All
totals are integers in cents. A category/month with a net-zero total still appears
if at least one transaction contributed to it.

## ASSUMPTIONS (pinned so the grader never grades a guess)

- `set_tags`, when present on the winning rule, replaces tags wholesale (tags are
  not accumulated across multiple matching rules — only the winner's tags apply).
- `description_contains` is case-insensitive; `merchant_equals` is case-sensitive.
- Spending sign convention is `-amount_cents` (debits negative in the input). This
  makes "refunds reduce spending" hold and keeps per-category/per-month totals as
  net spending in cents.
- Month key is `date[:7]` (the `YYYY-MM` prefix of the ISO date string).
````

**Graded behaviors:**

- `categorize_basic_match` — a matching rule sets category + tags on the txn
- `categorize_preserves_order_len` — categorize returns same length & order as input, fields preserved
- `categorize_uncategorized` — no matching rule -> category 'uncategorized', tags []
- `tags_type_is_list` — tags is always a list (and [] when the winner sets none)
- `priority_highest_wins` — highest-priority matching rule wins over a lower one
- `priority_tie_rule_order` — equal priority is broken by rule order (earliest wins)
- `priority_default_zero` — absent priority defaults to 0 (a priority>0 rule beats it)
- `regex_match` — description_regex matches via re.search
- `amount_range_match` — amount_min_cents / amount_max_cents bound matching
- `merchant_equals_match` — merchant_equals does exact-equality matching
- `malformed_rule_no_crash` — a malformed rule (bad regex / non-dict) never raises
- `summarize_by_category` — summarize totals spending per category
- `summarize_by_month` — summarize totals spending per YYYY-MM month
- `summarize_refunds_reduce` — refunds (positive amount_cents) REDUCE spending
- `no_input_mutation` — categorize does not mutate the input transactions
- `cli_categorize_json` — `python -m budgetrules categorize` emits JSON
- `cli_summarize_json` — `python -m budgetrules summarize` emits JSON


### cachelab · 10 checks

````
Start from an empty repository and implement a Python 3.11+ project named `cachelab`.

Build an in-memory cache simulator with TTLs, stale-while-revalidate behavior, and per-key stampede protection. Use only the Python standard library.

Expose:

```python
class Cache:
    def __init__(self, clock=None): ...
    def get(self, key: str, loader, ttl_seconds: int, stale_seconds: int = 0): ...
    def invalidate(self, key: str) -> None: ...
    def stats(self) -> dict: ...
```

`loader` is a callable used to compute the value on cache miss. If many threads request the same expired key concurrently, only one loader call should run for that key. Other threads should wait or receive a stale value when allowed by `stale_seconds`.

Different keys must not block each other.

Provide a fake clock for deterministic tests.

Include tests for cache hits, misses, TTL expiration, stale values, per-key locking, concurrent requests, invalidation, loader exceptions, and statistics.

Include a small CLI simulator:

```bash
python -m cachelab simulate scenario.json
```

The simulator should print JSON stats.

## Contract

- Expose `Cache` from `cachelab.public` (a `python -m cachelab simulate` CLI is also required).
- `Cache(clock=None)`: when provided, `clock` is a zero-argument callable returning the current time as a float in seconds; use it for all TTL/stale timing so tests can drive time deterministically.
- `stats()` returns a `dict` of integer counters reflecting cache activity (which counters you track is your choice).
- On a cold or expired key, exactly one `loader` call runs even under many concurrent `get`s for that key; a loader that raises must not cache a value (the next `get` re-runs the loader).
- The CLI `simulate` prints a JSON object of stats.
````

**Graded behaviors:**

- `hit` — second get within TTL is a cache hit (loader runs once)
- `miss` — first get on a cold key runs the loader (miss)
- `ttl_expiry` — value expires after ttl_seconds and reloads
- `stale` — stale-while-revalidate serves the old value during refresh
- `per_key_single_flight` — exactly one loader runs for a hot key under N concurrent gets
- `independent_keys` — different keys don't block each other; each loads independently
- `invalidate` — invalidate() forces the next get to reload
- `loader_exception` — a raising loader propagates and does not cache a value
- `stats` — stats() reports activity; hits and misses move independently
- `cli_simulate_json` — `python -m cachelab simulate scenario.json` prints JSON stats


### cellsim · 24 checks

````
Start from an empty repository and implement a Python 3.11+ project named `cellsim`.

Build a small spreadsheet calculation engine. Use only the Python standard library.

Expose these functions from `cellsim.public`:

```python
def load_sheet(path: str) -> dict: ...
def evaluate_sheet(sheet: dict) -> dict: ...
def get_cell_value(sheet: dict, cell: str) -> object: ...
def explain_cell(sheet: dict, cell: str) -> dict: ...
```

A sheet is JSON:

```json
{
  "cells": {
    "A1": 10,
    "A2": 20,
    "A3": "=A1+A2",
    "B1": "=SUM(A1:A3)",
    "B2": "=IF(B1>40,\"high\",\"low\")"
  }
}
```

Support: cell references; integer and decimal numbers; strings; + - * /; parentheses; SUM(range); MIN(range); MAX(range); AVG(range); IF(condition, true_value, false_value); comparison operators = != < <= > >=.

Detect and report circular references. Missing cells should evaluate as `0` only inside numeric formulas, but should be reported as missing when directly requested. Ranges must work for rectangular regions such as `A1:C3`. Formula evaluation must be deterministic and must not use Python `eval`.

Include a CLI:

```bash
python -m cellsim eval sheet.json
python -m cellsim cell sheet.json B2
python -m cellsim explain sheet.json B2
```

Include tests for arithmetic precedence, ranges, strings, nested formulas, cycle detection, missing cells, and error propagation.

## Contract

This section PINS the exact shapes the grading oracle relies on. It does not add
new behavior beyond the spec above; it only removes ambiguity so that a correct
implementation is not unfairly failed for choosing a different key name or type.

### Import path and CLI

- The public API is importable as `cellsim.public` (i.e. a package `cellsim`
  containing `public.py`). The four functions above are module-level callables.
- A CLI is runnable as `python -m cellsim` (i.e. the package has a `__main__.py`).
  All CLI output is a single JSON document printed to stdout.

### Value types

- A cell value is one of: an `int`, a `float`, a `str`, or a `bool`.
- Integer-valued numbers SHOULD be returned as `int` (e.g. `10`, `30`), and
  decimal numbers as `float` (e.g. `2.5`). `10 / 4` is `2.5`. The oracle compares
  numbers by numeric value with a small tolerance, so `30` and `30.0` are both
  accepted where a number is expected; bools are NOT accepted where a number is.
- Comparison operators (`=`, `!=`, `<`, `<=`, `>`, `>=`) and `IF` conditions
  produce/consume a boolean. `IF(cond, a, b)` returns `a` when `cond` is truthy,
  else `b`.

### `load_sheet(path) -> dict`

- Returns the parsed sheet dict. It has a `"cells"` key mapping cell names
  (e.g. `"A1"`) to raw values (number, string literal, or a `"="`-prefixed
  formula string). `load_sheet` does NOT evaluate; it only parses JSON.

### `evaluate_sheet(sheet) -> dict`

- Returns a dict with a `"cells"` key: a mapping from every cell name present in
  the input to its evaluated value (using the Value types above).
- A cell that cannot be evaluated because it participates in a circular
  reference is reported via a top-level `"errors"` key: a mapping from cell name
  to an error descriptor. An error descriptor is a dict that contains a `"type"`
  key whose value is the string `"circular"` for cycles. (Other error types may
  use other `"type"` strings.) A cell in `"errors"` need not also appear with a
  numeric value in `"cells"`.
- When the sheet has no errors, `"errors"` is either absent or an empty mapping.
- Evaluation is deterministic: repeated calls on the same sheet return equal
  results.

### `get_cell_value(sheet, cell) -> object`

- Returns the evaluated value of `cell` (a Value type) when the cell exists.
- When `cell` is NOT present in the sheet, this is a direct request for a missing
  cell and MUST be reported as missing — NOT silently coerced to `0`. "Reported
  as missing" means EITHER raising `KeyError` OR returning `None`. (The `0`
  coercion for missing cells applies ONLY when a missing cell is referenced from
  inside another cell's numeric formula, never to a direct `get_cell_value`.)
- When `cell` participates in a circular reference, this raises an exception OR
  returns an error descriptor dict carrying `"type": "circular"`.

### `explain_cell(sheet, cell) -> dict`

- Returns a dict describing how `cell` was computed. It MUST contain:
  - `"cell"`: the cell name (str), echoing the requested cell.
  - `"value"`: the evaluated value (a Value type), OR `None` / omitted when the
    cell is missing or errored.
  - `"references"`: a list (possibly empty) of the cell names this cell directly
    depends on. For a literal (non-formula) cell this is `[]`. For `=A1+A2` it is
    `["A1","A2"]` (order not significant; the oracle compares as a set). A range
    like `A1:A3` contributes its expanded member cells `["A1","A2","A3"]`.
- For a cell in a circular reference, `explain_cell` either includes
  `"type": "circular"` somewhere in the returned dict, or sets `"value"` to
  `None`; it MUST NOT raise.

### CLI

- `python -m cellsim eval sheet.json` prints the JSON of `evaluate_sheet`
  (a JSON object; the oracle only requires that stdout parses as JSON).
- `python -m cellsim cell sheet.json B2` prints a JSON document for the single
  cell value.
- `python -m cellsim explain sheet.json B2` prints the JSON of `explain_cell`.

### Ranges and operators

- `SUM`, `MIN`, `MAX`, `AVG` take a single range argument like `A1:C3` and
  operate over the rectangular block of cells (columns A..C, rows 1..3).
- Inside numeric formulas, a referenced cell that is absent contributes `0`.
- Strings are written with double quotes inside formulas: `"high"`.
````

**Graded behaviors:**

- `arith_precedence` — arithmetic precedence: =2+3*4 -> 14, =(2+3)*4 -> 20
- `cell_refs_decimal` — cell references resolve; decimal division yields a float
- `sub_div` — subtraction and division over cell references
- `sum_range` — SUM over a column range A1:A3
- `min_max_avg` — MIN/MAX/AVG over a range
- `rect_range` — ranges cover rectangular regions (A1:C2)
- `strings` — string literals and string-valued references round-trip
- `if_comparison` — IF with > comparison selects the right branch
- `comparison_ops` — comparison operators =, !=, <=, >= each work
- `nested_formulas` — nested/transitive formula chains resolve
- `missing_in_formula_zero` — a missing cell inside a numeric formula is 0
- `direct_missing_reported` — a directly requested missing cell is reported, not 0
- `get_cell_present` — get_cell_value returns an existing cell's value
- `cycle_evaluate` — evaluate_sheet detects & reports a circular reference (no crash)
- `cycle_get_cell` — get_cell_value reports a circular reference (raise or descriptor)
- `explain_refs` — explain_cell lists a cell's direct references
- `explain_range_expands` — explain_cell expands a range into member cells
- `explain_value` — explain_cell returns the cell name and computed value
- `explain_cycle_no_raise` — explain_cell on a cyclic cell does not raise
- `determinism` — evaluate_sheet is deterministic across repeated runs
- `no_python_eval` — formulas are parsed, not run through Python eval()
- `cli_eval_json` — `python -m cellsim eval` emits JSON
- `cli_cell_value` — `python -m cellsim cell` emits JSON carrying the value
- `cli_explain_json` — `python -m cellsim explain` emits JSON


### confstack · 20 checks

````
Start from an empty repository and implement a Python 3.11+ project named `confstack`.

Build a configuration loader with strict precedence. Use only the Python standard library.

Expose:

```python
def load_config(defaults: dict, config_file: str | None, env: dict, cli_args: list[str]) -> dict: ...
```

Precedence order:

```text
CLI flags > environment variables > config file > defaults
```

Config files are JSON. Environment variables use prefix `APP_`. CLI flags use `--key value`, `--nested.key value`, and booleans like `--feature-enabled` or `--no-feature-enabled`.

Infer booleans, integers, and strings. Support nested keys with dot notation.

Include a CLI:

```bash
python -m confstack show --defaults defaults.json --config app.json -- --port 9000 --no-debug
```

Include tests for precedence, nested keys, boolean false overriding true, missing config file, unknown flags, type inference, and deterministic output.

## Contract

This section pins the conventions the grader relies on. A correct implementation
MUST satisfy everything below. Where the SPEC above leaves a convention open, the
choice is pinned here so a defensible implementation is not failed on a guess.

### Import path and CLI
- The public API is importable as `confstack.public` and exposes the function
  `load_config(defaults, config_file, env, cli_args)` with the signature above.
- The CLI is invokable as `python -m confstack` (i.e. the package has a
  `__main__` module). The `show` subcommand prints the merged config and exits 0.

### Return value of `load_config`
- Returns the merged configuration as a plain nested `dict`.
- Dot-notation keys are EXPANDED into nested dicts in the returned value. A key
  written `a.b.c` (from any source — config dot-key, env, or CLI) becomes
  `{"a": {"b": {"c": <value>}}}` in the result. The literal dotted string is NOT
  retained as a flat top-level key.
- Output is deterministic: calling `load_config` twice with equal inputs returns
  equal dicts, and the CLI's serialized output is byte-for-byte stable across runs
  for equal inputs.

### Precedence (strict, highest wins)
```text
CLI flags > environment variables > config file > defaults
```
- Sources are merged so that, for any given leaf key, the value from the
  highest-precedence source that defines it wins.
- Merging is RECURSIVE over nested dicts (a "deep merge"): a higher-precedence
  source overriding `a.b` does NOT discard a lower-precedence `a.c`; both survive
  under `a`. Only the overlapping leaf is overridden.
- Boolean `false` from a higher-precedence source DOES override boolean `true`
  from a lower-precedence source (and vice versa). Presence, not truthiness,
  decides an override.

### Defaults
- `defaults` is a Python dict supplied by the caller. It may itself contain nested
  dicts and/or dot-notation keys; dot-notation keys in `defaults` are expanded the
  same way as every other source.

### Config file
- `config_file` is a path to a JSON object, or `None`.
- If `config_file` is `None`, the config-file layer contributes nothing.
- A missing config-file path (file does not exist) is NOT a hard error in
  `load_config`: the config-file layer simply contributes nothing and lower layers
  still apply. (It must not raise; it must not abort the merge.)
- A config file whose JSON contains dot-notation keys has them expanded.

### Environment variables
- Only variables whose name starts with the prefix `APP_` are consumed; all other
  env vars are ignored.
- The prefix `APP_` is stripped, and the REMAINDER is lowercased to form the key.
  So `APP_PORT` -> key `port`, `APP_DEBUG` -> key `debug`.
- Nested keys in env use a double underscore `__` as the nesting separator:
  `APP_DATABASE__HOST` -> `database.host` -> `{"database": {"host": ...}}`.
  (Single underscores within a segment are preserved as part of that key segment,
  e.g. `APP_LOG_LEVEL` -> key `log_level`.)
- Env values are strings and are passed through type inference (below).

### CLI flags (the `cli_args` list, and the CLI's post-`--` args)
- `--key value` sets `key` to `value`.
- `--nested.key value` sets the dotted key `nested.key` (expanded to nested dicts).
- `--flag` (no following value, or followed by another `--…` token) sets `flag`
  to boolean `true`.
- `--no-flag` sets `flag` to boolean `false`. The `no-` prefix is stripped from
  the FIRST segment of the key only; e.g. `--no-feature-enabled` sets
  `feature-enabled` to `false` (only the leading `no-` is removed, not later
  hyphens), and `--no-a.b` sets `a.b` to `false`.
- Flag key segments keep their hyphens: `--feature-enabled` -> key
  `feature-enabled` (hyphens are NOT converted to underscores).
- "Unknown" flags are not an error: there is no fixed schema, so any `--key`
  is accepted and merged. (The SPEC's "unknown flags" test means such flags are
  handled gracefully, not rejected.)
- CLI values (for the `--key value` form) are strings and are passed through
  type inference (below). The explicit boolean forms `--flag` / `--no-flag`
  yield real booleans directly.

### Type inference (applied to STRING values from env and from `--key value`)
Applied to a raw string value `s` (case-insensitive for booleans):
- If `s` lowercased is `"true"`  -> Python `True`.
- If `s` lowercased is `"false"` -> Python `False`.
- Else if `s` is an OPTIONALLY-SIGNED run of ASCII digits (matches `-?\d+`, e.g.
  `"9000"`, `"-3"`, `"0"`) -> Python `int`.
- Else -> the string `s` unchanged (this includes `"1.5"`, `"007abc"`, `""`,
  `"3.0"`, and any value with a leading `+`; only plain integers are coerced,
  floats are left as strings).
- Values that already arrive as non-strings (e.g. JSON numbers/booleans/objects
  from a config file or nested dicts in `defaults`) are used AS-IS and are NOT
  re-inferred.

### CLI `show` subcommand
- Usage: `python -m confstack show [--defaults defaults.json] [--config app.json] [-- <cli flags...>]`.
- `--defaults PATH` loads a JSON object used as the `defaults` dict (optional;
  absent or missing file -> empty defaults).
- `--config PATH` is passed as `config_file`.
- Everything after a literal `--` separator is passed through as `cli_args`.
- The process environment is used for the `env` layer (only `APP_`-prefixed vars).
- The merged config is printed as JSON to stdout and the process exits 0.
</content>
</invoke>
````

**Graded behaviors:**

- `infer_bool_true` — string 'true' is inferred as boolean True
- `infer_bool_false` — string 'false' is inferred as boolean False
- `infer_int` — an integer-looking string is inferred as int
- `infer_str` — a non-bool non-int string stays a string (e.g. '1.5')
- `expand_dotted_cli` — a --nested.key flag expands to a nested dict
- `nested_dict_preserved` — nested dict values are returned as nested dicts
- `prec_cli_over_env` — CLI flags override environment variables
- `prec_env_over_config` — environment variables override the config file
- `prec_config_over_default` — config file overrides defaults
- `bool_false_overrides_true` — a higher-precedence false overrides a lower true
- `deep_merge_siblings` — deep merge keeps non-overlapping sibling keys
- `missing_config_no_error` — a missing config file path does not raise
- `none_config_ok` — config_file=None contributes nothing and does not raise
- `no_flag_false` — --no-flag sets the flag to boolean false
- `bare_flag_true` — a bare --flag sets it to boolean true
- `env_prefix_filter` — only APP_-prefixed env vars are consumed
- `unknown_flag_ok` — an unknown --flag is accepted, not rejected
- `deterministic` — load_config is deterministic across repeated calls
- `cli_show_json` — `python -m confstack show` emits JSON and exits 0
- `cli_precedence` — the CLI applies CLI > config > defaults precedence


### cronsim · 14 checks

````
Start from an empty repository and implement a Python 3.11+ project named `cronsim`.

Build a cron schedule parser and simulator. Use only the Python standard library.

Expose these functions from `cronsim.public`:

```python
def parse_cron(expr: str) -> dict: ...
def next_runs(expr: str, start_iso: str, count: int, timezone: str = "UTC") -> list[str]: ...
def should_run(expr: str, instant_iso: str, timezone: str = "UTC") -> bool: ...
```

Support five-field cron expressions:

```text
minute hour day_of_month month day_of_week
```

Support:

```text
*
comma lists
ranges
step values such as */15 and 1-10/2
```

Use standard cron OR semantics for day-of-month and day-of-week when both are restricted.

Timezone handling must use `zoneinfo`. Output timestamps must be ISO strings in UTC.

Include a CLI:

```bash
python -m cronsim next "*/15 9-17 * * 1-5" --start 2026-01-01T00:00:00Z --count 10 --timezone America/New_York
```

Include tests for ranges, steps, lists, invalid expressions, leap years, daylight saving transitions, weekday numbering, and deterministic output.
````

**Graded behaviors:**

- `ranges_steps_lists` — ranges, */n steps and list/range fields fire correctly
- `lists` — comma lists in minute and day-of-month fields
- `stepped_range` — stepped range like 1-10/2 expands correctly
- `invalid` — invalid expressions are rejected (raise)
- `leap_year` — Feb 29 schedule only matches leap years
- `dst_spring` — noon's UTC offset shifts correctly across spring-forward
- `dst_fall` — noon's UTC offset shifts correctly across fall-back
- `weekday_numbering` — Sunday is both dow 0 and dow 7; numbering is correct
- `dom_dow_or` — dom/dow use OR semantics when both restricted
- `should_run_consistency` — should_run is True at a fired instant, False one minute later
- `determinism` — next_runs is deterministic across repeated calls
- `count_and_order` — next_runs honors count and returns strictly ascending instants
- `cli_next` — `python -m cronsim next ...` emits the correct UTC instants
- `cli_invalid` — the CLI exits nonzero on an invalid expression


### csvql · 19 checks

````
Start from an empty repository and implement a Python 3.11+ project named `csvql`.

Build a small query engine for CSV files. Use only the Python standard library.

Expose:

```python
def query_csv(path: str, query: str) -> list[dict]: ...
```

Support queries like:

```sql
SELECT name, age FROM people WHERE age >= 18 ORDER BY age DESC LIMIT 10
SELECT department, COUNT(*) FROM employees GROUP BY department ORDER BY COUNT(*) DESC
```

Required features:

```text
SELECT columns
SELECT *
WHERE with = != < <= > >=
AND / OR
ORDER BY one column
LIMIT
COUNT(*)
SUM(column)
AVG(column)
GROUP BY one column
```

Infer numbers where possible; otherwise treat values as strings.

Do not use SQL databases or third-party parsers. Implement parsing yourself.

Include a CLI:

```bash
python -m csvql query people.csv "SELECT name FROM people WHERE age >= 18"
```

Include tests for filtering, numeric comparison, string comparison, grouping, aggregates, order by, limit, malformed queries, and CSV quoting.

## Contract

This section pins the loose parts of the spec so that an automated grader can check
behavior rather than guess at an internal representation. An implementation is
"correct" when it honors the spec above AND the conventions below.

### Import path and CLI

- The public function lives at `csvql.public`:

  ```python
  from csvql.public import query_csv
  ```

- The CLI is invoked as a module with a `query` subcommand:

  ```bash
  python -m csvql query <path.csv> "<SQL query>"
  ```

  The CLI prints the result rows to stdout. The output MUST be machine-readable
  as JSON: either a JSON array of row objects, or one JSON object per line
  (JSON Lines). A grader will accept either form.

### Result shape

- `query_csv(path, query)` returns a `list[dict]` — a list of row dicts.
- Each row dict maps a SELECTed output column name (a `str`) to that row's value.
- Row order reflects `ORDER BY` / `LIMIT` when present; otherwise it follows the
  file's row order.

### Value typing (numeric inference)

- Values that look like numbers are inferred to numeric types: an integer-looking
  field (e.g. `"18"`) becomes an `int`, a decimal-looking field (e.g. `"3.5"`)
  becomes a `float`. Everything else stays a `str`.
- Comparisons in `WHERE` and ordering in `ORDER BY` use these inferred types, so
  `age >= 18` compares numerically and a name column compares lexicographically.
- A grader tolerates int-vs-float representation of the same numeric value (e.g.
  `18` and `18.0` are treated as equal), and for aggregate results tolerates an
  integer-valued result expressed as either `int` or `float`.

### Column naming

- For a plain column projection (`SELECT name, age`), each output key is the
  source column name exactly as written (`"name"`, `"age"`).
- `SELECT *` projects every source column, keyed by its CSV header name.
- For aggregates, the output key is the aggregate's source text, uppercased and
  whitespace-free:
  - `COUNT(*)` appears under the key `"COUNT(*)"`.
  - `SUM(amount)` appears under the key `"SUM(amount)"`.
  - `AVG(score)` appears under the key `"AVG(score)"`.
  A grader matches the aggregate value tolerantly: it accepts the value under the
  canonical key above, or under any key whose normalized text (uppercased,
  spaces removed) equals the canonical key. This tolerates a `COUNT(*)` alias
  such as `count` only if it normalizes to the same canonical text; otherwise the
  canonical `COUNT(*)`/`SUM(col)`/`AVG(col)` key is expected.

### GROUP BY rows

- `SELECT department, COUNT(*) FROM employees GROUP BY department` returns one row
  dict per distinct group. Each such row carries:
  - the group-by column under its own name (`"department"`), and
  - each aggregate under its aggregate key (`"COUNT(*)"`, etc.).
- `COUNT(*)` counts the rows in the group. `SUM(col)` / `AVG(col)` aggregate the
  numeric values of `col` within the group (non-numeric values are ignored for the
  arithmetic, matching "infer numbers where possible").
- Without a `GROUP BY`, a query whose projection is purely aggregates (e.g.
  `SELECT COUNT(*) FROM people`) returns a single row dict covering all matching
  rows.

### Errors

- A malformed or unsupported query (syntax error, unknown column in a way the
  engine cannot resolve, unsupported construct) raises an exception rather than
  returning a wrong-but-silent result. The exception type is unspecified; a grader
  only checks that *some* exception is raised for clearly malformed input.
````

**Graded behaviors:**

- `select_columns` — SELECT named columns returns list of row dicts with those keys
- `select_star` — SELECT * projects every source column
- `where_numeric` — WHERE numeric comparison (age >= 18) filters numerically, not lexically
- `where_string` — WHERE string equality (city = 'NYC') filters string columns
- `where_neq` — WHERE != excludes matching rows
- `where_and` — WHERE ... AND ... requires both conditions
- `where_or` — WHERE ... OR ... admits either condition
- `order_by_desc` — ORDER BY col DESC sorts descending
- `order_by_asc` — ORDER BY col (default ASC) sorts ascending
- `limit` — LIMIT n truncates the result to n rows
- `count_star` — COUNT(*) over whole table returns the row count under a COUNT(*) key
- `sum_agg` — SUM(col) totals a numeric column
- `avg_agg` — AVG(col) averages a numeric column
- `group_by_count` — GROUP BY col with COUNT(*) yields one row per group with group key + count
- `group_by_order` — GROUP BY with ORDER BY COUNT(*) DESC orders groups by aggregate
- `numeric_inference` — numeric fields are inferred as numbers (age compares/sorts numerically)
- `csv_quoting` — quoted CSV fields with embedded commas/quotes parse correctly
- `malformed_raises` — a malformed query raises an exception rather than returning silently
- `cli_query_json` — `python -m csvql query` emits machine-readable JSON rows


### flagwise · 19 checks

````
Start from an empty repository and implement a Python 3.11+ project named `flagwise`.

Build a deterministic feature flag evaluator. Use only the Python standard library.

Expose:

```python
def evaluate_flag(config: dict, flag_key: str, context: dict) -> dict: ...
def evaluate_all(config: dict, context: dict) -> dict: ...
```

Flag config:

```json
{
  "flags": {
    "new_checkout": {
      "enabled": true,
      "default": false,
      "rules": [
        {
          "if": {"country": {"equals": "US"}},
          "serve": true
        },
        {
          "if": {"plan": {"in": ["pro", "enterprise"]}},
          "rollout": 25
        }
      ]
    }
  }
}
```

Support conditions:

```text
equals
not_equals
in
not_in
exists
greater_than
less_than
and
or
not
```

Rollouts must be deterministic by hashing `flag_key` and `context["user_id"]`. A rollout of `25` means approximately 25 percent of users receive `true`.

The result must include:

```json
{
  "key": "new_checkout",
  "value": true,
  "reason": "rule_match",
  "matched_rule_index": 0
}
```

Include a CLI:

```bash
python -m flagwise eval --config flags.json --flag new_checkout --context user.json
python -m flagwise eval-all --config flags.json --context user.json
```

Include tests for rule ordering, disabled flags, defaults, deterministic rollout, nested boolean logic, missing context fields, and stable hashing.

## Contract

This section pins the shapes the held-out oracle checks. It does not add new
behavior beyond the spec above; it only fixes the wire/return format so a
conformant implementation is gradable. Anything not pinned here is free.

Import path and module:
- The package is importable as `flagwise`, and the public API lives at
  `flagwise.public`:
    - `evaluate_flag(config: dict, flag_key: str, context: dict) -> dict`
    - `evaluate_all(config: dict, context: dict) -> dict`
- The CLI is invocable as `python -m flagwise` with subcommands `eval` and
  `eval-all` (flags exactly as shown in the spec). All CLI output is JSON on
  stdout (a single JSON document).

`evaluate_flag` return shape (the result dict), pinned:
- `key`   -> str, equal to the `flag_key` that was evaluated.
- `value` -> the served value. For a boolean flag this is `true`/`false`; in
  general it is whatever the matched rule's `serve` / the flag `default`
  resolves to. Rollouts serve `true` (hit) or fall through (miss).
- `reason` -> a short string explaining the decision. The set of reason
  STRINGS is not pinned beyond these distinctions, which ARE required to be
  distinguishable from one another (any stable spelling is accepted, the
  oracle derives the mapping):
    - a rule matched and served            (the example uses "rule_match")
    - a rule's rollout bucket was hit and served true
    - the flag is disabled (`enabled: false`) -> serves the flag default
    - no rule matched -> serves the flag default
    - the flag_key is not present in the config
- `matched_rule_index` -> int index (0-based) into the flag's `rules` list of
  the rule that decided the value, when a rule decided it; otherwise `None`
  (JSON `null`). For disabled flags, unmatched defaults, and unknown flags it
  is `null`.

`evaluate_all` return shape, pinned:
- Returns a dict keyed by every flag key present in `config["flags"]`. Each
  value is the same per-flag result dict that `evaluate_flag` returns for that
  key (same `key`/`value`/`reason`/`matched_rule_index` contract).

Determinism contract:
- `evaluate_flag(config, flag_key, context)` is a pure function of its inputs:
  the same `(config, flag_key, context)` MUST yield an equal result dict on
  every call (no clocks, no RNG, no global state).
- Rollout bucketing MUST be a deterministic function of `flag_key` and
  `context["user_id"]` only (a hash of those two values). For a fixed flag and
  user the rollout decision is stable across calls and across processes. Over
  many distinct `user_id`s a rollout of N serves `true` to approximately N% of
  users (uniform bucketing; the oracle checks a tolerant band, not an exact
  count).

Condition contract (the `if` block of a rule):
- A condition is a mapping. A leaf condition maps a context FIELD name to an
  operator mapping, e.g. `{"country": {"equals": "US"}}` or
  `{"age": {"greater_than": 18}}`. Operators: `equals`, `not_equals`, `in`,
  `not_in`, `exists` (truthy/falsey presence test), `greater_than`,
  `less_than`.
- Boolean combinators `and` / `or` take a LIST of sub-conditions; `not` takes a
  single sub-condition (or a one-element list). These may nest arbitrarily.
- A missing context field makes a leaf condition that needs it evaluate to
  False (never raises); `exists` returns whether the field is present.
````

**Graded behaviors:**

- `result_shape` — evaluate_flag returns a dict with key/value/reason/matched_rule_index
- `rule_ordering` — the FIRST matching rule decides; matched_rule_index points at it
- `disabled_flag` — a disabled flag serves its default and matches no rule
- `default_no_match` — when no rule matches, the flag default is served
- `serve_value` — a matched rule serves its configured value (not just True)
- `op_equals` — equals condition matches on equality and only then
- `op_in` — in / not_in conditions match set membership
- `op_exists` — exists condition tests context-field presence
- `op_numeric` — greater_than / less_than compare numerically
- `nested_boolean` — nested and / or / not combinators evaluate correctly
- `missing_field` — a missing context field makes a leaf False without raising
- `unknown_flag` — an unknown flag_key yields a structured (non-raising) result
- `determinism_same` — same (flag, context) yields an equal result across repeated calls
- `rollout_stable_user` — a rollout decision is stable per user across many calls
- `rollout_hash_inputs` — rollout depends on flag_key + user_id (a stored hash, not hash())
- `rollout_distribution` — rollout N serves ~N% of many synthetic users (tolerant band)
- `evaluate_all_keys` — evaluate_all returns one contract result per configured flag
- `cli_eval_json` — `python -m flagwise eval` emits a JSON result dict
- `cli_eval_all_json` — `python -m flagwise eval-all` emits JSON keyed by flag


### ledgercore · 11 checks

````
Start from an empty repository and implement a Python 3.11+ project named `ledgercore`.

Build an event-sourced ledger with SQLite persistence. Use only the Python standard library.

Expose these functions from `ledgercore.public`:

```python
def init_db(db_path: str) -> None: ...
def append_event(db_path: str, event: dict) -> dict: ...
def get_account_balance(db_path: str, account_id: str) -> dict: ...
def get_account_statement(db_path: str, account_id: str) -> list[dict]: ...
def replay_account(db_path: str, account_id: str) -> dict: ...
def export_trial_balance(db_path: str) -> dict: ...
```

Supported event types:

```text
account_opened
deposit_posted
withdrawal_posted
transfer_posted
fee_charged
adjustment_posted
```

Events must be idempotent by `event_id`. Withdrawals and transfers must reject insufficient funds unless the account has overdraft enabled. Transfers must be atomic: either both sides post or neither does.

Every event must have:

```json
{
  "event_id": "evt_001",
  "type": "deposit_posted",
  "account_id": "acct_cash",
  "occurred_at": "2026-01-01T12:00:00Z",
  "amount_cents": 10000
}
```

Statements must be ordered by `occurred_at`, then insertion order. Replay must reconstruct the same balance as the stored projection. The system must reject duplicate event IDs with conflicting payloads but allow exact duplicate replays.

Include a CLI:

```bash
python -m ledgercore init-db --db ledger.db
python -m ledgercore append --db ledger.db --event event.json
python -m ledgercore balance --db ledger.db --account acct_cash
python -m ledgercore statement --db ledger.db --account acct_cash
python -m ledgercore trial-balance --db ledger.db
```

Include tests for idempotency, overdraft rejection, atomic transfers, replay consistency, event ordering, and trial balance correctness.

## Contract

- Expose the functions from `ledgercore.public` (a `python -m ledgercore` CLI is also required). Money is integer cents.
- `transfer_posted` events name the source account under `account_id` and the destination under `to_account_id`, moving `amount_cents` from source to destination.
- Overdraft is enabled per account via an `overdraft` boolean on its `account_opened` event (default false). `adjustment_posted` carries a signed `amount_cents`.
- `get_account_balance(db, acct)` returns a dict with the balance under `balance_cents`.
- `replay_account` returns a dict whose balance (under `balance_cents`) equals the stored projection.
- `export_trial_balance(db)` returns a dict with per-account balances and a `"total_cents"` grand total (the sum of all account balances).
- Rejections (insufficient funds without overdraft; a duplicate `event_id` with a conflicting payload) are signaled by raising an exception; an exact-duplicate `event_id` replay is a no-op, not an error.
````

**Graded behaviors:**

- `basic_balance` — open+deposit+withdraw yields the correct projected balance
- `idempotent_replay` — exact-duplicate event_id replays do not double-apply
- `idempotent_conflict` — a conflicting payload under a used event_id is rejected
- `overdraft_reject` — an overdrawing withdrawal is rejected when overdraft is off
- `overdraft_allow` — an overdrawing withdrawal succeeds when overdraft is enabled
- `transfer_atomic_ok` — a valid transfer debits source and credits dest
- `transfer_atomic_fail` — an underfunded transfer posts neither leg (atomicity)
- `replay_consistency` — replay_account reproduces the stored projected balance
- `event_ordering` — statement is ordered by occurred_at then insertion order
- `trial_balance` — trial balance reconciles per-account sums and ledger total
- `cli_contract` — the `python -m ledgercore` CLI runs the commands and emits JSON


### microapi · 14 checks

````
Start from an empty repository and implement a Python 3.11+ project named `microapi`.

Build a small HTTP router on top of `http.server`. Use only the Python standard library.

Expose:

```python
class App:
    def route(self, method: str, path: str): ...
    def handle_request(self, method: str, path: str, headers: dict, body: bytes) -> tuple[int, dict, bytes]: ...
```

Support route patterns:

```text
/users
/users/{user_id}
/files/{path:*}
```

Support:

```text
path parameters
query strings
JSON request body parsing
JSON responses
404 for no route
405 for method not allowed
structured error responses
middleware functions
```

Include a runnable server:

```bash
python -m microapi serve --host 127.0.0.1 --port 8080
```

Provide example routes for a small in-memory todo API.

Include tests for routing precedence, path params, wildcard paths, query parsing, JSON parsing, middleware ordering, 404, 405, and error responses.

## Contract

This section pins the behavior the held-out oracle grades. Anywhere the SPEC above
left a shape unstated, the convention below is binding. The oracle never reads your
tests and never exercises the `serve` CLI; it drives everything in-process through
`handle_request`. Conform to this Contract and the SPEC and you pass.

### Import path

- The package is importable as `microapi`.
- `microapi.public` exposes a class named `App`.
- Constructing the app takes no required arguments: `app = App()`.

### Registering routes

- `app.route(method, path)` is a decorator factory: it returns a decorator that
  registers the wrapped function as the handler for `(method, path)` and returns the
  function unchanged. Usage:

  ```python
  app = App()

  @app.route("GET", "/users/{user_id}")
  def get_user(request, user_id):
      ...
  ```

- `method` is an uppercase HTTP method string (e.g. "GET", "POST").
- `path` is a pattern. Three segment kinds:
  - static segment, e.g. `users` — matches that literal segment only.
  - named parameter, e.g. `{user_id}` — matches exactly one path segment, captured
    under the name `user_id`.
  - trailing wildcard, e.g. `{path:*}` — matches one OR MORE remaining segments
    (the rest of the path, slashes included), captured under the name `path`. A
    wildcard is only valid as the final segment of a pattern.

### Handler signature and how path params are delivered

- A handler is called as `handler(request, **path_params)`.
  - `request` is the first positional argument: an object (or mapping) carrying at
    least the request method, path, headers, parsed query parameters, and parsed
    JSON body (see "request" below).
  - Each captured path parameter is passed as a KEYWORD argument whose name is the
    parameter name in the pattern. For `/users/{user_id}` the handler receives
    `user_id="..."`. For `/files/{path:*}` the handler receives `path="a/b/c"`.
  - Path parameter values are always `str`. They are URL-decoded (percent-decoded).
  - Wildcard values retain internal slashes and are NOT split.

### The `request` object

The first handler argument exposes, by attribute access:

- `request.method` -> str, the uppercase method.
- `request.path` -> str, the request path (no query string).
- `request.headers` -> dict, the headers passed to `handle_request` (keys as given).
- `request.query` -> dict[str, str]: parsed query string. For a repeated key, the
  LAST value wins. Values are URL-decoded. Empty when there is no query string.
- `request.json` -> the parsed JSON body (dict/list/scalar) when the request body is
  non-empty and is valid JSON; otherwise `None`. Parsing JSON never raises out of
  `handle_request` — a malformed body yields a structured 400 (see below).

### What a handler returns, and JSON response encoding

A handler returns the response body as a JSON-serializable Python value (commonly a
dict or list). The framework serializes it. Specifically:

- A handler may return a value `v` -> `(200, v)` semantics: status 200, body is
  `json.dumps(v)` UTF-8 encoded.
- A handler may return a 2-tuple `(status, v)` -> that status, body `json.dumps(v)`.
- The response `Content-Type` header is `application/json`.

### `handle_request` return shape

`app.handle_request(method, path, headers, body)` returns a 3-tuple:

```python
(status: int, headers: dict, body: bytes)
```

- `status` is the integer HTTP status code.
- `headers` is a dict of response headers; it includes
  `"Content-Type": "application/json"` for every response this oracle exercises.
- `body` is `bytes` (UTF-8). For every response this oracle exercises, `body`
  decodes to a JSON document.
- `headers` is the dict passed by the caller (may be empty `{}`). `body` is `bytes`
  (may be empty `b""`).

### Routing precedence

When more than one registered pattern matches a path, precedence is:

1. an all-static match (most specific) wins over
2. a match that used one or more named parameters, which wins over
3. a match that used a trailing wildcard (least specific).

So with `/users/me` (static), `/users/{user_id}` (param), and `/files/{path:*}`
registered, the path `/users/me` routes to the static handler, and `/users/42`
routes to the param handler.

### 404, 405, and error-body shape

- 404: no registered pattern matches the path (for any method). Status 404.
- 405: the path matches at least one registered pattern, but NOT for the requested
  method. Status 405.
- Every error response (404, 405, malformed-JSON 400, and any uncaught handler
  exception -> 500) has body that is a JSON OBJECT containing a top-level key
  `"error"` whose value is a non-empty string (or an object with a non-empty
  message). `Content-Type` is `application/json`.

### Middleware

- `app.use(middleware)` registers a middleware. A middleware is a callable
  `middleware(request, call_next) -> response`, where `call_next(request)` invokes
  the next middleware (or finally the matched handler) and returns its response.
- Middlewares run in REGISTRATION ORDER on the way in (the first registered wraps
  the outermost / runs first), and unwind in reverse on the way out.
- A middleware may short-circuit by returning without calling `call_next`.

### serve CLI (NOT exercised by the oracle)

- `python -m microapi serve --host 127.0.0.1 --port 8080` starts the HTTP server.
- The oracle does NOT start the server or open a socket; it tests `handle_request`
  in-process only. The CLI's existence and exact behavior are out of grading scope.
````

**Graded behaviors:**

- `static_route` — a static route returns 200 and its JSON body
- `path_param` — a named path param is delivered to the handler as the decoded value
- `wildcard` — a trailing wildcard captures the rest of the path, slashes intact
- `precedence_static_over_param` — an all-static match beats a param match for the same path
- `precedence_param_over_wildcard` — a param match beats a wildcard match for the same path
- `query_parse` — query-string params are parsed and exposed to the handler
- `json_body` — a JSON request body is parsed and exposed to the handler
- `json_response` — responses are JSON-encoded bytes with a JSON content-type
- `status_passthrough` — a handler-chosen status code is returned verbatim
- `not_found_404` — an unmatched path returns status 404
- `method_not_allowed_405` — a matched path with a wrong method returns 405, not 404
- `error_body_shape` — 404/405 bodies are JSON objects carrying an 'error' signal
- `middleware_order` — middlewares run in registration order, unwinding in reverse
- `middleware_short_circuit` — a middleware can short-circuit before the handler runs


### migrato · 14 checks

````
Start from an empty repository and implement a Python 3.11+ project named `migrato`.

Build a SQLite migration runner. Use only the Python standard library.

Expose:

```python
def init_migration_table(db_path: str) -> None: ...
def discover_migrations(migrations_dir: str) -> list[dict]: ...
def apply_migrations(db_path: str, migrations_dir: str) -> dict: ...
def migration_status(db_path: str, migrations_dir: str) -> list[dict]: ...
```

Migration files are named:

```text
001_create_users.sql
002_add_email.sql
003_backfill_status.sql
```

Each file may contain:

```sql
-- migrate:up
CREATE TABLE users (...);

-- migrate:down
DROP TABLE users;
```

Apply migrations in numeric order. Record checksum, applied timestamp, and filename. If an applied migration file changes checksum later, report an error.

Include a CLI:

```bash
python -m migrato status --db app.db --migrations migrations/
python -m migrato up --db app.db --migrations migrations/
```

Include tests for ordered application, idempotency, checksum mismatch, malformed files, failed migration rollback, status reporting, and empty migration directories.

## Contract

This section pins the details the SPEC above leaves open. The held-out grader checks
BEHAVIOR against this contract; anything not pinned here is left to the implementer and
must not be graded.

### Import path / package layout
- The package is importable as `migrato`. The four functions are importable from
  `migrato.public` (i.e. `from migrato.public import apply_migrations`).
- The CLI is invokable as `python -m migrato ...` (the package has a `__main__`).
- The package uses only the Python standard library (`sqlite3`, `hashlib`, `os`, etc.).

### Migration files
- A migration file lives in `migrations_dir` and is named `<NNN>_<slug>.sql` where the
  leading run of digits is the numeric order key (e.g. `001`, `002`, `010`). Files are
  applied in ascending NUMERIC order of that integer (so `2_x.sql` < `10_x.sql`), NOT
  lexicographically. Files whose name does not begin with at least one digit are ignored
  for ordering/application (treated as not a migration).
- A file may contain `-- migrate:up` and `-- migrate:down` section markers. The SQL after
  `-- migrate:up` (until `-- migrate:down` or EOF) is the "up" script that is executed when
  the migration is applied. If NO `-- migrate:up` marker is present, the entire file body
  is treated as the up script. The `-- migrate:down` section is recorded but not executed
  by `apply_migrations`.

### Checksum algorithm (PINNED)
- The checksum of a migration is the lowercase hex `sha256` of the RAW file BYTES
  (the whole file on disk, read in binary, not just the up-section, no normalization).
  i.e. `hashlib.sha256(open(path,"rb").read()).hexdigest()`.

### `init_migration_table(db_path)` -> None
- Creates the bookkeeping table if it does not already exist. Idempotent: calling it on a
  db that already has the table is a no-op and must not raise. `apply_migrations` and
  `migration_status` must also work without a separate prior call to it (they ensure the
  table themselves).

### `discover_migrations(migrations_dir)` -> list[dict]
- Returns one dict per migration file found in `migrations_dir`, in ascending numeric
  order. Each dict MUST contain at least these keys (extra keys allowed):
  - `"filename"`: the base filename, e.g. `"001_create_users.sql"` (basename, not a path).
  - `"checksum"`: the sha256 hex string defined above.
- An empty or missing-of-migrations directory yields `[]`.

### `apply_migrations(db_path, migrations_dir)` -> dict  (PINNED return shape)
- Applies every not-yet-applied migration, in ascending numeric order, inside the db at
  `db_path`. Each newly applied migration's `up` SQL is executed and a bookkeeping row is
  recorded with its filename, checksum, and an applied timestamp.
- Idempotent: a migration already recorded as applied (matching filename + checksum) is
  skipped, not re-run.
- Returns a dict with AT LEAST these keys (extra keys allowed):
  - `"applied"`: a `list` of the filenames (basenames, strings) applied DURING THIS call,
    in the order applied. On a fully up-to-date db this is `[]`.
  - `"error"`: `None` when the call succeeded with no checksum mismatch; otherwise a
    truthy value (a non-empty string message OR a dict carrying the offending filename).
- CHECKSUM-MISMATCH SEMANTICS (PINNED — error result, NOT an exception): if a migration
  file that is ALREADY recorded as applied now has a different checksum on disk,
  `apply_migrations` MUST NOT raise. It must return with `"error"` set to a truthy value
  and MUST NOT apply any later (higher-numbered) migration in that call (the mismatched,
  already-applied migration is not re-applied either). `"applied"` reflects only what was
  actually applied before the stop.

### `migration_status(db_path, migrations_dir)` -> list[dict]  (PINNED return shape)
- Returns one dict per discovered migration, in ascending numeric order. Each dict MUST
  contain AT LEAST these keys (extra keys allowed):
  - `"filename"`: the base filename string (e.g. `"001_create_users.sql"`).
  - `"applied"`: a `bool` — `True` iff that migration is recorded as applied in the db,
    else `False`.
- Calling `migration_status` against a db where the table does not yet exist returns every
  migration with `"applied": False` and must not raise.

### CLI (`python -m migrato`)
- `python -m migrato status --db <path> --migrations <dir>` reports status and exits 0.
- `python -m migrato up --db <path> --migrations <dir>` applies pending migrations and
  exits 0 on success. Output format is left to the implementer (not graded).
</content>
</invoke>
````

**Graded behaviors:**

- `discover_order` — discover_migrations returns migrations in ascending NUMERIC order
- `discover_checksum` — discover_migrations exposes a stable sha256-hex checksum per file
- `discover_empty` — an empty migrations directory discovers no migrations
- `apply_ordered` — apply_migrations applies all pending migrations in numeric order
- `apply_effect` — applied up-SQL actually runs against the db (tables created)
- `apply_idempotent` — re-running apply_migrations applies nothing the second time
- `status_shape` — migration_status returns per-file {filename, applied: bool}
- `status_reflects_apply` — migration_status flips applied False->True after apply
- `status_no_table` — migration_status on a fresh db reports all-unapplied, no crash
- `checksum_mismatch` — a changed checksum on an applied migration is signalled (no crash)
- `mismatch_no_partial` — on checksum mismatch, later migrations are NOT applied
- `up_marker` — the `-- migrate:up` section is the SQL that gets executed
- `cli_status_rc0` — `python -m migrato status` runs and exits 0
- `cli_up_rc0` — `python -m migrato up` runs, exits 0, and applies migrations


### orderplane · 12 checks

````
Start from an empty repository and implement a Python 3.11+ project named `orderplane`.

Build a self-contained order processing simulator with SQLite persistence, a public API layer, a warehouse summary layer, an admin summary layer, and analytics export. Use only the Python standard library.

Expose from `orderplane.public`:

```python
def init_db(db_path: str) -> None: ...
def load_catalog(db_path: str, catalog_path: str) -> None: ...
def place_order(db_path: str, order: dict) -> dict: ...
def cancel_order(db_path: str, order_id: str, reason: str) -> dict: ...
def fulfill_order(db_path: str, order_id: str, shipped_at: str) -> dict: ...
def customer_order_view(db_path: str, order_id: str) -> dict: ...
def admin_order_view(db_path: str, order_id: str) -> dict: ...
def warehouse_picklist(db_path: str, date: str) -> list[dict]: ...
def export_revenue(db_path: str, month: str) -> list[dict]: ...
```

`load_catalog` reads a catalog JSON file; `place_order` receives an order dict. These shapes (prices are integer cents):

```json
// catalog.json
{
  "tax_rate": 0.10,
  "shipping": {
    "taxed":   {"fee": 1000, "taxable": true},
    "untaxed": {"fee": 1000, "taxable": false},
    "free":    {"fee": 0,    "taxable": false}
  },
  "discount_codes": {
    "PCT10":  {"type": "percent", "value": 10},
    "AMT500": {"type": "amount",  "value": 500}
  },
  "products": [
    {"sku": "AAA", "name": "Alpha", "price": 1000, "inventory": 10}
  ]
}
```

```json
// order dict (discount_code and shipping_method optional; shipping_method keys into catalog.shipping)
{"client_order_id": "c1", "items": [{"sku": "AAA", "qty": 2}], "discount_code": "PCT10", "shipping_method": "taxed"}
```

The logic (how discounts, tax, shipping, inventory, idempotency, and cancellation behave) is up to you to derive from the rules below.

The system must support:

```text
products
inventory
orders
discount codes
sales tax
shipping fees
order cancellation
partial fulfillment
idempotent order submission by client_order_id
```

Rules:

```text
Inventory reservation happens when an order is placed.
Discounts apply before tax.
Shipping fees are taxable only when the catalog says they are.
Canceled unfulfilled items release inventory.
Fulfilled items cannot be canceled retroactively.
Revenue export must use finalized fulfilled order data, not recalculated cart data.
Customer view, admin view, warehouse picklist, and revenue export must agree on canonical order state.
```

Implement a CLI:

```bash
python -m orderplane init-db --db orders.db
python -m orderplane load-catalog --db orders.db --catalog catalog.json
python -m orderplane place-order --db orders.db --order order.json
python -m orderplane cancel --db orders.db --order order_1 --reason customer_request
python -m orderplane fulfill --db orders.db --order order_1 --at 2026-01-02T12:00:00Z
python -m orderplane customer-view --db orders.db --order order_1
python -m orderplane admin-view --db orders.db --order order_1
python -m orderplane picklist --db orders.db --date 2026-01-02
python -m orderplane export-revenue --db orders.db --month 2026-01
```

All CLI output must be JSON.

Include tests runnable with:

```bash
python -m unittest discover
```

Tests must cover inventory reservation, idempotent order placement, discounts, tax, taxable and non-taxable shipping, cancellation, partial fulfillment, revenue export, and agreement across customer/admin/warehouse/analytics surfaces.
````

**Graded behaviors:**

- `reservation` — placing an order reserves inventory (over-reservation rejected)
- `idempotent` — resubmitting the same client_order_id yields the same order
- `discount_before_tax` — discount lowers the taxable base (tax computed after discount)
- `amount_discount` — a fixed-amount discount code reduces the order total
- `taxable_shipping` — shipping fee is taxed only when the catalog flags it taxable
- `cancel_releases` — canceling an unfulfilled order releases reserved inventory
- `no_cancel_after_fulfill` — a fulfilled order cannot be canceled retroactively
- `revenue_fulfilled_only` — revenue export includes only fulfilled orders
- `revenue_matches_finalized` — exported revenue equals the order's finalized total
- `cross_surface` — customer/admin/warehouse/revenue surfaces agree on canonical state
- `cli_json_contract` — CLI subcommands emit JSON and drive canonical state
- `cli_picklist_export_json` — CLI picklist and export-revenue emit JSON lists


### patchwise · 15 checks

````
Start from an empty repository and implement a Python 3.11+ project named `patchwise`.

Build a line-based diff and patch tool. Use only the Python standard library.

Expose:

```python
def unified_diff(old: str, new: str, fromfile: str = "old", tofile: str = "new") -> str: ...
def apply_patch(old: str, patch: str) -> str: ...
```

Generate unified diffs with context lines. Apply unified diffs to old content. Detect failed patches and report structured errors.

Support files with and without trailing newlines.

Include a CLI:

```bash
python -m patchwise diff old.txt new.txt
python -m patchwise apply old.txt patch.diff --out new.txt
```

Do not shell out to system `diff` or `patch`.

Include tests for additions, deletions, replacements, multiple hunks, context mismatch, trailing newline behavior, empty files, and round-trip diff/apply.

## Contract

This section pins the behavioral contract the held-out oracle grades. It binds
import paths and observable behavior only; it does NOT pin internal file layout,
helper names, or the exact diff TEXT a correct implementation emits.

Import path / entry points:
- Public API lives at the import path `patchwise.public`.
- The CLI is invoked as `python -m patchwise` (i.e. a `patchwise.__main__`).

`unified_diff(old, new, fromfile="old", tofile="new") -> str`:
- Returns a unified-diff string (the standard `--- / +++ / @@ ... @@` format with
  context lines). The string MUST be parseable by `apply_patch`.
- When `old == new`, the returned diff applied to `old` MUST yield `old`
  unchanged (an empty diff is acceptable).

`apply_patch(old, patch) -> str`:
- On SUCCESS returns the patched string (the new content).
- The ROUND-TRIP invariant is the contract: for any `old`, `new`,
  `apply_patch(old, unified_diff(old, new)) == new` exactly (byte-for-byte,
  including trailing-newline behavior).
- FAILED PATCH (context mismatch — the patch's context/removed lines do not match
  `old`): the implementation MUST signal the failure via a STRUCTURED error.
  PINNED MECHANISM: `apply_patch` RAISES a CUSTOM (non-builtin) exception on a failed
  patch, exported from `patchwise.public` (accessible as `patchwise.public.<Name>` —
  any name, e.g. `PatchError` or `DiffError`). A failed patch MUST NOT silently return
  wrong/garbage content and MUST NOT raise a bare `KeyError`/`IndexError`/`TypeError`
  leaking internals.

Trailing newline:
- Files with and without a trailing newline are both supported, and the
  round-trip invariant holds for both (i.e. a file lacking a final newline
  round-trips back to lacking one; a file with one round-trips back with one).

Empty files:
- `old` and/or `new` may be the empty string `""`; round-trip still holds.

CLI (`python -m patchwise`):
- `python -m patchwise diff <old> <new>` reads the two files and writes a unified
  diff to STDOUT.
- `python -m patchwise apply <old> <patch> --out <new>` reads `<old>` and the
  patch file `<patch>`, applies the patch, and writes the result to the file
  named by `--out`.
- The diff produced by `diff` MUST be applyable by `apply` to reproduce the new
  file (CLI round-trip).

Fairness note (most important): a model's exact diff TEXT may legitimately differ
from the reference's (different context-line counts, hunk headers, ordering of
equal hunks, etc.). The oracle therefore verifies ROUND-TRIP BEHAVIOR
(`apply(old, unified_diff(old, new)) == new`) and that a hand-written, standard
unified diff applies correctly — it NEVER demands string-equality against any
particular diff text.
````

**Graded behaviors:**

- `roundtrip_addition` — round-trip holds when lines are added
- `roundtrip_deletion` — round-trip holds when lines are deleted
- `roundtrip_replace` — round-trip holds when lines are replaced
- `roundtrip_multi_hunk` — round-trip holds across multiple separated hunks
- `roundtrip_no_trailing_nl` — round-trip preserves a missing final newline
- `roundtrip_add_trailing_nl` — round-trip preserves added/kept trailing newline
- `roundtrip_empty_old` — round-trip holds when old is the empty string
- `roundtrip_empty_new` — round-trip holds when new is the empty string
- `roundtrip_identical` — diff of identical content round-trips unchanged
- `apply_handwritten_diff` — a hand-written standard unified diff applies correctly
- `diff_is_string` — unified_diff returns a string in unified-diff shape
- `context_mismatch_raises_patch_exc` — failed patch raises a 'Patch'-named exception from patchwise.public
- `context_mismatch_not_silent` — failed patch never silently returns wrong content
- `cli_diff_emits_diff` — `python -m patchwise diff` writes a unified diff to stdout
- `cli_roundtrip` — CLI diff then apply --out reproduces the new file


### slotfinder · 14 checks

````
Start from an empty repository and implement a Python 3.11+ project named `slotfinder`.

Build a meeting availability engine. Use only the Python standard library.

Expose:

```python
def find_slots(request: dict) -> list[dict]: ...
```

Request format:

```json
{
  "timezone": "America/New_York",
  "duration_minutes": 30,
  "search_start": "2026-01-05T00:00:00Z",
  "search_end": "2026-01-10T00:00:00Z",
  "participants": [
    {
      "id": "u1",
      "working_hours": [{"days": [1,2,3,4,5], "start": "09:00", "end": "17:00"}],
      "busy": [{"start": "2026-01-05T15:00:00Z", "end": "2026-01-05T16:00:00Z"}]
    }
  ]
}
```

Find slots where all participants are within working hours and not busy. Output slots in UTC ISO format.

Support timezones with `zoneinfo`, daylight saving transitions, multiple participants, multiple working-hour blocks, and configurable slot granularity with default 15 minutes.

Include a CLI:

```bash
python -m slotfinder find request.json
```

Include tests for overlapping busy periods, working-hour boundaries, multiple timezones, DST transitions, no availability, deterministic ordering, and granularity.

## Contract

This section pins the behavior the grader checks. Anything the prose above leaves
open is fixed here; do not contradict it.

### Import path & CLI
- Importable as `slotfinder.public` exposing `find_slots(request: dict) -> list[dict]`.
- CLI entry point `python -m slotfinder find <request.json>` that prints the result of
  `find_slots` to stdout as a JSON array. CLI output MUST be valid JSON (a JSON list of
  slot objects). Reading the request from the given file path is the only required form.

### Slot shape & ordering
- Each returned slot is a dict with exactly the keys `start` and `end`:
  `{"start": <ISO-UTC>, "end": <ISO-UTC>}`.
- `start` and `end` are ISO-8601 timestamps in UTC. They denote the same instant
  regardless of formatting; the grader compares instants, so either a `Z` suffix or a
  `+00:00` offset is accepted, with or without a `T` separator as produced by
  `datetime.isoformat()`. The slot length `end - start` equals `duration_minutes`.
- Slots are returned sorted ascending by start instant (deterministic ordering).
- Slots are NON-OVERLAPPING candidate windows: the returned set never contains two slots
  whose `[start, end)` intervals overlap.

### Time model (all comparisons are at instant level)
- `duration_minutes` (int, required): the length of each slot in minutes.
- `granularity_minutes` (int, optional, DEFAULT 15): candidate slot starts are aligned to
  a grid of this many minutes, measured from `search_start`. A candidate window is
  `[t, t + duration_minutes)` for grid points `t = search_start + k * granularity_minutes`.
- `search_start` / `search_end` are ISO-8601 UTC instants. Only windows fully inside
  `[search_start, search_end)` are considered (a window may not extend past `search_end`).
- A grid point `t` becomes a returned slot iff EVERY participant is, for the WHOLE window
  `[t, t + duration_minutes)`, both (1) within working hours and (2) not busy. Otherwise
  the grid point is rejected.
- Non-overlapping packing is greedy from the earliest valid grid point: scan grid points
  in ascending order; when a window is accepted, the next candidate considered starts at
  or after that window's end (windows already covered by an accepted slot are skipped).

### Working hours
- `working_hours` is a list of blocks; a participant with no blocks is never available.
  Multiple blocks union (a moment is "within working hours" if it falls in ANY block).
- `days` are ISO weekday integers in the participant's LOCAL timezone: Monday=1, Tuesday=2,
  Wednesday=3, Thursday=4, Friday=5, Saturday=6, Sunday=7.
- `start` / `end` are `"HH:MM"` wall-clock times in the participant's LOCAL timezone. A
  block covers the LOCAL-time half-open interval `[start, end)` on each listed day.
  `end` is exclusive; `end == start` means an empty block. `end` is on the same local day
  (no overnight wrap). A window is "within working hours" only if the participant's entire
  `[t, t+duration)` interval lies inside a single working-hours block for that local day.
- Day membership and the `[start, end)` wall-clock bounds are evaluated in LOCAL time, so
  they shift correctly across daylight-saving transitions (e.g. 09:00 local is a different
  UTC instant before and after a DST change). The reference uses `zoneinfo` for this.

### Timezone
- The top-level `timezone` is an IANA name used as the default local zone for participants.
- A participant MAY override with a `timezone` key; if absent, the top-level `timezone`
  applies. Each participant's working hours are interpreted in that participant's zone.

### Busy intervals
- `busy` is a list of `{"start": <ISO-UTC>, "end": <ISO-UTC>}` instants (UTC). Each is a
  half-open interval `[start, end)`. A window is blocked if it intersects ANY busy interval
  of ANY participant. Touching at an endpoint (window start == busy end, or window end ==
  busy start) does NOT count as a conflict.
````

**Graded behaviors:**

- `basic_availability` — single-participant slots match the derived expected set
- `busy_excludes` — a busy interval removes exactly the windows it overlaps
- `busy_endpoint_touch` — a window touching a busy interval at an endpoint is NOT blocked
- `working_hour_boundary` — no slot falls outside the local [start, end) working hours
- `multi_participant` — only the intersection of all participants' availability is returned
- `multi_block` — multiple working-hour blocks union correctly
- `dst_transition` — working hours track local wall-clock across a DST transition
- `no_availability` — a fully-busy / out-of-hours request returns an empty list
- `granularity_default` — default granularity is 15 minutes
- `granularity_custom` — a custom granularity changes the candidate grid
- `non_overlapping` — returned slots never overlap each other
- `deterministic_order` — slots are sorted ascending by start and stable across runs
- `slot_shape` — each slot is {start, end} ISO-UTC with length == duration_minutes
- `cli_json` — `python -m slotfinder find <file>` emits a JSON list of slots


### statichisel · 18 checks

````
Start from an empty repository and implement a Python 3.11+ project named `statichisel`.

Build a static site generator. Use only the Python standard library.

Input directory structure:

```text
site/
  pages/
    index.md
    about.md
  templates/
    base.html
  assets/
    style.css
  site.json
```

Markdown files begin with front matter:

```markdown
---
title: Home
slug: /
template: base.html
---

# Welcome

This is **bold** text.
```

Support Markdown features:

```text
headings
paragraphs
bold
italic
inline code
fenced code blocks
unordered lists
links
```

Template variables:

```text
{{ title }}
{{ content }}
{{ nav }}
```

Implement:

```python
def build_site(source_dir: str, output_dir: str) -> dict: ...
```

The build must copy assets, render pages, generate a navigation list, and return a JSON-serializable manifest of generated files.

Include a CLI:

```bash
python -m statichisel build site/ dist/
```

Include tests for front matter parsing, Markdown rendering, template rendering, asset copying, slug handling, malformed front matter, and deterministic output.

## Contract

This section pins the parts of the spec that the held-out grader checks. The grader
verifies BEHAVIOR and rendered CONTENT/STRUCTURE — never exact HTML byte-equality,
because the exact markup is implementation-specific. Anything not pinned here is free.

### Import path / package layout

- The package is importable as `statichisel`.
- `build_site` is importable as `statichisel.public.build_site` — i.e. the module
  `statichisel/public.py` exposes the top-level function `build_site`.
- The CLI is invocable as `python -m statichisel build <source_dir> <output_dir>`
  (i.e. `statichisel/__main__.py` exists and handles the `build` subcommand). The
  CLI's stdout format is NOT pinned (it may print a manifest, a summary, or nothing);
  the grader only requires exit code 0 and that the output files appear on disk.

### `build_site(source_dir, output_dir) -> dict`

- Signature: `build_site(source_dir: str, output_dir: str) -> dict`. Both arguments
  are filesystem path strings. `output_dir` need not pre-exist; `build_site` creates it.
- Returns a JSON-serializable manifest `dict`. The manifest MUST let a caller
  enumerate the generated files. The grader accepts ANY of these manifest shapes
  (it derives the file list tolerantly, it does not require one specific key):
    - a top-level key whose name contains "file"/"page"/"output"/"generated"/"manifest"
      mapping to a list of file entries, OR
    - a top-level key "outputs"/"results"/"build" holding such a list, OR
    - the manifest itself being a list of file entries.
  Each file entry is EITHER a path string, OR a dict carrying the path under any of
  the keys: `path`, `output`, `dest`, `file`, `url`, `target`, `name`. Paths may be
  absolute, or relative to `output_dir`, or relative to cwd — the grader normalizes
  by basename and by joining against `output_dir`, so any consistent convention passes.
- The manifest must be JSON-serializable (`json.dumps(manifest)` must not raise).
- Calling `build_site` twice with identical inputs must be deterministic: the manifest
  must serialize identically across runs, and the set of generated output files (by
  path relative to `output_dir`) must be identical across runs.

### Pages, front matter, slugs

- Each `.md` file under `pages/` begins with YAML-ish front matter delimited by lines
  of exactly `---` (a `---` line, key: value lines, a closing `---` line), followed by
  the Markdown body. Front matter keys used by the grader: `title`, `slug`, `template`.
- Each page produces exactly one generated HTML file in the output. The page reachable
  at slug `/` (the root/index page) must produce a file whose basename is `index.html`.
  A page with another slug (e.g. `about` or `/about`) must produce a corresponding
  `.html` file (the grader matches by the slug stem, e.g. `about` -> a file whose
  basename or path stem is `about`, accepting `about.html` or `about/index.html`).
- Malformed front matter (e.g. a missing closing `---`) must NOT crash the build: the
  build must complete and still produce output for the well-formed pages. (The grader
  does not pin what happens to the malformed page itself, only that the build survives
  and the well-formed pages still render.)

### Rendered content / structure (NOT exact HTML)

For a page whose Markdown body contains the listed feature, the grader checks the
rendered HTML of that page for the following STRUCTURE. Each is satisfied by common
HTML equivalents; exact tag choice and attributes are free:
- bold `**x**` -> the text `x` wrapped in `<strong>...</strong>` OR `<b>...</b>`.
- italic `*x*` / `_x_` -> the text `x` wrapped in `<em>...</em>` OR `<i>...</i>`.
- heading `# H` -> the text `H` wrapped in some `<h1>`..`<h6>` tag.
- inline code `` `x` `` -> the text `x` wrapped in `<code>...</code>`.
- fenced code block -> the block's contents wrapped in `<pre>` and/or `<code>`.
- unordered list (`- item` lines) -> `<ul>` with `<li>` item(s).
- link `[text](url)` -> an `<a>` tag whose `href` is `url` and whose anchor text is `text`.
- The page `title` from front matter appears substituted into the rendered output
  (the `{{ title }}` template variable is replaced with the front matter title; the
  literal string `{{ title }}` must NOT remain in the output).
- The template's `{{ content }}` is replaced by the page's rendered body, and the
  literal `{{ content }}` must NOT remain in the output.
- A navigation list (`{{ nav }}`) is generated and substituted: the rendered output of
  a page that uses `{{ nav }}` contains links/entries referencing the OTHER pages of
  the site (the grader checks that nav references more than one page, e.g. multiple
  `<a>` tags or page titles/slugs), and the literal `{{ nav }}` must NOT remain.

### Assets

- Every file under the source `assets/` directory is copied into the output. The
  grader checks that an asset placed at `assets/<name>` is present somewhere under
  `output_dir` after the build (matched by basename and by byte-equality of contents).
  The exact destination subpath (e.g. `assets/style.css` vs `style.css`) is NOT pinned.

# ASSUMES (conventions the grader fixes that the bare SPEC leaves open; pinned above
# so the model is graded against the Contract, never a hidden guess):
#  - `build_site` CREATES output_dir if absent (rather than requiring it to pre-exist).
#  - The root page is the one with front-matter slug `/`; it emits `index.html`.
#  - A non-root slug `s` emits a file whose basename/stem is `s` (`s.html` OR `s/index.html`).
#  - The manifest enumerates generated files under a "file"/"page"/"output"-ish key,
#    or "outputs"/"results"/"build", or is itself a list; entries are path strings or
#    dicts keyed by path/output/dest/file/url/target/name. (Tolerant — see above.)
#  - Asset destination subpath is free; only basename + byte-identical contents are checked.
#  - Malformed front matter degrades gracefully (no crash); the malformed page's own
#    fate is unspecified, only build survival + well-formed pages rendering are checked.
#  - CLI stdout format is free; only exit 0 + files-on-disk are checked.
#  - Markdown structure is checked by HTML EQUIVALENCE (strong|b, em|i, h1..h6, etc.),
#    never by exact bytes.
````

**Graded behaviors:**

- `manifest_dict` — build_site returns a JSON-serializable dict manifest
- `manifest_lists_files` — manifest enumerates the generated files
- `index_emitted` — the slug '/' page is emitted as index.html on disk
- `about_emitted` — a non-root slug page is emitted to its own .html file
- `title_substituted` — {{ title }} is replaced with the front-matter title
- `content_substituted` — {{ content }} is replaced (no literal {{ content }} remains)
- `bold` — bold **x** renders as <strong>/<b>
- `italic` — italic *x* renders as <em>/<i>
- `heading` — heading '# H' renders as an <h1>..<h6> tag
- `inline_code` — inline `code` renders as <code>
- `fenced_code` — fenced code block renders inside <pre>/<code>
- `unordered_list` — unordered list renders as <ul> with <li> items
- `link` — link [t](u) renders as <a href="u">t</a>
- `nav_generated` — {{ nav }} is replaced with a nav referencing other pages
- `assets_copied` — files under assets/ are copied into the output (byte-identical)
- `deterministic` — two identical builds produce identical manifest + output set
- `malformed_frontmatter_survives` — malformed front matter does not crash the build
- `cli_build_exit0` — `python -m statichisel build` exits 0 and writes files to disk


### ticketflow · 18 checks

````
Start from an empty repository and implement a Python 3.11+ project named `ticketflow`.

Build a support ticket assignment engine. Use only the Python standard library.

Expose:

```python
def assign_tickets(config: dict, tickets: list[dict], agents: list[dict]) -> dict: ...
def explain_assignment(ticket: dict, agent: dict, config: dict) -> dict: ...
```

Tickets include:

```json
{
  "ticket_id": "t1",
  "priority": "high",
  "language": "en",
  "product": "billing",
  "created_at": "2026-01-01T12:00:00Z"
}
```

Agents include:

```json
{
  "agent_id": "a1",
  "languages": ["en"],
  "skills": ["billing"],
  "capacity": 3,
  "current_load": 1
}
```

Assignment rules:

```text
agent must have matching language
agent must have matching skill
agent must have available capacity
higher priority tickets assigned first
older tickets break ties
least-loaded qualified agent wins
agent_id breaks ties deterministically
```

Return assigned tickets and unassigned reasons.

Include a CLI:

```bash
python -m ticketflow assign --config config.json --tickets tickets.json --agents agents.json
```

Include tests for priorities, capacity, skill matching, language matching, deterministic tie-breaking, unassigned reasons, and explanation output.

## Contract

This section PINS the conventions the spec above leaves open. A conformant
solution MUST satisfy them; the held-out grader checks BEHAVIOR against them.

### Import path / CLI

- The package is importable as `ticketflow`, and the two public functions are
  importable from the module `ticketflow.public`:
      from ticketflow.public import assign_tickets, explain_assignment
- The CLI is invoked as a module: `python -m ticketflow assign --config C --tickets T --agents A`,
  where C, T, A are paths to JSON files holding the `config` dict, the `tickets`
  list, and the `agents` list respectively. CLI stdout MUST be a single JSON
  object (the same shape `assign_tickets` returns). The CLI exits 0 on success.

### Ticket / agent fields

- A ticket has: `ticket_id` (str), `priority` (str), `language` (str),
  `product` (str), `created_at` (ISO-8601 UTC timestamp string, e.g.
  "2026-01-01T12:00:00Z").
- An agent has: `agent_id` (str), `languages` (list of str), `skills` (list of
  str), `capacity` (int, max concurrent tickets), `current_load` (int, tickets
  already held). An agent has available capacity when `current_load < capacity`.
- The ticket's required SKILL is its `product` value: an agent is skill-qualified
  for a ticket when `ticket["product"] in agent["skills"]`.
- An agent is language-qualified for a ticket when
  `ticket["language"] in agent["languages"]`.
- An agent is QUALIFIED for a ticket when it is language-qualified AND
  skill-qualified AND has available capacity at the moment of assignment.

### Priority ordering (PINNED)

Priority is one of these string values, ordered most-urgent → least-urgent:

      "urgent" > "high" > "medium" > "low"

Higher-priority tickets are assigned before lower-priority ones. A priority value
not in this list is treated as the LEAST urgent (ranked below "low"); ties among
unknown priorities fall through to the remaining tie-breakers.

### Assignment order & tie-break order (PINNED)

Tickets are assigned greedily, one at a time, in this total order:

  1. priority DESCENDING (urgent first, per the ordering above)
  2. then `created_at` ASCENDING (older — earlier timestamp — first)
  3. then `ticket_id` ASCENDING (lexicographic) as a final stable tie-break

For the ticket currently being assigned, among all agents QUALIFIED for it at
that moment, the winning agent is chosen by:

  1. `current_load` ASCENDING (least-loaded qualified agent wins), where
     `current_load` reflects assignments ALREADY made during this call (each
     assignment increments the chosen agent's effective load by 1)
  2. then `agent_id` ASCENDING (lexicographic) as a deterministic final tie-break

Assignment is single-pass and greedy in the ticket order above: once a ticket is
assigned to an agent, that consumes one unit of that agent's capacity for the
rest of the call. The whole computation is deterministic and independent of the
input ordering of the `tickets` and `agents` lists.

### `assign_tickets` return shape (PINNED)

`assign_tickets` returns a dict with EXACTLY these two top-level keys:

      {
        "assigned":   { <ticket_id>: <agent_id>, ... },
        "unassigned": { <ticket_id>: <reason>,   ... }
      }

- `assigned` maps each assigned ticket's `ticket_id` (str) to the `agent_id`
  (str) it was assigned to.
- `unassigned` maps each unassigned ticket's `ticket_id` (str) to a `reason`
  string explaining why no agent took it. The reason string MUST be one of
  these PINNED values (lowercase, exact):
    - "no_language_match"  — no agent speaks the ticket's language
    - "no_skill_match"     — at least one agent speaks the language, but none has
                             the ticket's product in their skills
    - "no_capacity"        — at least one agent is language- AND skill-qualified,
                             but every such agent is at capacity (current_load,
                             including assignments made this call, == capacity)
  When more than one reason could apply, report the FIRST that applies in the
  order listed above (language, then skill, then capacity). Every ticket id
  appears in exactly one of `assigned` / `unassigned`, never both, never neither.

### `explain_assignment` return shape (PINNED)

`explain_assignment(ticket, agent, config)` returns a dict describing whether the
given agent could take the given ticket, with at least these keys:

      {
        "eligible":          <bool>,   # True iff language AND skill AND capacity all hold
        "language_match":    <bool>,   # ticket language in agent languages
        "skill_match":       <bool>,   # ticket product in agent skills
        "capacity_available": <bool>,  # agent current_load < capacity
      }

`eligible` is the AND of the three boolean factors. `explain_assignment` is a
pure predicate over the single (ticket, agent) pair given; it does not consider
other agents or mutate load.

# ASSUMES (conventions the spec under-pins; fixed here so the oracle never grades a guess):
#  - The ticket's required skill is its `product` field (the spec shows `product`
#    on tickets and `skills` on agents but never names the join key).
#  - Priority value set + ordering is urgent>high>medium>low; unknown = least.
#  - "available capacity" means current_load < capacity (strict), load counts
#    assignments made earlier in the same call.
#  - Return shape: {"assigned": {tid:aid}, "unassigned": {tid:reason}} with the
#    three pinned reason strings; explain_assignment exposes the three boolean
#    factors plus their AND as `eligible`.
````

**Graded behaviors:**

- `assign_basic` — a qualified agent gets a matching ticket; return has assigned/unassigned maps
- `priority_order` — higher-priority ticket wins the only seat over a lower-priority one
- `created_at_tiebreak` — same priority: older (earlier created_at) ticket wins the only seat
- `ticket_id_tiebreak` — same priority and created_at: lower ticket_id wins the only seat
- `language_match` — ticket goes only to an agent that speaks its language
- `skill_match` — ticket goes only to an agent whose skills include its product
- `capacity_respected` — an agent at current_load==capacity is never assigned a ticket
- `least_loaded_wins` — among qualified agents the least-loaded one is chosen
- `agent_id_tiebreak` — equal load qualified agents: lowest agent_id wins
- `load_accumulates` — assignments within one call consume capacity (no overfill)
- `reason_no_language` — unassigned reason 'no_language_match' when no agent speaks the language
- `reason_no_skill` — unassigned reason 'no_skill_match' when language matches but skill does not
- `reason_no_capacity` — unassigned reason 'no_capacity' when qualified agents are all full
- `partition` — every ticket id appears in exactly one of assigned/unassigned
- `explain_eligible` — explain_assignment reports eligible=True with all three factors true
- `explain_factors` — explain_assignment isolates language/skill/capacity failures
- `determinism` — assign_tickets is order-independent and stable across runs
- `cli_assign_json` — `python -m ticketflow assign` emits the contract JSON object


## bug-fix (15)

_Find and fix a planted bug in a small working library; a hidden test verifies._


### backoff · 16 checks

````
You have inherited a small Python library named `backoff`: an exponential-backoff
delay schedule for retrying failed operations, with optional "full jitter". The
package is already written, imports cleanly, and the rough shape looks right --
delays grow with the attempt number and stop growing once they hit a ceiling.
A retry loop asks `delay(attempt)` for how long to wait before each attempt, and
`bounds(attempt)` for the `(low, high)` range to draw a randomised "jittered"
sleep from.

## Bug report

The waits are wrong in ways that only show up once you look at the actual
numbers rather than the general shape:

  1. The very first attempt waits too long. Attempt 0 is supposed to wait
     exactly `base` seconds, but it comes back already multiplied -- the whole
     schedule is shifted one step early, so every attempt waits as if it were
     the next one.

  2. The ceiling does not actually cap anything. Under enough retries the delay
     keeps doubling without bound and shoots far past `cap`, instead of
     flattening out at `cap`. The clamp is being applied in the wrong place, so
     for the usual case (`base` smaller than `cap`) it never bites.

  3. Sub-second delays collapse to zero (or to a coarse whole number). With a
     fractional `base` like 0.5s the early waits come back as `0` and the
     schedule jumps in whole-second steps -- the fractional precision is being
     thrown away somewhere in the math.

  4. The jitter range is wrong. "Full jitter" is supposed to spread the wait
     uniformly over the WHOLE interval from 0 up to the delay, i.e.
     `(low, high) == (0.0, delay(attempt))`. Instead the low bound comes back as
     half the delay, so the randomised sleep can never drop below `delay/2` and
     the retries are far less spread out than intended.

Find and fix the defects so the schedule honours the contract below exactly.
Keep the public API and behaviour otherwise unchanged.

## Contract

- Package name: `backoff`. The grader imports `backoff.public` (falling back to
  `backoff`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change signatures):
      Backoff(base, factor, cap)
      Backoff.delay(attempt: int) -> float
      Backoff.bounds(attempt: int) -> tuple[float, float]
- `attempt` is a zero-based, non-negative integer (0 is the first retry).
- `delay(attempt)` returns the un-jittered wait, in seconds, as a FLOAT:
      delay(attempt) == min(cap, base * factor ** attempt)
    * Attempt 0 waits exactly `base` (the exponent is `attempt`, NOT
      `attempt + 1`): `factor ** 0 == 1`.
    * Compute the FULL exponential `base * factor ** attempt` first, and only
      THEN clamp to `cap`. The cap is applied to the final product, never to
      `base` before the exponent -- so a small `base` with a big exponent is
      still clamped down to `cap`.
    * Keep full floating-point precision: do NOT round, truncate, or
      integer-divide. A `base` of 0.5 must yield 0.5, not 0. The return value is
      always a float (even when it equals `cap` or `base`).
- `bounds(attempt)` returns the inclusive full-jitter range `(low, high)`:
      bounds(attempt) == (0.0, delay(attempt))
    * `low` is always `0.0` (full jitter starts at zero, not at half the delay).
    * `high` is the already-capped `delay(attempt)`, so the jitter window itself
      is never wider than `cap`.
    * Both bounds are floats.

## I/O example

    >>> b = Backoff(base=0.5, factor=2.0, cap=10.0)
    >>> b.delay(0)        # base, attempt-0 exponent is 0 -> 0.5 * 1
    0.5
    >>> b.delay(1)        # 0.5 * 2
    1.0
    >>> b.delay(2)        # 0.5 * 4
    2.0
    >>> b.delay(5)        # 0.5 * 32 = 16.0 -> clamped to cap
    10.0
    >>> b.bounds(0)       # full jitter over [0, delay(0)]
    (0.0, 0.5)
    >>> b.bounds(5)       # high is the CAPPED delay, low is 0
    (0.0, 10.0)

- Standard library only.
````

**Graded behaviors:**

- `attempt_zero_is_base` — delay(0) == base exactly (exponent is attempt, not attempt+1)
- `attempt_one_is_base_factor` — delay(1) == base * factor (one step into the schedule)
- `exponential_growth` — delays double each attempt below the cap (factor**attempt)
- `cap_clamps_high_attempt` — a high attempt is clamped down to cap, not grown past it
- `cap_after_exponent` — small base * big exponent is still capped (cap applied to the product)
- `never_exceeds_cap` — no attempt 0..40 ever returns more than cap
- `delay_below_cap_exact` — an un-capped delay equals base*factor**attempt exactly
- `fractional_base_preserved` — a fractional base (0.5) yields 0.5, not 0 (no truncation)
- `fractional_growth_preserved` — fractional delays keep precision as they grow (0.5,1.0,2.0...)
- `delay_returns_float` — delay always returns a float (even at base and at cap)
- `factor_non_integer` — a non-integer factor (1.5) gives exact float powers, not truncated
- `bounds_low_is_zero` — bounds(attempt) low is 0.0 (full jitter starts at zero)
- `bounds_high_is_delay` — bounds(attempt) high equals delay(attempt)
- `bounds_high_capped` — bounds high at a capped attempt equals cap, not the uncapped delay
- `bounds_are_floats` — both bounds are floats
- `bounds_match_delay_series` — bounds high tracks the full delay series across attempts


### base62 · 16 checks

````
You have inherited a small Python library named `base62`: a base-62 integer
codec. The package is already written, imports cleanly, and the happy path looks
fine for small inputs -- single-character codes encode and decode, and a few
spot checks pass. It is used to turn database row ids into short, URL-safe
strings (and back), so `encode` and `decode` must be exact inverses for every
non-negative id.

## Bug report

Under real use the codec corrupts ids, and the symptoms are easy to miss because
single-digit values (0..61) happen to look correct:

  1. `encode(0)` comes back as the EMPTY string `""` instead of `"0"`. The id 0
     is a real, valid id and must encode to a single `"0"` digit. No non-negative
     integer should ever encode to an empty string.

  2. Any value that needs more than one digit comes out with its digits in the
     WRONG ORDER -- reversed. `encode(62)` yields `"01"` when it should be
     `"10"`, so the encoded codes don't round-trip and don't even sort sensibly.
     The most-significant digit must come FIRST, exactly like decimal notation.

  3. `decode` accepts GARBAGE: handed a string containing a character that is not
     in the alphabet (say a space, `"-"`, or `"!"`), it silently returns a wrong
     (often negative) number instead of rejecting the input. Invalid characters
     must be refused.

  4. `decode("")` silently returns `0`. The empty string is not a valid encoding
     of anything and must be rejected too, not quietly treated as zero.

Find and fix the defects so the codec honours the contract below exactly. Keep
the public API and behaviour otherwise unchanged.

## Contract

- Package name: `base62`. The grader imports `base62.public` (falling back to
  `base62`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change signatures):
      encode(n: int) -> str
      decode(s: str) -> int
- The alphabet is fixed, exactly these 62 characters in this order (index 0 is
  `'0'`, index 10 is `'A'`, index 36 is `'a'`, index 61 is `'z'`):

      0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz

  So `'0'`->0, `'9'`->9, `'A'`->10, `'Z'`->35, `'a'`->36, `'z'`->61.
- `encode(n)` for a non-negative integer `n`:
    * Produces the base-62 representation with the MOST-SIGNIFICANT digit first
      (big-endian), just like ordinary decimal.
    * `encode(0)` is the single digit `"0"` -- never `""`.
    * Never returns the empty string for any `n >= 0`.
- `decode(s)`:
    * Maps each character to its alphabet index and accumulates big-endian:
      `value = value * 62 + index_of(char)`, left to right.
    * RAISES (a `ValueError`) on any character outside the alphabet -- it never
      returns a silent garbage or negative value.
    * RAISES on the empty string `""` -- it is not a valid encoding.
    * TOLERATES non-canonical leading zeros: a string with leading `'0'`
      characters still decodes to its plain value (`decode("0A") == 10`,
      `decode("00") == 0`). Leading zeros do not change the value and must not be
      rejected.
- Round-trip law: for every non-negative integer `n`, `decode(encode(n)) == n`
  -- including 0, values straddling the base boundary (61/62/63), and large
  multi-digit integers well beyond 64 bits.

## I/O example

    >>> from base62 import encode, decode
    >>> encode(0)
    '0'
    >>> encode(61)
    'z'
    >>> encode(62)            # most-significant digit first
    '10'
    >>> decode('10')
    62
    >>> decode('0A')          # leading zero tolerated, value preserved
    10
    >>> decode(encode(123456789))
    123456789
    >>> decode('!')           # character outside the alphabet
    Traceback (most recent call last):
        ...
    ValueError: invalid base-62 character: '!'
    >>> decode('')            # empty string is not a valid encoding
    Traceback (most recent call last):
        ...
    ValueError: decode received an empty string

- Standard library only.
````

**Graded behaviors:**

- `alphabet_single_digits` — every single alphabet char encodes/decodes to its index value
- `encode_zero` — encode(0) is the single digit '0', not the empty string
- `decode_zero` — decode('0') is 0
- `encode_small_multidigit` — encode(62) is '10' (big-endian, most-significant first)
- `encode_boundary_values` — values around the base boundary encode big-endian correctly
- `decode_known_value` — decode('10') is 62 and a few fixed strings decode correctly
- `roundtrip_small` — decode(encode(n)) == n for every n in 0..1000
- `roundtrip_powers` — round-trips hold at base powers and their neighbours
- `roundtrip_large` — round-trips hold for large multi-digit integers
- `encode_is_big_endian` — encode output matches an independent big-endian reference
- `decode_leading_zero_ok` — decode tolerates non-canonical leading-zero strings ('0A' -> 10)
- `no_empty_output` — no non-negative integer encodes to the empty string
- `decode_rejects_unknown` — decode raises on a character outside the alphabet
- `decode_rejects_unknown_midstring` — decode raises on a bad char even after valid digits
- `decode_rejects_empty` — decode raises on the empty string (not silently 0)
- `decode_no_silent_negative` — decode never yields a negative result for any string


### csvparse · 8 checks

````
You are handed an existing Python 3.11+ package named `csvparse`. It is already in
your working directory. Fix the bug described below. Use only the Python standard
library. Keep it a self-contained package — do not pull in the `csv` module or any
third-party library; the point is a correct hand-rolled parser.

## Bug report

`csvparse.public.parse_csv(text)` reads CSV text (a header row followed by data
rows) and returns a list of dicts, one per row, mapping each header column to its
value. Plain rows parse fine. But rows that contain a QUOTED field with a comma
inside it are parsed into the WRONG NUMBER OF COLUMNS — the comma inside the quotes
is wrongly treated as a field separator, so the row is split into extra columns and
every column after that field is shifted/misaligned.

Repro:

    text = (
        "name,role,city\n"
        'Ada,"Smith, Jr.",London\n'
        "Bob,Engineer,Paris\n"
    )
    rows = parse_csv(text)

Expected:
    rows[0] == {"name": "Ada", "role": "Smith, Jr.", "city": "London"}
    rows[1] == {"name": "Bob", "role": "Engineer", "city": "Paris"}

Actual (buggy): the first row comes back with `role` == "Smith" and `city` == "Jr."
(the trailing "London" is dropped/misplaced) because the field was split on the
comma inside the quotes. The plain second row is unaffected.

## Contract

  - Package name stays `csvparse`; keep the public API `parse_csv(text) -> list[dict]`
    exposed from `csvparse.public` (and re-exported from `csvparse`).
  - The first non-empty record is the header row; each later record is one dict
    mapping header name -> field value, in header order.
  - Parse RFC-4180-style quoted fields correctly:
      * A field may be wrapped in double quotes; the surrounding quotes are stripped.
      * A comma inside a quoted field is part of the value, NOT a separator.
      * A doubled quote (`""`) inside a quoted field is an escaped literal `"`
        (e.g. the field `"She said ""hi"""` has the value: She said "hi").
  - Plain unquoted rows must keep working exactly as before.
  - Unquoted fields keep their plain text; an unquoted field never has quotes
    stripped from its interior.

Do not change the package name or the `parse_csv` signature. Fix the parser so the
repro above (and other quoted-comma / doubled-quote cases) parses into the correct
number of columns with correct header mapping.
````

**Graded behaviors:**

- `header_plain` — rows=[{'name': 'Ada', 'role': 'Engineer', 'city': 'London'}]
- `plain_multi_row` — rows=[{'a': '1', 'b': '2'}, {'a': '3', 'b': '4'}, {'a': '5', 'b': '6'}]
- `quoted_comma_single` — rows=[{'name': 'Ada', 'role': 'Smith, Jr.', 'city': 'London'}]
- `quoted_comma_alignment` — rows=[{'name': 'Ada', 'role': 'Smith, Jr.', 'city': 'London'}, {'name': 'Bob', 'role': 'Engineer', 'city': 'Paris'}]
- `doubled_quote_escape` — rows=[{'name': 'Ada', 'quote': 'She said "hi"'}]
- `quoted_comma_from_file` — rows=[{'product': 'Widget', 'note': 'red, large, sturdy', 'price': '9.99'}]
- `multiple_quoted_fields` — rows=[{'a': 'x, y', 'b': 'p, q, r', 'c': 'z'}]
- `plain_unquoted_unchanged` — rows=[{'id': '42', 'label': 'hello world'}]


### deepget · 22 checks

````
You have inherited a small Python library named `deepget`: two helpers for
reaching into nested data by a dotted-string path. `get(obj, path, default)`
reads a value out of arbitrarily nested dicts and lists; `set_(obj, path, value)`
writes one, creating any missing structure on the way. The package is already
written, imports cleanly, and the happy path works -- plain nested dicts read
and write fine, and indexing into a list with a numeric segment works for the
common case.

It is used to pull fields out of decoded JSON config and API payloads, where a
path like `servers.0.host` means "the `host` of the first element of the
`servers` list".

## Bug report

The helpers misbehave on the trickier shapes, and the failures are easy to miss
because the obvious cases look fine:

  1. A field that is genuinely PRESENT but set to `null`/`None` reads back as if
     it were missing -- `get` hands back the `default` instead of the real,
     stored `None`. Code that distinguishes "absent" from "explicitly null" (a
     cleared optional, say) can't tell the two apart.

  2. A numeric-looking segment is ALWAYS treated as a list index, even when the
     current container is a dict whose keys happen to be digit strings (think a
     JSON object like `{"2024": {...}}` keyed by year, or sparse maps keyed by
     id). Those entries become completely unreachable, and the lookup blows up
     or silently returns the default.

  3. `set_` only ever creates plain dicts for the missing intermediate steps, so
     a path that needs a LIST in the middle (e.g. `items.0.name` starting from
     an empty object) builds the wrong shape -- a dict keyed by the string "0"
     instead of a one-element list. Later `get`s along the intended path then
     fail.

  4. Out-of-range and negative list indices are mishandled on read: a path that
     runs off the end of a list raises instead of falling back to `default`, and
     a negative index quietly wraps around to grab an element from the END of the
     list rather than failing. Both should simply be treated as "not found".

Find and fix the defects so the helpers honour the contract below exactly. Keep
the public API and behaviour otherwise unchanged.

## Contract

- Package name: `deepget`. The grader imports `deepget.public` (falling back to
  `deepget`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change signatures):
      get(obj, path: str, default=None) -> value or default
      set_(obj, path: str, value) -> the (possibly new) root object
- A `path` is a `.`-separated string of segments, walked left to right, each
  segment selecting into the CURRENT container:
    * dict: the segment is a key. Try the raw STRING key first; only if that is
      absent and the segment is all digits, try the INTEGER key too. (So both
      `{"0": x}` and `{0: x}` are reachable via the segment `"0"`, string first.)
    * list: the segment must be a non-negative integer index that is in range
      (`0 <= i < len`). A non-numeric segment, a negative index, or an
      out-of-range index does NOT resolve.
    * anything else (e.g. descending into an int or string): does NOT resolve.
  "All digits" means `seg.isdigit()` -- no sign, no decimal point.
- `get(obj, path, default=None)`:
    * Returns the resolved value when every segment resolves. A resolved value
      of `None` is returned AS-IS -- a present `None` is a real value, NOT a
      miss.
    * Returns `default` if and only if some segment fails to resolve (missing
      key, bad/negative/out-of-range index, or descending into a non-container).
    * Never raises for an unresolvable path; it returns `default`.
    * An empty path (`""`) resolves to `obj` itself.
- `set_(obj, path, value)`:
    * Walks `obj` and stores `value` at the final segment, creating any missing
      intermediate container as it goes. The type of a freshly created
      intermediate depends on the NEXT segment: a numeric next segment creates a
      LIST, otherwise a `dict`.
    * Assigning into a list at an index past its end PADS the list with `None`
      up to that index, then assigns (so `set_([], "2", v)` yields `[None, None, v]`).
    * Returns the root object. For an empty path it returns `obj` unchanged.
    * Mutates in place where possible; it must not replace an existing
      list/dict intermediate that is already the right kind of container.

## I/O example

    >>> obj = {"a": {"b": [{"c": 1}, {"c": 2}]}}
    >>> get(obj, "a.b.1.c")
    2
    >>> get(obj, "a.b.9.c", default="?")     # index off the end -> default
    '?'
    >>> get(obj, "a.b.-1.c", default="?")    # negative does NOT wrap -> default
    '?'
    >>> get({"x": None}, "x", default="?")   # present None -> the None, not default
    >>> get({"2024": {"q": 4}}, "2024.q")    # digit-string DICT key, not a list index
    4
    >>> set_({}, "items.0.name", "hi")       # numeric next seg -> a LIST is created
    {'items': [{'name': 'hi'}]}
    >>> set_([], "2", "z")                    # pad past the end with None
    [None, None, 'z']

- Standard library only.
````

**Graded behaviors:**

- `get_nested_dict` — get walks plain nested dict keys
- `get_list_index` — get indexes a list with a numeric segment
- `get_mixed` — get walks a mixed dict/list path to a leaf
- `get_missing_key` — a missing dict key returns default
- `get_missing_returns_exact_default` — the exact default object is returned on a miss
- `get_empty_path` — an empty path resolves to obj itself
- `get_present_none` — a present None value is returned, not the default
- `get_none_deep` — a present None deep in the tree is returned, not default
- `get_none_vs_missing` — present-None and missing-key are distinguished
- `get_digit_string_dict_key` — a digit-string dict key is reachable (not forced to a list index)
- `get_int_dict_key` — an int dict key is reachable via the digit segment
- `get_string_key_wins_over_int` — the string key is tried before the int key
- `get_index_out_of_range` — an out-of-range list index returns default (no raise)
- `get_negative_index` — a negative list index returns default (no end-wrap)
- `get_descend_non_container` — descending into a non-container returns default
- `get_no_raise_on_bad_path` — an unresolvable path never raises
- `set_nested_dict` — set_ creates nested dicts and stores the value
- `set_into_existing` — set_ mutates an existing nested dict in place
- `set_creates_list_intermediate` — a numeric next segment makes set_ create a list
- `set_pads_list` — set_ pads a list with None past its end
- `set_preserves_sibling` — set_ does not clobber a sibling container it descends through
- `set_roundtrip` — a value written by set_ is readable by get along the same path


### graphpath · 18 checks

````
You have inherited a small Python library named `graphpath`: a shortest-path
finder over a weighted directed graph, using Dijkstra's algorithm. The package is
already written, imports cleanly, and the happy path works -- on a graph where the
greedy first route to each node also happens to be the cheapest, it returns the
right distance. It is used by a routing layer that asks `shortest(graph, src, dst)`
for the least-cost route between two nodes.

## Bug report

Under real graphs the finder misbehaves, and the symptoms are easy to miss
because small "already-sorted" example graphs look fine:

  1. On graphs where a node is first reached by an EXPENSIVE edge but a CHEAPER
     route to it exists through other nodes, the finder reports the expensive
     distance and route -- it seems to LOCK IN the first way it stumbles onto a
     node and never reconsiders, even when a shorter path is found a moment
     later. The answer comes back larger than the true shortest path.

  2. Asking for the route from a node to ITSELF (`src == dst`) returns the right
     distance (0) but an empty path. It should return the one-node path
     containing just that node.

  3. When `dst` simply CANNOT be reached from `src` (no directed route exists),
     the finder does not say so cleanly -- it hands back a bogus
     infinite-distance result with a nonsense path instead of signalling "no
     route" the way the contract requires.

  4. Even when the distance is right, the returned path reads BACKWARDS --
     from `dst` to `src` rather than from `src` to `dst`.

Find and fix the defects so the finder honours the contract below exactly.
Keep the public API and behaviour otherwise unchanged.

## Contract

- Package name: `graphpath`. The grader imports `graphpath.public` (falling back
  to `graphpath`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change the signature):
      shortest(graph, src, dst) -> (distance, path) | None
- Graph representation: an adjacency map `{node: {neighbor: weight}}`. Every
  weight is a non-negative number (ints or floats; `0` is allowed). A node with
  no outgoing edges maps to an empty dict `{}`. Nodes are any hashable key. The
  graph is DIRECTED: an edge `a -> b` does NOT imply `b -> a`.
- `shortest(graph, src, dst)` returns the minimum-total-weight route from `src`
  to `dst`:
    * On success, return the pair `(distance, path)` where `distance` is the
      summed edge weight of the cheapest route and `path` is the list of nodes
      from `src` to `dst` INCLUSIVE, in travel order (`path[0] == src`,
      `path[-1] == dst`). Consecutive nodes in `path` must be joined by real
      edges whose weights sum to exactly `distance`.
    * A node always reaches itself: if `src == dst`, return `(0, [src])` -- the
      single-node path -- even when that node has no outgoing edges.
    * If `dst` is UNREACHABLE from `src` (no directed route exists), return
      `None`. Do not return an infinite/sentinel distance or a partial path.
- Algorithm requirement (Dijkstra correctness): a node's distance is only final
  once it is settled as the closest un-settled node. A cheaper route discovered
  AFTER a node was first encountered must still win -- do not freeze a node's
  distance the first time any edge happens to reach it.

## I/O example

    >>> g = {"a": {"b": 1, "c": 4}, "b": {"c": 2, "d": 5}, "c": {"d": 1}, "d": {}}
    >>> shortest(g, "a", "d")        # a->b->c->d = 1+2+1
    (4, ['a', 'b', 'c', 'd'])
    >>> shortest(g, "a", "a")        # a node reaches itself for free
    (0, ['a'])
    >>> shortest(g, "d", "a")        # no route out of d -> a
    None
    >>> # a cheaper route found LATE still wins:
    >>> g2 = {"a": {"d": 10, "b": 1}, "b": {"c": 1}, "c": {"d": 1}, "d": {}}
    >>> shortest(g2, "a", "d")       # a->b->c->d = 3, not the direct a->d = 10
    (3, ['a', 'b', 'c', 'd'])

- Standard library only.
````

**Graded behaviors:**

- `single_edge` — a one-hop graph returns (weight, [src, dst])
- `chain_distance` — a simple chain returns the summed distance
- `chain_path` — a simple chain returns the full node list in src->dst order
- `picks_cheaper_branch` — the cheaper of two branches is chosen for distance and path
- `path_is_forward` — the path runs src -> dst, not reversed
- `late_cheap_route_distance` — a cheaper route discovered LATER wins (no settle-on-discovery)
- `late_cheap_route_path` — the late-discovered cheaper route is reflected in the path
- `relax_after_seen` — a node first seen via an expensive edge is later relaxed to the cheap one
- `src_eq_dst_distance` — src == dst has distance 0
- `src_eq_dst_path` — src == dst returns the single-node path [src]
- `src_eq_dst_no_outgoing` — src == dst works for a node with no outgoing edges
- `unreachable_is_none` — an unreachable dst returns None (not inf, not a tuple)
- `unreachable_isolated` — a node with no path to dst returns None
- `unreachable_wrong_direction` — a directed edge does not imply the reverse route exists
- `path_endpoints` — the path begins at src and ends at dst
- `path_is_valid_walk` — consecutive path nodes are joined by real edges summing to the distance
- `multi_hop_unique` — a longer unique-shortest route is found exactly (distance and path)
- `zero_weight_edges` — zero-weight edges are handled without skipping or looping


### intervalmerge · 19 checks

````
You have inherited a small Python library named `intervalmerge`: half-open
interval algebra. An interval is a pair `(start, end)` meaning the half-open
range `start <= x < end`. The package is already written, imports cleanly, and
the happy path works -- two clearly-overlapping, already-sorted intervals merge,
and a single hole in the middle of an interval splits it in two. It is used to
compute coverage windows: `merge` collapses a pile of ranges into canonical
form, and `subtract` removes blocked-out ranges from available ones.

## Bug report

Under real data the results are subtly wrong at the edges; coarse tests on
clean, sorted, clearly-overlapping inputs look fine and hide it:

  1. Two intervals that merely TOUCH come back as two intervals instead of one.
     Because the ranges are half-open, `[1, 2)` and `[2, 3)` are contiguous --
     together they cover exactly `[1, 3)` with no gap and no overlap -- and they
     are supposed to merge into `[1, 3)`. They don't; the seam is left open.

  2. If the input is not already sorted by start, `merge` produces garbage --
     it drops or duplicates ranges -- as if it assumed the caller pre-sorted.
     `subtract` inherits this, mangling any unsorted `a`.

  3. Zero-width ranges leak through. A `[x, x)` interval covers NO points and
     should simply vanish, but `merge` keeps it, and `subtract` manufactures
     them: removing a chunk that is flush with an interval's edge leaves a
     bogus `[edge, edge)` stub, and fully covering an interval yields a
     `[x, x)` instead of nothing at all.

  4. When two of the ranges being subtracted OVERLAP each other, `subtract`
     corrupts the output -- it can even emit a reversed `(hi, lo)` interval --
     because it lines `b` up by start but never coalesces the overlaps before
     carving them out.

Find and fix the defects so `merge` and `subtract` honour the contract below
exactly. Keep the public API and behaviour otherwise unchanged.

## Contract

- Package name: `intervalmerge`. The grader imports `intervalmerge.public`
  (falling back to `intervalmerge`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change signatures):
      merge(intervals) -> list[tuple]
      subtract(a, b)   -> list[tuple]
  where `intervals`, `a`, `b` are iterables of `(start, end)` pairs and each
  returned list is a list of `(start, end)` tuples.
- An interval `(start, end)` is the half-open range `start <= x < end`. A range
  with `start == end` is ZERO-WIDTH: it covers no points and must never appear
  in any output (neither passed through nor produced).
- Both functions return a NORMALISED list: sorted ascending by `start`,
  pairwise DISJOINT, and zero-width-free -- the unique minimal canonical form
  of the point set. Neither function may mutate its input arguments.
- `merge(intervals)`:
    * Returns the minimal set of disjoint intervals covering exactly the same
      points as the union of the inputs.
    * OVERLAPPING intervals merge. TOUCHING intervals merge too: since the
      ranges are half-open, `end == start` of the next means they are
      contiguous (no gap), so `[1, 2)` and `[2, 3)` become `[1, 3)`. A real gap
      (`[1, 2)` and `[3, 4)`) is preserved as two intervals.
    * Input may be in ANY order and may contain overlaps, duplicates, and
      zero-width ranges; the output is always canonical.
- `subtract(a, b)`:
    * Returns the canonical intervals covering every point in (the union of)
      `a` that is NOT in (the union of) `b`.
    * Where a `b` range lies strictly inside an `a` range, that `a` range is
      SPLIT into its left and right remainders. A `b` range flush with an edge
      trims that side with no leftover stub. A `b` range covering an `a` range
      entirely removes it (contributing nothing).
    * `a` and `b` may each be unsorted and may contain overlapping or
      zero-width ranges; both are normalised before the subtraction.

## I/O example

    >>> from intervalmerge import merge, subtract
    >>> merge([(2, 4), (1, 3)])         # unsorted + overlapping
    [(1, 4)]
    >>> merge([(1, 2), (2, 3)])         # touching half-open -> contiguous
    [(1, 3)]
    >>> merge([(1, 1), (2, 4)])         # zero-width [1,1) dropped
    [(2, 4)]
    >>> merge([(1, 2), (3, 4)])         # real gap preserved
    [(1, 2), (3, 4)]
    >>> subtract([(0, 10)], [(3, 5)])   # punch a hole -> split in two
    [(0, 3), (5, 10)]
    >>> subtract([(0, 10)], [(0, 3)])   # flush with the left edge -> no [0,0)
    [(3, 10)]
    >>> subtract([(0, 10)], [(0, 10)])  # fully covered -> nothing
    []
    >>> subtract([(0, 10)], [(3, 6), (5, 8)])  # overlapping holes coalesced
    [(0, 3), (8, 10)]

- Standard library only.
````

**Graded behaviors:**

- `merge_overlap_sorted` — sorted overlapping intervals merge into one
- `merge_nested` — an interval fully inside another merges to the outer
- `merge_gap_preserved` — a real gap between intervals is preserved (not merged)
- `merge_empty` — merging an empty list returns an empty list
- `merge_touching_halfopen` — touching half-open intervals [1,2)+[2,3) merge to [1,3)
- `merge_chain_touch` — a chain of touching intervals collapses to one span
- `merge_unsorted` — unsorted overlapping input is merged correctly
- `merge_unsorted_touch` — unsorted touching input is merged correctly
- `merge_zero_width_dropped` — a zero-width [x,x) input interval is dropped
- `merge_no_mutate` — merge does not mutate or reorder its input argument
- `subtract_single_mid_split` — a hole in the middle splits an interval into two
- `subtract_two_holes` — two interior holes split an interval into three pieces
- `subtract_flush_left` — a hole flush with the left edge leaves no zero-width piece
- `subtract_flush_right` — a hole flush with the right edge trims cleanly
- `subtract_full_cover` — a fully-covered interval yields nothing (no zero-width)
- `subtract_overlapping_holes` — overlapping holes in b are coalesced before carving
- `subtract_unsorted_a` — unsorted a is normalised before subtracting
- `subtract_empty_b` — subtracting nothing returns a normalised copy of a
- `subtract_adjacent_holes_touch` — touching holes carve a single contiguous gap


### ledgerfix · 10 checks

````
BUG REPORT — ledgerfix: transfers can create money / unbalance the books

You have an existing Python package `ledgerfix` (an in-memory double-entry money
ledger over integer cents). There is ONE bug. Find it and fix it. Keep the
public API exactly as it is; do not rewrite the package from scratch.

## Symptom

When `transfer(src, dst, amount)` is called and the SOURCE account is underfunded
(does not hold at least `amount`), the transfer is supposed to be rejected and
leave both balances untouched. Instead, money appears from nowhere: the
destination gets credited while the source is left unchanged, so the ledger's
grand total goes UP. The books no longer balance.

## Reproduction

    from ledgerfix import Ledger, InsufficientFunds

    led = Ledger()
    led.open_account("alice", 100)   # alice has 100 cents
    led.open_account("bob", 0)       # bob has 0 cents

    total_before = led.total_cents()  # 100

    # alice only has 100; this transfer of 500 must be rejected.
    try:
        led.transfer("alice", "bob", 500)
    except InsufficientFunds:
        pass  # expected: rejected, nothing moves

    print(led.balance("alice"))   # EXPECTED 100 ... ACTUAL 100
    print(led.balance("bob"))     # EXPECTED 0   ... ACTUAL 500  (!!)
    print(led.total_cents())      # EXPECTED 100 ... ACTUAL 600  (money created!)

A normal, fully-funded transfer (e.g. alice -> bob of 40 when alice has 100)
works fine and conserves the total — only the underfunded case is broken.

## Contract (must hold after your fix)

* Package name stays `ledgerfix`; import path `ledgerfix` / `ledgerfix.public`.
* Keep the existing public API and its names:
  - `Ledger.open_account(account, opening_cents=0)`
  - `Ledger.deposit(account, amount_cents) -> new_balance`
  - `Ledger.withdraw(account, amount_cents) -> new_balance`  (rejects insufficient funds)
  - `Ledger.transfer(src, dst, amount_cents) -> None`
  - `Ledger.balance(account) -> int`
  - `Ledger.total_cents() -> int`
  - `Ledger.accounts() -> dict`
  - exceptions `LedgerError`, `UnknownAccount`, `InsufficientFunds`
* All amounts are non-negative integer cents; accounts may never go negative.
* `transfer` must be ATOMIC: either BOTH legs apply (source debited, destination
  credited) or NEITHER does. An underfunded source must reject the transfer and
  leave BOTH balances exactly as they were.
* A valid (fully funded) transfer must move the funds and conserve the grand
  total returned by `total_cents()`.
* Deposits and withdrawals on a single account must keep working as before.

Do not change the package name or the public function/exception names.
````

**Graded behaviors:**

- `deposit_basic` — deposit credits the account by the given cents
- `withdraw_basic` — withdraw debits a funded account by the given cents
- `withdraw_underfunded_rejected` — an underfunded withdraw raises and leaves the balance unchanged
- `transfer_valid_moves` — a fully-funded transfer moves the funds
- `transfer_valid_conserves_total` — a fully-funded transfer conserves the grand total
- `transfer_underfunded_atomic` — an underfunded transfer leaves BOTH balances untouched (both legs or neither)
- `transfer_underfunded_conserves_total` — an underfunded transfer does not change the grand total (no money created)
- `transfer_underfunded_dest_untouched` — a rejected transfer never credits the destination
- `transfer_exact_funds_ok` — a transfer of the full balance succeeds and conserves the total
- `books_stay_balanced_over_sequence` — after mixed deposits/withdrawals/transfers the total equals net external flow


### lrucache · 16 checks

````
You have inherited a small Python library named `lrucache`: a fixed-capacity
least-recently-used (LRU) cache. The package is already written, imports
cleanly, and the happy path works -- you can `put` keys, read them back with
`get`, overwrite a value, and fill the cache up to its capacity. It is used as
an in-memory cache in front of a slow store: hot keys should stay resident and
the coldest key should be the one dropped when room is needed.

## Bug report

Under real traffic the cache misbehaves once it is full and entries start
getting evicted. The symptoms are hard to pin down because small, fill-and-read
tests that never push past capacity look completely fine:

  1. A key that is read CONSTANTLY still gets thrown out. A value the callers
     touch on nearly every request -- clearly the hottest, most-recently-used
     entry -- disappears from the cache anyway while colder keys survive. Reading
     a key is supposed to mark it as freshly used.

  2. When the cache makes room it drops the WRONG entry. Instead of evicting the
     stalest key, it throws away one of the freshest -- often the very key that
     was just inserted -- so brand-new writes vanish immediately while ancient
     keys linger.

  3. OVERWRITING an existing key doesn't seem to "count as a use". After you
     re-`put` a key with a new value (the value does update correctly), that key
     is still treated as stale and gets evicted as if it had not been touched.

  4. The cache holds MORE than it should: with capacity N you can find N+1 keys
     resident at once, and a cache built with capacity 0 -- which should store
     nothing at all -- happily hands a value back.

Find and fix the defects so the cache honours the contract below exactly. Keep
the public API and behaviour otherwise unchanged.

## Contract

- Package name: `lrucache`. The grader imports `lrucache.public` (falling back
  to `lrucache`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change signatures):
      LRUCache(capacity)                       # capacity is a non-negative int
      LRUCache.get(key) -> value | LRUCache.MISSING
      LRUCache.put(key, value) -> None
      LRUCache.MISSING                          # sentinel returned by get on a miss
- The cache holds at most `capacity` entries. It must NEVER hold more than
  `capacity` at any moment. A `capacity` of 0 means the cache stores nothing:
  every `get` is a miss.
- Recency model -- entries are ordered from least-recently-used (LRU) to
  most-recently-used (MRU). A key becomes the MRU end on every SUCCESSFUL use:
    * `get(key)` on a HIT returns the stored value AND moves that key to the MRU
      end. A `get` on a MISS returns the `MISSING` sentinel and changes nothing
      (it must not reorder, insert, or evict anything).
    * `put(key, value)` for a key already present updates the stored value AND
      moves that key to the MRU end. It is an update, not a new entry, so it
      never evicts anyone and never changes the number of entries.
    * `put(key, value)` for a NEW key inserts it at the MRU end. If the cache is
      already full (`capacity` entries), the current LRU entry is evicted FIRST
      to make room, so the count stays at `capacity`.
- Eviction always removes the single LEAST-recently-used entry -- the one whose
  key has gone the longest without a successful `get` or `put`. Never the MRU,
  never a freshly-touched key.
- `get` must not raise on a missing key; it returns `LRUCache.MISSING`.

## I/O example

    >>> c = LRUCache(capacity=2)
    >>> c.put("a", 1)
    >>> c.put("b", 2)
    >>> c.get("a")                  # hit: "a" is now most-recently-used
    1
    >>> c.put("c", 3)               # full -> evict the LRU, which is "b"
    >>> c.get("b") is c.MISSING     # "b" was the stalest, so it is gone
    True
    >>> c.get("a")                  # "a" survived because the get refreshed it
    1
    >>> c.get("c")
    3
    >>> c.put("a", 11)              # overwrite: updates value AND refreshes "a"
    >>> c.put("d", 4)              # full -> evict the LRU, which is now "c"
    >>> c.get("c") is c.MISSING
    True
    >>> c.get("a")                  # the re-put kept "a" fresh
    11

    >>> z = LRUCache(capacity=0)    # a zero-capacity cache stores nothing
    >>> z.put("x", 1)
    >>> z.get("x") is z.MISSING
    True

- Standard library only.
````

**Graded behaviors:**

- `put_get_roundtrip` — a stored key reads back its value
- `miss_returns_sentinel` — an absent key returns the MISSING sentinel
- `update_existing_value` — putting an existing key overwrites its value in place
- `fill_to_capacity_keeps_all` — filling exactly to capacity keeps every entry
- `evict_lru_basic` — inserting past capacity evicts the least-recently-used key
- `evict_keeps_most_recent` — the most-recently-inserted key survives an eviction
- `len_never_exceeds_capacity` — the cache never holds more than capacity entries
- `get_refreshes_recency` — a successful get protects its key from the next eviction
- `get_miss_no_recency_side_effect` — a missing get does not alter eviction order
- `put_update_refreshes_recency` — overwriting an existing key protects it from eviction
- `evict_picks_true_lru_after_gets` — after mixed gets the evicted key is the real LRU
- `capacity_zero_stores_nothing` — a capacity-0 cache never retains an entry
- `capacity_one_replaces` — a capacity-1 cache holds only the latest key
- `repeated_get_hot_key_survives` — a repeatedly-read hot key is never evicted
- `eviction_sequence_order` — a sequence of inserts evicts keys in LRU order
- `update_does_not_grow` — overwriting an existing key does not evict anyone


### luhn · 19 checks

````
You have inherited a small Python library named `luhn`: a validator and
check-digit generator for the Luhn (mod-10) checksum -- the scheme that guards
credit-card numbers, IMEIs, and similar identifiers. The package is already
written, imports cleanly, and the happy path works -- it validates the textbook
example number and rejects an obviously wrong one.

## Bug report

Under real input the library quietly accepts bad numbers and rejects good ones,
and the symptoms are length- and content-dependent, so a couple of hand-picked
examples look fine:

  1. Validation depends on the LENGTH of the number in a way it shouldn't. Some
     genuinely valid card numbers are rejected, while some numbers with a single
     mistyped digit are accepted -- and which ones flip seems to track whether
     the number has an odd or an even count of digits. It is as if the "every
     second digit" doubling is being measured from the wrong end, so the wrong
     digits get doubled whenever the length's parity changes.

  2. Numbers that should validate are rejected whenever doubling a digit pushes
     it to 10 or more. A digit like 8, once doubled to 16, is contributing the
     wrong amount to the checksum -- the two decimal digits of the doubled value
     are not being folded back together the way Luhn requires.

  3. `check_digit` returns the wrong digit exactly when the correct answer is 0.
     A partial number whose proper check digit is 0 instead gets a 10 (or some
     other nonsense), and appending it produces a number that does not validate.
     Every other check digit looks right, so the bug hides until a 0 comes up.

  4. Input that contains spaces or other separators -- the way humans actually
     write card numbers, in groups -- is mishandled, and so is empty / non-digit
     input. Grouped numbers that should validate are rejected, and junk that
     should be cleanly rejected instead slips through or raises.

Find and fix the defects so the library honours the contract below exactly.
Keep the public API and behaviour otherwise unchanged.

## Contract

- Package name: `luhn`. The grader imports `luhn.public` (falling back to
  `luhn`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change signatures):
      is_valid(number: str) -> bool
      check_digit(partial: str) -> int

- The Luhn checksum, defined precisely:
    * Take the string of digits. Walk it from the RIGHTMOST digit leftward.
    * DOUBLE every second digit counting from the right: the rightmost digit is
      NOT doubled, the next one to its left IS doubled, the next is not, and so
      on. (Equivalently: 0-indexing positions from the right starting at 0, the
      ODD-indexed positions -- 1, 3, 5, ... -- are doubled.)
    * Whenever doubling yields a value greater than 9, fold its digits: replace
      it with the sum of its two decimal digits, which is the same as
      subtracting 9 (e.g. 8 -> 16 -> 1+6 = 7; 6 -> 12 -> 1+2 = 3; 9 -> 18 -> 9).
    * Sum all the resulting values (doubled-and-folded where applicable, plain
      otherwise).
    * The number is VALID iff that total is a multiple of 10 (total % 10 == 0).

- `is_valid(number)`:
    * Returns True iff the cleaned digit string passes the Luhn checksum above.
    * Separators are tolerated and ignored: ASCII spaces are stripped before
      validating, so "4539 1488 0343 6467" is treated exactly like
      "4539148803436467".
    * Anything that, after stripping spaces, is EMPTY or contains a non-digit
      character returns False (never raises). An empty string is not a valid
      number. Note "0" (a single zero) is, by the checksum, valid.

- `check_digit(partial)`:
    * `partial` is the number SO FAR, without its final check digit. Return the
      single digit (an int in 0..9) that, when APPENDED to the right of
      `partial`, makes the whole thing pass `is_valid`.
    * Concretely: that appended digit sits at right-position 0 (it is not
      doubled). Compute the Luhn total of `partial` as if it had a 0 appended
      (so `partial`'s own digits all shift one place left, changing which get
      doubled), then the check digit is the amount needed to round that total up
      to the next multiple of 10. When the total is already a multiple of 10 the
      check digit is 0 (NOT 10).
    * Spaces in `partial` are stripped just as in `is_valid`.

## I/O example

    >>> from luhn import is_valid, check_digit
    >>> is_valid("4539148803436467")     # a valid Visa test number
    True
    >>> is_valid("4539 1488 0343 6467")  # same number, spaced groups
    True
    >>> is_valid("4539148803436466")     # last digit wrong
    False
    >>> is_valid("79927398713")          # classic 11-digit Luhn example (odd length)
    True
    >>> is_valid("0")                    # single zero: total 0, a multiple of 10
    True
    >>> is_valid("")                     # empty is not a number
    False
    >>> is_valid("12 34a")               # non-digit after stripping spaces
    False
    >>> check_digit("7992739871")        # appending 3 -> 79927398713 validates
    3
    >>> check_digit("123456781234567")   # this partial's check digit is 0
    0
    >>> is_valid("1234567812345670")     # ...and 0 appended validates
    True

- Standard library only.
</content>
</invoke>
````

**Graded behaviors:**

- `textbook_valid` — the classic 11-digit Luhn number validates
- `textbook_invalid` — a number with one wrong digit is rejected
- `valid_even_length` — a known-valid EVEN-length (16-digit) card validates
- `valid_odd_length` — a known-valid ODD-length number validates
- `parity_invalid_even` — a one-digit-off even-length number is rejected
- `parity_invalid_odd` — a one-digit-off odd-length number is rejected
- `parity_nofold_valid` — valid numbers with NO doubled-digit>9 validate (isolates the doubling end)
- `parity_nofold_invalid` — a non-doubled-digit error in a no-fold number is rejected
- `fold_over_nine` — a number exercising doubled digits >9 scores correctly
- `fold_distinguishes` — -9 folding (not -10) is what makes the valid one valid
- `spaces_ignored` — a spaced/grouped number validates like its compact form
- `spaces_invalid` — a spaced number with a bad digit is still rejected
- `empty_invalid` — the empty string is not valid
- `nondigit_invalid` — non-digit input returns False (does not raise)
- `single_zero_valid` — the single digit '0' is valid (total 0)
- `check_digit_basic` — check_digit on a partial returns the validating digit
- `check_digit_zero` — check_digit returns 0 (not 10) when the total is a multiple of 10
- `check_digit_roundtrip` — appending check_digit makes every sampled partial valid
- `validity_sweep` — is_valid matches the reference across mixed-length samples


### movavg · 16 checks

````
You have inherited a small Python library named `movavg`: a fixed-size sliding-
window aggregator. The package is already written, imports cleanly, and the
happy path works -- you push values in with `add`, and once enough have arrived
`mean()`, `min()` and `max()` report statistics over the most recent window.
It is used to summarise a live metric stream: a dashboard pushes each new
sample into a `Window(size)` and reads back the rolling average and extremes.

## Bug report

The rolling stats drift away from the true window the longer the stream runs,
and the errors are subtle enough that a quick eyeball of a short, all-increasing
sequence looks fine:

  1. The window seems to remember one too MANY samples. After a long stream the
     rolling average reacts more sluggishly than it should -- as if an extra,
     already-expired sample were still being counted. A `Window(3)` fed a long
     run behaves like it is averaging four values, not three.

  2. Before the window has filled up (fewer than `size` samples seen so far) the
     average comes out too LOW. Push two values into a `Window(5)` and the
     reported mean is much smaller than the average of those two values -- it is
     dividing by the full window size instead of by how many samples are
     actually present yet.

  3. `min()` / `max()` go STALE. Once the sample that held the current minimum
     (or maximum) slides out of the window, the reported extreme does not
     recover -- it keeps returning the long-gone value even though that sample
     is no longer in the window. A window of recent, larger values still claims
     its minimum is some tiny number that scrolled off ages ago.

  4. The mean is reported as a truncated whole number. Feeding values whose true
     average is fractional (e.g. 1, 2 -> 1.5) reports `1`, not `1.5`; the
     fractional part is silently dropped, so the rolling average is biased low.

Find and fix the defects so the aggregator honours the contract below exactly.
Keep the public API and behaviour otherwise unchanged.

## Contract

- Package name: `movavg`. The grader imports `movavg.public` (falling back to
  `movavg`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change signatures):
      Window(size)                 # size is a positive int; size <= 0 -> ValueError
      Window.add(x) -> None        # push one value (an int or float)
      Window.mean() -> float       # arithmetic mean of the current window
      Window.min()                 # smallest value in the current window
      Window.max()                 # largest value in the current window
      len(window) -> int           # how many values the window currently holds
- The "current window" is the most recent `min(N, size)` values added, where `N`
  is the total number of `add` calls so far.
- Capacity / eviction:
    * The window holds AT MOST `size` values. Once it is full, each `add` evicts
      the single OLDEST value so the count stays exactly `size` -- it never
      retains `size + 1`. After `k >= size` adds, `len(window) == size` and the
      window is exactly the last `size` values added, in order.
- Partial window (before it fills): while fewer than `size` values have been
  added, the window holds all of them and every statistic is computed over those
  present values only -- e.g. `mean()` divides by the count actually present
  (`len(window)`), NOT by `size`.
- `mean()`:
    * Returns the EXACT arithmetic mean as a float (real division, no rounding
      or truncation): the mean of `1` and `2` is `1.5`, not `1`.
    * Calling `mean()` on an empty window (no values added yet) raises
      `ValueError`.
- `min()` / `max()`:
    * Return the smallest / largest value among the values CURRENTLY in the
      window. When the value that held the extreme is evicted, the result must
      reflect the remaining window -- a stale extreme from an evicted sample is
      never returned.
    * Calling either on an empty window raises `ValueError`.

## I/O example

    >>> w = Window(3)
    >>> w.add(10)
    >>> w.mean()                  # partial window: mean of [10]
    10.0
    >>> w.add(20)
    >>> w.mean()                  # partial: mean of [10, 20], not /3
    15.0
    >>> w.add(30)
    >>> w.mean(), w.min(), w.max()
    (20.0, 10, 30)
    >>> w.add(40)                 # full -> evict 10; window is [20, 30, 40]
    >>> len(w)
    3
    >>> w.mean(), w.min(), w.max()
    (30.0, 20, 40)
    >>> w.add(5); w.add(6)        # window is now [40, 5, 6]
    >>> w.min(), w.max()          # 20/30 evicted; extreme must recover
    (5, 40)
    >>> Window(2).add(1) or Window(2)  # mean of [1, 2] is 1.5, not 1
    >>> u = Window(2); u.add(1); u.add(2); u.mean()
    1.5

- Standard library only.
````

**Graded behaviors:**

- `basic_full_mean` — a filled window reports the mean of exactly the last `size` values
- `basic_min_max` — min/max over a simple filled window are correct
- `len_tracks_count` — len() is the count present: < size while filling, == size once full
- `evicts_to_size` — after many adds the window holds exactly `size`, not `size+1`
- `rolling_mean_window` — the rolling mean uses only the last `size` values (no stale extra)
- `partial_mean_two` — a partial window means over the values present, not over `size`
- `partial_mean_one` — a single value gives mean == that value (divide by 1, not size)
- `partial_then_full` — mean is right both before and after the window first fills
- `min_recovers_after_evict` — min recovers once the sample holding it is evicted
- `max_recovers_after_evict` — max recovers once the sample holding it is evicted
- `extreme_tracks_window` — min/max reflect the live window across a long shifting stream
- `fractional_mean` — a fractional mean is returned exactly (1,2 -> 1.5), not truncated
- `fractional_mean_float` — mean returns a float type, not a truncated int
- `negative_values` — min/max/mean are correct with negative values in the window
- `size_one_window` — a size-1 window always reflects only the most recent value
- `empty_raises` — mean/min/max on an empty window raise (no value yet)


### pctstats · 12 checks

````
You are given an existing Python package, `pctstats`, a small descriptive-statistics
library. It has a bug. Fix it. Do not rewrite the library or change its public API —
make the smallest change that makes the behaviour correct.

## Bug report

High percentiles come out too high. `percentile(values, p)` is supposed to use the
nearest-rank method, but for large `p` (p90, p95, p99) it returns a value one
position too far up the sorted data — and on skewed data p95 comes back as the
maximum, which is clearly wrong.

Repro:

    >>> from pctstats import percentile
    >>> data = [1] * 19 + [100000]      # 20 values: nineteen 1s and a single outlier
    >>> percentile(data, 95)
    100000                              # WRONG — p95 should be a small value (1),
                                        # not the lone outlier / maximum
    >>> percentile([1, 2, 3, 4, 5, 6, 7, 8, 9, 10], 30)
    4                                   # WRONG — nearest-rank p30 of 1..10 is 3

The low end and the exact endpoints look fine; it's the ranks in between that are
off, and the error compounds at the top because the result saturates at the maximum.

`mean`, `minimum`, and `maximum` are correct and should stay correct.

## Contract

- Package name stays `pctstats`; keep the public API:
  `percentile(values, p)`, `mean(values)`, `minimum(values)`, `maximum(values)`.
- `percentile` uses the **nearest-rank** method (no interpolation):
  `rank = ceil(p / 100 * n)`, **1-based**, and the result is the value at that rank
  in the ascending-sorted data. The rank is clamped to `[1, n]`.
- `p` is a percentage in `[0, 100]`; values outside that range are clamped.
  `p = 0` returns the minimum; `p = 100` returns the maximum.
- An empty input sequence raises `ValueError` (already the case — keep it).
- Pure standard library only.
````

**Graded behaviors:**

- `p95_skewed_not_max` — p95 on [1]*19+[100000] is the small value 1, not the 100000 maximum
- `p50_skewed` — p50 on the skewed list is 1
- `p99_skewed` — p99 on the skewed list reaches the outlier 100000
- `p30_ten` — nearest-rank p30 of 1..10 is 3
- `p50_ten` — nearest-rank p50 of 1..10 is 5
- `p90_ten` — nearest-rank p90 of 1..10 is 9
- `p100_is_max` — p100 of 1..10 is the maximum 10
- `p0_is_min` — p0 of 1..10 is the minimum 1
- `single_value` — percentile of a single-element list is that element for any p
- `mean_correct` — mean of 1..10 is 5.5
- `minimum_correct` — minimum of the skewed list is 1
- `maximum_correct` — maximum of the skewed list is 100000


### semvercmp · 18 checks

````
You have inherited a small Python library named `semvercmp`: a Semantic
Versioning 2.0 precedence comparator. The package is already written, imports
cleanly, and the happy path works -- it orders plain `major.minor.patch`
versions correctly and compares two simple word-only pre-releases (like
`1.0.0-alpha` vs `1.0.0-beta`) the right way. It is used to decide which of two
release tags is newer: `compare(a, b)` returns -1 when `a` precedes `b`, 0 when
they have equal precedence, and 1 when `a` follows `b`.

## Bug report

The comparator is wrong at the edges, and the symptoms slipped through because
coarse testing (plain `x.y.z` versions and two word-only pre-releases) looks
fine:

  1. A pre-release is ranked as EQUAL to its final release: `compare("1.0.0-rc.1",
     "1.0.0")` returns 0. A released version is supposed to OUTRANK any of its
     own pre-releases -- `1.0.0-rc.1` must come BEFORE (be less than) `1.0.0`.

  2. Numeric pre-release identifiers are mis-ordered. `1.0.0-2` is reported as
     NEWER than `1.0.0-10`, because the identifiers are compared as text
     ("2" > "10" alphabetically). Numeric identifiers are supposed to compare as
     NUMBERS. Relatedly, a numeric identifier is supposed to rank BELOW an
     alphanumeric one regardless of letters, and that rule is not enforced
     either.

  3. When one pre-release is a leading run of the other, they are called EQUAL:
     `compare("1.0.0-alpha", "1.0.0-alpha.1")` returns 0. The version with MORE
     identifiers is supposed to win when every shared identifier matches, so
     `1.0.0-alpha` must come BEFORE `1.0.0-alpha.1`.

  4. Build metadata changes the answer when it must not. `compare("1.0.0+build.1",
     "1.0.0+build.2")` returns -1, and a build tag can even flip a comparison
     that should be a tie. Everything after a `+` is build metadata and must be
     IGNORED for precedence.

Find and fix the defects so the comparator honours the contract below exactly.
Keep the public API and behaviour otherwise unchanged.

## Contract

- Package name: `semvercmp`. The grader imports `semvercmp.public` (falling back
  to `semvercmp`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change the signature):
      compare(a: str, b: str) -> int      # -1, 0, or 1
- Inputs are valid SemVer 2.0 version strings of the shape
  `major.minor.patch` optionally followed by `-<pre-release>` and/or
  `+<build-metadata>`, in that order. `major`, `minor`, `patch` are
  non-negative integers; pre-release and build are dot-separated identifiers.
- `compare(a, b)` returns -1 if `a` has LOWER precedence than `b`, 1 if HIGHER,
  and 0 if they have EQUAL precedence. Precedence is computed as follows:

    a. Compare `major`, then `minor`, then `patch`, each NUMERICALLY (so
       `1.0.0` < `1.0.10` < `1.1.0` < `2.0.0`). The first differing field
       decides; if all three are equal, move on to the pre-release.

    b. A version WITHOUT a pre-release has HIGHER precedence than the otherwise
       identical version WITH one:
         `1.0.0-anything` < `1.0.0`.
       (Two versions that both lack a pre-release and are equal so far are
       equal.)

    c. When BOTH have a pre-release, compare their dot-separated identifiers
       left to right:
         * an identifier of only digits is NUMERIC; compare two numeric
           identifiers as integers (so `2` < `10`, never as text);
         * a numeric identifier always has LOWER precedence than an
           alphanumeric one (so `1.0.0-1` < `1.0.0-alpha`);
         * two alphanumeric identifiers compare by ASCII / lexical order;
         * the first identifier that differs decides.

    d. If every identifier they SHARE is equal, the pre-release with MORE
       identifiers has the HIGHER precedence:
         `1.0.0-alpha` < `1.0.0-alpha.1`.

    e. Build metadata (everything after the first `+`) is IGNORED entirely for
       precedence. It is NOT part of the comparison and never changes the
       result: `1.0.0+build.1`, `1.0.0+build.2`, and `1.0.0` all have equal
       precedence, and `1.0.0-alpha+x` equals `1.0.0-alpha`.

## I/O example

    >>> compare("1.0.0", "2.0.0")
    -1
    >>> compare("1.0.10", "1.0.2")          # patch compared numerically
    1
    >>> compare("1.0.0-alpha", "1.0.0")     # release outranks its pre-release
    -1
    >>> compare("1.0.0-2", "1.0.0-10")      # numeric identifiers: 2 < 10
    -1
    >>> compare("1.0.0-1", "1.0.0-alpha")   # numeric ranks below alphanumeric
    -1
    >>> compare("1.0.0-alpha", "1.0.0-alpha.1")   # more identifiers wins
    -1
    >>> compare("1.0.0+build.1", "1.0.0+build.2")  # build metadata ignored
    0
    >>> compare("1.0.0", "1.0.0")
    0

- Standard library only.
````

**Graded behaviors:**

- `core_numeric_order` — major/minor/patch compare numerically, most-significant first
- `core_multidigit` — multi-digit fields compare as numbers, not as text (10 > 2)
- `core_equal` — two identical plain versions are equal
- `antisymmetry` — swapping the arguments negates the result for plain versions
- `pre_word_order` — two word-only pre-releases order by ASCII (alpha < beta)
- `pre_below_release` — a pre-release ranks BELOW its own release (1.0.0-alpha < 1.0.0)
- `release_above_pre` — the release ranks ABOVE the pre-release (mirror, 1.0.0 > 1.0.0-alpha)
- `pre_below_release_rc` — a late-stage pre-release still ranks below the release (rc.1 < 1.0.0)
- `numeric_ident_value` — numeric identifiers compare by value, not text (2 < 10)
- `numeric_ident_value_big` — multi-digit numeric identifiers compare by value (9 < 100)
- `numeric_below_alpha` — a numeric identifier ranks below an alphanumeric one (1 < alpha)
- `alpha_above_numeric` — an alphanumeric identifier ranks above a numeric one (mirror)
- `more_identifiers_wins` — with shared identifiers equal, more identifiers wins (alpha < alpha.1)
- `more_identifiers_mirror` — the longer pre-release ranks higher (mirror of the tiebreak)
- `build_ignored_equal` — build metadata is ignored: +build.1 and +build.2 are equal
- `build_ignored_vs_plain` — build metadata is ignored vs a plain version (1.0.0+x == 1.0.0)
- `build_ignored_with_pre` — build metadata is ignored alongside a pre-release (-a+x == -a)
- `spec_ordering_chain` — the full SemVer spec precedence chain orders strictly ascending


### textwidth · 21 checks

````
You have inherited a small Python library named `textwidth`: a greedy word-wrap
utility. The package is already written, imports cleanly, and the happy path
works -- given a short sentence of ordinary single-spaced words and a generous
width, it packs whole words onto lines and returns the wrapped lines. It is used
to lay out plain-text output for a fixed-width terminal: every block of text is
passed through `wrap(text, width)` before printing.

## Bug report

Under real input the wrapper misbehaves at the edges, and the symptoms are easy
to miss because tidy, single-spaced sentences at a roomy width look fine:

  1. Lines come out NARROWER than they should. A word that would exactly fill
     the remaining room on a line gets bumped to the next line instead of
     packed onto the current one, so the output is ragged and uses more lines
     than necessary. Packing should fill each line right up to `width`.

  2. A word LONGER than `width` (a URL, a long identifier) blows the layout: it
     is emitted on a line all by itself and overflows past `width`, so the
     "fixed-width" output is no longer fixed-width. Such a word should be
     hard-broken across however many full-width lines it takes.

  3. Messy whitespace corrupts the output. Input with runs of multiple spaces,
     or tabs, or embedded newlines produces lines with stray blank words,
     doubled spaces, or wrong wrapping -- as if every space were its own word
     boundary. Any run of whitespace should collapse to a single boundary and
     only the actual words should survive.

  4. There is a spurious trailing empty line. Empty or all-whitespace input
     returns a list containing one empty string instead of an empty list, and
     text with trailing whitespace leaves a dangling `""` at the end of the
     result.

Find and fix the defects so the wrapper honours the contract below exactly.
Keep the public API and behaviour otherwise unchanged.

## Contract

- Package name: `textwidth`. The grader imports `textwidth.public` (falling back
  to `textwidth`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change the signature):
      wrap(text: str, width: int) -> list[str]
- Greedy packing: words are placed left-to-right; each word goes on the current
  line if it still fits, otherwise it starts a new line. Words on a line are
  joined by exactly ONE space.
- Width is INCLUSIVE: a line may be exactly `width` characters long. A word that
  brings the current line's length to exactly `width` (counting the single
  joining space) MUST stay on that line -- it fits. Only a word that would push
  the line PAST `width` starts a new line.
- Whitespace: the input is tokenised on ANY run of whitespace (spaces, tabs,
  newlines, multiple spaces all count as a single boundary). Leading and
  trailing whitespace is dropped. Empty strings are never emitted as words.
- Overlong word: a single word whose length is greater than `width` cannot fit
  on any line. It is HARD-BROKEN into consecutive pieces of exactly `width`
  characters (the final piece may be shorter). No emitted line ever exceeds
  `width` characters when `width >= 1`. The break happens at the character
  level, in order, with no characters lost or reordered. A line already in
  progress is flushed before the long word is broken; the long word's final
  short tail may then be joined by further words that still fit.
- No trailing empty line: the result contains no empty strings. Empty input,
  or input that is only whitespace, returns `[]` (an empty list, NOT `[""]`).
- `width <= 0`: there is no positive line width to pack into, so `wrap` returns
  `[]` for any input.

## I/O example

    >>> wrap("the quick brown fox", 9)
    ['the quick', 'brown fox']      # 'the quick' is exactly 9 -> it fits
    >>> wrap("aa bb cc", 5)
    ['aa bb', 'cc']                 # 'aa bb' is exactly 5 -> packed, not split
    >>> wrap("  lots   of\twhite\nspace  ", 11)
    ['lots of', 'white space']      # whitespace collapses; no blank words
    >>> wrap("supercalifragilistic", 7)
    ['superca', 'lifragi', 'listic']   # overlong word hard-broken at width 7
    >>> wrap("", 5)
    []                              # empty input -> empty list, no trailing ''
    >>> wrap("hello world", 0)
    []                              # width <= 0 -> empty list

- Standard library only.
````

**Graded behaviors:**

- `basic_wrap` — an ordinary sentence wraps greedily into the expected lines
- `single_word` — a lone word shorter than width is returned on one line
- `all_fit_one_line` — words that all fit return a single line, no extra splits
- `exact_fit_packs` — a word landing the line exactly at width stays on the line
- `exact_fit_two_words` — two words whose joined length equals width pack together
- `no_line_exceeds_width` — no emitted line is ever wider than width (ordinary text)
- `greedy_fills` — packing is greedy -- each line is filled as full as it can be
- `collapse_multi_space` — runs of multiple spaces collapse to a single boundary
- `collapse_tabs_newlines` — tabs and newlines are treated as whitespace boundaries
- `leading_trailing_ws` — leading and trailing whitespace is dropped, no blank words
- `no_empty_words` — the result never contains an empty string
- `empty_input` — empty input returns [] (not ['']), no trailing empty line
- `whitespace_only_input` — all-whitespace input returns [] (no blank line)
- `trailing_ws_no_dangle` — trailing whitespace does not leave a dangling '' line
- `overlong_hard_break` — a word longer than width is hard-broken into width pieces
- `overlong_exact_multiple` — a word an exact multiple of width breaks with no empty tail
- `overlong_no_overflow` — hard-broken pieces never exceed width; chars are preserved
- `overlong_flushes_current` — an overlong word flushes the in-progress line first
- `overlong_tail_joins` — the short tail of a broken word can take following words
- `width_zero` — width == 0 returns [] for any input
- `width_negative` — a negative width returns [] for any input


### tokenbucket · 16 checks

````
You have inherited a small Python library named `tokenbucket`: a continuous
token-bucket rate limiter. The package is already written, imports cleanly, and
the happy path works -- a fresh bucket starts full, hands out tokens until it is
empty, denies once empty, and refills as time passes. It is used to throttle an
API: every request calls `allow(now, cost)` and is let through only when it
returns True.

## Bug report

Under real traffic the limiter misbehaves at the edges, and the symptoms are
hard to pin down because coarse, whole-second testing looks fine:

  1. After a quiet period (no traffic for a while) the bucket lets through a
     huge BURST -- far more requests than `capacity` -- as if it had saved up
     unlimited credit while idle. It is supposed to hold at most `capacity`
     tokens no matter how long it sat idle.

  2. Under a steady stream of rapid, closely-spaced requests the effective
     throughput is WAY below `refill_per_sec` -- the bucket behaves as if it
     barely refills at all, starving callers, even though over the same wall
     time it should have accrued the full proportional amount. The refill seems
     to "forget" the time between very frequent calls.

  3. A request that is DENIED still seems to cost the caller something: a burst
     of large-`cost` requests that are all rejected leaves the bucket emptier
     than it was before, so legitimate small requests that should have fit are
     then refused too.

  4. A rare backwards jump in the supplied clock (an upstream time source
     glitch) wedges the limiter -- afterwards it under-grants for a while.

Find and fix the defects so the limiter honours the contract below exactly.
Keep the public API and behaviour otherwise unchanged.

## Contract

- Package name: `tokenbucket`. The grader imports `tokenbucket.public` (falling
  back to `tokenbucket`); keep both import paths working.
- Public API, UNCHANGED (do not rename anything or change signatures):
      TokenBucket(capacity, refill_per_sec)
      TokenBucket.allow(now: float, cost: int = 1) -> bool
      TokenBucket.tokens            # read-only current token count (a float)
- The bucket starts FULL: it holds exactly `capacity` tokens before the first
  call.
- `allow(now, cost)` does, in order:
    a. REFILL for the time elapsed since the previous call, then
    b. try to CONSUME `cost` tokens.
- Refill (continuous, fractional, capped AFTER):
    * Let `elapsed = now - last`, where `last` is the `now` of the previous
      call (on the very first call there is nothing to refill).
    * Add `elapsed * refill_per_sec` tokens -- the EXACT real-valued amount, with
      NO rounding or truncation. The bucket holds a fractional token count.
    * THEN clamp the total to at most `capacity`. (Refill first, cap second:
      tokens can momentarily compute above capacity during refill but must be
      clamped down to exactly `capacity`, never left above it.)
    * `elapsed` is clamped to be non-negative: if `now < last` (clock went
      backwards), refill nothing and do NOT move `last` backwards. The next
      forward call refills from the real, latest `last`.
- Consume:
    * If the bucket holds at least `cost` tokens (`tokens >= cost`, so a request
      for exactly the available amount is allowed), subtract `cost` and return
      True.
    * Otherwise return False and consume NOTHING -- the token count is left
      exactly as it was. A denial never drains the bucket.
- `cost` is a positive integer (default 1); `cost > 1` consumes that many tokens
  atomically (all-or-nothing).

## I/O example

    >>> b = TokenBucket(capacity=10, refill_per_sec=2.0)
    >>> b.allow(now=0.0, cost=10)     # spend the full bucket at t=0
    True
    >>> b.allow(now=0.0)              # empty now
    False
    >>> b.tokens
    0.0
    >>> b.allow(now=1.5, cost=3)      # 1.5s * 2/s = +3.0 tokens, exactly enough
    True
    >>> b.allow(now=1.5)              # drained again
    False
    >>> b.allow(now=10000.0, cost=1)  # long idle: refill caps AT capacity (10)
    True
    >>> b.tokens                      # 10 - 1, never above capacity
    9.0

- Standard library only.
````

**Graded behaviors:**

- `starts_full_allows` — a fresh full bucket allows up to capacity immediately
- `empties_then_denies` — draining the bucket then asking again is denied
- `coarse_refill` — a whole-second wait refills the expected whole tokens
- `denied_consumes_nothing` — a DENIED request leaves the token count unchanged
- `denied_then_exact` — after a denial the still-present tokens are spendable
- `cost_gt_one_allow` — cost>1 is allowed and consumes exactly cost tokens
- `cost_gt_one_boundary` — cost equal to available tokens is allowed (>=, not >)
- `cost_gt_one_over` — cost one above available is denied without draining
- `cap_after_refill` — a long idle gap caps the bucket AT capacity, not above
- `cap_exact_no_overflow` — refill that exactly reaches capacity does not overflow
- `fractional_refill_accrues` — many tiny sub-token steps accrue real fractional credit
- `fractional_partial` — a fractional refill grants the proportional token amount
- `monotonic_clamp_no_gain` — a backwards `now` neither adds nor removes tokens
- `monotonic_clamp_recovers` — after a backwards blip, forward time refills from the real last
- `burst_then_wait_recovery` — a burst that empties the bucket recovers after waiting
- `sustained_rate` — over a long run the allow-rate tracks refill_per_sec


### ttlcache · 16 checks

````
You have inherited a small Python library named `ttlcache`: an in-memory cache
where each entry carries a time-to-live (TTL). The package is already written,
imports cleanly, and most of it works. It ships with an injectable clock so its
behaviour is fully deterministic — you pass `Cache(clock=...)` a zero-argument
callable returning the current time as a number.

## Bug report

Entries are being served one tick PAST their TTL — they expire a moment late.

Reproduction:

    ticks = [0]
    c = Cache(clock=lambda: ticks[0])
    c.set("a", 1, ttl=10)      # set at t=0 with a 10-unit TTL

    ticks[0] = 9
    c.get("a")                 # -> 1   (correct: still fresh)

    ticks[0] = 10
    c.get("a")                 # -> 1   (WRONG: the 10-unit TTL has fully
                               #         elapsed, so this should be a miss)

    ticks[0] = 11
    c.get("a")                 # -> None (eventually expires, one tick too late)

In other words: a value set with `ttl=10` is still being returned at the exact
moment it should expire. It should be considered expired — a miss — as soon as
the elapsed time since `set` reaches the TTL, not strictly after it. The same
late-expiry shows up anywhere freshness is judged (membership tests, remaining
TTL, length, purge), because they all share the same notion of "expired".

A TTL of 0 (or negative) should mean the entry is already expired and never
served.

## Contract

- Package name: `ttlcache`. The grader imports `ttlcache.public` (falling back
  to `ttlcache`); keep both import paths working.
- Keep the PUBLIC API UNCHANGED: `Cache(clock=None)`, `Cache.set(key, value,
  ttl)`, `Cache.get(key, default=None)`, plus the existing helpers
  (`contains` / `in`, `delete`, `ttl_remaining`, `purge`, `clear`, `len`,
  `stats`). Do not rename anything or change signatures.
- Fix the expiry boundary so an entry expires exactly when its TTL elapses
  (i.e. at and after `set_time + ttl`), not one tick later. Hits/misses and the
  expiration counter must reflect the corrected boundary.
- Standard library only.
````

**Graded behaviors:**

- `set_get_basic` — a freshly set value is returned before its TTL elapses
- `boundary_just_before` — value is a HIT one tick before the TTL elapses
- `boundary_exact` — value is a MISS at the exact instant the TTL elapses
- `boundary_after` — value is a MISS after the TTL has elapsed
- `ttl_zero_expired` — ttl=0 entry is already expired (never served)
- `ttl_negative_expired` — negative ttl entry is already expired
- `miss_unknown_key` — an unknown key is a miss / returns the default
- `default_returned` — the supplied default is returned on a miss
- `stats_hits_misses` — hits/misses counters reflect the corrected boundary
- `stats_expiration_counted` — an expired-on-access entry increments expirations
- `contains_boundary` — membership (`in`) expires exactly at the TTL boundary
- `ttl_remaining_boundary` — ttl_remaining is None at and after the TTL boundary
- `len_counts_live_only` — len() counts only live (non-expired) entries
- `purge_removes_expired` — purge() evicts entries that are at/after their TTL
- `overwrite_resets_ttl` — re-setting a key resets its TTL from the new set time
- `delete_removes` — delete() removes a present key and reports it


## feature-add (15)

_Add a missing capability to working code without breaking it._


### cachetags · 19 checks

````
You have inherited a small Python library named `cachetags`: an in-memory cache.
The package is already written, imports cleanly, and its core operations work:
`set(key, value, now)`, `get(key, now, default=None)`, and `delete(key)`.

Time is INJECTED, never read from a real clock: every read and write takes
`now` (a number — the caller's current time) as an explicit argument. The base
cache does not actually use `now` yet (entries never expire), but it is already
part of the signature. This keeps the cache fully deterministic and testable.

## Task

Add two features on top of the existing cache, WITHOUT breaking plain
`get` / `set`:

(a) **Per-entry TTL.** `set(key, value, now, ttl=...)` makes the entry expire
    `ttl` time-units after it was set. A `get` at or after the expiry instant is
    a MISS.

(b) **Tag-based invalidation.** `set(key, value, now, tags=[...])` attaches tags
    to the entry. `invalidate_tag(tag, now)` drops every entry carrying that
    tag.

## Semantics (read carefully — this is the whole task)

- TTL is RELATIVE to the set time and resolved against the injected `now`: an
  entry set with `ttl=T` at time `t0` expires at `t0 + T`. The boundary is
  HALF-OPEN — the entry is a HIT for every `now` in the interval `[t0, t0+T)`
  and a MISS at and after `t0 + T`. So `ttl=10` set at `now=0` is a hit at
  `now=9` and a miss at `now=10`.

- `ttl=None` (the default) means the entry NEVER expires — plain `set` with no
  `ttl` keeps its old behavior.

- A `ttl` of 0 (or negative) means the entry is already expired the instant it
  is set: it is never served.

- `invalidate_tag(tag, now)` removes every entry that currently carries `tag`
  and returns the COUNT of entries it dropped.

- SUBTLE — an EXPIRED entry must also lose its tag membership. Once an entry's
  TTL has elapsed it is gone in every sense: it must NOT be returned by `get`,
  and `invalidate_tag` on one of its (former) tags must NOT count it (it is a
  no-op for that entry — there is nothing live to invalidate). An expired entry
  must never resurface.

- SUBTLE — re-`set`ting an existing key REPLACES its tags wholesale. The key
  loses its OLD tags and carries only the tags given on the latest `set` (or no
  tags, if `tags` is omitted). A later `invalidate_tag` with an OLD tag must not
  touch the re-set entry.

- Plain `get` / `set` / `delete` with no `ttl` and no `tags` must behave exactly
  as they do today.

## Example

    c = Cache()

    c.set("a", 1, now=0, ttl=10, tags=["red"])
    c.get("a", now=9)        # -> 1     (still within TTL)
    c.get("a", now=10)       # -> None  (TTL elapsed: a miss)

    c.set("b", 2, now=0, tags=["red", "blue"])
    c.invalidate_tag("red", now=0)   # -> 1  (drops "b")
    c.get("b", now=0)        # -> None

    # expired entry loses its tag membership:
    c.set("c", 3, now=0, ttl=5, tags=["green"])
    c.invalidate_tag("green", now=10)  # -> 0  ("c" already expired; nothing to drop)
    c.get("c", now=10)       # -> None

    # re-set replaces tags:
    c.set("d", 4, now=0, tags=["old"])
    c.set("d", 5, now=0, tags=["new"])   # "d" no longer carries "old"
    c.invalidate_tag("old", now=0)   # -> 0  (does not touch "d")
    c.get("d", now=0)        # -> 5

## Contract

- Package name: `cachetags`. The grader imports `cachetags.public` (falling back
  to `cachetags`); keep both import paths working.
- Public class `Cache` with methods:
    * `set(key, value, now, ttl=None, tags=None)` -> None
    * `get(key, now, default=None)` -> value or default
    * `delete(key)` -> bool (True iff the key was present)
    * `invalidate_tag(tag, now)` -> int (number of live entries dropped)
- `now`, `ttl`, and tag values are plain numbers / hashables; tags is any
  iterable of hashables. Time is whatever numeric type the caller passes as
  `now` — do not read a real clock.
- Standard library only. No persistence, no threading requirement.
````

**Graded behaviors:**

- `set_get_basic` — a value set with no ttl is returned later
- `ttl_hit_before` — an entry is a HIT one tick before its TTL elapses
- `ttl_miss_exact` — an entry is a MISS at the exact instant its TTL elapses
- `ttl_miss_after` — an entry is a MISS after its TTL has elapsed
- `ttl_none_never_expires` — ttl=None entry never expires
- `ttl_zero_already_expired` — ttl=0 entry is already expired the instant it is set
- `ttl_negative_already_expired` — negative-ttl entry is already expired
- `invalidate_single_tag` — invalidate_tag drops a live entry carrying that tag
- `invalidate_returns_count` — invalidate_tag returns the number of live entries dropped
- `invalidate_other_tag_untouched` — invalidate_tag leaves entries without that tag alone
- `invalidate_multiple_with_tag` — invalidate_tag drops every live entry carrying the tag
- `expired_loses_tag_membership` — invalidate_tag of a former tag of an EXPIRED entry counts 0
- `expired_not_resurfaced_after_invalidate` — an expired entry never resurfaces via the tag index
- `reset_replaces_tags` — re-set replaces tags: an OLD tag no longer invalidates the entry
- `reset_keeps_new_tag` — re-set keeps the NEW tag: it still invalidates the entry
- `invalidate_unknown_tag_zero` — invalidate_tag on an unknown tag returns 0 and changes nothing
- `regression_get_set_overwrite` — plain get/set/overwrite still work with no ttl/tags
- `regression_delete_return` — delete reports presence (True/False) with no ttl/tags
- `regression_get_default` — get returns the supplied default on a miss


### condschema · 24 checks

````
You have inherited a small Python library named `condschema`: a data validator.
The package is already written, imports cleanly, and works for the cases it
supports. Its single public entry point is:

    def validate(data, schema) -> list[error]: ...

It walks `data` against `schema` and returns a LIST OF ERROR DICTS. An EMPTY
list means the data is valid; a non-empty list means it failed one or more
rules.

## What works today

The validator handles a FLAT schema: a mapping of `field name -> field spec`.
Each spec may declare:

  - `type`     — one of "string" / "number" / "integer" / "bool" / "object" /
                 "list". (Note: a bool is NOT a number/integer, and a float is
                 NOT an integer; "number" accepts ints and floats.)
  - `required` — if True, the field must be present.

Each error is a dict::

    {"path": "<dotted path>", "code": "<rule that failed>", "message": "..."}

`path` is a dotted location into the data; a top-level field's path is just its
name. `code` is one of "required" (a required field is absent) or "type" (a
present field has the wrong type). The shipped validator reports ALL errors it
finds, in a STABLE order: schema-declared field order.

Examples of current behavior::

    validate({"name": "Ada"}, {"name": {"type": "string", "required": True}})
        -> []                                            # valid
    validate({}, {"name": {"type": "string", "required": True}})
        -> [{"path": "name", "code": "required", ...}]   # missing
    validate({"age": "x"}, {"age": {"type": "integer"}})
        -> [{"path": "age", "code": "type", ...}]        # wrong type

## The capability to ADD

Extend the validator with NESTED schemas and CONDITIONAL requirements. Flat
validation must keep working exactly as before.

### 1. Nested objects

A field spec of `type` "object" may carry a `fields` key: a nested schema
(same shape — field name -> spec). When the field is present AND is a dict,
validate it RECURSIVELY against `fields`. Errors from inside carry a DOTTED
path built from the enclosing field name and the inner path::

    schema = {"address": {"type": "object", "fields": {
                  "zip": {"type": "string", "required": True}}}}
    validate({"address": {}}, schema)
        -> [{"path": "address.zip", "code": "required", ...}]

If the field is present but is NOT a dict, report a single "type" error at the
field's own path and do NOT recurse into it.

### 2. Lists of items

A field spec of `type` "list" may carry an `items` key: a single field spec
applied to EVERY element of the list. When the field is present AND is a list,
validate each element against `items`. An element's path is the field name,
then the element's INDEX, then any inner path::

    schema = {"items": {"type": "list", "items": {
                  "type": "object", "fields": {
                      "sku": {"type": "string", "required": True}}}}}
    validate({"items": [{"sku": "A1"}, {}, {"sku": 5}]}, schema)
        -> [{"path": "items.1.sku", "code": "required", ...},
            {"path": "items.2.sku", "code": "type", ...}]

Elements are validated in index order. If the field is present but is NOT a
list, report a single "type" error at the field's own path and do NOT recurse.

### 3. Conditional requirements

A field spec may carry `requiredIf`: a `[sibling_field, value]` pair. The field
is required ONLY when, in the SAME object, the named sibling field is present
AND its value EQUALS `value`. Otherwise the field is optional.

    schema = {"country": {"type": "string"},
              "state":   {"type": "string", "requiredIf": ["country", "US"]}}
    validate({"country": "US"}, schema)
        -> [{"path": "state", "code": "required", ...}]   # state needed
    validate({"country": "CA"}, schema)
        -> []                                             # state optional
    validate({}, schema)
        -> []                                             # sibling absent -> optional

Subtleties (these are the whole task):

  - `requiredIf` is satisfied only by an EXACT match of the sibling's value.
    Do NOT conflate booleans with numbers: a sibling holding `True` does not
    equal `1`, and `1` does not equal `True`.
  - The sibling is looked up in the SAME object that owns the conditional
    field (the nested object for a nested spec), never globally.
  - A field that is required (by `required: True` OR by a satisfied
    `requiredIf`) and is ABSENT yields exactly ONE "required" error and no
    "type" error.
  - A field that is PRESENT is type-checked regardless of how its requirement
    was decided (plain or conditional). Presence and type are separate rules.
  - Report ALL errors, never first-only. The order is a stable pre-order walk:
    fields in schema-declared order; within a list, elements by ascending
    index; recurse into a field's nested errors before moving to the next
    sibling field.

## Contract

- Package name: `condschema`. The grader imports `condschema.public` (falling
  back to `condschema`); keep both import paths working.
- Public function `validate(data, schema) -> list[dict]`. Empty list == valid.
  Each error dict has at least the keys `path` (dotted string) and `code`
  (one of "required" / "type").
- A field present in `data` but absent from `schema` is unconstrained (ignored).
- A `required`/`requiredIf` field that is absent is reported once as "required"
  and is not type-checked.
- Standard library only. No third-party validation libraries.
````

**Graded behaviors:**

- `nested_object_dotted_path` — a missing field in a nested object reports a DOTTED path
- `nested_object_valid` — a satisfied nested object yields no errors
- `nested_object_type_no_recurse` — a non-dict where an object is expected: one type error, no recursion
- `nested_object_inner_type` — a wrong-typed value inside a nested object reports at the dotted path
- `deep_nesting_path` — two levels of object nesting build the full dotted path
- `list_item_index_path` — a bad element reports field.<index>.<inner> as its path
- `list_all_valid` — a list whose every element fits yields no errors
- `list_type_no_recurse` — a non-list where a list is expected: one type error, no recursion
- `list_scalar_items` — an items spec of a scalar type checks each element by index
- `list_order_by_index` — multiple bad elements are reported in ascending index order
- `requiredif_trigger_present` — requiredIf fires when the sibling equals the trigger value
- `requiredif_other_value` — requiredIf does NOT fire when the sibling has a different value
- `requiredif_sibling_absent` — requiredIf does NOT fire when the sibling is absent
- `requiredif_present_ok` — a satisfied requiredIf with the field present is valid
- `requiredif_bool_not_number` — requiredIf does not conflate True with 1 (exact match)
- `requiredif_present_is_type_checked` — a present conditional field is still type-checked
- `required_absent_no_type_error` — an absent required field gives ONE required error, no type error
- `all_errors_reported` — every violation is reported, not just the first
- `stable_pre_order` — errors come in schema order then ascending list index
- `regression_flat_valid` — a valid flat instance yields an empty list
- `regression_flat_required_missing` — a missing flat required field errors at its bare name
- `regression_flat_type_mismatch` — a wrong-typed flat field errors with code 'type'
- `regression_number_accepts_int` — type 'number' accepts an integer; 'integer' rejects a float
- `regression_bool_not_integer` — a bool does not satisfy type 'integer'


### csvgroupby · 10 checks

````
You are handed an existing, WORKING Python 3.11+ package named `csvgroupby`. It is
already in your working directory. Add ONE new capability described below WITHOUT
regressing what already works. Use only the Python standard library.

## What it does today

`csvgroupby.public.query(rows, sql)` runs a small SQL-subset query against `rows`
(a list of dicts, one per CSV record — every value is a string as read from the
file) and returns a list of row dicts.

It already supports:

    SELECT <cols> FROM <table> [WHERE <column> <op> <value>]

  - `<cols>` is `*` or a comma-separated list of column names.
  - `<op>` is one of: `>=`  `<=`  `>`  `<`  `=`  `!=`
  - WHERE compares with NUMERIC INFERENCE: a cell or literal that looks like a
    number is compared numerically, otherwise as a string. So
    `WHERE age >= 18` keeps rows whose `age` cell is the number 18 or greater,
    even though the cell is stored as a string.
  - The `<table>` name parses but is ignored (`rows` is the data source).

Example (already works):

    rows = [
        {"city": "NYC", "age": "30"},
        {"city": "LA",  "age": "17"},
        {"city": "NYC", "age": "22"},
    ]
    query(rows, "SELECT city FROM t WHERE age >= 18")
    # -> [{"city": "NYC"}, {"city": "NYC"}]

## The capability to ADD: GROUP BY <col> with COUNT(*)

Add support for a trailing `GROUP BY <column>` clause together with a `COUNT(*)`
aggregate in the select list:

    SELECT <col>, COUNT(*) FROM <table> [WHERE ...] GROUP BY <col>

Semantics:

  - GROUP BY produces ONE output row per DISTINCT value of the grouped column,
    in first-seen order.
  - `COUNT(*)` in the select list is the number of rows in that group, and it
    appears in each output row under the key `COUNT(*)`.
  - GROUP BY composes with WHERE: filter first, then group over the surviving
    rows (so a group's count reflects only rows that passed WHERE).

Examples:

    rows = [
        {"city": "NYC", "age": "30"},
        {"city": "LA",  "age": "17"},
        {"city": "NYC", "age": "22"},
        {"city": "LA",  "age": "40"},
        {"city": "NYC", "age": "12"},
    ]

    query(rows, "SELECT city, COUNT(*) FROM t GROUP BY city")
    # -> [
    #      {"city": "NYC", "COUNT(*)": 3},
    #      {"city": "LA",  "COUNT(*)": 2},
    #    ]

    query(rows, "SELECT city, COUNT(*) FROM t WHERE age >= 18 GROUP BY city")
    # -> [
    #      {"city": "NYC", "COUNT(*)": 2},   # ages 30, 22
    #      {"city": "LA",  "COUNT(*)": 1},   # age 40
    #    ]

## Contract

  - Package name stays `csvgroupby`; keep the public API
    `query(rows, sql) -> list[dict]` exposed from `csvgroupby.public`
    (and re-exported from `csvgroupby`).
  - DO NOT change the existing behavior or return shape: the result is always a
    list of row dicts; plain `SELECT <cols> ... [WHERE ...]` (no GROUP BY) keeps
    returning exactly the same projected/filtered rows it does today, and WHERE
    keeps its numeric-inference comparison.
  - The new aggregate value MUST appear under the key `COUNT(*)` (the literal
    string `COUNT(*)`).
  - Each GROUP BY output row identifies its group by the grouped column's value
    (e.g. `{"city": "NYC", "COUNT(*)": 3}`); one row per distinct group value,
    in first-seen order; count = number of WHERE-surviving rows in the group.
  - Standard library only; keep it a self-contained package.

Do not change the package name or the `query` signature. Add `GROUP BY <col>`
with `COUNT(*)` so the examples above work, and keep the existing SELECT/WHERE
behavior intact.
````

**Graded behaviors:**

- `groupby_basic_counts` — got={('str', 'NYC'): 3, ('str', 'LA'): 2} want={('str', 'NYC'): 3, ('str', 'LA'): 2}
- `groupby_one_row_per_group` — rows=2 distinct=2
- `groupby_honors_where` — got={('str', 'NYC'): 2, ('str', 'LA'): 1} want={('str', 'NYC'): 2, ('str', 'LA'): 1}
- `groupby_numeric_key` — got={('num', 1.0): 3, ('num', 2.0): 1} want={('num', 1.0): 3, ('num', 2.0): 1}
- `groupby_single_group` — count=3
- `select_cols_projection` — result=[{'city': 'NYC'}, {'city': 'LA'}, {'city': 'NYC'}, {'city': 'LA'}, {'city': 'NYC'}]
- `select_star_passthrough` — result=[{'city': 'NYC', 'age': '30'}, {'city': 'LA', 'age': '17'}, {'city': 'NYC', 'age': '22'}, {'city': 'LA', 'age': '40'}, {'city': 'NYC', 'age': '12'}]
- `where_numeric_ge` — ages=[22, 30, 40] want=[22, 30, 40]
- `where_not_equal_string` — cities=['NYC', 'NYC', 'NYC'] want=['NYC', 'NYC', 'NYC']
- `where_lt_numeric` — ages=[12, 17] want=[12, 17]


### cursorpage · 15 checks

````
You have inherited a small Python library named `cursorpage`: a paginator over
a list of records sorted by a key. The package is already written, imports
cleanly, and OFFSET pagination works:

    Paginator(records, key="score")   # records: dicts with int "id" + a sort key
    p.page(n, size)                   # the n-th page (0-based) of `size` records

Records are kept in a fully-deterministic order: by `key` ascending, with ties
broken by `"id"` ascending (so the order is stable even when several records
share a sort-key value).

## Task

Add CURSOR (keyset) pagination alongside the existing offset paging:

    p.page_after(cursor, size) -> {"items": [...], "next_cursor": <token-or-None>}

A cursor is an OPAQUE token that encodes the position of the last record handed
out so far. Walking the data by feeding each call's `next_cursor` into the next
call must visit every record EXACTLY ONCE — no duplicates, no gaps — and then
stop. Do NOT change or break offset `page(n, size)`.

## Semantics (read carefully — this is the whole task)

- The cursor encodes the (sort-key value, id) of the LAST record returned.
  Resumption is "strictly AFTER that position" in the sorted order. Because the
  order is (key, then id), this is what makes TIES work: two records with the
  same sort key are still separated by id, so a cursor landing in the middle of
  a run of tied keys resumes at exactly the next record — never re-emitting the
  ones already seen, never skipping the ones not yet seen.

- The token is opaque: callers must not have to parse or construct it. Encode it
  however you like (it must round-trip the (key, id) boundary).

- A `None` cursor — or any invalid / malformed / unrecognized token — starts
  from the BEGINNING (the first record). An invalid cursor must NOT raise.

- `next_cursor` is `None` EXACTLY when the page just returned is the last one
  (i.e. there are no more records after it). When more records remain,
  `next_cursor` is a token that resumes strictly after the last item of this
  page. An empty result (nothing after the cursor) therefore has
  `next_cursor=None`.

- `size` must be positive (raise `ValueError` otherwise), matching `page`.

## Example

    rows = [{"id": 3, "score": 5}, {"id": 1, "score": 5}, {"id": 2, "score": 9}]
    p = Paginator(rows, key="score")
    # sorted order is by (score, id): id 1 (s5), id 3 (s5), id 2 (s9)

    out = p.page_after(None, 2)
    [r["id"] for r in out["items"]]      # [1, 3]   -- note the tie on score 5
    out["next_cursor"] is None           # False    -- more remain

    out2 = p.page_after(out["next_cursor"], 2)
    [r["id"] for r in out2["items"]]     # [2]
    out2["next_cursor"] is None          # True     -- last page

Walking from None with next_cursor must yield ids [1, 3, 2] — every record once.

A cursor whose key value lands in the MIDDLE of a tie must not re-emit or skip:
resuming after (score=5, id=1) yields id 3 next (the other score-5 record),
then id 2 — never id 1 again, and never jumping past id 3.

## Contract

- Package name: `cursorpage`. The grader imports `cursorpage.public` (falling
  back to `cursorpage`); keep both import paths working.
- Public class `Paginator(records, key="id")` with:
    * `page(n, size) -> list` — UNCHANGED offset pagination.
    * `page_after(cursor, size) -> {"items": list, "next_cursor": str|None}` —
      cursor pagination as specified above. `cursor` may be `None`.
- The cursor returned in `next_cursor` is opaque (any string encoding); feeding
  it back into `page_after` resumes strictly after the last returned record.
- Standard library only. No persistence, no threading requirement.
````

**Graded behaviors:**

- `first_page_no_cursor` — page_after(None, size) returns the first `size` records in sorted order
- `first_page_next_cursor_present` — a non-final first page yields a non-None next_cursor
- `walk_all_once_no_ties` — walking via next_cursor visits every record exactly once (no ties)
- `last_page_cursor_none` — the final page returns next_cursor=None
- `walk_all_once_with_ties` — walking visits every record exactly once when sort keys tie
- `resume_mid_tie_no_dup_no_gap` — a cursor inside a run of tied keys resumes strictly after it
- `opaque_cursor_round_trips` — feeding back next_cursor resumes at the right place (cursor is opaque)
- `size_one_walk_full` — size=1 walk yields the full sorted order one at a time
- `size_larger_than_data` — size >= len returns all records with next_cursor=None
- `empty_paginator` — page_after(None, size) on empty data returns [] and next_cursor=None
- `none_cursor_from_start` — a None cursor starts from the beginning
- `invalid_cursor_from_start` — a garbage/invalid cursor starts from the beginning without raising
- `size_nonpositive_raises` — size <= 0 raises ValueError in page_after
- `regression_offset_paging` — offset page(n, size) still returns the correct sorted pages
- `regression_offset_out_of_range` — offset page() past the end returns an empty list


### eventbus · 19 checks

````
You have inherited a small Python library named `eventbus`: an in-memory
publish/subscribe message bus. The package is already written, imports cleanly,
and its core operations work: `subscribe(topic, fn)` registers a callback for an
EXACT topic, and `publish(topic, data)` invokes every callback subscribed to
that exact topic, passing `(topic, data)`.

Topics are dot-delimited strings of one or more non-empty segments, e.g.
`"order"`, `"order.created"`, `"sensor.kitchen.temp"`.

It ships with a FIRST attempt at wildcard subscriptions that does not actually
work: a subscription whose topic contains a `*` or `#` is stored, but matching
still compares topics for plain string equality, so wildcards never fire.

## Task

Make wildcard subscriptions work. A subscription topic may contain wildcard
segments; a published topic is always concrete (a publisher never publishes to a
wildcard).

## Semantics (read carefully — this is the whole task)

- `*` is a SINGLE-segment wildcard: it matches exactly one segment in that
  position. `"a.*"` matches `"a.b"` but NOT `"a.b.c"` (too many segments) and
  NOT `"a"` (too few). `"*"` matches any single-segment topic like `"a"`, but
  not `"a.b"`. A `*` may appear in any position: `"*.created"` matches
  `"order.created"` and `"user.created"`, not `"order.created.late"`.

- `#` is a MULTI-segment wildcard: it matches ONE OR MORE trailing segments, and
  is only meaningful as the LAST segment of a subscription. `"a.#"` matches
  `"a.b"` and `"a.b.c"` and `"a.b.c.d"`, but NOT `"a"` (it requires at least one
  segment after the prefix). `"#"` on its own matches any topic of one or more
  segments (i.e. everything).

- `*` and `#` are wildcards ONLY in subscription topics. In a PUBLISHED topic
  they are ordinary literal text: publishing to `"a.*"` delivers only to a
  subscriber whose subscription topic is literally `"a.*"` (or a wildcard that
  matches the literal segment `*`, e.g. `"a.*"`'s single-`*` matches the literal
  segment `"*"`), never via wildcard expansion.

- DELIVERY ORDER is deterministic: when a publish matches several subscriptions,
  the callbacks fire in the ORDER THE SUBSCRIPTIONS WERE REGISTERED (globally,
  across all topics/patterns), regardless of whether each match was exact or via
  a wildcard.

- FIRE ONCE PER SUBSCRIPTION: each `subscribe(...)` call is one subscription and
  fires AT MOST ONCE per matching publish, even if its pattern could be seen to
  match in more than one way. If the SAME callable is subscribed twice (two
  `subscribe` calls), that is two subscriptions and it is invoked twice.

- Exact-topic subscriptions must keep working exactly as before.

- `publish(topic, data)` returns the number of callbacks it invoked (an int).
  `subscribe(topic, fn)` returns an opaque subscription handle.

## Example

    bus = EventBus()
    seen = []

    bus.subscribe("order.created", lambda t, d: seen.append(("exact", t, d)))
    bus.subscribe("order.*",       lambda t, d: seen.append(("star", t, d)))
    bus.subscribe("order.#",       lambda t, d: seen.append(("hash", t, d)))

    n = bus.publish("order.created", 7)
    # all three match "order.created"; they fire in subscription order:
    assert seen == [
        ("exact", "order.created", 7),
        ("star",  "order.created", 7),
        ("hash",  "order.created", 7),
    ]
    assert n == 3

    seen.clear()
    bus.publish("order.created.late", 1)
    # "order.*" does NOT match (two segments after "order"); only "order.#" does:
    assert seen == [("hash", "order.created.late", 1)]

A subscriber whose pattern could match "twice" still fires once:

    bus = EventBus()
    hits = []
    bus.subscribe("#", lambda t, d: hits.append(t))   # matches everything, once
    bus.publish("a.b.c", None)
    assert hits == ["a.b.c"]   # one call, not one-per-segment

## Contract

- Package name: `eventbus`. The grader imports `eventbus.public` (falling back
  to `eventbus`); keep both import paths working.
- Public class `EventBus` with methods:
  - `subscribe(topic: str, fn) -> handle` — register `fn` for `topic` (which may
    contain `*` / `#` wildcard segments). Returns an opaque handle.
  - `publish(topic: str, data) -> int` — invoke every matching subscription's
    callback as `fn(topic, data)`, in global subscription order, each at most
    once; return the number of callbacks invoked.
- A `*` segment matches exactly one segment; a trailing `#` matches one or more
  trailing segments; both are wildcards only in subscription topics.
- Standard library only. No threading requirement, no persistence.
````

**Graded behaviors:**

- `star_matches_one_segment` — 'a.*' matches the one-segment-deeper topic 'a.b'
- `star_rejects_too_deep` — 'a.*' does NOT match the two-deeper topic 'a.b.c'
- `star_rejects_too_shallow` — 'a.*' does NOT match the bare prefix 'a'
- `star_middle_position` — '*.created' matches 'order.created' but not 'order.created.late'
- `star_bare_one_segment` — bare '*' matches a one-segment topic but not a two-segment one
- `hash_matches_one_trailing` — 'a.#' matches 'a.b' (one trailing segment)
- `hash_matches_many_trailing` — 'a.#' matches 'a.b.c.d' (several trailing segments)
- `hash_requires_at_least_one` — 'a.#' does NOT match the bare prefix 'a' (needs >= 1 trailing)
- `hash_bare_matches_everything` — bare '#' matches any topic of one-or-more segments
- `fire_once_per_subscription` — a subscription fires at most once per publish (bare '#' over a deep topic)
- `same_fn_twice_fires_twice` — the same callable subscribed twice is two subscriptions -> two calls
- `wildcard_chars_literal_in_publish` — '*'/'#' in a PUBLISHED topic are literal, not wildcards
- `scenario_exact_depth_ordered` — mixed roster, publish 'order.created': exact+'*'+'#' fire in global order
- `scenario_deep_only_hash_ordered` — mixed roster, publish 'order.created.late': only the '#' patterns fire, in order
- `scenario_prefix_nothing` — mixed roster, publish bare 'order': '*'/'#' need >=1 deeper segment, none fire
- `scenario_sibling_isolation` — mixed roster, publish 'user.created': only the cross-cutting wildcards fire, in order
- `publish_returns_match_count` — publish returns the number of callbacks it invoked
- `regression_exact_delivery` — exact-topic subscribe/publish still delivers (and only to exact matches)
- `regression_exact_order_and_count` — exact-topic: registration-order delivery and correct return count


### hsm · 14 checks

````
You have inherited a small Python library named `hsm`: a finite state machine.
The package is already written, imports cleanly, and works: a `Machine` has a
`current` state, you wire up transitions with `add_transition(state, event,
target)`, and `fire(event)` moves the machine to the target (raising
`UnknownEvent` if the current state has no transition for that event).

Today the machine is FLAT: states have no structure, an event is handled only if
the CURRENT state itself defines a transition for it, and changing state does
nothing beyond updating `current`.

## Task

Add HIERARCHICAL (nested) states with event bubbling and entry/exit hooks,
WITHOUT breaking the existing flat behavior.

## Semantics (read carefully — this is the whole task)

- `add_state(name, parent=None)` declares a state nested inside `parent`. States
  form ancestor chains up to a root (a state with no parent). A state never
  needs to be declared to be used as a flat state; only nesting needs it.

- BUBBLING. `fire(event)` first looks for a transition on the CURRENT (leaf)
  state. If the leaf defines none for that event, the event BUBBLES up the
  parent chain: the first ancestor that defines a transition for the event wins,
  and the machine moves to THAT transition's target. If no state in the chain
  handles the event, raise `UnknownEvent`.

- ENTRY/EXIT HOOKS. `on_enter(state, fn)` and `on_exit(state, fn)` register
  zero-argument callbacks. On a transition from the current leaf `source` to a
  `target`, the machine:
    * EXITS states starting at `source` and walking UP, firing each state's exit
      hooks, stopping BEFORE the least common ancestor (LCA) of source and
      target — the LCA itself is NOT exited;
    * then ENTERS states from just below the LCA DOWN to `target`, firing each
      state's enter hooks — the LCA itself is NOT entered.
  Exit hooks fire deepest-first (child before parent); entry hooks fire
  shallowest-first (parent before child).

- The states exited/entered are computed from the actual current LEAF state and
  the transition's TARGET — NOT from the ancestor where the matching transition
  was found while bubbling. (A transition declared on a parent still exits the
  child you were actually in.)

- A transition whose source and target are the SAME state is an EXTERNAL
  self-transition: that state IS exited and re-entered (it sits below its LCA,
  which is its parent).

- If source and target are in separate trees (no common ancestor), exit every
  state from source up to its root and enter every state from target's root down
  to target.

- `Machine.trace` is a list recording the firing order: each entry is a tuple
  `("exit", state)` or `("enter", state)`, appended in the exact order hooks
  fire, accumulated across every `fire` call. It must be present and correct
  even for states that have no registered callbacks.

## Example

    m = Machine("a")
    m.add_state("top")
    m.add_state("a", parent="top")
    m.add_state("b", parent="top")
    m.add_transition("top", "go", "b")   # 'go' is handled by the PARENT

    m.fire("go")                 # in 'a', no 'go' -> bubbles to 'top', target 'b'
    assert m.current == "b"
    # exit 'a' (up to LCA 'top', exclusive), enter 'b' (down from 'top', exclusive)
    assert m.trace == [("exit", "a"), ("enter", "b")]

Deeper nesting keeps the common ancestor untouched:

    m = Machine("a1")
    m.add_state("root")
    m.add_state("A", parent="root"); m.add_state("B", parent="root")
    m.add_state("a1", parent="A");   m.add_state("b1", parent="B")
    m.add_transition("a1", "x", "b1")
    m.trace.clear()
    m.fire("x")                  # a1 -> b1, LCA is 'root'
    assert m.current == "b1"
    # exit a1 then A; enter B then b1; 'root' is the LCA and is NOT touched
    assert m.trace == [("exit", "a1"), ("exit", "A"), ("enter", "B"), ("enter", "b1")]

## Contract

- Package name: `hsm`. The grader imports `hsm.public` (falling back to `hsm`);
  keep both import paths working.
- Public class `Machine(initial)` with attribute `current`, list attribute
  `trace`, and methods: `add_transition(state, event, target)`,
  `add_state(name, parent=None)`, `on_enter(state, fn)`, `on_exit(state, fn)`,
  `fire(event) -> str` (returns the new `current`), and the existing
  `reset() -> str`.
- `UnknownEvent(Exception)` importable from the package, raised by `fire` when
  no state in the chain handles the event.
- A flat machine (no `add_state` calls) must behave exactly as before: `fire`
  resolves a transition on `current`, moves there, and — with no nesting — the
  trace for such a move is `[("exit", source), ("enter", target)]`.
- Standard library only. No threading requirement.
````

**Graded behaviors:**

- `flat_move` — flat fire moves current to the target
- `flat_unknown_event` — flat fire of an unhandled event raises UnknownEvent
- `flat_trace` — flat move traces exit(source) then enter(target)
- `bubble_to_parent` — event unhandled by leaf bubbles to a parent that handles it
- `bubble_target_from_leaf` — bubbled transition exits the actual LEAF, not the handling ancestor
- `lca_not_exited_or_entered` — least common ancestor is neither exited nor entered
- `exit_order_deepest_first` — exit hooks fire deepest-first up to the LCA
- `enter_order_shallowest_first` — enter hooks fire shallowest-first down to the target
- `self_transition_reenters` — external self-transition exits and re-enters the state
- `hook_callbacks_fire_in_order` — registered enter/exit callbacks run in trace order
- `separate_trees_full_chains` — transition across trees exits/enters full chains
- `trace_accumulates` — trace accumulates across multiple fire calls
- `unknown_event_after_bubble` — event no ancestor handles raises UnknownEvent
- `regression_flat_reset` — reset returns to initial; flat wiring still works


### kvtxn · 14 checks

````
You have inherited a small Python library named `kvtxn`: an in-memory key-value
store. The package is already written, imports cleanly, and its core operations
work: `get(key, default=None)`, `set(key, value)`, and `delete(key)`.

It ships with a FIRST, single-level attempt at transactions
(`begin` / `commit` / `rollback`) that snapshots the whole store on `begin` and
restores it on `rollback`. That attempt keeps only ONE snapshot, so it does not
nest: a second `begin` clobbers the first and the outer scope is lost.

## Task

Replace the flat attempt with proper NESTED transactions (savepoints). A
transaction can be opened inside another transaction, and each level can be
independently committed or rolled back.

## Semantics (read carefully — this is the whole task)

- `begin()` opens a new savepoint, nested inside whatever transaction is
  currently open (if any). Transactions therefore form a stack.

- `rollback()` undoes every change (`set`/`delete`) made since the MATCHING
  `begin()` — i.e. since the most recent still-open savepoint — restoring each
  affected key to exactly the value it held at that `begin()` (or to absent, if
  it did not exist then). It closes that savepoint. It must NOT touch changes
  that an ENCLOSING scope had already made before this savepoint opened.

- `commit()` closes the CURRENT savepoint by FOLDING its changes into the
  enclosing scope:
    * If there is an enclosing (parent) transaction, the changes become part of
      the parent. They are NOT yet durable: a later `rollback()` of the parent
      must still undo them. In other words, committing an inner transaction does
      not protect its changes from an outer rollback.
    * If this is the top-level transaction (no parent), the changes become
      durable in the store.

- A nested `rollback()` must NOT discard changes that an outer scope already
  committed/made; conversely a nested `commit()` must NOT make changes durable
  while an outer transaction is still open.

- `commit()` or `rollback()` with no transaction open is an error: raise
  `TransactionError` (a subclass of `RuntimeError`). Importing the name
  `TransactionError` from the package must work.

- `get`/`set`/`delete` must keep working exactly as before when no transaction
  is open (changes apply directly to the store).

## Example

    s = Store()
    s.set("a", 1)

    s.begin()              # outer txn
    s.set("a", 2)
    s.begin()              # inner txn, nested in outer
    s.set("a", 3)
    s.rollback()           # undo inner only
    assert s.get("a") == 2 # back to the outer txn's value, NOT 1

    s.begin()              # another inner txn
    s.set("a", 99)
    s.commit()             # fold inner into outer: a == 99 within the outer txn
    assert s.get("a") == 99

    s.rollback()           # undo the outer txn — including the folded inner change
    assert s.get("a") == 1 # all the way back to before the outer begin

A delete inside a transaction is undoable too:

    s = Store()
    s.set("x", 10)
    s.begin()
    s.delete("x")
    assert s.get("x") is None
    s.rollback()
    assert s.get("x") == 10 # the delete is undone

## Contract

- Package name: `kvtxn`. The grader imports `kvtxn.public` (falling back to
  `kvtxn`); keep both import paths working.
- Public class `Store` with methods: `get(key, default=None)`,
  `set(key, value)`, `delete(key) -> bool` (True iff the key was present),
  `begin()`, `commit()`, `rollback()`.
- `TransactionError(RuntimeError)` importable from the package, raised by
  `commit`/`rollback` when no transaction is open.
- Standard library only. No persistence, no threading requirement.
````

**Graded behaviors:**

- `single_commit_durable` — begin; set; commit makes the change durable
- `single_rollback_restores` — begin; set; rollback restores the prior value
- `rollback_restores_absent` — rollback of a set on a NEW key removes it again
- `nested_rollback_inner_only` — nested rollback restores only the inner savepoint
- `nested_commit_then_outer_rollback` — inner commit folds to parent; outer rollback still undoes all
- `nested_commit_visible_in_parent` — after inner commit the value is visible within the outer txn
- `nested_rollback_keeps_outer` — nested rollback does not discard the outer scope's earlier change
- `rollback_after_delete_restores` — rollback restores a value deleted inside the txn
- `commit_after_delete_durable` — a delete committed at top level stays deleted
- `three_level_partial` — 3 levels deep: rollback one, commit one, rollback outer rewinds all
- `error_commit_no_txn` — commit with no open transaction raises TransactionError
- `error_rollback_no_txn` — rollback with no open transaction raises TransactionError
- `regression_get_set_delete` — get/set/delete still work with no active transaction
- `regression_delete_return` — delete reports presence (True/False) with no txn


### middleware · 14 checks

````
You have inherited a small Python library named `middleware`: a tiny in-process
request router. The package is already written, imports cleanly, and its core
operations work: `add(path, handler)` registers a handler callable for a path,
and `dispatch(path)` looks the path up and calls its handler with the request
(here the request is just the path string). Dispatching an unregistered path
raises `NotFound`.

It has NO middleware support: `dispatch` simply finds the handler and calls it.

## Task

Add MIDDLEWARE: small functions, registered with `use(fn)`, that wrap AROUND the
handler in concentric layers — the "onion" model. Before-logic runs on the way
in; after-logic unwinds on the way out.

## Semantics (read carefully — this is the whole task)

A middleware is a callable `fn(request, next)` where:

- `request` is the request object (the path string), and
- `next` is a zero-argument callable that runs the REST of the chain (the inner
  layers, ending in the handler) and returns that inner response.

A middleware returns the response for its layer. The usual shape is:

    def example(request, next):
        # ... before logic ...
        response = next()      # run inner layers + handler, get their response
        # ... after logic; may transform `response` ...
        return response

- `use(fn)` registers middleware. Middleware run in REGISTRATION ORDER on the
  way in: the FIRST registered middleware is the OUTERMOST layer (its before-
  logic runs first), and the handler sits at the centre. On the way out, after-
  logic unwinds in REVERSE registration order (innermost first, outermost last).

- A middleware that returns WITHOUT calling `next()` SHORT-CIRCUITS: the inner
  layers and the handler never run, and `next()` is never reached for any layer
  deeper than this one. But every OUTER middleware that has already run its
  before-logic still runs its after-logic on the way back out (each is still
  inside its own `next()` call) and still gets to transform the response.

- After-logic can TRANSFORM the response: whatever a middleware returns becomes
  the response handed back to the layer outside it. The value `dispatch` returns
  is whatever the OUTERMOST layer returns (or the handler's response directly
  when there is no middleware).

- `next` runs the chain LAZILY from the point it is called. In particular, if a
  middleware short-circuits BEFORE the centre is reached, the handler lookup
  never happens — so dispatching a path with no registered handler does NOT
  raise `NotFound` as long as some middleware short-circuits before the centre.
  `NotFound` is only raised if the centre is actually reached for an
  unregistered path.

- Plain routes with NO middleware must behave EXACTLY as before: `dispatch`
  finds the handler, calls it with the path, returns its response, and raises
  `NotFound` for an unregistered path.

## Example

    r = Router()
    r.add("/greet", lambda req: "hello")

    trail = []
    def outer(request, next):
        trail.append("outer-before")
        resp = next()
        trail.append("outer-after")
        return resp + "!"
    def inner(request, next):
        trail.append("inner-before")
        resp = next()
        trail.append("inner-after")
        return resp.upper()

    r.use(outer)               # registered first -> outermost
    r.use(inner)               # registered second -> inner

    r.dispatch("/greet")       # -> "HELLO!"
    # trail == ["outer-before", "inner-before", "inner-after", "outer-after"]

A short-circuiting before-middleware skips the handler but still unwinds the
outer layer's after-logic:

    r = Router()
    r.add("/secret", lambda req: "TOP SECRET")

    def audit(request, next):
        resp = next()
        return f"[audited] {resp}"            # outer: transforms on the way out
    def guard(request, next):
        return "403 Forbidden"                # inner: short-circuits, never calls next

    r.use(audit)
    r.use(guard)

    r.dispatch("/secret")      # -> "[audited] 403 Forbidden"
                               # handler never ran; audit's after-logic still did

## Contract

- Package name: `middleware`. The grader imports `middleware.public` (falling
  back to `middleware`); keep both import paths working.
- Public class `Router` with methods:
    * `add(path, handler)` — register a handler callable for a path.
    * `use(fn)` — register a middleware `fn(request, next)`.
    * `dispatch(path) -> response` — run the middleware chain (if any) around the
      handler for `path` and return the outermost layer's response.
- `NotFound` (a subclass of `KeyError`) importable from the package, raised by
  `dispatch` when the centre is reached for an unregistered path.
- Middleware registration order defines the layering: index 0 is outermost
  (before-logic first, after-logic last); the handler is the centre.
- A short-circuit (returning without calling `next`) skips the handler and all
  deeper layers, but the already-entered outer layers still run their after-logic.
- Standard library only. No threading requirement.
````

**Graded behaviors:**

- `single_middleware_wraps` — one middleware runs before+after around the handler
- `before_order_registration` — before-logic runs in registration order (outer first)
- `after_order_reverse` — after-logic unwinds in REVERSE order (inner first, outer last)
- `full_onion_trail` — full enter/exit trail is outer-in then inner-out
- `response_transform_outer_wins` — dispatch returns the outermost layer's transformed value
- `short_circuit_skips_handler` — a before-middleware returning without next skips the handler
- `short_circuit_skips_deeper` — short-circuit skips middleware registered AFTER it (deeper)
- `short_circuit_runs_outer_after` — short-circuit still runs the OUTER layers' after-logic
- `short_circuit_outer_transforms` — outer after-logic still transforms the short-circuit response
- `lazy_next_no_notfound` — short-circuit before centre: unregistered path does NOT raise
- `notfound_when_centre_reached` — centre reached for unregistered path still raises NotFound
- `three_layer_order` — 3 middleware: before 0,1,2 then after 2,1,0
- `regression_plain_dispatch` — REGRESSION: dispatch with no middleware calls the handler
- `regression_plain_notfound` — REGRESSION: unregistered path with no middleware raises NotFound


### querygroup · 16 checks

````
You have inherited a small Python library named `querygroup`: a tiny, chainable
query engine over a list of dicts ("rows"). The package is already written,
imports cleanly, and its core operations work:

- `Query(rows)` wraps a list of row dicts (immutable view; never mutated).
- `where(predicate)` returns a new Query keeping only rows where
  `predicate(row)` is truthy.
- `order_by(key, reverse=False)` returns a new Query sorted by `key` (a column
  name or a function `row -> value`); stable sort.
- `rows()` materialises the current rows as a list of dicts.

Every operation returns a NEW Query, so chains compose.

## Task

Add grouped aggregation: a `group_by(keys, aggregates=...)` method that collapses
rows into ONE output row per distinct key-tuple, with aggregates computed over
chosen fields. It must compose correctly when chained AFTER a `where` (group the
filtered rows, not the originals).

## Semantics (read carefully — this is the whole task)

- `keys` is either a single column name (a string) or a sequence of column
  names. The group key for a row is the tuple of that row's values at those
  columns (a missing column counts as `None`). A single string key still groups
  correctly (it is treated as a one-element key list).

- Each output row carries the group's key columns (with their shared values),
  plus one column per requested aggregate.

- `aggregates` is a sequence of `(func, field, alias)` triples:
    * `func` is one of `count`, `sum`, `avg`, `min`, `max`.
    * `field` is the column the aggregate runs over.
    * `alias` is the output column name, or `None` to use `"<func>_<field>"`
      (e.g. `("sum", "pay", None)` -> column `"sum_pay"`).

- GROUP ORDERING is FIRST-APPEARANCE: groups are emitted in the order their
  key-tuple is FIRST encountered while scanning the current rows in order. Do
  NOT sort the groups.

- `count` counts the number of ROWS in the group.

- `sum` / `avg` / `min` / `max` consider only the NON-`None` values of `field`
  in the group (a row whose `field` is `None`, or missing, is skipped for that
  aggregate — but it still counts toward `count`).

- `avg` is the exact mean of the non-`None` values (no rounding). Crucially it
  divides by the number of NON-`None` values, NOT by the row count.

- EMPTY / all-`None` handling, per group and per field:
    * `sum`  of no non-`None` values is `0`.
    * `avg`  of no non-`None` values is `None` (not 0, and not a crash).
    * `min` / `max` of no non-`None` values are `None`.

- `group_by` returns a new `Query` (so its result can be `where`/`order_by`/
  `rows()`-ed like any other Query). Grouping an empty Query yields an empty
  Query (no groups, no error).

## Example

    q = Query([
        {"dept": "eng", "pay": 10},
        {"dept": "ops", "pay": 20},
        {"dept": "eng", "pay": 30},
        {"dept": "eng", "pay": None},
    ])

    out = q.group_by("dept", [
        ("count", "pay", None),
        ("sum",   "pay", None),
        ("avg",   "pay", None),
        ("max",   "pay", None),
    ]).rows()

    # First-appearance order: "eng" before "ops".
    # eng has 3 rows; non-None pays are [10, 30] -> sum 40, avg 20.0, max 30.
    assert out == [
        {"dept": "eng", "count_pay": 3, "sum_pay": 40, "avg_pay": 20.0, "max_pay": 30},
        {"dept": "ops", "count_pay": 1, "sum_pay": 20, "avg_pay": 20.0, "max_pay": 20},
    ]

Composing after a filter, and a group whose field is entirely None:

    q.where(lambda r: r["dept"] == "eng").group_by(
        "dept", [("avg", "pay", None), ("sum", "pay", None)]
    ).rows()
    # -> [{"dept": "eng", "avg_pay": 20.0, "sum_pay": 40}]

    Query([{"d": "x", "n": None}]).group_by(
        "d", [("avg", "n", None), ("min", "n", None), ("sum", "n", None)]
    ).rows()
    # -> [{"d": "x", "avg_n": None, "min_n": None, "sum_n": 0}]

## Contract

- Package name: `querygroup`. The grader imports `querygroup.public` (falling
  back to `querygroup`); keep both import paths working.
- Public class `Query` with the existing methods unchanged, plus
  `group_by(keys, aggregates=None)` as specified above.
- `where` / `order_by` / `rows` must keep behaving exactly as before
  (regression).
- Standard library only.
````

**Graded behaviors:**

- `single_group_count_sum` — one group: count counts rows, sum sums the field
- `multi_group_basic` — two groups, each with correct count/sum
- `first_appearance_order` — groups emitted in first-appearance order, not sorted
- `avg_divides_by_nonnull` — avg divides by count of non-None values, not row count
- `count_includes_null_rows` — count counts a row even when its aggregate field is None
- `min_max_skip_null` — min/max ignore None values in the field
- `all_null_field_aggs` — all-None field: avg/min/max are None, sum is 0
- `empty_group_via_filter` — empty Query (after a filter) groups to no rows
- `group_after_where` — group_by composes on the filtered rows after where()
- `multi_key_tuple` — grouping on two key columns keys by the tuple
- `default_alias_names` — default aggregate column name is '<func>_<field>'
- `custom_alias_used` — an explicit alias names the output column
- `result_is_chainable` — group_by returns a Query that can be order_by'd / rows()'d
- `avg_exact_not_rounded` — avg is the exact mean, not rounded/truncated
- `regression_where` — where() still filters rows with no grouping
- `regression_order_by` — order_by() still sorts rows (stable, reverse) with no grouping


### routerwild · 12 checks

````
You have inherited a small Python library named `routerwild`: a pure, in-process
URL-style path router. The package is already written, imports cleanly, and
everything it currently does works. Nothing here touches the network — `Router`
is a plain data structure you build with `add(path, handler)` and query with
`match(path)`.

A path is a `/`-separated string. Today each segment is one of:

  - a STATIC segment that matches itself literally (`users`, `v1`); or
  - a PARAM segment written `{name}` that matches exactly one non-empty segment
    and captures it under `name`.

`match(path)` returns `(handler, params)` for the best match, or `(None, {})`
when nothing matches. Static segments already take precedence over param
segments at the same position, so a literal route beats a parameterised one.

Examples of what already works:

    r = Router()
    r.add("/users/{id}", "show_user")
    r.add("/users/me", "current_user")

    r.match("/users/42")        # -> ("show_user", {"id": "42"})
    r.match("/users/me")        # -> ("current_user", {})     (static beats param)
    r.match("/users/42/extra")  # -> (None, {})               (param is one segment)
    r.match("/nope")            # -> (None, {})

## Feature request

Add WILDCARD (catch-all) segments written `{name:*}` that capture the entire
REMAINING path, INCLUDING slashes, under `name`. A wildcard is only meaningful
as the LAST segment of a route.

    r = Router()
    r.add("/files/{path:*}", "serve")

    r.match("/files/a/b/c")     # -> ("serve", {"path": "a/b/c"})
    r.match("/files/readme")    # -> ("serve", {"path": "readme"})

Wildcards sit at the LOWEST precedence: static > param > wildcard. A more
specific route (static or param) at the same prefix must still win over a
catch-all:

    r = Router()
    r.add("/files/{path:*}", "wild")
    r.add("/files/readme",   "exact")

    r.match("/files/readme")    # -> ("exact", {})              (static beats wildcard)
    r.match("/files/a/b")       # -> ("wild", {"path": "a/b"})  (catch-all otherwise)

The existing static and param behaviour, the precedence among them, and the
`(handler, params)` / `(None, {})` return shape must all keep working unchanged.

## Contract

- Package name: `routerwild`. The grader imports `routerwild.public` (falling
  back to `routerwild`); keep both import paths working.
- Keep the PUBLIC API: `Router()`, `Router.add(path, handler)`, and
  `Router.match(path) -> (handler, params)` (a hit) or `(None, {})` (a miss).
  Do not rename anything or change these signatures.
- Keep STATIC and `{param}` matching working exactly as before, including
  static > param precedence and the `(handler, params)` / `(None, {})` returns.
- ADD `{name:*}` wildcard segments that capture the rest of the path (slashes
  included) under `name`, at the LOWEST precedence: static > param > wildcard.
- Standard library only.
````

**Graded behaviors:**

- `wild_captures_rest` — NEW: {path:*} matches /files/a/b/c capturing 'a/b/c'
- `wild_single_segment` — NEW: {path:*} also captures a single remaining segment
- `wild_includes_slashes` — NEW: wildcard capture keeps embedded slashes verbatim
- `wild_after_param` — NEW: a wildcard can follow a param, capturing the tail
- `static_beats_wildcard` — NEW: an exact static route beats a wildcard at the same prefix
- `param_beats_wildcard` — NEW: a param segment beats a wildcard at the same position
- `wild_no_false_match` — NEW: a wildcard route does not match an unrelated prefix
- `static_match` — REGRESSION: a static route matches and returns no params
- `param_capture` — REGRESSION: a {name} param matches one segment and captures it
- `static_beats_param` — REGRESSION: a static route beats a param at the same position
- `param_is_single_segment` — REGRESSION: a param matches exactly one segment, not more
- `no_match_returns_none` — REGRESSION: an unmatched path returns (None, {})


### schemaoneof · 17 checks

````
You have inherited a small Python library named `schemaoneof`: a validator for a
subset of JSON Schema. The package is already written, imports cleanly, and
works for the keywords it supports.

The single public entry point is:

    def validate(instance, schema) -> list[dict]: ...

It walks `instance` against `schema` and returns a LIST OF ERROR DICTS. An EMPTY
list means the instance is valid; a non-empty list means it failed one or more
constraints. (The exact error-dict shape is up to you — the shipped code uses
`{"path": ..., "keyword": ..., "message": ...}` — but "empty vs non-empty" is
the load-bearing signal.)

## What works today

The validator currently supports these keywords:

  - `type`        — "object" / "array" / "string" / "number" / "integer" /
                    "boolean" / "null". (Note: booleans are NOT numbers/integers,
                    and "number" accepts ints; "integer" rejects floats.)
  - `enum`        — instance must deep-equal one of the listed values.
  - `required`    — listed property names must be present on an object.
  - `properties`  — each present property is validated against its subschema.

Examples of current behavior:

    validate(5, {"type": "integer"})                 -> []          (valid)
    validate("x", {"type": "integer"})               -> [ ...1 err ]
    validate({"a": 1}, {"required": ["a", "b"]})      -> [ ...1 err ] (b missing)
    validate("green", {"enum": ["red", "green"]})     -> []          (valid)

## The capability to ADD

The validator is MISSING two JSON-Schema combinators. Add both:

### `oneOf`

`oneOf` is a list of subschemas. The instance must match EXACTLY ONE of them.

  - Zero subschemas match  -> ERROR (non-empty list).
  - Exactly one matches    -> VALID (no error from oneOf).
  - Two or more match      -> ERROR (non-empty list).

A subschema "matches" when validating the instance against it yields no errors.

Examples (desired behavior once added):

    validate(5,  {"oneOf": [{"type": "integer"}, {"type": "string"}]})  -> []
        # matches integer only -> exactly one -> valid

    validate(True, {"oneOf": [{"type": "integer"}, {"type": "string"}]}) -> [err]
        # matches neither (bool is not integer/string) -> zero -> error

    validate(5,  {"oneOf": [{"type": "integer"}, {"type": "number"}]})  -> [err]
        # 5 matches BOTH integer and number -> two -> error

### `not`

`not` is a single subschema. The instance must NOT match it. If the instance
DOES match the subschema, that is an ERROR.

Examples (desired behavior once added):

    validate(5,   {"not": {"type": "string"}})   -> []      (5 is not a string -> ok)
    validate("x", {"not": {"type": "string"}})   -> [err]   ("x" IS a string -> error)

## Contract

  - Package name: `schemaoneof`. The grader imports `schemaoneof.public`
    (falling back to `schemaoneof`); keep both import paths working.
  - Keep the existing keyword behavior UNCHANGED: `type`, `required`,
    `properties`, and `enum` must still validate exactly as they do now, and
    `validate` must still return a LIST (empty == valid).
  - ADD `oneOf` (exactly-one-of) and `not` (must-not-match) as described above.
    They compose with the other keywords on the same schema (all present
    keywords are checked).
  - Standard library only. Do not rename `validate` or change its meaning.
````

**Graded behaviors:**

- `oneof_exactly_one_valid` — oneOf with exactly one matching subschema is VALID
- `oneof_zero_matches_error` — oneOf with zero matching subschemas is an ERROR
- `oneof_two_matches_error` — oneOf with two matching subschemas is an ERROR
- `oneof_nested_object` — oneOf discriminates between two object shapes
- `not_match_is_error` — not is an ERROR when the subschema MATCHES the instance
- `not_nomatch_is_valid` — not is VALID when the subschema does NOT match
- `combinator_composes` — oneOf/not compose with sibling keywords on one schema
- `type_ok` — a value of the declared type validates clean
- `type_mismatch` — a value of the wrong type is an error
- `type_number_accepts_int` — type 'number' accepts an integer
- `required_present_ok` — an object with all required props is valid
- `required_missing_error` — a missing required property is an error
- `properties_ok` — a property whose value fits its subschema is valid
- `properties_nested_error` — a property whose value violates its subschema errors
- `enum_member_ok` — a value in the enum is valid
- `enum_nonmember_error` — a value not in the enum is an error
- `valid_empty_list` — a fully-valid instance yields an EMPTY error list


### searchprefix · 10 checks

````
You are handed an existing, WORKING Python 3.11+ package named `searchprefix`. It is
already in your working directory. Your job is to ADD ONE capability described below
WITHOUT breaking any of the existing behavior. Use only the Python standard library.

## What the package does today

`searchprefix.public.SearchIndex` is a small in-memory search index:

    from searchprefix.public import SearchIndex

    idx = SearchIndex()
    idx.add_document("d1", "Payment received and processed")
    idx.add_document("d2", "Refund payable next week")
    idx.add_document("d3", "Shipping label printed")

  - `add_document(doc_id, text)` tokenizes `text` into lowercase alphanumeric terms
    and records, per document, how many times each term appears.
  - `search(query) -> list[doc_id]` tokenizes the query the same way and returns the
    ids of documents that contain EVERY query term, using case-insensitive EXACT-term
    matching. Results are ranked by total term frequency (the summed count of the
    query terms within the document), highest first; ties break by ascending doc id,
    so the order is deterministic.

So today `search("payment")` returns `["d1"]` (exact term `payment`), and
`search("pay")` returns `[]` — because `pay` is not an exact term in any document
(the documents have `payment` and `payable`, not `pay`).

## The feature to ADD: prefix queries

Add support for PREFIX query tokens. A query token that ends with `*` matches any
document term that STARTS WITH the text before the `*`. Plain tokens (no `*`) keep
their current exact-match behavior.

Examples (against the three documents above):

  - `search("pay*")` matches d1 (term `payment`) and d2 (term `payable`) — both
    start with `pay`. It does NOT match d3. Ranking still follows term frequency.
  - `search("ship*")` matches d3 (`shipping`).
  - A prefix only matches at the START of a term: `search("ment*")` does NOT match
    `payment` (mid-word is not a prefix).
  - A plain token is unchanged: `search("payment")` still matches only d1, and
    `search("pay")` (no `*`) still matches nothing.

A query may mix plain and prefix tokens; as today, a document must match EVERY token
(each plain token by exact term, each prefix token by some term sharing the prefix).
A prefix token's contribution to the frequency score is the summed count of every
document term that starts with the prefix.

## Contract

  - Package name stays `searchprefix`; keep the class `SearchIndex` with the public
    API `add_document(doc_id, text)` and `search(query) -> list[doc_id]`, exposed
    from `searchprefix.public` (and re-exported from `searchprefix`).
  - Do NOT change the existing exact-term behavior, the ranking (term frequency
    descending, ascending-doc-id tie-break), or the determinism. Every query that
    worked before must still return the same result.
  - ADD prefix matching: a query token ending in `*` matches any document term
    beginning with the prefix before the `*`. The `*` is the prefix marker, not part
    of the matched text. A bare `*` (empty prefix) contributes nothing / is ignored.
  - Mid-word (non-prefix) substrings must NOT match.

Do not change the package name, the class name, or the method signatures. Add prefix
support so the examples above hold while every existing exact-search and ranking
behavior is preserved.
````

**Graded behaviors:**

- `prefix_matches_multiple` — search('pay*')=['d1', 'd2']
- `prefix_excludes_unrelated` — search('pay*')=['d1', 'd2']
- `prefix_other_term` — search('ship*')=['d3']
- `prefix_not_midword` — search('ment*')=[]
- `prefix_ranks_by_frequency` — search('pay*')=['d1', 'd2']
- `mixed_plain_and_prefix` — search('received pay*')=['d1']
- `exact_term_match` — search('payment')=['d1']
- `exact_not_prefix` — search('pay')=[]
- `rank_by_term_frequency` — search('alpha')=['r1', 'r2']
- `and_semantics_multi_term` — search('quick fox')=['m1']


### serialhook · 17 checks

````
You have inherited a small Python library named `serialhook`: a JSON-like
serializer. The package is already written, imports cleanly, and works for the
JSON basic types: `dumps(obj)` serializes a `dict` / `list` / `str` / `int` /
`float` / `bool` / `None` to a JSON string, and `loads(s)` parses it back. It is
a thin wrapper over the standard library's `json`, so its output for basic
values is byte-for-byte identical to `json.dumps` with default settings.

Two things are missing. There is NO way to serialize custom Python types
(datetime, Decimal, …) — they raise `TypeError` like `json` does — and there is
NO guard against circular references, so a structure that contains itself
recurses until the interpreter gives up.

## Task

Add (a) a custom-type HOOK mechanism and (b) circular-reference DETECTION, WITHOUT
breaking the existing basic-type behavior.

## Semantics (read carefully — this is the whole task)

### (a) Custom-type hooks

- Add `register(type, tag, encode, decode)`. After registering, a value of that
  `type` serializes to a TAGGED FORM: the JSON object
  `{"__type__": tag, "value": <encoded>}`, where `<encoded>` is whatever
  `encode(value)` returns. `loads` reverses it: a tagged form whose `tag` is
  registered is routed to that type's `decode` callable, reconstructing the
  original value.

- The encoded payload is itself serialized RECURSIVELY. So `encode` may return a
  payload that contains basic types OR other registered types (e.g. a dict that
  holds a datetime), and it must round-trip. On `loads`, decode the payload
  FIRST, then hand the decoded payload to the type's `decode`.

- Registered types must round-trip when NESTED anywhere — inside lists, inside
  dict values, and inside the payloads of other registered types — not just at
  the top level.

- Decode dispatch is BY TAG. A tagged form naming a tag that is not registered
  is an error: raise `UnknownTagError`. Do not silently return the raw tagged
  dict.

- Be careful not to mistake ordinary user data for a tagged form. A dict is a
  tagged form ONLY when it has exactly the two keys `__type__` and `value` and
  its `__type__` is a string. A plain dict that merely happens to contain a
  `"__type__"` key alongside other keys is ordinary data and must round-trip
  unchanged.

### (b) Circular-reference detection

- `dumps` must detect cycles and raise `CircularReferenceError` (a clear error),
  not recurse forever. Detection is along the CURRENT PATH (ancestry from the
  root to the value being written), not "seen anywhere".

- A value that merely appears more than once in SIBLING positions — a shared or
  diamond reference, e.g. the same inner list placed at two keys of a dict — is
  NOT a cycle and must serialize fine.

### General

- Do not change the basic-type behavior. `dumps` of a basic value must stay
  byte-identical to `json.dumps(value)` defaults, and `loads(dumps(x)) == x` for
  any basic `x`.

## Example

    from datetime import datetime
    from decimal import Decimal

    register(datetime, "datetime",
             lambda dt: dt.isoformat(),
             lambda s: datetime.fromisoformat(s))
    register(Decimal, "decimal", str, Decimal)

    dt = datetime(2020, 1, 2, 3, 4, 5)
    wire = dumps({"when": dt, "tags": ["a", "b"]})
    # wire == '{"when": {"__type__": "datetime", "value": "2020-01-02T03:04:05"}, "tags": ["a", "b"]}'
    back = loads(wire)
    assert back["when"] == dt          # nested registered type round-trips
    assert back["tags"] == ["a", "b"]  # basic data untouched

    # A registered type inside another registered type's payload:
    wire = dumps([Decimal("1.5"), {"d": Decimal("2")}])
    assert loads(wire) == [Decimal("1.5"), {"d": Decimal("2")}]

    # Shared (NOT circular) reference is fine:
    inner = [1, 2]
    dumps({"x": inner, "y": inner})    # OK — no error

    # Circular reference is rejected:
    a = []
    a.append(a)
    dumps(a)                           # raises CircularReferenceError

    # Basic round-trip stays byte-identical:
    assert dumps({"b": 1, "a": [True, None, 1.5]}) == '{"b": 1, "a": [true, null, 1.5]}'

    # A plain dict that happens to have a "__type__" key is data, not a tag:
    assert loads(dumps({"__type__": "x", "n": 1})) == {"__type__": "x", "n": 1}

## Contract

- Package name: `serialhook`. The grader imports `serialhook.public` (falling
  back to `serialhook`); keep both import paths working.
- Module-level functions `dumps(obj) -> str`, `loads(s) -> obj`, and
  `register(type, tag, encode, decode) -> None`, importable from the package.
- `CircularReferenceError` and `UnknownTagError`, both importable from the
  package, raised as described above. (They may share a common base class, but
  that is not required.)
- Standard library only. No threading requirement.
````

**Graded behaviors:**

- `register_top_level` — a registered type round-trips at the top level via the tagged form
- `tagged_wire_shape` — a registered value serializes to {'__type__': tag, 'value': <encoded>}
- `registered_in_list` — a registered type nested inside a list round-trips
- `registered_in_dict_value` — a registered type nested as a dict value round-trips
- `registered_in_payload` — a registered type inside another registered type's payload round-trips
- `two_types_mixed` — two registered types in one structure each decode via their own tag
- `unknown_tag_raises` — loads of a tagged form with an unregistered tag raises UnknownTagError
- `plain_type_key_is_data` — a plain dict with a '__type__' key among others is data, not a tag
- `exact_two_key_required` — a dict with __type__/value plus a third key is data, not a tag
- `cycle_self_list` — dumps of a list that contains itself raises CircularReferenceError
- `cycle_self_dict` — dumps of a dict that contains itself raises CircularReferenceError
- `cycle_indirect` — dumps of an indirect (a->b->a) cycle raises CircularReferenceError
- `shared_ref_ok` — a shared (diamond) reference is NOT a cycle and serializes fine
- `shared_ref_reused_after` — the same list reused in a sibling AND deeper position serializes fine
- `regression_basic_roundtrip` — basic types round-trip: loads(dumps(x)) == x
- `regression_byte_identical` — dumps of basic values is byte-identical to json.dumps defaults
- `regression_bool_int_distinct` — bool/int/float/None survive the round-trip with the right types


### tierlimit · 16 checks

````
You have inherited a small Python library named `tierlimit`: a fixed-window rate
limiter. The package is already written, imports cleanly, and works: it admits
at most `limit` requests in each fixed wall-clock window of `window` seconds,
against ONE global counter shared by every call.

`now` is supplied by the caller as an absolute, non-decreasing float (seconds),
so behaviour is deterministic — there is no real clock and no sleeping. The
window containing `now` is the half-open interval `[w0, w0 + window)` where
`w0 = floor(now / window) * window`; when a new window begins the count starts
over at zero.

## Task

Add PER-KEY rate limiting and TIERS on top of the existing global limiter,
without breaking the global path.

- Per-key limiting: a new method `allow_key(key, now)` limits each caller `key`
  against its OWN independent fixed window and count. Different keys never share
  a budget, and their windows advance independently (a key's window comes purely
  from the `now` values it is called with).

- Tiers: a tier is a NAME that maps to a per-window limit. `set_tier(key, tier)`
  assigns a key to a tier. The limiter is constructed with a `tiers` mapping and
  a `default_tier`; any key never assigned a tier uses the default tier.

## Semantics (read carefully — this is the whole task)

- Each key's window and count are INDEPENDENT. One key crossing a window
  boundary, or hitting its limit, must not disturb any other key. A brand-new
  key starts with an empty count in whatever window its first `now` falls in.

- `allow_key(key, now)` admits the request iff the count already spent in the
  key's CURRENT window is strictly less than the key's current tier limit; on
  admit it increments that count and returns True, otherwise it returns False
  and counts nothing.

- Changing a key's tier MID-WINDOW does NOT reset the key's count or start a
  fresh window. The key keeps the requests it has already spent in the current
  window; only the EFFECTIVE LIMIT used for the comparison changes, applied
  against that SAME window's existing count, taking effect from the next
  `allow_key`:
    * Lowering a key's tier mid-window can immediately push it over: a key that
      already spent 3 requests under a limit-5 tier is over a limit-2 tier, so
      its next `allow_key` in that window is denied — even though it never
      "used" the lower tier. The change is NOT retroactive: it does not revoke
      or refund requests already decided.
    * Raising a key's tier mid-window immediately grants more room in the SAME
      window (the existing count is measured against the higher limit).
  When the key next crosses into a fresh window, the count resets to zero and
  the new tier applies cleanly from there.

- `set_tier(key, tier)` with an unknown tier name raises `ValueError`.

- The GLOBAL path is unchanged: `allow(now)` still admits at most `limit`
  requests per window against a single global counter, and shares nothing with
  the per-key state (a global call must not consume any key's budget, and vice
  versa).

## Example

    r = RateLimiter(limit=5, window=10.0,
                    tiers={"free": 2, "pro": 5}, default_tier="free")

    r.allow_key("alice", 0.0)   # alice on default tier "free" (limit 2) -> True (1/2)
    r.allow_key("alice", 1.0)   # -> True (2/2)
    r.allow_key("alice", 2.0)   # 3rd in window [0,10) -> False (over free's 2)

    r.allow_key("bob", 2.0)     # bob is independent, fresh -> True

    r.set_tier("alice", "pro")  # mid-window upgrade; alice's count (2) is kept
    r.allow_key("alice", 3.0)   # now under pro's 5, same window -> True (3/5)

    r.allow_key("alice", 10.0)  # new window [10,20): count resets -> True (1/5)

A mid-window DOWNGRADE pushes a key over against its existing count:

    r.allow_key("carol", 0.0)   # default free (2): True (1/2)
    r.set_tier("carol", "pro")  # pro = 5
    r.allow_key("carol", 1.0)   # True (2/5)
    r.allow_key("carol", 2.0)   # True (3/5)
    r.set_tier("carol", "free") # downgrade to free (2); count is already 3
    r.allow_key("carol", 3.0)   # 3 >= 2 -> False, still in window [0,10)

## Contract

- Package name: `tierlimit`. The grader imports `tierlimit.public` (falling back
  to `tierlimit`); keep both import paths working.
- Public class `RateLimiter`, constructed as
  `RateLimiter(limit, window, tiers=None, default_tier="default")`:
    * `limit` (positive int) and `window` (positive float) keep their meaning for
      the global path. When `tiers` is None, a single tier named `"default"` is
      created mapping to `limit`, and `default_tier` defaults to `"default"` —
      so `RateLimiter(limit, window)` still constructs and runs the global path
      exactly as before (regression).
    * `tiers` maps tier name -> per-window limit (positive ints).
    * `default_tier` names the tier used for keys with no explicit tier; it must
      be a key of `tiers` (else raise `ValueError` at construction).
- Methods:
    * `allow(now: float) -> bool` — UNCHANGED global path.
    * `allow_key(key: str, now: float) -> bool` — per-key path described above.
    * `set_tier(key: str, tier: str) -> None` — assign a key's tier; unknown
      tier name raises `ValueError`.
- The grader ALWAYS passes explicit `now` floats (non-decreasing per key); there
  is no real clock and no sleeping.
- Standard library only. No persistence, no threading requirement.
````

**Graded behaviors:**

- `default_tier_basic` — unassigned key uses the default tier's limit
- `per_key_independent_count` — each key has its own count; one key's limit does not block another
- `per_key_independent_window` — each key's window advances independently of other keys
- `window_reset_per_key` — a key's count resets when it crosses into a new window
- `explicit_tier_limit` — set_tier selects that tier's limit for the key
- `upgrade_midwindow_keeps_count` — mid-window upgrade keeps the existing count and grants more room
- `downgrade_midwindow_over` — mid-window downgrade pushes an over-budget key past the lower limit
- `set_tier_not_retroactive` — set_tier does not refund/revoke already-decided requests in the window
- `downgrade_then_new_window` — after a downgrade, the next window resets the count under the new tier
- `tier_reassign_twice` — down-then-up mid-window tracks the live limit against the kept count
- `boundary_window_math` — window boundary is half-open [w0, w0+window) via floor
- `unknown_tier_raises` — set_tier with an unknown tier name raises ValueError
- `bad_default_tier_raises` — constructing with a default_tier not in tiers raises ValueError
- `regression_global_basic` — global allow(now) admits at most limit per window, resets each window
- `regression_global_isolated` — global path and per-key path do not share budget
- `regression_bare_constructor` — RateLimiter(limit, window) still constructs and runs the global path


### tmploop · 20 checks

````
You have inherited a small Python library named `tmploop`: a string templater.
The package is already written, imports cleanly, and works: a single function
`render(template, context)` substitutes `{{ var }}` placeholders with values
looked up from `context`. Lookups may be DOTTED — `{{ user.name }}` walks dict
keys (falling back to object attributes) — and a missing lookup renders as the
empty string. Literal text outside `{{ ... }}` is preserved verbatim.

What it CANNOT do yet is loop or branch: there are no block tags.

## Task

Add two nestable BLOCK TAGS to the templater, leaving plain `{{ var }}`
rendering exactly as it is.

- `{{#each items}} BODY {{/each}}` — iterate `items` in order, rendering BODY
  once per element.

- `{{#if cond}} A {{else}} B {{/if}}` — render A when `cond` is truthy, else B.
  The `{{else}}` arm is optional (`{{#if cond}} A {{/if}}` renders A or nothing).

The two blocks must NEST in any combination (an `each` inside an `if`, an `if`
inside an `each`, `each` inside `each`, to any depth), and the right closer must
pair with the right opener.

## Semantics (read carefully — this is the whole task)

### `each`

- Inside the body, these names are available for the CURRENT element:
    * `{{ this }}`    — the element itself.
    * `{{ @index }}`  — its 0-based position (an integer: 0, 1, 2, ...).
    * `{{ @first }}`  — boolean, true on the first element only.
    * `{{ @last }}`   — boolean, true on the last element only.
  `@first` / `@last` are usable as `{{#if}}` conditions.
- `{{ this.field }}` walks into the element with the usual dotted lookup. When
  the element is a dict, its keys are ALSO reachable bare (`{{ field }}`), and
  those bare names SHADOW the outer context for that one iteration. Names not
  found in the element fall back to the outer (enclosing) context.
- An `items` that is MISSING, `None`, empty, or not a list/sequence yields ZERO
  iterations (BODY is emitted zero times). Strings and dicts are treated as
  NON-iterable here — you iterate a list, not the characters of a string.
- After the loop, the loop-local names (`this`, `@index`, ...) are gone again;
  they do not leak to the enclosing scope.

### `if`

- `cond` is resolved with the same (dotted) lookup as a variable, then tested
  for Python truthiness. FALSY means: a missing variable, `None`, `False`, `0`,
  `""`, or an empty collection (`[]`, `{}`). Everything else is truthy.
- The `{{else}}` arm is optional. Exactly one arm renders.

### Unchanged

- A plain `{{ var }}` (including dotted `{{ a.b.c }}`) renders exactly as before:
  the looked-up value coerced with `str()`, or `""` if missing or `None`. Booleans
  render as `true` / `false` (lowercase).

## Whitespace (pin this exactly)

- The engine performs NO whitespace trimming around tags. Each tag span — from
  the opening `{{` to the closing `}}` — is removed exactly where it sits, and
  every other character (spaces, tabs, newlines, literal text) is preserved BYTE
  FOR BYTE. There is no "standalone tag" newline-eating and no body trimming.

      render("a {{#if t}} b {{/if}} c", {"t": True})  ==  "a  b  c"

  (Note the two spaces on each side: the spaces that flanked the now-removed
  tags survive.) This is what lets output be checked by exact equality.

- Inside `{{ ... }}` the inner expression IS stripped of surrounding whitespace,
  so `{{ name }}` and `{{name}}` resolve the same variable.

## Example

    render("{{#each xs}}[{{ @index }}:{{ this }}]{{/each}}", {"xs": ["a", "b", "c"]})
    # -> "[0:a][1:b][2:c]"

    render(
        "{{#each users}}{{#if @first}}{{ name }}{{else}}, {{ name }}{{/if}}{{/each}}",
        {"users": [{"name": "Ada"}, {"name": "Bo"}, {"name": "Cy"}]},
    )
    # -> "Ada, Bo, Cy"

    render(
        "{{#if items}}<ul>{{#each items}}<li>{{ this }}</li>{{/each}}</ul>{{else}}empty{{/if}}",
        {"items": []},
    )
    # -> "empty"        (an empty list is falsy, so the else arm renders)

A bare field of a dict element shadows the outer context for that iteration:

    render("{{#each rows}}{{ x }}{{/each}}", {"x": "OUT", "rows": [{"x": "A"}, {}]})
    # -> "AOUT"         (first row has its own x=A; the second {} falls back to OUT)

## Contract

- Package name: `tmploop`. The grader imports `tmploop.public` (falling back to
  `tmploop`); keep both import paths working.
- Public function `render(template: str, context: dict) -> str`. It returns a
  `str`; it does not print and does not write files.
- `render(template, None)` is allowed and behaves as if `context` were `{}`.
- A MALFORMED template — an unclosed `{{#each}}`/`{{#if}}`, a stray
  `{{/each}}`/`{{/if}}` with no opener, a mismatched closer (`{{#each}}...{{/if}}`),
  or an `{{else}}` outside an `{{#if}}` — raises an exception from `render` rather
  than silently producing wrong output or hanging.
- Standard library only. Do not use `eval`. No persistence, no threading.
````

**Graded behaviors:**

- `each_basic` — each iterates a list, exposing {{ this }}
- `each_index` — each exposes {{ @index }} (0-based)
- `each_first_last` — each exposes {{ @first }}/{{ @last }} usable as #if conditions
- `each_empty_zero` — an empty each collection iterates ZERO times
- `each_missing_zero` — a missing each collection iterates ZERO times
- `each_dotted_this` — {{ this.field }} walks into a dict element
- `each_dict_shadow` — a dict element's keys shadow outer scope, with fallback
- `each_string_not_iterated` — a string value is NOT iterated char-by-char by each
- `if_then_truthy` — #if renders the then-arm when cond is truthy
- `if_else_falsy` — #if renders the else-arm when cond is falsy
- `if_empty_collection_falsy` — an empty list/dict/0/'' is FALSY for #if
- `if_no_else_arm` — #if with no {{else}} emits nothing when cond is falsy
- `nest_each_in_if` — an each block nested inside an if block
- `nest_if_in_each` — an if block nested inside an each block (per-element)
- `nest_each_in_each` — each-in-each nests and closers pair correctly
- `whitespace_preserved` — literal whitespace around tags is preserved byte-for-byte
- `malformed_unclosed_raises` — an unclosed block raises rather than mis-rendering
- `malformed_mismatch_raises` — a mismatched closer ({{#each}}..{{/if}}) raises
- `regression_plain_var` — plain {{ var }} substitution still works
- `regression_dotted_and_missing` — dotted lookup works; missing renders as ''


## repair-to-green (15)

_A library ships with planted bugs and a visible failing test suite; make it green (a hidden superset grades)._


### calceval · 33 checks

````
BUG REPORT — calceval: the test suite is failing

You have an existing Python package `calceval`, a tiny infix arithmetic
evaluator. It ships with a unittest suite in `calceval/test_calceval.py`, and
right now several of those tests FAIL. Fix the code so that ALL the tests pass.
Do not rewrite the package from scratch and do not change its public API. Do NOT
use `eval` / `exec`; keep the hand-written parser.

## Symptom

`evaluate(expr)` parses an infix arithmetic expression and returns its value as a
float. Simple precedence already works, but several associativity / binding cases
come out wrong:

    from calceval.public import evaluate

    # subtraction parses right-associative instead of left:
    evaluate("10-2-3")
    #   EXPECTED 5.0     ((10-2)-3)
    #   ACTUAL   11.0    (10-(2-3))

    # division has the same right-associative defect:
    evaluate("100/10/2")
    #   EXPECTED 5.0     ((100/10)/2)
    #   ACTUAL   20.0    (100/(10/2))

    # '^' parses left-associative instead of right:
    evaluate("2^3^2")
    #   EXPECTED 512.0   (2^(3^2))
    #   ACTUAL   64.0    ((2^3)^2)

    # unary minus binds too tightly — tighter than '^' instead of looser:
    evaluate("-2^2")
    #   EXPECTED -4.0    (-(2^2))
    #   ACTUAL   4.0     ((-2)^2)

These defects interact: `-2^2^2` exercises the unary binding AND the exponent
associativity at once, and expressions like `10-2-3^1^2*2` touch all of them.

## Reproduce

Run the visible tests from the directory that contains the `calceval` package:

    python -m unittest calceval.test_calceval

## Contract (must hold after your fix)

* Package name stays `calceval`; import path `calceval` / `calceval.public`.
* Keep the public API exactly: `evaluate(expr: str) -> float` and the `CalcError`
  exception. Do not rename them. The result is ALWAYS a `float`.
* Supported tokens: integer and decimal numbers (`3`, `3.5`, `.5`, `10.`), the
  binary operators `+ - * / ^`, parentheses `( )`, and a prefix (unary) `-`
  (a prefix `+` is tolerated as a no-op). Whitespace is insignificant.
* PRECEDENCE, from loosest-binding to tightest-binding:
    1. `+` and `-`  (binary)  — LEFT-associative
    2. `*` and `/`            — LEFT-associative
    3. unary `-`  (prefix negation) — binds LOOSER than `^`
    4. `^`  (exponent)        — RIGHT-associative, the tightest binding
  Parentheses override all of the above.
* ASSOCIATIVITY in detail:
    - `a-b-c` is `(a-b)-c` and `a/b/c` is `(a/b)/c` (LEFT). So `10-2-3 == 5` and
      `100/10/2 == 5`, never `11` / `20`.
    - `a^b^c` is `a^(b^c)` (RIGHT). So `2^3^2 == 512`, never `64`.
* UNARY-vs-EXPONENT binding: prefix `-` binds LOOSER than `^`, so `-2^2` is
  `-(2^2) == -4`, and `-2^4 == -16`. To negate the base you must parenthesize:
  `(-2)^2 == 4`. A `^` right operand may itself be unary, so `2^-1 == 0.5`.
* MALFORMED input raises `CalcError`: an empty or all-whitespace string, an
  unbalanced parenthesis, a dangling/stray operator (`"2+"`, `"*3"`), an unknown
  character, or division by zero. Do NOT raise a bare `ValueError`/`ZeroDivisionError`
  — wrap them as `CalcError` (which may subclass `ValueError`).
* Do NOT use `eval` / `exec`. Standard library only.

Example:

    evaluate("-2^2^2")    #  -> -16.0    (-(2^(2^2)) == -(2^4))
    evaluate("10-2-3^1^2*2")  #  -> 2.0  ((10-2) - ((3^(1^2))*2) == 8 - 6)
````

**Graded behaviors:**

- `basic_mul_over_add` — * binds tighter than + (2+3*4 == 14)
- `basic_add_mul_chain` — mixed +/* precedence (2*3+4*5 == 26)
- `basic_sub_mul` — * binds tighter than - (10-2*3 == 4)
- `basic_parens` — parentheses override precedence ((2+3)*4 == 20)
- `basic_pow_over_mul` — ^ binds tighter than * (2*3^2 == 18)
- `basic_single_number` — a lone number evaluates to itself (42 -> 42.0)
- `basic_decimal` — decimals parse and compute (3.5*2 == 7.0)
- `basic_leading_dot` — leading-dot decimals parse (.5+.5 == 1.0)
- `left_assoc_sub` — subtraction is left-associative (10-2-3 == 5)
- `left_assoc_sub_long` — long subtraction chain (2-3-4-5 == -10)
- `left_assoc_div` — division is left-associative (100/10/2 == 5)
- `left_assoc_div_long` — long division chain (64/4/2/2 == 4)
- `left_assoc_mixed` — mixed +/- left to right (1+2-3+4 == 4)
- `left_assoc_sub_then_mul` — left-assoc minus around a product (20-2-3*2 == 12)
- `right_assoc_pow` — exponent is right-associative (2^3^2 == 512)
- `right_assoc_pow2` — exponent right-assoc again (2^2^3 == 256)
- `right_assoc_pow_triple` — triple exponent right-assoc (2^2^2^2 == 65536)
- `right_assoc_pow_zero` — right-assoc with a zero exponent (4^3^0 == 4)
- `unary_pow_binding` — unary minus binds looser than ^ (-2^2 == -4)
- `unary_pow_even` — unary over even power (-2^4 == -16)
- `paren_unary_pow` — parens flip the binding ((-2)^2 == 4)
- `unary_simple` — plain unary minus (-3+5 == 2)
- `unary_group` — unary minus over a group (-(2+3) == -5)
- `pow_negative_exp` — negative exponent via unary (2^-1 == 0.5)
- `interaction_unary_right_pow` — unary + right-assoc ^ (-2^2^2 == -16)
- `interaction_sub_pow` — left-assoc minus around right-assoc power (1-2^3^0 == -1)
- `interaction_full` — all rules at once (10-2-3^1^2*2 == 2)
- `err_empty` — empty string raises CalcError
- `err_blank` — all-whitespace raises CalcError
- `err_trailing_op` — a trailing operator raises CalcError
- `err_unbalanced` — unbalanced parens raise CalcError
- `err_bad_char` — an unknown character raises CalcError
- `err_div_zero` — division by zero raises CalcError


### cronmatch · 48 checks

````
BUG REPORT — cronmatch: the test suite is failing

You have an existing Python package `cronmatch`, a tiny matcher that decides
whether a given datetime is "due" under a 5-field cron expression. It ships with
a unittest suite in `cronmatch/test_cronmatch.py`, and right now several of those
tests FAIL. Fix the code so that ALL the tests pass. Do not rewrite the package
from scratch and do not change its public API.

## Symptom

`matches(cron_expr, dt)` parses a 5-field cron expression and returns True iff
`dt` satisfies every field. Plain `* * * * *`, exact values, simple ranges and
comma lists already work, but several STEP and day-of-week cases come out wrong:

    from datetime import datetime
    from cronmatch.public import matches

    # `*/n` is off by the field's minimum (wrong for month / day-of-month):
    matches("0 0 1 */3 *", datetime(2026, 1, 1, 0, 0))
    #   EXPECTED True   (months are 1,4,7,10 — January is in the set)
    #   ACTUAL   False  (buggy set is 0,3,6,9,12 — counts from 0, misses Jan)

    # a stepped RANGE `a-b/n` ignores the step:
    matches("10-30/10 * * * *", datetime(2026, 6, 18, 0, 15))
    #   EXPECTED False  (minutes are 10,20,30 — 15 is not in the set)
    #   ACTUAL   True   (buggy treats it as the whole range 10..30)

    # when BOTH day-of-month and day-of-week are restricted, they should OR:
    matches("0 0 13 * 5", datetime(2026, 6, 19, 0, 0))   # Fri the 19th
    #   EXPECTED True   (it is a Friday, so the day-of-week clause fires)
    #   ACTUAL   False  (buggy AND-s the two clauses, demands the 13th too)

These defects interact: an expression that uses a stepped range or a stepped
month AND constrains both day-of-month and day-of-week exercises more than one at
once.

## Reproduce

Run the visible tests from the directory that contains the `cronmatch` package:

    python -m unittest cronmatch.test_cronmatch

## Contract (must hold after your fix)

* Package name stays `cronmatch`; import path `cronmatch` / `cronmatch.public`.
* Keep the public API exactly: `matches(cron_expr: str, dt: datetime) -> bool`
  and the `CronError` exception. Do not rename them.
* `cron_expr` is a string of EXACTLY 5 whitespace-separated fields, in order:

    minute  hour  day-of-month  month  day-of-week

  Any other field count raises `CronError`. A field that cannot be parsed (e.g.
  a non-integer token) also raises `CronError`.
* Field value ranges (inclusive):
    - minute        0–59
    - hour          0–23
    - day-of-month  1–31
    - month         1–12
    - day-of-week   0–6, where 0 = Sunday, 1 = Monday, ... 6 = Saturday.
* Each field is one or more comma-separated terms; a datetime matches the field
  if ANY term matches. A term is one of:
    - `*`        — every value in the field's range.
    - `v`        — the single integer `v`.
    - `a-b`      — every value from `a` to `b` inclusive.
    - `*/n`      — every nth value across the field's full range, STARTING AT THE
                   FIELD'S MINIMUM. So minute `*/15` = {0,15,30,45}; month `*/3` =
                   {1,4,7,10} (NOT {0,3,6,9,12}); day-of-month `*/10` =
                   {1,11,21,31} (NOT {0,10,20,30}).
    - `a-b/n`    — every nth value from `a` to `b` inclusive. So `10-30/10` =
                   {10,20,30}; hour `8-18/2` = {8,10,12,14,16,18}.
* A datetime matches the WHOLE expression iff its minute, hour and month each
  match their field AND the day-of-month / day-of-week pair matches per the rule
  below.
* DAY-OF-MONTH / DAY-OF-WEEK semantics (standard cron): if BOTH the
  day-of-month field and the day-of-week field are restricted (i.e. neither is a
  bare `*`), the day matches when the day-of-month clause matches OR the
  day-of-week clause matches — either one firing is enough. If only one of the
  two is restricted, only that one constrains the day. If both are `*`, every day
  matches.

Example:

    # day-of-month 13 OR day-of-week Friday(5):
    matches("0 0 13 * 5", datetime(2026, 6, 13, 0, 0))   # Sat the 13th  -> True (dom)
    matches("0 0 13 * 5", datetime(2026, 6, 19, 0, 0))   # Fri the 19th  -> True (dow)
    matches("0 0 13 * 5", datetime(2026, 2, 13, 0, 0))   # Fri the 13th  -> True (both)
    matches("0 0 13 * 5", datetime(2026, 6, 18, 0, 0))   # Thu the 18th  -> False

    # stepped range + stepped month + dom/dow OR all at once:
    matches("0 0 10-20/5 * 5", datetime(2026, 6, 15, 0, 0))  # 15 in {10,15,20} -> True
    matches("0 0 1 */3 5",     datetime(2026, 7, 3, 0, 0))   # Jul in {1,4,7,10}, Fri -> True

Standard library only (`datetime`). Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `every_minute` — plain * * * * * matches any datetime
- `exact_match_true` — exact minute+hour matches
- `exact_minute_false` — wrong minute fails an exact match
- `exact_hour_false` — wrong hour fails an exact match
- `list_minute_true` — comma list matches a listed minute
- `list_minute_false` — comma list rejects an unlisted minute
- `simple_range_in` — a-b range matches inside the range
- `simple_range_out` — a-b range rejects outside the range
- `month_field_true` — month field matches the right month
- `month_field_false` — month field rejects the wrong month
- `dow_only_true` — day-of-week-only rule matches that weekday
- `dow_only_false` — day-of-week-only rule rejects other weekdays
- `dom_only_true` — day-of-month-only rule matches that day
- `dom_only_false` — day-of-month-only rule rejects other days
- `dow_sunday_zero` — day-of-week 0 means Sunday
- `step_minute_min0_hit` — */15 on minute (min 0) matches 30
- `step_minute_min0_miss` — */15 on minute (min 0) rejects 31
- `step_month_q_jan` — month */3 includes January (1,4,7,10)
- `step_month_q_jul` — month */3 includes July
- `step_month_q_jun_miss` — month */3 excludes June
- `step_month_q_mar_miss` — month */3 excludes March
- `step_month_2_mar` — month */2 includes March, excludes February
- `step_month_2_feb_miss` — month */2 excludes February (1,3,5,...)
- `step_dom_10_hit` — day-of-month */10 includes the 11th (1,11,21,31)
- `step_dom_10_miss` — day-of-month */10 excludes the 10th
- `step_dom_10_first` — day-of-month */10 includes the 1st
- `step_dom_7_hit` — day-of-month */7 includes the 8th (1,8,15,22,29)
- `range_step_min_hit` — minute 10-30/10 matches 20
- `range_step_min_miss15` — minute 10-30/10 rejects 15
- `range_step_min_miss25` — minute 10-30/10 rejects 25
- `range_step_hour_hit` — hour 8-18/2 matches 14
- `range_step_hour_miss9` — hour 8-18/2 rejects 9
- `range_step_hour_miss11` — hour 8-18/2 rejects 11
- `range_step_dom_hit` — day-of-month 5-25/10 matches the 15th
- `range_step_dom_miss` — day-of-month 5-25/10 rejects the 10th
- `or_dow_fires` — 0 0 13 * 5: a Friday that is not the 13th still matches
- `or_dom_fires` — 0 0 13 * 5: the 13th that is not a Friday still matches
- `or_both` — 0 0 13 * 5: a Friday the 13th matches
- `or_neither` — 0 0 13 * 5: a non-13th non-Friday does not match
- `or_dow_wed_fires` — 0 0 1 * 3: a Wednesday that is not the 1st matches
- `or_dom_first_fires` — 0 0 1 * 3: the 1st that is not a Wednesday matches
- `ix_step_range_or_dom` — 10-20/5 dom OR Fri: the 15th fires via dom (needs step+OR)
- `ix_step_range_or_dow` — 10-20/5 dom OR Fri: a Friday off the set fires via dow
- `ix_step_range_or_none` — 10-20/5 dom OR Fri: neither -> no match
- `ix_step_month_or_dow` — */3 month + 1st-OR-Fri: July Friday fires (needs step-min+OR)
- `ix_step_month_or_dom` — */3 month + 1st-OR-Fri: April 1st fires (needs step-min+OR)
- `bad_field_count_raises` — a non-5-field expression raises CronError
- `bad_token_raises` — an unparseable field token raises CronError


### datespan · 25 checks

````
BUG REPORT — datespan: the test suite is failing

You have an existing Python package `datespan`, a tiny business-day date
calculator (Monday–Friday; there is NO holiday calendar — only weekends are
skipped). It ships with a unittest suite in `datespan/test_datespan.py`, and
right now several of those tests FAIL. Fix the code so that ALL the tests pass.
Do not rewrite the package from scratch and do not change its public API.

## Symptom

`add_business_days(start, n)` and `business_days_between(a, b)` get simple
within-week cases right, but several weekend / negative / reversed cases come
out wrong. (Weekday shown in comments: Jun 15 2026 = Mon, Jun 19 = Fri, Jun 20 =
Sat, Jun 21 = Sun, Jun 22 = next Mon.)

    from datetime import date
    from datespan.public import add_business_days, business_days_between

    # negative n steps the wrong way over a weekend:
    add_business_days(date(2026, 6, 15), -1)   # Mon, go back 1 business day
    #   EXPECTED 2026-06-12 (the previous Fri)
    #   ACTUAL   2026-06-15 (bounced forward off the weekend)

    # a weekend start is not normalized onto a business day:
    add_business_days(date(2026, 6, 20), 0)    # Sat, 0 business days
    #   EXPECTED 2026-06-22 (snap forward to Mon)
    #   ACTUAL   2026-06-20 (Saturday returned unchanged)

    # the reversed count drops its sign:
    business_days_between(date(2026, 6, 22), date(2026, 6, 15))  # Mon -> earlier Mon
    #   EXPECTED -5
    #   ACTUAL    5  (magnitude right, sign missing)

These defects interact: counting backward from a weekend start across a weekend
exercises all three at once.

## Reproduce

Run the visible tests from the directory that contains the `datespan` package:

    python -m unittest datespan.test_datespan

## Contract (must hold after your fix)

* Package name stays `datespan`; import path `datespan` / `datespan.public`.
* Keep the public API exactly: `add_business_days(start: date, n: int) -> date`
  and `business_days_between(a: date, b: date) -> int`. Do not rename them.
* Business days are Monday through Friday. Saturday and Sunday are NEVER
  business days. There is no holiday list.

* `add_business_days(start, n)`:
    - `start` is FIRST normalized onto a business day: a Saturday or Sunday
      `start` is moved FORWARD to the following Monday. That normalized date is
      the anchor for everything below, including `n == 0` and `n < 0`.
    - `n == 0` returns the normalized anchor itself.
    - `n > 0` advances the anchor by `n` business days, skipping weekends.
    - `n < 0` moves BACKWARD by `abs(n)` business days, skipping weekends — so
      stepping back across a weekend lands on the previous Friday, never on a
      Saturday/Sunday.

      Examples:
        add_business_days(date(2026, 6, 15), 5)  -> date(2026, 6, 22)  # Mon -> next Mon
        add_business_days(date(2026, 6, 19), 1)  -> date(2026, 6, 22)  # Fri -> Mon
        add_business_days(date(2026, 6, 15), -1) -> date(2026, 6, 12)  # Mon -> prev Fri
        add_business_days(date(2026, 6, 20), 1)  -> date(2026, 6, 23)  # Sat->Mon, then +1 = Tue

* `business_days_between(a, b)`:
    - Returns the number of business days in the HALF-OPEN span `(a, b]` —
      EXCLUSIVE of `a`, INCLUSIVE of `b`. Equivalently: how many business-day
      steps it takes to walk from `a` to `b`.
    - `a == b` returns `0`.
    - If `a` is AFTER `b` the result is NEGATIVE: it is the negation of the
      count from `b` to `a`. So `business_days_between(a, b) ==
      -business_days_between(b, a)`.
    - Weekends never count (a Friday-to-Saturday span is `0`).

      Examples:
        business_days_between(date(2026, 6, 15), date(2026, 6, 16)) -> 1   # Mon -> Tue
        business_days_between(date(2026, 6, 15), date(2026, 6, 22)) -> 5   # Mon -> next Mon
        business_days_between(date(2026, 6, 19), date(2026, 6, 20)) -> 0   # Fri -> Sat
        business_days_between(date(2026, 6, 22), date(2026, 6, 15)) -> -5  # reversed

These behaviors interact — the first business day before a weekend-anchored
start, or the reversed count of a weekend-crossing span, depends on getting all
of normalization, direction, and endpoint/sign right at once.

Standard library only (`datetime`). Do not change the package name or the
public function names.
````

**Graded behaviors:**

- `add_fwd_within_week` — Mon + small n stays inside the week
- `add_fwd_to_friday` — Mon + 4 lands on Fri
- `add_fwd_skips_weekend` — Mon + 5 skips the weekend to next Mon
- `add_fwd_friday_rolls` — Fri + 1 rolls to Mon
- `add_zero_on_business_day` — n == 0 on a weekday returns that day
- `add_sat_zero_normalizes` — Sat + 0 snaps forward to Mon
- `add_sun_zero_normalizes` — Sun + 0 snaps forward to Mon
- `add_sat_forward` — Sat + 1 anchors on Mon then steps to Tue
- `add_sun_forward2` — Sun + 2 anchors on Mon then steps to Wed
- `add_neg_within_week` — Fri - 1 = Thu (no weekend crossed)
- `add_neg_crosses_weekend` — Mon - 1 = previous Fri
- `add_neg_two` — Mon - 2 = previous Thu
- `add_neg_full_week` — Mon - 5 = the Monday a week earlier
- `add_neg_from_tuesday` — next Mon - 1 = the Friday before
- `btw_fwd_one` — Mon -> Tue is 1 business day
- `btw_fwd_four` — Mon -> Fri is 4 business days
- `btw_same_day_zero` — a == b is 0
- `btw_fwd_skips_weekend` — Mon -> next Mon is 5 (weekend excluded)
- `btw_fri_to_sat_zero` — Fri -> Sat is 0
- `btw_fwd_month_span` — Mon -> a Wed a month later
- `btw_reverse_one` — Tue -> Mon is -1
- `btw_reverse_week` — next Mon -> earlier Mon is -5
- `btw_reverse_month_span` — reversed month span is negative
- `interaction_add_sun_negative` — Sun start, normalize forward to Mon, then step back across the weekend
- `interaction_between_reverse_weekend` — reversed span across a weekend is the negated weekend-skipping count


### decimalfmt · 28 checks

````
BUG REPORT — decimalfmt: the test suite is failing

You have an existing Python package `decimalfmt`, a tiny money formatter that
works in integer cents. It ships with a unittest suite in
`decimalfmt/test_decimalfmt.py`, and right now several of those tests FAIL. Fix
the code so that ALL the tests pass. Do not rewrite the package from scratch and
do not change its public API.

## Symptom

`format_amount(cents, places, sep)` turns an integer number of cents into a
human-readable string, and `parse_amount(s, places, sep)` turns it back. Small
positive amounts come out right, but larger / negative / fractional cases are
wrong, and the round trip is broken:

    from decimalfmt.public import format_amount, parse_amount

    # the negative sign lands in the wrong place:
    format_amount(-1234567)
    #   EXPECTED "-12,345.67"
    #   ACTUAL   "123,45.-67"   (sign after the decimal, not in front)

    # the fractional part is not zero-padded:
    format_amount(5)
    #   EXPECTED "0.05"
    #   ACTUAL   "0.5"          (5 cents lost its leading zero)

    # thousands grouping is applied from the wrong end:
    format_amount(1234567)
    #   EXPECTED "12,345.67"
    #   ACTUAL   "123,45.67"    (grouped from the LEFT, not the right)

    # and the round trip does not survive the grouping separators:
    parse_amount("12,345.67")
    #   EXPECTED 1234567
    #   ACTUAL   ValueError     (the "," was never stripped)

These defects interact: a large NEGATIVE amount with a single-cent fractional
part, formatted and then parsed back, exercises all of them at once.

## Reproduce

Run the visible tests from the directory that contains the `decimalfmt` package:

    python -m unittest decimalfmt.test_decimalfmt

## Contract (must hold after your fix)

* Package name stays `decimalfmt`; import path `decimalfmt` / `decimalfmt.public`.
* Keep the public API exactly: `format_amount(cents: int, places: int = 2,
  sep: str = ",") -> str`, `parse_amount(s: str, places: int = 2,
  sep: str = ",") -> int`, and the `MoneyError` exception. Do not rename them.
* `format_amount(cents, places=2, sep=",")` renders an integer number of cents:
    - The amount is `cents / 10**places` units. The integer (whole-unit) part is
      `abs(cents) // 10**places`; the fractional part is `abs(cents) % 10**places`.
    - SIGN: a single leading `-` in FRONT of the whole number when `cents < 0`,
      and nothing when `cents >= 0`. The sign never appears after a separator or
      after the decimal point. Zero is non-negative ("0.00", no sign).
    - GROUPING: the integer part is split into groups of three digits counting
      FROM THE RIGHT, joined by `sep` (e.g. `1234567` whole-units ->
      "1,234,567"). Fewer than four integer digits means no separator at all.
    - FRACTION: exactly `places` digits after a single `.`, ZERO-PADDED on the
      left, so `5` cents -> ".05" and a whole-dollar amount -> ".00". When
      `places == 0` there is no `.` and no fractional part.
    - `cents` must be an `int` (bool is not accepted); a bad `cents` or a
      negative / non-int `places` raises `MoneyError`.
* `parse_amount(s, places=2, sep=",")` is the INVERSE of `format_amount` for the
  same `places` and `sep`:
    - It honours a single leading `-`, STRIPS every `sep` separator, splits on
      the `.`, pads / truncates the fractional run to `places` digits, and
      returns the integer number of cents.
    - `parse_amount(format_amount(c)) == c` for every int `c`.
* ROUND TRIP: formatting then parsing must recover the original cents exactly,
  INCLUDING the sign, the grouping separators, and zero-padded fractions.

Example:

    format_amount(-100000005)            # -> "-1,000,000.05"
    parse_amount("-1,000,000.05")        # -> -100000005
    format_amount(1234567, places=3)     # -> "1,234.567"
    format_amount(-12345, places=0)      # -> "-12,345"

Standard library only. Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `fmt_small_positive` — small positive amount renders correctly
- `fmt_under_thousand` — amount under $10 with two-digit frac is fine
- `fmt_zero` — zero renders as 0.00 with no sign
- `fmt_negative_lead_sign` — negative sign sits in front of the number
- `fmt_small_negative` — small negative keeps a leading sign
- `fmt_negative_round_thousands` — negative grouped amount, sign in front
- `fmt_pad_single_cent` — 5 cents -> .05 (leading zero kept)
- `fmt_pad_whole_dollar` — whole dollar -> .00
- `fmt_pad_nine_cents` — 9 cents -> .09
- `fmt_no_pad_needed` — two-digit frac is unchanged
- `fmt_group_thousands` — four integer digits -> one separator
- `fmt_group_millions` — millions grouped from the right
- `fmt_group_ten_million` — an 8-digit integer part groups correctly
- `fmt_no_group_three_digits` — exactly three integer digits -> no separator
- `fmt_places0` — places=0 drops the fractional part
- `fmt_places3` — places=3 keeps three padded fractional digits
- `fmt_custom_sep` — a custom separator is honoured
- `rt_grouped_positive` — grouped positive round-trips
- `rt_grouped_negative` — grouped negative round-trips
- `rt_single_cent` — single-cent amount round-trips
- `rt_zero` — zero round-trips
- `rt_places3` — places=3 round-trips
- `parse_grouped_literal` — parse strips separators from a literal string
- `parse_negative_literal` — parse honours a leading minus
- `interaction_neg_group_pad_rt` — negative + grouping + single-cent pad + round-trip together
- `interaction_exact_string` — the interaction renders the exact contract string and parses back
- `bad_cents_raises` — non-int cents raises MoneyError
- `bad_places_raises` — negative places raises MoneyError


### graphbip · 20 checks

````
BUG REPORT — graphbip: the test suite is failing

You have an existing Python package `graphbip`, a bipartite 2-coloring routine
for an undirected graph. It ships with a unittest suite in
`graphbip/test_graphbip.py`, and right now several of those tests FAIL. Fix the
code so that ALL the tests pass. Do not rewrite the package from scratch and do
not change its public API.

## Symptom

`two_color(graph)` tries to paint every node one of two colors (0 / 1) so that
no edge joins two same-colored nodes, returning the `{node: color}` map on
success or `None` when the graph is not bipartite. A single connected bipartite
graph already colors correctly, but several other cases come out wrong:

    from graphbip.public import two_color

    # only the first component gets colored:
    two_color({"a": {"b"}, "b": {"a"}, "c": {"d"}, "d": {"c"}})
    #   EXPECTED a full 0/1 map over {a, b, c, d}
    #   ACTUAL   {"a": 0, "b": 1}            (c and d never colored)

    # an odd cycle is wrongly reported as 2-colorable:
    two_color({"a": {"b", "c"}, "b": {"a", "c"}, "c": {"a", "b"}})
    #   EXPECTED None                        (triangle is not bipartite)
    #   ACTUAL   {"a": 0, "b": 1, "c": 1}    (b and c share an edge AND a color)

    # a self-loop is wrongly accepted:
    two_color({"a": {"a", "b"}, "b": {"a"}})
    #   EXPECTED None                        (a is adjacent to itself)
    #   ACTUAL   {"a": 0, "b": 1}            (the self-loop is ignored)

These defects interact: a graph whose FIRST component is a clean bipartite piece
but whose LATER component contains an odd cycle (or a self-loop) must still come
back `None` — you cannot decide bipartiteness from the first component alone.

## Reproduce

Run the visible tests from the directory that contains the `graphbip` package:

    python -m unittest graphbip.test_graphbip

## Contract (must hold after your fix)

* Package name stays `graphbip`; import path `graphbip` / `graphbip.public`.
* Keep the public API exactly: `two_color(graph: dict) -> dict | None` and the
  `GraphError` exception. Do not rename them.
* GRAPH REPRESENTATION: `graph` is an adjacency map `{node: neighbors}` where
  `neighbors` is an iterable (a set OR a list) of the nodes adjacent to `node`.
  The graph is UNDIRECTED: if `b` is in `graph[a]` then `a` is in `graph[b]`.
  Nodes can be any hashable (strings, ints, ...). The graph MAY be DISCONNECTED
  (several separate pieces) and may contain ISOLATED nodes whose neighbor
  iterable is empty. Every key of `graph` is a node that must appear in the
  result.
* SUCCESS: return a dict mapping EVERY node of `graph` to either `0` or `1` such
  that for every edge `(u, v)`, `color[u] != color[v]`. The empty graph `{}`
  returns an empty dict `{}`. The specific colors are not fixed (any valid
  2-coloring is accepted), only that the assignment is complete and conflict-free.
* FAILURE: return `None` when the graph is NOT bipartite, i.e. it contains an
  odd-length cycle. Two cases to handle:
    - a SELF-LOOP (`node in graph[node]`) — a node adjacent to itself is an odd
      cycle of length 1 and is never 2-colorable;
    - any other ODD CYCLE — e.g. a triangle a-b-c-a. The decision is over the
      WHOLE graph: if ANY component is non-bipartite the answer is `None`, even
      if earlier components colored cleanly.
* A passing argument that is not a dict raises `GraphError`.

Example:

    # first component a-b is fine; second component c-d-e is a triangle:
    two_color({"a": {"b"}, "b": {"a"},
               "c": {"d", "e"}, "d": {"c", "e"}, "e": {"c", "d"}})
    #   -> None     (the graph as a whole is not bipartite)

Standard library only. Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `single_edge` — a single edge a-b is 2-colorable
- `even_path` — a-b-c-d (even path) is 2-colorable
- `even_cycle_4` — a 4-cycle is 2-colorable
- `list_adjacency` — neighbor iterables may be lists, not just sets
- `empty_graph` — the empty graph returns an empty coloring
- `two_components` — two separate edges -> all four nodes colored
- `isolated_node` — an isolated node is its own component and must be colored
- `three_components_mixed` — three components (edge, path, isolated) all colored
- `all_isolated` — a graph of only isolated nodes colors every node
- `two_components_one_isolated` — edge + bigger path + isolated all colored
- `triangle` — a triangle (3-cycle) is not bipartite
- `five_cycle` — a 5-cycle (odd) is not bipartite
- `odd_cycle_with_tail` — an odd cycle with a pendant tail is not bipartite
- `self_loop_with_edge` — a self-loop on a connected node -> not bipartite
- `self_loop_isolated` — a self-loop on an otherwise isolated node -> not bipartite
- `good_then_triangle` — clean first component, odd-cycle later -> None
- `good_then_self_loop` — clean first component, self-loop later -> None
- `triangle_then_good` — odd-cycle FIRST component still -> None
- `two_good_then_self_loop` — two clean components then a self-loop -> None
- `non_dict_raises` — a non-dict graph raises GraphError


### jsonquery · 22 checks

````
BUG REPORT — jsonquery: the test suite is failing

You have an existing Python package `jsonquery`, a tiny JSONPath-lite selector.
It ships with a unittest suite in `jsonquery/test_jsonquery.py`, and right now
several of those tests FAIL. Fix the code so that ALL the tests pass. Do not
rewrite the package from scratch and do not change its public API.

## Symptom

`select(obj, path)` walks a path expression over a nested dict/list structure
and is supposed to return a flat list of every value the path selects, in
document order. Simple key chains like `.a.b` already work, but the wildcard,
the recursive descent, and the missing-step behavior all come out wrong:

    from jsonquery.public import select

    doc = {
        "users": [
            {"name": "ada",   "id": 1, "roles": [{"id": 10}, {"id": 11}]},
            {"name": "linus", "id": 2, "roles": [{"id": 20}]},
        ],
        "owner": {"name": "grace", "id": 3},
    }

    # [*] is supposed to fan out to each element, flat:
    select(doc, ".users[*].name")
    #   EXPECTED ["ada", "linus"]
    #   ACTUAL   raises / wrong — [*] keeps the list nested instead of fanning out

    # ..key is recursive descent over EVERYTHING, top-down, lists included:
    select(doc, "..id")
    #   EXPECTED [1, 10, 11, 2, 20, 3]
    #   ACTUAL   [10, 11, 1, 20, 2, 3]-ish — wrong order, and ids inside list
    #            elements get missed entirely

    # a missing key must RAISE, not quietly vanish:
    select(doc, ".owner.missing")
    #   EXPECTED raises SelectError
    #   ACTUAL   returns []   (the mismatch is silently swallowed)

These defects interact: a path like `.users[*].missing` fans out with `[*]`
and THEN hits a missing key on each branch, and `.users[*]..id` fans out and
THEN recurses — so getting the combined cases right needs all three fixed.

## Reproduce

Run the visible tests from the directory that contains the `jsonquery` package:

    python -m unittest jsonquery.test_jsonquery

## Contract (must hold after your fix)

* Package name stays `jsonquery`; import path `jsonquery` / `jsonquery.public`.
* Keep the public API exactly: `select(obj, path: str) -> list` and the
  `SelectError` exception. Do not rename them.
* A `path` is a sequence of steps written together. The steps are:
    - `.key`    — descend into the mapping value at `key`. Keying a non-mapping,
      or a `key` that is absent, raises `SelectError`.
    - `[index]` — index into a list with a non-negative integer. Indexing a
      non-list, or an out-of-range index, raises `SelectError`.
    - `[*]`     — fan out to EVERY element of a list, in order, FLAT: it adds
      each element to the result frontier (it does NOT keep the list nested).
      `[*]` on a non-list raises `SelectError`.
    - `..key`   — recursive descent: collect the value under `key` EVERYWHERE at
      or below the current node, in PRE-ORDER (a node is recorded before its
      own children are scanned), and the scan descends through BOTH mapping
      values AND list elements. `..key` never raises for "not found" — an empty
      result is legal.
  A leading `.` / `..` is optional sugar: `users[0]` and `.users[0]` are the
  same; a leading bare `key` means `.key`.
* `select` ALWAYS returns a `list` (never a scalar), even for a single match
  (`.owner.name` -> `["grace"]`) and even when empty (`..nope` -> `[]`).
* Order is document order: list elements left to right, mapping values in
  insertion order, and for `..key` a node's own match comes before the matches
  found by recursing into that node.

Example (the full interaction):

    select(doc, ".users[*]..id")
    #   -> [1, 10, 11, 2, 20]
    # fan out the two users, then for each collect every id at/under it in
    # pre-order (the user's own id, then its roles' ids).

Standard library only. Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `simple_key_chain` — single match for a plain key chain
- `simple_returns_list` — a single match is still wrapped in a list
- `index_then_key` — index a list then descend a key
- `leading_bare_key` — a leading bare key is sugar for .key
- `deep_key_chain` — a deep plain key chain resolves
- `wildcard_terminal` — terminal [*] returns the elements flat, in order
- `wildcard_then_key` — [*] fans out, then .key maps over each element
- `wildcard_then_key_ids` — [*] then .id over each element
- `wildcard_then_index` — [*] fans out, then [index] into each sub-list
- `wildcard_double` — [*] after [*] fans out two levels, flat
- `descend_ids_preorder` — ..id is pre-order over dicts AND lists
- `descend_names` — ..name collects names top-down
- `descend_nested_preorder` — ..id pre-order through nested dicts + lists
- `descend_from_subtree` — .key then ..id scans only that subtree
- `descend_no_match_empty` — ..key with no match returns [] (never raises)
- `missing_key_raises` — a missing mapping key raises SelectError
- `index_out_of_range_raises` — an out-of-range index raises SelectError
- `key_of_non_mapping_raises` — keying a non-mapping raises SelectError
- `index_non_list_raises` — indexing a non-list raises SelectError
- `wildcard_then_missing_raises` — [*] fan-out then a missing key surfaces SelectError
- `wildcard_then_descend` — [*] fan-out then ..id pre-order into each element
- `key_wildcard_key` — .roles[*].id over a fanned-out user


### permgen · 27 checks

````
BUG REPORT — permgen: the test suite is failing

You have an existing Python package `permgen`, a tiny utility for the
lexicographic permutations of a list of DISTINCT items. It ships with a unittest
suite in `permgen/test_permgen.py`, and right now several of those tests FAIL.
Fix the code so that ALL the tests pass. Do not rewrite the package from scratch
and do not change its public API.

## Symptom

The package addresses permutations by RANK: list every permutation of `items` in
lexicographic order, numbered from 0. `nth_permutation(items, n)` is supposed to
return the rank-`n` permutation, and `permutation_rank(perm, items)` is supposed
to be its exact inverse. The identity (rank 0) works, but almost everything else
is wrong:

    from permgen.public import nth_permutation, permutation_rank

    # rank 0 is fine — the items in their given order:
    nth_permutation(["a", "b", "c", "d"], 0)
    #   EXPECTED ['a', 'b', 'c', 'd']
    #   ACTUAL   ['a', 'b', 'c', 'd']   (correct)

    # but other ranks decode the wrong permutation:
    nth_permutation([1, 2, 3], 4)
    #   EXPECTED [3, 1, 2]
    #   ACTUAL   [1, 3, 2]              (decodes the wrong digits)

    # and rank is not the inverse of nth_permutation:
    permutation_rank(["b", "a", "c", "d"], ["a", "b", "c", "d"])
    #   EXPECTED 6
    #   ACTUAL   11                     (counts against the wrong remaining set)

These defects interact: a round-trip `permutation_rank(nth_permutation(items,
n), items)` should give back `n` for every `n`, but right now it does not — you
have to fix all three to make the two functions exact inverses again.

## Reproduce

Run the visible tests from the directory that contains the `permgen` package:

    python -m unittest permgen.test_permgen

## Contract (must hold after your fix)

* Package name stays `permgen`; import path `permgen` / `permgen.public`.
* Keep the public API exactly: `nth_permutation(items, n) -> list`,
  `permutation_rank(perm, items) -> int`, and the `PermError` exception. Do not
  rename them.
* `items` is a sequence of DISTINCT items, taken in the given order as the
  lexicographic alphabet (so `items` itself is the smallest permutation, rank 0).
* `nth_permutation(items, n)`:
    - `n` is a 0-indexed rank with `0 <= n < len(items)!`; an `n` outside that
      range (or a non-int `n`) raises `PermError`. `nth_permutation([], 0)`
      returns `[]`.
    - Returns a NEW list (the input `items` is not mutated).
    - `nth_permutation(items, 0)` is always `list(items)` (the identity).
    - Permutations are enumerated in LEXICOGRAPHIC order by the position of each
      item in `items`. The decoding is FACTORADIC: the digit for the step with
      `i` items still available has place value `(i-1)!` and is an index into the
      list of REMAINING (not-yet-used) items.
* `permutation_rank(perm, items)`:
    - `perm` must be a permutation of `items` (a `perm` whose length differs from
      `items`, or that contains an item not in `items`, raises `PermError`).
    - Returns the 0-indexed rank, so `permutation_rank(list(items), items) == 0`.
    - It is the EXACT INVERSE of `nth_permutation`: for every valid `n`,
      `permutation_rank(nth_permutation(items, n), items) == n`, and for every
      permutation `perm`, `nth_permutation(items, permutation_rank(perm, items))
      == list(perm)`.

Example (the two are exact inverses):

    items = ["a", "b", "c", "d"]
    nth_permutation(items, 5)            # -> ['a', 'd', 'c', 'b']
    permutation_rank(['a', 'd', 'c', 'b'], items)   # -> 5
    nth_permutation(items, 23)           # -> ['d', 'c', 'b', 'a']  (the last one)
    permutation_rank(['d', 'c', 'b', 'a'], items)   # -> 23

Standard library only (`math`). Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `identity_nth_4` — nth(items, 0) is the identity (4 items)
- `identity_nth_3` — nth(items, 0) is the identity (3 items)
- `identity_nth_5` — nth(items, 0) is the identity (5 items)
- `nth_4_r1` — nth rank 1 of 4 items
- `nth_4_r5` — nth rank 5 of 4 items
- `nth_4_r11` — nth rank 11 of 4 items
- `nth_4_r23_last` — nth last rank (fully reversed) of 4 items
- `nth_4_r2` — nth rank 2 of 4 items
- `nth_4_r17` — nth rank 17 of 4 items
- `nth_3_r4` — nth rank 4 of 3 items
- `nth_5_r50` — nth rank 50 of 5 items
- `nth_5_r119_last` — nth last rank of 5 items
- `nth_enumerate_4` — nth decodes every rank of 4 items correctly
- `rank_identity_4` — rank of the identity is 0 (4 items)
- `rank_4_a` — rank of ['a','b','d','c']
- `rank_4_b` — rank of ['b','a','c','d']
- `rank_4_c` — rank of ['c','a','b','d']
- `rank_4_last` — rank of the fully reversed 4-item list
- `rank_3` — rank of [3,1,2] over [1,2,3]
- `rank_enumerate_4` — rank assigns the right index to every permutation of 4 items
- `rank_identity_5` — rank of the identity is 0 (5 items)
- `round_trip_nth_then_rank` — rank(nth(items, n)) == n for all n (4 items)
- `round_trip_rank_then_nth` — nth(items, rank(perm)) == perm for all perms (4 items)
- `round_trip_5` — rank(nth(items, 77)) == 77 (5 items)
- `nth_out_of_range_raises` — n >= len(items)! raises PermError
- `nth_empty_list` — nth([], 0) returns []
- `rank_length_mismatch_raises` — perm length != items raises PermError


### repaircalc · 31 checks

````
BUG REPORT — repaircalc: the test suite is failing

You have an existing Python package `repaircalc`, a small arithmetic expression
evaluator. It ships with a unittest suite in `repaircalc/test_repaircalc.py`, and
right now several of those tests FAIL. Fix the code so that ALL the tests pass.
Do not rewrite the package from scratch and do not change its public API.

## Symptom

`evaluate(expr)` is supposed to parse and evaluate ordinary arithmetic, but it
gets several expressions wrong:

    from repaircalc.public import evaluate

    evaluate("2+3*4")     # EXPECTED 14   ... ACTUAL 20   (precedence wrong)
    evaluate("10-3-2")    # EXPECTED 5    ... ACTUAL 9    (associativity wrong)
    evaluate("3.5+1.5")   # EXPECTED 5.0  ... ACTUAL 6    (decimals mishandled)

Simple cases like `evaluate("2+2") == 4`, parentheses, and division by a nonzero
value already work — only some precedence / associativity / decimal cases are
broken.

## Reproduce

Run the visible tests from the directory that contains the `repaircalc` package:

    python -m unittest repaircalc.test_repaircalc

## Contract (must hold after your fix)

* Package name stays `repaircalc`; import path `repaircalc` / `repaircalc.public`.
* Keep the public API exactly: `evaluate(expr: str) -> number` and the
  `CalcError` exception. Do not rename them.
* `evaluate` supports the binary operators `+`, `-`, `*`, `/`, parentheses for
  grouping, a leading unary `-`/`+`, and integer and decimal literals
  (e.g. `3`, `3.5`, `.5`).
* Standard precedence: `*` and `/` bind TIGHTER than `+` and `-`.
* All binary operators are LEFT-associative:
  `10 - 3 - 2 == 5` and `100 / 10 / 2 == 5`.
* Decimal literals keep their fractional value: `3.5 + 1.5 == 5.0`,
  `.5 + .5 == 1.0`.
* Division by zero raises `CalcError` (it must not crash with some other error).
* Do NOT use Python's built-in `eval` (or `exec`/`ast.literal_eval` tricks) —
  keep the hand-written parser. Standard library only.

Do not change the package name or the public function/exception names.
````

**Graded behaviors:**

- `basic_add` — 2+2 == 4
- `basic_sub` — 9-4 == 5
- `basic_single_number` — a bare literal evaluates to itself
- `basic_unary_minus` — leading unary minus negates
- `prec_add_then_mul` — 2+3*4 == 14 (mul binds tighter)
- `prec_mul_then_add` — 4*2+1 == 9
- `prec_two_products` — 2*3+4*5 == 26
- `prec_mixed_add_sub_mul` — 2 + 3 * 4 - 1 == 13
- `prec_div_in_sum` — 10 + 8/2 == 14
- `prec_sub_then_mul` — 20 - 2*3 == 14
- `assoc_sub_simple` — 10-3-2 == 5 (left assoc)
- `assoc_sub_chain` — 20-5-3-1 == 11
- `assoc_add_sub_mix` — 1+2-3+4 == 4
- `assoc_add_sub_mix2` — 12-4+2 == 10
- `assoc_div_chain` — 100/10/2 == 5
- `assoc_div_chain2` — 64/4/2 == 8
- `assoc_mul_div_lr` — 6/2*3 == 9
- `paren_group_add` — (2+3)*4 == 20
- `paren_inner_expr` — 2*(3+4) == 14
- `paren_nested` — ((1+2)*(3+4)) == 21
- `paren_over_sub` — (10-3)-2 == 5
- `paren_changes_sub_assoc` — 10-(3-2) == 9
- `paren_unary` — -(3+4) == -7
- `dec_add` — 3.5+1.5 == 5.0
- `dec_leading_dot` — .5+.5 == 1.0
- `dec_mul` — 2.5*4 == 10.0
- `dec_div` — 10/4 == 2.5
- `dec_precision` — 0.1+0.2 ≈ 0.3
- `dec_mixed` — 1.5*2+0.5 == 3.5
- `div_by_zero_raises` — division by zero raises CalcError
- `no_eval_used` — implementation does not use eval/exec/literal_eval


### repairmoney · 14 checks

````
BUG REPORT — repairmoney: the test suite is failing, fix the code so all tests pass

You have an existing Python package `repairmoney` (tiny money helpers over
integer cents). It LOOKS done, but its bundled test suite
(`repairmoney/test_repairmoney.py`) is RED. Run it, find the bugs, and fix the
code in `repairmoney/public.py` so that every test passes. Keep the public API
exactly as it is; do not rewrite the package from scratch and do not edit the
tests.

## Reproduction

    python -m pytest repairmoney/test_repairmoney.py
    # or, with no pytest installed:
    python repairmoney/test_repairmoney.py

Several tests fail. They point at money being rendered with the sign in the
wrong place, cents that are not zero-padded, and an "even" split whose parts do
not add back up to the original total.

## The two helpers

* `format_cents(cents: int) -> str`
  Renders an amount given as a whole number of integer cents (so $12.34 is the
  int `1234`; negative amounts are debts/refunds) as a `$D.DD` string:

      format_cents(1234)   ->  "$12.34"
      format_cents(-1234)  ->  "-$12.34"     # minus IN FRONT of the $
      format_cents(5)      ->  "$0.05"       # cents zero-padded to 2 digits
      format_cents(0)      ->  "$0.00"

* `split_evenly(cents: int, n: int) -> list[int]`
  Splits a total of `cents` into `n` integer-cent parts. The leftover cents
  (the remainder after the even base share) are distributed one per part to the
  earliest parts, so the parts ALWAYS sum back to the original `cents`:

      split_evenly(1000, 3)  ->  [334, 333, 333]   # sums to 1000
      split_evenly(1000, 4)  ->  [250, 250, 250, 250]
      split_evenly(100, 1)   ->  [100]

## Contract (must hold after your fix)

* Package name stays `repairmoney`; import path `repairmoney` /
  `repairmoney.public`. Keep the public names `format_cents` and `split_evenly`
  and their signatures.
* `format_cents`:
  - The minus sign for a negative amount goes IN FRONT of the `$`
    (`"-$12.34"`, never `"$-12.34"`).
  - The cents are ALWAYS two digits, zero-padded (`5` cents -> `".05"`).
  - `0` formats as `"$0.00"`.
* `split_evenly`:
  - Returns a list of exactly `n` integer cents.
  - The parts MUST sum EXACTLY to the input `cents`, for any `n >= 1`, whether
    or not `cents` divides evenly by `n` (no cent may be lost or duplicated).
  - The remainder cents are distributed one each to the earliest parts, so the
    parts differ from one another by at most one cent.

Do not change the package name or the public function names. Do not edit the
test file.
````

**Graded behaviors:**

- `fmt_positive` — format_cents renders a positive amount as $D.DD
- `fmt_zero` — format_cents renders 0 as $0.00
- `fmt_negative_sign_in_front` — a negative amount puts the minus sign in front of the $ (-$12.34)
- `fmt_pads_cents` — single-digit cents are zero-padded to two digits
- `fmt_negative_small_cents` — a small negative amount renders -$0.05
- `fmt_battery` — format_cents matches the oracle across many amounts
- `fmt_large` — a large amount renders all the dollar digits
- `split_divisible` — a divisible split yields n equal parts summing to total
- `split_non_divisible_sums_to_total` — a non-divisible split's parts sum EXACTLY to the total
- `split_remainder_distributed` — the remainder cent goes to the earliest part(s), parts within one cent
- `split_n_one` — split_evenly(total, 1) returns [total]
- `split_sum_invariant_battery` — across many (cents, n), parts have length n and sum to cents
- `split_negative_total` — a negative total splits into parts summing to it
- `split_large_conserves` — a large awkward split conserves every cent


### repairpager · 12 checks

````
BUG REPORT — repairpager: the test suite is failing

You have an existing Python package `repairpager`, a small pagination helper
that slices a list of items into pages. It ships with a visible test file,
`repairpager/test_repairpager.py`, and that test suite is currently FAILING.
Fix the code so all the tests pass. Do not rewrite the package from scratch and
do not change the public API or the return shape.

## How to run the tests

    python -m pytest repairpager/test_repairpager.py
    # or, if pytest is not available:
    python repairpager/test_repairpager.py

## Symptom

`paginate(items, page_size, page)` is returning wrong results: pages contain the
wrong items, the reported page count drops a trailing partial page, and the last
page incorrectly claims another page follows it. The visible tests pin down the
intended behaviour — make them green.

## Contract (must hold after your fix)

* Package name stays `repairpager`; import path `repairpager` / `repairpager.public`.
* Keep the public API exactly as it is:
  `paginate(items, page_size, page=1) -> dict`.
* Keep the return shape — a dict with these keys:
  - `items`        : list of the items on this page, in original order (may be empty)
  - `total_items`  : total number of input items
  - `total_pages`  : number of pages needed to cover ALL items (round UP so a
                     trailing partial page still counts; an empty input is one page)
  - `page`         : the 1-based page actually returned (clamped into range)
  - `has_next`     : True iff a page after this one exists
  - `has_prev`     : True iff a page before this one exists
* Page numbering is 1-based: `page=1` returns the FIRST `page_size` items
  (items index 0 .. page_size-1), `page=2` the next chunk, and so on.
* Slicing must be correct: page `p` returns items in the half-open index range
  `[(p-1)*page_size, p*page_size)`.
* `total_pages` must use a ceiling division: 5 items in chunks of 2 is 3 pages,
  not 2. An exact multiple (e.g. 6 items, size 3) is exactly that many pages.
* `has_next` is False on the last page; `has_prev` is False on the first page.
* `page_size` must be a positive integer; reject otherwise (the existing
  `ValueError` behaviour is fine — do not weaken it).

Do not change the package name or the public function name. Standard library only.
````

**Graded behaviors:**

- `first_page_items` — page 1 returns the first page_size items in order
- `middle_page_items` — an interior page returns the correct chunk
- `last_partial_page_items` — the trailing partial page holds the leftover items
- `total_pages_ceil_partial` — total_pages rounds up for a trailing partial page
- `total_pages_exact_multiple` — an exact multiple yields exactly that many pages
- `total_items_count` — total_items equals the number of input items
- `flags_first_page` — first page: has_next True, has_prev False
- `has_next_false_last_page` — last page: has_next False, has_prev True
- `flags_middle_page` — interior page: has_next and has_prev both True
- `single_full_page` — a single full page has no next or prev
- `empty_input` — empty input is a single empty page with no navigation
- `out_of_range_page_clamped` — a too-large page clamps to the last real page


### repairspans · 18 checks

````
BUG REPORT — repairspans: the test suite is failing — fix the code so all tests pass.

You have an existing Python package `repairspans`, a tiny library for working with
closed integer intervals. It ships with a test suite in the package
(`repairspans/test_repairspans.py`) that is currently FAILING. There are a few
planted bugs in the implementation. Find them and fix the code so that every test
passes. Do NOT edit the tests, and do not rewrite the package from scratch — keep
the public API exactly as it is.

## Symptom

Run the visible tests from the workspace root:

    python -m repairspans.test_repairspans

Several tests fail. The failures all stem from the handling of CLOSED intervals
(intervals that include both endpoints) and from the assumption that input is
already sorted. For example:

  * `merge([[1, 2], [2, 3]])` should be `[[1, 3]]` (the intervals touch at the
    point 2 and must collapse), but the buggy code returns `[[1, 2], [2, 3]]`.
  * `merge([[3, 4], [1, 2]])` (unsorted input) should be `[[1, 2], [3, 4]]`, but
    the buggy code mishandles the ordering.
  * `overlaps([1, 2], [2, 3])` should be `True` (they share the point 2), but the
    buggy code returns `False`.

## Contract (must hold after your fix)

* Package name stays `repairspans`; import path `repairspans` / `repairspans.public`.
* Keep the existing public API and its names — two functions, exported from both
  `repairspans` and `repairspans.public`:
  - `merge(intervals) -> list[list[int]]`
  - `overlaps(a, b) -> bool`
* Intervals are `[start, end]` two-element sequences with `start <= end`, and they
  are CLOSED: both endpoints are included.
* TOUCHING / ADJACENT BEHAVIOR (define it exactly this way): because the intervals
  are closed, two intervals that touch at a single shared endpoint count as
  overlapping. So `[1, 2]` and `[2, 3]` overlap, and `merge` collapses them into
  `[1, 3]`. Intervals that do NOT share a point (e.g. `[1, 2]` and `[3, 4]`,
  whose nearest endpoints differ by more than 0) stay separate.
* `overlaps(a, b)` returns `True` iff the two closed intervals share at least one
  point (nesting and identical intervals count as overlapping; touching counts;
  fully disjoint does not). It is symmetric: `overlaps(a, b) == overlaps(b, a)`.
* `merge(intervals)`:
  - returns a NEW list (does not mutate the input or its interval objects),
  - is order-independent: it must produce the correct result for UNSORTED input,
  - returns intervals sorted by start, with no two results overlapping or touching,
  - returns `[]` for empty input, and `[[s, e]]` (a copy) for a single interval.

Use only the Python standard library. Do not change the package name or the
public function names.
````

**Graded behaviors:**

- `merge_empty` — merge([]) returns []
- `merge_single` — merge of one interval returns that interval
- `merge_disjoint` — disjoint intervals stay separate
- `merge_overlapping` — overlapping intervals merge into one
- `merge_touching_adjacent` — touching closed intervals merge (off-by-one)
- `merge_unsorted_input` — unsorted input is handled correctly
- `merge_unsorted_overlap` — unsorted overlapping intervals merge
- `merge_nested` — a nested interval is absorbed without shrinking
- `merge_nested_unsorted` — nested interval absorbed regardless of order
- `merge_chain` — a touching chain collapses to one span
- `merge_duplicates` — identical intervals collapse to one
- `merge_no_mutation` — merge does not mutate its input
- `overlaps_overlap` — overlapping intervals report True
- `overlaps_touch` — touching closed intervals report True
- `overlaps_disjoint` — disjoint intervals report False
- `overlaps_nested` — a nested interval overlaps its container
- `overlaps_identical` — identical intervals overlap
- `overlaps_symmetric` — overlaps is symmetric in its arguments


### romanio · 27 checks

````
BUG REPORT — romanio: the test suite is failing

You have an existing Python package `romanio`, a tiny Roman-numeral converter.
It ships with a unittest suite in `romanio/test_romanio.py`, and right now
several of those tests FAIL. Fix the code so that ALL the tests pass. Do not
rewrite the package from scratch and do not change its public API.

## Symptom

`to_roman(n)` renders an integer as a Roman numeral and `from_roman(s)` parses
one back. Simple additive numerals already work, but the subtractive cases and
the range check are wrong:

    from romanio.public import to_roman, from_roman

    # to_roman spells the subtractive values additively:
    to_roman(4)      # EXPECTED "IV"      ACTUAL "IIII"
    to_roman(9)      # EXPECTED "IX"      ACTUAL "VIIII"
    to_roman(40)     # EXPECTED "XL"      ACTUAL "XXXX"
    to_roman(900)    # EXPECTED "CM"      ACTUAL "DCCCC"
    to_roman(1994)   # EXPECTED "MCMXCIV" ACTUAL "MDCCCCLXXXXIIII"

    # from_roman just sums symbols, ignoring subtractive pairs:
    from_roman("IV")      # EXPECTED 4    ACTUAL 6   (I + V)
    from_roman("IX")      # EXPECTED 9    ACTUAL 11
    from_roman("MCMXCIV") # EXPECTED 1994 ACTUAL ...wrong

    # to_roman accepts out-of-range values instead of refusing them:
    to_roman(0)      # EXPECTED RomanError   ACTUAL ""
    to_roman(4000)   # EXPECTED RomanError   ACTUAL "MMMM"

These defects interact: the round trip `from_roman(to_roman(n)) == n` only
holds for subtractive values (4, 9, 40, ..., 1994) once BOTH directions agree
on subtractive notation, and the range guard is what keeps `to_roman` honest at
the 1 and 3999 boundaries.

## Reproduce

Run the visible tests from the directory that contains the `romanio` package:

    python -m unittest romanio.test_romanio

## Contract (must hold after your fix)

* Package name stays `romanio`; import path `romanio` / `romanio.public`.
* Keep the public API exactly: `to_roman(n: int) -> str`,
  `from_roman(s: str) -> int`, and the `RomanError` exception. Do not rename
  them.
* `to_roman(n)` accepts an int with `1 <= n <= 3999`. Anything outside that
  range (including 0 and negatives) raises `RomanError`. A non-int raises
  `RomanError`.
* `from_roman(s)` accepts a numeral string (case-insensitive, surrounding
  whitespace ignored) and returns its integer value. An empty string, or a
  string containing a symbol that is not one of `M D C L X V I`, raises
  `RomanError`.
* SUBTRACTIVE notation is REQUIRED in both directions. The six subtractive
  pairs are:

      4   -> IV        40  -> XL        400 -> CD
      9   -> IX        90  -> XC        900 -> CM

  So `to_roman` must emit `IV` (not `IIII`), `IX` (not `VIIII`), `XL`, `XC`,
  `CD`, `CM`; and `from_roman` must read a smaller symbol placed before a
  larger one as a subtraction (`IV` = 5 - 1 = 4, `CM` = 1000 - 100 = 900).
* Everything else is additive, written largest-symbol-first (e.g. 2026 ->
  `MMXXVI`, 38 -> `XXXVIII`).
* ROUND TRIP: `from_roman(to_roman(n)) == n` for every `n` in `1..3999`.

Example:

    to_roman(1994)             # -> "MCMXCIV"
    from_roman("MCMXCIV")      # -> 1994
    to_roman(2949)             # -> "MMCMXLIX"
    from_roman("MMCMXLIX")     # -> 2949

Standard library only. Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `to_simple_i_to_iii` — to_roman renders 1,2,3 as I,II,III
- `to_additive_vi_viii` — to_roman renders 6 -> VI, 8 -> VIII
- `to_additive_thirtyeight` — to_roman renders 38 -> XXXVIII
- `from_additive_basics` — from_roman parses III,VI,XXX additively
- `to_sub_four` — to_roman(4) -> IV (not IIII)
- `to_sub_nine` — to_roman(9) -> IX (not VIIII)
- `to_sub_forty` — to_roman(40) -> XL
- `to_sub_ninety` — to_roman(90) -> XC
- `to_sub_four_hundred` — to_roman(400) -> CD
- `to_sub_nine_hundred` — to_roman(900) -> CM
- `to_sub_composite` — to_roman(1994) -> MCMXCIV (many subtractive pairs)
- `to_sub_2949` — to_roman(2949) -> MMCMXLIX
- `from_sub_four` — from_roman('IV') -> 4 (not 6)
- `from_sub_nine` — from_roman('IX') -> 9
- `from_sub_forty_ninety` — from_roman('XL'),('XC') -> 40,90
- `from_sub_cd_cm` — from_roman('CD'),('CM') -> 400,900
- `from_sub_composite` — from_roman('MCMXCIV') -> 1994
- `from_case_insensitive` — from_roman lowercases input ('mcmxciv' -> 1994)
- `round_trip_full_range` — from_roman(to_roman(n)) == n for all 1..3999
- `to_zero_raises` — to_roman(0) raises RomanError
- `to_negative_raises` — to_roman(-5) raises RomanError
- `to_too_big_raises` — to_roman(4000) raises RomanError
- `to_way_too_big_raises` — to_roman(10000) raises RomanError
- `to_boundary_one` — to_roman(1) -> I (lower boundary stays valid)
- `to_boundary_3999` — to_roman(3999) -> MMMCMXCIX (upper boundary stays valid)
- `from_bad_symbol_raises` — from_roman('IZ') raises RomanError
- `from_empty_raises` — from_roman('') raises RomanError


### rrulelite · 20 checks

````
BUG REPORT — rrulelite: the test suite is failing

You have an existing Python package `rrulelite`, a tiny recurrence-rule expander.
It ships with a unittest suite in `rrulelite/test_rrulelite.py`, and right now
several of those tests FAIL. Fix the code so that ALL the tests pass. Do not
rewrite the package from scratch and do not change its public API.

## Symptom

`expand(rule, start, limit)` walks a recurrence rule forward from `start` and is
supposed to return the dates it generates. Daily and weekly rules already work,
but several monthly / boundary cases come out wrong:

    from datetime import date
    from rrulelite.public import expand

    # interval is ignored for monthly rules:
    expand({"freq": "monthly", "interval": 2}, date(2026, 1, 10), 3)
    #   EXPECTED [2026-01-10, 2026-03-10, 2026-05-10]
    #   ACTUAL   [2026-01-10, 2026-02-10, 2026-03-10]   (steps 1 month, not 2)

    # month-end overflow spills instead of clamping:
    expand({"freq": "monthly"}, date(2026, 1, 31), 3)
    #   EXPECTED [2026-01-31, 2026-02-28, 2026-03-31]
    #   ACTUAL   [2026-01-31, 2026-03-03, 2026-03-31]   (Feb 31 -> Mar 3)

    # `until` drops the date that lands exactly on the bound:
    expand({"freq": "daily", "until": date(2026, 1, 3)}, date(2026, 1, 1), 10)
    #   EXPECTED [2026-01-01, 2026-01-02, 2026-01-03]
    #   ACTUAL   [2026-01-01, 2026-01-02]               (until treated exclusive)

These defects interact: a monthly rule with `interval > 1` that starts on a
month-end and carries an `until` bound exercises all three at once.

## Reproduce

Run the visible tests from the directory that contains the `rrulelite` package:

    python -m unittest rrulelite.test_rrulelite

## Contract (must hold after your fix)

* Package name stays `rrulelite`; import path `rrulelite` / `rrulelite.public`.
* Keep the public API exactly: `expand(rule: dict, start: date, limit: int) ->
  list[date]` and the `RRuleError` exception. Do not rename them.
* `rule` is a dict with:
    - `freq`: one of `"daily"`, `"weekly"`, `"monthly"` (required). An
      unsupported value raises `RRuleError`.
    - `interval`: a positive int step (default 1). For `freq == "monthly"`,
      `interval == 2` means EVERY OTHER month; for daily it means every 2nd day;
      for weekly every 2nd week. A non-positive / non-int interval raises
      `RRuleError`.
    - `until`: an optional `date`. It is an INCLUSIVE upper bound — a generated
      date EQUAL to `until` is kept, and the first date strictly AFTER `until`
      stops the expansion.
* The expansion ALWAYS starts at `start`: the first emitted date is `start`
  itself (unless `start` already exceeds `until`, in which case the result is
  empty). Subsequent dates are `start` plus 1*interval, 2*interval, ... steps.
* At most `limit` dates are returned. `limit <= 0` returns an empty list.
* MONTH-END semantics: stepping months clamps the day to the LAST valid day of
  the target month. From Jan 31, stepping one month gives Feb 28 in a common
  year and Feb 29 in a leap year (e.g. 2028), NOT an invalid Feb 31 and NOT a
  rolled-over Mar 2 / Mar 3. Stepping further keeps using the ORIGINAL day where
  the month allows it (Jan 31 -> Feb 28 -> Mar 31), i.e. clamping is per-step
  from `start`, not a permanent truncation.

Example:

    expand({"freq": "monthly", "interval": 2, "until": date(2026, 7, 31)},
           date(2026, 1, 31), 10)
    #   -> [2026-01-31, 2026-03-31, 2026-05-31, 2026-07-31]

Standard library only (`datetime`). Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `daily_simple` — daily interval=1 emits consecutive days from start
- `daily_interval3` — daily interval=3 steps every 3rd day
- `weekly_interval2` — weekly interval=2 steps every other week
- `monthly_simple` — monthly interval=1 emits consecutive months
- `monthly_interval2` — monthly interval=2 -> every other month
- `monthly_interval3_year_wrap` — monthly interval=3 wraps across year end
- `clamp_jan31_to_feb28` — Jan 31 + 1 month clamps to Feb 28 (common year)
- `clamp_leap_feb29` — Jan 31 + 1 month clamps to Feb 29 in a leap year (2028)
- `clamp_then_restore` — clamping is per-step: Jan 31 -> Feb 28 -> Mar 31
- `clamp_31_to_30day_month` — Mar 31 + 1 month clamps to Apr 30
- `clamp_interval2_endpoints` — interval=2 from Jan 31 -> Jan 31, Mar 31, May 31
- `until_inclusive_daily` — until keeps the date landing exactly on it (daily)
- `until_inclusive_monthly` — until keeps the exact-boundary monthly date
- `until_strict_past_stops` — a generated date strictly past until is excluded
- `until_start_past_until` — start beyond until yields empty list
- `limit_cap` — limit caps the number of emitted dates
- `limit_zero_empty` — limit <= 0 returns empty list
- `interaction_interval_clamp_until` — interval=2 + month-end clamp + inclusive until together
- `bad_freq_raises` — unsupported freq raises RRuleError
- `bad_interval_raises` — non-positive interval raises RRuleError


### textflow · 22 checks

````
BUG REPORT — textflow: the test suite is failing

You have an existing Python package `textflow`, a tiny full-text-justification
helper. It ships with a unittest suite in `textflow/test_textflow.py`, and right
now several of those tests FAIL. Fix the code so that ALL the tests pass. Do not
rewrite the package from scratch and do not change its public API.

## Symptom

`justify(words, width)` packs words greedily onto lines and pads each line out to
exactly `width` characters. Lines whose words happen to fill the width with plain
single spaces already come out right, but several padding / spacing cases are
wrong:

    from textflow.public import justify

    # extra spaces are pushed to the RIGHT gaps instead of the left:
    justify(["a", "b", "c", "next"], 8)
    #   EXPECTED ['a   b  c', 'next    ']
    #   ACTUAL   ['a  b   c', 'next    ']   (wider gap on the right, not left)

    # a single-word line is NOT padded out to width:
    justify(["longword", "tail"], 12)
    #   EXPECTED ['longword    ', 'tail        ']
    #   ACTUAL   ['longword', 'tail']             (neither line padded to width)

    # the LAST line is left-justified-and-padded, not stretched / dropped:
    justify(["alpha", "beta", "gamma"], 14)
    #   EXPECTED ['alpha     beta', 'gamma         ']
    #   ACTUAL   ['alpha     beta', 'gamma']      (last line not padded; a
    #                                              multi-word last line would get
    #                                              its single spaces stretched)

These defects interact: a paragraph that contains an uneven interior line, a
single-word interior line, AND a ragged final line exercises all three at once.

## Reproduce

Run the visible tests from the directory that contains the `textflow` package:

    python -m unittest textflow.test_textflow

## Contract (must hold after your fix)

* Package name stays `textflow`; import path `textflow` / `textflow.public`.
* Keep the public API exactly: `justify(words: list[str], width: int) ->
  list[str]` and the `JustifyError` exception. Do not rename them.
* `words` is a list of non-empty strings; `width` is a positive int. A word
  longer than `width`, an empty word, a non-positive `width`, or a non-list /
  non-int argument raises `JustifyError`. An empty `words` list returns `[]`.
* PACKING: words are placed greedily, one space assumed between adjacent words.
  A word joins the current line while `current_word_chars + gaps + len(word) <=
  width` (where `gaps` is the number of words already on the line); otherwise it
  starts a new line.
* OUTPUT: one string per line, and EVERY returned line is EXACTLY `width`
  characters wide.
* FULL JUSTIFICATION (every line EXCEPT the last, with 2+ words): the leftover
  space (`width` minus the total word characters) is spread across the gaps
  between words as evenly as possible. When it does not divide evenly, the EXTRA
  spaces go to the LEFT-most gaps, so earlier gaps are (by at most one space)
  wider than later ones. With G gaps and S leftover spaces, the first `S mod G`
  gaps get `S // G + 1` spaces and the rest get `S // G`.
* SINGLE-WORD LINE: a line holding exactly one word has no gap to stretch, so it
  is left-justified — the word followed by enough trailing spaces to reach
  `width`.
* LAST LINE: the final line is left-justified too — its words joined by SINGLE
  spaces, then padded on the right with spaces out to `width`. Its internal
  single spaces are NEVER stretched, even when it holds several words.

Example:

    justify(["practical", "no", "gap", "x", "the", "final", "row"], 10)
    #   -> ['practical ',   # single interior word: left-justified + padded
    #       'no  gap  x',   # interior, fully justified (even here)
    #       'the  final',   # interior, fully justified
    #       'row       ']   # last line: left-justified + padded

Standard library only. Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `single_line_exact` — words filling a line exactly pass through
- `even_distribution` — evenly divisible spaces look right under any rule
- `even_three_gaps` — three gaps that divide evenly look right
- `two_word_full_width` — two words that exactly fill a line pass through
- `even_multi_line` — consecutive even-fit lines pack and justify cleanly
- `greedy_pack_widths` — greedy packing yields exactly-width lines
- `uneven_extra_left_2gaps` — 1 extra space goes to the left gap
- `uneven_extra_left_3gaps` — 2 extra spaces go to the first two gaps
- `uneven_big_remainder` — remainder front-loaded across many gaps
- `uneven_width_invariant` — uneven lines are still exactly width wide
- `single_word_interior_padded` — lone interior word padded out to width
- `single_word_interior_width` — lone interior word line is exactly width
- `single_word_then_pack` — single-word line then a justified line
- `last_line_left_justified` — final line left-justified + padded, not stretched
- `last_line_multiword_single_spaced` — final line keeps single spaces then pads
- `last_line_is_width` — the last line is still exactly width wide
- `interaction_all_three` — uneven + single-word + last-line together
- `interaction_long_paragraph` — a longer paragraph exercises every path
- `interaction_widths` — every line of the interaction paragraph is width
- `empty_words_empty` — empty word list yields empty result
- `long_word_raises` — a word longer than width raises JustifyError
- `bad_width_raises` — a non-positive width raises JustifyError


### unitconv · 26 checks

````
BUG REPORT — unitconv: the test suite is failing

You have an existing Python package `unitconv`, a tiny unit converter. It ships
with a unittest suite in `unitconv/test_unitconv.py`, and right now several of
those tests FAIL. Fix the code so that ALL the tests pass. Do not rewrite the
package from scratch and do not change its public API.

## Symptom

`convert(value, from_unit, to_unit)` converts a number between units and returns
a float. Simple same-dimension conversions (mm <-> m <-> km, s <-> min <-> h)
already work, but compound SPEED units and cross-dimension guards come out wrong:

    from unitconv.public import convert

    # compound numerator scale is dropped:
    convert(2, "km/s", "m/s")
    #   EXPECTED 2000.0          (2 km/s = 2000 m/s)
    #   ACTUAL   2.0             ("km" treated as if it were "m")

    # compound denominator composed with the wrong operator:
    convert(3600, "m/h", "m/s")
    #   EXPECTED 1.0             (per HOUR -> divide by 3600 s)
    #   ACTUAL   46656000.0      (multiplied by 3600 instead of divided)

    # a real speed conversion exercises both at once:
    convert(36, "km/h", "m/s")
    #   EXPECTED 10.0            (36000 m / 3600 s)
    #   ACTUAL   wrong number

    # incompatible dimensions return a number instead of raising:
    convert(1, "m", "s")
    #   EXPECTED UnitError       (a length is not a time)
    #   ACTUAL   1.0             (no guard — runs the arithmetic anyway)

These defects interact: a `km/h -> m/s` conversion exercises the numerator-scale
bug AND the denominator-operator bug together, and a cross-dimension call like
`m -> km/h` must be rejected rather than silently computed.

## Reproduce

Run the visible tests from the directory that contains the `unitconv` package:

    python -m unittest unitconv.test_unitconv

## Contract (must hold after your fix)

* Package name stays `unitconv`; import path `unitconv` / `unitconv.public`.
* Keep the public API exactly: `convert(value, from_unit: str, to_unit: str) ->
  float` and the `UnitError` exception. Do not rename them.
* `convert` returns a `float`. `value` is an `int` or `float`.
* SIMPLE units convert to a canonical base by a single factor; a COMPOUND speed
  unit is written `"<length>/<time>"` and its factor to the base (metres per
  second) is the LENGTH factor DIVIDED BY the TIME factor — the numerator
  MULTIPLIES, the denominator DIVIDES:

      dimension  unit    factor to base (base shown in [])
      ---------  ------  ---------------------------------
      length     mm      0.001            [metre]
      length     m       1
      length     km      1000
      time       s       1                [second]
      time       min     60
      time       h       3600
      speed      m/s     (m factor) / (s factor)  = 1/1     [metre/second]
      speed      km/h    (km factor) / (h factor) = 1000/3600

* CONVERSION: take `value` to the canonical base via `from_unit`'s factor, then
  to `to_unit` by dividing by `to_unit`'s factor: `value * from_factor /
  to_factor`.
* COMPOUND PARSING: `"<length>/<time>"` splits on the single `/`. The part
  before the slash must be a length unit (the numerator), the part after must be
  a time unit (the denominator). Its dimension is `"speed"`.
* INCOMPATIBLE DIMENSIONS: `convert` only converts WITHIN one dimension.
  Converting across dimensions — a length to a time, a time to a speed, a length
  to a speed, etc. — raises `UnitError`. It must NOT return a number.
* UNKNOWN UNITS: an unrecognised unit string (either side) raises `UnitError`.

Example:

    convert(36, "km/h", "m/s")   # -> 10.0
    convert(10, "m/s", "km/h")   # -> 36.0
    convert(120, "m/min", "m/s") # -> 2.0
    convert(1, "m", "s")         # -> raises UnitError (length vs time)

Standard library only. Do not change the package name or the public
function/exception names.
````

**Graded behaviors:**

- `len_mm_to_m` — 1000 mm -> 1.0 m
- `len_km_to_m` — 2 km -> 2000 m
- `len_m_to_mm` — 1 m -> 1000 mm
- `len_km_to_mm` — 3 km -> 3_000_000 mm
- `time_min_to_h` — 90 min -> 1.5 h
- `time_h_to_s` — 1 h -> 3600 s
- `time_s_to_min` — 120 s -> 2 min
- `time_h_to_min` — 2 h -> 120 min
- `compound_km_per_s_parse` — 2 km/s -> 2000 m/s (km numerator scale kept)
- `compound_m_per_s_to_km_per_s` — 3000 m/s -> 3 km/s
- `compound_m_per_h_compose` — 3600 m/h -> 1 m/s (per-hour divides by 3600)
- `compound_m_per_s_to_m_per_h` — 1 m/s -> 3600 m/h
- `compound_m_per_min_compose` — 120 m/min -> 2 m/s
- `speed_kmh_to_ms` — 36 km/h -> 10 m/s
- `speed_kmh_to_ms_2` — 72 km/h -> 20 m/s
- `speed_ms_to_kmh` — 10 m/s -> 36 km/h
- `speed_ms_identity` — 5 m/s -> 5 m/s
- `speed_kmh_identity` — 1 km/h -> 1 km/h
- `incompat_len_to_time` — length -> time raises (m -> s)
- `incompat_time_to_len` — time -> length raises (min -> km)
- `incompat_speed_to_len` — speed -> length raises (m/s -> km)
- `incompat_len_to_speed` — length -> speed raises (m -> km/h)
- `incompat_time_to_speed` — time -> speed raises (h -> m/s)
- `unknown_from` — unknown from-unit raises (ly -> m)
- `unknown_to` — unknown to-unit raises (m -> ly)
- `unknown_compound_den` — unknown compound denominator raises (m/foo -> m/s)


## ambiguous-spec (12)

_Deliberately underspecified — NOT graded. The brief leaves real design decisions open; we observe how each model interprets it._


### auditlog · observed (no grader)

````
We need a small Python library called `auditlog` for keeping an audit log — a
record of who did what, and when. The kind of thing you reach for when someone
eventually asks "who changed this setting?" or "when was that account
deactivated?" and you'd like to have an answer instead of a shrug.

The shape of it is simple enough: things happen in our app — a user updates a
record, an admin grants a permission, a job deletes some data — and we want to
note each of those down as it happens, with enough context to be useful later.
Then, when someone comes asking, we want to be able to look back through the log
and pull out the entries that matter — everything a particular person did, or
everything that touched a particular thing, or whatever happened in some window
of time.

I care most about two things. First, that recording an event is easy enough that
we'll actually do it everywhere we should, rather than skipping it because it's a
hassle. Second, that the log is trustworthy — that when we read an entry back, we
can believe it says what really happened. Some of what we log will be sensitive,
so bear that in mind.

It should be importable as `auditlog`, and there should be an obvious way to both
write to the log and query it back. Beyond that the design is yours — lean on the
standard library, keep it clean rather than sprawling, and include some tests and
a short README showing how it's meant to be used. I'd rather see the choices
you'd actually defend than every option under the sun.
````


### cachelayer · observed (no grader)

````
We have a handful of functions scattered across our code that are genuinely
expensive to call — some hit the network, some chew through a big computation —
and we keep calling them with the same inputs over and over. I'd like a small
Python library, `cachelayer`, that lets us remember the result of a call so the
next call with the same inputs comes back fast instead of doing the work again.

The basic shape is: point it at one of these expensive lookups, and from then on
a repeat call with the same arguments returns the saved result. I want it to feel
natural to drop into existing code without rewriting how those functions are
called, and I want to be able to trust what's in there — a cached answer should be
one I'd actually be comfortable handing back, not something stale I've quietly
forgotten about.

How you let people attach caching to a function, how you decide two calls count as
"the same," and how you keep the cache from growing without bound are the
interesting parts, and I'm leaving them to you. Use your judgment; I'd rather see
the approach you'd actually defend than a pile of knobs. It should import as
`cachelayer` and be pleasant to use from our own code.

Keep it reasonably small and lean on the standard library where you can. Some
tests around the behavior that matters — that hits come back without redoing the
work, and that the cache does the right thing as it fills up — would give me
confidence it works the way you think it does.
````


### docsearch · observed (no grader)

````
I've got a growing pile of text documents — notes, docs, transcripts, that kind of
thing — and I want a small Python library, `docsearch`, that lets me search across
the whole collection and get back the documents that actually match what I'm looking
for.

The basic shape is: I give it a bunch of documents, then I ask it questions and it
hands me back the ones that are relevant, best matches first. Right now I mostly grep
through everything, which finds the literal string but has no sense of which hits
matter more, and falls apart the moment I half-remember the wording. I'd like something
that does better than that — where a search for a couple of words surfaces the
documents that are really about them, not just the ones that happen to contain the
characters somewhere.

What I care about most is that the ranking feels sensible — when I read the top few
results I should believe they're the best ones — and that it stays quick and pleasant
to use on a collection that keeps growing, since I'll be adding documents as I go. It
should be importable as `docsearch` and callable from my own code; if there's an
obvious way to poke at it from the command line too, that's a nice bonus, but use your
judgment.

I'm leaving the actual design to you. Lean on the standard library where it's
reasonable. Some tests around the searching and ranking would help me trust it, and a
short README with an example or two so I can see how you meant it to be used. I'd
rather have something clean and well-considered than something that tries to do
everything.
````


### featureflags · observed (no grader)

````
We want a small Python library, `featureflags`, for turning bits of functionality
on and off without having to redeploy. The idea is that our code can ask "is this
feature on right now?" and get a yes or no back, so we can ship things dark, flip
them on when we're ready, and roll them back just as fast if they misbehave.

Mostly we just want a clean way to check whether a feature is enabled, but a flag
shouldn't always have to be a flat on/off for everybody. We'd like to be able to
turn something on for some people and not others — say, our own team first, then a
slice of users, then everyone — and to have a flag behave differently depending on
where it's running. How much of that you build in versus leave for later is up to
you; I'd rather see a couple of those cases done well than a switchboard of options
half-wired.

It should be importable as `featureflags` and called straight from our own code.
The flags themselves have to live somewhere we can change without touching the
program, and I should be able to look at what's currently on without digging through
internals. When the code asks about a flag nobody's defined, it should do something
sane and predictable rather than blowing up a request.

Keep it lean and lean on the standard library where you can. Some tests around the
evaluation behavior would give me confidence it does what you think it does, and a
short README showing how it's meant to be used. I'd rather have something clean and
well-thought-through than something sprawling.
````


### formvalidate · observed (no grader)

````
We keep hand-rolling the same checks every time we accept data from a form or an
API request, and it always ends up as a tangle of `if` statements that nobody
wants to touch. I'd like a small Python library, `formvalidate`, that we can point
at user-submitted data and get back a clear answer: is this okay, and if not,
what's wrong with it?

The everyday case is the stuff you'd expect off a signup or a checkout form — a
required email, a name that can't be blank, an age that has to be a number in some
range, a couple of fields that are only required depending on what else was filled
in. What I care about most is that when something is invalid, whoever's on the
other end gets back something they can actually act on — clear enough to show a
user or log and move on — rather than a single cryptic exception that throws away
every problem but the first.

It should be pleasant to describe what valid data looks like, and it shouldn't
fall apart when the input is messier or more deeply structured than a flat handful
of keys. I want to import `formvalidate` and call it from our own code.

Beyond that, the shape is yours. Lean on the standard library, keep it small, and
make the choices you'd actually defend rather than piling on knobs. Some tests
around the interesting validation cases would give me confidence it does what you
think it does, and a short README with an example or two so I can see how it's
meant to be used.
````


### jobflow · observed (no grader)

````
We need a small Python library called `jobflow` for defining jobs that depend on
other jobs and then running them in the right order.

The idea is that someone can describe a bunch of jobs — a job is basically just a
unit of work, often wrapping a function or a command — and say which other jobs
each one needs to have finished first. Then they hand the whole thing to jobflow
and it figures out the order and runs everything. A "build this, then test it,
then deploy it" kind of workflow, but general enough to be useful for data
pipelines, scheduled chores, that sort of thing.

I care most about it being pleasant to define jobs and their dependencies, and
about it being dependable when something goes wrong — a run shouldn't just silently
do the wrong thing. It should be importable as `jobflow` and there should be an
obvious way to actually kick off a run, whether that's from code or the command
line; use your judgment there.

Beyond that I'm leaving the design to you. Keep it reasonably small and lean on the
standard library where you can. Some real tests covering the interesting cases would
be good, and a couple of examples in a README so I can see how it's meant to be used.
I'd rather have something clean and well-thought-through than something sprawling.
````


### mergeconf · observed (no grader)

````
We keep re-solving the same problem across our services: a config that comes from a few different places, and nobody can ever say what the final, effective value actually is. I'd like a small tool, `mergeconf`, that takes configuration from several sources and merges them into one effective configuration you can hand to the rest of the app.

The sources are the usual suspects you'd expect in a deployed service — there's a baseline we ship with, something operators drop in per environment, and the last-minute overrides people pass at runtime. mergeconf should pull those together and produce the single resolved view, plus enough of a trail that when someone asks "why is this value what it is?" we can actually answer them instead of guessing.

It should behave sensibly with nested config, not just a flat bag of keys, and it shouldn't fall over the first time two sources disagree or hand it something shaped differently than expected. Keep it reasonably ergonomic — I want to import `mergeconf` and call it from our own code, and also be able to run it directly to inspect a merge without writing a script.

Don't gold-plate it. I care more that the merge is predictable and that someone reading the result can trust it than about covering every exotic case. Use your judgment on the details; I'd rather see the choices you'd actually defend than a pile of options. Some tests around the merge behavior would give me confidence it does what you think it does.
````


### notifyhub · observed (no grader)

````
We send notifications to people from all over our codebase, and right now every
service does it slightly differently and badly. I'd like to pull that together into
a small Python package, `notifyhub`, that takes a message meant for some person and
actually gets it delivered to them.

The thing I keep wanting is to hand it a recipient and a message and have it sorted —
the same call should be able to reach someone however we reach them, whether that's
an email, a text, a ping to some webhook, or just a line in a log while we're
developing. I don't want callers to care which one it is or to rewrite anything when
we add another way to reach people later. And when a send doesn't go through, I want
that to be something we can see and reason about, not a thing that quietly vanishes.

We're not wiring up real email or SMS providers in this pass, so do whatever makes
sense to stand that part in for now. Beyond that I'm leaving the shape to you — how a
message gets pointed at the right channel, what happens on a failure, how much a
message can carry — those are your calls. I'd rather see the design you'd defend than
a switchboard of options.

It should import as `notifyhub` and there should be an obvious way to actually fire a
notification, from code or otherwise; use your judgment. Keep it lean, lean on the
standard library, and give me some tests around the parts that matter so I can trust
it does what you think it does.
````


### ratelimit · observed (no grader)

````
We're getting hammered by a handful of clients and need some basic rate limiting we
can put in front of our API before it tips over again. I'd like a small Python
library, `ratelimit`, that lets us decide whether a given request is allowed right
now or should be turned away, so we can stop one noisy caller from eating all the
capacity.

The shape of it is simple enough: something asks "can this go through?", usually on
behalf of a particular caller — an API key, a user id, an IP, whatever we happen to
have — and `ratelimit` tells us yes or no based on how much they've already done
recently. Different callers shouldn't interfere with each other. We'll mostly be
checking from our own request-handling code, so it should be comfortable to import
`ratelimit` and call into it, and it'd be handy to be able to exercise it directly
to watch a limit kick in without standing up the whole service.

What I care about most is that it behaves predictably under bursts — that the limit
it promises is the limit you actually get, and that it doesn't get confused when a
lot of checks land close together. I'd also like to be able to look at a decision and
understand why it came out the way it did, rather than just a bare yes/no.

Use your judgment on the rest; I'd rather see the approach you'd actually defend than
a pile of knobs. Lean on the standard library where you can, and include some tests
around the limiting behavior so I can trust it does what you think it does.
````


### retryflow · observed (no grader)

````
We keep writing the same fiddly retry code over and over: a network call flakes,
we wrap it in a loop, sleep a bit, try again, and every service does it slightly
differently. I'd like a small Python library, `retryflow`, that gives us one decent
way to retry an operation that might fail transiently — an HTTP request, a flaky
database connection, that sort of thing — so we stop reinventing it.

The thing I care about most is that retrying should be easy to reach for on an
existing piece of code without contorting it, and that it stays predictable: when
something keeps failing, the caller should end up with a clear outcome rather than a
swallowed error or a surprise. Not every failure is worth retrying, so there should
be a sensible way to say which ones are and aren't — retrying a "host unreachable" is
fine, retrying a "you're not authorized" is just wasting time.

Beyond that I'm leaving the design to you. Use your judgment on how the retrying
behaves over successive attempts and when it decides to stop, and on the shape callers
actually use to apply it. It should be importable as `retryflow` and there should be an
obvious way to use it from our own code. Lean on the standard library where you can and
keep it reasonably small. Some tests around the retry behavior would give me confidence
it does what you think it does, and a short note on how it's meant to be used.
````


### scheduler · observed (no grader)

````
I'd like a small Python package, `scheduler`, that runs tasks on a schedule — some
things need to happen every so often, others at particular times of day, and I want
one place to register that work and have it actually fire when it's supposed to.

The picture in my head is: I describe a handful of tasks, say when each one should
run, hand the lot to the scheduler, and let it tick along firing them at the right
moments. The kind of thing you'd reach for to send a nightly report, refresh a cache
every few minutes, do the weekly cleanup — ordinary recurring chores, nothing exotic.

What I care about most is that it's pleasant to register a task and say when it runs,
and that I can trust the timing — when a task was due, it ran, and I can tell that it
did. It should be importable as `scheduler`, and there should be an obvious way to
actually start the thing going, whether that's from code or the command line; use
your judgment.

Beyond that the design is yours. Keep it small and lean on the standard library where
you can — I'd rather see the choices you'd actually defend than a wall of options or
configuration. Some real tests around the scheduling behavior would give me
confidence it does what you think it does, and a couple of examples in a README so I
can see how it's meant to be used. Clean and well-thought-through beats sprawling.
````


### workflow · observed (no grader)

````
We run a lot of multi-step business processes and right now each one is hand-coded
as a tangle of if-statements, and nobody can tell at a glance what state something
is in or where it can go next. I'd like a small Python library, `workflow`, for
describing a process as a set of stages and the transitions between them, and then
driving an individual thing through those stages over its lifetime.

Think of an order: it starts somewhere, moves through stages like received, paid,
packed, shipped, and lands in a terminal state — but real processes aren't a
straight line. Some stages can go more than one way depending on what happened
(payment cleared vs. declined), and some paths need to be ruled out entirely. What
I want is to define the shape of the process once, and then for any given order be
able to ask where it is, advance it, and have the library refuse the moves that
don't make sense rather than quietly letting it skip a stage.

I care most about it being clear to define a process and trustworthy to drive — an
order should never end up somewhere the process doesn't allow, and when a move is
rejected I want to understand why. It should be importable as `workflow`, and there
should be an obvious way to actually run something through, whether from code or to
inspect a process directly; use your judgment there.

Beyond that the design is yours. Keep it small and lean on the standard library.
Some tests around the interesting transition cases, and a couple of examples in a
README, would give me confidence it does what you think it does. I'd rather see the
choices you'd actually defend than a pile of knobs.
````

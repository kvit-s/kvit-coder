"""Reference confstack.public — a configuration loader with strict precedence.

Precedence (highest wins): CLI flags > environment variables > config file > defaults.
Stdlib only.
"""
import json
import re
import os

_INT_RE = re.compile(r"-?\d+\Z")


def infer(value):
    """Infer bool/int/str from a raw string value.

    Rules (pinned in the Contract):
      - "true"/"false" (case-insensitive) -> bool
      - optionally-signed run of digits (-?\\d+) -> int
      - otherwise -> the string unchanged (floats stay strings)
    Non-strings are returned as-is (already typed by their source).
    """
    if not isinstance(value, str):
        return value
    low = value.lower()
    if low == "true":
        return True
    if low == "false":
        return False
    if _INT_RE.match(value):
        return int(value)
    return value


def _set_dotted(tree, dotted_key, value):
    """Insert value at a dot-notation key, creating nested dicts as needed."""
    parts = dotted_key.split(".")
    cur = tree
    for part in parts[:-1]:
        nxt = cur.get(part)
        if not isinstance(nxt, dict):
            nxt = {}
            cur[part] = nxt
        cur = nxt
    cur[parts[-1]] = value


def _expand(source):
    """Expand a (possibly dotted / nested) dict into a fully nested dict.

    Dot-notation keys are split into nesting levels. Nested-dict values are
    expanded recursively (so a nested dict may itself carry dotted keys).
    """
    out = {}
    for key, value in source.items():
        if isinstance(value, dict):
            value = _expand(value)
        _set_dotted(out, key, value)
    return out


def _deep_merge(base, overlay):
    """Recursively merge overlay onto base. Overlay leaves win; non-overlapping
    keys from both survive. Returns a new dict; inputs are not mutated."""
    result = dict(base)
    for key, value in overlay.items():
        if key in result and isinstance(result[key], dict) and isinstance(value, dict):
            result[key] = _deep_merge(result[key], value)
        else:
            result[key] = value
    return result


def _from_env(env):
    """Build a nested dict from APP_-prefixed env vars.

    APP_ prefix stripped, remainder lowercased, `__` -> nesting separator.
    Values pass through type inference.
    """
    flat = {}
    for name, raw in env.items():
        if not name.startswith("APP_"):
            continue
        remainder = name[len("APP_"):].lower()
        dotted = remainder.replace("__", ".")
        flat[dotted] = infer(raw)
    return _expand(flat)


def _from_cli(cli_args):
    """Parse CLI flags into a nested dict.

    Forms: `--key value`, `--nested.key value`, `--flag`, `--no-flag`.
    A bare flag (next token absent or another --token) becomes boolean True;
    a `--no-` prefix yields boolean False. `--key value` values pass through
    type inference.
    """
    flat = {}
    i = 0
    n = len(cli_args)
    while i < n:
        tok = cli_args[i]
        if not isinstance(tok, str) or not tok.startswith("--"):
            i += 1
            continue
        body = tok[2:]
        if body.startswith("no-"):
            # Strip leading `no-` from the first segment only.
            key = body[len("no-"):]
            flat[key] = False
            i += 1
            continue
        # Positive form: may be `--key value` or a bare boolean `--flag`.
        nxt = cli_args[i + 1] if i + 1 < n else None
        if nxt is None or (isinstance(nxt, str) and nxt.startswith("--")):
            flat[body] = True
            i += 1
        else:
            flat[body] = infer(nxt)
            i += 2
    return _expand(flat)


def _from_config_file(config_file):
    """Load + expand a JSON config file. Missing file or None -> empty layer."""
    if config_file is None:
        return {}
    if not os.path.exists(config_file):
        return {}
    with open(config_file) as f:
        data = json.load(f)
    if not isinstance(data, dict):
        return {}
    return _expand(data)


def load_config(defaults, config_file, env, cli_args):
    """Merge the four sources with strict precedence into a nested dict."""
    layers = [
        _expand(defaults or {}),
        _from_config_file(config_file),
        _from_env(env or {}),
        _from_cli(cli_args or []),
    ]
    merged = {}
    for layer in layers:
        merged = _deep_merge(merged, layer)
    return merged

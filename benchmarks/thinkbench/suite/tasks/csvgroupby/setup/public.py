"""csvgroupby.public — a tiny in-memory SQL-subset query engine over CSV rows.

Standard library only. ``rows`` is a list of dicts (one per CSV record, every
value a string as read from the file). ``query(rows, sql)`` parses a small SELECT
grammar, filters with WHERE, and returns a list of row dicts.

Supported grammar (case-insensitive keywords):

    SELECT <select-list> FROM <table> [WHERE <column> <op> <value>]

    <select-list> := '*' | <column> {',' <column>}
    <op>          := = | != | < | <= | > | >=

The table name is accepted but ignored (``rows`` is the data source); it only
exists so real-looking queries parse. WHERE compares with NUMERIC INFERENCE: a
cell or literal that looks like a number is compared numerically, otherwise as a
string.

NOTE: this engine does NOT yet support ``GROUP BY`` or ``COUNT(*)``.
"""
from __future__ import annotations

import re

# --- value typing ------------------------------------------------------------

_INT_RE = re.compile(r"[+-]?\d+$")
_FLOAT_RE = re.compile(r"[+-]?(\d+\.\d*|\.\d+|\d+)([eE][+-]?\d+)?$")


def _infer(value):
    """Infer a number from a string where possible; otherwise return the string."""
    if not isinstance(value, str):
        return value
    s = value.strip()
    if s == "":
        return value
    if _INT_RE.match(s):
        try:
            return int(s)
        except ValueError:
            pass
    if _FLOAT_RE.match(s):
        try:
            return float(s)
        except ValueError:
            pass
    return value


# --- tokenizer ---------------------------------------------------------------

_TOKEN_RE = re.compile(
    r"""
      \s*(?:
          (?P<op><=|>=|!=|<>|=|<|>)            # comparison operators
        | (?P<star>\*)
        | (?P<comma>,)
        | (?P<sqstr>'(?:[^']|'')*')            # single-quoted string literal
        | (?P<dqstr>"(?:[^"]|"")*")            # double-quoted string literal
        | (?P<num>[+-]?(?:\d+\.\d*|\.\d+|\d+)(?:[eE][+-]?\d+)?)
        | (?P<word>[A-Za-z_][A-Za-z0-9_]*)
      )
    """,
    re.VERBOSE,
)


def _tokenize(query_text):
    tokens = []
    pos = 0
    n = len(query_text)
    while pos < n:
        if query_text[pos].isspace():
            pos += 1
            continue
        m = _TOKEN_RE.match(query_text, pos)
        if not m or m.end() == pos:
            raise ValueError(f"cannot tokenize query at: {query_text[pos:pos + 20]!r}")
        pos = m.end()
        kind = m.lastgroup
        text = m.group(kind)
        tokens.append((kind, text))
    return tokens


# --- parser ------------------------------------------------------------------

_KEYWORDS = {"SELECT", "FROM", "WHERE", "GROUP", "BY"}


class _Parser:
    def __init__(self, tokens):
        self.toks = tokens
        self.i = 0

    def _peek(self):
        return self.toks[self.i] if self.i < len(self.toks) else (None, None)

    def _next(self):
        tok = self._peek()
        self.i += 1
        return tok

    def _at_keyword(self, *words):
        kind, text = self._peek()
        return kind == "word" and text.upper() in words

    def _expect_keyword(self, word):
        kind, text = self._next()
        if kind != "word" or text.upper() != word:
            raise ValueError(f"expected {word}, got {text!r}")

    def parse(self):
        self._expect_keyword("SELECT")
        select_list = self._parse_select_list()
        self._expect_keyword("FROM")
        kind, table = self._next()
        if kind != "word":
            raise ValueError(f"expected table name, got {table!r}")

        where = None
        if self._at_keyword("WHERE"):
            self._next()
            where = self._parse_condition()

        if self.i != len(self.toks):
            kind, text = self._peek()
            raise ValueError(f"unexpected trailing token {text!r}")

        return {"select": select_list, "table": table, "where": where}

    def _parse_select_list(self):
        kind, text = self._peek()
        if kind == "star":
            self._next()
            return [{"kind": "star"}]
        items = [self._parse_select_item()]
        while self._peek()[0] == "comma":
            self._next()
            items.append(self._parse_select_item())
        return items

    def _parse_select_item(self):
        name = self._parse_column_name()
        return {"kind": "column", "name": name}

    def _parse_column_name(self):
        kind, text = self._next()
        if kind != "word" or text.upper() in _KEYWORDS:
            raise ValueError(f"expected column name, got {text!r}")
        return text

    def _parse_condition(self):
        column = self._parse_column_name()
        kind, op = self._next()
        if kind != "op":
            raise ValueError(f"expected comparison operator, got {op!r}")
        if op == "<>":
            op = "!="
        value = self._parse_literal()
        return {"column": column, "op": op, "value": value}

    def _parse_literal(self):
        kind, text = self._next()
        if kind == "num":
            return _infer(text)
        if kind == "sqstr":
            return text[1:-1].replace("''", "'")
        if kind == "dqstr":
            return text[1:-1].replace('""', '"')
        if kind == "word":
            return _infer(text)
        raise ValueError(f"expected literal, got {text!r}")


def _parse(query_text):
    tokens = _tokenize(query_text)
    if not tokens:
        raise ValueError("empty query")
    return _Parser(tokens).parse()


# --- execution ---------------------------------------------------------------

_OPS = {
    "=": lambda a, b: a == b,
    "!=": lambda a, b: a != b,
    "<": lambda a, b: a < b,
    "<=": lambda a, b: a <= b,
    ">": lambda a, b: a > b,
    ">=": lambda a, b: a >= b,
}


def _compare(left, op, right):
    fn = _OPS[op]
    try:
        return fn(left, right)
    except TypeError:
        # cross-type comparison (e.g. number vs string): fall back to string compare,
        # except equality/inequality which are well-defined across types.
        if op == "=":
            return str(left) == str(right)
        if op == "!=":
            return str(left) != str(right)
        return fn(str(left), str(right))


def _eval_condition(row, cond):
    if cond["column"] not in row:
        return False
    cell = _infer(row.get(cond["column"]))
    return _compare(cell, cond["op"], cond["value"])


def _project(row, select_list):
    if select_list and select_list[0].get("kind") == "star":
        return dict(row)
    proj = {}
    for item in select_list:
        name = item["name"]
        proj[name] = row.get(name)
    return proj


def query(rows, sql):
    """Run a SQL-subset ``sql`` query against ``rows`` (a list of dicts).

    Returns a list of row dicts. Supports ``SELECT <cols> FROM <t>`` with an
    optional ``WHERE <col> <op> <val>`` filter (numeric inference on comparison).
    """
    plan = _parse(sql)

    filtered = [r for r in rows if _eval_condition(r, plan["where"])] if plan["where"] else list(rows)

    return [_project(r, plan["select"]) for r in filtered]

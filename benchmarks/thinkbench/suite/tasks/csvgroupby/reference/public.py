"""Reference csvgroupby.public — the in-memory query engine WITH GROUP BY support.

Standard library only. ``rows`` is a list of dicts (one per CSV record, every
value a string as read from the file). ``query(rows, sql)`` parses the SELECT
grammar, filters with WHERE, optionally groups, and returns a list of row dicts.

Supported grammar (case-insensitive keywords):

    SELECT <select-list> FROM <table>
        [WHERE <column> <op> <value>]
        [GROUP BY <column>]

    <select-list> := '*' | <item> {',' <item>}
    <item>        := <column> | COUNT(*)
    <op>          := = | != | < | <= | > | >=

GROUP BY <col> emits one output row per distinct value of <col>, in first-seen
order; ``COUNT(*)`` in the select list becomes the count of rows in that group,
reported under the key ``COUNT(*)``. GROUP BY composes with WHERE: filtering
happens first, then grouping over the surviving rows.
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
        | (?P<lparen>\()
        | (?P<rparen>\))
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
_AGG_FUNCS = {"COUNT"}


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
        group_by = None

        if self._at_keyword("WHERE"):
            self._next()
            where = self._parse_condition()
        if self._at_keyword("GROUP"):
            self._next()
            self._expect_keyword("BY")
            group_by = self._parse_column_name()

        if self.i != len(self.toks):
            kind, text = self._peek()
            raise ValueError(f"unexpected trailing token {text!r}")

        return {
            "select": select_list,
            "table": table,
            "where": where,
            "group_by": group_by,
        }

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
        kind, text = self._peek()
        if kind == "word" and text.upper() in _AGG_FUNCS:
            func = text.upper()
            self._next()
            if self._next()[0] != "lparen":
                raise ValueError(f"expected ( after {func}")
            inner_kind, _inner_text = self._next()
            if func == "COUNT":
                if inner_kind != "star":
                    raise ValueError("only COUNT(*) is supported")
            if self._next()[0] != "rparen":
                raise ValueError(f"expected ) closing {func}")
            return {"kind": "agg", "func": func, "key": "COUNT(*)"}
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

    Returns a list of row dicts. Supports ``SELECT <cols|COUNT(*)> FROM <t>`` with
    optional ``WHERE`` (numeric inference) and ``GROUP BY <col>``.
    """
    plan = _parse(sql)

    filtered = [r for r in rows if _eval_condition(r, plan["where"])] if plan["where"] else list(rows)

    select_list = plan["select"]
    group_by = plan["group_by"]

    if group_by is not None:
        # one output row per distinct group value, in first-seen order
        groups = {}
        order = []
        for r in filtered:
            key = r.get(group_by)
            hkey = (type(key).__name__, key)
            if hkey not in groups:
                groups[hkey] = []
                order.append((hkey, key))
            groups[hkey].append(r)
        result = []
        for hkey, key in order:
            grp = groups[hkey]
            out_row = {}
            for item in select_list:
                kind = item.get("kind")
                if kind == "star":
                    # SELECT * GROUP BY <col>: emit the group column only
                    out_row[group_by] = key
                elif kind == "agg":
                    out_row["COUNT(*)"] = len(grp)
                elif kind == "column":
                    if item["name"] == group_by:
                        out_row[group_by] = key
                    else:
                        # representative value for a non-grouped column
                        out_row[item["name"]] = grp[0].get(item["name"])
            # if the group column was not in the select list, still surface it so
            # each output row is identifiable by its group.
            if group_by not in out_row:
                out_row[group_by] = key
            result.append(out_row)
        return result

    # no GROUP BY: a bare COUNT(*) in the select list is a whole-table count
    if any(it.get("kind") == "agg" for it in select_list):
        out_row = {}
        for item in select_list:
            if item.get("kind") == "agg":
                out_row["COUNT(*)"] = len(filtered)
            elif item.get("kind") == "column":
                out_row[item["name"]] = filtered[0].get(item["name"]) if filtered else None
        return [out_row]

    return [_project(r, select_list) for r in filtered]

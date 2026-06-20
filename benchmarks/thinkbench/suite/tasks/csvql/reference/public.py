"""Reference csvql.public — a tiny SQL-subset query engine over CSV files.

Standard library only. Implements its own tokenizer + recursive-descent parser for
the supported SELECT grammar (no sqlite, no third-party parser). The public entry
point is ``query_csv(path, query) -> list[dict]``.

Supported grammar (case-insensitive keywords):

    SELECT <select-list> FROM <table>
        [WHERE <condition> {AND|OR <condition>}...]
        [GROUP BY <column>]
        [ORDER BY <column> [ASC|DESC]]
        [LIMIT <n>]

    <select-list> := '*' | <item> {',' <item>}
    <item>        := <column> | COUNT(*) | SUM(<column>) | AVG(<column>)
    <condition>   := <column> <op> <literal>
    <op>          := = | != | < | <= | > | >=

The table name is accepted but not used to locate the file (the path argument
identifies the CSV); it exists only so real-looking queries parse.
"""
import csv
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


def _tokenize(query):
    tokens = []
    pos = 0
    n = len(query)
    while pos < n:
        if query[pos].isspace():
            pos += 1
            continue
        m = _TOKEN_RE.match(query, pos)
        if not m or m.end() == pos:
            raise ValueError(f"cannot tokenize query at: {query[pos:pos + 20]!r}")
        pos = m.end()
        kind = m.lastgroup
        text = m.group(kind)
        tokens.append((kind, text))
    return tokens


# --- parser ------------------------------------------------------------------

_KEYWORDS = {"SELECT", "FROM", "WHERE", "AND", "OR", "GROUP", "BY", "ORDER", "LIMIT", "ASC", "DESC"}
_AGG_FUNCS = {"COUNT", "SUM", "AVG"}


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
        order_by = None
        limit = None

        if self._at_keyword("WHERE"):
            self._next()
            where = self._parse_where()
        if self._at_keyword("GROUP"):
            self._next()
            self._expect_keyword("BY")
            group_by = self._parse_column_name()
        if self._at_keyword("ORDER"):
            self._next()
            self._expect_keyword("BY")
            order_by = self._parse_order_by()
        if self._at_keyword("LIMIT"):
            self._next()
            kind, text = self._next()
            if kind != "num":
                raise ValueError(f"expected LIMIT number, got {text!r}")
            limit = int(float(text))

        if self.i != len(self.toks):
            kind, text = self._peek()
            raise ValueError(f"unexpected trailing token {text!r}")

        return {
            "select": select_list,
            "table": table,
            "where": where,
            "group_by": group_by,
            "order_by": order_by,
            "limit": limit,
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
            inner_kind, inner_text = self._next()
            if func == "COUNT":
                if inner_kind != "star":
                    raise ValueError("only COUNT(*) is supported")
                arg = "*"
            else:
                if inner_kind != "word":
                    raise ValueError(f"expected column inside {func}()")
                arg = inner_text
            if self._next()[0] != "rparen":
                raise ValueError(f"expected ) closing {func}")
            key = f"{func}(*)" if arg == "*" else f"{func}({arg})"
            return {"kind": "agg", "func": func, "arg": arg, "key": key}
        # plain column
        name = self._parse_column_name()
        return {"kind": "column", "name": name}

    def _parse_column_name(self):
        kind, text = self._next()
        if kind != "word" or text.upper() in _KEYWORDS:
            raise ValueError(f"expected column name, got {text!r}")
        return text

    def _parse_order_by(self):
        # ORDER BY one column OR one aggregate, optional ASC/DESC
        kind, text = self._peek()
        if kind == "word" and text.upper() in _AGG_FUNCS:
            item = self._parse_select_item()
            key = item["key"]
        else:
            key = self._parse_column_name()
        descending = False
        if self._at_keyword("ASC"):
            self._next()
        elif self._at_keyword("DESC"):
            self._next()
            descending = True
        return {"key": key, "descending": descending}

    def _parse_where(self):
        conditions = [self._parse_condition()]
        connectors = []
        while self._at_keyword("AND", "OR"):
            _, text = self._next()
            connectors.append(text.upper())
            conditions.append(self._parse_condition())
        return {"conditions": conditions, "connectors": connectors}

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
            # bareword literal (e.g. WHERE active = true) — treat as string, inferred
            return _infer(text)
        raise ValueError(f"expected literal, got {text!r}")


def _parse(query):
    tokens = _tokenize(query)
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


def _load_rows(path):
    with open(path, newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        rows = []
        for raw in reader:
            row = {k: _infer(v) for k, v in raw.items() if k is not None}
            rows.append(row)
    return rows


def _compare(left, op, right):
    fn = _OPS[op]
    try:
        return fn(left, right)
    except TypeError:
        # cross-type comparison (e.g. number vs string) — fall back to string compare,
        # except for equality which is well-defined across types.
        if op == "=":
            return str(left) == str(right)
        if op == "!=":
            return str(left) != str(right)
        return fn(str(left), str(right))


def _eval_condition(row, cond):
    cell = row.get(cond["column"])
    if cell is None and cond["column"] not in row:
        # unknown column never matches an ordering/inequality; treat as no-match
        return False
    return _compare(cell, cond["op"], cond["value"])


def _eval_where(row, where):
    if where is None:
        return True
    conds = where["conditions"]
    connectors = where["connectors"]
    # Left-to-right evaluation. AND binds the running result with the next condition;
    # OR likewise. (No precedence between AND/OR beyond left-to-right — sufficient for
    # the supported grammar and matches a simple reading of the brief.)
    result = _eval_condition(row, conds[0])
    for connector, cond in zip(connectors, conds[1:]):
        rhs = _eval_condition(row, cond)
        if connector == "AND":
            result = result and rhs
        else:
            result = result or rhs
    return result


def _numeric_values(rows, column):
    out = []
    for r in rows:
        v = r.get(column)
        if isinstance(v, bool):
            continue
        if isinstance(v, (int, float)):
            out.append(v)
    return out


def _compute_agg(rows, item):
    func, arg = item["func"], item["arg"]
    if func == "COUNT":
        return len(rows)
    vals = _numeric_values(rows, arg)
    if func == "SUM":
        return sum(vals)
    if func == "AVG":
        return (sum(vals) / len(vals)) if vals else 0
    raise ValueError(f"unsupported aggregate {func}")


def _project_plain(rows, select_list):
    if select_list and select_list[0].get("kind") == "star":
        return [dict(r) for r in rows]
    out = []
    for r in rows:
        proj = {}
        for item in select_list:
            name = item["name"]
            proj[name] = r.get(name)
        out.append(proj)
    return out


def _sort_key(value):
    # Sort tolerant to mixed types: numbers together, then strings, with a stable
    # ordering between the groups.
    if isinstance(value, bool):
        return (1, "", float(value))
    if isinstance(value, (int, float)):
        return (0, "", value)
    if value is None:
        return (2, "", 0.0)
    return (1, str(value), 0.0)


def query_csv(path, query):
    """Run a SQL-subset ``query`` against the CSV at ``path``; return list[dict]."""
    plan = _parse(query)
    rows = _load_rows(path)

    # WHERE
    rows = [r for r in rows if _eval_where(r, plan["where"])]

    select_list = plan["select"]
    has_agg = any(it.get("kind") == "agg" for it in select_list)
    group_by = plan["group_by"]

    if group_by is not None:
        # one output row per distinct group value, preserving first-seen order
        groups = {}
        order = []
        for r in rows:
            key = r.get(group_by)
            hkey = (type(key).__name__, key)
            if hkey not in groups:
                groups[hkey] = []
                order.append((hkey, key))
            groups[hkey].append(r)
        result = []
        for hkey, key in order:
            grp = groups[hkey]
            out_row = {group_by: key}
            for item in select_list:
                if item.get("kind") == "agg":
                    out_row[item["key"]] = _compute_agg(grp, item)
                elif item.get("kind") == "column" and item["name"] != group_by:
                    # representative value for a non-grouped column
                    out_row[item["name"]] = grp[0].get(item["name"])
            result.append(out_row)
    elif has_agg:
        # whole-table aggregate -> single row
        out_row = {}
        for item in select_list:
            if item.get("kind") == "agg":
                out_row[item["key"]] = _compute_agg(rows, item)
            elif item.get("kind") == "column":
                out_row[item["name"]] = rows[0].get(item["name"]) if rows else None
        result = [out_row]
    else:
        result = _project_plain(rows, select_list)

    # ORDER BY
    ob = plan["order_by"]
    if ob is not None:
        key_name = ob["key"]
        result.sort(key=lambda row: _sort_key(row.get(key_name)), reverse=ob["descending"])

    # LIMIT
    if plan["limit"] is not None:
        result = result[: max(0, plan["limit"])]

    return result

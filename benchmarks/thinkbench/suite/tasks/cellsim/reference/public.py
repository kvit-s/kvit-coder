"""Reference cellsim.public — a small spreadsheet calculation engine.

Stdlib-only. No use of Python `eval`. Formula evaluation is deterministic and
detects circular references. The public surface is the four functions named in
the brief; everything else is a private helper.
"""
import json
import re

# --- cell-name / range helpers ----------------------------------------------

_CELL_RE = re.compile(r"^([A-Za-z]+)([0-9]+)$")
_RANGE_RE = re.compile(r"^([A-Za-z]+[0-9]+):([A-Za-z]+[0-9]+)$")


def _col_to_num(col):
    n = 0
    for ch in col.upper():
        n = n * 26 + (ord(ch) - ord("A") + 1)
    return n


def _num_to_col(n):
    s = ""
    while n > 0:
        n, r = divmod(n - 1, 26)
        s = chr(ord("A") + r) + s
    return s


def _split_cell(name):
    m = _CELL_RE.match(name.strip())
    if not m:
        raise ValueError(f"bad cell name {name!r}")
    return m.group(1).upper(), int(m.group(2))


def _expand_range(spec):
    m = _RANGE_RE.match(spec.strip())
    if not m:
        raise ValueError(f"bad range {spec!r}")
    c1, r1 = _split_cell(m.group(1))
    c2, r2 = _split_cell(m.group(2))
    n1, n2 = _col_to_num(c1), _col_to_num(c2)
    lo_c, hi_c = min(n1, n2), max(n1, n2)
    lo_r, hi_r = min(r1, r2), max(r1, r2)
    cells = []
    for r in range(lo_r, hi_r + 1):
        for c in range(lo_c, hi_c + 1):
            cells.append(f"{_num_to_col(c)}{r}")
    return cells


# --- tokenizer ---------------------------------------------------------------

_TOKEN_RE = re.compile(
    r"""
      \s*(?:
        (?P<number>\d+\.\d+|\.\d+|\d+)
      | (?P<string>"(?:[^"\\]|\\.)*")
      | (?P<op><=|>=|!=|<>|=|<|>|\+|-|\*|/|\(|\)|,|:)
      | (?P<name>[A-Za-z][A-Za-z0-9]*)
      )
    """,
    re.VERBOSE,
)


def _tokenize(src):
    tokens = []
    pos = 0
    while pos < len(src):
        m = _TOKEN_RE.match(src, pos)
        if not m or m.end() == pos:
            if src[pos:].strip() == "":
                break
            raise ValueError(f"cannot tokenize at {src[pos:]!r}")
        pos = m.end()
        if m.group("number") is not None:
            tokens.append(("num", m.group("number")))
        elif m.group("string") is not None:
            raw = m.group("string")[1:-1]
            tokens.append(("str", raw.replace('\\"', '"').replace("\\\\", "\\")))
        elif m.group("op") is not None:
            tokens.append(("op", m.group("op")))
        elif m.group("name") is not None:
            tokens.append(("name", m.group("name")))
    return tokens


# --- recursive-descent parser → AST ------------------------------------------
#
# grammar (lowest to highest precedence):
#   expr    := compare
#   compare := add ( (=|!=|<|<=|>|>=) add )?
#   add     := mul ( (+|-) mul )*
#   mul     := unary ( (*|/) unary )*
#   unary   := (-)? primary
#   primary := number | string | call | range | cellref | ( expr )

_FUNCS = {"SUM", "MIN", "MAX", "AVG", "IF"}


class _Parser:
    def __init__(self, tokens):
        self.toks = tokens
        self.i = 0

    def _peek(self):
        return self.toks[self.i] if self.i < len(self.toks) else (None, None)

    def _next(self):
        t = self._peek()
        self.i += 1
        return t

    def _expect_op(self, op):
        kind, val = self._next()
        if kind != "op" or val != op:
            raise ValueError(f"expected {op!r}, got {val!r}")

    def parse(self):
        node = self._compare()
        if self.i != len(self.toks):
            raise ValueError("trailing tokens in formula")
        return node

    def _compare(self):
        left = self._add()
        kind, val = self._peek()
        if kind == "op" and val in ("=", "!=", "<>", "<", "<=", ">", ">="):
            self._next()
            right = self._add()
            return ("cmp", val, left, right)
        return left

    def _add(self):
        node = self._mul()
        while True:
            kind, val = self._peek()
            if kind == "op" and val in ("+", "-"):
                self._next()
                node = ("bin", val, node, self._mul())
            else:
                return node

    def _mul(self):
        node = self._unary()
        while True:
            kind, val = self._peek()
            if kind == "op" and val in ("*", "/"):
                self._next()
                node = ("bin", val, node, self._unary())
            else:
                return node

    def _unary(self):
        kind, val = self._peek()
        if kind == "op" and val == "-":
            self._next()
            return ("neg", self._unary())
        if kind == "op" and val == "+":
            self._next()
            return self._unary()
        return self._primary()

    def _primary(self):
        kind, val = self._next()
        if kind == "num":
            return ("num", val)
        if kind == "str":
            return ("str", val)
        if kind == "op" and val == "(":
            node = self._compare()
            self._expect_op(")")
            return node
        if kind == "name":
            up = val.upper()
            nkind, nval = self._peek()
            if nkind == "op" and nval == "(":
                self._next()
                if up == "IF":
                    args = self._arglist()
                    self._expect_op(")")
                    if len(args) != 3:
                        raise ValueError("IF takes 3 args")
                    return ("if", *args)
                if up in ("SUM", "MIN", "MAX", "AVG"):
                    node = self._range_arg()
                    self._expect_op(")")
                    return ("agg", up, node)
                raise ValueError(f"unknown function {val!r}")
            # bare name is a cell reference
            return ("ref", up)
        raise ValueError(f"unexpected token {val!r}")

    def _arglist(self):
        args = [self._compare()]
        while True:
            kind, val = self._peek()
            if kind == "op" and val == ",":
                self._next()
                args.append(self._compare())
            else:
                return args

    def _range_arg(self):
        # expect NAME : NAME
        kind, val = self._next()
        if kind != "name":
            raise ValueError("range requires a start cell")
        kind2, val2 = self._next()
        if kind2 != "op" or val2 != ":":
            raise ValueError("range requires ':'")
        kind3, val3 = self._next()
        if kind3 != "name":
            raise ValueError("range requires an end cell")
        return ("range", f"{val}:{val3}")


def _parse_formula(text):
    body = text[1:] if text.startswith("=") else text
    return _Parser(_tokenize(body)).parse()


# --- reference collection (for explain) --------------------------------------


def _collect_refs(node, acc):
    tag = node[0]
    if tag == "ref":
        acc.append(node[1])
    elif tag == "range":
        acc.extend(_expand_range(node[1]))
    elif tag == "agg":
        _collect_refs(node[2], acc)
    elif tag == "if":
        for child in node[1:]:
            _collect_refs(child, acc)
    elif tag in ("bin", "cmp"):
        _collect_refs(node[2], acc)
        _collect_refs(node[3], acc)
    elif tag == "neg":
        _collect_refs(node[1], acc)
    # num / str carry no refs


# --- evaluation engine -------------------------------------------------------


class _CircularError(Exception):
    def __init__(self, cell):
        super().__init__(f"circular reference at {cell}")
        self.cell = cell


def _to_number(v):
    """Coerce a cell value to a number for arithmetic; missing -> 0."""
    if v is None:
        return 0
    if isinstance(v, bool):
        return 1 if v else 0
    if isinstance(v, (int, float)):
        return v
    if isinstance(v, str):
        try:
            if "." in v:
                return float(v)
            return int(v)
        except ValueError:
            return 0
    return 0


def _num_literal(s):
    if "." in s:
        return float(s)
    return int(s)


class _Engine:
    """Evaluates cells lazily with memoization and cycle detection."""

    def __init__(self, cells):
        self.raw = cells
        self.cache = {}
        self.stack = []
        self.parsed = {}

    def _ast(self, name):
        if name not in self.parsed:
            self.parsed[name] = _parse_formula(self.raw[name])
        return self.parsed[name]

    def value(self, name):
        """Full evaluated value of a cell (formula result or literal)."""
        if name in self.cache:
            return self.cache[name]
        if name not in self.raw:
            # missing cell, referenced from a formula -> numeric 0
            return 0
        if name in self.stack:
            raise _CircularError(name)
        raw = self.raw[name]
        if isinstance(raw, str) and raw.startswith("="):
            self.stack.append(name)
            try:
                val = self._eval(self._ast(name))
            finally:
                self.stack.pop()
            self.cache[name] = val
            return val
        # literal
        self.cache[name] = raw
        return raw

    def _eval(self, node):
        tag = node[0]
        if tag == "num":
            return _num_literal(node[1])
        if tag == "str":
            return node[1]
        if tag == "ref":
            return self.value(node[1])
        if tag == "neg":
            return -_to_number(self._eval(node[1]))
        if tag == "bin":
            op = node[1]
            a = _to_number(self._eval(node[2]))
            b = _to_number(self._eval(node[3]))
            if op == "+":
                return a + b
            if op == "-":
                return a - b
            if op == "*":
                return a * b
            if op == "/":
                return a / b
        if tag == "cmp":
            op = node[1]
            a = self._eval(node[2])
            b = self._eval(node[3])
            a, b = self._coerce_pair(a, b)
            if op == "=":
                return a == b
            if op in ("!=", "<>"):
                return a != b
            if op == "<":
                return a < b
            if op == "<=":
                return a <= b
            if op == ">":
                return a > b
            if op == ">=":
                return a >= b
        if tag == "if":
            cond = self._eval(node[1])
            return self._eval(node[2]) if self._truthy(cond) else self._eval(node[3])
        if tag == "agg":
            fn = node[1]
            members = _expand_range(node[2][1])
            nums = [_to_number(self.value(m)) for m in members]
            if fn == "SUM":
                return sum(nums)
            if fn == "MIN":
                return min(nums) if nums else 0
            if fn == "MAX":
                return max(nums) if nums else 0
            if fn == "AVG":
                return sum(nums) / len(nums) if nums else 0
        raise ValueError(f"cannot evaluate node {node!r}")

    @staticmethod
    def _coerce_pair(a, b):
        # numeric comparison when both sides look numeric; else compare as-is
        an = isinstance(a, (int, float)) and not isinstance(a, bool)
        bn = isinstance(b, (int, float)) and not isinstance(b, bool)
        if an and bn:
            return a, b
        if isinstance(a, str) and isinstance(b, str):
            return a, b
        # mixed: fall back to numeric coercion
        return _to_number(a), _to_number(b)

    @staticmethod
    def _truthy(v):
        if isinstance(v, bool):
            return v
        if isinstance(v, (int, float)):
            return v != 0
        if isinstance(v, str):
            return v != ""
        return bool(v)


# --- public API --------------------------------------------------------------


def load_sheet(path):
    """Parse a sheet JSON file. Does NOT evaluate."""
    with open(path) as f:
        sheet = json.load(f)
    if "cells" not in sheet:
        sheet = {"cells": sheet}
    return sheet


def evaluate_sheet(sheet):
    """Evaluate every cell. Returns {"cells": {...}, "errors": {...}}."""
    cells = sheet.get("cells", {})
    engine = _Engine(cells)
    out_cells = {}
    errors = {}
    for name in cells:
        try:
            out_cells[name] = engine.value(name)
        except _CircularError as e:
            errors[name] = {"type": "circular", "cell": e.cell,
                            "message": str(e)}
        except Exception as e:  # noqa: BLE001 - structured, never raised out
            errors[name] = {"type": "error", "message": str(e)}
    result = {"cells": out_cells}
    if errors:
        result["errors"] = errors
    return result


def get_cell_value(sheet, cell):
    """Evaluated value of `cell`. Missing -> KeyError (reported as missing)."""
    cells = sheet.get("cells", {})
    if cell not in cells:
        raise KeyError(cell)
    engine = _Engine(cells)
    try:
        return engine.value(cell)
    except _CircularError as e:
        return {"type": "circular", "cell": e.cell, "message": str(e)}


def explain_cell(sheet, cell):
    """Describe how `cell` is computed: value + direct references."""
    cells = sheet.get("cells", {})
    if cell not in cells:
        return {"cell": cell, "value": None, "references": [], "missing": True}
    raw = cells[cell]
    refs = []
    if isinstance(raw, str) and raw.startswith("="):
        try:
            _collect_refs(_parse_formula(raw), refs)
        except Exception:  # noqa: BLE001 - explain never raises on parse issues
            refs = []
    # de-dupe while preserving order
    seen = set()
    ordered_refs = []
    for r in refs:
        if r not in seen:
            seen.add(r)
            ordered_refs.append(r)
    engine = _Engine(cells)
    info = {"cell": cell, "references": ordered_refs, "formula": raw}
    try:
        info["value"] = engine.value(cell)
    except _CircularError as e:
        info["value"] = None
        info["type"] = "circular"
        info["cell"] = cell
        info["circular_at"] = e.cell
    return info

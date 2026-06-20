"""repaircalc.public — a small recursive-descent arithmetic evaluator.

``evaluate(expr)`` parses and evaluates a numeric expression built from:

* integer and decimal literals (e.g. ``3``, ``3.5``, ``.5``)
* the binary operators ``+``, ``-``, ``*``, ``/``
* a leading unary ``-`` / ``+``
* parentheses for grouping

Standard precedence holds: ``*`` and ``/`` bind tighter than ``+`` and ``-``, and
all binary operators are LEFT-associative (so ``10 - 3 - 2 == 5`` and
``100 / 10 / 2 == 5``). It uses a hand-written tokenizer + recursive-descent
parser — never the built-in ``eval``.

Standard library only.
"""

from __future__ import annotations

from typing import List, Tuple, Union

Number = Union[int, float]


class CalcError(ValueError):
    """Raised when an expression is malformed or cannot be evaluated."""


# --- tokenizer ---------------------------------------------------------------

_OPS = set("+-*/()")


def _tokenize(expr: str) -> List[Tuple[str, object]]:
    """Turn ``expr`` into a list of ``(kind, value)`` tokens.

    ``kind`` is one of ``"num"``, ``"op"``. Whitespace is ignored.
    """
    tokens: List[Tuple[str, object]] = []
    i, n = 0, len(expr)
    while i < n:
        ch = expr[i]
        if ch.isspace():
            i += 1
            continue
        if ch in _OPS:
            tokens.append(("op", ch))
            i += 1
            continue
        if ch.isdigit() or ch == ".":
            j = i
            seen_dot = False
            while j < n and (expr[j].isdigit() or expr[j] == "."):
                if expr[j] == ".":
                    if seen_dot:
                        raise CalcError(f"malformed number near {expr[i:j+1]!r}")
                    seen_dot = True
                j += 1
            text = expr[i:j]
            if text == ".":
                raise CalcError("lone '.' is not a number")
            tokens.append(("num", _parse_number(text)))
            i = j
            continue
        raise CalcError(f"unexpected character {ch!r} at position {i}")
    return tokens


def _parse_number(text: str) -> Number:
    """Parse a literal as ``int`` when it has no decimal point, else ``float``."""
    if "." in text:
        return float(text)
    return int(text)


# --- recursive-descent parser/evaluator --------------------------------------

class _Parser:
    """Grammar (precedence climbing):

        expr   := term  (('+' | '-') term)*      # left-assoc, lowest precedence
        term   := factor (('*' | '/') factor)*   # left-assoc, higher precedence
        factor := ('+' | '-') factor | atom      # unary sign
        atom   := NUMBER | '(' expr ')'
    """

    def __init__(self, tokens: List[Tuple[str, object]]) -> None:
        self.tokens = tokens
        self.pos = 0

    def _peek(self):
        return self.tokens[self.pos] if self.pos < len(self.tokens) else None

    def _advance(self):
        tok = self.tokens[self.pos]
        self.pos += 1
        return tok

    def parse(self) -> Number:
        if not self.tokens:
            raise CalcError("empty expression")
        value = self._expr()
        if self.pos != len(self.tokens):
            raise CalcError(f"unexpected trailing token {self._peek()!r}")
        return value

    def _expr(self) -> Number:
        value = self._term()
        while True:
            tok = self._peek()
            if tok == ("op", "+"):
                self._advance()
                value = value + self._term()
            elif tok == ("op", "-"):
                self._advance()
                value = value - self._term()
            else:
                break
        return value

    def _term(self) -> Number:
        value = self._factor()
        while True:
            tok = self._peek()
            if tok == ("op", "*"):
                self._advance()
                value = value * self._factor()
            elif tok == ("op", "/"):
                self._advance()
                divisor = self._factor()
                if divisor == 0:
                    raise CalcError("division by zero")
                value = value / divisor
            else:
                break
        return value

    def _factor(self) -> Number:
        tok = self._peek()
        if tok == ("op", "+"):
            self._advance()
            return self._factor()
        if tok == ("op", "-"):
            self._advance()
            return -self._factor()
        return self._atom()

    def _atom(self) -> Number:
        tok = self._peek()
        if tok is None:
            raise CalcError("unexpected end of expression")
        kind, val = tok
        if kind == "num":
            self._advance()
            return val
        if tok == ("op", "("):
            self._advance()
            value = self._expr()
            if self._peek() != ("op", ")"):
                raise CalcError("missing closing parenthesis")
            self._advance()
            return value
        raise CalcError(f"unexpected token {tok!r}")


def evaluate(expr: str) -> Number:
    """Evaluate the arithmetic expression ``expr`` and return its numeric value.

    Returns an ``int`` when the result is an exact integer that involved no
    division, and a ``float`` otherwise. Raises :class:`CalcError` on malformed
    input or division by zero.
    """
    if not isinstance(expr, str):
        raise CalcError(f"expression must be a string, got {type(expr).__name__}")
    tokens = _tokenize(expr)
    return _Parser(tokens).parse()

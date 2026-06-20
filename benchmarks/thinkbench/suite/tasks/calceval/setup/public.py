"""calceval.public — a small infix arithmetic evaluator (stdlib only).

``evaluate(expr)`` parses and evaluates an infix arithmetic expression and
returns the result as a ``float``. It does NOT use ``eval`` / ``exec``; it is a
hand-written recursive-descent parser.

Supported grammar:

* ``+`` and ``-`` (binary): lowest precedence, LEFT-associative.
* ``*`` and ``/``: middle precedence, LEFT-associative.
* unary ``-`` (prefix negation).
* ``^`` (exponent): highest precedence, RIGHT-associative.
* parentheses ``( ... )`` override precedence.

Numbers may be integers or decimals (``3``, ``3.5``, ``.5``, ``10.``). The result
is always a ``float``. Whitespace is insignificant.

Malformed input (empty/blank expression, unbalanced parens, a stray operator, a
bad token, division by zero) raises :class:`CalcError`.

Standard library only.
"""

from __future__ import annotations

from typing import List


class CalcError(ValueError):
    """Raised when an expression is malformed or cannot be evaluated."""


# --- tokenizer ---------------------------------------------------------------
_OPS = set("+-*/^()")


def _tokenize(expr: str) -> List[str]:
    """Split ``expr`` into number / operator / paren tokens."""
    tokens: List[str] = []
    i = 0
    n = len(expr)
    while i < n:
        ch = expr[i]
        if ch.isspace():
            i += 1
            continue
        if ch in _OPS:
            tokens.append(ch)
            i += 1
            continue
        if ch.isdigit() or ch == ".":
            j = i
            seen_dot = False
            while j < n and (expr[j].isdigit() or expr[j] == "."):
                if expr[j] == ".":
                    if seen_dot:
                        raise CalcError(f"malformed number near {expr[i:j + 1]!r}")
                    seen_dot = True
                j += 1
            num = expr[i:j]
            if num == ".":
                raise CalcError("malformed number '.'")
            tokens.append(num)
            i = j
            continue
        raise CalcError(f"unexpected character {ch!r}")
    return tokens


# --- recursive-descent parser ------------------------------------------------
class _Parser:
    def __init__(self, tokens: List[str]) -> None:
        self.tokens = tokens
        self.pos = 0

    def _peek(self):
        return self.tokens[self.pos] if self.pos < len(self.tokens) else None

    def _advance(self) -> str:
        tok = self.tokens[self.pos]
        self.pos += 1
        return tok

    def parse(self) -> float:
        if not self.tokens:
            raise CalcError("empty expression")
        value = self._expr()
        if self.pos != len(self.tokens):
            raise CalcError(f"unexpected token {self._peek()!r}")
        return value

    def _expr(self) -> float:
        # '+' / '-' at the lowest precedence. We grab the left term and, if an
        # operator follows, combine it with the REST of the expression.
        value = self._term()
        if self._peek() in ("+", "-"):
            op = self._advance()
            rhs = self._expr()
            return value + rhs if op == "+" else value - rhs
        return value

    def _term(self) -> float:
        # '*' / '/' in the middle. Same shape: left factor combined with the
        # rest of the term.
        value = self._unary()
        if self._peek() in ("*", "/"):
            op = self._advance()
            rhs = self._term()
            if op == "*":
                return value * rhs
            if rhs == 0:
                raise CalcError("division by zero")
            return value / rhs
        return value

    def _unary(self) -> float:
        # Prefix '-' / '+': negate the atom, then let '^' apply on top.
        if self._peek() == "-":
            self._advance()
            return self._power(-self._atom())
        if self._peek() == "+":
            self._advance()
            return self._power(self._atom())
        return self._power(self._atom())

    def _power(self, base: float) -> float:
        # '^' at the highest precedence. Fold each exponent onto the running
        # base as we scan left to right.
        value = base
        while self._peek() == "^":
            self._advance()
            exponent = self._atom()
            value = float(value ** exponent)
        return value

    def _atom(self) -> float:
        tok = self._peek()
        if tok is None:
            raise CalcError("unexpected end of expression")
        if tok == "(":
            self._advance()
            value = self._expr()
            if self._peek() != ")":
                raise CalcError("missing closing parenthesis")
            self._advance()
            return value
        if tok in _OPS:
            raise CalcError(f"unexpected operator {tok!r}")
        self._advance()
        try:
            return float(tok)
        except ValueError:
            raise CalcError(f"invalid number {tok!r}")


def evaluate(expr: str) -> float:
    """Evaluate the infix arithmetic ``expr`` and return its value as a float."""
    if not isinstance(expr, str):
        raise CalcError(f"expr must be a str, got {type(expr).__name__}")
    tokens = _tokenize(expr)
    result = _Parser(tokens).parse()
    return float(result)

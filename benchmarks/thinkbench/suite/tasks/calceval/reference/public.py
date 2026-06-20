"""calceval.public — a small infix arithmetic evaluator (stdlib only).

``evaluate(expr)`` parses and evaluates an infix arithmetic expression and
returns the result as a ``float``. It does NOT use ``eval`` / ``exec``; it is a
hand-written recursive-descent parser.

Supported grammar (highest precedence last binds tightest):

* ``+`` and ``-`` (binary): lowest precedence, LEFT-associative.
* ``*`` and ``/``: middle precedence, LEFT-associative.
* unary ``-`` (prefix negation): binds LOOSER than ``^`` so ``-2^2 == -(2^2)``.
* ``^`` (exponent): highest precedence, RIGHT-associative so ``2^3^2 == 2^(3^2)``.
* parentheses ``( ... )`` override precedence.

Numbers may be integers or decimals (``3``, ``3.5``, ``.5``, ``10.``). The result
is always a ``float``. Whitespace is insignificant.

Malformed input (empty/blank expression, unbalanced parens, a stray operator, a
bad token, division by zero) raises :class:`CalcError`.

Standard library only.

This is the reference (fixed) solution.
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
# The parser is a classic precedence cascade:
#   expr  := term  (('+' | '-') term)*          left-assoc, lowest precedence
#   term  := unary (('*' | '/') unary)*         left-assoc
#   unary := '-' unary | power                  prefix minus, binds looser than ^
#   power := atom ('^' unary)?                  right-assoc, highest precedence
#   atom  := NUMBER | '(' expr ')'
#
# Note that ``power``'s right operand recurses back into ``unary`` (not into
# ``power``): that is what makes ``^`` right-associative AND lets ``2^-3`` work
# while still keeping ``-2^2`` parse as ``-(2^2)``.


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
        # LEFT-associative '+' / '-': fold the accumulator on the LEFT.
        value = self._term()
        while self._peek() in ("+", "-"):
            op = self._advance()
            rhs = self._term()
            value = value + rhs if op == "+" else value - rhs
        return value

    def _term(self) -> float:
        # LEFT-associative '*' / '/'.
        value = self._unary()
        while self._peek() in ("*", "/"):
            op = self._advance()
            rhs = self._unary()
            if op == "*":
                value = value * rhs
            else:
                if rhs == 0:
                    raise CalcError("division by zero")
                value = value / rhs
        return value

    def _unary(self) -> float:
        # Prefix '-' binds LOOSER than '^': '-2^2' is '-(2^2)'. We negate the
        # result of a full ``power`` (which consumes the '^'), not just the atom.
        if self._peek() == "-":
            self._advance()
            return -self._unary()
        if self._peek() == "+":  # tolerate a unary plus
            self._advance()
            return self._unary()
        return self._power()

    def _power(self) -> float:
        base = self._atom()
        if self._peek() == "^":
            self._advance()
            # RIGHT-associative: recurse into ``unary`` so '2^3^2' == '2^(3^2)'
            # and '2^-1' is allowed.
            exponent = self._unary()
            return float(base ** exponent)
        return base

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

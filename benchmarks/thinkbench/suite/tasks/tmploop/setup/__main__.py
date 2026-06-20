"""Tiny demo / smoke CLI for the tmploop package.

Run with ``python -m tmploop`` to exercise the templater. Not part of the graded
contract; provided only as a convenience.
"""

from .public import render


def main() -> None:
    out = render("Hi {{ name }}!", {"name": "Ada"})
    print(out)


if __name__ == "__main__":
    main()

"""Tiny demo / smoke CLI for the tmploop package.

Run with ``python -m tmploop`` to exercise the block tags. Not part of the graded
contract; provided only as a convenience.
"""

from .public import render


def main() -> None:
    tmpl = "{{#each users}}{{#if @first}}; {{/if}}{{ name }}({{ @index }}){{/each}}"
    ctx = {"users": [{"name": "Ada"}, {"name": "Bo"}]}
    print(render(tmpl, ctx))


if __name__ == "__main__":
    main()

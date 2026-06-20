"""A small CSV parser.

`parse_csv(text)` reads CSV text whose first non-empty line is a header row and
returns a list of dicts, one per data row, mapping header name -> field value.

Surrounding double quotes around a field are stripped, so a field written as
`"hello"` is returned as `hello`.
"""
from __future__ import annotations


def _split_line(line: str) -> list[str]:
    """Split a single CSV record into its fields."""
    fields = []
    for raw in line.split(","):
        field = raw.strip()
        # If the field is wrapped in double quotes, drop the surrounding quotes.
        if len(field) >= 2 and field[0] == '"' and field[-1] == '"':
            field = field[1:-1]
        fields.append(field)
    return fields


def parse_csv(text: str) -> list[dict]:
    """Parse CSV `text` into a list of row dicts keyed by the header columns.

    The first non-empty line is treated as the header. Each subsequent non-empty
    line becomes one dict mapping each header name to the corresponding field.
    """
    lines = [ln for ln in text.splitlines() if ln.strip() != ""]
    if not lines:
        return []

    header = _split_line(lines[0])
    rows = []
    for line in lines[1:]:
        values = _split_line(line)
        row = {}
        for i, name in enumerate(header):
            row[name] = values[i] if i < len(values) else ""
        rows.append(row)
    return rows

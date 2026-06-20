"""A small CSV parser (RFC-4180-style).

`parse_csv(text)` reads CSV text whose first non-empty line is a header row and
returns a list of dicts, one per data row, mapping header name -> field value.

Quoting rules:
  - A field may be wrapped in double quotes. Surrounding quotes are stripped.
  - A comma inside a quoted field is part of the value, not a separator.
  - A doubled quote (``""``) inside a quoted field is an escaped literal quote.
"""
from __future__ import annotations


def _parse_records(text: str) -> list[list[str]]:
    """Tokenize CSV `text` into a list of records (each a list of field strings).

    Uses a small state machine so commas (and line breaks) inside quoted fields
    are not treated as separators, and ``""`` inside a quoted field becomes a
    single literal quote.
    """
    records: list[list[str]] = []
    fields: list[str] = []
    field_chars: list[str] = []
    in_quotes = False
    started = False  # whether the current record has any content yet

    i = 0
    n = len(text)
    while i < n:
        ch = text[i]
        if in_quotes:
            if ch == '"':
                # A doubled quote inside a quoted field is one literal quote.
                if i + 1 < n and text[i + 1] == '"':
                    field_chars.append('"')
                    i += 2
                    continue
                in_quotes = False
                i += 1
                continue
            field_chars.append(ch)
            i += 1
            continue

        # Not inside quotes.
        if ch == '"':
            in_quotes = True
            started = True
            i += 1
            continue
        if ch == ",":
            fields.append("".join(field_chars))
            field_chars = []
            started = True
            i += 1
            continue
        if ch in ("\n", "\r"):
            # Consume \r\n as a single record terminator.
            if ch == "\r" and i + 1 < n and text[i + 1] == "\n":
                i += 1
            if started or field_chars or fields:
                fields.append("".join(field_chars))
                records.append(fields)
            fields = []
            field_chars = []
            started = False
            i += 1
            continue
        field_chars.append(ch)
        started = True
        i += 1

    # Flush a trailing record with no final newline.
    if started or field_chars or fields:
        fields.append("".join(field_chars))
        records.append(fields)
    return records


def parse_csv(text: str) -> list[dict]:
    """Parse CSV `text` into a list of row dicts keyed by the header columns.

    The first record is treated as the header. Each subsequent record becomes one
    dict mapping each header name to the corresponding field. Commas inside quoted
    fields and doubled-quote escapes are handled per RFC 4180.
    """
    records = _parse_records(text)
    # Drop fully-empty records (e.g. blank lines).
    records = [r for r in records if not (len(r) == 1 and r[0] == "")]
    if not records:
        return []

    header = records[0]
    rows = []
    for values in records[1:]:
        row = {}
        for idx, name in enumerate(header):
            row[name] = values[idx] if idx < len(values) else ""
        rows.append(row)
    return rows

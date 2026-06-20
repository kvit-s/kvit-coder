"""A small in-memory search index (with prefix-query support).

`SearchIndex` indexes short text documents and answers term queries.

  - ``add_document(doc_id, text)`` tokenizes ``text`` into lowercase terms and
    records, per term, how many times it appears in that document.
  - ``search(query)`` tokenizes the query the same way and returns the ids of
    documents that match EVERY query token (case-insensitive). Results are ranked
    by total term frequency (highest first); ties break by ascending document id
    so the order is deterministic.

Query tokens come in two flavours:

  - A PLAIN token (e.g. ``pay``) is an exact whole-term match: it matches a
    document term equal to ``pay`` only — NOT ``payment``.
  - A PREFIX token ends with ``*`` (e.g. ``pay*``): it matches a document if any
    of that document's terms STARTS WITH the prefix ``pay``. Its frequency
    contribution is the summed count of every term in the document that starts
    with the prefix.

The trailing ``*`` is the prefix marker only; a token of just ``*`` (empty
prefix) is ignored.
"""
from __future__ import annotations

import re

_TOKEN_RE = re.compile(r"[a-z0-9]+")


def _tokenize(text: str) -> list[str]:
    """Split ``text`` into lowercase alphanumeric terms."""
    return _TOKEN_RE.findall(text.lower())


def _parse_query(query: str) -> list[tuple[str, bool]]:
    """Parse ``query`` into (term, is_prefix) tokens.

    A token written ``foo*`` becomes ``("foo", True)``; a plain ``foo`` becomes
    ``("foo", False)``. The ``*`` marker is detected on the raw whitespace-split
    token BEFORE alphanumeric tokenization (so the ``*`` itself is not a term
    character). A bare ``*`` (empty prefix) is dropped.
    """
    tokens: list[tuple[str, bool]] = []
    for raw in query.lower().split():
        is_prefix = raw.endswith("*")
        # Pull the alphanumeric core out of the raw token (drops the trailing
        # ``*`` and any other punctuation). A raw token may yield 0 or 1 cores;
        # we use the first if present.
        cores = _TOKEN_RE.findall(raw)
        if not cores:
            continue
        term = cores[0]
        tokens.append((term, is_prefix))
    return tokens


class SearchIndex:
    """An in-memory inverted index over added documents."""

    def __init__(self) -> None:
        # doc_id -> {term -> count}
        self._docs: dict[object, dict[str, int]] = {}

    def add_document(self, doc_id: object, text: str) -> None:
        """Index ``text`` under ``doc_id``.

        Re-adding the same ``doc_id`` replaces its previous content.
        """
        counts: dict[str, int] = {}
        for term in _tokenize(text):
            counts[term] = counts.get(term, 0) + 1
        self._docs[doc_id] = counts

    def search(self, query: str) -> list:
        """Return the ids of documents matching every token in ``query``.

        Plain tokens match exactly; tokens ending in ``*`` match any document
        term sharing that prefix. Results are ranked by total term frequency
        (descending); ties break by ascending doc id.
        """
        tokens = _parse_query(query)
        if not tokens:
            return []

        scored: list[tuple[int, object]] = []
        for doc_id, counts in self._docs.items():
            total = 0
            matched_all = True
            for term, is_prefix in tokens:
                contribution = _token_score(counts, term, is_prefix)
                if contribution == 0:
                    matched_all = False
                    break
                total += contribution
            if matched_all:
                scored.append((total, doc_id))

        # Highest score first; ties broken by ascending doc id for determinism.
        scored.sort(key=lambda pair: (-pair[0], _sort_key(pair[1])))
        return [doc_id for _score, doc_id in scored]


def _token_score(counts: dict[str, int], term: str, is_prefix: bool) -> int:
    """Frequency contribution of one query token within a document.

    For a plain token, the count of the exact term (0 if absent). For a prefix
    token, the summed count of every document term starting with ``term``.
    """
    if is_prefix:
        return sum(c for t, c in counts.items() if t.startswith(term))
    return counts.get(term, 0)


def _sort_key(doc_id: object):
    """Stable, type-tolerant sort key for deterministic tie-breaking."""
    return (type(doc_id).__name__, str(doc_id))

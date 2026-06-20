"""A small in-memory search index.

`SearchIndex` indexes short text documents and answers term queries.

  - ``add_document(doc_id, text)`` tokenizes ``text`` into lowercase terms and
    records, per term, how many times it appears in that document.
  - ``search(query)`` tokenizes the query the same way and returns the ids of
    documents that contain EVERY query term (case-insensitive, exact-term
    matching). Results are ranked by total term frequency (the summed count, over
    the query terms, of how often those terms appear in the document), highest
    first. Ties break by ascending document id so the order is deterministic.

Only exact whole-term matches are supported: searching for ``pay`` matches a
document containing the term ``pay`` but NOT one containing only ``payment``.
"""
from __future__ import annotations

import re

_TOKEN_RE = re.compile(r"[a-z0-9]+")


def _tokenize(text: str) -> list[str]:
    """Split ``text`` into lowercase alphanumeric terms."""
    return _TOKEN_RE.findall(text.lower())


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
        """Return the ids of documents matching every term in ``query``.

        Matching is case-insensitive, exact-term only. Results are ranked by
        total term frequency (descending); ties break by ascending doc id.
        """
        terms = _tokenize(query)
        if not terms:
            return []

        scored: list[tuple[int, object]] = []
        for doc_id, counts in self._docs.items():
            # Every query term must appear in the document (exact match).
            if not all(term in counts for term in terms):
                continue
            score = sum(counts[term] for term in terms)
            scored.append((score, doc_id))

        # Highest score first; ties broken by ascending doc id for determinism.
        scored.sort(key=lambda pair: (-pair[0], _sort_key(pair[1])))
        return [doc_id for _score, doc_id in scored]


def _sort_key(doc_id: object):
    """Stable, type-tolerant sort key for deterministic tie-breaking."""
    return (type(doc_id).__name__, str(doc_id))

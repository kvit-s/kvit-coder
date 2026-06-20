"""searchprefix — a small in-memory search index.

Public API lives in `searchprefix.public`. Import `SearchIndex` from there.
"""
from .public import SearchIndex

__all__ = ["SearchIndex"]

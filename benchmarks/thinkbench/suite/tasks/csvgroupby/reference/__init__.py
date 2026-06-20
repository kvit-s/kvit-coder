"""csvgroupby — a tiny in-memory CSV query engine (reference, WITH GROUP BY).

Public API lives in `csvgroupby.public`. Import `query` from there.
"""
from .public import query

__all__ = ["query"]

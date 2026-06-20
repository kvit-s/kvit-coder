"""csvgroupby — a tiny in-memory CSV query engine.

Public API lives in `csvgroupby.public`. Import `query` from there:

    from csvgroupby import query
    rows = [{"city": "NYC", "age": "30"}, ...]
    out = query(rows, "SELECT city FROM t WHERE age >= 18")
"""
from .public import query

__all__ = ["query"]

"""kvtxn — a tiny in-memory key-value store with NESTED transactions.

Public API is re-exported here for convenience; the implementation lives in
``kvtxn.public``.

    >>> from kvtxn import Store
    >>> s = Store()
    >>> s.set("k", "v")
    >>> s.get("k")
    'v'
"""

from .public import Store, TransactionError

__all__ = ["Store", "TransactionError"]

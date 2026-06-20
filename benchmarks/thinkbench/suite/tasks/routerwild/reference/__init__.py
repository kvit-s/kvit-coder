"""routerwild — a tiny pure-in-process path router.

Public API lives in `routerwild.public`. Import `Router` from there.

    >>> from routerwild import Router
    >>> r = Router()
    >>> r.add("/users/{id}", "user")
    >>> r.match("/users/42")
    ('user', {'id': '42'})
"""
from .public import Router

__all__ = ["Router"]

"""repairpager — a tiny pagination helper over an in-memory sequence.

Public API lives in :mod:`repairpager.public`. The single entry point is
:func:`paginate`, which slices a list of items into a 1-based page and returns
the page's items alongside pagination metadata (``total_items``,
``total_pages``, ``page``, ``has_next``, ``has_prev``).

Standard library only. This is the reference (fixed) solution; it is NOT shown
to the agent and ships without the visible test file.
"""

from .public import paginate

__all__ = ["paginate"]

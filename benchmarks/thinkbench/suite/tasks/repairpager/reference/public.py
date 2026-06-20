"""repairpager.public — slice a sequence into 1-based pages with metadata.

``paginate(items, page_size, page)`` returns the requested page's items plus
pagination metadata. Page numbers are 1-based: ``page=1`` is the first page.

Return shape::

    {
        "items": list,        # the items on this page (possibly empty)
        "total_items": int,   # len(items)
        "total_pages": int,   # number of pages needed to cover all items
        "page": int,          # the (clamped) 1-based page actually returned
        "has_next": bool,     # True if a page after this one exists
        "has_prev": bool,     # True if a page before this one exists
    }

Standard library only.
"""

from __future__ import annotations

from typing import Any, Dict, List, Sequence


def paginate(items: Sequence[Any], page_size: int, page: int = 1) -> Dict[str, Any]:
    """Return ``page`` (1-based) of ``items`` in chunks of ``page_size``.

    ``page_size`` must be a positive integer. ``page`` is clamped into the
    valid range ``[1, total_pages]`` (an empty input still has one page).
    """
    if not isinstance(page_size, int) or isinstance(page_size, bool) or page_size <= 0:
        raise ValueError("page_size must be a positive integer")

    seq: List[Any] = list(items)
    total_items = len(seq)

    # total number of pages needed to cover every item; round UP so a trailing
    # partial page counts. An empty sequence is still a single (empty) page so
    # that page 1 is always valid.
    total_pages = max(1, (total_items + page_size - 1) // page_size)

    # clamp the requested page into [1, total_pages]
    if page < 1:
        page = 1
    elif page > total_pages:
        page = total_pages

    start = (page - 1) * page_size
    end = start + page_size
    page_items = seq[start:end]

    return {
        "items": page_items,
        "total_items": total_items,
        "total_pages": total_pages,
        "page": page,
        "has_next": page < total_pages,
        "has_prev": page > 1,
    }

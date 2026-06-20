"""Reference slotfinder.public — a meeting availability engine.

Finds non-overlapping candidate meeting windows where every participant is within
their local working hours AND free of busy intervals, over a UTC search range, with
configurable granularity. Timezones and DST are handled with the standard-library
``zoneinfo``; all instant comparisons happen in UTC.

This file is the intended solution; it is NOT shown to the model. It anchors what
"correct" means and self-tests the held-out grader.
"""
from datetime import datetime, time, timedelta, timezone
from zoneinfo import ZoneInfo

UTC = timezone.utc


def _parse_instant(s):
    """Parse an ISO-8601 instant to an aware UTC datetime.

    Accepts a trailing ``Z`` (which ``datetime.fromisoformat`` rejects before 3.11
    and which we normalise for safety) or an explicit offset. A naive timestamp is
    treated as UTC.
    """
    if isinstance(s, datetime):
        dt = s
    else:
        text = s.strip()
        if text.endswith(("Z", "z")):
            text = text[:-1] + "+00:00"
        dt = datetime.fromisoformat(text)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=UTC)
    return dt.astimezone(UTC)


def _parse_hhmm(s):
    h, m = s.split(":")
    return time(int(h), int(m))


def _zone_for(participant, default_zone_name):
    name = participant.get("timezone", default_zone_name)
    return ZoneInfo(name)


def _to_utc_iso(dt):
    """ISO-8601 UTC string for an aware datetime (canonical ``+00:00`` form)."""
    return dt.astimezone(UTC).isoformat()


def _within_working_hours(start_utc, end_utc, blocks, zone):
    """True iff the whole half-open window ``[start_utc, end_utc)`` lies inside a
    single working-hours block, evaluated in ``zone`` local time.

    A block lists ISO weekday ints (Mon=1..Sun=7) and a local ``[start, end)``
    wall-clock interval on each listed day. Because the bounds are computed from the
    LOCAL date, the corresponding UTC instants shift across DST automatically.
    """
    if not blocks:
        return False
    local_start = start_utc.astimezone(zone)
    # The window's local end is derived from the local start + duration so a window
    # is judged against the local day it begins on (windows never cross local days
    # at the granularities this engine targets; if they did, the per-day block check
    # below would simply reject them).
    duration = end_utc - start_utc
    iso_weekday = local_start.isoweekday()
    for block in blocks:
        if iso_weekday not in block.get("days", []):
            continue
        bstart = _parse_hhmm(block["start"])
        bend = _parse_hhmm(block["end"])
        if bend <= bstart:
            continue  # empty / non-positive block
        local_day = local_start.date()
        # Construct the block's local boundaries on this day, then convert to UTC.
        block_start_local = datetime.combine(local_day, bstart, tzinfo=zone)
        block_end_local = datetime.combine(local_day, bend, tzinfo=zone)
        block_start_utc = block_start_local.astimezone(UTC)
        block_end_utc = block_end_local.astimezone(UTC)
        if block_start_utc <= start_utc and end_utc <= block_end_utc:
            # Re-check the window does not span into a different local day's logic
            # by confirming its local end also falls on/within the same block window.
            if (local_start + duration) <= block_end_local:
                return True
    return False


def _is_busy(start_utc, end_utc, busy_intervals):
    """True iff the window intersects any busy interval (half-open, endpoint-touch
    is not a conflict)."""
    for iv in busy_intervals:
        b_start = _parse_instant(iv["start"])
        b_end = _parse_instant(iv["end"])
        if start_utc < b_end and b_start < end_utc:
            return True
    return False


def find_slots(request):
    duration = int(request["duration_minutes"])
    granularity = int(request.get("granularity_minutes", 15))
    if duration <= 0 or granularity <= 0:
        return []

    default_zone_name = request.get("timezone")
    search_start = _parse_instant(request["search_start"])
    search_end = _parse_instant(request["search_end"])

    participants = request.get("participants", [])
    # Pre-resolve each participant's zone + parsed busy list once.
    resolved = []
    for p in participants:
        resolved.append(
            (
                _zone_for(p, default_zone_name),
                p.get("working_hours", []),
                p.get("busy", []),
            )
        )

    step = timedelta(minutes=granularity)
    dur = timedelta(minutes=duration)

    slots = []
    t = search_start
    while t + dur <= search_end:
        end = t + dur
        ok = True
        for zone, blocks, busy in resolved:
            if not _within_working_hours(t, end, blocks, zone):
                ok = False
                break
            if _is_busy(t, end, busy):
                ok = False
                break
        if ok:
            slots.append({"start": _to_utc_iso(t), "end": _to_utc_iso(end)})
            # Greedy non-overlapping packing: advance to the first grid point at or
            # after this window's end.
            t = end
            # Re-align to the granularity grid measured from search_start.
            elapsed = t - search_start
            rem = elapsed % step
            if rem:
                t = t + (step - rem)
        else:
            t = t + step

    return slots

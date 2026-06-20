#!/usr/bin/env python3
"""Held-out behavior-level oracle for the greenfield `slotfinder` task.

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `slotfinder.public`
API: `find_slots(request) -> list[dict]`, and the `python -m slotfinder find` CLI),
NOT against the model's own tests and NOT against any particular internal layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

FIXED DENOMINATOR: the set of check ids is constant. If the package fails to import,
every check is recorded as FAILED (with the import error) and the score is forced to
0.0 — a model cannot shrink the denominator by failing to ship an importable package.

DERIVE, don't REQUIRE: the oracle recomputes the expected slot set itself from each
request (independently, with zoneinfo/DST handled exactly), then compares the model's
output at the INSTANT level — `Z` vs `+00:00` formatting is irrelevant, only the moment
in time matters. It never inspects internal file layout or helper names. Spots where it
assumes a convention the brief does not pin are marked `# ASSUMES`.
"""
import importlib
import json
import os
import subprocess
import sys
import tempfile
from datetime import datetime, time, timedelta, timezone
from zoneinfo import ZoneInfo

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

UTC = timezone.utc

# ---------------------------------------------------------------------------
# The FIXED list of check ids (denominator is constant regardless of import).
# ---------------------------------------------------------------------------
CHECK_IDS = [
    ("basic_availability", "single-participant slots match the derived expected set"),
    ("busy_excludes", "a busy interval removes exactly the windows it overlaps"),
    ("busy_endpoint_touch", "a window touching a busy interval at an endpoint is NOT blocked"),
    ("working_hour_boundary", "no slot falls outside the local [start, end) working hours"),
    ("multi_participant", "only the intersection of all participants' availability is returned"),
    ("multi_block", "multiple working-hour blocks union correctly"),
    ("dst_transition", "working hours track local wall-clock across a DST transition"),
    ("no_availability", "a fully-busy / out-of-hours request returns an empty list"),
    ("granularity_default", "default granularity is 15 minutes"),
    ("granularity_custom", "a custom granularity changes the candidate grid"),
    ("non_overlapping", "returned slots never overlap each other"),
    ("deterministic_order", "slots are sorted ascending by start and stable across runs"),
    ("slot_shape", "each slot is {start, end} ISO-UTC with length == duration_minutes"),
    ("cli_json", "`python -m slotfinder find <file>` emits a JSON list of slots"),
]

checks = []


def record(cid, desc, ok, detail):
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# ---------------------------------------------------------------------------
# Instant-level helpers (the oracle compares moments, not strings).
# ---------------------------------------------------------------------------
def parse_instant(s):
    """Parse an ISO-8601 instant (Z or offset, with or without T) to aware UTC."""
    if isinstance(s, datetime):
        dt = s
    else:
        text = str(s).strip()
        if text.endswith(("Z", "z")):
            text = text[:-1] + "+00:00"
        dt = datetime.fromisoformat(text)
    if dt.tzinfo is None:
        dt = dt.replace(tzinfo=UTC)
    return dt.astimezone(UTC)


def parse_hhmm(s):
    h, m = str(s).split(":")
    return time(int(h), int(m))


def slot_pair(slot):
    """(start_instant, end_instant) for a model-produced slot dict, or raise."""
    return (parse_instant(slot["start"]), parse_instant(slot["end"]))


def to_pairs(slots):
    return [slot_pair(s) for s in slots]


# ---------------------------------------------------------------------------
# Independent oracle: recompute the expected slot set from a request.
# This mirrors the Contract semantics but is written standalone so it is a true
# cross-check, not a call into the model's code.
# ---------------------------------------------------------------------------
def _within_hours(start_utc, end_utc, blocks, zone):
    if not blocks:
        return False
    local_start = start_utc.astimezone(zone)
    duration = end_utc - start_utc
    iso_wd = local_start.isoweekday()
    for block in blocks:
        if iso_wd not in block.get("days", []):
            continue
        bstart, bend = parse_hhmm(block["start"]), parse_hhmm(block["end"])
        if bend <= bstart:
            continue
        day = local_start.date()
        bs_local = datetime.combine(day, bstart, tzinfo=zone)
        be_local = datetime.combine(day, bend, tzinfo=zone)
        bs_utc, be_utc = bs_local.astimezone(UTC), be_local.astimezone(UTC)
        if bs_utc <= start_utc and end_utc <= be_utc and (local_start + duration) <= be_local:
            return True
    return False


def _busy(start_utc, end_utc, busy):
    for iv in busy:
        b_s, b_e = parse_instant(iv["start"]), parse_instant(iv["end"])
        if start_utc < b_e and b_s < end_utc:
            return True
    return False


def expected_slots(request):
    """Return the canonical expected slot set as a list of (start, end) UTC pairs."""
    duration = int(request["duration_minutes"])
    gran = int(request.get("granularity_minutes", 15))
    if duration <= 0 or gran <= 0:
        return []
    default_zone = request.get("timezone")
    s_start = parse_instant(request["search_start"])
    s_end = parse_instant(request["search_end"])
    resolved = []
    for p in request.get("participants", []):
        zone = ZoneInfo(p.get("timezone", default_zone))
        resolved.append((zone, p.get("working_hours", []), p.get("busy", [])))

    step = timedelta(minutes=gran)
    dur = timedelta(minutes=duration)
    out = []
    t = s_start
    while t + dur <= s_end:
        end = t + dur
        ok = all(
            _within_hours(t, end, blocks, zone) and not _busy(t, end, busy)
            for zone, blocks, busy in resolved
        )
        if ok:
            out.append((t, end))
            t = end
            elapsed = t - s_start
            rem = elapsed % step
            if rem:
                t = t + (step - rem)
        else:
            t = t + step
    return out


def same_set(model_pairs, expected_pairs):
    return sorted(model_pairs) == sorted(expected_pairs)


# ---------------------------------------------------------------------------
# Import the produced package (contract: slotfinder.public).
# ---------------------------------------------------------------------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("slotfinder.public")
    if not hasattr(pub, "find_slots"):
        raise AttributeError("slotfinder.public has no find_slots")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def call(request):
    """Call the model's find_slots; returns the raw list (may raise → caught by check)."""
    return pub.find_slots(request)


# ---------------------------------------------------------------------------
# Shared request fixtures (all instants UTC).
# America/New_York is UTC-5 in January (no DST) and observes spring-forward DST on
# 2026-03-08 02:00 local (-> 03:00), so 09:00 local = 14:00 UTC before and 13:00 UTC
# after. 2026-01-05 is a Monday (ISO weekday 1).
# ---------------------------------------------------------------------------
NY = "America/New_York"
LA = "America/Los_Angeles"


def req_basic(busy=None, granularity=None, duration=30):
    r = {
        "timezone": NY,
        "duration_minutes": duration,
        "search_start": "2026-01-05T00:00:00Z",
        "search_end": "2026-01-06T00:00:00Z",
        "participants": [
            {
                "id": "u1",
                "working_hours": [{"days": [1, 2, 3, 4, 5], "start": "09:00", "end": "11:00"}],
                "busy": busy or [],
            }
        ],
    }
    if granularity is not None:
        r["granularity_minutes"] = granularity
    return r


# ===========================================================================
# Behavior checks (only run when import_ok; otherwise all are force-failed below).
# ===========================================================================
def run_checks():
    # 1. basic availability: derive expected, compare instants exactly.
    def c_basic():
        r = req_basic()
        got = to_pairs(call(r))
        exp = expected_slots(r)
        return same_set(got, exp), f"got={len(got)} exp={len(exp)}"

    # 2. a busy interval removes exactly the overlapping windows.
    def c_busy_excludes():
        busy = [{"start": "2026-01-05T14:30:00Z", "end": "2026-01-05T15:00:00Z"}]
        r = req_basic(busy=busy)
        got = set(to_pairs(call(r)))
        exp = set(expected_slots(r))
        # The busy window (09:30-10:00 local = 14:30-15:00 UTC) must be gone, others kept.
        blocked = (parse_instant("2026-01-05T14:30:00Z"), parse_instant("2026-01-05T15:00:00Z"))
        return (got == exp and blocked not in got), f"got={len(got)} exp={len(exp)} blocked_present={blocked in got}"

    # 3. endpoint touch is NOT a conflict: a busy interval ending exactly at a window
    #    start must leave that window available.
    def c_busy_endpoint():
        # 30-min slots on the 09:00 grid: 09:00, 09:30, 10:00, 10:30 local.
        # Busy 08:30-09:00 local (13:30-14:00 UTC) touches the 09:00 window start only.
        busy = [{"start": "2026-01-05T13:30:00Z", "end": "2026-01-05T14:00:00Z"}]
        r = req_basic(busy=busy)
        got = set(to_pairs(call(r)))
        first = (parse_instant("2026-01-05T14:00:00Z"), parse_instant("2026-01-05T14:30:00Z"))
        exp = set(expected_slots(r))
        return (got == exp and first in got), f"first_present={first in got} got={len(got)}"

    # 4. working-hour boundary: with 09:00-11:00 local hours and 30-min slots, every
    #    returned slot must lie within [14:00Z, 16:00Z] and none start before/after.
    def c_boundary():
        r = req_basic()
        got = to_pairs(call(r))
        open_utc = parse_instant("2026-01-05T14:00:00Z")   # 09:00 EST
        close_utc = parse_instant("2026-01-05T16:00:00Z")  # 11:00 EST
        ok = all(s >= open_utc and e <= close_utc for s, e in got)
        # and the engine actually produced something (guard against trivial []).
        return (ok and len(got) >= 1), f"got={len(got)} all_in_bounds={ok}"

    # 5. multiple participants: intersection only. u1 in NY 09:00-17:00, u2 in LA
    #    09:00-17:00. Overlap of working hours (NY 12:00-17:00 == LA 09:00-14:00) is
    #    the only region with slots. Derive and compare.
    def c_multi():
        r = {
            "timezone": NY,
            "duration_minutes": 60,
            "search_start": "2026-01-05T00:00:00Z",
            "search_end": "2026-01-06T00:00:00Z",
            "participants": [
                {"id": "u1", "working_hours": [{"days": [1], "start": "09:00", "end": "17:00"}], "busy": []},
                {"id": "u2", "timezone": LA, "working_hours": [{"days": [1], "start": "09:00", "end": "17:00"}], "busy": []},
            ],
        }
        got = to_pairs(call(r))
        exp = expected_slots(r)
        # Sanity: the intersection is non-empty and bounded by NY-noon (17:00Z) start.
        ny_noon = parse_instant("2026-01-05T17:00:00Z")  # 12:00 EST = 09:00 PST
        in_overlap = all(s >= ny_noon for s, _ in got)
        return (same_set(got, exp) and len(got) >= 1 and in_overlap), f"got={len(got)} exp={len(exp)}"

    # 6. multiple working-hour blocks union (morning + afternoon, gap excluded).
    def c_multi_block():
        r = {
            "timezone": NY,
            "duration_minutes": 60,
            "search_start": "2026-01-05T00:00:00Z",
            "search_end": "2026-01-06T00:00:00Z",
            "participants": [
                {
                    "id": "u1",
                    "working_hours": [
                        {"days": [1], "start": "09:00", "end": "10:00"},
                        {"days": [1], "start": "14:00", "end": "15:00"},
                    ],
                    "busy": [],
                }
            ],
        }
        got = to_pairs(call(r))
        exp = expected_slots(r)
        # Expect exactly the 09:00 and 14:00 local windows (14:00Z and 19:00Z).
        return (same_set(got, exp) and len(exp) == 2), f"got={len(got)} exp={len(exp)}"

    # 7. DST: spring-forward weekend 2026-03-08. 09:00 local Friday 03-06 = 14:00 UTC;
    #    09:00 local Monday 03-09 (after DST) = 13:00 UTC. The engine must track local
    #    wall-clock, so the Monday slot's UTC start shifts by an hour vs the naive -5.
    def c_dst():
        r = {
            "timezone": NY,
            "duration_minutes": 60,
            "search_start": "2026-03-06T00:00:00Z",
            "search_end": "2026-03-10T00:00:00Z",
            "participants": [
                {"id": "u1", "working_hours": [{"days": [1, 5], "start": "09:00", "end": "10:00"}], "busy": []}
            ],
        }
        got = set(to_pairs(call(r)))
        exp = set(expected_slots(r))
        fri = (parse_instant("2026-03-06T14:00:00Z"), parse_instant("2026-03-06T15:00:00Z"))  # EST
        mon = (parse_instant("2026-03-09T13:00:00Z"), parse_instant("2026-03-09T14:00:00Z"))  # EDT
        return (got == exp and fri in got and mon in got), f"fri={fri in got} mon={mon in got} got={len(got)}"

    # 8. no availability: working hours present but the whole window is busy → [].
    def c_none():
        busy = [{"start": "2026-01-05T00:00:00Z", "end": "2026-01-06T00:00:00Z"}]
        r = req_basic(busy=busy)
        got = call(r)
        return (isinstance(got, list) and len(got) == 0), f"got={got!r}"

    # 9. default granularity == 15 min: omitting granularity must align starts to a
    #    15-min grid. With 15-min slots over 09:00-11:00 local there are 8 windows.
    def c_gran_default():
        r = req_basic(duration=15)  # no granularity_minutes -> default 15
        got = to_pairs(call(r))
        exp = expected_slots(r)
        return (same_set(got, exp) and len(exp) == 8), f"got={len(got)} exp={len(exp)}"

    # 10. custom granularity changes the grid: granularity 60 with 30-min slots over
    #    09:00-11:00 local yields starts only at 09:00 and 10:00 local (2 windows).
    def c_gran_custom():
        r = req_basic(duration=30, granularity=60)
        got = to_pairs(call(r))
        exp = expected_slots(r)
        return (same_set(got, exp) and len(exp) == 2), f"got={len(got)} exp={len(exp)}"

    # 11. non-overlapping: no two returned slots overlap.
    def c_non_overlap():
        r = req_basic(duration=30)
        got = sorted(to_pairs(call(r)))
        overlap = any(got[i][1] > got[i + 1][0] for i in range(len(got) - 1))
        return (len(got) >= 2 and not overlap), f"slots={len(got)} overlap={overlap}"

    # 12. deterministic order: ascending by start AND identical across two runs.
    def c_determinism():
        r = req_basic(duration=30)
        a = call(r)
        b = call(r)
        pa = to_pairs(a)
        ascending = all(pa[i][0] <= pa[i + 1][0] for i in range(len(pa) - 1))
        stable = json.dumps(a, sort_keys=True, default=str) == json.dumps(b, sort_keys=True, default=str)
        return (ascending and stable and len(pa) >= 2), f"ascending={ascending} stable={stable}"

    # 13. slot shape: exactly {start, end}, ISO-UTC parseable, length == duration.
    def c_shape():
        r = req_basic(duration=30)
        got = call(r)
        if not (isinstance(got, list) and got):
            return False, f"got={got!r}"
        for s in got:
            if not isinstance(s, dict) or set(s.keys()) != {"start", "end"}:
                return False, f"bad keys: {s!r}"
            st, en = parse_instant(s["start"]), parse_instant(s["end"])
            if (en - st) != timedelta(minutes=30):
                return False, f"bad length: {s!r}"
        return True, f"checked {len(got)} slots"

    run = [
        ("basic_availability", c_basic),
        ("busy_excludes", c_busy_excludes),
        ("busy_endpoint_touch", c_busy_endpoint),
        ("working_hour_boundary", c_boundary),
        ("multi_participant", c_multi),
        ("multi_block", c_multi_block),
        ("dst_transition", c_dst),
        ("no_availability", c_none),
        ("granularity_default", c_gran_default),
        ("granularity_custom", c_gran_custom),
        ("non_overlapping", c_non_overlap),
        ("deterministic_order", c_determinism),
        ("slot_shape", c_shape),
    ]
    desc = dict(CHECK_IDS)
    for cid, fn in run:
        try:
            ok, detail = fn()
        except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
            ok, detail = False, f"{type(e).__name__}: {e}"
        record(cid, desc[cid], ok, detail)


# --- CLI check: `python -m slotfinder find <file>` must emit a JSON list ------
def c_cli():
    desc = dict(CHECK_IDS)["cli_json"]
    fd, path = tempfile.mkstemp(suffix=".json", dir=ROOT)
    try:
        with os.fdopen(fd, "w") as f:
            json.dump(req_basic(), f)
        proc = subprocess.run(
            [sys.executable, "-m", "slotfinder", "find", path],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        parsed = json.loads(proc.stdout)  # raises if not JSON
        ok = isinstance(parsed, list)
        record("cli_json", desc, ok, f"rc={proc.returncode} type={type(parsed).__name__}")
    except Exception as e:  # noqa: BLE001
        record("cli_json", desc, False, f"{type(e).__name__}: {e}")
    finally:
        try:
            os.remove(path)
        except OSError:
            pass


# ===========================================================================
# Drive grading. FIXED DENOMINATOR: if import failed, every check id is recorded
# as failed with the import error; score is forced to 0.0.
# ===========================================================================
if import_ok:
    run_checks()
    c_cli()
else:
    for cid, desc in CHECK_IDS:
        record(cid, desc, False, f"import failed: {import_detail}")

passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "slotfinder",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)

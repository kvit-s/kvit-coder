#!/usr/bin/env python3
"""Held-out behavior-level oracle for feature-add task `eventbus`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it,
and it never imports the agent's own tests. It grades the produced `eventbus`
package against the BRIEF'S CONTRACT (wildcard pub/sub matching, plus the
unchanged exact-topic core), NOT against any particular internal file layout.

The defining behaviors a SUPERFICIAL implementation gets wrong:

  * `*` is SINGLE-segment: `"a.*"` must match `"a.b"` but reject `"a.b.c"` and
    `"a"`. A regex built from `*` -> `.*` (or a prefix-match shortcut) over-
    matches here.
  * `#` requires ONE OR MORE trailing segments: `"a.#"` matches `"a.b"` and
    `"a.b.c"` but NOT `"a"`. A `#` -> "zero-or-more" reading wrongly fires on
    the bare prefix.
  * DELIVERY ORDER is the GLOBAL subscription order across exact AND wildcard
    matches — not "exact first, wildcards after", and not per-bucket order.
  * FIRE ONCE per subscription per publish, even when a pattern could be seen to
    match in more than one way (e.g. bare `"#"` over a multi-segment topic).
  * `*`/`#` are wildcards only in SUBSCRIPTIONS; in a PUBLISHED topic they are
    literal text.
  * `publish` returns the count of callbacks invoked (an int).

The shipped first attempt stores wildcard subscriptions but matches by string
equality, so it passes the exact-topic and return-count regression checks while
failing every wildcard check — that's what makes the task discriminate (naive
lands well under 1.0, a careful matcher lands at 1.0).

Output: a single JSON scorecard on stdout. Each check runs in isolation, so the
score is continuous (passed / total), never all-or-nothing. FIXED DENOMINATOR:
the full check roster is declared up front, so an import failure records every
check as failed and forces score 0.0. Exit code is 0 whenever grading ran to
completion (even score 0.0); the process never raises out.
"""
import importlib
import json
import os
import sys

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# --- fixed denominator: the full roster of checks, declared before any run ----
#
# Several checks ("scenario_*") publish DIFFERENT concrete topics against ONE
# shared mixed roster of subscriptions and assert the FULL ordered list of who
# fires. That makes each such check exercise global ordering AND '*'-arity AND
# '#'-arity at once, so a single naive mistake (exact-first bucketing, a greedy
# '*', a zero-or-more '#') fails SEVERAL checks — which is what pushes a naive
# implementation well under 1.0 while a careful one stays at 1.0.
CHECK_SPECS = [
    ("star_matches_one_segment", "'a.*' matches the one-segment-deeper topic 'a.b'"),
    ("star_rejects_too_deep", "'a.*' does NOT match the two-deeper topic 'a.b.c'"),
    ("star_rejects_too_shallow", "'a.*' does NOT match the bare prefix 'a'"),
    ("star_middle_position", "'*.created' matches 'order.created' but not 'order.created.late'"),
    ("star_bare_one_segment", "bare '*' matches a one-segment topic but not a two-segment one"),
    ("hash_matches_one_trailing", "'a.#' matches 'a.b' (one trailing segment)"),
    ("hash_matches_many_trailing", "'a.#' matches 'a.b.c.d' (several trailing segments)"),
    ("hash_requires_at_least_one", "'a.#' does NOT match the bare prefix 'a' (needs >= 1 trailing)"),
    ("hash_bare_matches_everything", "bare '#' matches any topic of one-or-more segments"),
    ("fire_once_per_subscription", "a subscription fires at most once per publish (bare '#' over a deep topic)"),
    ("same_fn_twice_fires_twice", "the same callable subscribed twice is two subscriptions -> two calls"),
    ("wildcard_chars_literal_in_publish", "'*'/'#' in a PUBLISHED topic are literal, not wildcards"),
    ("scenario_exact_depth_ordered", "mixed roster, publish 'order.created': exact+'*'+'#' fire in global order"),
    ("scenario_deep_only_hash_ordered", "mixed roster, publish 'order.created.late': only the '#' patterns fire, in order"),
    ("scenario_prefix_nothing", "mixed roster, publish bare 'order': '*'/'#' need >=1 deeper segment, none fire"),
    ("scenario_sibling_isolation", "mixed roster, publish 'user.created': only the cross-cutting wildcards fire, in order"),
    ("publish_returns_match_count", "publish returns the number of callbacks it invoked"),
    ("regression_exact_delivery", "exact-topic subscribe/publish still delivers (and only to exact matches)"),
    ("regression_exact_order_and_count", "exact-topic: registration-order delivery and correct return count"),
]
CHECK_IDS = [cid for cid, _ in CHECK_SPECS]
DESC = dict(CHECK_SPECS)

results = {}  # cid -> {"passed": bool, "detail": str}


def record(cid, passed, detail=""):
    results[cid] = {"passed": bool(passed), "detail": str(detail or "")}


def check(cid, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


# --- import the produced package (contract: eventbus.public, fallback eventbus)
import_ok = True
import_detail = ""
EventBus = None
try:
    try:
        mod = importlib.import_module("eventbus.public")
    except Exception:
        mod = importlib.import_module("eventbus")
    EventBus = getattr(mod, "EventBus")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    # Helper: build a bus, subscribe (label, topic) pairs in order appending to a
    # shared log, publish once, and return the ordered list of labels that fired
    # plus publish()'s return value.
    def run(subs, pub_topic, data=None):
        bus = EventBus()
        log = []

        def make(label):
            return lambda t, d, _l=label: log.append((_l, t, d))

        for label, topic in subs:
            bus.subscribe(topic, make(label))
        n = bus.publish(pub_topic, data)
        return log, n

    # 1. '*' matches exactly one segment deeper.
    def c_star_matches_one_segment():
        log, _ = run([("s", "a.*")], "a.b", 1)
        labels = [e[0] for e in log]
        return labels == ["s"], f"fired={labels} (expected ['s'] for 'a.*' vs 'a.b')"

    check("star_matches_one_segment", c_star_matches_one_segment)

    # 2. '*' rejects a topic that is too deep.
    def c_star_rejects_too_deep():
        log, n = run([("s", "a.*")], "a.b.c", 1)
        labels = [e[0] for e in log]
        return labels == [] and n == 0, f"fired={labels} n={n} (expected none for 'a.*' vs 'a.b.c')"

    check("star_rejects_too_deep", c_star_rejects_too_deep)

    # 3. '*' rejects a topic that is too shallow (the bare prefix).
    def c_star_rejects_too_shallow():
        log, n = run([("s", "a.*")], "a", 1)
        labels = [e[0] for e in log]
        return labels == [] and n == 0, f"fired={labels} n={n} (expected none for 'a.*' vs 'a')"

    check("star_rejects_too_shallow", c_star_rejects_too_shallow)

    # 4. '*' works in a non-final position, and still enforces exact arity.
    def c_star_middle_position():
        log_hit, _ = run([("s", "*.created")], "order.created", 1)
        log_miss, _ = run([("s", "*.created")], "order.created.late", 1)
        hit = [e[0] for e in log_hit]
        miss = [e[0] for e in log_miss]
        ok = hit == ["s"] and miss == []
        return ok, f"order.created->{hit} order.created.late->{miss} (expected ['s']/[])"

    check("star_middle_position", c_star_middle_position)

    # 5. bare '*' matches a single-segment topic but not a deeper one.
    def c_star_bare_one_segment():
        log_hit, _ = run([("s", "*")], "alpha", 1)
        log_miss, _ = run([("s", "*")], "alpha.beta", 1)
        hit = [e[0] for e in log_hit]
        miss = [e[0] for e in log_miss]
        ok = hit == ["s"] and miss == []
        return ok, f"'alpha'->{hit} 'alpha.beta'->{miss} (expected ['s']/[])"

    check("star_bare_one_segment", c_star_bare_one_segment)

    # 6. '#' matches exactly one trailing segment.
    def c_hash_matches_one_trailing():
        log, _ = run([("s", "a.#")], "a.b", 1)
        labels = [e[0] for e in log]
        return labels == ["s"], f"fired={labels} (expected ['s'] for 'a.#' vs 'a.b')"

    check("hash_matches_one_trailing", c_hash_matches_one_trailing)

    # 7. '#' matches many trailing segments.
    def c_hash_matches_many_trailing():
        log, _ = run([("s", "a.#")], "a.b.c.d", 1)
        labels = [e[0] for e in log]
        return labels == ["s"], f"fired={labels} (expected ['s'] for 'a.#' vs 'a.b.c.d')"

    check("hash_matches_many_trailing", c_hash_matches_many_trailing)

    # 8. '#' requires AT LEAST ONE trailing segment: it must NOT match the prefix.
    def c_hash_requires_at_least_one():
        log, n = run([("s", "a.#")], "a", 1)
        labels = [e[0] for e in log]
        return labels == [] and n == 0, f"fired={labels} n={n} (expected none for 'a.#' vs 'a')"

    check("hash_requires_at_least_one", c_hash_requires_at_least_one)

    # 9. bare '#' matches everything (one or more segments).
    def c_hash_bare_matches_everything():
        a, _ = run([("s", "#")], "x", 1)
        b, _ = run([("s", "#")], "x.y", 1)
        c, _ = run([("s", "#")], "x.y.z.w", 1)
        ok = ([e[0] for e in a] == ["s"]
              and [e[0] for e in b] == ["s"]
              and [e[0] for e in c] == ["s"])
        return ok, f"'x'->{[e[0] for e in a]} 'x.y'->{[e[0] for e in b]} 'x.y.z.w'->{[e[0] for e in c]}"

    check("hash_bare_matches_everything", c_hash_bare_matches_everything)

    # 10-13. SCENARIO checks: one shared mixed roster, several concrete publishes.
    #
    # The roster deliberately interleaves EXACT and WILDCARD subscriptions, and
    # is ordered so that in every scenario an exact match falls BETWEEN two
    # wildcard matches. The expected fire-order is the registration order across
    # both kinds, so an exact-first / wildcard-last bucketing scheme gets every
    # scenario's order wrong even when it picks the right SET. The same roster,
    # hit with different concrete topics, also pins '*' arity and the '#'-needs-
    # one rule — so each scenario independently catches a greedy '*' or a zero-or-
    # more '#'. The grader computes each expected list itself, by registration
    # order, from the matcher rules in the brief.
    SCENARIO_ROSTER = [
        ("hashOrder", "order.#"),            # 0: trailing '#' under 'order'
        ("exactCreated", "order.created"),   # 1: exact, 2 segs
        ("starOrder", "order.*"),            # 2: single '*' under 'order'
        ("hashAll", "#"),                    # 3: bare '#': every >=1-seg topic
        ("starCreated", "*.created"),        # 4: single '*' in the head position
        ("exactLate", "order.created.late"), # 5: exact, 3 segs
        ("exactBareOrder", "order"),         # 6: exact, 1 seg
        ("starStar", "*.*"),                 # 7: two single '*' segments
    ]

    def scenario(pub_topic):
        return run(SCENARIO_ROSTER, pub_topic, "D")

    # 10. publish 'order.created' (2 segs): the exact 'order.created' sits between
    #     wildcard matches, so order distinguishes bucketing from global order.
    def c_scenario_exact_depth_ordered():
        log, n = scenario("order.created")
        labels = [e[0] for e in log]
        # hashOrder('order.#'), exactCreated, starOrder('order.*'), hashAll('#'),
        # starCreated('*.created'), starStar('*.*'); the 3-seg exact + bare-order do not.
        expected = ["hashOrder", "exactCreated", "starOrder", "hashAll", "starCreated", "starStar"]
        return labels == expected and n == len(expected), \
            f"order={labels} n={n} (expected {expected}, {len(expected)})"

    check("scenario_exact_depth_ordered", c_scenario_exact_depth_ordered)

    # 11. publish 'order.created.late' (3 segs): only the two '#' patterns and the
    #     3-seg exact match; EVERY single-'*' pattern is the wrong arity. The
    #     exact 'order.created.late' again sits between the two '#' matches.
    def c_scenario_deep_only_hash_ordered():
        log, n = scenario("order.created.late")
        labels = [e[0] for e in log]
        expected = ["hashOrder", "hashAll", "exactLate"]
        return labels == expected and n == len(expected), \
            f"order={labels} n={n} (expected {expected}, {len(expected)})"

    check("scenario_deep_only_hash_ordered", c_scenario_deep_only_hash_ordered)

    # 12. publish bare 'order' (1 seg): 'order.#'/'order.*'/'*.created'/'*.*' all
    #     need a deeper segment; only bare '#' and the exact 'order' match. Order
    #     puts the wildcard '#' (registered first) BEFORE the exact.
    def c_scenario_prefix_nothing():
        log, n = scenario("order")
        labels = [e[0] for e in log]
        expected = ["hashAll", "exactBareOrder"]
        return labels == expected and n == len(expected), \
            f"order={labels} n={n} (expected {expected}, {len(expected)})"

    check("scenario_prefix_nothing", c_scenario_prefix_nothing)

    # 13. publish 'user.created' (2 segs, different head): nothing rooted at
    #     'order' matches; only the cross-cutting wildcards do, in order.
    def c_scenario_sibling_isolation():
        log, n = scenario("user.created")
        labels = [e[0] for e in log]
        expected = ["hashAll", "starCreated", "starStar"]
        return labels == expected and n == len(expected), \
            f"order={labels} n={n} (expected {expected}, {len(expected)})"

    check("scenario_sibling_isolation", c_scenario_sibling_isolation)

    # 11. FIRE ONCE: bare '#' over a deep topic fires the one subscription once.
    def c_fire_once_per_subscription():
        log, n = run([("s", "#")], "a.b.c", 1)
        labels = [e[0] for e in log]
        return labels == ["s"] and n == 1, f"fired={labels} n={n} (expected ['s'], 1 — not one-per-segment)"

    check("fire_once_per_subscription", c_fire_once_per_subscription)

    # 12. the SAME callable subscribed twice is two subscriptions -> two calls.
    def c_same_fn_twice_fires_twice():
        bus = EventBus()
        calls = []
        fn = lambda t, d: calls.append(t)
        bus.subscribe("a.b", fn)
        bus.subscribe("a.*", fn)
        n = bus.publish("a.b", 1)
        return len(calls) == 2 and n == 2, f"calls={len(calls)} n={n} (expected 2/2)"

    check("same_fn_twice_fires_twice", c_same_fn_twice_fires_twice)

    # 13. '*'/'#' in a PUBLISHED topic are literal text, not wildcards.
    def c_wildcard_chars_literal_in_publish():
        # Subscriber 'lit' is the literal topic 'a.*'; subscriber 'real' is the
        # literal topic 'a.b'. Publishing the concrete topic 'a.*' must hit the
        # literal subscription (and the single-'*' wildcard, which matches the
        # literal segment '*'), but must NOT be expanded to also hit 'a.b'.
        bus = EventBus()
        log = []
        bus.subscribe("a.*", lambda t, d: log.append("wild"))   # '*' wildcard seg
        bus.subscribe("a.b", lambda t, d: log.append("ab"))     # literal a.b
        n = bus.publish("a.*", 1)   # concrete topic whose 2nd segment is "*"
        # 'a.*' wildcard matches (its '*' matches literal segment '*'); 'a.b' must not.
        ok = log == ["wild"] and n == 1
        return ok, f"fired={log} n={n} (expected ['wild'], 1 — publish '*' is literal)"

    check("wildcard_chars_literal_in_publish", c_wildcard_chars_literal_in_publish)

    # 14. publish returns the number of callbacks invoked.
    def c_publish_returns_match_count():
        subs = [("a", "x.y"), ("b", "x.*"), ("c", "x.#"), ("d", "x.z")]
        _, n = run(subs, "x.y", 1)
        # 'x.y' exact, 'x.*' single, 'x.#' trailing all match; 'x.z' does not.
        return n == 3, f"publish returned {n} (expected 3)"

    check("publish_returns_match_count", c_publish_returns_match_count)

    # 15. REGRESSION: exact-topic delivery still works and stays exact.
    def c_regression_exact_delivery():
        bus = EventBus()
        got = []
        bus.subscribe("user.login", lambda t, d: got.append((t, d)))
        bus.subscribe("user.logout", lambda t, d: got.append(("other", d)))
        bus.publish("user.login", 42)
        ok = got == [("user.login", 42)]  # only the exact match, with (topic, data)
        return ok, f"got={got} (expected [('user.login', 42)])"

    check("regression_exact_delivery", c_regression_exact_delivery)

    # 16. REGRESSION: exact-topic registration-order delivery and return count.
    def c_regression_exact_order_and_count():
        subs = [("first", "t"), ("second", "t"), ("third", "t")]
        log, n = run(subs, "t", "d")
        labels = [e[0] for e in log]
        ok = labels == ["first", "second", "third"] and n == 3
        return ok, f"order={labels} n={n} (expected ['first','second','third'], 3)"

    check("regression_exact_order_and_count", c_regression_exact_order_and_count)


# --- assemble the scorecard with a FIXED denominator -------------------------
checks_out = []
for cid in CHECK_IDS:
    r = results.get(cid)
    if r is None:
        r = {"passed": False, "detail": "not run (import failed)" if not import_ok else "not run"}
    checks_out.append({"id": cid, "desc": DESC[cid], "passed": r["passed"], "detail": r["detail"]})

passed = sum(1 for c in checks_out if c["passed"])
total = len(checks_out)  # always len(CHECK_SPECS): fixed denominator
card = {
    "task": "eventbus",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks_out,
}
print(json.dumps(card))
sys.exit(0)

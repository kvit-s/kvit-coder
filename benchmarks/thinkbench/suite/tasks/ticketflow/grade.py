#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield task `ticketflow`.

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `ticketflow.public`
API and the `python -m ticketflow assign` CLI), NOT against the model's own tests
and NOT against any particular internal file layout.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

Fairness hardening:
  * FIXED DENOMINATOR. The set of checks is fixed up front. If the package fails to
    import, every check is recorded as FAILED (not skipped) so the denominator is
    identical to a passing run, and the score is FORCED to 0.0.
  * DERIVE, don't REQUIRE. Behavior is read from the contract-pinned return shape
    (assigned/unassigned mappings) and the CLI's JSON; we never require the model's
    private helpers, file layout, or its own tests.
  * NEVER STRICTER THAN brief+Contract. Every check traces to a pinned rule. Spots
    where the brief under-pins a convention are marked `# ASSUMES`; those are pinned
    in brief.txt's Contract, so we never grade a guess.
"""
import importlib
import json
import os
import subprocess
import sys
import tempfile

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# ---- the FIXED check registry (id, description). Denominator is len(CHECK_SPECS),
#      independent of whether the package imports. -----------------------------
CHECK_SPECS = [
    ("assign_basic", "a qualified agent gets a matching ticket; return has assigned/unassigned maps"),
    ("priority_order", "higher-priority ticket wins the only seat over a lower-priority one"),
    ("created_at_tiebreak", "same priority: older (earlier created_at) ticket wins the only seat"),
    ("ticket_id_tiebreak", "same priority and created_at: lower ticket_id wins the only seat"),
    ("language_match", "ticket goes only to an agent that speaks its language"),
    ("skill_match", "ticket goes only to an agent whose skills include its product"),
    ("capacity_respected", "an agent at current_load==capacity is never assigned a ticket"),
    ("least_loaded_wins", "among qualified agents the least-loaded one is chosen"),
    ("agent_id_tiebreak", "equal load qualified agents: lowest agent_id wins"),
    ("load_accumulates", "assignments within one call consume capacity (no overfill)"),
    ("reason_no_language", "unassigned reason 'no_language_match' when no agent speaks the language"),
    ("reason_no_skill", "unassigned reason 'no_skill_match' when language matches but skill does not"),
    ("reason_no_capacity", "unassigned reason 'no_capacity' when qualified agents are all full"),
    ("partition", "every ticket id appears in exactly one of assigned/unassigned"),
    ("explain_eligible", "explain_assignment reports eligible=True with all three factors true"),
    ("explain_factors", "explain_assignment isolates language/skill/capacity failures"),
    ("determinism", "assign_tickets is order-independent and stable across runs"),
    ("cli_assign_json", "`python -m ticketflow assign` emits the contract JSON object"),
]
CHECK_IDS = [cid for cid, _ in CHECK_SPECS]
DESCS = dict(CHECK_SPECS)

results = {}  # cid -> (passed: bool, detail: str)


def record(cid, passed, detail=""):
    results[cid] = (bool(passed), str(detail or ""))


def run_check(cid, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, ok, detail)


# ---- tolerant accessors over the pinned return shape ------------------------
def get_map(result, key):
    """Pull the assigned/unassigned mapping; tolerate it being absent -> {}."""
    if isinstance(result, dict) and isinstance(result.get(key), dict):
        return result[key]
    return {}


def ticket(tid, priority="medium", language="en", product="billing", created_at="2026-01-01T00:00:00Z"):
    return {
        "ticket_id": tid,
        "priority": priority,
        "language": language,
        "product": product,
        "created_at": created_at,
    }


def agent(aid, languages=("en",), skills=("billing",), capacity=1, current_load=0):
    return {
        "agent_id": aid,
        "languages": list(languages),
        "skills": list(skills),
        "capacity": capacity,
        "current_load": current_load,
    }


# ---- import the produced package (contract: ticketflow.public) ---------------
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("ticketflow.public")
    # both pinned functions must be present and callable to grade behavior
    if not callable(getattr(pub, "assign_tickets", None)):
        raise ImportError("ticketflow.public.assign_tickets missing or not callable")
    if not callable(getattr(pub, "explain_assignment", None)):
        raise ImportError("ticketflow.public.explain_assignment missing or not callable")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


if import_ok:
    A = pub.assign_tickets
    E = pub.explain_assignment

    # 1. basic assignment + return shape
    def c_assign_basic():
        r = A({}, [ticket("t1")], [agent("a1")])
        asg = get_map(r, "assigned")
        return (asg.get("t1") == "a1"), f"result={r!r}"

    run_check("assign_basic", c_assign_basic)

    # 2. priority: only one seat. Disambiguated so ONLY a priority comparison can pick
    #    the winner — the low ticket has the smallest id AND earliest created_at AND comes
    #    first, so input-order / created_at-asc / id-asc all pick it. Only honoring
    #    priority picks "zhi"; a priority-ignoring engine picks "aaa" and fails.
    def c_priority_order():
        tickets = [
            ticket("aaa", priority="low", created_at="2026-01-01T00:00:00Z"),
            ticket("zhi", priority="high", created_at="2026-01-02T00:00:00Z"),
        ]
        r = A({}, tickets, [agent("a1", capacity=1)])
        asg = get_map(r, "assigned")
        return (asg.get("zhi") == "a1" and "aaa" not in asg), f"assigned={asg!r}"

    run_check("priority_order", c_priority_order)

    # 3. created_at tiebreak: same priority, older wins the one seat
    def c_created_at_tiebreak():
        tickets = [
            ticket("new", priority="high", created_at="2026-01-02T00:00:00Z"),
            ticket("old", priority="high", created_at="2026-01-01T00:00:00Z"),
        ]
        r = A({}, tickets, [agent("a1", capacity=1)])
        asg = get_map(r, "assigned")
        return (asg.get("old") == "a1" and "new" not in asg), f"assigned={asg!r}"

    run_check("created_at_tiebreak", c_created_at_tiebreak)

    # 4. ticket_id tiebreak: identical priority + created_at, lower id wins the seat
    def c_ticket_id_tiebreak():
        tickets = [
            ticket("zzz", priority="high", created_at="2026-01-01T00:00:00Z"),
            ticket("aaa", priority="high", created_at="2026-01-01T00:00:00Z"),
        ]
        r = A({}, tickets, [agent("a1", capacity=1)])
        asg = get_map(r, "assigned")
        return (asg.get("aaa") == "a1" and "zzz" not in asg), f"assigned={asg!r}"

    run_check("ticket_id_tiebreak", c_ticket_id_tiebreak)

    # 5. language: ticket goes only to the language-matching agent
    def c_language_match():
        agents = [agent("fr", languages=["fr"]), agent("en", languages=["en"])]
        r = A({}, [ticket("t1", language="en")], agents)
        asg = get_map(r, "assigned")
        return (asg.get("t1") == "en"), f"assigned={asg!r}"

    run_check("language_match", c_language_match)

    # 6. skill: ticket goes only to the skill (product)-matching agent
    def c_skill_match():
        agents = [agent("net", skills=["network"]), agent("bill", skills=["billing"])]
        r = A({}, [ticket("t1", product="billing")], agents)
        asg = get_map(r, "assigned")
        return (asg.get("t1") == "bill"), f"assigned={asg!r}"

    run_check("skill_match", c_skill_match)

    # 7. capacity: a full agent (load==capacity) never gets the ticket
    def c_capacity_respected():
        r = A({}, [ticket("t1")], [agent("full", capacity=1, current_load=1)])
        asg = get_map(r, "assigned")
        return ("t1" not in asg), f"result={r!r}"

    run_check("capacity_respected", c_capacity_respected)

    # 8. least-loaded qualified agent wins (both qualified, pick lower load)
    def c_least_loaded_wins():
        agents = [
            agent("busy", capacity=5, current_load=3),
            agent("idle", capacity=5, current_load=0),
        ]
        r = A({}, [ticket("t1")], agents)
        asg = get_map(r, "assigned")
        return (asg.get("t1") == "idle"), f"assigned={asg!r}"

    run_check("least_loaded_wins", c_least_loaded_wins)

    # 9. equal load -> lowest agent_id wins (deterministic), input order shuffled
    def c_agent_id_tiebreak():
        agents = [agent("b", capacity=2, current_load=0), agent("a", capacity=2, current_load=0)]
        r = A({}, [ticket("t1")], agents)
        asg = get_map(r, "assigned")
        return (asg.get("t1") == "a"), f"assigned={asg!r}"

    run_check("agent_id_tiebreak", c_agent_id_tiebreak)

    # 10. load accumulates within a call: 3 tickets, one agent capacity 2 -> 2 assigned, 1 over
    def c_load_accumulates():
        tickets = [ticket("t1"), ticket("t2"), ticket("t3")]
        r = A({}, tickets, [agent("a1", capacity=2, current_load=0)])
        asg = get_map(r, "assigned")
        un = get_map(r, "unassigned")
        # exactly two assigned to a1, one unassigned (no double-booking beyond capacity)
        n_assigned = sum(1 for v in asg.values() if v == "a1")
        return (n_assigned == 2 and len(un) == 1), f"assigned={asg!r} unassigned={un!r}"

    run_check("load_accumulates", c_load_accumulates)

    # 11. unassigned reason: no agent speaks the language -> 'no_language_match'
    def c_reason_no_language():
        r = A({}, [ticket("t1", language="de")], [agent("a1", languages=["en"])])
        un = get_map(r, "unassigned")
        return (un.get("t1") == "no_language_match"), f"unassigned={un!r}"

    run_check("reason_no_language", c_reason_no_language)

    # 12. reason: language matches but no skill -> 'no_skill_match'
    def c_reason_no_skill():
        r = A({}, [ticket("t1", language="en", product="billing")],
              [agent("a1", languages=["en"], skills=["network"])])
        un = get_map(r, "unassigned")
        return (un.get("t1") == "no_skill_match"), f"unassigned={un!r}"

    run_check("reason_no_skill", c_reason_no_skill)

    # 13. reason: qualified agent exists but is full -> 'no_capacity'
    def c_reason_no_capacity():
        r = A({}, [ticket("t1")], [agent("a1", capacity=1, current_load=1)])
        un = get_map(r, "unassigned")
        return (un.get("t1") == "no_capacity"), f"unassigned={un!r}"

    run_check("reason_no_capacity", c_reason_no_capacity)

    # 14. partition: every ticket id appears in exactly one of assigned/unassigned
    def c_partition():
        tickets = [
            ticket("t1"),  # assignable
            ticket("t2", language="de"),  # no language
            ticket("t3"),  # second to a1 -> over capacity
        ]
        r = A({}, tickets, [agent("a1", languages=["en"], capacity=1)])
        asg = get_map(r, "assigned")
        un = get_map(r, "unassigned")
        ids = {"t1", "t2", "t3"}
        akeys, ukeys = set(asg), set(un)
        disjoint = akeys.isdisjoint(ukeys)
        covers = (akeys | ukeys) == ids
        return (disjoint and covers), f"assigned={akeys!r} unassigned={ukeys!r}"

    run_check("partition", c_partition)

    # 15. explain: a fully eligible pair -> eligible True, all three factors True
    def c_explain_eligible():
        res = E(ticket("t1", language="en", product="billing"),
                agent("a1", languages=["en"], skills=["billing"], capacity=2, current_load=0),
                {})
        if not isinstance(res, dict):
            return False, f"type={type(res).__name__}"
        return (
            res.get("eligible") is True
            and res.get("language_match") is True
            and res.get("skill_match") is True
            and res.get("capacity_available") is True
        ), f"explain={res!r}"

    run_check("explain_eligible", c_explain_eligible)

    # 16. explain isolates each failure factor (and eligible follows)
    def c_explain_factors():
        t = ticket("t1", language="en", product="billing")
        no_lang = E(t, agent("a", languages=["fr"], skills=["billing"], capacity=2, current_load=0), {})
        no_skill = E(t, agent("a", languages=["en"], skills=["network"], capacity=2, current_load=0), {})
        no_cap = E(t, agent("a", languages=["en"], skills=["billing"], capacity=1, current_load=1), {})
        ok_lang = (no_lang.get("language_match") is False and no_lang.get("eligible") is False)
        ok_skill = (no_skill.get("skill_match") is False and no_skill.get("eligible") is False)
        ok_cap = (no_cap.get("capacity_available") is False and no_cap.get("eligible") is False)
        return (ok_lang and ok_skill and ok_cap), f"lang={no_lang!r} skill={no_skill!r} cap={no_cap!r}"

    run_check("explain_factors", c_explain_factors)

    # 17. determinism: shuffled inputs + repeated runs give the same assignment
    def c_determinism():
        tickets = [
            ticket("t3", priority="low", created_at="2026-01-03T00:00:00Z"),
            ticket("t1", priority="high", created_at="2026-01-01T00:00:00Z"),
            ticket("t2", priority="high", created_at="2026-01-02T00:00:00Z"),
        ]
        agents = [
            agent("b", capacity=2, current_load=1),
            agent("a", capacity=2, current_load=1),
        ]
        r1 = A({}, tickets, agents)
        r2 = A({}, list(reversed(tickets)), list(reversed(agents)))
        s1 = json.dumps(r1, sort_keys=True, default=str)
        s2 = json.dumps(r2, sort_keys=True, default=str)
        return (s1 == s2), ("stable" if s1 == s2 else f"{s1} != {s2}")

    run_check("determinism", c_determinism)


# ---- CLI: `python -m ticketflow assign` must emit the contract JSON object ---
def c_cli_assign_json():
    files = {}
    tmp = []
    try:
        payloads = {
            "config": {},
            "tickets": [ticket("t1"), ticket("t2", language="de")],
            "agents": [agent("a1", languages=["en"], capacity=2)],
        }
        for name, data in payloads.items():
            fd, path = tempfile.mkstemp(suffix=f"_{name}.json", dir=ROOT)
            tmp.append(path)
            with os.fdopen(fd, "w") as f:
                json.dump(data, f)
            files[name] = path
        proc = subprocess.run(
            [sys.executable, "-m", "ticketflow", "assign",
             "--config", files["config"], "--tickets", files["tickets"], "--agents", files["agents"]],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        out = json.loads(proc.stdout)  # raises if not JSON
        ok = (
            proc.returncode == 0
            and isinstance(out, dict)
            and isinstance(out.get("assigned"), dict)
            and isinstance(out.get("unassigned"), dict)
            and out["assigned"].get("t1") == "a1"
            and out["unassigned"].get("t2") == "no_language_match"
        )
        return ok, f"rc={proc.returncode} out={proc.stdout[:200]!r} err={proc.stderr[:200]!r}"
    finally:
        for path in tmp:
            try:
                os.remove(path)
            except OSError:
                pass


run_check("cli_assign_json", c_cli_assign_json)


# ---- assemble the scorecard over the FIXED registry -------------------------
# On import failure every id was never run -> record it FAILED here, preserving the
# denominator so a non-importing package scores 0/N, not 0/0.
checks = []
for cid in CHECK_IDS:
    passed, detail = results.get(cid, (False, "not run (import failed)" if not import_ok else "not run"))
    checks.append({"id": cid, "desc": DESCS[cid], "passed": bool(passed), "detail": detail})

passed = sum(1 for c in checks if c["passed"])
total = len(checks)  # fixed denominator == len(CHECK_SPECS)
card = {
    "task": "ticketflow",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    # forced 0.0 when import failed, regardless of any check that slipped through.
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)

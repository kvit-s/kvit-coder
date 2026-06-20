"""Reference ticketflow.public — a deterministic support-ticket assignment engine.

Greedy single-pass assignment over a total order on tickets (priority desc, then
older created_at, then ticket_id asc). For each ticket, the least-loaded qualified
agent wins, agent_id breaking ties. Pure stdlib.
"""

# Priority ordering, most-urgent -> least-urgent. Lower rank number == more urgent.
PRIORITY_RANK = {"urgent": 0, "high": 1, "medium": 2, "low": 3}
# Unknown priorities rank below "low" (least urgent).
_UNKNOWN_PRIORITY_RANK = len(PRIORITY_RANK)


def _priority_rank(priority):
    return PRIORITY_RANK.get(priority, _UNKNOWN_PRIORITY_RANK)


def _ticket_sort_key(ticket):
    """Total order: priority DESC (rank ASC), created_at ASC, ticket_id ASC."""
    return (
        _priority_rank(ticket.get("priority")),
        str(ticket.get("created_at", "")),
        str(ticket.get("ticket_id", "")),
    )


def explain_assignment(ticket, agent, config=None):
    """Pure predicate: can this one agent take this one ticket? No global state."""
    language_match = ticket.get("language") in (agent.get("languages") or [])
    skill_match = ticket.get("product") in (agent.get("skills") or [])
    capacity_available = agent.get("current_load", 0) < agent.get("capacity", 0)
    eligible = bool(language_match and skill_match and capacity_available)
    return {
        "eligible": eligible,
        "language_match": bool(language_match),
        "skill_match": bool(skill_match),
        "capacity_available": bool(capacity_available),
    }


def _unassigned_reason(ticket, agents, loads):
    """First-applicable reason, in the pinned order language -> skill -> capacity.

    `loads` is the effective current_load (including assignments made this call).
    """
    lang = ticket.get("language")
    product = ticket.get("product")
    lang_agents = [a for a in agents if lang in (a.get("languages") or [])]
    if not lang_agents:
        return "no_language_match"
    skill_agents = [a for a in lang_agents if product in (a.get("skills") or [])]
    if not skill_agents:
        return "no_skill_match"
    # language- and skill-qualified agents exist, but all are at capacity.
    return "no_capacity"


def assign_tickets(config, tickets, agents):
    """Assign tickets to agents per the pinned contract. Deterministic."""
    # Effective load per agent, seeded from current_load and bumped on assignment.
    loads = {a.get("agent_id"): a.get("current_load", 0) for a in agents}

    assigned = {}
    unassigned = {}

    for ticket in sorted(tickets, key=_ticket_sort_key):
        tid = ticket.get("ticket_id")
        lang = ticket.get("language")
        product = ticket.get("product")

        qualified = [
            a
            for a in agents
            if lang in (a.get("languages") or [])
            and product in (a.get("skills") or [])
            and loads.get(a.get("agent_id"), 0) < a.get("capacity", 0)
        ]

        if not qualified:
            unassigned[tid] = _unassigned_reason(ticket, agents, loads)
            continue

        # least current_load (effective), then agent_id ascending.
        winner = min(
            qualified,
            key=lambda a: (loads.get(a.get("agent_id"), 0), str(a.get("agent_id"))),
        )
        aid = winner.get("agent_id")
        assigned[tid] = aid
        loads[aid] = loads.get(aid, 0) + 1

    return {"assigned": assigned, "unassigned": unassigned}

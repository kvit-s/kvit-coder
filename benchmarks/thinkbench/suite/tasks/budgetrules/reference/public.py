"""Reference budgetrules.public — a rule-based transaction categorizer.

Stdlib only. `categorize` tags each transaction with the winning rule's category and
tags (highest priority wins, ties by rule order, no match -> "uncategorized").
`summarize` totals net spending (-amount_cents) by category and by month.
"""
import re


def _as_str(v):
    return v if isinstance(v, str) else None


def _rule_matches(rule, txn):
    """True iff every condition PRESENT in `rule` holds for `txn`.

    A condition that is absent imposes no constraint. Any malformed condition or bad
    value causes the rule to simply NOT match (never raises)."""
    if not isinstance(rule, dict):
        return False

    description = txn.get("description")
    if not isinstance(description, str):
        description = "" if description is None else str(description)
    amount = txn.get("amount_cents")

    if "description_contains" in rule:
        needle = _as_str(rule.get("description_contains"))
        if needle is None or needle.lower() not in description.lower():
            return False

    if "description_regex" in rule:
        pattern = _as_str(rule.get("description_regex"))
        if pattern is None:
            return False
        try:
            if re.search(pattern, description) is None:
                return False
        except re.error:
            return False

    if "amount_min_cents" in rule:
        lo = rule.get("amount_min_cents")
        if not isinstance(lo, (int, float)) or isinstance(lo, bool):
            return False
        if not isinstance(amount, (int, float)) or isinstance(amount, bool):
            return False
        if amount < lo:
            return False

    if "amount_max_cents" in rule:
        hi = rule.get("amount_max_cents")
        if not isinstance(hi, (int, float)) or isinstance(hi, bool):
            return False
        if not isinstance(amount, (int, float)) or isinstance(amount, bool):
            return False
        if amount > hi:
            return False

    if "merchant_equals" in rule:
        merchant = _as_str(rule.get("merchant_equals"))
        if merchant is None or description != merchant:
            return False

    return True


def _priority(rule):
    p = rule.get("priority", 0) if isinstance(rule, dict) else 0
    if isinstance(p, bool) or not isinstance(p, (int, float)):
        return 0
    return p


def categorize(transactions, rules):
    """Return a new list, same order/length as `transactions`, each element a copy of
    the input txn plus `category` (str) and `tags` (list[str])."""
    rule_list = list(rules) if isinstance(rules, list) else []
    out = []
    for txn in transactions:
        base = dict(txn) if isinstance(txn, dict) else {"value": txn}

        winner = None
        winner_pri = None
        for idx, rule in enumerate(rule_list):
            try:
                if not _rule_matches(rule, base):
                    continue
            except Exception:  # noqa: BLE001 - defensive: a bad rule never aborts
                continue
            pri = _priority(rule)
            # Highest priority wins; ties broken by earliest rule order (strict >).
            if winner is None or pri > winner_pri:
                winner, winner_pri = rule, pri

        if winner is None:
            base["category"] = "uncategorized"
            base["tags"] = []
        else:
            cat = _as_str(winner.get("set_category"))
            base["category"] = cat if cat is not None else "uncategorized"
            tags = winner.get("set_tags")
            if isinstance(tags, list):
                base["tags"] = [t for t in tags if isinstance(t, str)]
            else:
                base["tags"] = []
        out.append(base)
    return out


def _spending(txn):
    """Money that left the account: -amount_cents. Debits (negative input) -> positive
    spending; refunds/credits (positive input) -> negative spending (reduce totals)."""
    amt = txn.get("amount_cents")
    if isinstance(amt, bool) or not isinstance(amt, (int, float)):
        return 0
    return -amt


def summarize(categorized):
    """Total net spending by category and by month (YYYY-MM). Integer cents."""
    by_category = {}
    by_month = {}
    for txn in categorized:
        if not isinstance(txn, dict):
            continue
        spend = _spending(txn)

        cat = txn.get("category")
        if not isinstance(cat, str):
            cat = "uncategorized"
        by_category[cat] = by_category.get(cat, 0) + spend

        date = txn.get("date")
        if isinstance(date, str) and len(date) >= 7:
            month = date[:7]
            by_month[month] = by_month.get(month, 0) + spend

    by_category = {k: int(v) for k, v in by_category.items()}
    by_month = {k: int(v) for k, v in by_month.items()}
    return {"by_category": by_category, "by_month": by_month}

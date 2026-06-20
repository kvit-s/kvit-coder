"""Reference flagwise.public — a deterministic feature flag evaluator.

Pure standard library. Rollout bucketing is a deterministic hash of
flag_key + context["user_id"], so the same (flag, user) always lands in the
same bucket across calls and processes.
"""
import hashlib
import json

MISSING = object()

# --- reasons (any stable spelling is fine; the oracle derives the mapping) ----
REASON_RULE_MATCH = "rule_match"
REASON_ROLLOUT = "rollout"
REASON_DEFAULT = "default"
REASON_DISABLED = "disabled"
REASON_UNKNOWN_FLAG = "unknown_flag"


def load_config(path):
    with open(path) as f:
        return json.load(f)


# --- condition evaluation -----------------------------------------------------
_LEAF_OPS = {"equals", "not_equals", "in", "not_in", "exists", "greater_than", "less_than"}


def _eval_leaf_field(field, op_map, context):
    """Evaluate one field -> {operator: operand} mapping against the context."""
    present = field in context
    value = context.get(field, MISSING)
    result = True
    for op, operand in op_map.items():
        if op == "exists":
            ok = present if operand else (not present)
        elif op == "equals":
            ok = present and value == operand
        elif op == "not_equals":
            # a missing field is "not equal" to a concrete operand
            ok = (not present) or value != operand
        elif op == "in":
            ok = present and value in operand
        elif op == "not_in":
            ok = (not present) or value not in operand
        elif op == "greater_than":
            ok = present and _safe_cmp(value, operand, lambda a, b: a > b)
        elif op == "less_than":
            ok = present and _safe_cmp(value, operand, lambda a, b: a < b)
        else:
            ok = False  # unknown leaf operator -> condition cannot pass
        if not ok:
            return False
    return result


def _safe_cmp(a, b, fn):
    try:
        return fn(a, b)
    except TypeError:
        return False


def evaluate_condition(cond, context):
    """Evaluate a condition mapping. Combinators: and / or / not.

    A missing context field never raises; the leaf simply evaluates False
    (except `exists`, which tests presence).
    """
    if cond is None:
        return True
    if not isinstance(cond, dict):
        return bool(cond)

    result = True
    for key, val in cond.items():
        if key == "and":
            ok = all(evaluate_condition(sub, context) for sub in val)
        elif key == "or":
            ok = any(evaluate_condition(sub, context) for sub in val)
        elif key == "not":
            subs = val if isinstance(val, list) else [val]
            ok = not all(evaluate_condition(sub, context) for sub in subs)
        elif isinstance(val, dict) and (set(val) & _LEAF_OPS):
            ok = _eval_leaf_field(key, val, context)
        elif isinstance(val, dict):
            # nested field condition without recognised operators -> treat as
            # an equality-on-structure leaf (field must equal the mapping).
            ok = key in context and context[key] == val
        else:
            # shorthand: {field: literal} means equals
            ok = key in context and context[key] == val
        if not ok:
            return False
    return result


# --- deterministic rollout ----------------------------------------------------
def rollout_bucket(flag_key, user_id):
    """Stable bucket in [0, 100) derived from flag_key + user_id.

    Uses SHA-256 over a delimited concatenation so the mapping is reproducible
    across processes and Python invocations (unlike the salted builtin hash()).
    """
    seed = f"{flag_key}:{user_id}".encode("utf-8")
    digest = hashlib.sha256(seed).hexdigest()
    return int(digest[:8], 16) % 100


def _in_rollout(flag_key, context, percent):
    if percent is None:
        return False
    if percent >= 100:
        return True
    if percent <= 0:
        return False
    user_id = context.get("user_id", "")
    return rollout_bucket(flag_key, user_id) < percent


# --- flag evaluation ----------------------------------------------------------
def _result(key, value, reason, idx):
    return {"key": key, "value": value, "reason": reason, "matched_rule_index": idx}


def evaluate_flag(config, flag_key, context):
    flags = config.get("flags", {})
    if flag_key not in flags:
        return _result(flag_key, None, REASON_UNKNOWN_FLAG, None)

    flag = flags[flag_key]
    default = flag.get("default", False)

    if not flag.get("enabled", True):
        return _result(flag_key, default, REASON_DISABLED, None)

    for idx, rule in enumerate(flag.get("rules", [])):
        cond = rule.get("if")
        if not evaluate_condition(cond, context):
            continue
        if "rollout" in rule:
            if _in_rollout(flag_key, context, rule.get("rollout")):
                served = rule.get("serve", True)
                return _result(flag_key, served, REASON_ROLLOUT, idx)
            # rollout miss: rule did not decide, fall through to next rule
            continue
        served = rule.get("serve", True)
        return _result(flag_key, served, REASON_RULE_MATCH, idx)

    return _result(flag_key, default, REASON_DEFAULT, None)


def evaluate_all(config, context):
    return {key: evaluate_flag(config, key, context) for key in config.get("flags", {})}

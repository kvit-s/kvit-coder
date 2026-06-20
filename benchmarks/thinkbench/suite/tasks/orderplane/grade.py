#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield Task (orderplane).

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `orderplane.public`
API and the `python -m orderplane` CLI), NOT against the model's own tests and NOT
against any particular internal file layout or database schema.

Output: a JSON scorecard on stdout. Each check is independent, so the score is
continuous (passed / total), never binary. Exit code is 0 whenever grading ran to
completion (even score 0.0); nonzero only on a grader-internal failure.

Tolerance: the brief under-specifies the catalog JSON shape, the order JSON shape,
and the exact return-dict key names. This oracle accepts any contract-conformant
representation and checks BEHAVIOR (reservation, idempotency, discount-before-tax,
the taxable-shipping flag, cancellation/fulfillment transitions, cross-surface
agreement), not incidental key names. Every spot where it must assume a convention
the brief does not pin is marked `# ASSUMES`; those checks are kept either tolerant
or relative (comparing two scenarios the brief DOES pin against each other) so a
correct-but-differently-shaped solution is never penalised for our guess.
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

checks = []


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    checks.append({"id": cid, "desc": desc, "passed": bool(ok), "detail": str(detail or "")})


# --------------------------------------------------------------------------- #
# fixtures: fresh temp db + catalog file per scenario
# --------------------------------------------------------------------------- #
_tmpfiles = []


def fresh_db():
    fd, path = tempfile.mkstemp(suffix=".db", dir=ROOT)
    os.close(fd)
    os.remove(path)  # let the implementation create it
    _tmpfiles.append(path)
    return path


def write_json(obj):
    fd, path = tempfile.mkstemp(suffix=".json", dir=ROOT)
    with os.fdopen(fd, "w") as f:
        json.dump(obj, f)
    _tmpfiles.append(path)
    return path


# ASSUMES catalog JSON shape. The brief lists products/inventory/discount codes/
# sales tax/shipping fees but does not pin the file layout. We use a reasonable
# nested shape; a solution that accepts a different shape simply needs its own
# fixtures — but since the grader controls the catalog it feeds, we test the
# RULES against THIS shape and keep tax/discount/shipping numbers self-consistent.
def catalog(tax_rate=0.10):
    return {
        "tax_rate": tax_rate,
        # ASSUMES shipping methods keyed by name, each {fee, taxable}.
        "shipping": {
            "taxed": {"fee": 1000, "taxable": True},
            "untaxed": {"fee": 1000, "taxable": False},
            "free": {"fee": 0, "taxable": False},
        },
        # ASSUMES discount codes keyed by code, {type: percent|amount, value}.
        "discount_codes": {
            "PCT10": {"type": "percent", "value": 10},
            "AMT500": {"type": "amount", "value": 500},
        },
        # ASSUMES products carry sku/name/price(integer cents)/inventory(qty).
        "products": [
            {"sku": "AAA", "name": "Alpha", "price": 1000, "inventory": 10},
            {"sku": "BBB", "name": "Beta", "price": 2000, "inventory": 4},
        ],
    }


# ASSUMES order JSON shape: client_order_id (idempotency key), items [{sku, qty}],
# optional discount_code, optional shipping_method (key into catalog.shipping),
# optional placed_at timestamp. These mirror the brief's own CLI/term list.
def order(client_order_id, items, discount_code=None, shipping_method=None, placed_at=None):
    o = {"client_order_id": client_order_id, "items": items}
    if discount_code is not None:
        o["discount_code"] = discount_code
    if shipping_method is not None:
        o["shipping_method"] = shipping_method
    if placed_at is not None:
        o["placed_at"] = placed_at
    return o


# --------------------------------------------------------------------------- #
# tolerant readers — find values without committing to key names
# --------------------------------------------------------------------------- #
MISSING = object()


def deep_find_number(blob, key_substrings, exclude_substrings=()):
    """Find the first numeric value whose key contains any of key_substrings and
    none of exclude_substrings (case-insensitive). Prefers a TOP-LEVEL match (so
    an order-level 'total' wins over a per-line 'line_total') before descending."""
    subs = [s.lower() for s in key_substrings]
    excl = [s.lower() for s in exclude_substrings]

    def keymatch(k):
        kl = str(k).lower()
        return any(s in kl for s in subs) and not any(e in kl for e in excl)

    # pass 1: top-level keys only
    if isinstance(blob, dict):
        for k, v in blob.items():
            if isinstance(v, (int, float)) and not isinstance(v, bool) and keymatch(k):
                return v
    # pass 2: full depth-first sweep
    stack = [blob]
    while stack:
        cur = stack.pop(0)
        if isinstance(cur, dict):
            for k, v in cur.items():
                if isinstance(v, (int, float)) and not isinstance(v, bool) and keymatch(k):
                    return v
            stack.extend(cur.values())
        elif isinstance(cur, list):
            stack.extend(cur)
    return MISSING


def find_total(blob):
    """Tolerantly extract the order GRAND total. Excludes per-line/sub totals so a
    'line_total' or 'item_subtotal' never masquerades as the order total."""
    return deep_find_number(
        blob,
        ["total", "grand", "amount_due", "payable"],
        exclude_substrings=["line", "sub", "item", "unit"],
    )


def find_tax(blob):
    return deep_find_number(blob, ["tax"], exclude_substrings=["rate", "bp"])


def find_status(blob):
    """Tolerantly extract an order status string."""
    if isinstance(blob, dict):
        for k in ("status", "state", "order_status"):
            v = blob.get(k)
            if isinstance(v, str):
                return v.lower()
        # nested
        for v in blob.values():
            if isinstance(v, dict):
                s = find_status(v)
                if s:
                    return s
    return None


def is_errorish(blob):
    """Does a result clearly signal a rejection / error / refusal?"""
    if not isinstance(blob, (dict, list)):
        return False
    s = json.dumps(blob, default=str).lower()
    return any(tok in s for tok in ('"error"', "reject", "cannot", "invalid", "fail", "insufficient", "forbid"))


def deep_find_string_id(blob):
    """Tolerantly find an order id string: a value under a key containing 'order'
    and 'id', else any 'order_id'-ish key, else any string value under an 'id' key."""
    # ASSUMES the created order's id is returned under some key. The brief's CLI
    # uses ids like 'order_1', so we look for a string value keyed by *order*id*.
    if not isinstance(blob, dict):
        return None
    candidates = []
    for k, v in blob.items():
        kl = str(k).lower()
        if isinstance(v, str):
            if "order" in kl and "id" in kl and "client" not in kl:
                return v
            if kl in ("id", "order", "order_id", "orderid"):
                candidates.append(v)
    if candidates:
        return candidates[0]
    # nested fallback
    for v in blob.values():
        if isinstance(v, dict):
            got = deep_find_string_id(v)
            if got:
                return got
    return None


# --------------------------------------------------------------------------- #
# import the produced package (contract: orderplane.public)
# --------------------------------------------------------------------------- #
import_ok = True
import_detail = ""
pub = None
try:
    pub = importlib.import_module("orderplane.public")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


def setup(tax_rate=0.10):
    """Fresh db with the catalog loaded; returns db path."""
    db = fresh_db()
    pub.init_db(db)
    pub.load_catalog(db, write_json(catalog(tax_rate)))
    return db


if import_ok:
    # 1. inventory reservation on placement — a second order that would exceed the
    #    REMAINING (un-reserved) stock must be rejected, proving the first reserved.
    def c_reservation():
        db = setup()
        # BBB has inventory 4. Reserve 3.
        r1 = pub.place_order(db, order("c1", [{"sku": "BBB", "qty": 3}]))
        if is_errorish(r1):
            return False, f"first order rejected unexpectedly: {r1!r}"
        # Only 1 left; ordering 2 must fail because 3 are reserved.
        r2 = pub.place_order(db, order("c2", [{"sku": "BBB", "qty": 2}]))
        return is_errorish(r2), f"second-order (over remaining stock) result={r2!r}"

    check("reservation", "placing an order reserves inventory (over-reservation rejected)", c_reservation)

    # 2. idempotent placement by client_order_id — repeat submit = same order, and
    #    it does NOT reserve a second time (a third distinct order can still use stock).
    def c_idempotent():
        db = setup()
        o = order("dupe", [{"sku": "BBB", "qty": 4}])  # reserve all 4 of BBB
        r1 = pub.place_order(db, o)
        r2 = pub.place_order(db, o)  # same client_order_id
        if is_errorish(r1) or is_errorish(r2):
            return False, f"placement errored: {r1!r} / {r2!r}"
        # ASSUMES the order id is exposed under some key containing 'order' (e.g.
        # order_id). Compare the two responses for the SAME canonical order.
        t1 = json.dumps(r1, sort_keys=True, default=str)
        t2 = json.dumps(r2, sort_keys=True, default=str)
        same = (t1 == t2)
        # And no double-reservation: a fresh order for 1 more BBB must be rejected
        # (only 4 existed, all reserved once). If it were reserved twice we'd be at
        # -4 but availability logic should still reject; if NOT reserved at all the
        # second identical call would have freed nothing. Core signal is t1==t2.
        return same, f"identical resubmission identical={same}"

    check("idempotent", "resubmitting the same client_order_id yields the same order", c_idempotent)

    # 3. discounts apply BEFORE tax. Relative check: with a 10% item discount the
    #    tax charged must be strictly LESS than with no discount (same cart), and
    #    the total must drop by more than the bare discount (because tax also drops).
    def c_discount_before_tax():
        db1 = setup()
        plain = pub.place_order(db1, order("p", [{"sku": "AAA", "qty": 10}]))  # 10*1000=10000
        db2 = setup()
        disc = pub.place_order(db2, order("d", [{"sku": "AAA", "qty": 10}], discount_code="PCT10"))
        tax_plain, tax_disc = find_tax(plain), find_tax(disc)
        tot_plain, tot_disc = find_total(plain), find_total(disc)
        if MISSING in (tax_plain, tax_disc, tot_plain, tot_disc):
            return False, f"missing fields plain={plain!r} disc={disc!r}"
        # tax dropped because it's computed on the discounted base
        tax_dropped = tax_disc < tax_plain
        # total drop exceeds the bare 1000 discount because tax fell too
        total_drop = tot_plain - tot_disc
        return (tax_dropped and total_drop > 1000), (
            f"tax {tax_plain}->{tax_disc}, total drop={total_drop} (expect >1000)"
        )

    check("discount_before_tax", "discount lowers the taxable base (tax computed after discount)", c_discount_before_tax)

    # 4. amount discount path also reduces total (sanity that 'amount' codes work).
    def c_amount_discount():
        db1 = setup()
        plain = pub.place_order(db1, order("p", [{"sku": "AAA", "qty": 5}]))
        db2 = setup()
        disc = pub.place_order(db2, order("d", [{"sku": "AAA", "qty": 5}], discount_code="AMT500"))
        tp, td = find_total(plain), find_total(disc)
        if MISSING in (tp, td):
            return False, f"no totals plain={plain!r} disc={disc!r}"
        return (td < tp), f"total {tp} -> {td} with 500-off code"

    check("amount_discount", "a fixed-amount discount code reduces the order total", c_amount_discount)

    # 5. shipping taxable only when catalog says so. Relative check: two identical
    #    carts, one with a 'taxable' shipping method and one 'non-taxable', same fee.
    #    The taxable one must charge MORE tax (and thus a higher total).
    def c_taxable_shipping():
        db1 = setup()
        taxed = pub.place_order(db1, order("t", [{"sku": "AAA", "qty": 1}], shipping_method="taxed"))
        db2 = setup()
        untaxed = pub.place_order(db2, order("u", [{"sku": "AAA", "qty": 1}], shipping_method="untaxed"))
        tx_taxed, tx_untaxed = find_tax(taxed), find_tax(untaxed)
        if MISSING in (tx_taxed, tx_untaxed):
            return False, f"no tax fields taxed={taxed!r} untaxed={untaxed!r}"
        return (tx_taxed > tx_untaxed), (
            f"tax with taxable shipping={tx_taxed} > non-taxable={tx_untaxed}"
        )

    check("taxable_shipping", "shipping fee is taxed only when the catalog flags it taxable", c_taxable_shipping)

    # 6. cancellation of an UNFULFILLED order releases its reservation, so the
    #    freed stock becomes orderable again.
    def c_cancel_releases():
        db = setup()
        r = pub.place_order(db, order("c", [{"sku": "BBB", "qty": 4}]))  # reserve all 4
        oid = deep_find_string_id(r)
        if oid is None:
            return False, f"no order id in {r!r}"
        # all BBB reserved; a new order for 1 must currently fail
        blocked = pub.place_order(db, order("x", [{"sku": "BBB", "qty": 1}]))
        if not is_errorish(blocked):
            return False, f"stock not actually reserved before cancel: {blocked!r}"
        cancel = pub.cancel_order(db, oid, "customer_request")
        if is_errorish(cancel) and find_status(cancel) not in ("canceled", "cancelled"):
            return False, f"cancel failed: {cancel!r}"
        # now the freed stock should be orderable
        after = pub.place_order(db, order("y", [{"sku": "BBB", "qty": 1}]))
        return (not is_errorish(after)), f"post-cancel reorder result={after!r}"

    check("cancel_releases", "canceling an unfulfilled order releases reserved inventory", c_cancel_releases)

    # 7. fulfilled items cannot be canceled retroactively.
    def c_no_cancel_after_fulfill():
        db = setup()
        r = pub.place_order(db, order("c", [{"sku": "AAA", "qty": 1}]))
        oid = deep_find_string_id(r)
        if oid is None:
            return False, f"no order id in {r!r}"
        pub.fulfill_order(db, oid, "2026-01-02T12:00:00Z")
        res = pub.cancel_order(db, oid, "too_late")
        # must NOT end up canceled: either an error/refusal, or status stays fulfilled
        st = find_status(res)
        refused = is_errorish(res) or st in ("fulfilled", "shipped", "complete", "completed")
        # double-check canonical state via customer view
        cv = pub.customer_order_view(db, oid)
        cv_st = find_status(cv)
        not_canceled = cv_st not in ("canceled", "cancelled")
        return (refused and not_canceled), f"cancel-result status={st!r}, canonical status={cv_st!r}"

    check("no_cancel_after_fulfill", "a fulfilled order cannot be canceled retroactively", c_no_cancel_after_fulfill)

    # 8. revenue export uses FULFILLED orders only: a placed-but-unfulfilled order
    #    and a canceled order must NOT appear; a fulfilled one must.
    def c_revenue_fulfilled_only():
        db = setup()
        # fulfilled order in 2026-01
        rf = pub.place_order(db, order("f", [{"sku": "AAA", "qty": 1}], placed_at="2026-01-05T00:00:00Z"))
        fid = deep_find_string_id(rf)
        pub.fulfill_order(db, fid, "2026-01-06T00:00:00Z")
        # placed but not fulfilled
        pub.place_order(db, order("p", [{"sku": "AAA", "qty": 1}], placed_at="2026-01-05T00:00:00Z"))
        # canceled
        rc = pub.place_order(db, order("x", [{"sku": "AAA", "qty": 1}], placed_at="2026-01-05T00:00:00Z"))
        pub.cancel_order(db, deep_find_string_id(rc), "customer_request")

        rev = pub.export_revenue(db, "2026-01")
        if not isinstance(rev, list):
            return False, f"export not a list: {rev!r}"
        # exactly one revenue row (the fulfilled order)
        # ASSUMES each row references its order id somewhere; count rows that do.
        rows_for_fulfilled = [r for r in rev if fid and fid in json.dumps(r, default=str)]
        ok = (len(rev) == 1) and (len(rows_for_fulfilled) == 1 or len(rev) == 1)
        return ok, f"rows={len(rev)} (expect 1, the fulfilled order); rev={rev!r}"

    check("revenue_fulfilled_only", "revenue export includes only fulfilled orders", c_revenue_fulfilled_only)

    # 9. revenue uses FINALIZED fulfilled data, not recomputed cart: the exported
    #    total for the fulfilled order must equal that order's persisted total
    #    (from its own view), to the cent.
    def c_revenue_matches_finalized():
        db = setup()
        r = pub.place_order(db, order("f", [{"sku": "AAA", "qty": 3}], discount_code="PCT10",
                                       shipping_method="taxed", placed_at="2026-02-01T00:00:00Z"))
        oid = deep_find_string_id(r)
        finalized = pub.fulfill_order(db, oid, "2026-02-02T00:00:00Z")
        view_total = find_total(finalized)
        if view_total is MISSING:
            view_total = find_total(pub.customer_order_view(db, oid))
        rev = pub.export_revenue(db, "2026-02")
        if not isinstance(rev, list) or not rev:
            return False, f"empty export: {rev!r}"
        rev_total = find_total(rev[0]) if isinstance(rev[0], dict) else MISSING
        if MISSING in (view_total, rev_total):
            return False, f"missing totals view={view_total!r} rev={rev_total!r}"
        return (rev_total == view_total), f"export total {rev_total} == order total {view_total}"

    check("revenue_matches_finalized", "exported revenue equals the order's finalized total", c_revenue_matches_finalized)

    # 10. cross-surface agreement: customer view, admin view, warehouse picklist,
    #     and revenue export agree on canonical order state.
    def c_cross_surface():
        db = setup()
        r = pub.place_order(db, order("c", [{"sku": "AAA", "qty": 2}], placed_at="2026-03-04T00:00:00Z"))
        oid = deep_find_string_id(r)
        if oid is None:
            return False, f"no order id in {r!r}"
        cv = pub.customer_order_view(db, oid)
        av = pub.admin_order_view(db, oid)
        # before fulfillment: same status across customer & admin; appears on picklist;
        # absent from revenue.
        st_match = find_status(cv) == find_status(av)
        picklist = pub.warehouse_picklist(db, "2026-03-04")
        on_picklist = isinstance(picklist, list) and any(
            oid in json.dumps(row, default=str) for row in picklist
        )
        rev_before = pub.export_revenue(db, "2026-03")
        absent_from_rev = isinstance(rev_before, list) and not any(
            oid in json.dumps(row, default=str) for row in rev_before
        )
        # after fulfillment: off the picklist, present in revenue, status agrees.
        pub.fulfill_order(db, oid, "2026-03-05T00:00:00Z")
        picklist2 = pub.warehouse_picklist(db, "2026-03-04")
        off_picklist = isinstance(picklist2, list) and not any(
            oid in json.dumps(row, default=str) for row in picklist2
        )
        rev_after = pub.export_revenue(db, "2026-03")
        in_rev = isinstance(rev_after, list) and any(
            oid in json.dumps(row, default=str) for row in rev_after
        )
        cv2, av2 = pub.customer_order_view(db, oid), pub.admin_order_view(db, oid)
        st_match2 = find_status(cv2) == find_status(av2)
        ok = st_match and on_picklist and absent_from_rev and off_picklist and in_rev and st_match2
        return ok, (
            f"pre: status_match={st_match} on_picklist={on_picklist} absent_rev={absent_from_rev}; "
            f"post: off_picklist={off_picklist} in_rev={in_rev} status_match={st_match2}"
        )

    check("cross_surface", "customer/admin/warehouse/revenue surfaces agree on canonical state", c_cross_surface)


# --------------------------------------------------------------------------- #
# CLI: all output must be JSON, and must drive the same canonical state.
# --------------------------------------------------------------------------- #
def run_cli(args, db):
    proc = subprocess.run(
        [sys.executable, "-m", "orderplane", *args, "--db", db],
        capture_output=True, text=True, timeout=60, cwd=ROOT,
    )
    return proc


def c_cli_contract():
    db = fresh_db()
    cat = write_json(catalog())
    ordfile = write_json(order("cli1", [{"sku": "AAA", "qty": 2}],
                               shipping_method="taxed", placed_at="2026-04-01T00:00:00Z"))
    # init-db, load-catalog, place-order, customer-view, export — all JSON.
    steps = [
        ["init-db"],
        ["load-catalog", "--catalog", cat],
        ["place-order", "--order", ordfile],
    ]
    for s in steps:
        p = run_cli(s, db)
        json.loads(p.stdout)  # raises if not JSON
    # find the placed order id from place-order output
    p_place = run_cli(["place-order", "--order", ordfile], db)  # idempotent repeat
    placed = json.loads(p_place.stdout)
    oid = deep_find_string_id(placed) or "order_1"
    # customer-view JSON
    p_cv = run_cli(["customer-view", "--order", oid], db)
    json.loads(p_cv.stdout)
    return True, f"init/load/place/customer-view all emitted JSON; oid={oid}"


check("cli_json_contract", "CLI subcommands emit JSON and drive canonical state", c_cli_contract)


def c_cli_picklist_export_json():
    db = fresh_db()
    cat = write_json(catalog())
    ordfile = write_json(order("cli2", [{"sku": "AAA", "qty": 1}],
                               placed_at="2026-05-01T00:00:00Z"))
    run_cli(["init-db"], db)
    run_cli(["load-catalog", "--catalog", cat], db)
    run_cli(["place-order", "--order", ordfile], db)
    pl = json.loads(run_cli(["picklist", "--date", "2026-05-01"], db).stdout)
    rev = json.loads(run_cli(["export-revenue", "--month", "2026-05"], db).stdout)
    return (isinstance(pl, list) and isinstance(rev, list)), f"picklist/export return JSON lists; pl={pl!r}"


check("cli_picklist_export_json", "CLI picklist and export-revenue emit JSON lists", c_cli_picklist_export_json)


# --------------------------------------------------------------------------- #
# scorecard
# --------------------------------------------------------------------------- #
def cleanup():
    for p in _tmpfiles:
        try:
            os.remove(p)
        except OSError:
            pass


try:
    passed = sum(1 for c in checks if c["passed"])
    total = len(checks)
    card = {
        "task": "orderplane",
        "import_ok": import_ok,
        "import_detail": import_detail,
        "passed": passed,
        "total": total,
        "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
        "checks": checks,
    }
    print(json.dumps(card))
finally:
    cleanup()

sys.exit(0)

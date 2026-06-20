"""Reference orderplane.public — an order processing simulator on SQLite.

Money is represented internally in integer cents to keep arithmetic exact.

Catalog JSON shape (reference convention):

    {
      "tax_rate": 0.08,                # fraction applied to taxable subtotal
      "shipping": {
        "standard": {"fee": 599, "taxable": true},
        "free":     {"fee": 0,   "taxable": false}
      },
      "discount_codes": {
        "SAVE10": {"type": "percent", "value": 10},   # 10% off item subtotal
        "FIVEOFF": {"type": "amount", "value": 500}    # 500 cents off item subtotal
      },
      "products": [
        {"sku": "WIDGET", "name": "Widget", "price": 1000, "inventory": 5},
        {"sku": "GADGET", "name": "Gadget", "price": 2500, "inventory": 3}
      ]
    }

Order JSON shape (reference convention):

    {
      "client_order_id": "c-123",        # idempotency key
      "items": [{"sku": "WIDGET", "qty": 2}],
      "discount_code": "SAVE10",          # optional
      "shipping_method": "standard"       # optional, key into catalog.shipping
    }

All monetary amounts in returned dicts are integer cents.
"""
import json
import sqlite3


# --------------------------------------------------------------------------- #
# schema / connection
# --------------------------------------------------------------------------- #
def _connect(db_path):
    conn = sqlite3.connect(db_path)
    conn.row_factory = sqlite3.Row
    conn.execute("PRAGMA foreign_keys = ON")
    return conn


def init_db(db_path):
    conn = _connect(db_path)
    try:
        conn.executescript(
            """
            CREATE TABLE IF NOT EXISTS config (
                key   TEXT PRIMARY KEY,
                value TEXT NOT NULL
            );
            CREATE TABLE IF NOT EXISTS products (
                sku       TEXT PRIMARY KEY,
                name      TEXT NOT NULL,
                price     INTEGER NOT NULL
            );
            CREATE TABLE IF NOT EXISTS inventory (
                sku       TEXT PRIMARY KEY,
                on_hand   INTEGER NOT NULL,      -- physical units present
                reserved  INTEGER NOT NULL DEFAULT 0
            );
            CREATE TABLE IF NOT EXISTS shipping_methods (
                method  TEXT PRIMARY KEY,
                fee     INTEGER NOT NULL,
                taxable INTEGER NOT NULL          -- 0/1
            );
            CREATE TABLE IF NOT EXISTS discount_codes (
                code   TEXT PRIMARY KEY,
                type   TEXT NOT NULL,             -- 'percent' | 'amount'
                value  INTEGER NOT NULL
            );
            CREATE TABLE IF NOT EXISTS orders (
                order_id        TEXT PRIMARY KEY,
                client_order_id TEXT UNIQUE,
                status          TEXT NOT NULL,    -- placed | fulfilled | canceled
                discount_code   TEXT,
                shipping_method TEXT,
                shipping_fee    INTEGER NOT NULL,
                shipping_taxable INTEGER NOT NULL,
                item_subtotal   INTEGER NOT NULL, -- before discount
                discount_amount INTEGER NOT NULL,
                tax_rate_bp     INTEGER NOT NULL, -- basis points (rate * 10000)
                tax_amount      INTEGER NOT NULL,
                total           INTEGER NOT NULL,
                reason          TEXT,
                placed_at       TEXT,
                shipped_at      TEXT,
                canceled_at     TEXT
            );
            CREATE TABLE IF NOT EXISTS order_items (
                order_id  TEXT NOT NULL,
                sku       TEXT NOT NULL,
                qty       INTEGER NOT NULL,
                unit_price INTEGER NOT NULL,
                fulfilled INTEGER NOT NULL DEFAULT 0,
                canceled  INTEGER NOT NULL DEFAULT 0,
                PRIMARY KEY (order_id, sku)
            );
            """
        )
        conn.commit()
    finally:
        conn.close()


# --------------------------------------------------------------------------- #
# catalog
# --------------------------------------------------------------------------- #
def load_catalog(db_path, catalog_path):
    with open(catalog_path) as f:
        catalog = json.load(f)
    conn = _connect(db_path)
    try:
        tax_rate = float(catalog.get("tax_rate", 0.0))
        conn.execute(
            "INSERT OR REPLACE INTO config(key, value) VALUES('tax_rate', ?)",
            (str(tax_rate),),
        )
        for p in catalog.get("products", []):
            conn.execute(
                "INSERT OR REPLACE INTO products(sku, name, price) VALUES(?,?,?)",
                (p["sku"], p.get("name", p["sku"]), int(p["price"])),
            )
            conn.execute(
                "INSERT OR REPLACE INTO inventory(sku, on_hand, reserved) "
                "VALUES(?, ?, COALESCE((SELECT reserved FROM inventory WHERE sku=?), 0))",
                (p["sku"], int(p.get("inventory", 0)), p["sku"]),
            )
        for method, spec in catalog.get("shipping", {}).items():
            conn.execute(
                "INSERT OR REPLACE INTO shipping_methods(method, fee, taxable) VALUES(?,?,?)",
                (method, int(spec.get("fee", 0)), 1 if spec.get("taxable") else 0),
            )
        for code, spec in catalog.get("discount_codes", {}).items():
            conn.execute(
                "INSERT OR REPLACE INTO discount_codes(code, type, value) VALUES(?,?,?)",
                (code, spec["type"], int(spec["value"])),
            )
        conn.commit()
    finally:
        conn.close()


# --------------------------------------------------------------------------- #
# pricing helpers
# --------------------------------------------------------------------------- #
def _tax_rate(conn):
    row = conn.execute("SELECT value FROM config WHERE key='tax_rate'").fetchone()
    return float(row["value"]) if row else 0.0


def _compute_discount(conn, code, item_subtotal):
    if not code:
        return 0
    row = conn.execute(
        "SELECT type, value FROM discount_codes WHERE code=?", (code,)
    ).fetchone()
    if not row:
        return 0
    if row["type"] == "percent":
        disc = item_subtotal * int(row["value"]) // 100
    else:  # amount
        disc = int(row["value"])
    return max(0, min(disc, item_subtotal))


def _next_order_id(conn):
    n = conn.execute("SELECT COUNT(*) AS c FROM orders").fetchone()["c"]
    return f"order_{n + 1}"


# --------------------------------------------------------------------------- #
# place_order
# --------------------------------------------------------------------------- #
def place_order(db_path, order):
    conn = _connect(db_path)
    try:
        client_id = order.get("client_order_id")

        # idempotency: a repeat submission returns the SAME order, no new reservation
        if client_id is not None:
            existing = conn.execute(
                "SELECT order_id FROM orders WHERE client_order_id=?", (client_id,)
            ).fetchone()
            if existing:
                return _customer_view(conn, existing["order_id"])

        items = order.get("items", [])

        # check availability and gather prices
        priced = []
        item_subtotal = 0
        for it in items:
            sku, qty = it["sku"], int(it["qty"])
            prod = conn.execute(
                "SELECT price FROM products WHERE sku=?", (sku,)
            ).fetchone()
            if prod is None:
                return {"status": "rejected", "error": f"unknown sku {sku}"}
            inv = conn.execute(
                "SELECT on_hand, reserved FROM inventory WHERE sku=?", (sku,)
            ).fetchone()
            available = (inv["on_hand"] - inv["reserved"]) if inv else 0
            if qty > available:
                return {
                    "status": "rejected",
                    "error": f"insufficient inventory for {sku}",
                    "sku": sku,
                    "requested": qty,
                    "available": available,
                }
            unit = int(prod["price"])
            priced.append((sku, qty, unit))
            item_subtotal += unit * qty

        # shipping
        method = order.get("shipping_method")
        shipping_fee, shipping_taxable = 0, 0
        if method:
            srow = conn.execute(
                "SELECT fee, taxable FROM shipping_methods WHERE method=?", (method,)
            ).fetchone()
            if srow:
                shipping_fee = int(srow["fee"])
                shipping_taxable = int(srow["taxable"])

        # discount BEFORE tax
        discount = _compute_discount(conn, order.get("discount_code"), item_subtotal)
        discounted_items = item_subtotal - discount

        # tax: on discounted item subtotal + (shipping if taxable)
        rate = _tax_rate(conn)
        taxable_base = discounted_items + (shipping_fee if shipping_taxable else 0)
        tax_amount = round(taxable_base * rate)

        total = discounted_items + shipping_fee + tax_amount

        order_id = _next_order_id(conn)
        placed_at = order.get("placed_at") or order.get("created_at")

        conn.execute(
            "INSERT INTO orders(order_id, client_order_id, status, discount_code, "
            "shipping_method, shipping_fee, shipping_taxable, item_subtotal, "
            "discount_amount, tax_rate_bp, tax_amount, total, reason, placed_at, "
            "shipped_at, canceled_at) VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)",
            (
                order_id, client_id, "placed", order.get("discount_code"),
                method, shipping_fee, shipping_taxable, item_subtotal,
                discount, round(rate * 10000), tax_amount, total, None,
                placed_at, None, None,
            ),
        )
        for sku, qty, unit in priced:
            conn.execute(
                "INSERT INTO order_items(order_id, sku, qty, unit_price, fulfilled, canceled) "
                "VALUES(?,?,?,?,0,0)",
                (order_id, sku, qty, unit),
            )
            # RESERVE inventory at placement
            conn.execute(
                "UPDATE inventory SET reserved = reserved + ? WHERE sku=?", (qty, sku)
            )
        conn.commit()
        return _customer_view(conn, order_id)
    finally:
        conn.close()


# --------------------------------------------------------------------------- #
# cancel_order
# --------------------------------------------------------------------------- #
def cancel_order(db_path, order_id, reason):
    conn = _connect(db_path)
    try:
        o = conn.execute(
            "SELECT status FROM orders WHERE order_id=?", (order_id,)
        ).fetchone()
        if o is None:
            return {"status": "error", "error": f"unknown order {order_id}"}
        if o["status"] == "fulfilled":
            # Fulfilled items cannot be canceled retroactively.
            return {
                "status": "error",
                "error": "cannot cancel a fulfilled order",
                "order_id": order_id,
            }
        if o["status"] == "canceled":
            return _customer_view(conn, order_id)

        # release inventory for every still-unfulfilled item
        rows = conn.execute(
            "SELECT sku, qty, fulfilled, canceled FROM order_items WHERE order_id=?",
            (order_id,),
        ).fetchall()
        for r in rows:
            if not r["fulfilled"] and not r["canceled"]:
                conn.execute(
                    "UPDATE inventory SET reserved = reserved - ? WHERE sku=?",
                    (r["qty"], r["sku"]),
                )
                conn.execute(
                    "UPDATE order_items SET canceled = 1 WHERE order_id=? AND sku=?",
                    (order_id, r["sku"]),
                )
        conn.execute(
            "UPDATE orders SET status='canceled', reason=?, canceled_at=? WHERE order_id=?",
            (reason, "canceled", order_id),
        )
        conn.commit()
        return _customer_view(conn, order_id)
    finally:
        conn.close()


# --------------------------------------------------------------------------- #
# fulfill_order
# --------------------------------------------------------------------------- #
def fulfill_order(db_path, order_id, shipped_at):
    conn = _connect(db_path)
    try:
        o = conn.execute(
            "SELECT status FROM orders WHERE order_id=?", (order_id,)
        ).fetchone()
        if o is None:
            return {"status": "error", "error": f"unknown order {order_id}"}
        if o["status"] == "canceled":
            return {"status": "error", "error": "cannot fulfill a canceled order"}

        rows = conn.execute(
            "SELECT sku, qty, fulfilled, canceled FROM order_items WHERE order_id=?",
            (order_id,),
        ).fetchall()
        for r in rows:
            if not r["fulfilled"] and not r["canceled"]:
                # ship reserved units: reduce on_hand and release the reservation
                conn.execute(
                    "UPDATE inventory SET on_hand = on_hand - ?, reserved = reserved - ? "
                    "WHERE sku=?",
                    (r["qty"], r["qty"], r["sku"]),
                )
                conn.execute(
                    "UPDATE order_items SET fulfilled = 1 WHERE order_id=? AND sku=?",
                    (order_id, r["sku"]),
                )
        conn.execute(
            "UPDATE orders SET status='fulfilled', shipped_at=? WHERE order_id=?",
            (shipped_at, order_id),
        )
        conn.commit()
        return _customer_view(conn, order_id)
    finally:
        conn.close()


# --------------------------------------------------------------------------- #
# views (canonical state read straight from persisted columns)
# --------------------------------------------------------------------------- #
def _order_row(conn, order_id):
    return conn.execute(
        "SELECT * FROM orders WHERE order_id=?", (order_id,)
    ).fetchone()


def _items(conn, order_id):
    return conn.execute(
        "SELECT sku, qty, unit_price, fulfilled, canceled FROM order_items "
        "WHERE order_id=? ORDER BY sku",
        (order_id,),
    ).fetchall()


def _customer_view(conn, order_id):
    o = _order_row(conn, order_id)
    if o is None:
        return {"status": "error", "error": f"unknown order {order_id}"}
    items = [
        {
            "sku": r["sku"],
            "qty": r["qty"],
            "unit_price": r["unit_price"],
            "line_total": r["unit_price"] * r["qty"],
            "fulfilled": bool(r["fulfilled"]),
            "canceled": bool(r["canceled"]),
        }
        for r in _items(conn, order_id)
    ]
    return {
        "order_id": o["order_id"],
        "client_order_id": o["client_order_id"],
        "status": o["status"],
        "items": items,
        "item_subtotal": o["item_subtotal"],
        "discount_amount": o["discount_amount"],
        "shipping_fee": o["shipping_fee"],
        "tax_amount": o["tax_amount"],
        "total": o["total"],
    }


def customer_order_view(db_path, order_id):
    conn = _connect(db_path)
    try:
        return _customer_view(conn, order_id)
    finally:
        conn.close()


def admin_order_view(db_path, order_id):
    conn = _connect(db_path)
    try:
        o = _order_row(conn, order_id)
        if o is None:
            return {"status": "error", "error": f"unknown order {order_id}"}
        view = _customer_view(conn, order_id)
        # admin adds operational / financial detail on top of the canonical state
        view.update(
            {
                "discount_code": o["discount_code"],
                "shipping_method": o["shipping_method"],
                "shipping_taxable": bool(o["shipping_taxable"]),
                "tax_rate": o["tax_rate_bp"] / 10000.0,
                "reason": o["reason"],
                "placed_at": o["placed_at"],
                "shipped_at": o["shipped_at"],
                "canceled_at": o["canceled_at"],
            }
        )
        return view
    finally:
        conn.close()


def warehouse_picklist(db_path, date):
    """Items that need physically picking: placed (not fulfilled, not canceled)
    orders. `date` filters by the order's shipped/placed day when present; an
    order with no date still appears so the warehouse never loses work."""
    conn = _connect(db_path)
    try:
        rows = conn.execute(
            "SELECT o.order_id AS order_id, i.sku AS sku, i.qty AS qty, "
            "o.placed_at AS placed_at "
            "FROM orders o JOIN order_items i ON o.order_id = i.order_id "
            "WHERE o.status = 'placed' AND i.fulfilled = 0 AND i.canceled = 0 "
            "ORDER BY o.order_id, i.sku"
        ).fetchall()
        out = []
        for r in rows:
            placed = r["placed_at"]
            if date and placed and not str(placed).startswith(date):
                continue
            out.append(
                {"order_id": r["order_id"], "sku": r["sku"], "qty": r["qty"]}
            )
        return out
    finally:
        conn.close()


def export_revenue(db_path, month):
    """Finalized revenue from FULFILLED orders only, read from persisted order
    totals (never recomputed from a cart). `month` is a YYYY-MM prefix matched
    against shipped_at."""
    conn = _connect(db_path)
    try:
        rows = conn.execute(
            "SELECT order_id, shipped_at, item_subtotal, discount_amount, "
            "shipping_fee, tax_amount, total FROM orders "
            "WHERE status='fulfilled' ORDER BY order_id"
        ).fetchall()
        out = []
        for r in rows:
            shipped = r["shipped_at"] or ""
            if month and not str(shipped).startswith(month):
                continue
            net = r["item_subtotal"] - r["discount_amount"]
            out.append(
                {
                    "order_id": r["order_id"],
                    "shipped_at": r["shipped_at"],
                    "net_revenue": net,
                    "shipping": r["shipping_fee"],
                    "tax": r["tax_amount"],
                    "total": r["total"],
                }
            )
        return out
    finally:
        conn.close()

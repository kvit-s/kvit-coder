"""Reference microapi.public — a small stdlib HTTP router over http.server.

Contract-conformant implementation used to anchor "correct" and to self-test the
held-out grader (../grade.py). NOT shown to the agent.

Routing model: patterns are compiled into segment lists. Matching a path tries every
registered route and ranks the matches by specificity (static > param > wildcard), so
precedence does not depend on registration order. 404 vs 405 is decided by whether the
PATH matched any route at all (for any method).
"""
import json
from urllib.parse import parse_qs, unquote


class Request:
    """The first positional argument handed to every handler."""

    def __init__(self, method, path, headers, query, json_body):
        self.method = method
        self.path = path
        self.headers = headers
        self.query = query
        self.json = json_body


# --- pattern compilation -----------------------------------------------------
# Each compiled segment is (kind, name): kind in {"static","param","wildcard"}.
# For static, name holds the literal text; for param/wildcard, the capture name.

def _compile(pattern):
    raw = [s for s in pattern.split("/") if s != ""]
    segs = []
    for idx, s in enumerate(raw):
        if s.startswith("{") and s.endswith("}"):
            inner = s[1:-1]
            if inner.endswith(":*"):
                name = inner[:-2]
                if idx != len(raw) - 1:
                    raise ValueError("wildcard must be the final segment")
                segs.append(("wildcard", name))
            else:
                segs.append(("param", inner))
        else:
            segs.append(("static", s))
    return segs


def _match(segs, parts):
    """Try to match compiled `segs` against path `parts` (list of decoded segments).

    Returns (params, score) or None. score is a specificity tuple; LARGER is more
    specific. We score per-segment: static=2, param=1, wildcard=0, so an all-static
    match outranks any param match, which outranks any wildcard match.
    """
    params = {}
    score = []
    i = 0
    for j, (kind, name) in enumerate(segs):
        if kind == "wildcard":
            if i >= len(parts):
                return None  # wildcard needs at least one remaining segment
            params[name] = "/".join(parts[i:])
            score.append(0)
            i = len(parts)
            return params, tuple(score)
        if i >= len(parts):
            return None
        if kind == "static":
            if parts[i] != name:
                return None
            score.append(2)
        else:  # param
            params[name] = parts[i]
            score.append(1)
        i += 1
    if i != len(parts):
        return None  # leftover path segments and no wildcard to absorb them
    return params, tuple(score)


class App:
    def __init__(self):
        self._routes = []  # list of (method, segs, handler)
        self._middleware = []  # list of callables

    def route(self, method, path):
        method = method.upper()
        segs = _compile(path)

        def deco(fn):
            self._routes.append((method, segs, fn))
            return fn

        return deco

    def use(self, middleware):
        self._middleware.append(middleware)
        return middleware

    # --- dispatch ------------------------------------------------------------
    def _resolve(self, method, parts):
        """Return (handler, params, path_matched).

        path_matched is True if ANY route's pattern matched the path (regardless of
        method) — used to pick 405 over 404. Among method-matching candidates the most
        specific (largest score) wins.
        """
        path_matched = False
        best = None  # (score, handler, params)
        for rmethod, segs, handler in self._routes:
            m = _match(segs, parts)
            if m is None:
                continue
            path_matched = True
            if rmethod != method:
                continue
            params, score = m
            if best is None or score > best[0]:
                best = (score, handler, params)
        if best is None:
            return None, None, path_matched
        return best[1], best[2], path_matched

    def handle_request(self, method, path, headers, body):
        method = (method or "").upper()
        headers = headers or {}

        # split path / query
        if "?" in path:
            raw_path, _, raw_query = path.partition("?")
        else:
            raw_path, raw_query = path, ""

        query = {}
        if raw_query:
            for k, vals in parse_qs(raw_query, keep_blank_values=True).items():
                query[k] = vals[-1]  # last value wins

        parts = [unquote(s) for s in raw_path.split("/") if s != ""]

        # parse JSON body (never raises out)
        json_body = None
        bad_json = False
        if body:
            try:
                text = body.decode("utf-8") if isinstance(body, (bytes, bytearray)) else body
                json_body = json.loads(text)
            except Exception:  # noqa: BLE001
                bad_json = True

        request = Request(method, raw_path, headers, query, json_body)

        handler, params, path_matched = self._resolve(method, parts)

        if handler is None:
            if path_matched:
                return self._error(405, "method not allowed")
            return self._error(404, "not found")

        if bad_json:
            return self._error(400, "invalid JSON body")

        def final(req):
            return handler(req, **params)

        # build the middleware chain: first registered is outermost
        chain = final
        for mw in reversed(self._middleware):
            chain = self._wrap(mw, chain)

        try:
            result = chain(request)
        except Exception as e:  # noqa: BLE001
            return self._error(500, f"internal error: {type(e).__name__}")

        return self._encode(result)

    @staticmethod
    def _wrap(mw, nxt):
        def wrapped(req):
            return mw(req, nxt)

        return wrapped

    # --- response encoding ---------------------------------------------------
    @staticmethod
    def _encode(result):
        status = 200
        value = result
        if isinstance(result, tuple) and len(result) == 2 and isinstance(result[0], int):
            status, value = result
        body = json.dumps(value).encode("utf-8")
        return status, {"Content-Type": "application/json"}, body

    @staticmethod
    def _error(status, message):
        body = json.dumps({"error": message}).encode("utf-8")
        return status, {"Content-Type": "application/json"}, body

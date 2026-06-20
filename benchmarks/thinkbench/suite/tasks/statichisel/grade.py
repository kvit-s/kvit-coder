#!/usr/bin/env python3
"""Held-out behavior-level oracle for greenfield task `statichisel` (static site gen).

Dropped into the workspace ONLY after the agent stops — the agent never sees it.
Grades the produced package against the BRIEF'S CONTRACT (the `statichisel.public`
`build_site` API and the `python -m statichisel build` CLI), NOT against the model's
own tests and NOT against any particular internal file layout or exact HTML bytes.

The grader writes its OWN source site tree into a temp dir, calls `build_site` into a
separate temp output dir, and drives the CLI on a third pair of temp dirs. It then
inspects the GENERATED FILES ON DISK + the returned manifest. HTML is checked for
required CONTENT/STRUCTURE (bold wrapped in <strong>/<b>, the title substituted,
assets copied, nav present, ...), never for byte-equality — the markup is impl-specific.

FIXED DENOMINATOR: the full list of checks is the same whether or not the package
imports. On an import failure every check is recorded as FAILED (so the denominator
never shrinks to make a broken package look better) and the final score is forced to
0.0. Each check is independent; the score is continuous (passed / total), never binary.

Tolerance: the brief under-specifies the manifest shape, asset destination subpath,
slug->file mapping, and CLI stdout. This oracle DERIVES rather than REQUIRES: it accepts
any contract-conformant representation and checks BEHAVIOR. Spots where it fixes a
convention the bare SPEC leaves open are marked `# ASSUMES` and are pinned in brief.txt's
Contract section, so we never grade a hidden guess.

Output: a JSON scorecard on stdout. Exit code 0 whenever grading ran to completion (even
score 0.0); nonzero only on a grader-internal failure.
"""
import importlib
import json
import os
import re
import shutil
import subprocess
import sys
import tempfile

ROOT = os.getcwd()
if ROOT not in sys.path:
    sys.path.insert(0, ROOT)

# ----------------------------------------------------------------------------
# The FIXED check roster. IDs + descriptions are declared up front so the
# denominator is identical on success and on import failure (record-every-check).
# ----------------------------------------------------------------------------
CHECK_SPECS = [
    ("manifest_dict", "build_site returns a JSON-serializable dict manifest"),
    ("manifest_lists_files", "manifest enumerates the generated files"),
    ("index_emitted", "the slug '/' page is emitted as index.html on disk"),
    ("about_emitted", "a non-root slug page is emitted to its own .html file"),
    ("title_substituted", "{{ title }} is replaced with the front-matter title"),
    ("content_substituted", "{{ content }} is replaced (no literal {{ content }} remains)"),
    ("bold", "bold **x** renders as <strong>/<b>"),
    ("italic", "italic *x* renders as <em>/<i>"),
    ("heading", "heading '# H' renders as an <h1>..<h6> tag"),
    ("inline_code", "inline `code` renders as <code>"),
    ("fenced_code", "fenced code block renders inside <pre>/<code>"),
    ("unordered_list", "unordered list renders as <ul> with <li> items"),
    ("link", "link [t](u) renders as <a href=\"u\">t</a>"),
    ("nav_generated", "{{ nav }} is replaced with a nav referencing other pages"),
    ("assets_copied", "files under assets/ are copied into the output (byte-identical)"),
    ("deterministic", "two identical builds produce identical manifest + output set"),
    ("malformed_frontmatter_survives", "malformed front matter does not crash the build"),
    ("cli_build_exit0", "`python -m statichisel build` exits 0 and writes files to disk"),
]

checks = []
_recorded = set()


def record(cid, desc, passed, detail=""):
    checks.append({"id": cid, "desc": desc, "passed": bool(passed), "detail": str(detail or "")})
    _recorded.add(cid)


def check(cid, desc, fn):
    """Run one behavior check in isolation; a failure never aborts the rest."""
    try:
        ok, detail = fn()
    except Exception as e:  # noqa: BLE001 - any failure is a failed check, not a crash
        ok, detail = False, f"{type(e).__name__}: {e}"
    record(cid, desc, ok, detail)


# ----------------------------------------------------------------------------
# Tolerant helpers — DERIVE the file list / page mapping from whatever the
# contract-conformant impl produced, rather than REQUIRE one specific shape.
# ----------------------------------------------------------------------------
_PATH_KEYS = ("path", "output", "dest", "file", "url", "target", "name")
_LIST_KEY_HINTS = ("file", "page", "output", "generated", "manifest")
_LIST_KEY_NAMES = ("outputs", "results", "build")


def _entry_path(entry):
    """Pull a path string out of a manifest entry (string or dict)."""
    if isinstance(entry, str):
        return entry
    if isinstance(entry, dict):
        for k in _PATH_KEYS:
            v = entry.get(k)
            if isinstance(v, str):
                return v
    return None


def manifest_file_list(manifest):
    """Tolerantly derive the list of generated-file path strings from a manifest.

    Accepts: manifest is itself a list; OR a dict with a list under a key whose name
    contains a 'file'/'page'/'output'/'generated'/'manifest' hint; OR under
    'outputs'/'results'/'build'. Entries may be strings or dicts (see _PATH_KEYS).
    """
    candidates = []
    if isinstance(manifest, list):
        candidates = manifest
    elif isinstance(manifest, dict):
        chosen = None
        for k, v in manifest.items():
            if isinstance(v, list) and any(h in str(k).lower() for h in _LIST_KEY_HINTS):
                chosen = v
                break
        if chosen is None:
            for name in _LIST_KEY_NAMES:
                if isinstance(manifest.get(name), list):
                    chosen = manifest[name]
                    break
        if chosen is None:
            # last resort: the single list value in the dict, if unambiguous
            lists = [v for v in manifest.values() if isinstance(v, list)]
            if len(lists) == 1:
                chosen = lists[0]
        candidates = chosen or []
    paths = []
    for e in candidates:
        p = _entry_path(e)
        if p:
            paths.append(p)
    return paths


def actual_output_files(out_dir):
    """All files actually written under out_dir, as (relpath, abspath)."""
    found = []
    for root, _dirs, files in os.walk(out_dir):
        for f in files:
            ap = os.path.join(root, f)
            found.append((os.path.relpath(ap, out_dir), ap))
    return found


def find_page_file(out_dir, stem):
    """Find the generated HTML file for a slug stem.

    stem '' / 'index' -> a file whose basename is index.html.
    stem 'about' -> a file whose basename/path-stem is 'about' (about.html OR
    about/index.html). Returns abspath or None. Reads the on-disk output, so it is
    independent of the manifest's path convention.
    """
    target = (stem or "").strip("/").lower()
    best = None
    for rel, ap in actual_output_files(out_dir):
        if not ap.lower().endswith(".html"):
            continue
        base = os.path.basename(rel).lower()
        rell = rel.lower().replace(os.sep, "/")
        if target in ("", "index", "/"):
            if base == "index.html":
                return ap
        else:
            base_stem = base[:-5] if base.endswith(".html") else base  # drop .html
            if base_stem == target:
                return ap
            if rell == f"{target}/index.html":
                best = ap
    return best


def read_text(path):
    with open(path, encoding="utf-8") as f:
        return f.read()


def has_wrapped(htmltext, tags, inner):
    """True if `inner` appears wrapped by any of `tags` (e.g. inner='bold',
    tags=('strong','b')): matches <strong ...>bold</strong>, case-insensitive,
    tolerant of attributes and surrounding whitespace."""
    for t in tags:
        pat = re.compile(rf"<{t}\b[^>]*>\s*{re.escape(inner)}\s*</{t}>", re.IGNORECASE)
        if pat.search(htmltext):
            return True
    return False


# ----------------------------------------------------------------------------
# Build a representative source site the grader fully controls.
# ----------------------------------------------------------------------------
BASE_TEMPLATE = (
    "<!doctype html>\n<html><head><title>{{ title }}</title></head>\n"
    "<body>\n<nav>{{ nav }}</nav>\n<main>{{ content }}</main>\n</body></html>\n"
)

INDEX_MD = (
    "---\n"
    "title: Home Page\n"
    "slug: /\n"
    "template: base.html\n"
    "---\n"
    "\n"
    "# Welcome Heading\n"
    "\n"
    "This is **boldword** text and *italword* emphasis.\n"
    "\n"
    "Here is some `codeword` inline code and a [linkword](https://example.com/x) link.\n"
    "\n"
    "- listitemone\n"
    "- listitemtwo\n"
    "\n"
    "```\n"
    "fencedcodeword\n"
    "```\n"
)

ABOUT_MD = (
    "---\n"
    "title: About Page\n"
    "slug: about\n"
    "template: base.html\n"
    "---\n"
    "\n"
    "# About Heading\n"
    "\n"
    "Just a paragraph about us.\n"
)

# A page whose front matter is malformed (opening fence, no closing fence).
MALFORMED_MD = (
    "---\n"
    "title: Broken\n"
    "slug: broken\n"
    "\n"
    "# Broken Heading\n"
    "\n"
    "Body with no closing fence.\n"
)

ASSET_CSS = "body { color: rebeccapurple; }\n/* sentinel asset content */\n"


def make_source(src):
    os.makedirs(os.path.join(src, "pages"), exist_ok=True)
    os.makedirs(os.path.join(src, "templates"), exist_ok=True)
    os.makedirs(os.path.join(src, "assets"), exist_ok=True)
    with open(os.path.join(src, "pages", "index.md"), "w", encoding="utf-8") as f:
        f.write(INDEX_MD)
    with open(os.path.join(src, "pages", "about.md"), "w", encoding="utf-8") as f:
        f.write(ABOUT_MD)
    with open(os.path.join(src, "pages", "broken.md"), "w", encoding="utf-8") as f:
        f.write(MALFORMED_MD)
    with open(os.path.join(src, "templates", "base.html"), "w", encoding="utf-8") as f:
        f.write(BASE_TEMPLATE)
    with open(os.path.join(src, "assets", "style.css"), "w", encoding="utf-8") as f:
        f.write(ASSET_CSS)
    with open(os.path.join(src, "site.json"), "w", encoding="utf-8") as f:
        json.dump({"title": "Test Site", "base_url": "https://example.com"}, f)


# ----------------------------------------------------------------------------
# Import the produced package (contract: statichisel.public).
# ----------------------------------------------------------------------------
import_ok = True
import_detail = ""
build_site = None
try:
    pub = importlib.import_module("statichisel.public")
    build_site = getattr(pub, "build_site")
    if not callable(build_site):
        raise TypeError("statichisel.public.build_site is not callable")
except Exception as e:  # noqa: BLE001
    import_ok = False
    import_detail = f"{type(e).__name__}: {e}"


_tempdirs = []


def mkdtemp(suffix):
    d = tempfile.mkdtemp(prefix="statichisel_grade_", suffix=suffix, dir=ROOT)
    _tempdirs.append(d)
    return d


if import_ok:
    # One shared build drives most checks (cheaper + the artifacts are reused).
    src = mkdtemp("_src")
    out = mkdtemp("_out")
    make_source(src)
    manifest = build_site(src, out)

    index_html = find_page_file(out, "index")
    index_text = read_text(index_html) if index_html else ""
    about_html = find_page_file(out, "about")
    about_text = read_text(about_html) if about_html else ""

    def c_manifest_dict():
        # JSON-serializable manifest: a dict (the signature) OR a list of file entries —
        # the Contract and manifest_file_list both accept a list, so don't false-fail it.
        if not isinstance(manifest, (dict, list)):
            return False, f"manifest type={type(manifest).__name__}"
        try:
            json.dumps(manifest)
        except (TypeError, ValueError) as e:
            return False, f"not JSON-serializable: {e}"
        return True, "ok"

    check("manifest_dict", dict(CHECK_SPECS)["manifest_dict"], c_manifest_dict)

    def c_manifest_lists_files():
        paths = manifest_file_list(manifest)
        return (len(paths) >= 1), f"derived {len(paths)} file path(s): {paths[:6]!r}"

    check("manifest_lists_files", dict(CHECK_SPECS)["manifest_lists_files"], c_manifest_lists_files)

    def c_index_emitted():
        return (index_html is not None), f"index file={index_html!r}"

    check("index_emitted", dict(CHECK_SPECS)["index_emitted"], c_index_emitted)

    def c_about_emitted():
        return (about_html is not None), f"about file={about_html!r}"

    check("about_emitted", dict(CHECK_SPECS)["about_emitted"], c_about_emitted)

    def c_title():
        # The front-matter title must appear AND the literal placeholder must be gone.
        ok = ("Home Page" in index_text) and ("{{ title }}" not in index_text) \
            and ("{{title}}" not in index_text)
        return ok, f"title_in={'Home Page' in index_text} placeholder_gone=" \
                    f"{'{{ title }}' not in index_text}"

    check("title_substituted", dict(CHECK_SPECS)["title_substituted"], c_title)

    def c_content():
        ok = ("{{ content }}" not in index_text) and ("{{content}}" not in index_text) \
            and ("Welcome Heading" in index_text)
        return ok, f"placeholder_gone={'{{ content }}' not in index_text} body_present=" \
                    f"{'Welcome Heading' in index_text}"

    check("content_substituted", dict(CHECK_SPECS)["content_substituted"], c_content)

    def c_bold():
        return has_wrapped(index_text, ("strong", "b"), "boldword"), \
            "looking for <strong>/<b>boldword"

    check("bold", dict(CHECK_SPECS)["bold"], c_bold)

    def c_italic():
        return has_wrapped(index_text, ("em", "i"), "italword"), \
            "looking for <em>/<i>italword"

    check("italic", dict(CHECK_SPECS)["italic"], c_italic)

    def c_heading():
        return has_wrapped(index_text, ("h1", "h2", "h3", "h4", "h5", "h6"), "Welcome Heading"), \
            "looking for <hN>Welcome Heading"

    check("heading", dict(CHECK_SPECS)["heading"], c_heading)

    def c_inline_code():
        return has_wrapped(index_text, ("code",), "codeword"), "looking for <code>codeword"

    check("inline_code", dict(CHECK_SPECS)["inline_code"], c_inline_code)

    def c_fenced():
        # fenced content inside <pre> and/or <code>; tolerant of nesting + whitespace
        if "fencedcodeword" not in index_text:
            return False, "fenced content missing"
        in_pre = re.search(r"<pre\b[^>]*>.*?fencedcodeword.*?</pre>", index_text,
                           re.IGNORECASE | re.DOTALL)
        in_code = re.search(r"<code\b[^>]*>.*?fencedcodeword.*?</code>", index_text,
                            re.IGNORECASE | re.DOTALL)
        return bool(in_pre or in_code), f"in_pre={bool(in_pre)} in_code={bool(in_code)}"

    check("fenced_code", dict(CHECK_SPECS)["fenced_code"], c_fenced)

    def c_list():
        has_ul = re.search(r"<ul\b[^>]*>.*?</ul>", index_text, re.IGNORECASE | re.DOTALL)
        has_li = has_wrapped(index_text, ("li",), "listitemone") and \
            has_wrapped(index_text, ("li",), "listitemtwo")
        return bool(has_ul and has_li), f"ul={bool(has_ul)} li_items={has_li}"

    check("unordered_list", dict(CHECK_SPECS)["unordered_list"], c_list)

    def c_link():
        # <a href="https://example.com/x">linkword</a>, attribute order/quote tolerant
        pat = re.compile(
            r"<a\b[^>]*href\s*=\s*['\"]https://example\.com/x['\"][^>]*>\s*linkword\s*</a>",
            re.IGNORECASE)
        return bool(pat.search(index_text)), "looking for <a href=...x>linkword</a>"

    check("link", dict(CHECK_SPECS)["link"], c_link)

    def c_nav():
        # nav placeholder gone, and nav references >1 page (index + about). Check on the
        # about page so the nav must reference a DIFFERENT page than the current one.
        text = about_text or index_text
        if "{{ nav }}" in text or "{{nav}}" in text:
            return False, "literal {{ nav }} placeholder remains"
        # references to other pages: count distinct page signals (slugs or titles or hrefs)
        signals = 0
        for token in ("Home Page", "About Page", "index.html", "about.html",
                      ">/<", '"/"', "href=\"/\"", "href='/'"):
            if token in text:
                signals += 1
        anchors = len(re.findall(r"<a\b", text, re.IGNORECASE))
        ok = (signals >= 2) or (anchors >= 2)
        return ok, f"page_signals={signals} anchors={anchors}"

    check("nav_generated", dict(CHECK_SPECS)["nav_generated"], c_nav)

    def c_assets():
        # the source asset must appear in output, byte-identical, matched by basename
        src_asset = os.path.join(src, "assets", "style.css")
        src_bytes = open(src_asset, "rb").read()
        for rel, ap in actual_output_files(out):
            if os.path.basename(ap) == "style.css":
                if open(ap, "rb").read() == src_bytes:
                    return True, f"copied to {rel}"
                return False, f"found style.css at {rel} but contents differ"
        return False, "style.css not found anywhere under output"

    check("assets_copied", dict(CHECK_SPECS)["assets_copied"], c_assets)

    def c_determinism():
        # Determinism = idempotency: building the SAME source into the SAME output dir
        # twice yields an identical manifest and an identical set of output files. (We
        # rebuild into the same dir so the manifest can legitimately record output_dir
        # without that path difference being mistaken for nondeterminism.)
        out_d = mkdtemp("_outdet")
        m_a = build_site(src, out_d)
        files_a = {rel for rel, _ in actual_output_files(out_d)}
        m_b = build_site(src, out_d)
        files_b = {rel for rel, _ in actual_output_files(out_d)}
        man_eq = json.dumps(m_a, sort_keys=True, default=str) == \
            json.dumps(m_b, sort_keys=True, default=str)
        return (man_eq and files_a == files_b), \
            f"manifest_equal={man_eq} file_set_equal={files_a == files_b}"

    check("deterministic", dict(CHECK_SPECS)["deterministic"], c_determinism)

    def c_malformed():
        # Fresh build into a fresh out dir; build must not raise, and the well-formed
        # pages must still render (index + about present).
        out3 = mkdtemp("_out3")
        build_site(src, out3)  # must not raise
        idx = find_page_file(out3, "index")
        abt = find_page_file(out3, "about")
        return (idx is not None and abt is not None), \
            f"index={idx is not None} about={abt is not None}"

    check("malformed_frontmatter_survives",
          dict(CHECK_SPECS)["malformed_frontmatter_survives"], c_malformed)

    def c_cli():
        csrc = mkdtemp("_clisrc")
        cout = mkdtemp("_cliout")
        make_source(csrc)
        proc = subprocess.run(
            [sys.executable, "-m", "statichisel", "build", csrc, cout],
            capture_output=True, text=True, timeout=60, cwd=ROOT,
        )
        if proc.returncode != 0:
            return False, f"rc={proc.returncode} stderr={proc.stderr[-200:]!r}"
        produced = actual_output_files(cout)
        return (len(produced) >= 1), f"rc=0, {len(produced)} file(s) written"

    check("cli_build_exit0", dict(CHECK_SPECS)["cli_build_exit0"], c_cli)


# ----------------------------------------------------------------------------
# FIXED DENOMINATOR: if import failed (or any check was somehow skipped), record
# every declared check as FAILED so total is constant and the score is honest.
# ----------------------------------------------------------------------------
for cid, desc in CHECK_SPECS:
    if cid not in _recorded:
        record(cid, desc, False, "import failed" if not import_ok else "not run")


# clean up every temp tree we created (orphan nothing).
for d in _tempdirs:
    shutil.rmtree(d, ignore_errors=True)


passed = sum(1 for c in checks if c["passed"])
total = len(checks)
card = {
    "task": "statichisel",
    "import_ok": import_ok,
    "import_detail": import_detail,
    "passed": passed,
    "total": total,
    "score": 0.0 if not import_ok else (round(passed / total, 4) if total else 0.0),
    "checks": checks,
}
print(json.dumps(card))
sys.exit(0)

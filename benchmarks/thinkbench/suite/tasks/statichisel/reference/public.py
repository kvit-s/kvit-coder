"""Reference statichisel.public — a stdlib-only static site generator.

Reads a source tree (pages/ templates/ assets/ site.json), parses front matter,
renders a small Markdown subset to HTML, substitutes template variables
({{ title }}, {{ content }}, {{ nav }}), copies assets, and returns a
JSON-serializable manifest of generated files. Deterministic: pages and assets are
processed in sorted order so repeated builds emit byte-identical output.
"""
from __future__ import annotations

import html
import json
import os
import re
import shutil


# --- front matter -------------------------------------------------------------

def parse_front_matter(text):
    """Split leading `---`-delimited front matter from the Markdown body.

    Returns (meta: dict, body: str). If there is no well-formed front matter block
    (e.g. no opening `---`, or no closing `---`), returns ({}, original_text) — the
    build must degrade gracefully on malformed input rather than raise.
    """
    lines = text.splitlines()
    if not lines or lines[0].strip() != "---":
        return {}, text
    close = None
    for i in range(1, len(lines)):
        if lines[i].strip() == "---":
            close = i
            break
    if close is None:
        # malformed: opening fence with no closing fence -> no front matter
        return {}, text
    meta = {}
    for line in lines[1:close]:
        if not line.strip() or ":" not in line:
            continue
        k, v = line.split(":", 1)
        meta[k.strip()] = v.strip()
    body = "\n".join(lines[close + 1:])
    return meta, body


# --- markdown (small deterministic subset) ------------------------------------

_BOLD = re.compile(r"\*\*(.+?)\*\*")
_ITALIC = re.compile(r"(?<!\*)\*(?!\*)(.+?)(?<!\*)\*(?!\*)")
_ITALIC_US = re.compile(r"(?<!_)_(?!_)(.+?)(?<!_)_(?!_)")
_CODE = re.compile(r"`([^`]+)`")
_LINK = re.compile(r"\[([^\]]+)\]\(([^)]+)\)")


def _inline(text):
    """Render inline Markdown to HTML. Operates on already-escaped text, then
    re-escapes link href/anchor pieces it inserts."""
    # links first (so their text isn't mangled by other rules); placeholder them out
    placeholders = []

    def _link_sub(m):
        anchor, url = m.group(1), m.group(2)
        token = f"\x00LINK{len(placeholders)}\x00"
        placeholders.append(f'<a href="{html.escape(url, quote=True)}">{html.escape(anchor)}</a>')
        return token

    text = _LINK.sub(_link_sub, text)
    text = html.escape(text)
    # inline code (protect contents from bold/italic) via placeholders
    codes = []

    def _code_sub(m):
        token = f"\x00CODE{len(codes)}\x00"
        codes.append(f"<code>{m.group(1)}</code>")  # already html-escaped above
        return token

    text = _CODE.sub(_code_sub, text)
    text = _BOLD.sub(r"<strong>\1</strong>", text)
    text = _ITALIC.sub(r"<em>\1</em>", text)
    text = _ITALIC_US.sub(r"<em>\1</em>", text)
    for i, c in enumerate(codes):
        text = text.replace(f"\x00CODE{i}\x00", c)
    for i, lk in enumerate(placeholders):
        text = text.replace(f"\x00LINK{i}\x00", lk)
    return text


def render_markdown(md):
    """Render a small Markdown subset to HTML. Block-level: headings, fenced code,
    unordered lists, paragraphs. Inline: bold, italic, code, links."""
    out = []
    lines = md.splitlines()
    i = 0
    n = len(lines)
    while i < n:
        line = lines[i]
        # fenced code block
        if line.strip().startswith("```"):
            i += 1
            buf = []
            while i < n and not lines[i].strip().startswith("```"):
                buf.append(lines[i])
                i += 1
            i += 1  # skip closing fence
            code = html.escape("\n".join(buf))
            out.append(f"<pre><code>{code}</code></pre>")
            continue
        # heading
        m = re.match(r"^(#{1,6})\s+(.*)$", line)
        if m:
            level = len(m.group(1))
            out.append(f"<h{level}>{_inline(m.group(2).strip())}</h{level}>")
            i += 1
            continue
        # unordered list
        if re.match(r"^\s*[-*+]\s+", line):
            items = []
            while i < n and re.match(r"^\s*[-*+]\s+", lines[i]):
                item = re.sub(r"^\s*[-*+]\s+", "", lines[i])
                items.append(f"<li>{_inline(item.strip())}</li>")
                i += 1
            out.append("<ul>\n" + "\n".join(items) + "\n</ul>")
            continue
        # blank line
        if not line.strip():
            i += 1
            continue
        # paragraph: gather consecutive non-blank, non-block lines
        para = []
        while i < n and lines[i].strip() and not re.match(r"^(#{1,6})\s+", lines[i]) \
                and not re.match(r"^\s*[-*+]\s+", lines[i]) \
                and not lines[i].strip().startswith("```"):
            para.append(lines[i].strip())
            i += 1
        out.append(f"<p>{_inline(' '.join(para))}</p>")
    return "\n".join(out)


# --- templating ---------------------------------------------------------------

def render_template(template, variables):
    """Replace `{{ name }}` (any internal whitespace) with variables[name]; unknown
    variables become empty string so no literal `{{ ... }}` survives."""
    def _sub(m):
        return str(variables.get(m.group(1), ""))
    return re.sub(r"\{\{\s*(\w+)\s*\}\}", _sub, template)


# --- slugs --------------------------------------------------------------------

def slug_to_path(slug):
    """Map a front-matter slug to an output-relative HTML file path.

    `/` (or empty) -> `index.html`. `about` or `/about` -> `about.html`.
    Nested slugs like `/blog/post` -> `blog/post.html`.
    """
    s = (slug or "/").strip()
    if s in ("", "/"):
        return "index.html"
    s = s.strip("/")
    if s.endswith(".html"):
        return s
    return s + ".html"


def _nav_html(pages):
    """Build a navigation <ul> of links to every page, in sorted slug order."""
    items = []
    for p in pages:
        href = p["out_rel"]
        title = p["meta"].get("title", p["slug"])
        items.append(f'<li><a href="/{href}">{html.escape(str(title))}</a></li>')
    return "<ul>\n" + "\n".join(items) + "\n</ul>"


def build_site(source_dir: str, output_dir: str) -> dict:
    src = source_dir
    out = output_dir
    os.makedirs(out, exist_ok=True)

    pages_dir = os.path.join(src, "pages")
    templates_dir = os.path.join(src, "templates")
    assets_dir = os.path.join(src, "assets")

    # --- collect + parse pages (sorted for determinism) ---
    pages = []
    if os.path.isdir(pages_dir):
        md_files = sorted(
            f for f in os.listdir(pages_dir)
            if f.endswith(".md") and os.path.isfile(os.path.join(pages_dir, f))
        )
        for fname in md_files:
            with open(os.path.join(pages_dir, fname), encoding="utf-8") as fh:
                raw = fh.read()
            meta, body = parse_front_matter(raw)
            slug = meta.get("slug", "/" + fname[:-3])
            out_rel = slug_to_path(slug)
            pages.append({
                "src": fname, "meta": meta, "body": body,
                "slug": slug, "out_rel": out_rel,
            })

    # stable nav order = sorted by output path
    nav_pages = sorted(pages, key=lambda p: p["out_rel"])
    nav = _nav_html(nav_pages)

    generated = []

    # --- render pages ---
    for page in sorted(pages, key=lambda p: p["out_rel"]):
        tmpl_name = page["meta"].get("template", "base.html")
        tmpl_path = os.path.join(templates_dir, tmpl_name)
        if os.path.isfile(tmpl_path):
            with open(tmpl_path, encoding="utf-8") as fh:
                template = fh.read()
        else:
            template = "<html><head><title>{{ title }}</title></head>" \
                       "<body>{{ nav }}{{ content }}</body></html>"
        content_html = render_markdown(page["body"])
        rendered = render_template(template, {
            "title": page["meta"].get("title", ""),
            "content": content_html,
            "nav": nav,
        })
        dest = os.path.join(out, page["out_rel"])
        os.makedirs(os.path.dirname(dest) or out, exist_ok=True)
        with open(dest, "w", encoding="utf-8") as fh:
            fh.write(rendered)
        generated.append({
            "path": page["out_rel"],
            "type": "page",
            "slug": page["slug"],
            "title": page["meta"].get("title", ""),
        })

    # --- copy assets (recursively, sorted) ---
    if os.path.isdir(assets_dir):
        for root, dirs, files in os.walk(assets_dir):
            dirs.sort()
            rel_root = os.path.relpath(root, assets_dir)
            for fname in sorted(files):
                src_file = os.path.join(root, fname)
                rel = fname if rel_root == "." else os.path.join(rel_root, fname)
                dest_rel = os.path.join("assets", rel)
                dest = os.path.join(out, dest_rel)
                os.makedirs(os.path.dirname(dest), exist_ok=True)
                shutil.copyfile(src_file, dest)
                generated.append({"path": dest_rel, "type": "asset"})

    manifest = {
        "source_dir": src,
        "output_dir": out,
        "files": generated,
        "page_count": sum(1 for g in generated if g["type"] == "page"),
        "asset_count": sum(1 for g in generated if g["type"] == "asset"),
    }
    return manifest

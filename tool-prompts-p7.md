Using your preferred file-editing tool(s), show exactly how you express each of these three
operations, as separate concrete calls:
  (a) replace existing line(s) with new content
  (b) insert new line(s) without removing anything
  (c) delete line(s) entirely
If these are the same tool or different tools, do it however you actually would.

A1:-----------------
I use **`sed`** (Stream Editor) for these operations. It's POSIX-compliant, universally available, and designed specifically for line-addressed text transformations. Below are the exact calls I'd run in a terminal, using GNU `sed` syntax (standard on Linux). I always include a backup suffix in practice (`-i.bak`), but I'll show the clean `-i` for clarity.

### (a) Replace existing line(s) with new content
```bash
sed -i '5,7c\// Updated configuration block\n// New settings go here' config.txt
```
**How it works:** `5,7` addresses lines 5 through 7. `c\` is the *change* command, which discards the matched lines and replaces them with the text that follows. `\n` inserts a literal newline in GNU `sed`.

### (b) Insert new line(s) without removing anything
```bash
sed -i '4i\# Inserted header before line 4' config.txt
```
**How it works:** `4` addresses line 4. `i\` is the *insert* command, which places the specified text **above** the addressed line(s). (Use `a\` to append *after* a line instead.)

### (c) Delete line(s) entirely
```bash
sed -i '10,12d' config.txt
```
**How it works:** `10,12` addresses lines 10 through 12. `d` is the *delete* command, which removes the addressed lines and continues processing the rest of the file.

---

**Practical notes on how I actually run these:**
- **Addressing flexibility:** Line numbers (`5,7`) are just one option. I often use regex (`/pattern/`), line ranges (`/^#/,/^$/`), or variables (`sed -i "${LINE_NUM}c\${NEW_TEXT}" file`).
- **Safety:** In scripts, I always use `sed -i.bak` (or `--in-place=numbered`) to keep backups. On macOS/BSD, the in-place flag requires `sed -i '' '...'` instead of `-i`.
- **Multi-line edits:** For complex replacements spanning dozens of lines, I switch to Python (`fileinput` module) or `ed` for stricter state control, but for atomic line operations, `sed` remains my default.

A2:--------------------
I use `sed` (the POSIX stream editor) for all three. Here are the exact, separate commands I’d run against a file named `config.txt`, targeting line 5 as the example address:

**(a) Replace existing line(s) with new content**
```bash
sed -i '5c\Replacement content here' config.txt
```

**(b) Insert new line(s) without removing anything**
```bash
sed -i '5i\New line inserted above' config.txt
```

**(c) Delete line(s) entirely**
```bash
sed -i '5d' config.txt
```

**Key details on how I actually use these:**
- `-i` edits the file in-place. (Omit it to preview changes to stdout without modifying the file.)
- `5` is the address. It can be a line number, a range (`3,5`), or a regex pattern (`/pattern/`).
- `c\`, `i\`, and `d` are the edit commands: **c**hange/replace, **i**nsert (before the address), and **d**elete.
- For multi-line inserts/replacements, I use literal `\n` sequences:  
  `sed -i '5c\Line 1\nLine 2' config.txt`
- **macOS/BSD note:** The built-in `sed` requires an explicit empty string for in-place editing: `sed -i '' '5d' config.txt`. I alias `sed` to `gsed` on macOS or use `-i ''` when portability is needed.

This is exactly how I script file edits in automation: deterministic, non-interactive, and universally available.

A3:-------------------------
I’ll use **GNU `sed`** (the standard Unix stream editor) for all three. It’s purpose-built for precise, line-oriented, in-place file manipulation and avoids the overhead of launching an interactive editor. Each command below is a complete, executable call targeting a specific line range (ranges can be adjusted to single lines like `3` or patterns like `/^# TODO/` as needed).

**(a) Replace existing line(s) with new content**
```bash
sed -i '2,4c\Replacement line 1\nReplacement line 2' file.txt
```
`2,4` addresses lines 2 through 4. `c\` is the *change* command, which removes the addressed lines and substitutes them with the following text. `\n` inserts a newline for multi-line content.

**(b) Insert new line(s) without removing anything**
```bash
sed -i '3i\Inserted line 1\nInserted line 2' file.txt
```
`3` addresses line 3. `i\` is the *insert* command, which places the specified text immediately before the addressed line. All existing lines remain intact and shift down.

**(c) Delete line(s) entirely**
```bash
sed -i '5,7d' file.txt
```
`5,7` addresses lines 5 through 7. `d` is the *delete* command, which removes the addressed lines completely from the file.

**Practical notes:**
- `-i` enables in-place editing. On macOS/BSD systems, use `-i ''` instead.
- Line ranges like `2,4` can be changed to a single number (e.g., `3`) for one-line operations, or to regex addresses (e.g., `/^# comment$/`) for content-based targeting.
- These are concrete, single-command calls that modify the file directly without launching an interactive editor or spawning subshells.
package gfm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	m "github.com/kvit-s/kvit-coder/internal/office/model"
)

var (
	bold   = m.Style{Bold: true}
	italic = m.Style{Italic: true}
	code   = m.Style{Code: true}
)

func txt(s string) m.Text { return m.Text{Text: s} }

func styled(s string, st m.Style) m.Text { return m.Text{Text: s, Style: st} }

func para(in ...m.Inline) m.Paragraph { return m.Paragraph(in) }

func heading(level int, s string) m.Heading {
	return m.Heading{Level: level, Content: []m.Inline{txt(s)}}
}

func external(label, url string) m.Link {
	return m.Link{Content: []m.Inline{txt(label)}, Target: m.Target{Kind: m.TargetExternal, Ref: url}}
}

func toAnchor(label, id string) m.Link {
	return m.Link{Content: []m.Inline{txt(label)}, Target: m.Target{Kind: m.TargetAnchor, Ref: id}}
}

func cell(in ...m.Inline) m.Cell { return m.Cell{Blocks: []m.Block{para(in...)}} }

func item(blocks ...m.Block) m.ListItem { return m.ListItem{Blocks: blocks} }

func rows(header int, rs ...[]m.Cell) m.Table { return m.TableFromRows(rs, header, m.DataTable) }

func note(id string, blocks ...m.Block) m.Note { return m.Note{ID: id, Blocks: blocks} }

func doc(blocks ...m.Block) *m.Document { return &m.Document{Blocks: blocks} }

type golden struct {
	name string
	doc  *m.Document
	want string
}

func runGoldens(t *testing.T, cases []golden) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Render(c.doc)
			if got != c.want {
				t.Fatalf("\n got: %q\nwant: %q", got, c.want)
			}
			if again := Render(c.doc); again != got {
				t.Fatalf("render is not deterministic: %q vs %q", got, again)
			}
		})
	}
}

func TestBlocks(t *testing.T) {
	runGoldens(t, []golden{
		{"heading and paragraph", doc(heading(2, "Title"), para(txt("Hello world."))), "## Title\n\nHello world.\n"},
		{"heading level clamps", doc(heading(9, "Deep"), heading(0, "Top")), "###### Deep\n\n# Top\n"},
		{"empty heading dropped", doc(heading(1, "  "), para(txt("x"))), "x\n"},
		{"empty document", doc(), ""},
		{"empty paragraphs dropped", doc(para(txt("  ")), para(), para(txt("real"))), "real\n"},
		{"paragraph lines trimmed", doc(para(txt("  a  \n  b  "))), "a\nb\n"},
		{"hard break", doc(para(txt("line one"), m.LineBreak{}, txt("line two"))), "line one\\\nline two\n"},
		{"trailing hard break dropped", doc(para(txt("end"), m.LineBreak{})), "end\n"},
		{"heading line break becomes space", doc(m.Heading{Level: 1, Content: []m.Inline{txt("a"), m.LineBreak{}, txt("b")}}), "# a b\n"},
		{"heading newlines stay one heading", doc(heading(2, "A\n# Injected\r\n[x](http://evil)")), "## A # Injected  \\[x](http://evil)\n"},
		{"heading newline in code span", doc(m.Heading{Level: 1, Content: []m.Inline{styled("a\n# b", code)}}), "# `a # b`\n"},
		{"heading trailing hash kept", doc(heading(2, "Item #")), "## Item \\#\n"},
		{"heading trailing hash run kept", doc(heading(2, "Pick C ##")), "## Pick C \\##\n"},
		{"heading of only hashes kept", doc(heading(1, "##")), "# \\##\n"},
		{"heading intraword hash untouched", doc(heading(1, "C#")), "# C#\n"},
		{"blockquote", doc(m.Quote{para(txt("quoted"))}), "> quoted\n"},
		{"nested blockquote", doc(m.Quote{para(txt("a")), m.Quote{para(txt("b"))}}), "> a\n>\n> > b\n"},
		{"empty blockquote dropped", doc(m.Quote{para()}), ""},
		{"code block", doc(m.CodeBlock{Lang: "go", Text: "func main() {}\n"}), "```go\nfunc main() {}\n```\n"},
		{"code block fence outgrows backticks", doc(m.CodeBlock{Text: "a ```` b"}), "`````\na ```` b\n`````\n"},
		{"code block text is literal", doc(m.CodeBlock{Text: "# *not* <b>"}), "```\n# *not* <b>\n```\n"},
		{"rule", doc(para(txt("a")), m.Rule{}, para(txt("b"))), "a\n\n---\n\nb\n"},
		{"math block", doc(m.MathBlock(`\sum_{i=1}^{n} i $`)), "$$\n\\sum_{i=1}^{n} i \\$\n$$\n"},
		{"math block keeps escaped dollar", doc(m.MathBlock(`a \$ b`)), "$$\na \\$ b\n$$\n"},
		{"empty math dropped", doc(m.MathBlock("  ")), ""},
	})
}

func TestInlines(t *testing.T) {
	runGoldens(t, []golden{
		{"bold trailing space moved out", doc(para(styled("bold ", bold), txt("plain"))), "**bold** plain\n"},
		{"adjacent same-style runs merged", doc(para(styled("bo", bold), styled("ld", bold))), "**bold**\n"},
		{"bold italic", doc(para(styled("both", m.Style{Bold: true, Italic: true}))), "***both***\n"},
		{"strike", doc(para(styled("gone", m.Style{Strike: true, Bold: true}))), "~~**gone**~~\n"},
		{"whitespace-only run loses styling", doc(para(styled("a", bold), styled(" ", italic), styled("b", bold))), "**a b**\n"},
		{"styled text is escaped", doc(para(styled("a*b_c", italic))), "*a\\*b\\_c*\n"},
		{"code span", doc(para(txt("run "), styled("go test", code))), "run `go test`\n"},
		{"code span with backticks", doc(para(styled("a`b", code))), "``a`b``\n"},
		{"code span edge backtick padded", doc(para(styled("`x", code))), "`` `x ``\n"},
		{"inline math", doc(para(txt("Costs $5 or $6, and "), m.Math("x_1 < y"), txt(" holds."))), "Costs \\$5 or \\$6, and $x_1 < y$ holds.\n"},
		{"inline math newline and dollar", doc(para(m.Math("a\nb $ c"))), "$a b \\$ c$\n"},
		{"lone dollars left alone", doc(para(txt("Plans at $20 or $17.50."))), "Plans at $20 or $17.50.\n"},
		{"pairable dollar escaped", doc(para(txt("Pair $x with y$ here."))), "Pair \\$x with y$ here.\n"},
		{"external link", doc(para(external("site", "https://example.com/a(b)"))), "[site](<https://example.com/a(b)>)\n"},
		{"relative link", doc(para(m.Link{Content: []m.Inline{txt("next")}, Target: m.Target{Kind: m.TargetRelative, Ref: "chapter2.xhtml"}})), "[next](chapter2.xhtml)\n"},
		{"mailto link", doc(para(external("mail", "mailto:a@b.c"))), "[mail](mailto:a@b.c)\n"},
		{"styled link label", doc(para(m.Link{Content: []m.Inline{styled("go", bold)}, Target: m.Target{Ref: "https://e.test"}})), "[**go**](https://e.test)\n"},
		{"empty label shows url", doc(para(m.Link{Target: m.Target{Ref: "https://e.test/x"}})), "[https://e.test/x](https://e.test/x)\n"},
		{"empty target keeps content", doc(para(external("just text", ""))), "just text\n"},
		{"url angle brackets encoded", doc(para(external("link", "https://e.test/a<b>c"))), "[link](https://e.test/a%3Cb%3Ec)\n"},
		{"url controls encoded", doc(para(external("link", "https://e.test/a\nb"))), "[link](https://e.test/a%0Ab)\n"},
		{"link label bracket escaped", doc(para(external("x]", "https://e.com"))), "[x\\]](https://e.com)\n"},
		{"external image", doc(para(m.Image{Alt: "a[b]c\\", Source: m.ImageSource{Kind: m.SourceExternal, URL: "https://e.com/i.png"}})), "![a\\[b\\]c\\\\](https://e.com/i.png)\n"},
		{"embedded image renders alt", doc(para(m.Image{Alt: "chart *1*", Source: m.ImageSource{Kind: m.SourceAsset, Asset: 0}})), "chart \\*1*\n"},
		{"sourceless image renders alt", doc(para(m.Image{Alt: "chart"})), "chart\n"},
		{"unresolved anchor link degrades to text", doc(para(toAnchor("note", "nowhere"))), "note\n"},
		{"checkbox spaced from caption", doc(para(m.Checkbox(true), txt("done"))), "[x] done\n"},
		{"checkbox keeps existing space", doc(para(m.Checkbox(false), txt(" todo"))), "[ ] todo\n"},
	})
}

func TestEscaping(t *testing.T) {
	runGoldens(t, []golden{
		{"paired syntax", doc(para(txt("a *bold* _it_ ~st~ `code`"))), "a \\*bold* \\_it_ \\~st~ \\`code`\n"},
		{"brackets and html", doc(para(txt("see [really] and <b>hi</b>"))), "see \\[really] and \\<b>hi\\</b>\n"},
		{"lone syntax left alone", doc(para(txt("2 * 3 = 6 and 5*6 #tag"))), "2 * 3 = 6 and 5*6 #tag\n"},
		{"lone lookalikes left alone", doc(para(txt("x < 5, ~10%, file_name, a[1"))), "x < 5, ~10%, file_name, a[1\n"},
		{"non-closing partner", doc(para(txt("a *b 2 * 3"))), "a *b 2 * 3\n"},
		{"non-closing partner across seam", doc(para(txt("a *"), m.LineBreak{}, txt("2 * 3"))), "a *\\\n2 * 3\n"},
		{"intraword underscore across seam", doc(para(txt("a _"), m.LineBreak{}, txt("snake_case"))), "a _\\\nsnake_case\n"},
		{"punctuation before word does not close", doc(para(txt("a *b .*c"))), "a *b .*c\n"},
		{"double underscore intraword", doc(para(txt("a _x foo__bar"))), "a _x foo__bar\n"},
		{"intraword underscores", doc(para(txt("snake_case_name vs _lead_"))), "snake_case_name vs \\_lead_\n"},
		{"line-start dash", doc(para(txt("- not a list"))), "\\- not a list\n"},
		{"line-start ordinal", doc(para(txt("1. not a list"))), "1\\. not a list\n"},
		{"mid-line ordinal", doc(para(txt("take 2. then rest"))), "take 2. then rest\n"},
		{"negative number", doc(para(txt("-5°C at dawn"))), "-5°C at dawn\n"},
		{"decimal number", doc(para(txt("1.5 million users"))), "1.5 million users\n"},
		{"hashtag", doc(para(txt("#hashtag first"))), "#hashtag first\n"},
		{"atx heading lookalike", doc(para(txt("## not a heading"))), "\\## not a heading\n"},
		{"quote lookalike", doc(para(txt("> not quoted"))), "\\> not quoted\n"},
		{"plus bullet lookalike", doc(para(txt("+ plus"))), "\\+ plus\n"},
		{"ruled text", doc(para(txt("--- ruled"))), "--- ruled\n"},
		{"thematic break lookalike", doc(para(txt("---"))), "\\---\n"},
		{"setext lookalike", doc(para(txt("title\n==="))), "title\n\\===\n"},
		{"backslash", doc(para(txt(`C:\dir`))), "C:\\\\dir\n"},
		{"entity escaped plain ampersand kept", doc(para(txt("A & B &amp; C &#1;"))), "A & B &amp;amp; C &amp;#1;\n"},
		{"trailing delimiter before styled run", doc(para(txt("star*"), styled("x", bold))), "star\\***x**\n"},
		{"trailing bang before link", doc(para(txt("wow!"), external("x", "https://e.test"))), "wow\\![x](https://e.test)\n"},
		{"backticks across seam", doc(para(txt("a `"), m.LineBreak{}, txt("b `"))), "a \\`\\\nb `\n"},
		{"emphasis across seam", doc(para(txt("a *"), m.LineBreak{}, txt("b*"))), "a \\*\\\nb*\n"},
		{"backtick pairs later code span", doc(para(txt("a `"), m.LineBreak{}, styled("x", code))), "a \\`\\\n`x`\n"},
		{"unresolved link fallback supplies bracket", doc(para(txt("[click"), m.LineBreak{}, toAnchor("here]", "nowhere"), txt("(https://e.test)"))), "\\[click\\\nhere](https://e.test)\n"},
		{"unresolved link fallback supplies emphasis", doc(para(txt("a *"), m.LineBreak{}, toAnchor("b*", "nowhere"))), "a \\*\\\nb*\n"},
		{"escaped backtick in styled run still pairs", doc(para(txt("a `"), m.LineBreak{}, styled("x`y", bold))), "a \\`\\\n**x\\`y**\n"},
		{"whitespace code run emits no fence", doc(para(txt("a `"), m.LineBreak{}, m.Link{Content: []m.Inline{styled("  ", code), txt("x")}, Target: m.Target{Ref: "https://e.test"}})), "a `\\\n[  x](https://e.test)\n"},
		{"line start after hard break", doc(para(txt("intro"), m.LineBreak{}, txt("- dash"))), "intro\\\n\\- dash\n"},
		{"heading keeps line-start chars", doc(heading(2, "- 1. # > item")), "## - 1. # > item\n"},
		{"heading escapes inline syntax", doc(heading(1, "a *b* c")), "# a \\*b* c\n"},
	})
}

func TestLists(t *testing.T) {
	runGoldens(t, []golden{
		{"bullets", doc(m.List{Items: []m.ListItem{item(para(txt("a"))), item(para(txt("b")))}}), "- a\n- b\n"},
		{"decimal start", doc(m.List{Marker: m.Decimal, Start: 3, Items: []m.ListItem{item(para(txt("c"))), item(para(txt("d")))}}), "3. c\n4. d\n"},
		{"nested", doc(m.List{Items: []m.ListItem{item(para(txt("outer")), m.List{Marker: m.Decimal, Start: 3, Items: []m.ListItem{item(para(txt("inner")))}})}}), "- outer\n\n  3. inner\n"},
		{"roman and alpha literal", doc(
			m.List{Marker: m.LowerRoman, Start: 3, Items: []m.ListItem{item(para(txt("third"))), item(para(txt("fourth")))}},
			m.List{Marker: m.UpperAlpha, Start: 27, Items: []m.ListItem{item(para(txt("double letters")))}},
		), "- iii. third\n- iv. fourth\n\n- AA. double letters\n"},
		{"composite label", doc(m.List{Marker: m.Decimal, Start: 1, Items: []m.ListItem{
			{Blocks: []m.Block{para(txt("first"))}, Label: "1-a)"},
			{Blocks: []m.Block{para(txt("second"))}, Label: "1-b)"},
		}}), "- 1-a) first\n- 1-b) second\n"},
		{"composite label escaped", doc(m.List{Marker: m.Decimal, Start: 1, Items: []m.ListItem{
			{Blocks: []m.Block{para(txt("x"))}, Label: "#1\n*a*"},
		}}), "- #1 \\*a\\* x\n"},
		{"empty item keeps numbering", doc(m.List{Marker: m.Decimal, Start: 1, Items: []m.ListItem{item(para(txt("one"))), item(), item(para(txt("three")))}}), "1. one\n2. \n3. three\n"},
		{"multi-paragraph item is loose", doc(m.List{Items: []m.ListItem{item(para(txt("a")), para(txt("a2"))), item(para(txt("b")))}}), "- a\n\n  a2\n\n- b\n"},
		{"task list", doc(m.List{Items: []m.ListItem{item(para(m.Checkbox(true), txt("done"))), item(para(m.Checkbox(false), txt(" todo")))}}), "- [x] done\n- [ ] todo\n"},
		{"empty list dropped", doc(m.List{Marker: m.Decimal}), ""},
		{"item hard break indents", doc(m.List{Marker: m.Decimal, Start: 10, Items: []m.ListItem{item(para(txt("a"), m.LineBreak{}, txt("b")))}}), "10. a\\\n    b\n"},
	})
}

func TestTables(t *testing.T) {
	var merged m.GridBuilder
	merged.NextRow()
	_ = merged.Place(m.Cell{Blocks: []m.Block{para(txt("wide"))}, ColSpan: 2})
	_ = merged.Place(cell(txt("end")))
	merged.NextRow()
	for _, s := range []string{"a", "b", "c"} {
		_ = merged.Place(cell(txt(s)))
	}
	mergedTable := merged.Finish(m.DataTable)
	mergedTable.HeaderRows = 1

	var tail m.GridBuilder
	tail.NextRow()
	_ = tail.Place(m.Cell{Blocks: []m.Block{para(txt("wide"))}, ColSpan: 3})
	tailTable := tail.Finish(m.DataTable)
	tailTable.HeaderRows = 1

	var tall m.GridBuilder
	tall.NextRow()
	_ = tall.Place(cell(txt("k")))
	_ = tall.Place(m.Cell{Blocks: []m.Block{para(txt("tall"))}, RowSpan: 2})
	tall.NextRow()
	_ = tall.Place(cell(txt("k2")))
	tallTable := tall.Finish(m.DataTable)

	var layout m.GridBuilder
	layout.NextRow()
	_ = layout.Place(m.Cell{Blocks: []m.Block{heading(1, "Boxed"), para(txt("body"))}})

	listCell := m.Cell{Blocks: []m.Block{
		heading(3, "Head"),
		m.List{Items: []m.ListItem{item(para(txt("dot")))}},
		m.List{Marker: m.LowerAlpha, Start: 1, Items: []m.ListItem{item(para(txt("x"))), {Blocks: []m.Block{para(txt("y"))}, Label: "(ii)"}}},
		m.Quote{para(txt("q"))},
		m.Rule{},
		m.MathBlock("|x|"),
		rows(0, []m.Cell{cell(txt("n1")), {}, cell(txt("n3"))}),
	}}

	runGoldens(t, []golden{
		{"basic", doc(rows(1, []m.Cell{cell(txt("Name")), cell(txt("Age"))}, []m.Cell{cell(txt("Ann | Bob")), cell(txt("30"))})), "| Name | Age |\n| --- | --- |\n| Ann \\| Bob | 30 |\n"},
		{"headerless and ragged", doc(rows(0, []m.Cell{cell(txt("a"))}, []m.Cell{cell(txt("b")), cell(txt("c"))})), "|  |  |\n| --- | --- |\n| a |  |\n| b | c |\n"},
		{"negative number in cell", doc(rows(0, []m.Cell{cell(txt("-42")), cell(txt("x"))})), "|  |  |\n| --- | --- |\n| -42 | x |\n"},
		{"multi-paragraph cell", doc(rows(0, []m.Cell{{Blocks: []m.Block{para(txt("one")), para(txt("two"))}}, cell(txt("x"))})), "|  |  |\n| --- | --- |\n| one<br>two | x |\n"},
		{"cell line break", doc(rows(0, []m.Cell{cell(txt("a"), m.LineBreak{}, txt("b"))})), "|  |\n| --- |\n| a<br>b |\n"},
		{"cell edge whitespace trimmed", doc(rows(0, []m.Cell{cell(txt("  padded\t")), cell(txt("plain"))})), "|  |  |\n| --- | --- |\n| padded | plain |\n"},
		{"trailing empty rows and columns trimmed", doc(rows(1,
			[]m.Cell{cell(txt("a")), cell(txt("b")), {}},
			[]m.Cell{cell(txt("c")), {}, {}},
			[]m.Cell{{}, {}, {}},
		)), "| a | b |\n| --- | --- |\n| c |  |\n"},
		{"interior empty row kept", doc(rows(0, []m.Cell{cell(txt("a"))}, []m.Cell{{}}, []m.Cell{cell(txt("b"))})), "|  |\n| --- |\n| a |\n|  |\n| b |\n"},
		{"all-empty table dropped", doc(rows(1, []m.Cell{cell(txt(" "))})), ""},
		{"col span renders blank covered", doc(mergedTable), "| wide |  | end |\n| --- | --- | --- |\n| a | b | c |\n"},
		{"trailing covered columns preserved", doc(tailTable), "| wide |  |  |\n| --- | --- | --- |\n"},
		{"row span renders blank covered", doc(tallTable), "|  |  |\n| --- | --- |\n| k | tall |\n| k2 |  |\n"},
		{"layout 1x1 unwrapped", doc(layout.Finish(m.LayoutTable)), "# Boxed\n\nbody\n"},
		{"data 1x1 kept", doc(rows(0, []m.Cell{cell(txt("x"))})), "|  |\n| --- |\n| x |\n"},
		{"url pipe", doc(rows(0, []m.Cell{cell(m.Link{Target: m.Target{Ref: "https://e.test/a|b"}})})), "|  |\n| --- |\n| [https://e.test/a\\|b](https://e.test/a%7Cb) |\n"},
		{"code span pipes", doc(rows(0, []m.Cell{cell(styled("a | b", code)), cell(styled(`a \| b`, code))})), "|  |  |\n| --- | --- |\n| `a \\| b` | `a \\\\\\| b` |\n"},
		{"math pipes", doc(rows(0, []m.Cell{cell(txt("abs")), cell(m.Math("|x|"))})), "|  |  |\n| --- | --- |\n| abs | $\\|x\\|$ |\n"},
		{"code block in cell", doc(rows(0, []m.Cell{{Blocks: []m.Block{m.CodeBlock{Text: "let `x` = 1;"}}}})), "|  |\n| --- |\n| ``let `x` = 1;`` |\n"},
		{"checkbox in cell", doc(rows(0, []m.Cell{cell(m.Checkbox(true)), cell(m.Checkbox(false), txt("Wall"))})), "|  |  |\n| --- | --- |\n| [x] | [ ] Wall |\n"},
		{"cell line start chars kept", doc(rows(0, []m.Cell{cell(txt("# 1. - >"))})), "|  |\n| --- |\n| # 1. - > |\n"},
		{"block content flattens", doc(rows(0, []m.Cell{listCell})), "|  |\n| --- |\n| **Head**<br>• dot<br>a. x<br>(ii) y<br>q<br>$\\|x\\|$<br>n1 /  / n3 |\n"},
	})
}

func TestFootnotes(t *testing.T) {
	runGoldens(t, []golden{
		{"renumbered by first reference", &m.Document{
			Blocks: []m.Block{para(txt("Claim."), m.NoteRef("b"), txt(" More."), m.NoteRef("a"), txt(" Again."), m.NoteRef("b"))},
			Notes: []m.Note{
				note("a", para(txt("Second note."))),
				note("b", para(txt("First note.")), para(txt("With a second paragraph."))),
			},
		}, "Claim.[^1] More.[^2] Again.[^1]\n\n[^1]: First note.\n\n    With a second paragraph.\n\n[^2]: Second note.\n"},
		{"empty and unreferenced notes", &m.Document{
			Blocks: []m.Block{para(txt("Text"), m.NoteRef("empty"))},
			Notes:  []m.Note{note("empty", para()), note("orphan", para(txt("Kept.")))},
		}, "Text\n\n[^1]: Kept.\n"},
		{"duplicate ids render one definition", &m.Document{
			Blocks: []m.Block{para(txt("Text"), m.NoteRef("a"))},
			Notes:  []m.Note{note("a", para(txt("First wins."))), note("a", para(txt("Duplicate dropped.")))},
		}, "Text[^1]\n\n[^1]: First wins.\n"},
		{"blank duplicate does not suppress later definition", &m.Document{
			Blocks: []m.Block{para(txt("Text"), m.NoteRef("a"))},
			Notes:  []m.Note{note("a", para()), note("a", para(txt("Usable.")))},
		}, "Text[^1]\n\n[^1]: Usable.\n"},
		{"references inside notes, lists, tables and quotes", &m.Document{
			Blocks: []m.Block{
				m.List{Items: []m.ListItem{item(para(txt("item"), m.NoteRef("c")))}},
				rows(0, []m.Cell{cell(txt("cell"), m.NoteRef("a"))}),
				m.Quote{para(m.Link{Content: []m.Inline{txt("q"), m.NoteRef("b")}, Target: m.Target{Ref: "https://e.test"}})},
			},
			Notes: []m.Note{
				note("a", para(txt("A"), m.NoteRef("d"))),
				note("b", para(txt("B"))),
				note("c", para(txt("C"))),
				note("d", para(txt("D"))),
			},
		}, "- item[^1]\n\n|  |\n| --- |\n| cell[^2] |\n\n> [q[^4]](https://e.test)\n\n[^1]: C\n\n[^2]: A[^3]\n\n[^3]: D\n\n[^4]: B\n"},
		{"dangling reference renders nothing", doc(para(txt("x"), m.NoteRef("missing"))), "x\n"},
	})
}

func TestAnchors(t *testing.T) {
	runGoldens(t, []golden{
		{"anchor on paragraph round trips", doc(
			para(m.Anchor("My Mark"), txt("Target here.")),
			para(toAnchor("jump", "My Mark")),
		), "<a id=\"my-mark\"></a>Target here.\n\n[jump](#my-mark)\n"},
		{"untargeted anchor renders nothing", doc(para(m.Anchor("standalone"), txt("No link points here."))), "No link points here.\n"},
		{"untargeted anchor does not split runs", doc(para(styled("a", bold), m.Anchor("x"), styled("b", bold))), "**ab**\n"},
		{"heading anchor uses slug", doc(
			m.Heading{Level: 2, Anchor: "bm1", Content: []m.Inline{txt("Section Two")}},
			para(toAnchor("go", "bm1")),
		), "## Section Two\n\n[go](#section-two)\n"},
		{"anchor inside heading uses slug", doc(
			m.Heading{Level: 1, Content: []m.Inline{m.Anchor("_Toc1"), txt("Intro")}},
			para(toAnchor("back", "_Toc1")),
		), "# Intro\n\n[back](#intro)\n"},
		{"duplicate heading slugs deduped", doc(
			heading(1, "Same"),
			m.Heading{Level: 1, Anchor: "x", Content: []m.Inline{txt("Same")}},
			para(toAnchor("second", "x")),
		), "# Same\n\n# Same\n\n[second](#same-1)\n"},
		{"anchor id avoids heading slug", doc(
			heading(1, "Intro"),
			para(m.Anchor("intro"), txt("para")),
			para(toAnchor("p", "intro")),
		), "# Intro\n\n<a id=\"intro-1\"></a>para\n\n[p](#intro-1)\n"},
		{"slug keeps word characters", doc(
			m.Heading{Level: 1, Anchor: "h", Content: []m.Inline{txt("C++ & Go: étude_x — 日本")}},
			para(toAnchor("h", "h")),
		), "# C++ & Go: étude_x — 日本\n\n[h](#c--go-étude_x--日本)\n"},
		{"punctuation-only heading slug", doc(
			m.Heading{Level: 1, Anchor: "h", Content: []m.Inline{txt("!!!")}},
			para(toAnchor("h", "h")),
		), "# !!!\n\n[h](#section)\n"},
		{"anchor targeted from a note and a cell", &m.Document{
			Blocks: []m.Block{
				para(txt("See"), m.NoteRef("n")),
				rows(0, []m.Cell{cell(m.Anchor("Cell Mark!"), txt("v"))}),
			},
			Notes: []m.Note{note("n", para(toAnchor("cell", "Cell Mark!")))},
		}, "See[^1]\n\n|  |\n| --- |\n| <a id=\"cell-mark\"></a>v |\n\n[^1]: [cell](#cell-mark)\n"},
		{"empty-label anchor link dropped", doc(para(m.Anchor("a"), txt("t")), para(txt("x "), toAnchor("", "a"))), "<a id=\"a\"></a>t\n\nx\n"},
	})
}

func FuzzRender(f *testing.F) {
	f.Add("a *b* _c_ `d` $e$ [f] <g> &amp; \\ | 1. - # > ===", uint8(0))
	f.Add(" **\n\n``` $$ \x00 ~~", uint8(255))
	f.Fuzz(func(t *testing.T, s string, style uint8) {
		st := m.Style{Bold: style&1 != 0, Italic: style&2 != 0, Strike: style&4 != 0, Code: style&8 != 0}
		inlines := []m.Inline{txt(s), styled(s, st), m.LineBreak{}, m.Math(s), external(s, s), m.Anchor(s), toAnchor(s, s), m.Image{Alt: s}, m.Checkbox(true), txt(s)}
		d := &m.Document{
			Blocks: []m.Block{
				m.Heading{Level: int(style), Anchor: s, Content: inlines},
				para(inlines...),
				m.List{Marker: m.MarkerKind(style % 6), Start: int(style), Items: []m.ListItem{{Blocks: []m.Block{para(inlines...)}, Label: s}}},
				rows(1, []m.Cell{cell(inlines...), {Blocks: []m.Block{m.CodeBlock{Text: s}, m.MathBlock(s)}}}),
				m.Quote{m.CodeBlock{Lang: "x", Text: s}},
			},
			Notes: []m.Note{note(s, para(txt(s)))},
		}
		first, second := Render(d), Render(d)
		if first != second {
			t.Fatal("render is not deterministic")
		}
	})
}

func TestAllShapesGolden(t *testing.T) {
	var grid m.GridBuilder
	grid.NextRow()
	for _, s := range []string{"Region", "Q1", "Q2"} {
		_ = grid.Place(cell(styled(s, bold)))
	}
	grid.NextRow()
	_ = grid.Place(m.Cell{Blocks: []m.Block{para(txt("North"))}, RowSpan: 2})
	_ = grid.Place(m.Cell{Blocks: []m.Block{para(txt("$1 | $2"))}, ColSpan: 2})
	grid.NextRow()
	_ = grid.Place(cell(txt("3")))
	_ = grid.Place(cell(m.Math(`\frac{1}{2}`)))
	table := grid.Finish(m.DataTable)
	table.HeaderRows = 1

	d := &m.Document{
		Blocks: []m.Block{
			m.Heading{Level: 1, Anchor: "top", Content: []m.Inline{txt("Quarterly "), styled("Report", italic)}},
			para(txt("Revenue grew"), m.NoteRef("fn2"), txt(" per "), external("the dashboard", "https://example.com/q?a=1&b=(2)"), txt(", see "), toAnchor("the table", "tbl"), txt(".")),
			para(m.Image{Alt: "logo", Source: m.ImageSource{Kind: m.SourceAsset}}, txt(" "), m.Image{Alt: "chart", Source: m.ImageSource{Kind: m.SourceExternal, URL: "https://example.com/c.png"}}),
			m.List{Marker: m.Decimal, Start: 1, Items: []m.ListItem{
				item(para(styled("Plan", bold), txt(" the work"))),
				item(para(txt("Do it")), m.List{Marker: m.LowerRoman, Start: 1, Items: []m.ListItem{
					item(para(m.Checkbox(true), txt("draft"))),
					{Blocks: []m.Block{para(m.Checkbox(false), txt("review"))}, Label: "2-b)"},
				}}),
			}},
			para(m.Anchor("tbl"), txt("Figures:")),
			table,
			m.Quote{para(txt("Numbers are "), styled("unaudited", m.Style{Strike: true})), m.CodeBlock{Lang: "sql", Text: "SELECT 1;"}},
			m.MathBlock(`E = mc^2`),
			m.Rule{},
			para(txt("Back to "), toAnchor("top", "top"), m.NoteRef("fn1")),
		},
		Notes: []m.Note{
			{ID: "fn1", Kind: m.Endnote, Blocks: []m.Block{para(txt("Endnote body."))}},
			{ID: "fn2", Kind: m.Footnote, Blocks: []m.Block{para(txt("Source: finance.")), m.List{Items: []m.ListItem{item(para(txt("ledger")))}}}},
		},
		Assets: []m.Asset{{ID: 0, MediaType: "image/png", OriginPart: "word/media/image1.png", Bytes: []byte{0x89, 'P', 'N', 'G'}}},
	}
	want, err := os.ReadFile(filepath.Join("testdata", "all-shapes.md"))
	if err != nil {
		t.Fatal(err)
	}
	got := Render(d)
	if got != string(want) {
		t.Fatalf("golden mismatch\n got:\n%s\nwant:\n%s", got, want)
	}
	if strings.Contains(got, "\r") || Render(d) != got {
		t.Fatal("render must be deterministic LF output")
	}
}

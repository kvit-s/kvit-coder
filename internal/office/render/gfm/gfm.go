// Package gfm serializes the document model to GitHub-Flavored Markdown.
package gfm

import (
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

type renderer struct {
	notes   map[string]int
	anchors anchorMap
}

// Render serializes doc deterministically. Footnotes are numbered in
// first-reference order with unreferenced notes after, and their definitions
// follow the body.
func Render(doc *model.Document) string {
	r := &renderer{notes: numberNotes(doc), anchors: resolveAnchors(doc)}
	parts := r.blockParts(doc.Blocks)
	type numbered struct {
		note *model.Note
		num  int
	}
	var defs []numbered
	for i := range doc.Notes {
		if num, ok := r.notes[doc.Notes[i].ID]; ok {
			defs = append(defs, numbered{&doc.Notes[i], num})
		}
	}
	slices.SortStableFunc(defs, func(a, b numbered) int { return a.num - b.num })
	rendered := map[int]bool{}
	for _, def := range defs {
		body := r.blocks(def.note.Blocks)
		if body == "" || rendered[def.num] {
			continue
		}
		rendered[def.num] = true
		ls := lines(body)
		var sb strings.Builder
		sb.WriteString("[^" + strconv.Itoa(def.num) + "]: " + ls[0])
		for _, line := range ls[1:] {
			sb.WriteByte('\n')
			if line != "" {
				sb.WriteString("    " + line)
			}
		}
		parts = append(parts, sb.String())
	}
	out := strings.Join(parts, "\n\n")
	if out != "" {
		out += "\n"
	}
	return out
}

func numberNotes(doc *model.Document) map[string]int {
	valid := map[string]*model.Note{}
	for i := range doc.Notes {
		n := &doc.Notes[i]
		if _, seen := valid[n.ID]; !seen && !model.EmptyBlocks(n.Blocks) {
			valid[n.ID] = n
		}
	}
	nums := map[string]int{}
	var visit func(inlines []model.Inline)
	visit = func(inlines []model.Inline) {
		for _, in := range inlines {
			switch in := in.(type) {
			case model.NoteRef:
				note, ok := valid[string(in)]
				if _, numbered := nums[string(in)]; ok && !numbered {
					nums[string(in)] = len(nums) + 1
					walkInlines(note.Blocks, visit)
				}
			case model.Link:
				visit(in.Content)
			}
		}
	}
	walkInlines(doc.Blocks, visit)
	for _, n := range doc.Notes {
		if _, numbered := nums[n.ID]; valid[n.ID] != nil && !numbered {
			nums[n.ID] = len(nums) + 1
		}
	}
	return nums
}

func (r *renderer) blockParts(blocks []model.Block) []string {
	var parts []string
	for _, b := range blocks {
		if s := r.block(b); s != "" {
			parts = append(parts, s)
		}
	}
	return parts
}

func (r *renderer) blocks(blocks []model.Block) string {
	return strings.Join(r.blockParts(blocks), "\n\n")
}

func (r *renderer) block(b model.Block) string {
	switch b := b.(type) {
	case model.Heading:
		text := headingText(r.inlines(b.Content, headingContext, false))
		if text == "" {
			return ""
		}
		return strings.Repeat("#", min(max(b.Level, 1), 6)) + " " + text
	case model.Paragraph:
		return trimParagraph(r.inlines(b, blockContext, false))
	case model.List:
		return r.list(b)
	case model.Table:
		if b.Kind == model.LayoutTable && b.IsSingleCell() {
			return r.blocks(b.Grid()[0][0].Cell.Blocks)
		}
		return r.table(b)
	case model.Quote:
		inner := r.blocks(b)
		if inner == "" {
			return ""
		}
		ls := lines(inner)
		for i, l := range ls {
			if l == "" {
				ls[i] = ">"
			} else {
				ls[i] = "> " + l
			}
		}
		return strings.Join(ls, "\n")
	case model.CodeBlock:
		fence := backtickFence(b.Text, 3)
		return fence + b.Lang + "\n" + strings.TrimRight(b.Text, "\n") + "\n" + fence
	case model.Rule:
		return "---"
	case model.MathBlock:
		tex := strings.TrimSpace(string(b))
		if tex == "" {
			return ""
		}
		return "$$\n" + escapeUnescaped(tex, '$') + "\n$$"
	}
	return ""
}

// list renders ordered non-decimal kinds and composite labels as bullets
// carrying the literal marker, since GFM has only decimal ordered lists.
func (r *renderer) list(list model.List) string {
	if len(list.Items) == 0 {
		return ""
	}
	items := make([]string, 0, len(list.Items))
	loose := false
	for i, it := range list.Items {
		var marker string
		switch {
		case it.Label != "":
			marker = "- " + escapeMarkerLabel(it.Label, blockContext) + " "
		case list.Marker == model.Bullet:
			marker = "- "
		case list.Marker == model.Decimal:
			marker = strconv.Itoa(list.Start+i) + ". "
		default:
			marker = "- " + list.Marker.Label(list.Start+i) + " "
		}
		if len(it.Blocks) > 1 {
			loose = true
		}
		indent := strings.Repeat(" ", len([]rune(marker)))
		ls := lines(r.blocks(it.Blocks))
		var sb strings.Builder
		sb.WriteString(marker)
		if len(ls) > 0 {
			sb.WriteString(ls[0])
			ls = ls[1:]
		}
		for _, line := range ls {
			sb.WriteByte('\n')
			if line == "" {
				loose = true
			} else {
				sb.WriteString(indent + line)
			}
		}
		items = append(items, sb.String())
	}
	if loose {
		return strings.Join(items, "\n\n")
	}
	return strings.Join(items, "\n")
}

// trimParagraph trims each line, keeping hard-break backslashes, and drops a
// hard break that ends the paragraph.
func trimParagraph(text string) string {
	ls := lines(text)
	first, last := -1, -1
	for i, l := range ls {
		t := strings.TrimLeftFunc(l, unicode.IsSpace)
		if !endsWithHardBreak(t) {
			t = strings.TrimRightFunc(t, unicode.IsSpace)
		}
		if strings.TrimSpace(strings.TrimRight(t, `\`)) == "" {
			t = ""
		}
		ls[i] = t
		if t != "" {
			if first < 0 {
				first = i
			}
			last = i
		}
	}
	if first < 0 {
		return ""
	}
	out := strings.Join(ls[first:last+1], "\n")
	if endsWithHardBreak(out) {
		out = strings.TrimRightFunc(out[:len(out)-1], unicode.IsSpace)
	}
	return out
}

func endsWithHardBreak(line string) bool {
	return (len(line)-len(strings.TrimRight(line, `\`)))%2 == 1
}

// lines splits like a line iterator: no trailing empty line for a final
// newline, and a carriage return before each newline is dropped.
func lines(s string) []string {
	if s == "" {
		return nil
	}
	ls := strings.Split(strings.TrimSuffix(s, "\n"), "\n")
	for i, l := range ls {
		ls[i] = strings.TrimSuffix(l, "\r")
	}
	return ls
}

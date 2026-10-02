package docx

import (
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

type listKey struct {
	instance int
	marker   model.MarkerKind
}

// listEntry is one resolved list paragraph with any blocks anchored in it.
type listEntry struct {
	level  int
	key    listKey
	number int
	label  string
	blocks []model.Block
}

// buildLists folds a flat run of list paragraphs into nested lists, splitting
// at identity or marker changes and at ordered-sequence discontinuities so
// start + index reproduces the source numbers.
func buildLists(entries []listEntry) []model.Block {
	if len(entries) == 0 {
		return nil
	}
	minLevel := entries[0].level
	for _, e := range entries {
		minLevel = min(minLevel, e.level)
	}
	var (
		out     []model.Block
		current *model.List
		key     listKey
		last    int
	)
	flush := func() {
		if current != nil && len(current.Items) > 0 {
			out = append(out, *current)
		}
		current = nil
	}
	for i := 0; i < len(entries); {
		e := entries[i]
		if e.level <= minLevel {
			i++
			if current == nil || key != e.key || e.key.marker.Ordered() && last+1 != e.number {
				flush()
				current = &model.List{Marker: e.key.marker, Start: 1}
				if e.key.marker.Ordered() {
					current.Start = e.number
				}
				key = e.key
			}
			current.Items = append(current.Items, model.ListItem{Blocks: e.blocks, Label: e.label})
			last = e.number
			continue
		}
		j := i
		for j < len(entries) && entries[j].level > minLevel {
			j++
		}
		sub := buildLists(entries[i:j])
		i = j
		if len(sub) == 0 {
			continue
		}
		if current == nil {
			current = &model.List{Marker: model.Bullet, Start: 1}
			key = listKey{instance: -1, marker: model.Bullet}
		}
		if len(current.Items) == 0 {
			current.Items = append(current.Items, model.ListItem{})
		}
		item := &current.Items[len(current.Items)-1]
		item.Blocks = append(item.Blocks, sub...)
	}
	flush()
	return out
}

type blockKind int

const (
	noBlock blockKind = iota
	quoteBlock
	codeBlock
)

// blockKindOf maps the built-in paragraph style names Word and Pandoc use
// for quotations and preformatted text.
func blockKindOf(styleName string) blockKind {
	switch strings.ToLower(strings.TrimSpace(strings.ReplaceAll(styleName, "_20_", " "))) {
	case "quote", "intense quote", "block text", "quotations":
		return quoteBlock
	case "html preformatted", "source code", "preformatted text":
		return codeBlock
	}
	return noBlock
}

// styledRun folds consecutive paragraphs of one styled container into a
// single block: producers write a multi-paragraph quote, and code one line
// per paragraph.
type styledRun struct {
	kind  blockKind
	quote []model.Block
	code  []string
}

func (r *styledRun) push(kind blockKind, inlines []model.Inline, out *[]model.Block) {
	if r.kind != kind {
		r.flush(out)
		r.kind = kind
	}
	if kind == codeBlock {
		r.code = append(r.code, model.PlainText(inlines))
	} else if !model.IsEmpty(inlines) {
		r.quote = append(r.quote, model.Paragraph(inlines))
	}
}

func (r *styledRun) flush(out *[]model.Block) {
	switch r.kind {
	case quoteBlock:
		if len(r.quote) > 0 {
			*out = append(*out, model.Quote(r.quote))
		}
	case codeBlock:
		first, last := -1, -1
		for i, l := range r.code {
			if strings.TrimSpace(l) != "" {
				if first < 0 {
					first = i
				}
				last = i
			}
		}
		if first >= 0 {
			*out = append(*out, model.CodeBlock{Text: strings.Join(r.code[first:last+1], "\n")})
		}
	}
	*r = styledRun{}
}

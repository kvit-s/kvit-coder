package gfm

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

type resolvedAnchor struct {
	fragment string
	emitHTML bool
}

// anchorMap maps anchor ids to their rendered fragments. Heading-coincident
// anchors reuse the heading's slug; other anchors get an HTML id only when a
// link targets them, since producers mark far more positions than they
// reference.
type anchorMap map[string]resolvedAnchor

func (a anchorMap) fragment(id string) (string, bool) {
	r, ok := a[id]
	return r.fragment, ok
}

func (a anchorMap) htmlID(id string) (string, bool) {
	r, ok := a[id]
	return r.fragment, ok && r.emitHTML
}

func resolveAnchors(doc *model.Document) anchorMap {
	resolved := anchorMap{}
	linked := map[string]bool{}
	var collectTargets func(inlines []model.Inline)
	collectTargets = func(inlines []model.Inline) {
		for _, in := range inlines {
			if link, ok := in.(model.Link); ok {
				if link.Target.Kind == model.TargetAnchor {
					linked[link.Target.Ref] = true
				}
				collectTargets(link.Content)
			}
		}
	}
	walkInlines(doc.Blocks, collectTargets)
	for _, n := range doc.Notes {
		walkInlines(n.Blocks, collectTargets)
	}

	ids := uniqueIDs{used: map[string]bool{}, next: map[string]int{}}
	walkBlocks(doc.Blocks, func(b model.Block) {
		h, ok := b.(model.Heading)
		if !ok {
			return
		}
		slug := ids.claim(gfmSlug(model.PlainText(h.Content)))
		bind := func(id string) {
			if _, ok := resolved[id]; !ok {
				resolved[id] = resolvedAnchor{fragment: slug}
			}
		}
		if h.Anchor != "" {
			bind(h.Anchor)
		}
		forEachAnchor(h.Content, bind)
	})

	assign := func(inlines []model.Inline) {
		forEachAnchor(inlines, func(id string) {
			if _, done := resolved[id]; linked[id] && !done {
				resolved[id] = resolvedAnchor{fragment: ids.claim(sanitizeID(id)), emitHTML: true}
			}
		})
	}
	walkInlines(doc.Blocks, assign)
	for _, n := range doc.Notes {
		walkInlines(n.Blocks, assign)
	}
	return resolved
}

func walkBlocks(blocks []model.Block, f func(model.Block)) {
	for _, b := range blocks {
		f(b)
		switch b := b.(type) {
		case model.List:
			for _, it := range b.Items {
				walkBlocks(it.Blocks, f)
			}
		case model.Table:
			for _, row := range b.Grid() {
				for _, slot := range row {
					if !slot.Covered {
						walkBlocks(slot.Cell.Blocks, f)
					}
				}
			}
		case model.Quote:
			walkBlocks(b, f)
		}
	}
}

func walkInlines(blocks []model.Block, f func([]model.Inline)) {
	walkBlocks(blocks, func(b model.Block) {
		switch b := b.(type) {
		case model.Heading:
			f(b.Content)
		case model.Paragraph:
			f(b)
		}
	})
}

func forEachAnchor(inlines []model.Inline, f func(string)) {
	for _, in := range inlines {
		switch in := in.(type) {
		case model.Anchor:
			f(string(in))
		case model.Link:
			forEachAnchor(in.Content, f)
		}
	}
}

type uniqueIDs struct {
	used map[string]bool
	next map[string]int
}

func (u *uniqueIDs) claim(base string) string {
	if !u.used[base] {
		u.used[base] = true
		return base
	}
	n := max(u.next[base], 1)
	for {
		candidate := base + "-" + strconv.Itoa(n)
		n++
		if !u.used[candidate] {
			u.used[candidate] = true
			u.next[base] = n
			return candidate
		}
	}
}

// gfmSlug lowercases, turns spaces into hyphens and keeps only letters,
// numbers, marks, connector punctuation and hyphens; an empty slug becomes
// "section" so the heading stays linkable.
func gfmSlug(text string) string {
	slug := strings.Map(func(c rune) rune {
		c = unicode.ToLower(c)
		switch {
		case c == ' ':
			return '-'
		case c == '-' || isAlnum(c) || unicode.IsMark(c) || unicode.Is(unicode.Pc, c):
			return c
		}
		return -1
	}, strings.TrimSpace(text))
	if slug == "" {
		return "section"
	}
	return slug
}

func sanitizeID(id string) string {
	var sb strings.Builder
	prevDash := false
	for _, c := range id {
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '_') {
			c = '-'
		}
		if c == '-' && prevDash {
			continue
		}
		prevDash = c == '-'
		sb.WriteRune(c)
	}
	if out := strings.Trim(sb.String(), "-"); out != "" {
		return out
	}
	return "anchor"
}

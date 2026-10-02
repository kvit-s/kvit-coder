package pptx

import "github.com/kvit-s/kvit-coder/internal/office/model"

type listKey struct {
	instance int
	marker   model.MarkerKind
}

// listEntry is one flat, resolved list paragraph.
type listEntry struct {
	level  int
	key    listKey
	number int
	label  string
	blocks []model.Block
}

// buildLists folds a flat run into nested lists, splitting wherever the list
// identity or marker changes or an ordered sequence is non-contiguous, so
// the renderer's start+index numbering reproduces the source.
func buildLists(entries []listEntry) []model.Block {
	if len(entries) == 0 {
		return nil
	}
	minLevel := entries[0].level
	for _, e := range entries[1:] {
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
			key, last = listKey{instance: -1, marker: model.Bullet}, 0
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

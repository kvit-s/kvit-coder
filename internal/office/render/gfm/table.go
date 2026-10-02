package gfm

import (
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

type renderedCell struct {
	text    string
	covered bool
}

// table renders covered positions as blank cells, since GFM has no span
// syntax, and always emits a header row because GFM tables require one.
func (r *renderer) table(t model.Table) string {
	grid := t.Grid()
	if len(grid) == 0 {
		return ""
	}
	width := 0
	for _, row := range grid {
		width = max(width, len(row))
	}
	rendered := make([][]renderedCell, len(grid))
	for i, row := range grid {
		cells := make([]renderedCell, width)
		for j, slot := range row {
			if slot.Covered {
				cells[j].covered = true
			} else {
				cells[j].text = r.cell(slot.Cell)
			}
		}
		rendered[i] = cells
	}
	for len(rendered) > 1 && blankRow(rendered[len(rendered)-1]) {
		rendered = rendered[:len(rendered)-1]
	}
	width = 0
	for _, row := range rendered {
		for j := len(row) - 1; j >= 0; j-- {
			if row[j].text != "" || row[j].covered {
				width = max(width, j+1)
				break
			}
		}
	}
	if width == 0 {
		return ""
	}
	header := make([]renderedCell, width)
	if t.HeaderRows >= 1 {
		copy(header, rendered[0])
		rendered = rendered[1:]
	}
	var sb strings.Builder
	writeRow(&sb, header)
	sb.WriteString("\n|" + strings.Repeat(" --- |", width))
	for _, row := range rendered {
		sb.WriteByte('\n')
		writeRow(&sb, row[:width])
	}
	return sb.String()
}

func blankRow(row []renderedCell) bool {
	for _, c := range row {
		if c.text != "" || c.covered {
			return false
		}
	}
	return true
}

func writeRow(sb *strings.Builder, cells []renderedCell) {
	sb.WriteByte('|')
	for _, c := range cells {
		sb.WriteByte(' ')
		sb.WriteString(c.text)
		sb.WriteString(" |")
	}
}

// cell flattens block content into one table-cell line; edge whitespace is
// padding the table's own padding would swallow.
func (r *renderer) cell(c model.Cell) string {
	var parts []string
	for _, b := range c.Blocks {
		parts = r.cellBlock(b, parts)
	}
	if len(parts) == 1 && !strings.Contains(parts[0], "\n") {
		return strings.TrimSpace(parts[0])
	}
	var kept []string
	for _, l := range lines(strings.Join(parts, "<br>")) {
		if l = strings.TrimSpace(l); l != "" {
			kept = append(kept, l)
		}
	}
	return strings.Join(kept, "<br>")
}

func (r *renderer) cellBlock(b model.Block, parts []string) []string {
	switch b := b.(type) {
	case model.Heading:
		if t := strings.TrimSpace(r.inlines(b.Content, cellContext, false)); t != "" {
			parts = append(parts, "**"+t+"**")
		}
	case model.Paragraph:
		if t := r.inlines(b, cellContext, false); strings.TrimSpace(t) != "" {
			parts = append(parts, t)
		}
	case model.List:
		for i, it := range b.Items {
			var inner []string
			for _, ib := range it.Blocks {
				inner = r.cellBlock(ib, inner)
			}
			if len(inner) == 0 {
				continue
			}
			var marker string
			switch {
			case it.Label != "":
				marker = escapeMarkerLabel(it.Label, cellContext)
			case b.Marker == model.Bullet:
				marker = "•"
			default:
				marker = b.Marker.Label(b.Start + i)
			}
			parts = append(parts, marker+" "+strings.Join(inner, " "))
		}
	case model.Table:
		for _, row := range b.Grid() {
			cells := make([]string, len(row))
			filled := false
			for j, slot := range row {
				if !slot.Covered {
					cells[j] = r.cell(slot.Cell)
					filled = filled || cells[j] != ""
				}
			}
			if filled {
				parts = append(parts, strings.Join(cells, " / "))
			}
		}
	case model.Quote:
		for _, qb := range b {
			parts = r.cellBlock(qb, parts)
		}
	case model.CodeBlock:
		if t := strings.TrimSpace(b.Text); t != "" {
			parts = append(parts, codeSpan(t, cellContext))
		}
	case model.MathBlock:
		if strings.TrimSpace(string(b)) != "" {
			parts = append(parts, mathSpan(string(b), cellContext))
		}
	}
	return parts
}

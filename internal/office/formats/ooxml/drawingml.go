package ooxml

import (
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

// ChartBlocks renders a chart part as its bold title plus a categories by
// series table of the cached display strings; c:f formulas are not text.
func ChartBlocks(root *opc.Element) []model.Block {
	var blocks []model.Block
	if title := root.FirstDescendant(opc.NSChart, "title"); title != nil {
		if text := CleanText(drawingText(title)); strings.TrimSpace(text) != "" {
			blocks = append(blocks, model.Paragraph{model.Text{Text: text, Style: model.Style{Bold: true}}})
		}
	}
	type series struct {
		name   string
		values []string
	}
	var (
		categories []string
		all        []series
	)
	for ser := range root.Descendants(opc.NSChart, "ser") {
		var s series
		if tx := ser.Child(opc.NSChart, "tx"); tx != nil {
			if v := tx.FirstDescendant(opc.NSChart, "v"); v != nil {
				s.name = CleanText(v.Text())
			}
		}
		if len(categories) == 0 {
			categories = cachedValues(ser.Child(opc.NSChart, "cat"))
		}
		s.values = cachedValues(ser.Child(opc.NSChart, "val"))
		all = append(all, s)
	}
	if len(all) == 0 || len(categories) == 0 {
		return blocks
	}
	var axisTitle string
	if ax := root.FirstDescendant(opc.NSChart, "catAx"); ax != nil {
		if title := ax.Child(opc.NSChart, "title"); title != nil {
			axisTitle = CleanText(drawingText(title))
		}
	}
	header := []model.Cell{plainCell(axisTitle)}
	for _, s := range all {
		header = append(header, plainCell(s.name))
	}
	rows := [][]model.Cell{header}
	for i, category := range categories {
		row := []model.Cell{plainCell(category)}
		for _, s := range all {
			var v string
			if i < len(s.values) {
				v = s.values[i]
			}
			row = append(row, plainCell(v))
		}
		rows = append(rows, row)
	}
	return append(blocks, model.TableFromRows(rows, 1, model.DataTable))
}

func cachedValues(e *opc.Element) []string {
	if e == nil {
		return nil
	}
	var out []string
	for v := range e.Descendants(opc.NSChart, "v") {
		out = append(out, CleanText(v.Text()))
	}
	return out
}

func plainCell(text string) model.Cell {
	return model.Cell{Blocks: []model.Block{model.Paragraph{model.Text{Text: text}}}}
}

// DiagramBlocks renders a SmartArt data part as a bullet list of its text
// points in order.
func DiagramBlocks(root *opc.Element) []model.Block {
	var items []model.ListItem
	for pt := range root.Descendants(opc.NSDiagram, "pt") {
		t := pt.Child(opc.NSDiagram, "t")
		if t == nil {
			continue
		}
		if text := CleanText(t.Text()); strings.TrimSpace(text) != "" {
			items = append(items, model.ListItem{Blocks: []model.Block{model.Paragraph{model.Text{Text: text}}}})
		}
	}
	if len(items) == 0 {
		return nil
	}
	return []model.Block{model.List{Marker: model.Bullet, Start: 1, Items: items}}
}

// drawingText joins the non-blank paragraphs of DrawingML rich text, or
// falls back to all of the element's text.
func drawingText(e *opc.Element) string {
	var parts []string
	for p := range e.Descendants(opc.NSDrawingML, "p") {
		if text := p.Text(); strings.TrimSpace(text) != "" {
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return e.Text()
	}
	return strings.Join(parts, " ")
}

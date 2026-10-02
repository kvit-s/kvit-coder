package docx

import (
	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

// maxGridFiller bounds gridBefore/gridAfter/gridSpan values from the source.
const maxGridFiller = 1000

type tableCell struct {
	elem *opc.Element
	// merged holds legacy hMerge continuation cells folded into this origin.
	merged  []*opc.Element
	colSpan int
	rowSpan int
	// covered marks a vMerge continuation belonging to the origin above.
	covered bool
}

func fillerCells(trPr *opc.Element, local string) []tableCell {
	n, _ := intVal(trPr, local)
	cells := make([]tableCell, min(max(n, 0), maxGridFiller))
	for i := range cells {
		cells[i] = tableCell{colSpan: 1, rowSpan: 1}
	}
	return cells
}

// parseTable collects the raw cell matrix, resolves vertical merges into row
// spans by grid column, and builds the canonical grid. gridBefore/gridAfter
// filler becomes empty cells so every cell keeps its grid column.
func (c *converter) parseTable(tbl *opc.Element) ([]model.Block, error) {
	var matrix [][]tableCell
	declaredHeader, counting := 0, true
	for tr := range tbl.Children(nsW, "tr") {
		trPr := child(tr, "trPr")
		row := fillerCells(trPr, "gridBefore")
		row = collectRowCells(tr, row)
		row = append(row, fillerCells(trPr, "gridAfter")...)
		matrix = append(matrix, row)
		// tblHeader is ST_OnOff: an explicit false is not a header row.
		if on, _ := onOff(trPr, "tblHeader"); counting && on {
			declaredHeader++
		} else {
			counting = false
		}
	}

	type pos struct{ row, idx int }
	active := map[int]pos{}
	for r := range matrix {
		next := map[int]pos{}
		col := 0
		for i := range matrix[r] {
			cell := &matrix[r][i]
			owner, register := pos{r, i}, true
			if cell.covered {
				origin, ok := active[col]
				if ok {
					originCell := &matrix[origin.row][origin.idx]
					originCell.rowSpan++
					// A continuation never covers more columns than its
					// origin spans, so hostile gridSpans add no slots.
					cell.colSpan = min(cell.colSpan, originCell.colSpan)
					owner = origin
				} else {
					cell.covered, register = false, false
				}
			}
			for k := col; register && k < col+cell.colSpan; k++ {
				next[k] = owner
			}
			col += cell.colSpan
		}
		active = next
	}

	var b model.GridBuilder
	for _, row := range matrix {
		b.NextRow()
		for _, cell := range row {
			if cell.covered {
				for range cell.colSpan {
					b.Covered()
				}
				continue
			}
			var blocks []model.Block
			for _, e := range append([]*opc.Element{cell.elem}, cell.merged...) {
				if e == nil {
					continue
				}
				inner, err := c.parseBlocks(e)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, inner...)
			}
			if err := b.Place(model.Cell{Blocks: blocks, ColSpan: cell.colSpan, RowSpan: cell.rowSpan}); err != nil {
				return nil, err
			}
		}
	}
	table := b.Finish(model.DataTable)
	if len(table.Grid()) == 0 {
		return nil, nil
	}
	table.HeaderRows = model.ResolveHeaderRows(table, declaredHeader)
	return []model.Block{table}, nil
}

func collectRowCells(parent *opc.Element, cells []tableCell) []tableCell {
	for e := range parent.Elements() {
		if e.Space != nsW {
			continue
		}
		switch e.Local {
		case "tc":
			tcPr := child(e, "tcPr")
			vMerge := child(tcPr, "vMerge")
			covered := false
			if vMerge != nil {
				v, _ := vMerge.Attr(nsW, "val")
				covered = v != "restart"
			}
			span, ok := intVal(tcPr, "gridSpan")
			if !ok || span < 1 {
				span = 1
			}
			span = min(span, maxGridFiller)
			if hMerge := child(tcPr, "hMerge"); hMerge != nil {
				if v, _ := hMerge.Attr(nsW, "val"); v != "restart" && len(cells) > 0 && !cells[len(cells)-1].covered {
					prev := &cells[len(cells)-1]
					prev.colSpan += span
					prev.merged = append(prev.merged, e)
					continue
				}
			}
			cells = append(cells, tableCell{elem: e, colSpan: span, rowSpan: 1, covered: covered})
		case "sdt":
			if content := child(e, "sdtContent"); content != nil {
				cells = collectRowCells(content, cells)
			}
		case "customXml":
			cells = collectRowCells(e, cells)
		}
	}
	return cells
}

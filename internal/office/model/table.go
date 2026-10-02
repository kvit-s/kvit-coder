package model

import (
	"math/bits"
	"slices"
)

// Fixed, non-configurable table budgets. MaxExpansion bounds the positions
// span expansion may claim per table; MaxGridSlots bounds the positions a
// spreadsheet frontend may materialize for one sheet.
const (
	MaxExpansion = 4_000_000
	MaxGridSlots = 4_000_000
)

type LimitError struct {
	Limit  string
	Detail string
}

func (e *LimitError) Error() string {
	return "resource limit exceeded (" + e.Limit + "): " + e.Detail
}

type TableKind int

const (
	DataTable TableKind = iota
	LayoutTable
)

// Cell spans below 1 count as 1.
type Cell struct {
	Blocks  []Block
	ColSpan int
	RowSpan int
}

// EmptyBlocks reports whether blocks render nothing; only paragraphs count
// toward emptiness.
func EmptyBlocks(blocks []Block) bool {
	for _, b := range blocks {
		p, ok := b.(Paragraph)
		if !ok || !IsEmpty(p) {
			return false
		}
	}
	return true
}

// IsEmpty reports whether the cell renders nothing.
func (c Cell) IsEmpty() bool { return EmptyBlocks(c.Blocks) }

// Slot is one grid position: an origin holding the cell, or a position
// covered by the span of the origin at (OriginRow, OriginCol).
type Slot struct {
	Covered   bool
	OriginRow int
	OriginCol int
	Cell      Cell
}

// Table is a canonical grid: every logical position appears exactly once,
// content and spans live only on origins, and each position a span claims
// holds a covered slot pointing back at its origin. Grids are built only by
// GridBuilder or TableFromRows. Rows may be ragged.
type Table struct {
	grid       [][]Slot
	HeaderRows int
	Kind       TableKind
}

func (t Table) Grid() [][]Slot { return t.grid }

func (t Table) IsSingleCell() bool {
	return len(t.grid) == 1 && len(t.grid[0]) == 1 && !t.grid[0][0].Covered
}

// TableFromRows builds a span-less table; any spans on the cells are ignored.
func TableFromRows(rows [][]Cell, headerRows int, kind TableKind) Table {
	var b GridBuilder
	for _, row := range rows {
		b.NextRow()
		for _, cell := range row {
			cell.ColSpan, cell.RowSpan = 1, 1
			_ = b.Place(cell)
		}
	}
	t := b.Finish(kind)
	t.HeaderRows = headerRows
	return t
}

type position struct{ row, col int }

var emptySlot = Slot{Cell: Cell{ColSpan: 1, RowSpan: 1}}

// GridBuilder is the sole constructor of table grids. The zero value is ready
// to use.
type GridBuilder struct {
	grid            [][]Slot
	pending         map[position]position
	expansion       uint64
	keepCoveredTail bool
}

// NextRow starts a row with room for as many slots as the previous one.
func (b *GridBuilder) NextRow() {
	var row []Slot
	if n := len(b.grid); n > 0 {
		row = make([]Slot, 0, len(b.grid[n-1]))
	}
	b.grid = append(b.grid, row)
}

// KeepCoveredTail keeps trailing rows holding only covered positions: a
// spreadsheet merge region is real extent even when every covered cell is
// empty.
func (b *GridBuilder) KeepCoveredTail() {
	b.keepCoveredTail = true
}

func (b *GridBuilder) rowIndex() int {
	if len(b.grid) == 0 {
		b.grid = append(b.grid, nil)
	}
	return len(b.grid) - 1
}

func (b *GridBuilder) skipPending(row int) {
	for {
		at := position{row, len(b.grid[row])}
		origin, ok := b.pending[at]
		if !ok {
			return
		}
		delete(b.pending, at)
		b.grid[row] = append(b.grid[row], Slot{Covered: true, OriginRow: origin.row, OriginCol: origin.col})
	}
}

// Place puts cell at the next free position of the current row. Positions
// its spans claim become pending: an explicit covered marker consumes one
// through Covered, and omitted ones materialize as later cells are placed.
// Spans overlapping earlier claims are clamped. The whole span area is
// charged against MaxExpansion before any expansion.
func (b *GridBuilder) Place(cell Cell) error {
	cols, rows := uint64(max(cell.ColSpan, 1)), uint64(max(cell.RowSpan, 1))
	hi, area := bits.Mul64(cols, rows)
	if hi != 0 {
		area = ^uint64(0)
	}
	b.expansion += min(area-1, ^uint64(0)-b.expansion)
	if b.expansion > MaxExpansion {
		return &LimitError{Limit: "max_expansion", Detail: "table span expansion exceeds the content budget"}
	}
	row := b.rowIndex()
	b.skipPending(row)
	col := len(b.grid[row])
	colSpan, rowSpan := int(cols), int(rows)
	for dc := 1; dc < colSpan; dc++ {
		if _, ok := b.pending[position{row, col + dc}]; ok {
			colSpan = dc
			break
		}
	}
clamp:
	for dr := 1; dr < rowSpan; dr++ {
		for dc := range colSpan {
			if _, ok := b.pending[position{row + dr, col + dc}]; ok {
				rowSpan = dr
				break clamp
			}
		}
	}
	cell.ColSpan, cell.RowSpan = colSpan, rowSpan
	b.grid[row] = append(b.grid[row], Slot{Cell: cell})
	if b.pending == nil {
		b.pending = map[position]position{}
	}
	for dr := range rowSpan {
		for dc := range colSpan {
			if dr != 0 || dc != 0 {
				b.pending[position{row + dr, col + dc}] = position{row, col}
			}
		}
	}
	return nil
}

// Covered consumes one explicitly written covered position. It reports false
// when no span accounts for the position; the stray marker then becomes an
// empty cell.
func (b *GridBuilder) Covered() bool {
	row := b.rowIndex()
	at := position{row, len(b.grid[row])}
	origin, ok := b.pending[at]
	if !ok {
		b.grid[row] = append(b.grid[row], emptySlot)
		return false
	}
	delete(b.pending, at)
	b.grid[row] = append(b.grid[row], Slot{Covered: true, OriginRow: origin.row, OriginCol: origin.col})
	return true
}

func (b *GridBuilder) Finish(kind TableKind) Table {
	byRow := map[int][]int{}
	for at := range b.pending {
		if at.row < len(b.grid) {
			byRow[at.row] = append(byRow[at.row], at.col)
		}
	}
	for row, cols := range byRow {
		slices.Sort(cols)
		for _, col := range cols {
			for len(b.grid[row]) < col {
				b.grid[row] = append(b.grid[row], emptySlot)
			}
			origin := b.pending[position{row, col}]
			b.grid[row] = append(b.grid[row], Slot{Covered: true, OriginRow: origin.row, OriginCol: origin.col})
		}
	}
	for len(b.grid) > 0 && b.trailingFiller(b.grid[len(b.grid)-1]) {
		b.grid = b.grid[:len(b.grid)-1]
	}
	for r, row := range b.grid {
		for c := range row {
			if cell := &row[c].Cell; !row[c].Covered {
				cell.RowSpan = min(cell.RowSpan, len(b.grid)-r)
				cell.ColSpan = min(cell.ColSpan, len(row)-c)
			}
		}
	}
	return Table{grid: b.grid, Kind: kind}
}

func (b *GridBuilder) trailingFiller(row []Slot) bool {
	for _, s := range row {
		if s.Covered && b.keepCoveredTail || !s.Covered && !s.Cell.IsEmpty() {
			return false
		}
	}
	return true
}

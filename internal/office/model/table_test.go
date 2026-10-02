package model

import (
	"errors"
	"math"
	"math/rand/v2"
	"slices"
	"testing"
)

func textCell(s string) Cell {
	return Cell{Blocks: []Block{Paragraph{Text{Text: s}}}}
}

func spanning(s string, cols, rows int) Cell {
	c := textCell(s)
	c.ColSpan, c.RowSpan = cols, rows
	return c
}

func widths(t Table) []int {
	var out []int
	for _, row := range t.Grid() {
		out = append(out, len(row))
	}
	return out
}

func mustPlace(t *testing.T, b *GridBuilder, c Cell) {
	t.Helper()
	if err := b.Place(c); err != nil {
		t.Fatal(err)
	}
}

func assertExactlyOnce(t *testing.T, tbl Table) {
	t.Helper()
	grid := tbl.Grid()
	for r, row := range grid {
		for c, slot := range row {
			if slot.Covered {
				if slot.OriginRow >= len(grid) || slot.OriginCol >= len(grid[slot.OriginRow]) {
					t.Fatalf("covered (%d,%d) points outside the grid", r, c)
				}
				origin := grid[slot.OriginRow][slot.OriginCol]
				if origin.Covered || r < slot.OriginRow || c < slot.OriginCol ||
					r >= slot.OriginRow+origin.Cell.RowSpan || c >= slot.OriginCol+origin.Cell.ColSpan {
					t.Fatalf("covered (%d,%d) not claimed by origin (%d,%d)", r, c, slot.OriginRow, slot.OriginCol)
				}
				continue
			}
			if slot.Cell.ColSpan < 1 || slot.Cell.RowSpan < 1 {
				t.Fatalf("origin (%d,%d) has span %dx%d", r, c, slot.Cell.ColSpan, slot.Cell.RowSpan)
			}
			for dr := range slot.Cell.RowSpan {
				for dc := range slot.Cell.ColSpan {
					if dr == 0 && dc == 0 {
						continue
					}
					rr, cc := r+dr, c+dc
					if rr >= len(grid) || cc >= len(grid[rr]) {
						t.Fatalf("span of origin (%d,%d) not backed at (%d,%d)", r, c, rr, cc)
					}
					s := grid[rr][cc]
					if !s.Covered || s.OriginRow != r || s.OriginCol != c {
						t.Fatalf("span of origin (%d,%d) not backed at (%d,%d)", r, c, rr, cc)
					}
				}
			}
		}
	}
}

func TestColSpanCoversPositions(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	mustPlace(t, &b, Cell{ColSpan: 2})
	mustPlace(t, &b, textCell("end"))
	tbl := b.Finish(DataTable)
	if got := widths(tbl); !slices.Equal(got, []int{3}) {
		t.Fatalf("widths %v", got)
	}
	if s := tbl.Grid()[0][1]; !s.Covered || s.OriginRow != 0 || s.OriginCol != 0 {
		t.Fatalf("slot %+v", s)
	}
}

func TestRowSpanSkipsNextRowPosition(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	mustPlace(t, &b, Cell{RowSpan: 2})
	mustPlace(t, &b, textCell("b1"))
	b.NextRow()
	mustPlace(t, &b, textCell("b2"))
	tbl := b.Finish(DataTable)
	if got := widths(tbl); !slices.Equal(got, []int{2, 2}) {
		t.Fatalf("widths %v", got)
	}
	if !tbl.Grid()[1][0].Covered || tbl.Grid()[1][1].Cell.IsEmpty() {
		t.Fatalf("grid %+v", tbl.Grid())
	}
}

func TestExplicitCoveredConsumesExactlyOne(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	mustPlace(t, &b, Cell{ColSpan: 2})
	if !b.Covered() {
		t.Fatal("covered position must be accounted for")
	}
	mustPlace(t, &b, textCell("end"))
	if got := widths(b.Finish(DataTable)); !slices.Equal(got, []int{3}) {
		t.Fatalf("widths %v", got)
	}
}

func TestCoveredRowConsumption(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	mustPlace(t, &b, Cell{RowSpan: 2})
	mustPlace(t, &b, textCell("b1"))
	b.NextRow()
	if !b.Covered() {
		t.Fatal("covered position must be accounted for")
	}
	mustPlace(t, &b, textCell("b2"))
	if got := widths(b.Finish(DataTable)); !slices.Equal(got, []int{2, 2}) {
		t.Fatalf("widths %v", got)
	}
}

func TestStrayCoveredBecomesEmptyCell(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	if b.Covered() {
		t.Fatal("stray covered marker must report false")
	}
	mustPlace(t, &b, textCell("x"))
	tbl := b.Finish(DataTable)
	if got := widths(tbl); !slices.Equal(got, []int{2}) {
		t.Fatalf("widths %v", got)
	}
	if tbl.Grid()[0][0].Covered {
		t.Fatal("stray marker must become an empty origin")
	}
}

func TestOverlappingSpansClampTheLateOrigin(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	mustPlace(t, &b, spanning("tall", 1, 3))
	mustPlace(t, &b, textCell("a"))
	b.NextRow()
	b.Covered()
	mustPlace(t, &b, spanning("wide", 2, 2))
	b.NextRow()
	b.Covered()
	mustPlace(t, &b, textCell("tail"))
	assertExactlyOnce(t, b.Finish(DataTable))
}

func TestConflictingSpanRectanglesStayConsistent(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	mustPlace(t, &b, spanning("block", 2, 2))
	b.NextRow()
	mustPlace(t, &b, spanning("late", 2, 1))
	tbl := b.Finish(DataTable)
	row := tbl.Grid()[1]
	if !row[0].Covered || !row[1].Covered || row[2].Covered || row[2].Cell.ColSpan != 2 {
		t.Fatalf("row %+v", row)
	}
	assertExactlyOnce(t, tbl)
}

func TestTrimmedRowsClampSurvivingSpans(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	mustPlace(t, &b, spanning("x", 1, 3))
	b.NextRow()
	b.NextRow()
	tbl := b.Finish(DataTable)
	if len(tbl.Grid()) != 1 || tbl.Grid()[0][0].Cell.RowSpan != 1 {
		t.Fatalf("grid %+v", tbl.Grid())
	}
	assertExactlyOnce(t, tbl)
}

func TestKeepCoveredTailRetainsMergeRows(t *testing.T) {
	var b GridBuilder
	b.KeepCoveredTail()
	b.NextRow()
	mustPlace(t, &b, spanning("x", 1, 3))
	b.NextRow()
	b.NextRow()
	tbl := b.Finish(DataTable)
	if len(tbl.Grid()) != 3 || tbl.Grid()[0][0].Cell.RowSpan != 3 {
		t.Fatalf("grid %+v", tbl.Grid())
	}
	assertExactlyOnce(t, tbl)
}

func TestCoveredTailBehindShortRowGapMaterializes(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	for range 5 {
		mustPlace(t, &b, textCell("h"))
	}
	mustPlace(t, &b, spanning("tall", 1, 2))
	b.NextRow()
	mustPlace(t, &b, textCell("only"))
	tbl := b.Finish(DataTable)
	if len(tbl.Grid()[1]) != 6 {
		t.Fatalf("widths %v", widths(tbl))
	}
	if s := tbl.Grid()[1][5]; !s.Covered || s.OriginRow != 0 || s.OriginCol != 5 {
		t.Fatalf("slot %+v", s)
	}
	assertExactlyOnce(t, tbl)
}

func TestTrailingEmptyRowsTrimmed(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	mustPlace(t, &b, textCell("x"))
	b.NextRow()
	mustPlace(t, &b, Cell{})
	if got := widths(b.Finish(DataTable)); !slices.Equal(got, []int{1}) {
		t.Fatalf("widths %v", got)
	}
}

func TestPlaceWithoutRowStartsOne(t *testing.T) {
	var b GridBuilder
	mustPlace(t, &b, textCell("x"))
	if got := widths(b.Finish(DataTable)); !slices.Equal(got, []int{1}) {
		t.Fatalf("widths %v", got)
	}
}

func TestHugeSpanHitsTheExpansionBudgetBeforeExpanding(t *testing.T) {
	var b GridBuilder
	b.NextRow()
	err := b.Place(Cell{ColSpan: math.MaxInt, RowSpan: math.MaxInt})
	var limit *LimitError
	if !errors.As(err, &limit) || limit.Limit != "max_expansion" {
		t.Fatalf("want max_expansion limit, got %v", err)
	}
}

func TestAccumulatedSpansHitTheExpansionBudget(t *testing.T) {
	var b GridBuilder
	b.expansion = MaxExpansion - 10
	b.NextRow()
	if err := b.Place(Cell{ColSpan: 3, RowSpan: 3}); err != nil {
		t.Fatalf("8 more positions stay within budget: %v", err)
	}
	var limit *LimitError
	if err := b.Place(Cell{ColSpan: 2, RowSpan: 2}); !errors.As(err, &limit) {
		t.Fatalf("3 more positions exceed budget, got %v", err)
	}
}

func TestTableFromRows(t *testing.T) {
	tbl := TableFromRows([][]Cell{
		{spanning("a", 5, 5), textCell("b")},
		{textCell("c")},
	}, 1, DataTable)
	if got := widths(tbl); !slices.Equal(got, []int{2, 1}) {
		t.Fatalf("spans must be ignored, widths %v", got)
	}
	if tbl.HeaderRows != 1 || tbl.Kind != DataTable {
		t.Fatalf("table %+v", tbl)
	}
	if !TableFromRows([][]Cell{{textCell("x")}}, 0, LayoutTable).IsSingleCell() {
		t.Fatal("1x1 table is a single cell")
	}
}

func runGridOps(b *GridBuilder, ops []byte) {
	for i := 0; i+2 < len(ops); i += 3 {
		switch ops[i] % 5 {
		case 0:
			b.NextRow()
		case 1:
			b.Covered()
		default:
			cell := Cell{ColSpan: int(ops[i+1]%6) - 1, RowSpan: int(ops[i+2]%6) - 1}
			if ops[i+1]%3 == 0 {
				cell.Blocks = []Block{Paragraph{Text{Text: "v"}}}
			}
			_ = b.Place(cell)
		}
	}
}

func TestAdversarialSpanLayoutsPlaceExactlyOnce(t *testing.T) {
	rng := rand.New(rand.NewPCG(13, 13))
	for range 3000 {
		ops := make([]byte, 3*rng.IntN(40))
		for i := range ops {
			ops[i] = byte(rng.UintN(256))
		}
		for _, keep := range []bool{false, true} {
			var b GridBuilder
			if keep {
				b.KeepCoveredTail()
			}
			runGridOps(&b, ops)
			assertExactlyOnce(t, b.Finish(DataTable))
		}
	}
}

func FuzzGridBuilder(f *testing.F) {
	f.Add([]byte{2, 5, 5, 0, 0, 0, 3, 2, 2, 1, 0, 0, 4, 1, 4})
	f.Add([]byte{4, 4, 1, 2, 1, 4, 0, 0, 0, 1, 0, 0, 2, 5, 5, 0, 0, 0, 0, 0, 0})
	f.Fuzz(func(t *testing.T, ops []byte) {
		var b GridBuilder
		runGridOps(&b, ops)
		assertExactlyOnce(t, b.Finish(DataTable))
	})
}

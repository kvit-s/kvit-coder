package pptx

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"io"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

const relsNS = "http://schemas.openxmlformats.org/package/2006/relationships"

func deck(t *testing.T, spTree string) []byte {
	t.Helper()
	parts := map[string]string{
		"ppt/presentation.xml": fmt.Sprintf(`<?xml version="1.0"?><p:presentation xmlns:p="%s" xmlns:r="%s"><p:sldIdLst><p:sldId id="256" r:id="rId1"/></p:sldIdLst></p:presentation>`,
			nsP, opc.NSRelationships),
		"ppt/_rels/presentation.xml.rels": fmt.Sprintf(`<?xml version="1.0"?><Relationships xmlns="%s"><Relationship Id="rId1" Type="%s" Target="slides/slide1.xml"/></Relationships>`,
			relsNS, opc.RelSlide),
		"ppt/slides/slide1.xml": fmt.Sprintf(`<?xml version="1.0"?><p:sld xmlns:p="%s" xmlns:a="%s"><p:cSld><p:spTree>%s</p:spTree></p:cSld></p:sld>`,
			nsP, opc.NSDrawingML, spTree),
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	slices.Sort(names)
	for _, name := range names {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.WriteString(f, parts[name]); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func frame(tbl string) string {
	return `<p:graphicFrame><a:graphic><a:graphicData>` + tbl + `</a:graphicData></a:graphic></p:graphicFrame>`
}

func tc(attrs, text string) string {
	return `<a:tc` + attrs + `><a:txBody><a:p><a:r><a:t>` + text + `</a:t></a:r></a:p></a:txBody></a:tc>`
}

func tables(doc *model.Document) []model.Table {
	var out []model.Table
	for _, b := range doc.Blocks {
		if t, ok := b.(model.Table); ok {
			out = append(out, t)
		}
	}
	return out
}

type slotShape struct {
	Covered          bool
	ColSpan, RowSpan int
}

func shape(t model.Table) [][]slotShape {
	var out [][]slotShape
	for _, row := range t.Grid() {
		var cells []slotShape
		for _, s := range row {
			if s.Covered {
				cells = append(cells, slotShape{Covered: true})
				continue
			}
			cells = append(cells, slotShape{ColSpan: s.Cell.ColSpan, RowSpan: s.Cell.RowSpan})
		}
		out = append(out, cells)
	}
	return out
}

func TestTableSpansClampToTheDeclaredGrid(t *testing.T) {
	grid := `<a:tblGrid><a:gridCol w="1"/><a:gridCol w="1"/><a:gridCol w="1"/></a:tblGrid>`
	origin := slotShape{ColSpan: 1, RowSpan: 1}
	covered := slotShape{Covered: true}
	tests := []struct {
		name string
		tbl  string
		want [][]slotShape
	}{
		{
			name: "legitimate merge is kept",
			tbl: `<a:tbl>` + grid +
				`<a:tr>` + tc(` gridSpan="2" rowSpan="2"`, "a") + tc(` hMerge="1"`, "") + tc("", "b") + `</a:tr>` +
				`<a:tr>` + tc(` vMerge="1"`, "") + tc(` hMerge="1" vMerge="1"`, "") + tc("", "c") + `</a:tr></a:tbl>`,
			want: [][]slotShape{
				{{ColSpan: 2, RowSpan: 2}, covered, origin},
				{covered, covered, origin},
			},
		},
		{
			name: "huge gridSpan clamps to the grid",
			tbl:  `<a:tbl>` + grid + `<a:tr>` + tc(` gridSpan="3999999"`, "a") + `</a:tr><a:tr>` + tc("", "b") + tc("", "c") + tc("", "d") + `</a:tr></a:tbl>`,
			want: [][]slotShape{
				{{ColSpan: 3, RowSpan: 1}, covered, covered},
				{origin, origin, origin},
			},
		},
		{
			name: "huge rowSpan clamps to the remaining rows",
			tbl:  `<a:tbl>` + grid + `<a:tr>` + tc(` rowSpan="3999999"`, "a") + tc("", "b") + `</a:tr><a:tr>` + tc(` vMerge="1"`, "") + tc("", "c") + `</a:tr></a:tbl>`,
			want: [][]slotShape{
				{{ColSpan: 1, RowSpan: 2}, origin},
				{covered, origin},
			},
		},
		{
			name: "missing grid clamps to the row's cell count",
			tbl:  `<a:tbl><a:tr>` + tc(` gridSpan="3999999"`, "a") + tc("", "b") + `</a:tr></a:tbl>`,
			want: [][]slotShape{
				{{ColSpan: 2, RowSpan: 1}, covered, origin},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			doc, err := Parse(deck(t, frame(tt.tbl)))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			got := tables(doc)
			if len(got) != 1 {
				t.Fatalf("got %d tables, want 1", len(got))
			}
			if got := shape(got[0]); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("grid = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestHugeGridSpansStayCheap(t *testing.T) {
	tbl := `<a:tbl><a:tblGrid><a:gridCol w="1"/><a:gridCol w="1"/></a:tblGrid><a:tr>` + tc(` gridSpan="3999999"`, "x") + `</a:tr></a:tbl>`
	data := deck(t, strings.Repeat(frame(tbl), 8))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	doc, err := Parse(data)
	runtime.ReadMemStats(&after)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if n := len(tables(doc)); n != 8 {
		t.Errorf("got %d tables, want 8", n)
	}
	if alloc := after.TotalAlloc - before.TotalAlloc; alloc > 16<<20 {
		t.Errorf("Parse allocated %d bytes, want under 16 MiB", alloc)
	}
}

func TestExpansionBudgetSpansThePresentation(t *testing.T) {
	root, err := opc.ParseXML([]byte(fmt.Sprintf(`<a:tbl xmlns:a="%s"><a:tblGrid><a:gridCol/><a:gridCol/><a:gridCol/><a:gridCol/></a:tblGrid><a:tr>%s</a:tr><a:tr>%s</a:tr></a:tbl>`,
		opc.NSDrawingML, tc(` gridSpan="4" rowSpan="2"`, "a"), tc(` hMerge="1"`, ""))))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		spent uint64
		limit bool
	}{
		{"fits in the remaining budget", model.MaxExpansion - 7, false},
		{"exceeds the remaining budget", model.MaxExpansion - 6, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := &slide{c: &converter{expansion: tt.spent}}
			_, err := s.table(root, nil)
			var limit *model.LimitError
			if got := errors.As(err, &limit) && limit.Limit == "max_expansion"; got != tt.limit {
				t.Errorf("limit error = %v (err %v), want %v", got, err, tt.limit)
			}
		})
	}
}

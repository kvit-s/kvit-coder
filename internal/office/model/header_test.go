package model

import (
	"strings"
	"testing"
)

func detectHeader(rows ...[]string) int {
	cells := make([][]Cell, len(rows))
	for i, row := range rows {
		for _, text := range row {
			cells[i] = append(cells[i], Cell{Blocks: []Block{Paragraph{Text{Text: text}}}})
		}
	}
	return ResolveHeaderRows(TableFromRows(cells, 0, DataTable), 0)
}

func TestResolveHeaderRows(t *testing.T) {
	cases := []struct {
		name string
		rows [][]string
		want int
	}{
		{"labels over numbers", [][]string{{"name", "qty"}, {"a", "1"}, {"b", "2"}}, 1},
		{"labels over dates and booleans", [][]string{{"when", "ok"}, {"2026-03-15", "TRUE"}, {"2026-03-16", "FALSE"}}, 1},
		{"numeric first row is data", [][]string{{"36", "12", "aka"}, {"173", "57", "aka"}, {"306", "220", "aka"}}, 0},
		{"label repeated below is data", [][]string{{"x", "aka"}, {"y", "aka"}, {"z", "aka"}}, 0},
		{"text over text reads row shape", [][]string{{"col1", "col2"}, {"naïve", "café"}, {"Αθήνα", "数据"}}, 1},
		{"long label is prose", [][]string{{strings.Repeat("l", maxLabelRunes+1), "col2"}, {"a", "b"}, {"c", "d"}}, 0},
		{"title line narrower than body", [][]string{{"Q1 report"}, {"a", "1"}, {"b", "2"}}, 0},
		{"unlabelled column", [][]string{{"name", ""}, {"a", "1"}, {"b", "2"}}, 0},
		{"duplicate labels", [][]string{{"name", "name"}, {"a", "1"}, {"b", "2"}}, 0},
		{"single row", [][]string{{"name", "qty"}}, 0},
		{"unlabelled index column", [][]string{{"", "qty"}, {"a", "1"}, {"b", "2"}}, 1},
		{"mixed column abstains", [][]string{{"kind", "value"}, {"percent", "15.5%"}, {"date", "2026-03-15"}}, 1},
	}
	for _, c := range cases {
		if got := detectHeader(c.rows...); got != c.want {
			t.Errorf("%s: got %d, want %d", c.name, got, c.want)
		}
	}
	one := []Cell{{Blocks: []Block{Paragraph{Text{Text: "1"}}}}}
	if got := ResolveHeaderRows(TableFromRows([][]Cell{one, one}, 0, DataTable), 5); got != 2 {
		t.Errorf("declared header rows cap at the row count, got %d", got)
	}
}

func TestClassifyValues(t *testing.T) {
	cases := map[string]valueKind{
		"1,234.56":                 kindNumber,
		"-1.5e3":                   kindNumber,
		"15.5%":                    kindNumber,
		"-0-":                      kindText,
		"NaN":                      kindText,
		"0x1p3":                    kindText,
		"2026-03-15":               kindDate,
		"15/03/2026 09:04":         kindDate,
		"2026-03-15T09:04:54.123Z": kindDate,
		"26:30:15":                 kindDate,
		"yes":                      kindBool,
		"  ":                       kindBlank,
	}
	for value, want := range cases {
		if got := classify(value); got != want {
			t.Errorf("classify(%q) = %v, want %v", value, got, want)
		}
	}
}

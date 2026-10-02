// Package xlsx converts SpreadsheetML workbooks (.xlsx, .xlsm) to the
// document model: visible sheets in order, each a table of cell values
// formatted through their number formats, with hidden rows, columns and
// sheets omitted and merges remapped onto the surviving grid.
package xlsx

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/formats/ooxml"
	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

const (
	NSSpreadsheetML = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"

	maxRows = 1_048_576
	maxCols = 16_384
)

func malformed(detail string) error {
	return &opc.MalformedError{Detail: detail}
}

// Parse converts a workbook package. Crossing model.MaxGridSlots across the
// workbook, or model.MaxExpansion, is a *model.LimitError.
func Parse(data []byte) (*model.Document, error) {
	pkg, err := opc.Open(data)
	var bad *opc.MalformedError
	switch {
	case errors.As(err, &bad):
		return nil, malformed("not a readable workbook container")
	case err != nil:
		return nil, err
	}
	wbPart, err := mainPart(pkg)
	if err != nil {
		return nil, err
	}
	body, err := pkg.Part(wbPart)
	var missing *opc.MissingPartError
	switch {
	case errors.As(err, &missing):
		return nil, malformed("not a readable workbook container")
	case err != nil:
		return nil, err
	}
	body = bytes.TrimPrefix(body, []byte{0xEF, 0xBB, 0xBF})
	if first := bytes.TrimLeft(body, " \t\r\n\f"); len(first) == 0 {
		return nil, malformed("not a readable workbook container")
	} else if first[0] != '<' && first[0] != 0xFF && first[0] != 0xFE {
		return nil, malformed("binary workbook containers are not supported")
	}
	return parseWorkbook(pkg, wbPart)
}

// mainPart prefers the root relationship over the conventional location,
// which a package may also hold as a leftover.
func mainPart(pkg *opc.Package) (string, error) {
	rels, err := pkg.Rels("")
	if err != nil {
		return "", err
	}
	if rel, ok := rels.FirstOfType(opc.RelOfficeDocument); ok {
		if t, err := opc.Resolve("", rel.Target); err == nil {
			return t.Path, nil
		}
	}
	for _, name := range []string{"xl/workbook.xml", "xl/workbook.bin"} {
		if pkg.Has(name) {
			return name, nil
		}
	}
	return "", malformed("not a readable workbook container")
}

func parseWorkbook(pkg *opc.Package, wbPart string) (*model.Document, error) {
	workbook, err := pkg.XML(wbPart)
	if err != nil {
		return nil, err
	}
	if !workbook.Is(NSSpreadsheetML, "workbook") {
		return nil, malformed("main part is not a workbook")
	}
	rels, err := pkg.Rels(wbPart)
	if err != nil {
		return nil, err
	}
	date1904 := false
	for pr := range workbook.Descendants(NSSpreadsheetML, "workbookPr") {
		v, _ := pr.QualifiedAttr("", "date1904")
		date1904 = boolAttr(v)
		break
	}
	sst, err := pkg.OptionalXML(rels.PartOfType(opc.RelSharedStrings, "sharedStrings.xml"))
	if err != nil {
		return nil, err
	}
	shared := sharedStrings(sst)
	stylesRoot, err := pkg.OptionalXML(rels.PartOfType(opc.RelStyles, "styles.xml"))
	if err != nil {
		return nil, err
	}
	xfs := readStyles(stylesRoot)

	type sheetRef struct{ name, part string }
	var sheets []sheetRef
	for list := range workbook.Descendants(NSSpreadsheetML, "sheets") {
		for sheet := range list.Children(NSSpreadsheetML, "sheet") {
			if state, _ := sheet.QualifiedAttr("", "state"); state == "hidden" || state == "veryHidden" {
				continue
			}
			name, _ := sheet.QualifiedAttr("", "name")
			name = ooxml.CleanText(name)
			id, _ := sheet.QualifiedAttr(opc.NSRelationships, "id")
			part, ok := rels.PartPath(id)
			if !ok {
				model.Log.Warn("skipping sheet with no worksheet relationship", "sheet", name)
				continue
			}
			sheets = append(sheets, sheetRef{name, part})
		}
		break
	}

	doc := &model.Document{}
	failed := 0
	var slots uint64
	for _, s := range sheets {
		root, err := pkg.OptionalXML(s.part)
		if err != nil {
			return nil, err
		}
		if root == nil || !root.Is(NSSpreadsheetML, "worksheet") {
			model.Log.Warn("skipping unreadable sheet", "sheet", s.name)
			failed++
			continue
		}
		content := readSheet(root, shared, xfs, date1904)
		if content.checkboxes, err = readVMLCheckboxes(pkg, s.part); err != nil {
			return nil, err
		}
		table, ok, err := buildTable(content, &slots)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if len(sheets) > 1 {
			doc.Blocks = append(doc.Blocks, model.Heading{Level: 2, Content: []model.Inline{model.Text{Text: s.name}}})
		}
		doc.Blocks = append(doc.Blocks, table)
	}
	if len(sheets) > 0 && failed == len(sheets) {
		return nil, malformed("no sheet in the workbook could be read")
	}
	return doc, nil
}

func sharedStrings(root *opc.Element) []string {
	if root == nil || !root.Is(NSSpreadsheetML, "sst") {
		return nil
	}
	var out []string
	for si := range root.Children(NSSpreadsheetML, "si") {
		out = append(out, ooxml.CleanText(richText(si)))
	}
	return out
}

// richText concatenates a single t or rich-text r runs; phonetic guides
// (rPh) are not content.
func richText(item *opc.Element) string {
	var sb strings.Builder
	for child := range item.Elements() {
		switch {
		case child.Is(NSSpreadsheetML, "t"):
			sb.WriteString(child.Text())
		case child.Is(NSSpreadsheetML, "r"):
			if t := child.Child(NSSpreadsheetML, "t"); t != nil {
				sb.WriteString(t.Text())
			}
		}
	}
	return sb.String()
}

// readStyles resolves each cellXfs entry to its number format; nil is
// General.
func readStyles(root *opc.Element) []*numberFormat {
	if root == nil {
		return nil
	}
	custom := map[int]string{}
	for fmts := range root.Descendants(NSSpreadsheetML, "numFmts") {
		for nf := range fmts.Children(NSSpreadsheetML, "numFmt") {
			idText, _ := nf.QualifiedAttr("", "numFmtId")
			code, hasCode := nf.QualifiedAttr("", "formatCode")
			if id, ok := parseUint(idText, 32); ok && hasCode {
				custom[int(id)] = code
			}
		}
	}
	var xfs []*numberFormat
	cache := map[int]*numberFormat{}
	for list := range root.Descendants(NSSpreadsheetML, "cellXfs") {
		for xf := range list.Children(NSSpreadsheetML, "xf") {
			idText, _ := xf.QualifiedAttr("", "numFmtId")
			n, _ := parseUint(idText, 32)
			id := int(n)
			f, ok := cache[id]
			if !ok {
				f = resolveFormat(id, custom)
				cache[id] = f
			}
			xfs = append(xfs, f)
		}
		break
	}
	return xfs
}

// resolveFormat looks up the file's own numFmt first, then the built-in
// table; unknown ids and unsupported codes render General, never a guess.
func resolveFormat(id int, custom map[int]string) *numberFormat {
	code, ok := custom[id]
	if !ok {
		code = builtinCode(id)
	}
	if code == "" {
		return nil
	}
	f := parseNumberFormat(code)
	if f == nil {
		model.Log.Debug("unsupported number format, rendering as General", "code", code)
	}
	return f
}

type cellPos struct{ row, col int }

type mergeRegion struct{ r1, c1, r2, c2 int }

type colRange struct{ lo, hi int }

type sheetContent struct {
	cells      map[cellPos]string
	checkboxes map[cellPos][]checkbox
	hiddenRows map[int]bool
	hiddenCols []colRange
	merges     []mergeRegion
}

func readSheet(ws *opc.Element, shared []string, xfs []*numberFormat, date1904 bool) sheetContent {
	out := sheetContent{cells: map[cellPos]string{}, hiddenRows: map[int]bool{}}
	for cols := range ws.Children(NSSpreadsheetML, "cols") {
		for col := range cols.Children(NSSpreadsheetML, "col") {
			if hidden, _ := col.QualifiedAttr("", "hidden"); !boolAttr(hidden) {
				continue
			}
			lo, loOK := oneBased(col, "min")
			hi, hiOK := oneBased(col, "max")
			if loOK && hiOK && lo <= hi {
				out.hiddenCols = append(out.hiddenCols, colRange{lo, min(hi, maxCols-1)})
			}
		}
	}
	nextRow := 0
	for sheetData := range ws.Children(NSSpreadsheetML, "sheetData") {
		for row := range sheetData.Children(NSSpreadsheetML, "row") {
			r, ok := oneBased(row, "r")
			if !ok {
				r = nextRow
			}
			if r >= maxRows {
				continue
			}
			nextRow = r + 1
			if hidden, _ := row.QualifiedAttr("", "hidden"); boolAttr(hidden) {
				out.hiddenRows[r] = true
			}
			nextCol := 0
			for c := range row.Children(NSSpreadsheetML, "c") {
				at := cellPos{r, nextCol}
				if ref, ok := c.QualifiedAttr("", "r"); ok {
					if at, ok = parseRef(ref); !ok {
						continue
					}
				}
				nextCol = at.col + 1
				if at.row >= maxRows || at.col >= maxCols {
					continue
				}
				if text := cellText(c, shared, xfs, date1904); text != "" {
					out.cells[at] = text
				}
			}
		}
	}
	for mergeCells := range ws.Children(NSSpreadsheetML, "mergeCells") {
		for merge := range mergeCells.Children(NSSpreadsheetML, "mergeCell") {
			ref, _ := merge.QualifiedAttr("", "ref")
			m, ok := parseRegion(ref)
			if !ok {
				model.Log.Debug("skipping unparseable merge reference", "ref", ref)
				continue
			}
			if m.r1 != m.r2 || m.c1 != m.c2 {
				out.merges = append(out.merges, m)
			}
		}
	}
	return out
}

func oneBased(e *opc.Element, attr string) (int, bool) {
	v, _ := e.QualifiedAttr("", attr)
	n, ok := parseUint(v, 32)
	if !ok || n == 0 {
		return 0, false
	}
	return int(n) - 1, true
}

func cellText(c *opc.Element, shared []string, xfs []*numberFormat, date1904 bool) string {
	var f *numberFormat
	sAttr, _ := c.QualifiedAttr("", "s")
	if i, _ := parseUint(sAttr, 64); i < uint64(len(xfs)) {
		f = xfs[i]
	}
	value := func() string {
		if v := c.Child(NSSpreadsheetML, "v"); v != nil {
			return v.Text()
		}
		return ""
	}
	t, ok := c.QualifiedAttr("", "t")
	if !ok {
		t = "n"
	}
	switch t {
	case "s":
		i, ok := parseUint(strings.TrimSpace(value()), 64)
		if !ok || i >= uint64(len(shared)) {
			model.Log.Debug("shared string index out of range", "index", value())
			return ""
		}
		return formatAsText(f, shared[i])
	case "str":
		return formatAsText(f, ooxml.CleanText(value()))
	case "inlineStr":
		text := ""
		if is := c.Child(NSSpreadsheetML, "is"); is != nil {
			text = ooxml.CleanText(richText(is))
		}
		return formatAsText(f, text)
	case "b":
		switch strings.TrimSpace(value()) {
		case "1", "true":
			return "TRUE"
		case "0", "false":
			return "FALSE"
		}
		return ""
	case "e", "d":
		return ooxml.CleanText(value())
	}
	v := strings.TrimSpace(value())
	if v == "" {
		return ""
	}
	n, ok := parseFloat(v)
	if !ok {
		model.Log.Debug("unparseable numeric cell value", "value", v)
		return ""
	}
	return renderNumber(f, n, date1904)
}

func renderNumber(f *numberFormat, n float64, date1904 bool) string {
	if f == nil {
		return ooxml.CleanText(formatFloat(n))
	}
	r := f.formatNumber(n)
	switch r.kind {
	case renderedText:
		return ooxml.CleanText(r.text)
	case renderedDateTime:
		return ooxml.CleanText(renderSerial(n, r.parts, date1904))
	}
	return ooxml.CleanText(r.prefix + formatFloat(r.value) + r.suffix)
}

func formatAsText(f *numberFormat, text string) string {
	if f == nil {
		return text
	}
	if s, ok := f.formatText(text); ok {
		return ooxml.CleanText(s)
	}
	return text
}

// buildTable materializes a sheet: visibility filtering, the populated
// extent widened by intersecting merges (a merge anchored on the only
// populated cell survives at full size), and merges remapped onto the
// surviving rows and columns. slots accumulates across the workbook.
func buildTable(sheet sheetContent, slots *uint64) (model.Table, bool, error) {
	hiddenRows := make([]int, 0, len(sheet.hiddenRows))
	for r := range sheet.hiddenRows {
		hiddenRows = append(hiddenRows, r)
	}
	slices.Sort(hiddenRows)
	hiddenCols := expandRanges(sheet.hiddenCols)
	isHidden := func(sorted []int, v int) bool {
		_, found := slices.BinarySearch(sorted, v)
		return found
	}

	cells := make(map[cellPos][]model.Inline, len(sheet.cells))
	for at, boxes := range sheet.checkboxes {
		if at.row < maxRows && at.col < maxCols {
			text, hasText := sheet.cells[at]
			delete(sheet.cells, at)
			cells[at] = cellInlines(text, hasText, boxes)
		}
	}
	for at, text := range sheet.cells {
		cells[at] = []model.Inline{model.Text{Text: text}}
	}
	merges := sheet.merges[:0]
	for _, m := range sheet.merges {
		vr, rowOK := firstVisible(hiddenRows, m.r1, m.r2)
		vc, colOK := firstVisible(hiddenCols, m.c1, m.c2)
		if !rowOK || !colOK {
			continue
		}
		if origin := (cellPos{m.r1, m.c1}); (cellPos{vr, vc}) != origin {
			if inlines, ok := cells[origin]; ok {
				delete(cells, origin)
				cells[cellPos{vr, vc}] = inlines
			}
		}
		merges = append(merges, m)
	}

	found := false
	var r1, c1, r2, c2 int
	for at := range cells {
		if isHidden(hiddenRows, at.row) || isHidden(hiddenCols, at.col) {
			continue
		}
		if !found {
			r1, c1, r2, c2, found = at.row, at.col, at.row, at.col, true
			continue
		}
		r1, c1, r2, c2 = min(r1, at.row), min(c1, at.col), max(r2, at.row), max(c2, at.col)
	}
	if !found {
		return model.Table{}, false, nil
	}
	merges = slices.DeleteFunc(merges, func(m mergeRegion) bool {
		return m.r1 > r2 || m.r2 < r1 || m.c1 > c2 || m.c2 < c1
	})
	for _, m := range merges {
		r1, c1, r2, c2 = min(r1, m.r1), min(c1, m.c1), max(r2, m.r2), max(c2, m.c2)
	}

	var rowMap, colMap []int
	for r := r1; r <= r2; r++ {
		if !isHidden(hiddenRows, r) {
			rowMap = append(rowMap, r)
		}
	}
	for c := c1; c <= c2; c++ {
		if !isHidden(hiddenCols, c) {
			colMap = append(colMap, c)
		}
	}
	if len(rowMap) == 0 || len(colMap) == 0 {
		return model.Table{}, false, nil
	}
	*slots += uint64(len(rowMap)) * uint64(len(colMap))
	if *slots > model.MaxGridSlots {
		return model.Table{}, false, &model.LimitError{Limit: "max_grid_slots", Detail: "workbook extent covers " + strconv.FormatUint(*slots, 10) + " grid positions"}
	}

	visibleSpan := func(sorted []int, lo, hi int) (int, int) {
		a, _ := slices.BinarySearch(sorted, lo)
		b, _ := slices.BinarySearch(sorted, hi+1)
		return a, b - a
	}
	type span struct{ cols, rows int }
	origins := map[cellPos]span{}
	covered := map[cellPos]bool{}
	var expansion uint64
	for _, m := range merges {
		r0, rn := visibleSpan(rowMap, m.r1, m.r2)
		c0, cn := visibleSpan(colMap, m.c1, m.c2)
		if rn*cn <= 1 {
			continue
		}
		expansion += uint64(rn)*uint64(cn) - 1
		if expansion > model.MaxExpansion {
			return model.Table{}, false, &model.LimitError{Limit: "max_expansion", Detail: "merge region expansion exceeds the content budget"}
		}
		origins[cellPos{r0, c0}] = span{cn, rn}
		for r := r0; r < r0+rn; r++ {
			for c := c0; c < c0+cn; c++ {
				if r != r0 || c != c0 {
					covered[cellPos{r, c}] = true
				}
			}
		}
	}

	var b model.GridBuilder
	b.KeepCoveredTail()
	for ri, row := range rowMap {
		b.NextRow()
		for ci, col := range colMap {
			if covered[cellPos{ri, ci}] {
				b.Covered()
				continue
			}
			var cell model.Cell
			if inlines, ok := cells[cellPos{row, col}]; ok {
				cell.Blocks = []model.Block{model.Paragraph(inlines)}
			}
			if s, ok := origins[cellPos{ri, ci}]; ok {
				cell.ColSpan, cell.RowSpan = s.cols, s.rows
			}
			if err := b.Place(cell); err != nil {
				return model.Table{}, false, err
			}
		}
	}
	table := b.Finish(model.DataTable)
	if len(table.Grid()) == 0 {
		return model.Table{}, false, nil
	}
	table.HeaderRows = model.ResolveHeaderRows(table, 0)
	return table, true, nil
}

// expandRanges coalesces before expanding, so the output is bounded by the
// coordinate space rather than the range count.
func expandRanges(ranges []colRange) []int {
	slices.SortFunc(ranges, func(a, b colRange) int {
		if a.lo != b.lo {
			return a.lo - b.lo
		}
		return a.hi - b.hi
	})
	var out []int
	next := 0
	for _, r := range ranges {
		for v := max(r.lo, next); v <= r.hi; v++ {
			out = append(out, v)
		}
		next = max(next, r.hi+1)
	}
	return out
}

// firstVisible measures the hidden run starting at lo by binary search, since
// hidden[j]-j never decreases along the run, so a long run costs no linear
// scan per query.
func firstVisible(hidden []int, lo, hi int) (int, bool) {
	start, _ := slices.BinarySearch(hidden, lo)
	tail := hidden[start:]
	n := sort.Search(len(tail), func(j int) bool { return tail[j] != lo+j })
	first := lo + n
	return first, first <= hi
}

// renderSerial keeps the ISO-like output: elapsed formats as a duration,
// sub-day serials as a time of day, everything else as a date with a
// midnight time omitted.
func renderSerial(serial float64, parts dateParts, date1904 bool) string {
	switch {
	case math.IsInf(serial, 0) || math.IsNaN(serial):
		return formatFloat(serial)
	case parts.elapsed:
		return formatDurationDays(serial)
	case !parts.date:
		_, frac := math.Modf(serial)
		return formatTimeOfDay(frac)
	case math.Abs(serial) < 1:
		if parts.time {
			return formatTimeOfDay(serial)
		}
		return formatFloat(serial)
	case serial < 0 || serial >= 2_958_466:
		return formatFloat(serial)
	}
	whole, frac := math.Modf(serial)
	days := int64(whole)
	// Serial 60 is the fictitious 1900-02-29; it keeps its number rather than
	// collapsing onto a real date. Tested before the seconds carry, so a
	// value late on serial 59 still resolves to its own day.
	if !date1904 && days == 60 {
		return formatFloat(serial)
	}
	secs := int64(math.Round(frac * 86_400))
	if secs >= 86_400 {
		secs = 0
		days++
	}
	var civil int64
	if date1904 {
		civil = days + daysFromCivil(1904, 1, 1)
	} else {
		if days >= 60 {
			days--
		}
		civil = days + daysFromCivil(1899, 12, 31)
	}
	y, m, d := civilFromDays(civil)
	if y < 1 || y > 9999 {
		return formatFloat(serial)
	}
	out := fmt.Sprintf("%04d-%02d-%02d", y, m, d)
	if parts.time && secs != 0 {
		out += fmt.Sprintf(" %02d:%02d:%02d", secs/3600, secs%3600/60, secs%60)
	}
	return out
}

func daysFromCivil(y int64, m, d int) int64 {
	if m <= 2 {
		y--
	}
	era := floorDiv(y, 400)
	yoe := y - era*400
	mp := int64((m + 9) % 12)
	doy := (153*mp+2)/5 + int64(d) - 1
	doe := yoe*365 + yoe/4 - yoe/100 + doy
	return era*146_097 + doe - 719_468
}

func civilFromDays(z int64) (int64, int, int) {
	z += 719_468
	era := floorDiv(z, 146_097)
	doe := z - era*146_097
	yoe := (doe - doe/1460 + doe/36_524 - doe/146_096) / 365
	y := yoe + era*400
	doy := doe - (365*yoe + yoe/4 - yoe/100)
	mp := (5*doy + 2) / 153
	d := int(doy - (153*mp+2)/5 + 1)
	m := int(mp + 3)
	if mp >= 10 {
		m = int(mp - 9)
	}
	if m <= 2 {
		y++
	}
	return y, m, d
}

// formatFloat renders at the 15 significant digits a spreadsheet stores:
// shortest round-trip formatting past that surfaces the binary
// representation (3554.7000000000003).
func formatFloat(f float64) string {
	switch {
	case math.IsNaN(f):
		return "NaN"
	case math.IsInf(f, 1):
		return "inf"
	case math.IsInf(f, -1):
		return "-inf"
	}
	rounded, _ := strconv.ParseFloat(strconv.FormatFloat(f, 'e', 14, 64), 64)
	return strconv.FormatFloat(rounded, 'f', -1, 64)
}

func formatTimeOfDay(days float64) string {
	total := roundSeconds(math.Abs(days)*86_400) % 86_400
	return fmt.Sprintf("%02d:%02d:%02d", total/3600, total%3600/60, total%60)
}

func formatDurationDays(days float64) string {
	sign := ""
	if days < 0 {
		sign = "-"
	}
	total := roundSeconds(math.Abs(days) * 86_400)
	return fmt.Sprintf("%s%d:%02d:%02d", sign, total/3600, total%3600/60, total%60)
}

// roundSeconds saturates rather than wrapping on durations past the range.
func roundSeconds(secs float64) uint64 {
	r := math.Round(secs)
	if r >= math.MaxUint64 {
		return math.MaxUint64
	}
	return uint64(r)
}

// parseRef reads a cell reference (C3) as a zero-based position; column
// letters are bijective base-26.
func parseRef(ref string) (cellPos, bool) {
	digitsAt := strings.IndexFunc(ref, func(r rune) bool { return r >= '0' && r <= '9' })
	if digitsAt <= 0 {
		return cellPos{}, false
	}
	col := 0
	for _, ch := range ref[:digitsAt] {
		ch = asciiLower(ch)
		if ch < 'a' || ch > 'z' {
			return cellPos{}, false
		}
		if col = col*26 + int(ch-'a') + 1; col > maxCols {
			return cellPos{}, false
		}
	}
	row, ok := parseUint(ref[digitsAt:], 32)
	if !ok || row < 1 || row > maxRows {
		return cellPos{}, false
	}
	return cellPos{int(row) - 1, col - 1}, true
}

func parseRegion(ref string) (mergeRegion, bool) {
	a, b, found := strings.Cut(ref, ":")
	if !found {
		b = a
	}
	p, okA := parseRef(strings.TrimSpace(a))
	q, okB := parseRef(strings.TrimSpace(b))
	if !okA || !okB {
		return mergeRegion{}, false
	}
	return mergeRegion{min(p.row, q.row), min(p.col, q.col), max(p.row, q.row), max(p.col, q.col)}, true
}

func boolAttr(v string) bool {
	v = strings.TrimSpace(v)
	return v == "1" || v == "true"
}

func parseFloat(s string) (float64, bool) {
	if strings.ContainsAny(s, "xX_pP") {
		return 0, false
	}
	f, err := strconv.ParseFloat(s, 64)
	return f, err == nil || errors.Is(err, strconv.ErrRange)
}

// parseUint accepts a leading plus sign and yields 0 on any failure, the
// fallback every caller wants for an unreadable index.
func parseUint(s string, bitSize int) (uint64, bool) {
	if s == "" {
		return 0, false
	}
	n, err := strconv.ParseUint(strings.TrimPrefix(s, "+"), 10, bitSize)
	if err != nil {
		return 0, false
	}
	return n, true
}

package xlsx

import (
	"slices"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/office/formats/ooxml"
	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

const (
	relVMLDrawing = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/vmlDrawing"
	nsVML         = "urn:schemas-microsoft-com:vml"
	nsExcelVML    = "urn:schemas-microsoft-com:office:excel"
)

type checkbox struct {
	checked bool
	caption string
}

// cellInlines puts a cell's own text first, then each anchored checkbox with
// its caption.
func cellInlines(text string, hasText bool, boxes []checkbox) []model.Inline {
	var out []model.Inline
	if hasText {
		out = append(out, model.Text{Text: text})
	}
	for _, b := range boxes {
		if len(out) > 0 {
			out = append(out, model.Text{Text: " "})
		}
		out = append(out, model.Checkbox(b.checked))
		if b.caption != "" {
			out = append(out, model.Text{Text: " " + b.caption})
		}
	}
	return out
}

// readVMLCheckboxes collects form control checkboxes from the worksheet's
// legacy drawings, keyed by the cell each anchor starts in.
func readVMLCheckboxes(pkg *opc.Package, sheetPart string) (map[cellPos][]checkbox, error) {
	rels, err := pkg.Rels(sheetPart)
	if err != nil {
		return nil, err
	}
	var targets []string
	for _, rel := range rels.List {
		if rel.Type != relVMLDrawing || rel.External {
			continue
		}
		if t, err := opc.Resolve(sheetPart, rel.Target); err == nil {
			targets = append(targets, t.Path)
		}
	}
	slices.Sort(targets)
	out := map[cellPos][]checkbox{}
	for _, target := range slices.Compact(targets) {
		root, err := pkg.OptionalXML(target)
		if err != nil {
			return nil, err
		}
		if root != nil {
			vmlCheckboxes(root, out)
		}
	}
	return out, nil
}

func vmlCheckboxes(root *opc.Element, out map[cellPos][]checkbox) {
	for shape := range root.Descendants(nsVML, "shape") {
		data := shape.Child(nsExcelVML, "ClientData")
		if data == nil {
			continue
		}
		if kind, _ := data.QualifiedAttr("", "ObjectType"); kind != "Checkbox" {
			continue
		}
		if style, _ := shape.QualifiedAttr("", "style"); strings.Contains(strings.ReplaceAll(style, " ", ""), "visibility:hidden") {
			continue
		}
		anchor := data.Child(nsExcelVML, "Anchor")
		if anchor == nil {
			model.Log.Debug("skipping a checkbox with no readable anchor")
			continue
		}
		at, ok := anchorCell(anchor.Text())
		if !ok {
			model.Log.Debug("skipping a checkbox with no readable anchor")
			continue
		}
		checked := false
		if c := data.Child(nsExcelVML, "Checked"); c != nil {
			switch strings.TrimSpace(c.Text()) {
			case "0":
			case "1":
				checked = true
			default:
				continue
			}
		}
		caption := ""
		if tb := shape.Child(nsVML, "textbox"); tb != nil {
			caption = strings.Join(strings.Fields(ooxml.CleanText(tb.Text())), " ")
		}
		out[at] = append(out[at], checkbox{checked, caption})
	}
}

// anchorCell reads the start of an x:Anchor: LeftColumn, LeftOffset, TopRow.
func anchorCell(anchor string) (cellPos, bool) {
	fields := strings.Split(anchor, ",")
	if len(fields) < 3 {
		return cellPos{}, false
	}
	var n [3]uint64
	for i := range n {
		v, ok := parseUint(strings.TrimSpace(fields[i]), 32)
		if !ok {
			return cellPos{}, false
		}
		n[i] = v
	}
	return cellPos{int(n[2]), int(n[0])}, true
}

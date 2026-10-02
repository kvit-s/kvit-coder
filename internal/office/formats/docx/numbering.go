package docx

import (
	"math"
	"strconv"

	"github.com/kvit-s/kvit-coder/internal/office/model"
	"github.com/kvit-s/kvit-coder/internal/office/opc"
)

const numLevels = 9

// numberPiece is literal number text, or a reference to a zero-based
// level's current number when level >= 0.
type numberPiece struct {
	literal string
	level   int
}

type levelDef struct {
	marker     model.MarkerKind
	suppressed bool
	start      int
	// restart is w:lvlRestart: absent restarts on any shallower level, 0
	// never restarts, n restarts when a level below n appears.
	restart    int
	hasRestart bool
	pattern    []numberPiece
	legal      bool
}

func defaultLevel() levelDef { return levelDef{marker: model.Bullet, start: 1} }

type instance struct {
	levels  [numLevels]levelDef
	pstyles [numLevels]string
}

func (in *instance) styleLevel(styleID string) (int, bool) {
	for i, p := range in.pstyles {
		if p != "" && p == styleID {
			return i, true
		}
	}
	return 0, false
}

type abstractNum struct {
	inst         instance
	numStyleLink string
}

// parseNumbering resolves numId -> w:num (with level overrides) ->
// abstractNum (following numStyleLink through the numbering style's own
// numId) -> levels.
func parseNumbering(root *opc.Element, styleNumID func(string) (int, bool)) (map[int]*instance, error) {
	abstracts := map[string]*abstractNum{}
	for abs := range root.Children(nsW, "abstractNum") {
		id, ok := abs.Attr(nsW, "abstractNumId")
		if !ok {
			continue
		}
		a := &abstractNum{}
		for i := range a.inst.levels {
			a.inst.levels[i] = defaultLevel()
		}
		for lvl := range abs.Children(nsW, "lvl") {
			if ilvl := levelIndex(lvl); ilvl < numLevels {
				a.inst.levels[ilvl] = parseLevel(lvl)
				a.inst.pstyles[ilvl], _ = val(lvl, "pStyle")
			}
		}
		a.numStyleLink, _ = val(abs, "numStyleLink")
		abstracts[id] = a
	}

	type direct struct {
		absID string
		num   *opc.Element
	}
	nums := map[int]direct{}
	var order []int
	for num := range root.Children(nsW, "num") {
		idText, _ := num.Attr(nsW, "numId")
		numID, err := strconv.ParseUint(idText, 10, 31)
		absID, ok := val(num, "abstractNumId")
		if err != nil || !ok {
			continue
		}
		if _, dup := nums[int(numID)]; !dup {
			order = append(order, int(numID))
		}
		nums[int(numID)] = direct{absID, num}
	}

	instances := map[int]*instance{}
	for _, numID := range order {
		d := nums[numID]
		abs, err := resolveAbstract(d.absID, abstracts, func(styleID string) (string, bool) {
			linked, ok := styleNumID(styleID)
			if !ok {
				return "", false
			}
			next, ok := nums[linked]
			return next.absID, ok
		})
		if err != nil {
			return nil, err
		}
		if abs == nil {
			model.Log.Warn("numbering instance references an unknown abstract", "numId", numID, "abstractNumId", d.absID)
			continue
		}
		inst := abs.inst
		for over := range d.num.Children(nsW, "lvlOverride") {
			ilvl := levelIndex(over)
			if ilvl >= numLevels {
				continue
			}
			if lvl := child(over, "lvl"); lvl != nil {
				inst.levels[ilvl] = parseLevel(lvl)
				inst.pstyles[ilvl], _ = val(lvl, "pStyle")
			}
			if v, ok := val(over, "startOverride"); ok {
				if start, ok := parseStart(v); ok {
					inst.levels[ilvl].start = start
				}
			}
		}
		instances[numID] = &inst
	}
	return instances, nil
}

func resolveAbstract(absID string, abstracts map[string]*abstractNum, link func(string) (string, bool)) (*abstractNum, error) {
	seen := map[string]bool{}
	for {
		if seen[absID] {
			return nil, &opc.MalformedError{Detail: "numbering indirection cycle at abstract " + strconv.Quote(absID)}
		}
		seen[absID] = true
		abs, ok := abstracts[absID]
		if !ok {
			return nil, nil
		}
		if abs.numStyleLink == "" {
			return abs, nil
		}
		next, ok := link(abs.numStyleLink)
		if !ok {
			return abs, nil
		}
		absID = next
	}
}

func levelIndex(e *opc.Element) int {
	v, _ := e.Attr(nsW, "ilvl")
	n, _ := levelIndexValue(v)
	return n
}

func levelIndexValue(v string) (int, bool) {
	n, err := strconv.ParseUint(v, 10, 31)
	return int(n), err == nil
}

// parseStart clamps ST_DecimalNumber to the non-negative xsd:int range so
// untrusted start values can never overflow the counters.
func parseStart(v string) (int, bool) {
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return int(min(max(n, 0), math.MaxInt32)), true
}

func parseLevel(lvl *opc.Element) levelDef {
	def := defaultLevel()
	fmtVal, ok := val(lvl, "numFmt")
	if !ok {
		fmtVal = "bullet"
	}
	switch fmtVal {
	case "none":
		def.suppressed = true
	case "bullet":
	case "lowerLetter":
		def.marker = model.LowerAlpha
	case "upperLetter":
		def.marker = model.UpperAlpha
	case "lowerRoman":
		def.marker = model.LowerRoman
	case "upperRoman":
		def.marker = model.UpperRoman
	default:
		def.marker = model.Decimal
	}
	if v, ok := val(lvl, "start"); ok {
		if start, ok := parseStart(v); ok {
			def.start = start
		}
	}
	if v, ok := val(lvl, "lvlRestart"); ok {
		if n, err := strconv.ParseUint(v, 10, 32); err == nil {
			def.restart, def.hasRestart = int(n), true
		}
	}
	if !def.suppressed && def.marker.Ordered() {
		if text, ok := val(lvl, "lvlText"); ok {
			def.pattern = parsePercentPattern(text)
		}
	}
	def.legal, _ = onOff(lvl, "isLgl")
	return def
}

// parsePercentPattern tokenizes w:lvlText: %1-%9 reference levels, anything
// else is literal.
func parsePercentPattern(text string) []numberPiece {
	var out []numberPiece
	rs := []rune(text)
	for i := 0; i < len(rs); i++ {
		if rs[i] == '%' && i+1 < len(rs) && rs[i+1] >= '1' && rs[i+1] <= '9' {
			out = append(out, numberPiece{level: int(rs[i+1] - '1')})
			i++
			continue
		}
		if n := len(out); n > 0 && out[n-1].level < 0 {
			out[n-1].literal += string(rs[i])
			continue
		}
		out = append(out, numberPiece{literal: string(rs[i]), level: -1})
	}
	return out
}

// counterState holds one instance's document-order counters, shared across
// interruptions so a later paragraph continues the count.
type counterState struct {
	value          [numLevels]int
	initialized    [numLevels]bool
	restartPending [numLevels]bool
}

// next advances the counter at (numID, ilvl) and returns the effective
// number plus the composite label when the level's number text is not
// reproducible from the marker kind and number alone.
func (c *converter) nextNumber(numID, ilvl int, inst *instance) (int, string) {
	ilvl = min(ilvl, numLevels-1)
	st, ok := c.counters[numID]
	if !ok {
		st = &counterState{}
		c.counters[numID] = st
	}
	def := inst.levels[ilvl]
	if !st.initialized[ilvl] || st.restartPending[ilvl] {
		st.value[ilvl] = def.start
		st.initialized[ilvl] = true
		st.restartPending[ilvl] = false
	} else {
		st.value[ilvl]++
	}
	for deeper := ilvl + 1; deeper < numLevels; deeper++ {
		d := inst.levels[deeper]
		if !d.hasRestart || d.restart != 0 && ilvl < d.restart {
			st.restartPending[deeper] = true
		}
	}
	value := st.value[ilvl]
	if def.suppressed || len(def.pattern) == 0 {
		return value, ""
	}
	var label string
	for _, piece := range def.pattern {
		if piece.level < 0 {
			label += piece.literal
			continue
		}
		l := min(piece.level, numLevels-1)
		kind := inst.levels[l].marker
		if inst.levels[l].suppressed || def.legal {
			kind = model.Decimal
		}
		n := inst.levels[l].start
		if st.initialized[l] && !st.restartPending[l] {
			n = st.value[l]
		}
		label += kind.Ordinal(n)
	}
	if label == def.marker.Label(value) {
		return value, ""
	}
	return value, label
}

package gfm

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/kvit-s/kvit-coder/internal/office/model"
)

// normalize drops empty runs and untargeted anchors, strips styling from
// whitespace-only runs, merges adjacent same-style runs, re-joins styled runs
// split only by whitespace, and unwraps links without a destination.
func (r *renderer) normalize(inlines []model.Inline) []model.Inline {
	var out []model.Inline
	for _, in := range inlines {
		switch in := in.(type) {
		case model.Text:
			if in.Text == "" {
				continue
			}
			if strings.TrimSpace(in.Text) == "" {
				in.Style = model.Style{}
			}
			n := len(out)
			if n > 0 {
				if prev, ok := out[n-1].(model.Text); ok && prev.Style == in.Style {
					out[n-1] = model.Text{Text: prev.Text + in.Text, Style: in.Style}
					continue
				}
			}
			if in.Style != (model.Style{}) && !in.Style.Code && n >= 2 {
				ws, wsOK := out[n-1].(model.Text)
				prev, prevOK := out[n-2].(model.Text)
				if wsOK && prevOK && ws.Style == (model.Style{}) && strings.TrimSpace(ws.Text) == "" && prev.Style == in.Style {
					out = out[:n-1]
					out[n-2] = model.Text{Text: prev.Text + ws.Text + in.Text, Style: in.Style}
					continue
				}
			}
			out = append(out, in)
		case model.Link:
			if in.Target.Ref == "" {
				if !model.IsEmpty(in.Content) {
					out = append(out, r.normalize(in.Content)...)
				}
				continue
			}
			out = append(out, in)
		case model.Anchor:
			if _, ok := r.anchors.htmlID(string(in)); ok {
				out = append(out, in)
			}
		case model.Math:
			if tex := strings.TrimSpace(string(in)); tex != "" {
				out = append(out, model.Math(tex))
			}
		default:
			out = append(out, in)
		}
	}
	return out
}

func (r *renderer) inlines(inlines []model.Inline, ctx context, inLabel bool) string {
	runs := r.normalize(inlines)
	suffix := r.delimsAhead(runs)
	var out strings.Builder
	for idx, run := range runs {
		var next model.Inline
		if idx+1 < len(runs) {
			next = runs[idx+1]
		}
		switch run := run.(type) {
		case model.Text:
			nextActive, nextNonspace := false, false
			switch next := next.(type) {
			case model.Link, model.Image, model.NoteRef, model.Math:
				nextActive = true
			case model.Text:
				nextActive = next.Style != model.Style{}
			case model.Anchor, model.Checkbox:
				nextNonspace = true
			case model.LineBreak:
				nextNonspace = ctx != headingContext
			}
			r.textRun(&out, run, ctx, escapeOpts{
				trailingActive:   nextActive,
				trailingNonspace: nextNonspace,
				trailingDelims:   suffix[idx+1],
				inLabel:          inLabel,
			})
		case model.NoteRef:
			if num, ok := r.notes[string(run)]; ok {
				out.WriteString("[^" + strconv.Itoa(num) + "]")
			}
		case model.Link:
			r.link(&out, run, ctx)
		case model.Image:
			image(&out, run, ctx, inLabel)
		case model.Anchor:
			if id, ok := r.anchors.htmlID(string(run)); ok {
				out.WriteString(`<a id="` + id + `"></a>`)
			}
		case model.LineBreak:
			switch ctx {
			case blockContext:
				out.WriteString("\\\n")
			case headingContext:
				out.WriteByte(' ')
			case cellContext:
				out.WriteByte('\n')
			}
		case model.Math:
			out.WriteString(mathSpan(string(run), ctx))
		case model.Checkbox:
			out.WriteString(model.CheckboxText(bool(run)))
			if next != nil && !startsWithSpace(next) {
				out.WriteByte(' ')
			}
		}
	}
	return out.String()
}

func (r *renderer) link(out *strings.Builder, link model.Link, ctx context) {
	url := link.Target.Ref
	if link.Target.Kind == model.TargetAnchor {
		fragment, ok := r.anchors.fragment(link.Target.Ref)
		if !ok {
			out.WriteString(r.inlines(link.Content, ctx, false))
			return
		}
		url = "#" + fragment
	}
	label := r.inlines(link.Content, ctx, true)
	if strings.TrimSpace(label) == "" {
		if link.Target.Kind == model.TargetAnchor {
			return
		}
		label = escapeText(spaceControls(url), ctx, escapeOpts{trailingActive: true, inLabel: true})
	}
	out.WriteString("[" + label + "](" + formatURL(url) + ")")
}

func image(out *strings.Builder, img model.Image, ctx context, inLabel bool) {
	alt := strings.TrimSpace(img.Alt)
	if img.Source.Kind == model.SourceExternal {
		out.WriteString("![" + escapeText(alt, ctx, escapeOpts{inLabel: true}) + "](" + formatURL(img.Source.URL) + ")")
		return
	}
	if alt != "" {
		out.WriteString(escapeText(alt, ctx, escapeOpts{inLabel: inLabel}))
	}
}

func (r *renderer) delimsAhead(runs []model.Inline) []delims {
	suffix := make([]delims, len(runs)+1)
	for idx := len(runs) - 1; idx >= 0; idx-- {
		suffix[idx] = suffix[idx+1]
		suffix[idx].union(r.delimsOf(runs[idx]))
	}
	return suffix
}

// delimsOf reports what one run contributes to an earlier run's pairing
// partners. Styled content is escaped, which neutralizes everything but
// backticks (code spans ignore escapes) and ]; emphasis cannot cross a link
// boundary but a code span can.
func (r *renderer) delimsOf(run model.Inline) delims {
	var d delims
	switch run := run.(type) {
	case model.Text:
		switch {
		case run.Style.Code:
			d.insert('`')
		case run.Style == model.Style{}:
			d.insertClosers(run.Text)
		default:
			if run.Style.Bold || run.Style.Italic {
				d.insert('*')
			}
			if run.Style.Strike {
				d.insert('~')
			}
			if strings.ContainsRune(run.Text, '`') {
				d.insert('`')
			}
			if strings.ContainsRune(run.Text, ']') {
				d.insert(']')
			}
			d.insertClosers(strings.Map(func(c rune) rune {
				if c == '$' || unicode.IsSpace(c) {
					return c
				}
				return 'x'
			}, run.Text))
		}
	case model.Link:
		if _, ok := r.anchors.fragment(run.Target.Ref); run.Target.Kind == model.TargetAnchor && !ok {
			for _, inner := range r.normalize(run.Content) {
				d.union(r.delimsOf(inner))
			}
		} else if emitsBacktick(run.Content) || run.Target.Kind != model.TargetAnchor && strings.ContainsRune(run.Target.Ref, '`') {
			d.insert('`')
		}
	case model.Image:
		if run.Source.Kind != model.SourceExternal {
			d.insertClosers(run.Alt)
		} else if strings.ContainsRune(run.Alt, '`') {
			d.insert('`')
		}
	}
	return d
}

func startsWithSpace(run model.Inline) bool {
	switch run := run.(type) {
	case model.Text:
		return strings.IndexFunc(run.Text, unicode.IsSpace) == 0
	case model.LineBreak:
		return true
	}
	return false
}

func emitsBacktick(inlines []model.Inline) bool {
	for _, in := range inlines {
		switch in := in.(type) {
		case model.Text:
			if strings.ContainsRune(in.Text, '`') || in.Style.Code && strings.TrimSpace(in.Text) != "" {
				return true
			}
		case model.Link:
			if emitsBacktick(in.Content) || in.Target.Kind != model.TargetAnchor && strings.ContainsRune(in.Target.Ref, '`') {
				return true
			}
		case model.Image:
			if strings.ContainsRune(in.Alt, '`') {
				return true
			}
		}
	}
	return false
}

func (r *renderer) textRun(out *strings.Builder, run model.Text, ctx context, o escapeOpts) {
	if run.Style == (model.Style{}) {
		s := out.String()
		o.atLineStart = s == "" || strings.HasSuffix(s, "\n")
		out.WriteString(escapeText(run.Text, ctx, o))
		return
	}
	core := strings.TrimLeftFunc(run.Text, unicode.IsSpace)
	out.WriteString(run.Text[:len(run.Text)-len(core)])
	trimmed := strings.TrimRightFunc(core, unicode.IsSpace)
	trail := core[len(trimmed):]
	core = trimmed
	if core != "" {
		if run.Style.Code {
			out.WriteString(codeSpan(core, ctx))
		} else {
			var open, closer string
			if run.Style.Strike {
				open, closer = "~~", "~~"
			}
			if run.Style.Bold {
				open, closer = open+"**", "**"+closer
			}
			if run.Style.Italic {
				open, closer = open+"*", "*"+closer
			}
			out.WriteString(open)
			out.WriteString(escapeText(core, ctx, escapeOpts{styled: true, inLabel: o.inLabel}))
			out.WriteString(closer)
		}
	}
	out.WriteString(trail)
}

// mathSpan renders inline math between $ hugging the source: a line break
// would end the span and a bare $ would close it early. Rows split into
// cells before math parses, so a bare pipe is syntax in a cell.
func mathSpan(tex string, ctx context) string {
	source := escapeUnescaped(strings.ReplaceAll(strings.TrimSpace(tex), "\n", " "), '$')
	if ctx == cellContext {
		source = escapeUnescaped(source, '|')
	}
	return "$" + source + "$"
}

func codeSpan(text string, ctx context) string {
	text = strings.ReplaceAll(text, "\n", " ")
	fence := backtickFence(text, 1)
	pad := ""
	if strings.HasPrefix(text, "`") || strings.HasSuffix(text, "`") {
		pad = " "
	}
	if ctx == cellContext {
		text = escapeCellCodeSpan(text)
	}
	return fence + pad + text + pad + fence
}

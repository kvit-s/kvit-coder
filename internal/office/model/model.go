// Package model is the intermediate representation office-format frontends
// produce and the GFM renderer consumes. Only fully resolved content lives
// here: style cascades, numbering and references are resolved before these
// values are built, and a Document stays self-contained after its source is
// gone.
package model

import "strings"

type Document struct {
	Blocks []Block
	Notes  []Note
	Assets []Asset
}

type NoteKind int

const (
	Footnote NoteKind = iota
	Endnote
)

type Note struct {
	ID     string
	Kind   NoteKind
	Blocks []Block
}

type AssetID int

type Asset struct {
	ID         AssetID
	MediaType  string
	OriginPart string
	Bytes      []byte
}

type Block interface{ block() }

// Heading levels are the source's outline depth, 1-based and unclamped.
// Anchor is the id the document targets this heading by, "" when none.
type Heading struct {
	Level   int
	Anchor  string
	Content []Inline
}

type Paragraph []Inline

type Quote []Block

type CodeBlock struct {
	Lang string
	Text string
}

type Rule struct{}

// MathBlock is a displayed formula as LaTeX without delimiters.
type MathBlock string

func (Heading) block()   {}
func (Paragraph) block() {}
func (List) block()      {}
func (Table) block()     {}
func (Quote) block()     {}
func (CodeBlock) block() {}
func (Rule) block()      {}
func (MathBlock) block() {}

type Inline interface{ inline() }

type Style struct {
	Bold   bool
	Italic bool
	Strike bool
	Code   bool
}

type Text struct {
	Text  string
	Style Style
}

type Link struct {
	Content []Inline
	Target  Target
}

type TargetKind int

const (
	TargetExternal TargetKind = iota
	TargetRelative
	TargetAnchor
)

// Target is an absolute URL, a scheme-less relative reference kept as
// written, or a document-scoped anchor id (a heading anchor or Anchor node).
type Target struct {
	Kind TargetKind
	Ref  string
}

type SourceKind int

const (
	SourceUnavailable SourceKind = iota
	SourceExternal
	SourceAsset
)

type ImageSource struct {
	Kind  SourceKind
	URL   string
	Asset AssetID
}

type Image struct {
	Alt    string
	Source ImageSource
}

type Anchor string

type NoteRef string

type LineBreak struct{}

// Math is an inline formula as LaTeX without delimiters.
type Math string

type Checkbox bool

func (Text) inline()      {}
func (Link) inline()      {}
func (Image) inline()     {}
func (Anchor) inline()    {}
func (NoteRef) inline()   {}
func (LineBreak) inline() {}
func (Math) inline()      {}
func (Checkbox) inline()  {}

func CheckboxText(checked bool) string {
	if checked {
		return "[x]"
	}
	return "[ ]"
}

// PlainText flattens inlines to their text: link text, image alt text and
// formula source are kept, line breaks become newlines, anchors and note
// references contribute nothing.
func PlainText(inlines []Inline) string {
	var sb strings.Builder
	writePlainText(&sb, inlines)
	return sb.String()
}

func writePlainText(sb *strings.Builder, inlines []Inline) {
	for _, in := range inlines {
		switch in := in.(type) {
		case Text:
			sb.WriteString(in.Text)
		case Link:
			writePlainText(sb, in.Content)
		case Image:
			sb.WriteString(in.Alt)
		case Math:
			sb.WriteString(string(in))
		case Checkbox:
			sb.WriteString(CheckboxText(bool(in)))
		case LineBreak:
			sb.WriteByte('\n')
		}
	}
}

// IsEmpty reports whether nothing would render visibly: only whitespace,
// empty-target links, anchors and line breaks. Images, note references and
// checkboxes always count as content.
func IsEmpty(inlines []Inline) bool {
	for _, in := range inlines {
		switch in := in.(type) {
		case Text:
			if strings.TrimSpace(in.Text) != "" {
				return false
			}
		case Link:
			if in.Target.Ref != "" || !IsEmpty(in.Content) {
				return false
			}
		case Math:
			if strings.TrimSpace(string(in)) != "" {
				return false
			}
		case Image, NoteRef, Checkbox:
			return false
		}
	}
	return true
}

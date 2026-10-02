package tools

import (
	"bytes"
	"fmt"
	"strings"

	htmltomarkdown "github.com/JohannesKaufmann/html-to-markdown/v2"
	"golang.org/x/net/html"
)

// prunedTags are the subtrees dropped before conversion. Which parts of a page
// are content is a judgement that changes per site, so it lives here rather
// than in the converter: the library's job is turning a cleaned tree into
// well-formed markdown, and this is the cleaning.
var prunedTags = map[string]bool{
	"script": true, "style": true, "noscript": true, "svg": true,
	"nav": true, "header": true, "footer": true, "form": true,
	"iframe": true, "canvas": true, "template": true,
}

// prunedRoles and prunedClassHints catch the same furniture when it is marked
// up as a div, which is more common than the semantic tags above.
var prunedRoles = map[string]bool{
	"navigation": true, "banner": true, "contentinfo": true, "search": true,
}

// convertPage turns an HTML document into markdown, dropping the parts of the
// page that are furniture rather than content. It returns the markdown and the
// document title.
func convertPage(raw []byte) (markdown string, title string, err error) {
	doc, err := html.Parse(bytes.NewReader(raw))
	if err != nil {
		return "", "", fmt.Errorf("parsing html: %w", err)
	}

	title = strings.TrimSpace(findTitle(doc))
	prune(doc)

	md, err := htmltomarkdown.ConvertNode(doc)
	if err != nil {
		return "", title, fmt.Errorf("converting to markdown: %w", err)
	}
	return strings.TrimSpace(collapseBlankRuns(string(md))), title, nil
}

// findTitle returns the contents of <title>, which survives pruning because it
// is read before the tree is cut.
func findTitle(n *html.Node) string {
	if n.Type == html.ElementNode && n.Data == "title" {
		var sb strings.Builder
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.TextNode {
				sb.WriteString(c.Data)
			}
		}
		return sb.String()
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if t := findTitle(c); t != "" {
			return t
		}
	}
	return ""
}

// prune removes furniture subtrees in place.
func prune(n *html.Node) {
	var next *html.Node
	for c := n.FirstChild; c != nil; c = next {
		next = c.NextSibling
		if c.Type == html.ElementNode && shouldPrune(c) {
			n.RemoveChild(c)
			continue
		}
		prune(c)
	}
}

func shouldPrune(n *html.Node) bool {
	if prunedTags[n.Data] {
		return true
	}
	for _, a := range n.Attr {
		if a.Key == "role" && prunedRoles[strings.ToLower(a.Val)] {
			return true
		}
		if a.Key == "aria-hidden" && a.Val == "true" {
			return true
		}
	}
	return false
}

// collapseBlankRuns squeezes runs of blank lines to one. Pruning leaves holes
// where the furniture was, and a page that is a third empty lines wastes the
// caller's line budget on nothing.
func collapseBlankRuns(s string) string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	blank := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "" {
			blank++
			if blank > 1 {
				continue
			}
		} else {
			blank = 0
		}
		out = append(out, strings.TrimRight(l, " \t"))
	}
	return strings.Join(out, "\n")
}

// outlineEntry is one heading in a converted page, with the line it starts on.
type outlineEntry struct {
	Line    int    `json:"line"`
	Level   int    `json:"level"`
	Heading string `json:"heading"`
}

// buildOutline lists the markdown headings with the line numbers they occupy in
// the file on disk, so a Read starting at the line an entry gives lands where
// the model expected. This is appendix A.1's index tool, and it is nearly free
// here: the heading levels came from the page's own <h1>-<h6>, so building the
// outline is counting '#' prefixes in output that already exists.
//
// Lines are 1-based, matching what Read reports.
func buildOutline(markdown string) []outlineEntry {
	var out []outlineEntry
	inFence := false
	for i, line := range strings.Split(markdown, "\n") {
		trimmed := strings.TrimSpace(line)
		// A '#' inside a fenced code block is a comment, not a heading.
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence || !strings.HasPrefix(trimmed, "#") {
			continue
		}
		level := 0
		for level < len(trimmed) && trimmed[level] == '#' {
			level++
		}
		if level > 6 || level == len(trimmed) || trimmed[level] != ' ' {
			continue
		}
		text := strings.TrimSpace(trimmed[level:])
		if text == "" {
			continue
		}
		out = append(out, outlineEntry{Line: i + 1, Level: level, Heading: text})
	}
	return out
}

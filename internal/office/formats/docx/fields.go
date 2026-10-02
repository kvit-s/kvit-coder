package docx

import (
	"strings"
	"unicode"

	"github.com/kvit-s/kvit-coder/internal/office/formats/ooxml"
	"github.com/kvit-s/kvit-coder/internal/office/model"
)

type fieldFrame struct {
	instr    strings.Builder
	inResult bool
	inlines  []model.Inline
}

// fieldResult wraps a finished field's result in a link when the
// instruction is a HYPERLINK, and passes it through otherwise.
func fieldResult(instr string, content []model.Inline) []model.Inline {
	if target, ok := hyperlinkTarget(instr); ok && !model.IsEmpty(content) {
		return []model.Inline{model.Link{Content: content, Target: target}}
	}
	return content
}

type fieldToken struct {
	word string
	sw   rune
}

// tokenizeField splits a field instruction into quoted strings (with \" and
// \\ escapes), backslash switches and bare words.
func tokenizeField(instr string) []fieldToken {
	var out []fieldToken
	rs := []rune(instr)
	for i := 0; i < len(rs); {
		c := rs[i]
		switch {
		case unicode.IsSpace(c):
			i++
		case c == '"':
			i++
			var word strings.Builder
		quoted:
			for i < len(rs) {
				c := rs[i]
				i++
				switch c {
				case '"':
					break quoted
				case '\\':
					if i >= len(rs) {
						break quoted
					}
					esc := rs[i]
					i++
					if esc != '"' && esc != '\\' {
						word.WriteByte('\\')
					}
					word.WriteRune(esc)
				default:
					word.WriteRune(c)
				}
			}
			out = append(out, fieldToken{word: word.String()})
		case c == '\\':
			i++
			if i < len(rs) {
				out = append(out, fieldToken{sw: unicode.ToLower(rs[i])})
				i++
			}
		default:
			start := i
			for i < len(rs) && !unicode.IsSpace(rs[i]) {
				i++
			}
			out = append(out, fieldToken{word: string(rs[start:i])})
		}
	}
	return out
}

// hyperlinkTarget interprets a HYPERLINK field instruction. The \l, \o and
// \t switches take an argument, but only when the next token is a word.
func hyperlinkTarget(instr string) (model.Target, bool) {
	tokens := tokenizeField(instr)
	if len(tokens) == 0 || tokens[0].sw != 0 || !strings.EqualFold(tokens[0].word, "HYPERLINK") {
		return model.Target{}, false
	}
	var url, anchor string
	for i := 1; i < len(tokens); i++ {
		tok := tokens[i]
		if tok.sw == 0 {
			if url == "" && strings.TrimSpace(tok.word) != "" {
				url = strings.TrimSpace(tok.word)
			}
			continue
		}
		if !strings.ContainsRune("lot", tok.sw) || i+1 >= len(tokens) || tokens[i+1].sw != 0 {
			continue
		}
		i++
		if arg := strings.TrimSpace(tokens[i].word); tok.sw == 'l' && arg != "" {
			anchor = arg
		}
	}
	switch {
	case url != "" && anchor != "":
		return classifyTarget(url + "#" + anchor), true
	case url != "":
		return classifyTarget(url), true
	case anchor != "":
		return model.Target{Kind: model.TargetAnchor, Ref: anchor}, true
	}
	return model.Target{}, false
}

func classifyTarget(url string) model.Target {
	if anchor, ok := strings.CutPrefix(url, "#"); ok {
		return model.Target{Kind: model.TargetAnchor, Ref: anchor}
	}
	if ooxml.IsAbsoluteURI(url) {
		return model.Target{Kind: model.TargetExternal, Ref: url}
	}
	return model.Target{Kind: model.TargetRelative, Ref: url}
}

package opc

import (
	"encoding/xml"
	"errors"
	"strconv"
	"strings"
	"unicode/utf8"
)

var errNotScannable = errors.New("opc: markup needs the full decoder")

// scanXML parses the common shape of OOXML parts straight from memory: UTF-8
// markup with ASCII names, quoted attributes, comments, processing
// instructions and the predefined or numeric character references. It
// decodes exactly what the non-strict encoding/xml decoder yields for such
// input, and returns errNotScannable for anything else (DOCTYPE, CDATA, other
// entities, unquoted or valueless attributes, transcoded encodings, any
// character or syntax the decoder rejects or repairs), leaving that input to
// the decoder. Limit errors are final, as the decoder would reach them at
// the same token.
func scanXML(data []byte, maxNodes int) (*Element, error) {
	if !plainChars(data) {
		return nil, errNotScannable
	}
	src := string(data)
	b := &treeBuilder{maxNodes: maxNodes}
	var attrs []xml.Attr
	for i := 0; i < len(src); {
		if src[i] != '<' {
			end := strings.IndexByte(src[i:], '<')
			if end < 0 {
				end = len(src)
			} else {
				end += i
			}
			text, ok := decodeText(src[i:end], false)
			if !ok {
				return nil, errNotScannable
			}
			if err := b.text(text); err != nil {
				return nil, err
			}
			i = end
			continue
		}
		if i+1 == len(src) {
			return nil, errNotScannable
		}
		switch src[i+1] {
		case '/':
			n := nameLen(src, i+2)
			if n == 0 || !splittable(src[i+2:i+2+n]) {
				return nil, errNotScannable
			}
			k := skipSpace(src, i+2+n)
			if k == len(src) || src[k] != '>' {
				return nil, errNotScannable
			}
			b.end()
			i = k + 1
		case '?':
			n := nameLen(src, i+2)
			if n == 0 {
				return nil, errNotScannable
			}
			k := skipSpace(src, i+2+n)
			end := strings.Index(src[k:], "?>")
			if end < 0 || src[i+2:i+2+n] == "xml" && !plainDeclaration(src[k:k+end]) {
				return nil, errNotScannable
			}
			i = k + end + 2
		case '!':
			if !strings.HasPrefix(src[i:], "<!--") {
				return nil, errNotScannable
			}
			body := src[i+4:]
			dashes := strings.Index(body, "--")
			if dashes < 0 || dashes+2 == len(body) || body[dashes+2] != '>' {
				return nil, errNotScannable
			}
			i += 4 + dashes + 3
		default:
			n := nameLen(src, i+1)
			if n == 0 || !splittable(src[i+1:i+1+n]) {
				return nil, errNotScannable
			}
			name := splitName(src[i+1 : i+1+n])
			attrs = attrs[:0]
			k := i + 1 + n
			selfClosing := false
		attributes:
			for {
				k = skipSpace(src, k)
				if k == len(src) {
					return nil, errNotScannable
				}
				switch src[k] {
				case '>':
					k++
					break attributes
				case '/':
					if k+1 == len(src) || src[k+1] != '>' {
						return nil, errNotScannable
					}
					k += 2
					selfClosing = true
					break attributes
				}
				n := nameLen(src, k)
				if n == 0 || !splittable(src[k:k+n]) {
					return nil, errNotScannable
				}
				attr := xml.Attr{Name: splitName(src[k : k+n])}
				k = skipSpace(src, k+n)
				if k == len(src) || src[k] != '=' {
					return nil, errNotScannable
				}
				k = skipSpace(src, k+1)
				if k == len(src) || src[k] != '"' && src[k] != '\'' {
					return nil, errNotScannable
				}
				end := strings.IndexByte(src[k+1:], src[k])
				if end < 0 {
					return nil, errNotScannable
				}
				value, ok := decodeText(src[k+1:k+1+end], true)
				if !ok {
					return nil, errNotScannable
				}
				attr.Value = value
				attrs = append(attrs, attr)
				k += end + 2
			}
			if err := b.start(name, attrs); err != nil {
				return nil, err
			}
			if selfClosing {
				b.end()
			}
			i = k
		}
	}
	return b.finish()
}

// plainChars reports valid UTF-8 made only of XML characters; the decoder
// rejects the rest wherever it reads text.
func plainChars(data []byte) bool {
	for i := 0; i < len(data); {
		c := data[i]
		if c < utf8.RuneSelf {
			if c < 0x20 && c != '\t' && c != '\n' && c != '\r' {
				return false
			}
			i++
			continue
		}
		r, size := utf8.DecodeRune(data[i:])
		if r == utf8.RuneError && size == 1 || r == 0xFFFE || r == 0xFFFF {
			return false
		}
		i += size
	}
	return true
}

func isNameStart(c byte) bool {
	return 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || c == '_'
}

// nameLen is the length of the ASCII name at src[i:], or 0 when there is
// none or it runs into a byte the decoder would read as part of the name.
func nameLen(src string, i int) int {
	if i == len(src) || !isNameStart(src[i]) {
		return 0
	}
	j := i + 1
	for j < len(src) {
		c := src[j]
		if !isNameStart(c) && !('0' <= c && c <= '9') && c != ':' && c != '.' && c != '-' {
			break
		}
		j++
	}
	if j < len(src) && src[j] >= utf8.RuneSelf {
		return 0
	}
	return j - i
}

func splittable(name string) bool {
	return strings.Count(name, ":") <= 1
}

func splitName(s string) xml.Name {
	space, local, ok := strings.Cut(s, ":")
	if !ok || space == "" || local == "" {
		return xml.Name{Local: s}
	}
	return xml.Name{Space: space, Local: local}
}

func skipSpace(src string, i int) int {
	for i < len(src) && (src[i] == ' ' || src[i] == '\t' || src[i] == '\n' || src[i] == '\r') {
		i++
	}
	return i
}

// plainDeclaration reports an XML declaration the decoder reads without
// switching readers: version 1.0 or none, and an encoding the input already
// satisfies.
func plainDeclaration(content string) bool {
	if v := procInstParam("version", content); v != "" && v != "1.0" {
		return false
	}
	switch strings.ToLower(procInstParam("encoding", content)) {
	case "", "utf-8", "utf-16", "utf-16le", "utf-16be", "us-ascii", "ascii":
		return true
	}
	return false
}

// procInstParam reads a pseudo-attribute the way the decoder does.
func procInstParam(param, s string) string {
	param += "="
	var sep byte
	i := 0
	for i < len(s) {
		sub := s[i:]
		k := strings.Index(sub, param)
		if k < 0 || len(param)+k >= len(sub) {
			return ""
		}
		i += len(param) + k + 1
		if c := sub[len(param)+k]; c == '\'' || c == '"' {
			sep = c
			break
		}
	}
	if sep == 0 {
		return ""
	}
	j := strings.IndexByte(s[i:], sep)
	if j < 0 {
		return ""
	}
	return s[i : i+j]
}

// decodeText resolves character references and folds CR and CRLF to LF, as
// the decoder does for text and quoted values. It reports false for input
// the decoder rejects or keeps literally.
func decodeText(raw string, quoted bool) (string, bool) {
	if strings.IndexByte(raw, '<') >= 0 || !quoted && strings.Contains(raw, "]]>") {
		return "", false
	}
	if strings.IndexByte(raw, '&') < 0 && strings.IndexByte(raw, '\r') < 0 {
		return raw, true
	}
	var sb strings.Builder
	sb.Grow(len(raw))
	for i := 0; i < len(raw); i++ {
		switch c := raw[i]; c {
		case '\r':
			sb.WriteByte('\n')
			if i+1 < len(raw) && raw[i+1] == '\n' {
				i++
			}
		case '&':
			semi := strings.IndexByte(raw[i:], ';')
			if semi < 0 {
				return "", false
			}
			ref := raw[i+1 : i+semi]
			r, ok := resolveReference(ref)
			if !ok {
				return "", false
			}
			sb.WriteRune(r)
			i += semi
		default:
			sb.WriteByte(c)
		}
	}
	return sb.String(), true
}

func resolveReference(ref string) (rune, bool) {
	switch ref {
	case "lt":
		return '<', true
	case "gt":
		return '>', true
	case "amp":
		return '&', true
	case "apos":
		return '\'', true
	case "quot":
		return '"', true
	}
	digits, ok := strings.CutPrefix(ref, "#")
	if !ok {
		return 0, false
	}
	base := 10
	if hex, ok := strings.CutPrefix(digits, "x"); ok {
		base, digits = 16, hex
	}
	if digits == "" || strings.TrimLeft(digits, "0123456789abcdefABCDEF") != "" || base == 10 && strings.TrimLeft(digits, "0123456789") != "" {
		return 0, false
	}
	n, err := strconv.ParseUint(digits, base, 64)
	if err != nil || n > utf8.MaxRune {
		return 0, false
	}
	r := rune(n)
	if r != 0x09 && r != 0x0A && r != 0x0D && !(r >= 0x20 && r <= 0xD7FF) && !(r >= 0xE000 && r <= 0xFFFD) && r < 0x10000 {
		return 0, false
	}
	return r, true
}

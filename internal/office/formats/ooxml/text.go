// Package ooxml holds the content extraction the office frontends share:
// text cleaning, target and media classification, and the textual content
// of DrawingML charts and SmartArt.
package ooxml

import (
	"path"
	"strings"
	"unicode"
)

// CleanText drops control characters and layout-only invisibles, turns NBSP
// and line breaks (a CRLF pair is one) into spaces, and keeps tabs. ZWNJ and
// ZWJ are kept: they carry meaning in shaping and emoji sequences.
func CleanText(text string) string {
	var sb strings.Builder
	sb.Grow(len(text))
	text = strings.ReplaceAll(text, "\r\n", "\n")
	for _, c := range text {
		switch {
		case c == '\u00a0' || c == '\r' || c == '\n':
			sb.WriteByte(' ')
		case c == '\t':
			sb.WriteByte('\t')
		case c == '\u00ad' || c == '\u200b' || c == '\ufeff' || unicode.IsControl(c):
		default:
			sb.WriteRune(c)
		}
	}
	return sb.String()
}

// IsAbsoluteURI reports an RFC 3986 scheme before the first colon. Windows
// drive paths parse as one-letter schemes but are local file paths.
func IsAbsoluteURI(s string) bool {
	scheme, _, ok := strings.Cut(s, ":")
	if !ok || scheme == "" || !isASCIILetter(scheme[0]) {
		return false
	}
	for i := 1; i < len(scheme); i++ {
		c := scheme[i]
		if !isASCIILetter(c) && !(c >= '0' && c <= '9') && c != '+' && c != '-' && c != '.' {
			return false
		}
	}
	drivePath := len(scheme) == 1 && len(s) >= 3 && (s[2] == '\\' || s[2] == '/')
	return !drivePath
}

func isASCIILetter(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// MediaType maps an embedded part's extension to its media type.
func MediaType(part string) string {
	switch strings.ToLower(strings.TrimPrefix(path.Ext(part), ".")) {
	case "png":
		return "image/png"
	case "jpg", "jpeg":
		return "image/jpeg"
	case "gif":
		return "image/gif"
	case "bmp":
		return "image/bmp"
	case "tif", "tiff":
		return "image/tiff"
	case "svg":
		return "image/svg+xml"
	case "emf":
		return "image/emf"
	case "wmf":
		return "image/wmf"
	case "webp":
		return "image/webp"
	}
	return "application/octet-stream"
}

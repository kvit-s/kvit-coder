package tools

import "strings"

// CRLF handling (Windows support, decision 8).
//
// Decision: preserve-and-count-on-\n. Line numbers count \n, so they stay
// stable on CRLF files, and file bytes are otherwise untouched: splitting on
// \n keeps the trailing \r on each line, and joining on \n restores the
// original \r\n. Every file a Windows editor touched will have CRLF, so Edit
// must not convert a whole file to LF as a side effect of changing one line.
//
// Matching normalizes for comparison (strip \r) but writes preservation:
// a search for "foo\n" finds "foo\r\n" in the file, and the replacement
// adopts the file's prevailing ending so new lines do not mix endings.
func hasCRLF(content string) bool {
	return strings.Contains(content, "\r\n")
}

// normalizeForMatch strips \r so an LF search matches CRLF content.
func normalizeForMatch(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

// prevailingEnding reports the file's line ending: \r\n when the file
// contains it, else \n.
func prevailingEnding(content string) string {
	if hasCRLF(content) {
		return "\r\n"
	}
	return "\n"
}

// adaptReplacement converts an LF replacement to the file's prevailing
// ending, so an edit in a CRLF file does not mix endings line by line.
func adaptReplacement(replacement, fileContent string) string {
	if prevailingEnding(fileContent) == "\r\n" && !strings.Contains(replacement, "\r\n") {
		return strings.ReplaceAll(replacement, "\n", "\r\n")
	}
	return replacement
}

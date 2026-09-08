package session

import "strings"

// TitleFallbackWords is how many leading words of the prompt become the
// session title when no summarizer model is configured or its call fails.
const TitleFallbackWords = 5

// MaxTitleChars caps a title's length. A summarizer asked for 3-6 words
// stays far below it; it only trims a pathological answer or a very long
// fallback prompt.
const MaxTitleChars = 80

// FirstWordsTitle returns the prompt's first n words, joined by single
// spaces. It is the title fallback when the summarizer model is missing or
// its call fails. n <= 0 selects TitleFallbackWords. It returns "" for an
// empty prompt (a wake turn carries none, so there is nothing to title).
func FirstWordsTitle(prompt string, n int) string {
	if n <= 0 {
		n = TitleFallbackWords
	}
	words := strings.Fields(prompt)
	if len(words) == 0 {
		return ""
	}
	if len(words) > n {
		words = words[:n]
	}
	return NormalizeTitle(strings.Join(words, " "))
}

// NormalizeTitle trims a raw title down to what is stored in meta.json:
// the first non-empty line, without surrounding quotes or backticks,
// whitespace collapsed, capped at MaxTitleChars. It returns "" when
// nothing remains, so the caller can fall back.
func NormalizeTitle(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, line := range strings.Split(s, "\n") {
		if t := strings.TrimSpace(line); t != "" {
			s = t
			break
		}
	}
	s = strings.Trim(s, `"'`+"`")
	s = strings.TrimSpace(s)
	s = strings.Join(strings.Fields(s), " ")
	if s == "" {
		return ""
	}
	if len(s) > MaxTitleChars {
		cut := strings.LastIndex(s[:MaxTitleChars], " ")
		if cut <= 0 {
			s = s[:MaxTitleChars]
		} else {
			s = s[:cut]
		}
		s = strings.TrimSpace(s)
	}
	return strings.Trim(s, `"'`+"`")
}

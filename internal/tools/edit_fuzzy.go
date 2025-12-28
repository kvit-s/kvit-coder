package tools

import (
	"strings"
)

// FuzzyMatcher provides fuzzy string matching capabilities
type FuzzyMatcher struct {
	Threshold float64 // Similarity threshold (0.0 to 1.0)
}

// NewFuzzyMatcher creates a new FuzzyMatcher with the given threshold
func NewFuzzyMatcher(threshold float64) *FuzzyMatcher {
	return &FuzzyMatcher{Threshold: threshold}
}

// LevenshteinDistance calculates the edit distance between two strings
func LevenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}

	// Create distance matrix
	d := make([][]int, len(s1)+1)
	for i := range d {
		d[i] = make([]int, len(s2)+1)
		d[i][0] = i
	}
	for j := range d[0] {
		d[0][j] = j
	}

	// Fill distance matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			d[i][j] = min(
				d[i-1][j]+1,      // deletion
				d[i][j-1]+1,      // insertion
				d[i-1][j-1]+cost, // substitution
			)
		}
	}

	return d[len(s1)][len(s2)]
}

// SimilarityRatio calculates the similarity ratio between two strings (0.0 to 1.0)
// Based on the formula: 1 - (distance / max(len(s1), len(s2)))
func SimilarityRatio(s1, s2 string) float64 {
	if len(s1) == 0 && len(s2) == 0 {
		return 1.0
	}

	distance := LevenshteinDistance(s1, s2)
	maxLen := max(len(s1), len(s2))
	return 1.0 - float64(distance)/float64(maxLen)
}

// SequenceMatcherRatio implements a ratio similar to Python's difflib.SequenceMatcher.ratio()
// Uses the Ratcliff/Obershelp algorithm: 2 * matching_chars / total_chars
// This is faster than Levenshtein for large strings and better suited for code comparison
func SequenceMatcherRatio(s1, s2 string) float64 {
	if len(s1) == 0 && len(s2) == 0 {
		return 1.0
	}
	if len(s1) == 0 || len(s2) == 0 {
		return 0.0
	}

	matches := countMatchingChars(s1, s2)
	return 2.0 * float64(matches) / float64(len(s1)+len(s2))
}

// countMatchingChars recursively counts matching characters using longest common substring
// This is the core of the Ratcliff/Obershelp algorithm
func countMatchingChars(s1, s2 string) int {
	// Find longest common substring
	start1, start2, length := longestCommonSubstring(s1, s2)
	if length == 0 {
		return 0
	}

	// Count matches recursively on both sides
	matches := length

	// Left side
	if start1 > 0 && start2 > 0 {
		matches += countMatchingChars(s1[:start1], s2[:start2])
	}

	// Right side
	end1 := start1 + length
	end2 := start2 + length
	if end1 < len(s1) && end2 < len(s2) {
		matches += countMatchingChars(s1[end1:], s2[end2:])
	}

	return matches
}

// longestCommonSubstring finds the longest common substring between two strings
// Returns start positions in s1 and s2, and the length
func longestCommonSubstring(s1, s2 string) (start1, start2, length int) {
	if len(s1) == 0 || len(s2) == 0 {
		return 0, 0, 0
	}

	// For very long strings, use a simpler approach to avoid O(n*m) memory
	if len(s1) > 1000 || len(s2) > 1000 {
		return longestCommonSubstringLinear(s1, s2)
	}

	// Standard DP approach for shorter strings
	// Use rolling array to save memory: only need current and previous row
	prev := make([]int, len(s2)+1)
	curr := make([]int, len(s2)+1)

	maxLen := 0
	endPos1 := 0
	endPos2 := 0

	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			if s1[i-1] == s2[j-1] {
				curr[j] = prev[j-1] + 1
				if curr[j] > maxLen {
					maxLen = curr[j]
					endPos1 = i
					endPos2 = j
				}
			} else {
				curr[j] = 0
			}
		}
		prev, curr = curr, prev
		// Clear curr for next iteration
		for k := range curr {
			curr[k] = 0
		}
	}

	if maxLen == 0 {
		return 0, 0, 0
	}
	return endPos1 - maxLen, endPos2 - maxLen, maxLen
}

// longestCommonSubstringLinear finds LCS using a hash-based approach for long strings
// Less accurate but O(n+m) time and space
func longestCommonSubstringLinear(s1, s2 string) (start1, start2, length int) {
	// Use line-based comparison for very long strings (common in code)
	lines1 := strings.Split(s1, "\n")
	lines2 := strings.Split(s2, "\n")

	// Build index of lines in s2
	lineIndex := make(map[string][]int)
	for i, line := range lines2 {
		lineIndex[line] = append(lineIndex[line], i)
	}

	// Find longest run of matching lines
	maxRunLen := 0
	maxRunStart1 := 0
	maxRunStart2 := 0

	for i, line := range lines1 {
		for _, j := range lineIndex[line] {
			// Count consecutive matching lines
			runLen := 0
			for i+runLen < len(lines1) && j+runLen < len(lines2) &&
				lines1[i+runLen] == lines2[j+runLen] {
				runLen++
			}
			if runLen > maxRunLen {
				maxRunLen = runLen
				maxRunStart1 = i
				maxRunStart2 = j
			}
		}
	}

	if maxRunLen == 0 {
		return 0, 0, 0
	}

	// Convert line positions back to character positions
	charStart1 := 0
	for i := 0; i < maxRunStart1; i++ {
		charStart1 += len(lines1[i]) + 1
	}
	charStart2 := 0
	for i := 0; i < maxRunStart2; i++ {
		charStart2 += len(lines2[i]) + 1
	}

	// Calculate character length of matched lines
	charLen := 0
	for i := 0; i < maxRunLen; i++ {
		charLen += len(lines1[maxRunStart1+i])
		if i < maxRunLen-1 {
			charLen++ // newline
		}
	}

	return charStart1, charStart2, charLen
}

// FindBestMatch finds the best matching substring in content for the search string
// Uses LINE-BASED matching like Aider for performance (O(lines²) instead of O(chars²))
// Returns the start position, end position, similarity ratio, and the matched text
func (fm *FuzzyMatcher) FindBestMatch(content, search string) (start, end int, ratio float64, matched string, found bool) {
	if fm.Threshold <= 0 || len(search) == 0 {
		return 0, 0, 0, "", false
	}

	contentLines := strings.Split(content, "\n")
	searchLines := strings.Split(search, "\n")
	searchLineCount := len(searchLines)

	// Use ±10% scale on LINE count (like Aider), not characters
	scale := 0.1
	minLen := int(float64(searchLineCount) * (1 - scale))
	maxLen := int(float64(searchLineCount) * (1 + scale))

	if minLen < 1 {
		minLen = 1
	}
	if maxLen > len(contentLines) {
		maxLen = len(contentLines)
	}

	// Estimate if fuzzy search would take too long (~2 seconds budget)
	// Based on benchmarks: 500 lines * 3 window sizes ≈ 8.5ms
	// Cost scales with: iterations * avgChunkSize
	windowRange := maxLen - minLen + 1
	avgPositions := len(contentLines) - (minLen+maxLen)/2
	if avgPositions < 1 {
		avgPositions = 1
	}
	iterations := windowRange * avgPositions
	avgChunkChars := len(search) // approximate chunk size
	estimatedCost := iterations * avgChunkChars

	// Threshold calibrated from benchmarks:
	// 500 lines, 3 windows, ~150 chars/chunk = 225,000 cost units in 8.5ms
	// For 2 second budget: 225,000 * (2000/8.5) ≈ 53 million
	const maxCost = 50_000_000
	if estimatedCost > maxCost {
		return 0, 0, 0, "", false
	}

	bestRatio := 0.0
	bestStartLine := -1
	bestEndLine := -1

	// Iterate over line-based windows (much faster than character-based)
	for length := minLen; length <= maxLen; length++ {
		for i := 0; i <= len(contentLines)-length; i++ {
			chunk := strings.Join(contentLines[i:i+length], "\n")
			r := SequenceMatcherRatio(chunk, search)

			if r > bestRatio && r >= fm.Threshold {
				bestRatio = r
				bestStartLine = i
				bestEndLine = i + length
			}
		}
	}

	if bestStartLine < 0 {
		return 0, 0, 0, "", false
	}

	// Convert line indices back to byte positions
	byteStart := 0
	for i := 0; i < bestStartLine; i++ {
		byteStart += len(contentLines[i]) + 1 // +1 for newline
	}

	byteEnd := byteStart
	for i := bestStartLine; i < bestEndLine; i++ {
		byteEnd += len(contentLines[i])
		if i < bestEndLine-1 {
			byteEnd++ // Add newline between lines
		}
	}

	// Handle edge case: if content doesn't end with newline
	if byteEnd > len(content) {
		byteEnd = len(content)
	}

	matchedText := strings.Join(contentLines[bestStartLine:bestEndLine], "\n")
	return byteStart, byteEnd, bestRatio, matchedText, true
}

// FindMostSimilarLine finds the most similar line in content to the search string
// Useful for error messages suggesting what the user might have meant
func FindMostSimilarLine(content, search string) (lineNum int, line string, ratio float64) {
	lines := strings.Split(content, "\n")
	bestRatio := 0.0
	bestLineNum := 0
	bestLine := ""

	// Also try matching against first line of search if it's multiline
	searchFirstLine := search
	if idx := strings.Index(search, "\n"); idx > 0 {
		searchFirstLine = search[:idx]
	}

	for i, line := range lines {
		// Compare against full search
		r1 := SimilarityRatio(strings.TrimSpace(line), strings.TrimSpace(search))
		// Compare against first line of search
		r2 := SimilarityRatio(strings.TrimSpace(line), strings.TrimSpace(searchFirstLine))

		r := max(r1, r2)
		if r > bestRatio {
			bestRatio = r
			bestLineNum = i + 1
			bestLine = line
		}
	}

	return bestLineNum, bestLine, bestRatio
}

// FindSimilarChunk finds a chunk of lines similar to the search text
// Returns the start line number, the chunk, and similarity ratio
func FindSimilarChunk(content, search string, contextLines int) (startLine int, chunk string, ratio float64) {
	contentLines := strings.Split(content, "\n")
	searchLines := strings.Split(search, "\n")
	searchLineCount := len(searchLines)

	if searchLineCount == 0 {
		return 0, "", 0
	}

	bestRatio := 0.0
	bestStartLine := 0
	bestChunk := ""

	// Slide a window of searchLineCount lines through content
	for i := 0; i <= len(contentLines)-searchLineCount; i++ {
		chunk := strings.Join(contentLines[i:i+searchLineCount], "\n")
		r := SimilarityRatio(NormalizeWhitespace(chunk), NormalizeWhitespace(search))

		if r > bestRatio {
			bestRatio = r
			bestStartLine = i + 1
			bestChunk = chunk
		}
	}

	// If we found a good match, expand with context
	if bestRatio > 0.5 && contextLines > 0 {
		start := bestStartLine - 1 - contextLines
		if start < 0 {
			start = 0
		}
		end := bestStartLine - 1 + searchLineCount + contextLines
		if end > len(contentLines) {
			end = len(contentLines)
		}
		bestChunk = strings.Join(contentLines[start:end], "\n")
	}

	return bestStartLine, bestChunk, bestRatio
}

// MatchWithNormalization tries to match search text with progressive normalization levels
// Returns position and the normalization level that succeeded:
// 0 = exact, 1 = rstrip, 2 = full strip, 3 = fuzzy
func MatchWithNormalization(content, search string, fuzzyThreshold float64) (start, end int, level int, found bool) {
	// Level 0: Exact match
	if idx := strings.Index(content, search); idx >= 0 {
		return idx, idx + len(search), 0, true
	}

	// Level 1: Right-strip whitespace normalization
	normalizedContent := NormalizeWhitespaceRstrip(content)
	normalizedSearch := NormalizeWhitespaceRstrip(search)
	if idx := strings.Index(normalizedContent, normalizedSearch); idx >= 0 {
		// Map back to original position
		return mapNormalizedPositionToOriginal(content, normalizedContent, idx, len(normalizedSearch))
	}

	// Level 2: Full strip normalization
	normalizedContent = NormalizeWhitespace(content)
	normalizedSearch = NormalizeWhitespace(search)
	if idx := strings.Index(normalizedContent, normalizedSearch); idx >= 0 {
		return mapNormalizedPositionToOriginal(content, normalizedContent, idx, len(normalizedSearch))
	}

	// Level 3: Fuzzy match (if enabled)
	if fuzzyThreshold > 0 {
		matcher := NewFuzzyMatcher(fuzzyThreshold)
		start, end, _, _, found := matcher.FindBestMatch(content, search)
		if found {
			return start, end, 3, true
		}
	}

	return 0, 0, -1, false
}

// mapNormalizedPositionToOriginal maps a position in normalized text back to original
func mapNormalizedPositionToOriginal(original, normalized string, normStart, normLen int) (start, end int, level int, found bool) {
	// This is a simplified mapping - for more accurate mapping, we'd need to track
	// the character correspondence during normalization
	// For now, find the matching content in original by line-based matching

	// Split both into lines
	origLines := strings.Split(original, "\n")
	normLines := strings.Split(normalized, "\n")

	// Find which line the normalized position starts at
	lineStart := 0
	charCount := 0
	for i, line := range normLines {
		if charCount+len(line) >= normStart {
			lineStart = i
			break
		}
		charCount += len(line) + 1 // +1 for newline
	}

	// Find the same lines in original
	origStart := 0
	for i := 0; i < lineStart && i < len(origLines); i++ {
		origStart += len(origLines[i]) + 1
	}

	// Find end position by counting matched lines in normalized
	_ = normStart + normLen // normEndPos - used for reference but not needed for line-based mapping
	lineEnd := lineStart
	charCount = 0
	for i := lineStart; i < len(normLines); i++ {
		if charCount >= normLen {
			break
		}
		charCount += len(normLines[i]) + 1
		lineEnd = i
	}

	// Calculate end in original
	origEnd := origStart
	for i := lineStart; i <= lineEnd && i < len(origLines); i++ {
		origEnd += len(origLines[i])
		if i < lineEnd {
			origEnd++ // Add newline
		}
	}

	// Clamp to bounds
	if origEnd > len(original) {
		origEnd = len(original)
	}

	return origStart, origEnd, 1, true
}

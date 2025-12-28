package tools

import (
	"strings"
	"testing"
)

func TestSequenceMatcherRatio(t *testing.T) {
	tests := []struct {
		name     string
		s1       string
		s2       string
		wantMin  float64
		wantMax  float64
	}{
		{
			name:    "identical strings",
			s1:      "hello world",
			s2:      "hello world",
			wantMin: 1.0,
			wantMax: 1.0,
		},
		{
			name:    "empty strings",
			s1:      "",
			s2:      "",
			wantMin: 1.0,
			wantMax: 1.0,
		},
		{
			name:    "one empty string",
			s1:      "hello",
			s2:      "",
			wantMin: 0.0,
			wantMax: 0.0,
		},
		{
			name:    "completely different",
			s1:      "abc",
			s2:      "xyz",
			wantMin: 0.0,
			wantMax: 0.1,
		},
		{
			name:    "partial match",
			s1:      "hello world",
			s2:      "hello there",
			wantMin: 0.5,
			wantMax: 0.7,
		},
		{
			name:    "similar code blocks",
			s1:      "def foo():\n    return 42",
			s2:      "def foo():\n    return 43",
			wantMin: 0.9,
			wantMax: 1.0,
		},
		{
			name:    "whitespace difference",
			s1:      "func test() {\n    return nil\n}",
			s2:      "func test() {\n  return nil\n}",
			wantMin: 0.85,
			wantMax: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SequenceMatcherRatio(tt.s1, tt.s2)
			if got < tt.wantMin || got > tt.wantMax {
				t.Errorf("SequenceMatcherRatio() = %v, want between %v and %v", got, tt.wantMin, tt.wantMax)
			}
		})
	}
}

func TestLongestCommonSubstring(t *testing.T) {
	tests := []struct {
		name       string
		s1         string
		s2         string
		wantLen    int
		wantSubstr string
	}{
		{
			name:       "identical",
			s1:         "hello",
			s2:         "hello",
			wantLen:    5,
			wantSubstr: "hello",
		},
		{
			name:       "common middle",
			s1:         "abcdef",
			s2:         "xxcdexx",
			wantLen:    3,
			wantSubstr: "cde",
		},
		{
			name:       "no common",
			s1:         "abc",
			s2:         "xyz",
			wantLen:    0,
			wantSubstr: "",
		},
		{
			name:       "empty string",
			s1:         "",
			s2:         "hello",
			wantLen:    0,
			wantSubstr: "",
		},
		{
			name:       "multiline code",
			s1:         "func foo() {\n    return 1\n}",
			s2:         "func bar() {\n    return 1\n}",
			wantLen:    19,
			wantSubstr: "() {\n    return 1\n}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start1, _, length := longestCommonSubstring(tt.s1, tt.s2)
			if length != tt.wantLen {
				t.Errorf("longestCommonSubstring() length = %v, want %v", length, tt.wantLen)
			}
			if length > 0 {
				gotSubstr := tt.s1[start1 : start1+length]
				if gotSubstr != tt.wantSubstr {
					t.Errorf("longestCommonSubstring() substr = %q, want %q", gotSubstr, tt.wantSubstr)
				}
			}
		})
	}
}

func TestFindBestMatch_LineBased(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		search    string
		threshold float64
		wantFound bool
		wantRatio float64
	}{
		{
			name: "exact match",
			content: `func main() {
    fmt.Println("hello")
    return
}`,
			search: `    fmt.Println("hello")
    return`,
			threshold: 0.8,
			wantFound: true,
			wantRatio: 1.0,
		},
		{
			name: "fuzzy match with minor difference",
			content: `func main() {
    fmt.Println("hello")
    return
}`,
			search: `    fmt.Println("hello world")
    return`,
			threshold: 0.8,
			wantFound: true,
			wantRatio: 0.8,
		},
		{
			name: "no match below threshold",
			content: `func main() {
    fmt.Println("hello")
}`,
			search: `completely different
code block here`,
			threshold: 0.8,
			wantFound: false,
			wantRatio: 0.0,
		},
		{
			name: "match in middle of file",
			content: `package main

import "fmt"

func helper() {
    // helper code
}

func target() {
    doSomething()
    doMore()
}

func other() {
    // other code
}`,
			search: `func target() {
    doSomething()
    doMore()
}`,
			threshold: 0.8,
			wantFound: true,
			wantRatio: 1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fm := NewFuzzyMatcher(tt.threshold)
			_, _, ratio, _, found := fm.FindBestMatch(tt.content, tt.search)

			if found != tt.wantFound {
				t.Errorf("FindBestMatch() found = %v, want %v", found, tt.wantFound)
			}
			if found && ratio < tt.wantRatio {
				t.Errorf("FindBestMatch() ratio = %v, want >= %v", ratio, tt.wantRatio)
			}
		})
	}
}

func TestFindBestMatch_Performance(t *testing.T) {
	// Test that line-based matching is fast even for large files
	// Generate a large file content (500 lines)
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, "    line number "+string(rune('0'+i%10))+" with some content here")
	}
	content := strings.Join(lines, "\n")

	// Search for a 6-line block
	search := strings.Join(lines[200:206], "\n")

	fm := NewFuzzyMatcher(0.8)

	// This should complete very quickly (< 100ms)
	start, end, ratio, _, found := fm.FindBestMatch(content, search)

	if !found {
		t.Error("Expected to find match in large file")
	}
	if ratio < 0.99 {
		t.Errorf("Expected high ratio for exact match, got %v", ratio)
	}
	if start < 0 || end <= start {
		t.Errorf("Invalid positions: start=%d, end=%d", start, end)
	}
}

func TestFindBestMatch_PositionAccuracy(t *testing.T) {
	content := `line one
line two
target line A
target line B
target line C
line six
line seven`

	search := `target line A
target line B
target line C`

	fm := NewFuzzyMatcher(0.8)
	start, end, _, matched, found := fm.FindBestMatch(content, search)

	if !found {
		t.Fatal("Expected to find match")
	}

	// Verify the matched text is correct
	if matched != search {
		t.Errorf("Matched text = %q, want %q", matched, search)
	}

	// Verify byte positions are correct
	extracted := content[start:end]
	if extracted != search {
		t.Errorf("Extracted from positions = %q, want %q", extracted, search)
	}
}

func TestMatchWithNormalization_FuzzyFallback(t *testing.T) {
	content := `func example() {
    oldCode := "value"
    return oldCode
}`

	// Slightly different search that won't match exactly
	search := `func example() {
    oldCode := "different"
    return oldCode
}`

	// Should fall back to fuzzy matching (level 3)
	start, end, level, found := MatchWithNormalization(content, search, 0.8)

	if !found {
		t.Error("Expected fuzzy match to succeed")
	}
	if level != 3 {
		t.Errorf("Expected level 3 (fuzzy), got %d", level)
	}
	if start < 0 || end <= start {
		t.Errorf("Invalid positions: start=%d, end=%d", start, end)
	}
}

func TestMatchWithNormalization_ExactFirst(t *testing.T) {
	content := `func example() {
    return 42
}`
	search := `func example() {
    return 42
}`

	start, end, level, found := MatchWithNormalization(content, search, 0.8)

	if !found {
		t.Error("Expected exact match to succeed")
	}
	if level != 0 {
		t.Errorf("Expected level 0 (exact), got %d", level)
	}
	if content[start:end] != search {
		t.Errorf("Extracted = %q, want %q", content[start:end], search)
	}
}

func TestFindBestMatch_SkipsTooExpensive(t *testing.T) {
	// Generate a very large file that would be too expensive to fuzzy match
	var lines []string
	for i := 0; i < 10000; i++ {
		lines = append(lines, "line number with some content that makes it longer for the test case here")
	}
	content := strings.Join(lines, "\n")

	// Large search block
	search := strings.Join(lines[5000:5100], "\n") // 100 lines

	fm := NewFuzzyMatcher(0.8)
	_, _, _, _, found := fm.FindBestMatch(content, search)

	// Should skip because estimated cost is too high
	// 10000 lines * ~20 window sizes * ~7500 chars = way over 50 million
	if found {
		t.Error("Expected fuzzy match to be skipped for expensive search")
	}
}

func TestFindBestMatch_AllowsReasonableSize(t *testing.T) {
	// Generate a moderate-sized file that should still work
	var lines []string
	for i := 0; i < 500; i++ {
		lines = append(lines, "line number "+string(rune('0'+i%10)))
	}
	content := strings.Join(lines, "\n")

	// Small search block
	search := strings.Join(lines[200:205], "\n") // 5 lines

	fm := NewFuzzyMatcher(0.8)
	_, _, ratio, _, found := fm.FindBestMatch(content, search)

	if !found {
		t.Error("Expected fuzzy match to work for reasonable size")
	}
	if ratio < 0.99 {
		t.Errorf("Expected high ratio for exact match, got %v", ratio)
	}
}

func BenchmarkSequenceMatcherRatio(b *testing.B) {
	s1 := strings.Repeat("func example() {\n    return 42\n}\n", 10)
	s2 := strings.Repeat("func example() {\n    return 43\n}\n", 10)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SequenceMatcherRatio(s1, s2)
	}
}

func BenchmarkFindBestMatch_SmallFile(b *testing.B) {
	content := strings.Repeat("line of code here\n", 100)
	search := "line of code here\nline of code here\nline of code here"
	fm := NewFuzzyMatcher(0.8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fm.FindBestMatch(content, search)
	}
}

func BenchmarkFindBestMatch_LargeFile(b *testing.B) {
	content := strings.Repeat("line of code here with more content\n", 500)
	search := "line of code here with more content\nline of code here with more content\nline of code here with more content"
	fm := NewFuzzyMatcher(0.8)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		fm.FindBestMatch(content, search)
	}
}

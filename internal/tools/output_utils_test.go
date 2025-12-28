package tools

import (
	"strings"
	"testing"
)

func TestTruncateContent_SmallContent(t *testing.T) {
	content := []byte("Line 1\nLine 2\nLine 3\n")

	result := TruncateContent(content, 150, 24*1024, 75, 12*1024)

	if result.WasTruncated {
		t.Errorf("expected no truncation for small content")
	}

	if result.Content != string(content) {
		t.Errorf("content should be unchanged for small content")
	}

	if result.TotalLines != 3 {
		t.Errorf("expected 3 lines, got %d", result.TotalLines)
	}

	if result.TotalBytes != len(content) {
		t.Errorf("expected %d bytes, got %d", len(content), result.TotalBytes)
	}
}

func TestTruncateContent_LargeContent(t *testing.T) {
	// Create content with 300 lines (exceeds 150 limit)
	var lines []string
	for i := 1; i <= 300; i++ {
		lines = append(lines, "Line "+string(rune('0'+i%10)))
	}
	content := []byte(strings.Join(lines, "\n") + "\n")

	// Truncate to 150 lines max, 75 per side
	result := TruncateContent(content, 150, 24*1024, 75, 12*1024)

	if !result.WasTruncated {
		t.Errorf("expected truncation for large content")
	}

	if result.TotalLines != 300 {
		t.Errorf("expected 300 total lines, got %d", result.TotalLines)
	}

	// Check that truncation marker is present
	if !strings.Contains(result.Content, "truncated") {
		t.Errorf("expected truncation marker in content")
	}
}

func TestTruncateContent_LargeBytes(t *testing.T) {
	// Create content larger than 24KB
	content := make([]byte, 50*1024) // 50KB of data
	for i := range content {
		content[i] = 'A'
	}

	result := TruncateContent(content, 150, 24*1024, 75, 12*1024)

	if !result.WasTruncated {
		t.Errorf("expected truncation for content exceeding byte limit")
	}

	if result.TotalBytes != len(content) {
		t.Errorf("expected %d total bytes, got %d", len(content), result.TotalBytes)
	}
}

func TestTruncateBothEnds(t *testing.T) {
	var lines []string
	for i := 1; i <= 100; i++ {
		lines = append(lines, "Line "+string(rune('0'+i%10)))
	}
	content := []byte(strings.Join(lines, "\n"))

	firstPart, lastPart := truncateBothEnds(content, 20, 10000)

	// Should have first 10 and last 10 lines
	if !strings.Contains(firstPart, "Line") {
		t.Errorf("first part should contain lines")
	}

	if !strings.Contains(lastPart, "Line") {
		t.Errorf("last part should contain lines")
	}

	// First part should start with first line
	if !strings.HasPrefix(firstPart, "Line") {
		t.Errorf("first part should start with first line")
	}
}

func TestTruncateContent_ExactLimit(t *testing.T) {
	// Create content exactly at the limit (150 lines)
	var lines []string
	for i := 1; i <= 150; i++ {
		lines = append(lines, "Line")
	}
	content := []byte(strings.Join(lines, "\n") + "\n")

	result := TruncateContent(content, 150, 24*1024, 75, 12*1024)

	// Should NOT be truncated since it's exactly at the limit
	if result.WasTruncated {
		t.Errorf("expected no truncation for content at exact limit")
	}
}

func TestTruncateContent_OneOverLimit(t *testing.T) {
	// Create content just over the limit (151 lines)
	var lines []string
	for i := 1; i <= 151; i++ {
		lines = append(lines, "Line")
	}
	content := []byte(strings.Join(lines, "\n") + "\n")

	result := TruncateContent(content, 150, 24*1024, 75, 12*1024)

	// Should be truncated since it's over the limit
	if !result.WasTruncated {
		t.Errorf("expected truncation for content over limit")
	}

	if result.TotalLines != 151 {
		t.Errorf("expected 151 total lines, got %d", result.TotalLines)
	}
}

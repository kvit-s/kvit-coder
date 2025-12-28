package tools

import (
	"os"
	"strings"
	"testing"
)

func TestOutputBuffer_SmallOutput(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()

	buf := NewOutputBuffer(tempMgr)
	defer buf.Close()

	// Write small output (under thresholds)
	smallOutput := "Line 1\nLine 2\nLine 3\n"
	_, _ = buf.Write([]byte(smallOutput))

	result, err := buf.FormatForLLM()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should get full output without truncation message
	if result != smallOutput {
		t.Errorf("Expected full output, got truncated")
	}
	if strings.Contains(result, "Output truncated") {
		t.Error("Expected no truncation message for small output")
	}
}

func TestOutputBuffer_LargeLineCount(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()

	buf := NewOutputBuffer(tempMgr)
	defer buf.Close()

	// Write 200 lines (exceeds 150 line threshold but under 24KB)
	var sb strings.Builder
	for i := 1; i <= 200; i++ {
		sb.WriteString("Line ")
		sb.WriteString(strings.Repeat("X", 10))
		sb.WriteString("\n")
	}
	_, _ = buf.Write([]byte(sb.String()))

	result, err := buf.FormatForLLM()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should be truncated
	if !strings.Contains(result, "SHELL OUTPUT TRUNCATED") {
		t.Error("Expected truncation message for large line count")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("Expected truncation marker")
	}

	// Verify both beginning and end are present
	// The first line should be from the beginning
	if !strings.Contains(result, "Line XXXXXXXXXX") {
		t.Error("Expected beginning lines to be present")
	}
}

func TestOutputBuffer_LargeByteCount(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()

	buf := NewOutputBuffer(tempMgr)
	defer buf.Close()

	// Write 50KB (exceeds 24KB threshold)
	largeOutput := strings.Repeat("X", 50*1024)
	_, _ = buf.Write([]byte(largeOutput))

	result, err := buf.FormatForLLM()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should be truncated
	if !strings.Contains(result, "SHELL OUTPUT TRUNCATED") {
		t.Error("Expected truncation message for large byte count")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("Expected truncation marker")
	}

	// Result should be under 15KB (12KB data split between first/last + truncation message)
	if len(result) > 15*1024 {
		t.Errorf("Expected result under 15KB, got %d bytes", len(result))
	}
}

func TestOutputBuffer_SpillToTempFile(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()

	buf := NewOutputBuffer(tempMgr)
	defer buf.Close()

	// Write 25MB (exceeds 20MB threshold, should spill to file)
	chunkSize := 1024 * 1024 // 1MB chunks
	chunk := strings.Repeat("X", chunkSize)
	for i := 0; i < 25; i++ {
		_, _ = buf.Write([]byte(chunk))
	}

	result, err := buf.FormatForLLM()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should be truncated and mention temp file
	if !strings.Contains(result, "SHELL OUTPUT TRUNCATED") {
		t.Error("Expected truncation message for spilled output")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("Expected truncation marker")
	}
	if !strings.Contains(result, "Complete output saved to:") {
		t.Error("Expected temp file path in truncation message")
	}
	if !strings.Contains(result, "investigate") {
		t.Error("Expected investigation instructions in truncation message")
	}
}

func TestOutputBuffer_ExactlyAtThreshold(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()

	buf := NewOutputBuffer(tempMgr)
	defer buf.Close()

	// Write exactly 150 lines with total size under 24KB
	// This should NOT be truncated (threshold is <= 150)
	var sb strings.Builder
	for i := 1; i <= 150; i++ {
		sb.WriteString("Line ")
		sb.WriteString(strings.Repeat("X", 10))
		sb.WriteString("\n")
	}
	output := sb.String()
	_, _ = buf.Write([]byte(output))

	result, err := buf.FormatForLLM()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should NOT be truncated (at threshold, which is allowed)
	if strings.Contains(result, "OUTPUT TRUNCATED") {
		t.Error("Expected no truncation at exactly 150 lines")
	}
	if strings.Contains(result, "BEGINNING OF OUTPUT") {
		t.Error("Expected no 'BEGINNING OF OUTPUT' marker at threshold")
	}
	if result != output {
		t.Error("Expected full output at threshold")
	}
}

func TestOutputBuffer_JustAboveThreshold(t *testing.T) {
	tempMgr := NewTempFileManager(os.TempDir())
	defer tempMgr.CleanupAll()

	buf := NewOutputBuffer(tempMgr)
	defer buf.Close()

	// Write 151 lines with total size under 24KB
	// This should be truncated (over the 150 line threshold)
	var sb strings.Builder
	for i := 1; i <= 151; i++ {
		sb.WriteString("Line ")
		sb.WriteString(strings.Repeat("X", 10))
		sb.WriteString("\n")
	}
	_, _ = buf.Write([]byte(sb.String()))

	result, err := buf.FormatForLLM()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Should be truncated
	if !strings.Contains(result, "SHELL OUTPUT TRUNCATED") {
		t.Error("Expected truncation for 151 lines")
	}
	if !strings.Contains(result, "truncated") {
		t.Error("Expected truncation marker")
	}
}

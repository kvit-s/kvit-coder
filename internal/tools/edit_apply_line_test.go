package tools

import (
	"testing"
)

func TestApplyLineEdit(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		startLine int
		endLine   int
		newText   string
		expected  string
		wantErr   bool
	}{
		{
			name:      "replace single line",
			content:   "line1\nline2\nline3\n",
			startLine: 2,
			endLine:   2,
			newText:   "replaced\n",
			expected:  "line1\nreplaced\nline3\n",
		},
		{
			name:      "replace multiple lines",
			content:   "line1\nline2\nline3\nline4\n",
			startLine: 2,
			endLine:   3,
			newText:   "replaced\n",
			expected:  "line1\nreplaced\nline4\n",
		},
		{
			name:      "delete single line with empty string",
			content:   "line1\nline2\nline3\n",
			startLine: 2,
			endLine:   2,
			newText:   "",
			expected:  "line1\nline3\n",
		},
		{
			name:      "delete multiple lines with empty string",
			content:   "line1\nline2\nline3\nline4\nline5\n",
			startLine: 2,
			endLine:   4,
			newText:   "",
			expected:  "line1\nline5\n",
		},
		{
			name:      "delete first line",
			content:   "line1\nline2\nline3\n",
			startLine: 1,
			endLine:   1,
			newText:   "",
			expected:  "line2\nline3\n",
		},
		{
			name:      "delete last line",
			content:   "line1\nline2\nline3",
			startLine: 3,
			endLine:   3,
			newText:   "",
			expected:  "line1\nline2\n",
		},
		{
			name:      "insert mode (endLine=0)",
			content:   "line1\nline2\nline3\n",
			startLine: 2,
			endLine:   0,
			newText:   "inserted\n",
			expected:  "line1\ninserted\nline2\nline3\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, _, _, err := ApplyLineEdit(tt.content, tt.startLine, tt.endLine, tt.newText)
			if (err != nil) != tt.wantErr {
				t.Errorf("ApplyLineEdit() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if result != tt.expected {
				t.Errorf("ApplyLineEdit() = %q, want %q", result, tt.expected)
			}
		})
	}
}

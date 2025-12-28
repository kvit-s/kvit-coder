package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeAndValidatePath(t *testing.T) {
	// Get current working directory for tests
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	workspaceRoot := filepath.Join(cwd, "testworkspace")

	tests := []struct {
		name          string
		inputPath     string
		expectedPath  string
		expectedOut   bool
		expectError   bool
	}{
		{
			name:          "Relative path within workspace",
			inputPath:     "foo/bar.txt",
			expectedPath:  filepath.Join(workspaceRoot, "foo/bar.txt"),
			expectedOut:   false,
			expectError:   false,
		},
		{
			name:          "Absolute path within workspace",
			inputPath:     filepath.Join(workspaceRoot, "file.txt"),
			expectedPath:  filepath.Join(workspaceRoot, "file.txt"),
			expectedOut:   false,
			expectError:   false,
		},
		{
			name:          "Path with .. escaping workspace",
			inputPath:     "../../etc/passwd",
			expectedPath:  filepath.Clean(filepath.Join(workspaceRoot, "../../etc/passwd")),
			expectedOut:   true,
			expectError:   false,
		},
		{
			name:          "Absolute path outside workspace",
			inputPath:     "/etc/passwd",
			expectedPath:  "/etc/passwd",
			expectedOut:   true,
			expectError:   false,
		},
		{
			name:          "Path with . should normalize",
			inputPath:     "./foo/./bar.txt",
			expectedPath:  filepath.Join(workspaceRoot, "foo/bar.txt"),
			expectedOut:   false,
			expectError:   false,
		},
		{
			name:          "Current directory",
			inputPath:     ".",
			expectedPath:  workspaceRoot,
			expectedOut:   false,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			normalizedPath, outside, err := NormalizeAndValidatePath(workspaceRoot, tt.inputPath)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if normalizedPath != tt.expectedPath {
				t.Errorf("expected path %q, got %q", tt.expectedPath, normalizedPath)
			}

			if outside != tt.expectedOut {
				t.Errorf("expected outside=%v, got outside=%v", tt.expectedOut, outside)
			}
		})
	}
}

func TestNormalizeAndValidatePath_HomeDirExpansion(t *testing.T) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home directory")
	}

	workspaceRoot := "/workspace"

	normalizedPath, outside, err := NormalizeAndValidatePath(workspaceRoot, "~/test.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedPath := filepath.Join(homeDir, "test.txt")
	if normalizedPath != expectedPath {
		t.Errorf("expected %q, got %q", expectedPath, normalizedPath)
	}

	// Home dir is typically outside workspace
	if !outside {
		t.Errorf("expected home dir to be outside workspace")
	}
}

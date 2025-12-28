package tools

import (
	"os"
	"path/filepath"
	"strings"
)

// NormalizeAndValidatePath normalizes a path and checks if it's outside workspace
// Returns: (normalizedPath, isOutside, error)
func NormalizeAndValidatePath(workspaceRoot, inputPath string) (string, bool, error) {
	// Step 1: Expand ~ to home directory
	path := inputPath
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", false, err
		}
		path = filepath.Join(home, path[2:])
	}

	// Step 2: Convert to absolute path
	var absPath string
	if filepath.IsAbs(path) {
		absPath = path
	} else {
		absPath = filepath.Join(workspaceRoot, path)
	}

	// Step 3: Clean the path to resolve .. and .
	absPath = filepath.Clean(absPath)
	workspaceAbs := filepath.Clean(workspaceRoot)

	// Step 4: Check if path is outside workspace
	relPath, err := filepath.Rel(workspaceAbs, absPath)
	if err != nil {
		return "", false, err
	}

	// Step 5: If relative path starts with "..", it's outside workspace
	outside := strings.HasPrefix(relPath, "..")
	return absPath, outside, nil
}

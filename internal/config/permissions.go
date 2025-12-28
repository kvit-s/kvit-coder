package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// AccessType defines the type of file access being requested
type AccessType int

const (
	AccessRead AccessType = iota
	AccessWrite
)

// PermissionResult indicates the result of a permission check
type PermissionResult int

const (
	PermissionGranted PermissionResult = iota
	PermissionReadOnly
	PermissionDenied
	PermissionPromptRequired
)

// CheckPathPermission validates if a path can be accessed based on workspace config
func (c *Config) CheckPathPermission(path string, accessType AccessType) (PermissionResult, error) {
	// Resolve path relative to workspace root (not current working directory)
	var absPath string
	if filepath.IsAbs(path) {
		absPath = filepath.Clean(path)
	} else {
		// Relative paths are resolved against workspace root
		absPath = filepath.Clean(filepath.Join(c.Workspace.Root, path))
	}

	// Check denied paths first (highest priority)
	for _, denied := range c.Workspace.DeniedPaths {
		deniedAbs, _ := filepath.Abs(expandPath(denied))
		if strings.HasPrefix(absPath, deniedAbs) {
			return PermissionDenied, fmt.Errorf("path is in denied_paths")
		}
	}

	// Check if within workspace root
	workspaceAbs, _ := filepath.Abs(c.Workspace.Root)
	if strings.HasPrefix(absPath, workspaceAbs) {
		return PermissionGranted, nil
	}

	// Check allowed_paths (read+write)
	for _, allowed := range c.Workspace.AllowedPaths {
		allowedAbs, _ := filepath.Abs(expandPath(allowed))
		if strings.HasPrefix(absPath, allowedAbs) {
			return PermissionGranted, nil
		}
	}

	// Check allowed_read_paths (read-only)
	for _, allowedRead := range c.Workspace.AllowedReadPaths {
		allowedReadAbs, _ := filepath.Abs(expandPath(allowedRead))
		if strings.HasPrefix(absPath, allowedReadAbs) {
			if accessType == AccessWrite {
				return PermissionReadOnly, fmt.Errorf("path is read-only")
			}
			return PermissionGranted, nil
		}
	}

	// Path is outside workspace and not in any allowed list
	// Check path_safety_mode to determine how to handle
	switch c.Workspace.PathSafetyMode {
	case "block":
		return PermissionDenied, fmt.Errorf("path outside workspace")
	case "warn", "ask_once", "ask_always":
		// Let CheckPathSafety handle these modes
		return PermissionPromptRequired, nil
	default:
		// Legacy behavior: use AllowOutsideWorkspace flag
		if c.Workspace.AllowOutsideWorkspace {
			return PermissionPromptRequired, nil
		}
		return PermissionDenied, fmt.Errorf("path outside workspace")
	}
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

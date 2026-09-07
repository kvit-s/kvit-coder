package prompt

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

// environmentTimeout bounds the git calls. The prompt must be ready whether or
// not git answers.
const environmentTimeout = 2 * time.Second

// maxListedEntries is how much of the top-level listing to include. Enough to
// recognise the project, not enough to be a directory dump.
const maxListedEntries = 40

// Environment describes where this session is running, so the model does not
// have to spend its first three tool calls finding out. None of it is
// derivable from the tool documentation, and all of it changes per session,
// which is exactly what belongs in a prompt and not in a template.
func Environment(workspaceRoot string) string {
	var sb strings.Builder
	sb.WriteString("# ENVIRONMENT\n")
	fmt.Fprintf(&sb, "Working directory: %s\n", workspaceRoot)
	fmt.Fprintf(&sb, "Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	if shell := os.Getenv("SHELL"); shell != "" {
		fmt.Fprintf(&sb, "Shell: %s\n", shell)
	}
	fmt.Fprintf(&sb, "Today: %s\n", time.Now().Format("2006-01-02"))

	if branch := gitOutput(workspaceRoot, "rev-parse", "--abbrev-ref", "HEAD"); branch != "" {
		fmt.Fprintf(&sb, "Git branch: %s\n", branch)
		if status := gitStatus(workspaceRoot); status != "" {
			sb.WriteString("Uncommitted changes:\n")
			sb.WriteString(status)
			sb.WriteString("\n")
		} else {
			sb.WriteString("Working tree is clean.\n")
		}
	}

	if listing := topLevel(workspaceRoot); listing != "" {
		sb.WriteString("Top level:\n")
		sb.WriteString(listing)
		sb.WriteString("\n")
	}

	sb.WriteString("\n")
	return sb.String()
}

func gitOutput(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), environmentTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// gitStatus lists what is modified, capped: the point is to say that the tree
// is dirty and roughly where, not to reproduce git status.
func gitStatus(dir string) string {
	out := gitOutput(dir, "status", "--short", "--untracked-files=normal")
	if out == "" {
		return ""
	}
	lines := strings.Split(out, "\n")
	if len(lines) > 20 {
		extra := len(lines) - 20
		lines = append(lines[:20], fmt.Sprintf("  … and %d more", extra))
	}
	for i, line := range lines {
		if !strings.HasPrefix(line, "  ") {
			lines[i] = "  " + line
		}
	}
	return strings.Join(lines, "\n")
}

// topLevel lists what is in the workspace root, directories marked with a
// trailing slash, so the model can recognise the shape of the project.
func topLevel(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if e.IsDir() {
			name += string(filepath.Separator)
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return ""
	}
	sort.Strings(names)
	truncated := false
	if len(names) > maxListedEntries {
		names = names[:maxListedEntries]
		truncated = true
	}
	line := "  " + strings.Join(names, " ")
	if truncated {
		line += " …"
	}
	return line
}

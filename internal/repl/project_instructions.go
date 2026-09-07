package repl

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/kvit-s/kvit-coder/internal/config"
	"github.com/kvit-s/kvit-coder/internal/llm"
)

// ProjectInstructions is a project-local instruction file loaded at startup.
type ProjectInstructions struct {
	Path    string
	Content string
}

// LoadProjectInstructions loads the configured project instruction file.
// Missing or empty files are treated as absent.
func LoadProjectInstructions(cfg *config.Config, launchDir string) (*ProjectInstructions, error) {
	if cfg == nil || !cfg.Agent.ProjectInstructions.IsEnabled() {
		return nil, nil
	}

	path, err := resolveProjectInstructionsPath(cfg.Agent.ProjectInstructions.PathOrDefault(), launchDir)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("read project instructions %q: %w", path, err)
	}

	content := string(data)
	if strings.TrimSpace(content) == "" {
		return nil, nil
	}

	return &ProjectInstructions{
		Path:    path,
		Content: content,
	}, nil
}

func resolveProjectInstructionsPath(path, launchDir string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", nil
	}

	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory for project instructions: %w", err)
		}
		if path == "~" {
			path = home
		} else {
			path = filepath.Join(home, strings.TrimPrefix(path, "~/"))
		}
	}

	if !filepath.IsAbs(path) {
		if launchDir == "" {
			var err error
			launchDir, err = os.Getwd()
			if err != nil {
				return "", fmt.Errorf("resolve launch directory: %w", err)
			}
		}
		path = filepath.Join(launchDir, path)
	}

	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve project instructions path %q: %w", path, err)
	}
	return abs, nil
}

func promptWithProjectInstructions(promptText string, instructions *ProjectInstructions) string {
	if instructions == nil {
		return promptText
	}
	return projectInstructionsPrefix(instructions) + promptText
}

func stripProjectInstructions(messages []llm.Message, instructions *ProjectInstructions) []llm.Message {
	if instructions == nil {
		return messages
	}

	prefix := projectInstructionsPrefix(instructions)
	stripped := make([]llm.Message, len(messages))
	copy(stripped, messages)
	for i := range stripped {
		if stripped[i].Role == llm.RoleUser && strings.HasPrefix(stripped[i].Content, prefix) {
			stripped[i].Content = strings.TrimPrefix(stripped[i].Content, prefix)
		}
	}
	return stripped
}

func projectInstructionsPrefix(instructions *ProjectInstructions) string {
	content := instructions.Content
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	return fmt.Sprintf("Project instructions loaded from %s:\n\n<project_instructions>\n%s</project_instructions>\n\nUser request:\n", instructions.Path, content)
}

package prompt

import (
	"bytes"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// TemplateEngine wraps Go's text/template with prompt-specific functionality.
type TemplateEngine struct {
	templates *template.Template
	fs        fs.FS
}

// NewTemplateEngine creates a new template engine from the given filesystem.
// The filesystem should contain template files in the expected directory structure.
func NewTemplateEngine(templatesFS fs.FS) (*TemplateEngine, error) {
	engine := &TemplateEngine{
		fs: templatesFS,
	}

	// Create base template with custom functions
	engine.templates = template.New("").Funcs(templateFuncs())

	// Parse all templates from the filesystem
	if err := engine.parseAllTemplates(); err != nil {
		return nil, fmt.Errorf("parse templates: %w", err)
	}

	return engine, nil
}

// parseAllTemplates walks the filesystem and parses all .tmpl files.
func (e *TemplateEngine) parseAllTemplates() error {
	return fs.WalkDir(e.fs, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-template files
		if d.IsDir() || !strings.HasSuffix(path, ".tmpl") {
			return nil
		}

		// Read template content
		content, err := fs.ReadFile(e.fs, path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		// Parse template with its path as the name
		_, err = e.templates.New(path).Parse(string(content))
		if err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		return nil
	})
}

// Render executes the named template with the given context.
func (e *TemplateEngine) Render(name string, ctx PromptContext) (string, error) {
	var buf bytes.Buffer
	if err := e.templates.ExecuteTemplate(&buf, name, ctx); err != nil {
		return "", fmt.Errorf("execute template %s: %w", name, err)
	}
	return buf.String(), nil
}

// RenderString renders a template string directly (for dynamic tool prompts).
func (e *TemplateEngine) RenderString(tmplStr string, ctx PromptContext) (string, error) {
	tmpl, err := template.New("dynamic").Funcs(templateFuncs()).Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template string: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return "", fmt.Errorf("execute template string: %w", err)
	}
	return buf.String(), nil
}

// HasTemplate checks if a template with the given name exists.
func (e *TemplateEngine) HasTemplate(name string) bool {
	return e.templates.Lookup(name) != nil
}

// ListTemplates returns all loaded template names.
func (e *TemplateEngine) ListTemplates() []string {
	var names []string
	for _, t := range e.templates.Templates() {
		if t.Name() != "" {
			names = append(names, t.Name())
		}
	}
	return names
}

// Reload re-parses all templates from the filesystem.
// Useful for hot-reload during development.
func (e *TemplateEngine) Reload() error {
	// Create fresh template set
	e.templates = template.New("").Funcs(templateFuncs())
	return e.parseAllTemplates()
}

// layeredFS implements fs.FS with fallback from override to base filesystem.
// Files in the override FS take precedence over files in the base FS.
type layeredFS struct {
	override fs.FS
	base     fs.FS
}

// Open implements fs.FS, checking override first then base.
func (l *layeredFS) Open(name string) (fs.File, error) {
	if f, err := l.override.Open(name); err == nil {
		return f, nil
	}
	return l.base.Open(name)
}

// ReadDir implements fs.ReadDirFS for directory listing.
func (l *layeredFS) ReadDir(name string) ([]fs.DirEntry, error) {
	// Collect entries from both filesystems
	seen := make(map[string]fs.DirEntry)

	// Add base entries first
	if baseRD, ok := l.base.(fs.ReadDirFS); ok {
		if entries, err := baseRD.ReadDir(name); err == nil {
			for _, e := range entries {
				seen[e.Name()] = e
			}
		}
	}

	// Override with custom entries
	if overrideRD, ok := l.override.(fs.ReadDirFS); ok {
		if entries, err := overrideRD.ReadDir(name); err == nil {
			for _, e := range entries {
				seen[e.Name()] = e
			}
		}
	}

	// Convert to slice
	result := make([]fs.DirEntry, 0, len(seen))
	for _, e := range seen {
		result = append(result, e)
	}

	// Sort for consistent ordering
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name() < result[j].Name()
	})

	return result, nil
}

// ReadFile implements fs.ReadFileFS for reading file contents.
func (l *layeredFS) ReadFile(name string) ([]byte, error) {
	// Try override first
	if rf, ok := l.override.(fs.ReadFileFS); ok {
		if data, err := rf.ReadFile(name); err == nil {
			return data, nil
		}
	}
	// Fall back to base
	if rf, ok := l.base.(fs.ReadFileFS); ok {
		return rf.ReadFile(name)
	}
	return nil, fmt.Errorf("ReadFile not supported")
}

// collectTemplateFiles returns all .tmpl files in the filesystem.
func collectTemplateFiles(fsys fs.FS) ([]string, error) {
	var files []string
	err := fs.WalkDir(fsys, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".tmpl") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// LoadTemplates creates a template engine, preferring filesystem override if configured.
// When templates_dir is set, it layers custom templates over embedded ones.
func LoadTemplates(cfg TemplateConfig) (*TemplateEngine, error) {
	var templatesFS fs.FS
	var overridden, embedded []string

	if cfg.TemplatesDir != "" {
		// Check override directory exists
		if _, err := os.Stat(cfg.TemplatesDir); err != nil {
			return nil, fmt.Errorf("templates directory not found: %s", cfg.TemplatesDir)
		}

		overrideFS := os.DirFS(cfg.TemplatesDir)

		// Collect files from override directory
		overrideFiles, err := collectTemplateFiles(overrideFS)
		if err != nil {
			return nil, fmt.Errorf("scan override templates: %w", err)
		}
		overrideSet := make(map[string]bool)
		for _, f := range overrideFiles {
			overrideSet[f] = true
			overridden = append(overridden, f)
		}

		// Collect files from embedded that aren't overridden
		embeddedFiles, err := collectTemplateFiles(embeddedPrompts)
		if err != nil {
			return nil, fmt.Errorf("scan embedded templates: %w", err)
		}
		for _, f := range embeddedFiles {
			if !overrideSet[f] {
				embedded = append(embedded, f)
			}
		}

		// Log template sources
		sort.Strings(overridden)
		sort.Strings(embedded)

		if len(overridden) > 0 {
			log.Printf("Templates from %s: %d files", cfg.TemplatesDir, len(overridden))
			for _, f := range overridden {
				log.Printf("  [override] %s", f)
			}
		}
		if len(embedded) > 0 {
			log.Printf("Templates from embedded: %d files", len(embedded))
			for _, f := range embedded {
				log.Printf("  [embedded] %s", f)
			}
		}

		// Create layered filesystem
		templatesFS = &layeredFS{
			override: overrideFS,
			base:     embeddedPrompts,
		}
	} else {
		// Use embedded templates only
		templatesFS = embeddedPrompts
	}

	return NewTemplateEngine(templatesFS)
}

// TemplateConfig holds configuration for template loading.
type TemplateConfig struct {
	// TemplatesDir overrides embedded templates with filesystem directory
	TemplatesDir string

	// HotReload enables template reloading (dev mode)
	HotReload bool
}

// CreateTemplatesDir creates the prompts directory structure for custom templates.
func CreateTemplatesDir(baseDir string) error {
	dirs := []string{
		filepath.Join(baseDir, "prompts"),
		filepath.Join(baseDir, "prompts", "sections"),
		filepath.Join(baseDir, "prompts", "sections", "examples"),
		filepath.Join(baseDir, "prompts", "tools"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("create %s: %w", dir, err)
		}
	}

	return nil
}

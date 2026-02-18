package steps

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/bmatcuk/doublestar/v4"
	"github.com/systemstart/many-templates/pkg/api"
)

type templateStep struct {
	name string
	cfg  *api.TemplateConfig
}

// NewTemplateStep creates a template step.
func NewTemplateStep(name string, cfg *api.TemplateConfig) Step {
	return &templateStep{name: name, cfg: cfg}
}

func (s *templateStep) Name() string { return s.name }

func (s *templateStep) Run(ctx StepContext) (*StepResult, error) {
	files, err := filterFiles(os.DirFS(ctx.WorkDir), s.cfg.Files.Include, s.cfg.Files.Exclude)
	if err != nil {
		return nil, fmt.Errorf("filtering files: %w", err)
	}

	slog.Info("template step processing files", "step", s.name, "count", len(files))

	for _, file := range files {
		if err := processFile(ctx.WorkDir, file, ctx.TemplateData); err != nil {
			return nil, fmt.Errorf("processing %s: %w", file, err)
		}
	}

	return &StepResult{}, nil
}

func globFS(fsys fs.FS, patterns []string) ([]string, error) {
	var result []string
	for _, pattern := range patterns {
		matches, err := doublestar.Glob(fsys, pattern)
		if err != nil {
			return nil, fmt.Errorf("glob %q: %w", pattern, err)
		}
		result = append(result, matches...)
	}
	slices.Sort(result)
	result = slices.Compact(result)
	return result, nil
}

func filterFiles(fsys fs.FS, include, exclude []string) ([]string, error) {
	if len(include) == 0 {
		include = []string{api.DefaultFileInclude}
	}

	included, err := globFS(fsys, include)
	if err != nil {
		return nil, fmt.Errorf("include filter: %w", err)
	}

	excluded, err := globFS(fsys, exclude)
	if err != nil {
		return nil, fmt.Errorf("exclude filter: %w", err)
	}

	var result []string
	for _, f := range included {
		info, err := fs.Stat(fsys, f)
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", f, err)
		}
		if info.IsDir() {
			continue
		}
		if slices.Contains(excluded, f) {
			continue
		}
		result = append(result, f)
	}
	return result, nil
}

func processFile(workDir, filename string, data map[string]any) error {
	absPath := filepath.Join(workDir, filename)

	content, err := os.ReadFile(absPath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	tmpl, err := template.New(filepath.Base(filename)).Funcs(sprig.FuncMap()).Parse(string(content))
	if err != nil {
		return fmt.Errorf("parsing template: %w", err)
	}

	out, err := os.Create(absPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}

	execErr := tmpl.Execute(out, data)

	if closeErr := out.Close(); closeErr != nil {
		if execErr != nil {
			return fmt.Errorf("executing template: %w", execErr)
		}
		return fmt.Errorf("closing output file: %w", closeErr)
	}
	if execErr != nil {
		return fmt.Errorf("executing template: %w", execErr)
	}

	slog.Debug("template rendered", "file", filename)
	return nil
}

package steps

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig/v3"
	"github.com/systemstart/many-templates/pkg/api"
)

type generateStep struct {
	name string
	cfg  *api.GenerateConfig
}

// NewGenerateStep creates a generate step.
func NewGenerateStep(name string, cfg *api.GenerateConfig) Step {
	return &generateStep{name: name, cfg: cfg}
}

func (s *generateStep) Name() string { return s.name }

func (s *generateStep) Run(ctx StepContext) (*StepResult, error) {
	tmpl, err := template.New(s.name).Funcs(sprig.FuncMap()).Parse(s.cfg.Template)
	if err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx.TemplateData); err != nil {
		return nil, fmt.Errorf("executing template: %w", err)
	}

	outPath := filepath.Join(ctx.WorkDir, s.cfg.Output)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o750); err != nil {
		return nil, fmt.Errorf("creating parent directories: %w", err)
	}

	if err := os.WriteFile(outPath, buf.Bytes(), 0o600); err != nil {
		return nil, fmt.Errorf("writing output file: %w", err)
	}

	slog.Info("generate step wrote file", "step", s.name, "output", s.cfg.Output)
	return &StepResult{}, nil
}

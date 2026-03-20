package steps

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/systemstart/many-templates/pkg/api"
)

type helmStep struct {
	name string
	cfg  *api.HelmConfig
}

// NewHelmStep creates a helm step.
func NewHelmStep(name string, cfg *api.HelmConfig) Step {
	return &helmStep{name: name, cfg: cfg}
}

func (s *helmStep) Name() string { return s.name }

func (s *helmStep) Run(ctx StepContext) (*StepResult, error) {
	if _, err := exec.LookPath("helm"); err != nil {
		return nil, fmt.Errorf("helm binary not found in PATH: %w", err)
	}

	args := s.buildArgs(ctx.WorkDir)

	slog.Info("running helm template", "step", s.name, "chart", s.cfg.Chart)

	cmd := exec.Command("helm", args...)
	cmd.Dir = ctx.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("helm template failed: %w\nstderr: %s", err, stderr.String())
	}

	if s.cfg.OutputFile != "" {
		if err := writeOutputFile(filepath.Join(ctx.WorkDir, s.cfg.OutputFile), stdout.Bytes()); err != nil {
			return nil, err
		}
	}

	return &StepResult{}, nil
}

func (s *helmStep) buildArgs(workDir string) []string {
	chart := s.cfg.Chart
	if !filepath.IsAbs(chart) {
		chart = filepath.Join(workDir, chart)
	}

	args := []string{"template", s.cfg.ReleaseName, chart}

	ns := s.cfg.Namespace
	if ns == "" {
		ns = "default"
	}
	args = append(args, "--namespace", ns)

	for _, vf := range s.cfg.ValuesFiles {
		if !filepath.IsAbs(vf) {
			vf = filepath.Join(workDir, vf)
		}
		args = append(args, "--values", vf)
	}

	for k, v := range s.cfg.Set {
		args = append(args, "--set", fmt.Sprintf("%s=%s", k, v))
	}

	return args
}

func writeOutputFile(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writing output file: %w", err)
	}
	return nil
}

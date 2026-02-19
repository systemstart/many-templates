package steps

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/systemstart/many-templates/pkg/api"
	"gopkg.in/yaml.v3"
)

const (
	kustomizationFilename = "kustomization.yaml"
	helmChartsDir         = "charts"
)

type kustomizeStep struct {
	name string
	cfg  *api.KustomizeConfig
}

// NewKustomizeStep creates a kustomize step.
func NewKustomizeStep(name string, cfg *api.KustomizeConfig) Step {
	return &kustomizeStep{name: name, cfg: cfg}
}

func (s *kustomizeStep) Name() string { return s.name }

func (s *kustomizeStep) Run(ctx StepContext) (*StepResult, error) {
	dir := s.cfg.Dir
	if dir == "" {
		dir = "."
	}
	dir = filepath.Join(ctx.WorkDir, dir)

	if _, err := exec.LookPath("kustomize"); err != nil {
		return nil, fmt.Errorf("kustomize binary not found in PATH: %w", err)
	}

	args := []string{"build", dir}
	if s.cfg.EnableHelm {
		args = append(args, "--enable-helm")
	}

	slog.Info("running kustomize", "step", s.name, "dir", dir, "enableHelm", s.cfg.EnableHelm)

	cmd := exec.Command("kustomize", args...)
	cmd.Dir = ctx.WorkDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kustomize build failed: %w\nstderr: %s", err, stderr.String())
	}

	result := &StepResult{Output: stdout.Bytes()}

	if s.cfg.EnableHelm {
		result.Cleanup = collectKustomizeCleanup(dir)
	}

	return result, nil
}

// kustomizationFile is a minimal representation for collecting cleanup paths.
type kustomizationFile struct {
	Resources  []string         `yaml:"resources"`
	HelmCharts []helmChartEntry `yaml:"helmCharts"`
}

type helmChartEntry struct {
	ValuesFile string `yaml:"valuesFile"`
}

func collectKustomizeCleanup(dir string) []string {
	data, err := os.ReadFile(filepath.Join(dir, kustomizationFilename))
	if err != nil {
		slog.Warn("could not read kustomization.yaml for cleanup", "dir", dir, "error", err)
		return nil
	}

	var kf kustomizationFile
	if err := yaml.Unmarshal(data, &kf); err != nil {
		slog.Warn("could not parse kustomization.yaml for cleanup", "dir", dir, "error", err)
		return nil
	}

	cleanup := []string{kustomizationFilename, helmChartsDir}

	for _, hc := range kf.HelmCharts {
		if hc.ValuesFile != "" {
			cleanup = append(cleanup, hc.ValuesFile)
		}
	}

	cleanup = append(cleanup, kf.Resources...)

	return cleanup
}

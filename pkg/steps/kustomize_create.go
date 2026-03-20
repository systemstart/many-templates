package steps

import (
	"bytes"
	"fmt"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/systemstart/many-templates/pkg/api"
)

type kustomizeCreateStep struct {
	name string
	cfg  *api.KustomizeCreateConfig
}

// NewKustomizeCreateStep creates a kustomize-create step.
func NewKustomizeCreateStep(name string, cfg *api.KustomizeCreateConfig) Step {
	return &kustomizeCreateStep{name: name, cfg: cfg}
}

func (s *kustomizeCreateStep) Name() string { return s.name }

func (s *kustomizeCreateStep) Run(ctx StepContext) (*StepResult, error) {
	if _, err := exec.LookPath("kustomize"); err != nil {
		return nil, fmt.Errorf("kustomize binary not found in PATH: %w", err)
	}

	dir := s.cfg.Dir
	if dir == "" {
		dir = "."
	}

	args := s.buildArgs()

	slog.Info("running kustomize create", "step", s.name, "dir", dir, "args", args)

	cmd := exec.Command("kustomize", args...)
	cmd.Dir = filepath.Join(ctx.WorkDir, dir)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("kustomize create failed: %w\nstderr: %s", err, stderr.String())
	}

	return &StepResult{}, nil
}

func (s *kustomizeCreateStep) buildArgs() []string {
	args := []string{"create"}

	if s.cfg.Autodetect {
		args = append(args, "--autodetect")
	}
	if s.cfg.Recursive {
		args = append(args, "--recursive")
	}
	if len(s.cfg.Resources) > 0 {
		args = append(args, "--resources", strings.Join(s.cfg.Resources, ","))
	}
	if s.cfg.Namespace != "" {
		args = append(args, "--namespace", s.cfg.Namespace)
	}
	if s.cfg.NamePrefix != "" {
		args = append(args, "--nameprefix", s.cfg.NamePrefix)
	}
	if s.cfg.NameSuffix != "" {
		args = append(args, "--namesuffix", s.cfg.NameSuffix)
	}
	if len(s.cfg.Annotations) > 0 {
		args = append(args, "--annotations", formatMapFlag(s.cfg.Annotations))
	}
	if len(s.cfg.Labels) > 0 {
		args = append(args, "--labels", formatMapFlag(s.cfg.Labels))
	}

	return args
}

// formatMapFlag joins map entries as k1=v1,k2=v2 with sorted keys for deterministic output.
func formatMapFlag(m map[string]string) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	pairs := make([]string, 0, len(m))
	for _, k := range keys {
		pairs = append(pairs, k+"="+m[k])
	}
	return strings.Join(pairs, ",")
}

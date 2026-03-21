package steps

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/systemstart/many-templates/pkg/api"
)

type copyStep struct {
	name string
	cfg  *api.CopyConfig
}

// NewCopyStep creates a copy step.
func NewCopyStep(name string, cfg *api.CopyConfig) Step {
	return &copyStep{name: name, cfg: cfg}
}

func (s *copyStep) Name() string { return s.name }

func (s *copyStep) Run(ctx StepContext) (*StepResult, error) {
	if ctx.SourceDir == "" {
		return nil, fmt.Errorf("source directory is not set")
	}

	files, err := filterFiles(os.DirFS(ctx.SourceDir), s.cfg.Files.Include, s.cfg.Files.Exclude)
	if err != nil {
		return nil, fmt.Errorf("filtering files: %w", err)
	}

	slog.Info("copy step processing files", "step", s.name, "count", len(files))

	dest := s.cfg.Dest
	if dest == "" {
		dest = "."
	}

	for _, file := range files {
		srcPath := filepath.Join(ctx.SourceDir, file)
		dstPath := filepath.Join(ctx.WorkDir, dest, file)

		if err := os.MkdirAll(filepath.Dir(dstPath), 0o750); err != nil {
			return nil, fmt.Errorf("creating directory for %s: %w", file, err)
		}

		data, readErr := os.ReadFile(srcPath)
		if readErr != nil {
			return nil, fmt.Errorf("reading %s: %w", file, readErr)
		}

		if writeErr := os.WriteFile(dstPath, data, 0o600); writeErr != nil {
			return nil, fmt.Errorf("writing %s: %w", file, writeErr)
		}

		slog.Debug("copied file", "file", file)
	}

	return &StepResult{}, nil
}

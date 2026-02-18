package api

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// LoadPipeline reads a .many.yaml file, sets Dir/FilePath, and validates it.
func LoadPipeline(filename string) (*Pipeline, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading pipeline file: %w", err)
	}

	var p Pipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("parsing pipeline file: %w", err)
	}

	absPath, err := filepath.Abs(filename)
	if err != nil {
		return nil, fmt.Errorf("resolving absolute path: %w", err)
	}
	p.FilePath = absPath
	p.Dir = filepath.Dir(absPath)

	if err := p.Validate(); err != nil {
		return nil, fmt.Errorf("validating pipeline %s: %w", filename, err)
	}

	return &p, nil
}

package api

import (
	"fmt"
	"path/filepath"
	"strings"
)

var validStepTypes = map[string]bool{
	StepTypeTemplate:  true,
	StepTypeKustomize: true,
	StepTypeHelm:      true,
	StepTypeSplit:     true,
	StepTypeGenerate:  true,
}

var validSplitStrategies = map[string]bool{
	SplitByKind:     true,
	SplitByResource: true,
	SplitByGroup:    true,
	SplitByKindDir:  true,
	SplitByCustom:   true,
}

// Validate checks the pipeline configuration for errors.
func (p *Pipeline) Validate() error {
	if len(p.Pipeline) == 0 {
		return fmt.Errorf("pipeline has no steps")
	}

	if err := validateSources(p.Source, "pipeline"); err != nil {
		return err
	}

	return validateSteps(p.Pipeline)
}

func validateSteps(steps []StepConfig) error {
	names := make(map[string]int)
	outputProducers := make(map[string]bool)

	for i, step := range steps {
		if step.Name == "" {
			return fmt.Errorf("step %d: name is required", i)
		}
		if prev, exists := names[step.Name]; exists {
			return fmt.Errorf("step %d: duplicate step name %q (first defined at step %d)", i, step.Name, prev)
		}
		names[step.Name] = i

		if err := validateSources(step.Source, fmt.Sprintf("step %q", step.Name)); err != nil {
			return err
		}

		if !validStepTypes[step.Type] {
			return fmt.Errorf("step %q: unknown type %q", step.Name, step.Type)
		}

		if err := validateStepConfig(step, outputProducers); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}

		if step.Type == StepTypeKustomize || step.Type == StepTypeHelm {
			outputProducers[step.Name] = true
		}
	}

	return nil
}

func validateStepConfig(step StepConfig, outputProducers map[string]bool) error {
	switch step.Type {
	case StepTypeTemplate:
		if step.Template == nil {
			return fmt.Errorf("template config is required")
		}
	case StepTypeKustomize:
		if step.Kustomize == nil {
			return fmt.Errorf("kustomize config is required")
		}
	case StepTypeHelm:
		return validateHelmConfig(step)
	case StepTypeSplit:
		return validateSplitConfig(step, outputProducers)
	case StepTypeGenerate:
		return validateGenerateConfig(step)
	}
	return nil
}

func validateHelmConfig(step StepConfig) error {
	if step.Helm == nil {
		return fmt.Errorf("helm config is required")
	}
	if step.Helm.Chart == "" {
		return fmt.Errorf("helm.chart is required")
	}
	if step.Helm.ReleaseName == "" {
		return fmt.Errorf("helm.releaseName is required")
	}
	return nil
}

func validateGenerateConfig(step StepConfig) error {
	if step.Generate == nil {
		return fmt.Errorf("generate config is required")
	}
	if step.Generate.Output == "" {
		return fmt.Errorf("generate.output is required")
	}
	if step.Generate.Template == "" {
		return fmt.Errorf("generate.template is required")
	}
	return nil
}

func validateSplitConfig(step StepConfig, outputProducers map[string]bool) error {
	if step.Split == nil {
		return fmt.Errorf("split config is required")
	}
	if step.Split.Input == "" {
		return fmt.Errorf("split.input is required")
	}
	if !outputProducers[step.Split.Input] {
		return fmt.Errorf("split.input %q does not reference an earlier kustomize or helm step", step.Split.Input)
	}
	if step.Split.By != "" && !validSplitStrategies[step.Split.By] {
		valid := make([]string, 0, len(validSplitStrategies))
		for k := range validSplitStrategies {
			valid = append(valid, k)
		}
		return fmt.Errorf("split.by %q is not valid (valid: %s)", step.Split.By, strings.Join(valid, ", "))
	}
	if step.Split.By == SplitByCustom && step.Split.FileNameTemplate == "" {
		return fmt.Errorf("split.fileNameTemplate is required when split.by is %q", SplitByCustom)
	}
	return nil
}

func validateSources(sources Sources, label string) error {
	for i, entry := range sources {
		if err := validateSourceEntry(entry); err != nil {
			return fmt.Errorf("%s: source[%d]: %w", label, i, err)
		}
	}
	return nil
}

func validateSourceEntry(entry SourceEntry) error {
	if entry.SchemeCount() == 0 {
		return fmt.Errorf("exactly one of oci, https, file, or ocm must be set")
	}
	if entry.SchemeCount() > 1 {
		return fmt.Errorf("exactly one of oci, https, file, or ocm must be set, got %d", entry.SchemeCount())
	}
	if entry.Recursive && entry.OCM == "" {
		return fmt.Errorf("recursive is only valid when ocm is set")
	}
	if entry.Path != "" {
		if err := validateSourcePath(entry.Path); err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
	}
	return nil
}

func validateSourcePath(p string) error {
	if filepath.IsAbs(p) {
		return fmt.Errorf("path must be relative, got %q", p)
	}
	cleaned := filepath.Clean(p)
	if cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return fmt.Errorf("path must not traverse above pipeline directory, got %q", p)
	}
	return nil
}

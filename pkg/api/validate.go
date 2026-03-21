package api

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
)

var validStepTypes = map[string]bool{
	StepTypeTemplate:        true,
	StepTypeKustomizeBuild:  true,
	StepTypeKustomizeCreate: true,
	StepTypeHelm:            true,
	StepTypeSplit:           true,
	StepTypeGenerate:        true,
	StepTypeCopy:            true,
}

var sha256Re = regexp.MustCompile(`^[0-9a-f]{64}$`)

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

	return validateSteps(p.Pipeline)
}

func validateSteps(steps []StepConfig) error {
	names := make(map[string]int)

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

		if err := validateStepConfig(step); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}

		if err := validateExcludePatterns(step.Exclude); err != nil {
			return fmt.Errorf("step %q: %w", step.Name, err)
		}
	}

	return nil
}

func validateStepConfig(step StepConfig) error {
	switch step.Type {
	case StepTypeTemplate:
		if step.Template == nil {
			return fmt.Errorf("template config is required")
		}
	case StepTypeKustomizeBuild:
		return validateKustomizeBuildConfig(step)
	case StepTypeKustomizeCreate:
		return validateKustomizeCreateConfig(step)
	case StepTypeHelm:
		return validateHelmConfig(step)
	case StepTypeSplit:
		return validateSplitConfig(step)
	case StepTypeGenerate:
		return validateGenerateConfig(step)
	case StepTypeCopy:
		return validateCopyConfig(step)
	}
	return nil
}

func validateKustomizeBuildConfig(step StepConfig) error {
	if step.KustomizeBuild == nil {
		return fmt.Errorf("kustomize-build config is required")
	}
	if step.KustomizeBuild.OutputFile == "" {
		return fmt.Errorf("kustomize-build.outputFile is required")
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

func validateSplitConfig(step StepConfig) error {
	if step.Split == nil {
		return fmt.Errorf("split config is required")
	}
	if step.Split.Input == "" {
		return fmt.Errorf("split.input is required")
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

func validateCopyConfig(step StepConfig) error {
	if step.Copy == nil {
		return fmt.Errorf("copy config is required")
	}
	if step.Copy.Dest != "" {
		if err := validateSourcePath(step.Copy.Dest); err != nil {
			return fmt.Errorf("copy.dest: %w", err)
		}
	}
	return nil
}

func validateExcludePatterns(patterns []string) error {
	for i, p := range patterns {
		if !doublestar.ValidatePattern(p) {
			return fmt.Errorf("exclude[%d]: invalid glob pattern %q", i, p)
		}
	}
	return nil
}

func validateKustomizeCreateConfig(step StepConfig) error {
	if step.KustomizeCreate == nil {
		return fmt.Errorf("kustomize-create config is required")
	}
	cfg := step.KustomizeCreate
	if !cfg.Autodetect && len(cfg.Resources) == 0 {
		return fmt.Errorf("kustomize-create: at least one of autodetect or resources must be set")
	}
	if cfg.Recursive && !cfg.Autodetect {
		return fmt.Errorf("kustomize-create: recursive requires autodetect to be enabled")
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
	if err := validateSourceScheme(entry); err != nil {
		return err
	}
	if err := validateHelmFields(entry); err != nil {
		return err
	}
	if err := validateSHA256Field(entry); err != nil {
		return err
	}
	if entry.Path != "" {
		if err := validateSourcePath(entry.Path); err != nil {
			return fmt.Errorf("invalid path: %w", err)
		}
	}
	return nil
}

func validateSourceScheme(entry SourceEntry) error {
	if entry.SchemeCount() == 0 {
		return fmt.Errorf("exactly one of oci, https, file, ocm, or helm must be set")
	}
	if entry.SchemeCount() > 1 {
		return fmt.Errorf("exactly one of oci, https, file, ocm, or helm must be set, got %d", entry.SchemeCount())
	}
	if entry.Recursive && entry.OCM == "" {
		return fmt.Errorf("recursive is only valid when ocm is set")
	}
	return nil
}

func validateHelmFields(entry SourceEntry) error {
	if entry.Helm != "" && entry.Repo == "" {
		return fmt.Errorf("repo is required when helm is set")
	}
	if entry.Repo != "" && entry.Helm == "" {
		return fmt.Errorf("repo is only valid when helm is set")
	}
	if entry.Version != "" && entry.Helm == "" {
		return fmt.Errorf("version is only valid when helm is set")
	}
	return nil
}

func validateSHA256Field(entry SourceEntry) error {
	if entry.SHA256 == "" {
		return nil
	}
	if entry.HTTPS == "" {
		return fmt.Errorf("sha256 is only supported for https sources")
	}
	if !sha256Re.MatchString(entry.SHA256) {
		return fmt.Errorf("sha256 must be exactly 64 lowercase hex characters")
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

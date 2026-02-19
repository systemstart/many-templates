package api

import (
	"strings"
	"testing"
)

func TestValidate_ValidPipeline(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name: "render",
				Type: StepTypeTemplate,
				Template: &TemplateConfig{
					Files: FileFilter{Include: []string{"**/*.yaml"}},
				},
			},
			{
				Name:      "build",
				Type:      StepTypeKustomize,
				Kustomize: &KustomizeConfig{Dir: "."},
			},
			{
				Name: "split",
				Type: StepTypeSplit,
				Split: &SplitConfig{
					Input:     "build",
					By:        SplitByKind,
					OutputDir: "manifests/",
				},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_EmptyPipeline(t *testing.T) {
	p := &Pipeline{}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for empty pipeline")
	}
	if !strings.Contains(err.Error(), "no steps") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_MissingStepName(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Type: StepTypeTemplate, Template: &TemplateConfig{}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_DuplicateStepName(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}},
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for duplicate name")
	}
	if !strings.Contains(err.Error(), "duplicate step name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_UnknownType(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: "unknown"},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
	if !strings.Contains(err.Error(), "unknown type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_MissingTemplateConfig(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing template config")
	}
	if !strings.Contains(err.Error(), "template config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_MissingKustomizeConfig(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeKustomize},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "kustomize config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_HelmMissingChart(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeHelm, Helm: &HelmConfig{ReleaseName: "x"}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing chart")
	}
	if !strings.Contains(err.Error(), "helm.chart is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_HelmMissingReleaseName(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeHelm, Helm: &HelmConfig{Chart: "./chart"}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing releaseName")
	}
	if !strings.Contains(err.Error(), "helm.releaseName is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SplitMissingInput(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeSplit, Split: &SplitConfig{By: SplitByKind}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing split input")
	}
	if !strings.Contains(err.Error(), "split.input is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SplitInputReferencesNonExistent(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "render", Type: StepTypeTemplate, Template: &TemplateConfig{}},
			{Name: "split", Type: StepTypeSplit, Split: &SplitConfig{Input: "render", By: SplitByKind}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for split referencing non-output step")
	}
	if !strings.Contains(err.Error(), "does not reference an earlier kustomize or helm step") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SplitInvalidStrategy(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "build", Type: StepTypeKustomize, Kustomize: &KustomizeConfig{Dir: "."}},
			{Name: "split", Type: StepTypeSplit, Split: &SplitConfig{Input: "build", By: "invalid"}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for invalid strategy")
	}
	if !strings.Contains(err.Error(), "not valid") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SplitCustomWithoutTemplate(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "build", Type: StepTypeKustomize, Kustomize: &KustomizeConfig{Dir: "."}},
			{Name: "split", Type: StepTypeSplit, Split: &SplitConfig{Input: "build", By: SplitByCustom}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for custom without fileNameTemplate")
	}
	if !strings.Contains(err.Error(), "fileNameTemplate is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_ValidGenerateStep(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name: "gen",
				Type: StepTypeGenerate,
				Generate: &GenerateConfig{
					Output:   "out.yaml",
					Template: "key: value",
				},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_MissingGenerateConfig(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeGenerate},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing generate config")
	}
	if !strings.Contains(err.Error(), "generate config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_GenerateMissingOutput(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeGenerate, Generate: &GenerateConfig{Template: "x"}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing generate output")
	}
	if !strings.Contains(err.Error(), "generate.output is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_GenerateMissingTemplate(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeGenerate, Generate: &GenerateConfig{Output: "out.yaml"}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing generate template")
	}
	if !strings.Contains(err.Error(), "generate.template is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_HelmValidFull(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name: "chart",
				Type: StepTypeHelm,
				Helm: &HelmConfig{
					Chart:       "./charts/app",
					ReleaseName: "app",
					Namespace:   "default",
					ValuesFiles: []string{"values.yaml"},
					Set:         map[string]string{"key": "val"},
				},
			},
			{
				Name: "split",
				Type: StepTypeSplit,
				Split: &SplitConfig{
					Input:     "chart",
					By:        SplitByResource,
					OutputDir: "out/",
				},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got: %v", err)
	}
}

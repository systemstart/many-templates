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
				Name:           "build",
				Type:           StepTypeKustomizeBuild,
				KustomizeBuild: &KustomizeBuildConfig{Dir: ".", OutputFile: "build.yaml"},
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

func TestValidate_MissingKustomizeBuildConfig(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeKustomizeBuild},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "kustomize-build config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_KustomizeBuildMissingOutputFile(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeKustomizeBuild, KustomizeBuild: &KustomizeBuildConfig{Dir: "."}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing outputFile")
	}
	if !strings.Contains(err.Error(), "kustomize-build.outputFile is required") {
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

func TestValidate_SplitInputIsFilePath(t *testing.T) {
	// split.input is now a file path, not a step reference — any non-empty string is valid
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "render", Type: StepTypeTemplate, Template: &TemplateConfig{}},
			{Name: "split", Type: StepTypeSplit, Split: &SplitConfig{Input: "render", By: SplitByKind}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_SplitInvalidStrategy(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "build", Type: StepTypeKustomizeBuild, KustomizeBuild: &KustomizeBuildConfig{Dir: ".", OutputFile: "build.yaml"}},
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
			{Name: "build", Type: StepTypeKustomizeBuild, KustomizeBuild: &KustomizeBuildConfig{Dir: ".", OutputFile: "build.yaml"}},
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

func TestValidate_SourceNoScheme(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{Path: "somewhere/"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for source with no scheme")
	}
	if !strings.Contains(err.Error(), "exactly one of oci, https, file, ocm, or helm must be set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SourceMultipleSchemes(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCI: "x", HTTPS: "y"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for source with multiple schemes")
	}
	if !strings.Contains(err.Error(), "exactly one of oci, https, file, ocm, or helm must be set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SourcePathTraversal(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCI: "ghcr.io/myorg/image:v1", Path: "../escape"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "must not traverse") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SourceAbsolutePath(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCI: "ghcr.io/myorg/image:v1", Path: "/absolute/path"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
	if !strings.Contains(err.Error(), "path must be relative") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SourceValid(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCI: "ghcr.io/myorg/image:v1", Path: "subdir/"}}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_SourceOCMValid(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCM: "ghcr.io/myorg/ocm//comp:v1"}}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_SourceRecursiveWithoutOCM(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCI: "ghcr.io/myorg/image:v1", Recursive: true}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for recursive without ocm")
	}
	if !strings.Contains(err.Error(), "recursive is only valid when ocm is set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SourceRecursiveWithOCM(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCM: "ghcr.io/myorg/ocm//comp:v1", Recursive: true}}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_StepSourceInvalid(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name:     "a",
				Type:     StepTypeTemplate,
				Template: &TemplateConfig{},
				Source:   Sources{{Path: "only-path"}},
			},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for step source with no scheme")
	}
	if !strings.Contains(err.Error(), "exactly one of oci, https, file, ocm, or helm must be set") {
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

func TestValidate_KustomizeCreateValid(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name: "create",
				Type: StepTypeKustomizeCreate,
				KustomizeCreate: &KustomizeCreateConfig{
					Autodetect: true,
					Namespace:  "staging",
				},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_KustomizeCreateValidResources(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name: "create",
				Type: StepTypeKustomizeCreate,
				KustomizeCreate: &KustomizeCreateConfig{
					Resources: []string{"deployment.yaml", "../base"},
				},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_MissingKustomizeCreateConfig(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeKustomizeCreate},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing kustomize-create config")
	}
	if !strings.Contains(err.Error(), "kustomize-create config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_KustomizeCreateNeitherAutodetectNorResources(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name:            "create",
				Type:            StepTypeKustomizeCreate,
				KustomizeCreate: &KustomizeCreateConfig{Namespace: "test"},
			},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error when neither autodetect nor resources is set")
	}
	if !strings.Contains(err.Error(), "at least one of autodetect or resources must be set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_KustomizeCreateRecursiveWithoutAutodetect(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name: "create",
				Type: StepTypeKustomizeCreate,
				KustomizeCreate: &KustomizeCreateConfig{
					Resources: []string{"deploy.yaml"},
					Recursive: true,
				},
			},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for recursive without autodetect")
	}
	if !strings.Contains(err.Error(), "recursive requires autodetect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SHA256InvalidLength(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{HTTPS: "https://example.com/file.yaml", SHA256: "abcd"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for short sha256")
	}
	if !strings.Contains(err.Error(), "64 lowercase hex") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SHA256Uppercase(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{HTTPS: "https://example.com/file.yaml", SHA256: "ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789ABCDEF0123456789"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for uppercase sha256")
	}
	if !strings.Contains(err.Error(), "64 lowercase hex") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SHA256NonHex(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{HTTPS: "https://example.com/file.yaml", SHA256: "zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for non-hex sha256")
	}
	if !strings.Contains(err.Error(), "64 lowercase hex") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SHA256OnOCISource(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCI: "ghcr.io/myorg/image:v1", SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for sha256 on OCI source")
	}
	if !strings.Contains(err.Error(), "sha256 is only supported for https sources") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SHA256ValidOnHTTPS(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{HTTPS: "https://example.com/file.yaml", SHA256: "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"}}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_ExcludePatterns_Valid(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name:     "render",
				Type:     StepTypeTemplate,
				Template: &TemplateConfig{},
				Exclude:  []string{"**/*.tmp", "build-output/**"},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline with exclude patterns, got error: %v", err)
	}
}

func TestValidate_ExcludePatterns_Invalid(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name:     "render",
				Type:     StepTypeTemplate,
				Template: &TemplateConfig{},
				Exclude:  []string{"[invalid"},
			},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for invalid exclude pattern")
	}
	if !strings.Contains(err.Error(), "invalid glob pattern") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SourceHelmValid(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{Helm: "mychart", Repo: "https://charts.example.com"}}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_SourceHelmWithVersion(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{Helm: "mychart", Repo: "https://charts.example.com", Version: "1.2.3"}}},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_SourceHelmMissingRepo(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{Helm: "mychart"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for helm without repo")
	}
	if !strings.Contains(err.Error(), "repo is required when helm is set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SourceRepoWithoutHelm(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCI: "ghcr.io/myorg/image:v1", Repo: "https://charts.example.com"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for repo without helm")
	}
	if !strings.Contains(err.Error(), "repo is only valid when helm is set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_SourceVersionWithoutHelm(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeTemplate, Template: &TemplateConfig{}, Source: Sources{{OCI: "ghcr.io/myorg/image:v1", Version: "1.0.0"}}},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for version without helm")
	}
	if !strings.Contains(err.Error(), "version is only valid when helm is set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_ValidCopyStep(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name: "copy-files",
				Type: StepTypeCopy,
				Copy: &CopyConfig{
					Files: FileFilter{Include: []string{"manifests/**/*.yaml"}},
					Dest:  "output/",
				},
			},
		},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("expected valid pipeline, got error: %v", err)
	}
}

func TestValidate_MissingCopyConfig(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{Name: "a", Type: StepTypeCopy},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for missing copy config")
	}
	if !strings.Contains(err.Error(), "copy config is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidate_CopyDestTraversal(t *testing.T) {
	p := &Pipeline{
		Pipeline: []StepConfig{
			{
				Name: "a",
				Type: StepTypeCopy,
				Copy: &CopyConfig{
					Files: FileFilter{Include: []string{"**/*"}},
					Dest:  "../escape",
				},
			},
		},
	}
	err := p.Validate()
	if err == nil {
		t.Fatal("expected error for dest path traversal")
	}
	if !strings.Contains(err.Error(), "must not traverse") {
		t.Fatalf("unexpected error: %v", err)
	}
}

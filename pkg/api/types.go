package api

const (
	DefaultFileInclude = "**/*"

	StepTypeTemplate  = "template"
	StepTypeKustomize = "kustomize"
	StepTypeHelm      = "helm"
	StepTypeSplit     = "split"
	StepTypeGenerate  = "generate"

	SplitByKind     = "kind"
	SplitByResource = "resource"
	SplitByGroup    = "group"
	SplitByKindDir  = "kind-dir"
	SplitByCustom   = "custom"
)

// Pipeline is the .many.yaml configuration format.
type Pipeline struct {
	Context  map[string]any `yaml:"context"`
	Pipeline []StepConfig   `yaml:"pipeline"`

	// Set by the loader, not from YAML.
	Dir      string `yaml:"-"`
	FilePath string `yaml:"-"`
}

// StepConfig defines a single step within a pipeline.
type StepConfig struct {
	Name      string           `yaml:"name"`
	Type      string           `yaml:"type"`
	Template  *TemplateConfig  `yaml:"template,omitempty"`
	Kustomize *KustomizeConfig `yaml:"kustomize,omitempty"`
	Helm      *HelmConfig      `yaml:"helm,omitempty"`
	Split     *SplitConfig     `yaml:"split,omitempty"`
	Generate  *GenerateConfig  `yaml:"generate,omitempty"`
}

// FileFilter defines include/exclude glob patterns.
type FileFilter struct {
	Include []string `yaml:"include"`
	Exclude []string `yaml:"exclude"`
}

// TemplateConfig configures the template step.
type TemplateConfig struct {
	Files FileFilter `yaml:"files"`
}

// KustomizeConfig configures the kustomize step.
type KustomizeConfig struct {
	Dir        string `yaml:"dir"`
	EnableHelm bool   `yaml:"enableHelm"`
}

// HelmConfig configures the helm step.
type HelmConfig struct {
	Chart       string            `yaml:"chart"`
	ReleaseName string            `yaml:"releaseName"`
	Namespace   string            `yaml:"namespace"`
	ValuesFiles []string          `yaml:"valuesFiles"`
	Set         map[string]string `yaml:"set"`
}

// GenerateConfig configures the generate step.
type GenerateConfig struct {
	Output   string `yaml:"output"`
	Template string `yaml:"template"`
}

// SplitConfig configures the split step.
type SplitConfig struct {
	Input             string `yaml:"input"`
	By                string `yaml:"by"`
	OutputDir         string `yaml:"outputDir"`
	FileNameTemplate  string `yaml:"fileNameTemplate"`
	CanonicalKeyOrder *bool  `yaml:"canonicalKeyOrder,omitempty"` // default true
}

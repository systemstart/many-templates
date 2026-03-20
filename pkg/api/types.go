package api

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

const (
	DefaultFileInclude = "**/*"

	StepTypeTemplate        = "template"
	StepTypeKustomizeBuild  = "kustomize-build"
	StepTypeKustomizeCreate = "kustomize-create"
	StepTypeHelm            = "helm"
	StepTypeSplit           = "split"
	StepTypeGenerate        = "generate"

	SplitByKind     = "kind"
	SplitByResource = "resource"
	SplitByGroup    = "group"
	SplitByKindDir  = "kind-dir"
	SplitByCustom   = "custom"
)

// SourceEntry represents a single source to fetch and overlay.
type SourceEntry struct {
	OCI       string `yaml:"oci,omitempty"`
	HTTPS     string `yaml:"https,omitempty"`
	File      string `yaml:"file,omitempty"`
	OCM       string `yaml:"ocm,omitempty"`
	Helm      string `yaml:"helm,omitempty"`      // Helm chart name
	Repo      string `yaml:"repo,omitempty"`      // Helm chart repository URL (helm only)
	Version   string `yaml:"version,omitempty"`   // Helm chart version (helm only)
	SHA256    string `yaml:"sha256,omitempty"`    // optional checksum for HTTPS sources
	Recursive bool   `yaml:"recursive,omitempty"` // only valid with OCM
	Path      string `yaml:"path,omitempty"`      // target subdirectory within pipeline dir
	Temporary bool   `yaml:"temporary,omitempty"` // remove overlaid files after pipeline/step execution
}

// URI returns the resolve-compatible URI string.
func (e SourceEntry) URI() string {
	switch {
	case e.OCI != "":
		return "oci://" + e.OCI
	case e.HTTPS != "":
		return e.HTTPS
	case e.File != "":
		return e.File
	case e.OCM != "":
		return "ocm://" + e.OCM
	case e.Helm != "":
		return "helm://" + e.Helm
	default:
		return ""
	}
}

// SchemeCount returns how many scheme keys are set (for validation).
func (e SourceEntry) SchemeCount() int {
	n := 0
	if e.OCI != "" {
		n++
	}
	if e.HTTPS != "" {
		n++
	}
	if e.File != "" {
		n++
	}
	if e.OCM != "" {
		n++
	}
	if e.Helm != "" {
		n++
	}
	return n
}

// Sources handles YAML polymorphism: single map or list of maps.
type Sources []SourceEntry

// UnmarshalYAML decodes either a single source map or a list of source maps.
func (s *Sources) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind == yaml.SequenceNode {
		var list []SourceEntry
		if err := value.Decode(&list); err != nil {
			return fmt.Errorf("decoding source list: %w", err)
		}
		*s = list
		return nil
	}
	if value.Kind == yaml.MappingNode {
		var single SourceEntry
		if err := value.Decode(&single); err != nil {
			return fmt.Errorf("decoding source entry: %w", err)
		}
		*s = Sources{single}
		return nil
	}
	return fmt.Errorf("source must be a map or list of maps")
}

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
	Name            string                 `yaml:"name"`
	Type            string                 `yaml:"type"`
	Source          Sources                `yaml:"source,omitempty"`
	Exclude         []string               `yaml:"exclude,omitempty"`
	Template        *TemplateConfig        `yaml:"template,omitempty"`
	KustomizeBuild  *KustomizeBuildConfig  `yaml:"kustomize-build,omitempty"`
	KustomizeCreate *KustomizeCreateConfig `yaml:"kustomize-create,omitempty"`
	Helm            *HelmConfig            `yaml:"helm,omitempty"`
	Split           *SplitConfig           `yaml:"split,omitempty"`
	Generate        *GenerateConfig        `yaml:"generate,omitempty"`
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

// KustomizeBuildConfig configures the kustomize-build step.
type KustomizeBuildConfig struct {
	Dir        string `yaml:"dir"`
	EnableHelm bool   `yaml:"enableHelm"`
	OutputFile string `yaml:"outputFile,omitempty"`
}

// HelmConfig configures the helm step.
type HelmConfig struct {
	Chart       string            `yaml:"chart"`
	ReleaseName string            `yaml:"releaseName"`
	Namespace   string            `yaml:"namespace"`
	ValuesFiles []string          `yaml:"valuesFiles"`
	Set         map[string]string `yaml:"set"`
	OutputFile  string            `yaml:"outputFile,omitempty"`
}

// GenerateConfig configures the generate step.
type GenerateConfig struct {
	Output   string `yaml:"output"`
	Template string `yaml:"template"`
}

// KustomizeCreateConfig configures the kustomize-create step.
type KustomizeCreateConfig struct {
	Dir         string            `yaml:"dir"`
	Autodetect  bool              `yaml:"autodetect"`
	Recursive   bool              `yaml:"recursive"`
	Resources   []string          `yaml:"resources"`
	Namespace   string            `yaml:"namespace"`
	NamePrefix  string            `yaml:"nameprefix"`
	NameSuffix  string            `yaml:"namesuffix"`
	Annotations map[string]string `yaml:"annotations"`
	Labels      map[string]string `yaml:"labels"`
}

// SplitConfig configures the split step.
type SplitConfig struct {
	Input             string `yaml:"input"`
	By                string `yaml:"by"`
	OutputDir         string `yaml:"outputDir"`
	FileNameTemplate  string `yaml:"fileNameTemplate"`
	CanonicalKeyOrder *bool  `yaml:"canonicalKeyOrder,omitempty"` // default true
}

// InstancesConfig is the top-level instances file format.
type InstancesConfig struct {
	Instances []Instance `yaml:"instances"`
}

// Instance defines a single instance in instances mode.
type Instance struct {
	Name    string         `yaml:"name"`
	Input   string         `yaml:"input"`
	Output  string         `yaml:"output"`
	Include []string       `yaml:"include"`
	Context map[string]any `yaml:"context"`
}

package steps

import (
	"bytes"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/systemstart/many-templates/pkg/api"
	"gopkg.in/yaml.v3"
)

// Manifest represents a single parsed Kubernetes manifest.
type Manifest struct {
	APIVersion string
	Kind       string
	Name       string
	Namespace  string
	Group      string // extracted from apiVersion
	Raw        []byte
	Data       map[string]any
}

type splitStep struct {
	name string
	cfg  *api.SplitConfig
}

// NewSplitStep creates a split step.
func NewSplitStep(name string, cfg *api.SplitConfig) Step {
	return &splitStep{name: name, cfg: cfg}
}

func (s *splitStep) Name() string { return s.name }

func (s *splitStep) Run(ctx StepContext) (*StepResult, error) {
	if len(ctx.InputData) == 0 {
		return nil, fmt.Errorf("no input data provided")
	}

	canonicalOrder := s.cfg.CanonicalKeyOrder == nil || *s.cfg.CanonicalKeyOrder
	manifests, err := parseMultiDocYAML(ctx.InputData, canonicalOrder)
	if err != nil {
		return nil, fmt.Errorf("parsing multi-doc YAML: %w", err)
	}

	slog.Info("split step", "step", s.name, "manifests", len(manifests), "strategy", s.cfg.By)

	strategy, err := getStrategy(s.cfg.By)
	if err != nil {
		return nil, err
	}

	assignments, err := strategy.Assign(manifests, s.cfg)
	if err != nil {
		return nil, fmt.Errorf("assigning manifests: %w", err)
	}

	outputDir := s.cfg.OutputDir
	if outputDir == "" {
		outputDir = "."
	}
	outputDir = filepath.Join(ctx.WorkDir, outputDir)

	if err := writeAssignments(outputDir, assignments); err != nil {
		return nil, err
	}

	if err := writeKustomization(ctx.WorkDir, s.cfg.OutputDir, assignments); err != nil {
		return nil, err
	}

	return &StepResult{}, nil
}

func writeKustomization(workDir, outputDirRel string, assignments map[string][]Manifest) error {
	prefix := outputDirRel
	if prefix == "" {
		prefix = "."
	}

	paths := make([]string, 0, len(assignments))
	for relPath := range assignments {
		paths = append(paths, filepath.Join(prefix, relPath))
	}
	sort.Strings(paths)

	var buf bytes.Buffer
	buf.WriteString("apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n")
	for _, p := range paths {
		buf.WriteString("  - ")
		buf.WriteString(p)
		buf.WriteByte('\n')
	}

	kustomizationPath := filepath.Join(workDir, "kustomization.yaml")
	if err := os.WriteFile(kustomizationPath, buf.Bytes(), 0o600); err != nil {
		return fmt.Errorf("writing kustomization.yaml: %w", err)
	}
	slog.Debug("split wrote kustomization.yaml", "path", kustomizationPath, "resources", len(paths))
	return nil
}

func writeAssignments(outputDir string, assignments map[string][]Manifest) error {
	for relPath, docs := range assignments {
		absPath := filepath.Join(outputDir, relPath)
		if err := os.MkdirAll(filepath.Dir(absPath), 0o750); err != nil {
			return fmt.Errorf("creating directory for %s: %w", relPath, err)
		}

		data := marshalDocs(docs)

		if err := os.WriteFile(absPath, data, 0o600); err != nil {
			return fmt.Errorf("writing %s: %w", relPath, err)
		}
		slog.Debug("split wrote file", "path", relPath, "manifests", len(docs))
	}
	return nil
}

func marshalDocs(docs []Manifest) []byte {
	var buf bytes.Buffer
	for i, m := range docs {
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(m.Raw)
		if !bytes.HasSuffix(m.Raw, []byte("\n")) {
			buf.WriteByte('\n')
		}
	}
	return buf.Bytes()
}

func parseMultiDocYAML(data []byte, canonicalOrder bool) ([]Manifest, error) {
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	var manifests []Manifest

	for {
		var node yaml.Node
		err := decoder.Decode(&node)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("decoding YAML document: %w", err)
		}
		if isEmptyDoc(&node) {
			continue
		}

		if canonicalOrder {
			reorderMappingKeys(&node)
		}

		m, err := buildManifest(&node)
		if err != nil {
			return nil, err
		}
		manifests = append(manifests, m)
	}

	return manifests, nil
}

func isEmptyDoc(node *yaml.Node) bool {
	if node.Kind == 0 {
		return true
	}
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return true
		}
		c := node.Content[0]
		return c.Kind == yaml.ScalarNode && c.Tag == "!!null"
	}
	return false
}

// priorityKeys defines the canonical top-of-manifest key order for Kubernetes resources.
var priorityKeys = map[string]int{
	"apiVersion": 0,
	"kind":       1,
	"metadata":   2,
}

// reorderMappingKeys reorders top-level mapping keys so that apiVersion, kind,
// and metadata appear first (in that order), followed by the remaining keys in
// their original order.
func reorderMappingKeys(node *yaml.Node) {
	// DocumentNode wraps the actual mapping
	if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
		node = node.Content[0]
	}
	if node.Kind != yaml.MappingNode {
		return
	}

	type pair struct {
		key *yaml.Node
		val *yaml.Node
	}

	pairs := make([]pair, 0, len(node.Content)/2)
	for i := 0; i+1 < len(node.Content); i += 2 {
		pairs = append(pairs, pair{node.Content[i], node.Content[i+1]})
	}

	sort.SliceStable(pairs, func(i, j int) bool {
		pi, oki := priorityKeys[pairs[i].key.Value]
		pj, okj := priorityKeys[pairs[j].key.Value]
		if oki && okj {
			return pi < pj
		}
		if oki {
			return true
		}
		return false
	})

	for i, p := range pairs {
		node.Content[i*2] = p.key
		node.Content[i*2+1] = p.val
	}
}

func buildManifest(node *yaml.Node) (Manifest, error) {
	var doc map[string]any
	if err := node.Decode(&doc); err != nil {
		return Manifest{}, fmt.Errorf("decoding document fields: %w", err)
	}

	m := Manifest{Data: doc}

	if v, ok := doc["apiVersion"].(string); ok {
		m.APIVersion = v
		m.Group = extractGroup(v)
	}
	if v, ok := doc["kind"].(string); ok {
		m.Kind = v
	}
	if meta, ok := doc["metadata"].(map[string]any); ok {
		if v, ok := meta["name"].(string); ok {
			m.Name = v
		}
		if v, ok := meta["namespace"].(string); ok {
			m.Namespace = v
		}
	}

	raw, err := yaml.Marshal(node)
	if err != nil {
		return Manifest{}, fmt.Errorf("re-marshaling document: %w", err)
	}
	m.Raw = raw

	return m, nil
}

// extractGroup extracts the API group from an apiVersion string.
// "apps/v1" -> "apps", "v1" -> "core", "networking.k8s.io/v1" -> "networking.k8s.io"
func extractGroup(apiVersion string) string {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 1 {
		return "core"
	}
	return parts[0]
}

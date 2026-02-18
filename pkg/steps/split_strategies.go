package steps

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/systemstart/many-templates/pkg/api"
)

// Strategy assigns manifests to file paths.
type Strategy interface {
	Assign(manifests []Manifest, cfg *api.SplitConfig) (map[string][]Manifest, error)
}

func getStrategy(name string) (Strategy, error) {
	switch name {
	case api.SplitByKind, "":
		return &kindStrategy{}, nil
	case api.SplitByResource:
		return &resourceStrategy{}, nil
	case api.SplitByGroup:
		return &groupStrategy{}, nil
	case api.SplitByKindDir:
		return &kindDirStrategy{}, nil
	case api.SplitByCustom:
		return &customStrategy{}, nil
	default:
		return nil, fmt.Errorf("unknown split strategy: %s", name)
	}
}

// kindStrategy groups all manifests of the same Kind into one file.
type kindStrategy struct{}

func (s *kindStrategy) Assign(manifests []Manifest, _ *api.SplitConfig) (map[string][]Manifest, error) {
	result := make(map[string][]Manifest)
	for _, m := range manifests {
		filename := strings.ToLower(m.Kind) + ".yaml"
		result[filename] = append(result[filename], m)
	}
	return result, nil
}

// resourceStrategy puts each resource in its own file: <kind>-<name>.yaml
type resourceStrategy struct{}

func (s *resourceStrategy) Assign(manifests []Manifest, _ *api.SplitConfig) (map[string][]Manifest, error) {
	result := make(map[string][]Manifest)
	for _, m := range manifests {
		filename := strings.ToLower(m.Kind) + "-" + strings.ToLower(m.Name) + ".yaml"
		filename = disambiguate(result, filename, m)
		result[filename] = append(result[filename], m)
	}
	return result, nil
}

// groupStrategy creates dirs per API group, files per resource.
type groupStrategy struct{}

func (s *groupStrategy) Assign(manifests []Manifest, _ *api.SplitConfig) (map[string][]Manifest, error) {
	result := make(map[string][]Manifest)
	for _, m := range manifests {
		dir := strings.ToLower(m.Group)
		filename := strings.ToLower(m.Kind) + "-" + strings.ToLower(m.Name) + ".yaml"
		path := disambiguate(result, dir+"/"+filename, m)
		result[path] = append(result[path], m)
	}
	return result, nil
}

// kindDirStrategy creates dirs per Kind, files per resource name.
type kindDirStrategy struct{}

var irregularPlurals = map[string]string{
	"ingress": "ingresses",
}

func pluralize(kind string) string {
	lower := strings.ToLower(kind)
	if p, ok := irregularPlurals[lower]; ok {
		return p
	}
	if strings.HasSuffix(lower, "s") {
		return lower + "es"
	}
	if strings.HasSuffix(lower, "y") {
		return lower[:len(lower)-1] + "ies"
	}
	return lower + "s"
}

func (s *kindDirStrategy) Assign(manifests []Manifest, _ *api.SplitConfig) (map[string][]Manifest, error) {
	result := make(map[string][]Manifest)
	for _, m := range manifests {
		dir := pluralize(m.Kind)
		filename := strings.ToLower(m.Name) + ".yaml"
		path := disambiguate(result, dir+"/"+filename, m)
		result[path] = append(result[path], m)
	}
	return result, nil
}

// disambiguate appends the namespace to a path when it would collide with an existing entry.
func disambiguate(result map[string][]Manifest, path string, m Manifest) string {
	if _, exists := result[path]; exists && m.Namespace != "" {
		ext := filepath.Ext(path)
		return path[:len(path)-len(ext)] + "-" + strings.ToLower(m.Namespace) + ext
	}
	return path
}

// customStrategy uses a Go template to determine file paths.
type customStrategy struct{}

func (s *customStrategy) Assign(manifests []Manifest, cfg *api.SplitConfig) (map[string][]Manifest, error) {
	tmpl, err := template.New("filename").Parse(cfg.FileNameTemplate)
	if err != nil {
		return nil, fmt.Errorf("parsing fileNameTemplate: %w", err)
	}

	result := make(map[string][]Manifest)
	for _, m := range manifests {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, m.Data); err != nil {
			return nil, fmt.Errorf("executing fileNameTemplate for %s/%s: %w", m.Kind, m.Name, err)
		}
		path := buf.String()
		result[path] = append(result[path], m)
	}
	return result, nil
}

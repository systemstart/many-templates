package api

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestSources_UnmarshalYAML_SingleMap(t *testing.T) {
	input := `oci: ghcr.io/myorg/shared-base:v1.0.0`
	var s Sources
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(s))
	}
	if s[0].OCI != "ghcr.io/myorg/shared-base:v1.0.0" {
		t.Errorf("expected oci value, got %q", s[0].OCI)
	}
}

func TestSources_UnmarshalYAML_List(t *testing.T) {
	input := `
- oci: ghcr.io/myorg/base-crds:v3.0.0
  path: crds/
- https: https://example.com/config-bundle.tar.gz
  path: config/
`
	var s Sources
	if err := yaml.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(s) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(s))
	}
	if s[0].OCI != "ghcr.io/myorg/base-crds:v3.0.0" {
		t.Errorf("expected oci value, got %q", s[0].OCI)
	}
	if s[0].Path != "crds/" {
		t.Errorf("expected path 'crds/', got %q", s[0].Path)
	}
	if s[1].HTTPS != "https://example.com/config-bundle.tar.gz" {
		t.Errorf("expected https value, got %q", s[1].HTTPS)
	}
	if s[1].Path != "config/" {
		t.Errorf("expected path 'config/', got %q", s[1].Path)
	}
}

func TestSources_UnmarshalYAML_InvalidScalar(t *testing.T) {
	input := `"just a string"`
	var s Sources
	if err := yaml.Unmarshal([]byte(input), &s); err == nil {
		t.Fatal("expected error for scalar value")
	}
}

func TestSourceEntry_URI(t *testing.T) {
	tests := []struct {
		name     string
		entry    SourceEntry
		expected string
	}{
		{
			name:     "oci",
			entry:    SourceEntry{OCI: "ghcr.io/myorg/image:v1"},
			expected: "oci://ghcr.io/myorg/image:v1",
		},
		{
			name:     "https",
			entry:    SourceEntry{HTTPS: "https://example.com/bundle.tar.gz"},
			expected: "https://example.com/bundle.tar.gz",
		},
		{
			name:     "file",
			entry:    SourceEntry{File: "/tmp/local-dir"},
			expected: "/tmp/local-dir",
		},
		{
			name:     "file with scheme",
			entry:    SourceEntry{File: "file:///tmp/local-dir"},
			expected: "file:///tmp/local-dir",
		},
		{
			name:     "empty",
			entry:    SourceEntry{},
			expected: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.URI()
			if got != tt.expected {
				t.Errorf("URI() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestSourceEntry_SchemeCount(t *testing.T) {
	tests := []struct {
		name     string
		entry    SourceEntry
		expected int
	}{
		{"none", SourceEntry{}, 0},
		{"oci only", SourceEntry{OCI: "x"}, 1},
		{"https only", SourceEntry{HTTPS: "x"}, 1},
		{"file only", SourceEntry{File: "x"}, 1},
		{"oci+https", SourceEntry{OCI: "x", HTTPS: "y"}, 2},
		{"all three", SourceEntry{OCI: "x", HTTPS: "y", File: "z"}, 3},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.entry.SchemeCount()
			if got != tt.expected {
				t.Errorf("SchemeCount() = %d, want %d", got, tt.expected)
			}
		})
	}
}

func TestLoadPipeline_WithSource(t *testing.T) {
	dir := t.TempDir()
	pipelineYAML := `
pipeline:
  - name: render
    type: template
    source:
      - file: /tmp/extra
        path: extra/
    template:
      files:
        include: ["**/*.yaml"]
`
	pipelineFile := filepath.Join(dir, ".many.yaml")
	if err := os.WriteFile(pipelineFile, []byte(pipelineYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPipeline(pipelineFile)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check step-level source
	if len(p.Pipeline) != 1 {
		t.Fatalf("expected 1 step, got %d", len(p.Pipeline))
	}
	step := p.Pipeline[0]
	if len(step.Source) != 1 {
		t.Fatalf("expected 1 step source, got %d", len(step.Source))
	}
	if step.Source[0].File != "/tmp/extra" {
		t.Errorf("expected file source, got %+v", step.Source[0])
	}
	if step.Source[0].Path != "extra/" {
		t.Errorf("expected path 'extra/', got %q", step.Source[0].Path)
	}
}

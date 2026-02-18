package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPipeline_Valid(t *testing.T) {
	content := `
context:
  domain: example.com
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["**/*.yaml"]
  - name: build
    type: kustomize
    kustomize:
      dir: "."
  - name: split
    type: split
    split:
      input: build
      by: kind
      outputDir: manifests/
`
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	p, err := LoadPipeline(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(p.Pipeline) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(p.Pipeline))
	}
	if p.Dir != dir {
		t.Fatalf("expected Dir=%q, got %q", dir, p.Dir)
	}
	if p.Context["domain"] != "example.com" {
		t.Fatalf("expected domain=example.com, got %v", p.Context["domain"])
	}
}

func TestLoadPipeline_FileNotFound(t *testing.T) {
	_, err := LoadPipeline("/nonexistent/.many.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "reading pipeline file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPipeline_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	if err := os.WriteFile(f, []byte("{{invalid"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPipeline(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parsing pipeline file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadPipeline_ValidationFails(t *testing.T) {
	content := `
pipeline:
  - name: ""
    type: template
`
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	if err := os.WriteFile(f, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadPipeline(f)
	if err == nil {
		t.Fatal("expected validation error")
	}
	if !strings.Contains(err.Error(), "validating pipeline") {
		t.Fatalf("unexpected error: %v", err)
	}
}

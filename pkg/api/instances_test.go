package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadInstances_Valid(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(f, []byte(`
instances:
  - name: alpha
    output: alpha/
    context:
      key: val
  - name: beta
    input: src/
    output: beta/
    include:
      - app1
      - app2
`), 0o600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadInstances(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Instances) != 2 {
		t.Fatalf("expected 2 instances, got %d", len(cfg.Instances))
	}
	if cfg.Instances[0].Name != "alpha" {
		t.Errorf("expected name 'alpha', got %q", cfg.Instances[0].Name)
	}
	if cfg.Instances[1].Input != "src/" {
		t.Errorf("expected input 'src/', got %q", cfg.Instances[1].Input)
	}
	if len(cfg.Instances[1].Include) != 2 {
		t.Errorf("expected 2 includes, got %d", len(cfg.Instances[1].Include))
	}
}

func TestLoadInstances_EmptyList(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(f, []byte("instances: []\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInstances(f)
	if err == nil {
		t.Fatal("expected error for empty instances list")
	}
	if !strings.Contains(err.Error(), "instances list is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInstances_MissingName(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(f, []byte(`
instances:
  - output: out/
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInstances(f)
	if err == nil {
		t.Fatal("expected error for missing name")
	}
	if !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInstances_MissingOutput(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(f, []byte(`
instances:
  - name: alpha
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInstances(f)
	if err == nil {
		t.Fatal("expected error for missing output")
	}
	if !strings.Contains(err.Error(), "output is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInstances_DuplicateNames(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(f, []byte(`
instances:
  - name: alpha
    output: a/
  - name: alpha
    output: b/
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInstances(f)
	if err == nil {
		t.Fatal("expected error for duplicate names")
	}
	if !strings.Contains(err.Error(), "duplicate name") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInstances_DuplicateOutputs(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(f, []byte(`
instances:
  - name: alpha
    output: same/
  - name: beta
    output: same/
`), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInstances(f)
	if err == nil {
		t.Fatal("expected error for duplicate outputs")
	}
	if !strings.Contains(err.Error(), "duplicate output path") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInstances_FileNotFound(t *testing.T) {
	_, err := LoadInstances("/nonexistent/path/instances.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "reading instances file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadInstances_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "instances.yaml")
	if err := os.WriteFile(f, []byte("{{invalid yaml"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadInstances(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
	if !strings.Contains(err.Error(), "parsing instances file") {
		t.Fatalf("unexpected error: %v", err)
	}
}

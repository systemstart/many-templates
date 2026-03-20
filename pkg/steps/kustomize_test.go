package steps

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func TestCollectKustomizeCleanup(t *testing.T) {
	tests := []struct {
		name     string
		yaml     string
		expected []string
	}{
		{
			name: "with helm charts",
			yaml: `
resources:
  - secret.yaml
  - configmap.yaml
helmCharts:
  - name: dex
    repo: https://charts.dexidp.io
    releaseName: dex
    namespace: dex
    version: 0.22.1
    valuesFile: values.yaml
  - name: other
    repo: https://example.com
    releaseName: other
    namespace: other
    version: 1.0.0
`,
			expected: []string{"charts", "configmap.yaml", "kustomization.yaml", "secret.yaml", "values.yaml"},
		},
		{
			name: "without helm charts",
			yaml: `
resources:
  - deployment.yaml
`,
			expected: []string{"charts", "deployment.yaml", "kustomization.yaml"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "kustomization.yaml"), []byte(tt.yaml), 0o600); err != nil {
				t.Fatal(err)
			}

			cleanup := collectKustomizeCleanup(dir)
			sort.Strings(cleanup)

			if len(cleanup) != len(tt.expected) {
				t.Fatalf("expected %d paths, got %d: %v", len(tt.expected), len(cleanup), cleanup)
			}
			for i, p := range cleanup {
				if p != tt.expected[i] {
					t.Errorf("path[%d] = %q, want %q", i, p, tt.expected[i])
				}
			}
		})
	}
}

func TestCollectKustomizeCleanup_NoFile(t *testing.T) {
	cleanup := collectKustomizeCleanup(t.TempDir())
	if cleanup != nil {
		t.Errorf("expected nil cleanup for missing kustomization.yaml, got %v", cleanup)
	}
}

func skipWithoutKustomize(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not in PATH")
	}
}

func TestKustomizeStep_Run(t *testing.T) {
	skipWithoutKustomize(t)

	dir := t.TempDir()
	writeTestFile(t, dir, "kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - configmap.yaml
`)
	writeTestFile(t, dir, "configmap.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value
`)

	step := NewKustomizeBuildStep("build", &api.KustomizeBuildConfig{Dir: ".", OutputFile: "out.yaml"})
	result, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	content, err := os.ReadFile(filepath.Join(dir, "out.yaml"))
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
	if !strings.Contains(string(content), "test-cm") {
		t.Error("output file should contain configmap name")
	}
}

func TestKustomizeStep_RunSubdir(t *testing.T) {
	skipWithoutKustomize(t)

	dir := t.TempDir()
	sub := filepath.Join(dir, "overlay")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, sub, "kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - service.yaml
`)
	writeTestFile(t, sub, "service.yaml", `apiVersion: v1
kind: Service
metadata:
  name: my-svc
spec:
  ports:
    - port: 80
`)

	_, err := NewKustomizeBuildStep("build", &api.KustomizeBuildConfig{Dir: "overlay", OutputFile: "out.yaml"}).Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, readErr := os.ReadFile(filepath.Join(dir, "out.yaml"))
	if readErr != nil {
		t.Fatalf("expected output file to exist: %v", readErr)
	}
	if !strings.Contains(string(content), "my-svc") {
		t.Error("output should contain the service")
	}
}

func TestKustomizeStep_RunInvalidDir(t *testing.T) {
	skipWithoutKustomize(t)

	_, err := NewKustomizeBuildStep("build", &api.KustomizeBuildConfig{Dir: "nonexistent"}).Run(StepContext{WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for nonexistent kustomize dir")
	}
	if !strings.Contains(err.Error(), "kustomize build failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestKustomizeStep_OutputFile(t *testing.T) {
	skipWithoutKustomize(t)

	dir := t.TempDir()
	writeTestFile(t, dir, "kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - configmap.yaml
`)
	writeTestFile(t, dir, "configmap.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value
`)

	step := NewKustomizeBuildStep("build", &api.KustomizeBuildConfig{OutputFile: "out/result.yaml"})
	_, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be written
	content, err := os.ReadFile(filepath.Join(dir, "out", "result.yaml"))
	if err != nil {
		t.Fatalf("expected output file to exist: %v", err)
	}
	if !strings.Contains(string(content), "test-cm") {
		t.Error("output file should contain configmap name")
	}
}

func TestKustomizeStep_DefaultDir(t *testing.T) {
	skipWithoutKustomize(t)

	dir := t.TempDir()
	writeTestFile(t, dir, "kustomization.yaml", `apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - secret.yaml
`)
	writeTestFile(t, dir, "secret.yaml", `apiVersion: v1
kind: Secret
metadata:
  name: my-secret
type: Opaque
data:
  key: dmFsdWU=
`)

	_, err := NewKustomizeBuildStep("build", &api.KustomizeBuildConfig{OutputFile: "out.yaml"}).Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	content, readErr := os.ReadFile(filepath.Join(dir, "out.yaml"))
	if readErr != nil {
		t.Fatalf("expected output file to exist: %v", readErr)
	}
	if !strings.Contains(string(content), "my-secret") {
		t.Error("output should contain the secret")
	}
}

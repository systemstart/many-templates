package steps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

// createTestChart builds a minimal Helm chart in dir/test-chart with the given template content.
func createTestChart(t *testing.T, dir, templateName, templateContent string) {
	t.Helper()
	chart := filepath.Join(dir, "test-chart")
	tmplDir := filepath.Join(chart, "templates")
	if err := os.MkdirAll(tmplDir, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTestFile(t, chart, "Chart.yaml", "apiVersion: v2\nname: test-chart\nversion: 0.1.0\n")
	writeTestFile(t, chart, "values.yaml", "replicaCount: 1\n")
	writeTestFile(t, tmplDir, templateName, templateContent)
}

func skipWithoutHelm(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not in PATH")
	}
}

func TestHelmStep_Run(t *testing.T) {
	skipWithoutHelm(t)

	dir := t.TempDir()
	createTestChart(t, dir, "configmap.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Release.Name }}-cm
data:
  replicas: "{{ .Values.replicaCount }}"
`)

	step := NewHelmStep("render", &api.HelmConfig{
		Chart:       "test-chart",
		ReleaseName: "myrelease",
		Namespace:   "test-ns",
	})

	result, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || len(result.Output) == 0 {
		t.Fatal("expected non-empty output")
	}

	output := string(result.Output)
	if !strings.Contains(output, "myrelease-cm") {
		t.Errorf("output should contain release name in configmap, got:\n%s", output)
	}
}

func TestHelmStep_RunWithValuesFile(t *testing.T) {
	skipWithoutHelm(t)

	dir := t.TempDir()
	createTestChart(t, dir, "deploy.yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
`)

	writeTestFile(t, dir, "custom-values.yaml", "replicaCount: 5\n")

	step := NewHelmStep("render", &api.HelmConfig{
		Chart:       "test-chart",
		ReleaseName: "myapp",
		ValuesFiles: []string{"custom-values.yaml"},
	})

	result, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(result.Output), "replicas: 5") {
		t.Errorf("output should contain overridden replica count, got:\n%s", string(result.Output))
	}
}

func TestHelmStep_RunWithSet(t *testing.T) {
	skipWithoutHelm(t)

	dir := t.TempDir()
	createTestChart(t, dir, "deploy.yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .Release.Name }}
spec:
  replicas: {{ .Values.replicaCount }}
`)

	step := NewHelmStep("render", &api.HelmConfig{
		Chart:       "test-chart",
		ReleaseName: "myapp",
		Set:         map[string]string{"replicaCount": "3"},
	})

	result, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(result.Output), "replicas: 3") {
		t.Errorf("output should contain set replica count, got:\n%s", string(result.Output))
	}
}

func TestHelmStep_RunInvalidChart(t *testing.T) {
	skipWithoutHelm(t)

	dir := t.TempDir()

	step := NewHelmStep("render", &api.HelmConfig{
		Chart:       "nonexistent-chart",
		ReleaseName: "test",
	})

	_, err := step.Run(StepContext{WorkDir: dir})
	if err == nil {
		t.Fatal("expected error for nonexistent chart")
	}
	if !strings.Contains(err.Error(), "helm template failed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHelmStep_DefaultNamespace(t *testing.T) {
	skipWithoutHelm(t)

	dir := t.TempDir()
	createTestChart(t, dir, "sa.yaml", `apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .Release.Name }}
  namespace: {{ .Release.Namespace }}
`)

	step := NewHelmStep("render", &api.HelmConfig{
		Chart:       "test-chart",
		ReleaseName: "myapp",
	})

	result, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(string(result.Output), "namespace: default") {
		t.Errorf("expected default namespace, got:\n%s", string(result.Output))
	}
}

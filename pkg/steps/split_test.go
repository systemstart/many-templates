package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func TestExtractGroup(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"v1", "core"},
		{"apps/v1", "apps"},
		{"networking.k8s.io/v1", "networking.k8s.io"},
		{"rbac.authorization.k8s.io/v1", "rbac.authorization.k8s.io"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractGroup(tt.input)
			if got != tt.want {
				t.Errorf("extractGroup(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestParseMultiDocYAML(t *testing.T) {
	input := []byte(`apiVersion: v1
kind: Service
metadata:
  name: my-svc
  namespace: default
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deploy
`)

	manifests, err := parseMultiDocYAML(input, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifests) != 2 {
		t.Fatalf("expected 2 manifests, got %d", len(manifests))
	}

	if manifests[0].Kind != "Service" {
		t.Errorf("manifest 0 kind = %q, want Service", manifests[0].Kind)
	}
	if manifests[0].Name != "my-svc" {
		t.Errorf("manifest 0 name = %q, want my-svc", manifests[0].Name)
	}
	if manifests[0].Namespace != "default" {
		t.Errorf("manifest 0 namespace = %q, want default", manifests[0].Namespace)
	}
	if manifests[0].Group != "core" {
		t.Errorf("manifest 0 group = %q, want core", manifests[0].Group)
	}

	if manifests[1].Kind != "Deployment" {
		t.Errorf("manifest 1 kind = %q, want Deployment", manifests[1].Kind)
	}
	if manifests[1].Group != "apps" {
		t.Errorf("manifest 1 group = %q, want apps", manifests[1].Group)
	}
}

func TestParseMultiDocYAML_SkipsEmptyDocs(t *testing.T) {
	input := []byte(`---
---
apiVersion: v1
kind: Service
metadata:
  name: svc
---
`)

	manifests, err := parseMultiDocYAML(input, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}
}

func TestParseMultiDocYAML_InvalidYAML(t *testing.T) {
	input := []byte(`{invalid: [`)
	_, err := parseMultiDocYAML(input, false)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestCanonicalKeyOrder(t *testing.T) {
	// Input has keys in alphabetical order (as kustomize/helm might produce)
	input := []byte(`data:
  key: value
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
spec:
  replicas: 1
`)

	manifests, err := parseMultiDocYAML(input, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(manifests) != 1 {
		t.Fatalf("expected 1 manifest, got %d", len(manifests))
	}

	raw := string(manifests[0].Raw)
	apiIdx := strings.Index(raw, "apiVersion:")
	kindIdx := strings.Index(raw, "kind:")
	metaIdx := strings.Index(raw, "metadata:")
	dataIdx := strings.Index(raw, "data:")
	specIdx := strings.Index(raw, "spec:")

	if apiIdx > kindIdx {
		t.Error("apiVersion should come before kind")
	}
	if kindIdx > metaIdx {
		t.Error("kind should come before metadata")
	}
	if metaIdx > dataIdx {
		t.Error("metadata should come before data")
	}
	if dataIdx > specIdx {
		t.Error("data should come before spec (original order preserved)")
	}
}

func TestCanonicalKeyOrder_Disabled(t *testing.T) {
	// With canonicalOrder=false, original order should be preserved
	input := []byte(`data:
  key: value
apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`)

	manifests, err := parseMultiDocYAML(input, false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	raw := string(manifests[0].Raw)
	dataIdx := strings.Index(raw, "data:")
	apiIdx := strings.Index(raw, "apiVersion:")

	if dataIdx > apiIdx {
		t.Error("with canonical ordering disabled, original order (data before apiVersion) should be preserved")
	}
}

func TestSplitStep_Run(t *testing.T) {
	dir := t.TempDir()

	input := []byte(`apiVersion: v1
kind: Service
metadata:
  name: my-svc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: my-deploy
`)

	step := NewSplitStep("split", &api.SplitConfig{
		Input:     "build",
		By:        api.SplitByKind,
		OutputDir: "out",
	})

	ctx := StepContext{
		WorkDir:   dir,
		InputData: input,
	}

	result, err := step.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// Check that files were written
	svcData, err := os.ReadFile(filepath.Join(dir, "out", "service.yaml"))
	if err != nil {
		t.Fatalf("reading service.yaml: %v", err)
	}
	if len(svcData) == 0 {
		t.Fatal("service.yaml is empty")
	}

	deployData, err := os.ReadFile(filepath.Join(dir, "out", "deployment.yaml"))
	if err != nil {
		t.Fatalf("reading deployment.yaml: %v", err)
	}
	if len(deployData) == 0 {
		t.Fatal("deployment.yaml is empty")
	}

	// Check kustomization.yaml was generated at WorkDir level with prefixed paths
	kustomData, err := os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("reading kustomization.yaml: %v", err)
	}
	kustomStr := string(kustomData)
	if !strings.Contains(kustomStr, "out/deployment.yaml") {
		t.Error("kustomization.yaml should reference out/deployment.yaml")
	}
	if !strings.Contains(kustomStr, "out/service.yaml") {
		t.Error("kustomization.yaml should reference out/service.yaml")
	}
}

func TestSplitStep_NoInput(t *testing.T) {
	step := NewSplitStep("split", &api.SplitConfig{
		Input: "build",
		By:    api.SplitByKind,
	})

	_, err := step.Run(StepContext{WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for no input data")
	}
}

func TestSplitStep_DefaultOutputDir(t *testing.T) {
	dir := t.TempDir()

	input := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
`)

	step := NewSplitStep("split", &api.SplitConfig{
		Input: "build",
		By:    api.SplitByKind,
	})

	ctx := StepContext{
		WorkDir:   dir,
		InputData: input,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.ReadFile(filepath.Join(dir, "configmap.yaml")); err != nil {
		t.Fatalf("expected configmap.yaml in workdir: %v", err)
	}

	// kustomization.yaml should be in workdir with unprefixed paths
	kustomData, err := os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("reading kustomization.yaml: %v", err)
	}
	if !strings.Contains(string(kustomData), "configmap.yaml") {
		t.Error("kustomization.yaml should reference configmap.yaml")
	}
}

func TestIsEmptyDoc(t *testing.T) {
	tests := []struct {
		name  string
		input string
		count int
	}{
		{"null doc", "---\nnull\n---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n", 1},
		{"empty separator", "---\n---\napiVersion: v1\nkind: Service\nmetadata:\n  name: svc\n", 1},
		{"tilde null", "---\n~\n---\napiVersion: v1\nkind: Pod\nmetadata:\n  name: pod\n", 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifests, err := parseMultiDocYAML([]byte(tt.input), false)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(manifests) != tt.count {
				t.Errorf("expected %d manifest(s), got %d", tt.count, len(manifests))
			}
		})
	}
}

func TestMarshalDocs_SingleDoc(t *testing.T) {
	m := Manifest{Raw: []byte("apiVersion: v1\nkind: Service\n")}
	out := marshalDocs([]Manifest{m})
	if strings.Contains(string(out), "---") {
		t.Error("single doc should not contain --- separator")
	}
	if !strings.Contains(string(out), "apiVersion: v1") {
		t.Error("output should contain the manifest")
	}
}

func TestMarshalDocs_MultiDoc(t *testing.T) {
	m1 := Manifest{Raw: []byte("apiVersion: v1\nkind: Service\n")}
	m2 := Manifest{Raw: []byte("apiVersion: v1\nkind: Pod\n")}
	out := marshalDocs([]Manifest{m1, m2})
	if !strings.Contains(string(out), "---\n") {
		t.Error("multi doc should contain --- separator")
	}
}

func TestMarshalDocs_NoTrailingNewline(t *testing.T) {
	m := Manifest{Raw: []byte("apiVersion: v1")}
	out := marshalDocs([]Manifest{m})
	if !strings.HasSuffix(string(out), "\n") {
		t.Error("output should end with newline even if raw doesn't")
	}
}

func TestSplitStep_MissingKindOrName(t *testing.T) {
	dir := t.TempDir()

	// Manifest with no kind field
	input := []byte(`apiVersion: v1
metadata:
  name: test
`)

	step := NewSplitStep("split", &api.SplitConfig{
		Input: "build",
		By:    api.SplitByKind,
	})

	ctx := StepContext{
		WorkDir:   dir,
		InputData: input,
	}

	// Should still work (kind will be empty string, filename will be .yaml)
	_, err := step.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSplitStep_ResourceStrategy(t *testing.T) {
	dir := t.TempDir()

	input := []byte(`apiVersion: v1
kind: Service
metadata:
  name: svc-a
---
apiVersion: v1
kind: Service
metadata:
  name: svc-b
`)

	step := NewSplitStep("split", &api.SplitConfig{
		Input:     "build",
		By:        api.SplitByResource,
		OutputDir: "out",
	})

	ctx := StepContext{
		WorkDir:   dir,
		InputData: input,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should produce separate files per resource
	if _, err := os.Stat(filepath.Join(dir, "out", "service-svc-a.yaml")); err != nil {
		t.Error("expected service-svc-a.yaml")
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "service-svc-b.yaml")); err != nil {
		t.Error("expected service-svc-b.yaml")
	}
}

func TestSplitStep_GroupStrategy(t *testing.T) {
	dir := t.TempDir()

	input := []byte(`apiVersion: v1
kind: Service
metadata:
  name: svc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy
`)

	step := NewSplitStep("split", &api.SplitConfig{
		Input:     "build",
		By:        api.SplitByGroup,
		OutputDir: "out",
	})

	ctx := StepContext{
		WorkDir:   dir,
		InputData: input,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "out", "core", "service-svc.yaml")); err != nil {
		t.Error("expected core/service-svc.yaml")
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "apps", "deployment-deploy.yaml")); err != nil {
		t.Error("expected apps/deployment-deploy.yaml")
	}
}

func TestSplitStep_KindDirStrategy(t *testing.T) {
	dir := t.TempDir()

	input := []byte(`apiVersion: v1
kind: Service
metadata:
  name: svc
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: deploy
`)

	step := NewSplitStep("split", &api.SplitConfig{
		Input:     "build",
		By:        api.SplitByKindDir,
		OutputDir: "out",
	})

	ctx := StepContext{
		WorkDir:   dir,
		InputData: input,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "out", "services", "svc.yaml")); err != nil {
		t.Error("expected services/svc.yaml")
	}
	if _, err := os.Stat(filepath.Join(dir, "out", "deployments", "deploy.yaml")); err != nil {
		t.Error("expected deployments/deploy.yaml")
	}
}

func TestSplitStep_CustomStrategy(t *testing.T) {
	dir := t.TempDir()

	input := []byte(`apiVersion: v1
kind: Service
metadata:
  name: svc
  namespace: prod
`)

	step := NewSplitStep("split", &api.SplitConfig{
		Input:            "build",
		By:               api.SplitByCustom,
		OutputDir:        "out",
		FileNameTemplate: `{{ .metadata.namespace }}/{{ .kind }}-{{ .metadata.name }}.yaml`,
	})

	ctx := StepContext{
		WorkDir:   dir,
		InputData: input,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "out", "prod", "Service-svc.yaml")); err != nil {
		t.Error("expected prod/Service-svc.yaml")
	}
}

func TestSplitStep_CanonicalKeyOrderDisabled(t *testing.T) {
	dir := t.TempDir()

	input := []byte(`kind: ConfigMap
apiVersion: v1
metadata:
  name: test
data:
  key: value
`)

	boolFalse := false
	step := NewSplitStep("split", &api.SplitConfig{
		Input:             "build",
		By:                api.SplitByKind,
		CanonicalKeyOrder: &boolFalse,
	})

	ctx := StepContext{
		WorkDir:   dir,
		InputData: input,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "configmap.yaml"))
	if err != nil {
		t.Fatal(err)
	}

	// With canonical order disabled, kind should stay before apiVersion
	kindIdx := strings.Index(string(content), "kind:")
	apiIdx := strings.Index(string(content), "apiVersion:")
	if kindIdx > apiIdx {
		t.Error("with canonical order disabled, original key order should be preserved")
	}
}

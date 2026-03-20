package steps

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func skipWithoutKustomizeCreate(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("kustomize"); err != nil {
		t.Skip("kustomize not in PATH")
	}
}

func TestKustomizeCreateStep_Autodetect(t *testing.T) {
	skipWithoutKustomizeCreate(t)

	dir := t.TempDir()
	writeTestFile(t, dir, "deployment.yaml", `apiVersion: apps/v1
kind: Deployment
metadata:
  name: test-deploy
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test
  template:
    metadata:
      labels:
        app: test
    spec:
      containers:
        - name: app
          image: nginx
`)

	step := NewKustomizeCreateStep("create", &api.KustomizeCreateConfig{
		Autodetect: true,
	})

	_, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("kustomization.yaml not created: %v", err)
	}
	if !strings.Contains(string(content), "deployment.yaml") {
		t.Errorf("expected kustomization.yaml to list deployment.yaml, got:\n%s", content)
	}
}

func TestKustomizeCreateStep_Resources(t *testing.T) {
	skipWithoutKustomizeCreate(t)

	dir := t.TempDir()
	writeTestFile(t, dir, "service.yaml", `apiVersion: v1
kind: Service
metadata:
  name: my-svc
spec:
  ports:
    - port: 80
`)

	step := NewKustomizeCreateStep("create", &api.KustomizeCreateConfig{
		Resources: []string{"service.yaml"},
	})

	_, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("kustomization.yaml not created: %v", err)
	}
	if !strings.Contains(string(content), "service.yaml") {
		t.Errorf("expected kustomization.yaml to list service.yaml, got:\n%s", content)
	}
}

func TestKustomizeCreateStep_AllFlags(t *testing.T) {
	skipWithoutKustomizeCreate(t)

	dir := t.TempDir()
	writeTestFile(t, dir, "cm.yaml", `apiVersion: v1
kind: ConfigMap
metadata:
  name: test-cm
data:
  key: value
`)

	step := NewKustomizeCreateStep("create", &api.KustomizeCreateConfig{
		Resources:   []string{"cm.yaml"},
		Namespace:   "staging",
		NamePrefix:  "acme-",
		NameSuffix:  "-v2",
		Annotations: map[string]string{"app.kubernetes.io/managed-by": "many"},
		Labels:      map[string]string{"env": "production"},
	})

	_, err := step.Run(StepContext{WorkDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "kustomization.yaml"))
	if err != nil {
		t.Fatalf("kustomization.yaml not created: %v", err)
	}

	s := string(content)
	checks := []string{"staging", "acme-", "-v2", "app.kubernetes.io/managed-by", "many", "env", "production"}
	for _, want := range checks {
		if !strings.Contains(s, want) {
			t.Errorf("expected kustomization.yaml to contain %q, got:\n%s", want, s)
		}
	}
}

func TestKustomizeCreateStep_MissingBinary(t *testing.T) {
	if _, err := exec.LookPath("kustomize"); err == nil {
		t.Skip("kustomize is available, skipping missing binary test")
	}

	step := NewKustomizeCreateStep("create", &api.KustomizeCreateConfig{
		Autodetect: true,
	})

	_, err := step.Run(StepContext{WorkDir: t.TempDir()})
	if err == nil {
		t.Fatal("expected error for missing kustomize binary")
	}
	if !strings.Contains(err.Error(), "kustomize binary not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestFormatMapFlag(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]string
		want string
	}{
		{
			name: "single entry",
			m:    map[string]string{"key": "val"},
			want: "key=val",
		},
		{
			name: "sorted keys",
			m:    map[string]string{"z": "3", "a": "1", "m": "2"},
			want: "a=1,m=2,z=3",
		},
		{
			name: "annotation-style keys",
			m:    map[string]string{"app.kubernetes.io/name": "app", "app.kubernetes.io/env": "prod"},
			want: "app.kubernetes.io/env=prod,app.kubernetes.io/name=app",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatMapFlag(tt.m)
			if got != tt.want {
				t.Errorf("formatMapFlag() = %q, want %q", got, tt.want)
			}
		})
	}
}

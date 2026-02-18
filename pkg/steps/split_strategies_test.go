package steps

import (
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

var testManifests = []Manifest{
	{APIVersion: "v1", Kind: "Service", Name: "my-svc", Namespace: "default", Group: "core"},
	{APIVersion: "apps/v1", Kind: "Deployment", Name: "my-deploy", Namespace: "default", Group: "apps"},
	{APIVersion: "apps/v1", Kind: "Deployment", Name: "other-deploy", Namespace: "staging", Group: "apps"},
	{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Name: "my-ingress", Group: "networking.k8s.io"},
}

func TestKindStrategy(t *testing.T) {
	s := &kindStrategy{}
	result, err := s.Assign(testManifests, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(result["service.yaml"]) != 1 {
		t.Errorf("expected 1 service, got %d", len(result["service.yaml"]))
	}
	if len(result["deployment.yaml"]) != 2 {
		t.Errorf("expected 2 deployments, got %d", len(result["deployment.yaml"]))
	}
	if len(result["ingress.yaml"]) != 1 {
		t.Errorf("expected 1 ingress, got %d", len(result["ingress.yaml"]))
	}
}

func TestResourceStrategy(t *testing.T) {
	s := &resourceStrategy{}
	result, err := s.Assign(testManifests, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result["service-my-svc.yaml"]; !ok {
		t.Error("expected service-my-svc.yaml")
	}
	if _, ok := result["deployment-my-deploy.yaml"]; !ok {
		t.Error("expected deployment-my-deploy.yaml")
	}
	if _, ok := result["ingress-my-ingress.yaml"]; !ok {
		t.Error("expected ingress-my-ingress.yaml")
	}
}

func TestGroupStrategy(t *testing.T) {
	s := &groupStrategy{}
	result, err := s.Assign(testManifests, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result["core/service-my-svc.yaml"]; !ok {
		t.Error("expected core/service-my-svc.yaml")
	}
	if _, ok := result["apps/deployment-my-deploy.yaml"]; !ok {
		t.Error("expected apps/deployment-my-deploy.yaml")
	}
	if _, ok := result["networking.k8s.io/ingress-my-ingress.yaml"]; !ok {
		t.Error("expected networking.k8s.io/ingress-my-ingress.yaml")
	}
}

func TestKindDirStrategy(t *testing.T) {
	s := &kindDirStrategy{}
	result, err := s.Assign(testManifests, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result["services/my-svc.yaml"]; !ok {
		t.Error("expected services/my-svc.yaml")
	}
	if _, ok := result["deployments/my-deploy.yaml"]; !ok {
		t.Error("expected deployments/my-deploy.yaml")
	}
	if _, ok := result["ingresses/my-ingress.yaml"]; !ok {
		t.Error("expected ingresses/my-ingress.yaml")
	}
}

func TestCustomStrategy(t *testing.T) {
	s := &customStrategy{}
	cfg := &api.SplitConfig{
		FileNameTemplate: `{{ .kind }}/{{ .metadata.name }}.yaml`,
	}

	manifests := []Manifest{
		{Kind: "Service", Name: "svc", Data: map[string]any{
			"kind":     "Service",
			"metadata": map[string]any{"name": "svc"},
		}},
	}

	result, err := s.Assign(manifests, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := result["Service/svc.yaml"]; !ok {
		t.Errorf("expected Service/svc.yaml, got %v", result)
	}
}

func TestCustomStrategy_InvalidTemplate(t *testing.T) {
	s := &customStrategy{}
	cfg := &api.SplitConfig{
		FileNameTemplate: `{{ .invalid`,
	}

	_, err := s.Assign([]Manifest{{Data: map[string]any{}}}, cfg)
	if err == nil {
		t.Fatal("expected error for invalid template")
	}
}

func TestGetStrategy(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{api.SplitByKind, false},
		{api.SplitByResource, false},
		{api.SplitByGroup, false},
		{api.SplitByKindDir, false},
		{api.SplitByCustom, false},
		{"", false}, // defaults to kind
		{"unknown", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := getStrategy(tt.name)
			if (err != nil) != tt.wantErr {
				t.Errorf("getStrategy(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
			}
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Deployment", "deployments"},
		{"Service", "services"},
		{"Ingress", "ingresses"},
		{"Policy", "policies"},
		{"ConfigMap", "configmaps"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := pluralize(tt.input)
			if got != tt.want {
				t.Errorf("pluralize(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

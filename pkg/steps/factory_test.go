package steps

import (
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func TestNewStep(t *testing.T) {
	tests := []struct {
		name    string
		cfg     api.StepConfig
		wantErr bool
	}{
		{
			name: "template step",
			cfg: api.StepConfig{
				Name:     "render",
				Type:     api.StepTypeTemplate,
				Template: &api.TemplateConfig{},
			},
		},
		{
			name: "kustomize step",
			cfg: api.StepConfig{
				Name:      "build",
				Type:      api.StepTypeKustomize,
				Kustomize: &api.KustomizeConfig{},
			},
		},
		{
			name: "helm step",
			cfg: api.StepConfig{
				Name: "chart",
				Type: api.StepTypeHelm,
				Helm: &api.HelmConfig{Chart: "mychart", ReleaseName: "rel"},
			},
		},
		{
			name: "split step",
			cfg: api.StepConfig{
				Name:  "split",
				Type:  api.StepTypeSplit,
				Split: &api.SplitConfig{Input: "build", By: api.SplitByKind},
			},
		},
		{
			name: "generate step",
			cfg: api.StepConfig{
				Name:     "gen",
				Type:     api.StepTypeGenerate,
				Generate: &api.GenerateConfig{Output: "out.yaml", Template: "v: ok"},
			},
		},
		{
			name: "unknown type",
			cfg: api.StepConfig{
				Name: "bad",
				Type: "unknown",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step, err := NewStep(tt.cfg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NewStep() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if step == nil {
					t.Fatal("expected non-nil step")
				}
				if step.Name() != tt.cfg.Name {
					t.Errorf("Name() = %q, want %q", step.Name(), tt.cfg.Name)
				}
			}
		})
	}
}

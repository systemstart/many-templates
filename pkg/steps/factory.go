package steps

import (
	"fmt"

	"github.com/systemstart/many-templates/pkg/api"
)

// NewStep creates a Step implementation from a StepConfig.
func NewStep(cfg api.StepConfig) (Step, error) {
	switch cfg.Type {
	case api.StepTypeTemplate:
		return NewTemplateStep(cfg.Name, cfg.Template), nil
	case api.StepTypeKustomizeBuild:
		return NewKustomizeBuildStep(cfg.Name, cfg.KustomizeBuild), nil
	case api.StepTypeKustomizeCreate:
		return NewKustomizeCreateStep(cfg.Name, cfg.KustomizeCreate), nil
	case api.StepTypeHelm:
		return NewHelmStep(cfg.Name, cfg.Helm), nil
	case api.StepTypeSplit:
		return NewSplitStep(cfg.Name, cfg.Split), nil
	case api.StepTypeGenerate:
		return NewGenerateStep(cfg.Name, cfg.Generate), nil
	case api.StepTypeCopy:
		return NewCopyStep(cfg.Name, cfg.Copy), nil
	default:
		return nil, fmt.Errorf("unknown step type: %s", cfg.Type)
	}
}

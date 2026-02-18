package steps

// StepContext provides the runtime context for a step.
type StepContext struct {
	WorkDir      string
	TemplateData map[string]any
	InputData    []byte // output from a prior step (used by split)
}

// StepResult holds the output of a step.
type StepResult struct {
	Output  []byte   // multi-doc YAML stream (kustomize/helm)
	Cleanup []string // paths relative to WorkDir to remove after pipeline
}

// Step is the interface all pipeline steps implement.
type Step interface {
	Name() string
	Run(ctx StepContext) (*StepResult, error)
}

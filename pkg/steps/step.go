package steps

// StepContext provides the runtime context for a step.
type StepContext struct {
	WorkDir      string
	SourceDir    string
	TemplateData map[string]any
}

// StepResult holds the output of a step.
type StepResult struct {
	Cleanup []string // paths relative to WorkDir to remove after pipeline
}

// Step is the interface all pipeline steps implement.
type Step interface {
	Name() string
	Run(ctx StepContext) (*StepResult, error)
}

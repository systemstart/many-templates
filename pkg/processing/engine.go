package processing

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/systemstart/many-templates/pkg/api"
	"github.com/systemstart/many-templates/pkg/steps"
)

// RunPipeline executes a single pipeline's steps sequentially.
func RunPipeline(pipeline *api.Pipeline, globalContext map[string]any) error {
	ctx := MergeContext(globalContext, pipeline.Context)
	if err := InterpolateContext(ctx); err != nil {
		return fmt.Errorf("interpolating context: %w", err)
	}

	outputs := make(map[string][]byte)

	for _, stepCfg := range pipeline.Pipeline {
		slog.Info("running step", "pipeline", pipeline.FilePath, "step", stepCfg.Name, "type", stepCfg.Type)
		if err := runStep(stepCfg, pipeline, ctx, outputs); err != nil {
			return err
		}
	}

	return nil
}

func runStep(stepCfg api.StepConfig, pipeline *api.Pipeline, ctx map[string]any, outputs map[string][]byte) error {
	step, err := steps.NewStep(stepCfg)
	if err != nil {
		return fmt.Errorf("creating step %q: %w", stepCfg.Name, err)
	}

	sctx := steps.StepContext{
		WorkDir:      pipeline.Dir,
		TemplateData: ctx,
	}

	if stepCfg.Type == api.StepTypeSplit && stepCfg.Split != nil {
		input, ok := outputs[stepCfg.Split.Input]
		if !ok {
			return fmt.Errorf("step %q: input %q not found in step outputs", stepCfg.Name, stepCfg.Split.Input)
		}
		sctx.InputData = input
	}

	result, err := step.Run(sctx)
	if err != nil {
		return fmt.Errorf("step %q failed: %w", stepCfg.Name, err)
	}

	if result != nil && len(result.Output) > 0 {
		outputs[stepCfg.Name] = result.Output
	}
	if result != nil {
		removeBuildArtifacts(pipeline.Dir, result.Cleanup)
	}
	return nil
}

func removeBuildArtifacts(dir string, relativePaths []string) {
	for _, rel := range relativePaths {
		p := filepath.Join(dir, rel)
		slog.Info("cleaning up build artifact", "path", p)
		if err := os.RemoveAll(p); err != nil {
			slog.Warn("failed to remove build artifact", "path", p, "error", err)
		}
	}
}

// RunAll discovers pipelines, executes each, and returns a summary of results.
// It copies the source tree to outputDir first, then processes in-place.
func RunAll(inputDir, outputDir string, globalContext map[string]any, maxDepth int, contextFile string) error {
	if err := copyTree(inputDir, outputDir); err != nil {
		return fmt.Errorf("copying source tree: %w", err)
	}

	removeContextFile(contextFile, inputDir, outputDir)

	pipelines, err := DiscoverPipelines(outputDir, maxDepth)
	if err != nil {
		return fmt.Errorf("discovering pipelines: %w", err)
	}

	if len(pipelines) == 0 {
		slog.Warn("no .many.yaml files found", "dir", inputDir)
		return nil
	}

	slog.Info("discovered pipelines", "count", len(pipelines))

	var failed []string
	for _, p := range pipelines {
		slog.Info("executing pipeline", "path", p.FilePath)
		if pErr := RunPipeline(p, globalContext); pErr != nil {
			slog.Error("pipeline failed", "path", p.FilePath, "error", pErr)
			failed = append(failed, p.FilePath)
		} else {
			slog.Info("pipeline succeeded", "path", p.FilePath)
		}
	}

	if err := removeConfigFiles(outputDir); err != nil {
		slog.Error("failed to clean up .many.yaml files", "error", err)
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d pipeline(s) failed: %v", len(failed), failed)
	}

	return nil
}

// RunSingle copies the source tree, runs one specific pipeline, and cleans up.
// The pipelineFile must be a path within inputDir.
func RunSingle(pipelineFile, inputDir, outputDir string, globalContext map[string]any, contextFile string) error {
	if err := copyTree(inputDir, outputDir); err != nil {
		return fmt.Errorf("copying source tree: %w", err)
	}

	removeContextFile(contextFile, inputDir, outputDir)

	rel, ok := relativeToInput(pipelineFile, inputDir)
	if !ok {
		return fmt.Errorf("pipeline file %q is not within input directory %q", pipelineFile, inputDir)
	}

	outputPipelineFile := filepath.Join(outputDir, rel)
	pipeline, err := api.LoadPipeline(outputPipelineFile)
	if err != nil {
		return fmt.Errorf("loading pipeline: %w", err)
	}

	slog.Info("executing single pipeline", "path", pipeline.FilePath)
	if pErr := RunPipeline(pipeline, globalContext); pErr != nil {
		return fmt.Errorf("pipeline failed: %w", pErr)
	}

	if err := removeConfigFiles(outputDir); err != nil {
		slog.Error("failed to clean up .many.yaml files", "error", err)
	}

	return nil
}

// RunInstances processes each instance: filtered copy, pipeline discovery, execution.
func RunInstances(cfg *api.InstancesConfig, inputDir, outputDir string, globalContext map[string]any, maxDepth int, contextFile string) error {
	var failed []string

	for _, inst := range cfg.Instances {
		slog.Info("processing instance", "name", inst.Name)

		if err := runInstance(inst, inputDir, outputDir, globalContext, maxDepth, contextFile); err != nil {
			slog.Error("instance failed", "name", inst.Name, "error", err)
			failed = append(failed, inst.Name)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d instance(s) failed: %v", len(failed), failed)
	}

	return nil
}

func runInstance(inst api.Instance, inputDir, outputDir string, globalContext map[string]any, maxDepth int, contextFile string) error {
	instInputDir := inputDir
	if inst.Input != "" {
		instInputDir = filepath.Join(inputDir, inst.Input)
	}
	instOutputDir := filepath.Join(outputDir, inst.Output)
	instContext := MergeContext(globalContext, inst.Context)

	if err := copyTreeFiltered(instInputDir, instOutputDir, inst.Include); err != nil {
		return fmt.Errorf("copying tree: %w", err)
	}

	removeContextFile(contextFile, instInputDir, instOutputDir)

	pipelines, err := DiscoverPipelines(instOutputDir, maxDepth)
	if err != nil {
		return fmt.Errorf("discovering pipelines: %w", err)
	}

	if len(pipelines) == 0 {
		slog.Warn("no .many.yaml files found for instance", "name", inst.Name)
	}

	slog.Info("discovered pipelines for instance", "name", inst.Name, "count", len(pipelines))

	var pipelineFailed bool
	for _, p := range pipelines {
		slog.Info("executing pipeline", "instance", inst.Name, "path", p.FilePath)
		if pErr := RunPipeline(p, instContext); pErr != nil {
			slog.Error("pipeline failed", "instance", inst.Name, "path", p.FilePath, "error", pErr)
			pipelineFailed = true
		}
	}

	if err := removeConfigFiles(instOutputDir); err != nil {
		slog.Error("failed to clean up .many.yaml files", "instance", inst.Name, "error", err)
	}

	if pipelineFailed {
		return fmt.Errorf("one or more pipelines failed")
	}
	return nil
}

func copyTreeFiltered(src, dst string, include []string) error {
	if len(include) == 0 {
		return copyTree(src, dst)
	}

	includeSet := make(map[string]bool, len(include))
	for _, name := range include {
		includeSet[name] = true
	}

	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}

		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}

		// Filter immediate subdirectories of root.
		if d.IsDir() && filepath.Dir(rel) == "." && rel != "." && !includeSet[d.Name()] {
			return filepath.SkipDir
		}

		return copyEntry(dst, rel, path, d)
	})
	if err != nil {
		return fmt.Errorf("copying filtered tree: %w", err)
	}
	return nil
}

func copyTree(src, dst string) error {
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}
		return copyEntry(dst, rel, path, d)
	})
	if err != nil {
		return fmt.Errorf("copying tree: %w", err)
	}
	return nil
}

func copyEntry(dst, rel, srcPath string, d fs.DirEntry) error {
	target := filepath.Join(dst, rel)

	if d.IsDir() {
		if err := os.MkdirAll(target, 0o750); err != nil {
			return fmt.Errorf("creating directory %s: %w", target, err)
		}
		return nil
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", srcPath, err)
	}

	info, err := d.Info()
	if err != nil {
		return fmt.Errorf("stat %s: %w", srcPath, err)
	}

	if err := os.WriteFile(target, data, info.Mode()); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}
	return nil
}

func removeContextFile(contextFile, inputDir, outputDir string) {
	if contextFile == "" {
		return
	}
	rel, ok := relativeToInput(contextFile, inputDir)
	if !ok {
		return
	}
	target := filepath.Join(outputDir, rel)
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to remove context file from output", "path", target, "error", err)
	} else if err == nil {
		slog.Debug("removed context file from output", "path", target)
	}
}

func relativeToInput(file, inputDir string) (string, bool) {
	absFile, err := filepath.Abs(file)
	if err != nil {
		return "", false
	}
	absInput, err := filepath.Abs(inputDir)
	if err != nil {
		return "", false
	}
	rel, err := filepath.Rel(absInput, absFile)
	if err != nil {
		return "", false
	}
	if filepath.IsAbs(rel) || len(rel) >= 2 && rel[:2] == ".." {
		return "", false
	}
	return rel, true
}

func removeConfigFiles(root string) error {
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}
		if !d.IsDir() && d.Name() == configFilename {
			slog.Debug("removing config file from output", "path", path)
			if rmErr := os.Remove(path); rmErr != nil {
				return fmt.Errorf("removing %s: %w", path, rmErr)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("cleaning config files: %w", err)
	}
	return nil
}

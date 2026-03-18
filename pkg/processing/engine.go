package processing

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/systemstart/many-templates/pkg/api"
	"github.com/systemstart/many-templates/pkg/resolve"
	"github.com/systemstart/many-templates/pkg/steps"
)

// RunPipeline executes a single pipeline's steps sequentially.
func RunPipeline(pipeline *api.Pipeline, globalContext map[string]any) error {
	ctx := MergeContext(globalContext, pipeline.Context)
	if err := InterpolateContext(ctx); err != nil {
		return fmt.Errorf("interpolating context: %w", err)
	}

	if len(pipeline.Source) > 0 {
		cleanup, err := resolveSources(pipeline.Source, pipeline.Dir)
		if err != nil {
			return fmt.Errorf("resolving pipeline sources: %w", err)
		}
		if cleanup != nil {
			defer cleanup()
		}
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
	cleanup, err := resolveStepSources(stepCfg, pipeline.Dir)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	step, err := steps.NewStep(stepCfg)
	if err != nil {
		return fmt.Errorf("creating step %q: %w", stepCfg.Name, err)
	}

	sctx, err := buildStepContext(stepCfg, pipeline.Dir, ctx, outputs)
	if err != nil {
		return err
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

func resolveStepSources(stepCfg api.StepConfig, dir string) (func(), error) {
	if len(stepCfg.Source) == 0 {
		return nil, nil
	}
	cleanup, err := resolveSources(stepCfg.Source, dir)
	if err != nil {
		return nil, fmt.Errorf("step %q: resolving sources: %w", stepCfg.Name, err)
	}
	return cleanup, nil
}

func buildStepContext(stepCfg api.StepConfig, workDir string, ctx map[string]any, outputs map[string][]byte) (steps.StepContext, error) {
	sctx := steps.StepContext{
		WorkDir:      workDir,
		TemplateData: ctx,
	}
	if stepCfg.Type == api.StepTypeSplit && stepCfg.Split != nil {
		input, ok := outputs[stepCfg.Split.Input]
		if !ok {
			return sctx, fmt.Errorf("step %q: input %q not found in step outputs", stepCfg.Name, stepCfg.Split.Input)
		}
		sctx.InputData = input
	}
	return sctx, nil
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
	instInputDir, cleanup, err := resolveInstanceInput(inst.Input, inputDir)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
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

// resolveInstanceInput determines the effective input directory for an instance.
func resolveInstanceInput(input, inputDir string) (string, func(), error) {
	if input == "" {
		return inputDir, nil, nil
	}
	if resolve.IsRemote(input) {
		resolved, cleanup, err := resolve.Resolve(input)
		if err != nil {
			return "", nil, fmt.Errorf("resolving remote input %q: %w", input, err)
		}
		return resolved, cleanup, nil
	}
	return filepath.Join(inputDir, input), nil, nil
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

// resolveSources fetches all source entries and overlays them into targetDir.
func resolveSources(sources api.Sources, targetDir string) (cleanup func(), err error) {
	var cleanups []func()
	runCleanups := func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}

	for i, entry := range sources {
		entryCleanup, err := resolveAndOverlay(entry, targetDir)
		if err != nil {
			runCleanups()
			return nil, fmt.Errorf("source[%d]: %w", i, err)
		}
		if entryCleanup != nil {
			cleanups = append(cleanups, entryCleanup)
		}
	}

	if len(cleanups) == 0 {
		return nil, nil
	}
	return runCleanups, nil
}

// resolveAndOverlay resolves a single source entry and overlays it into targetDir.
func resolveAndOverlay(entry api.SourceEntry, targetDir string) (func(), error) {
	uri := entry.URI()
	if uri == "" {
		return nil, nil
	}

	var localPath string
	var cleanup func()
	var err error

	if entry.Recursive && entry.OCM != "" {
		localPath, cleanup, err = resolve.ResolveOCMRecursive(entry.OCM)
	} else {
		localPath, cleanup, err = resolve.Resolve(uri)
	}
	if err != nil {
		return nil, fmt.Errorf("resolving %q: %w", uri, err)
	}

	dest := targetDir
	if entry.Path != "" {
		dest = filepath.Join(targetDir, entry.Path)
	}

	if err := overlaySource(localPath, dest); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, fmt.Errorf("overlaying %q: %w", uri, err)
	}

	return cleanup, nil
}

// overlaySource copies resolved content into dest.
// If resolvedPath is a directory, its contents are copied recursively.
// If resolvedPath is a file, it is copied into dest/.
func overlaySource(resolvedPath, dest string) error {
	info, err := os.Stat(resolvedPath)
	if err != nil {
		return fmt.Errorf("stat %s: %w", resolvedPath, err)
	}

	if info.IsDir() {
		return copyTree(resolvedPath, dest)
	}

	// Single file: ensure dest directory exists, then copy the file.
	if err := os.MkdirAll(dest, 0o750); err != nil {
		return fmt.Errorf("creating directory %s: %w", dest, err)
	}
	target := filepath.Join(dest, filepath.Base(resolvedPath))
	data, err := os.ReadFile(resolvedPath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", resolvedPath, err)
	}
	if err := os.WriteFile(target, data, info.Mode()); err != nil {
		return fmt.Errorf("writing %s: %w", target, err)
	}
	return nil
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

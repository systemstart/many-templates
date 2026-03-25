package processing

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"github.com/systemstart/many-templates/pkg/api"
	"github.com/systemstart/many-templates/pkg/resolve"
	"github.com/systemstart/many-templates/pkg/steps"
)

const stagingDirName = ".many-tmp"

// cleanStagingDir removes a stale staging directory if it exists.
// Best-effort, non-fatal.
func cleanStagingDir(stagingDir string) {
	if _, err := os.Stat(stagingDir); err == nil {
		slog.Warn("removing stale staging directory", "path", stagingDir)
		if err := os.RemoveAll(stagingDir); err != nil {
			slog.Warn("failed to remove stale staging directory", "path", stagingDir, "error", err)
		}
	}
}

// promoteStaging moves all entries from stagingDir into targetDir, then removes
// the now-empty staging directory.
func promoteStaging(stagingDir, targetDir string) error {
	entries, err := os.ReadDir(stagingDir)
	if err != nil {
		return fmt.Errorf("reading staging directory: %w", err)
	}

	for _, e := range entries {
		src := filepath.Join(stagingDir, e.Name())
		dst := filepath.Join(targetDir, e.Name())
		if err := os.Rename(src, dst); err != nil {
			return fmt.Errorf("promoting %s: %w", e.Name(), err)
		}
	}

	if err := os.Remove(stagingDir); err != nil {
		return fmt.Errorf("removing staging directory: %w", err)
	}
	return nil
}

// RunPipeline executes a single pipeline's steps sequentially in workDir.
// File sources with relative paths are resolved relative to pipeline.Dir.
// When updateSHA256 is true, HTTPS sources with empty sha256 fields will have
// their computed hashes written back to the pipeline file.
func RunPipeline(pipeline *api.Pipeline, globalContext map[string]any, workDir string, updateSHA256 bool) error {
	ctx := MergeContext(globalContext, pipeline.Context)
	if err := InterpolateContext(ctx); err != nil {
		return fmt.Errorf("interpolating context: %w", err)
	}

	for _, stepCfg := range pipeline.Pipeline {
		slog.Info("running step", "pipeline", pipeline.FilePath, "step", stepCfg.Name, "type", stepCfg.Type)
		if err := runStep(stepCfg, pipeline, ctx, workDir, updateSHA256); err != nil {
			return err
		}
	}

	return nil
}

func runStep(stepCfg api.StepConfig, pipeline *api.Pipeline, ctx map[string]any, workDir string, updateSHA256 bool) error {
	if len(stepCfg.Source) > 0 {
		pipelinePath := ""
		if updateSHA256 {
			pipelinePath = pipeline.FilePath
		}
		cleanup, err := resolveSources(stepCfg.Source, workDir, pipeline.Dir, pipelinePath)
		if err != nil {
			return fmt.Errorf("step %q: resolving sources: %w", stepCfg.Name, err)
		}
		if cleanup != nil {
			defer cleanup()
		}
	}

	step, err := steps.NewStep(stepCfg)
	if err != nil {
		return fmt.Errorf("creating step %q: %w", stepCfg.Name, err)
	}

	sctx := buildStepContext(workDir, pipeline.Dir, ctx)

	result, err := step.Run(sctx)
	if err != nil {
		return fmt.Errorf("step %q failed: %w", stepCfg.Name, err)
	}

	if result != nil {
		removeBuildArtifacts(workDir, result.Cleanup)
	}

	if len(stepCfg.Exclude) > 0 {
		if err := applyExcludes(workDir, stepCfg.Exclude); err != nil {
			return fmt.Errorf("step %q: applying excludes: %w", stepCfg.Name, err)
		}
	}

	return nil
}

func buildStepContext(workDir string, sourceDir string, ctx map[string]any) steps.StepContext {
	return steps.StepContext{
		WorkDir:      workDir,
		SourceDir:    sourceDir,
		TemplateData: ctx,
	}
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

// applyExcludes walks workDir, matches files against glob patterns, and deletes
// matching files. Empty directories are cleaned up bottom-up afterwards.
func applyExcludes(workDir string, patterns []string) error {
	toRemove, err := collectExcludedFiles(workDir, patterns)
	if err != nil {
		return err
	}

	for _, p := range toRemove {
		slog.Debug("excluding file", "path", p)
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("removing excluded file %s: %w", p, err)
		}
	}

	return cleanEmptyDirs(workDir)
}

func collectExcludedFiles(workDir string, patterns []string) ([]string, error) {
	var toRemove []string

	err := filepath.WalkDir(workDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}
		if d.IsDir() {
			return nil
		}

		rel, relErr := filepath.Rel(workDir, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}

		matched, matchErr := matchesAnyPattern(filepath.ToSlash(rel), patterns)
		if matchErr != nil {
			return matchErr
		}
		if matched {
			toRemove = append(toRemove, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking for excludes: %w", err)
	}
	return toRemove, nil
}

func matchesAnyPattern(path string, patterns []string) (bool, error) {
	for _, pattern := range patterns {
		matched, err := doublestar.Match(pattern, path)
		if err != nil {
			return false, fmt.Errorf("matching pattern %q against %q: %w", pattern, path, err)
		}
		if matched {
			return true, nil
		}
	}
	return false, nil
}

// cleanEmptyDirs removes empty directories under root, bottom-up.
func cleanEmptyDirs(root string) error {
	// Collect all directories, then check from deepest to shallowest.
	var dirs []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // best-effort
		}
		if d.IsDir() && path != root {
			dirs = append(dirs, path)
		}
		return nil
	})
	if err != nil {
		return nil // best-effort
	}

	// Sort deepest first.
	sort.Slice(dirs, func(i, j int) bool {
		return strings.Count(dirs[i], string(filepath.Separator)) > strings.Count(dirs[j], string(filepath.Separator))
	})

	for _, d := range dirs {
		entries, readErr := os.ReadDir(d)
		if readErr != nil {
			continue
		}
		if len(entries) == 0 {
			_ = os.Remove(d)
		}
	}
	return nil
}

// filterByInclude keeps pipelines whose top-level directory (relative to baseDir)
// is in the include set, or that are at root (".").
func filterByInclude(pipelines []*api.Pipeline, include []string, baseDir string) []*api.Pipeline {
	if len(include) == 0 {
		return pipelines
	}

	includeSet := make(map[string]bool, len(include))
	for _, name := range include {
		includeSet[name] = true
	}

	var filtered []*api.Pipeline
	for _, p := range pipelines {
		rel, err := filepath.Rel(baseDir, p.Dir)
		if err != nil {
			slog.Warn("cannot compute relative path for pipeline", "dir", p.Dir, "base", baseDir, "error", err)
			continue
		}

		// Extract top-level directory component.
		topLevel := rel
		if i := strings.IndexByte(filepath.ToSlash(rel), '/'); i >= 0 {
			topLevel = rel[:i]
		}

		// Always keep root-level pipelines; filter subdirectories by include set.
		if topLevel == "." || includeSet[topLevel] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// RunAll discovers pipelines in inputDir, executes each in a fresh temp directory,
// copies results to a staging directory, and promotes to outputDir on success.
func RunAll(inputDir, outputDir string, globalContext map[string]any, maxDepth int, updateSHA256 bool) error {
	absInputDir, err := filepath.Abs(inputDir)
	if err != nil {
		return fmt.Errorf("resolving input directory: %w", err)
	}

	stagingDir := filepath.Join(outputDir, stagingDirName)
	cleanStagingDir(stagingDir)

	if err := os.MkdirAll(stagingDir, 0o750); err != nil {
		return fmt.Errorf("creating staging directory: %w", err)
	}

	pipelines, err := DiscoverPipelines(absInputDir, maxDepth)
	if err != nil {
		return fmt.Errorf("discovering pipelines: %w", err)
	}

	if len(pipelines) == 0 {
		slog.Warn("no .many.yaml files found", "dir", inputDir)
		return promoteStaging(stagingDir, outputDir)
	}

	slog.Info("discovered pipelines", "count", len(pipelines))

	var failed []string
	for _, p := range pipelines {
		if err := executePipelineToStaging(p, globalContext, absInputDir, stagingDir, updateSHA256); err != nil {
			slog.Error("pipeline failed", "path", p.FilePath, "error", err)
			failed = append(failed, p.FilePath)
		}
	}

	if err := removeConfigFiles(stagingDir); err != nil {
		slog.Error("failed to clean up .many.yaml files", "error", err)
	}

	if len(failed) > 0 {
		slog.Error("staging directory preserved for inspection", "path", stagingDir)
		return fmt.Errorf("%d pipeline(s) failed: %v", len(failed), failed)
	}

	return promoteStaging(stagingDir, outputDir)
}

// executePipelineToStaging runs a pipeline in a fresh temp dir and copies
// the results to the appropriate location in stagingDir.
func executePipelineToStaging(p *api.Pipeline, ctx map[string]any, baseDir, stagingDir string, updateSHA256 bool) error {
	slog.Info("executing pipeline", "path", p.FilePath)

	workDir, err := os.MkdirTemp("", "many-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	if err := RunPipeline(p, ctx, workDir, updateSHA256); err != nil {
		return err
	}

	slog.Info("pipeline succeeded", "path", p.FilePath)

	rel, err := filepath.Rel(baseDir, p.Dir)
	if err != nil {
		return fmt.Errorf("computing relative path for pipeline: %w", err)
	}

	destDir := filepath.Join(stagingDir, rel)
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}
	return copyTree(workDir, destDir)
}

// RunSingle loads a pipeline directly, runs it in a temp directory, and promotes
// results to outputDir.
func RunSingle(pipelineFile, inputDir, outputDir string, globalContext map[string]any, updateSHA256 bool) error {
	pipeline, err := api.LoadPipeline(pipelineFile)
	if err != nil {
		return fmt.Errorf("loading pipeline: %w", err)
	}

	rel, ok := relativeToInput(pipelineFile, inputDir)
	if !ok {
		return fmt.Errorf("pipeline file %q is not within input directory %q", pipelineFile, inputDir)
	}

	stagingDir := filepath.Join(outputDir, stagingDirName)
	cleanStagingDir(stagingDir)

	if err := os.MkdirAll(stagingDir, 0o750); err != nil {
		return fmt.Errorf("creating staging directory: %w", err)
	}

	workDir, err := os.MkdirTemp("", "many-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()

	slog.Info("executing single pipeline", "path", pipeline.FilePath)
	if pErr := RunPipeline(pipeline, globalContext, workDir, updateSHA256); pErr != nil {
		slog.Error("staging directory preserved for inspection", "path", stagingDir)
		return fmt.Errorf("pipeline failed: %w", pErr)
	}

	pipelineRel := filepath.Dir(rel)
	destDir := filepath.Join(stagingDir, pipelineRel)
	if err := os.MkdirAll(destDir, 0o750); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}
	if err := copyTree(workDir, destDir); err != nil {
		return fmt.Errorf("copying pipeline output: %w", err)
	}

	if err := removeConfigFiles(stagingDir); err != nil {
		slog.Error("failed to clean up .many.yaml files", "error", err)
	}

	return promoteStaging(stagingDir, outputDir)
}

// RunInstances processes each instance: pipeline discovery, temp-dir execution, promotion.
func RunInstances(cfg *api.InstancesConfig, inputDir, outputDir string, globalContext map[string]any, maxDepth int, updateSHA256 bool) error {
	var failed []string

	for _, inst := range cfg.Instances {
		slog.Info("processing instance", "name", inst.Name)

		if err := runInstance(inst, inputDir, outputDir, globalContext, maxDepth, updateSHA256); err != nil {
			slog.Error("instance failed", "name", inst.Name, "error", err)
			failed = append(failed, inst.Name)
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("%d instance(s) failed: %v", len(failed), failed)
	}

	return nil
}

func runInstance(inst api.Instance, inputDir, outputDir string, globalContext map[string]any, maxDepth int, updateSHA256 bool) error {
	instInputDir, cleanup, err := resolveInstanceInput(inst.Input, inputDir)
	if err != nil {
		return err
	}
	if cleanup != nil {
		defer cleanup()
	}

	instOutputDir := filepath.Join(outputDir, inst.Output)
	instContext := MergeContext(globalContext, inst.Context)

	stagingDir, err := prepareStagingDir(instOutputDir)
	if err != nil {
		return err
	}

	pipelines, err := DiscoverPipelines(instInputDir, maxDepth)
	if err != nil {
		return fmt.Errorf("discovering pipelines: %w", err)
	}

	pipelines = filterByInclude(pipelines, inst.Include, instInputDir)

	if len(pipelines) == 0 {
		slog.Warn("no .many.yaml files found for instance", "name", inst.Name)
	}

	slog.Info("discovered pipelines for instance", "name", inst.Name, "count", len(pipelines))

	var pipelineFailed bool
	for _, p := range pipelines {
		if err := executePipelineToStaging(p, instContext, instInputDir, stagingDir, updateSHA256); err != nil {
			slog.Error("pipeline failed", "instance", inst.Name, "path", p.FilePath, "error", err)
			pipelineFailed = true
		}
	}

	if err := removeConfigFiles(stagingDir); err != nil {
		slog.Error("failed to clean up .many.yaml files", "instance", inst.Name, "error", err)
	}

	if pipelineFailed {
		slog.Error("staging directory preserved for inspection", "path", stagingDir)
		return fmt.Errorf("one or more pipelines failed")
	}

	return promoteStaging(stagingDir, instOutputDir)
}

// prepareStagingDir creates the output directory and a clean staging subdirectory.
func prepareStagingDir(outputDir string) (string, error) {
	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		return "", fmt.Errorf("creating output directory: %w", err)
	}
	stagingDir := filepath.Join(outputDir, stagingDirName)
	cleanStagingDir(stagingDir)
	if err := os.MkdirAll(stagingDir, 0o750); err != nil {
		return "", fmt.Errorf("creating staging directory: %w", err)
	}
	return stagingDir, nil
}

// resolveInstanceInput determines the effective input directory for an instance.
// The returned path is always absolute.
func resolveInstanceInput(input, inputDir string) (string, func(), error) {
	var dir string
	var cleanup func()

	switch {
	case input == "":
		dir = inputDir
	case resolve.IsRemote(input):
		resolved, c, _, err := resolve.Resolve(input, "")
		if err != nil {
			return "", nil, fmt.Errorf("resolving remote input %q: %w", input, err)
		}
		dir, cleanup = resolved, c
	default:
		dir = filepath.Join(inputDir, input)
	}

	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", nil, fmt.Errorf("resolving absolute path for input: %w", err)
	}
	return abs, cleanup, nil
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
// File sources with relative paths are resolved relative to baseDir.
// If pipelineFilePath is non-empty, any HTTPS sources with empty sha256 fields
// will have their computed hashes written back to the pipeline file.
func resolveSources(sources api.Sources, targetDir, baseDir, pipelineFilePath string) (cleanup func(), err error) {
	cleanups, updates, err := resolveAllEntries(sources, targetDir, baseDir)
	if err != nil {
		return nil, err
	}

	if len(updates) > 0 && pipelineFilePath != "" {
		if err := api.UpdateSourceSHA256(pipelineFilePath, updates); err != nil {
			slog.Warn("failed to write back computed sha256", "file", pipelineFilePath, "error", err)
		}
	}

	if len(cleanups) == 0 {
		return nil, nil
	}
	return func() {
		for i := len(cleanups) - 1; i >= 0; i-- {
			cleanups[i]()
		}
	}, nil
}

func resolveAllEntries(sources api.Sources, targetDir, baseDir string) ([]func(), map[string]string, error) {
	var cleanups []func()
	var updates map[string]string

	for i, entry := range sources {
		entryCleanup, update, err := resolveAndOverlay(entry, targetDir, baseDir)
		if err != nil {
			for j := len(cleanups) - 1; j >= 0; j-- {
				cleanups[j]()
			}
			return nil, nil, fmt.Errorf("source[%d]: %w", i, err)
		}
		if entryCleanup != nil {
			cleanups = append(cleanups, entryCleanup)
		}
		if update != nil {
			if updates == nil {
				updates = make(map[string]string)
			}
			updates[update.url] = update.sha256
		}
	}

	return cleanups, updates, nil
}

// sha256Update records a computed sha256 for an HTTPS source URL.
type sha256Update struct {
	url    string
	sha256 string
}

// resolveAndOverlay resolves a single source entry and overlays it into targetDir.
// File sources with relative paths are resolved relative to baseDir.
// If the entry is an HTTPS source with an empty sha256 and a hash was computed,
// it is returned as a sha256Update.
func resolveAndOverlay(entry api.SourceEntry, targetDir, baseDir string) (func(), *sha256Update, error) {
	uri := entry.URI()
	if uri == "" {
		return nil, nil, nil
	}

	// For file sources with relative paths, resolve relative to baseDir.
	if entry.File != "" && !filepath.IsAbs(uri) {
		uri = filepath.Join(baseDir, uri)
	}

	localPath, cleanup, computed, err := resolveEntry(entry, uri)
	if err != nil {
		return nil, nil, fmt.Errorf("resolving %q: %w", uri, err)
	}

	dest := targetDir
	if entry.Path != "" {
		dest = filepath.Join(targetDir, entry.Path)
	}

	if overlayErr := overlaySource(localPath, dest); overlayErr != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, nil, fmt.Errorf("overlaying %q: %w", uri, overlayErr)
	}

	update := buildSHA256Update(entry, computed)
	return cleanup, update, nil
}

func buildSHA256Update(entry api.SourceEntry, computed string) *sha256Update {
	if entry.HTTPS != "" && entry.SHA256 == "" && computed != "" {
		return &sha256Update{url: entry.HTTPS, sha256: computed}
	}
	return nil
}

func resolveEntry(entry api.SourceEntry, uri string) (string, func(), string, error) {
	if entry.Recursive && entry.OCM != "" {
		path, cleanup, err := resolve.ResolveOCMRecursive(entry.OCM)
		if err != nil {
			return "", nil, "", fmt.Errorf("resolving OCM recursive: %w", err)
		}
		return path, cleanup, "", nil
	}
	if entry.Helm != "" {
		path, cleanup, err := resolve.ResolveHelm(entry.Helm, entry.Repo, entry.Version)
		if err != nil {
			return "", nil, "", fmt.Errorf("resolving helm chart: %w", err)
		}
		return path, cleanup, "", nil
	}
	path, cleanup, computed, err := resolve.Resolve(uri, entry.SHA256)
	if err != nil {
		return "", nil, "", fmt.Errorf("resolving source: %w", err)
	}
	return path, cleanup, computed, nil
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
		return overlayDir(resolvedPath, dest)
	}

	return overlaySingleFile(resolvedPath, dest, info)
}

func overlayDir(src, dest string) error {
	err := filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}
		rel, relErr := filepath.Rel(src, path)
		if relErr != nil {
			return fmt.Errorf("computing relative path for %s: %w", path, relErr)
		}
		target := filepath.Join(dest, rel)
		if d.IsDir() {
			if mkErr := os.MkdirAll(target, 0o750); mkErr != nil {
				return fmt.Errorf("creating directory %s: %w", target, mkErr)
			}
			return nil
		}
		_, copyErr := copyFileEntry(path, target, d)
		if copyErr != nil {
			return copyErr
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("copying tree: %w", err)
	}
	return nil
}

func copyFileEntry(srcPath, target string, d fs.DirEntry) (string, error) {
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", srcPath, err)
	}
	info, err := d.Info()
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", srcPath, err)
	}
	if err := os.WriteFile(target, data, info.Mode()); err != nil {
		return "", fmt.Errorf("writing %s: %w", target, err)
	}
	return target, nil
}

func overlaySingleFile(resolvedPath, dest string, info os.FileInfo) error {
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

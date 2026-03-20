package processing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func TestRunPipeline_ContextMerge(t *testing.T) {
	// Create a source directory with a template file.
	sourceDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(sourceDir, "test.yaml"), []byte("{{ .global }}-{{ .local }}"), 0o600); err != nil {
		t.Fatal(err)
	}

	workDir := t.TempDir()

	pipeline := &api.Pipeline{
		Dir:     sourceDir,
		Context: map[string]any{"local": "L"},
		Pipeline: []api.StepConfig{
			{
				Name:     "render",
				Type:     api.StepTypeTemplate,
				Source:   api.Sources{{File: "."}},
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"*"}}},
			},
		},
	}

	globalCtx := map[string]any{"global": "G", "local": "overridden"}

	if err := RunPipeline(pipeline, globalCtx, workDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workDir, "test.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "G-L" {
		t.Errorf("expected 'G-L', got %q", string(content))
	}
}

func TestCopyTree(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create source files
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := copyTree(src, dst); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify
	content, err := os.ReadFile(filepath.Join(dst, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "hello" {
		t.Errorf("expected 'hello', got %q", string(content))
	}

	content, err = os.ReadFile(filepath.Join(dst, "sub", "b.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "world" {
		t.Errorf("expected 'world', got %q", string(content))
	}
}

func TestRemoveConfigFiles(t *testing.T) {
	root := t.TempDir()

	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".many.yaml"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", ".many.yaml"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.yaml"), []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := removeConfigFiles(root); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, ".many.yaml")); !os.IsNotExist(err) {
		t.Error("root .many.yaml should be removed")
	}
	if _, err := os.Stat(filepath.Join(root, "sub", ".many.yaml")); !os.IsNotExist(err) {
		t.Error("sub/.many.yaml should be removed")
	}
	if _, err := os.Stat(filepath.Join(root, "keep.yaml")); err != nil {
		t.Error("keep.yaml should still exist")
	}
}

func TestRunAll_Integration(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	// Create a source tree with a .many.yaml and a template file.
	if err := os.WriteFile(filepath.Join(src, ".many.yaml"), []byte(`
context:
  name: world
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("Hello {{ .name }}!"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := RunAll(src, dst, nil, -1, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Check rendered output
	content, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "Hello world!" {
		t.Errorf("expected 'Hello world!', got %q", string(content))
	}

	// .many.yaml should be removed from output
	if _, err := os.Stat(filepath.Join(dst, ".many.yaml")); !os.IsNotExist(err) {
		t.Error(".many.yaml should be removed from output")
	}
}

func TestRunAll_NoPipelines(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}

	// No pipelines => empty output (nothing promoted)
	if err := RunAll(src, dst, nil, -1, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// file.txt should NOT be in output (no pipeline, no copy)
	if _, err := os.Stat(filepath.Join(dst, "file.txt")); !os.IsNotExist(err) {
		t.Error("file.txt should not be in output when no pipelines found")
	}
}

func TestRemoveBuildArtifacts(t *testing.T) {
	dir := t.TempDir()

	// Create files and directories that represent build artifacts
	writeFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("kustomization.yaml", "resources: [secret.yaml]")
	writeFile("values.yaml", "key: val")
	writeFile("secret.yaml", "apiVersion: v1")

	if err := os.MkdirAll(filepath.Join(dir, "charts", "test"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile("charts/test/Chart.yaml", "name: test")

	if err := os.MkdirAll(filepath.Join(dir, "manifests"), 0o750); err != nil {
		t.Fatal(err)
	}
	writeFile("manifests/deploy.yaml", "apiVersion: apps/v1")

	// Run cleanup
	removeBuildArtifacts(dir, []string{
		"kustomization.yaml",
		"charts",
		"values.yaml",
		"secret.yaml",
	})

	// Verify artifacts are removed
	for _, name := range []string{"kustomization.yaml", "values.yaml", "secret.yaml", "charts"} {
		if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
			t.Errorf("%s should be removed", name)
		}
	}

	// Verify manifests are preserved
	if _, err := os.Stat(filepath.Join(dir, "manifests", "deploy.yaml")); err != nil {
		t.Error("manifests/deploy.yaml should still exist")
	}
}

func TestRunSingle(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	if err := os.WriteFile(filepath.Join(src, ".many.yaml"), []byte(`
context:
  v: replaced
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.yaml"]
        exclude: [".many.yaml"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "data.yaml"), []byte("value: {{ .v }}"), 0o600); err != nil {
		t.Fatal(err)
	}

	pipelineFile := filepath.Join(src, ".many.yaml")
	if err := RunSingle(pipelineFile, src, dst, nil, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dst, "data.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "value: replaced" {
		t.Errorf("expected 'value: replaced', got %q", string(content))
	}

	// .many.yaml should be cleaned up
	if _, err := os.Stat(filepath.Join(dst, ".many.yaml")); !os.IsNotExist(err) {
		t.Error(".many.yaml should be removed from output")
	}
}

func TestRelativeToInput(t *testing.T) {
	inputDir := t.TempDir()
	inner := filepath.Join(inputDir, "sub", "file.yaml")
	if err := os.MkdirAll(filepath.Join(inputDir, "sub"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inner, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}

	// File inside input dir
	rel, ok := relativeToInput(inner, inputDir)
	if !ok {
		t.Fatal("expected ok=true for file inside input dir")
	}
	if rel != filepath.Join("sub", "file.yaml") {
		t.Errorf("expected sub/file.yaml, got %q", rel)
	}

	// File outside input dir
	outside := filepath.Join(t.TempDir(), "other.yaml")
	if err := os.WriteFile(outside, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, ok = relativeToInput(outside, inputDir)
	if ok {
		t.Error("expected ok=false for file outside input dir")
	}

	// File at input root
	root := filepath.Join(inputDir, "root.yaml")
	if err := os.WriteFile(root, []byte("test"), 0o600); err != nil {
		t.Fatal(err)
	}
	rel, ok = relativeToInput(root, inputDir)
	if !ok {
		t.Fatal("expected ok=true for file at input root")
	}
	if rel != "root.yaml" {
		t.Errorf("expected root.yaml, got %q", rel)
	}
}

func TestRunAll_MaxDepth(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	// Create nested pipelines at depth 0 and depth 1
	writeTestFile(t, filepath.Join(src, ".many.yaml"), `
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`)
	writeTestFile(t, filepath.Join(src, "root.txt"), "root")

	sub := filepath.Join(src, "sub")
	mkdirAll(t, sub)
	writeTestFile(t, filepath.Join(sub, ".many.yaml"), `
context:
  val: deep
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["*.txt"]
`)
	writeTestFile(t, filepath.Join(sub, "deep.txt"), "{{ .val }}")

	// maxDepth=0 should only process root pipeline
	if err := RunAll(src, dst, nil, 0, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// root.txt should be rendered
	assertFileContent(t, filepath.Join(dst, "root.txt"), "root")

	// sub/deep.txt should exist but NOT be rendered (sub pipeline skipped,
	// but root pipeline's source: file: "." copies the entire directory)
	assertFileContent(t, filepath.Join(dst, "sub", "deep.txt"), "{{ .val }}")
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func mkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o750); err != nil {
		t.Fatal(err)
	}
}

func assertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("expected file %s to exist: %v", path, err)
	}
	if string(content) != expected {
		t.Errorf("expected %q, got %q in %s", expected, string(content), path)
	}
}

func assertNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected %s to not exist", path)
	}
}

func setupInstancesSource(t *testing.T) string {
	t.Helper()
	src := t.TempDir()
	appDir := filepath.Join(src, "app")
	mkdirAll(t, appDir)
	writeTestFile(t, filepath.Join(appDir, ".many.yaml"), `
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`)
	writeTestFile(t, filepath.Join(appDir, "greeting.txt"), "Hello {{ .name }}!")
	return src
}

func TestRunInstances_Integration(t *testing.T) {
	src := setupInstancesSource(t)
	dst := filepath.Join(t.TempDir(), "output")

	cfg := &api.InstancesConfig{
		Instances: []api.Instance{
			{Name: "alpha", Output: "alpha", Context: map[string]any{"name": "Alpha"}},
			{Name: "beta", Output: "beta", Context: map[string]any{"name": "Beta"}},
		},
	}

	if err := RunInstances(cfg, src, dst, nil, -1, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "alpha", "app", "greeting.txt"), "Hello Alpha!")
	assertFileContent(t, filepath.Join(dst, "beta", "app", "greeting.txt"), "Hello Beta!")
	assertNotExists(t, filepath.Join(dst, "alpha", "app", ".many.yaml"))
	assertNotExists(t, filepath.Join(dst, "beta", "app", ".many.yaml"))
}

func setupMultiAppSource(t *testing.T, apps []string) string {
	t.Helper()
	src := t.TempDir()
	pipelineYAML := `
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`
	for _, name := range apps {
		dir := filepath.Join(src, name)
		mkdirAll(t, dir)
		writeTestFile(t, filepath.Join(dir, ".many.yaml"), pipelineYAML)
		writeTestFile(t, filepath.Join(dir, "data.txt"), "{{ .v }}")
	}
	return src
}

func TestRunInstances_IncludeFilter(t *testing.T) {
	src := setupMultiAppSource(t, []string{"app1", "app2"})
	dst := filepath.Join(t.TempDir(), "output")

	cfg := &api.InstancesConfig{
		Instances: []api.Instance{
			{Name: "filtered", Output: "out", Include: []string{"app1"}, Context: map[string]any{"v": "ok"}},
		},
	}

	if err := RunInstances(cfg, src, dst, nil, -1, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "out", "app1", "data.txt"), "ok")
	assertNotExists(t, filepath.Join(dst, "out", "app2"))
}

func TestRunInstances_FailedInstance(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")
	pipelineYAML := `
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`
	// good and bad apps
	for _, name := range []string{"good", "bad"} {
		dir := filepath.Join(src, name)
		mkdirAll(t, dir)
		writeTestFile(t, filepath.Join(dir, ".many.yaml"), pipelineYAML)
	}
	writeTestFile(t, filepath.Join(src, "good", "file.txt"), "{{ .v }}")
	writeTestFile(t, filepath.Join(src, "bad", "file.txt"), "{{ .missing | fail }}")

	cfg := &api.InstancesConfig{
		Instances: []api.Instance{
			{Name: "bad-inst", Output: "bad-out", Include: []string{"bad"}},
			{Name: "good-inst", Output: "good-out", Include: []string{"good"}, Context: map[string]any{"v": "works"}},
		},
	}

	err := RunInstances(cfg, src, dst, nil, -1, true)
	if err == nil {
		t.Fatal("expected error for failed instance")
	}
	if !strings.Contains(err.Error(), "instance(s) failed") {
		t.Errorf("unexpected error: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "good-out", "good", "file.txt"), "works")
}

func TestRunSingle_NonexistentPipelineFile(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := RunSingle(filepath.Join(src, ".many.yaml"), src, dst, nil, true)
	if err == nil {
		t.Fatal("expected error for nonexistent pipeline file")
	}
}

func TestRunSingle_PipelineOutsideInput(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	// Create a valid pipeline file outside the input directory
	otherDir := t.TempDir()
	pipelineFile := filepath.Join(otherDir, ".many.yaml")
	writeTestFile(t, pipelineFile, `
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["*.txt"]
`)
	writeTestFile(t, filepath.Join(src, "file.txt"), "content")

	err := RunSingle(pipelineFile, src, dst, nil, true)
	if err == nil {
		t.Fatal("expected error for pipeline file outside input dir")
	}
	if !strings.Contains(err.Error(), "not within input directory") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunInstances_WithInputSubdirectory(t *testing.T) {
	src := t.TempDir()

	// Create a subdirectory with pipeline
	sub := filepath.Join(src, "apps")
	mkdirAll(t, sub)
	writeTestFile(t, filepath.Join(sub, ".many.yaml"), `
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`)
	writeTestFile(t, filepath.Join(sub, "greeting.txt"), "Hello {{ .who }}!")

	dst := filepath.Join(t.TempDir(), "output")

	cfg := &api.InstancesConfig{
		Instances: []api.Instance{
			{Name: "sub-inst", Input: "apps", Output: "out", Context: map[string]any{"who": "World"}},
		},
	}

	if err := RunInstances(cfg, src, dst, nil, -1, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "out", "greeting.txt"), "Hello World!")
}

func TestRunInstances_EmptyInput(t *testing.T) {
	src := setupInstancesSource(t)
	dst := filepath.Join(t.TempDir(), "output")

	cfg := &api.InstancesConfig{
		Instances: []api.Instance{
			{Name: "empty-input", Input: "", Output: "out", Context: map[string]any{"name": "Test"}},
		},
	}

	if err := RunInstances(cfg, src, dst, nil, -1, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "out", "app", "greeting.txt"), "Hello Test!")
}

func TestRunPipeline_WithStepSourceBasic(t *testing.T) {
	// Create a source directory with template files
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "base.yaml"), "name: {{ .name }}")

	// Create pipeline working directory
	workDir := t.TempDir()

	pipeline := &api.Pipeline{
		Dir:     sourceDir,
		Context: map[string]any{"name": "test-value"},
		Pipeline: []api.StepConfig{
			{
				Name:     "render",
				Type:     api.StepTypeTemplate,
				Source:   api.Sources{{File: "."}},
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"*.yaml"}}},
			},
		},
	}

	if err := RunPipeline(pipeline, nil, workDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The source file should have been overlaid and rendered
	assertFileContent(t, filepath.Join(workDir, "base.yaml"), "name: test-value")
}

func TestRunPipeline_WithSourcePath(t *testing.T) {
	// Create a source directory with files
	sourceDir := t.TempDir()
	mkdirAll(t, filepath.Join(sourceDir, "crds"))
	writeTestFile(t, filepath.Join(sourceDir, "crds", "crd.yaml"), "kind: CRD")

	// Create pipeline working directory
	workDir := t.TempDir()

	pipeline := &api.Pipeline{
		Dir: sourceDir,
		Pipeline: []api.StepConfig{
			{
				Name: "render",
				Type: api.StepTypeTemplate,
				Source: api.Sources{{
					File: ".",
					Path: "subdir/",
				}},
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"**/*.yaml"}}},
			},
		},
	}

	if err := RunPipeline(pipeline, nil, workDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Files should land in the subdir/ path
	assertFileContent(t, filepath.Join(workDir, "subdir", "crds", "crd.yaml"), "kind: CRD")
}

func TestRunPipeline_WithStepSource(t *testing.T) {
	// Create a source directory with a template file
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "step-file.yaml"), "value: {{ .val }}")

	// Create pipeline working directory
	workDir := t.TempDir()

	pipeline := &api.Pipeline{
		Dir:     sourceDir,
		Context: map[string]any{"val": "from-step"},
		Pipeline: []api.StepConfig{
			{
				Name:     "render",
				Type:     api.StepTypeTemplate,
				Source:   api.Sources{{File: "."}},
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"*.yaml"}}},
			},
		},
	}

	if err := RunPipeline(pipeline, nil, workDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The step source file should be overlaid and rendered
	assertFileContent(t, filepath.Join(workDir, "step-file.yaml"), "value: from-step")
}

func TestOverlaySource_Directory(t *testing.T) {
	// Create source directory with nested files
	src := t.TempDir()
	mkdirAll(t, filepath.Join(src, "sub"))
	writeTestFile(t, filepath.Join(src, "a.txt"), "alpha")
	writeTestFile(t, filepath.Join(src, "sub", "b.txt"), "beta")

	// Overlay into a new destination
	dst := filepath.Join(t.TempDir(), "dest")
	paths, err := overlaySource(src, dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "a.txt"), "alpha")
	assertFileContent(t, filepath.Join(dst, "sub", "b.txt"), "beta")

	if len(paths) == 0 {
		t.Fatal("expected non-empty overlaid paths")
	}
}

func TestOverlaySource_SingleFile(t *testing.T) {
	// Create a single source file
	src := t.TempDir()
	writeTestFile(t, filepath.Join(src, "single.yaml"), "content: here")

	// Overlay the single file into a destination directory
	dst := filepath.Join(t.TempDir(), "dest")
	paths, err := overlaySource(filepath.Join(src, "single.yaml"), dst)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileContent(t, filepath.Join(dst, "single.yaml"), "content: here")

	if len(paths) != 1 {
		t.Fatalf("expected 1 overlaid path, got %d", len(paths))
	}
	if paths[0] != filepath.Join(dst, "single.yaml") {
		t.Errorf("expected %q, got %q", filepath.Join(dst, "single.yaml"), paths[0])
	}
}

func TestRemoveOverlaidFiles(t *testing.T) {
	dir := t.TempDir()

	// Create directory structure: dir/sub/deep/file.txt and dir/other.txt
	subDir := filepath.Join(dir, "sub")
	deepDir := filepath.Join(subDir, "deep")
	mkdirAll(t, deepDir)
	writeTestFile(t, filepath.Join(deepDir, "file.txt"), "temp")
	writeTestFile(t, filepath.Join(dir, "other.txt"), "keep") // not in overlaid paths

	overlaidPaths := []string{
		subDir,
		deepDir,
		filepath.Join(deepDir, "file.txt"),
	}

	removeOverlaidFiles(overlaidPaths)

	// file.txt should be removed
	assertNotExists(t, filepath.Join(deepDir, "file.txt"))
	// deep/ should be removed (was empty after file removal)
	assertNotExists(t, deepDir)
	// sub/ should be removed (was empty after deep/ removal)
	assertNotExists(t, subDir)
	// other.txt should remain (not in overlaid paths)
	assertFileContent(t, filepath.Join(dir, "other.txt"), "keep")
}

func TestRemoveOverlaidFiles_PreservesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()

	subDir := filepath.Join(dir, "shared")
	mkdirAll(t, subDir)
	writeTestFile(t, filepath.Join(subDir, "temp.txt"), "temp")
	writeTestFile(t, filepath.Join(subDir, "keep.txt"), "keep") // not in overlaid paths

	overlaidPaths := []string{
		subDir,
		filepath.Join(subDir, "temp.txt"),
	}

	removeOverlaidFiles(overlaidPaths)

	// temp.txt should be removed
	assertNotExists(t, filepath.Join(subDir, "temp.txt"))
	// shared/ should still exist because keep.txt is still there
	assertFileContent(t, filepath.Join(subDir, "keep.txt"), "keep")
}

func TestRunPipeline_TemporarySourceCleaned(t *testing.T) {
	// Create a source directory with files
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "upstream.yaml"), "kind: Deployment")

	// Create pipeline working directory
	workDir := t.TempDir()

	pipeline := &api.Pipeline{
		Dir: sourceDir,
		Pipeline: []api.StepConfig{
			{
				Name: "render",
				Type: api.StepTypeTemplate,
				Source: api.Sources{{
					File:      ".",
					Temporary: true,
				}},
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"*.yaml"}}},
			},
		},
	}

	if err := RunPipeline(pipeline, nil, workDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Temporary source file should be removed after pipeline execution
	assertNotExists(t, filepath.Join(workDir, "upstream.yaml"))
}

func TestRunPipeline_NonTemporarySourceRemains(t *testing.T) {
	// Create a source directory with files
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "base.yaml"), "kind: Service")

	// Create pipeline working directory
	workDir := t.TempDir()

	pipeline := &api.Pipeline{
		Dir: sourceDir,
		Pipeline: []api.StepConfig{
			{
				Name: "render",
				Type: api.StepTypeTemplate,
				Source: api.Sources{{
					File: ".",
				}},
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"*.yaml"}}},
			},
		},
	}

	if err := RunPipeline(pipeline, nil, workDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Non-temporary source file should remain
	assertFileContent(t, filepath.Join(workDir, "base.yaml"), "kind: Service")
}

func TestRunPipeline_TemporarySourceWithPath(t *testing.T) {
	// Create a source directory with files
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "upstream.yaml"), "kind: Deployment")

	// Create pipeline working directory with an existing file
	workDir := t.TempDir()
	writeTestFile(t, filepath.Join(workDir, "existing.yaml"), "kind: Namespace")

	pipeline := &api.Pipeline{
		Dir: sourceDir,
		Pipeline: []api.StepConfig{
			{
				Name: "render",
				Type: api.StepTypeTemplate,
				Source: api.Sources{{
					File:      ".",
					Path:      "vendor/",
					Temporary: true,
				}},
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"**/*.yaml"}}},
			},
		},
	}

	if err := RunPipeline(pipeline, nil, workDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Temporary source file and its directory should be cleaned up
	assertNotExists(t, filepath.Join(workDir, "vendor", "upstream.yaml"))
	assertNotExists(t, filepath.Join(workDir, "vendor"))

	// Existing file should remain
	assertFileContent(t, filepath.Join(workDir, "existing.yaml"), "kind: Namespace")
}

func TestRunAll_FailedPipeline(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	// Invalid template that will fail
	if err := os.WriteFile(filepath.Join(src, ".many.yaml"), []byte(`
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "bad.txt"), []byte("{{ .missing | fail }}"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := RunAll(src, dst, nil, -1, true)
	if err == nil {
		t.Fatal("expected error for failed pipeline")
	}
	if !strings.Contains(err.Error(), "pipeline(s) failed") {
		t.Errorf("unexpected error message: %v", err)
	}

	// Staging directory should be preserved for inspection
	stagingDir := filepath.Join(dst, ".many-tmp")
	if _, err := os.Stat(stagingDir); os.IsNotExist(err) {
		t.Error(".many-tmp staging directory should be preserved on failure")
	}
}

func TestRunAll_Success_NoStagingDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	writeTestFile(t, filepath.Join(src, ".many.yaml"), `
context:
  name: world
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`)
	writeTestFile(t, filepath.Join(src, "hello.txt"), "Hello {{ .name }}!")

	if err := RunAll(src, dst, nil, -1, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Staging directory should not exist after success
	assertNotExists(t, filepath.Join(dst, ".many-tmp"))

	// Output should be in dst directly
	assertFileContent(t, filepath.Join(dst, "hello.txt"), "Hello world!")
}

func TestRunSingle_Failure_LeavesStagingDir(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	writeTestFile(t, filepath.Join(src, ".many.yaml"), `
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`)
	writeTestFile(t, filepath.Join(src, "bad.txt"), "{{ .missing | fail }}")

	pipelineFile := filepath.Join(src, ".many.yaml")
	err := RunSingle(pipelineFile, src, dst, nil, true)
	if err == nil {
		t.Fatal("expected error for failed pipeline")
	}

	// Staging directory should be preserved
	stagingDir := filepath.Join(dst, ".many-tmp")
	if _, err := os.Stat(stagingDir); os.IsNotExist(err) {
		t.Error(".many-tmp staging directory should be preserved on failure")
	}
}

func TestRunInstances_PartialFailure_IndependentStaging(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")
	pipelineYAML := `
pipeline:
  - name: render
    type: template
    source:
      file: "."
    template:
      files:
        include: ["*.txt"]
`
	for _, name := range []string{"good", "bad"} {
		dir := filepath.Join(src, name)
		mkdirAll(t, dir)
		writeTestFile(t, filepath.Join(dir, ".many.yaml"), pipelineYAML)
	}
	writeTestFile(t, filepath.Join(src, "good", "file.txt"), "{{ .v }}")
	writeTestFile(t, filepath.Join(src, "bad", "file.txt"), "{{ .missing | fail }}")

	cfg := &api.InstancesConfig{
		Instances: []api.Instance{
			{Name: "bad-inst", Output: "bad-out", Include: []string{"bad"}},
			{Name: "good-inst", Output: "good-out", Include: []string{"good"}, Context: map[string]any{"v": "works"}},
		},
	}

	err := RunInstances(cfg, src, dst, nil, -1, true)
	if err == nil {
		t.Fatal("expected error for failed instance")
	}

	// Successful instance should be promoted (no staging dir)
	assertFileContent(t, filepath.Join(dst, "good-out", "good", "file.txt"), "works")
	assertNotExists(t, filepath.Join(dst, "good-out", ".many-tmp"))

	// Failed instance should have staging dir preserved
	if _, err := os.Stat(filepath.Join(dst, "bad-out", ".many-tmp")); os.IsNotExist(err) {
		t.Error("failed instance should have .many-tmp preserved")
	}
}

func TestCleanStagingDir_RemovesStale(t *testing.T) {
	dir := t.TempDir()
	stagingDir := filepath.Join(dir, ".many-tmp")
	mkdirAll(t, stagingDir)
	writeTestFile(t, filepath.Join(stagingDir, "stale.txt"), "old data")

	cleanStagingDir(stagingDir)

	assertNotExists(t, stagingDir)
}

func TestPromoteStaging(t *testing.T) {
	parent := t.TempDir()
	stagingDir := filepath.Join(parent, ".many-tmp")
	mkdirAll(t, stagingDir)
	mkdirAll(t, filepath.Join(stagingDir, "sub"))
	writeTestFile(t, filepath.Join(stagingDir, "a.txt"), "alpha")
	writeTestFile(t, filepath.Join(stagingDir, "sub", "b.txt"), "beta")

	if err := promoteStaging(stagingDir, parent); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Files should be in parent
	assertFileContent(t, filepath.Join(parent, "a.txt"), "alpha")
	assertFileContent(t, filepath.Join(parent, "sub", "b.txt"), "beta")

	// Staging dir should be gone
	assertNotExists(t, stagingDir)
}

// --- New tests for applyExcludes and filterByInclude ---

func TestApplyExcludes(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "keep.yaml"), "keep")
	writeTestFile(t, filepath.Join(dir, "remove.tmp"), "remove")
	writeTestFile(t, filepath.Join(dir, "also-keep.txt"), "keep")

	if err := applyExcludes(dir, []string{"*.tmp"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertFileContent(t, filepath.Join(dir, "keep.yaml"), "keep")
	assertFileContent(t, filepath.Join(dir, "also-keep.txt"), "keep")
	assertNotExists(t, filepath.Join(dir, "remove.tmp"))
}

func TestApplyExcludes_DoublestarPattern(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "sub", "deep"))
	writeTestFile(t, filepath.Join(dir, "top.tmp"), "remove")
	writeTestFile(t, filepath.Join(dir, "sub", "mid.tmp"), "remove")
	writeTestFile(t, filepath.Join(dir, "sub", "deep", "bottom.tmp"), "remove")
	writeTestFile(t, filepath.Join(dir, "sub", "keep.yaml"), "keep")

	if err := applyExcludes(dir, []string{"**/*.tmp"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertNotExists(t, filepath.Join(dir, "top.tmp"))
	assertNotExists(t, filepath.Join(dir, "sub", "mid.tmp"))
	assertNotExists(t, filepath.Join(dir, "sub", "deep", "bottom.tmp"))
	assertFileContent(t, filepath.Join(dir, "sub", "keep.yaml"), "keep")
}

func TestApplyExcludes_CleansEmptyDirs(t *testing.T) {
	dir := t.TempDir()
	mkdirAll(t, filepath.Join(dir, "empty-after", "nested"))
	writeTestFile(t, filepath.Join(dir, "empty-after", "nested", "file.tmp"), "remove")
	writeTestFile(t, filepath.Join(dir, "keep.yaml"), "keep")

	if err := applyExcludes(dir, []string{"**/*.tmp"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	assertNotExists(t, filepath.Join(dir, "empty-after", "nested", "file.tmp"))
	assertNotExists(t, filepath.Join(dir, "empty-after", "nested"))
	assertNotExists(t, filepath.Join(dir, "empty-after"))
	assertFileContent(t, filepath.Join(dir, "keep.yaml"), "keep")
}

func TestFilterByInclude(t *testing.T) {
	baseDir := "/base"
	pipelines := []*api.Pipeline{
		{Dir: "/base/app1"},
		{Dir: "/base/app2"},
		{Dir: "/base/app3"},
		{Dir: "/base"},
	}

	filtered := filterByInclude(pipelines, []string{"app1", "app3"}, baseDir)

	if len(filtered) != 3 {
		t.Fatalf("expected 3 pipelines (app1, app3, root), got %d", len(filtered))
	}

	dirs := make([]string, len(filtered))
	for i, p := range filtered {
		dirs[i] = p.Dir
	}
	for _, expected := range []string{"/base/app1", "/base/app3", "/base"} {
		found := false
		for _, d := range dirs {
			if d == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected pipeline dir %q in filtered result", expected)
		}
	}
}

func TestFilterByInclude_EmptyInclude(t *testing.T) {
	pipelines := []*api.Pipeline{
		{Dir: "/base/app1"},
		{Dir: "/base/app2"},
	}

	filtered := filterByInclude(pipelines, nil, "/base")

	if len(filtered) != 2 {
		t.Fatalf("expected 2 pipelines (empty include = keep all), got %d", len(filtered))
	}
}

func TestRunStep_WithExclude(t *testing.T) {
	// Create a source directory with files
	sourceDir := t.TempDir()
	writeTestFile(t, filepath.Join(sourceDir, "app.yaml"), "kind: Deployment")
	writeTestFile(t, filepath.Join(sourceDir, "build.tmp"), "build artifact")
	writeTestFile(t, filepath.Join(sourceDir, "secret.env"), "PASSWORD=x")

	workDir := t.TempDir()

	pipeline := &api.Pipeline{
		Dir: sourceDir,
		Pipeline: []api.StepConfig{
			{
				Name:     "render",
				Type:     api.StepTypeTemplate,
				Source:   api.Sources{{File: "."}},
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"*.yaml"}}},
				Exclude:  []string{"*.tmp", "*.env"},
			},
		},
	}

	if err := RunPipeline(pipeline, nil, workDir, true); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// app.yaml should remain
	assertFileContent(t, filepath.Join(workDir, "app.yaml"), "kind: Deployment")
	// Excluded files should be removed
	assertNotExists(t, filepath.Join(workDir, "build.tmp"))
	assertNotExists(t, filepath.Join(workDir, "secret.env"))
}

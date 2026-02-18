package processing

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func TestRunPipeline_ContextMerge(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("{{ .global }}-{{ .local }}"), 0600); err != nil {
		t.Fatal(err)
	}

	pipeline := &api.Pipeline{
		Dir:     dir,
		Context: map[string]any{"local": "L"},
		Pipeline: []api.StepConfig{
			{
				Name:     "render",
				Type:     api.StepTypeTemplate,
				Template: &api.TemplateConfig{Files: api.FileFilter{Include: []string{"*"}}},
			},
		},
	}

	globalCtx := map[string]any{"global": "G", "local": "overridden"}

	if err := RunPipeline(pipeline, globalCtx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "test.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "G-L" {
		t.Errorf("expected 'G-L', got %q", string(content))
	}
}

func TestRunPipeline_StepOutputRegistry(t *testing.T) {
	dir := t.TempDir()

	// Pipeline with a split step referencing a non-existent output
	pipeline := &api.Pipeline{
		Dir: dir,
		Pipeline: []api.StepConfig{
			{
				Name: "split",
				Type: api.StepTypeSplit,
				Split: &api.SplitConfig{
					Input: "missing",
					By:    api.SplitByKind,
				},
			},
		},
	}

	err := RunPipeline(pipeline, nil)
	if err == nil {
		t.Fatal("expected error for missing input")
	}
	if !strings.Contains(err.Error(), "not found in step outputs") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCopyTree(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create source files
	if err := os.MkdirAll(filepath.Join(src, "sub"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "sub", "b.txt"), []byte("world"), 0600); err != nil {
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

	if err := os.MkdirAll(filepath.Join(root, "sub"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".many.yaml"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", ".many.yaml"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "keep.yaml"), []byte("test"), 0600); err != nil {
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

	// Create a source tree with a .many.yaml and a template file
	if err := os.WriteFile(filepath.Join(src, ".many.yaml"), []byte(`
context:
  name: world
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["*.txt"]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("Hello {{ .name }}!"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := RunAll(src, dst, nil, -1, ""); err != nil {
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

	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Should succeed with no pipelines (just copies tree)
	if err := RunAll(src, dst, nil, -1, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dst, "file.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "content" {
		t.Errorf("expected 'content', got %q", string(content))
	}
}

func TestRemoveBuildArtifacts(t *testing.T) {
	dir := t.TempDir()

	// Create files and directories that represent build artifacts
	writeFile := func(name, content string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	writeFile("kustomization.yaml", "resources: [secret.yaml]")
	writeFile("values.yaml", "key: val")
	writeFile("secret.yaml", "apiVersion: v1")

	if err := os.MkdirAll(filepath.Join(dir, "charts", "test"), 0750); err != nil {
		t.Fatal(err)
	}
	writeFile("charts/test/Chart.yaml", "name: test")

	if err := os.MkdirAll(filepath.Join(dir, "manifests"), 0750); err != nil {
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
    template:
      files:
        include: ["*.yaml"]
        exclude: [".many.yaml"]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "data.yaml"), []byte("value: {{ .v }}"), 0600); err != nil {
		t.Fatal(err)
	}

	pipelineFile := filepath.Join(src, ".many.yaml")
	if err := RunSingle(pipelineFile, src, dst, nil, ""); err != nil {
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
	if err := os.MkdirAll(filepath.Join(inputDir, "sub"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(inner, []byte("test"), 0600); err != nil {
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
	if err := os.WriteFile(outside, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	_, ok = relativeToInput(outside, inputDir)
	if ok {
		t.Error("expected ok=false for file outside input dir")
	}

	// File at input root
	root := filepath.Join(inputDir, "root.yaml")
	if err := os.WriteFile(root, []byte("test"), 0600); err != nil {
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

func TestRemoveContextFile(t *testing.T) {
	src := t.TempDir()
	dst := t.TempDir()

	// Create context file in both src and dst
	if err := os.WriteFile(filepath.Join(src, "context.yaml"), []byte("key: val"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "context.yaml"), []byte("key: val"), 0600); err != nil {
		t.Fatal(err)
	}

	// Should remove from dst
	removeContextFile(filepath.Join(src, "context.yaml"), src, dst)

	if _, err := os.Stat(filepath.Join(dst, "context.yaml")); !os.IsNotExist(err) {
		t.Error("context.yaml should be removed from output")
	}

	// Empty contextFile is a no-op
	removeContextFile("", src, dst)

	// Context file outside input dir is a no-op
	outside := filepath.Join(t.TempDir(), "external.yaml")
	if err := os.WriteFile(outside, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "external.yaml"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}
	removeContextFile(outside, src, dst)
	if _, err := os.Stat(filepath.Join(dst, "external.yaml")); err != nil {
		t.Error("external.yaml should NOT be removed (outside input dir)")
	}

	// Nonexistent file in output is a no-op (no panic)
	removeContextFile(filepath.Join(src, "missing.yaml"), src, dst)
}

func TestRunAll_WithContextFile(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	if err := os.WriteFile(filepath.Join(src, ".many.yaml"), []byte(`
context:
  name: world
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["*.txt"]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "hello.txt"), []byte("Hello {{ .name }}!"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "context.yaml"), []byte("name: world"), 0600); err != nil {
		t.Fatal(err)
	}

	ctxFile := filepath.Join(src, "context.yaml")
	if err := RunAll(src, dst, nil, -1, ctxFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// context.yaml should be removed from output
	if _, err := os.Stat(filepath.Join(dst, "context.yaml")); !os.IsNotExist(err) {
		t.Error("context.yaml should be removed from output")
	}

	// Template should still render
	content, err := os.ReadFile(filepath.Join(dst, "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "Hello world!" {
		t.Errorf("expected 'Hello world!', got %q", string(content))
	}
}

func TestRunAll_MaxDepth(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	// Create nested pipelines at depth 0 and depth 1
	if err := os.WriteFile(filepath.Join(src, ".many.yaml"), []byte(`
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["*.txt"]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0600); err != nil {
		t.Fatal(err)
	}

	sub := filepath.Join(src, "sub")
	if err := os.MkdirAll(sub, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, ".many.yaml"), []byte(`
context:
  val: deep
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["*.txt"]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "deep.txt"), []byte("{{ .val }}"), 0600); err != nil {
		t.Fatal(err)
	}

	// maxDepth=0 should only process root
	if err := RunAll(src, dst, nil, 0, ""); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// deep.txt should NOT be rendered (sub pipeline skipped)
	content, err := os.ReadFile(filepath.Join(dst, "sub", "deep.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "{{ .val }}" {
		t.Errorf("sub/deep.txt should not be rendered at maxDepth=0, got %q", string(content))
	}
}

func TestRunSingle_WithContextFile(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	if err := os.WriteFile(filepath.Join(src, ".many.yaml"), []byte(`
context:
  v: ok
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["*.yaml"]
        exclude: [".many.yaml", "context.yaml"]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "data.yaml"), []byte("value: {{ .v }}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "context.yaml"), []byte("v: ok"), 0600); err != nil {
		t.Fatal(err)
	}

	ctxFile := filepath.Join(src, "context.yaml")
	pipelineFile := filepath.Join(src, ".many.yaml")
	if err := RunSingle(pipelineFile, src, dst, nil, ctxFile); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "context.yaml")); !os.IsNotExist(err) {
		t.Error("context.yaml should be removed from output")
	}

	content, err := os.ReadFile(filepath.Join(dst, "data.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "value: ok" {
		t.Errorf("expected 'value: ok', got %q", string(content))
	}
}

func TestRunSingle_NonexistentPipelineFile(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	if err := os.WriteFile(filepath.Join(src, "file.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	err := RunSingle(filepath.Join(src, ".many.yaml"), src, dst, nil, "")
	if err == nil {
		t.Fatal("expected error for nonexistent pipeline file")
	}
}

func TestRunAll_FailedPipeline(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "output")

	// Invalid template that will fail
	if err := os.WriteFile(filepath.Join(src, ".many.yaml"), []byte(`
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["*.txt"]
`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "bad.txt"), []byte("{{ .missing | fail }}"), 0600); err != nil {
		t.Fatal(err)
	}

	err := RunAll(src, dst, nil, -1, "")
	if err == nil {
		t.Fatal("expected error for failed pipeline")
	}
	if !strings.Contains(err.Error(), "pipeline(s) failed") {
		t.Errorf("unexpected error message: %v", err)
	}
}

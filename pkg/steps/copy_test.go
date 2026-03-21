package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func TestCopyStep_BasicInclude(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()

	// Create source files.
	if err := os.MkdirAll(filepath.Join(srcDir, "manifests"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "manifests", "deploy.yaml"), []byte("kind: Deployment"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "README.md"), []byte("readme"), 0o600); err != nil {
		t.Fatal(err)
	}

	step := NewCopyStep("copy-manifests", &api.CopyConfig{
		Files: api.FileFilter{Include: []string{"manifests/**/*.yaml"}},
	})

	ctx := StepContext{
		WorkDir:   workDir,
		SourceDir: srcDir,
	}

	result, err := step.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	// The yaml file should be copied.
	content, err := os.ReadFile(filepath.Join(workDir, "manifests", "deploy.yaml"))
	if err != nil {
		t.Fatalf("expected file to be copied: %v", err)
	}
	if string(content) != "kind: Deployment" {
		t.Fatalf("unexpected content: %q", string(content))
	}

	// README.md should NOT be copied.
	if _, err := os.Stat(filepath.Join(workDir, "README.md")); !os.IsNotExist(err) {
		t.Fatal("README.md should not have been copied")
	}
}

func TestCopyStep_WithExclude(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()

	if err := os.MkdirAll(filepath.Join(srcDir, "manifests", "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "manifests", "deploy.yaml"), []byte("deploy"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "manifests", "tmp", "scratch.yaml"), []byte("scratch"), 0o600); err != nil {
		t.Fatal(err)
	}

	step := NewCopyStep("copy-filtered", &api.CopyConfig{
		Files: api.FileFilter{
			Include: []string{"manifests/**/*.yaml"},
			Exclude: []string{"manifests/tmp/**"},
		},
	})

	ctx := StepContext{
		WorkDir:   workDir,
		SourceDir: srcDir,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// deploy.yaml should exist.
	if _, err := os.Stat(filepath.Join(workDir, "manifests", "deploy.yaml")); err != nil {
		t.Fatalf("deploy.yaml should be copied: %v", err)
	}

	// scratch.yaml should NOT exist.
	if _, err := os.Stat(filepath.Join(workDir, "manifests", "tmp", "scratch.yaml")); !os.IsNotExist(err) {
		t.Fatal("scratch.yaml should have been excluded")
	}
}

func TestCopyStep_WithDest(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(srcDir, "config.yaml"), []byte("key: value"), 0o600); err != nil {
		t.Fatal(err)
	}

	step := NewCopyStep("copy-to-subdir", &api.CopyConfig{
		Files: api.FileFilter{Include: []string{"*.yaml"}},
		Dest:  "output/configs",
	})

	ctx := StepContext{
		WorkDir:   workDir,
		SourceDir: srcDir,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(workDir, "output", "configs", "config.yaml"))
	if err != nil {
		t.Fatalf("expected file at dest subdir: %v", err)
	}
	if string(content) != "key: value" {
		t.Fatalf("unexpected content: %q", string(content))
	}
}

func TestCopyStep_PreservesDirectoryStructure(t *testing.T) {
	srcDir := t.TempDir()
	workDir := t.TempDir()

	dirs := []string{"a", "a/b", "a/b/c"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(srcDir, d), 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a", "one.yaml"), []byte("1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a", "b", "two.yaml"), []byte("2"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "a", "b", "c", "three.yaml"), []byte("3"), 0o600); err != nil {
		t.Fatal(err)
	}

	step := NewCopyStep("copy-tree", &api.CopyConfig{
		Files: api.FileFilter{Include: []string{"**/*.yaml"}},
	})

	ctx := StepContext{
		WorkDir:   workDir,
		SourceDir: srcDir,
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := map[string]string{
		"a/one.yaml":       "1",
		"a/b/two.yaml":     "2",
		"a/b/c/three.yaml": "3",
	}
	for path, want := range expected {
		content, err := os.ReadFile(filepath.Join(workDir, path))
		if err != nil {
			t.Fatalf("expected %s to exist: %v", path, err)
		}
		if string(content) != want {
			t.Fatalf("%s: got %q, want %q", path, string(content), want)
		}
	}
}

func TestCopyStep_MissingSourceDir(t *testing.T) {
	workDir := t.TempDir()

	step := NewCopyStep("copy-no-source", &api.CopyConfig{
		Files: api.FileFilter{Include: []string{"**/*"}},
	})

	ctx := StepContext{
		WorkDir:   workDir,
		SourceDir: "",
	}

	_, err := step.Run(ctx)
	if err == nil {
		t.Fatal("expected error for missing source dir")
	}
	if !strings.Contains(err.Error(), "source directory is not set") {
		t.Fatalf("unexpected error: %v", err)
	}
}

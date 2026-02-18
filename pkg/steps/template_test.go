package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func TestTemplateStep_Run(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte("host: {{ .domain }}"), 0600); err != nil {
		t.Fatal(err)
	}

	step := NewTemplateStep("render", &api.TemplateConfig{
		Files: api.FileFilter{Include: []string{"**/*.yaml"}},
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{"domain": "example.com"},
	}

	result, err := step.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}

	content, err := os.ReadFile(filepath.Join(dir, "test.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "host: example.com" {
		t.Fatalf("expected 'host: example.com', got %q", string(content))
	}
}

func TestTemplateStep_DefaultInclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("v: {{ .x }}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "sub", "b.yaml"), []byte("w: {{ .x }}"), 0600); err != nil {
		t.Fatal(err)
	}

	step := NewTemplateStep("render", &api.TemplateConfig{
		Files: api.FileFilter{},
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{"x": "42"},
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, f := range []string{"a.yaml", "sub/b.yaml"} {
		content, _ := os.ReadFile(filepath.Join(dir, f))
		if !strings.Contains(string(content), "42") {
			t.Fatalf("file %s not rendered: %q", f, string(content))
		}
	}
}

func TestTemplateStep_Exclude(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "include.yaml"), []byte("v: {{ .x }}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "exclude.yaml"), []byte("v: {{ .x }}"), 0600); err != nil {
		t.Fatal(err)
	}

	step := NewTemplateStep("render", &api.TemplateConfig{
		Files: api.FileFilter{
			Include: []string{"**/*.yaml"},
			Exclude: []string{"exclude.yaml"},
		},
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{"x": "done"},
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	included, _ := os.ReadFile(filepath.Join(dir, "include.yaml"))
	if string(included) != "v: done" {
		t.Fatalf("include.yaml not rendered: %q", string(included))
	}

	excluded, _ := os.ReadFile(filepath.Join(dir, "exclude.yaml"))
	if string(excluded) != "v: {{ .x }}" {
		t.Fatalf("exclude.yaml should not be rendered: %q", string(excluded))
	}
}

func TestTemplateStep_SprigFunctions(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.yaml"), []byte(`v: {{ "hello" | upper }}`), 0600); err != nil {
		t.Fatal(err)
	}

	step := NewTemplateStep("render", &api.TemplateConfig{
		Files: api.FileFilter{Include: []string{"*.yaml"}},
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{},
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "test.yaml"))
	if string(content) != "v: HELLO" {
		t.Fatalf("expected 'v: HELLO', got %q", string(content))
	}
}

func TestTemplateStep_InvalidTemplateSyntax(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte("v: {{ .unclosed"), 0600); err != nil {
		t.Fatal(err)
	}

	step := NewTemplateStep("render", &api.TemplateConfig{
		Files: api.FileFilter{Include: []string{"*.yaml"}},
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{},
	}

	_, err := step.Run(ctx)
	if err == nil {
		t.Fatal("expected error for invalid template syntax")
	}
	if !strings.Contains(err.Error(), "parsing template") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTemplateStep_TemplateExecutionError(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "bad.yaml"), []byte(`{{ fail "boom" }}`), 0600); err != nil {
		t.Fatal(err)
	}

	step := NewTemplateStep("render", &api.TemplateConfig{
		Files: api.FileFilter{Include: []string{"*.yaml"}},
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{},
	}

	_, err := step.Run(ctx)
	if err == nil {
		t.Fatal("expected error for template execution failure")
	}
	if !strings.Contains(err.Error(), "executing template") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestTemplateStep_NoMatchingFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "test.txt"), []byte("plain text"), 0600); err != nil {
		t.Fatal(err)
	}

	step := NewTemplateStep("render", &api.TemplateConfig{
		Files: api.FileFilter{Include: []string{"*.yaml"}},
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{},
	}

	// No error, just zero files processed
	_, err := step.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTemplateStep_ExcludeAll(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.yaml"), []byte("v: {{ .x }}"), 0600); err != nil {
		t.Fatal(err)
	}

	step := NewTemplateStep("render", &api.TemplateConfig{
		Files: api.FileFilter{
			Include: []string{"*.yaml"},
			Exclude: []string{"*.yaml"},
		},
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{"x": "val"},
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be unchanged (not rendered)
	content, _ := os.ReadFile(filepath.Join(dir, "a.yaml"))
	if string(content) != "v: {{ .x }}" {
		t.Errorf("file should not be rendered when excluded, got %q", string(content))
	}
}

package steps

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/systemstart/many-templates/pkg/api"
)

func TestGenerateStep_Run(t *testing.T) {
	dir := t.TempDir()

	step := NewGenerateStep("gen", &api.GenerateConfig{
		Output:   "output.yaml",
		Template: "host: {{ .domain }}",
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

	content, err := os.ReadFile(filepath.Join(dir, "output.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "host: example.com" {
		t.Fatalf("expected 'host: example.com', got %q", string(content))
	}
}

func TestGenerateStep_NestedOutputPath(t *testing.T) {
	dir := t.TempDir()

	step := NewGenerateStep("gen", &api.GenerateConfig{
		Output:   "sub/dir/output.yaml",
		Template: "value: ok",
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{},
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(dir, "sub", "dir", "output.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != "value: ok" {
		t.Fatalf("expected 'value: ok', got %q", string(content))
	}
}

func TestGenerateStep_SprigFunctions(t *testing.T) {
	dir := t.TempDir()

	step := NewGenerateStep("gen", &api.GenerateConfig{
		Output:   "output.yaml",
		Template: `v: {{ "hello" | upper }}`,
	})

	ctx := StepContext{
		WorkDir:      dir,
		TemplateData: map[string]any{},
	}

	if _, err := step.Run(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	content, _ := os.ReadFile(filepath.Join(dir, "output.yaml"))
	if string(content) != "v: HELLO" {
		t.Fatalf("expected 'v: HELLO', got %q", string(content))
	}
}

func TestGenerateStep_TemplateParseError(t *testing.T) {
	dir := t.TempDir()

	step := NewGenerateStep("gen", &api.GenerateConfig{
		Output:   "output.yaml",
		Template: "{{ .unclosed",
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

func TestGenerateStep_TemplateExecutionError(t *testing.T) {
	dir := t.TempDir()

	step := NewGenerateStep("gen", &api.GenerateConfig{
		Output:   "output.yaml",
		Template: `{{ fail "boom" }}`,
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

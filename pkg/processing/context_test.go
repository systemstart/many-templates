package processing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadContextFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "context.yaml")
	if err := os.WriteFile(f, []byte("domain: example.com\nport: 8080\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, err := LoadContextFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx["domain"] != "example.com" {
		t.Errorf("expected domain=example.com, got %v", ctx["domain"])
	}
	if ctx["port"] != 8080 {
		t.Errorf("expected port=8080, got %v", ctx["port"])
	}
}

func TestLoadContextFile_Empty(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "context.yaml")
	if err := os.WriteFile(f, []byte(""), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, err := LoadContextFile(f)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ctx == nil {
		t.Fatal("expected non-nil map")
	}
	if len(ctx) != 0 {
		t.Errorf("expected empty map, got %v", ctx)
	}
}

func TestLoadContextFile_NotFound(t *testing.T) {
	_, err := LoadContextFile("/nonexistent/context.yaml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadContextFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "context.yaml")
	if err := os.WriteFile(f, []byte("{{invalid"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadContextFile(f)
	if err == nil {
		t.Fatal("expected error for invalid YAML")
	}
}

func TestMergeContext(t *testing.T) {
	tests := []struct {
		name   string
		global map[string]any
		local  map[string]any
		check  func(t *testing.T, merged map[string]any)
	}{
		{
			name:   "local overrides global",
			global: map[string]any{"domain": "global.com", "port": 8080},
			local:  map[string]any{"domain": "local.com", "extra": "value"},
			check: func(t *testing.T, m map[string]any) {
				t.Helper()
				if m["domain"] != "local.com" {
					t.Errorf("expected local override, got %v", m["domain"])
				}
				if m["port"] != 8080 {
					t.Errorf("expected global port preserved, got %v", m["port"])
				}
				if m["extra"] != "value" {
					t.Errorf("expected local extra, got %v", m["extra"])
				}
			},
		},
		{
			name:  "nil global",
			local: map[string]any{"key": "val"},
			check: func(t *testing.T, m map[string]any) {
				t.Helper()
				if m["key"] != "val" {
					t.Errorf("expected key=val, got %v", m["key"])
				}
			},
		},
		{
			name:   "nil local",
			global: map[string]any{"key": "val"},
			check: func(t *testing.T, m map[string]any) {
				t.Helper()
				if m["key"] != "val" {
					t.Errorf("expected key=val, got %v", m["key"])
				}
			},
		},
		{
			name: "both nil",
			check: func(t *testing.T, m map[string]any) {
				t.Helper()
				if len(m) != 0 {
					t.Errorf("expected empty map, got %v", m)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.check(t, MergeContext(tt.global, tt.local))
		})
	}
}

func TestInterpolateContext_Basic(t *testing.T) {
	ctx := map[string]any{
		"domain":  "example.com",
		"app_url": "https://app.{{ .domain }}",
	}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx["app_url"] != "https://app.example.com" {
		t.Errorf("expected https://app.example.com, got %v", ctx["app_url"])
	}
}

func TestInterpolateContext_NestedMap(t *testing.T) {
	ctx := map[string]any{
		"domain": "example.com",
		"services": map[string]any{
			"api_url": "https://api.{{ .domain }}",
		},
	}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	svc := ctx["services"].(map[string]any)
	if svc["api_url"] != "https://api.example.com" {
		t.Errorf("expected https://api.example.com, got %v", svc["api_url"])
	}
}

func TestInterpolateContext_Slice(t *testing.T) {
	ctx := map[string]any{
		"domain": "example.com",
		"urls":   []any{"https://app.{{ .domain }}", "https://api.{{ .domain }}"},
	}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	urls := ctx["urls"].([]any)
	if urls[0] != "https://app.example.com" {
		t.Errorf("expected https://app.example.com, got %v", urls[0])
	}
	if urls[1] != "https://api.example.com" {
		t.Errorf("expected https://api.example.com, got %v", urls[1])
	}
}

func TestInterpolateContext_NonTemplateStrings(t *testing.T) {
	ctx := map[string]any{"name": "hello world"}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx["name"] != "hello world" {
		t.Errorf("expected hello world, got %v", ctx["name"])
	}
}

func TestInterpolateContext_NonStringValues(t *testing.T) {
	ctx := map[string]any{"port": 8080, "enabled": true}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx["port"] != 8080 {
		t.Errorf("expected 8080, got %v", ctx["port"])
	}
	if ctx["enabled"] != true {
		t.Errorf("expected true, got %v", ctx["enabled"])
	}
}

func TestInterpolateContext_NoTemplatesNoop(t *testing.T) {
	ctx := map[string]any{"a": "plain", "b": "also plain"}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ctx["a"] != "plain" || ctx["b"] != "also plain" {
		t.Errorf("unexpected change: %v", ctx)
	}
}

func TestInterpolateContext_Errors(t *testing.T) {
	tests := []struct {
		name string
		ctx  map[string]any
	}{
		{
			name: "template parse error",
			ctx:  map[string]any{"bad": "{{ .foo | }"},
		},
		{
			name: "template execution error with required",
			ctx:  map[string]any{"bad": `{{ required "need foo" .foo }}`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := InterpolateContext(tt.ctx); err == nil {
				t.Fatal("expected error, got nil")
			}
		})
	}
}

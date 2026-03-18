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
	t.Run("local overrides global", func(t *testing.T) {
		m := MergeContext(
			map[string]any{"domain": "global.com", "port": 8080},
			map[string]any{"domain": "local.com", "extra": "value"},
		)
		assertMerged(t, m, "domain", "local.com")
		assertMerged(t, m, "port", 8080)
		assertMerged(t, m, "extra", "value")
	})

	t.Run("nil global", func(t *testing.T) {
		m := MergeContext(nil, map[string]any{"key": "val"})
		assertMerged(t, m, "key", "val")
	})

	t.Run("nil local", func(t *testing.T) {
		m := MergeContext(map[string]any{"key": "val"}, nil)
		assertMerged(t, m, "key", "val")
	})

	t.Run("both nil", func(t *testing.T) {
		m := MergeContext(nil, nil)
		if len(m) != 0 {
			t.Errorf("expected empty map, got %v", m)
		}
	})

	t.Run("deep merge nested map partially overridden", func(t *testing.T) {
		m := MergeContext(
			map[string]any{"db": map[string]any{"host": "global-db", "port": 5432}},
			map[string]any{"db": map[string]any{"host": "local-db"}},
		)
		assertNestedMerged(t, m, "db", "host", "local-db")
		assertNestedMerged(t, m, "db", "port", 5432)
	})

	t.Run("deep merge deeply nested 3+ levels", func(t *testing.T) {
		m := MergeContext(
			map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"x": 1, "y": 2}}}},
			map[string]any{"a": map[string]any{"b": map[string]any{"c": map[string]any{"y": 99, "z": 3}}}},
		)
		c := m["a"].(map[string]any)["b"].(map[string]any)["c"].(map[string]any)
		assertMerged(t, c, "x", 1)
		assertMerged(t, c, "y", 99)
		assertMerged(t, c, "z", 3)
	})

	t.Run("deep merge local replaces map with scalar", func(t *testing.T) {
		m := MergeContext(
			map[string]any{"db": map[string]any{"host": "global-db"}},
			map[string]any{"db": "just-a-string"},
		)
		assertMerged(t, m, "db", "just-a-string")
	})

	t.Run("deep merge local adds new nested key", func(t *testing.T) {
		m := MergeContext(
			map[string]any{"db": map[string]any{"host": "global-db"}},
			map[string]any{"db": map[string]any{"port": 3306}, "cache": map[string]any{"ttl": 60}},
		)
		assertNestedMerged(t, m, "db", "host", "global-db")
		assertNestedMerged(t, m, "db", "port", 3306)
		assertNestedMerged(t, m, "cache", "ttl", 60)
	})

	t.Run("deep merge slices replaced not merged", func(t *testing.T) {
		m := MergeContext(
			map[string]any{"tags": []any{"a", "b"}},
			map[string]any{"tags": []any{"c"}},
		)
		tags := m["tags"].([]any)
		if len(tags) != 1 || tags[0] != "c" {
			t.Errorf("expected [c], got %v", tags)
		}
	})
}

func assertMerged(t *testing.T, m map[string]any, key string, want any) {
	t.Helper()
	if m[key] != want {
		t.Errorf("key %q = %v, want %v", key, m[key], want)
	}
}

func assertNestedMerged(t *testing.T, m map[string]any, outer, inner string, want any) {
	t.Helper()
	sub := m[outer].(map[string]any)
	if sub[inner] != want {
		t.Errorf("%s.%s = %v, want %v", outer, inner, sub[inner], want)
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

func TestInterpolateContext_SliceWithNestedMap(t *testing.T) {
	ctx := map[string]any{
		"domain": "example.com",
		"items": []any{
			map[string]any{"url": "https://{{ .domain }}/api"},
		},
	}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	items := ctx["items"].([]any)
	item := items[0].(map[string]any)
	if item["url"] != "https://example.com/api" {
		t.Errorf("expected https://example.com/api, got %v", item["url"])
	}
}

func TestInterpolateContext_SliceWithNestedSlice(t *testing.T) {
	ctx := map[string]any{
		"domain": "example.com",
		"nested": []any{
			[]any{"https://{{ .domain }}"},
		},
	}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	nested := ctx["nested"].([]any)
	inner := nested[0].([]any)
	if inner[0] != "https://example.com" {
		t.Errorf("expected https://example.com, got %v", inner[0])
	}
}

func TestInterpolateContext_SliceWithNonStringValues(t *testing.T) {
	ctx := map[string]any{
		"mixed": []any{"plain", 42, true, nil},
	}
	if err := InterpolateContext(ctx); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	mixed := ctx["mixed"].([]any)
	if mixed[0] != "plain" {
		t.Errorf("expected 'plain', got %v", mixed[0])
	}
	if mixed[1] != 42 {
		t.Errorf("expected 42, got %v", mixed[1])
	}
}

func TestInterpolateContext_SliceError(t *testing.T) {
	ctx := map[string]any{
		"items": []any{"{{ .x | fail }}"},
	}
	if err := InterpolateContext(ctx); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestInterpolateContext_NestedMapError(t *testing.T) {
	ctx := map[string]any{
		"sub": map[string]any{
			"bad": "{{ .x | fail }}",
		},
	}
	if err := InterpolateContext(ctx); err == nil {
		t.Fatal("expected error, got nil")
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

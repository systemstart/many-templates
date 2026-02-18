package processing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadContextFile(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "context.yaml")
	if err := os.WriteFile(f, []byte("domain: example.com\nport: 8080\n"), 0600); err != nil {
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
	if err := os.WriteFile(f, []byte(""), 0600); err != nil {
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
	if err := os.WriteFile(f, []byte("{{invalid"), 0600); err != nil {
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

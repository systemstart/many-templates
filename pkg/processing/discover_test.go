package processing

import (
	"os"
	"path/filepath"
	"testing"
)

const validPipeline = `
context:
  domain: example.com
pipeline:
  - name: render
    type: template
    template:
      files:
        include: ["**/*.yaml"]
`

func setupDiscoverTree(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	// root/.many.yaml
	if err := os.WriteFile(filepath.Join(root, ".many.yaml"), []byte(validPipeline), 0600); err != nil {
		t.Fatal(err)
	}

	// root/child/.many.yaml
	child := filepath.Join(root, "child")
	if err := os.MkdirAll(child, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(child, ".many.yaml"), []byte(validPipeline), 0600); err != nil {
		t.Fatal(err)
	}

	// root/child/grandchild/.many.yaml
	grandchild := filepath.Join(child, "grandchild")
	if err := os.MkdirAll(grandchild, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(grandchild, ".many.yaml"), []byte(validPipeline), 0600); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestDiscoverPipelines_Unlimited(t *testing.T) {
	root := setupDiscoverTree(t)

	pipelines, err := DiscoverPipelines(root, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pipelines) != 3 {
		t.Fatalf("expected 3 pipelines, got %d", len(pipelines))
	}

	// Should be sorted by depth (root first)
	if pipelines[0].Dir != root {
		t.Errorf("expected first pipeline at root %q, got %q", root, pipelines[0].Dir)
	}
}

func TestDiscoverPipelines_MaxDepth0(t *testing.T) {
	root := setupDiscoverTree(t)

	pipelines, err := DiscoverPipelines(root, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pipelines) != 1 {
		t.Fatalf("expected 1 pipeline (root only), got %d", len(pipelines))
	}
}

func TestDiscoverPipelines_MaxDepth1(t *testing.T) {
	root := setupDiscoverTree(t)

	pipelines, err := DiscoverPipelines(root, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pipelines) != 2 {
		t.Fatalf("expected 2 pipelines (root + child), got %d", len(pipelines))
	}
}

func TestDiscoverPipelines_NoPipelines(t *testing.T) {
	root := t.TempDir()

	pipelines, err := DiscoverPipelines(root, -1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pipelines) != 0 {
		t.Fatalf("expected 0 pipelines, got %d", len(pipelines))
	}
}

func TestDiscoverPipelines_InvalidPipeline(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".many.yaml"), []byte("{{invalid"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := DiscoverPipelines(root, -1)
	if err == nil {
		t.Fatal("expected error for invalid pipeline")
	}
}

func TestPathDepth(t *testing.T) {
	tests := []struct {
		path string
		want int
	}{
		{".", 0},
		{"a", 1},
		{"a/b", 2},
		{"a/b/c", 3},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := pathDepth(tt.path)
			if got != tt.want {
				t.Errorf("pathDepth(%q) = %d, want %d", tt.path, got, tt.want)
			}
		})
	}
}

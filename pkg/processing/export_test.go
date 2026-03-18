package processing

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopyTreeExported(t *testing.T) {
	// Create a source tree with a subdirectory and files.
	src := t.TempDir()
	sub := filepath.Join(src, "sub")
	if err := os.MkdirAll(sub, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "root.txt"), []byte("root"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "nested.txt"), []byte("nested"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Copy to a new destination.
	dst := filepath.Join(t.TempDir(), "out")
	if err := CopyTree(src, dst); err != nil {
		t.Fatal(err)
	}

	// Verify files exist with correct content.
	for _, tc := range []struct {
		rel     string
		content string
	}{
		{"root.txt", "root"},
		{filepath.Join("sub", "nested.txt"), "nested"},
	} {
		got, err := os.ReadFile(filepath.Join(dst, tc.rel))
		if err != nil {
			t.Errorf("reading %s: %v", tc.rel, err)
			continue
		}
		if string(got) != tc.content {
			t.Errorf("%s: got %q, want %q", tc.rel, got, tc.content)
		}
	}
}

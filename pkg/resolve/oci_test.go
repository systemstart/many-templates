package resolve

import (
	"os/exec"
	"strings"
	"testing"
)

func skipWithoutCrane(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("crane"); err != nil {
		t.Skip("crane not in PATH")
	}
}

func TestResolveOCI_MissingCrane(t *testing.T) {
	// Only run this test if crane is NOT available.
	if _, err := exec.LookPath("crane"); err == nil {
		t.Skip("crane is available, skipping missing-crane test")
	}

	_, _, err := resolveOCI("registry.example.com/repo:tag")
	if err == nil {
		t.Fatal("expected error when crane is missing, got nil")
	}
	if !strings.Contains(err.Error(), "crane binary not found") {
		t.Errorf("expected 'crane binary not found' error, got %q", err.Error())
	}
}

func TestResolveOCI_Integration(t *testing.T) {
	skipWithoutCrane(t)

	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Pull a small public image (busybox is ~2MB).
	path, cleanup, err := resolveOCI("busybox:latest")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if path == "" {
		t.Fatal("expected non-empty path")
	}
}

package resolve

import (
	"os/exec"
	"strings"
	"testing"
)

func skipWithoutOCM(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("ocm"); err != nil {
		t.Skip("ocm not in PATH")
	}
}

func TestResolveOCM_MissingOCM(t *testing.T) {
	// Only run this test if ocm is NOT available.
	if _, err := exec.LookPath("ocm"); err == nil {
		t.Skip("ocm is available, skipping missing-ocm test")
	}

	_, _, err := resolveOCM("ghcr.io/myorg/ocm//github.com/myorg/comp:v1")
	if err == nil {
		t.Fatal("expected error when ocm is missing, got nil")
	}
	if !strings.Contains(err.Error(), "ocm binary not found") {
		t.Errorf("expected 'ocm binary not found' error, got %q", err.Error())
	}
}

func TestResolveOCM_Integration(t *testing.T) {
	skipWithoutOCM(t)
	t.Skip("requires a published OCM component")
}

func TestRepoPrefix(t *testing.T) {
	tests := []struct {
		ref  string
		want string
	}{
		{"ghcr.io/myorg/ocm//github.com/myorg/comp:v1", "ghcr.io/myorg/ocm//"},
		{"localhost:5000//comp:v1", "localhost:5000//"},
		{"no-double-slash:v1", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.ref, func(t *testing.T) {
			got := repoPrefix(tt.ref)
			if got != tt.want {
				t.Errorf("repoPrefix(%q) = %q, want %q", tt.ref, got, tt.want)
			}
		})
	}
}

func TestResolveOCMRecursive_MissingOCM(t *testing.T) {
	if _, err := exec.LookPath("ocm"); err == nil {
		t.Skip("ocm is available, skipping missing-ocm test")
	}

	_, _, err := ResolveOCMRecursive("ghcr.io/myorg/ocm//github.com/myorg/comp:v1")
	if err == nil {
		t.Fatal("expected error when ocm is missing, got nil")
	}
	if !strings.Contains(err.Error(), "ocm binary not found") {
		t.Errorf("expected 'ocm binary not found' error, got %q", err.Error())
	}
}

func TestResolveOCMRecursive_Integration(t *testing.T) {
	skipWithoutOCM(t)
	t.Skip("requires a published OCM component")
}

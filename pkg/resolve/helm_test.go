package resolve

import (
	"os/exec"
	"testing"
)

func TestResolveHelm_MissingBinary(t *testing.T) {
	// Save PATH and set it to empty to ensure helm is not found.
	t.Setenv("PATH", "")

	_, _, err := ResolveHelm("mychart", "https://charts.example.com", "1.0.0")
	if err == nil {
		t.Fatal("expected error when helm binary is missing")
	}
	if got := err.Error(); !contains(got, "helm binary not found") {
		t.Fatalf("unexpected error: %s", got)
	}
}

func TestResolveHelm_InvalidChart(t *testing.T) {
	if _, err := exec.LookPath("helm"); err != nil {
		t.Skip("helm not installed, skipping")
	}

	_, _, err := ResolveHelm("nonexistent-chart", "https://127.0.0.1:1", "0.0.0")
	if err == nil {
		t.Fatal("expected error for invalid chart repo")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

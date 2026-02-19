package steps

import (
	"os"
	"path/filepath"
	"testing"
)

// writeTestFile writes content to a file in dir, failing the test on error.
func writeTestFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

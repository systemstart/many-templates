package resolve

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// resolveOCI pulls an OCI image using crane export and extracts the flattened
// filesystem into a temp directory.
func resolveOCI(ref string) (string, func(), error) {
	cranePath, err := exec.LookPath("crane")
	if err != nil {
		return "", nil, fmt.Errorf("crane binary not found in PATH — install with: go install github.com/google/go-containerregistry/cmd/crane@latest")
	}

	dir, err := os.MkdirTemp("", "many-oci-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	cmd := exec.Command(cranePath, "export", ref, "-")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("creating stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("starting crane export: %w", err)
	}

	// crane export produces an uncompressed tar.
	if err := extractTar(stdout, dir); err != nil {
		_ = cmd.Wait()
		cleanup()
		return "", nil, fmt.Errorf("extracting crane output: %w\nstderr: %s", err, stderr.String())
	}

	if err := cmd.Wait(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("crane export failed: %w\nstderr: %s", err, stderr.String())
	}

	return unwrapSingleRoot(dir), cleanup, nil
}

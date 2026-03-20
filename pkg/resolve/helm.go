package resolve

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

// ResolveHelm pulls a Helm chart from a repository and returns a local path
// to the extracted chart directory.
func ResolveHelm(chart, repo, version string) (string, func(), error) {
	helmPath, err := exec.LookPath("helm")
	if err != nil {
		return "", nil, fmt.Errorf("helm binary not found in PATH — install from: https://helm.sh/docs/intro/install/")
	}

	dir, err := os.MkdirTemp("", "many-helm-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	args := []string{"pull", chart, "--repo", repo, "--untar", "-d", dir}
	if version != "" {
		args = append(args, "--version", version)
	}

	cmd := exec.Command(helmPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("helm pull failed: %w\nstderr: %s", err, stderr.String())
	}

	return unwrapSingleRoot(dir), cleanup, nil
}

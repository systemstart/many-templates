package resolve

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"gopkg.in/yaml.v3"
)

// resolveOCM downloads OCM component resources using the ocm CLI.
func resolveOCM(ref string) (string, func(), error) {
	ocmPath, err := exec.LookPath("ocm")
	if err != nil {
		return "", nil, fmt.Errorf("ocm binary not found in PATH — install from: https://ocm.software/docs/getting-started/installing-the-ocm-cli/")
	}

	dir, err := os.MkdirTemp("", "many-ocm-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	cmd := exec.Command(ocmPath, "download", "resources", ref, "-O", dir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("ocm download resources failed: %w\nstderr: %s", err, stderr.String())
	}

	return unwrapSingleRoot(dir), cleanup, nil
}

// ResolveOCMRecursive downloads resources for the given OCM component reference
// and all its component references, merging everything into a single temp directory.
func ResolveOCMRecursive(ref string) (string, func(), error) {
	ocmPath, err := exec.LookPath("ocm")
	if err != nil {
		return "", nil, fmt.Errorf("ocm binary not found in PATH — install from: https://ocm.software/docs/getting-started/installing-the-ocm-cli/")
	}

	dir, err := os.MkdirTemp("", "many-ocm-recursive-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	// Download resources for the main component.
	if err := downloadOCMResources(ocmPath, ref, dir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("downloading main component: %w", err)
	}

	// Discover and download referenced sub-components.
	refs, err := getOCMReferences(ocmPath, ref)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("getting component references: %w", err)
	}

	for _, subRef := range refs {
		if err := downloadOCMResources(ocmPath, subRef, dir); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("downloading referenced component %q: %w", subRef, err)
		}
	}

	return unwrapSingleRoot(dir), cleanup, nil
}

// downloadOCMResources runs `ocm download resources <ref> -O <destDir>`.
func downloadOCMResources(ocmPath, ref, destDir string) error {
	cmd := exec.Command(ocmPath, "download", "resources", ref, "-O", destDir)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ocm download resources %q failed: %w\nstderr: %s", ref, err, stderr.String())
	}
	return nil
}

// ocmReference represents one entry in `ocm get references` YAML output.
type ocmReference struct {
	ComponentName string `yaml:"componentName"`
	Version       string `yaml:"version"`
}

// ocmReferencesOutput is the top-level structure of `ocm get references -o yaml`.
type ocmReferencesOutput struct {
	Items []ocmReference `yaml:"items"`
}

// getOCMReferences runs `ocm get references <ref> -o yaml` and returns fully
// qualified component references for each discovered sub-component.
func getOCMReferences(ocmPath, ref string) ([]string, error) {
	cmd := exec.Command(ocmPath, "get", "references", ref, "-o", "yaml")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ocm get references failed: %w\nstderr: %s", err, stderr.String())
	}

	if strings.TrimSpace(stdout.String()) == "" {
		return nil, nil
	}

	var out ocmReferencesOutput
	if err := yaml.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("parsing ocm references output: %w", err)
	}

	prefix := repoPrefix(ref)
	var refs []string
	for _, item := range out.Items {
		refs = append(refs, prefix+item.ComponentName+":"+item.Version)
	}
	return refs, nil
}

// repoPrefix extracts the repository portion before "//" from an OCM reference.
// For example, "ghcr.io/myorg/ocm//github.com/myorg/comp:v1" returns "ghcr.io/myorg/ocm//".
func repoPrefix(ref string) string {
	idx := strings.Index(ref, "//")
	if idx < 0 {
		return ""
	}
	return ref[:idx+2]
}

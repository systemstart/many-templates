package resolve

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// resolveHTTPS downloads the resource at the given URL.
// Tarball URLs (.tar.gz, .tgz) are extracted into a temp directory.
// All other URLs are downloaded as a single file.
func resolveHTTPS(url string) (string, func(), error) {
	resp, err := httpClient.Get(url) //nolint:noctx // no long-lived context available
	if err != nil {
		return "", nil, fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("HTTP GET %s: status %d", url, resp.StatusCode)
	}

	if isTarballURL(url) {
		return downloadTarball(resp.Body)
	}
	return downloadSingleFile(resp.Body)
}

func isTarballURL(url string) bool {
	// Strip query string and fragment before checking suffix.
	path := url
	if i := strings.Index(path, "?"); i != -1 {
		path = path[:i]
	}
	if i := strings.Index(path, "#"); i != -1 {
		path = path[:i]
	}
	return strings.HasSuffix(path, ".tar.gz") || strings.HasSuffix(path, ".tgz")
}

func downloadTarball(body io.Reader) (string, func(), error) {
	dir, err := os.MkdirTemp("", "many-https-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	if err := extractTarGz(body, dir); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("extracting tarball: %w", err)
	}

	return unwrapSingleRoot(dir), cleanup, nil
}

func downloadSingleFile(body io.Reader) (string, func(), error) {
	f, err := os.CreateTemp("", "many-https-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp file: %w", err)
	}

	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("downloading file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("closing temp file: %w", err)
	}

	path := f.Name()
	cleanup := func() { _ = os.Remove(path) }
	return path, cleanup, nil
}

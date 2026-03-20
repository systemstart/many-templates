package resolve

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 5 * time.Minute}

// resolveHTTPS downloads the resource at the given URL.
// Tarball URLs (.tar.gz, .tgz) are extracted into a temp directory.
// All other URLs are downloaded as a single file.
// The third return value is the computed sha256 hex digest of the downloaded content.
func resolveHTTPS(url, expectedSHA256 string) (string, func(), string, error) {
	resp, err := httpClient.Get(url) //nolint:noctx // no long-lived context available
	if err != nil {
		return "", nil, "", fmt.Errorf("HTTP GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", nil, "", fmt.Errorf("HTTP GET %s: status %d", url, resp.StatusCode)
	}

	hasher := sha256.New()
	body := io.TeeReader(resp.Body, hasher)

	var p string
	var cleanup func()
	if isTarballURL(url) {
		p, cleanup, err = downloadTarball(body)
	} else {
		p, cleanup, err = downloadSingleFile(body, url)
	}
	if err != nil {
		return "", nil, "", err
	}

	computed := hex.EncodeToString(hasher.Sum(nil))
	if expectedSHA256 != "" && computed != expectedSHA256 {
		if cleanup != nil {
			cleanup()
		}
		return "", nil, "", fmt.Errorf("sha256 mismatch for %s: expected %s, got %s", url, expectedSHA256, computed)
	}

	return p, cleanup, computed, nil
}

// filenameFromURL extracts the last path segment from a URL, stripping query
// and fragment. Returns "download" as a fallback if the URL has no usable name.
func filenameFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil {
		name := path.Base(u.Path)
		if name != "" && name != "." && name != "/" {
			return name
		}
	}
	return "download"
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

func downloadSingleFile(body io.Reader, rawURL string) (string, func(), error) {
	dir, err := os.MkdirTemp("", "many-https-*")
	if err != nil {
		return "", nil, fmt.Errorf("creating temp dir: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(dir) }

	filePath := dir + "/" + filenameFromURL(rawURL)
	f, err := os.Create(filePath)
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("creating file: %w", err)
	}

	if _, err := io.Copy(f, body); err != nil {
		_ = f.Close()
		cleanup()
		return "", nil, fmt.Errorf("downloading file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return "", nil, fmt.Errorf("closing temp file: %w", err)
	}

	return filePath, cleanup, nil
}

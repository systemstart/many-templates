package resolve

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveHTTPS_Tarball(t *testing.T) {
	entries := []tarEntry{
		{name: "project/", isDir: true},
		{name: "project/config.yaml", content: "key: value"},
		{name: "project/sub/", isDir: true},
		{name: "project/sub/data.txt", content: "hello"},
	}
	archive := buildTarGz(t, entries)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/gzip")
		_, _ = w.Write(archive.Bytes())
	}))
	defer srv.Close()

	path, cleanup, computed, err := resolveHTTPS(srv.URL+"/repo.tar.gz", "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if computed == "" {
		t.Error("expected non-empty computed sha256")
	}

	// Should unwrap single root "project/"
	data, err := os.ReadFile(filepath.Join(path, "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "key: value" {
		t.Errorf("got %q, want %q", string(data), "key: value")
	}

	data, err = os.ReadFile(filepath.Join(path, "sub", "data.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello" {
		t.Errorf("got %q, want %q", string(data), "hello")
	}
}

func TestResolveHTTPS_SingleFile(t *testing.T) {
	content := "global:\n  env: prod\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	defer srv.Close()

	path, cleanup, computed, err := resolveHTTPS(srv.URL+"/context.yaml", "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	// Computed sha256 should match the content.
	sum := sha256.Sum256([]byte(content))
	expected := hex.EncodeToString(sum[:])
	if computed != expected {
		t.Errorf("computed sha256 = %q, want %q", computed, expected)
	}

	// Filename should be derived from URL, not random.
	if filepath.Base(path) != "context.yaml" {
		t.Errorf("expected filename 'context.yaml', got %q", filepath.Base(path))
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != content {
		t.Errorf("got %q, want %q", string(data), content)
	}
}

func TestResolveHTTPS_404(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	_, _, _, err := resolveHTTPS(srv.URL+"/missing.yaml", "")
	if err == nil {
		t.Fatal("expected error for 404, got nil")
	}
	if !strings.Contains(err.Error(), "status 404") {
		t.Errorf("expected status 404 in error, got %q", err.Error())
	}
}

func TestResolveHTTPS_Cleanup(t *testing.T) {
	content := "temp"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(content))
	}))
	defer srv.Close()

	path, cleanup, _, err := resolveHTTPS(srv.URL+"/file.txt", "")
	if err != nil {
		t.Fatal(err)
	}

	// File should exist before cleanup.
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("file should exist before cleanup: %v", err)
	}

	cleanup()

	// File should be gone after cleanup.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("file should not exist after cleanup")
	}
}

func TestResolveHTTPS_TarballCleanup(t *testing.T) {
	entries := []tarEntry{
		{name: "data.txt", content: "hello"},
	}
	archive := buildTarGz(t, entries)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(archive.Bytes())
	}))
	defer srv.Close()

	path, cleanup, _, err := resolveHTTPS(srv.URL+"/archive.tar.gz", "")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("path should exist before cleanup: %v", err)
	}

	cleanup()

	// The parent temp dir (or the path itself) should be gone.
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("path should not exist after cleanup")
	}
}

func TestIsTarballURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com/archive.tar.gz", true},
		{"https://example.com/archive.tgz", true},
		{"https://example.com/archive.tar.gz?token=abc", true},
		{"https://example.com/archive.tgz#section", true},
		{"https://example.com/archive.tar.gz?a=1#frag", true},
		{"https://example.com/file.yaml", false},
		{"https://example.com/file.txt", false},
		{"https://example.com/file.yaml#tar.gz", false},
	}
	for _, tt := range tests {
		if got := isTarballURL(tt.url); got != tt.want {
			t.Errorf("isTarballURL(%q) = %v, want %v", tt.url, got, tt.want)
		}
	}
}

func TestFilenameFromURL(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/some-operator.yaml", "some-operator.yaml"},
		{"https://example.com/path/to/config.yaml", "config.yaml"},
		{"https://example.com/file.yaml?token=abc", "file.yaml"},
		{"https://example.com/file.yaml#section", "file.yaml"},
		{"https://example.com/file.yaml?a=1#frag", "file.yaml"},
		{"https://example.com/", "download"},
		{"https://example.com", "download"},
		{"://broken", "download"},
	}
	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := filenameFromURL(tt.url)
			if got != tt.want {
				t.Errorf("filenameFromURL(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}

func TestResolveHTTPS_SingleFilePreservesName(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("content"))
	}))
	defer srv.Close()

	path, cleanup, _, err := resolveHTTPS(srv.URL+"/path/to/some-operator.yaml", "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

	if filepath.Base(path) != "some-operator.yaml" {
		t.Errorf("expected filename 'some-operator.yaml', got %q", filepath.Base(path))
	}
}

func TestResolveHTTPS_ConnectionRefused(t *testing.T) {
	_, _, _, err := resolveHTTPS("https://127.0.0.1:1/nope.yaml", "")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestResolveHTTPS_SHA256Match(t *testing.T) {
	content := []byte("hello world\n")
	sum := sha256.Sum256(content)
	checksum := hex.EncodeToString(sum[:])

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	path, cleanup, computed, err := resolveHTTPS(srv.URL+"/file.yaml", checksum)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer cleanup()

	if computed != checksum {
		t.Errorf("computed sha256 = %q, want %q", computed, checksum)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(content) {
		t.Errorf("got %q, want %q", string(data), string(content))
	}
}

func TestResolveHTTPS_SHA256Mismatch(t *testing.T) {
	content := []byte("hello world\n")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(content)
	}))
	defer srv.Close()

	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	_, _, _, err := resolveHTTPS(srv.URL+"/file.yaml", wrongHash)
	if err == nil {
		t.Fatal("expected error for sha256 mismatch, got nil")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Errorf("expected 'sha256 mismatch' in error, got %q", err.Error())
	}
	if !strings.Contains(err.Error(), wrongHash) {
		t.Errorf("expected wrong hash in error, got %q", err.Error())
	}
	// Should contain the actual hash
	sum := sha256.Sum256(content)
	actual := hex.EncodeToString(sum[:])
	if !strings.Contains(err.Error(), actual) {
		t.Errorf("expected actual hash %q in error, got %q", actual, err.Error())
	}
}

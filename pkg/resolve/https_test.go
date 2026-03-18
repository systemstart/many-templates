package resolve

import (
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

	path, cleanup, err := resolveHTTPS(srv.URL + "/repo.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

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

	path, cleanup, err := resolveHTTPS(srv.URL + "/context.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()

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

	_, _, err := resolveHTTPS(srv.URL + "/missing.yaml")
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

	path, cleanup, err := resolveHTTPS(srv.URL + "/file.txt")
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

	path, cleanup, err := resolveHTTPS(srv.URL + "/archive.tar.gz")
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

func TestResolveHTTPS_ConnectionRefused(t *testing.T) {
	_, _, err := resolveHTTPS("https://127.0.0.1:1/nope.yaml")
	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

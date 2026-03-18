package resolve

import (
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	tests := []struct {
		name      string
		uri       string
		wantPath  string
		wantErr   string // substring to match; "" means no error expected
		wantClean bool   // true if cleanup should be non-nil
	}{
		{
			name:     "bare relative path",
			uri:      "./infra",
			wantPath: "./infra",
		},
		{
			name:     "bare absolute path",
			uri:      "/tmp/infra",
			wantPath: "/tmp/infra",
		},
		{
			name:     "file scheme relative path",
			uri:      "file://./infra",
			wantPath: "./infra",
		},
		{
			name:     "file scheme absolute path",
			uri:      "file:///tmp/infra",
			wantPath: "/tmp/infra",
		},
		{
			name:    "unknown scheme",
			uri:     "ftp://example.com/file",
			wantErr: "unsupported scheme: ftp://",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, cleanup, err := Resolve(tt.uri)

			if tt.wantErr != "" {
				requireErrorContains(t, err, tt.wantErr)
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if path != tt.wantPath {
				t.Errorf("path = %q, want %q", path, tt.wantPath)
			}
			assertCleanup(t, cleanup, tt.wantClean)
		})
	}
}

func TestResolve_OCIBranch(t *testing.T) {
	// Exercise the oci:// branch of Resolve(). The actual OCI pull will fail
	// (missing crane or bad ref), but we cover the dispatch path.
	_, _, err := Resolve("oci://invalid-ref-that-wont-resolve")
	if err == nil {
		t.Fatal("expected error for bad OCI ref, got nil")
	}
}

func TestResolve_OCMBranch(t *testing.T) {
	// Exercise the ocm:// branch of Resolve().
	_, _, err := Resolve("ocm://invalid//comp:v1")
	if err == nil {
		t.Fatal("expected error for bad OCM ref, got nil")
	}
}

func TestResolve_HTTPSBranch(t *testing.T) {
	// Exercise the https:// branch of Resolve() with an unreachable URL.
	_, _, err := Resolve("https://127.0.0.1:1/nonexistent")
	if err == nil {
		t.Fatal("expected error for unreachable HTTPS URL, got nil")
	}
}

func TestIsRemote(t *testing.T) {
	tests := []struct {
		uri  string
		want bool
	}{
		{"oci://ghcr.io/myorg/image:v1", true},
		{"https://example.com/archive.tar.gz", true},
		{"ocm://ghcr.io/myorg/ocm//comp:v1", true},
		{"ftp://example.com/file", true},
		{"./local/path", false},
		{"/absolute/path", false},
		{"relative/path", false},
	}
	for _, tt := range tests {
		t.Run(tt.uri, func(t *testing.T) {
			got := IsRemote(tt.uri)
			if got != tt.want {
				t.Errorf("IsRemote(%q) = %v, want %v", tt.uri, got, tt.want)
			}
		})
	}
}

func requireErrorContains(t *testing.T, err error, substr string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected error containing %q, got nil", substr)
	}
	if !strings.Contains(err.Error(), substr) {
		t.Fatalf("expected error containing %q, got %q", substr, err.Error())
	}
}

func assertCleanup(t *testing.T, cleanup func(), want bool) {
	t.Helper()
	if want && cleanup == nil {
		t.Error("expected non-nil cleanup function")
	}
	if !want && cleanup != nil {
		t.Error("expected nil cleanup function")
	}
}

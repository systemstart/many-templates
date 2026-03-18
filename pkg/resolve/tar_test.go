package resolve

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// helper: create an in-memory tar archive from a list of entries.
type tarEntry struct {
	name    string
	content string
	isDir   bool
	mode    int64
}

func buildTar(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, e := range entries {
		if e.mode == 0 {
			if e.isDir {
				e.mode = 0o750
			} else {
				e.mode = 0o644
			}
		}
		hdr := &tar.Header{
			Name: e.name,
			Size: int64(len(e.content)),
			Mode: e.mode,
		}
		if e.isDir {
			hdr.Typeflag = tar.TypeDir
			hdr.Size = 0
		} else {
			hdr.Typeflag = tar.TypeReg
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if !e.isDir {
			if _, err := tw.Write([]byte(e.content)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func buildTarGz(t *testing.T, entries []tarEntry) *bytes.Buffer {
	t.Helper()
	tarBuf := buildTar(t, entries)
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	if _, err := gw.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return &buf
}

func TestExtractTar(t *testing.T) {
	entries := []tarEntry{
		{name: "mydir/", isDir: true},
		{name: "mydir/hello.txt", content: "hello world"},
		{name: "mydir/sub/", isDir: true},
		{name: "mydir/sub/deep.txt", content: "deep"},
	}

	dest := t.TempDir()
	buf := buildTar(t, entries)
	if err := extractTar(buf, dest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "mydir", "hello.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Errorf("got %q, want %q", string(data), "hello world")
	}

	data, err = os.ReadFile(filepath.Join(dest, "mydir", "sub", "deep.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "deep" {
		t.Errorf("got %q, want %q", string(data), "deep")
	}
}

func TestExtractTarGz(t *testing.T) {
	entries := []tarEntry{
		{name: "file.yaml", content: "key: value"},
	}

	dest := t.TempDir()
	buf := buildTarGz(t, entries)
	if err := extractTarGz(buf, dest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "file.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "key: value" {
		t.Errorf("got %q, want %q", string(data), "key: value")
	}
}

func TestExtractTar_PathTraversal(t *testing.T) {
	entries := []tarEntry{
		{name: "../etc/passwd", content: "evil"},
	}

	dest := t.TempDir()
	buf := buildTar(t, entries)
	err := extractTar(buf, dest)
	if err == nil {
		t.Fatal("expected error for path traversal, got nil")
	}
	if got := err.Error(); !strings.Contains(got, "invalid path") {
		t.Errorf("expected error about invalid path, got %q", got)
	}
}

func TestExtractTar_AbsolutePath(t *testing.T) {
	entries := []tarEntry{
		{name: "/tmp/evil", content: "evil"},
	}

	dest := t.TempDir()
	buf := buildTar(t, entries)
	err := extractTar(buf, dest)
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestExtractTar_SkipsSymlinks(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	hdr := &tar.Header{
		Name:     "link",
		Typeflag: tar.TypeSymlink,
		Linkname: "/etc/passwd",
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := extractTar(&buf, dest); err != nil {
		t.Fatal(err)
	}

	// symlink should not exist
	if _, err := os.Lstat(filepath.Join(dest, "link")); !os.IsNotExist(err) {
		t.Error("expected symlink to be skipped")
	}
}

func TestExtractTar_Empty(t *testing.T) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	if err := extractTar(&buf, dest); err != nil {
		t.Fatalf("unexpected error for empty tar: %v", err)
	}
}

func TestExtractTar_ImplicitParentDirs(t *testing.T) {
	// File entries without explicit directory entries — extractFile must
	// create the parent directory via MkdirAll.
	entries := []tarEntry{
		{name: "a/b/c.txt", content: "deep"},
	}

	dest := t.TempDir()
	buf := buildTar(t, entries)
	if err := extractTar(buf, dest); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(filepath.Join(dest, "a", "b", "c.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "deep" {
		t.Errorf("got %q, want %q", string(data), "deep")
	}
}

func TestExtractTarGz_InvalidGzip(t *testing.T) {
	buf := bytes.NewBufferString("not gzip data")
	dest := t.TempDir()
	err := extractTarGz(buf, dest)
	if err == nil {
		t.Fatal("expected error for invalid gzip, got nil")
	}
}

func TestUnwrapSingleRoot(t *testing.T) {
	t.Run("single directory child", func(t *testing.T) {
		dir := t.TempDir()
		child := filepath.Join(dir, "repo-v1.0.0")
		if err := os.Mkdir(child, 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(child, "file.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		got := unwrapSingleRoot(dir)
		if got != child {
			t.Errorf("unwrapSingleRoot = %q, want %q", got, child)
		}
	})

	t.Run("single file child", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "only.txt"), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}

		got := unwrapSingleRoot(dir)
		if got != dir {
			t.Errorf("unwrapSingleRoot = %q, want %q (unchanged)", got, dir)
		}
	})

	t.Run("multiple children", func(t *testing.T) {
		dir := t.TempDir()
		if err := os.Mkdir(filepath.Join(dir, "a"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Mkdir(filepath.Join(dir, "b"), 0o750); err != nil {
			t.Fatal(err)
		}

		got := unwrapSingleRoot(dir)
		if got != dir {
			t.Errorf("unwrapSingleRoot = %q, want %q (unchanged)", got, dir)
		}
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		got := unwrapSingleRoot(dir)
		if got != dir {
			t.Errorf("unwrapSingleRoot = %q, want %q (unchanged)", got, dir)
		}
	})
}

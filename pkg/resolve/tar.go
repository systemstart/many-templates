package resolve

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// maxFileSize is the maximum size of a single file extracted from a tar archive (1 GB).
const maxFileSize = 1 << 30

// extractTarGz decompresses a gzip stream and extracts the tar archive into destDir.
func extractTarGz(r io.Reader, destDir string) error {
	gz, err := gzip.NewReader(r)
	if err != nil {
		return fmt.Errorf("gzip decompress: %w", err)
	}
	defer func() { _ = gz.Close() }()
	return extractTar(gz, destDir)
}

// extractTar walks a tar archive and writes entries into destDir.
// Security: rejects entries with ".." path components or absolute paths. Symlinks are skipped.
func extractTar(r io.Reader, destDir string) error {
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("reading tar: %w", err)
		}

		clean, err := sanitizeTarPath(hdr.Name)
		if err != nil {
			return err
		}

		target := filepath.Join(destDir, clean)

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return fmt.Errorf("creating directory %s: %w", clean, err)
			}
		case tar.TypeReg:
			if err := extractFile(target, clean, hdr, tr); err != nil {
				return err
			}
		default:
			// Skip symlinks and other special types.
			continue
		}
	}
	return nil
}

// sanitizeTarPath validates and cleans a tar entry path.
func sanitizeTarPath(name string) (string, error) {
	clean := filepath.Clean(name)
	if strings.HasPrefix(clean, "/") || strings.HasPrefix(clean, "..") ||
		strings.Contains(clean, string(filepath.Separator)+"..") {
		return "", fmt.Errorf("tar entry has invalid path: %s", name)
	}
	return clean, nil
}

// extractFile writes a single regular file from a tar archive.
func extractFile(target, clean string, hdr *tar.Header, tr io.Reader) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return fmt.Errorf("creating parent directory for %s: %w", clean, err)
	}
	mode := hdr.FileInfo().Mode()
	f, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", clean, err)
	}
	if _, err := io.Copy(f, io.LimitReader(tr, maxFileSize)); err != nil {
		_ = f.Close()
		return fmt.Errorf("writing file %s: %w", clean, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("closing file %s: %w", clean, err)
	}
	return nil
}

// unwrapSingleRoot returns the path to the single child directory if dir
// contains exactly one entry and that entry is a directory (e.g. GitHub
// archive tarballs like repo-v1.0.0/). Otherwise it returns dir unchanged.
func unwrapSingleRoot(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		return dir
	}
	if entries[0].IsDir() {
		return filepath.Join(dir, entries[0].Name())
	}
	return dir
}

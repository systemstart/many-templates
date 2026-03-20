package api

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUpdateSourceSHA256_UpdatesEmptySHA256(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	content := `source:
  - https: "https://example.com/file.yaml"
    sha256: ""
    temporary: true
pipeline: []
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{
		"https://example.com/file.yaml": "abc123def456",
	}

	if err := UpdateSourceSHA256(f, updates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	result := string(data)
	if !strings.Contains(result, `sha256: "abc123def456"`) {
		t.Errorf("expected updated sha256 value in output, got:\n%s", result)
	}
	// Should still have https
	if !strings.Contains(result, "https://example.com/file.yaml") {
		t.Errorf("expected https URL preserved, got:\n%s", result)
	}
}

func TestUpdateSourceSHA256_InsertsMissingSHA256(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	content := `source:
  - https: "https://example.com/file.yaml"
    temporary: true
pipeline: []
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{
		"https://example.com/file.yaml": "deadbeef",
	}

	if err := UpdateSourceSHA256(f, updates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	result := string(data)
	if !strings.Contains(result, `sha256: "deadbeef"`) {
		t.Errorf("expected inserted sha256, got:\n%s", result)
	}
}

func TestUpdateSourceSHA256_PreservesComments(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	content := `source:
  # renovate: datasource=github-releases depName=example/repo
  - https: "https://example.com/file.yaml"
    sha256: ""
    temporary: true
pipeline: []
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{
		"https://example.com/file.yaml": "newsha256",
	}

	if err := UpdateSourceSHA256(f, updates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	result := string(data)
	if !strings.Contains(result, "renovate:") {
		t.Errorf("expected comment preserved, got:\n%s", result)
	}
	if !strings.Contains(result, `sha256: "newsha256"`) {
		t.Errorf("expected updated sha256, got:\n%s", result)
	}
}

func TestUpdateSourceSHA256_NoMatchNoOp(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	content := `source:
  - https: "https://example.com/file.yaml"
    sha256: "existing"
pipeline: []
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// URL doesn't match
	updates := map[string]string{
		"https://other.com/file.yaml": "newsha",
	}

	if err := UpdateSourceSHA256(f, updates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	// File should be unchanged (no match = no write).
	if string(data) != content {
		t.Errorf("expected file unchanged, got:\n%s", string(data))
	}
}

func TestUpdateSourceSHA256_EmptyUpdates(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	content := `source:
  - https: "https://example.com/file.yaml"
    sha256: ""
pipeline: []
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := UpdateSourceSHA256(f, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	if string(data) != content {
		t.Errorf("expected file unchanged, got:\n%s", string(data))
	}
}

func TestUpdateSourceSHA256_MultipleEntries(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	content := `source:
  - https: "https://example.com/a.yaml"
    sha256: ""
  - https: "https://example.com/b.yaml"
    sha256: ""
pipeline: []
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{
		"https://example.com/a.yaml": "sha_a",
		"https://example.com/b.yaml": "sha_b",
	}

	if err := UpdateSourceSHA256(f, updates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	result := string(data)
	if !strings.Contains(result, `sha256: "sha_a"`) {
		t.Errorf("expected sha_a, got:\n%s", result)
	}
	if !strings.Contains(result, `sha256: "sha_b"`) {
		t.Errorf("expected sha_b, got:\n%s", result)
	}
}

func TestUpdateSourceSHA256_PreservesDocMarker(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	content := `---
source:
  - https: "https://example.com/file.yaml"
    sha256: ""
pipeline: []
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{
		"https://example.com/file.yaml": "abc",
	}

	if err := UpdateSourceSHA256(f, updates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(string(data), "---\n") {
		t.Errorf("expected leading ---, got:\n%s", string(data))
	}
}

func TestUpdateSourceSHA256_NoDocMarker(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, ".many.yaml")
	content := `source:
  - https: "https://example.com/file.yaml"
    sha256: ""
pipeline: []
`
	if err := os.WriteFile(f, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	updates := map[string]string{
		"https://example.com/file.yaml": "abc",
	}

	if err := UpdateSourceSHA256(f, updates); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}

	if strings.HasPrefix(string(data), "---") {
		t.Errorf("should not have leading --- when original didn't, got:\n%s", string(data))
	}
}

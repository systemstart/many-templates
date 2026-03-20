package api

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// UpdateSourceSHA256 reads a .many.yaml file, finds HTTPS source entries whose
// URL matches a key in updates, and sets the corresponding sha256 value.
// It uses yaml.Node-based round-tripping to preserve comments and formatting.
func UpdateSourceSHA256(filePath string, updates map[string]string) error {
	if len(updates) == 0 {
		return nil
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filePath, err)
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return fmt.Errorf("parsing %s: %w", filePath, err)
	}

	if doc.Kind != yaml.DocumentNode || len(doc.Content) == 0 {
		return fmt.Errorf("unexpected YAML structure in %s", filePath)
	}

	if !walkAndUpdate(doc.Content[0], updates) {
		return nil
	}

	if err := writeYAMLFile(filePath, &doc, data); err != nil {
		return err
	}

	for url, sha := range updates {
		slog.Info("updated sha256 in pipeline file", "file", filePath, "url", url, "sha256", sha)
	}

	return nil
}

// writeYAMLFile encodes a YAML document and writes it atomically to filePath,
// preserving the document marker ("---") if present in the original data.
func writeYAMLFile(filePath string, doc *yaml.Node, originalData []byte) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return fmt.Errorf("encoding %s: %w", filePath, err)
	}
	if err := enc.Close(); err != nil {
		return fmt.Errorf("closing encoder for %s: %w", filePath, err)
	}

	result := buf.String()

	// Normalize: always strip "---\n" from encoder output, then add back if original had it.
	hasDocMarker := strings.HasPrefix(strings.TrimSpace(string(originalData)), "---")
	result = strings.TrimPrefix(result, "---\n")
	if hasDocMarker {
		result = "---\n" + result
	}

	// Write atomically: write to temp file, then rename.
	tmp := filePath + ".tmp"
	if err := os.WriteFile(tmp, []byte(result), 0o644); err != nil {
		return fmt.Errorf("writing temp file %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, filePath); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("renaming %s to %s: %w", tmp, filePath, err)
	}

	return nil
}

// walkAndUpdate recursively walks the YAML node tree looking for mapping nodes
// that contain an "https" key matching a URL in updates. When found, it updates
// the sibling "sha256" value (or inserts one if missing).
func walkAndUpdate(node *yaml.Node, updates map[string]string) bool {
	if node == nil {
		return false
	}

	modified := false

	if node.Kind == yaml.MappingNode {
		modified = updateMapping(node, updates)
		// Also recurse into mapping values.
		for i := 1; i < len(node.Content); i += 2 {
			if walkAndUpdate(node.Content[i], updates) {
				modified = true
			}
		}
		return modified
	}

	for _, child := range node.Content {
		if walkAndUpdate(child, updates) {
			modified = true
		}
	}
	return modified
}

// updateMapping checks if a mapping node has an "https" key whose value matches
// an entry in updates. If so, it updates or inserts the "sha256" key.
func updateMapping(node *yaml.Node, updates map[string]string) bool {
	httpsIdx := -1
	sha256Idx := -1

	for i := 0; i < len(node.Content)-1; i += 2 {
		key := node.Content[i]
		if key.Kind == yaml.ScalarNode {
			switch key.Value {
			case "https":
				httpsIdx = i
			case "sha256":
				sha256Idx = i
			}
		}
	}

	if httpsIdx < 0 {
		return false
	}

	httpsValue := node.Content[httpsIdx+1].Value
	newSHA, ok := updates[httpsValue]
	if !ok {
		return false
	}

	if sha256Idx >= 0 {
		// Update existing sha256 value.
		node.Content[sha256Idx+1].Value = newSHA
		node.Content[sha256Idx+1].Style = yaml.DoubleQuotedStyle
	} else {
		// Insert sha256 key+value after the https pair.
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: "sha256",
		}
		valueNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Tag:   "!!str",
			Value: newSHA,
			Style: yaml.DoubleQuotedStyle,
		}

		insertPos := httpsIdx + 2
		newContent := make([]*yaml.Node, 0, len(node.Content)+2)
		newContent = append(newContent, node.Content[:insertPos]...)
		newContent = append(newContent, keyNode, valueNode)
		newContent = append(newContent, node.Content[insertPos:]...)
		node.Content = newContent
	}

	return true
}

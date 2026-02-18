package processing

import (
	"fmt"
	"maps"
	"os"

	"gopkg.in/yaml.v3"
)

// LoadContextFile reads a YAML file and returns it as a map.
func LoadContextFile(filename string) (map[string]any, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading context file: %w", err)
	}

	var ctx map[string]any
	if err := yaml.Unmarshal(data, &ctx); err != nil {
		return nil, fmt.Errorf("parsing context file: %w", err)
	}

	if ctx == nil {
		ctx = make(map[string]any)
	}

	return ctx, nil
}

// MergeContext performs a shallow merge of local context over global context.
// Local keys override global keys at the top level.
func MergeContext(global, local map[string]any) map[string]any {
	merged := make(map[string]any, len(global)+len(local))
	maps.Copy(merged, global)
	maps.Copy(merged, local)
	return merged
}

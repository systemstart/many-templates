package processing

import (
	"bytes"
	"fmt"
	"maps"
	"os"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig/v3"
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

// InterpolateContext performs a single-pass template render of string values
// in the context map against the map itself. Strings containing "{{" are
// parsed as Go templates with Sprig functions and executed with ctx as data.
func InterpolateContext(ctx map[string]any) error {
	return interpolateMap(ctx, ctx)
}

func interpolateMap(m map[string]any, root map[string]any) error {
	for k, v := range m {
		switch val := v.(type) {
		case string:
			rendered, err := renderString(val, root)
			if err != nil {
				return fmt.Errorf("key %q: %w", k, err)
			}
			m[k] = rendered
		case map[string]any:
			if err := interpolateMap(val, root); err != nil {
				return err
			}
		case []any:
			if err := interpolateSlice(val, root); err != nil {
				return err
			}
		}
	}
	return nil
}

func interpolateSlice(s []any, root map[string]any) error {
	for i, v := range s {
		switch val := v.(type) {
		case string:
			rendered, err := renderString(val, root)
			if err != nil {
				return fmt.Errorf("index %d: %w", i, err)
			}
			s[i] = rendered
		case map[string]any:
			if err := interpolateMap(val, root); err != nil {
				return err
			}
		case []any:
			if err := interpolateSlice(val, root); err != nil {
				return err
			}
		}
	}
	return nil
}

func renderString(s string, data map[string]any) (string, error) {
	if !strings.Contains(s, "{{") {
		return s, nil
	}
	tmpl, err := template.New("").Funcs(sprig.FuncMap()).Parse(s)
	if err != nil {
		return "", fmt.Errorf("parsing template: %w", err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("executing template: %w", err)
	}
	return buf.String(), nil
}

// MergeContext performs a shallow merge of local context over global context.
// Local keys override global keys at the top level.
func MergeContext(global, local map[string]any) map[string]any {
	merged := make(map[string]any, len(global)+len(local))
	maps.Copy(merged, global)
	maps.Copy(merged, local)
	return merged
}

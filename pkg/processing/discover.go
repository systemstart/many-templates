package processing

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"

	"github.com/systemstart/many-templates/pkg/api"
)

const configFilename = ".many.yaml"

// DiscoverPipelines walks root looking for .many.yaml files up to maxDepth.
// A maxDepth of -1 means unlimited. 0 means only root itself.
// Results are sorted by path depth (parents before children).
func DiscoverPipelines(root string, maxDepth int) ([]*api.Pipeline, error) {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolving root path: %w", err)
	}

	paths, err := collectConfigPaths(absRoot, maxDepth)
	if err != nil {
		return nil, err
	}

	slices.SortFunc(paths, func(a, b string) int {
		return pathDepth(a) - pathDepth(b)
	})

	return loadAll(paths)
}

func collectConfigPaths(absRoot string, maxDepth int) ([]string, error) {
	var paths []string
	err := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk error at %s: %w", path, err)
		}

		if d.IsDir() && maxDepth >= 0 {
			rel, relErr := filepath.Rel(absRoot, path)
			if relErr != nil {
				return fmt.Errorf("computing relative path for %s: %w", path, relErr)
			}
			if pathDepth(rel) > maxDepth {
				return filepath.SkipDir
			}
		}

		if !d.IsDir() && d.Name() == configFilename {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory tree: %w", err)
	}
	return paths, nil
}

func loadAll(paths []string) ([]*api.Pipeline, error) {
	pipelines := make([]*api.Pipeline, 0, len(paths))
	for _, p := range paths {
		pipeline, err := api.LoadPipeline(p)
		if err != nil {
			return nil, fmt.Errorf("loading %s: %w", p, err)
		}
		pipelines = append(pipelines, pipeline)
	}
	return pipelines, nil
}

func pathDepth(p string) int {
	if p == "." {
		return 0
	}
	return strings.Count(filepath.ToSlash(p), "/") + 1
}

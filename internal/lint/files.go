package lint

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// readFile reads a file from disk and returns its bytes.
func readFile(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading file %q: %w", path, err)
	}
	return data, nil
}

// CollectFiles resolves file paths from the provided sources. If a
// path is a directory, it collects all YAML/YML files within it.
// When recursive is true, it walks subdirectories as well.
func CollectFiles(paths []string, recursive bool) ([]string, error) {
	var files []string

	for _, p := range paths {
		info, err := os.Stat(p)
		if err != nil {
			return nil, fmt.Errorf("cannot access %q: %w", p, err)
		}

		if !info.IsDir() {
			files = append(files, p)
			continue
		}

		if recursive {
			err = filepath.Walk(p, func(path string, fi os.FileInfo, walkErr error) error {
				if walkErr != nil {
					return walkErr
				}
				if !fi.IsDir() && isYAMLFile(path) {
					files = append(files, path)
				}
				return nil
			})
		} else {
			entries, readErr := os.ReadDir(p)
			if readErr != nil {
				return nil, fmt.Errorf("reading directory %q: %w", p, readErr)
			}
			for _, entry := range entries {
				if !entry.IsDir() && isYAMLFile(entry.Name()) {
					files = append(files, filepath.Join(p, entry.Name()))
				}
			}
		}
		if err != nil {
			return nil, fmt.Errorf("walking directory %q: %w", p, err)
		}
	}

	return files, nil
}

// ReadFromStdin reads all data from a reader (intended for stdin).
func ReadFromStdin(r io.Reader) ([]byte, error) {
	return io.ReadAll(r)
}

// isYAMLFile checks if a filename has a YAML extension.
func isYAMLFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".yaml" || ext == ".yml"
}

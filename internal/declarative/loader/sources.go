package loader

import (
	"fmt"
	"os"
	"strings"
)

// SourceType represents the type of configuration source
type SourceType int

const (
	// SourceTypeFile represents a single file source
	SourceTypeFile SourceType = iota
	// SourceTypeDirectory represents a directory source
	SourceTypeDirectory
	// SourceTypeSTDIN represents stdin source
	SourceTypeSTDIN
)

// Source represents a configuration source with its type
type Source struct {
	Path string
	Type SourceType
}

// ParseSources parses the filename flag values into individual sources
func ParseSources(filenames []string) ([]Source, error) {
	var sources []Source

	for _, filename := range filenames {
		// Handle comma-separated values
		parts := strings.Split(filename, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)
			if part == "" {
				continue
			}

			sourceType, err := detectSourceType(part)
			if err != nil {
				return nil, fmt.Errorf("invalid source %s: %w", part, err)
			}

			sources = append(sources, Source{
				Path: part,
				Type: sourceType,
			})
		}
	}

	// If no sources provided, default to current directory
	if len(sources) == 0 {
		sources = append(sources, Source{
			Path: ".",
			Type: SourceTypeDirectory,
		})
	}

	return sources, nil
}

// detectSourceType determines the type of a configuration source
func detectSourceType(source string) (SourceType, error) {
	// Check for stdin
	if source == "-" {
		return SourceTypeSTDIN, nil
	}

	// Check if file/directory exists
	info, err := os.Stat(source)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, fmt.Errorf("does not exist")
		}
		return 0, err
	}

	if info.IsDir() {
		return SourceTypeDirectory, nil
	}

	return SourceTypeFile, nil
}

// ValidateYAMLFile checks if a file has a valid YAML extension
func ValidateYAMLFile(path string) bool {
	ext := strings.ToLower(strings.TrimPrefix(path, "."))
	// Get the last extension for files like config.yaml.bak
	parts := strings.Split(path, ".")
	if len(parts) > 1 {
		ext = strings.ToLower(parts[len(parts)-1])
	}
	return ext == "yaml" || ext == "yml"
}

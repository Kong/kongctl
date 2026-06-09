package loader

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// ErrNoSources is returned when no configuration sources are provided.
var ErrNoSources = errors.New("no configuration sources specified; use -f to specify files, directories, or URLs")

// SourceType represents the type of configuration source
type SourceType int

const (
	// SourceTypeFile represents a single file source
	SourceTypeFile SourceType = iota
	// SourceTypeDirectory represents a directory source
	SourceTypeDirectory
	// SourceTypeSTDIN represents stdin source
	SourceTypeSTDIN
	// SourceTypeURL represents an HTTP(S) URL source
	SourceTypeURL
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
		parts := strings.SplitSeq(filename, ",")
		for part := range parts {
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

	if len(sources) == 0 {
		return nil, ErrNoSources
	}

	return sources, nil
}

// detectSourceType determines the type of a configuration source
func detectSourceType(source string) (SourceType, error) {
	// Check for stdin
	if source == "-" {
		return SourceTypeSTDIN, nil
	}

	if sourceType, ok, err := detectURLSourceType(source); ok || err != nil {
		return sourceType, err
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

func detectURLSourceType(source string) (SourceType, bool, error) {
	if !strings.Contains(source, "://") {
		return 0, false, nil
	}

	parsed, err := url.Parse(source)
	if err != nil {
		return 0, true, fmt.Errorf("invalid URL: %w", err)
	}

	switch parsed.Scheme {
	case "http", "https":
		if parsed.Host == "" {
			return 0, true, fmt.Errorf("URL must include a host")
		}
		return SourceTypeURL, true, nil
	default:
		return 0, true, fmt.Errorf("unsupported URL scheme %q", parsed.Scheme)
	}
}

func isURLSourcePath(source string) bool {
	sourceType, ok, err := detectURLSourceType(source)
	return err == nil && ok && sourceType == SourceTypeURL
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

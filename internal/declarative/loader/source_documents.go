package loader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	decerrors "github.com/kong/kongctl/internal/declarative/errors"
)

type sourceDocument struct {
	content    []byte
	sourcePath string
	rootDir    string
}

func (l *Loader) readSourceDocuments(
	ctx context.Context,
	sources []Source,
	recursive bool,
) ([]sourceDocument, error) {
	documents := make([]sourceDocument, 0, len(sources))
	for _, source := range sources {
		rootDir := l.resolveSourceRoot(source)
		switch source.Type {
		case SourceTypeFile:
			document, err := readFileSourceDocument(source.Path, rootDir)
			if err != nil {
				return nil, err
			}
			documents = append(documents, document)
		case SourceTypeDirectory:
			directoryDocuments, err := readDirectorySourceDocuments(source.Path, rootDir, recursive)
			if err != nil {
				return nil, err
			}
			documents = append(documents, directoryDocuments...)
		case SourceTypeSTDIN:
			document, err := readSTDINSourceDocument(rootDir)
			if err != nil {
				return nil, err
			}
			documents = append(documents, document)
		case SourceTypeURL:
			content, err := FetchURLWithOptions(ctx, source.Path, l.urlFetchOptions)
			if err != nil {
				return nil, err
			}
			documents = append(documents, sourceDocument{
				content: content, sourcePath: source.Path, rootDir: rootDir,
			})
		default:
			return nil, decerrors.FormatConfigurationError(
				source.Path,
				0,
				fmt.Sprintf("unknown source type: %v", source.Type),
			)
		}
	}
	return documents, nil
}

func readFileSourceDocument(path, rootDir string) (sourceDocument, error) {
	if !ValidateYAMLFile(path) {
		return sourceDocument{}, fmt.Errorf("file %s does not have .yaml or .yml extension", path)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return sourceDocument{}, fmt.Errorf("failed to read file %s: %w", path, err)
	}
	return sourceDocument{content: content, sourcePath: path, rootDir: rootDir}, nil
}

func readDirectorySourceDocuments(dirPath, rootDir string, recursive bool) ([]sourceDocument, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	documents := make([]sourceDocument, 0)
	subdirectoryCount := 0
	for _, entry := range entries {
		path := filepath.Join(dirPath, entry.Name())
		if entry.IsDir() {
			subdirectoryCount++
			if !recursive {
				continue
			}
			nested, err := readDirectorySourceDocumentsIfPresent(path, rootDir)
			if err != nil {
				return nil, err
			}
			documents = append(documents, nested...)
			continue
		}
		if !ValidateYAMLFile(path) {
			continue
		}
		document, err := readFileSourceDocument(path, rootDir)
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}

	if len(documents) == 0 {
		if subdirectoryCount > 0 && !recursive {
			return nil, fmt.Errorf(
				"no YAML files found in directory '%s'. Found %d subdirectories. Use -R to search subdirectories",
				dirPath,
				subdirectoryCount,
			)
		}
		return nil, fmt.Errorf("no YAML files found in directory '%s'", dirPath)
	}
	return documents, nil
}

func readDirectorySourceDocumentsIfPresent(dirPath, rootDir string) ([]sourceDocument, error) {
	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read directory %s: %w", dirPath, err)
	}

	documents := make([]sourceDocument, 0)
	for _, entry := range entries {
		path := filepath.Join(dirPath, entry.Name())
		if entry.IsDir() {
			nested, err := readDirectorySourceDocumentsIfPresent(path, rootDir)
			if err != nil {
				return nil, err
			}
			documents = append(documents, nested...)
			continue
		}
		if !ValidateYAMLFile(path) {
			continue
		}
		document, err := readFileSourceDocument(path, rootDir)
		if err != nil {
			return nil, err
		}
		documents = append(documents, document)
	}
	return documents, nil
}

func readSTDINSourceDocument(rootDir string) (sourceDocument, error) {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return sourceDocument{}, fmt.Errorf("failed to stat stdin: %w", err)
	}
	if (stat.Mode() & os.ModeCharDevice) != 0 {
		return sourceDocument{}, fmt.Errorf("no data provided on stdin")
	}
	content, err := io.ReadAll(os.Stdin)
	if err != nil {
		return sourceDocument{}, fmt.Errorf("failed to read stdin: %w", err)
	}
	return sourceDocument{content: content, sourcePath: "stdin", rootDir: rootDir}, nil
}

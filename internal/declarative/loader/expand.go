package loader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"gopkg.in/yaml.v3" //nolint:gomodguard_v2 // yaml.v3 preserves deferred tags and inspects document boundaries
)

// ExpandedDocument is an input document after local !file content is included
// and _templates/_extends are expanded. !env, !ref, !secret and lookup tags
// remain unresolved placeholders.
type ExpandedDocument struct {
	SourcePath string
	// Content is the expanded YAML; line numbers refer to it, not to the source.
	Content []byte
	// Empty is true when nothing remains once _templates is removed, such as a
	// file that only defines templates.
	Empty bool
}

// ExpandSources runs the same preprocessing as LoadFromSources up to and
// including template expansion. It does not parse resources, apply defaults,
// resolve references, validate resource types, read environment values or fetch secrets.
// YAML, tag declarations and template syntax are checked. Scalar and list inputs
// are supported as fragments; multiple YAML documents per input are rejected.
func (l *Loader) ExpandSources(ctx context.Context, sources []Source, recursive bool) ([]ExpandedDocument, error) {
	preparedDocuments, err := l.prepareSourceDocuments(ctx, sources, recursive, prepareForLint)
	if err != nil {
		return nil, err
	}

	documents := make([]ExpandedDocument, 0, len(preparedDocuments))
	for _, prepared := range preparedDocuments {
		var root yaml.Node
		if err := yaml.Unmarshal(prepared.content, &root); err != nil {
			return nil, fmt.Errorf("failed to parse expanded YAML in %s: %w", prepared.sourcePath, err)
		}
		documents = append(documents, ExpandedDocument{
			SourcePath: prepared.sourcePath,
			Content:    prepared.content,
			Empty: isEmptyYAMLDocument(&root) ||
				(root.Content[0].Kind == yaml.MappingNode && len(root.Content[0].Content) == 0),
		})
	}
	return documents, nil
}

func validateExpansionDocument(content []byte, source string) error {
	decoder := yaml.NewDecoder(bytes.NewReader(content))
	var document yaml.Node
	if err := decoder.Decode(&document); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("failed to parse YAML in %s: %w", source, err)
	}
	if err := decoder.Decode(&document); !errors.Is(err, io.EOF) {
		return fmt.Errorf("input %s must contain one YAML document; split documents into separate files", source)
	}
	return nil
}

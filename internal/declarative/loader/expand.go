package loader

import (
	"context"
	"fmt"

	"sigs.k8s.io/yaml"
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
// resolve references, validate, read environment values or fetch secrets.
func (l *Loader) ExpandSources(ctx context.Context, sources []Source, recursive bool) ([]ExpandedDocument, error) {
	preparedDocuments, err := l.prepareSourceDocuments(ctx, sources, recursive)
	if err != nil {
		return nil, err
	}

	documents := make([]ExpandedDocument, 0, len(preparedDocuments))
	for _, prepared := range preparedDocuments {
		var root map[string]any
		if err := yaml.Unmarshal(prepared.content, &root); err != nil {
			return nil, fmt.Errorf("failed to parse expanded YAML in %s: %w", prepared.sourcePath, err)
		}
		documents = append(documents, ExpandedDocument{
			SourcePath: prepared.sourcePath,
			Content:    prepared.content,
			Empty:      len(root) == 0,
		})
	}
	return documents, nil
}
